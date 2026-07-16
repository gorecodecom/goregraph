package scan

import (
	"reflect"
	"testing"
)

func TestWorkspaceSymbolAPIUsageContainsFullFrontendToJavaChain(t *testing.T) {
	frontendProject := "frontend/app"
	backendProject := "microservices/ms-user"
	userPage := CanonicalSymbolRecord{
		ID:              "symbol:user-page",
		Project:         frontendProject,
		ProjectKind:     "frontend",
		Language:        "typescript",
		Kind:            "function",
		Name:            "UserPage",
		QualifiedName:   "UserPage",
		ExportName:      "UserPage",
		DeclarationFile: "src/pages/UserPage.tsx",
		DeclarationLine: 8,
		EvidenceIDs:     []string{frontendProject + "#evidence:user-page"},
		Analyzer:        "typescript-source",
		Confidence:      ConfidenceExact,
		Coverage:        CoverageComplete,
	}
	loadUser := CanonicalSymbolRecord{
		ID:              "symbol:load-user",
		Project:         frontendProject,
		ProjectKind:     "frontend",
		Language:        "typescript",
		Kind:            "function",
		Name:            "loadUser",
		QualifiedName:   "loadUser",
		ExportName:      "loadUser",
		DeclarationFile: "src/api/users.ts",
		DeclarationLine: 12,
		EvidenceIDs:     []string{frontendProject + "#evidence:load-user"},
		Analyzer:        "typescript-source",
		Confidence:      ConfidenceExact,
		Coverage:        CoverageComplete,
	}
	userService := CanonicalSymbolRecord{
		ID:              "symbol:user-service",
		Project:         backendProject,
		Service:         "ms-user",
		ProjectKind:     "backend",
		Package:         "com.example.user",
		Artifact:        "com.example:ms-user",
		Language:        "java",
		Kind:            "class",
		Name:            "UserService",
		QualifiedName:   "com.example.user.UserService",
		DeclarationFile: "src/main/java/com/example/user/UserService.java",
		DeclarationLine: 15,
		EvidenceIDs:     []string{backendProject + "#evidence:user-service"},
		Analyzer:        "java-source",
		Confidence:      ConfidenceExact,
		Coverage:        CoverageComplete,
	}
	symbols := WorkspaceSymbolIndexRecord{
		SchemaVersion: SchemaVersion,
		Symbols:       []CanonicalSymbolRecord{userPage, loadUser, userService},
	}
	matches := []WorkspaceContractMatchRecord{{
		ID:                "contract:get-user",
		APIProject:        frontendProject,
		APIHTTPMethod:     "GET",
		APIPath:           "/users/{userId}",
		APIFile:           loadUser.DeclarationFile,
		APILine:           loadUser.DeclarationLine,
		APICaller:         loadUser.Name,
		BackendProject:    backendProject,
		BackendService:    "ms-user",
		BackendHTTPMethod: "GET",
		BackendPath:       "/users/{userId}",
		BackendHandler:    "UserController.get",
		BackendFile:       "src/main/java/com/example/user/UserController.java",
		BackendLine:       41,
		Issue:             contractIssueMatched,
		Confidence:        "RESOLVED",
		ConfidenceScore:   0.9,
		Reason:            "http method and path pattern match backend route",
	}}
	flows := []WorkspaceFeatureFlowRecord{{
		ID:                "flow:get-user",
		FrontendProject:   frontendProject,
		FrontendComponent: userPage.Name,
		FrontendCaller:    loadUser.Name,
		FrontendFile:      loadUser.DeclarationFile,
		FrontendLine:      loadUser.DeclarationLine,
		FrontendSteps: []CodeFlowStep{
			{
				Name: userPage.Name, Kind: "component", Language: "typescript",
				File: userPage.DeclarationFile, Line: userPage.DeclarationLine,
				Confidence: "EXTRACTED", EvidenceIDs: []string{"evidence:user-page-flow"},
			},
			{
				Name: loadUser.Name, Kind: "api_helper", Language: "typescript",
				File: loadUser.DeclarationFile, Line: loadUser.DeclarationLine,
				Confidence: "EXTRACTED", EvidenceIDs: []string{"evidence:load-user-flow"},
			},
		},
		HTTPMethod:        "GET",
		Path:              "/users/{userId}",
		BackendProject:    backendProject,
		BackendService:    "ms-user",
		BackendController: "UserController",
		BackendMethod:     "get",
		BackendFile:       matches[0].BackendFile,
		BackendLine:       matches[0].BackendLine,
		BackendSteps: []SpringEndpointFlowStep{
			{
				Owner: "UserService", Method: "find", Kind: "service",
				File: userService.DeclarationFile, Line: 22, Confidence: "EXTRACTED",
			},
			{
				Owner: "UserRepository", Method: "find", Kind: "repository",
				File: "src/main/java/com/example/user/UserRepository.java", Line: 11, Confidence: "EXTRACTED",
			},
		},
		Confidence: "RESOLVED",
		Reason:     "frontend api contract resolved to indexed backend endpoint",
	}}
	traces := BuildWorkspaceEndpointTraces(matches, flows, nil)

	usages := BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, traces)

	var reached []CanonicalSymbolUsageRecord
	directCount := 0
	for _, usage := range usages {
		switch usage.Category {
		case SymbolUsageReachedThroughAPI:
			reached = append(reached, usage)
		case SymbolUsageDirectReference:
			directCount++
		}
	}
	if len(reached) != 1 {
		t.Fatalf("reached through API usages = %#v, want exactly one", reached)
	}
	if directCount != 0 {
		t.Fatalf("direct-reference count = %d, want 0", directCount)
	}
	usage := reached[0]
	if usage.ProviderSymbolID != userService.ID ||
		usage.ConsumerSymbolID != userPage.ID ||
		usage.ConsumerProject != frontendProject ||
		usage.Category != SymbolUsageReachedThroughAPI ||
		usage.Transport != "http" ||
		usage.Resolution != SymbolResolutionExact {
		t.Fatalf("usage = %#v", usage)
	}
	wantKinds := []string{
		"frontend_symbol",
		"api_helper",
		"http_contract",
		"workspace_contract",
		"spring_route",
		"spring_handler",
		"java_implementation",
		"selected_symbol",
	}
	if len(usage.APIPath) != len(wantKinds) {
		t.Fatalf("API path = %#v, want %d steps", usage.APIPath, len(wantKinds))
	}
	for position, kind := range wantKinds {
		step := usage.APIPath[position]
		if step.Position != position || step.Kind != kind {
			t.Fatalf("API path step %d = %#v, want kind %q at position %d", position, step, kind, position)
		}
	}
	assertSymbolAPIPathStep(t, usage.APIPath[0], userPage.ID, userPage.DeclarationFile, userPage.DeclarationLine, []string{
		frontendProject + "#evidence:user-page",
		frontendProject + "#evidence:user-page-flow",
	})
	assertSymbolAPIPathStep(t, usage.APIPath[1], loadUser.ID, loadUser.DeclarationFile, loadUser.DeclarationLine, []string{
		frontendProject + "#evidence:load-user",
		frontendProject + "#evidence:load-user-flow",
	})
	assertSymbolAPIPathStep(t, usage.APIPath[2], "", matches[0].APIFile, matches[0].APILine, nil)
	assertSymbolAPIPathStep(t, usage.APIPath[4], "", matches[0].BackendFile, matches[0].BackendLine, nil)
	assertSymbolAPIPathStep(t, usage.APIPath[5], "", matches[0].BackendFile, matches[0].BackendLine, nil)
	assertSymbolAPIPathStep(t, usage.APIPath[6], "", userService.DeclarationFile, 22, nil)
	assertSymbolAPIPathStep(t, usage.APIPath[7], userService.ID, userService.DeclarationFile, userService.DeclarationLine, userService.EvidenceIDs)
	if !reflect.DeepEqual(usage.EvidenceIDs, []string{
		frontendProject + "#evidence:load-user",
		frontendProject + "#evidence:load-user-flow",
		frontendProject + "#evidence:user-page",
		frontendProject + "#evidence:user-page-flow",
		backendProject + "#evidence:user-service",
	}) {
		t.Fatalf("usage evidence = %#v", usage.EvidenceIDs)
	}
}

func TestWorkspaceSymbolAPIUsageStartsAtExactFrontendComponent(t *testing.T) {
	symbols := WorkspaceSymbolIndexRecord{
		SchemaVersion: SchemaVersion,
		Symbols: []CanonicalSymbolRecord{
			{
				ID: "symbol:prepare-user", Project: "frontend/app", Language: "typescript",
				Kind: "function", Name: "prepareUser", QualifiedName: "prepareUser",
				DeclarationFile: "src/pages/UserPage.tsx", DeclarationLine: 5,
				Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
			{
				ID: "symbol:user-page", Project: "frontend/app", Language: "typescript",
				Kind: "function", Name: "UserPage", QualifiedName: "UserPage", ExportName: "UserPage",
				DeclarationFile: "src/pages/UserPage.tsx", DeclarationLine: 8,
				Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
			{
				ID: "symbol:load-user", Project: "frontend/app", Language: "typescript",
				Kind: "function", Name: "loadUser", QualifiedName: "loadUser", ExportName: "loadUser",
				DeclarationFile: "src/api/users.ts", DeclarationLine: 12,
				Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
			{
				ID: "symbol:user-service", Project: "microservices/ms-user", Language: "java",
				Kind: "class", Name: "UserService", QualifiedName: "com.example.UserService",
				DeclarationFile: "src/main/java/com/example/UserService.java", DeclarationLine: 10,
				Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
		},
	}
	matches := []WorkspaceContractMatchRecord{{
		ID: "contract:get-user", APIProject: "frontend/app", APIHTTPMethod: "GET",
		APIPath: "/users/{id}", APIFile: "src/api/users.ts", APILine: 12, APICaller: "loadUser",
		BackendProject: "microservices/ms-user", BackendHTTPMethod: "GET", BackendPath: "/users/{id}",
		BackendHandler: "UserController.get", BackendFile: "src/main/java/com/example/UserController.java",
		BackendLine: 30, Issue: contractIssueMatched, Confidence: "RESOLVED",
	}}
	flows := []WorkspaceFeatureFlowRecord{{
		FrontendProject: "frontend/app", FrontendComponent: "UserPage", FrontendCaller: "loadUser",
		FrontendFile: "src/api/users.ts", FrontendLine: 12, HTTPMethod: "GET", Path: "/users/{id}",
		BackendProject: "microservices/ms-user", BackendController: "UserController", BackendMethod: "get",
		BackendFile: "src/main/java/com/example/UserController.java", BackendLine: 30,
		FrontendSteps: []CodeFlowStep{
			{Name: "prepareUser", Kind: "function", File: "src/pages/UserPage.tsx", Line: 5, Confidence: "EXTRACTED"},
			{Name: "UserPage", Kind: "component", File: "src/pages/UserPage.tsx", Line: 8, Confidence: "EXTRACTED"},
			{Name: "loadUser", Kind: "api_helper", File: "src/api/users.ts", Line: 12, Confidence: "EXTRACTED"},
		},
		BackendSteps: []SpringEndpointFlowStep{{
			Owner: "UserService", Method: "find", Kind: "service",
			File: "src/main/java/com/example/UserService.java", Line: 17, Confidence: "EXTRACTED",
		}},
		Confidence: "RESOLVED",
	}}

	usages := BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, BuildWorkspaceEndpointTraces(matches, flows, nil))

	if len(usages) != 1 {
		t.Fatalf("usages = %#v, want one exact HTTP usage", usages)
	}
	if usages[0].ConsumerSymbolID != "symbol:user-page" {
		t.Fatalf("consumer = %q, want exact frontend component", usages[0].ConsumerSymbolID)
	}

	flows[0].FrontendSteps[1].File = "src/pages/OtherPage.tsx"
	usages = BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, BuildWorkspaceEndpointTraces(matches, flows, nil))
	for _, usage := range usages {
		if usage.Category == SymbolUsageReachedThroughAPI {
			t.Fatalf("mismatched component file produced reached-through-API usage: %#v", usage)
		}
	}

	flows[0].FrontendRouteFile = "src/pages/UserPage.tsx"
	flows[0].FrontendRouteLine = 8
	flows[0].FrontendSteps = flows[0].FrontendSteps[2:]
	usages = BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, BuildWorkspaceEndpointTraces(matches, flows, nil))
	if len(usages) != 1 || usages[0].ConsumerSymbolID != "symbol:user-page" {
		t.Fatalf("route-level component evidence did not select exact frontend component: %#v", usages)
	}
}

func TestWorkspaceSymbolAPIUsagesRemainSeparateFromDirectReferences(t *testing.T) {
	direct := CanonicalSymbolUsageRecord{
		ID:               StableWorkspaceUsageID("symbol:shared", "frontend/app", "symbol:user-page", SymbolUsageDirectReference, "imports_value", "symbol:shared", "src/pages/UserPage.tsx", 4),
		ProviderSymbolID: "symbol:shared",
		ConsumerProject:  "frontend/app",
		ConsumerSymbolID: "symbol:user-page",
		Category:         SymbolUsageDirectReference,
		Language:         "typescript",
		RelationKind:     "imports_value",
		SourceFile:       "src/pages/UserPage.tsx",
		SourceLine:       4,
		Confidence:       ConfidenceExact,
		Resolution:       SymbolResolutionExact,
	}
	api := CanonicalSymbolUsageRecord{
		ID:               StableWorkspaceUsageID("symbol:shared", "frontend/app", "symbol:user-page", SymbolUsageReachedThroughAPI, symbolAPIRelationKind, "contract:get-shared", "src/pages/UserPage.tsx", 8),
		ProviderSymbolID: "symbol:shared",
		ConsumerProject:  "frontend/app",
		ConsumerSymbolID: "symbol:user-page",
		Category:         SymbolUsageReachedThroughAPI,
		Language:         "typescript",
		RelationKind:     symbolAPIRelationKind,
		SourceFile:       "src/pages/UserPage.tsx",
		SourceLine:       8,
		Confidence:       ConfidenceExact,
		Resolution:       SymbolResolutionExact,
		Transport:        "http",
	}

	usages := mergeWorkspaceSymbolUsageRecords([]CanonicalSymbolUsageRecord{direct}, []CanonicalSymbolUsageRecord{api})

	if len(usages) != 2 {
		t.Fatalf("merged usages = %#v, want separate direct and API records", usages)
	}
	counts := map[SymbolUsageCategory]int{}
	ids := map[string]bool{}
	for _, usage := range usages {
		counts[usage.Category]++
		ids[usage.ID] = true
	}
	if counts[SymbolUsageDirectReference] != 1 || counts[SymbolUsageReachedThroughAPI] != 1 {
		t.Fatalf("category counts = %#v, want one direct and one API usage", counts)
	}
	if len(ids) != 2 {
		t.Fatalf("usage IDs were merged across categories: %#v", usages)
	}
}

func TestWorkspaceSymbolAPIUsageCarriesPartialFrontendContext(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	flows[0].FrontendConfidence = "WEAK_MATCH"
	flows[0].FrontendReason = "frontend route context reaches the API file without an exact route owner"

	usages := BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, BuildWorkspaceEndpointTraces(matches, flows, nil))

	if len(usages) != 1 || usages[0].Category != SymbolUsageReachedThroughAPI {
		t.Fatalf("usages = %#v, want one HTTP reachability usage", usages)
	}
	if usages[0].Confidence != ConfidenceNormalized {
		t.Fatalf("confidence = %q, want %q for partial frontend context", usages[0].Confidence, ConfidenceNormalized)
	}
	if !reflect.DeepEqual(usages[0].Limitations, []string{"frontend_route_context_partial"}) {
		t.Fatalf("limitations = %#v", usages[0].Limitations)
	}
}

func TestWorkspaceSymbolAPIUsageClassifiesAmbiguousAndUnresolvedJavaSteps(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	duplicate := symbols.Symbols[len(symbols.Symbols)-1]
	duplicate.ID = "symbol:user-service-duplicate"
	symbols.Symbols = append(symbols.Symbols, duplicate)

	ambiguous := BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, BuildWorkspaceEndpointTraces(matches, flows, nil))

	if len(ambiguous) != 2 {
		t.Fatalf("ambiguous usages = %#v, want one record per candidate", ambiguous)
	}
	wantCandidates := []string{"symbol:user-service", "symbol:user-service-duplicate"}
	for _, usage := range ambiguous {
		if usage.Category != SymbolUsageAmbiguous ||
			usage.Resolution != SymbolResolutionAmbiguous ||
			!reflect.DeepEqual(usage.CandidateSymbolIDs, wantCandidates) {
			t.Fatalf("ambiguous usage = %#v", usage)
		}
		if usage.APIPath[len(usage.APIPath)-1].SymbolID != usage.ProviderSymbolID {
			t.Fatalf("ambiguous path does not disclose selected candidate: %#v", usage)
		}
		for position, step := range usage.APIPath {
			if step.Position != position {
				t.Fatalf("ambiguous path positions = %#v", usage.APIPath)
			}
		}
	}

	symbols.Symbols = symbols.Symbols[:len(symbols.Symbols)-2]
	unresolved := BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, BuildWorkspaceEndpointTraces(matches, flows, nil))
	if len(unresolved) != 1 ||
		unresolved[0].Category != SymbolUsageUnresolved ||
		unresolved[0].Resolution != SymbolResolutionUnresolved ||
		unresolved[0].ProviderSymbolID != "" {
		t.Fatalf("unresolved usages = %#v", unresolved)
	}
}

func TestWorkspaceSymbolAPIProjectionMergesAndValidatesAllCategories(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	direct := CanonicalSymbolUsageRecord{
		ID:               "usage:direct",
		ProviderSymbolID: "symbol:load-user",
		ConsumerProject:  "frontend/app",
		ConsumerSymbolID: "symbol:user-page",
		Category:         SymbolUsageDirectReference,
		Language:         "typescript",
		RelationKind:     "calls_function",
		SourceFile:       "src/pages/UserPage.tsx",
		SourceLine:       9,
		Confidence:       ConfidenceExact,
		Resolution:       SymbolResolutionExact,
		Reason:           "one indexed declaration matches the evidenced target",
		Analyzer:         "typescript-source",
	}
	usageIndex := WorkspaceSymbolUsageIndexRecord{
		SchemaVersion: SchemaVersion,
		Usages:        []CanonicalSymbolUsageRecord{direct},
	}
	traces := BuildWorkspaceEndpointTraces(matches, flows, nil)

	merged, err := finalizeWorkspaceSymbolUsageProjection(symbols, usageIndex, matches, flows, traces)

	if err != nil {
		t.Fatalf("finalizeWorkspaceSymbolUsageProjection returned error: %v", err)
	}
	if len(merged.Usages) != 2 {
		t.Fatalf("merged usages = %#v, want direct and HTTP records", merged.Usages)
	}
	counts := map[SymbolUsageCategory]int{}
	for _, usage := range merged.Usages {
		counts[usage.Category]++
	}
	if counts[SymbolUsageDirectReference] != 1 || counts[SymbolUsageReachedThroughAPI] != 1 {
		t.Fatalf("merged category counts = %#v", counts)
	}
}

func workspaceSymbolAPIMinimalFixture() (WorkspaceSymbolIndexRecord, []WorkspaceContractMatchRecord, []WorkspaceFeatureFlowRecord) {
	symbols := WorkspaceSymbolIndexRecord{
		SchemaVersion: SchemaVersion,
		Symbols: []CanonicalSymbolRecord{
			{
				ID: "symbol:user-page", Project: "frontend/app", Language: "typescript",
				Kind: "function", Name: "UserPage", QualifiedName: "UserPage", ExportName: "UserPage",
				DeclarationFile: "src/pages/UserPage.tsx", DeclarationLine: 8,
				Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
			{
				ID: "symbol:load-user", Project: "frontend/app", Language: "typescript",
				Kind: "function", Name: "loadUser", QualifiedName: "loadUser", ExportName: "loadUser",
				DeclarationFile: "src/api/users.ts", DeclarationLine: 12,
				Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
			{
				ID: "symbol:user-service", Project: "microservices/ms-user", Language: "java",
				Kind: "class", Name: "UserService", QualifiedName: "com.example.UserService",
				DeclarationFile: "src/main/java/com/example/UserService.java", DeclarationLine: 10,
				Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
		},
	}
	matches := []WorkspaceContractMatchRecord{{
		ID: "contract:get-user", APIProject: "frontend/app", APIHTTPMethod: "GET",
		APIPath: "/users/{id}", APIFile: "src/api/users.ts", APILine: 12, APICaller: "loadUser",
		BackendProject: "microservices/ms-user", BackendHTTPMethod: "GET", BackendPath: "/users/{id}",
		BackendHandler: "UserController.get", BackendFile: "src/main/java/com/example/UserController.java",
		BackendLine: 30, Issue: contractIssueMatched, Confidence: "RESOLVED",
	}}
	flows := []WorkspaceFeatureFlowRecord{{
		FrontendProject: "frontend/app", FrontendComponent: "UserPage", FrontendCaller: "loadUser",
		FrontendFile: "src/api/users.ts", FrontendLine: 12, HTTPMethod: "GET", Path: "/users/{id}",
		BackendProject: "microservices/ms-user", BackendController: "UserController", BackendMethod: "get",
		BackendFile: "src/main/java/com/example/UserController.java", BackendLine: 30,
		FrontendSteps: []CodeFlowStep{
			{Name: "UserPage", Kind: "component", File: "src/pages/UserPage.tsx", Line: 8, Confidence: "EXTRACTED"},
			{Name: "loadUser", Kind: "api_helper", File: "src/api/users.ts", Line: 12, Confidence: "EXTRACTED"},
		},
		BackendSteps: []SpringEndpointFlowStep{{
			Owner: "UserService", Method: "find", Kind: "service",
			File: "src/main/java/com/example/UserService.java", Line: 17, Confidence: "EXTRACTED",
		}},
		Confidence: "RESOLVED",
	}}
	return symbols, matches, flows
}

func assertSymbolAPIPathStep(t *testing.T, step SymbolAPIPathStepRecord, symbolID, file string, line int, evidence []string) {
	t.Helper()
	if step.SymbolID != symbolID || step.File != file || step.Line != line || !reflect.DeepEqual(step.EvidenceIDs, evidence) {
		t.Fatalf("API path step = %#v, want symbol %q at %s:%d with evidence %#v", step, symbolID, file, line, evidence)
	}
}
