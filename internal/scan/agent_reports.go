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
	return primaryReport("Agent Guide", "Start with compact generated tasks, follow stable evidence IDs, and read only the cited source locations needed for the change. UNAVAILABLE or FAILED coverage means analysis is incomplete; it never proves that behavior is absent.", "coverage")
}

func primaryReport(title, summary, task string) string {
	return fmt.Sprintf("# GoreGraph %s\n\n%s\n\n## Start here\n\n```bash\ngoregraph query . %s --format markdown --limit 20\ngoregraph query . coverage --format markdown --limit 20\n```\n\nFor MCP clients, use the equivalent read-only task tool. Run `goregraph doctor .` when freshness or coverage is uncertain.\n", title, summary, task)
}
