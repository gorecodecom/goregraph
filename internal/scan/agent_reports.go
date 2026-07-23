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

Use the source-backed Context Pack once for the complete task.

` + "```bash\n" + `goregraph context . --query "<current coding task>" --budget-tokens 4000 --max-files 12
` + "```" + `

MCP: ` + "`task_context`" + `

` + "```text\n" + `Call goregraph context once with the complete task before reading indexed source.
Treat source_sections as current source already read; do not re-read or grep included ranges.
If source_coverage is complete, continue from the included source without another navigation read.
If source_coverage is partial or none, inspect only exact project/path entries listed in source_omissions; report pathless omissions as uncertainty without broad source discovery.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.
` + "```" + `

- Read generated AI context only from ` + "`goregraph-out/agent/`" + ` or ` + "`.goregraph-workspace/agent/`" + `.
- Do not read ` + "`index/`" + `, ` + "`dashboard/`" + `, dashboard assets, or ` + "`index/symbol-usages.json`" + ` as AI context.
- Run ` + "`goregraph doctor .`" + ` only when the context command reports missing or stale output.
`
}

func primaryReport(title, summary string) string {
	return fmt.Sprintf("# GoreGraph %s\n\n%s\n\n## Explore\n\nUse this Dashboard report for human exploration. Open the workspace Dashboard and Code Explorer when cross-project navigation is needed.\n", title, summary)
}
