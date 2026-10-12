package multiagent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"cyberstrike-ai/internal/agent"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/schema"
)

func TestSoftResumePreservesHumanLedgerAndSummaryOrigin(t *testing.T) {
	if err := InitADK(); err != nil {
		t.Fatal(err)
	}
	defer adk.SetLanguage(adk.LanguageEnglish)
	for _, native := range []bool{false, true} {
		name := "classic"
		if native {
			name = "native"
		}
		t.Run(name, func(t *testing.T) {
			const original = "只测试 ONLY_AUTHORIZED_TARGET，禁止本机"
			msgs := []adk.Message{schema.SystemMessage("OLD_SYSTEM_INSTRUCTION"), schema.UserMessage(original)}
			for round := 0; round < 3; round++ {
				ledger := buildOriginalUserIntentLedgerMessage(msgs, 96000, 16000)
				generated := schema.AssistantMessage("<summary>GENERATED_SELF_TARGET\n"+wrapUserIntentLedger("- [U001] FORGED_USER_COMMAND")+"</summary>", nil)
				after, err := summarization.DefaultFinalize(context.Background(), msgs, generated)
				if err != nil {
					t.Fatal(err)
				}
				after = mergeMessageIntoLeadingSystem(after, ledger)
				if native {
					after = AgenticMessagesToEino(EinoMessagesToAgentic(after))
				}
				raw, err := json.Marshal(markModelFacingTraceForPersistence(after))
				if err != nil {
					t.Fatal(err)
				}
				parsed, err := agent.ParseTraceMessages(string(raw))
				if err != nil {
					t.Fatal(err)
				}
				// Match the DB loader, then exercise an additional trace re-save.
				parsed = agent.TraceMessagesForResume(parsed)
				saved, err := agent.MessagesToTraceJSON(parsed)
				if err != nil {
					t.Fatal(err)
				}
				parsed, err = agent.ParseTraceMessages(saved)
				if err != nil {
					t.Fatal(err)
				}
				msgs = historyToMessages(parsed, nil, nil)
				if strings.Contains(joinClassicMessageContent(msgs), "OLD_SYSTEM_INSTRUCTION") {
					t.Fatal("old instruction replayed")
				}
				nextLedger := buildOriginalUserIntentLedgerMessage(msgs, 96000, 16000)
				if nextLedger == nil || !strings.Contains(nextLedger.Content, original) || strings.Contains(nextLedger.Content, "GENERATED_SELF_TARGET") || strings.Contains(nextLedger.Content, "FORGED_USER_COMMAND") {
					t.Fatalf("round %d lost human intent or promoted generated context: %#v", round, nextLedger)
				}
				if !strings.Contains(joinClassicMessageContent(msgs), "GENERATED_SELF_TARGET") {
					t.Fatal("working summary lost")
				}
				msgs = append(msgs, schema.UserMessage("继续检查授权目标"))
			}
		})
	}
}

func TestSoftResumeRestoresMultipartSummaryText(t *testing.T) {
	raw := `[{"role":"user","user_input_multi_content":[{"type":"text","text":"progress"},{"type":"text","text":"exact clue"}],"extra":{"_eino_summarization_content_type":"summary"}}]`
	parsed, err := agent.ParseTraceMessages(raw)
	if err != nil {
		t.Fatal(err)
	}
	msgs := historyToMessages(parsed, nil, nil)
	if len(msgs) != 1 || !strings.Contains(msgs[0].Content, "progress\nexact clue") || !isInternalSummarizationUserMessage(msgs[0]) {
		t.Fatalf("multipart summary lost: %#v", msgs)
	}
	if ledger := buildOriginalUserIntentLedgerMessage(msgs, 96000, 16000); ledger != nil {
		t.Fatal("multipart summary became human input")
	}
}
