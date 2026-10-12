package handler

import (
	"path/filepath"
	"strings"
	"testing"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"go.uber.org/zap"
)

func TestProjectBlackboardInitialInjectionUsesCurrentConversation(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "injection.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := db.CreateProject(&database.Project{Name: "same project"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateConversation("a", database.ConversationCreateMeta{ProjectID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.CreateConversation("b", database.ConversationCreateMeta{ProjectID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []*database.ProjectFact{
		{ProjectID: p.ID, FactKey: "note/context-a", SourceConversationID: a.ID, Summary: "own continuation"},
		{ProjectID: p.ID, FactKey: "note/context-b", SourceConversationID: b.ID, Summary: "other continuation"},
		{ProjectID: p.ID, FactKey: "finding/shared", Category: "finding", Summary: "shared result", Confidence: "confirmed"},
	} {
		if _, err := db.UpsertProjectFact(f); err != nil {
			t.Fatal(err)
		}
	}
	h := &AgentHandler{db: db, config: &config.Config{Project: config.ProjectConfig{Enabled: true}}, logger: zap.NewNop()}
	block := h.projectBlackboardBlock(a.ID)
	if !strings.Contains(block, "note/context-a") || !strings.Contains(block, "finding/shared") || strings.Contains(block, "note/context-b") {
		t.Fatalf("initial prompt mixed sessions: %s", block)
	}
}
