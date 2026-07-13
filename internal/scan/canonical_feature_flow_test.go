package scan

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildCanonicalFeatureFlowUsesStableReferences(t *testing.T) {
	legacy := WorkspaceFeatureFlowRecord{
		ID: "flow:users", FrontendProject: "web", FrontendFile: "src/users.ts", FrontendLine: 8,
		FrontendCaller: "loadUsers", HTTPMethod: "GET", Path: "/users", BackendProject: "services/users",
		BackendService: "users", BackendController: "UserController", BackendMethod: "list",
		BackendFile: "UserController.java", BackendLine: 20, Confidence: "RESOLVED", Reason: "matched contract",
	}
	first := BuildCanonicalFeatureFlow(legacy)
	second := BuildCanonicalFeatureFlow(legacy)
	if first.ModelVersion != 1 || len(first.Nodes) < 2 || len(first.Edges) == 0 {
		t.Fatalf("flow=%#v", first)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("canonical projection is not deterministic\nfirst=%#v\nsecond=%#v", first, second)
	}
	if err := ValidateCanonicalFeatureFlow(first); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCanonicalFeatureFlowRejectsDanglingEdges(t *testing.T) {
	flow := WorkspaceFeatureFlowRecord{ModelVersion: 1, Nodes: []CanonicalFlowNodeRecord{{ID: "node:a", Kind: "api_call"}}, Edges: []CanonicalFlowEdgeRecord{{ID: "edge:a", FromNodeID: "node:a", ToNodeID: "node:missing", EdgeType: "invokes_api", Confidence: "RESOLVED", Reason: "matched contract"}}}
	if err := ValidateCanonicalFeatureFlow(flow); err == nil {
		t.Fatal("dangling canonical edge accepted")
	}
}

func TestWorkspaceFeatureFlowLegacyJSONRemainsCompatible(t *testing.T) {
	var flow WorkspaceFeatureFlowRecord
	if err := json.Unmarshal([]byte(`{"id":"legacy","frontend_project":"web","frontend_file":"api.ts","http_method":"GET","path":"/users","backend_project":"users","confidence":"RESOLVED"}`), &flow); err != nil {
		t.Fatal(err)
	}
	if flow.ID != "legacy" || flow.ModelVersion != 0 || len(flow.Nodes) != 0 || len(flow.Edges) != 0 {
		t.Fatalf("legacy flow changed=%#v", flow)
	}
	if err := ValidateCanonicalFeatureFlow(flow); err != nil {
		t.Fatalf("legacy flow should remain valid: %v", err)
	}
}
