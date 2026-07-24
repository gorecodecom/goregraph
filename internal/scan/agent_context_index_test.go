package scan

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestBuildProjectAgentContextIndexIsCompactAndDeterministic(t *testing.T) {
	routes := []CodeRouteRecord{{
		RouteID: "route:delete", HTTPMethod: "DELETE",
		Path:    "/cadasters/{cadasterId}/regulations/{objectId}",
		Handler: "CadasterRegulationController.deleteFromCadaster",
		File:    "src/CadasterRegulationController.java", Line: 182,
		Confidence: "EXACT", EvidenceIDs: []string{"evidence:route"},
	}}
	symbols := []RichSymbolRecord{
		{
			ID: "symbol:controller", Name: "deleteFromCadaster",
			QualifiedName: "CadasterRegulationController.deleteFromCadaster",
			Kind:          "method", Language: "java",
			File: "src/CadasterRegulationController.java", Line: 182,
			Confidence: ConfidenceExact,
		},
		{
			ID: "symbol:operations", Name: "deleteRegulationFromCadaster",
			QualifiedName: "CadasterRegulationOperationsService.deleteRegulationFromCadaster",
			Kind:          "method", Language: "java",
			File: "src/CadasterRegulationOperationsService.java", Line: 45,
			Confidence: ConfidenceExact, EvidenceIDs: []string{"evidence:symbol"},
		},
		{
			ID: "symbol:local-helper", Name: "formatDebugMessage",
			QualifiedName: "CadasterRegulationOperationsService.formatDebugMessage",
			Kind:          "method", Language: "java",
			File: "src/CadasterRegulationOperationsService.java", Line: 90,
			Confidence: ConfidenceExact,
		},
	}
	relations := []RichRelationRecord{{
		ID: "relation:delete", From: "src/CadasterRegulationController.java",
		To:   "CadasterRegulationOperationsService.deleteRegulationFromCadaster",
		Type: "call", FromSymbolID: "symbol:controller", ToSymbolID: "symbol:operations",
		Line: 201, Confidence: "EXACT",
		Reason: "qualified method call", EvidenceIDs: []string{"evidence:call"},
	}}

	first := BuildProjectAgentContextIndex("ms-cadasterregulation", "2026-07-16T00:00:00Z",
		routes, nil, symbols, relations, nil, nil, nil, nil)
	second := BuildProjectAgentContextIndex("ms-cadasterregulation", "2026-07-16T00:00:00Z",
		routes, nil, symbols, relations, nil, nil, nil, nil)

	if diff := cmpJSON(first, second); diff != "" {
		t.Fatalf("context index is not deterministic: %s", diff)
	}
	if len(first.Facts) != 3 || len(first.Edges) != 1 {
		t.Fatalf("context index = %#v", first)
	}
	if hasContextFact(first.Facts, "symbol", "formatDebugMessage") {
		t.Fatalf("isolated local helper leaked into compact context: %#v", first.Facts)
	}
	for _, fact := range first.Facts {
		if fact.Search == "" {
			t.Fatalf("fact missing compact search aliases: %#v", fact)
		}
	}
	route := findContextFact(first.Facts, "route", "DELETE /cadasters/{cadasterId}/regulations/{objectId}")
	for _, alias := range []string{"DELETE", "cadasters", "cadaster", "Id", "delete", "From", "CadasterRegulationController.java"} {
		if !strings.Contains(route.Search, alias) {
			t.Fatalf("route search %q missing alias %q", route.Search, alias)
		}
	}
	if !slices.Equal(route.EvidenceIDs, []string{"evidence:route"}) {
		t.Fatalf("route evidence ids = %#v", route.EvidenceIDs)
	}
}

func TestBuildProjectAgentContextIndexDoesNotPromoteUnrelatedMethodRelations(t *testing.T) {
	routes := []CodeRouteRecord{{
		RouteID: "route:users", HTTPMethod: "GET", Path: "/users",
		Handler: "UserController.list", File: "src/UserController.java", Line: 20,
	}}
	symbols := []RichSymbolRecord{
		{
			ID: "route-caller", Name: "list", QualifiedName: "UserController.list",
			Kind: "method", Language: "java", File: "src/UserController.java", Line: 20,
		},
		{
			ID: "route-target", Name: "findUsers", QualifiedName: "UserService.findUsers",
			Kind: "method", Language: "java", File: "src/UserService.java", Line: 30,
		},
		{
			ID: "unrelated-caller", Name: "refreshCache", QualifiedName: "CacheJob.refreshCache",
			Kind: "method", Language: "java", File: "src/CacheJob.java", Line: 40,
		},
		{
			ID: "unrelated-target", Name: "evictExpired", QualifiedName: "CacheService.evictExpired",
			Kind: "method", Language: "java", File: "src/CacheService.java", Line: 50,
		},
	}
	relations := []RichRelationRecord{
		{
			ID: "route-call", From: "src/UserController.java", To: "UserService.findUsers",
			Type: "call", FromSymbolID: "route-caller", ToSymbolID: "route-target", Line: 25,
		},
		{
			ID: "unrelated-call", From: "src/CacheJob.java", To: "CacheService.evictExpired",
			Type: "call", FromSymbolID: "unrelated-caller", ToSymbolID: "unrelated-target", Line: 45,
		},
	}

	first := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		routes, nil, symbols, relations, nil, nil, nil, nil)
	reversedRelations := slices.Clone(relations)
	slices.Reverse(reversedRelations)
	second := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		routes, nil, symbols, reversedRelations, nil, nil, nil, nil)
	if diff := cmpJSON(first, second); diff != "" {
		t.Fatalf("relation order changed compact context: %s", diff)
	}

	if !hasContextFact(first.Facts, "symbol", "findUsers") {
		t.Fatalf("route call target missing: %#v", first.Facts)
	}
	for _, name := range []string{"refreshCache", "evictExpired"} {
		if hasContextFact(first.Facts, "symbol", name) {
			t.Fatalf("unrelated relation symbol %q leaked: %#v", name, first.Facts)
		}
	}
	if len(first.Edges) != 1 || first.Edges[0].ToLabel != "UserService.findUsers" {
		t.Fatalf("compact relation edges = %#v", first.Edges)
	}
}

func TestBuildProjectAgentContextIndexDoesNotTreatSourceLessHelperCallAsRouteCall(t *testing.T) {
	routes := []CodeRouteRecord{
		{
			RouteID: "route:users", HTTPMethod: "GET", Path: "/users",
			Handler: "UserController.list", File: "src/UserController.java", Line: 20,
		},
		{
			RouteID: "route:user", HTTPMethod: "GET", Path: "/users/{id}",
			Handler: "UserController.get", File: "src/UserController.java", Line: 100,
		},
	}
	symbols := []RichSymbolRecord{{
		ID: "helper-target", Name: "evictExpired", QualifiedName: "CacheService.evictExpired",
		Kind: "method", Language: "java", File: "src/CacheService.java", Line: 50,
	}}
	relations := []RichRelationRecord{{
		ID: "helper-call", From: "src/UserController.java", To: "CacheService.evictExpired",
		Type: "call", ToSymbolID: "helper-target", Line: 70,
	}}

	index := BuildProjectAgentContextIndex(
		"app",
		"2026-07-16T00:00:00Z",
		routes,
		nil,
		symbols,
		relations,
		nil,
		nil,
		nil,
		nil,
	)
	if hasContextFact(index.Facts, "symbol", "evictExpired") {
		t.Fatalf("source-less helper call leaked into route context: %#v", index)
	}
}

func TestBuildProjectAgentContextIndexKeepsClassAndUsageNavigation(t *testing.T) {
	symbols := []RichSymbolRecord{
		{ID: "java-class", Name: "UserService", QualifiedName: "com.example.UserService", Kind: "class", Language: "java", File: "backend/UserService.java", Line: 10},
		{ID: "kotlin-annotation", Name: "Audited", QualifiedName: "com.example.Audited", Kind: "annotation", Language: "kotlin", File: "backend/Audited.kt", Line: 4},
		{ID: "ts-class", Name: "UserClient", QualifiedName: "src/user#UserClient", Kind: "class", Language: "typescript", ExportName: "UserClient", File: "frontend/user.ts", Line: 3},
		{ID: "ts-type", Name: "UserDTO", QualifiedName: "src/user#UserDTO", Kind: "type", Language: "typescript", ExportName: "UserDTO", File: "frontend/user.ts", Line: 8},
		{ID: "ts-component", Name: "UserCard", QualifiedName: "src/UserCard#UserCard", Kind: "component", Language: "typescript", ExportName: "UserCard", File: "frontend/UserCard.tsx", Line: 7},
		{ID: "js-hook", Name: "useUser", QualifiedName: "src/useUser#useUser", Kind: "function", Language: "javascript", ExportName: "useUser", File: "frontend/useUser.js", Line: 5},
		{ID: "private-ts-class", Name: "PrivateCache", QualifiedName: "src/user#PrivateCache", Kind: "class", Language: "typescript", File: "frontend/user.ts", Line: 20},
		{ID: "local-function", Name: "formatUser", QualifiedName: "src/user#formatUser", Kind: "function", Language: "typescript", File: "frontend/user.ts", Line: 30},
	}
	relations := []RichRelationRecord{{
		ID: "usage", From: "frontend/user.ts", To: "com.example.UserService", Type: "use",
		FromSymbolID: "ts-class", ToSymbolID: "java-class", Line: 14, Confidence: "EXACT",
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, symbols, relations, nil, nil, nil, nil)

	for _, name := range []string{"UserService", "Audited", "UserClient", "UserDTO", "UserCard", "useUser"} {
		if !hasContextFact(index.Facts, "symbol", name) {
			t.Fatalf("navigation fact %q missing: %#v", name, index.Facts)
		}
	}
	for _, name := range []string{"PrivateCache", "formatUser"} {
		if hasContextFact(index.Facts, "symbol", name) {
			t.Fatalf("non-navigation symbol %q leaked: %#v", name, index.Facts)
		}
	}
	if len(index.Edges) != 1 || index.Edges[0].FromFactID == "" || index.Edges[0].ToFactID == "" {
		t.Fatalf("cross-file symbol usage edge = %#v", index.Edges)
	}
}

func TestBuildProjectAgentContextIndexCompactsFlowsTestsContractsAndCoverage(t *testing.T) {
	routes := []CodeRouteRecord{{
		Kind: "backend", HTTPMethod: "POST", Path: "/users",
		Handler: "UserController.create", File: "backend/UserController.java", Line: 20,
	}}
	flows := []CodeFlowRecord{{
		HTTPMethod: "POST", Path: "/users", Handler: "UserController.create",
		File: "backend/UserController.java", Line: 20,
		Steps: []CodeFlowStep{
			{Name: "UserController.create", Kind: "handler", File: "backend/UserController.java", Line: 20, Confidence: "EXACT"},
			{Name: "UserRepository.save", Kind: "repository", File: "backend/UserRepository.java", Line: 42, Confidence: "EXACT", EvidenceIDs: []string{"evidence:persistence"}},
		},
	}}
	tests := []TestMapRecord{{
		TestFile: "backend/UserControllerTest.java", TestClass: "UserControllerTest",
		TestMethod: "createsUser", TargetClass: "UserController", TargetMethod: "create",
		HTTPMethod: "POST", Path: "/users", Type: "endpoint", Line: 15, Confidence: "EXACT",
	}}
	contracts := []APIContractRecord{{
		Language: "typescript", HTTPMethod: "POST", Path: "/users", Caller: "createUser",
		File: "frontend/api.ts", Line: 12, Confidence: "EXACT",
	}}
	evidence := []EvidenceRecord{{
		ID: "evidence:test", Project: "app", File: "backend/UserControllerTest.java",
		Start: EvidenceLocation{Line: 15}, Reason: "must not be embedded", SourceHash: "secret-hash",
	}, {
		ID: "evidence:contract", Project: "app", File: "frontend/api.ts",
		Start: EvidenceLocation{Line: 12}, Reason: "must not be embedded", SourceHash: "secret-hash",
	}}
	capabilities := []CapabilityRecord{
		{ID: CapabilityRoutes, Project: "app", Language: "java", Coverage: CoverageComplete, Reason: "routes covered"},
		{ID: CapabilityCalls, Project: "app", Language: "java", Coverage: CoveragePartial, Reason: "calls partial"},
		{ID: CapabilityTests, Project: "app", Language: "java", Coverage: CoverageComplete, Reason: "tests covered"},
		{ID: CapabilityAPIClients, Project: "app", Language: "typescript", Coverage: CoverageComplete, Reason: "clients covered"},
		{ID: CapabilityPersistence, Project: "app", Language: "java", Coverage: CoverageComplete, Reason: "persistence covered"},
		{ID: CapabilityRoutes, Project: "app", Language: "maven", Coverage: CoverageUnavailable, Reason: "build row"},
		{ID: CapabilityCalls, Project: "app", Language: "yaml", Coverage: CoverageUnavailable, Reason: "configuration row"},
		{ID: CapabilityMessaging, Project: "app", Language: "java", Coverage: CoverageComplete, Reason: "out of scope"},
	}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		routes, flows, nil, nil, tests, contracts, evidence, capabilities)

	for _, key := range [][2]string{
		{"persistence", "UserRepository.save"},
		{"test", "createsUser"},
		{"api_contract", "POST /users"},
	} {
		if !hasContextFact(index.Facts, key[0], key[1]) {
			t.Fatalf("compact fact %v missing: %#v", key, index.Facts)
		}
	}
	if len(index.Edges) != 3 {
		t.Fatalf("semantic flow/test/http edges = %#v", index.Edges)
	}
	if len(index.Coverage) != 5 {
		t.Fatalf("compact coverage = %#v", index.Coverage)
	}
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"must not be embedded", "secret-hash", `"evidence":`} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("context index leaked evidence payload %q: %s", forbidden, body)
		}
	}
	for _, id := range []string{"evidence:test", "evidence:contract", "evidence:persistence"} {
		if !strings.Contains(string(body), id) {
			t.Fatalf("context index missing evidence id %q: %s", id, body)
		}
	}
}

func TestProjectAgentContextIndexPreservesContractOperationalSignals(t *testing.T) {
	index := BuildProjectAgentContextIndex(
		"libraries/jobs",
		"fixed",
		nil,
		nil,
		[]RichSymbolRecord{{
			ID: "client", Name: "JobMgmtClient",
			QualifiedName: "example.JobMgmtClient",
			Kind:          "class", File: "src/JobMgmtClient.java", Line: 10,
		}},
		nil,
		nil,
		[]APIContractRecord{{
			HTTPMethod: "GET",
			Caller:     "JobMgmtClient.getJobs",
			File:       "src/JobMgmtClient.java",
			Line:       42,
			Confidence: "PARTIAL",
			Reason:     "spring RestClient receiver with unresolved dynamic path; retryable method",
			Auth: []AuthRecord{{
				Kind:       "basic",
				Expression: "service-user,super-secret",
				Source:     "spring_client_interceptor",
				Confidence: "EXTRACTED",
			}},
		}},
		nil,
		nil,
	)

	fact := findContextFactByQualified(index.Facts, "JobMgmtClient.getJobs")
	if fact.Kind != "api_contract" ||
		!strings.Contains(fact.Summary, "auth basic") ||
		!strings.Contains(fact.Summary, "retryable") ||
		!strings.Contains(fact.Search, "basic") ||
		!strings.Contains(fact.Search, "retryable") {
		t.Fatalf("contract operational signals = %#v", fact)
	}
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "service-user") ||
		strings.Contains(string(body), "super-secret") {
		t.Fatalf("compact context leaked credential expressions: %s", body)
	}
}

func TestProjectAgentContextLinksJavaCallerToContract(t *testing.T) {
	const contractPath = "/job-management/catalogs/{catalogId}/items/{itemId}"
	symbols := []RichSymbolRecord{
		{
			ID: "symbol:catalog-operations", Name: "deleteItem",
			QualifiedName: "CatalogOperations.deleteItem", Kind: "method", Language: "java",
			File: "src/main/java/example/CatalogOperations.java", Line: 14,
		},
		{
			ID: "symbol:job-client", Name: "deleteRelatedJobs",
			QualifiedName: "JobClient.deleteRelatedJobs", Kind: "method", Language: "java",
			File: "src/main/java/example/JobClient.java", Line: 12,
		},
		{
			ID: "symbol:health-operations", Name: "checkHealth",
			QualifiedName: "HealthOperations.checkHealth", Kind: "method", Language: "java",
			File: "src/main/java/example/HealthOperations.java", Line: 8,
		},
		{
			ID: "symbol:health-client", Name: "health",
			QualifiedName: "HealthClient.health", Kind: "method", Language: "java",
			File: "src/main/java/example/HealthClient.java", Line: 6,
		},
	}
	relations := []RichRelationRecord{
		{
			ID: "relation:catalog-to-jobs", From: "src/main/java/example/CatalogOperations.java",
			To: "JobClient.deleteRelatedJobs", Type: "call", Language: "java", Line: 20,
			FromSymbolID: "symbol:catalog-operations", ToSymbolID: "symbol:job-client",
			Resolution: SymbolResolutionExact, Confidence: "RESOLVED",
		},
		{
			ID: "relation:health-check", From: "src/main/java/example/HealthOperations.java",
			To: "HealthClient.health", Type: "call", Language: "java", Line: 10,
			FromSymbolID: "symbol:health-operations", ToSymbolID: "symbol:health-client",
			Resolution: SymbolResolutionExact, Confidence: "EXACT",
		},
	}
	contracts := []APIContractRecord{
		{
			Language: "java", HTTPMethod: "DELETE", Path: contractPath,
			Caller: "JobClient.deleteRelatedJobs", File: "src/main/java/example/JobClient.java",
			Line: 13, Confidence: "EXACT",
		},
		{
			Language: "java", HTTPMethod: "GET", Path: "/health",
			Caller: "HealthClient.health", File: "src/main/java/example/HealthClient.java",
			Line: 7, Confidence: "EXACT",
		},
	}

	build := func(symbols []RichSymbolRecord, relations []RichRelationRecord, contracts []APIContractRecord) AgentContextIndexRecord {
		t.Helper()
		return BuildProjectAgentContextIndex(
			"libraries/shared-model", "fixed", nil, nil, symbols, relations, nil, contracts, nil, nil,
		)
	}
	forward := build(symbols, relations, contracts)
	reversedSymbols := slices.Clone(symbols)
	slices.Reverse(reversedSymbols)
	reversedRelations := slices.Clone(relations)
	slices.Reverse(reversedRelations)
	reversedContracts := slices.Clone(contracts)
	slices.Reverse(reversedContracts)
	backward := build(reversedSymbols, reversedRelations, reversedContracts)

	if diff := cmpJSON(forward, backward); diff != "" {
		t.Fatalf("Java caller-contract context depends on input order: %s", diff)
	}
	operations := findContextFact(forward.Facts, "symbol", "deleteItem")
	client := findContextFact(forward.Facts, "symbol", "deleteRelatedJobs")
	contract := findContextFact(forward.Facts, "api_contract", "DELETE "+contractPath)
	for name, fact := range map[string]AgentContextFactRecord{
		"catalog operations": operations,
		"job client":         client,
		"API contract":       contract,
	} {
		if fact.ID == "" {
			t.Fatalf("%s fact missing: %#v", name, forward.Facts)
		}
	}
	if !hasContextEdge(forward.Edges, operations.ID, client.ID, "call") {
		t.Fatalf("catalog caller-to-client edge missing: %#v", forward.Edges)
	}
	if !hasContextEdge(forward.Edges, client.ID, contract.ID, "call") {
		t.Fatalf("Java client-to-contract edge missing: %#v", forward.Edges)
	}
	for _, edge := range forward.Edges {
		if edge.ID == "" || edge.FromFactID == "" || edge.ToFactID == "" {
			t.Fatalf("unstable Java caller-contract edge identity: %#v", edge)
		}
	}
}

func TestBuildProjectAgentContextIndexNormalizesRoutesAndFlowTransitions(t *testing.T) {
	routes := []CodeRouteRecord{{
		Kind: "backend", HTTPMethod: "delete", Path: "users/{id}",
		Handler: "UserController.deleteUser", File: `src\UserController.java`, Line: 10,
	}}
	flows := []CodeFlowRecord{{
		HTTPMethod: "DELETE", Path: "/users/{id}", Handler: "UserController.deleteUser",
		File: "src/UserController.java", Line: 10,
		Steps: []CodeFlowStep{
			{Name: "deleteUser", Owner: "UserController", Kind: "route_handler", File: "src/UserController.java", Line: 10},
			{Name: "restoreState", Owner: "UserService", Kind: "service_method", File: "src/UserService.java", Line: 20},
			{Name: "save", Owner: "UserRepository", Kind: "repository_method", File: "src/UserRepository.java", Line: 30},
		},
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		routes, flows, nil, nil, nil, nil, nil, nil)

	route := findContextFact(index.Facts, "route", "DELETE /users/{id}")
	if route.ID == "" || route.HTTPMethod != "DELETE" || route.Path != "/users/{id}" || route.File != "src/UserController.java" {
		t.Fatalf("normalized route = %#v", route)
	}
	if hasContextFact(index.Facts, "symbol", "deleteUser") {
		t.Fatalf("route handler step duplicated route fact: %#v", index.Facts)
	}
	if !hasContextFact(index.Facts, "symbol", "restoreState") {
		t.Fatalf("restoreState should remain an ordinary symbol: %#v", index.Facts)
	}
	if !hasContextFact(index.Facts, "persistence", "save") {
		t.Fatalf("repository step missing persistence fact: %#v", index.Facts)
	}
	if len(index.Edges) != 2 || index.Edges[0].Kind != "call" || index.Edges[1].Kind != "persistence" {
		t.Fatalf("flow transitions = %#v", index.Edges)
	}
}

func TestBuildProjectAgentContextIndexPreservesFlowBranches(t *testing.T) {
	routes := []CodeRouteRecord{{
		Kind: "backend", HTTPMethod: "DELETE", Path: "/tasks/{id}",
		Handler: "TaskController.deleteTask", File: "src/TaskController.java", Line: 10,
	}}
	flows := []CodeFlowRecord{{
		HTTPMethod: "DELETE", Path: "/tasks/{id}",
		Handler: "TaskController.deleteTask", File: "src/TaskController.java", Line: 10,
		Steps: []CodeFlowStep{
			{
				Name: "deleteTask", Owner: "TaskController", Kind: "route_handler",
				File: "src/TaskController.java", Line: 10,
			},
			{
				Name: "deleteTaskInternal", Owner: "TaskController", Kind: "call",
				File: "src/TaskController.java", Line: 20,
				Caller: "TaskController.deleteTask", CallerFile: "src/TaskController.java", CallerLine: 10,
			},
			{
				Name: "deleteTask", Owner: "TaskService", Kind: "service_method",
				File: "src/TaskService.java", Line: 30,
				Caller: "TaskController.deleteTaskInternal", CallerFile: "src/TaskController.java", CallerLine: 20,
			},
			{
				Name: "checkAccess", Owner: "AccessService", Kind: "service_method",
				File: "src/AccessService.java", Line: 40,
				Caller: "TaskController.deleteTaskInternal", CallerFile: "src/TaskController.java", CallerLine: 20,
			},
		},
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-24T00:00:00Z",
		routes, flows, nil, nil, nil, nil, nil, nil)

	handler := findContextFactByQualified(index.Facts, "TaskController.deleteTask")
	helper := findContextFactByQualified(index.Facts, "TaskController.deleteTaskInternal")
	service := findContextFactByQualified(index.Facts, "TaskService.deleteTask")
	access := findContextFactByQualified(index.Facts, "AccessService.checkAccess")
	if !hasContextEdge(index.Edges, handler.ID, helper.ID, "call") ||
		!hasContextEdge(index.Edges, helper.ID, service.ID, "call") ||
		!hasContextEdge(index.Edges, helper.ID, access.ID, "call") {
		t.Fatalf("flow branch edges missing: %#v", index.Edges)
	}
	if hasContextEdge(index.Edges, service.ID, access.ID, "call") {
		t.Fatalf("sibling flow steps were linearized: %#v", index.Edges)
	}
}

func TestBuildProjectAgentContextIndexUsesOneSelectedRoutePerFlow(t *testing.T) {
	routes := []CodeRouteRecord{
		{
			RouteID: "route:admin", Kind: "backend", HTTPMethod: "GET", Path: "/users",
			Handler: "AdminUserController.listUsers", File: "admin/AdminUserController.java", Line: 10,
		},
		{
			RouteID: "route:public", Kind: "backend", HTTPMethod: "GET", Path: "/users",
			Handler: "PublicUserController.listUsers", File: "public/PublicUserController.java", Line: 20,
		},
	}
	flows := []CodeFlowRecord{{
		RouteID: "route:public", HTTPMethod: "GET", Path: "/users",
		Handler: "PublicUserController.listUsers", File: "public/PublicUserController.java", Line: 20,
		Steps: []CodeFlowStep{
			{Name: "listUsers", Owner: "PublicUserController", Kind: "route_handler", File: "public/PublicUserController.java", Line: 20},
			{Name: "findUsers", Owner: "UserService", Kind: "service_method", File: "service/UserService.java", Line: 30},
		},
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		routes, flows, nil, nil, nil, nil, nil, nil)

	if len(index.Edges) != 1 {
		t.Fatalf("flow edges = %#v", index.Edges)
	}
	publicRoute := findContextFactByQualified(index.Facts, "PublicUserController.listUsers")
	if index.Edges[0].FromFactID != publicRoute.ID || index.Edges[0].ToLabel != "UserService.findUsers" {
		t.Fatalf("selected flow edge = %#v, public route = %#v", index.Edges[0], publicRoute)
	}
}

func TestBuildProjectAgentContextIndexFallsBackFromDuplicateRouteIDToMethodPath(t *testing.T) {
	routes := []CodeRouteRecord{
		{
			RouteID: "route:duplicate", Kind: "backend", HTTPMethod: "GET", Path: "/admin",
			Handler: "Routes.listAdmins", File: "src/Routes.java", Line: 19,
		},
		{
			RouteID: "route:duplicate", Kind: "backend", HTTPMethod: "POST", Path: "/users",
			Handler: "Routes.createUser", File: "src/Routes.java", Line: 50,
		},
	}
	flows := []CodeFlowRecord{{
		RouteID: "route:duplicate", HTTPMethod: "POST", Path: "/users",
		Handler: "Routes.createUser", File: "src/Routes.java", Line: 20,
		Steps: []CodeFlowStep{
			{Name: "createUser", Owner: "Routes", Kind: "route_handler", File: "src/Routes.java", Line: 20},
			{Name: "save", Owner: "UserService", Kind: "service_method", File: "src/UserService.java", Line: 30},
		},
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		routes, flows, nil, nil, nil, nil, nil, nil)

	if len(index.Edges) != 1 || index.Edges[0].FromLabel != "POST /users" ||
		index.Edges[0].ToLabel != "UserService.save" {
		t.Fatalf("duplicate route ID bound wrong flow route: %#v", index.Edges)
	}
}

func TestBuildProjectAgentContextIndexRejectsLexicalAndAmbiguousRelations(t *testing.T) {
	symbols := []RichSymbolRecord{
		{ID: "consumer-a", Name: "ConsumerA", QualifiedName: "pkg.ConsumerA", Kind: "class", Language: "java", File: "a/Consumer.java", Line: 1},
		{ID: "consumer-b", Name: "ConsumerB", QualifiedName: "pkg.ConsumerB", Kind: "class", Language: "java", File: "a/Consumer.java", Line: 1},
		{ID: "provider", Name: "Provider", QualifiedName: "pkg.Provider", Kind: "class", Language: "java", File: "b/Provider.java", Line: 1},
	}
	relations := []RichRelationRecord{
		{ID: "local", From: "a/Consumer.java", To: "pkg.Provider", Type: "calls_local", FromSymbolID: "consumer-a", ToSymbolID: "provider", Line: 5},
		{ID: "ambiguous", From: "a/Consumer.java", To: "pkg.Provider", Type: "call", ToSymbolID: "provider", Line: 5},
	}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, symbols, relations, nil, nil, nil, nil)

	if len(index.Edges) != 0 {
		t.Fatalf("lexical or ambiguous relations leaked: %#v", index.Edges)
	}
}

func TestBuildProjectAgentContextIndexRejectsNonPromotableRelationsBeforeSelection(t *testing.T) {
	symbols := []RichSymbolRecord{
		{ID: "consumer", Name: "loadUsers", QualifiedName: "src/page#loadUsers", Kind: "function", Language: "typescript", File: "src/page.ts", Line: 5},
		{ID: "provider", Name: "fetchUsers", QualifiedName: "src/api#fetchUsers", Kind: "function", Language: "typescript", File: "src/api.ts", Line: 8},
	}
	relations := []RichRelationRecord{{
		ID: "computed-call", From: "src/page.ts", To: "src/api#fetchUsers",
		Type: "calls_export", FromSymbolID: "consumer", ToSymbolID: "provider",
		TargetQualifiedName: "src/api#fetchUsers", Resolution: SymbolResolutionUnresolved,
		NonPromotable: true, Line: 12,
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, symbols, relations, nil, nil, nil, nil)

	if len(index.Facts) != 0 || len(index.Edges) != 0 {
		t.Fatalf("non-promotable relation leaked into compact context: %#v", index)
	}
}

func TestBuildProjectAgentContextIndexResolvesUniqueLegacyFileTarget(t *testing.T) {
	symbols := []RichSymbolRecord{
		{ID: "consumer", Name: "Consumer", QualifiedName: "pkg.Consumer", Kind: "class", Language: "java", File: "a/Consumer.java", Line: 1},
		{ID: "provider", Name: "Provider", QualifiedName: "pkg.Provider", Kind: "class", Language: "java", File: "b/Provider.java", Line: 1},
	}
	relations := []RichRelationRecord{{
		ID: "legacy-call", From: "a/Consumer.java", To: `b\Provider.java`,
		Type: "calls", Line: 12, Confidence: "EXTRACTED",
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, symbols, relations, nil, nil, nil, nil)

	if len(index.Edges) != 1 || index.Edges[0].FromLabel != "pkg.Consumer" ||
		index.Edges[0].ToLabel != "pkg.Provider" {
		t.Fatalf("legacy file-target edge = %#v", index.Edges)
	}
}

func TestBuildProjectAgentContextIndexRejectsAmbiguousLegacyFileTarget(t *testing.T) {
	symbols := []RichSymbolRecord{
		{ID: "consumer", Name: "Consumer", QualifiedName: "pkg.Consumer", Kind: "class", Language: "java", File: "a/Consumer.java", Line: 1},
		{ID: "provider", Name: "Provider", QualifiedName: "pkg.Provider", Kind: "class", Language: "java", File: "b/Provider.java", Line: 1},
		{ID: "helper", Name: "ProviderHelper", QualifiedName: "pkg.ProviderHelper", Kind: "class", Language: "java", File: "b/Provider.java", Line: 20},
	}
	relations := []RichRelationRecord{{
		ID: "legacy-call", From: "a/Consumer.java", To: "b/Provider.java",
		Type: "calls", Line: 12, Confidence: "EXTRACTED",
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, symbols, relations, nil, nil, nil, nil)

	if len(index.Edges) != 0 {
		t.Fatalf("ambiguous legacy file-target edge leaked: %#v", index.Edges)
	}
}

func TestBuildProjectAgentContextIndexRejectsUnresolvedFileOnlyTarget(t *testing.T) {
	symbols := []RichSymbolRecord{
		{ID: "consumer", Name: "Consumer", QualifiedName: "pkg.Consumer", Kind: "class", Language: "java", File: "a/Consumer.java", Line: 1},
		{ID: "provider", Name: "Provider", QualifiedName: "pkg.Provider", Kind: "class", Language: "java", File: "b/Provider.java", Line: 1},
	}
	relations := []RichRelationRecord{{
		ID: "unresolved-file-call", From: "a/Consumer.java", To: "b/Provider.java",
		Type: "calls", Resolution: SymbolResolutionUnresolved, Line: 12,
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, symbols, relations, nil, nil, nil, nil)

	if len(index.Edges) != 0 {
		t.Fatalf("unresolved file-only target leaked: %#v", index.Edges)
	}
}

func TestBuildProjectAgentContextIndexAllowsExactQualifiedUnresolvedTarget(t *testing.T) {
	symbols := []RichSymbolRecord{
		{ID: "consumer", Name: "Consumer", QualifiedName: "pkg.Consumer", Kind: "class", Language: "java", File: "a/Consumer.java", Line: 1},
		{ID: "provider", Name: "Provider", QualifiedName: "pkg.Provider", Kind: "class", Language: "java", File: "b/Provider.java", Line: 1},
	}
	relations := []RichRelationRecord{{
		ID: "unresolved-qualified-call", From: "a/Consumer.java", To: "b/Provider.java",
		TargetQualifiedName: "pkg.Provider", Type: "calls",
		Resolution: SymbolResolutionUnresolved, Line: 12,
	}}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, symbols, relations, nil, nil, nil, nil)

	if len(index.Edges) != 1 || index.Edges[0].FromLabel != "pkg.Consumer" ||
		index.Edges[0].ToLabel != "pkg.Provider" {
		t.Fatalf("exact-qualified unresolved target edge = %#v", index.Edges)
	}
}

func TestBuildProjectAgentContextIndexMergesDuplicatesIndependentOfInputOrder(t *testing.T) {
	exact := RichSymbolRecord{
		ID: "exact", Name: "UserService", QualifiedName: "pkg.UserService",
		Kind: "class", Language: "java", File: "UserService.java", Line: 1,
		Confidence: ConfidenceExact, EvidenceIDs: []string{"evidence:a"},
	}
	inferred := exact
	inferred.ID = "inferred"
	inferred.Confidence = ConfidenceInferred
	inferred.EvidenceIDs = []string{"evidence:b"}

	first := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, []RichSymbolRecord{exact, inferred}, nil, nil, nil, nil, nil)
	second := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, []RichSymbolRecord{inferred, exact}, nil, nil, nil, nil, nil)

	if diff := cmpJSON(first, second); diff != "" {
		t.Fatalf("duplicate merging depends on input order: %s", diff)
	}
	if len(first.Facts) != 1 || first.Facts[0].Confidence != "EXACT" ||
		!slices.Equal(first.Facts[0].EvidenceIDs, []string{"evidence:a", "evidence:b"}) {
		t.Fatalf("merged fact = %#v", first.Facts)
	}
}

func TestBuildProjectAgentContextIndexAggregatesCoverageFailures(t *testing.T) {
	capabilities := []CapabilityRecord{
		{ID: CapabilityCalls, Language: "java", SourceClass: "code", Coverage: CoverageComplete, Reason: "java calls"},
		{ID: CapabilityCalls, Language: "typescript", SourceClass: "code", Coverage: CoverageFailed, StatusReason: "typescript analyzer failed"},
	}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, nil, nil, nil, nil, nil, capabilities)

	if len(index.Coverage) != 1 || index.Coverage[0].Coverage != "FAILED" ||
		index.Coverage[0].Reason != "typescript analyzer failed" {
		t.Fatalf("aggregated coverage = %#v", index.Coverage)
	}
}

func TestBuildProjectAgentContextIndexKeepsDeterministicNonEmptyCoverageReason(t *testing.T) {
	withoutReason := CapabilityRecord{
		ID: CapabilityCalls, Language: "java", SourceClass: "code", Coverage: CoverageComplete,
	}
	withReason := CapabilityRecord{
		ID: CapabilityCalls, Language: "typescript", SourceClass: "code", Coverage: CoverageComplete,
		StatusReason: "TypeScript calls are indexed.",
	}

	first := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, nil, nil, nil, nil, nil, []CapabilityRecord{withoutReason, withReason})
	second := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, nil, nil, nil, nil, nil, []CapabilityRecord{withReason, withoutReason})

	if diff := cmpJSON(first, second); diff != "" {
		t.Fatalf("coverage reason depends on input order: %s", diff)
	}
	if len(first.Coverage) != 1 || first.Coverage[0].Reason != "TypeScript calls are indexed." {
		t.Fatalf("coverage reason = %#v", first.Coverage)
	}
}

func TestBuildProjectAgentContextIndexFiltersNonSemanticEdges(t *testing.T) {
	symbols := []RichSymbolRecord{
		{ID: "consumer", Name: "Consumer", QualifiedName: "pkg.Consumer", Kind: "class", Language: "java", File: "a/Consumer.java", Line: 1},
		{ID: "provider", Name: "Provider", QualifiedName: "pkg.Provider", Kind: "class", Language: "java", File: "b/Provider.java", Line: 1},
	}
	relations := []RichRelationRecord{
		{ID: "use", From: "a/Consumer.java", To: "pkg.Provider", Type: "use", FromSymbolID: "consumer", ToSymbolID: "provider", Line: 5, Confidence: "EXACT"},
		{ID: "duplicate-use", From: "a/Consumer.java", To: "pkg.Provider", Type: "use", FromSymbolID: "consumer", ToSymbolID: "provider", Line: 5, Confidence: "EXACT"},
		{ID: "import", From: "a/Consumer.java", To: "pkg.Provider", Type: "imports", FromSymbolID: "consumer", ToSymbolID: "provider", Line: 2, Confidence: "EXACT"},
		{ID: "lexical", From: "a/Consumer.java", To: "pkg.Provider", Type: "lexical", FromSymbolID: "consumer", ToSymbolID: "provider", Line: 7, Confidence: "EXACT"},
		{ID: "unresolved-same-file", From: "a/Consumer.java", To: "missing", Type: "call", FromSymbolID: "consumer", Line: 9, Confidence: "INFERRED"},
	}

	index := BuildProjectAgentContextIndex("app", "2026-07-16T00:00:00Z",
		nil, nil, symbols, relations, nil, nil, nil, nil)

	if len(index.Edges) != 1 || index.Edges[0].Kind != "use" {
		t.Fatalf("filtered edges = %#v", index.Edges)
	}
}

func TestBuildWorkspaceAgentContextIndexAddsCompactCatalogFacts(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Indexed: true},
		{Path: "frontend/web", Indexed: true},
	}}
	catalog := APICatalogRecord{SchemaVersion: SchemaVersion, Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders", ProviderService: "orders",
		HTTPMethod: "GET", Path: "/orders/{id}", Handler: "OrderController.get",
		File: "services/orders/src/OrderController.java", Line: 20,
		RequestType: "OrderRequest", ResponseType: "OrderResponse", Confidence: ConfidenceExact,
		Parameters: []APIParameterRecord{{Name: ".goregraph-dashboard.json", Source: "expanded-parameter-evidence"}},
		Security: []SecurityEvidenceRecord{{
			Kind: SecurityBearer, Summary: "expanded-security-evidence", Confidence: ConfidenceNormalized,
		}},
		Consumers: []APIConsumerRecord{{
			ID: "consumer:web", Project: "frontend/web", Service: "web", Caller: "loadOrder",
			File: "frontend/web/src/api.ts", Line: 7, Confidence: ConfidenceNormalized,
			Limitations: []string{"expanded-consumer-evidence"},
		}},
		Limitations: []string{"expanded-endpoint-evidence"},
	}}}

	index := BuildWorkspaceAgentContextIndex(
		registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed",
	)
	endpoint := findContextFact(index.Facts, "api_endpoint", "GET /orders/{id}")
	if endpoint.ID == "" {
		t.Fatalf("endpoint fact missing: %#v", index.Facts)
	}
	for _, value := range []string{
		"services/orders", "OrderController.get", "bearer", "OrderRequest", "OrderResponse",
	} {
		if !strings.Contains(endpoint.Summary+" "+endpoint.Search, value) {
			t.Fatalf("endpoint context %q missing %q", endpoint.Summary+" "+endpoint.Search, value)
		}
	}
	security := findContextFact(index.Facts, "endpoint_security", SecurityBearer)
	consumer := findContextFact(index.Facts, "api_consumer", "loadOrder")
	if security.ID == "" || consumer.ID == "" {
		t.Fatalf("catalog detail facts missing: %#v", index.Facts)
	}
	if !hasContextEdge(index.Edges, endpoint.ID, security.ID, "requires_auth") ||
		!hasContextEdge(index.Edges, consumer.ID, endpoint.ID, "consumes_endpoint") {
		t.Fatalf("catalog edges missing: %#v", index.Edges)
	}
	for _, forbidden := range []string{
		".goregraph-dashboard.json",
		"expanded-parameter-evidence",
		"expanded-security-evidence",
		"expanded-consumer-evidence",
		"expanded-endpoint-evidence",
	} {
		if contextIndexContains(index, forbidden) {
			t.Fatalf("expanded catalog data %q leaked into context", forbidden)
		}
	}
}

func TestBuildWorkspaceAgentContextIndexBoundsCompactCatalogConsumers(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Indexed: true},
		{Path: "frontend/web", Indexed: true},
	}}
	consumers := make([]APIConsumerRecord, 100)
	for index := range consumers {
		consumers[index] = APIConsumerRecord{
			ID: fmt.Sprintf("consumer:%03d", index), Project: "frontend/web", Service: "web",
			Caller: fmt.Sprintf("loadOrder%03d", index), File: "frontend/web/src/api.ts", Line: index + 1,
			Confidence: ConfidenceNormalized, Limitations: []string{"expanded-consumer-evidence"},
		}
	}
	catalog := APICatalogRecord{Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders",
		HTTPMethod: "GET", Path: "/orders/{id}", Consumers: consumers,
	}}}

	index := BuildWorkspaceAgentContextIndex(
		registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed",
	)
	if got := countContextFacts(index.Facts, "api_consumer"); got != 5 {
		t.Fatalf("consumer facts = %d, want 5: %#v", got, index.Facts)
	}
	if got := countContextEdges(index.Edges, "consumes_endpoint"); got != 5 {
		t.Fatalf("consumer edges = %d, want 5: %#v", got, index.Edges)
	}
	endpoint := findContextFact(index.Facts, "api_endpoint", "GET /orders/{id}")
	if !strings.Contains(endpoint.Summary, "95") || !strings.Contains(endpoint.Summary, "omitted") {
		t.Fatalf("endpoint summary does not preserve omitted consumer count: %#v", endpoint)
	}
	if contextIndexContains(index, "expanded-consumer-evidence") {
		t.Fatal("expanded consumer evidence leaked into bounded context")
	}
}

func TestBuildWorkspaceAgentContextIndexCompactCatalogIsDeterministic(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Indexed: true},
		{Path: "frontend/web", Indexed: true},
	}}
	catalog := APICatalogRecord{Endpoints: []APIEndpointRecord{
		{
			ID: "endpoint:orders", ProviderProject: "services/orders", HTTPMethod: "GET", Path: "/orders/{id}",
			Security: []SecurityEvidenceRecord{
				{Kind: SecurityRole, Expression: "admin", Confidence: ConfidenceNormalized},
				{Kind: SecurityBearer, Confidence: ConfidenceNormalized},
			},
			Consumers: []APIConsumerRecord{
				{ID: "consumer:b", Project: "frontend/web", Service: "web", Caller: "loadB", File: "frontend/web/src/b.ts", Line: 2},
				{ID: "consumer:a", Project: "frontend/web", Service: "web", Caller: "loadA", File: "frontend/web/src/a.ts", Line: 1},
			},
		},
		{ID: "endpoint:health", ProviderProject: "services/orders", HTTPMethod: "GET", Path: "/health"},
	}}
	reversed := catalog
	reversed.Endpoints = slices.Clone(catalog.Endpoints)
	for index := range reversed.Endpoints {
		reversed.Endpoints[index].Security = slices.Clone(reversed.Endpoints[index].Security)
		slices.Reverse(reversed.Endpoints[index].Security)
		reversed.Endpoints[index].Consumers = slices.Clone(reversed.Endpoints[index].Consumers)
		slices.Reverse(reversed.Endpoints[index].Consumers)
	}
	slices.Reverse(reversed.Endpoints)

	forward := BuildWorkspaceAgentContextIndex(
		registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed",
	)
	backward := BuildWorkspaceAgentContextIndex(
		registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, reversed, "fixed",
	)
	if diff := cmpJSON(forward, backward); diff != "" {
		t.Fatalf("catalog input order changed compact context: %s", diff)
	}
}

func TestBuildWorkspaceAgentContextIndexIncludesCompactConsumerCallAuth(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Indexed: true},
		{Path: "frontend/web", Indexed: true},
	}}
	catalog := APICatalogRecord{Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders", HTTPMethod: "GET", Path: "/orders/{id}",
		Consumers: []APIConsumerRecord{{
			ID: "consumer:web", Project: "frontend/web", Service: "web", Caller: "loadOrder",
			File: "frontend/web/src/api.ts", Line: 7,
			CallAuth: []SecurityEvidenceRecord{
				{Kind: SecurityOAuth2, Summary: "expanded-auth-summary", Expression: "secret-expression"},
				{Kind: SecurityBearer, Summary: "authorization-secret"},
				{Kind: SecurityBearer, Expression: "duplicate-secret"},
			},
		}},
	}}}

	index := BuildWorkspaceAgentContextIndex(
		registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed",
	)
	consumer := findContextFact(index.Facts, "api_consumer", "loadOrder")
	if consumer.Summary != "consumer service web; auth bearer, oauth2" {
		t.Fatalf("consumer auth summary = %q", consumer.Summary)
	}
	for _, kind := range []string{SecurityBearer, SecurityOAuth2} {
		if !strings.Contains(consumer.Search, kind) {
			t.Fatalf("consumer search %q missing auth kind %q", consumer.Search, kind)
		}
	}
	edge := findContextEdge(index.Edges, "consumes_endpoint")
	if edge.Reason != "catalog consumer auth bearer, oauth2" {
		t.Fatalf("consumer edge reason = %q", edge.Reason)
	}
	for _, forbidden := range []string{
		"expanded-auth-summary", "secret-expression", "authorization-secret", "duplicate-secret",
	} {
		if contextIndexContains(index, forbidden) {
			t.Fatalf("consumer auth detail %q leaked into context", forbidden)
		}
	}
}

func TestBuildWorkspaceAgentContextIndexUsesUnknownForMissingConsumerCallAuth(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Indexed: true},
		{Path: "frontend/web", Indexed: true},
	}}
	catalog := APICatalogRecord{Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders", HTTPMethod: "GET", Path: "/orders/{id}",
		Consumers: []APIConsumerRecord{{
			ID: "consumer:web", Project: "frontend/web", Service: "web", Caller: "loadOrder",
			File: "frontend/web/src/api.ts", Line: 7, CallAuth: []SecurityEvidenceRecord{},
		}},
	}}}

	index := BuildWorkspaceAgentContextIndex(
		registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed",
	)
	consumer := findContextFact(index.Facts, "api_consumer", "loadOrder")
	edge := findContextEdge(index.Edges, "consumes_endpoint")
	for _, value := range []string{consumer.Summary, consumer.Search, edge.Reason} {
		if !strings.Contains(value, SecurityUnknown) || strings.Contains(value, SecurityPublic) {
			t.Fatalf("missing call auth was not represented as unknown: fact=%#v edge=%#v", consumer, edge)
		}
	}
}

func TestBuildWorkspaceAgentContextIndexRequiresAuthOnlyForCredentialKinds(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{
		Path: "services/orders", Indexed: true,
	}}}
	kinds := []string{
		SecurityBasic, SecurityBearer, SecurityOAuth2, SecurityAPIKey, SecuritySession,
		SecurityMTLS, SecurityRole, SecurityAuthenticated, SecurityPublic, SecurityUnknown,
	}
	security := make([]SecurityEvidenceRecord, 0, len(kinds))
	for _, kind := range kinds {
		security = append(security, SecurityEvidenceRecord{Kind: kind, Source: "test"})
	}
	catalog := APICatalogRecord{Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders", HTTPMethod: "GET", Path: "/orders/{id}",
		Security: security,
	}}}

	index := BuildWorkspaceAgentContextIndex(
		registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed",
	)
	endpoint := findContextFact(index.Facts, "api_endpoint", "GET /orders/{id}")
	for _, kind := range kinds {
		securityFact := findContextFact(index.Facts, "endpoint_security", kind)
		if securityFact.ID == "" {
			t.Fatalf("security fact %q missing: %#v", kind, index.Facts)
		}
		wantEdge := kind != SecurityPublic && kind != SecurityUnknown
		if got := hasContextEdge(index.Edges, endpoint.ID, securityFact.ID, "requires_auth"); got != wantEdge {
			t.Fatalf("requires_auth for %q = %v, want %v: %#v", kind, got, wantEdge, index.Edges)
		}
	}
}

func TestBuildWorkspaceAgentContextIndexLimitsConsumersPerProjectAndService(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Indexed: true},
		{Path: "frontend/a", Indexed: true},
		{Path: "frontend/b", Indexed: true},
	}}
	consumers := make([]APIConsumerRecord, 0, 12)
	for _, project := range []string{"frontend/a", "frontend/b"} {
		for index := 0; index < 6; index++ {
			consumers = append(consumers, APIConsumerRecord{
				ID: fmt.Sprintf("%s:%d", project, index), Project: project, Service: "web",
				Caller: fmt.Sprintf("loadOrder%d", index), File: project + "/src/api.ts", Line: index + 1,
			})
		}
	}
	catalog := APICatalogRecord{Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders",
		HTTPMethod: "GET", Path: "/orders/{id}", Consumers: consumers,
	}}}

	index := BuildWorkspaceAgentContextIndex(
		registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed",
	)
	for _, project := range []string{"frontend/a", "frontend/b"} {
		if got := countContextFactsForProject(index.Facts, "api_consumer", project); got != 5 {
			t.Fatalf("consumer facts for %q = %d, want 5: %#v", project, got, index.Facts)
		}
	}
	if got := countContextEdges(index.Edges, "consumes_endpoint"); got != 10 {
		t.Fatalf("consumer edges = %d, want 10: %#v", got, index.Edges)
	}
	endpoint := findContextFact(index.Facts, "api_endpoint", "GET /orders/{id}")
	if !strings.Contains(endpoint.Summary, "2 consumer call sites omitted") {
		t.Fatalf("endpoint omitted count = %#v", endpoint)
	}
}

func TestBuildWorkspaceAgentContextIndexKeepsConflictingSecurityAsDistinctFact(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{
		Path: "services/orders", Indexed: true,
	}}}
	catalog := APICatalogRecord{Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders", HTTPMethod: "GET", Path: "/orders/{id}",
		Security: []SecurityEvidenceRecord{
			{Kind: SecurityBearer, Source: "security_config", File: "services/orders/Security.java", Line: 10},
			{Kind: SecurityBearer, Source: "security_config", File: "services/orders/Security.java", Line: 10, Conflicting: true},
		},
	}}}

	index := BuildWorkspaceAgentContextIndex(
		registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed",
	)
	if got := countNamedContextFacts(index.Facts, "endpoint_security", SecurityBearer); got != 2 {
		t.Fatalf("bearer security facts = %d, want 2: %#v", got, index.Facts)
	}
	conflicting := 0
	for _, fact := range index.Facts {
		if fact.Kind == "endpoint_security" && strings.Contains(fact.Summary, "conflicting evidence") {
			conflicting++
		}
	}
	if conflicting != 1 {
		t.Fatalf("conflicting security facts = %d, want 1: %#v", conflicting, index.Facts)
	}
}

func TestBuildWorkspaceAgentContextIndexBoundsCatalogFactEvidenceIDsDeterministically(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Indexed: true},
		{Path: "frontend/web", Indexed: true},
	}}
	evidenceIDs := make([]string, 20)
	for index := range evidenceIDs {
		evidenceIDs[index] = fmt.Sprintf("evidence:%02d", index)
	}
	build := func(ids []string) AgentContextIndexRecord {
		t.Helper()
		return BuildWorkspaceAgentContextIndex(
			registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{},
			APICatalogRecord{Endpoints: []APIEndpointRecord{{
				ID: "endpoint:orders", ProviderProject: "services/orders", HTTPMethod: "GET", Path: "/orders/{id}",
				EvidenceIDs: slices.Clone(ids),
				Security: []SecurityEvidenceRecord{{
					Kind: SecurityBearer, Source: "test", EvidenceIDs: slices.Clone(ids),
				}},
				Consumers: []APIConsumerRecord{{
					ID: "consumer:web", Project: "frontend/web", Service: "web", Caller: "loadOrder",
					File: "frontend/web/src/api.ts", Line: 7, EvidenceIDs: slices.Clone(ids),
				}},
			}}},
			"fixed",
		)
	}
	reversedIDs := slices.Clone(evidenceIDs)
	slices.Reverse(reversedIDs)
	forward := build(evidenceIDs)
	backward := build(reversedIDs)
	if diff := cmpJSON(forward, backward); diff != "" {
		t.Fatalf("evidence ID input order changed compact context: %s", diff)
	}
	for _, fact := range forward.Facts {
		if fact.Kind != "api_endpoint" && fact.Kind != "endpoint_security" && fact.Kind != "api_consumer" {
			continue
		}
		if len(fact.EvidenceIDs) != 8 {
			t.Fatalf("%s evidence IDs = %d, want 8: %#v", fact.Kind, len(fact.EvidenceIDs), fact)
		}
		if !slices.Equal(fact.EvidenceIDs, evidenceIDs[:8]) {
			t.Fatalf("%s evidence IDs = %#v, want %#v", fact.Kind, fact.EvidenceIDs, evidenceIDs[:8])
		}
	}
}

func cmpJSON(left, right any) string {
	leftBody, _ := json.Marshal(left)
	rightBody, _ := json.Marshal(right)
	if string(leftBody) == string(rightBody) {
		return ""
	}
	return string(leftBody) + " != " + string(rightBody)
}

func contextIndexContains(index AgentContextIndexRecord, value string) bool {
	body, _ := json.Marshal(index)
	return strings.Contains(string(body), value)
}

func countContextEdges(edges []AgentContextEdgeRecord, kind string) int {
	count := 0
	for _, edge := range edges {
		if edge.Kind == kind {
			count++
		}
	}
	return count
}

func findContextEdge(edges []AgentContextEdgeRecord, kind string) AgentContextEdgeRecord {
	for _, edge := range edges {
		if edge.Kind == kind {
			return edge
		}
	}
	return AgentContextEdgeRecord{}
}

func countContextFactsForProject(facts []AgentContextFactRecord, kind, project string) int {
	count := 0
	for _, fact := range facts {
		if fact.Kind == kind && fact.Project == project {
			count++
		}
	}
	return count
}

func countNamedContextFacts(facts []AgentContextFactRecord, kind, name string) int {
	count := 0
	for _, fact := range facts {
		if fact.Kind == kind && fact.Name == name {
			count++
		}
	}
	return count
}

func hasContextFact(facts []AgentContextFactRecord, kind, name string) bool {
	return findContextFact(facts, kind, name).ID != ""
}

func hasContextEdge(edges []AgentContextEdgeRecord, fromFactID, toFactID, kind string) bool {
	for _, edge := range edges {
		if edge.FromFactID == fromFactID && edge.ToFactID == toFactID && edge.Kind == kind {
			return true
		}
	}
	return false
}

func findContextFact(facts []AgentContextFactRecord, kind, name string) AgentContextFactRecord {
	for _, fact := range facts {
		if fact.Kind == kind && fact.Name == name {
			return fact
		}
	}
	return AgentContextFactRecord{}
}

func findContextFactByQualified(facts []AgentContextFactRecord, qualified string) AgentContextFactRecord {
	for _, fact := range facts {
		if fact.Qualified == qualified {
			return fact
		}
	}
	return AgentContextFactRecord{}
}
