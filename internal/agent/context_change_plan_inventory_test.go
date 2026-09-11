package agent

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveCorrectionPlanEvidenceIntentDoesNotInventMissingTransition(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantRetry bool
	}{
		{
			name:      "German correction plan",
			query:     "Erstelle einen Korrekturplan mit Produktions-, Konfigurations- und Testdateien für interne Schnittstellen und Wiederholungsverhalten.",
			wantRetry: true,
		},
		{
			name:  "English fix plan",
			query: "Prepare a fix plan listing production, configuration, and test files for internal interfaces.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidenceQuery := contextEvidenceQueryForProtocol(test.query, AdaptiveV2)
			if !contextQueryRequestsExactEvidenceInventory(evidenceQuery) {
				t.Fatalf("adaptive evidence query did not request an exact inventory: %q", evidenceQuery)
			}
			if contextQueryPlansMissingTransition(test.query) || contextQueryPlansMissingTransition(evidenceQuery) {
				t.Fatalf("correction plan invented a missing transition: raw=%q evidence=%q", test.query, evidenceQuery)
			}
			if strings.Contains(strings.ToLower(evidenceQuery), "new call chain") ||
				strings.Contains(strings.ToLower(evidenceQuery), "missing") {
				t.Fatalf("correction plan gained proof vocabulary: %q", evidenceQuery)
			}
			if test.wantRetry && !contextQueryRequestsConcern(evidenceQuery, contextConcernResilience) {
				t.Fatalf("Wiederholungsverhalten did not retain resilience intent: %q", evidenceQuery)
			}
			if got := contextEvidenceQueryForProtocol(test.query, StrictV1); got != test.query {
				t.Fatalf("strict query changed: got %q, want %q", got, test.query)
			}
		})
	}
}

func TestCorrectionPlanProviderTestsPrioritizeSelectedDeletionAction(t *testing.T) {
	const correctionQuery = "Prepare a fix plan listing exact production and test files for services/jobs and PaymentServiceRetryableTest retry behavior."
	const ordinaryQuery = "Explain services/jobs and PaymentServiceRetryableTest retry behavior and test files."
	const legacyQuery = "Plan the missing internal HTTP contract and exact production and test files to change for services/jobs and PaymentServiceRetryableTest retry behavior."
	tests := []struct {
		name          string
		query         string
		protocol      string
		methods       []string
		wantService   string
		wantInventory bool
	}{
		{name: "adaptive DELETE correction", query: correctionQuery, protocol: AdaptiveV2, methods: []string{"DELETE"}, wantService: "JobCleanupServiceTest", wantInventory: true},
		{name: "strict correction", query: correctionQuery, protocol: StrictV1, methods: []string{"DELETE"}, wantService: "PaymentServiceRetryableTest"},
		{name: "adaptive GET correction", query: correctionQuery, protocol: AdaptiveV2, methods: []string{"GET"}, wantService: "PaymentServiceRetryableTest", wantInventory: true},
		{name: "adaptive absent endpoint", query: correctionQuery, protocol: AdaptiveV2, wantService: "PaymentServiceRetryableTest", wantInventory: true},
		{name: "adaptive multiple DELETE endpoints", query: correctionQuery, protocol: AdaptiveV2, methods: []string{"DELETE", "DELETE"}, wantService: "PaymentServiceRetryableTest", wantInventory: true},
		{name: "adaptive mixed endpoints", query: correctionQuery, protocol: AdaptiveV2, methods: []string{"DELETE", "GET"}, wantService: "PaymentServiceRetryableTest", wantInventory: true},
		{name: "adaptive ordinary query", query: ordinaryQuery, protocol: AdaptiveV2, methods: []string{"DELETE"}, wantService: "PaymentServiceRetryableTest"},
		{name: "legacy strict missing transition without endpoint", query: legacyQuery, protocol: StrictV1, wantService: "JobCleanupServiceTest", wantInventory: true},
		{name: "legacy adaptive missing transition with GET", query: legacyQuery, protocol: AdaptiveV2, methods: []string{"GET"}, wantService: "JobCleanupServiceTest", wantInventory: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pack := correctionPlanInventoryPack(test.query, test.protocol)
			pack.Concerns = []ContextConcern{{Kind: contextConcernTests, Project: "services/jobs"}}
			for _, method := range test.methods {
				pack.Endpoints = append(pack.Endpoints, ContextEndpoint{Provider: "services/catalog", HTTPMethod: method})
			}
			index := correctionPlanDeletionTestIndex(test.query)
			want := []ContextPlanFile{
				{Project: "services/jobs", Path: "src/test/java/example/JobManagementControllerTest.java", Use: "provider_test"},
				{Project: "services/jobs", Path: "src/test/java/example/" + test.wantService + ".java", Use: "provider_test"},
			}
			for attempt := 0; attempt < 2; attempt++ {
				selected := contextPlanFileProviderTests(pack, index, index.Facts, map[string]bool{"services/jobs": true})
				if len(selected) != 2 || selected[1].fact.Name != test.wantService {
					t.Fatalf("provider test selection = %#v, want service %s", selected, test.wantService)
				}
				got := contextPlanFiles(pack, index)
				if test.wantInventory && !reflect.DeepEqual(got, want) {
					t.Fatalf("plan files = %#v, want %#v", got, want)
				}
				if !test.wantInventory && len(got) != 0 {
					t.Fatalf("ineligible query gained plan files: %#v", got)
				}
				for left, right := 0, len(index.Facts)-1; left < right; left, right = left+1, right-1 {
					index.Facts[left], index.Facts[right] = index.Facts[right], index.Facts[left]
				}
			}
		})
	}
}

func TestAdaptiveCorrectionPlanDeletionTestDeduplicatesRepresentedPath(t *testing.T) {
	const query = "Prepare a correction plan listing exact production and test files for services/jobs and PaymentServiceRetryableTest retry behavior."
	pack := correctionPlanInventoryPack(query, AdaptiveV2)
	pack.Endpoints = []ContextEndpoint{{Provider: "services/catalog", HTTPMethod: "DELETE"}}
	pack.SourceSections = []ContextSourceSection{{
		Project: "services/jobs", Path: "src/test/java/example/PaymentServiceRetryableTest.java", StartLine: 10, EndLine: 20,
	}}
	index := correctionPlanDeletionTestIndex(query)
	want := []ContextPlanFile{
		{Project: "services/jobs", Path: "src/test/java/example/JobManagementControllerTest.java", Use: "provider_test"},
		{Project: "services/jobs", Path: "src/test/java/example/JobCleanupServiceTest.java", Use: "provider_test"},
	}
	if got := contextPlanFiles(pack, index); !reflect.DeepEqual(got, want) {
		t.Fatalf("plan files with represented retry reference = %#v, want %#v", got, want)
	}
	pack.Files = []ContextFile{{Project: "services/jobs", Path: "src/test/java/example/JobCleanupServiceTest.java"}}
	if got := contextPlanFiles(pack, index); !reflect.DeepEqual(got, want[:1]) {
		t.Fatalf("plan files with represented cleanup = %#v, want %#v", got, want[:1])
	}
	if !contextPackRepresentsSourcePath(pack, "services/jobs", "src/test/java/example/PaymentServiceRetryableTest.java") {
		t.Fatal("represented retry reference was lost")
	}
}

func correctionPlanDeletionTestIndex(query string) scan.AgentContextIndexRecord {
	index := correctionPlanInventoryIndex()
	index.Facts = append(index.Facts,
		scan.AgentContextFactRecord{ID: "jobs-cleanup-test", Project: "services/jobs", Kind: "symbol", Name: "JobCleanupServiceTest", File: "src/test/java/example/JobCleanupServiceTest.java", Line: 30, Confidence: "EXACT", Search: "job cleanup persistence service test"},
		scan.AgentContextFactRecord{ID: "jobs-payment-retry-test", Project: "services/jobs", Kind: "symbol", Name: "PaymentServiceRetryableTest", File: "src/test/java/example/PaymentServiceRetryableTest.java", Line: 40, Confidence: "EXACT", Search: query},
	)
	return index
}

func TestAdaptiveCorrectionPlanInventoriesUseSelectedExactProviderRoles(t *testing.T) {
	index := correctionPlanInventoryIndex()
	wantTests := []ContextPlanFile{{
		Project: "services/jobs", Path: "src/test/java/example/JobManagementControllerTest.java", Use: "provider_test",
	}}
	wantProduction := []ContextProductionPlanFiles{{
		Project: "services/jobs", ProviderContract: "src/main/java/example/JobManagementController.java",
	}}
	queries := []string{
		"Erstelle einen Korrekturplan mit Produktions-, Konfigurations- und Testdateien für interne Schnittstellen und Wiederholungsverhalten.",
		"Prepare a fix plan listing production, configuration, and test files for internal interfaces and retry behavior.",
	}
	for _, query := range queries {
		pack := correctionPlanInventoryPack(query, AdaptiveV2)
		if got := contextPlanFiles(pack, index); !reflect.DeepEqual(got, wantTests) {
			t.Errorf("test inventory for %q = %#v, want %#v", query, got, wantTests)
		}
		if got := contextProductionPlanFiles(pack, index); !reflect.DeepEqual(got, wantProduction) {
			t.Errorf("production inventory for %q = %#v, want %#v", query, got, wantProduction)
		}
	}

	modelQuery := "Prepare a fix plan listing production, configuration, and test files for the CleanupRecord model and internal interfaces."
	modelPack := correctionPlanInventoryPack(modelQuery, AdaptiveV2)
	modelPack.Contracts = nil
	modelPack.selectedSourceFactIDs = []string{"jobs-model"}
	if got := contextPlanFiles(modelPack, index); !reflect.DeepEqual(got, wantTests) {
		t.Errorf("selected-model test inventory = %#v, want %#v", got, wantTests)
	}
	modelWantProduction := []ContextProductionPlanFiles{{
		Project: "services/jobs", ProviderContract: "src/main/java/example/JobManagementController.java",
		PrimaryPersistence: []string{"src/main/java/example/CleanupRecordRepository.java"},
	}}
	if got := contextProductionPlanFiles(modelPack, index); !reflect.DeepEqual(got, modelWantProduction) {
		t.Errorf("selected-model production inventory = %#v, want %#v", got, modelWantProduction)
	}
	for _, directory := range []string{"config", "configuration"} {
		t.Run("selected model in "+directory, func(t *testing.T) {
			configurationIndex := correctionPlanInventoryIndex()
			for factIndex := range configurationIndex.Facts {
				if configurationIndex.Facts[factIndex].ID == "jobs-model" {
					configurationIndex.Facts[factIndex].File = "src/main/java/example/" + directory + "/CleanupRecord.java"
				}
			}
			if got := contextPlanFiles(modelPack, configurationIndex); len(got) != 0 {
				t.Errorf("selected configuration model authorized test inventory: %#v", got)
			}
			if got := contextProductionPlanFiles(modelPack, configurationIndex); len(got) != 0 {
				t.Errorf("selected configuration model authorized production inventory: %#v", got)
			}
		})
	}
}

func TestAdaptiveCorrectionPlanInventoriesUseSelectedExactManagementEndpoint(t *testing.T) {
	const query = "DELETE /catalog/items/{itemId}: For services/jobs, prepare a fix plan listing production, configuration, and test files for the CleanupRecord model and internal interfaces."
	index := correctionPlanInventoryIndex()
	for factIndex := range index.Facts {
		if index.Facts[factIndex].ID == "jobs-model" {
			index.Facts[factIndex].Confidence = ""
		}
	}
	pack := correctionPlanInventoryPack(query, AdaptiveV2)
	pack.Contracts = nil
	pack.selectedSourceFactIDs = []string{"jobs-management-endpoint", "jobs-model"}
	wantTests := []ContextPlanFile{{
		Project: "services/jobs", Path: "src/test/java/example/JobManagementControllerTest.java", Use: "provider_test",
	}}
	wantProduction := []ContextProductionPlanFiles{{
		Project: "services/jobs", ProviderContract: "src/main/java/example/JobManagementController.java",
		PrimaryPersistence: []string{"src/main/java/example/CleanupRecordRepository.java"},
	}}
	if got := contextPlanFiles(pack, index); !reflect.DeepEqual(got, wantTests) {
		t.Errorf("selected exact management endpoint test inventory = %#v, want %#v", got, wantTests)
	}
	if got := contextProductionPlanFiles(pack, index); !reflect.DeepEqual(got, wantProduction) {
		t.Errorf("selected exact management endpoint production inventory = %#v, want %#v", got, wantProduction)
	}
}

func TestAdaptiveCorrectionPlanInventoriesUseExactContractInCurrentSourceSection(t *testing.T) {
	const query = "DELETE /catalog/items/{itemId}: For services/jobs, prepare a fix plan listing production, configuration, and test files for the CleanupRecord model and internal interfaces."
	index := correctionPlanInventoryIndex()
	pack := correctionPlanInventoryPack(query, AdaptiveV2)
	pack.Contracts = nil
	pack.selectedSourceFactIDs = nil
	pack.SourceSections = []ContextSourceSection{{
		Project: "services/jobs", Path: "src/main/java/example/JobManagementController.java",
		StartLine: 115, EndLine: 146, Role: "call_chain",
		RenderMode: "declaration_body", SourceState: "indexed_range_current",
	}}

	wantTests := []ContextPlanFile{{
		Project: "services/jobs", Path: "src/test/java/example/JobManagementControllerTest.java", Use: "provider_test",
	}}
	if got := contextPlanFiles(pack, index); !reflect.DeepEqual(got, wantTests) {
		t.Errorf("current exact contract source test inventory = %#v, want %#v", got, wantTests)
	}
	if got := contextProductionPlanProviderProjects(pack, index, "services/catalog"); !reflect.DeepEqual(got, map[string]bool{"services/jobs": true}) {
		t.Fatalf("current exact contract source provider projects = %#v", got)
	}
	wantProduction := []ContextProductionPlanFiles{{
		Project: "services/jobs", PrimaryPersistence: []string{"src/main/java/example/CleanupRecordRepository.java"},
	}}
	if got := contextProductionPlanFiles(pack, index); !reflect.DeepEqual(got, wantProduction) {
		t.Errorf("source-selected production inventory = %#v, want %#v", got, wantProduction)
	}
	if !contextPackRepresentsSourcePath(pack, "services/jobs", "src/main/java/example/JobManagementController.java") {
		t.Fatal("provider contract is absent from source and production metadata union")
	}
}

func TestAdaptiveCorrectionPlanSourceSectionFallbackRequiresExactSelectedRange(t *testing.T) {
	const query = "DELETE /catalog/items/{itemId}: For services/jobs, prepare a fix plan listing production, configuration, and test files for the CleanupRecord model and internal interfaces."
	tests := []struct {
		name    string
		section *ContextSourceSection
	}{
		{
			name: "wrong project",
			section: &ContextSourceSection{Project: "services/archive", Path: "src/main/java/example/JobManagementController.java",
				StartLine: 115, EndLine: 146, SourceState: "indexed_range_current"},
		},
		{
			name: "wrong path",
			section: &ContextSourceSection{Project: "services/jobs", Path: "src/main/java/example/OtherManagementController.java",
				StartLine: 115, EndLine: 146, SourceState: "indexed_range_current"},
		},
		{
			name: "outside returned range",
			section: &ContextSourceSection{Project: "services/jobs", Path: "src/main/java/example/JobManagementController.java",
				StartLine: 122, EndLine: 146, SourceState: "indexed_range_current"},
		},
		{
			name: "stale source section",
			section: &ContextSourceSection{Project: "services/jobs", Path: "src/main/java/example/JobManagementController.java",
				StartLine: 115, EndLine: 146, SourceState: "current_source_changed_since_index"},
		},
		{
			name: "configuration source",
			section: &ContextSourceSection{Project: "services/jobs", Path: "src/main/java/example/config/JobManagementController.java",
				StartLine: 115, EndLine: 146, SourceState: "indexed_range_current"},
		},
		{name: "unselected fact"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pack := correctionPlanInventoryPack(query, AdaptiveV2)
			pack.Contracts = nil
			pack.selectedSourceFactIDs = nil
			if test.section != nil {
				pack.SourceSections = []ContextSourceSection{*test.section}
			}
			if got := contextPlanFiles(pack, correctionPlanInventoryIndex()); len(got) != 0 {
				t.Errorf("%s authorized test provider inventory: %#v", test.name, got)
			}
			if got := contextProductionPlanProviderProjects(pack, correctionPlanInventoryIndex(), "services/catalog"); len(got) != 0 {
				t.Errorf("%s authorized production provider project: %#v", test.name, got)
			}
			if got := contextProductionPlanFiles(pack, correctionPlanInventoryIndex()); len(got) != 0 {
				t.Errorf("%s authorized production provider inventory: %#v", test.name, got)
			}
		})
	}
}

func TestAdaptiveCorrectionPlanManagementEndpointFallbackRequiresSelectedExactContractFact(t *testing.T) {
	const providerQuery = "DELETE /catalog/items/{itemId}: For services/jobs, prepare a fix plan listing production, configuration, and test files for the CleanupRecord model and internal interfaces."
	tests := []struct {
		name               string
		query              string
		selectedIDs        []string
		endpointConfidence string
	}{
		{name: "partial endpoint", query: providerQuery, selectedIDs: []string{"jobs-management-endpoint", "jobs-model"}, endpointConfidence: "PARTIAL"},
		{name: "unselected exact endpoint", query: providerQuery, selectedIDs: []string{"jobs-model"}, endpointConfidence: "EXACT"},
		{name: "selected noncontract helper", query: providerQuery, selectedIDs: []string{"jobs-helper", "jobs-model"}, endpointConfidence: "EXACT"},
		{name: "selected management test", query: providerQuery, selectedIDs: []string{"jobs-management-test", "jobs-model"}, endpointConfidence: "EXACT"},
		{name: "selected management config", query: providerQuery, selectedIDs: []string{"jobs-management-config", "jobs-model"}, endpointConfidence: "EXACT"},
		{name: "selected invalid-path endpoint", query: providerQuery, selectedIDs: []string{"jobs-management-invalid", "jobs-model"}, endpointConfidence: "EXACT"},
		{
			name:        "caller-only explicit scope",
			query:       "DELETE /catalog/items/{itemId}: For services/catalog, prepare a fix plan listing production, configuration, and test files for internal interfaces.",
			selectedIDs: []string{"jobs-management-endpoint"}, endpointConfidence: "EXACT",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := correctionPlanInventoryIndex()
			for factIndex := range index.Facts {
				switch index.Facts[factIndex].ID {
				case "jobs-model":
					index.Facts[factIndex].Confidence = ""
				case "jobs-management-endpoint":
					index.Facts[factIndex].Confidence = test.endpointConfidence
				}
			}
			pack := correctionPlanInventoryPack(test.query, AdaptiveV2)
			pack.Contracts = nil
			pack.selectedSourceFactIDs = test.selectedIDs
			if got := contextPlanFiles(pack, index); len(got) != 0 {
				t.Errorf("%s authorized test provider inventory: %#v", test.name, got)
			}
			if got := contextProductionPlanFiles(pack, index); len(got) != 0 {
				t.Errorf("%s authorized production provider inventory: %#v", test.name, got)
			}
		})
	}
}

func TestContextProductionPlanFilesConfigurationExclusionPreservesStrict(t *testing.T) {
	const contractPath = "src/main/java/example/config/JobManagementController.java"
	for _, protocol := range []string{StrictV1, AdaptiveV2} {
		t.Run(protocol, func(t *testing.T) {
			pack, index := productionPlanFilesFixture()
			pack.ProtocolVersion = protocol
			pack.Query = "Plan the missing internal HTTP contract and identify the exact production and test files to change."
			pack.selectionQuery = pack.Query
			for i := range index.Facts {
				if index.Facts[i].ID == "jobs-route" || index.Facts[i].ID == "jobs-controller" {
					index.Facts[i].File = contractPath
				}
			}
			var want []ContextProductionPlanFiles
			if protocol == StrictV1 {
				want = []ContextProductionPlanFiles{{Project: "services/jobs", ProviderContract: contractPath}}
			}
			got := contextProductionPlanFiles(pack, index)
			if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got, want)) {
				t.Fatalf("%s configuration-path provider inventory = %#v, want %#v", protocol, got, want)
			}
		})
	}
}

func TestCorrectionPlanInventoriesKeepStrictScopeAndAmbiguityBoundaries(t *testing.T) {
	index := correctionPlanInventoryIndex()
	query := "Prepare a fix plan listing production, configuration, and test files for internal interfaces."

	strict := correctionPlanInventoryPack(query, StrictV1)
	if got := contextPlanFiles(strict, index); len(got) != 0 {
		t.Fatalf("strict test inventory = %#v, want none", got)
	}
	if got := contextProductionPlanFiles(strict, index); len(got) != 0 {
		t.Fatalf("strict production inventory = %#v, want none", got)
	}

	ordinary := correctionPlanInventoryPack("Diagnose the current internal interface and tests.", AdaptiveV2)
	if got := contextPlanFiles(ordinary, index); len(got) != 0 {
		t.Fatalf("ordinary diagnosis test inventory = %#v, want none", got)
	}
	if got := contextProductionPlanFiles(ordinary, index); len(got) != 0 {
		t.Fatalf("ordinary diagnosis production inventory = %#v, want none", got)
	}

	productionOnly := correctionPlanInventoryPack(
		"Prepare a fix plan listing exact production files for internal interfaces.",
		AdaptiveV2,
	)
	if got := contextPlanFiles(productionOnly, index); len(got) != 0 {
		t.Fatalf("correction plan without requested tests gained test inventory: %#v", got)
	}
	wantProductionOnly := []ContextProductionPlanFiles{{
		Project: "services/jobs", ProviderContract: "src/main/java/example/JobManagementController.java",
	}}
	if got := contextProductionPlanFiles(productionOnly, index); !reflect.DeepEqual(got, wantProductionOnly) {
		t.Fatalf("production-only correction plan inventory = %#v, want %#v", got, wantProductionOnly)
	}

	callerOnly := correctionPlanInventoryPack(
		"For services/catalog, prepare a fix plan listing production, configuration, and test files for internal interfaces.",
		AdaptiveV2,
	)
	if got := contextPlanFiles(callerOnly, index); len(got) != 0 {
		t.Fatalf("caller-only scope test inventory = %#v, want none", got)
	}
	if got := contextProductionPlanFiles(callerOnly, index); len(got) != 0 {
		t.Fatalf("caller-only scope production inventory = %#v, want none", got)
	}

	unrelated := correctionPlanInventoryPack(query, AdaptiveV2)
	unrelated.Contracts = nil
	unrelated.selectedSourceFactIDs = []string{"archive-helper"}
	if got := contextPlanFiles(unrelated, index); len(got) != 0 {
		t.Fatalf("unrelated selected project test inventory = %#v, want none", got)
	}
	if got := contextProductionPlanFiles(unrelated, index); len(got) != 0 {
		t.Fatalf("unrelated selected project production inventory = %#v, want none", got)
	}

	ambiguous := correctionPlanInventoryPack(query, AdaptiveV2)
	ambiguous.Entrypoints = append(ambiguous.Entrypoints, ContextLocation{Project: "services/audit"})
	if got := contextPlanFiles(ambiguous, index); len(got) != 0 {
		t.Fatalf("ambiguous-entrypoint test inventory = %#v, want none", got)
	}
	if got := contextProductionPlanFiles(ambiguous, index); len(got) != 0 {
		t.Fatalf("ambiguous-entrypoint production inventory = %#v, want none", got)
	}
}

func TestAdaptiveCorrectionPlanProviderFallbackRejectsNonExactContracts(t *testing.T) {
	index := correctionPlanInventoryIndex()
	query := "Prepare a fix plan listing production, configuration, and test files for internal interfaces."
	for _, confidence := range []string{"RESOLVED", "EXTRACTED"} {
		t.Run(confidence, func(t *testing.T) {
			pack := correctionPlanInventoryPack(query, AdaptiveV2)
			pack.Contracts[0].Confidence = confidence
			if got := contextPlanFiles(pack, index); len(got) != 0 {
				t.Errorf("%s contract authorized test provider inventory: %#v", confidence, got)
			}
			if got := contextProductionPlanFiles(pack, index); len(got) != 0 {
				t.Fatalf("%s contract authorized production provider inventory: %#v", confidence, got)
			}
		})
	}
}

func TestAdaptiveCorrectionPlanProviderFallbackRequiresSelectedExactModel(t *testing.T) {
	const query = "DELETE /catalog/items/{itemId}: For services/jobs, prepare a fix plan listing production, configuration, and test files for the CleanupRecord model and internal interfaces."
	tests := []struct {
		name        string
		selectedIDs []string
		confidence  string
	}{
		{name: "planned model is not selected"},
		{name: "selected helper is not a model", selectedIDs: []string{"jobs-helper"}},
		{name: "selected model is not exact", selectedIDs: []string{"jobs-model"}, confidence: "RESOLVED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := correctionPlanInventoryIndex()
			if test.confidence != "" {
				for factIndex := range index.Facts {
					if index.Facts[factIndex].ID == "jobs-model" {
						index.Facts[factIndex].Confidence = test.confidence
					}
				}
			}
			pack := correctionPlanInventoryPack(query, AdaptiveV2)
			pack.Contracts = nil
			pack.selectedSourceFactIDs = test.selectedIDs
			_, _, modelProjects := contextEvidenceProjectRoles(pack, index)
			if !modelProjects["services/jobs"] {
				t.Fatal("fixture did not exercise the broader planned model role")
			}
			if got := contextPlanFiles(pack, index); len(got) != 0 {
				t.Errorf("%s authorized test provider inventory: %#v", test.name, got)
			}
			if got := contextProductionPlanFiles(pack, index); len(got) != 0 {
				t.Fatalf("%s authorized production provider inventory: %#v", test.name, got)
			}
		})
	}
}

func TestBuildContextPlansGermanCorrectionInventoryWithoutChangingProofSemantics(t *testing.T) {
	index := runtimeShapeReleaseQualityMissingContractIndex()
	index.Facts = append(index.Facts, scan.AgentContextFactRecord{
		ID: "jobs-cleanup-service-test", Project: "services/jobs", Kind: "symbol",
		Name: "JobCleanupServiceTest", Qualified: "jobs.JobCleanupServiceTest",
		File: "src/test/java/example/JobCleanupServiceTest.java", Line: 8, EndLine: 12,
		Confidence: "EXACT", Search: "job cleanup service test",
	})
	root := writeReleaseQualityMissingContractFixtureWithIndex(t, index)
	query := "DELETE /catalog/items/{itemId}: Korrekturplan für interne Schnittstellen über libraries/job-client und services/jobs. " +
		"Nenne Produktions-, Konfigurations- und Testdateien für CatalogJobEntity, Authentifizierung, Persistenz und Wiederholungsverhalten."
	request := ContextRequest{Root: root, Query: query, ProtocolVersion: AdaptiveV2, BudgetTokens: 4000, MaxFiles: 12}

	pack, err := BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Query != query {
		t.Fatalf("public query = %q, want original %q", pack.Query, query)
	}
	if contextQueryPlansMissingTransition(pack.Query) || contextQueryPlansMissingTransition(contextSelectionQuery(pack)) {
		t.Fatalf("correction plan changed raw missing-transition semantics: public=%q selection=%q", pack.Query, contextSelectionQuery(pack))
	}
	if !hasContextConcernKindInPack(pack, contextConcernResilience) {
		t.Fatalf("compiled concerns omit resilience: %#v", pack.Concerns)
	}
	if len(pack.PlanFiles) == 0 {
		t.Fatalf("compiled correction plan has no provider test inventory: %#v", pack.PlanFiles)
	}
	if !contextPackRepresentsSourcePath(pack, "services/jobs", "src/main/java/example/JobManagementController.java") {
		t.Fatalf("compiled correction plan has no provider contract evidence: production=%#v source=%#v omissions=%#v", pack.ProductionPlanFiles, pack.SourceSections, pack.SourceOmissions)
	}
	if pack.EstimatedTokens > request.BudgetTokens || contextSourceFileCount(pack) > request.MaxFiles {
		t.Fatalf("compiled correction plan exceeds bounds: tokens=%d/%d files=%d/%d", pack.EstimatedTokens, request.BudgetTokens, contextSourceFileCount(pack), request.MaxFiles)
	}
	if contextSourceFileCount(pack) != request.MaxFiles {
		t.Fatalf("correction-plan fixture did not saturate the file budget: files=%d/%d", contextSourceFileCount(pack), request.MaxFiles)
	}
	foundPrimaryMutation := false
	for _, section := range pack.SourceSections {
		if section.Project == "services/catalog" &&
			section.Path == "src/main/java/example/CatalogController.java" &&
			strings.Contains(section.Content, "deleteItem") {
			foundPrimaryMutation = true
		}
	}
	if !foundPrimaryMutation {
		t.Fatalf("saturated correction plan lost primary mutation evidence: %#v", pack.SourceSections)
	}
	encoded, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "services/archive") {
		t.Fatalf("compiled correction plan leaked unrelated project: %s", encoded)
	}
	again, err := BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	againEncoded, err := json.Marshal(again)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(againEncoded) {
		t.Fatalf("saturated correction plan is not deterministic:\nfirst=%s\nsecond=%s", encoded, againEncoded)
	}
}

func correctionPlanInventoryPack(query, protocol string) ContextPack {
	return ContextPack{
		Query: query, selectionQuery: query, ProtocolVersion: protocol,
		Entrypoints: []ContextLocation{{ID: "catalog-route", Project: "services/catalog", Confidence: "EXACT"}},
		Contracts:   []ContextLocation{{ID: "jobs-contract", Project: "services/jobs", Confidence: "EXACT"}},
	}
}

func correctionPlanInventoryIndex() scan.AgentContextIndexRecord {
	return scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "catalog-route", Project: "services/catalog", Kind: "route", Name: "DELETE /catalog/items/{itemId}", File: "src/main/java/example/CatalogController.java", Confidence: "EXACT"},
		{ID: "jobs-contract", Project: "services/jobs", Kind: "api_contract", Name: "GET /job-management/jobs", File: "src/main/java/example/JobManagementController.java", Confidence: "EXACT"},
		{ID: "jobs-management-endpoint", Project: "services/jobs", Kind: "api_endpoint", Name: "GET /job-management/jobs", Qualified: "jobs.JobManagementController.listJobs", File: "src/main/java/example/JobManagementController.java", Line: 121, Confidence: "EXACT"},
		{ID: "jobs-management-test", Project: "services/jobs", Kind: "symbol", Name: "JobManagementControllerTest", Qualified: "jobs.JobManagementControllerTest", File: "src/test/java/example/JobManagementControllerTest.java", Confidence: "EXACT"},
		{ID: "jobs-management-config", Project: "services/jobs", Kind: "api_endpoint", Name: "GET /job-management/jobs", Qualified: "jobs.JobManagementController.listJobs", File: "src/main/java/example/config/JobManagementController.java", Line: 121, Confidence: "EXACT"},
		{ID: "jobs-management-invalid", Project: "services/jobs", Kind: "api_endpoint", Name: "GET /job-management/jobs", Qualified: "jobs.JobManagementController.listJobs", File: "/private/example/JobManagementController.java", Confidence: "EXACT"},
		{ID: "jobs-controller", Project: "services/jobs", Kind: "symbol", Name: "JobManagementController", File: "src/main/java/example/JobManagementController.java", Confidence: "EXACT"},
		{ID: "jobs-model", Project: "services/jobs", Kind: "record", Name: "CleanupRecord", Qualified: "jobs.CleanupRecord", File: "src/main/java/example/CleanupRecord.java", Confidence: "EXACT", Search: "cleanup record model"},
		{ID: "jobs-model-repository", Project: "services/jobs", Kind: "symbol", Name: "CleanupRecordRepository", Qualified: "jobs.CleanupRecordRepository", File: "src/main/java/example/CleanupRecordRepository.java", Confidence: "EXACT", Search: "cleanup record primary persistence repository"},
		{ID: "jobs-helper", Project: "services/jobs", Kind: "symbol", Name: "CleanupConfiguration", Qualified: "jobs.CleanupConfiguration", File: "src/main/java/example/CleanupConfiguration.java", Confidence: "EXACT", Search: "cleanup helper configuration"},
		{ID: "jobs-controller-test", Project: "services/jobs", Kind: "symbol", Name: "JobManagementControllerTest", File: "src/test/java/example/JobManagementControllerTest.java", Confidence: "EXACT"},
		{ID: "archive-helper", Project: "services/archive", Kind: "symbol", Name: "ArchiveHelper", File: "src/main/java/example/ArchiveHelper.java", Confidence: "EXACT"},
		{ID: "archive-test", Project: "services/archive", Kind: "symbol", Name: "ArchiveManagementControllerTest", File: "src/test/java/example/ArchiveManagementControllerTest.java", Confidence: "EXACT"},
	}}
}

func hasContextConcernKindInPack(pack ContextPack, kind string) bool {
	for _, concern := range pack.Concerns {
		if normalizedContextConcernKind(concern.Kind) == kind {
			return true
		}
	}
	return false
}
