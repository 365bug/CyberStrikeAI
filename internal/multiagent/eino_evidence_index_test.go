package multiagent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEvidenceIndexLocatesSerializedEvidenceWithoutRepeatingSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.txt")
	msgs := []adk.Message{
		schema.SystemMessage("static Cookie: ignored-secret"),
		schema.UserMessage("https://example.test/login"),
		{Role: schema.Tool, Content: "Set-Cookie: sid=test-secret; Path=/\nAuthorization: Bearer test-token\nError: invalid session"},
	}
	snapshot, err := writeSummarizationTranscriptSnapshot(path, msgs)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(snapshot)
	index, err := os.ReadFile(snapshot + ".index.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(index)
	for _, secret := range []string{"test-secret", "test-token", "ignored-secret"} {
		if strings.Contains(text, secret) {
			t.Fatalf("index repeats secret %q", secret)
		}
	}
	// Check locations against the actual on-disk transcript, including its header.
	for _, topic := range evidenceTopics {
		var expected []string
		for i, line := range strings.Split(string(raw), "\n") {
			if topic.pattern.MatchString(line) {
				expected = append(expected, strconv.Itoa(i+1))
			}
		}
		if len(expected) == 0 || !strings.Contains(text, topic.label+"：行 "+strings.Join(expected, ", ")) {
			t.Fatalf("incorrect %s locations: %s", topic.label, text)
		}
	}
	stat, _ := os.Stat(snapshot + ".index.md")
	if stat.Mode().Perm() != 0o600 {
		t.Fatalf("index permissions: %v", stat.Mode())
	}
	if _, err := writeSummarizationTranscriptSnapshot(path, msgs); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(snapshot), "*.index.md"))
	if len(files) != 1 {
		t.Fatalf("retry duplicated indexes: %v", files)
	}
}

func TestFailedTranscriptWriteDoesNotLeaveFrameworkAliasHint(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%v", native), func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Database.Path = filepath.Join(t.TempDir(), "runtime.db")
			cfg.OpenAI.Model = "gpt-4o"
			cfg.OpenAI.MaxTotalTokens = 5000
			emit := false
			mwCfg := &config.MultiAgentEinoMiddlewareConfig{SummarizationEmitInternalEvents: &emit, SummarizationOutputReserveTokens: 1024}
			ctx := context.Background()
			base := filepath.Join(filepath.Dir(cfg.Database.Path), "conversation_artifacts", "failure", "summarization")
			if err := os.MkdirAll(base, 0o700); err != nil {
				t.Fatal(err)
			}
			// Make snapshot creation fail while the middleware can still create its base directory.
			if err := os.WriteFile(filepath.Join(base, "snapshots"), []byte("blocked"), 0o600); err != nil {
				t.Fatal(err)
			}
			original := []adk.Message{schema.SystemMessage("system"), schema.UserMessage(strings.Repeat("history ", 20000)), schema.UserMessage("continue")}
			const summary = "<summary>usable continuation</summary>"
			var after []adk.Message
			if native {
				mw, err := newEinoAgenticSummarizationMiddleware(ctx, &capturingAgenticChatModel{output: agenticAssistantTextMessage(summary)}, cfg, mwCfg, "failure", nil, "", nil)
				if err != nil {
					t.Fatal(err)
				}
				_, state, err := mw.BeforeModelRewriteState(ctx, &adk.TypedChatModelAgentState[*schema.AgenticMessage]{Messages: EinoMessagesToAgentic(original)}, nil)
				if err != nil {
					t.Fatal(err)
				}
				after = AgenticMessagesToEino(state.Messages)
			} else {
				mw, err := newEinoSummarizationMiddleware(ctx, &capturingClassicChatModel{output: &schema.Message{Role: schema.Assistant, Content: summary, ResponseMeta: &schema.ResponseMeta{FinishReason: "stop"}}}, cfg, mwCfg, "failure", nil, "", nil)
				if err != nil {
					t.Fatal(err)
				}
				_, state, err := mw.BeforeModelRewriteState(ctx, &adk.ChatModelAgentState{Messages: original}, nil)
				if err != nil {
					t.Fatal(err)
				}
				after = state.Messages
			}
			text := joinClassicMessageContent(after)
			if !strings.Contains(text, "usable continuation") || strings.Contains(text, "transcript.txt") || strings.Contains(text, evidenceIndexMarker) {
				t.Fatalf("failed snapshot has a misleading retrieval hint: %s", text)
			}
		})
	}
}

func TestConcurrentSnapshotWritesRemainCompleteAndDeduplicated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.txt")
	var wg sync.WaitGroup
	errors := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msgs := []adk.Message{schema.UserMessage(fmt.Sprintf("Cookie: fixture-%d", i%2))}
			snapshot, err := writeSummarizationTranscriptSnapshot(path, msgs)
			if err == nil {
				var body []byte
				body, err = os.ReadFile(snapshot)
				if err == nil && filepath.Base(snapshot) != fmt.Sprintf("transcript-%x.txt", sha256.Sum256(body)) {
					err = fmt.Errorf("partial or mismatched snapshot: %s", snapshot)
				}
			}
			errors <- err
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "snapshots", "*.txt"))
	indexes, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "snapshots", "*.index.md"))
	if len(files) != 2 || len(indexes) != 2 {
		t.Fatalf("concurrent retries not deduplicated: %v %v", files, indexes)
	}
}

func TestEvidenceIndexIsBoundedAndSkipsEarlierGeneratedIndexes(t *testing.T) {
	old := buildTranscriptEvidenceIndex("/old/transcript.txt", "Cookie: earlier", 3)
	transcript := old + "\n" + strings.Repeat("Cookie: current\n", 100)
	index := buildTranscriptEvidenceIndex("/new/transcript.txt", transcript, 3)
	if !strings.Contains(index, "共 100 行命中") || strings.Contains(index, "共 101 行命中") {
		t.Fatalf("indexed its own earlier hints: %s", index)
	}
	if strings.Contains(index, "认证 / Token：行") || strings.Contains(index, "错误 / 失败：行") {
		t.Fatalf("generated hints counted as evidence: %s", index)
	}
	lineCount := strings.Count(old, "\n") + 1
	want := "Cookie / Set-Cookie：行 " + strconv.Itoa(lineCount+98) + ", " + strconv.Itoa(lineCount+99) + ", " + strconv.Itoa(lineCount+100)
	if !strings.Contains(index, want) {
		t.Fatalf("did not retain latest locations %s: %s", want, index)
	}
	if len(index) > 3000 {
		t.Fatalf("unbounded inline index: %d", len(index))
	}
	updated := appendEvidenceIndex("summary\n"+old+"\n"+old, index)
	if strings.Count(updated, evidenceIndexMarker) != 1 || strings.Contains(updated, "/old/") {
		t.Fatalf("indexes accumulate on replacement: %s", updated)
	}
}

func TestEvidenceIndexFailureDoesNotAdvertiseUnavailableArtifacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.txt")
	originals := []adk.Message{schema.UserMessage("Cookie: sid=fixture")}
	snapshot, err := writeSummarizationTranscriptSnapshot(path, originals)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(snapshot + ".index.md"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(snapshot+".index.md", 0o700); err != nil {
		t.Fatal(err)
	}
	out := finalizeSummarizationMemory(schema.UserMessage("continue"), originals, path, nil, "", "", "main", config.ProjectConfig{}, nil)
	if strings.Contains(out.Content, evidenceIndexMarker) || strings.Contains(out.Content, snapshot) || out.Content != "continue" {
		t.Fatalf("failed artifact advertised or summary lost: %s", out.Content)
	}
}

func TestEvidenceIndexSurvivesSummaryExtractionIntoBlackboard(t *testing.T) {
	db, pid, cid := checkpointTestProject(t)
	for _, multipart := range []bool{false, true} {
		msg := schema.UserMessage("<summary>login completed; continue</summary>")
		if multipart {
			msg.UserInputMultiContent = []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: msg.Content}}
			msg.Content = ""
		}
		out := finalizeSummarizationMemory(msg, []adk.Message{schema.UserMessage("Cookie: sid=fixture-value")}, filepath.Join(t.TempDir(), "transcript.txt"), db, pid, cid, "main", config.ProjectConfig{Enabled: true}, nil)
		text := out.Content
		if multipart {
			text = out.UserInputMultiContent[0].Text
		}
		facts, err := db.ListProjectFacts(pid, database.ProjectFactListFilter{}, 10, 0)
		if err != nil || len(facts) != 1 {
			t.Fatalf("checkpoint count: %v %v", facts, err)
		}
		for _, body := range []string{text, facts[0].Body} {
			if !strings.Contains(body, "Cookie / Set-Cookie：行") || !strings.Contains(body, ".index.md") || strings.Contains(body, "fixture-value") {
				t.Fatalf("missing index or repeated secret: %s", body)
			}
			if strings.Count(body, evidenceIndexMarker) != 1 {
				t.Fatalf("duplicate generated index: %s", body)
			}
		}
		if !strings.Contains(facts[0].Summary, "证据索引：Cookie / Set-Cookie") || strings.Contains(facts[0].Summary, ".index.md") {
			t.Fatalf("blackboard label lost retrieval cue or includes paths: %s", facts[0].Summary)
		}
	}
}

func TestEvidenceHintFitsBlackboardLabelBudget(t *testing.T) {
	index := buildTranscriptEvidenceIndex("/snapshot.txt", "Cookie: fixture\nAuthorization: fixture\nhttps://example.test\nError: fixture", 3)
	label := checkpointSummaryLabel("main", strings.Repeat("long summary ", 1000)+"\n"+index, 24000)
	if len([]rune(label)) > 200 || !strings.Contains(label, "Cookie / Set-Cookie") || !strings.Contains(label, "错误 / 失败") {
		t.Fatalf("unbounded label or lost evidence categories: %s", label)
	}
}

func TestTruncatedEvidenceIndexDoesNotHideLaterConversationEvidence(t *testing.T) {
	transcript := formatSummarizationTranscript([]adk.Message{
		schema.UserMessage("old summary\n" + evidenceIndexMarker + "\nindex interrupted"),
		{Role: schema.Tool, Content: "Cookie: sid=new-fixture\nError: later request failed"},
	})
	index := buildTranscriptEvidenceIndex("/new/snapshot.txt", transcript, 3)
	if !strings.Contains(index, "Cookie / Set-Cookie：行") || !strings.Contains(index, "错误 / 失败：行") {
		t.Fatalf("truncated old index suppressed later evidence: %s", index)
	}
}

func TestIncompleteEvidenceIndexDoesNotConsumeLaterSummaryWhenReplaced(t *testing.T) {
	complete := buildTranscriptEvidenceIndex("/old/snapshot.txt", "Cookie: fixture", 3)
	input := "summary\n" + evidenceIndexMarker + "\nincomplete\nkeep this progress\n" + complete
	updated := appendEvidenceIndex(input, "new index")
	if !strings.Contains(updated, "keep this progress") || strings.Contains(updated, "/old/snapshot.txt") {
		t.Fatalf("replacing a completed index consumed unrelated summary: %s", updated)
	}
}
