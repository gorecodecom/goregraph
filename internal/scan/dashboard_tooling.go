package scan

import (
	"regexp"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/testresults"
)

// DashboardToolingObservation identifies a literal declaration, not runtime state.
type DashboardToolingObservation struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Line  int    `json:"line"`
}

// DashboardToolingSource keeps navigation evidence out of the agent response.
type DashboardToolingSource struct {
	AgentAuditSource
	Observations []DashboardToolingObservation `json:"observations,omitempty"`
}

// DashboardToolingRecord is the optional offline tooling inventory for one project.
type DashboardToolingRecord struct {
	Version   int                      `json:"version"`
	Sources   []DashboardToolingSource `json:"sources"`
	Total     int                      `json:"total"`
	Truncated bool                     `json:"truncated,omitempty"`
}

var toolingA11yMode = regexp.MustCompile(`\ba11y\s*:\s*\{[^{}]*?\btest\s*:\s*['"](off|error|todo)['"]`)
var toolingFailurePolicy = regexp.MustCompile(`^\s*allow_failure\s*:\s*(true|false)\s*(?:#.*)?$`)

func extractDashboardToolingObservations(source AgentAuditSource, body string) []DashboardToolingObservation {
	var result []DashboardToolingObservation
	if source.Kind == "configuration" || source.Kind == "story" {
		lexical := scanJSLexicalSource(body)
		for _, match := range toolingA11yMode.FindAllStringSubmatchIndex(lexical.comments, 32) {
			if match[0] >= len(lexical.code) || isJSSpace(lexical.code[match[0]]) {
				continue
			}
			result = append(result, DashboardToolingObservation{Kind: "a11y_test_literal", Value: body[match[2]:match[3]], Line: 1 + strings.Count(body[:match[0]], "\n")})
		}
	}
	if source.Kind == "ci" {
		// Ignore block-scalar bodies, where YAML-looking text can be shell input.
		blockIndent := -1
		for i, line := range strings.Split(body, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if blockIndent >= 0 && indent > blockIndent {
				continue
			}
			blockIndent = -1
			if strings.HasSuffix(trimmed, "|") || strings.HasSuffix(trimmed, ">") || strings.HasSuffix(trimmed, "|-") || strings.HasSuffix(trimmed, ">-") {
				blockIndent = indent
				continue
			}
			if match := toolingFailurePolicy.FindStringSubmatch(line); len(match) > 0 && len(result) < 32 {
				result = append(result, DashboardToolingObservation{Kind: "allow_failure_literal", Value: match[1], Line: i + 1})
			}
		}
	}
	return result
}

func buildDashboardTooling(sources []AgentAuditSource, observations map[string][]DashboardToolingObservation) DashboardToolingRecord {
	copied := append([]AgentAuditSource(nil), sources...)
	for i := range copied {
		copied[i].References = append([]AgentAuditReference(nil), copied[i].References...)
	}
	copied = finalizeAgentAuditSources(copied, "")
	byFile := map[string]AgentAuditSource{}
	selected := map[string]bool{}
	var queue []string
	for _, source := range copied {
		byFile[source.File] = source
		seed := len(source.Topics) > 0 && source.Kind != "ci"
		if source.Kind == "ci" {
			seed = source.File == ".gitlab-ci.yml" || strings.HasPrefix(source.File, ".github/workflows/")
		}
		if seed {
			selected[source.File] = true
			queue = append(queue, source.File)
		}
	}
	for cursor := 0; cursor < len(queue); cursor++ {
		for _, ref := range byFile[queue[cursor]].References {
			if _, ok := byFile[ref.File]; ok && !selected[ref.File] {
				selected[ref.File] = true
				queue = append(queue, ref.File)
			}
		}
	}
	sort.Strings(queue)
	record := DashboardToolingRecord{Version: 1, Total: len(queue), Sources: []DashboardToolingSource{}}
	if len(queue) > 2048 {
		queue = queue[:2048]
		record.Truncated = true
	}
	for _, file := range queue {
		record.Sources = append(record.Sources, DashboardToolingSource{AgentAuditSource: byFile[file], Observations: observations[file]})
	}
	return record
}

func workspaceDashboardTooling(indexed []workspaceIndexProject) map[string]DashboardToolingRecord {
	records := map[string]DashboardToolingRecord{}
	for _, project := range indexed {
		records[project.record.Path] = project.tooling
	}
	return records
}

func workspaceDashboardResults(indexed []workspaceIndexProject) map[string]testresults.Record {
	records := map[string]testresults.Record{}
	for _, project := range indexed {
		records[project.record.Path] = project.results
	}
	return records
}
