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

` + "```text\n" + `Call goregraph context once with a focused query containing the caller's problem statement and requested evidence scope before reading indexed source.
Preserve the caller's domain language, identifiers, and requested evidence; exclude workspace setup, tool policy, safety constraints, and output-format instructions. Do not translate or add inferred repository or component responsibilities.
If the context command fails, do not read context-index.json or any generated index; only a missing or stale output error permits goregraph doctor ., otherwise stop using GoreGraph and follow the caller's fallback policy.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path and start_line/end_line ranges listed in source_omissions; do not inspect outside those ranges or other files. Report pathless or unbounded omissions as uncertainty.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
A missing future call, route, or symbol required by the requested fix is evidence of the current gap, not a source-fallback trigger; assess entrypoint reliability from the existing production path.
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
