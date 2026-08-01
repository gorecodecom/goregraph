package agent

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	missingContractEnglishQuery = "When DELETE /catalog/items/{itemId} removes an item in services/catalog, plan cleanup of related jobs through libraries/job-client and services/jobs. Cover the current path, missing HTTP contract, task types and lookup attributes, authentication, configuration, retry behavior, persistence, side effects, and tests."
	missingContractGermanQuery  = "Wenn DELETE /catalog/items/{itemId} einen Eintrag in services/catalog löscht, plane das Entfernen verbundener Aufgaben über libraries/job-client und services/jobs. Berücksichtige aktuellen Pfad, fehlenden HTTP-Vertrag, Aufgabenarten und Suchattribute, Authentifizierung, Konfiguration, Wiederholung, Persistenz, Nebenwirkungen und Tests."
	releaseQualityQuery         = "When DELETE /catalog/items/{itemId} removes an item in services/catalog, plan a release-ready cleanup of related jobs through libraries/job-client and services/jobs. Cover the provider base job model in services/jobs and its catalogId/itemId lookup attributes, JobClientConfig Spring configuration properties for base URL, credentials, timeouts and retry limits, JobClientAuth setBasicAuth headers, JobSecurity technical-role policy, CatalogJobRepository and CatalogChangeJobRepository persistence, and JobManagementControllerTest and JobServiceTest coverage."
	broadReleaseQualityQuery    = "When DELETE /catalog/items/{itemId} removes an item in services/catalog, analyze the current and required cross-service cleanup through libraries/job-client and services/jobs. Cover the public and internal HTTP contracts, both job types and lookup attributes, client and provider authentication and configuration, retry and failure behavior, persistence, side effects, and the exact production and executable test files required for a release-ready change."
)

func TestContextMissingTransitionOrderingGapRequiresCrossProjectPlan(t *testing.T) {
	const query = "Analyze the current and required new call chain and internal HTTP contract for cross-service job cleanup."
	pack := ContextPack{
		Query: query, selectionQuery: query,
		Endpoints:      []ContextEndpoint{{Provider: "services/catalog"}},
		Contracts:      []ContextLocation{{Project: "libraries/job-client"}},
		SourceSections: []ContextSourceSection{{Path: "src/CatalogController.java"}},
	}
	gap := contextMissingTransitionOrderingGap(pack)
	if gap == nil || gap.Scope != "cross_service_ordering" {
		t.Fatalf("cross-service ordering gap = %#v", gap)
	}

	existing := pack
	existing.Query = "Explain the current cross-service job cleanup call chain."
	existing.selectionQuery = existing.Query
	if gap := contextMissingTransitionOrderingGap(existing); gap != nil {
		t.Fatalf("existing-flow ordering gap = %#v, want none", gap)
	}

	local := pack
	local.Contracts = append([]ContextLocation(nil), pack.Contracts...)
	local.Contracts[0].Project = "services/catalog"
	if gap := contextMissingTransitionOrderingGap(local); gap != nil {
		t.Fatalf("single-project ordering gap = %#v, want none", gap)
	}

	metadata := pack
	metadata.SourceSections = nil
	if gap := contextMissingTransitionOrderingGap(metadata); gap != nil {
		t.Fatalf("metadata-only ordering gap = %#v, want none", gap)
	}
}

func TestBuildContextBudgetsFinalDecisionMetadata(t *testing.T) {
	root := writeMissingContractContextFixture(t)
	const budget = 1750
	pack, err := BuildContext(ContextRequest{
		Root:         root,
		Query:        missingContractEnglishQuery,
		BudgetTokens: budget,
		MaxFiles:     DefaultContextMaxFiles,
	})
	if err != nil {
		t.Fatalf("context failed instead of compacting final decision metadata: %v", err)
	}
	if !contextHasUncertainty(pack, "requested_http_contract") {
		t.Fatalf("missing contract uncertainty = %#v", pack.Uncertainties)
	}
	if pack.SourceCoverage != "complete" {
		if pack.SourceCoverage != "partial" || len(pack.SourceOmissions) == 0 {
			t.Fatalf("source coverage = %q, omissions %#v", pack.SourceCoverage, pack.SourceOmissions)
		}
		for _, omission := range pack.SourceOmissions {
			if omission.Path == "" {
				t.Fatalf("bounded metadata produced a pathless omission: %#v", pack.SourceOmissions)
			}
		}
	}
	if pack.EstimatedTokens > budget {
		t.Fatalf("final pack exceeded budget: %d", pack.EstimatedTokens)
	}
}

func TestBuildContextReturnsStructuredPackWhenFinalMetadataCannotFit(t *testing.T) {
	root := writeMissingContractContextFixture(t)
	const budget = 1400
	pack, err := BuildContext(ContextRequest{
		Root:         root,
		Query:        missingContractEnglishQuery,
		BudgetTokens: budget,
		MaxFiles:     DefaultContextMaxFiles,
	})
	if err != nil {
		t.Fatalf("context failed instead of returning a structured pack: %v", err)
	}
	if pack.ContextID == "" {
		t.Fatal("structured pack has no context id")
	}
	if pack.SourceCoverage == "" {
		t.Fatal("structured pack has no source coverage")
	}
	if pack.EstimatedTokens > budget {
		t.Fatalf("structured pack exceeded budget: %d", pack.EstimatedTokens)
	}
}

func TestBuildContextExcludesDisconnectedOperationalOmissionForMissingTransition(t *testing.T) {
	index := missingContractContextIndex()
	index.Facts = append(
		index.Facts,
		scan.AgentContextFactRecord{
			ID: "jobs-housekeeping-route", Project: "services/jobs", Kind: "route",
			Name:       "DELETE /internal/jobs/housekeeping",
			Qualified:  "JobHousekeepingController.purgeArchivedJobs",
			HTTPMethod: "DELETE", Path: "/internal/jobs/housekeeping",
			File: "src/main/java/example/JobHousekeepingController.java",
			Line: 8, EndLine: 13, Confidence: "EXACT",
			Search: "job task housekeeping purge archived batch endpoint",
		},
		scan.AgentContextFactRecord{
			ID: "jobs-housekeeping-service", Project: "services/jobs", Kind: "symbol",
			Name:      "purgeArchivedJobs",
			Qualified: "JobHousekeepingService.purgeArchivedJobs",
			File:      "src/main/java/example/JobHousekeepingService.java",
			Line:      8, EndLine: 13, Confidence: "EXACT",
			Search: "job task housekeeping purge archived batch service",
		},
	)
	index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
		ID:         "jobs-housekeeping-call",
		FromFactID: "jobs-housekeeping-route", ToFactID: "jobs-housekeeping-service",
		Kind: "call", Confidence: "EXACT",
	})
	root := writeMissingContractContextIndexFixture(t, index)
	query := "When DELETE /catalog/items/{itemId} removes an item in services/catalog, " +
		"plan deletion of related jobs through libraries/job-client and services/jobs. " +
		"Cover the current path, missing HTTP contract, task types and lookup attributes, and persistence."

	var pack ContextPack
	foundBoundedDomainOmission := false
	for budget := MinContextBudgetTokens; budget <= 2400; budget += 25 {
		candidate, err := BuildContext(ContextRequest{
			Root: root, Query: query, BudgetTokens: budget, MaxFiles: 4,
		})
		if err != nil || candidate.FallbackRequired ||
			!contextHasUncertainty(candidate, "requested_http_contract") {
			continue
		}
		for _, omission := range candidate.SourceOmissions {
			if omission.Path != "" &&
				(omission.Role == contextConcernDomainModel || omission.Role == "persistence") {
				pack = candidate
				foundBoundedDomainOmission = true
				break
			}
		}
		if foundBoundedDomainOmission {
			break
		}
	}
	if !foundBoundedDomainOmission {
		t.Fatal("fixture has no bounded domain or persistence omission")
	}
	if len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "catalog-route" {
		t.Fatalf("primary entrypoint changed: %#v", pack.Entrypoints)
	}
	for _, omission := range pack.SourceOmissions {
		if strings.Contains(strings.ToLower(omission.Path), "housekeeping") {
			t.Fatalf("disconnected operational source consumed an omission slot: %#v", pack.SourceOmissions)
		}
	}
	if pack.RetryAllowed {
		t.Fatalf("disconnected operational source enabled retry: %#v", pack)
	}
}

func TestFinalContextBudgetFallbackPreservesBoundedContract(t *testing.T) {
	request := ContextRequest{
		Query:        "x",
		BudgetTokens: MinContextBudgetTokens,
		MaxFiles:     DefaultContextMaxFiles,
	}
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-24T08:00:00Z",
	}
	metadataPack, err := newContextEnvelope(index, request)
	if err != nil {
		t.Fatal(err)
	}
	metadataPack.ContextID = strings.Repeat("a", 24)
	metadataPack.Confidence = "MEDIUM"
	metadataPack.Files = []ContextFile{{
		Project: "services/catalog",
		Path:    strings.Repeat("oversized/", 100),
	}}

	pack, err := finalContextBudgetFallback(metadataPack, index, request, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || pack.FallbackReason != finalContextBudgetFallbackReason {
		t.Fatalf("fallback contract = %#v", pack)
	}
	if pack.SourceCoverage != "none" || len(pack.SourceSections) != 0 ||
		pack.RetryAllowed || len(pack.RetryAnchors) != 0 {
		t.Fatalf("fallback source state = %#v", pack)
	}
	if pack.ContextID != metadataPack.ContextID {
		t.Fatalf("context id = %q, want %q", pack.ContextID, metadataPack.ContextID)
	}
	if pack.EstimatedTokens > request.BudgetTokens ||
		len(body) > contextByteBudget(request.BudgetTokens) {
		t.Fatalf(
			"fallback exceeded budget: tokens=%d/%d bytes=%d/%d",
			pack.EstimatedTokens,
			request.BudgetTokens,
			len(body),
			contextByteBudget(request.BudgetTokens),
		)
	}
}

func TestBuildContextSupportsMissingContractChangeAnalysis(t *testing.T) {
	root := writeMissingContractContextFixture(t)
	pack, err := BuildContext(ContextRequest{Root: root, Query: missingContractEnglishQuery})
	if err != nil {
		t.Fatal(err)
	}

	if len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "catalog-route" {
		t.Fatalf("primary entrypoint = %#v", pack.Entrypoints)
	}
	for _, relationship := range pack.CallChain {
		if relationship.From == "CatalogOperations.deleteItem" &&
			strings.Contains(relationship.To, "Job") {
			t.Fatalf("fabricated future relationship: %#v", relationship)
		}
	}

	selected := contextSelectedFactSet(pack)
	for _, factID := range []string{
		"catalog-route", "catalog-operations", "catalog-repository",
		"job-client", "job-contract", "job-config", "job-auth", "job-retry",
		"jobs-route", "jobs-service",
	} {
		if !selected[factID] {
			t.Errorf("required evidence %q not selected", factID)
		}
	}
	if selected["jobs-find-all"] {
		t.Error("generic inherited findAll displaced the declared finder")
	}
	paths := contextSourcePathSet(pack)
	for _, path := range []string{
		"src/main/java/example/CatalogJobEntity.java",
		"src/main/java/example/CatalogChangeJobEntity.java",
		"src/main/java/example/CatalogJobRepository.java",
		"src/main/java/example/CatalogChangeJobRepository.java",
	} {
		if !paths[path] {
			t.Errorf("required domain evidence %q missing from %#v", path, pack.SourceSections)
		}
	}
	for _, identity := range []string{"catalogId", "itemId", "changeId"} {
		if !contextSourceContainsStableIdentity(pack, identity) {
			t.Errorf("lookup identity %q missing from source sections", identity)
		}
	}
	for _, operationalEvidence := range []string{
		"@Retryable",
		"configuration.getAllJobsPath",
		"basicAuthentication",
		"publishDeletion",
	} {
		if !contextSourceContainsStableIdentity(pack, operationalEvidence) {
			t.Errorf("operational evidence %q missing from source sections", operationalEvidence)
		}
	}
	if !paths["src/main/java/example/JobHousekeeping.java"] {
		t.Errorf("side-effect method source missing from %#v", pack.SourceSections)
	}
	for _, path := range []string{
		"src/main/java/example/MailProperties.java",
		"src/main/java/example/AsyncExceptionHandler.java",
		"src/main/java/example/CatalogTopicRepository.java",
	} {
		if paths[path] {
			t.Errorf("generic distractor %q displaced domain evidence", path)
		}
	}
	if !contextHasUncertainty(pack, "requested_http_contract") {
		t.Fatalf("missing contract uncertainty = %#v", pack.Uncertainties)
	}
	if pack.FallbackRequired || pack.RetryAllowed {
		t.Fatalf(
			"pack decision = fallback %v retry %v coverage %q omissions %#v",
			pack.FallbackRequired,
			pack.RetryAllowed,
			pack.SourceCoverage,
			pack.SourceOmissions,
		)
	}
	if pack.SourceCoverage != "complete" {
		if pack.SourceCoverage != "partial" || len(pack.SourceOmissions) == 0 {
			t.Fatalf("missing evidence was not represented: %#v", pack.SourceOmissions)
		}
		for _, omission := range pack.SourceOmissions {
			if omission.Path == "" {
				t.Fatalf("missing evidence lacks a targeted path: %#v", pack.SourceOmissions)
			}
		}
	}
	for _, concern := range pack.Concerns {
		if !concern.Covered || !contextSourceRequiresRenderedConcernEvidence(concern.Kind) {
			continue
		}
		supported := false
		internalConcern := newContextConcern(
			concern.Kind,
			normalizeContextProject(concern.Project),
			true,
			nil,
			concern.Reason,
		)
		for _, section := range pack.SourceSections {
			if contextSourceSectionSupportsConcern(section, internalConcern) {
				supported = true
				break
			}
		}
		if !supported {
			t.Errorf("covered concern lacks actionable rendered source: %#v", concern)
		}
	}
	for _, section := range pack.SourceSections {
		if section.Role != "test" {
			continue
		}
		if section.RenderMode == "signature" ||
			!contextSourceSectionHasExecutableTest(section, contextSourceSemanticContent(section.Content)) {
			t.Errorf("test source lacks executable rendered body: %#v", section)
		}
	}
	if pack.EstimatedTokens > DefaultContextBudgetTokens ||
		len(pack.SourceSections) > MaxContextSourceSections ||
		len(pack.Files) > DefaultContextMaxFiles {
		t.Fatalf(
			"pack bounds = tokens %d sections %d files %d",
			pack.EstimatedTokens,
			len(pack.SourceSections),
			len(pack.Files),
		)
	}
}

func TestBuildContextProvesReleaseQualityWithoutPrivateRules(t *testing.T) {
	root := writeReleaseQualityMissingContractFixture(t)
	pack, err := BuildContext(ContextRequest{Root: root, Query: releaseQualityQuery})
	if err != nil {
		t.Fatal(err)
	}

	hasProjectPath := func(project, path string) bool {
		project = normalizeContextProject(project)
		path = contextPackSourceFile(path)
		for _, file := range pack.Files {
			if normalizeContextProject(file.Project) == project &&
				contextPackSourceFile(file.Path) == path {
				return true
			}
		}
		for _, section := range pack.SourceSections {
			if normalizeContextProject(section.Project) == project &&
				contextPackSourceFile(section.Path) == path {
				return true
			}
		}
		return false
	}
	for _, want := range []struct {
		project string
		path    string
	}{
		{project: "services/jobs", path: "src/main/java/example/BaseCatalogJobEntity.java"},
		{project: "libraries/job-client", path: "src/main/java/example/JobClientConfig.java"},
		{project: "libraries/job-client", path: "src/main/java/example/JobClientAuth.java"},
		{project: "services/jobs", path: "src/main/java/example/JobSecurity.java"},
		{project: "services/jobs", path: "src/main/java/example/CatalogJobRepository.java"},
		{project: "services/jobs", path: "src/main/java/example/CatalogChangeJobRepository.java"},
		{project: "services/jobs", path: "src/test/java/example/JobManagementControllerTest.java"},
		{project: "services/jobs", path: "src/test/java/example/JobServiceTest.java"},
	} {
		if !hasProjectPath(want.project, want.path) {
			t.Errorf(
				"required production/test inventory %q missing",
				want.project+":"+want.path,
			)
		}
	}
	for _, want := range []string{
		"catalogId",
		"itemId",
		"setBasicAuth",
		"TECHNICAL_USER",
		"@ConfigurationProperties",
	} {
		if !contextSourceContainsStableIdentity(pack, want) {
			t.Errorf("rendered evidence %q missing", want)
		}
	}
	if contextPackContainsFileSuffix(pack, "services/catalog/src/main/java/example/CatalogJobEntity.java") {
		t.Fatal("wrong-project duplicate model displaced provider evidence")
	}
	if pack.EstimatedTokens > DefaultContextBudgetTokens ||
		len(pack.SourceSections) > MaxContextSourceSections ||
		contextSourceFileCount(pack) > DefaultContextMaxFiles {
		t.Fatalf(
			"release-quality pack exceeds limits: tokens=%d sections=%d aggregate_files=%d",
			pack.EstimatedTokens,
			len(pack.SourceSections),
			contextSourceFileCount(pack),
		)
	}
}

func TestBuildContextBalancesBroadReleaseEvidence(t *testing.T) {
	root := writeReleaseQualityMissingContractFixture(t)
	const budgetTokens = 4000
	const maxFiles = 12

	var (
		firstBuild []byte
		pack       ContextPack
	)
	for build := 0; build < 3; build++ {
		candidate, err := BuildContext(ContextRequest{
			Root: root, Query: broadReleaseQualityQuery,
			BudgetTokens: budgetTokens, MaxFiles: maxFiles,
		})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if build == 0 {
			firstBuild = encoded
			pack = candidate
		} else if string(encoded) != string(firstBuild) {
			t.Fatalf("build %d changed the context pack:\nfirst: %s\nnext:  %s", build+1, firstBuild, encoded)
		}
	}

	for _, want := range []struct {
		project string
		path    string
	}{
		{project: "libraries/job-client", path: "src/main/java/example/JobClient.java"},
		{project: "libraries/job-client", path: "src/main/java/example/JobClientConfig.java"},
		{project: "libraries/job-client", path: "src/main/java/example/JobClientAuth.java"},
		{project: "services/catalog", path: "src/main/resources/application.yml"},
		{project: "services/catalog", path: "src/test/resources/application-test.yml"},
		{project: "services/jobs", path: "src/main/java/example/BaseCatalogJobEntity.java"},
		{project: "services/jobs", path: "src/main/java/example/JobManagementController.java"},
		{project: "services/jobs", path: "src/main/java/example/JobSecurity.java"},
		{project: "services/jobs", path: "src/main/java/example/CatalogJobRepository.java"},
		{project: "services/jobs", path: "src/main/java/example/CatalogChangeJobRepository.java"},
		{project: "services/jobs", path: "src/test/java/example/JobManagementControllerTest.java"},
		{project: "services/jobs", path: "src/test/java/example/JobServiceTest.java"},
	} {
		if !contextPackRepresentsSourcePath(pack, want.project, want.path) {
			t.Errorf("required broad release evidence %q missing", want.project+":"+want.path)
		}
	}
	if contextPackRepresentsSourcePath(
		pack,
		"services/catalog",
		"src/main/java/example/CatalogJobEntity.java",
	) {
		t.Error("wrong-project duplicate model displaced provider evidence")
	}
	for _, sentinel := range []string{
		"https://jobs.invalid",
		"fixture-client-user",
		"fixture-client-password",
		"https://jobs-test.invalid",
		"fixture-test-user",
		"fixture-test-password",
	} {
		if strings.Contains(string(firstBuild), sentinel) {
			t.Errorf("sentinel configuration value %q leaked into final context", sentinel)
		}
	}
	if pack.BudgetTokens != budgetTokens ||
		contextSourceFileCount(pack) != maxFiles ||
		len(pack.SourceSections) != maxFiles ||
		len(pack.SourceOmissions) != 3 {
		t.Errorf(
			"broad release bounds = tokens %d/%d aggregate_files %d/%d sections %d/%d omissions %d/3",
			pack.BudgetTokens,
			budgetTokens,
			contextSourceFileCount(pack),
			maxFiles,
			len(pack.SourceSections),
			maxFiles,
			len(pack.SourceOmissions),
		)
	}
}

func TestBuildContextBalancesNaturalProductionAndTestFilePlan(t *testing.T) {
	root := writeReleaseQualityMissingContractFixture(t)
	query := "When DELETE /catalog/items/{itemId} removes an item in services/catalog, " +
		"analyze the required cross-service cleanup through libraries/job-client and services/jobs. " +
		"Cover authentication, configuration, retries, persistence, side effects, and identify " +
		"the production and test files to change or create."
	pack, err := BuildContext(ContextRequest{
		Root: root, Query: query, BudgetTokens: 4000, MaxFiles: 12,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct {
		project string
		path    string
	}{
		{project: "libraries/job-client", path: "src/main/java/example/JobClientConfig.java"},
		{project: "libraries/job-client", path: "src/main/java/example/JobClientAuth.java"},
		{project: "services/catalog", path: "src/main/resources/application.yml"},
		{project: "services/catalog", path: "src/test/resources/application-test.yml"},
		{project: "services/jobs", path: "src/main/java/example/JobManagementController.java"},
		{project: "services/jobs", path: "src/main/java/example/JobSecurity.java"},
		{project: "services/jobs", path: "src/test/java/example/JobManagementControllerTest.java"},
		{project: "services/jobs", path: "src/test/java/example/JobServiceTest.java"},
	} {
		if !contextPackRepresentsSourcePath(pack, want.project, want.path) {
			t.Errorf("natural file-plan evidence %q missing", want.project+":"+want.path)
		}
	}
	encoded, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{
		"https://jobs.invalid",
		"fixture-client-user",
		"fixture-client-password",
		"https://jobs-test.invalid",
		"fixture-test-user",
		"fixture-test-password",
	} {
		if strings.Contains(string(encoded), sentinel) {
			t.Errorf("sentinel configuration value %q leaked into natural file plan", sentinel)
		}
	}
}

func TestBuildContextKeepsCoherentReleasePlanEvidence(t *testing.T) {
	root := writeReleaseQualityMissingContractFixtureWithIndex(
		t,
		runtimeShapeReleaseQualityMissingContractIndex(),
	)
	query := "When DELETE /catalog/items/{itemId} removes an item in services/catalog, " +
		"analyze the required cross-service cleanup through libraries/job-client and services/jobs. " +
		"Cover authentication, configuration, retries, persistence, side effects, and identify " +
		"the production and test files to change or create."
	pack, err := BuildContext(ContextRequest{
		Root: root, Query: query, BudgetTokens: 4000, MaxFiles: 12,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct {
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
		{project: "services/jobs", path: "src/test/java/example/JobManagementControllerTest.java"},
		{project: "services/jobs", path: "src/test/java/example/JobServiceTest.java"},
	} {
		if !contextPackRepresentsSourcePath(pack, want.project, want.path) {
			t.Errorf("runtime-shaped release evidence %q missing", want.project+":"+want.path)
		}
	}
	foundAuthenticationConfiguration := false
	for _, section := range pack.SourceSections {
		if normalizeContextProject(section.Project) != "services/jobs" ||
			contextPackSourceFile(section.Path) != "src/main/resources/application.properties" {
			continue
		}
		if section.StartLine != 2 || !strings.Contains(section.Content, "technical-user") {
			t.Fatalf("jobs authentication configuration section = lines %d-%d %q, want technical-user on line 2", section.StartLine, section.EndLine, section.Content)
		}
		if strings.Contains(section.Content, "task-isbns") {
			t.Fatalf("jobs authentication configuration includes unrelated task-isbns property: %q", section.Content)
		}
		foundAuthenticationConfiguration = true
		break
	}
	if !foundAuthenticationConfiguration {
		t.Fatal("jobs authentication configuration was not rendered as a source section")
	}
	if pack.EstimatedTokens > 4000 ||
		contextSourceFileCount(pack) > 12 ||
		len(pack.SourceSections) > 12 ||
		len(pack.SourceOmissions) > MaxContextSourceOmissions {
		t.Fatalf(
			"runtime-shaped release pack exceeds bounds: tokens=%d aggregate_files=%d sections=%d omissions=%d",
			pack.EstimatedTokens,
			contextSourceFileCount(pack),
			len(pack.SourceSections),
			len(pack.SourceOmissions),
		)
	}
}

func contextPackRepresentsSourcePath(pack ContextPack, project, path string) bool {
	project = normalizeContextProject(project)
	path = contextPackSourceFile(path)
	matches := func(actualProject, actualPath string) bool {
		return normalizeContextProject(actualProject) == project &&
			contextPackSourceFile(actualPath) == path
	}
	for _, section := range pack.SourceSections {
		if matches(section.Project, section.Path) {
			return true
		}
	}
	for _, file := range pack.Files {
		if matches(file.Project, file.Path) {
			return true
		}
	}
	for _, omission := range pack.SourceOmissions {
		if matches(omission.Project, omission.Path) &&
			omission.StartLine > 0 && omission.EndLine >= omission.StartLine {
			return true
		}
	}
	return false
}

func TestCompileContextPackKeepsTestForAcceptedRelatedProvider(t *testing.T) {
	pack := compileMissingContractRankPack(
		t,
		missingContractRankIndexWithProviderTests(),
		missingContractEnglishQuery,
		DefaultContextBudgetTokens,
		DefaultContextMaxFiles,
	)

	testIDs := map[string]int{}
	for _, test := range pack.Tests {
		testIDs[test.ID]++
	}
	if !reflect.DeepEqual(testIDs, map[string]int{
		"catalog-test": 1,
		"jobs-test":    1,
	}) {
		t.Fatalf("primary and related provider tests = %#v", pack.Tests)
	}
	testTargets := map[string]int{}
	for _, relationship := range pack.CallChain {
		if relationship.Kind == "test_target" {
			testTargets[relationship.From+" -> "+relationship.To]++
		}
		if relationship.From == "CatalogOperations.deleteItem" &&
			strings.Contains(relationship.To, "Job") {
			t.Fatalf("fabricated future relationship: %#v", relationship)
		}
	}
	if !reflect.DeepEqual(testTargets, map[string]int{
		"CatalogControllerTest.deletesItem -> DELETE /catalog/items/{itemId}": 1,
		"JobManagementControllerTest.listJobs -> GET /job-management/jobs":    1,
	}) {
		t.Fatalf("observed test targets = %#v", pack.CallChain)
	}
	selected := contextSelectedFactSet(pack)
	for _, factID := range []string{"catalog-test", "jobs-route", "jobs-test"} {
		if !selected[factID] {
			t.Errorf("required rank evidence %q not selected", factID)
		}
	}
	selectedEdges := map[string]bool{}
	for _, edgeID := range pack.selectedEdgeIDs {
		selectedEdges[edgeID] = true
	}
	for _, edgeID := range []string{"current-test", "adjacent-test"} {
		if !selectedEdges[edgeID] {
			t.Errorf("required observed edge %q not selected: %#v", edgeID, pack.selectedEdgeIDs)
		}
	}
	filePaths := map[string]bool{}
	for _, file := range pack.Files {
		filePaths[file.Path] = true
	}
	for _, path := range []string{
		"src/test/java/example/CatalogControllerTest.java",
		"src/test/java/example/JobManagementControllerTest.java",
	} {
		if !filePaths[path] {
			t.Errorf("test file %q missing from %#v", path, pack.Files)
		}
	}
}

func TestBuildContextRetainsRelatedProviderTestWithinCrossProjectMetadataBudget(t *testing.T) {
	root := writeMissingContractContextIndexFixture(t, missingContractRankIndexWithProviderTests())
	query := "When DELETE /catalog/items/{itemId} removes an item in services/catalog, " +
		"plan cleanup of related jobs through the current job client and services/jobs. " +
		"Cover the current path, missing HTTP contract, task types and lookup attributes, " +
		"authentication, configuration, retry behavior, persistence, side effects, and tests."
	pack, err := BuildContext(ContextRequest{
		Root:         root,
		Query:        query,
		BudgetTokens: DefaultContextBudgetTokens,
		MaxFiles:     DefaultContextMaxFiles,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !contextSelectedFactSet(pack)["jobs-test"] {
		t.Fatalf("related provider test missing from metadata: %#v", pack.Tests)
	}
	for _, relationship := range pack.CallChain {
		if relationship.From == "CatalogOperations.deleteItem" &&
			strings.Contains(relationship.To, "Job") {
			t.Fatalf("fabricated future relationship: %#v", relationship)
		}
	}
	if pack.EstimatedTokens > DefaultContextBudgetTokens {
		t.Fatalf("final pack exceeded budget: %d", pack.EstimatedTokens)
	}
}

func TestCompileContextPackRelatedProviderTestFailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		mutate func(*scan.AgentContextIndexRecord)
	}{
		{
			name:  "query does not request tests",
			query: strings.Replace(missingContractEnglishQuery, ", and tests.", ".", 1),
		},
		{
			name: "lexical similarity without edge",
			mutate: func(index *scan.AgentContextIndexRecord) {
				index.Edges = removeMissingContractEdge(index.Edges, "adjacent-test")
			},
		},
		{
			name: "cross-project test",
			mutate: func(index *scan.AgentContextIndexRecord) {
				missingContractFactByID(index.Facts, "jobs-test").Project = "services/audit"
			},
		},
		{
			name: "missing test source",
			mutate: func(index *scan.AgentContextIndexRecord) {
				missingContractFactByID(index.Facts, "jobs-test").File = ""
			},
		},
		{
			name: "non-test source fact",
			mutate: func(index *scan.AgentContextIndexRecord) {
				missingContractFactByID(index.Facts, "jobs-test").Kind = "symbol"
			},
		},
		{
			name: "unresolved edge",
			mutate: func(index *scan.AgentContextIndexRecord) {
				missingContractEdgeByID(index.Edges, "adjacent-test").Confidence = "PARTIAL"
			},
		},
		{
			name: "blank provider confidence",
			mutate: func(index *scan.AgentContextIndexRecord) {
				missingContractFactByID(index.Facts, "jobs-route").Confidence = ""
			},
		},
		{
			name: "partial provider confidence",
			mutate: func(index *scan.AgentContextIndexRecord) {
				missingContractFactByID(index.Facts, "jobs-route").Confidence = "PARTIAL"
			},
		},
		{
			name: "matched provider confidence",
			mutate: func(index *scan.AgentContextIndexRecord) {
				missingContractFactByID(index.Facts, "jobs-route").Confidence = "MATCHED"
			},
		},
		{
			name: "same test targets multiple production facts",
			mutate: func(index *scan.AgentContextIndexRecord) {
				index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
					ID: "ambiguous-test", FromFactID: "jobs-test", ToFactID: "jobs-service",
					Kind: "test_target", Confidence: "EXTRACTED",
				})
			},
		},
		{
			name: "same test targets reliable production in another project",
			mutate: func(index *scan.AgentContextIndexRecord) {
				index.Facts = append(index.Facts, scan.AgentContextFactRecord{
					ID: "audit-route", Project: "services/audit", Kind: "route",
					Name: "GET /audit/jobs", Qualified: "AuditJobController.listJobs",
					HTTPMethod: "GET", Path: "/audit/jobs",
					File: "src/main/java/example/AuditJobController.java",
					Line: 10, Confidence: "EXACT",
				})
				index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
					ID: "cross-project-test", FromFactID: "jobs-test", ToFactID: "audit-route",
					Kind: "test_target", Confidence: "EXTRACTED",
				})
			},
		},
		{
			name: "same qualified handler has multiple method lines",
			mutate: func(index *scan.AgentContextIndexRecord) {
				overload := *missingContractFactByID(index.Facts, "jobs-route-symbol")
				overload.ID = "jobs-route-overload"
				overload.Line += 6
				index.Facts = append(index.Facts, overload)
				index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
					ID: "overloaded-test", FromFactID: "jobs-test", ToFactID: overload.ID,
					Kind: "test_target", Confidence: "EXTRACTED",
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := missingContractRankIndexWithProviderTests()
			if test.mutate != nil {
				test.mutate(&index)
			}
			query := test.query
			if query == "" {
				query = missingContractEnglishQuery
			}
			pack := compileMissingContractRankPack(
				t,
				index,
				query,
				DefaultContextBudgetTokens,
				DefaultContextMaxFiles,
			)
			for _, selected := range pack.Tests {
				if selected.ID == "jobs-test" {
					t.Fatalf("ineligible related provider test was selected: %#v", pack)
				}
			}
		})
	}
}

func TestCompileContextPackRelatedProviderTestIsDeterministicAndBoundedPerProvider(t *testing.T) {
	index := missingContractRankIndexWithProviderTests()
	second := *missingContractFactByID(index.Facts, "jobs-test")
	second.ID = "jobs-test-z"
	second.Name = "listJobsAlternative"
	second.Qualified = "JobManagementControllerTest.listJobsAlternative"
	index.Facts = append(index.Facts, second)
	index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
		ID: "adjacent-test-z", FromFactID: second.ID, ToFactID: "jobs-route",
		Kind: "test_target", Confidence: "EXTRACTED",
	})
	reversed := index
	reversed.Facts = append([]scan.AgentContextFactRecord(nil), index.Facts...)
	reversed.Edges = append([]scan.AgentContextEdgeRecord(nil), index.Edges...)
	slices.Reverse(reversed.Facts)
	slices.Reverse(reversed.Edges)

	build := func(index scan.AgentContextIndexRecord) ContextPack {
		t.Helper()
		return compileMissingContractRankPack(
			t,
			index,
			missingContractEnglishQuery,
			DefaultContextBudgetTokens,
			DefaultContextMaxFiles,
		)
	}
	forward := build(index)
	backward := build(reversed)
	for _, pack := range []ContextPack{forward, backward} {
		relatedTests := 0
		for _, selected := range pack.Tests {
			if normalizeContextProject(selected.Project) == "services/jobs" {
				relatedTests++
				if selected.ID != "jobs-test" {
					t.Fatalf("deterministic related test = %#v, want jobs-test", selected)
				}
			}
		}
		if relatedTests != 1 {
			t.Fatalf("related provider tests = %#v, want exactly one", pack.Tests)
		}
	}
	if !reflect.DeepEqual(forward.Tests, backward.Tests) ||
		!reflect.DeepEqual(forward.CallChain, backward.CallChain) ||
		!reflect.DeepEqual(forward.Files, backward.Files) ||
		!reflect.DeepEqual(forward.selectedFactIDs, backward.selectedFactIDs) ||
		!reflect.DeepEqual(forward.selectedEdgeIDs, backward.selectedEdgeIDs) ||
		forward.ContextID != backward.ContextID {
		t.Fatalf("related provider test selection changed with index order:\nforward: %#v\nbackward: %#v", forward, backward)
	}
}

func TestRelatedProviderTestTargetRequiresProviderKind(t *testing.T) {
	tests := []struct {
		name string
		fact scan.AgentContextFactRecord
		want bool
	}{
		{
			name: "route",
			fact: scan.AgentContextFactRecord{
				ID: "route", Project: "services/jobs", Kind: "route",
				File: "JobController.java", Confidence: "EXACT",
			},
			want: true,
		},
		{
			name: "http symbol",
			fact: scan.AgentContextFactRecord{
				ID: "handler", Project: "services/jobs", Kind: "symbol",
				HTTPMethod: "GET", Path: "/jobs", File: "JobController.java", Confidence: "EXACT",
			},
			want: true,
		},
		{
			name: "service symbol",
			fact: scan.AgentContextFactRecord{
				ID: "service", Project: "services/jobs", Kind: "symbol",
				File: "JobService.java", Confidence: "EXACT",
			},
		},
		{
			name: "persistence",
			fact: scan.AgentContextFactRecord{
				ID: "repository", Project: "services/jobs", Kind: "persistence",
				File: "JobRepository.java", Confidence: "EXACT",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := reliableRelatedProviderTestTarget(test.fact); got != test.want {
				t.Fatalf("reliableRelatedProviderTestTarget() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCompileContextPackRejectsRelatedProviderTestAtomicallyAtBoundaries(t *testing.T) {
	t.Run("max files", func(t *testing.T) {
		index := missingContractRankIndexWithProviderTests()
		baseline := index
		baseline.Edges = removeMissingContractEdge(baseline.Edges, "adjacent-test")
		for maxFiles := MinContextMaxFiles; maxFiles < DefaultContextMaxFiles; maxFiles++ {
			want, wantErr := tryCompileMissingContractRankPack(
				baseline,
				missingContractEnglishQuery,
				DefaultContextBudgetTokens,
				maxFiles,
			)
			got, gotErr := tryCompileMissingContractRankPack(
				index,
				missingContractEnglishQuery,
				DefaultContextBudgetTokens,
				maxFiles,
			)
			if wantErr == nil &&
				gotErr == nil &&
				contextSelectedFactSet(want)["jobs-route"] &&
				!contextSelectedFactSet(got)["jobs-test"] &&
				reflect.DeepEqual(got, want) {
				return
			}
		}
		t.Fatal("no MaxFiles boundary rejected the related provider test atomically")
	})

	t.Run("support cap", func(t *testing.T) {
		index := missingContractRankIndexWithProviderTests()
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: "jobs-configuration", Project: "services/jobs", Kind: "configuration",
			Name: "jobConfiguration", Qualified: "JobConfiguration.jobConfiguration",
			File: "src/main/java/example/JobConfiguration.java", Line: 8, EndLine: 12,
			Confidence: "EXACT", Search: "job task configuration",
		})
		baseline := index
		baseline.Edges = removeMissingContractEdge(baseline.Edges, "adjacent-test")
		want := compileMissingContractRankPack(
			t,
			baseline,
			missingContractEnglishQuery,
			DefaultContextBudgetTokens,
			DefaultContextMaxFiles,
		)
		got := compileMissingContractRankPack(
			t,
			index,
			missingContractEnglishQuery,
			DefaultContextBudgetTokens,
			DefaultContextMaxFiles,
		)
		if !contextSelectedFactSet(want)["jobs-configuration"] {
			t.Fatalf("support-cap fixture did not saturate selected support: %#v", want.selectedFactIDs)
		}
		assertRelatedProviderTestRejectedAtomically(t, got, want)
	})

	t.Run("token budget", func(t *testing.T) {
		index := missingContractRankIndexWithProviderTests()
		baseline := index
		baseline.Edges = removeMissingContractEdge(baseline.Edges, "adjacent-test")
		const budget = 1028
		want := compileMissingContractRankPack(
			t,
			baseline,
			missingContractEnglishQuery,
			budget,
			DefaultContextMaxFiles,
		)
		got := compileMissingContractRankPack(
			t,
			index,
			missingContractEnglishQuery,
			budget,
			DefaultContextMaxFiles,
		)
		if !contextSelectedFactSet(want)["jobs-route"] {
			t.Fatalf("token-boundary fixture lost the accepted provider: %#v", want.selectedFactIDs)
		}
		assertRelatedProviderTestRejectedAtomically(t, got, want)
	})
}

func assertRelatedProviderTestRejectedAtomically(
	t *testing.T,
	got ContextPack,
	want ContextPack,
) {
	t.Helper()
	if contextSelectedFactSet(got)["jobs-test"] {
		t.Fatalf("related provider test crossed a saturated boundary: %#v", got.Tests)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rejected related provider test changed the existing pack:\ngot:  %#v\nwant: %#v", got, want)
	}
}

func missingContractRankIndexWithProviderTests() scan.AgentContextIndexRecord {
	index := missingContractContextIndex()
	providerAlias := *missingContractFactByID(index.Facts, "jobs-route")
	providerAlias.ID = "jobs-route-symbol"
	providerAlias.Kind = "symbol"
	providerAlias.Name = "listJobs"
	providerAlias.Line += 4
	providerAlias.HTTPMethod = ""
	providerAlias.Path = ""
	index.Facts = append(index.Facts, scan.AgentContextFactRecord{
		ID: "catalog-test", Project: "services/catalog", Kind: "test",
		Name: "deletesItem", Qualified: "CatalogControllerTest.deletesItem",
		File: "src/test/java/example/CatalogControllerTest.java",
		Line: 10, EndLine: 18, Confidence: "EXACT", Search: "delete catalog item test",
	}, providerAlias)
	index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
		ID: "current-test", FromFactID: "catalog-test", ToFactID: "catalog-route",
		Kind: "test_target", Confidence: "EXACT",
	}, scan.AgentContextEdgeRecord{
		ID: "adjacent-test-alias", FromFactID: "jobs-test", ToFactID: providerAlias.ID,
		Kind: "test_target", Confidence: "EXTRACTED",
	})
	missingContractFactByID(index.Facts, "jobs-test").Confidence = "EXTRACTED"
	missingContractEdgeByID(index.Edges, "adjacent-test").Confidence = "EXTRACTED"
	return index
}

func compileMissingContractRankPack(
	t *testing.T,
	index scan.AgentContextIndexRecord,
	query string,
	budgetTokens int,
	maxFiles int,
) ContextPack {
	t.Helper()
	pack, err := tryCompileMissingContractRankPack(index, query, budgetTokens, maxFiles)
	if err != nil {
		t.Fatal(err)
	}
	return pack
}

func tryCompileMissingContractRankPack(
	index scan.AgentContextIndexRecord,
	query string,
	budgetTokens int,
	maxFiles int,
) (ContextPack, error) {
	request, err := normalizeContextRequest(ContextRequest{
		Query: query, BudgetTokens: budgetTokens, MaxFiles: maxFiles,
	})
	if err != nil {
		return ContextPack{}, err
	}
	pack, err := compileContextPack(index, request)
	if err != nil {
		return ContextPack{}, err
	}
	pack.ContextID = contextIdentity(
		pack.Freshness,
		pack.selectedFactIDs,
		pack.selectedEdgeIDs,
		pack.selectedConcernKeys,
	)
	return pack, nil
}

func missingContractFactByID(
	facts []scan.AgentContextFactRecord,
	id string,
) *scan.AgentContextFactRecord {
	for index := range facts {
		if facts[index].ID == id {
			return &facts[index]
		}
	}
	return &scan.AgentContextFactRecord{}
}

func missingContractEdgeByID(
	edges []scan.AgentContextEdgeRecord,
	id string,
) *scan.AgentContextEdgeRecord {
	for index := range edges {
		if edges[index].ID == id {
			return &edges[index]
		}
	}
	return &scan.AgentContextEdgeRecord{}
}

func removeMissingContractEdge(
	edges []scan.AgentContextEdgeRecord,
	id string,
) []scan.AgentContextEdgeRecord {
	result := make([]scan.AgentContextEdgeRecord, 0, len(edges))
	for _, edge := range edges {
		if edge.ID != id {
			result = append(result, edge)
		}
	}
	return result
}

func TestBuildContextReportsMissingSideEffectFacetsWithoutRerankingEndpoint(t *testing.T) {
	root := writeMissingContractContextFixture(t)
	query := "When DELETE /catalog/items/{itemId} removes an item in services/catalog, " +
		"plan cleanup of related jobs through libraries/job-client and services/jobs. " +
		"Cover task types and lookup attributes plus mail, audit, and user information separately."
	pack, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}

	if len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "catalog-route" {
		t.Fatalf("evidence planning changed primary entrypoint: %#v", pack.Entrypoints)
	}
	if pack.SourceCoverage == "complete" {
		t.Fatalf("generic side-effect source covered three requested facets: %#v", pack.Concerns)
	}
	var sideEffectOmission *ContextSourceOmission
	for index := range pack.SourceOmissions {
		if strings.HasSuffix(pack.SourceOmissions[index].Path, "JobHousekeeping.java") {
			sideEffectOmission = &pack.SourceOmissions[index]
			break
		}
	}
	if sideEffectOmission == nil {
		t.Fatalf("missing side-effect evidence lacks targeted omission: %#v", pack.SourceOmissions)
	}
	for _, facet := range []string{"mail", "audit", "user_information"} {
		if !strings.Contains(sideEffectOmission.Reason, facet) {
			t.Errorf("coalesced omission lacks %q: %#v", facet, sideEffectOmission)
		}
	}
	if pack.EstimatedTokens > pack.BudgetTokens || pack.FallbackRequired || pack.RetryAllowed {
		t.Fatalf("bounded one-shot contract changed: %#v", pack)
	}
}

func TestMissingContractChangeAnalysisEnglishGermanParity(t *testing.T) {
	root := writeMissingContractContextFixture(t)
	english, err := BuildContext(ContextRequest{Root: root, Query: missingContractEnglishQuery})
	if err != nil {
		t.Fatal(err)
	}
	german, err := BuildContext(ContextRequest{Root: root, Query: missingContractGermanQuery})
	if err != nil {
		t.Fatal(err)
	}

	englishSnapshot := missingContractContextSnapshotForPack(english)
	germanSnapshot := missingContractContextSnapshotForPack(german)
	if !reflect.DeepEqual(germanSnapshot, englishSnapshot) {
		t.Fatalf("German context diverged:\nGerman:  %#v\nEnglish: %#v", germanSnapshot, englishSnapshot)
	}
}

func TestSourceConcernCandidatesRequireDomainEvidence(t *testing.T) {
	queryTokens := contextExpandedTokenSet(
		"cadaster task configuration persistence side effects mail retry",
	)
	generic := scan.AgentContextFactRecord{
		ID: "generic", Project: "libraries/client", Kind: "symbol",
		Name: "MailProperties", Search: "mail properties",
	}
	taskConfig := scan.AgentContextFactRecord{
		ID: "task-config", Project: "libraries/client", Kind: "symbol",
		Name: "CadasterTaskMgmtConfig", Search: "cadaster task configuration",
	}

	domainTokens := contextConcernDomainQueryTokens(queryTokens)
	if contextSourceFactMatchesDomain(generic, domainTokens) ||
		!contextSourceFactMatchesDomain(taskConfig, domainTokens) {
		t.Fatalf("domain matching accepted generic source or rejected task config")
	}
}

func TestExplicitProjectConcernCandidatesRequireDomainIdentity(t *testing.T) {
	queryTokens := contextExpandedTokenSet(
		"libraries/common job configuration side effects mail",
	)
	facts := []scan.AgentContextFactRecord{
		{
			ID: "generic-mail", Project: "libraries/common", Kind: "configuration",
			Name: "MailProperties", Search: "common mail configuration",
		},
		{
			ID: "job-mail", Project: "libraries/common", Kind: "symbol",
			Name: "JobHousekeeping", Search: "common job mail side effect",
		},
	}

	candidates := contextExplicitProjectConcernCandidates(
		queryTokens,
		"libraries/common",
		contextConcernSideEffects,
		facts,
		map[string]bool{},
	)
	if !reflect.DeepEqual(candidates, []string{"job-mail"}) {
		t.Fatalf("side-effect candidates = %v, want domain-specific evidence", candidates)
	}
}

func TestSourceAnchorTokensIncludeSelectedEndpoint(t *testing.T) {
	pack := ContextPack{Endpoints: []ContextEndpoint{{
		HTTPMethod: "DELETE",
		Path:       "/cadasters/{cadasterId}/regulations/{objectId}",
		Handler:    "CadasterRegulationController.deleteFromCadaster",
	}}}

	anchors := contextSourceAnchorTokens(pack, nil)
	for _, want := range []string{"cadaster", "regulation", "object"} {
		if !anchors[want] {
			t.Fatalf("selected endpoint anchor %q missing from %#v", want, anchors)
		}
	}
}

func TestSourceConcernScoreDoesNotTreatSearchTextAsAnchorIdentity(t *testing.T) {
	concern := newContextConcern(contextConcernPersistence, "services/tasks", true, nil, "")
	anchors := contextExpandedTokenSet("cadaster regulation object")
	relevant := scan.AgentContextFactRecord{
		Project: "services/tasks", Kind: "persistence",
		Name: "findByCadasterIdAndObjectId", Qualified: "TaskRepository.findByCadasterIdAndObjectId",
		File: "TaskRepository.java", Search: "persistence", Confidence: "EXACT",
	}
	searchOnly := scan.AgentContextFactRecord{
		Project: "services/tasks", Kind: "persistence",
		Name: "findByCadasterIdAndUserId", Qualified: "CadasterRepository.findByCadasterIdAndUserId",
		File: "CadasterRepository.java", Search: "persistence cadaster regulation object", Confidence: "EXACT",
	}

	relevantScore := contextSourceConcernFactScore(relevant, concern, "persistence", anchors)
	searchOnlyScore := contextSourceConcernFactScore(searchOnly, concern, "persistence", anchors)
	if relevantScore <= searchOnlyScore {
		t.Fatalf("identity score %d did not beat search-only score %d", relevantScore, searchOnlyScore)
	}
}

func TestSourceConcernScoreIgnoresGenericMissingCallGuard(t *testing.T) {
	facts := jobClientResilienceSourceFacts()
	concern := newContextConcern(
		contextConcernResilience,
		"libraries/job-client",
		true,
		[]string{"client-retry", "client-call"},
		"requested retry policy",
	)
	const query = "Provide adjacent client retry evidence."
	for _, candidateQuery := range []string{
		query,
		query + " Do not invent the missing call or route.",
	} {
		if got := highestSourceConcernFact(facts, concern, candidateQuery, nil); got.ID != "client-retry" {
			t.Fatalf("highest resilience fact for %q = %q, want client-retry", candidateQuery, got.ID)
		}
	}
}

func TestSourceConcernScoreKeepsEnglishGermanRetryRangeAligned(t *testing.T) {
	facts := jobClientResilienceSourceFacts()
	concern := newContextConcern(
		contextConcernResilience,
		"libraries/job-client",
		true,
		[]string{"client-retry", "client-call"},
		"requested retry policy",
	)
	const source = `package example;

import org.springframework.retry.annotation.Backoff;
import org.springframework.retry.annotation.Retryable;

final class JobClientRetry {
  @Retryable(retryFor = JobClientException.class, maxAttempts = 3, backoff = @Backoff(delay = 100))
  <T> T execute(JobClientCall<T> call) {
    return call.invoke();
  }
}

interface JobClientCall<T> {
  T invoke();
}

final class JobClientException extends RuntimeException {}
`
	queries := map[string]string{
		"English": "Plan the smallest production change that removes jobs when a catalog item is deleted. Show the current public deletion path, prove that the future job deletion contract is absent, and provide adjacent client configuration, authentication, retry, provider persistence, side-effect, and test evidence. Do not invent the missing call or route.",
		"German":  "Plane die kleinste produktionsreife Änderung, durch die beim Löschen eines Katalogeintrags auch die zugehörigen Aufgaben entfernt werden. Zeige den aktuellen öffentlichen Löschpfad, belege das Fehlen des zukünftigen Aufgaben-Löschvertrags und liefere angrenzende Belege zu Client-Konfiguration, Authentifizierung, Retry, Provider-Persistenz, Nebenwirkungen und Tests. Erfinde weder den fehlenden Aufruf noch die fehlende Route.",
	}
	index := scan.AgentContextIndexRecord{Facts: append(
		slices.Clone(facts),
		scan.AgentContextFactRecord{ID: "catalog", Project: "services/catalog"},
		scan.AgentContextFactRecord{ID: "jobs", Project: "services/jobs"},
	)}
	for language, query := range queries {
		t.Run(language, func(t *testing.T) {
			candidates := contextSourceCandidatesForConcerns(
				ContextPack{Query: query, selectionQuery: query},
				index,
				[]contextConcern{concern},
			)
			if len(candidates) != 1 || candidates[0].FactID != "client-retry" {
				t.Fatalf("resilience source candidates = %#v, want client-retry", candidates)
			}
			section, err := renderSourceCandidate(
				candidates[0],
				sourceFile{Path: candidates[0].Path, Lines: strings.Split(source, "\n")},
				"declaration_body",
			)
			if err != nil {
				t.Fatal(err)
			}
			if section.StartLine != 6 || section.EndLine != 11 {
				t.Fatalf("resilience declaration range = %d-%d, want 6-11", section.StartLine, section.EndLine)
			}
		})
	}
}

func TestSourceConcernCallAnchorRemainsEffectiveAndRenderable(t *testing.T) {
	facts := jobClientResilienceSourceFacts()
	call := facts[1]
	concern := newContextConcern(
		contextConcernResilience,
		"libraries/job-client",
		true,
		[]string{call.ID},
		"requested retry policy",
	)
	const query = "Provide resilience evidence."
	unanchoredScore := contextSourceConcernFactScore(call, concern, query, nil)
	anchoredScore := contextSourceConcernFactScore(call, concern, query, map[string]bool{"call": true})
	if anchoredScore <= unanchoredScore {
		t.Fatalf("call anchor score = %d, want greater than %d", anchoredScore, unanchoredScore)
	}

	const source = `final class JobClientRetry {}

interface JobClientCall<T> {
  T invoke();
}`
	section, err := renderSourceCandidate(
		sourceCandidate{
			FactID: call.ID, FactIDs: []string{call.ID},
			Project: call.Project, Path: call.File,
			StartLine: 3, EndLine: 5,
			Kind: call.Kind, Name: call.Name, Qualified: call.Qualified,
		},
		sourceFile{Path: call.File, Lines: strings.Split(source, "\n")},
		"declaration_body",
	)
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 3 || section.EndLine != 5 {
		t.Fatalf("anchored call declaration range = %d-%d, want 3-5", section.StartLine, section.EndLine)
	}
}

func TestSourceConcernProjectDomainTokensPreserveSemanticQueryTokens(t *testing.T) {
	const query = "client call calls"
	semanticTokens := contextSourceConcernSemanticQueryTokens(query)
	wantSemanticTokens := map[string]bool{"client": true}
	fact := scan.AgentContextFactRecord{
		ID: "client", Project: "libraries/client", Kind: "api_contract",
		Name: "JobClient", Qualified: "example.JobClient",
		File: "JobClient.java", Confidence: "EXACT",
	}
	concern := newContextConcern(
		contextConcernHTTPContract,
		"libraries/client",
		true,
		[]string{fact.ID},
		"requested client contract",
	)
	scoreBefore := contextSourceConcernFactScoreWithTokensAndIndex(
		fact,
		concern,
		query,
		semanticTokens,
		nil,
		scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{fact}},
	)

	domainTokens := contextSourceConcernProjectDomainQueryTokens(
		semanticTokens,
		map[string][]string{"libraries/client": {"client", "libraries/client"}},
		map[string]bool{"libraries/client": true},
	)
	if len(domainTokens) != 0 {
		t.Fatalf("project domain tokens = %#v, want empty", domainTokens)
	}
	if !reflect.DeepEqual(semanticTokens, wantSemanticTokens) {
		t.Fatalf("semantic query tokens mutated to %#v, want %#v", semanticTokens, wantSemanticTokens)
	}
	scoreAfter := contextSourceConcernFactScoreWithTokensAndIndex(
		fact,
		concern,
		query,
		semanticTokens,
		nil,
		scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{fact}},
	)
	if scoreAfter != scoreBefore {
		t.Fatalf("source concern score changed from %d to %d", scoreBefore, scoreAfter)
	}
}

func jobClientResilienceSourceFacts() []scan.AgentContextFactRecord {
	return []scan.AgentContextFactRecord{
		{
			ID: "client-retry", Project: "libraries/job-client", Kind: "symbol",
			Name: "JobClientRetry", Qualified: "example.JobClientRetry",
			File: "JobClientRetry.java", Line: 6, EndLine: 11, Confidence: "EXACT",
			Search: "JobClientRetry Job Client Retry example.JobClientRetry",
		},
		{
			ID: "client-call", Project: "libraries/job-client", Kind: "symbol",
			Name: "JobClientCall", Qualified: "example.JobClientCall",
			File: "JobClientRetry.java", Line: 13, EndLine: 15, Confidence: "EXACT",
			Search: "JobClientCall Job Client Call example.JobClientCall",
		},
	}
}

func highestSourceConcernFact(
	facts []scan.AgentContextFactRecord,
	concern contextConcern,
	query string,
	anchorTokens map[string]bool,
) scan.AgentContextFactRecord {
	ranked := slices.Clone(facts)
	sort.Slice(ranked, func(left, right int) bool {
		return contextSourceConcernFactLess(
			ranked[left],
			ranked[right],
			concern,
			query,
			anchorTokens,
			scan.AgentContextIndexRecord{Facts: facts},
		)
	})
	return ranked[0]
}

func TestPublicConfigurationConcernPrefersDomainConfigHolder(t *testing.T) {
	seed := scan.AgentContextFactRecord{
		ID: "route", Project: "services/regulations", Kind: "route",
		Name:       "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
		HTTPMethod: "DELETE", Path: "/cadasters/{cadasterId}/regulations/{objectId}",
		File: "RegulationController.java", Confidence: "EXACT",
		Search: "delete cadaster regulation object",
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		seed,
		{
			ID: "application-accessor", Project: "services/tasks", Kind: "symbol",
			Name: "isParallelBatching", Qualified: "ApplicationConfig.isParallelBatching",
			File: "ApplicationConfig.java", Confidence: "EXACT",
			Search: "cadaster task configuration parallel batching",
		},
		{
			ID: "task-client-config", Project: "libraries/common", Kind: "symbol",
			Name: "CadasterTaskMgmtConfig", Qualified: "example.CadasterTaskMgmtConfig",
			File: "CadasterTaskMgmtConfig.java", Confidence: "EXACT",
			Search: "cadaster task management configuration base url credentials timeout retries",
		},
	}}
	query := "Delete a cadaster regulation and its tasks across services/regulations, services/tasks, and libraries/common. Cover configuration."

	concerns := publicContextConcerns(planContextConcerns(query, index, seed))
	for _, concern := range concerns {
		if concern.Kind != contextConcernConfiguration {
			continue
		}
		if concern.Project != "libraries/common" {
			t.Fatalf("configuration concern project = %q, want domain config holder: %#v", concern.Project, concerns)
		}
		return
	}
	t.Fatalf("configuration concern missing: %#v", concerns)
}

func TestScopedConfigurationRankRewardsConfigHolderOverAccessor(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{
			ID: "application-accessor", Project: "services/tasks", Kind: "symbol",
			Name: "isParallelBatching", Qualified: "CadasterTaskApplicationConfig.isParallelBatching",
			File: "CadasterTaskApplicationConfig.java", Confidence: "EXACT",
			Search: "cadaster task configuration parallel batching",
		},
		{
			ID: "task-client-config", Project: "libraries/common", Kind: "symbol",
			Name: "CadasterTaskMgmtConfig", Qualified: "example.CadasterTaskMgmtConfig",
			File: "CadasterTaskMgmtConfig.java", Confidence: "EXACT",
			Search: "cadaster task management configuration",
		},
	}
	query := "cadaster task configuration"

	accessorRank := contextScopedConcernRank(
		query,
		"services/tasks",
		contextConcernConfiguration,
		[]string{"application-accessor"},
		facts,
	)
	configHolderRank := contextScopedConcernRank(
		query,
		"libraries/common",
		contextConcernConfiguration,
		[]string{"task-client-config"},
		facts,
	)
	if configHolderRank <= accessorRank {
		t.Fatalf("config holder rank %d <= generated accessor rank %d", configHolderRank, accessorRank)
	}
}

func TestConfigurationFactShapePenalizesAccessorBeforeSuffixBonus(t *testing.T) {
	holder := scan.AgentContextFactRecord{
		Name: "CadasterTaskConfig", File: "CadasterTaskConfig.java",
	}
	accessor := scan.AgentContextFactRecord{
		Name: "getTaskConfig", File: "CadasterTaskClient.java",
	}

	holderScore := contextConcernFactShapeScore(holder, contextConcernConfiguration)
	accessorScore := contextConcernFactShapeScore(accessor, contextConcernConfiguration)
	if accessorScore >= 0 || holderScore <= accessorScore {
		t.Fatalf("configuration shape scores = holder %d, accessor %d", holderScore, accessorScore)
	}
}

func TestSideEffectConcernCandidatesIncludeSameActionProductionMethod(t *testing.T) {
	queryTokens := contextExpandedTokenSet(
		"delete cadaster regulation tasks with mail protocol logging and user information side effects",
	)
	facts := []scan.AgentContextFactRecord{
		{
			ID: "mail-type", Project: "services/tasks", Kind: "symbol",
			Name: "CadasterTaskMailService", Qualified: "example.CadasterTaskMailService",
			File: "CadasterTaskMailService.java", Confidence: "EXACT",
			Search: "cadaster task mail side effect",
		},
		{
			ID: "delete-method", Project: "services/tasks", Kind: "symbol",
			Name: "deleteCadasterTask", Qualified: "CadasterTaskService.deleteCadasterTask",
			File: "CadasterTaskService.java", Confidence: "EXACT",
			Search: "delete cadaster regulation task",
		},
	}

	candidates := contextExplicitProjectConcernCandidates(
		queryTokens,
		"services/tasks",
		contextConcernSideEffects,
		facts,
		map[string]bool{},
	)
	found := false
	for _, candidate := range candidates {
		found = found || candidate == "delete-method"
	}
	if !found {
		t.Fatalf("same-action production method missing from side-effect candidates: %v", candidates)
	}
}

func TestPlanContextConcernsIgnoresMetaCreateAction(t *testing.T) {
	query := "Root-cause analysis: when a regulation is removed from a cadaster, " +
		"connected tasks remain in services/jobs. Identify the internal API contract, " +
		"mail side effects, and production and test files to change/create."
	seed := scan.AgentContextFactRecord{
		ID: "delete-regulation", Project: "services/regulations", Kind: "api_endpoint",
		Name:       "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
		Qualified:  "RegulationController.deleteRegulation",
		HTTPMethod: "DELETE", Path: "/cadasters/{cadasterId}/regulations/{objectId}",
		File:   "RegulationController.java",
		Search: "delete cadaster regulation",
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		seed,
		{
			ID: "create-mail", Project: "services/jobs", Kind: "symbol",
			Name: "createTaskAndSendMail", Qualified: "TaskService.createTaskAndSendMail",
			File:   "TaskService.java",
			Search: "cadaster regulation task mail side effect",
		},
		{
			ID: "delete-task", Project: "services/jobs", Kind: "symbol",
			Name: "deleteTask", Qualified: "TaskService.deleteTask",
			File:   "TaskService.java",
			Search: "delete cadaster regulation task",
		},
		{
			ID: "create-contract", Project: "services/jobs", Kind: "api_endpoint",
			Name: "POST /tasks", Qualified: "TaskController.createTask",
			HTTPMethod: "POST", Path: "/tasks",
			File:   "TaskController.java",
			Search: "create cadaster regulation task internal API contract",
		},
		{
			ID: "delete-contract", Project: "services/jobs", Kind: "api_endpoint",
			Name: "DELETE /tasks/regulation-change", Qualified: "TaskController.deleteTask",
			HTTPMethod: "DELETE", Path: "/tasks/regulation-change",
			File:   "TaskController.java",
			Search: "delete cadaster regulation task internal API contract",
		},
	}}

	concerns := planContextConcerns(query, index, seed)
	project, ok := findContextConcern(
		concerns,
		contextConcernProject+":services/jobs",
	)
	if !ok || !reflect.DeepEqual(
		project.candidateFactIDs,
		[]string{"delete-contract", "delete-task"},
	) {
		t.Fatalf(
			"project candidates = %#v, want delete action only",
			project.candidateFactIDs,
		)
	}
	httpContract, ok := findContextConcern(
		concerns,
		contextConcernHTTPContract+":services/jobs",
	)
	if !ok || !reflect.DeepEqual(httpContract.candidateFactIDs, []string{"delete-contract"}) {
		t.Fatalf(
			"HTTP contract candidates = %#v, want delete action only",
			httpContract.candidateFactIDs,
		)
	}
	sideEffects, ok := findContextConcern(
		concerns,
		contextConcernSideEffects+":services/jobs",
	)
	if !ok || !reflect.DeepEqual(
		sideEffects.candidateFactIDs,
		[]string{"delete-contract", "delete-task"},
	) {
		t.Fatalf(
			"side-effect candidates = %#v, want delete action only",
			sideEffects.candidateFactIDs,
		)
	}
}

func TestContextSourceOptionActionIgnoresMetaFileOperations(t *testing.T) {
	query := "Read-only root-cause analysis: when a regulation is removed from a cadaster, " +
		"connected tasks remain. Identify the exact production and test files to " +
		"change/create, error handling, retry logic, and tests."
	pack := ContextPack{Query: query, selectionQuery: query}
	deleteOption := contextSourceOption{candidate: sourceCandidate{
		Name:      "deleteRegulationChangeTask",
		Qualified: "CadasterTaskController.deleteRegulationChangeTask",
	}}
	createOption := contextSourceOption{candidate: sourceCandidate{
		Name:      "createTaskRegulationChange",
		Qualified: "CadasterTaskController.createTaskRegulationChange",
	}}

	if !contextSourceOptionActionAligned(pack, deleteOption) {
		t.Fatal("primary delete action was rejected")
	}
	if contextSourceOptionActionAligned(pack, createOption) {
		t.Fatal("meta instruction to create files became a domain create action")
	}
}

func TestSupportRouteMatchesRequestedActionWithoutExactFuturePath(t *testing.T) {
	route := scan.AgentContextFactRecord{
		ID: "task-delete", Project: "services/tasks", Kind: "route",
		Name: "DELETE /task-management/tasks/{taskId}", HTTPMethod: "DELETE",
		Path: "/task-management/tasks/{taskId}", Confidence: "EXACT",
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{route}}
	operational, score := contextSupportOperationalScore(
		newContextForwardUtility(index),
		route,
		"Wenn ein Eintrag entfernt wird, müssen verbundene Aufgaben gelöscht werden.",
		nil,
	)
	if !operational || score <= 0 {
		t.Fatalf("same-action support route = operational %v score %d", operational, score)
	}
}

func TestSupportRouteIgnoresMetaCreateAction(t *testing.T) {
	query := "Root-cause analysis: when a regulation is removed from a cadaster, " +
		"connected tasks remain. Identify production and test files to change/create."
	deleteRoute := scan.AgentContextFactRecord{
		ID: "task-delete", Project: "services/tasks", Kind: "route",
		Name: "DELETE /tasks/regulation-change", HTTPMethod: "DELETE",
		Path: "/tasks/regulation-change", Confidence: "EXACT",
	}
	createRoute := scan.AgentContextFactRecord{
		ID: "task-create", Project: "services/tasks", Kind: "route",
		Name: "POST /tasks", HTTPMethod: "POST",
		Path: "/tasks", Confidence: "EXACT",
	}
	index := scan.AgentContextIndexRecord{
		Facts: []scan.AgentContextFactRecord{deleteRoute, createRoute},
	}
	utility := newContextForwardUtility(index)

	deleteOperational, _ := contextSupportOperationalScore(
		utility,
		deleteRoute,
		query,
		nil,
	)
	createOperational, _ := contextSupportOperationalScore(
		utility,
		createRoute,
		query,
		nil,
	)
	if !deleteOperational || createOperational {
		t.Fatalf(
			"support actions = delete %v create %v, want delete only",
			deleteOperational,
			createOperational,
		)
	}
}

func TestRetryFactActionIgnoresMetaCreateAction(t *testing.T) {
	query := "Root-cause analysis: when a regulation is removed from a cadaster, " +
		"connected tasks remain. Identify production and test files to change/create."
	deleteFact := scan.AgentContextFactRecord{
		ID: "delete-task", Name: "deleteTask", Qualified: "TaskService.deleteTask",
	}
	createFact := scan.AgentContextFactRecord{
		ID: "create-task", Name: "createTask", Qualified: "TaskService.createTask",
	}
	index := scan.AgentContextIndexRecord{
		Facts: []scan.AgentContextFactRecord{deleteFact, createFact},
	}
	if !contextRetryFactMatchesAction(deleteFact, index, query) {
		t.Fatal("primary delete action was rejected for retry")
	}
	if contextRetryFactMatchesAction(createFact, index, query) {
		t.Fatal("meta create action matched retry fact")
	}
}

func TestSourceConcernScoreIgnoresMetaCreateAction(t *testing.T) {
	query := "Root-cause analysis: when a regulation is removed from a cadaster, " +
		"connected tasks remain. Identify mail side effects and files to change/create."
	concern := newContextConcern(
		contextConcernSideEffects,
		"services/tasks",
		true,
		[]string{"delete-task", "create-task"},
		"requested side effects",
	)
	deleteFact := scan.AgentContextFactRecord{
		ID: "delete-task", Project: "services/tasks", Kind: "symbol",
		Name: "deleteTask", Qualified: "TaskService.deleteTask",
		Search: "cadaster regulation task mail side effect",
	}
	createFact := scan.AgentContextFactRecord{
		ID: "create-task", Project: "services/tasks", Kind: "symbol",
		Name: "createTask", Qualified: "TaskService.createTask",
		Search: "cadaster regulation task mail side effect",
	}

	deleteScore := contextSourceConcernFactScore(deleteFact, concern, query, nil)
	createScore := contextSourceConcernFactScore(createFact, concern, query, nil)
	if deleteScore <= createScore {
		t.Fatalf(
			"source concern scores = delete %d create %d, want delete higher",
			deleteScore,
			createScore,
		)
	}
}

type missingContractContextSnapshot struct {
	FactIDs        []string
	ConcernKeys    []string
	EntrypointID   string
	SourceRoles    []string
	Fallback       bool
	Retry          bool
	SourceCoverage string
	SourcePaths    []string
	SourceModes    []string
}

func missingContractContextSnapshotForPack(pack ContextPack) missingContractContextSnapshot {
	factIDs := append([]string(nil), pack.selectedFactIDs...)
	sort.Strings(factIDs)
	concernKeys := append([]string(nil), pack.selectedConcernKeys...)
	sort.Strings(concernKeys)
	sourceRoles := make([]string, 0, len(pack.SourceSections))
	sourcePaths := make([]string, 0, len(pack.SourceSections))
	sourceModes := make([]string, 0, len(pack.SourceSections))
	for _, section := range pack.SourceSections {
		sourceRoles = append(sourceRoles, section.Project+":"+section.Role)
		sourcePaths = append(sourcePaths, section.Project+":"+section.Path)
		sourceModes = append(sourceModes, section.Project+":"+section.Path+":"+section.RenderMode)
	}
	sort.Strings(sourcePaths)
	sort.Strings(sourceModes)
	entrypointID := ""
	if len(pack.Entrypoints) == 1 {
		entrypointID = pack.Entrypoints[0].ID
	}
	return missingContractContextSnapshot{
		FactIDs:        factIDs,
		ConcernKeys:    concernKeys,
		EntrypointID:   entrypointID,
		SourceRoles:    sourceRoles,
		Fallback:       pack.FallbackRequired,
		Retry:          pack.RetryAllowed,
		SourceCoverage: pack.SourceCoverage,
		SourcePaths:    sourcePaths,
		SourceModes:    sourceModes,
	}
}

func contextSelectedFactSet(pack ContextPack) map[string]bool {
	result := make(map[string]bool, len(pack.selectedFactIDs))
	for _, factID := range pack.selectedFactIDs {
		result[factID] = true
	}
	return result
}

func contextHasUncertainty(pack ContextPack, scope string) bool {
	for _, uncertainty := range pack.Uncertainties {
		if uncertainty.Scope == scope {
			return true
		}
	}
	return false
}

func contextSourcePathSet(pack ContextPack) map[string]bool {
	result := make(map[string]bool, len(pack.SourceSections))
	for _, section := range pack.SourceSections {
		result[section.Path] = true
	}
	return result
}

func contextSourceContainsStableIdentity(pack ContextPack, value string) bool {
	for _, section := range pack.SourceSections {
		if strings.Contains(section.Content, value) {
			return true
		}
	}
	return false
}

func contextPackContainsFileSuffix(pack ContextPack, suffix string) bool {
	suffix = filepath.ToSlash(suffix)
	for _, file := range pack.Files {
		if strings.HasSuffix(filepath.ToSlash(file.Path), suffix) ||
			strings.HasSuffix(filepath.ToSlash(filepath.Join(file.Project, file.Path)), suffix) {
			return true
		}
	}
	for _, section := range pack.SourceSections {
		if strings.HasSuffix(filepath.ToSlash(section.Path), suffix) ||
			strings.HasSuffix(filepath.ToSlash(filepath.Join(section.Project, section.Path)), suffix) {
			return true
		}
	}
	return false
}

func writeMissingContractContextFixture(t *testing.T) string {
	t.Helper()
	return writeMissingContractContextIndexFixture(t, missingContractContextIndex())
}

func writeReleaseQualityMissingContractFixture(t *testing.T) string {
	return writeReleaseQualityMissingContractFixtureWithIndex(t, releaseQualityMissingContractIndex())
}

func writeReleaseQualityMissingContractFixtureWithIndex(
	t *testing.T,
	index scan.AgentContextIndexRecord,
) string {
	t.Helper()
	root := writeMissingContractContextIndexFixture(t, index)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("libraries/job-client", "src/main/java/example/JobClient.java"),
		contextReleaseQualityJobClientFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/catalog", "src/main/java/example/CatalogJobEntity.java"),
		contextReleaseQualityConsumerModelFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/jobs", "src/main/java/example/BaseCatalogJobEntity.java"),
		contextReleaseQualityBaseModelFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("libraries/job-client", "src/main/java/example/JobClientConfig.java"),
		contextReleaseQualityClientConfigFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("libraries/job-client", "src/main/java/example/JobClientAuth.java"),
		contextReleaseQualityClientAuthFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/jobs", "src/main/java/example/JobSecurity.java"),
		contextReleaseQualityServerPolicyFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/jobs", "src/main/java/example/JobManagementController.java"),
		contextReleaseQualityManagementControllerFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/jobs", "src/main/resources/application.properties"),
		"task-isbns=fixture-generic\ntechnical-user=fixture-technical-user\n",
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/jobs", "src/test/java/example/JobServiceTest.java"),
		contextReleaseQualityServiceTestFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/catalog", "src/main/resources/application.yml"),
		contextReleaseQualityApplicationConfigurationFixtureSource(false),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/catalog", "src/test/resources/application-test.yml"),
		contextReleaseQualityApplicationConfigurationFixtureSource(true),
	)
	return root
}

func runtimeShapeReleaseQualityMissingContractIndex() scan.AgentContextIndexRecord {
	index := releaseQualityMissingContractIndex()
	facts := make([]scan.AgentContextFactRecord, 0, len(index.Facts))
	for factIndex := range index.Facts {
		fact := &index.Facts[factIndex]
		if fact.ID == "regular-comment-repository" {
			continue
		}
		switch fact.ID {
		case "job-client-config":
			fact.Kind = "symbol"
		case "job-server-policy":
			fact.Kind = "endpoint_security"
			fact.Name = "role"
			fact.Qualified = "GET /job-management/jobs role"
			fact.Line = 11
			fact.EndLine = 11
		case "jobs-test":
			fact.Kind = "symbol"
			fact.Name = "JobManagementControllerTest"
			fact.Qualified = "jobs.JobManagementControllerTest"
		case "jobs-service-test":
			fact.Kind = "symbol"
			fact.Name = "JobServiceTest"
			fact.Qualified = "jobs.JobServiceTest"
		}
		facts = append(facts, *fact)
	}
	index.Facts = facts
	index.Facts = append(index.Facts,
		scan.AgentContextFactRecord{
			ID: "jobs-generic-configuration", Project: "services/jobs", Kind: "configuration",
			Name: "task-isbns", File: "src/main/resources/application.properties",
			Line: 1, EndLine: 1, Confidence: "EXACT", Search: "task isbn configuration",
		},
		scan.AgentContextFactRecord{
			ID: "jobs-authentication-configuration", Project: "services/jobs", Kind: "configuration",
			Name: "technical-user", File: "src/main/resources/application.properties",
			Line: 2, EndLine: 2, Confidence: "EXACT", Search: "technical user authentication configuration",
		},
	)
	return index
}

func releaseQualityMissingContractIndex() scan.AgentContextIndexRecord {
	index := missingContractContextIndex()
	index.Facts = append(
		index.Facts,
		scan.AgentContextFactRecord{
			ID: "consumer-duplicate-model", Project: "services/catalog", Kind: "symbol",
			Name: "CatalogJobEntity", Qualified: "catalog.CatalogJobEntity",
			File: "src/main/java/example/CatalogJobEntity.java",
			Line: 8, EndLine: 9, Confidence: "EXACT",
			Search: "catalog job task model catalogId itemId",
		},
		scan.AgentContextFactRecord{
			ID: "base-job-model", Project: "services/jobs", Kind: "symbol",
			Name: "BaseCatalogJobEntity", Qualified: "jobs.BaseCatalogJobEntity",
			File: "src/main/java/example/BaseCatalogJobEntity.java",
			Line: 8, EndLine: 13, Confidence: "EXACT",
			Search: "job task base model catalogId itemId",
		},
		scan.AgentContextFactRecord{
			ID: "job-client-config", Project: "libraries/job-client", Kind: "configuration",
			Name: "JobClientConfig", Qualified: "client.JobClientConfig",
			File: "src/main/java/example/JobClientConfig.java",
			Line: 8, EndLine: 16, Confidence: "EXACT",
			Search: "job client configuration base url credentials timeout retries",
		},
		scan.AgentContextFactRecord{
			ID: "job-client-auth", Project: "libraries/job-client", Kind: "authentication",
			Name: "setBasicAuth", Qualified: "client.JobClientAuth.setBasicAuth",
			File: "src/main/java/example/JobClientAuth.java",
			Line: 8, EndLine: 12, Confidence: "EXACT",
			Search: "job client basic authentication credentials",
		},
		scan.AgentContextFactRecord{
			ID: "job-server-policy", Project: "services/jobs", Kind: "authentication",
			Name: "securityFilterChain", Qualified: "jobs.JobSecurity.securityFilterChain",
			File: "src/main/java/example/JobSecurity.java",
			Line: 8, EndLine: 16, Confidence: "EXACT",
			Search: "job server basic authentication technical role security policy",
		},
		scan.AgentContextFactRecord{
			ID: "jobs-service-test", Project: "services/jobs", Kind: "test",
			Name: "deletesBothJobVariants", Qualified: "jobs.JobServiceTest.deletesBothJobVariants",
			File: "src/test/java/example/JobServiceTest.java",
			Line: 8, EndLine: 15, Confidence: "EXACT",
			Search: "job deletion persistence side effects test",
		},
		scan.AgentContextFactRecord{
			ID: "catalog-job-client-configuration", Project: "services/catalog", Kind: "configuration",
			Name: "jobClientConfiguration", Qualified: "catalog.application.jobClientConfiguration",
			File: "src/main/resources/application.yml",
			Line: 1, EndLine: 6, Confidence: "EXACT",
			Search: "catalog job client configuration base url credentials timeout retries",
		},
		scan.AgentContextFactRecord{
			ID: "catalog-job-client-test-configuration", Project: "services/catalog", Kind: "configuration",
			Name: "jobClientTestConfiguration", Qualified: "catalog.applicationTest.jobClientConfiguration",
			File: "src/test/resources/application-test.yml",
			Line: 1, EndLine: 6, Confidence: "EXACT",
			Search: "catalog job client test profile configuration base url credentials timeout retries",
		},
	)
	index.Edges = append(
		index.Edges,
		scan.AgentContextEdgeRecord{
			ID: "regular-job-base", FromFactID: "regular-job-model",
			ToFactID: "base-job-model", Kind: "extends", Confidence: "EXACT",
		},
		scan.AgentContextEdgeRecord{
			ID: "change-job-base", FromFactID: "change-job-model",
			ToFactID: "base-job-model", Kind: "extends", Confidence: "EXACT",
		},
	)
	return index
}

func missingContractContextIndex() scan.AgentContextIndexRecord {
	return scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-23T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{ID: "catalog-route", Project: "services/catalog", Kind: "route", Name: "DELETE /catalog/items/{itemId}", Qualified: "CatalogController.deleteItem", HTTPMethod: "DELETE", Path: "/catalog/items/{itemId}", File: "src/main/java/example/CatalogController.java", Line: 8, EndLine: 12, Confidence: "EXACT", Search: "delete catalog item"},
			{ID: "catalog-operations", Project: "services/catalog", Kind: "symbol", Name: "deleteItem", Qualified: "CatalogOperations.deleteItem", File: "src/main/java/example/CatalogOperations.java", Line: 9, EndLine: 14, Confidence: "EXACT", Search: "delete catalog item operations"},
			{ID: "catalog-repository", Project: "services/catalog", Kind: "persistence", Name: "deleteById", Qualified: "CatalogRepository.deleteById", File: "src/main/java/example/CatalogRepository.java", Line: 5, EndLine: 7, Confidence: "RESOLVED", Search: "delete catalog item persistence"},
			{ID: "job-client", Project: "libraries/job-client", Kind: "symbol", Name: "listJobs", Qualified: "JobClient.listJobs", File: "src/main/java/example/JobClient.java", Line: 14, EndLine: 20, Confidence: "EXACT", Search: "job task client configuration authentication retry"},
			{ID: "job-contract", Project: "libraries/job-client", Kind: "api_contract", Name: "GET /job-management/jobs", Qualified: "JobClient.listJobs", HTTPMethod: "GET", Path: "/job-management/jobs", File: "src/main/java/example/JobClient.java", Line: 16, EndLine: 18, Confidence: "RESOLVED", Search: "job task client contract"},
			{ID: "job-config", Project: "libraries/job-client", Kind: "configuration", Name: "getAllJobsPath", Qualified: "JobClient.listJobs", File: "src/main/java/example/JobClient.java", Line: 14, EndLine: 20, Confidence: "EXACT", Search: "job task client configuration base url"},
			{ID: "job-auth", Project: "libraries/job-client", Kind: "authentication", Name: "basicAuthentication", Qualified: "JobClient.listJobs", File: "src/main/java/example/JobClient.java", Line: 14, EndLine: 20, Confidence: "EXACT", Search: "job task client basic authentication"},
			{ID: "job-retry", Project: "libraries/job-client", Kind: "resilience", Name: "retryPolicy", Qualified: "JobClient.listJobs", File: "src/main/java/example/JobClient.java", Line: 14, EndLine: 20, Confidence: "EXACT", Search: "job task retry exception handling resilience"},
			{ID: "jobs-route", Project: "services/jobs", Kind: "route", Name: "GET /job-management/jobs", Qualified: "JobManagementController.listJobs", HTTPMethod: "GET", Path: "/job-management/jobs", File: "src/main/java/example/JobManagementController.java", Line: 10, EndLine: 14, Confidence: "EXACT", Search: "job task management endpoint"},
			{ID: "jobs-service", Project: "services/jobs", Kind: "symbol", Name: "listJobs", Qualified: "JobService.listJobs", File: "src/main/java/example/JobService.java", Line: 12, EndLine: 18, Confidence: "EXACT", Search: "job task management service side effect"},
			{ID: "jobs-finder", Project: "services/jobs", Kind: "persistence", Name: "findByCatalogIdAndItemId", Qualified: "CatalogJobRepository.findByCatalogIdAndItemId", File: "src/main/java/example/CatalogJobRepository.java", Line: 7, EndLine: 9, Confidence: "EXACT", Search: "regular job task catalog item finder persistence"},
			{ID: "jobs-find-all", Project: "services/jobs", Kind: "persistence", Name: "findAll", Qualified: "CatalogJobRepository.findAll", File: "src/main/java/example/CatalogJobRepository.java", Line: 6, EndLine: 6, Confidence: "RESOLVED", Search: "job repository inherited find all persistence"},
			{ID: "jobs-side-effect", Project: "services/jobs", Kind: "symbol", Name: "publishDeletion", Qualified: "JobHousekeeping.publishDeletion", File: "src/main/java/example/JobHousekeeping.java", Line: 8, EndLine: 13, Confidence: "EXACT", Search: "job task deletion logging mail user information side effect"},
			{ID: "jobs-test", Project: "services/jobs", Kind: "test", Name: "listJobs", Qualified: "JobManagementControllerTest.listJobs", File: "src/test/java/example/JobManagementControllerTest.java", Line: 10, EndLine: 18, Confidence: "EXACT", Search: "job task management test"},
			{ID: "regular-job-model", Project: "services/jobs", Kind: "symbol", Name: "CatalogJobEntity", Qualified: "example.CatalogJobEntity", File: "src/main/java/example/CatalogJobEntity.java", Line: 8, EndLine: 12, Confidence: "EXACT", Search: "regular job task model catalogId itemId"},
			{ID: "change-job-model", Project: "services/jobs", Kind: "symbol", Name: "CatalogChangeJobEntity", Qualified: "example.CatalogChangeJobEntity", File: "src/main/java/example/CatalogChangeJobEntity.java", Line: 8, EndLine: 12, Confidence: "EXACT", Search: "change job task model catalogId itemId changeId"},
			{ID: "change-job-repository", Project: "services/jobs", Kind: "persistence", Name: "findByCatalogIdAndItemId", Qualified: "CatalogChangeJobRepository.findByCatalogIdAndItemId", File: "src/main/java/example/CatalogChangeJobRepository.java", Line: 7, EndLine: 9, Confidence: "EXACT", Search: "change job task catalog item persistence"},
			{ID: "regular-comment-repository", Project: "services/jobs", Kind: "persistence", Name: "findByJobIdOrderByCreated", Qualified: "CatalogJobCommentRepository.findByJobIdOrderByCreated", File: "src/main/java/example/CatalogJobCommentRepository.java", Line: 7, EndLine: 8, Confidence: "EXACT", Search: "regular job comment dependency persistence"},
			{ID: "generic-mail-properties", Project: "libraries/job-client", Kind: "configuration", Name: "MailProperties", Qualified: "example.MailProperties", File: "src/main/java/example/MailProperties.java", Line: 8, EndLine: 8, Confidence: "EXACT", Search: "mail configuration"},
			{ID: "generic-async-handler", Project: "libraries/job-client", Kind: "resilience", Name: "AsyncExceptionHandler", Qualified: "example.AsyncExceptionHandler", File: "src/main/java/example/AsyncExceptionHandler.java", Line: 8, EndLine: 8, Confidence: "EXACT", Search: "exception handling resilience"},
			{ID: "unrelated-topic-repository", Project: "services/catalog", Kind: "persistence", Name: "findTopic", Qualified: "CatalogTopicRepository.findTopic", File: "src/main/java/example/CatalogTopicRepository.java", Line: 7, EndLine: 8, Confidence: "EXACT", Search: "catalog topic persistence"},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "current-1", FromFactID: "catalog-route", ToFactID: "catalog-operations", Kind: "call", Confidence: "EXACT"},
			{ID: "current-2", FromFactID: "catalog-operations", ToFactID: "catalog-repository", Kind: "persistence", Confidence: "RESOLVED"},
			{ID: "adjacent-1", FromFactID: "job-client", ToFactID: "job-contract", Kind: "call", Confidence: "EXACT"},
			{ID: "adjacent-2", FromFactID: "job-contract", ToFactID: "jobs-route", Kind: "http_contract", Confidence: "RESOLVED"},
			{ID: "adjacent-3", FromFactID: "jobs-route", ToFactID: "jobs-service", Kind: "call", Confidence: "EXACT"},
			{ID: "adjacent-4", FromFactID: "jobs-service", ToFactID: "jobs-finder", Kind: "persistence", Confidence: "RESOLVED"},
			{ID: "adjacent-5", FromFactID: "jobs-service", ToFactID: "jobs-side-effect", Kind: "call", Confidence: "RESOLVED"},
			{ID: "adjacent-test", FromFactID: "jobs-test", ToFactID: "jobs-route", Kind: "test_target", Confidence: "EXACT"},
		},
	}
}

func writeMissingContractContextIndexFixture(t *testing.T, index scan.AgentContextIndexRecord) string {
	t.Helper()
	root := t.TempDir()
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), index)

	factsByPath := make(map[string][]scan.AgentContextFactRecord)
	for _, fact := range index.Facts {
		factsByPath[filepath.Join(fact.Project, fact.File)] = append(factsByPath[filepath.Join(fact.Project, fact.File)], fact)
	}
	for path, facts := range factsByPath {
		writeContextSourceFile(t, root, path, crossServiceSource(".java", path, facts))
	}
	writeContextSourceFile(
		t,
		root,
		filepath.Join("libraries/job-client", "src/main/java/example/JobClient.java"),
		contextJobClientFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/jobs", "src/test/java/example/JobManagementControllerTest.java"),
		contextJobManagementControllerTestFixtureSource(),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/jobs", "src/main/java/example/CatalogJobEntity.java"),
		contextDomainModelFixtureSource("CatalogJobEntity", "", "catalogId", "itemId"),
	)
	writeContextSourceFile(
		t,
		root,
		filepath.Join("services/jobs", "src/main/java/example/CatalogChangeJobEntity.java"),
		contextDomainModelFixtureSource("CatalogChangeJobEntity", "CatalogJobEntity", "catalogId", "itemId", "changeId"),
	)
	return root
}

func contextJobClientFixtureSource() string {
	lines := numberedSourceLines(24)
	lines[13] = "@Retryable(retryFor = JobClientException.class)"
	lines[14] = "List<JobPayload> listJobs(String catalogId) {"
	lines[15] = "  String path = configuration.getAllJobsPath();"
	lines[16] = "  HttpHeaders headers = basicAuthentication(configuration.credentials());"
	lines[17] = "  return restClient.get(path, headers, catalogId);"
	lines[18] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextJobManagementControllerTestFixtureSource() string {
	lines := numberedSourceLines(20)
	lines[9] = "@Test"
	lines[10] = "void listJobs() {"
	lines[11] = "  assert true;"
	lines[12] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextDomainModelFixtureSource(name, parent string, fields ...string) string {
	lines := numberedSourceLines(20)
	declaration := "class " + name
	if parent != "" {
		declaration += " extends " + parent
	}
	lines[7] = declaration + " {"
	for index, field := range fields {
		lines[8+index] = "  long " + field + ";"
	}
	lines[8+len(fields)] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextReleaseQualityBaseModelFixtureSource() string {
	lines := numberedSourceLines(20)
	lines[7] = "class BaseCatalogJobEntity {"
	lines[8] = "  long catalogId;"
	lines[9] = "  long itemId;"
	lines[10] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextReleaseQualityJobClientFixtureSource() string {
	lines := numberedSourceLines(24)
	lines[13] = "@Retryable(retryFor = JobClientException.class)"
	lines[14] = "List<JobPayload> listJobs(String catalogId) {"
	lines[15] = "  String path = configuration.getAllJobsPath();"
	lines[16] = "  HttpHeaders headers = basicAuthentication(configuration.credentials());"
	lines[17] = "  new JobClientAuth().setBasicAuth(headers, configuration);"
	lines[18] = "  return restClient.get(path, headers, catalogId);"
	lines[19] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextReleaseQualityConsumerModelFixtureSource() string {
	lines := numberedSourceLines(20)
	lines[7] = "class CatalogJobEntity {"
	lines[8] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextReleaseQualityClientConfigFixtureSource() string {
	lines := numberedSourceLines(20)
	lines[7] = `@ConfigurationProperties(prefix = "jobs")`
	lines[8] = "class JobClientConfig {"
	lines[9] = "  String baseUrl;"
	lines[10] = "  String username;"
	lines[11] = "  String password;"
	lines[12] = "  Duration connectTimeout;"
	lines[13] = "  Duration readTimeout;"
	lines[14] = "  int maxRetries;"
	lines[15] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextReleaseQualityClientAuthFixtureSource() string {
	lines := numberedSourceLines(20)
	lines[7] = "class JobClientAuth {"
	lines[8] = "  void setBasicAuth(HttpHeaders headers, JobClientConfig config) {"
	lines[9] = "    headers.setBasicAuth(config.username, config.password);"
	lines[10] = "  }"
	lines[11] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextReleaseQualityServerPolicyFixtureSource() string {
	lines := numberedSourceLines(20)
	lines[7] = "class JobSecurity {"
	lines[8] = "  SecurityFilterChain securityFilterChain(HttpSecurity http) {"
	lines[9] = `    return http.securityMatcher("/job-management/**")`
	lines[10] = `      .authorizeHttpRequests(auth -> auth.anyRequest().hasRole("TECHNICAL_USER"))`
	lines[11] = "      .httpBasic(Customizer.withDefaults()).build();"
	lines[12] = "  }"
	lines[13] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextReleaseQualityManagementControllerFixtureSource() string {
	lines := numberedSourceLines(20)
	lines[9] = "@RestController"
	lines[10] = "class JobManagementController {"
	lines[11] = "  @GetMapping(\"/job-management/jobs\")"
	lines[12] = "  List<JobPayload> listJobs() { return jobService.listJobs(); }"
	lines[13] = "}"
	return strings.Join(lines, "\n") + "\n"
}

func contextReleaseQualityApplicationConfigurationFixtureSource(testProfile bool) string {
	if testProfile {
		return strings.Join([]string{
			"jobs:",
			"  base-url: https://jobs-test.invalid",
			"  username: fixture-test-user",
			"  password: fixture-test-password",
			"  connect-timeout: 5s",
			"  max-retries: 1",
		}, "\n") + "\n"
	}
	return strings.Join([]string{
		"jobs:",
		"  base-url: https://jobs.invalid",
		"  username: fixture-client-user",
		"  password: fixture-client-password",
		"  connect-timeout: 1s",
		"  max-retries: 3",
	}, "\n") + "\n"
}

func contextReleaseQualityServiceTestFixtureSource() string {
	lines := numberedSourceLines(20)
	lines[7] = "@Test"
	lines[8] = "void deletesBothJobVariants() {"
	lines[9] = "  Object result = deleteItem();"
	lines[10] = "  assertNotNull(result);"
	lines[11] = "}"
	return strings.Join(lines, "\n") + "\n"
}
