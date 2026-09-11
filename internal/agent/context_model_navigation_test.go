package agent

import (
	"reflect"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func intrinsicModelNavigationFixture() (ContextPack, scan.AgentContextIndexRecord) {
	pack, index := testInventorySideEffectFixture()
	pack.Contracts = nil
	pack.selectedSourceFactIDs = nil
	pack.SourceSections = []ContextSourceSection{{Project: "services/jobs", Path: "src/main/java/example/ShipmentTaskEntity.java", StartLine: 8, EndLine: 10, SourceState: "indexed_range_current", Content: "8\tpackage example;\n9\t@Entity\n10\tpublic class ShipmentTaskEntity {"}}
	return pack, index
}

func TestAdaptivePlanInventoryIntrinsicModelDeclaration(t *testing.T) {
	for _, mode := range []string{"current", "abstract", "qualified", "record", "generic", "method", "field", "constructor", "helper-method", "wrong-name", "later-declaration", "comment", "block-comment", "string", "nonexact", "stale", "relocated", "missing-body", "omitted", "other-project", "other-path", "outside-range", "zero-line", "invalid-range", "test-source", "configuration", "helper-class", "wrong-kind", "explicit-scope", "ambiguous-entrypoint", "strict"} {
		t.Run(mode, func(t *testing.T) {
			pack, index := intrinsicModelNavigationFixture()
			section := &pack.SourceSections[0]
			var fact *scan.AgentContextFactRecord
			for i := range index.Facts {
				if index.Facts[i].ID == "selected-model" {
					fact = &index.Facts[i]
					break
				}
			}
			fact.Kind = "symbol"
			switch mode {
			case "abstract":
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tpublic abstract class ShipmentTaskEntity {"
			case "qualified":
				fact.Name = "example.ShipmentTaskEntity"
				fact.Qualified = fact.Name
			case "record":
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tpublic record ShipmentTaskEntity(String id) {"
			case "generic":
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tpublic class ShipmentTaskEntity<T> {"
			case "method":
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tpublic ShipmentTaskEntity find() {"
			case "field":
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tprivate ShipmentTaskEntity entity;"
			case "constructor":
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tpublic ShipmentTaskEntity() {"
			case "helper-method":
				fact.Name = "createShipmentTaskEntity"
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tpublic Object createShipmentTaskEntity() {"
			case "wrong-name":
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tpublic class AnotherEntity {"
			case "later-declaration":
				fact.Line = 9
			case "comment":
				section.Content = "8\tpackage example;\n9\t@Entity\n10\t// public class ShipmentTaskEntity {"
			case "block-comment":
				section.Content = "8\t/*\n9\tcomment\n10\tpublic class ShipmentTaskEntity {"
			case "string":
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tString value = \"public class ShipmentTaskEntity {\";"
			case "nonexact":
				fact.Confidence = "RESOLVED"
			case "stale":
				section.SourceState = "stale"
			case "relocated":
				section.SourceState = "relocated"
			case "missing-body":
				section.Content = ""
			case "omitted":
				pack.SourceOmissions = []ContextSourceOmission{{Project: section.Project, Path: section.Path, StartLine: 8, EndLine: 10}}
				pack.SourceSections = nil
			case "other-project":
				section.Project = "services/archive"
			case "other-path":
				section.Path = "src/main/java/example/OtherEntity.java"
			case "outside-range":
				section.EndLine = 9
			case "zero-line":
				fact.Line = 0
			case "invalid-range":
				section.StartLine = 11
			case "test-source":
				fact.File = "src/test/java/example/ShipmentTaskEntity.java"
				section.Path = fact.File
			case "configuration":
				fact.File = "src/main/java/example/config/ShipmentTaskEntity.java"
				section.Path = fact.File
			case "helper-class":
				fact.Name = "PlainHelper"
				section.Content = "8\tpackage example;\n9\t@Entity\n10\tpublic class PlainHelper {"
			case "wrong-kind":
				fact.Kind = "method"
			case "explicit-scope":
				pack.Query = "For services/catalog, prepare a correction plan listing exact production and test files."
				pack.selectionQuery = pack.Query
			case "ambiguous-entrypoint":
				pack.Entrypoints = append(pack.Entrypoints, ContextLocation{Project: "services/archive", Confidence: "EXACT"})
			case "strict":
				pack.ProtocolVersion = StrictV1
			}
			before := cloneContextPack(pack)
			tests := contextPlanFiles(pack, index)
			production := contextProductionPlanFiles(pack, index)
			positive := mode == "current" || mode == "abstract" || mode == "qualified" || mode == "record" || mode == "generic"
			if positive {
				if len(production) != 1 || production[0].ProviderContract != "src/main/java/example/JobManagementController.java" {
					t.Errorf("model lost contract navigation: %#v", production)
				}
				for _, path := range []string{"src/test/java/example/JobManagementControllerTest.java", "src/test/java/example/ShipmentTaskNotificationServiceTest.java", "src/test/java/example/ShipmentTaskNotificationTests.java"} {
					found := false
					for _, file := range tests {
						if file.Path == path {
							found = true
						}
					}
					if !found {
						t.Errorf("missing test %s: %#v", path, tests)
					}
				}
				if len(tests) > 4 {
					t.Fatalf("test inventory exceeded budget: %#v", tests)
				}
				for _, file := range tests {
					if file.Project != "services/jobs" || file.Path == "src/test/java/example/InvoiceNotificationServiceTest.java" {
						t.Errorf("unrelated test: %#v", file)
					}
				}
				for i, j := 0, len(index.Facts)-1; i < j; i, j = i+1, j-1 {
					index.Facts[i], index.Facts[j] = index.Facts[j], index.Facts[i]
				}
				if !reflect.DeepEqual(tests, contextPlanFiles(pack, index)) || !reflect.DeepEqual(production, contextProductionPlanFiles(pack, index)) {
					t.Fatal("index ordering changed navigation")
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
