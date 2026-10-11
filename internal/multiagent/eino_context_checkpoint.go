package multiagent

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// Content-addressed snapshots keep earlier evidence reachable after the mutable
// transcript.txt alias advances to a later compaction round.
func writeSummarizationTranscriptSnapshot(path string, msgs []adk.Message) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	body := []byte(formatSummarizationTranscript(msgs))
	dir := filepath.Join(filepath.Dir(path), "snapshots")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	digest := sha256.Sum256(body)
	snapshot := filepath.Join(dir, fmt.Sprintf("transcript-%x.txt", digest))
	if info, err := os.Stat(snapshot); os.IsNotExist(err) {
		if err := writeContextArtifactAtomically(snapshot, body); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else if !info.Mode().IsRegular() {
		return "", fmt.Errorf("snapshot is not a regular file: %s", snapshot)
	}
	// One immutable companion per snapshot avoids a shared mutable catalog and
	// keeps discovery scoped to this conversation/agent, including concurrent runs.
	indexPath := snapshot + ".index.md"
	if info, err := os.Stat(indexPath); os.IsNotExist(err) {
		index := buildTranscriptEvidenceIndex(snapshot, string(body), 32)
		if err := writeContextArtifactAtomically(indexPath, []byte(index)); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else if !info.Mode().IsRegular() {
		return "", fmt.Errorf("evidence index is not a regular file: %s", indexPath)
	}
	if err := writeContextArtifactAtomically(path, body); err != nil {
		return "", err
	}
	return snapshot, nil
}

func writeContextArtifactAtomically(path string, body []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".context-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err = file.Write(body); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// Persist generated working memory separately from verified project findings.
// One key per conversation/agent scope is updated; retries do not grow the fact index.
func persistSummarizationCheckpoint(db *database.DB, projectID, conversationID, memoryScope, summary, snapshot string, cfg config.ProjectConfig) (string, error) {
	if db == nil || !cfg.Enabled || !cfg.ContextCheckpointsEnabled() || strings.TrimSpace(projectID) == "" || strings.TrimSpace(conversationID) == "" || strings.TrimSpace(summary) == "" {
		return "", nil
	}
	// Bind storage to the current conversation rather than trusting a stale run's project ID.
	bound, err := db.GetConversationProjectID(conversationID)
	if err != nil {
		return "", err
	}
	if bound != projectID {
		return "", fmt.Errorf("checkpoint project does not match conversation")
	}
	hash := sha256.Sum256([]byte(conversationID + "\x00" + memoryScope))
	key := fmt.Sprintf("note/context-%x", hash[:16])
	body := "## 会话续接记录（自动摘要，尚未经独立验证）\n来源对话: " + conversationID + "\n代理范围: " + memoryScope + "\n\n" + summary
	if snapshot != "" {
		body += "\n\n压缩前原始证据快照（按需 read_file）: " + snapshot
	}
	_, err = db.UpsertProjectFact(&database.ProjectFact{
		ProjectID: projectID, FactKey: key, Category: "note", Confidence: "tentative",
		SourceConversationID: conversationID,
		Summary:              checkpointSummaryLabel(memoryScope, summary, cfg.FactSummaryMaxRunesEffective()),
		Body:                 body,
	})
	if err != nil {
		return "", err
	}
	return key, nil
}

func checkpointSummaryLabel(scope, summary string, cap int) string {
	// A generated note must not turn a permissive fact-summary setting into
	// automatic reinjection of the entire compaction output.
	if cap > 200 {
		cap = 200
	}
	// Keep retrieval cues in the short blackboard label without injecting paths
	// or the full evidence index into every model request.
	evidenceHint := ""
	if start := strings.Index(summary, evidenceIndexMarker); start >= 0 {
		section := summary[start:]
		if end := strings.Index(section, evidenceIndexEnd); end >= 0 {
			section = section[:end]
		}
		var topics []string
		for _, topic := range evidenceTopics {
			if strings.Contains(section, "- "+topic.label+"：行 ") {
				topics = append(topics, topic.label)
			}
		}
		if len(topics) > 0 {
			evidenceHint = "；证据索引：" + strings.Join(topics, "、")
		}
		summary = summary[:start]
	}
	var lines []string
	for _, line := range strings.Split(summary, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "<") {
			continue
		}
		lines = append(lines, strings.TrimLeft(line, "- *"))
		if len(lines) == 4 {
			break
		}
	}
	label := "自动续接（" + strings.Join(strings.Fields(scope), " ") + "）：" + strings.Join(lines, "；")
	available := cap - len([]rune(evidenceHint))
	if available < 1 {
		return truncateIndexLabel(label+evidenceHint, cap)
	}
	return truncateIndexLabel(label, available) + evidenceHint
}

func truncateIndexLabel(text string, cap int) string {
	if cap < 1 {
		return ""
	}
	runes := []rune(text)
	if len(runes) > cap {
		return string(runes[:cap-1]) + "…"
	}
	return text
}

func finalizeSummarizationMemory(msg adk.Message, originals []adk.Message, transcriptPath string, db *database.DB, projectID, conversationID, memoryScope string, cfg config.ProjectConfig, logger *zap.Logger) adk.Message {
	msg = stripAnalysisFromSummarizationMessage(msg)
	snapshot, err := writeSummarizationTranscriptSnapshot(transcriptPath, originals)
	if err != nil && logger != nil {
		logger.Warn("保存上下文证据快照失败", zap.Error(err))
	}
	// Only advertise an artifact that was successfully written.
	evidenceIndex := ""
	if err == nil {
		msg = appendTranscriptPathToSummarizationMessage(msg, snapshot)
		if snapshot != "" {
			evidenceIndex = buildTranscriptEvidenceIndex(snapshot, formatSummarizationTranscript(originals), 3)
			msg = appendEvidenceIndexToMessage(msg, evidenceIndex)
		}
	}
	text := ""
	if msg != nil {
		text = msg.Content
		if text == "" {
			for _, part := range msg.UserInputMultiContent {
				if part.Type == schema.ChatMessagePartTypeText {
					text = part.Text
					break
				}
			}
		}
	}
	if inner, ok := extractSummarizationSummaryBody(text); ok {
		text = inner
	}
	if evidenceIndex != "" {
		text = appendEvidenceIndex(text, evidenceIndex)
	}
	key, err := persistSummarizationCheckpoint(db, projectID, conversationID, memoryScope, text, snapshot, cfg)
	if err != nil {
		if logger != nil {
			logger.Warn("保存黑板续接记录失败", zap.String("conversationId", conversationID), zap.Error(err))
		}
		return msg
	}
	if key == "" || msg == nil {
		return msg
	}
	section := "本轮续接摘要已保存到项目黑板 get_project_fact(" + key + ")；这是未独立验证的工作记录，使用前核对证据与当前任务范围。"
	clone := *msg
	if clone.Content != "" {
		clone.Content = appendSummarizationSection(clone.Content, section)
	}
	if len(clone.UserInputMultiContent) > 0 {
		clone.UserInputMultiContent = append([]schema.MessageInputPart(nil), clone.UserInputMultiContent...)
		for i, part := range clone.UserInputMultiContent {
			if part.Type == schema.ChatMessagePartTypeText {
				clone.UserInputMultiContent[i].Text = appendSummarizationSection(part.Text, section)
				break
			}
		}
	}
	return &clone
}

func summarizationMemoryScope(scopes []string) string {
	if len(scopes) > 0 && strings.TrimSpace(scopes[0]) != "" {
		return strings.TrimSpace(scopes[0])
	}
	return "main"
}

func summarizationScopeID(scope string) string {
	hash := sha256.Sum256([]byte(scope))
	return fmt.Sprintf("%x", hash[:16])
}
