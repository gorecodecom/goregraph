package scan

import "testing"

func TestBuildDataFlowsKeepsKnownFieldsAndExplicitGaps(t *testing.T) {
	flows := []WorkspaceFeatureFlowRecord{{ID: "flow:1", HTTPMethod: "POST", Path: "/users", FrontendProject: "web", BackendProject: "users", BackendRequestFields: []DTOFieldRecord{{Name: "email", Type: "string", Confidence: "EXTRACTED"}}, PersistencePath: []PersistenceStepRecord{{Repository: "UserRepository", Method: "save", Entity: "User", File: "repo.go", Line: 4, Confidence: "MATCHED"}}, BackendResponseFields: []DTOFieldRecord{{Name: "id", Type: "string", Confidence: "EXTRACTED"}}}}
	records := BuildDataFlows(flows)
	if len(records) != 1 {
		t.Fatalf("len=%d", len(records))
	}
	record := records[0]
	if len(record.Nodes) < 4 || len(record.Edges) < 3 {
		t.Fatalf("incomplete data flow: %#v", record)
	}
	if len(record.Gaps) == 0 {
		t.Fatalf("unknown transformation was not represented as a gap: %#v", record)
	}
	for _, edge := range record.Edges {
		if edge.Confidence == "" {
			t.Fatalf("edge lacks confidence: %#v", edge)
		}
	}
}
