package scan

import (
	"fmt"
	"strings"
)

func renderWorkspaceSummaryEntry(project string, files int, coverage CoverageRecord) string {
	return primaryReport("Workspace Summary", fmt.Sprintf("Project `%s` contains %d indexed files and %d capability records.", project, files, len(coverage.Capabilities)))
}

func renderArchitectureEntry(routes []CodeRouteRecord, relations []RichRelationRecord) string {
	return primaryReport("Architecture", fmt.Sprintf("GoreGraph indexed %d routes and %d normalized relations. Use generated evidence before reading broad source areas.", len(routes), len(relations)))
}

func renderCanonicalDiagnosticsEntry(records []CanonicalDiagnosticRecord) string {
	var b strings.Builder
	b.WriteString(primaryReport("Diagnostics", fmt.Sprintf("GoreGraph generated %d canonical diagnostics. An empty result is meaningful only when coverage is sufficient.", len(records))))
	for _, record := range records {
		b.WriteString(fmt.Sprintf("- `%s` %s — %s\n", record.Code, record.Title, record.Explanation))
	}
	return b.String()
}

func renderAgentGuideEntry() string {
	return `# GoreGraph Agent Guide

Use GoreGraph once to obtain a small navigation pack, then verify the cited source.

` + "```bash\n" + `goregraph context . --query "<current coding task>" --budget-tokens 1800 --max-files 12
` + "```" + `

MCP: ` + "`task_context`" + `

- Read only the cited file ranges required to verify the answer.
- If ` + "`fallback_required`" + ` is true, stop using GoreGraph and inspect source directly.
- A reliable production entrypoint has kind ` + "`route`" + `, ` + "`symbol`" + `, or ` + "`backend_handler`" + ` and confidence ` + "`EXACT`" + `, ` + "`RESOLVED`" + `, or ` + "`EXTRACTED`" + `.
- If confidence is low or the result does not contain exactly one reliable production entrypoint, stop using GoreGraph and inspect source directly.
- If additional narrowing is necessary, at most one narrower retry with that entrypoint's exact returned route or qualified symbol is allowed.
- Never retry with a call-chain value.
- After that retry, inspect source directly; do not make a third context call.
- Do not call coverage, diagnostics, tests, data-flow, evidence, or symbol tools in sequence.
- Read generated AI context only from ` + "`goregraph-out/agent/`" + ` or ` + "`.goregraph-workspace/agent/`" + `.
- Do not read ` + "`index/`" + `, ` + "`dashboard/`" + `, dashboard assets, or ` + "`index/symbol-usages.json`" + ` as AI context.
- Run ` + "`goregraph doctor .`" + ` only when the context command reports missing or stale output.
`
}

func primaryReport(title, summary string) string {
	return fmt.Sprintf("# GoreGraph %s\n\n%s\n\n## Explore\n\nUse this Dashboard report for human exploration. Open the workspace Dashboard and Code Explorer when cross-project navigation is needed.\n", title, summary)
}
