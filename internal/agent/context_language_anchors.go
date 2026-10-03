package agent

import (
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

// Short owner/member anchors are usable only when one indexed declaration matches.
// Overloads and matching owners in different modules remain ambiguous.
func contextUniqueLanguageAnchors(facts []scan.AgentContextFactRecord, anchors []string) map[string]int {
	requested := map[string]int{}
	for index, anchor := range anchors {
		anchor = strings.TrimRight(strings.TrimSpace(anchor), ".!?")
		anchor = strings.TrimSuffix(anchor, "()")
		if strings.Contains(anchor, ".") && !strings.ContainsAny(anchor, "/\\() :") {
			term := normalizeContextTerm(anchor)
			if requested[term] == 0 {
				requested[term] = len(anchors) - index
			}
		}
	}
	matches := map[string][]string{}
	for _, fact := range facts {
		ext := strings.ToLower(filepath.Ext(fact.File))
		if fact.Kind != "symbol" || ext != ".cs" && ext != ".swift" {
			continue
		}
		qualified, _, _ := strings.Cut(fact.Qualified, "(")
		if strings.Contains(qualified, ":") {
			continue
		}
		parts := strings.Split(qualified, ".")
		for start := 0; start+1 < len(parts); start++ {
			anchor := normalizeContextTerm(strings.Join(parts[start:], "."))
			if requested[anchor] > 0 {
				matches[anchor] = append(matches[anchor], fact.ID)
			}
		}
	}
	result := map[string]int{}
	for anchor, ids := range matches {
		if len(ids) == 1 {
			result[ids[0]] = max(result[ids[0]], requested[anchor])
		}
	}
	return result
}
