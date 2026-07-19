package scan

import (
	"strings"
	"testing"
)

func TestAgentGuideUsesOneBoundedContextWorkflow(t *testing.T) {
	assertAgentGuideContract(t, renderAgentGuideEntry())
}

func TestPrimaryDashboardReportsDoNotPromoteAgentQueryCascades(t *testing.T) {
	reports := map[string]string{
		"workspace summary": renderWorkspaceSummaryEntry("example", 3, CoverageRecord{}),
		"architecture":      renderArchitectureEntry(nil, nil),
		"diagnostics":       renderCanonicalDiagnosticsEntry(nil),
	}
	for name, report := range reports {
		t.Run(name, func(t *testing.T) {
			for _, forbidden := range []string{"goregraph query", "MCP", "task_context", "service_context"} {
				if strings.Contains(report, forbidden) {
					t.Fatalf("human dashboard report promotes %q:\n%s", forbidden, report)
				}
			}
			for _, want := range []string{"Dashboard", "Code Explorer"} {
				if !strings.Contains(report, want) {
					t.Fatalf("human dashboard report missing %q:\n%s", want, report)
				}
			}
		})
	}
}

func assertAgentGuideContract(t *testing.T, guide string) {
	t.Helper()
	const assistedInstruction = `Call task_context once before indexed source discovery. Treat source_sections as already read.
Retry only when retry_allowed is true, use one retry_anchor, and pass context_id as previous_context_id.
If duplicate_of is present, use the first pack and do not read more source because of the duplicate response.`

	command := `goregraph context . --query "<current coding task>" --budget-tokens 4000 --max-files 12`
	if count := strings.Count(guide, command); count != 1 {
		t.Fatalf("bounded context command occurs %d times, want exactly once:\n%s", count, guide)
	}
	if count := strings.Count(guide, assistedInstruction); count != 1 {
		t.Fatalf("agent guide contains the exact assisted instruction %d times, want 1:\n%s", count, guide)
	}
	for _, want := range []string{
		"MCP: `task_context`",
		"`goregraph-out/agent/`",
		"`.goregraph-workspace/agent/`",
		"Do not read `index/`, `dashboard/`, dashboard assets, or `index/symbol-usages.json` as AI context",
		"Run `goregraph doctor .` only when the context command reports missing or stale output",
	} {
		if !strings.Contains(guide, want) {
			t.Fatalf("agent guide missing %q:\n%s", want, guide)
		}
	}
	for _, forbidden := range []string{
		"goregraph query",
		"MCP: `coverage`",
		"MCP: `service_context`",
		"MCP: `tests`",
		"MCP: `diagnostics`",
		"MCP: `data_flow`",
		"MCP: `evidence`",
		"MCP: `workspace_delta`",
		"symbol_resolve",
		"symbol_usages",
	} {
		if strings.Contains(guide, forbidden) {
			t.Fatalf("agent guide still promotes query cascade %q:\n%s", forbidden, guide)
		}
	}
}
