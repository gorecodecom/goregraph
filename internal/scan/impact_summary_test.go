package scan

import "testing"

func TestBuildImpactSummariesSeparatesDirectAndIndirectConsumers(t *testing.T) {
	flows := []WorkspaceFeatureFlowRecord{{ID: "a", HTTPMethod: "GET", Path: "/users", Nodes: []CanonicalFlowNodeRecord{{ID: "route", Kind: "frontend_route", Project: "web"}, {ID: "api", Kind: "api_call", Project: "web"}, {ID: "endpoint", Kind: "endpoint", Project: "users"}, {ID: "test", Kind: "test", Project: "users"}}, Edges: []CanonicalFlowEdgeRecord{{ID: "r-a", FromNodeID: "route", ToNodeID: "api", EdgeType: "calls", Confidence: "INFERRED", Reason: "route flow"}, {ID: "a-e", FromNodeID: "api", ToNodeID: "endpoint", EdgeType: "invokes_api", Confidence: "RESOLVED", Reason: "matched contract"}, {ID: "e-t", FromNodeID: "endpoint", ToNodeID: "test", EdgeType: "verified_by", Confidence: "EXACT", Reason: "test map"}}}}
	got := BuildImpactSummaries(flows, WorkspaceServiceMapRecord{}, WorkspaceCoverageSummaryRecord{}, 2)
	if len(got) == 0 || len(got[0].DirectConsumers) == 0 || len(got[0].IndirectConsumers) == 0 || len(got[0].DependentTests) == 0 || got[0].RiskLevel == "" {
		t.Fatalf("impact=%#v", got)
	}
}

func TestBuildImpactSummariesBoundsCyclesAndReportsCoverageUncertainty(t *testing.T) {
	flow := WorkspaceFeatureFlowRecord{ID: "cycle", Nodes: []CanonicalFlowNodeRecord{{ID: "a", Kind: "api_call"}, {ID: "b", Kind: "endpoint"}}, Edges: []CanonicalFlowEdgeRecord{{ID: "a-b", FromNodeID: "a", ToNodeID: "b", EdgeType: "invokes_api", Confidence: "RESOLVED", Reason: "match"}, {ID: "b-a", FromNodeID: "b", ToNodeID: "a", EdgeType: "callback", Confidence: "INFERRED", Reason: "callback"}}}
	coverage := WorkspaceCoverageSummaryRecord{KnownProjects: 2, IndexedProjects: 1, ReferencedServices: 2, IndexedReferencedServices: 1}
	got := BuildImpactSummaries([]WorkspaceFeatureFlowRecord{flow}, WorkspaceServiceMapRecord{}, coverage, 1)
	if len(got) != 1 || len(got[0].IndirectConsumers) != 0 || len(got[0].CoverageUncertainty) == 0 {
		t.Fatalf("impact=%#v", got)
	}
}
