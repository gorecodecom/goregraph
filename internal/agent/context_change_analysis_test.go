package agent

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	missingContractEnglishQuery = "When DELETE /catalog/items/{itemId} removes an item in services/catalog, plan cleanup of related jobs through libraries/job-client and services/jobs. Cover the current path, missing HTTP contract, task types and lookup attributes, authentication, configuration, retry behavior, persistence, side effects, and tests."
	missingContractGermanQuery  = "Wenn DELETE /catalog/items/{itemId} einen Eintrag in services/catalog löscht, plane das Entfernen verbundener Aufgaben über libraries/job-client und services/jobs. Berücksichtige aktuellen Pfad, fehlenden HTTP-Vertrag, Aufgabenarten und Suchattribute, Authentifizierung, Konfiguration, Wiederholung, Persistenz, Nebenwirkungen und Tests."
)

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
	if pack.FallbackRequired || pack.RetryAllowed || pack.SourceCoverage != "complete" {
		t.Fatalf(
			"pack decision = fallback %v retry %v coverage %q omissions %#v",
			pack.FallbackRequired,
			pack.RetryAllowed,
			pack.SourceCoverage,
			pack.SourceOmissions,
		)
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

func writeMissingContractContextFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
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
