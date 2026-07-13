package scan

import (
	"fmt"
	"strings"
)

func renderWorkspaceSummaryEntry(project string, files int, coverage CoverageRecord) string {
	return primaryReport("Workspace Summary", fmt.Sprintf("Project `%s` contains %d indexed files and %d capability records.", project, files, len(coverage.Capabilities)), "workspace-summary")
}

func renderArchitectureEntry(routes []CodeRouteRecord, relations []RichRelationRecord) string {
	return primaryReport("Architecture", fmt.Sprintf("GoreGraph indexed %d routes and %d normalized relations. Use generated evidence before reading broad source areas.", len(routes), len(relations)), "service-context")
}

func renderCanonicalDiagnosticsEntry(records []CanonicalDiagnosticRecord) string {
	var b strings.Builder
	b.WriteString(primaryReport("Diagnostics", fmt.Sprintf("GoreGraph generated %d canonical diagnostics. An empty result is meaningful only when coverage is sufficient.", len(records)), "diagnostics"))
	for _, record := range records {
		b.WriteString(fmt.Sprintf("- `%s` %s — %s\n", record.Code, record.Title, record.Explanation))
	}
	return b.String()
}

func renderAgentGuideEntry() string {
	return `# GoreGraph Agent Guide

Start with the smallest bounded generated task. Follow stable evidence IDs and read only the cited source files needed for the change; incomplete coverage is uncertainty, never proof that behavior is absent.

## Coverage orientation

` + "```bash\n" + `goregraph query . coverage --format markdown --limit 20
` + "```" + `

## Endpoint change

` + "```bash\n" + `goregraph query . task-context --query "GET /route" --format markdown --limit 20
` + "```" + `
MCP: ` + "`task_context`" + `

## Service impact

` + "```bash\n" + `goregraph query . service-context --query service-name --format markdown --limit 20
` + "```" + `
MCP: ` + "`service_context`" + `

## Test gaps

` + "```bash\n" + `goregraph query . tests --query route-or-file --format markdown --limit 20
` + "```" + `
MCP: ` + "`tests`" + `

## Open contracts

` + "```bash\n" + `goregraph query . diagnostics --query service-or-route --format markdown --limit 20
` + "```" + `
MCP: ` + "`diagnostics`" + `

## Data flow

` + "```bash\n" + `goregraph query . data-flow --query route --format markdown --limit 20
` + "```" + `
MCP: ` + "`data_flow`" + `

## Workspace delta

` + "```bash\n" + `goregraph query <after-snapshot> workspace-delta --query <before-snapshot> --format markdown --limit 20
` + "```" + `
MCP: ` + "`workspace_delta`" + `

## Evidence lookup

` + "```bash\n" + `goregraph query . evidence --query evidence-id --format markdown --limit 20
` + "```" + `
MCP: ` + "`evidence`" + `

## Freshness check

` + "```bash\n" + `goregraph query . task-context --query route-or-file --format markdown --limit 5
goregraph doctor .
` + "```" + `
MCP: ` + "`task_context`" + `
`
}

func primaryReport(title, summary, task string) string {
	return fmt.Sprintf("# GoreGraph %s\n\n%s\n\n## Start here\n\n```bash\ngoregraph query . %s --format markdown --limit 20\ngoregraph query . coverage --format markdown --limit 20\n```\n\nFor MCP clients, use the equivalent read-only task tool. Run `goregraph doctor .` when freshness or coverage is uncertain.\n", title, summary, task)
}
