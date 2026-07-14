package scan

import "testing"

func TestWorkspaceCoveragePrioritizesUsefulScans(t *testing.T) {
	context := WorkspaceContextRecord{
		Projects:              []WorkspaceProjectRecord{{Path: "services/a", Service: "a", Indexed: false}, {Path: "services/b", Service: "b", Indexed: false}},
		ReferencedServices:    []string{"a", "b"},
		MissingServiceDetails: []WorkspaceMissingServiceRecord{{Service: "b", Contracts: 2, Project: "services/b"}, {Service: "a", Contracts: 7, Project: "services/a"}},
	}
	got := BuildWorkspaceCoverage(context, WorkspaceContractSummaryRecord{Total: 9, MissingRoute: 9})
	if len(got.NextScans) != 2 || got.NextScans[0].Service != "a" {
		t.Fatalf("coverage=%#v", got)
	}
	if got.KnownProjects != 2 || got.IndexedProjects != 0 || got.ReferencedServices != 2 || got.IndexedReferencedServices != 0 {
		t.Fatalf("coverage counts=%#v", got)
	}
	if got.NextScans[0].Command != "goregraph scan services/a" {
		t.Fatalf("scan command=%q", got.NextScans[0].Command)
	}
}

func TestWorkspaceCoverageSortsTiesDeterministically(t *testing.T) {
	context := WorkspaceContextRecord{MissingServiceDetails: []WorkspaceMissingServiceRecord{{Service: "z", Contracts: 2, Project: "services/z"}, {Service: "a", Contracts: 2, Project: "services/a"}}}
	got := BuildWorkspaceCoverage(context, WorkspaceContractSummaryRecord{})
	if got.NextScans[0].Project != "services/a" {
		t.Fatalf("next scans=%#v", got.NextScans)
	}
}
