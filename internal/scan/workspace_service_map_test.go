package scan

import "testing"

func TestBuildWorkspaceServiceMapCreatesDirectedServiceEdges(t *testing.T) {
	registry := WorkspaceRegistryRecord{
		Root: "/workspace",
		Projects: []WorkspaceProjectRecord{
			{Name: "frontend", Path: "frontend/app", Kind: "frontend", Indexed: true},
			{Name: "ms-user", Path: "microservices/ms-user", Kind: "backend", Service: "ms-user", Indexed: true},
		},
	}
	matches := []WorkspaceContractMatchRecord{
		{
			ID:                "contract:get-user",
			APIProject:        "frontend/app",
			APIHTTPMethod:     "GET",
			APIPath:           "/users/{userId}",
			APIFile:           "src/api/users.ts",
			BackendProject:    "microservices/ms-user",
			BackendService:    "ms-user",
			BackendHTTPMethod: "GET",
			BackendPath:       "/users/{userId}",
			BackendHandler:    "UserController.get",
			Confidence:        "RESOLVED",
			ConfidenceScore:   1,
			Issue:             "",
		},
	}

	serviceMap := BuildWorkspaceServiceMap(registry, matches, nil, nil)

	if serviceMap.Root != "/workspace" {
		t.Fatalf("root = %q, want /workspace", serviceMap.Root)
	}
	if len(serviceMap.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2: %#v", len(serviceMap.Nodes), serviceMap.Nodes)
	}
	if len(serviceMap.Edges) != 1 {
		t.Fatalf("edges = %d, want 1: %#v", len(serviceMap.Edges), serviceMap.Edges)
	}
	edge := serviceMap.Edges[0]
	if edge.FromProject != "frontend/app" || edge.ToProject != "microservices/ms-user" {
		t.Fatalf("edge direction = %s -> %s, want frontend/app -> microservices/ms-user", edge.FromProject, edge.ToProject)
	}
	if edge.Direction != "frontend/app -> microservices/ms-user" {
		t.Fatalf("direction label = %q", edge.Direction)
	}
	if edge.Resolved != 1 || edge.Total != 1 {
		t.Fatalf("edge counts resolved/total = %d/%d, want 1/1", edge.Resolved, edge.Total)
	}
	if len(edge.Endpoints) != 1 || edge.Endpoints[0] != "GET /users/{userId}" {
		t.Fatalf("endpoints = %#v, want GET /users/{userId}", edge.Endpoints)
	}
}

func TestWorkspaceServiceMapCarriesCanonicalFeatureFlows(t *testing.T) {
	flow := BuildCanonicalFeatureFlow(WorkspaceFeatureFlowRecord{ID: "flow", FrontendProject: "web", FrontendFile: "api.ts", HTTPMethod: "GET", Path: "/users", BackendProject: "users", Confidence: "RESOLVED"})
	got := BuildWorkspaceServiceMap(WorkspaceRegistryRecord{}, nil, []WorkspaceFeatureFlowRecord{flow}, nil)
	if len(got.FeatureFlows) != 1 || got.FeatureFlows[0].ModelVersion != 1 {
		t.Fatalf("map=%#v", got)
	}
}

func TestBuildWorkspaceContractSummaryBalancesEveryContract(t *testing.T) {
	matches := make([]WorkspaceContractMatchRecord, 0, 178)
	appendMatches := func(count int, issue, confidence string) {
		for range count {
			matches = append(matches, WorkspaceContractMatchRecord{Issue: issue, Confidence: confidence})
		}
	}
	appendMatches(150, contractIssueMatched, "RESOLVED")
	appendMatches(20, contractIssueIndexedBackendRouteMissing, "UNRESOLVED")
	appendMatches(3, contractIssueMissingRoute, "UNRESOLVED")
	appendMatches(3, contractIssueMethodMismatch, "MISMATCH")
	appendMatches(1, contractIssueDynamicEndpointUnresolved, "UNRESOLVED")
	appendMatches(1, contractIssueFrontendInternalAPI, "OUT_OF_SCOPE")

	summary := BuildWorkspaceContractSummary(matches)
	if summary.Total != 178 || summary.Resolved != 150 || summary.MissingRoute != 23 ||
		summary.MethodMismatch != 3 || summary.DynamicUnresolved != 1 ||
		summary.OutOfScope != 1 || summary.Other != 0 {
		t.Fatalf("unexpected contract summary: %#v", summary)
	}
	accounted := summary.Resolved + summary.MissingRoute + summary.MethodMismatch +
		summary.DynamicUnresolved + summary.OutOfScope + summary.Other
	if summary.Total != accounted {
		t.Fatalf("contract total %d does not balance with categories %d", summary.Total, accounted)
	}
}

func TestBuildWorkspaceServiceMapPublishesContractSummary(t *testing.T) {
	serviceMap := BuildWorkspaceServiceMap(WorkspaceRegistryRecord{}, []WorkspaceContractMatchRecord{
		{Issue: contractIssueMatched, Confidence: "RESOLVED"},
		{Issue: contractIssueMethodMismatch, Confidence: "MISMATCH"},
	}, nil, nil)
	if serviceMap.ContractSummary.Total != 2 || serviceMap.ContractSummary.Resolved != 1 || serviceMap.ContractSummary.MethodMismatch != 1 {
		t.Fatalf("service map contract summary = %#v", serviceMap.ContractSummary)
	}
}

func TestBuildWorkspaceServiceMapAggregatesMismatchAndUnresolvedCounts(t *testing.T) {
	registry := WorkspaceRegistryRecord{
		Root: "/workspace",
		Projects: []WorkspaceProjectRecord{
			{Name: "frontend", Path: "frontend/app", Kind: "frontend", Indexed: true},
			{Name: "ms-product", Path: "microservices/ms-product", Kind: "backend", Service: "ms-product", Indexed: true},
		},
	}
	matches := []WorkspaceContractMatchRecord{
		{
			ID:                "contract:get-products",
			APIProject:        "frontend/app",
			APIHTTPMethod:     "GET",
			APIPath:           "/productservice/users/{userId}/products",
			BackendProject:    "microservices/ms-product",
			BackendHTTPMethod: "GET",
			BackendPath:       "/users/{userId}/products",
			Confidence:        "RESOLVED",
			ConfidenceScore:   1,
		},
		{
			ID:                "contract:put-service",
			APIProject:        "frontend/app",
			APIHTTPMethod:     "PUT",
			APIPath:           "/productservice/users/{userId}/services/{serviceCode}",
			BackendProject:    "microservices/ms-product",
			BackendHTTPMethod: "GET",
			BackendPath:       "/users/{userId}/services/{serviceCode}",
			Confidence:        "MISMATCH",
			Issue:             "method_mismatch",
		},
	}

	serviceMap := BuildWorkspaceServiceMap(registry, matches, nil, nil)

	if len(serviceMap.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(serviceMap.Edges))
	}
	edge := serviceMap.Edges[0]
	if edge.Total != 2 || edge.Resolved != 1 || edge.Mismatched != 1 || edge.Unresolved != 0 {
		t.Fatalf("counts total/resolved/mismatch/unresolved = %d/%d/%d/%d", edge.Total, edge.Resolved, edge.Mismatched, edge.Unresolved)
	}
	if edge.Risk != "has_mismatches" {
		t.Fatalf("risk = %q, want has_mismatches", edge.Risk)
	}
	if len(edge.Problems) != 1 || edge.Problems[0] != "PUT /productservice/users/{userId}/services/{serviceCode} - method_mismatch" {
		t.Fatalf("problems = %#v", edge.Problems)
	}
}

func TestBuildWorkspaceServiceMapMapsServiceCandidateToIndexedProject(t *testing.T) {
	registry := WorkspaceRegistryRecord{
		Root: "/workspace",
		Projects: []WorkspaceProjectRecord{
			{Name: "frontend", Path: "frontend/app", Kind: "frontend", Indexed: true},
			{Name: "ms-documentexport", Path: "microservices/ms-documentexport", Kind: "backend", Service: "ms-documentexport", Indexed: true},
		},
	}
	matches := []WorkspaceContractMatchRecord{
		{
			ID:               "contract:availability",
			APIProject:       "frontend/app",
			APIHTTPMethod:    "GET",
			APIPath:          "/documentexport/modules/{isbn}/documents/{objectId}/availability",
			ServiceCandidate: "ms-documentexport",
			Confidence:       "UNRESOLVED",
			Issue:            "indexed_backend_route_missing",
		},
	}

	serviceMap := BuildWorkspaceServiceMap(registry, matches, nil, nil)

	if len(serviceMap.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(serviceMap.Edges))
	}
	if serviceMap.Edges[0].ToProject != "microservices/ms-documentexport" {
		t.Fatalf("to project = %q, want indexed project path", serviceMap.Edges[0].ToProject)
	}
	if serviceMap.Edges[0].Unresolved != 1 || serviceMap.Edges[0].Risk != "has_unresolved" {
		t.Fatalf("unresolved edge not preserved: %#v", serviceMap.Edges[0])
	}
}

func TestBuildWorkspaceServiceMapKeepsAllFrontendProjectsVisible(t *testing.T) {
	registry := WorkspaceRegistryRecord{
		Root: "/workspace",
		Projects: []WorkspaceProjectRecord{
			{Name: "frontend", Path: "frontend/frontend", Kind: "frontend", Indexed: true},
			{Name: "frontend-monorepo", Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true},
			{Name: "frontends", Path: "frontend/frontends", Kind: "frontend", Indexed: true},
			{Name: "playwright", Path: "frontend/playwright", Kind: "frontend", Indexed: true},
			{Name: "shop-frontend-2024", Path: "frontend/shop-frontend-2024", Kind: "frontend", Indexed: true},
			{Name: "ms-user", Path: "microservices/ms-user", Kind: "backend", Service: "ms-user", Indexed: true},
		},
	}
	matches := []WorkspaceContractMatchRecord{
		{
			ID:                "contract:get-user",
			APIProject:        "frontend/frontend-monorepo",
			APIHTTPMethod:     "GET",
			APIPath:           "/users/{userId}",
			BackendProject:    "microservices/ms-user",
			BackendHTTPMethod: "GET",
			BackendPath:       "/users/{userId}",
			Confidence:        "RESOLVED",
		},
	}

	serviceMap := BuildWorkspaceServiceMap(registry, matches, nil, nil)

	for _, project := range []string{
		"frontend/frontend",
		"frontend/frontend-monorepo",
		"frontend/frontends",
		"frontend/playwright",
		"frontend/shop-frontend-2024",
	} {
		node := requireServiceMapNode(t, serviceMap, project)
		if node.Role != "frontend" {
			t.Fatalf("node %s role = %q, want frontend", project, node.Role)
		}
		if node.Domain != "frontend:frontend" || node.DomainSource != "workspace_path" || node.DomainConfidence != "PARTIAL" {
			t.Fatalf("node %s architecture fallback = %#v", project, node)
		}
	}
}

func TestBuildWorkspaceServiceMapUsesGenericArchitectureFallbacks(t *testing.T) {
	registry := WorkspaceRegistryRecord{
		Root: "/workspace",
		Projects: []WorkspaceProjectRecord{
			{Name: "ms-documenttopic", Path: "microservices/ms-documenttopic", Kind: "backend", Service: "ms-documenttopic", Indexed: true},
			{Name: "ms-cadaster", Path: "microservices/ms-cadaster", Kind: "backend", Service: "ms-cadaster", Indexed: true},
			{Name: "ms-userservice", Path: "microservices/ms-userservice", Kind: "backend", Service: "ms-userservice", Indexed: true},
			{Name: "tools", Path: "tools/importer", Kind: "backend", Indexed: true},
		},
	}

	serviceMap := BuildWorkspaceServiceMap(registry, nil, nil, nil)

	if node := requireServiceMapNode(t, serviceMap, "microservices/ms-documenttopic"); node.Domain != "backend:microservices" {
		t.Fatalf("document service fallback = %q", node.Domain)
	}
	if node := requireServiceMapNode(t, serviceMap, "microservices/ms-cadaster"); node.Domain != "backend:microservices" {
		t.Fatalf("cadaster service fallback = %q", node.Domain)
	}
	if node := requireServiceMapNode(t, serviceMap, "microservices/ms-userservice"); node.Domain != "backend:microservices" {
		t.Fatalf("user service fallback = %q", node.Domain)
	}
	if node := requireServiceMapNode(t, serviceMap, "tools/importer"); node.Domain != "backend:tools" {
		t.Fatalf("tools service fallback = %q", node.Domain)
	}
}

func TestBuildWorkspaceServiceMapIncludesBackendServiceDependencies(t *testing.T) {
	registry := WorkspaceRegistryRecord{
		Root: "/workspace",
		Projects: []WorkspaceProjectRecord{
			{Name: "ms-cadaster", Path: "microservices/ms-cadaster", Kind: "backend", Service: "ms-cadaster", Indexed: true},
			{Name: "ms-userservice", Path: "microservices/ms-userservice", Kind: "backend", Service: "ms-userservice", Indexed: true},
			{Name: "ms-licenseservice", Path: "microservices/ms-licenseservice", Kind: "backend", Service: "ms-licenseservice", Indexed: true},
		},
	}
	dependencies := []WorkspaceServiceDependencyRecord{
		{
			FromProject:   "microservices/ms-cadaster",
			ToService:     "ms-userservice",
			Kind:          "java_service_client",
			Evidence:      "src/main/java/CadasterController.java imports com.weka.common.userservice.UserMgmtService",
			Confidence:    "EXTRACTED",
			ResolutionKey: "userservice",
		},
		{
			FromProject:   "microservices/ms-cadaster",
			ToService:     "ms-licenseservice",
			Kind:          "java_service_client",
			Evidence:      "src/main/java/CadasterController.java imports com.weka.common.licenseservice.LicenseMgmtService",
			Confidence:    "EXTRACTED",
			ResolutionKey: "licenseservice",
		},
	}

	serviceMap := BuildWorkspaceServiceMap(registry, nil, nil, dependencies)

	userEdge := requireServiceMapEdge(t, serviceMap, "microservices/ms-cadaster", "microservices/ms-userservice")
	if userEdge.Total != 1 || userEdge.Resolved != 1 || userEdge.Risk != "resolved" {
		t.Fatalf("unexpected user edge: %#v", userEdge)
	}
	if len(userEdge.Evidence) != 1 || userEdge.Evidence[0] == "" {
		t.Fatalf("user edge should keep service-client evidence: %#v", userEdge)
	}
	licenseEdge := requireServiceMapEdge(t, serviceMap, "microservices/ms-cadaster", "microservices/ms-licenseservice")
	if licenseEdge.Total != 1 || licenseEdge.Resolved != 1 {
		t.Fatalf("unexpected license edge: %#v", licenseEdge)
	}
}

func requireServiceMapNode(t *testing.T, serviceMap WorkspaceServiceMapRecord, project string) WorkspaceServiceNodeRecord {
	t.Helper()
	for _, node := range serviceMap.Nodes {
		if node.Project == project {
			return node
		}
	}
	t.Fatalf("missing service map node for project %s in %#v", project, serviceMap.Nodes)
	return WorkspaceServiceNodeRecord{}
}

func requireServiceMapEdge(t *testing.T, serviceMap WorkspaceServiceMapRecord, fromProject, toProject string) WorkspaceServiceEdgeRecord {
	t.Helper()
	for _, edge := range serviceMap.Edges {
		if edge.FromProject == fromProject && edge.ToProject == toProject {
			return edge
		}
	}
	t.Fatalf("missing service map edge %s -> %s in %#v", fromProject, toProject, serviceMap.Edges)
	return WorkspaceServiceEdgeRecord{}
}
