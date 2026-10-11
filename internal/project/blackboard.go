package project

import (
	"fmt"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
)

// AppendSystemPromptBlock 将附加块追加到 system prompt。
func AppendSystemPromptBlock(base, block string) string {
	base = strings.TrimSpace(base)
	block = strings.TrimSpace(block)
	if block == "" {
		return base
	}
	if base == "" {
		return block
	}
	return base + "\n\n" + block
}

const (
	factIndexFooterGetDetail = "需要完整内容（攻击链、POC、请求响应等）时必须调用 get_project_fact(fact_key)，禁止凭摘要臆造细节。"
	factIndexFooterWriteHint = "写入事实 links 时用 from（来源 fact_key → 当前 fact），如 finding 上 {from:target/*, type:discovered_on}；body 写可复现全流程（发现/利用类 fact_key 建议 finding|chain|exploit|poc/ 前缀）。"
	factIndexFooterEmpty     = "需要写入请使用 upsert_project_fact；需要详情请调用 get_project_fact(fact_key)。"
)

// BuildFactIndexBlock 为 Agent 系统提示生成项目黑板索引（key + summary + 关系边 + 攻击路径，不含 body）。
func BuildFactIndexBlock(db *database.DB, projectID string, cfg config.ProjectConfig) (string, error) {
	if db == nil || !cfg.Enabled {
		return "", nil
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", nil
	}

	proj, err := db.GetProject(projectID)
	if err != nil {
		return "", err
	}

	facts, err := db.ListProjectFactsForIndex(projectID, cfg.DefaultInjectDeprecated)
	if err != nil {
		return "", err
	}
	allEdges, _ := db.ListProjectFactEdgesByProject(projectID)
	_, incomingByTarget := indexEdgeGroupMaps(allEdges)

	maxRunes := cfg.FactIndexMaxRunesEffective()
	header := fmt.Sprintf("## 项目黑板索引（project: %s, id: %s）\n", indexSingleLine(proj.Name), proj.ID)
	if len(facts) == 0 {
		block := wrapFactIndexBlock(header + "（暂无事实）\n" + factIndexFooterEmpty)
		if len([]rune(block)) <= maxRunes {
			return block, nil
		}
		return minimalFactIndexBlock(maxRunes), nil
	}

	sortFactsForIndex(facts)

	pathMaxRunes := cfg.FactIndexPathMaxRunesEffective()
	footer := factIndexFooterGetDetail + "\n" + factIndexFooterWriteHint
	// Include markers, separators and the worst-case omission notice in the cap.
	omissionReserve := len([]rune(factIndexOmissionNotice(len(facts))))
	framingRunes := len([]rune(wrapFactIndexBlock(header+footer))) + omissionReserve
	available := maxRunes - framingRunes
	if available <= 0 {
		return minimalFactIndexBlock(maxRunes), nil
	}
	if pathMaxRunes > available {
		pathMaxRunes = available
	}
	factsBudget := available - pathMaxRunes
	if factsBudget < 800 {
		factsBudget = available
		pathMaxRunes = 0
	}

	indexedKeys := make(map[string]struct{}, len(facts))
	var b strings.Builder
	b.WriteString(header)
	used := 0
	omitted := 0

	for _, f := range facts {
		line := fmt.Sprintf("- [%s] %s — %s (%s)", f.FactKey, f.Category, truncateIndexSummary(indexSingleLine(f.Summary), cfg.FactSummaryMaxRunesEffective()), f.Confidence)
		line += FormatFactIndexLinksHint(f.FactKey, incomingByTarget[f.FactKey])
		line += "\n"
		lineRunes := len([]rune(line))
		if used+lineRunes > factsBudget {
			omitted++
			continue
		}
		indexedKeys[f.FactKey] = struct{}{}
		b.WriteString(line)
		used += lineRunes
	}

	if omitted > 0 {
		b.WriteString(factIndexOmissionNotice(omitted))
	}

	if pathSection := BuildFactPathOverviewSection(allEdges, indexedKeys, pathMaxRunes-1); len(indexedKeys) > 0 && pathSection != "" {
		b.WriteString("\n")
		b.WriteString(pathSection)
	}

	b.WriteString(footer)
	return wrapFactIndexBlock(b.String()), nil
}

func indexSingleLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func truncateIndexSummary(s string, maxRunes int) string {
	runes := []rune(s)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes-1]) + "…"
	}
	return s
}

func factIndexOmissionNotice(count int) string {
	return fmt.Sprintf("\n（另有 %d 条未列入索引，请使用 list_project_facts 或 search_project_facts 查询。）\n", count)
}

func minimalFactIndexBlock(maxRunes int) string {
	block := wrapFactIndexBlock("## 项目黑板索引\n索引预算不足；使用 list_project_facts 或 search_project_facts 按需查询。")
	if len([]rune(block)) > maxRunes {
		return ""
	}
	return block
}
