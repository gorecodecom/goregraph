package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
	if err := os.MkdirAll(filepath.Join(root, "goregraph-out", "index"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "goregraph-out", "index", "directed-traces.json"), body, 0o644); err != nil {
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
	if err := os.MkdirAll(filepath.Join(workspaceDir, "index"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "index", "workspace-service-map.json"), body, 0o644); err != nil {
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
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-16T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users", File: "internal/users.go", Line: 12, Confidence: "EXACT"},
			{ID: "test", Kind: "test", Name: "TestListUsers", File: "internal/users_test.go", Line: 8, Confidence: "EXACT"},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "test-target", FromFactID: "test", ToFactID: "route",
			FromLabel: "TestListUsers", ToLabel: "GET /users", Kind: "test_target",
		}},
	})

	result, err := (Service{}).Run(Request{Root: root, Task: "task-context", Query: "GET /users"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 || result.Items[0].Kind != "task_context" ||
		result.Truncated || result.Continuation != "" {
		t.Fatalf("unexpected task context: %#v", result)
	}
	pack, ok := result.Items[0].Data["context"].(ContextPack)
	if !ok || len(pack.Entrypoints) != 1 || len(pack.Tests) != 1 ||
		len(pack.CallChain) != 1 {
		t.Fatalf("incomplete task context: %#v", result.Items[0].Data)
	}
	if pack.BudgetTokens != DefaultContextBudgetTokens ||
		len(pack.Files) > DefaultContextMaxFiles ||
		pack.EstimatedTokens > pack.BudgetTokens {
		t.Fatalf("task context bounds = %#v", pack)
	}
	if result.Freshness != pack.Freshness || len(result.CoverageWarnings) != 0 ||
		result.SuggestedNext != "" {
		t.Fatalf("task context envelope = %#v", result)
	}
	for _, legacy := range []string{
		"target", "services", "endpoints", "risks", "test_links",
		"verification_commands", "coverage_warnings", "suggested_next",
	} {
		if _, exists := result.Items[0].Data[legacy]; exists {
			t.Fatalf("legacy task-context key %q remains: %#v", legacy, result.Items[0].Data)
		}
	}
}

func TestServiceEndpointTaskContextSerialization(t *testing.T) {
	root := writeServiceEndpointContextFixture(t)
	query := "GET /orders authentication"
	expected, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}

	result, err := (Service{}).Run(Request{Root: root, Task: "task-context", Query: query})
	if err != nil {
		t.Fatal(err)
	}
	pack, ok := result.Items[0].Data["context"].(ContextPack)
	if !ok {
		t.Fatalf("service context type = %T, want ContextPack", result.Items[0].Data["context"])
	}
	if !reflect.DeepEqual(pack.Endpoints, expected.Endpoints) {
		t.Fatalf("service endpoints = %#v, want %#v", pack.Endpoints, expected.Endpoints)
	}
	if len(pack.Endpoints) != 1 {
		t.Fatalf("service endpoints = %#v", pack.Endpoints)
	}
	endpoint := pack.Endpoints[0]
	if endpoint.Provider != "services/orders" || endpoint.HTTPMethod != "GET" || endpoint.Path != "/orders/{id}" ||
		endpoint.Security != "bearer" || len(endpoint.Consumers) != 8 || endpoint.OmittedConsumers != 5 {
		t.Fatalf("service endpoint details = %#v", endpoint)
	}
	if pack.BudgetTokens != DefaultContextBudgetTokens || pack.FallbackRequired {
		t.Fatalf("service context bounds/fallback = %#v", pack)
	}

	body, err := json.Marshal(result.Items[0].Data["context"])
	if err != nil {
		t.Fatal(err)
	}
	assertServiceEndpointContextWireKeys(t, body)
	var roundTrip ContextPack
	if err := json.Unmarshal(body, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip.Endpoints, expected.Endpoints) {
		t.Fatalf("serialized service endpoints = %#v, want %#v", roundTrip.Endpoints, expected.Endpoints)
	}
	for _, forbidden := range []string{"api-catalog.json", ".goregraph-dashboard.json", "/invoices"} {
		if bytes := string(body); strings.Contains(bytes, forbidden) {
			t.Fatalf("service context contains %q: %s", forbidden, bytes)
		}
	}
}

func TestTaskContextUsesExplicitCompilerBoundsAndIgnoresPagination(t *testing.T) {
	root := t.TempDir()
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), contextIndexWithFact("route", "GET users details"))

	result, err := (Service{}).Run(Request{
		Root: root, Task: "task-context", Query: "GET users details",
		Limit: 1, Continuation: encodeContinuation("task-context", 1),
		BudgetTokens: 700, MaxFiles: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	pack := result.Items[0].Data["context"].(ContextPack)
	if pack.BudgetTokens != 700 || len(pack.Files) > 3 ||
		result.Truncated || result.Continuation != "" {
		t.Fatalf("explicit context bounds or pagination = %#v / %#v", pack, result)
	}
}

func TestTaskContextDoesNotUseGenericLimitAsDefaultMaxFiles(t *testing.T) {
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "route", Kind: "route", Name: "GET /users",
			HTTPMethod: "GET", Path: "/users", File: "route.go", Confidence: "EXACT",
		}},
	}
	for number := 0; number < 14; number++ {
		id := fmt.Sprintf("neighbor-%02d", number)
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: id, Kind: "symbol", Name: id, File: id + ".go",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge-" + id, FromFactID: "route", ToFactID: id,
			FromLabel: "route", ToLabel: id, Kind: "call",
		})
	}
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), index)

	result, err := (Service{}).Run(Request{
		Root: root, Task: "task-context", Query: "GET /users", Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	pack := result.Items[0].Data["context"].(ContextPack)
	if len(pack.Files) != DefaultContextMaxFiles {
		t.Fatalf("default context files = %d, want %d: %#v", len(pack.Files), DefaultContextMaxFiles, pack.Files)
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
	if err := os.MkdirAll(filepath.Join(workspaceOut, "index"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceOut, "index", "feature-flows.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := (Service{}).Run(Request{Root: root, Task: "impact-summary", Query: "/documents", Limit: 5})
	if err != nil || len(result.Items) != 1 || result.Items[0].Kind != "impact_summary" {
		t.Fatalf("workspace impact result=%#v err=%v", result, err)
	}
}

func TestServiceReturnsSymbolInventory(t *testing.T) {
	workspace, project, symbols, _ := writeSymbolProjectionFixture(t)

	first, err := (Service{}).Run(Request{
		Root: project, Task: "symbol-inventory", Query: "microservices/ms-user", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Count != 1 || !first.Truncated || first.Continuation == "" {
		t.Fatalf("first inventory page = %#v", first)
	}
	symbol, ok := first.Items[0].Data["symbol"].(scan.CanonicalSymbolRecord)
	if !ok || symbol.Project != "microservices/ms-user" {
		t.Fatalf("inventory symbol = %#v", first.Items[0].Data["symbol"])
	}
	if len(first.CoverageWarnings) == 0 || !strings.Contains(first.CoverageWarnings[0], "PARTIAL") {
		t.Fatalf("inventory coverage warnings = %#v", first.CoverageWarnings)
	}

	second, err := (Service{}).Run(Request{
		Root: workspace, Task: "symbol-inventory", Query: "microservices/ms-user",
		Limit: 1, Continuation: first.Continuation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Count != 1 || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("second inventory page = %#v", second)
	}
	if second.Items[0].ID != symbols.Symbols[1].ID {
		t.Fatalf("second inventory ID = %q, want %q", second.Items[0].ID, symbols.Symbols[1].ID)
	}
}

func TestServiceResolvesSymbolCandidatesWithoutGuessing(t *testing.T) {
	workspace, _, symbols, _ := writeSymbolProjectionFixture(t)

	ambiguous, err := (Service{}).Run(Request{
		Root: workspace, Task: "symbol-resolve", Query: "UserService", Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ambiguous.Count != 2 {
		t.Fatalf("same-name candidates = %#v", ambiguous)
	}
	gotIDs := []string{ambiguous.Items[0].ID, ambiguous.Items[1].ID}
	wantIDs := []string{symbols.Symbols[0].ID, symbols.Symbols[2].ID}
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("same-name candidate IDs = %#v, want %#v", gotIDs, wantIDs)
	}
	for _, item := range ambiguous.Items {
		if item.Resolution != "" {
			t.Fatalf("resolver selected candidate %#v", item)
		}
	}

	unique, err := (Service{}).Run(Request{
		Root: workspace, Task: "symbol-resolve", Query: "com.weka.UserRepository", Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if unique.Count != 1 || unique.Items[0].ID != symbols.Symbols[1].ID {
		t.Fatalf("qualified-name result = %#v", unique)
	}
}

func TestServiceReportsUsageCoverageGapsWithoutMatchingRecords(t *testing.T) {
	workspace, _, symbols, _ := writeSymbolProjectionFixture(t)

	matched, err := (Service{}).Run(Request{
		Root: workspace, Task: "symbol-usages", Query: symbols.Symbols[0].ID, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if matched.Count != 1 {
		t.Fatalf("matched direct usage result = %#v", matched)
	}
	if len(matched.CoverageWarnings) != 1 ||
		!strings.Contains(matched.CoverageWarnings[0], "frontend/app / typescript / direct_usages") {
		t.Fatalf("matched direct usage coverage warnings = %#v", matched.CoverageWarnings)
	}

	direct, err := (Service{}).Run(Request{
		Root: workspace, Task: "symbol-usages", Query: symbols.Symbols[1].ID, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if direct.Count != 0 {
		t.Fatalf("direct usage result = %#v", direct)
	}
	if len(direct.CoverageWarnings) != 1 ||
		!strings.Contains(direct.CoverageWarnings[0], "frontend/app / typescript / direct_usages") {
		t.Fatalf("direct usage coverage warnings = %#v", direct.CoverageWarnings)
	}

	api, err := (Service{}).Run(Request{
		Root: workspace, Task: "symbol-api-consumers", Query: symbols.Symbols[1].ID, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if api.Count != 0 {
		t.Fatalf("API usage result = %#v", api)
	}
	if len(api.CoverageWarnings) != 1 ||
		!strings.Contains(api.CoverageWarnings[0], "frontend/legacy / javascript / http_reachability") {
		t.Fatalf("API usage coverage warnings = %#v", api.CoverageWarnings)
	}
}

func TestServiceReturnsAllRelevantSymbolCoverageWarnings(t *testing.T) {
	workspace, _, symbols, usages := writeSymbolProjectionFixture(t)
	usages.Coverage = nil
	for index := 14; index >= 0; index-- {
		usages.Coverage = append(usages.Coverage, scan.SymbolCoverageRecord{
			Project:    fmt.Sprintf("frontend/app-%02d", index),
			Language:   "typescript",
			Capability: "direct_usages",
			Coverage:   scan.CoveragePartial,
			Reason:     "dynamic imports are not statically resolved",
		})
	}
	writeWorkspaceProjectionJSON(t, workspace, "symbol-usages.json", usages)

	result, err := (Service{}).Run(Request{
		Root: workspace, Task: "symbol-usages", Query: symbols.Symbols[1].ID, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CoverageWarnings) != 15 {
		t.Fatalf("coverage warning count = %d, want 15: %#v", len(result.CoverageWarnings), result.CoverageWarnings)
	}
	if !strings.Contains(result.CoverageWarnings[0], "frontend/app-00") ||
		!strings.Contains(result.CoverageWarnings[14], "frontend/app-14") {
		t.Fatalf("coverage warnings are not deterministic: %#v", result.CoverageWarnings)
	}
	for _, warning := range result.CoverageWarnings {
		if strings.Contains(warning, "omitted") || strings.Contains(warning, "continue the coverage query") {
			t.Fatalf("symbol warning contains invalid continuation guidance: %q", warning)
		}
	}
}

func writeSymbolProjectionFixture(t *testing.T) (string, string, scan.WorkspaceSymbolIndexRecord, scan.WorkspaceSymbolUsageIndexRecord) {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), "weka")
	project := filepath.Join(workspace, "microservices", "ms-user")
	if err := os.MkdirAll(filepath.Join(workspace, ".goregraph-workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	symbols := scan.WorkspaceSymbolIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Root:          filepath.ToSlash(workspace),
		Symbols: []scan.CanonicalSymbolRecord{
			{
				ID: "symbol:01", Project: "microservices/ms-user", Language: "java",
				Kind: "class", Name: "UserService", QualifiedName: "com.weka.UserService",
				DeclarationFile: "src/UserService.java", DeclarationLine: 10,
				Analyzer: "java-source", Confidence: scan.ConfidenceExact, Coverage: scan.CoveragePartial,
				Limitations: []string{"reflection may add runtime consumers"},
			},
			{
				ID: "symbol:02", Project: "microservices/ms-user", Language: "java",
				Kind: "interface", Name: "UserRepository", QualifiedName: "com.weka.UserRepository",
				DeclarationFile: "src/UserRepository.java", DeclarationLine: 5,
				Analyzer: "java-source", Confidence: scan.ConfidenceExact, Coverage: scan.CoverageComplete,
			},
			{
				ID: "symbol:03", Project: "microservices/ms-order", Language: "java",
				Kind: "class", Name: "UserService", QualifiedName: "com.weka.order.UserService",
				DeclarationFile: "src/UserService.java", DeclarationLine: 8,
				Analyzer: "java-source", Confidence: scan.ConfidenceExact, Coverage: scan.CoverageComplete,
			},
		},
		Coverage: []scan.SymbolCoverageRecord{{
			Project: "microservices/ms-user", Language: "java", Capability: "declarations",
			Coverage: scan.CoveragePartial, Reason: "reflection is not statically resolved",
			Limitations: []string{"reflection"},
		}},
	}
	usages := scan.WorkspaceSymbolUsageIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Root:          filepath.ToSlash(workspace),
		Usages: []scan.CanonicalSymbolUsageRecord{{
			ID: "usage:01", ProviderSymbolID: "symbol:01", ConsumerProject: "microservices/ms-order",
			Category: scan.SymbolUsageDirectReference, Language: "java", RelationKind: "calls_method_owner",
			SourceFile: "src/OrderService.java", SourceLine: 20, Confidence: scan.ConfidenceExact,
			Resolution: scan.SymbolResolutionExact, Reason: "qualified Java reference",
			Analyzer: "workspace-symbols", EvidenceIDs: []string{"microservices/ms-order#evidence:1"},
		}},
		Coverage: []scan.SymbolCoverageRecord{
			{
				Project: "microservices/ms-user", Language: "java", Capability: "declarations",
				Coverage: scan.CoveragePartial, Reason: "reflection is not statically resolved",
				Limitations: []string{"reflection"},
			},
			{
				Project: "frontend/app", Language: "typescript", Capability: "direct_usages",
				Coverage: scan.CoveragePartial, Reason: "dynamic imports are not statically resolved",
				Limitations: []string{"dynamic_import"},
			},
			{
				Project: "frontend/legacy", Language: "javascript", Capability: "http_reachability",
				Coverage: scan.CoverageFailed, Reason: "flows.json could not be read",
				Limitations: []string{"flows_unreadable"},
			},
		},
	}
	writeWorkspaceProjectionJSON(t, workspace, "symbol-index.json", symbols)
	writeWorkspaceProjectionJSON(t, workspace, "symbol-usages.json", usages)
	return workspace, project, symbols, usages
}

func writeWorkspaceProjectionJSON(t *testing.T, workspace, name string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace, ".goregraph-workspace", "index", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTaskContextJSON(t *testing.T, root, name string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "goregraph-out", "index", name)
	if name == "manifest.json" {
		path = filepath.Join(root, "goregraph-out", name)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeServiceEndpointContextFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), endpointContextIndexFixture())
	return root
}

func assertServiceEndpointContextWireKeys(t *testing.T, body []byte) {
	t.Helper()
	var contextObject map[string]json.RawMessage
	if err := json.Unmarshal(body, &contextObject); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"endpoints", "budget_tokens", "fallback_required"} {
		if _, ok := contextObject[key]; !ok {
			t.Fatalf("service context JSON missing public key %q: %s", key, body)
		}
	}
	for _, alias := range []string{"endpoint", "Endpoints", "budgetTokens", "BudgetTokens", "fallbackRequired", "FallbackRequired"} {
		if _, ok := contextObject[alias]; ok {
			t.Fatalf("service context JSON contains alias key %q: %s", alias, body)
		}
	}

	var endpoints []map[string]json.RawMessage
	if err := json.Unmarshal(contextObject["endpoints"], &endpoints); err != nil || len(endpoints) != 1 {
		t.Fatalf("service endpoints JSON = %s, error = %v", contextObject["endpoints"], err)
	}
	endpoint := endpoints[0]
	for _, key := range []string{"http_method", "omitted_consumers"} {
		if _, ok := endpoint[key]; !ok {
			t.Fatalf("service endpoint JSON missing public key %q: %s", key, contextObject["endpoints"])
		}
	}
	for _, alias := range []string{"method", "httpMethod", "HTTPMethod", "omittedConsumers", "OmittedConsumers"} {
		if _, ok := endpoint[alias]; ok {
			t.Fatalf("service endpoint JSON contains alias key %q: %s", alias, contextObject["endpoints"])
		}
	}

	var consumers []map[string]json.RawMessage
	if err := json.Unmarshal(endpoint["consumers"], &consumers); err != nil || len(consumers) == 0 {
		t.Fatalf("service consumers JSON = %s, error = %v", endpoint["consumers"], err)
	}
	if _, ok := consumers[0]["authentication"]; !ok {
		t.Fatalf("service consumer JSON missing public key %q: %s", "authentication", endpoint["consumers"])
	}
	if _, ok := consumers[0]["auth"]; ok {
		t.Fatalf("service consumer JSON contains alias key %q: %s", "auth", endpoint["consumers"])
	}
	if _, ok := consumers[0]["Authentication"]; ok {
		t.Fatalf("service consumer JSON contains alias key %q: %s", "Authentication", endpoint["consumers"])
	}
}

func endpointContextIndexFixture() scan.AgentContextIndexRecord {
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "endpoint:orders", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders/{id}",
				Qualified: "OrderController.get", HTTPMethod: "GET", Path: "/orders/{id}",
				File: "services/orders/src/OrderController.java", Line: 20,
				Summary: "provider orders; security bearer; request OrderRequest; response OrderResponse; 3 consumer call sites omitted",
				Search:  "GET /orders/{id} authentication bearer services orders", Confidence: "EXACT",
			},
			{
				ID: "endpoint:billing", Project: "services/billing", Kind: "api_endpoint", Name: "GET /invoices",
				HTTPMethod: "GET", Path: "/invoices", File: "services/billing/src/InvoiceController.java", Line: 12,
				Summary: "provider billing; security public", Search: "GET /invoices billing public", Confidence: "EXACT",
			},
		},
	}
	for number := 0; number < 10; number++ {
		id := fmt.Sprintf("consumer:%02d", number)
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: id, Project: fmt.Sprintf("frontend/web-%02d", number), Kind: "api_consumer", Name: id,
			File: fmt.Sprintf("frontend/web-%02d/src/api.ts", number), Line: 7,
			Summary: "consumer service web; auth bearer", Confidence: "RESOLVED",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + id, FromFactID: id, ToFactID: "endpoint:orders",
			Kind: "consumes_endpoint", Reason: "catalog consumer auth bearer", Confidence: "RESOLVED",
		})
	}
	for _, metadata := range []struct {
		id   string
		file string
	}{
		{id: "consumer:catalog", file: ".goregraph-workspace/agent/api-catalog.json"},
		{id: "consumer:dashboard", file: ".goregraph-dashboard.json"},
	} {
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: metadata.id, Project: "frontend/metadata", Kind: "api_consumer", Name: metadata.id,
			File: metadata.file, Line: 1, Summary: "consumer service metadata; auth unknown",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + metadata.id, FromFactID: metadata.id, ToFactID: "endpoint:orders", Kind: "consumes_endpoint",
		})
	}
	return index
}
