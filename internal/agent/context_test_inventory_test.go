package agent

import (
	"reflect"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func testInventorySideEffectFixture() (ContextPack, scan.AgentContextIndexRecord) {
	pack := correctionPlanInventoryPack("Prepare a correction plan listing exact production and test files, side effects and regression tests.", AdaptiveV2)
	pack.selectedSourceFactIDs = []string{"selected-model"}
	index := correctionPlanInventoryIndex()
	fact := func(id, kind, name, path string) scan.AgentContextFactRecord {
		return scan.AgentContextFactRecord{ID: id, Project: "services/jobs", Kind: kind, Name: name, File: path, Line: 10, Confidence: "EXACT"}
	}
	index.Facts = append(index.Facts,
		fact("selected-model", "record", "ShipmentTaskEntity", "src/main/java/example/ShipmentTaskEntity.java"),
		fact("effect-class", "symbol", "ShipmentTaskNotificationService", "src/main/java/example/ShipmentTaskNotificationService.java"),
		fact("effect-operation", "side_effects", "sendNotification", "src/main/java/example/ShipmentTaskNotificationService.java"),
		fact("effect-unit", "symbol", "ShipmentTaskNotificationServiceTest", "src/test/java/example/ShipmentTaskNotificationServiceTest.java"),
		fact("effect-integration", "symbol", "ShipmentTaskNotificationTests", "src/test/java/example/ShipmentTaskNotificationTests.java"),
		fact("neighbor-class", "symbol", "InvoiceNotificationService", "src/main/java/example/InvoiceNotificationService.java"),
		fact("neighbor-effect", "side_effects", "sendNotification", "src/main/java/example/InvoiceNotificationService.java"),
		fact("neighbor-test", "symbol", "InvoiceNotificationServiceTest", "src/test/java/example/InvoiceNotificationServiceTest.java"),
	)
	return pack, index
}

func TestAdaptiveTestInventoryDistinctSideEffects(t *testing.T) {
	pack, index := testInventorySideEffectFixture()
	got := contextPlanFiles(pack, index)
	for _, path := range []string{"src/test/java/example/ShipmentTaskNotificationServiceTest.java", "src/test/java/example/ShipmentTaskNotificationTests.java"} {
		found := false
		for _, file := range got {
			if file.Path == path && file.Use == "side_effect_test" {
				found = true
			}
		}
		if !found {
			t.Errorf("missing distinct side-effect identity %s: %#v", path, got)
		}
	}
	for _, file := range got {
		if file.Path == "src/test/java/example/InvoiceNotificationServiceTest.java" {
			t.Errorf("unrelated neighbor: %#v", got)
		}
	}
}

func TestAdaptiveTestInventoryCurrentModelAuthorizesProvider(t *testing.T) {
	pack, index := testInventorySideEffectFixture()
	pack.Query = "Prepare a correction plan listing exact production and test files for the ShipmentTaskEntity model and side effects."
	pack.selectionQuery = pack.Query
	pack.Contracts = nil
	pack.selectedSourceFactIDs = nil
	pack.SourceSections = []ContextSourceSection{{Project: "services/jobs", Path: "src/main/java/example/ShipmentTaskEntity.java", StartLine: 8, EndLine: 12, SourceState: "indexed_range_current", Role: "domain_model"}}
	got := contextPlanFiles(pack, index)
	if len(got) == 0 {
		t.Fatal("current exact model declaration did not authorize provider navigation")
	}
	pack.SourceSections[0].SourceState = "stale"
	if got := contextPlanFiles(pack, index); len(got) != 0 {
		t.Fatalf("stale model authorized provider inventory: %#v", got)
	}
}

func TestAdaptiveTestInventorySourceAuthorityAndConfidence(t *testing.T) {
	for _, mode := range []string{"current", "stale", "stale-seed", "outside-range", "navigation", "low-model", "low-test", "explicit-scope", "strict"} {
		t.Run(mode, func(t *testing.T) {
			pack, index := testInventorySideEffectFixture()
			pack.selectedSourceFactIDs = nil
			section := ContextSourceSection{Project: "services/jobs", Path: "src/main/java/example/ShipmentTaskEntity.java", StartLine: 8, EndLine: 12, SourceState: "indexed_range_current"}
			pack.SourceSections = []ContextSourceSection{section}
			switch mode {
			case "stale":
				pack.SourceSections[0].SourceState = "stale"
			case "stale-seed":
				pack.selectedSourceFactIDs = []string{"selected-model"}
				pack.SourceSections[0].SourceState = "stale"
			case "outside-range":
				pack.SourceSections[0].StartLine = 11
			case "navigation":
				pack.SourceSections = nil
				pack.Files = []ContextFile{{Project: section.Project, Path: section.Path}}
			case "low-model":
				for i := range index.Facts {
					if index.Facts[i].ID == "selected-model" {
						index.Facts[i].Confidence = "RESOLVED"
					}
				}
			case "low-test":
				for i := range index.Facts {
					if index.Facts[i].ID == "effect-unit" || index.Facts[i].ID == "effect-integration" {
						index.Facts[i].Confidence = "RESOLVED"
					}
				}
			case "explicit-scope":
				pack.Query = "For services/catalog, prepare a correction plan listing production and test files."
				pack.selectionQuery = pack.Query
			case "strict":
				pack.ProtocolVersion = StrictV1
			}
			count := 0
			for _, file := range contextPlanFiles(pack, index) {
				if file.Use == "side_effect_test" {
					count++
				}
			}
			want := 0
			if mode == "current" {
				want = 2
			}
			if count != want {
				t.Fatalf("side effect count %d, want %d", count, want)
			}
		})
	}
}

func TestAdaptiveTestInventoryDeduplicatesBeforeBounding(t *testing.T) {
	pack, index := testInventorySideEffectFixture()
	pack.Files = []ContextFile{{Project: "services/jobs", Path: "src/test/java/example/JobManagementControllerTest.java"}}
	for _, name := range []string{"ShipmentTaskNotificationIntegrationTest", "ShipmentTaskNotificationFailureTest", "ShipmentTaskNotificationRegressionTest"} {
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{ID: name, Project: "services/jobs", Kind: "symbol", Name: name, File: "src/test/java/example/" + name + ".java", Line: 10, Confidence: "EXACT"})
	}
	got := contextPlanFiles(pack, index)
	if len(got) != maximumContextPlanFiles {
		t.Fatalf("inventory did not use deduplicated budget: %#v", got)
	}
	for i, j := 0, len(index.Facts)-1; i < j; i, j = i+1, j-1 {
		index.Facts[i], index.Facts[j] = index.Facts[j], index.Facts[i]
	}
	if again := contextPlanFiles(pack, index); !reflect.DeepEqual(got, again) {
		t.Fatalf("unstable selection: %#v versus %#v", got, again)
	}
}

func TestAdaptivePlanInventoryCurrentPublicEndpointNavigation(t *testing.T) {
	for _, mode := range []string{"current", "stale", "nonexact", "configuration", "test-source", "other-project", "outside-range", "invalid-range", "zero-line", "class-only", "missing-method", "missing-route", "explicit-scope", "ambiguous-entrypoint", "strict"} {
		t.Run(mode, func(t *testing.T) {
			pack, index := testInventorySideEffectFixture()
			pack.Contracts = nil
			pack.selectedSourceFactIDs = nil
			endpoint := scan.AgentContextFactRecord{ID: "public-endpoint", Project: "services/jobs", Kind: "api_endpoint", Name: "DELETE /shipments/{id}/tasks/{taskId}", Qualified: "ShipmentTaskController.deleteTask", HTTPMethod: "DELETE", Path: "/shipments/{id}/tasks/{taskId}", File: "src/main/java/example/ShipmentTaskController.java", Line: 42, Confidence: "EXACT"}
			section := ContextSourceSection{Project: endpoint.Project, Path: endpoint.File, StartLine: 40, EndLine: 48, SourceState: "indexed_range_current", Role: "call_chain"}
			switch mode {
			case "stale":
				section.SourceState = "stale"
			case "nonexact":
				endpoint.Confidence = "EXTRACTED"
			case "configuration":
				endpoint.File = "src/main/java/example/config/ShipmentTaskController.java"
				section.Path = endpoint.File
			case "test-source":
				endpoint.File = "src/test/java/example/ShipmentTaskControllerTest.java"
				section.Path = endpoint.File
			case "other-project":
				section.Project = "services/archive"
			case "outside-range":
				section.StartLine = 43
			case "invalid-range":
				section.EndLine = 39
			case "zero-line":
				endpoint.Line = 0
			case "class-only":
				endpoint.Kind = "symbol"
				endpoint.Name = "ShipmentTaskController"
			case "missing-method":
				endpoint.HTTPMethod = ""
			case "missing-route":
				endpoint.Path = ""
			case "explicit-scope":
				pack.Query = "For services/catalog, prepare a correction plan listing exact production and test files."
				pack.selectionQuery = pack.Query
			case "ambiguous-entrypoint":
				pack.Entrypoints = append(pack.Entrypoints, ContextLocation{Project: "services/archive", Confidence: "EXACT"})
			case "strict":
				pack.ProtocolVersion = StrictV1
			}
			index.Facts = append(index.Facts, endpoint)
			pack.SourceSections = []ContextSourceSection{section}
			before := pack
			tests := contextPlanFiles(pack, index)
			production := contextProductionPlanFiles(pack, index)
			if mode == "current" {
				if len(tests) == 0 {
					t.Fatal("current public endpoint lost test navigation")
				}
				if len(production) != 1 || production[0].Project != "services/jobs" || production[0].ProviderContract != "src/main/java/example/JobManagementController.java" {
					t.Fatalf("internal contract navigation = %#v", production)
				}
				for _, test := range tests {
					if test.Project != "services/jobs" {
						t.Fatalf("unrelated project: %#v", tests)
					}
				}
			} else if len(tests) != 0 || len(production) != 0 {
				t.Fatalf("%s authorized navigation: tests=%#v production=%#v", mode, tests, production)
			}
			if !reflect.DeepEqual(pack, before) {
				t.Fatal("navigation mutated pack evidence")
			}
		})
	}
}
