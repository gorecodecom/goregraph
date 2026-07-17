package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestEstimateContextTokensUsesJSONRunes(t *testing.T) {
	value := map[string]string{"name": "Größe"}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	got, err := EstimateContextTokens(value)
	if err != nil {
		t.Fatal(err)
	}
	want := (len([]rune(string(body))) + 3) / 4
	if got != want {
		t.Fatalf("EstimateContextTokens() = %d, want %d", got, want)
	}
	if _, err := EstimateContextTokens(make(chan int)); err == nil {
		t.Fatal("unsupported JSON value was accepted")
	}
}

func TestBuildContextRejectsInvalidBounds(t *testing.T) {
	for _, request := range []ContextRequest{
		{Root: t.TempDir(), Query: "delete user", BudgetTokens: 255},
		{Root: t.TempDir(), Query: "delete user", BudgetTokens: 4001},
		{Root: t.TempDir(), Query: "delete user", MaxFiles: -1},
		{Root: t.TempDir(), Query: "delete user", MaxFiles: 21},
		{Root: t.TempDir(), Query: "   "},
	} {
		if _, err := BuildContext(request); err == nil {
			t.Fatalf("accepted invalid request: %#v", request)
		}
	}
}

func TestBuildContextLoadsWorkspaceBeforeProject(t *testing.T) {
	root := t.TempDir()
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), contextIndexWithFact("workspace", "workspace route"))
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), contextIndexWithFact("project", "project route"))

	pack, err := BuildContext(ContextRequest{Root: root, Query: "workspace route"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "workspace" {
		t.Fatalf("workspace context did not win: %#v", pack.Entrypoints)
	}
}

func TestBuildContextLoadsDetectedParentWorkspaceBeforeProject(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "users")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	writeContextIndexAt(t, filepath.Join(workspace, ".goregraph-workspace", "agent", "context-index.json"), contextIndexWithFact("workspace", "parent workspace route"))
	writeContextIndexAt(t, filepath.Join(project, "goregraph-out", "agent", "context-index.json"), contextIndexWithFact("project", "project route"))

	pack, err := BuildContext(ContextRequest{Root: project, Query: "parent workspace route"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "workspace" {
		t.Fatalf("detected workspace context did not win: %#v", pack.Entrypoints)
	}
}

func TestBuildContextDoesNotFallThroughMalformedWorkspaceIndex(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json")
	if err := os.MkdirAll(filepath.Dir(workspacePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workspacePath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), contextIndexWithFact("project", "project route"))

	if _, err := BuildContext(ContextRequest{Root: root, Query: "project route"}); err == nil ||
		!strings.Contains(err.Error(), "context index") {
		t.Fatalf("malformed authoritative workspace index error = %v", err)
	}
}

func TestBuildContextUsesConfiguredProjectOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "goregraph.yml"), []byte("output: custom-out\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeContextIndexAt(t, filepath.Join(root, "custom-out", "agent", "context-index.json"), contextIndexWithFact("custom", "custom output route"))

	pack, err := BuildContext(ContextRequest{Root: root, Query: "custom output route"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "custom" {
		t.Fatalf("configured output was not loaded: %#v", pack.Entrypoints)
	}
}

func TestBuildContextRejectsInvalidIndexGraphs(t *testing.T) {
	validFact := scan.AgentContextFactRecord{ID: "fact", Kind: "route", Name: "GET /users"}
	for name, index := range map[string]scan.AgentContextIndexRecord{
		"schema": {
			SchemaVersion: scan.SchemaVersion - 1,
		},
		"empty fact id": {
			SchemaVersion: scan.SchemaVersion,
			Facts:         []scan.AgentContextFactRecord{{Kind: "route", Name: "GET /users"}},
		},
		"duplicate fact id": {
			SchemaVersion: scan.SchemaVersion,
			Facts:         []scan.AgentContextFactRecord{validFact, validFact},
		},
		"empty edge id": {
			SchemaVersion: scan.SchemaVersion,
			Facts:         []scan.AgentContextFactRecord{validFact},
			Edges:         []scan.AgentContextEdgeRecord{{FromFactID: "fact", ToFactID: "fact"}},
		},
		"duplicate edge id": {
			SchemaVersion: scan.SchemaVersion,
			Facts:         []scan.AgentContextFactRecord{validFact},
			Edges: []scan.AgentContextEdgeRecord{
				{ID: "edge", FromFactID: "fact", ToFactID: "fact"},
				{ID: "edge", FromFactID: "fact", ToFactID: "fact"},
			},
		},
		"fact edge collision": {
			SchemaVersion: scan.SchemaVersion,
			Facts:         []scan.AgentContextFactRecord{validFact},
			Edges:         []scan.AgentContextEdgeRecord{{ID: "fact", FromFactID: "fact", ToFactID: "fact"}},
		},
		"dangling endpoint": {
			SchemaVersion: scan.SchemaVersion,
			Facts:         []scan.AgentContextFactRecord{validFact},
			Edges:         []scan.AgentContextEdgeRecord{{ID: "edge", FromFactID: "fact", ToFactID: "missing"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := writeContextIndexFixture(t, index)
			if _, err := BuildContext(ContextRequest{Root: root, Query: "GET /users"}); err == nil {
				t.Fatalf("invalid graph was accepted: %#v", index)
			}
		})
	}
}

func TestBuildContextRanksExactRouteAndExpandsBoundedProductionChain(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-16T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "regulation", Kind: "route", Name: "DELETE /cadasters/{cadasterId}/regulations/{objectId}", HTTPMethod: "DELETE", Path: "/cadasters/{cadasterId}/regulations/{objectId}", Qualified: "CadasterRegulationController.deleteFromCadaster", File: "Controller.java", Line: 182, Confidence: "EXACT", Search: "delete cadaster regulation"},
			{ID: "service", Project: "regulation", Kind: "symbol", Name: "remove", Qualified: "CadasterRegulationOperationsService.remove", File: "OperationsService.java", Line: 45, Confidence: "EXACT"},
			{ID: "test", Project: "regulation", Kind: "test", Name: "testDelete", File: "ControllerDeleteTest.java", Line: 59, Confidence: "RESOLVED"},
			{ID: "second-hop", Project: "regulation", Kind: "persistence", Name: "RegulationRepository.delete", File: "Repository.java", Line: 12},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "call", FromFactID: "route", ToFactID: "service", FromLabel: "controller", ToLabel: "service", Kind: "calls", Confidence: "EXACT"},
			{ID: "test-target", FromFactID: "test", ToFactID: "route", FromLabel: "test", ToLabel: "controller", Kind: "test_target", Confidence: "RESOLVED"},
			{ID: "second", FromFactID: "service", ToFactID: "second-hop", FromLabel: "service", ToLabel: "repository", Kind: "persistence"},
		},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
		BudgetTokens: 1800, MaxFiles: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.Entrypoints) == 0 || pack.Entrypoints[0].ID != "route" {
		t.Fatalf("ranked pack = %#v", pack)
	}
	if len(pack.CallChain) != 3 || len(pack.Tests) != 1 || pack.Tests[0].ID != "test" {
		t.Fatalf("bounded expansion = %#v", pack)
	}
	if len(pack.Persistence) != 1 || !contextPackHasFile(pack, "Repository.java") {
		t.Fatalf("production second hop is missing: %#v", pack)
	}
	if pack.CallChain[0].From == "call" || pack.CallChain[0].To == "route" {
		t.Fatalf("relationships should expose labels, not IDs: %#v", pack.CallChain)
	}
}

func TestBuildContextExpandsGermanTaskTermsForTechnicalFacts(t *testing.T) {
	query := "Wenn eine Vorschrift aus einem Kataster entfernt wird, bleiben die verbundenen Aufgaben bestehen."
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-16T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "route", Project: "regulation", Kind: "route",
				Name:       "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
				HTTPMethod: "DELETE", Path: "/cadasters/{cadasterId}/regulations/{objectId}",
				Qualified: "CadasterRegulationController.deleteFromCadaster",
				File:      "Controller.java", Line: 182, Confidence: "EXACT",
				Search: "delete cadaster regulation",
			},
			{
				ID: "service", Project: "regulation", Kind: "symbol",
				Name: "removeRegulation", Qualified: "CadasterRegulationOperationsService.removeRegulation",
				File: "OperationsService.java", Line: 45, Confidence: "EXACT",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "call", FromFactID: "route", ToFactID: "service",
			Kind: "call", Confidence: "EXACT",
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || pack.Confidence == "LOW" ||
		len(pack.Entrypoints) == 0 || pack.Entrypoints[0].ID != "route" ||
		len(pack.CallChain) != 1 {
		t.Fatalf("German task did not resolve technical facts: %#v", pack)
	}
	if pack.Query != query {
		t.Fatalf("reported query = %q, want original %q", pack.Query, query)
	}
}

func TestBuildContextPrioritizesPrimaryGermanActionOverAffectedEntity(t *testing.T) {
	query := "Wenn eine Vorschrift aus einem Kataster entfernt wird, bleiben die verbundenen Aufgaben bestehen."
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-16T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "regulation-route", Project: "z-regulation", Kind: "route",
				Name:       "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
				HTTPMethod: "DELETE", Path: "/cadasters/{cadasterId}/regulations/{objectId}",
				Qualified: "CadasterRegulationController.deleteFromCadaster",
				File:      "CadasterRegulationController.java", Line: 182, Confidence: "EXACT",
				Search: "delete cadasters cadaster regulations regulation",
			},
			{
				ID: "task-route", Project: "a-task", Kind: "route",
				Name:       "DELETE /cadasters/{cadasterId}/regulations/{objectId}/tasks/{taskId}",
				HTTPMethod: "DELETE", Path: "/cadasters/{cadasterId}/regulations/{objectId}/tasks/{taskId}",
				Qualified: "CadasterTaskController.deleteTask",
				File:      "CadasterTaskController.java", Line: 80, Confidence: "EXACT",
				Search: "delete cadasters cadaster regulations regulation tasks task",
			},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Entrypoints) == 0 || pack.Entrypoints[0].ID != "regulation-route" {
		t.Fatalf("primary removal action did not win over affected tasks: %#v", pack.Entrypoints)
	}
}

func TestBuildContextUsesProductionEntrypointsForLongAnalysisRequests(t *testing.T) {
	query := "Im Vorschriftendienst bleiben beim Entfernen einer Vorschrift aus einem Kataster die mit dieser Vorschrift verbundenen Aufgaben bestehen. " +
		"Analysiere repositoryübergreifend in ms-cadasterregulation, ms-cadastertask und ms-common den öffentlichen REST-Endpunkt, " +
		"die bestehende und benötigte Aufrufkette, Aufgabenarten und Suchattribute, internen API-Vertrag, Authentifizierung/Konfiguration, " +
		"Persistenz, Protokollierung/E-Mail/Benutzerinformationen, Produktions- und Testdateien sowie Fehlerbehandlung und Retry-Logik; keine Implementierung."
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-17T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "route", Project: "ms-cadasterregulation", Kind: "route",
				Name:       "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
				HTTPMethod: "DELETE", Path: "/cadasters/{cadasterId}/regulations/{objectId}",
				Qualified: "CadasterRegulationController.deleteFromCadaster",
				File:      "CadasterRegulationController.java", Line: 195, Confidence: "EXTRACTED",
				Search: "delete remove cadaster cadasters regulation regulations",
			},
			{
				ID: "controller", Project: "ms-cadasterregulation", Kind: "symbol",
				Name: "deleteFromCadaster", Qualified: "CadasterRegulationController.deleteFromCadaster",
				File: "CadasterRegulationController.java", Line: 196, Confidence: "EXTRACTED",
			},
			{
				ID: "service", Project: "ms-cadasterregulation", Kind: "symbol",
				Name: "deleteRegulationFromCadaster", Qualified: "CadasterRegulationOperationsService.deleteRegulationFromCadaster",
				File: "CadasterRegulationOperationsService.java", Line: 46, Confidence: "EXTRACTED",
			},
			{
				ID: "task-test", Project: "ms-cadastertask", Kind: "test",
				Name:      "testDeleteRegChangeTask_otherResponsible_withCc_mailSent",
				Qualified: "CadasterTaskMailTests.testDeleteRegChangeTask_otherResponsible_withCc_mailSent",
				File:      "CadasterTaskMailTests.java", Line: 800, Confidence: "MATCHED",
				Search: "delete regulation task responsible mail user retry test",
			},
			{
				ID: "query-test", Project: "ms-cadasterregulation", Kind: "test",
				Name:      "testGetRegulationChanges_changes_searchInComments_okay",
				Qualified: "CadasterRegulationControllerQueryTest.testGetRegulationChanges_changes_searchInComments_okay",
				File:      "CadasterRegulationControllerQueryTest.java", Line: 400, Confidence: "MATCHED",
				Search: "regulation changes search comments task test persistence api retry",
			},
			{
				ID: "test-class-symbol", Project: "ms-cadasterregulation", Kind: "symbol",
				Name:       "CadasterRegulationControllerDeleteTest",
				Qualified:  "com.weka.vd.api.cadasterregulation.controller.CadasterRegulationControllerDeleteTest",
				File:       "src/test/java/com/weka/vd/api/cadasterregulation/controller/CadasterRegulationControllerDeleteTest.java",
				Confidence: "EXACT",
				Search:     "delete remove cadaster cadasters regulation regulations related tasks test",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "route-controller", FromFactID: "route", ToFactID: "controller", Kind: "call", Reason: "flow", Confidence: "EXTRACTED"},
			{ID: "controller-service", FromFactID: "controller", ToFactID: "service", Kind: "call", Reason: "flow", Confidence: "EXTRACTED"},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "route" {
		t.Fatalf("long analysis request did not start at the production route: %#v", pack)
	}
	if len(pack.CallChain) != 2 || !contextPackHasFile(pack, "CadasterRegulationOperationsService.java") {
		t.Fatalf("long analysis request omitted the bounded production chain: %#v", pack)
	}
	for _, entrypoint := range pack.Entrypoints {
		if entrypoint.Kind == "test" {
			t.Fatalf("test leaked into production entrypoints: %#v", pack.Entrypoints)
		}
	}
}

func TestBuildContextFallsBackForUnreliableTestOnlyMatches(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "test", Project: "tasks", Kind: "test",
				Name: "testDeleteRegulationTaskRetry", File: "TaskTest.java",
				Confidence: "MATCHED", Search: "delete remove regulation regulations task retry",
			},
		},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "Entferne eine Vorschrift und analysiere Aufgaben sowie Retry-Logik",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || len(pack.Entrypoints) != 0 {
		t.Fatalf("unreliable test-only match must require source fallback: %#v", pack)
	}
}

func TestBuildContextIsDeterministicAcrossInputOrder(t *testing.T) {
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{
			{ID: "b", Project: "b", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users", File: "b.go", Line: 2, Confidence: "EXACT"},
			{ID: "a", Project: "a", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users", File: "a.go", Line: 1, Confidence: "EXACT"},
		},
		Coverage: []scan.AgentContextCoverageRecord{
			{Project: "b", Capability: "routes", Coverage: "PARTIAL", Reason: "b"},
			{Project: "a", Capability: "routes", Coverage: "COMPLETE", Reason: "a"},
		},
	}
	forwardRoot := writeContextIndexFixture(t, index)
	reversed := index
	reversed.Facts = append([]scan.AgentContextFactRecord(nil), index.Facts...)
	reversed.Coverage = append([]scan.AgentContextCoverageRecord(nil), index.Coverage...)
	reverseContextFacts(reversed.Facts)
	reverseContextCoverage(reversed.Coverage)
	reversedRoot := writeContextIndexFixture(t, reversed)

	forward, err := BuildContext(ContextRequest{Root: forwardRoot, Query: "GET /users"})
	if err != nil {
		t.Fatal(err)
	}
	backward, err := BuildContext(ContextRequest{Root: reversedRoot, Query: "GET /users"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(forward, backward) {
		t.Fatalf("pack depends on input order:\nforward: %#v\nreverse: %#v", forward, backward)
	}
	if forward.Entrypoints[0].ID != "a" {
		t.Fatalf("stable tie-break selected %q", forward.Entrypoints[0].ID)
	}
}

func TestBuildContextExactMatchingPreservesTokenOrder(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "persistence", Kind: "persistence", Name: "delete user",
			File: "repository.go", Line: 10, Confidence: "EXACT",
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "user delete"})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Confidence == "EXACT" {
		t.Fatalf("permuted query was treated as an exact name: %#v", pack)
	}
}

func TestBuildContextExpandsOnlyRetainedSeeds(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "top", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users", File: "top.go", Line: 10, Confidence: "EXACT"},
			{ID: "rejected-seed", Kind: "symbol", Name: "users", Search: "get users", File: "rejected.go", Line: 20, Confidence: "EXACT"},
			{ID: "neighbor", Kind: "symbol", Name: "audit", File: "top.go", Line: 30},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "rejected-edge", FromFactID: "rejected-seed", ToFactID: "neighbor",
			FromLabel: "users", ToLabel: "audit", Kind: "call",
		}},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "GET /users", BudgetTokens: 256, MaxFiles: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "top" {
		t.Fatalf("optional seed unexpectedly fit: %#v", pack.Entrypoints)
	}
	if len(pack.CallChain) != 0 {
		t.Fatalf("edge from rejected seed leaked into pack: %#v", pack.CallChain)
	}
}

func TestBuildContextKeepsOnlyHighestRankedProductionSeed(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "top", Project: "z-project", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users", File: "top.go", Confidence: "EXACT"},
			{ID: "lower", Project: "a-project", Kind: "symbol", Name: "users", Search: "get users", File: "lower.go", Confidence: "EXACT"},
			{ID: "top-test", Project: "z-project", Kind: "test", Name: "ZTopTest", File: "top_test.go"},
			{ID: "lower-test", Project: "a-project", Kind: "test", Name: "ALowerTest", File: "lower_test.go"},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "top-edge", FromFactID: "top-test", ToFactID: "top", FromLabel: "z-test", ToLabel: "z-top", Kind: "test_target"},
			{ID: "lower-edge", FromFactID: "lower-test", ToFactID: "lower", FromLabel: "a-test", ToLabel: "a-lower", Kind: "test_target"},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /users"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "top" {
		t.Fatalf("primary production entrypoint was not isolated: %#v", pack.Entrypoints)
	}
	if len(pack.CallChain) != 1 || pack.CallChain[0].From != "z-test" ||
		len(pack.Tests) != 1 || pack.Tests[0].ID != "top-test" {
		t.Fatalf("lower-ranked seed context leaked into the pack: relationships=%#v tests=%#v", pack.CallChain, pack.Tests)
	}
}

func TestBuildContextSortsCallChainByPublishedFields(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users", File: "route.go", Confidence: "EXACT"},
			{ID: "z", Kind: "symbol", Name: "z-neighbor", File: "z.go"},
			{ID: "a", Kind: "symbol", Name: "a-neighbor", File: "a.go"},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "fallback-labels", FromFactID: "route", ToFactID: "z", Kind: "call"},
			{ID: "explicit-labels", FromFactID: "route", ToFactID: "a", FromLabel: "A", ToLabel: "a-neighbor", Kind: "call"},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /users"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.CallChain) != 2 || pack.CallChain[0].From != "A" ||
		pack.CallChain[1].From != "GET /users" {
		t.Fatalf("call chain is not canonical by published fields: %#v", pack.CallChain)
	}
}

func TestBuildContextFallsBackWithoutQueryCascade(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "unrelated", Kind: "symbol", Name: "InvoiceService",
			File: "InvoiceService.java", Line: 10,
			Search: "invoice service",
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "remove regulation tasks"})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || pack.FallbackReason == "" || len(pack.Entrypoints) != 0 {
		t.Fatalf("fallback pack = %#v", pack)
	}
	if len(pack.Uncertainties) > 1 || pack.EstimatedTokens > 256 {
		t.Fatalf("fallback must be tiny: %#v", pack)
	}
}

func TestBuildContextScopesCoverageAndFallsBackWhenAllSelectedScopesFail(t *testing.T) {
	index := contextIndexWithFact("route", "GET users details")
	index.Facts[0].Project = "users"
	index.Facts[0].HTTPMethod = "GET"
	index.Facts[0].Path = "/users/details"
	index.Coverage = []scan.AgentContextCoverageRecord{
		{Project: "users", Capability: "routes", Coverage: "FAILED", Reason: "parser failed"},
		{Project: "other", Capability: "routes", Coverage: "FAILED", Reason: "unrelated"},
		{Project: "users", Capability: "tests", Coverage: "PARTIAL", Reason: "unrelated capability"},
	}
	root := writeContextIndexFixture(t, index)

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /users/details"})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || len(pack.Uncertainties) != 1 ||
		pack.Uncertainties[0].Scope != "users/routes" {
		t.Fatalf("scoped fallback = %#v", pack)
	}
}

func TestBuildContextUnmappedAuthenticationPreventsAllIncompleteFallback(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "app", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users", File: "route.go", Confidence: "EXACT"},
			{ID: "auth", Project: "app", Kind: "authentication", Name: "authenticated", File: "security.go", Confidence: "EXACT"},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "auth-edge", Project: "app", FromFactID: "route", ToFactID: "auth",
			FromLabel: "GET /users", ToLabel: "authenticated", Kind: "authentication",
		}},
		Coverage: []scan.AgentContextCoverageRecord{{
			Project: "app", Capability: "routes", Coverage: "FAILED", Reason: "route parser failed",
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /users"})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.CallChain) != 1 {
		t.Fatalf("unmapped retained authentication was treated as failed coverage: %#v", pack)
	}
}

func TestBuildContextHonorsFileLimitAcrossLocations(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users", File: "users.go", Line: 10, Confidence: "EXACT"},
			{ID: "test", Kind: "test", Name: "TestUsers", File: "users_test.go", Line: 20},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "test-target", FromFactID: "test", ToFactID: "route", Kind: "test_target",
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /users", MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Files) != 1 || len(pack.Tests) != 0 || uniqueContextPackFiles(pack) != 1 {
		t.Fatalf("file limit was not applied across locations: %#v", pack)
	}
}

func TestCloneContextPackDeepCopiesBudgetProbeSlices(t *testing.T) {
	original := ContextPack{
		Entrypoints: []ContextLocation{{ID: "entry", EvidenceIDs: []string{"evidence"}}},
		Files:       []ContextFile{{Path: "entry.go", Role: "entrypoint"}},
		Tests:       make([]ContextLocation, 1, 4),
	}
	clone := cloneContextPack(original)
	clone.Entrypoints[0].EvidenceIDs[0] = "changed"
	clone.Files[0].Role = "changed"
	clone.Tests = append(clone.Tests, ContextLocation{ID: "rejected"})

	if original.Entrypoints[0].EvidenceIDs[0] != "evidence" ||
		original.Files[0].Role != "entrypoint" ||
		len(original.Tests) != 1 {
		t.Fatalf("budget probe mutated original pack: %#v", original)
	}
}

func TestBuildContextPublishesFixedPointEstimate(t *testing.T) {
	root := writeContextIndexFixture(t, contextIndexWithFact("route", "GET users details"))
	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET users details"})
	if err != nil {
		t.Fatal(err)
	}
	estimated, err := EstimateContextTokens(pack)
	if err != nil {
		t.Fatal(err)
	}
	if pack.EstimatedTokens != estimated || pack.EstimatedTokens > pack.BudgetTokens {
		t.Fatalf("estimated tokens = %d, recalculated = %d, budget = %d", pack.EstimatedTokens, estimated, pack.BudgetTokens)
	}
}

func TestBuildContextAccountsForFinalConfidenceDuringBudgetProbes(t *testing.T) {
	request := ContextRequest{
		Query: "GET /users", BudgetTokens: 256, MaxFiles: DefaultContextMaxFiles,
	}
	for evidenceLength := 0; evidenceLength <= 600; evidenceLength++ {
		index := scan.AgentContextIndexRecord{
			SchemaVersion: scan.SchemaVersion,
			Facts: []scan.AgentContextFactRecord{{
				ID: "route", Kind: "route", Name: "GET /users",
				HTTPMethod: "GET", Path: "/users", File: "users.go",
				Confidence:  "EXACT",
				EvidenceIDs: []string{strings.Repeat("e", evidenceLength)},
			}},
		}
		pack, err := compileContextPack(index, request)
		if err != nil {
			t.Fatalf("evidence length %d caused a budget error: %v", evidenceLength, err)
		}
		if pack.EstimatedTokens > request.BudgetTokens {
			t.Fatalf("evidence length %d exceeded budget: %#v", evidenceLength, pack)
		}
	}
}

func TestBuildContextDropsOptionalRelationshipAtExactConfidenceBoundary(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users", File: "users.go", Confidence: "EXACT"},
			{ID: "handler", Kind: "symbol", Name: "handler", File: "users.go", Confidence: "EXACT"},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "edge", FromFactID: "route", ToFactID: "handler",
			FromLabel: "route", ToLabel: "handler", Kind: "call",
			Reason: strings.Repeat("r", 561),
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /users", BudgetTokens: 256})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Confidence != "EXACT" || pack.EstimatedTokens > 256 || len(pack.CallChain) != 0 {
		t.Fatalf("exact boundary pack retained oversized optional relationship: %#v", pack)
	}
}

func TestBuildContextCompactsEvidenceAtMediumConfidenceBoundary(t *testing.T) {
	evidenceIDs := make([]string, 14)
	for index := range evidenceIDs {
		evidenceIDs[index] = fmt.Sprintf("evidence:%032x", index)
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "seed", Kind: "symbol", Name: "operationxxxxxxxxxxxxxx",
			Qualified: "abcdefghijklmnop", Search: "delete users", File: "repo.go",
			EvidenceIDs: evidenceIDs,
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "delete users", BudgetTokens: 256})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Confidence != "MEDIUM" || pack.EstimatedTokens > 256 || len(pack.Entrypoints) != 1 {
		t.Fatalf("medium boundary pack = %#v", pack)
	}
	if len(pack.Entrypoints[0].EvidenceIDs) != len(evidenceIDs) {
		t.Fatalf("medium boundary retained %d evidence IDs, want pre-fix full list", len(pack.Entrypoints[0].EvidenceIDs))
	}
}

func TestBuildContextPreservesLargestFittingEvidencePrefix(t *testing.T) {
	evidenceIDs := make([]string, 30)
	for index := range evidenceIDs {
		evidenceIDs[index] = fmt.Sprintf("evidence:%032x", index)
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "route", Kind: "route", Name: "GET /users", HTTPMethod: "GET", Path: "/users",
			File: "users.go", Confidence: "EXACT", EvidenceIDs: evidenceIDs,
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /users", BudgetTokens: 256})
	if err != nil {
		t.Fatal(err)
	}
	retained := pack.Entrypoints[0].EvidenceIDs
	if len(retained) == 0 || len(retained) >= len(evidenceIDs) ||
		!reflect.DeepEqual(retained, evidenceIDs[:len(retained)]) {
		t.Fatalf("evidence prefix was not preserved deterministically: %#v", retained)
	}
	candidate := cloneContextPack(pack)
	candidate.Entrypoints[0].EvidenceIDs = append(candidate.Entrypoints[0].EvidenceIDs, evidenceIDs[len(retained)])
	candidate, err = finalizeContextEstimate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.EstimatedTokens <= 256 {
		t.Fatalf("retained evidence prefix was not maximal: current=%d candidate=%d", pack.EstimatedTokens, candidate.EstimatedTokens)
	}
}

func TestBuildContextEvidenceDoesNotDisplaceDirectRelationship(t *testing.T) {
	evidenceIDs := make([]string, 30)
	for index := range evidenceIDs {
		evidenceIDs[index] = fmt.Sprintf("evidence:%032x", index)
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "route", Kind: "route", Name: "GET /users",
				HTTPMethod: "GET", Path: "/users", File: "users.go",
				Confidence: "EXACT", EvidenceIDs: evidenceIDs,
			},
			{ID: "handler", Kind: "symbol", Name: "handler", File: "users.go"},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "call", FromFactID: "route", ToFactID: "handler",
			FromLabel: "GET /users", ToLabel: "handler", Kind: "call",
		}},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "GET /users", BudgetTokens: 256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.CallChain) != 1 || len(pack.Entrypoints) != 1 ||
		len(pack.Entrypoints[0].EvidenceIDs) == 0 {
		t.Fatalf("optional evidence displaced structural context: %#v", pack)
	}
	assertContextEvidencePrefixIsMaximal(
		t,
		pack,
		evidenceIDs,
		func(candidate *ContextPack, ids []string) {
			candidate.Entrypoints[0].EvidenceIDs = ids
		},
	)
}

func TestBuildContextFinalizesHighConfidenceBeforeEvidenceBackfill(t *testing.T) {
	evidenceIDs := make([]string, 200)
	for index := range evidenceIDs {
		evidenceIDs[index] = fmt.Sprintf("e%03d", index)
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "seed", Kind: "symbol", Name: "operation",
				Search: "delete users", File: "users.go",
			},
			{
				ID: "contract", Kind: "api_contract", Name: "POST /audit",
				HTTPMethod: "POST", Path: "/audit", File: "users.go",
				EvidenceIDs: evidenceIDs,
			},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "contract-edge", FromFactID: "seed", ToFactID: "contract",
			FromLabel: "operation", ToLabel: "POST /audit", Kind: "http_contract",
			Reason: "ok",
		}},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "delete users", BudgetTokens: 256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Confidence != "HIGH" || len(pack.CallChain) != 1 ||
		len(pack.Contracts) != 1 {
		t.Fatalf("relationship-dependent confidence or contract was lost: %#v", pack)
	}
	assertContextEvidencePrefixIsMaximal(
		t,
		pack,
		evidenceIDs,
		func(candidate *ContextPack, ids []string) {
			candidate.Contracts[0].EvidenceIDs = ids
		},
	)
}

func assertContextEvidencePrefixIsMaximal(
	t *testing.T,
	pack ContextPack,
	allEvidence []string,
	set func(*ContextPack, []string),
) {
	t.Helper()
	var retained []string
	switch {
	case len(pack.Contracts) > 0:
		retained = pack.Contracts[0].EvidenceIDs
	case len(pack.Entrypoints) > 0:
		retained = pack.Entrypoints[0].EvidenceIDs
	}
	if len(retained) >= len(allEvidence) {
		return
	}
	candidate := cloneContextPack(pack)
	next := append(append([]string(nil), retained...), allEvidence[len(retained)])
	set(&candidate, next)
	candidate, err := finalizeContextEstimate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.EstimatedTokens <= pack.BudgetTokens {
		t.Fatalf(
			"evidence prefix was not maximal: retained=%d candidate=%d budget=%d",
			len(retained),
			candidate.EstimatedTokens,
			pack.BudgetTokens,
		)
	}
}

func TestScopedContextUncertaintiesIgnoreUnknownCoverage(t *testing.T) {
	uncertainties, allIncomplete := scopedContextUncertainties(
		[]scan.AgentContextCoverageRecord{
			{Project: "app", Capability: "routes", Coverage: "UNKNOWN", Reason: "future value"},
			{Project: "app", Capability: "tests", Coverage: "", Reason: "missing value"},
		},
		map[string]bool{"app\x00routes": true, "app\x00tests": true},
	)
	if allIncomplete || len(uncertainties) != 0 {
		t.Fatalf("unknown coverage was treated as explicit failure: allIncomplete=%v uncertainties=%#v", allIncomplete, uncertainties)
	}
}

func TestBuildContextRejectsMandatoryEnvelopeOverflow(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion})
	if _, err := BuildContext(ContextRequest{
		Root: root, Query: strings.Repeat("very-long-query ", 500), BudgetTokens: 256,
	}); err == nil {
		t.Fatal("mandatory envelope overflow was accepted")
	}
}

func contextIndexWithFact(id, search string) scan.AgentContextIndexRecord {
	return scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{{
			ID: id, Kind: "route", Name: search, Search: search,
			File: id + ".go", Line: 1, Confidence: "EXACT",
		}},
	}
}

func writeContextIndexFixture(t *testing.T, index scan.AgentContextIndexRecord) string {
	t.Helper()
	root := t.TempDir()
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), index)
	return root
}

func writeContextIndexAt(t *testing.T, path string, index scan.AgentContextIndexRecord) {
	t.Helper()
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func contextPackHasFile(pack ContextPack, path string) bool {
	for _, file := range pack.Files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func uniqueContextPackFiles(pack ContextPack) int {
	files := map[string]bool{}
	add := func(project, path string) {
		if path != "" {
			files[project+"\x00"+path] = true
		}
	}
	for _, location := range pack.Entrypoints {
		add(location.Project, location.File)
	}
	for _, location := range pack.Contracts {
		add(location.Project, location.File)
	}
	for _, location := range pack.Persistence {
		add(location.Project, location.File)
	}
	for _, location := range pack.Tests {
		add(location.Project, location.File)
	}
	for _, file := range pack.Files {
		add(file.Project, file.Path)
	}
	return len(files)
}

func reverseContextFacts(values []scan.AgentContextFactRecord) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseContextCoverage(values []scan.AgentContextCoverageRecord) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
