package scan

import "testing"

func TestBuildWorkspaceEndpointTracesCreatesReadableResolvedTrace(t *testing.T) {
	matches := []WorkspaceContractMatchRecord{
		{
			ID:                "contract:get-user",
			APIProject:        "frontend/app",
			APIHTTPMethod:     "GET",
			APIPath:           "/users/{userId}",
			APIFile:           "src/api/users.ts",
			APILine:           12,
			APICaller:         "getUser",
			BackendProject:    "microservices/ms-user",
			BackendService:    "ms-user",
			BackendHTTPMethod: "GET",
			BackendPath:       "/users/{userId}",
			BackendHandler:    "UserController.get",
			BackendFile:       "src/main/java/UserController.java",
			BackendLine:       41,
			Confidence:        "RESOLVED",
			ConfidenceScore:   1,
		},
	}
	flows := []WorkspaceFeatureFlowRecord{
		{
			ID:                "flow:get-user",
			FrontendProject:   "frontend/app",
			FrontendFile:      "src/api/users.ts",
			FrontendLine:      12,
			FrontendCaller:    "getUser",
			HTTPMethod:        "GET",
			Path:              "/users/{userId}",
			BackendProject:    "microservices/ms-user",
			BackendService:    "ms-user",
			BackendFile:       "src/main/java/UserController.java",
			BackendLine:       41,
			BackendController: "UserController",
			BackendMethod:     "get",
			FrontendSteps: []CodeFlowStep{
				{Name: "UserPage.load", Kind: "component", File: "src/pages/UserPage.tsx", Line: 8, Confidence: "EXTRACTED"},
			},
			BackendSteps: []SpringEndpointFlowStep{
				{Owner: "UserService", Method: "find", Kind: "service", File: "src/main/java/UserService.java", Line: 20, Confidence: "EXTRACTED"},
			},
			Confidence: "RESOLVED",
		},
	}

	index := BuildWorkspaceEndpointTraces(matches, flows, nil)

	if len(index.Traces) != 1 {
		t.Fatalf("traces = %d, want 1: %#v", len(index.Traces), index.Traces)
	}
	trace := index.Traces[0]
	if trace.Route != "GET /users/{userId}" {
		t.Fatalf("route = %q", trace.Route)
	}
	if trace.FromProject != "frontend/app" || trace.ToProject != "microservices/ms-user" {
		t.Fatalf("trace direction = %s -> %s", trace.FromProject, trace.ToProject)
	}
	if trace.Status != "RESOLVED" {
		t.Fatalf("status = %q, want RESOLVED", trace.Status)
	}
	wantKinds := []string{"frontend_step", "api_contract", "backend_route", "backend_handler", "backend_step"}
	if len(trace.Steps) != len(wantKinds) {
		t.Fatalf("steps = %d, want %d: %#v", len(trace.Steps), len(wantKinds), trace.Steps)
	}
	for i, kind := range wantKinds {
		if trace.Steps[i].Kind != kind {
			t.Fatalf("step %d kind = %q, want %q: %#v", i, trace.Steps[i].Kind, kind, trace.Steps)
		}
	}
	if len(trace.Edges) != len(trace.Steps)-1 {
		t.Fatalf("edges = %d, want %d", len(trace.Edges), len(trace.Steps)-1)
	}
	if trace.Edges[0].Direction != "UserPage.load -> getUser" {
		t.Fatalf("first direction = %q", trace.Edges[0].Direction)
	}
}

func TestBuildWorkspaceEndpointTracesUsesServiceCandidateForUnresolvedTarget(t *testing.T) {
	matches := []WorkspaceContractMatchRecord{
		{
			ID:               "contract:availability",
			APIProject:       "frontend/app",
			APIHTTPMethod:    "GET",
			APIPath:          "/documentexport/modules/{isbn}/documents/{objectId}/availability",
			APIFile:          "src/api/export.ts",
			ServiceCandidate: "ms-documentexport",
			Confidence:       "UNRESOLVED",
			Issue:            "indexed_backend_route_missing",
		},
	}

	index := BuildWorkspaceEndpointTraces(matches, nil, nil)

	if len(index.Traces) != 1 {
		t.Fatalf("traces = %d, want 1", len(index.Traces))
	}
	if index.Traces[0].ToProject != "ms-documentexport" {
		t.Fatalf("to project = %q, want service candidate", index.Traces[0].ToProject)
	}
	if index.Traces[0].Risk != "indexed_backend_route_missing" {
		t.Fatalf("risk = %q", index.Traces[0].Risk)
	}
}
