package scan

import (
	"encoding/json"
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
	symbols := []RichSymbolRecord{{
		ID: "symbol:operations", Name: "deleteRegulationFromCadaster",
		QualifiedName: "CadasterRegulationOperationsService.deleteRegulationFromCadaster",
		Kind:          "method", Language: "java",
		File: "src/CadasterRegulationOperationsService.java", Line: 45,
		Confidence: ConfidenceExact, EvidenceIDs: []string{"evidence:symbol"},
	}, {
		ID: "symbol:local-helper", Name: "formatDebugMessage",
		QualifiedName: "CadasterRegulationOperationsService.formatDebugMessage",
		Kind:          "method", Language: "java",
		File: "src/CadasterRegulationOperationsService.java", Line: 90,
		Confidence: ConfidenceExact,
	}}
	relations := []RichRelationRecord{{
		ID: "relation:delete", From: "src/CadasterRegulationController.java",
		To:   "CadasterRegulationOperationsService.deleteRegulationFromCadaster",
		Type: "call", Line: 201, Confidence: "EXACT",
		Reason: "qualified method call", EvidenceIDs: []string{"evidence:call"},
	}}

	first := BuildProjectAgentContextIndex("ms-cadasterregulation", "2026-07-16T00:00:00Z",
		routes, nil, symbols, relations, nil, nil, nil, nil)
	second := BuildProjectAgentContextIndex("ms-cadasterregulation", "2026-07-16T00:00:00Z",
		routes, nil, symbols, relations, nil, nil, nil, nil)

	if diff := cmpJSON(first, second); diff != "" {
		t.Fatalf("context index is not deterministic: %s", diff)
	}
	if len(first.Facts) != 2 || len(first.Edges) != 1 {
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

func cmpJSON(left, right any) string {
	leftBody, _ := json.Marshal(left)
	rightBody, _ := json.Marshal(right)
	if string(leftBody) == string(rightBody) {
		return ""
	}
	return string(leftBody) + " != " + string(rightBody)
}

func hasContextFact(facts []AgentContextFactRecord, kind, name string) bool {
	return findContextFact(facts, kind, name).ID != ""
}

func findContextFact(facts []AgentContextFactRecord, kind, name string) AgentContextFactRecord {
	for _, fact := range facts {
		if fact.Kind == kind && fact.Name == name {
			return fact
		}
	}
	return AgentContextFactRecord{}
}
