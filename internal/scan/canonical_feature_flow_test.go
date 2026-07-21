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

func TestBuildCanonicalFeatureFlowMergesRepeatedNodesAndEdges(t *testing.T) {
	legacy := WorkspaceFeatureFlowRecord{
		ID:              "flow:repeated-transition",
		FrontendProject: "web",
		FrontendSteps: []CodeFlowStep{
			{Name: "A", Kind: "frontend_step", File: "src/flow.ts", Line: 10, Confidence: "EXACT", Reason: "step A", EvidenceIDs: []string{"evidence:a-2"}},
			{Name: "B", Kind: "frontend_step", File: "src/flow.ts", Line: 20, Confidence: "EXACT", Reason: "step B", EvidenceIDs: []string{"evidence:b-2"}},
			{Name: "C", Kind: "frontend_step", File: "src/flow.ts", Line: 30, Confidence: "EXACT", Reason: "step C", EvidenceIDs: []string{"evidence:c"}},
			{Name: "A", Kind: "frontend_step", File: "src/flow.ts", Line: 10, Confidence: "EXACT", Reason: "step A", EvidenceIDs: []string{"evidence:a-1"}},
			{Name: "B", Kind: "frontend_step", File: "src/flow.ts", Line: 20, Confidence: "EXACT", Reason: "step B", EvidenceIDs: []string{"evidence:b-1"}},
		},
		FrontendFile:      "src/api.ts",
		FrontendLine:      40,
		FrontendCaller:    "loadItems",
		HTTPMethod:        "GET",
		Path:              "/items",
		BackendProject:    "services/items",
		BackendService:    "items",
		BackendController: "ItemController",
		BackendMethod:     "list",
		BackendFile:       "ItemController.java",
		BackendLine:       20,
		Confidence:        "RESOLVED",
		Reason:            "matched contract",
	}

	first := BuildCanonicalFeatureFlow(legacy)
	second := BuildCanonicalFeatureFlow(legacy)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("canonical projection is not deterministic\nfirst=%#v\nsecond=%#v", first, second)
	}
	if len(first.Nodes) != 5 {
		t.Fatalf("nodes=%#v, want five unique nodes", first.Nodes)
	}
	if got := []string{first.Nodes[0].Symbol, first.Nodes[1].Symbol, first.Nodes[2].Symbol}; !reflect.DeepEqual(got, []string{"A", "B", "C"}) {
		t.Fatalf("first-occurrence node order=%#v", got)
	}
	if !reflect.DeepEqual(first.Nodes[0].EvidenceIDs, []string{"evidence:a-1", "evidence:a-2"}) {
		t.Fatalf("node A evidence=%#v", first.Nodes[0].EvidenceIDs)
	}
	if !reflect.DeepEqual(first.Nodes[1].EvidenceIDs, []string{"evidence:b-1", "evidence:b-2"}) {
		t.Fatalf("node B evidence=%#v", first.Nodes[1].EvidenceIDs)
	}
	if len(first.Edges) != 5 {
		t.Fatalf("edges=%#v, want five unique transitions", first.Edges)
	}
	wantTransitions := [][2]string{
		{first.Nodes[0].ID, first.Nodes[1].ID},
		{first.Nodes[1].ID, first.Nodes[2].ID},
		{first.Nodes[2].ID, first.Nodes[0].ID},
		{first.Nodes[1].ID, first.Nodes[3].ID},
		{first.Nodes[3].ID, first.Nodes[4].ID},
	}
	for index, want := range wantTransitions {
		if got := [2]string{first.Edges[index].FromNodeID, first.Edges[index].ToNodeID}; got != want {
			t.Fatalf("edge %d transition=%#v, want %#v", index, got, want)
		}
	}
	if !reflect.DeepEqual(first.Edges[0].EvidenceIDs, []string{"evidence:b-1", "evidence:b-2"}) {
		t.Fatalf("repeated edge evidence=%#v", first.Edges[0].EvidenceIDs)
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

func TestValidateCanonicalFeatureFlowRejectsDuplicateEdges(t *testing.T) {
	flow := WorkspaceFeatureFlowRecord{
		ModelVersion: 1,
		Nodes: []CanonicalFlowNodeRecord{
			{ID: "node:a", Kind: "frontend_step"},
			{ID: "node:b", Kind: "frontend_step"},
		},
		Edges: []CanonicalFlowEdgeRecord{
			{ID: "edge:a-b", FromNodeID: "node:a", ToNodeID: "node:b", EdgeType: "calls", Confidence: "EXACT", Reason: "test edge"},
			{ID: "edge:a-b", FromNodeID: "node:a", ToNodeID: "node:b", EdgeType: "calls", Confidence: "EXACT", Reason: "test edge"},
		},
	}
	if err := ValidateCanonicalFeatureFlow(flow); err == nil {
		t.Fatal("duplicate canonical edge accepted")
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
