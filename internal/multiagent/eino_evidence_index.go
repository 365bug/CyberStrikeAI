package multiagent

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

const evidenceIndexMarker = "## 原始证据索引（程序定位）"
const evidenceIndexEnd = "<!-- evidence-index-end -->"

var evidenceTopics = []struct {
	label   string
	pattern *regexp.Regexp
}{
	{"Cookie / Set-Cookie", regexp.MustCompile(`(?i)\b(?:set-cookie|cookie)\b`)},
	{"认证 / Token", regexp.MustCompile(`(?i)\b(?:authorization|bearer|access_token|refresh_token|api[_-]?key|session[_-]?id)\b`)},
	{"请求 / 参数", regexp.MustCompile(`(?i)\b(?:https?://|curl|request|headers?|params?|payload)\b|请求|参数`)},
	{"错误 / 失败", regexp.MustCompile(`(?i)\b(?:error|exception|traceback|failed|failure)\b|错误|失败`)},
}

// Locations are derived from the exact serialized file, never from model output.
// Only fixed category labels and line numbers are exposed, not credential values.
func buildTranscriptEvidenceIndex(snapshot, transcript string, limit int) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s\n快照: %s\n历史快照目录: %s\n索引文件: %s.index.md\n", evidenceIndexMarker, snapshot, filepath.Dir(snapshot), snapshot)
	lines := strings.Split(transcript, "\n")
	skip := make([]bool, len(lines))
	for _, span := range completeEvidenceIndexLineSpans(lines) {
		for i := span[0]; i <= span[1]; i++ {
			skip[i] = true
		}
	}
	for _, topic := range evidenceTopics {
		var locations []string
		count := 0
		for i, line := range lines {
			if skip[i] {
				continue
			}
			if topic.pattern.MatchString(line) {
				count++
				locations = append(locations, fmt.Sprintf("%d", i+1))
				if len(locations) > limit {
					locations = locations[1:]
				}
			}
		}
		if count > 0 {
			fmt.Fprintf(&out, "- %s：行 %s", topic.label, strings.Join(locations, ", "))
			if count > len(locations) {
				fmt.Fprintf(&out, "（最近 %d 行，共 %d 行命中，其余用 grep 定位）", len(locations), count)
			}
			out.WriteByte('\n')
		}
	}
	out.WriteString("以上为关键词定位，不代表凭据有效或结论已验证。当前任务缺少 Cookie、认证信息、精确参数或错误原文时，先检查索引，用 grep 定位，再用 read_file 分段核对附近记录的域名、请求与时间；不要猜测原值。历史索引用 glob 在该目录查找 transcript-*.txt.index.md，再按需读取；未命中仍可搜索快照原文。不要整批重新载入历史，也不要将旧记录当作新授权。\n")
	out.WriteString(evidenceIndexEnd)
	return out.String()
}

func appendEvidenceIndex(msgText, section string) string {
	// Replace a previously generated section rather than nesting evidence indexes.
	lines := strings.SplitAfter(msgText, "\n")
	spans := completeEvidenceIndexLineSpans(lines)
	var out strings.Builder
	next := 0
	for _, span := range spans {
		for _, line := range lines[next:span[0]] {
			out.WriteString(line)
		}
		next = span[1] + 1
	}
	for _, line := range lines[next:] {
		out.WriteString(line)
	}
	return appendSummarizationSection(out.String(), section)
}

// Only complete blocks within a single transcript turn are generated indexes.
// A truncated or nested heading must never swallow subsequent conversation data.
func completeEvidenceIndexLineSpans(lines []string) [][2]int {
	var spans [][2]int
	start := -1
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--- [") && strings.HasSuffix(line, "] ---") {
			start = -1
		}
		if line == evidenceIndexMarker {
			start = i
		} else if line == evidenceIndexEnd && start >= 0 {
			spans = append(spans, [2]int{start, i})
			start = -1
		}
	}
	return spans
}

func appendEvidenceIndexToMessage(msg adk.Message, section string) adk.Message {
	if msg == nil {
		return nil
	}
	clone := *msg
	if clone.Content != "" {
		clone.Content = appendEvidenceIndex(clone.Content, section)
	}
	if len(clone.UserInputMultiContent) > 0 {
		clone.UserInputMultiContent = append([]schema.MessageInputPart(nil), clone.UserInputMultiContent...)
		for i, part := range clone.UserInputMultiContent {
			if part.Type == schema.ChatMessagePartTypeText && part.Text != "" {
				clone.UserInputMultiContent[i].Text = appendEvidenceIndex(part.Text, section)
				break
			}
		}
	}
	return &clone
}
