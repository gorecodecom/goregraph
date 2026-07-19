package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func cmpContextJSON(left, right any) string {
	leftBody, _ := json.Marshal(left)
	rightBody, _ := json.Marshal(right)
	if string(leftBody) == string(rightBody) {
		return ""
	}
	return string(leftBody) + " != " + string(rightBody)
}

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

func TestContextBudgetsReserveSpaceForSource(t *testing.T) {
	if got := contextMetadataBudget(DefaultContextBudgetTokens); got != DefaultContextMetadataBudgetTokens {
		t.Fatalf("metadata budget = %d, want %d", got, DefaultContextMetadataBudgetTokens)
	}
	if got := contextMetadataBudget(700); got != 700 {
		t.Fatalf("small metadata budget = %d, want 700", got)
	}
	if got := contextByteBudget(DefaultContextBudgetTokens); got != DefaultContextMaxBytes {
		t.Fatalf("default byte budget = %d, want %d", got, DefaultContextMaxBytes)
	}
	if got := contextByteBudget(MaxContextBudgetTokens); got != MaxContextBytes {
		t.Fatalf("maximum byte budget = %d, want %d", got, MaxContextBytes)
	}
}

func TestBuildContextRejectsInvalidBounds(t *testing.T) {
	for _, request := range []ContextRequest{
		{Root: t.TempDir(), Query: "delete user", BudgetTokens: 255},
		{Root: t.TempDir(), Query: "delete user", BudgetTokens: 6001},
		{Root: t.TempDir(), Query: "delete user", MaxFiles: MinContextMaxFiles - 1},
		{Root: t.TempDir(), Query: "delete user", MaxFiles: MaxContextMaxFiles + 1},
		{Root: t.TempDir(), Query: "   "},
	} {
		if _, err := BuildContext(request); err == nil {
			t.Fatalf("accepted invalid request: %#v", request)
		}
	}
}

func TestContextMaxFilesSharesExportedMinimum(t *testing.T) {
	normalized, err := normalizeContextRequest(ContextRequest{Query: "delete user", MaxFiles: MinContextMaxFiles})
	if err != nil {
		t.Fatal(err)
	}
	if normalized.MaxFiles != MinContextMaxFiles {
		t.Fatalf("max files = %d, want %d", normalized.MaxFiles, MinContextMaxFiles)
	}

	source, err := os.ReadFile("context.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"request.MaxFiles < MinContextMaxFiles",
		"\n\t\t\tMinContextMaxFiles,\n\t\t\tMaxContextMaxFiles",
	} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("context max-files does not share bound %q", want)
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

func TestLoadContextIndexCarriesExactSourceScopes(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "services", "users")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, ".goregraph-workspace.yml"), []byte("workspace: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		root       string
		indexPath  string
		wantRoot   string
		workspace  bool
		outputFile string
	}{
		{
			name:       "nested configured project output",
			root:       projectRoot,
			indexPath:  filepath.Join(projectRoot, "build", "generated", "goregraph", "agent", "context-index.json"),
			wantRoot:   projectRoot,
			outputFile: "output: build/generated/goregraph\n",
		},
		{
			name:      "workspace requested directly",
			root:      workspaceRoot,
			indexPath: filepath.Join(workspaceRoot, ".goregraph-workspace", "agent", "context-index.json"),
			wantRoot:  workspaceRoot,
			workspace: true,
		},
		{
			name:      "detected parent workspace",
			root:      projectRoot,
			indexPath: filepath.Join(workspaceRoot, ".goregraph-workspace", "agent", "context-index.json"),
			wantRoot:  workspaceRoot,
			workspace: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.outputFile != "" {
				if err := os.WriteFile(filepath.Join(test.root, "goregraph.yml"), []byte(test.outputFile), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			writeContextIndexAt(t, test.indexPath, contextIndexWithFact(test.name, test.name))

			loaded, err := loadContextIndex(ContextRequest{Root: test.root})
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Path != test.indexPath || loaded.ScopeRoot != test.wantRoot || loaded.Workspace != test.workspace {
				t.Fatalf("loaded index = %#v, want path %q, root %q, workspace %t", loaded, test.indexPath, test.wantRoot, test.workspace)
			}
		})
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

func TestBuildContextRanksEmbeddedExact(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		target     scan.AgentContextFactRecord
		wantReason string
	}{
		{
			name:  "route",
			query: "Analyze broad dependent cleanup work around DELETE /cadasters/{cadasterId}/regulations/{objectId} before implementation.",
			target: scan.AgentContextFactRecord{
				ID: "route", Kind: "route", Name: "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
				HTTPMethod: "DELETE", Path: "/cadasters/{cadasterId}/regulations/{objectId}", File: "CadasterRegulationController.java", Confidence: "EXACT",
			},
			wantReason: "embedded exact route",
		},
		{
			name:  "qualified symbol",
			query: "Analyze broad dependent cleanup work around CadasterRegulationOperationsService.removeRegulation before implementation.",
			target: scan.AgentContextFactRecord{
				ID: "qualified", Kind: "symbol", Name: "removeRegulation", Qualified: "CadasterRegulationOperationsService.removeRegulation", File: "CadasterRegulationOperationsService.java", Confidence: "EXACT",
			},
			wantReason: "embedded exact qualified name",
		},
		{
			name:  "source file",
			query: "Analyze broad dependent cleanup work around src/main/java/example/CadasterRegulationController.java before implementation.",
			target: scan.AgentContextFactRecord{
				ID: "file", Kind: "symbol", Name: "removeRegulation", File: "src/main/java/example/CadasterRegulationController.java", Confidence: "EXACT",
			},
			wantReason: "embedded exact file",
		},
		{
			name:  "backtick name",
			query: "Analyze broad dependent cleanup work around `removeRegulation` before implementation.",
			target: scan.AgentContextFactRecord{
				ID: "name", Kind: "symbol", Name: "removeRegulation", File: "CadasterRegulationOperationsService.java", Confidence: "EXACT",
			},
			wantReason: "embedded exact name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			broad := scan.AgentContextFactRecord{
				ID: "broad", Kind: "symbol", Name: "reviewBroadCleanup", File: "BroadCleanup.java",
				Search: test.query, Confidence: "EXACT",
			}
			testSource := test.target
			testSource.ID = "test-source"
			testSource.File = "src/test/java/example/CadasterRegulationTest.java"
			testSource.Search = test.query

			ranked := rankContextFacts([]scan.AgentContextFactRecord{
				broad, testSource, test.target,
			}, test.query)
			seeds := selectContextSeeds(ranked)
			if len(seeds) != 1 || seeds[0].fact.ID != test.target.ID {
				t.Fatalf("first production seed = %#v, ranked = %#v, want %q", seeds, ranked, test.target.ID)
			}
			if seeds[0].score <= scoreExactRoute || seeds[0].reason != test.wantReason {
				t.Fatalf("embedded exact seed = %#v, want score above %d and reason %q", seeds[0], scoreExactRoute, test.wantReason)
			}
		})
	}
}

func TestContextQueryAnchorsPreserveFirstAppearance(t *testing.T) {
	query := "Inspect `removeRegulation` before DELETE /cadasters/{cadasterId}/regulations/{objectId} in src/main/java/example/CadasterRegulationController.java."
	want := []string{
		"removeRegulation",
		"DELETE /cadasters/{cadasterId}/regulations/{objectId}",
		"src/main/java/example/CadasterRegulationController.java",
	}
	if got := contextQueryAnchors(query); !reflect.DeepEqual(got, want) {
		t.Fatalf("contextQueryAnchors() = %#v, want %#v", got, want)
	}
}

func TestContextQueryAnchorsCapAtEight(t *testing.T) {
	query := "`one` `two` `three` `four` `five` `six` `seven` `eight` `nine`"
	want := []string{"one", "two", "three", "four", "five", "six", "seven", "eight"}
	if got := contextQueryAnchors(query); !reflect.DeepEqual(got, want) {
		t.Fatalf("contextQueryAnchors() = %#v, want %#v", got, want)
	}
}

func TestContextQueryAnchorsRejectValuesLongerThan256Runes(t *testing.T) {
	query := "`" + strings.Repeat("a", maximumContextQueryAnchorRunes+1) + "` `removeRegulation`"
	want := []string{"removeRegulation"}
	if got := contextQueryAnchors(query); !reflect.DeepEqual(got, want) {
		t.Fatalf("contextQueryAnchors() = %#v, want %#v", got, want)
	}
}

func TestContextQueryAnchorsRecognizeSupportedFilesAndSentencePunctuation(t *testing.T) {
	query := "Inspect scripts/release.sh. config/application.yaml, docs/context.md! data/catalog.json? and scripts/setup.zsh."
	want := []string{
		"scripts/release.sh",
		"config/application.yaml",
		"docs/context.md",
		"data/catalog.json",
		"scripts/setup.zsh",
	}
	if got := contextQueryAnchors(query); !reflect.DeepEqual(got, want) {
		t.Fatalf("contextQueryAnchors() = %#v, want %#v", got, want)
	}
}

func TestBuildContextReturnsOnlyRelevantEndpointSecurityAndBoundedConsumers(t *testing.T) {
	facts := []scan.AgentContextFactRecord{{
		ID: "endpoint", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders/{id}",
		Path: "/orders/{id}", HTTPMethod: "GET", Qualified: "OrderController.get",
		File: "services/orders/src/OrderController.java", Line: 20,
		Summary:    "provider orders; security bearer; request OrderRequest; response OrderResponse",
		Search:     "GET /orders/{id} authentication bearer services orders",
		Confidence: "EXACT",
	}}
	edges := make([]scan.AgentContextEdgeRecord, 0, 25)
	for index := 0; index < 25; index++ {
		id := fmt.Sprintf("consumer:%02d", index)
		facts = append(facts, scan.AgentContextFactRecord{
			ID: id, Project: fmt.Sprintf("frontend/web-%02d", index), Kind: "api_consumer", Name: id,
			File: fmt.Sprintf("frontend/web-%02d/src/api.ts", index), Line: 7,
			Summary: "consumer service web; auth bearer", Confidence: "RESOLVED",
		})
		edges = append(edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + id, FromFactID: id, ToFactID: "endpoint",
			Kind: "consumes_endpoint", Reason: "catalog consumer auth bearer", Confidence: "RESOLVED",
		})
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion, Facts: facts, Edges: edges,
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "who calls GET /orders/{id} and how is it authenticated",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 1 || pack.Endpoints[0].Path != "/orders/{id}" {
		t.Fatalf("endpoints = %#v", pack.Endpoints)
	}
	endpoint := pack.Endpoints[0]
	if endpoint.Provider != "services/orders" || endpoint.Handler != "OrderController.get" ||
		endpoint.RequestType != "OrderRequest" || endpoint.ResponseType != "OrderResponse" ||
		endpoint.Security != "bearer" {
		t.Fatalf("endpoint details = %#v", endpoint)
	}
	if len(endpoint.Consumers) != 8 || endpoint.OmittedConsumers != 17 {
		t.Fatalf("consumer bounds = %#v", endpoint)
	}
	for _, consumer := range endpoint.Consumers {
		if consumer.Authentication != "bearer" {
			t.Fatalf("consumer authentication = %#v", consumer)
		}
	}
	if pack.EstimatedTokens > pack.BudgetTokens {
		t.Fatalf("budget exceeded: %#v", pack)
	}
}

func TestBuildContextEndpointSecurityRendersUnknownExactly(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "endpoint", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders/{id}",
				Path: "/orders/{id}", HTTPMethod: "GET", File: "services/orders/src/OrderController.java",
				Summary: "provider orders; security unknown", Search: "GET /orders/{id} unknown",
				Confidence: "EXACT",
			},
			{
				ID: "consumer", Project: "frontend/web", Kind: "api_consumer", Name: "loadOrder",
				File: "frontend/web/src/api.ts", Line: 7, Summary: "consumer service web; auth unknown",
				Search: "public role dashboard labels unrelated to call authentication",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "consumer-edge", FromFactID: "consumer", ToFactID: "endpoint",
			Kind: "consumes_endpoint", Reason: "catalog consumer auth unknown",
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /orders/{id} authentication"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 1 || pack.Endpoints[0].Security != "No auth evidence detected" ||
		len(pack.Endpoints[0].Consumers) != 1 ||
		pack.Endpoints[0].Consumers[0].Authentication != "No auth evidence detected" {
		t.Fatalf("unknown authentication wording = %#v", pack.Endpoints)
	}
}

func TestBuildContextDisambiguatesSamePathEndpointProvider(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{
			ID: "orders", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders/{id}",
			HTTPMethod: "GET", Path: "/orders/{id}", File: "services/orders/src/OrderController.java",
			Summary: "provider orders; security bearer", Search: "GET /orders/{id} services orders bearer",
			Confidence: "EXACT",
		},
		{
			ID: "archive", Project: "services/archive", Kind: "api_endpoint", Name: "GET /orders/{id}",
			HTTPMethod: "GET", Path: "/orders/{id}", File: "services/archive/src/ArchiveController.java",
			Summary: "provider archive; security public", Search: "GET /orders/{id} services archive public",
			Confidence: "EXACT",
		},
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion, Facts: facts,
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "show GET /orders/{id} from services/archive",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 1 || pack.Endpoints[0].Provider != "services/archive" {
		t.Fatalf("provider collision = %#v", pack.Endpoints)
	}
}

func TestBuildContextDisambiguatesEndpointByCompactProviderService(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "orders", Project: "workspace/a", Kind: "api_endpoint", Name: "GET /orders/{id}",
				HTTPMethod: "GET", Path: "/orders/{id}", File: "workspace/a/src/Orders.java",
				Summary: "provider ordering-api; security bearer", Search: "GET /orders/{id} ordering-api bearer",
				Confidence: "EXACT",
			},
			{
				ID: "archive", Project: "workspace/b", Kind: "api_endpoint", Name: "GET /orders/{id}",
				HTTPMethod: "GET", Path: "/orders/{id}", File: "workspace/b/src/Archive.java",
				Summary: "provider archive-api; security public", Search: "GET /orders/{id} archive-api public",
				Confidence: "EXACT",
			},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "show GET /orders/{id} from archive-api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 1 || pack.Endpoints[0].Provider != "workspace/b" {
		t.Fatalf("compact provider service did not disambiguate endpoint: %#v", pack.Endpoints)
	}
}

func TestBuildContextRequiresProviderEvidenceForSamePathEndpointCollision(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "orders", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders", HTTPMethod: "GET", Path: "/orders", File: "services/orders/src/Orders.java", Search: "GET /orders services orders", Confidence: "EXACT"},
			{ID: "archive", Project: "services/archive", Kind: "api_endpoint", Name: "GET /orders", HTTPMethod: "GET", Path: "/orders", File: "services/archive/src/Orders.java", Search: "GET /orders services archive", Confidence: "EXACT"},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "show GET /orders"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 0 {
		t.Fatalf("ambiguous endpoint was selected: %#v", pack.Endpoints)
	}
}

func TestBuildContextConsidersBelowThresholdProviderInEndpointCollision(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "orders", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders",
				HTTPMethod: "GET", Path: "/orders", File: "services/orders/src/Orders.java",
				Summary: "provider orders; security bearer", Search: "orders authentication",
				Confidence: "EXACT",
			},
			{
				ID: "archive", Project: "services/archive", Kind: "api_endpoint", Name: "GET /orders",
				HTTPMethod: "GET", Path: "/orders", File: "services/archive/src/Orders.java",
				Summary: "provider archive", Search: "archive",
				Confidence: "EXACT",
			},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "orders authentication"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 0 {
		t.Fatalf("below-threshold provider was ignored during collision detection: %#v", pack.Endpoints)
	}
}

func TestBuildContextEndpointPrefersProductiveSourceOverTest(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{ID: "test-endpoint", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders", HTTPMethod: "GET", Path: "/orders", File: "services/orders/src/test/OrdersTest.java", Line: 10, Search: "GET /orders services orders", Confidence: "EXACT"},
		{ID: "production-endpoint", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders", HTTPMethod: "GET", Path: "/orders", File: "services/orders/src/main/Orders.java", Line: 20, Search: "GET /orders services orders", Confidence: "EXACT"},
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion, Facts: facts,
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /orders"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 1 || pack.Endpoints[0].File != "services/orders/src/main/Orders.java" {
		t.Fatalf("endpoint source preference = %#v", pack.Endpoints)
	}
}

func TestBuildContextUnrelatedQueryHasNoEndpointBlock(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "job", Project: "services/billing", Kind: "symbol", Name: "rebuild invoice cache", File: "services/billing/src/CacheJob.java", Confidence: "EXACT", Search: "rebuild invoice cache"},
			{ID: "endpoint", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders", HTTPMethod: "GET", Path: "/orders", File: "services/orders/src/Orders.java", Search: "GET /orders services orders", Confidence: "EXACT"},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "rebuild invoice cache"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 0 {
		t.Fatalf("unrelated endpoint leaked into context: %#v", pack.Endpoints)
	}
}

func TestBuildContextEndpointSelectionIsDeterministicAndOmitsGeneratedFiles(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{ID: "endpoint", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders", HTTPMethod: "GET", Path: "/orders", File: "services/orders/src/Orders.java", Search: "GET /orders services orders", Confidence: "EXACT"},
		{ID: "consumer-source", Project: "frontend/web", Kind: "api_consumer", Name: "loadOrders", File: "frontend/web/src/api.ts", Line: 4, Summary: "consumer service web; auth bearer"},
		{ID: "consumer-dashboard", Project: "frontend/web", Kind: "api_consumer", Name: "dashboard", File: ".goregraph-dashboard.json", Line: 1, Summary: "consumer service web; auth unknown"},
		{ID: "consumer-catalog", Project: "frontend/web", Kind: "api_consumer", Name: "catalog", File: ".goregraph-workspace/agent/api-catalog.json", Line: 1, Summary: "consumer service web; auth unknown"},
	}
	edges := []scan.AgentContextEdgeRecord{
		{ID: "source", FromFactID: "consumer-source", ToFactID: "endpoint", Kind: "consumes_endpoint", Reason: "catalog consumer auth bearer"},
		{ID: "dashboard", FromFactID: "consumer-dashboard", ToFactID: "endpoint", Kind: "consumes_endpoint", Reason: "catalog consumer auth unknown"},
		{ID: "catalog", FromFactID: "consumer-catalog", ToFactID: "endpoint", Kind: "consumes_endpoint", Reason: "catalog consumer auth unknown"},
	}
	build := func(reverse bool) ContextPack {
		t.Helper()
		indexFacts := append([]scan.AgentContextFactRecord(nil), facts...)
		indexEdges := append([]scan.AgentContextEdgeRecord(nil), edges...)
		if reverse {
			slices.Reverse(indexFacts)
			slices.Reverse(indexEdges)
		}
		root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
			SchemaVersion: scan.SchemaVersion, Facts: indexFacts, Edges: indexEdges,
		})
		pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /orders consumers"})
		if err != nil {
			t.Fatal(err)
		}
		return pack
	}

	forward := build(false)
	backward := build(true)
	if diff := cmpContextJSON(forward, backward); diff != "" {
		t.Fatalf("endpoint context depends on input order: %s", diff)
	}
	for _, file := range forward.Files {
		if strings.Contains(file.Path, ".goregraph-dashboard.json") || strings.Contains(file.Path, "api-catalog.json") {
			t.Fatalf("generated metadata file leaked into context: %#v", forward.Files)
		}
	}
}

func TestBuildContextExcludesGeneratedMetadataConsumers(t *testing.T) {
	facts := []scan.AgentContextFactRecord{{
		ID: "endpoint", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders",
		HTTPMethod: "GET", Path: "/orders", File: "services/orders/src/Orders.java",
		Summary: "provider orders; security bearer", Search: "GET /orders services orders bearer",
		Confidence: "EXACT",
	}}
	edges := make([]scan.AgentContextEdgeRecord, 0, 9)
	for index := 0; index < 7; index++ {
		id := fmt.Sprintf("consumer:%d", index)
		facts = append(facts, scan.AgentContextFactRecord{
			ID: id, Project: "frontend/web", Kind: "api_consumer", Name: id,
			File: fmt.Sprintf("frontend/web/src/api-%d.ts", index), Line: index + 1,
			Summary: "consumer service web; auth bearer",
		})
		edges = append(edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + id, FromFactID: id, ToFactID: "endpoint", Kind: "consumes_endpoint",
		})
	}
	for _, metadata := range []struct {
		id   string
		file string
	}{
		{id: "catalog", file: ".goregraph-workspace/agent/api-catalog.json"},
		{id: "dashboard", file: ".goregraph-dashboard.json"},
	} {
		facts = append(facts, scan.AgentContextFactRecord{
			ID: metadata.id, Project: "frontend/web", Kind: "api_consumer", Name: metadata.id,
			File: metadata.file, Line: 1, Summary: "consumer service web; auth unknown",
		})
		edges = append(edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + metadata.id, FromFactID: metadata.id, ToFactID: "endpoint", Kind: "consumes_endpoint",
		})
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion, Facts: facts, Edges: edges,
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /orders"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 1 || len(pack.Endpoints[0].Consumers) != 7 || pack.Endpoints[0].OmittedConsumers != 0 {
		t.Fatalf("generated consumers affected endpoint bounds: %#v", pack.Endpoints)
	}
	for _, consumer := range pack.Endpoints[0].Consumers {
		if strings.Contains(consumer.File, "api-catalog.json") || strings.Contains(consumer.File, ".goregraph-dashboard.json") {
			t.Fatalf("generated metadata consumer leaked: %#v", pack.Endpoints[0].Consumers)
		}
	}
}

func TestBuildContextEndpointSecurityDoesNotInferFromProjectOrSearch(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "endpoint", Project: "services/session", Kind: "api_endpoint", Name: "GET /health",
			HTTPMethod: "GET", Path: "/health", File: "services/session/src/Health.java",
			Summary: "provider session", Search: "GET /health services session",
			Confidence: "EXACT",
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /health"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 1 || pack.Endpoints[0].Security != "No auth evidence detected" {
		t.Fatalf("project or search aliases inferred endpoint security: %#v", pack.Endpoints)
	}
}

func TestBuildContextEndpointSecurityConfidenceRequiresExactRoute(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "orders", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders",
				HTTPMethod: "GET", Path: "/orders", File: "services/orders/src/Orders.java",
				Summary: "provider orders", Search: "GET /orders services orders", Confidence: "EXACT",
			},
			{
				ID: "orders-detail", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders/{id}",
				HTTPMethod: "GET", Path: "/orders/{id}", File: "services/orders/src/Orders.java",
				Summary: "provider orders; security bearer", Search: "GET /orders/{id} services orders bearer",
				Confidence: "EXACT",
			},
			{
				ID: "detail-security", Project: "services/orders", Kind: "endpoint_security", Name: "bearer",
				Qualified: "GET /orders/{id} bearer", Confidence: "EXACT",
			},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /orders"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 1 || pack.Endpoints[0].Path != "/orders" ||
		pack.Endpoints[0].Security != "No auth evidence detected" || pack.Endpoints[0].SecurityConfidence != "" {
		t.Fatalf("detail security leaked into collection endpoint: %#v", pack.Endpoints)
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

func TestBuildContextAddsSupportingFactsFromNamedProjects(t *testing.T) {
	query := "When a catalog entry is deleted, related jobs remain. Analyze services/catalog, services/jobs, and libraries/shared-model for the public endpoint, internal client authentication and retry, identifiers, persistence, and tests."
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "catalog-route", Project: "services/catalog", Kind: "route",
				Name: "DELETE /catalog/{catalogId}/entries/{entryId}", HTTPMethod: "DELETE",
				Path: "/catalog/{catalogId}/entries/{entryId}", File: "CatalogController.go",
				Confidence: "EXACT", Search: "delete catalog entry related",
			},
			{
				ID: "catalog-service", Project: "services/catalog", Kind: "symbol",
				Name: "deleteEntry", File: "CatalogService.go", Confidence: "EXACT",
			},
			{
				ID: "jobs-client", Project: "services/jobs", Kind: "symbol",
				Name: "deleteEntryJobs", File: "JobsClient.go", Confidence: "EXACT",
				Search: "delete entry jobs internal client authentication retry",
			},
			{
				ID: "jobs-secondary", Project: "services/jobs", Kind: "symbol",
				Name: "retryJobs", File: "JobsRetry.go", Confidence: "EXACT",
				Search: "jobs retry persistence",
			},
			{
				ID: "jobs-test-target", Project: "services/jobs", Kind: "test_target",
				Name: "deleteEntryJobsTarget", File: "JobsTarget.go", Confidence: "EXACT",
				Search: "delete entry jobs public endpoint internal client authentication retry identifiers persistence tests",
			},
			{
				ID: "jobs-test-source", Project: "services/jobs", Kind: "symbol",
				Name: "deleteEntryJobsTest", File: "src/test/JobsClient_test.go", Confidence: "EXACT",
				Search: "delete entry jobs internal client authentication retry persistence tests",
			},
			{
				ID: "jobs-empty-source", Project: "services/jobs", Kind: "symbol",
				Name: "deleteEntryJobsWithoutSource", Confidence: "EXACT",
				Search: "delete entry jobs public endpoint internal client authentication retry identifiers persistence tests",
			},
			{
				ID: "jobs-generated-metadata", Project: "services/jobs", Kind: "symbol",
				Name: "deleteEntryJobsMetadata", File: "api-catalog.json", Confidence: "EXACT",
				Search: "delete entry jobs public endpoint internal client authentication retry identifiers persistence tests",
			},
			{
				ID: "shared-model", Project: "libraries/shared-model", Kind: "symbol",
				Name: "JobReference", File: "JobReference.go", Confidence: "EXACT",
				Search: "entry job catalog identifier persistence",
			},
			{
				ID: "shared-metadata", Project: "libraries/shared-model", Kind: "metadata",
				Name: "SharedModelMetadata", File: "SharedModelMetadata.go", Confidence: "EXACT",
				Search: "entry job catalog identifier persistence tests",
			},
			{
				ID: "reporting", Project: "services/reporting", Kind: "symbol",
				Name: "deleteReport", File: "Reporting.go", Confidence: "EXACT",
				Search: "delete entry retry persistence",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "catalog-call", FromFactID: "catalog-route", ToFactID: "catalog-service",
			Kind: "call", Reason: "catalog deletion",
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "catalog-route" {
		t.Fatalf("support selection changed the primary entrypoint: %#v", pack)
	}
	if len(pack.CallChain) != 1 || pack.CallChain[0].From != "DELETE /catalog/{catalogId}/entries/{entryId}" || pack.CallChain[0].To != "deleteEntry" {
		t.Fatalf("support selection changed the primary chain: %#v", pack.CallChain)
	}
	for _, path := range []string{"CatalogController.go", "CatalogService.go", "JobsClient.go", "JobReference.go"} {
		if !contextPackHasFile(pack, path) {
			t.Fatalf("named project file %q missing from context: %#v", path, pack.Files)
		}
	}
	for _, path := range []string{"JobsRetry.go", "JobsTarget.go", "Reporting.go", "src/test/JobsClient_test.go", "api-catalog.json", "SharedModelMetadata.go"} {
		if contextPackHasFile(pack, path) {
			t.Fatalf("ineligible support file %q leaked into context: %#v", path, pack.Files)
		}
	}
	for _, file := range pack.Files {
		if file.Path == "JobsClient.go" || file.Path == "JobReference.go" {
			if file.Role != "related_project" || file.Reason != "full task project match" {
				t.Fatalf("support file metadata = %#v", file)
			}
		}
	}
}

func TestBuildContextAddsOnlyStrongCrossProjectSupportWhenProjectsAreUnnamed(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "catalog-route", Project: "services/catalog", Kind: "route",
				Name: "DELETE /catalog/{id}/entries/{entryId}", HTTPMethod: "DELETE",
				Path: "/catalog/{id}/entries/{entryId}", File: "CatalogController.go",
				Confidence: "EXACT", Search: "delete catalog entry related",
			},
			{
				ID: "worker-client", Project: "services/worker", Kind: "symbol",
				Name: "deleteEntryJobs", File: "WorkerClient.ts", Confidence: "EXACT",
				Search: "delete entry job internal client authentication retry",
			},
			{
				ID: "notification-retry", Project: "services/notifications", Kind: "symbol",
				Name: "retry", File: "RetryNotification.py", Confidence: "EXACT", Search: "retry",
			},
		},
	})

	pack, err := BuildContext(ContextRequest{
		Root:  root,
		Query: "When a catalog entry is deleted, related jobs remain. Analyze internal client authentication and retry behavior.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "catalog-route" {
		t.Fatalf("unnamed support changed the primary entrypoint: %#v", pack)
	}
	if !contextPackHasFile(pack, "WorkerClient.ts") {
		t.Fatalf("strong unnamed project support missing: %#v", pack.Files)
	}
	if contextPackHasFile(pack, "RetryNotification.py") {
		t.Fatalf("single-token unnamed support leaked into context: %#v", pack.Files)
	}
}

func TestBuildContextNamedProjectRequiresSemanticMatch(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog", HTTPMethod: "GET", Path: "/catalog", File: "Catalog.go", Confidence: "EXACT"},
			{ID: "worker", Project: "services/worker", Kind: "symbol", Name: "Worker", File: "Worker.go", Confidence: "EXACT", Search: "services worker"},
		},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /catalog. Analyze services/worker."})
	if err != nil {
		t.Fatal(err)
	}
	if contextPackHasFile(pack, "Worker.go") {
		t.Fatalf("project-name-only support was accepted: %#v", pack.Files)
	}
	if !contextPackHasUncertainty(pack, "services/worker/project_context", "no relevant production fact selected") {
		t.Fatalf("missing semantic project match was not surfaced: %#v", pack.Uncertainties)
	}
}

func TestBuildContextProjectAliasesRejectAmbiguousBasenames(t *testing.T) {
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog", HTTPMethod: "GET", Path: "/catalog", File: "Catalog.go", Confidence: "EXACT"},
			{ID: "service-jobs", Project: "services/jobs", Kind: "symbol", Name: "authenticateJobs", File: "ServiceJobs.go", Confidence: "EXACT", Search: "authentication"},
			{ID: "library-jobs", Project: "libraries/jobs", Kind: "symbol", Name: "persistJobs", File: "LibraryJobs.go", Confidence: "EXACT", Search: "persistence"},
			{ID: "service-shared", Project: "services/shared-model", Kind: "symbol", Name: "retryShared", File: "ServiceShared.go", Confidence: "EXACT", Search: "retry"},
			{ID: "library-shared", Project: "libraries/shared_model", Kind: "symbol", Name: "persistShared", File: "LibraryShared.go", Confidence: "EXACT", Search: "persistence"},
			{ID: "short-project", Project: "x", Kind: "symbol", Name: "authenticateShort", File: "ShortProject.go", Confidence: "EXACT", Search: "authentication"},
		},
	}

	bareRoot := writeContextIndexFixture(t, index)
	bare, err := BuildContext(ContextRequest{Root: bareRoot, Query: "GET /catalog. Analyze jobs authentication."})
	if err != nil {
		t.Fatal(err)
	}
	if contextPackHasFile(bare, "ServiceJobs.go") || contextPackHasFile(bare, "LibraryJobs.go") {
		t.Fatalf("ambiguous basename selected project support: %#v", bare.Files)
	}

	fullRoot := writeContextIndexFixture(t, index)
	full, err := BuildContext(ContextRequest{Root: fullRoot, Query: "GET /catalog. Analyze services/jobs authentication."})
	if err != nil {
		t.Fatal(err)
	}
	if !contextPackHasFile(full, "ServiceJobs.go") || contextPackHasFile(full, "LibraryJobs.go") {
		t.Fatalf("exact full path did not select only its project: %#v", full.Files)
	}

	for _, query := range []string{
		"GET /catalog. Analyze ./services/jobs authentication.",
		`GET /catalog. Analyze .\services\jobs authentication.`,
	} {
		root := writeContextIndexFixture(t, index)
		pack, buildErr := BuildContext(ContextRequest{Root: root, Query: query})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if !contextPackHasFile(pack, "ServiceJobs.go") || contextPackHasFile(pack, "LibraryJobs.go") {
			t.Fatalf("normalized full path %q did not select only its project: %#v", query, pack.Files)
		}
	}

	for _, query := range []string{
		"GET /catalog. Analyze workspace/services/jobs authentication.",
		"GET /catalog. Analyze services/jobs/archive authentication.",
	} {
		root := writeContextIndexFixture(t, index)
		pack, buildErr := BuildContext(ContextRequest{Root: root, Query: query})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if contextPackHasFile(pack, "ServiceJobs.go") || contextPackHasFile(pack, "LibraryJobs.go") {
			t.Fatalf("longer path %q falsely selected project support: %#v", query, pack.Files)
		}
	}

	normalizedBareRoot := writeContextIndexFixture(t, index)
	normalizedBare, err := BuildContext(ContextRequest{Root: normalizedBareRoot, Query: "GET /catalog. Analyze shared model retry."})
	if err != nil {
		t.Fatal(err)
	}
	if contextPackHasFile(normalizedBare, "ServiceShared.go") || contextPackHasFile(normalizedBare, "LibraryShared.go") {
		t.Fatalf("normalized ambiguous basename selected project support: %#v", normalizedBare.Files)
	}

	normalizedFullRoot := writeContextIndexFixture(t, index)
	normalizedFull, err := BuildContext(ContextRequest{Root: normalizedFullRoot, Query: "GET /catalog. Analyze services/shared-model retry."})
	if err != nil {
		t.Fatal(err)
	}
	if !contextPackHasFile(normalizedFull, "ServiceShared.go") || contextPackHasFile(normalizedFull, "LibraryShared.go") {
		t.Fatalf("normalized full path did not select only its project: %#v", normalizedFull.Files)
	}

	shortRoot := writeContextIndexFixture(t, index)
	short, err := BuildContext(ContextRequest{Root: shortRoot, Query: "GET /catalog. Analyze x authentication."})
	if err != nil {
		t.Fatal(err)
	}
	if !contextPackHasFile(short, "ShortProject.go") {
		t.Fatalf("exact one-character full path did not select its project: %#v", short.Files)
	}
}

func TestBuildContextNamedProjectCoverageAddsUncertaintyWithoutFallback(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog",
			HTTPMethod: "GET", Path: "/catalog", File: "Catalog.go", Confidence: "EXACT",
		}},
		Coverage: []scan.AgentContextCoverageRecord{{
			Project: "services/jobs", Capability: "calls", Coverage: "UNAVAILABLE",
			Reason: "project agent context projection unavailable",
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /catalog. Analyze services/jobs retry behavior."})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "route" {
		t.Fatalf("missing named support changed reliable primary selection: %#v", pack)
	}
	if !contextPackHasUncertainty(pack, "services/jobs/project_context", "project agent context projection unavailable") {
		t.Fatalf("missing named project coverage was silent: %#v", pack.Uncertainties)
	}
}

func TestBuildContextNamedProjectSupportRetainsCoverageUncertainty(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog", HTTPMethod: "GET", Path: "/catalog", File: "Catalog.go", Confidence: "EXACT"},
			{ID: "worker", Project: `services\worker`, Kind: "symbol", Name: "retryCatalog", File: "Worker.go", Confidence: "EXACT", Search: "retry authentication"},
		},
		Coverage: []scan.AgentContextCoverageRecord{{
			Project: "./services/worker", Capability: "calls", Coverage: "PARTIAL",
			Reason: "dynamic calls may be unresolved",
		}},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "GET /catalog. Analyze services/worker retry authentication.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || !contextPackHasFile(pack, "Worker.go") {
		t.Fatalf("support coverage changed support selection or fallback: %#v", pack)
	}
	if !contextPackHasUncertainty(pack, "services/worker/calls", "dynamic calls may be unresolved") {
		t.Fatalf("accepted support coverage was not surfaced: %#v", pack.Uncertainties)
	}
}

func TestBuildContextNamedProjectCoverageUncertaintySurvivesGlobalCap(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog", HTTPMethod: "GET", Path: "/catalog", File: "Catalog.go", Confidence: "EXACT"},
			{ID: "handler", Project: "services/catalog", Kind: "symbol", Name: "getCatalog", File: "CatalogService.go"},
			{ID: "contract", Project: "services/catalog", Kind: "api_contract", Name: "CatalogClient", File: "CatalogClient.go"},
			{ID: "persistence", Project: "services/catalog", Kind: "persistence", Name: "CatalogRepository.find", File: "CatalogRepository.go"},
			{ID: "test", Project: "services/catalog", Kind: "test", Name: "TestCatalog", File: "catalog_test.go"},
			{ID: "auth", Project: "services/catalog", Kind: "authentication", Name: "authenticated", File: "Security.go"},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "handler-edge", FromFactID: "route", ToFactID: "handler", Kind: "call"},
			{ID: "contract-edge", FromFactID: "route", ToFactID: "contract", Kind: "http_contract"},
			{ID: "persistence-edge", FromFactID: "route", ToFactID: "persistence", Kind: "persistence"},
			{ID: "test-edge", FromFactID: "test", ToFactID: "route", Kind: "test_target"},
			{ID: "auth-edge", FromFactID: "route", ToFactID: "auth", Kind: "authentication"},
		},
		Coverage: []scan.AgentContextCoverageRecord{
			{Project: "services/catalog", Capability: "api_clients", Coverage: "PARTIAL", Reason: "contracts partial"},
			{Project: "services/catalog", Capability: "calls", Coverage: "PARTIAL", Reason: "calls partial"},
			{Project: "services/catalog", Capability: "persistence", Coverage: "PARTIAL", Reason: "persistence partial"},
			{Project: "services/catalog", Capability: "routes", Coverage: "PARTIAL", Reason: "routes partial"},
			{Project: "services/catalog", Capability: "tests", Coverage: "PARTIAL", Reason: "tests partial"},
			{Project: "services/missing", Capability: "calls", Coverage: "UNAVAILABLE", Reason: "projection unavailable"},
		},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "GET /catalog. Analyze services/missing retry behavior.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired {
		t.Fatalf("missing support changed the reliable primary fallback decision: %#v", pack)
	}
	if len(pack.Uncertainties) != maximumContextUncertainty ||
		!contextPackHasUncertainty(pack, "services/missing/project_context", "projection unavailable") {
		t.Fatalf("missing project uncertainty was displaced by primary coverage: %#v", pack.Uncertainties)
	}
	seen := map[string]bool{}
	for _, uncertainty := range pack.Uncertainties {
		key := uncertainty.Scope + "\x00" + uncertainty.Reason
		if seen[key] {
			t.Fatalf("duplicate uncertainty leaked into context: %#v", pack.Uncertainties)
		}
		seen[key] = true
	}
}

func TestBuildContextProjectSupportProtectsPrimaryBudget(t *testing.T) {
	longSupportFile := strings.Repeat("worker/", 90) + "WorkerClient.java"
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog", HTTPMethod: "GET", Path: "/catalog", File: "Catalog.go", Confidence: "EXACT"},
			{ID: "worker", Project: "services/worker", Kind: "symbol", Name: "retryCatalog", File: longSupportFile, Confidence: "EXACT", Search: "retry authentication"},
		},
	}

	largeRoot := writeContextIndexFixture(t, index)
	large, err := BuildContext(ContextRequest{Root: largeRoot, Query: "GET /catalog. Analyze services/worker retry authentication."})
	if err != nil {
		t.Fatal(err)
	}
	if !contextPackHasFile(large, longSupportFile) {
		t.Fatalf("support fixture did not fit the default budget: %#v", large.Files)
	}

	smallRoot := writeContextIndexFixture(t, index)
	small, err := BuildContext(ContextRequest{
		Root: smallRoot, Query: "GET /catalog. Analyze services/worker retry authentication.", BudgetTokens: 256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if small.FallbackRequired || len(small.Entrypoints) != 1 || small.Entrypoints[0].ID != "route" {
		t.Fatalf("optional support displaced the primary entrypoint: %#v", small)
	}
	if contextPackHasFile(small, longSupportFile) {
		t.Fatalf("oversized optional support exceeded the budget: %#v", small)
	}
}

func TestBuildContextProjectSupportContinuesAfterRejectedCandidates(t *testing.T) {
	t.Run("smaller fact from same project", func(t *testing.T) {
		oversizedFile := strings.Repeat("worker/", 90) + "OversizedWorker.go"
		root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
			SchemaVersion: scan.SchemaVersion,
			Facts: []scan.AgentContextFactRecord{
				{ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog", HTTPMethod: "GET", Path: "/catalog", File: "Catalog.go", Confidence: "EXACT"},
				{ID: "worker-large", Project: "services/worker", Kind: "symbol", Name: "retryCatalogLarge", File: oversizedFile, Confidence: "EXACT", Search: "retry authentication persistence"},
				{ID: "worker-small", Project: "services/worker", Kind: "symbol", Name: "retryCatalog", File: "Worker.go", Confidence: "EXACT", Search: "retry authentication"},
			},
		})

		pack, err := BuildContext(ContextRequest{
			Root: root, Query: "GET /catalog. Analyze services/worker retry authentication persistence.", BudgetTokens: 256,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !contextPackHasFile(pack, "Worker.go") || contextPackHasFile(pack, oversizedFile) {
			t.Fatalf("rejected top fact blocked smaller same-project support: %#v", pack.Files)
		}
	})

	t.Run("later project", func(t *testing.T) {
		workerFile := strings.Repeat("worker/", 90) + "OversizedWorker.go"
		sharedFile := strings.Repeat("shared/", 90) + "OversizedShared.go"
		root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
			SchemaVersion: scan.SchemaVersion,
			Facts: []scan.AgentContextFactRecord{
				{ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog", HTTPMethod: "GET", Path: "/catalog", File: "Catalog.go", Confidence: "EXACT"},
				{ID: "worker-large", Project: "services/worker", Kind: "symbol", Name: "retryCatalog", File: workerFile, Confidence: "EXACT", Search: "retry authentication persistence"},
				{ID: "shared-large", Project: "libraries/shared", Kind: "symbol", Name: "CatalogIdentifier", File: sharedFile, Confidence: "EXACT", Search: "identifier persistence"},
				{ID: "notifications", Project: "services/notifications", Kind: "symbol", Name: "notifyRetry", File: "Notification.go", Confidence: "EXACT", Search: "retry delivery"},
			},
		})

		pack, err := BuildContext(ContextRequest{
			Root:         root,
			Query:        "GET /catalog. Analyze services/worker retry authentication persistence, libraries/shared identifier persistence, and services/notifications retry delivery.",
			BudgetTokens: 256,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !contextPackHasFile(pack, "Notification.go") || contextPackHasFile(pack, workerFile) || contextPackHasFile(pack, sharedFile) {
			t.Fatalf("rejected projects blocked later qualifying support: %#v", pack.Files)
		}
	})
}

func TestBuildContextProjectSupportIsDeterministicAcrossInputOrder(t *testing.T) {
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog", HTTPMethod: "GET", Path: "/catalog", File: "Catalog.go", Confidence: "EXACT"},
			{ID: "worker-z", Project: "services/worker", Kind: "symbol", Name: "retryZ", File: "WorkerZ.go", Confidence: "EXACT", Search: "retry authentication"},
			{ID: "worker-a", Project: "services/worker", Kind: "symbol", Name: "retryA", File: "WorkerA.go", Confidence: "EXACT", Search: "retry authentication"},
			{ID: "shared", Project: "libraries/shared", Kind: "symbol", Name: "CatalogIdentifier", File: "Identifier.kt", Confidence: "EXACT", Search: "catalog identifier persistence"},
		},
		Edges: []scan.AgentContextEdgeRecord{{ID: "self", FromFactID: "route", ToFactID: "route", Kind: "call"}},
		Coverage: []scan.AgentContextCoverageRecord{
			{Project: "services/worker", Capability: "calls", Coverage: "COMPLETE"},
			{Project: "libraries/shared", Capability: "persistence", Coverage: "PARTIAL", Reason: "some stores unresolved"},
		},
	}
	reversed := index
	reversed.Facts = append([]scan.AgentContextFactRecord(nil), index.Facts...)
	reversed.Edges = append([]scan.AgentContextEdgeRecord(nil), index.Edges...)
	reversed.Coverage = append([]scan.AgentContextCoverageRecord(nil), index.Coverage...)
	slices.Reverse(reversed.Facts)
	slices.Reverse(reversed.Edges)
	slices.Reverse(reversed.Coverage)

	query := "GET /catalog. Analyze services/worker retry authentication and libraries/shared identifier persistence."
	forwardRoot := writeContextIndexFixture(t, index)
	reverseRoot := writeContextIndexFixture(t, reversed)
	forward, err := BuildContext(ContextRequest{Root: forwardRoot, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	backward, err := BuildContext(ContextRequest{Root: reverseRoot, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	if !contextPackHasFile(forward, "WorkerA.go") || !contextPackHasFile(forward, "Identifier.kt") {
		t.Fatalf("determinism fixture did not select expected supports: %#v", forward.Files)
	}
	if !reflect.DeepEqual(forward, backward) {
		t.Fatalf("project support depends on input order:\nforward: %#v\nreverse: %#v", forward, backward)
	}
}

func TestBuildContextProjectSupportAcceptsMixedFileExtensions(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "route", Name: "GET /catalog", HTTPMethod: "GET", Path: "/catalog", File: "CatalogController.go", Confidence: "EXACT"},
			{ID: "web", Project: "clients/web", Kind: "symbol", Name: "retryCatalog", File: "catalog-client.ts", Confidence: "EXACT", Search: "retry authentication"},
			{ID: "model", Project: "libraries/model", Kind: "symbol", Name: "CatalogIdentifier", File: "CatalogIdentifier.java", Confidence: "EXACT", Search: "catalog identifier persistence"},
		},
	})

	pack, err := BuildContext(ContextRequest{
		Root:  root,
		Query: "GET /catalog. Analyze clients/web retry authentication and libraries/model identifier persistence.",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"CatalogController.go", "catalog-client.ts", "CatalogIdentifier.java"} {
		if !contextPackHasFile(pack, path) {
			t.Fatalf("mixed-extension file %q missing from context: %#v", path, pack.Files)
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

func TestBuildContextRetainsOnlyFirstIndexedEvidence(t *testing.T) {
	evidenceIDs := []string{"evidence:first", "evidence:second", "evidence:third"}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "seed", Kind: "symbol", Name: "deleteUsers",
			Search: "delete users", File: "repo.go",
			EvidenceIDs: evidenceIDs,
		}},
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "delete users"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Entrypoints) != 1 || !reflect.DeepEqual(pack.Entrypoints[0].EvidenceIDs, evidenceIDs[:1]) {
		t.Fatalf("entrypoint evidence = %#v, want %#v", pack.Entrypoints, evidenceIDs[:1])
	}
}

func TestBuildContextAttachesCentralSourcePath(t *testing.T) {
	root := writeSourceBackedContextFixture(t, false)

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.SourceSections) < 2 {
		t.Fatalf("source-backed pack = %#v", pack)
	}
	if pack.SourceSections[0].Role != "entrypoint" || pack.SourceSections[0].RenderMode != "body" {
		t.Fatalf("first section = %#v", pack.SourceSections[0])
	}
	if !strings.Contains(pack.SourceSections[0].Content, "deleteFromCadaster") ||
		!strings.Contains(pack.SourceSections[1].Content, "removeRegulation") {
		t.Fatalf("central source path is missing: %#v", pack.SourceSections)
	}
	if pack.SourceCoverage != "complete" || len(pack.SourceOmissions) != 0 || pack.SourceUnrepresented != 0 {
		t.Fatalf("complete source coverage = %#v", pack)
	}
}

func TestBuildContextSourceFallbackAndLowConfidenceContainNoBodies(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "weak", Kind: "symbol", Name: "unrelatedThing", File: "missing.go",
			Search: "unrelated thing",
		}},
	})

	for _, query := range []string{"nothing relevant here", "unrelated thing"} {
		pack, err := BuildContext(ContextRequest{Root: root, Query: query})
		if err != nil {
			t.Fatal(err)
		}
		if len(pack.SourceSections) != 0 || pack.SourceCoverage != "none" {
			t.Fatalf("query %q source fallback = %#v", query, pack)
		}
	}
}

func TestBuildContextSourceIncludesTestsOnlyWhenRequested(t *testing.T) {
	root := writeSourceBackedContextFixture(t, false)
	route := "DELETE /cadasters/{cadasterId}/regulations/{objectId}"

	production, err := BuildContext(ContextRequest{Root: root, Query: route})
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range production.SourceSections {
		if section.Role == "test" || strings.Contains(section.Content, "deletesRegulation") {
			t.Fatalf("production query included test source: %#v", production.SourceSections)
		}
	}
	if len(production.Tests) != 1 {
		t.Fatalf("test metadata was not retained: %#v", production.Tests)
	}

	withTests, err := BuildContext(ContextRequest{Root: root, Query: route + ". tests"})
	if err != nil {
		t.Fatal(err)
	}
	testIndex := -1
	for index, section := range withTests.SourceSections {
		if section.Role == "test" {
			testIndex = index
		}
	}
	if testIndex < 2 || !strings.Contains(withTests.SourceSections[testIndex].Content, "deletesRegulation") {
		t.Fatalf("explicit test query source = %#v", withTests.SourceSections)
	}
}

func TestBuildContextSourceOperationalFailuresBecomeStableOmissions(t *testing.T) {
	for _, test := range []struct {
		name   string
		body   []byte
		remove bool
		mode   os.FileMode
		want   string
	}{
		{name: "missing", remove: true, want: "source file is missing"},
		{name: "unreadable", body: []byte("func target() {}\n"), mode: 0, want: "source file is unreadable"},
		{name: "non UTF-8", body: []byte{0xff, 0xfe}, mode: 0o644, want: "source file is not UTF-8 text"},
	} {
		t.Run(test.name, func(t *testing.T) {
			index := scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts: []scan.AgentContextFactRecord{{
					ID: "target", Kind: "symbol", Name: "target", File: "target.go",
					Line: 1, EndLine: 1, Search: "target", Confidence: "EXACT",
				}},
			}
			root := writeContextIndexFixture(t, index)
			path := filepath.Join(root, "target.go")
			if !test.remove {
				if err := os.WriteFile(path, test.body, test.mode); err != nil {
					t.Fatal(err)
				}
			}

			pack, err := BuildContext(ContextRequest{Root: root, Query: "target"})
			if err != nil {
				t.Fatal(err)
			}
			if len(pack.SourceSections) != 0 || len(pack.SourceOmissions) != 1 ||
				pack.SourceOmissions[0].Reason != test.want || pack.SourceCoverage != "none" {
				t.Fatalf("operational omission = %#v", pack)
			}
		})
	}
}

func TestBuildContextSourceCapsRemainExplicit(t *testing.T) {
	const candidateCount = 9
	facts := make([]scan.AgentContextFactRecord, 0, candidateCount)
	edges := make([]scan.AgentContextEdgeRecord, 0, candidateCount-1)
	facts = append(facts, scan.AgentContextFactRecord{
		ID: "seed", Kind: "symbol", Name: "centralTask", File: "source-0.go",
		Line: 1, EndLine: 1, Search: "central task", Confidence: "EXACT",
	})
	for index := 1; index < candidateCount; index++ {
		id := fmt.Sprintf("step-%d", index)
		facts = append(facts, scan.AgentContextFactRecord{
			ID: id, Kind: "symbol", Name: id, File: fmt.Sprintf("source-%d.go", index),
			Line: 1, EndLine: 1, Confidence: "EXACT",
		})
		edges = append(edges, scan.AgentContextEdgeRecord{
			ID: "edge-" + id, FromFactID: "seed", ToFactID: id,
			FromLabel: "centralTask", ToLabel: id, Kind: "call", Confidence: "EXACT",
		})
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion, Facts: facts, Edges: edges,
	})
	for _, fact := range facts {
		writeContextSourceFile(t, root, fact.File, "func "+fact.Name+"() {}\n")
	}

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "central task", BudgetTokens: MaxContextBudgetTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.SourceSections) != MaxContextSourceSections ||
		len(pack.SourceOmissions) != MaxContextSourceOmissions ||
		pack.SourceUnrepresented != candidateCount-MaxContextSourceSections-MaxContextSourceOmissions ||
		pack.SourceCoverage != "partial" {
		t.Fatalf("bounded source accounting = %#v", pack)
	}
}

func TestContextSourceCandidatesPreserveSelectedFactsAndMergeOnlyOverlaps(t *testing.T) {
	pack := ContextPack{
		Query:       "production path",
		Entrypoints: []ContextLocation{{ID: "entry"}, {ID: "second-entry"}},
		Contracts:   []ContextLocation{{ID: "contract"}},
		selectedSourceFactIDs: []string{
			"entry", "overlap", "bridge", "second-entry", "contract", "test", "generated", "empty",
		},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "entry", Project: "app", Kind: "symbol", Name: "entry", File: "app.go", Line: 10, EndLine: 20},
		{ID: "overlap", Project: "app", Kind: "symbol", Name: "overlap", File: "app.go", Line: 18, EndLine: 30},
		{ID: "bridge", Project: "app", Kind: "symbol", Name: "bridge", File: "app.go", Line: 30, EndLine: 40},
		{ID: "second-entry", Project: "app", Kind: "symbol", Name: "secondEntry", File: "app.go", Line: 40, EndLine: 50},
		{ID: "contract", Project: "app", Kind: "api_contract", Name: "contract", File: "contract.go", Line: 4, EndLine: 8},
		{ID: "test", Project: "app", Kind: "test", Name: "testEntry", File: "app_test.go", Line: 1, EndLine: 3},
		{ID: "generated", Project: "app", Kind: "symbol", Name: "generated", File: "build/generated/api-catalog.json", Line: 1},
		{ID: "empty", Project: "app", Kind: "symbol", Name: "empty"},
	}}

	candidates := contextSourceCandidates(pack, index)
	if len(candidates) != 2 {
		t.Fatalf("production candidates = %#v", candidates)
	}
	if candidates[0].FactID != "entry" || candidates[0].Role != "entrypoint" ||
		candidates[0].StartLine != 10 || candidates[0].EndLine != 50 || candidates[0].Name != "entry" {
		t.Fatalf("merged entrypoint candidate = %#v", candidates[0])
	}
	if candidates[1].FactID != "contract" || candidates[1].Role != "contract" {
		t.Fatalf("contract candidate = %#v", candidates[1])
	}
}

func TestContextSourceCandidatesMatchAPIEndpointExactly(t *testing.T) {
	pack := ContextPack{
		Query:       "GET /users",
		Entrypoints: []ContextLocation{{ID: "route"}},
		Endpoints: []ContextEndpoint{{
			Provider: "users", HTTPMethod: "GET", Path: "/users", Handler: "UsersAPI.list",
			File: "Users.java", Line: 20,
		}},
		selectedSourceFactIDs: []string{"endpoint", "endpoint-impostor", "route"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "route", Project: "users", Kind: "route", Name: "GET /users", Qualified: "UsersController.list", HTTPMethod: "GET", Path: "/users", File: "Users.java", Line: 5, EndLine: 8},
		{ID: "endpoint", Project: "users", Kind: "api_endpoint", Name: "GET /users", Qualified: "UsersAPI.list", HTTPMethod: "GET", Path: "/users", File: "Users.java", Line: 20, EndLine: 23},
		{ID: "endpoint-impostor", Project: "users", Kind: "api_endpoint", Name: "GET /users", Qualified: "OtherAPI.list", HTTPMethod: "GET", Path: "/users", File: "Other.java", Line: 20, EndLine: 23},
	}}

	candidates := contextSourceCandidates(pack, index)
	if len(candidates) != 3 || candidates[0].FactID != "route" || candidates[1].FactID != "endpoint" ||
		candidates[0].Role != "entrypoint" || candidates[1].Role != "entrypoint" ||
		candidates[2].FactID != "endpoint-impostor" || candidates[2].Role != "call_chain" {
		t.Fatalf("endpoint candidates = %#v", candidates)
	}
}

func TestContextSourceCandidatesGateEntrypointAndTestFileBodies(t *testing.T) {
	pack := ContextPack{
		Query:                 "production behavior",
		Entrypoints:           []ContextLocation{{ID: "exact-test"}},
		selectedSourceFactIDs: []string{"exact-test", "helper"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "exact-test", Kind: "test", Name: "TestBehavior", File: "behavior.go", Line: 1},
		{ID: "helper", Kind: "symbol", Name: "testHelper", File: "behavior_test.go", Line: 2},
	}}

	if candidates := contextSourceCandidates(pack, index); len(candidates) != 0 {
		t.Fatalf("production query included test-source candidates: %#v", candidates)
	}
	pack.Query = "production behavior. tests"
	candidates := contextSourceCandidates(pack, index)
	if len(candidates) != 2 || candidates[0].Role != "test" || candidates[1].Role != "test" {
		t.Fatalf("test query candidates = %#v", candidates)
	}
}

func TestBuildContextSourceJSONIsPrivateAndInputOrderDeterministic(t *testing.T) {
	forwardRoot := writeSourceBackedContextFixture(t, false)
	reverseRoot := writeSourceBackedContextFixture(t, true)
	request := ContextRequest{Query: "DELETE /cadasters/{cadasterId}/regulations/{objectId}"}
	request.Root = forwardRoot
	forward, err := BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	request.Root = reverseRoot
	reversed, err := BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	forwardJSON, err := json.Marshal(forward)
	if err != nil {
		t.Fatal(err)
	}
	reversedJSON, err := json.Marshal(reversed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(forwardJSON, reversedJSON) {
		t.Fatalf("source pack depends on input order:\nforward: %s\nreverse: %s", forwardJSON, reversedJSON)
	}
	if bytes.Contains(forwardJSON, []byte("private-service-fact-id")) ||
		bytes.Contains(forwardJSON, []byte("selectedSourceFactIDs")) ||
		bytes.Contains(forwardJSON, []byte("selected_source_fact_ids")) {
		t.Fatalf("private selected facts leaked into JSON: %s", forwardJSON)
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

func writeSourceBackedContextFixture(t *testing.T, reverse bool) string {
	t.Helper()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-19T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "route", Project: "regulation", Kind: "route",
				Name:      "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
				Qualified: "CadasterController.deleteFromCadaster", HTTPMethod: "DELETE",
				Path: "/cadasters/{cadasterId}/regulations/{objectId}",
				File: "Controller.java", Line: 2, EndLine: 4, Confidence: "EXACT",
			},
			{
				ID: "private-service-fact-id", Project: "regulation", Kind: "symbol",
				Name: "removeRegulation", Qualified: "CadasterService.removeRegulation",
				File: "Service.java", Line: 2, EndLine: 4, Confidence: "EXACT",
			},
			{
				ID: "repository", Project: "regulation", Kind: "persistence",
				Name: "deleteRegulation", Qualified: "RegulationRepository.deleteRegulation",
				File: "Repository.java", Line: 2, EndLine: 4, Confidence: "EXACT",
			},
			{
				ID: "test", Project: "regulation", Kind: "test",
				Name: "deletesRegulation", Qualified: "CadasterControllerTest.deletesRegulation",
				File: "ControllerTest.java", Line: 2, EndLine: 4, Confidence: "EXACT",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "call", FromFactID: "route", ToFactID: "private-service-fact-id", FromLabel: "deleteFromCadaster", ToLabel: "removeRegulation", Kind: "call", Confidence: "EXACT"},
			{ID: "persistence", FromFactID: "private-service-fact-id", ToFactID: "repository", FromLabel: "removeRegulation", ToLabel: "deleteRegulation", Kind: "persistence", Confidence: "EXACT"},
			{ID: "test-target", FromFactID: "test", ToFactID: "route", FromLabel: "deletesRegulation", ToLabel: "deleteFromCadaster", Kind: "test_target", Confidence: "EXACT"},
		},
	}
	if reverse {
		slices.Reverse(index.Facts)
		slices.Reverse(index.Edges)
	}
	root := writeContextIndexFixture(t, index)
	writeContextSourceFile(t, root, "Controller.java", "public class Controller {\n    public void deleteFromCadaster() {\n        service.removeRegulation();\n    }\n}\n")
	writeContextSourceFile(t, root, "Service.java", "public class Service {\n    public void removeRegulation() {\n        repository.deleteRegulation();\n    }\n}\n")
	writeContextSourceFile(t, root, "Repository.java", "public class Repository {\n    public void deleteRegulation() {\n        records.remove();\n    }\n}\n")
	writeContextSourceFile(t, root, "ControllerTest.java", "public class ControllerTest {\n    public void deletesRegulation() {\n        controller.deleteFromCadaster();\n    }\n}\n")
	return root
}

func writeContextSourceFile(t *testing.T, root, relativePath, content string) {
	t.Helper()
	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
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

func contextPackHasUncertainty(pack ContextPack, scope, reason string) bool {
	for _, uncertainty := range pack.Uncertainties {
		if uncertainty.Scope == scope && strings.Contains(uncertainty.Reason, reason) {
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
