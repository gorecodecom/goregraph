package agent

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func configurationCompactionFixture() (ContextPack, scan.AgentContextIndexRecord) {
	pack, index := configurationIdentityFixture()
	pack.SourceSections[0].Role = "call_chain"
	pack.SourceSections[0].ReadReceipt = "r1:fixture:8-10"
	pack.SourceSections = append(pack.SourceSections, ContextSourceSection{Project: "services/catalog", Path: "src/main/java/example/Configuration.java", StartLine: 12, EndLine: 14, Role: "call_chain", RenderMode: "full", SourceState: "indexed_range_current", ReadReceipt: "r1:other:12-14", Content: "12\tpublic class Configuration {\n13\t int port;\n14\t}"})
	for _, section := range pack.SourceSections {
		pack.Files = append(pack.Files, ContextFile{Project: section.Project, Path: section.Path, StartLine: section.StartLine, EndLine: section.EndLine, Role: section.Role})
	}
	pack.Files = append(pack.Files, ContextFile{Project: "services/jobs", Path: "src/main/java/example/UnreturnedController.java", StartLine: 20, EndLine: 30, Role: "call_chain"})
	pack.ConfigurationResources = contextConfigurationResources(pack, index)
	pack.SourceUnrepresented = 3
	pack.SourceCoverage = "partial"
	pack.SourceOmissions = []ContextSourceOmission{{Project: "services/jobs", Path: "src/main/java/example/UnreturnedEntity.java", StartLine: 40, EndLine: 50, Role: "domain_model", Reason: "source section does not fit the response budget"}}
	pack.VerificationRequests = []ContextVerificationRequest{{Project: "services/jobs", Path: "src/main/java/example/UnreturnedEntity.java", StartLine: 40, EndLine: 50, Reason: "source section does not fit the response budget"}}
	pack.selectedSourceFactIDs = []string{"selected-model"}
	pack.selectedFactIDs = []string{"selected-model", "catalog-route"}
	pack.selectedEdgeIDs = []string{"selected-edge"}
	pack.RetryAllowed = true
	pack.RetryAnchors = []string{"selected-model"}
	return pack, index
}

func TestInferredConfigurationCompactsOnlyRepeatedSourceNavigation(t *testing.T) {
	for _, mode := range []string{"all-resources", "partial-resources", "relocated-current", "canonical-path"} {
		t.Run(mode, func(t *testing.T) {
			pack, index := configurationCompactionFixture()
			if mode == "relocated-current" {
				pack.SourceSections[0].SourceState = "relocated_current"
			}
			if mode == "canonical-path" {
				pack.Files[0].Path = "./" + pack.Files[0].Path
			}
			expected := cloneContextPack(pack)
			expected.Files = expected.Files[2:]
			if mode == "partial-resources" {
				expected.ConfigurationResources[1].Resources = expected.ConfigurationResources[1].Resources[:1]
			}
			// Budget the hand-selected expected output; it retains every source/proof field.
			expected.BudgetTokens = 2000
			estimate, err := finalizeContextEstimate(expected)
			if err != nil {
				t.Fatal(err)
			}
			pack.BudgetTokens = estimate.EstimatedTokens
			before := cloneContextPack(pack)
			got, handled, err := fitInferredConfigurationNavigationWithinBudget(pack, index, ContextRequest{BudgetTokens: pack.BudgetTokens, MaxFiles: 12})
			if err != nil {
				t.Fatal(err)
			}
			if !handled || !reflect.DeepEqual(got.Files, expected.Files) || !reflect.DeepEqual(got.ConfigurationResources, expected.ConfigurationResources) {
				t.Fatalf("%s did not retain identities through duplicate compaction: handled=%t files=%#v resources=%#v", mode, handled, got.Files, got.ConfigurationResources)
			}
			if got.EstimatedTokens > pack.BudgetTokens {
				t.Fatalf("budget exceeded: %d/%d", got.EstimatedTokens, pack.BudgetTokens)
			}
			// Apart from repeated navigation and budgeted resources, all output state is preserved.
			preserved := cloneContextPack(got)
			preserved.Files = before.Files
			preserved.ConfigurationResources = before.ConfigurationResources
			preserved.EstimatedTokens = before.EstimatedTokens
			if !reflect.DeepEqual(preserved, before) || !reflect.DeepEqual(pack, before) {
				t.Fatal("compaction changed source/receipt/proof state or mutated input")
			}
		})
	}
}

func TestInferredConfigurationCompactionPreservesDistinctMetadata(t *testing.T) {
	for _, mode := range []string{"role", "range", "reason", "confidence", "project", "path", "stale", "no-content", "no-source", "ordinary", "strict", "explicit-scope", "already-fits", "cannot-fit"} {
		t.Run(mode, func(t *testing.T) {
			pack, index := configurationCompactionFixture()
			// Keep one candidate to make each negative independently decisive.
			pack.Files = pack.Files[:1]
			pack.SourceSections = pack.SourceSections[:1]
			switch mode {
			case "role":
				pack.Files[0].Role = "domain_model"
			case "range":
				pack.Files[0].StartLine--
			case "reason":
				pack.Files[0].Reason = "selected exact model"
			case "confidence":
				pack.Files[0].Confidence = "EXACT"
			case "project":
				pack.Files[0].Project = "services/archive"
			case "path":
				pack.Files[0].Path = "src/main/java/example/OtherEntity.java"
			case "stale":
				pack.SourceSections[0].SourceState = "stale"
			case "no-content":
				pack.SourceSections[0].Content = ""
			case "no-source":
				pack.SourceSections = nil
			case "ordinary":
				pack.Query = "Show exact configuration files for authentication."
				pack.selectionQuery = pack.Query
			case "strict":
				pack.ProtocolVersion = StrictV1
			case "explicit-scope":
				pack.Query += " Limit to services/jobs."
				pack.selectionQuery = pack.Query
			case "cannot-fit":
				pack.SourceSections[0].Content = strings.Repeat("x", 10000)
			}
			estimate, err := finalizeContextEstimate(pack)
			if err != nil {
				t.Fatal(err)
			}
			pack.BudgetTokens = max(MinContextBudgetTokens, estimate.EstimatedTokens-25)
			if mode == "already-fits" {
				pack.BudgetTokens = estimate.EstimatedTokens + 100
			}
			if mode == "cannot-fit" {
				pack.BudgetTokens = MinContextBudgetTokens
			}
			before := cloneContextPack(pack)
			got, handled, err := fitInferredConfigurationNavigationWithinBudget(pack, index, ContextRequest{BudgetTokens: pack.BudgetTokens, MaxFiles: 12})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Files, before.Files) {
				t.Fatalf("%s lost nonduplicate or out-of-scope navigation: %#v", mode, got.Files)
			}
			preserved := cloneContextPack(got)
			preserved.ConfigurationResources = before.ConfigurationResources
			preserved.EstimatedTokens = before.EstimatedTokens
			if !reflect.DeepEqual(preserved, before) || !reflect.DeepEqual(pack, before) {
				t.Fatalf("%s mutated source/receipt/proof state or input", mode)
			}
			if mode == "cannot-fit" && (handled || !reflect.DeepEqual(got, before)) {
				t.Fatal("unsolved fit did not return original pack")
			}
		})
	}
}
