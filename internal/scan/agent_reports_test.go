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
	command := `goregraph context . --query "<current coding task>" --budget-tokens 4000 --max-files 12`
	if count := strings.Count(guide, command); count != 1 {
		t.Fatalf("bounded context command occurs %d times, want exactly once:\n%s", count, guide)
	}
	for _, want := range []string{
		"MCP: `task_context`",
		"cited file ranges",
		"`fallback_required` is true",
		"stop using GoreGraph and inspect source directly",
		"exactly one reliable production entrypoint",
		"`EXACT`, `RESOLVED`, or `EXTRACTED`",
		"at most one narrower retry",
		"Never retry with a call-chain value",
		"After that retry, inspect source directly",
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
