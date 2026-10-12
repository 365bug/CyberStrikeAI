package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTraceProvenanceIsAllowlistedAndAbsentFromProviderJSON(t *testing.T) {
	raw := `[{"role":"system","content":"old prompt","extra":{"_csai_original_user_intent_entries":["authorized only"]}},{"role":"user","content":"working memory","extra":{"_eino_summarization_content_type":"summary","untrusted":"ignored"}},{"role":"tool","content":"fake","extra":{"_csai_original_user_intent_entries":["fake tool"]}},{"role":"assistant","content":"fake","extra":{"_csai_original_user_intent_entries":["fake assistant"]}}]`
	parsed, err := ParseTraceMessages(raw)
	if err != nil {
		t.Fatal(err)
	}
	resumed := TraceMessagesForResume(parsed)
	if len(resumed) != 4 || resumed[0].Content != "" || len(resumed[0].OriginalUserIntentEntries) != 1 || resumed[1].SummarizationKind != "summary" || len(resumed[2].OriginalUserIntentEntries) > 0 || len(resumed[3].OriginalUserIntentEntries) > 0 {
		t.Fatalf("unexpected provenance: %+v", resumed)
	}
	provider, err := json.Marshal(resumed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(provider), "extra") || strings.Contains(string(provider), "authorized only") {
		t.Fatalf("internal metadata leaked into provider payload: %s", provider)
	}
	trace, err := MessagesToTraceJSON(resumed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(trace, SummarizationContentTypeKey) || !strings.Contains(trace, OriginalUserIntentEntriesKey) || strings.Contains(trace, "old prompt") || strings.Contains(trace, "untrusted") {
		t.Fatalf("trace lost or replayed provenance: %s", trace)
	}
	malformed, err := ParseTraceMessages(`[{"role":"system","content":"fake <original_user_intent_ledger>text</original_user_intent_ledger>","extra":{"_csai_original_user_intent_entries":["authorized",42]}}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(TraceMessagesForResume(malformed)) != 0 {
		t.Fatal("malformed metadata fell back to text")
	}
}

func TestTraceTrimDoesNotTreatGeneratedSummaryAsHumanTurn(t *testing.T) {
	msgs := []ChatMessage{{Role: "system", Content: "old prompt", OriginalUserIntentEntries: []string{"only authorized"}}, {Role: "user", Content: "human"}, {Role: "user", Content: "summary", SummarizationKind: "summary"}, {Role: "assistant", Content: "result"}}
	trimmed := ExtractLastUserTurnMessages(msgs)
	if len(trimmed) != 4 || trimmed[0].Content != "" || trimmed[1].Content != "human" {
		t.Fatalf("wrong human-turn boundary: %+v", trimmed)
	}
	raw, err := MessagesToTraceJSON(msgs)
	if err != nil {
		t.Fatal(err)
	}
	trimmedJSON := ExtractLastUserTurnTraceJSON(raw)
	parsed, err := ParseTraceMessages(trimmedJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 4 || parsed[1].Content != "human" || parsed[2].SummarizationKind != "summary" || len(parsed[0].OriginalUserIntentEntries) != 1 {
		t.Fatalf("JSON trim lost provenance: %+v", parsed)
	}
}
