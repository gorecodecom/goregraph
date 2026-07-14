package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestServiceReturnsDirectedTraceAndTraversal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	index := scan.DirectedTraceIndexRecord{SchemaVersion: 1, Traces: []scan.DirectedTraceRecord{{ID: "trace:1", Route: "GET /users", Nodes: []scan.DirectedTraceNodeRecord{{ID: "entry", Role: scan.TraceRoleAPIClient}, {ID: "repo", Role: scan.TraceRoleRepository}}, Edges: []scan.DirectedTraceEdgeRecord{{From: "entry", To: "repo"}}, EntryNodes: []string{"entry"}, ExitNodes: []string{"repo"}, MainPath: []string{"entry", "repo"}}}}
	body, _ := json.Marshal(index)
	if err := os.WriteFile(filepath.Join(root, "goregraph-out", "directed-traces.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	service := Service{}
	result, err := service.Run(Request{Root: root, Task: "endpoint-trace", Query: "GET /users"})
	if err != nil || len(result.Items) != 1 {
		t.Fatalf("trace result: %#v %v", result, err)
	}
	traversed, err := service.Run(Request{Root: root, Task: "trace-from", Query: "trace:1#entry"})
	if err != nil || len(traversed.Items) != 1 {
		t.Fatalf("traversal result: %#v %v", traversed, err)
	}
}

func TestServiceBoundsResultsAndContinuesDeterministically(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	service := Service{}
	first, err := service.Run(Request{Root: root, Task: "coverage", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Schema != scan.SchemaVersion || first.Task != "coverage" || len(first.Items) != 2 || !first.Truncated || first.Continuation == "" {
		t.Fatalf("unexpected first result: %#v", first)
	}
	second, err := service.Run(Request{Root: root, Task: "coverage", Limit: 2, Continuation: first.Continuation})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) == 0 || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("continuation did not advance: %#v", second)
	}
}

func TestWorkspaceSummaryReturnsCanonicalContractCounts(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, ".goregraph-workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	serviceMap := scan.WorkspaceServiceMapRecord{
		SchemaVersion: scan.SchemaVersion,
		Root:          root,
		ContractSummary: scan.WorkspaceContractSummaryRecord{
			Total: 2, Resolved: 1, MissingRoute: 1,
		},
	}
	body, err := json.Marshal(serviceMap)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "workspace-service-map.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := (Service{}).Run(Request{Root: root, Task: "workspace-summary"})
	if err != nil {
		t.Fatal(err)
	}
	summary, ok := result.Items[0].Data["contract_summary"].(scan.WorkspaceContractSummaryRecord)
	if !ok || summary.Total != 2 || summary.Resolved != 1 || summary.MissingRoute != 1 {
		t.Fatalf("workspace summary contract counts = %#v", result.Items[0].Data["contract_summary"])
	}
}

func TestServiceRejectsUnsafeBoundsAndReportsCoverage(t *testing.T) {
	service := Service{}
	if _, err := service.Run(Request{Root: t.TempDir(), Task: "coverage", Limit: 101}); err == nil {
		t.Fatal("limit above maximum accepted")
	}
	if _, err := service.Run(Request{Root: t.TempDir(), Task: "coverage", Detail: "verbose"}); err == nil {
		t.Fatal("invalid detail accepted")
	}
}

func TestServiceExposesDiagnosticFamilyAccounting(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "goregraph-out"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTaskContextJSON(t, root, "diagnostic-families.json", []scan.DiagnosticFamilyRecord{{
		FamilyID: "diagnostic-family:dynamic", Code: "dynamic_endpoint_unresolved", Service: "frontend/app",
		ObservedCount: 2, ResolvedCount: 1, UnresolvedCount: 1, LikelyOwner: "frontend/app",
		AffectedProjects: []string{"frontend/app"}, NextChecks: []string{"Inspect variant."},
	}})
	result, err := (Service{}).Run(Request{Root: root, Task: "diagnostics"})
	if err != nil || len(result.Items) != 1 {
		t.Fatalf("diagnostics result=%#v err=%v", result, err)
	}
	data := result.Items[0].Data
	if data["observed_count"] != 2 || data["unresolved_count"] != 1 || data["likely_owner"] != "frontend/app" {
		t.Fatalf("diagnostic family data=%#v", data)
	}
}

func TestServiceExposesReferenceCapabilityEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "client.ts"), []byte("export const load = () => fetch('/users')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	service := Service{}
	coverage, err := service.Run(Request{Root: root, Task: "coverage", Query: "api_clients"})
	if err != nil || len(coverage.Items) != 1 || len(coverage.Items[0].EvidenceIDs) == 0 {
		t.Fatalf("coverage evidence: %#v %v", coverage, err)
	}
	evidence, err := service.Run(Request{Root: root, Task: "evidence", Query: coverage.Items[0].EvidenceIDs[0]})
	if err != nil || len(evidence.Items) != 1 || evidence.Items[0].File != "client.ts" {
		t.Fatalf("architecture evidence: %#v %v", evidence, err)
	}
}

func TestCoverageBoundsEvidenceByDetail(t *testing.T) {
	record := scan.CapabilityRecord{ID: scan.CapabilityPersistence, Language: "java", Coverage: scan.CoverageComplete, Reason: "supported"}
	for index := 0; index < 25; index++ {
		record.EvidenceIDs = append(record.EvidenceIDs, "evidence")
	}
	if got := capabilityEvidence(record, "summary"); len(got) != 0 {
		t.Fatalf("summary evidence count = %d, want 0", len(got))
	}
	if got := capabilityEvidence(record, "standard"); len(got) != 10 {
		t.Fatalf("standard evidence count = %d, want 10", len(got))
	}
	if got := capabilityEvidence(record, "full"); len(got) != 25 {
		t.Fatalf("full evidence count = %d, want 25", len(got))
	}
}

func TestServiceBuildsBoundedTaskContext(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	writeTaskContextJSON(t, root, "routes.json", []scan.CodeRouteRecord{{
		RouteID: "route:users", App: "accounts", HTTPMethod: "GET", Path: "/users",
		Handler: "ListUsers", File: "internal/users.go", Line: 12,
		Confidence: "EXACT", EvidenceIDs: []string{"evidence:route"},
	}})
	writeTaskContextJSON(t, root, "test-map.json", []scan.TestMapRecord{{
		TestFile: "internal/users_test.go", TestMethod: "TestListUsers",
		TargetFile: "internal/users.go", HTTPMethod: "GET", Path: "/users",
		Type: "endpoint", Line: 8, Confidence: "EXACT",
	}})
	writeTaskContextJSON(t, root, "diagnostics-canonical.json", []scan.CanonicalDiagnosticRecord{{
		ID: "diagnostic:users", Code: "method_mismatch", Title: "Method mismatch",
		Explanation:       "GET route conflicts with a backend POST route.",
		AffectedArtifacts: []string{"GET /users", "internal/users.go"},
		EvidenceIDs:       []string{"evidence:route", "evidence:diagnostic"},
	}})

	result, err := (Service{}).Run(Request{Root: root, Task: "task-context", Query: "GET /users", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 || result.Items[0].Kind != "task_context" {
		t.Fatalf("unexpected task context: %#v", result)
	}
	context := result.Items[0].Data
	if context["target"] != "GET /users" || len(context["endpoints"].([]Item)) != 1 ||
		len(context["tests"].([]Item)) != 1 || len(context["risks"].([]Item)) != 1 {
		t.Fatalf("incomplete task context: %#v", context)
	}
	if freshness, ok := context["freshness"].(string); !ok || !strings.Contains(freshness, "source fingerprint") {
		t.Fatalf("task context freshness = %#v", context["freshness"])
	}
	if got := result.Items[0].EvidenceIDs; len(got) != 2 ||
		got[0] != "evidence:diagnostic" || got[1] != "evidence:route" {
		t.Fatalf("evidence IDs = %#v", got)
	}
}

func TestTaskContextExposesCanonicalTestLinksAndVerification(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "goregraph-out"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTaskContextJSON(t, root, "routes.json", []scan.CodeRouteRecord{})
	writeTaskContextJSON(t, root, "test-map.json", []scan.TestMapRecord{})
	writeTaskContextJSON(t, root, "diagnostics-canonical.json", []scan.CanonicalDiagnosticRecord{})
	writeTaskContextJSON(t, root, "capabilities.json", []scan.CapabilityRecord{})
	writeTaskContextJSON(t, root, "workspace-feature-flows.json", []scan.WorkspaceFeatureFlowRecord{{ID: "flow", HTTPMethod: "GET", Path: "/users", TestLinks: []scan.TestLinkRecord{{ID: "link", Relation: "direct", TestFile: "UserTest.java", Confidence: "EXACT", Reason: "calls endpoint"}}, VerificationCommands: []scan.VerificationCommandRecord{{Tool: "maven", Display: "mvn test", Confidence: "EXACT", Reason: "detected Maven"}}}})
	result, err := (Service{}).Run(Request{Root: root, Task: "task-context", Query: "GET /users", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	data := result.Items[0].Data
	if len(data["test_links"].([]scan.TestLinkRecord)) != 1 || len(data["verification_commands"].([]scan.VerificationCommandRecord)) != 1 {
		t.Fatalf("task context data=%#v", data)
	}
}

func TestServiceReturnsImpactSummary(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "goregraph-out"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTaskContextJSON(t, root, "workspace-feature-flows.json", []scan.WorkspaceFeatureFlowRecord{{ID: "flow", HTTPMethod: "GET", Path: "/users", Nodes: []scan.CanonicalFlowNodeRecord{{ID: "api", Kind: "api_call"}, {ID: "endpoint", Kind: "endpoint"}}, Edges: []scan.CanonicalFlowEdgeRecord{{ID: "edge", FromNodeID: "api", ToNodeID: "endpoint", EdgeType: "invokes_api", Confidence: "RESOLVED", Reason: "match"}}}})
	result, err := (Service{}).Run(Request{Root: root, Task: "impact-summary", Query: "/users", Limit: 5})
	if err != nil || len(result.Items) != 1 || result.Items[0].Kind != "impact_summary" {
		t.Fatalf("impact result=%#v err=%v", result, err)
	}
}

func TestServiceReturnsImpactSummaryFromWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	workspaceOut := filepath.Join(root, ".goregraph-workspace")
	if err := os.MkdirAll(workspaceOut, 0o755); err != nil {
		t.Fatal(err)
	}
	flows := []scan.WorkspaceFeatureFlowRecord{{ID: "flow", HTTPMethod: "GET", Path: "/documents", Nodes: []scan.CanonicalFlowNodeRecord{{ID: "api", Kind: "api_call"}, {ID: "endpoint", Kind: "endpoint"}}, Edges: []scan.CanonicalFlowEdgeRecord{{ID: "edge", FromNodeID: "api", ToNodeID: "endpoint", EdgeType: "invokes_api", Confidence: "RESOLVED", Reason: "match"}}}}
	body, err := json.Marshal(flows)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceOut, "feature-flows.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := (Service{}).Run(Request{Root: root, Task: "impact-summary", Query: "/documents", Limit: 5})
	if err != nil || len(result.Items) != 1 || result.Items[0].Kind != "impact_summary" {
		t.Fatalf("workspace impact result=%#v err=%v", result, err)
	}
}

func writeTaskContextJSON(t *testing.T, root, name string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "goregraph-out", name), body, 0o644); err != nil {
		t.Fatal(err)
	}
}
