package agent

import (
	"reflect"
	"slices"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestContextProductionPlanFilesRequiresMissingTransitionInventory(t *testing.T) {
	pack, index := productionPlanFilesFixture()
	tests := []struct {
		name  string
		query string
	}{
		{name: "ordinary inventory", query: "Identify the exact production files for the current internal HTTP contract."},
		{name: "missing transition without inventory", query: "Explain the missing internal HTTP contract for deleting catalog jobs."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pack.Query = test.query
			pack.selectionQuery = test.query
			if got := contextProductionPlanFiles(pack, index); len(got) != 0 {
				t.Fatalf("production plan files = %#v, want none", got)
			}
		})
	}
}

func TestContextProductionPlanFilesSelectsExactProviderAndPrimaryPersistence(t *testing.T) {
	pack, index := productionPlanFilesFixture()
	want := []ContextProductionPlanFiles{{
		Project:          "services/jobs",
		ProviderContract: "src/main/java/example/JobManagementController.java",
		PrimaryPersistence: []string{
			"src/main/java/example/CatalogChangeJobRepository.java",
			"src/main/java/example/CatalogJobRepository.java",
		},
	}}

	if got := contextProductionPlanFiles(pack, index); !reflect.DeepEqual(got, want) {
		t.Fatalf("production plan files = %#v, want %#v", got, want)
	}
}

func TestContextProductionPlanFilesDoesNotGuessPersistenceWithoutDomainIntent(t *testing.T) {
	pack, index := productionPlanFilesFixture()
	pack.Query = "Plan the missing internal HTTP contract and identify the exact production and test files to change."
	pack.selectionQuery = pack.Query
	want := []ContextProductionPlanFiles{{
		Project:          "services/jobs",
		ProviderContract: "src/main/java/example/JobManagementController.java",
	}}

	if got := contextProductionPlanFiles(pack, index); !reflect.DeepEqual(got, want) {
		t.Fatalf("production plan without domain intent = %#v, want %#v", got, want)
	}
}

func TestContextProductionPlanFilesIgnoresFactOrder(t *testing.T) {
	pack, index := productionPlanFilesFixture()
	want := contextProductionPlanFiles(pack, index)
	slices.Reverse(index.Facts)

	if got := contextProductionPlanFiles(pack, index); !reflect.DeepEqual(got, want) {
		t.Fatalf("reversed production plan files = %#v, want %#v", got, want)
	}
}

func productionPlanFilesFixture() (ContextPack, scan.AgentContextIndexRecord) {
	query := "Plan the missing internal HTTP contract for deleting catalog jobs, cover both job types and their catalog and item lookup attributes, and identify the exact production and test files to change."
	pack := ContextPack{
		Query: query, selectionQuery: query,
		Entrypoints: []ContextLocation{{Project: "services/catalog"}},
		Concerns: []ContextConcern{
			{Project: "services/catalog", Kind: contextConcernEntrypoint, Covered: true},
			{Project: "services/jobs", Kind: contextConcernHTTPContract, Covered: true},
			{Project: "services/jobs", Kind: contextConcernPersistence, Covered: true},
		},
		Files: []ContextFile{{
			Project: "services/jobs", Path: "src/main/java/example/RepresentedManagementController.java",
		}},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "jobs-route", Project: "services/jobs", Kind: "route",
			Name: "GET /job-management/jobs", Qualified: "JobManagementController.listJobs",
			HTTPMethod: "GET", Path: "/job-management/jobs",
			File: "src/main/java/example/JobManagementController.java", Confidence: "EXACT",
			Search: "internal job management endpoint",
		},
		{
			ID: "jobs-controller", Project: "services/jobs", Kind: "symbol",
			Name: "JobManagementController", Qualified: "jobs.JobManagementController",
			File: "src/main/java/example/JobManagementController.java", Confidence: "EXACT",
			Search: "internal job management controller",
		},
		{
			ID: "represented-controller", Project: "services/jobs", Kind: "route",
			Name: "DELETE /job-management/jobs", Qualified: "RepresentedManagementController.deleteJobs",
			HTTPMethod: "DELETE", Path: "/job-management/jobs",
			File: "src/main/java/example/RepresentedManagementController.java", Confidence: "EXACT",
			Search: "internal job management endpoint",
		},
		{
			ID: "regular-repository", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobRepository", Qualified: "jobs.CatalogJobRepository",
			File: "src/main/java/example/CatalogJobRepository.java", Confidence: "EXACT",
			Search: "catalog job primary persistence repository",
		},
		{
			ID: "change-repository", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogChangeJobRepository", Qualified: "jobs.CatalogChangeJobRepository",
			File: "src/main/java/example/CatalogChangeJobRepository.java", Confidence: "EXACT",
			Search: "catalog change job primary persistence repository",
		},
		{
			ID: "comment-repository", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobCommentRepository", Qualified: "jobs.CatalogJobCommentRepository",
			File: "src/main/java/example/CatalogJobCommentRepository.java", Confidence: "EXACT",
			Search: "catalog job dependent comment persistence repository",
		},
		{
			ID: "generic-persistence", Project: "services/jobs", Kind: "persistence",
			Name: "findAll", Qualified: "OrphanRepository.findAll",
			File: "src/main/java/example/OrphanRepository.java", Confidence: "EXACT",
			Search: "generic persistence operation",
		},
		{
			ID: "test-repository", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobRepository", Qualified: "jobs.CatalogJobRepository",
			File: "src/test/java/example/CatalogJobRepository.java", Confidence: "EXACT",
			Search: "catalog job persistence test",
		},
		{
			ID: "foreign-repository", Project: "services/payments", Kind: "symbol",
			Name: "CatalogJobRepository", Qualified: "payments.CatalogJobRepository",
			File: "src/main/java/example/CatalogJobRepository.java", Confidence: "EXACT",
			Search: "catalog job primary persistence repository",
		},
		{
			ID: "absolute-repository", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobRepository", Qualified: "jobs.CatalogJobRepository",
			File: "/private/source/CatalogJobRepository.java", Confidence: "EXACT",
			Search: "catalog job primary persistence repository",
		},
	}}
	return pack, index
}
