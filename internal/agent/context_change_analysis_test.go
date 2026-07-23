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
	missingContractEnglishQuery = "When DELETE /catalog/items/{itemId} removes an item in services/catalog, plan cleanup of related jobs through libraries/job-client and services/jobs. Cover the current path, missing HTTP contract, authentication, configuration, retry behavior, persistence, side effects, and tests."
	missingContractGermanQuery  = "Wenn DELETE /catalog/items/{itemId} einen Eintrag in services/catalog löscht, plane das Entfernen verbundener Aufgaben über libraries/job-client und services/jobs. Berücksichtige aktuellen Pfad, fehlenden HTTP-Vertrag, Authentifizierung, Konfiguration, Wiederholung, Persistenz, Nebenwirkungen und Tests."
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
		"jobs-route", "jobs-service", "jobs-finder", "jobs-side-effect", "jobs-test",
	} {
		if !selected[factID] {
			t.Errorf("required evidence %q not selected", factID)
		}
	}
	if selected["jobs-find-all"] {
		t.Error("generic inherited findAll displaced the declared finder")
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

type missingContractContextSnapshot struct {
	FactIDs        []string
	ConcernKeys    []string
	EntrypointID   string
	SourceRoles    []string
	Fallback       bool
	Retry          bool
	SourceCoverage string
}

func missingContractContextSnapshotForPack(pack ContextPack) missingContractContextSnapshot {
	factIDs := append([]string(nil), pack.selectedFactIDs...)
	sort.Strings(factIDs)
	concernKeys := append([]string(nil), pack.selectedConcernKeys...)
	sort.Strings(concernKeys)
	sourceRoles := make([]string, 0, len(pack.SourceSections))
	for _, section := range pack.SourceSections {
		sourceRoles = append(sourceRoles, section.Project+":"+section.Role)
	}
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
			{ID: "job-config", Project: "libraries/job-client", Kind: "configuration", Name: "getAllJobsPath", Qualified: "JobClientConfig.getAllJobsPath", File: "src/main/java/example/JobClientConfig.java", Line: 10, EndLine: 13, Confidence: "EXACT", Search: "job task client configuration base url"},
			{ID: "job-auth", Project: "libraries/job-client", Kind: "authentication", Name: "basicAuthentication", Qualified: "JobClientAuth.basicAuthentication", File: "src/main/java/example/JobClientAuth.java", Line: 7, EndLine: 10, Confidence: "EXACT", Search: "job task client basic authentication"},
			{ID: "job-retry", Project: "libraries/job-client", Kind: "resilience", Name: "retryPolicy", Qualified: "JobClientRetry.retryPolicy", File: "src/main/java/example/JobClientRetry.java", Line: 7, EndLine: 12, Confidence: "EXACT", Search: "job task retry exception handling resilience"},
			{ID: "jobs-route", Project: "services/jobs", Kind: "route", Name: "GET /job-management/jobs", Qualified: "JobManagementController.listJobs", HTTPMethod: "GET", Path: "/job-management/jobs", File: "src/main/java/example/JobManagementController.java", Line: 10, EndLine: 14, Confidence: "EXACT", Search: "job task management endpoint"},
			{ID: "jobs-service", Project: "services/jobs", Kind: "symbol", Name: "listJobs", Qualified: "JobService.listJobs", File: "src/main/java/example/JobService.java", Line: 12, EndLine: 18, Confidence: "EXACT", Search: "job task management service side effect"},
			{ID: "jobs-finder", Project: "services/jobs", Kind: "persistence", Name: "findByCatalogIdAndItemId", Qualified: "JobRepository.findByCatalogIdAndItemId", File: "src/main/java/example/JobRepository.java", Line: 7, EndLine: 8, Confidence: "EXACT", Search: "job task catalog item finder persistence"},
			{ID: "jobs-find-all", Project: "services/jobs", Kind: "persistence", Name: "findAll", Qualified: "JobRepository.findAll", File: "src/main/java/example/JobRepository.java", Line: 6, EndLine: 6, Confidence: "RESOLVED", Search: "job repository inherited find all persistence"},
			{ID: "jobs-side-effect", Project: "services/jobs", Kind: "symbol", Name: "publishDeletion", Qualified: "JobHousekeeping.publishDeletion", File: "src/main/java/example/JobHousekeeping.java", Line: 8, EndLine: 13, Confidence: "EXACT", Search: "job task deletion logging mail user information side effect"},
			{ID: "jobs-test", Project: "services/jobs", Kind: "test", Name: "listJobs", Qualified: "JobManagementControllerTest.listJobs", File: "src/test/java/example/JobManagementControllerTest.java", Line: 10, EndLine: 18, Confidence: "EXACT", Search: "job task management test"},
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
	return root
}
