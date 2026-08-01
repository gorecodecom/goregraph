package agent

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestContextPlanFilesKeepsProviderTestsAndPairedCallerPatterns(t *testing.T) {
	const query = "When DELETE /catalog/items/{itemId} removes an item, plan the missing internal HTTP contract and cross-service cleanup through services/jobs, including retries, the existing InventoryClient test patterns, and the exact production and test files to change or create."
	pack := ContextPack{
		Query: query, selectionQuery: query,
		Entrypoints: []ContextLocation{
			{Project: "services/catalog", ID: "catalog-route"},
			{Project: "services/catalog", ID: "catalog-operations"},
		},
		Concerns: []ContextConcern{
			{Kind: contextConcernProject, Project: "services/jobs", Covered: true},
			{Kind: contextConcernTests, Project: "services/jobs", Covered: false},
		},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "provider-controller-test", Project: "services/jobs", Kind: "symbol", Name: "JobManagementControllerTest", Qualified: "example.JobManagementControllerTest", File: "src/test/java/example/JobManagementControllerTest.java", Line: 20, Confidence: "EXACT", Search: "job management internal controller test"},
		{ID: "provider-cleanup-service-test", Project: "services/jobs", Kind: "symbol", Name: "JobCleanupServiceTest", Qualified: "example.JobCleanupServiceTest", File: "src/test/java/example/JobCleanupServiceTest.java", Line: 30, Confidence: "EXACT", Search: "job cleanup service test"},
		{ID: "provider-mail-service-test", Project: "services/jobs", Kind: "symbol", Name: "JobMailServiceTest", Qualified: "example.JobMailServiceTest", File: "src/test/java/example/JobMailServiceTest.java", Line: 35, Confidence: "EXACT", Search: query},
		{ID: "caller-mock", Project: "services/catalog", Kind: "symbol", Name: "InventoryClientMock", Qualified: "example.InventoryClientMock", File: "src/test/java/example/InventoryClientMock.java", Line: 10, Confidence: "EXACT", Search: "outbound inventory client mock"},
		{ID: "caller-retry", Project: "services/catalog", Kind: "symbol", Name: "InventoryClientRetryableTest", Qualified: "example.InventoryClientRetryableTest", File: "src/test/java/example/InventoryClientRetryableTest.java", Line: 15, Confidence: "EXACT", Search: "outbound inventory client retry test"},
		{ID: "unmatched-mock", Project: "services/catalog", Kind: "symbol", Name: "AuditClientMock", File: "src/test/java/example/AuditClientMock.java", Confidence: "EXACT"},
		{ID: "partial-test", Project: "services/jobs", Kind: "symbol", Name: "JobRepositoryTest", File: "src/test/java/example/JobRepositoryTest.java", Confidence: "PARTIAL"},
		{ID: "production-symbol", Project: "services/jobs", Kind: "symbol", Name: "JobServiceTest", File: "src/main/java/example/JobServiceTest.java", Confidence: "EXACT"},
		{ID: "unrelated-test", Project: "services/audit", Kind: "symbol", Name: "AuditManagementControllerTest", File: "src/test/java/example/AuditManagementControllerTest.java", Confidence: "EXACT"},
	}}

	got := contextPlanFiles(pack, index)
	want := []ContextPlanFile{
		{Project: "services/jobs", Path: "src/test/java/example/JobManagementControllerTest.java", Use: "provider_test"},
		{Project: "services/jobs", Path: "src/test/java/example/JobCleanupServiceTest.java", Use: "provider_test"},
		{Project: "services/catalog", Path: "src/test/java/example/InventoryClientMock.java", Use: "mock_pattern"},
		{Project: "services/catalog", Path: "src/test/java/example/InventoryClientRetryableTest.java", Use: "retry_pattern"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan files = %#v, want %#v", got, want)
	}
}

func TestContextPlanFilesRequiresMissingTransitionExactInventory(t *testing.T) {
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "provider-test", Project: "services/jobs", Kind: "symbol",
		Name: "JobServiceTest", File: "src/test/java/example/JobServiceTest.java",
		Confidence: "EXACT",
	}}}
	pack := ContextPack{
		Query:       "Explain the existing job service tests.",
		Entrypoints: []ContextLocation{{Project: "services/catalog"}},
		Concerns:    []ContextConcern{{Kind: contextConcernTests, Project: "services/jobs"}},
	}
	if got := contextPlanFiles(pack, index); len(got) != 0 {
		t.Fatalf("ordinary-query plan files = %#v, want none", got)
	}
}

func TestContextPlanFilesAcceptsInflectedGermanFilePlan(t *testing.T) {
	const query = "Plane den fehlenden internen API-Vertrag. Nenne alle zu ändernden oder anzulegenden Produktions- und Testdateien sowie die erforderlichen Tests."
	pack := ContextPack{
		Query: query, selectionQuery: query,
		Endpoints: []ContextEndpoint{{Provider: "services/catalog"}},
		Concerns:  []ContextConcern{{Kind: contextConcernTests, Project: "services/jobs"}},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "provider-test", Project: "services/jobs", Kind: "symbol",
		Name: "JobServiceTest", File: "src/test/java/example/JobServiceTest.java",
		Confidence: "EXACT",
	}}}

	want := []ContextPlanFile{{
		Project: "services/jobs", Path: "src/test/java/example/JobServiceTest.java", Use: "provider_test",
	}}
	if got := contextPlanFiles(pack, index); !reflect.DeepEqual(got, want) {
		t.Fatalf("inflected German plan files = %#v, want %#v", got, want)
	}
}

func TestContextPlanFilesAcceptsAffectedGermanFilePlan(t *testing.T) {
	const query = "Plane den fehlenden internen API-Vertrag. Nenne die betroffenen Produktions- und Testdateien sowie die erforderlichen Tests."
	pack := ContextPack{
		Query: query, selectionQuery: query,
		Endpoints: []ContextEndpoint{{Provider: "services/catalog"}},
		Concerns:  []ContextConcern{{Kind: contextConcernTests, Project: "services/jobs"}},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "provider-test", Project: "services/jobs", Kind: "symbol",
		Name: "JobServiceTest", File: "src/test/java/example/JobServiceTest.java",
		Confidence: "EXACT",
	}}}

	want := []ContextPlanFile{{
		Project: "services/jobs", Path: "src/test/java/example/JobServiceTest.java", Use: "provider_test",
	}}
	if got := contextPlanFiles(pack, index); !reflect.DeepEqual(got, want) {
		t.Fatalf("affected German plan files = %#v, want %#v", got, want)
	}
}

func TestContextPlanFilesRequiresUnambiguousEntrypointProject(t *testing.T) {
	const query = "Plan the missing internal HTTP contract, reuse the existing InventoryClient test patterns, and identify the exact production and test files to change or create."
	pack := ContextPack{
		Query: query, selectionQuery: query,
		Entrypoints: []ContextLocation{
			{Project: "services/catalog"},
			{Project: "services/audit"},
		},
		Concerns: []ContextConcern{{Kind: contextConcernProject, Project: "services/jobs"}},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "provider-test", Project: "services/jobs", Kind: "symbol",
		Name: "JobServiceTest", File: "src/test/java/example/JobServiceTest.java",
		Confidence: "EXACT",
	}}}

	if got := contextPlanFiles(pack, index); len(got) != 0 {
		t.Fatalf("ambiguous-entrypoint plan files = %#v, want none", got)
	}
}

func TestContextPlanFilesUsesRepresentedPairOnlyForValidation(t *testing.T) {
	const query = "Plan the missing internal HTTP contract, reuse the existing InventoryClient test patterns, and identify the exact production and test files to change or create."
	pack := ContextPack{
		Query: query, selectionQuery: query,
		Entrypoints: []ContextLocation{{Project: "services/catalog"}},
		Concerns:    []ContextConcern{{Kind: contextConcernProject, Project: "services/jobs"}},
		SourceOmissions: []ContextSourceOmission{{
			Project: "services/catalog", Path: "src/test/java/example/InventoryClientRetryableTest.java",
			StartLine: 10, EndLine: 20,
		}},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "caller-mock", Project: "services/catalog", Kind: "symbol", Name: "InventoryClientMock", File: "src/test/java/example/InventoryClientMock.java", Confidence: "EXACT"},
		{ID: "caller-retry", Project: "services/catalog", Kind: "symbol", Name: "InventoryClientRetryableTest", File: "src/test/java/example/InventoryClientRetryableTest.java", Confidence: "EXACT"},
		{ID: "unrelated-mock", Project: "services/catalog", Kind: "symbol", Name: "ContractClientMock", File: "src/test/java/example/ContractClientMock.java", Confidence: "EXACT", Search: query},
		{ID: "unrelated-retry", Project: "services/catalog", Kind: "symbol", Name: "ContractClientRetryableTest", File: "src/test/java/example/ContractClientRetryableTest.java", Confidence: "EXACT", Search: query},
	}}

	want := []ContextPlanFile{{
		Project: "services/catalog", Path: "src/test/java/example/InventoryClientMock.java", Use: "mock_pattern",
	}}
	if got := contextPlanFiles(pack, index); !reflect.DeepEqual(got, want) {
		t.Fatalf("plan files = %#v, want unrepresented half %#v", got, want)
	}
}

func TestContextPlanFilePatternRelevanceMapsUserInformationOnly(t *testing.T) {
	pack := ContextPack{Query: "Berücksichtige Nebenwirkungen bei Benutzerinformationen."}
	userMock := scan.AgentContextFactRecord{Name: "UserMgmtServiceMock"}
	userRetry := scan.AgentContextFactRecord{Name: "UserMgmtServiceRetryableTest"}
	if !contextPlanFilePatternRelevant(pack, userMock, userRetry) {
		t.Fatal("UserMgmt pattern did not match requested Benutzerinformationen")
	}
	licenseMock := scan.AgentContextFactRecord{Name: "LicenseMgmtServiceMock"}
	licenseRetry := scan.AgentContextFactRecord{Name: "LicenseMgmtServiceRetryableTest"}
	if contextPlanFilePatternRelevant(pack, licenseMock, licenseRetry) {
		t.Fatal("unrelated LicenseMgmt pattern matched requested Benutzerinformationen")
	}
}

func TestCompactContextPlanFileInventoryDropsOnlyRedundantReasons(t *testing.T) {
	withPlanFiles := ContextPack{
		Files: []ContextFile{{
			Project: "services/catalog", Path: "CatalogController.java",
			Role: "entrypoint", Reason: "selected required entrypoint evidence", Confidence: "EXACT",
		}},
		PlanFiles: []ContextPlanFile{{
			Project: "services/jobs", Path: "JobServiceTest.java", Use: "provider_test",
		}},
	}
	compacted := compactContextPlanFileInventory(withPlanFiles)
	if compacted.Files[0].Reason != "" ||
		compacted.Files[0].Role != "entrypoint" ||
		compacted.Files[0].Confidence != "EXACT" {
		t.Fatalf("compacted file metadata = %#v", compacted.Files[0])
	}

	ordinary := ContextPack{Files: []ContextFile{{Reason: "selected call evidence"}}}
	if got := compactContextPlanFileInventory(ordinary); got.Files[0].Reason != "selected call evidence" {
		t.Fatalf("ordinary file reason = %q, want unchanged", got.Files[0].Reason)
	}
}

func TestContextFinalDecisionBudgetReserveDoesNotPrechargePlanFiles(t *testing.T) {
	const query = "Plan the missing internal HTTP contract and exact production and test files to change or create."
	pack, err := finalizeContextEstimate(ContextPack{
		Schema: 3, Query: query, selectionQuery: query, BudgetTokens: 4000,
		Entrypoints: []ContextLocation{{Project: "services/catalog"}},
		Concerns: []ContextConcern{
			{Kind: contextConcernProject, Project: "services/jobs", Covered: true},
		},
		Files: []ContextFile{{
			Project: "services/catalog", Path: "CatalogController.java",
			Role: "entrypoint", Reason: "x",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "provider-service-test", Project: "services/jobs", Kind: "symbol",
		Name: "JobServiceTest", File: "src/test/java/example/JobServiceTest.java",
		Confidence: "EXACT",
	}}}

	reserve, err := contextFinalDecisionBudgetReserve(pack, index)
	if err != nil {
		t.Fatal(err)
	}
	if reserve != 0 {
		t.Fatalf("plan-file-only final decision reserve = %d, want 0", reserve)
	}
}

func TestBuildContextKeepsCompactPlanFileEvidence(t *testing.T) {
	index := runtimeShapeReleaseQualityMissingContractIndex()
	index.Facts = append(index.Facts,
		scan.AgentContextFactRecord{
			ID: "catalog-inventory-client-mock", Project: "services/catalog", Kind: "symbol",
			Name: "InventoryClientMock", Qualified: "catalog.InventoryClientMock",
			File: "src/test/java/example/InventoryClientMock.java", Line: 8, EndLine: 12,
			Confidence: "EXACT", Search: "existing outbound client mock pattern",
		},
		scan.AgentContextFactRecord{
			ID: "catalog-inventory-client-retry-test", Project: "services/catalog", Kind: "symbol",
			Name: "InventoryClientRetryableTest", Qualified: "catalog.InventoryClientRetryableTest",
			File: "src/test/java/example/InventoryClientRetryableTest.java", Line: 8, EndLine: 12,
			Confidence: "EXACT", Search: "existing outbound client retry test pattern",
		},
	)
	root := writeReleaseQualityMissingContractFixtureWithIndex(t, index)
	query := "When DELETE /catalog/items/{itemId} removes an item in services/catalog, " +
		"plan the missing internal HTTP contract through libraries/job-client and services/jobs. " +
		"Cover authentication, configuration, retries, persistence, side effects, reuse the existing " +
		"InventoryClient mock and retry test patterns, and identify " +
		"the exact production and test files to change or create."
	request := ContextRequest{
		Root: root, Query: query, BudgetTokens: 4000, MaxFiles: 12,
	}

	pack, err := BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	want := []ContextPlanFile{
		{Project: "services/catalog", Path: "src/test/java/example/InventoryClientMock.java", Use: "mock_pattern"},
	}
	if !reflect.DeepEqual(pack.PlanFiles, want) {
		t.Fatalf("plan files = %#v, want %#v", pack.PlanFiles, want)
	}
	for _, evidence := range []struct {
		project string
		path    string
	}{
		{project: "libraries/job-client", path: "src/main/java/example/JobClient.java"},
		{project: "libraries/job-client", path: "src/main/java/example/JobClientConfig.java"},
		{project: "libraries/job-client", path: "src/main/java/example/JobClientAuth.java"},
		{project: "services/catalog", path: "src/main/resources/application.yml"},
		{project: "services/catalog", path: "src/test/resources/application-test.yml"},
		{project: "services/jobs", path: "src/main/java/example/JobManagementController.java"},
		{project: "services/jobs", path: "src/main/java/example/JobSecurity.java"},
		{project: "services/jobs", path: "src/main/java/example/CatalogJobRepository.java"},
		{project: "services/jobs", path: "src/main/java/example/CatalogChangeJobRepository.java"},
		{project: "services/jobs", path: "src/main/resources/application.properties"},
	} {
		if !contextPackRepresentsSourcePath(pack, evidence.project, evidence.path) {
			t.Errorf("prior production evidence %q missing", evidence.project+":"+evidence.path)
		}
	}
	for _, evidence := range []struct {
		project string
		path    string
	}{
		{project: "services/jobs", path: "src/test/java/example/JobServiceTest.java"},
		{project: "services/catalog", path: "src/test/java/example/InventoryClientRetryableTest.java"},
	} {
		if !contextPackRepresentsSourcePath(pack, evidence.project, evidence.path) {
			t.Errorf("represented plan-file counterpart %q missing", evidence.project+":"+evidence.path)
		}
	}
	if pack.EstimatedTokens > 4000 ||
		contextSourceFileCount(pack) > 12 ||
		len(pack.SourceSections) > 12 ||
		len(pack.SourceOmissions) > MaxContextSourceOmissions {
		t.Fatalf(
			"compact plan-file pack exceeds bounds: tokens=%d aggregate_files=%d sections=%d omissions=%d",
			pack.EstimatedTokens,
			contextSourceFileCount(pack),
			len(pack.SourceSections),
			len(pack.SourceOmissions),
		)
	}

	again, err := BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(again)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("compact plan-file pack is not deterministic:\nfirst=%s\nsecond=%s", firstJSON, secondJSON)
	}
}
