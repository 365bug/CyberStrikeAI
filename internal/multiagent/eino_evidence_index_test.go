package multiagent

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
