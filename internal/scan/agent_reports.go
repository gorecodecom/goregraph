package scan

import (
	"fmt"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agentguide"
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

` + "```text\n" + agentguide.AssistedInstruction + "\n```" + `

- Read generated AI context only from ` + "`goregraph-out/agent/`" + ` or ` + "`.goregraph-workspace/agent/`" + `.
- Do not read ` + "`index/`" + `, ` + "`dashboard/`" + `, dashboard assets, or ` + "`index/symbol-usages.json`" + ` as AI context.
- Run ` + "`goregraph doctor .`" + ` only when the context command reports missing or stale output.
`
}

func primaryReport(title, summary string) string {
	return fmt.Sprintf("# GoreGraph %s\n\n%s\n\n## Explore\n\nUse this Dashboard report for human exploration. Open the workspace Dashboard and Code Explorer when cross-project navigation is needed.\n", title, summary)
}
