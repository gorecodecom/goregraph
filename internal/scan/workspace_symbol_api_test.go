package scan

import (
	"reflect"
	"strings"
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

func TestWorkspaceSymbolAPIUsageJoinsFullCallSiteIdentityOrderIndependently(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	matches[0].APILine = 22
	matches[0].APICaller = "loadUserDetails"
	wrong := flows[0]
	wrong.ID = "flow:wrong"
	wrong.FrontendComponent = "UserPage"
	wrong.FrontendCaller = "loadUser"
	wrong.FrontendLine = 12
	wrong.FrontendSteps[1].Name = "loadUser"
	exact := flows[0]
	exact.ID = "flow:exact"
	exact.FrontendComponent = "UserDetailsPage"
	exact.FrontendCaller = "loadUserDetails"
	exact.FrontendLine = 22
	exact.FrontendSteps = []CodeFlowStep{
		{Name: "UserDetailsPage", Kind: "component", File: "src/pages/UserDetailsPage.tsx", Line: 18, Confidence: "EXTRACTED"},
		{Name: "loadUserDetails", Kind: "api_helper", File: "src/api/users.ts", Line: 22, Confidence: "EXTRACTED"},
	}
	symbols.Symbols = append(symbols.Symbols,
		CanonicalSymbolRecord{
			ID: "symbol:user-details-page", Project: "frontend/app", Language: "typescript",
			Kind: "function", Name: "UserDetailsPage", QualifiedName: "UserDetailsPage", ExportName: "UserDetailsPage",
			DeclarationFile: "src/pages/UserDetailsPage.tsx", DeclarationLine: 18,
			Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
		CanonicalSymbolRecord{
			ID: "symbol:load-user-details", Project: "frontend/app", Language: "typescript",
			Kind: "function", Name: "loadUserDetails", QualifiedName: "loadUserDetails", ExportName: "loadUserDetails",
			DeclarationFile: "src/api/users.ts", DeclarationLine: 22,
			Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)

	for _, ordered := range [][]WorkspaceFeatureFlowRecord{{wrong, exact}, {exact, wrong}} {
		usages := BuildWorkspaceSymbolAPIUsages(symbols, matches, ordered, BuildWorkspaceEndpointTraces(matches, ordered, nil))
		if len(usages) != 1 ||
			usages[0].Category != SymbolUsageReachedThroughAPI ||
			usages[0].ConsumerSymbolID != "symbol:user-details-page" {
			t.Fatalf("call-site join for order %#v = %#v", []string{ordered[0].ID, ordered[1].ID}, usages)
		}
	}
}

func TestWorkspaceSymbolAPIUsageMarksMultipleSurvivingFlowsAmbiguous(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	matches[0].APILine = 0
	matches[0].APICaller = ""
	first := flows[0]
	first.ID = "flow:first"
	second := flows[0]
	second.ID = "flow:second"
	second.FrontendSteps = append([]CodeFlowStep(nil), flows[0].FrontendSteps...)
	second.FrontendComponent = "UserDetailsPage"
	second.FrontendSteps[0] = CodeFlowStep{
		Name: "UserDetailsPage", Kind: "component",
		File: "src/pages/UserDetailsPage.tsx", Line: 18, Confidence: "EXTRACTED",
	}
	symbols.Symbols = append(symbols.Symbols, CanonicalSymbolRecord{
		ID: "symbol:user-details-page", Project: "frontend/app", Language: "typescript",
		Kind: "function", Name: "UserDetailsPage", QualifiedName: "UserDetailsPage", ExportName: "UserDetailsPage",
		DeclarationFile: "src/pages/UserDetailsPage.tsx", DeclarationLine: 18,
		Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
	})
	wantCandidates := []string{"symbol:user-details-page", "symbol:user-page"}

	for _, ordered := range [][]WorkspaceFeatureFlowRecord{{first, second}, {second, first}} {
		usages := BuildWorkspaceSymbolAPIUsages(symbols, matches, ordered, BuildWorkspaceEndpointTraces(matches, ordered, nil))
		if len(usages) != 2 {
			t.Fatalf("ambiguous flow usages for order %#v = %#v", []string{ordered[0].ID, ordered[1].ID}, usages)
		}
		for _, usage := range usages {
			if usage.Category != SymbolUsageAmbiguous ||
				usage.Resolution != SymbolResolutionAmbiguous ||
				!reflect.DeepEqual(usage.CandidateSymbolIDs, wantCandidates) {
				t.Fatalf("ambiguous flow usage = %#v", usage)
			}
			if usage.ConsumerSymbolID != wantCandidates[0] && usage.ConsumerSymbolID != wantCandidates[1] {
				t.Fatalf("ambiguous consumer = %q", usage.ConsumerSymbolID)
			}
		}
	}
}

func TestWorkspaceSymbolAPIUsageKeepsSameOriginFlowCandidatesAmbiguous(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	matches[0].APILine = 0
	matches[0].APICaller = ""
	first := flows[0]
	first.ID = "flow:first"
	second := flows[0]
	second.ID = "flow:second"
	second.FrontendRouteID = "route:alternate"
	wantPaths := []string{"flow:first", "flow:second"}

	usages := BuildWorkspaceSymbolAPIUsages(symbols, matches, []WorkspaceFeatureFlowRecord{second, first}, BuildWorkspaceEndpointTraces(matches, []WorkspaceFeatureFlowRecord{second, first}, nil))

	if len(usages) != 2 {
		t.Fatalf("same-origin ambiguous usages = %#v", usages)
	}
	for _, usage := range usages {
		if usage.Category != SymbolUsageAmbiguous ||
			usage.Resolution != SymbolResolutionAmbiguous ||
			!reflect.DeepEqual(usage.CandidateSymbolIDs, []string{"symbol:user-page"}) ||
			!reflect.DeepEqual(usage.CandidatePathIDs, wantPaths) {
			t.Fatalf("same-origin ambiguous usage = %#v", usage)
		}
	}
}

func TestWorkspaceSymbolAPIUsagePreservesAllCandidatePathsWhenOriginUnresolved(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	symbols.Symbols = symbols.Symbols[1:]
	matches[0].APILine = 0
	matches[0].APICaller = ""
	first := flows[0]
	first.ID = "flow:first"
	second := flows[0]
	second.ID = "flow:second"
	second.FrontendRouteID = "route:alternate"
	wantPaths := []string{"flow:first", "flow:second"}

	usages := BuildWorkspaceSymbolAPIUsages(
		symbols,
		matches,
		[]WorkspaceFeatureFlowRecord{second, first},
		BuildWorkspaceEndpointTraces(matches, []WorkspaceFeatureFlowRecord{second, first}, nil),
	)

	if len(usages) != 1 ||
		usages[0].Category != SymbolUsageUnresolved ||
		!reflect.DeepEqual(usages[0].CandidatePathIDs, wantPaths) {
		t.Fatalf("unresolved origin usages = %#v, want every candidate path", usages)
	}
}

func TestWorkspaceSymbolAPIUsagePreservesAllCandidatePathsWithProviderAmbiguity(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	duplicateProvider := symbols.Symbols[len(symbols.Symbols)-1]
	duplicateProvider.ID = "symbol:user-service-duplicate"
	symbols.Symbols = append(symbols.Symbols, duplicateProvider)
	matches[0].APILine = 0
	matches[0].APICaller = ""
	first := flows[0]
	first.ID = "flow:first"
	second := flows[0]
	second.ID = "flow:second"
	second.FrontendRouteID = "route:alternate"
	wantPaths := []string{"flow:first", "flow:second"}
	wantProviders := []string{"symbol:user-service", "symbol:user-service-duplicate"}

	usages := BuildWorkspaceSymbolAPIUsages(
		symbols,
		matches,
		[]WorkspaceFeatureFlowRecord{second, first},
		BuildWorkspaceEndpointTraces(matches, []WorkspaceFeatureFlowRecord{second, first}, nil),
	)

	if len(usages) != 4 {
		t.Fatalf("provider/flow ambiguous usages = %#v", usages)
	}
	for _, usage := range usages {
		if usage.Category != SymbolUsageAmbiguous ||
			!reflect.DeepEqual(usage.CandidateSymbolIDs, wantProviders) ||
			!reflect.DeepEqual(usage.CandidatePathIDs, wantPaths) {
			t.Fatalf("provider/flow ambiguous usage = %#v", usage)
		}
	}
}

func TestWorkspaceSymbolAPIUsagePreservesAllCandidatePathsWhenBackendStepsMissing(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	matches[0].APILine = 0
	matches[0].APICaller = ""
	first := flows[0]
	first.ID = "flow:first"
	first.BackendSteps = nil
	second := flows[0]
	second.ID = "flow:second"
	second.FrontendRouteID = "route:alternate"
	second.BackendSteps = nil
	wantPaths := []string{"flow:first", "flow:second"}

	usages := BuildWorkspaceSymbolAPIUsages(
		symbols,
		matches,
		[]WorkspaceFeatureFlowRecord{second, first},
		BuildWorkspaceEndpointTraces(matches, []WorkspaceFeatureFlowRecord{second, first}, nil),
	)

	if len(usages) != 2 {
		t.Fatalf("missing backend step usages = %#v", usages)
	}
	for _, usage := range usages {
		if usage.Category != SymbolUsageUnresolved ||
			!reflect.DeepEqual(usage.CandidatePathIDs, wantPaths) {
			t.Fatalf("missing backend step usage = %#v, want every candidate path", usage)
		}
	}
}

func TestWorkspaceSymbolAPIUsagePreservesSingleCandidatePathForNonExactProviders(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	flows[0].ID = "flow:only"
	wantPaths := []string{"flow:only"}

	withoutProvider := symbols
	withoutProvider.Symbols = withoutProvider.Symbols[:len(withoutProvider.Symbols)-1]
	unresolved := BuildWorkspaceSymbolAPIUsages(
		withoutProvider,
		matches,
		flows,
		BuildWorkspaceEndpointTraces(matches, flows, nil),
	)
	if len(unresolved) != 1 ||
		unresolved[0].Category != SymbolUsageUnresolved ||
		!reflect.DeepEqual(unresolved[0].CandidatePathIDs, wantPaths) ||
		workspaceSymbolUsageHasLimitation(unresolved[0], "feature_flow_join_ambiguous") ||
		strings.Contains(unresolved[0].Reason, "multiple feature flows") {
		t.Fatalf("single-path unresolved provider usage = %#v", unresolved)
	}

	ambiguousSymbols := symbols
	duplicate := ambiguousSymbols.Symbols[len(ambiguousSymbols.Symbols)-1]
	duplicate.ID = "symbol:user-service-duplicate"
	ambiguousSymbols.Symbols = append(ambiguousSymbols.Symbols, duplicate)
	ambiguous := BuildWorkspaceSymbolAPIUsages(
		ambiguousSymbols,
		matches,
		flows,
		BuildWorkspaceEndpointTraces(matches, flows, nil),
	)
	if len(ambiguous) != 2 {
		t.Fatalf("single-path ambiguous provider usages = %#v", ambiguous)
	}
	for _, usage := range ambiguous {
		if usage.Category != SymbolUsageAmbiguous ||
			!reflect.DeepEqual(usage.CandidatePathIDs, wantPaths) ||
			workspaceSymbolUsageHasLimitation(usage, "feature_flow_join_ambiguous") ||
			strings.Contains(usage.Reason, "multiple feature flows") {
			t.Fatalf("single-path ambiguous provider usage = %#v", usage)
		}
	}
}

func TestWorkspaceSymbolAPIUsagePreservesJavaScriptLanguageWhenUnresolved(t *testing.T) {
	t.Run("helper and API facts", func(t *testing.T) {
		symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
		symbols.Symbols = symbols.Symbols[1:]
		symbols.Symbols[0].Language = "javascript"
		for index := range flows[0].FrontendSteps {
			flows[0].FrontendSteps[index].Language = "javascript"
		}

		usages := BuildWorkspaceSymbolAPIUsages(
			symbols,
			matches,
			flows,
			BuildWorkspaceEndpointTraces(matches, flows, nil),
		)

		if len(usages) != 1 ||
			usages[0].Category != SymbolUsageUnresolved ||
			usages[0].Language != "javascript" ||
			!reflect.DeepEqual(usages[0].Limitations, []string{"frontend_origin_unresolved"}) {
			t.Fatalf("JavaScript missing-origin usage = %#v", usages)
		}
	})

	t.Run("project capability", func(t *testing.T) {
		_, matches, _ := workspaceSymbolAPIMinimalFixture()
		matches[0].APIFile = "src/api/users"
		symbols := WorkspaceSymbolIndexRecord{
			Coverage: []SymbolCoverageRecord{{
				Project:    "frontend/app",
				Language:   "javascript",
				Capability: "declarations",
				Coverage:   CoveragePartial,
			}},
		}

		usages := BuildWorkspaceSymbolAPIUsages(symbols, matches, nil, WorkspaceEndpointTraceIndexRecord{})

		if len(usages) != 1 ||
			usages[0].Category != SymbolUsageUnresolved ||
			usages[0].Language != "javascript" ||
			!reflect.DeepEqual(usages[0].Limitations, []string{"workspace_feature_flow_missing"}) {
			t.Fatalf("JavaScript capability-derived usage = %#v", usages)
		}
	})

	t.Run("API extension before mixed project capability", func(t *testing.T) {
		_, matches, _ := workspaceSymbolAPIMinimalFixture()
		symbols := WorkspaceSymbolIndexRecord{
			Coverage: []SymbolCoverageRecord{
				{
					Project:    "frontend/app",
					Language:   "javascript",
					Capability: "declarations",
					Coverage:   CoveragePartial,
				},
				{
					Project:    "frontend/app",
					Language:   "typescript",
					Capability: "declarations",
					Coverage:   CoveragePartial,
				},
			},
		}

		usages := BuildWorkspaceSymbolAPIUsages(symbols, matches, nil, WorkspaceEndpointTraceIndexRecord{})

		if len(usages) != 1 ||
			usages[0].Category != SymbolUsageUnresolved ||
			usages[0].Language != "typescript" {
			t.Fatalf("mixed-project TypeScript API usage = %#v", usages)
		}
	})
}

func TestWorkspaceSymbolAPIUsageCoverageReportsCompletePartialAndFailedEvidence(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	frontend := workspaceIndexProject{
		record:    WorkspaceProjectRecord{Path: "frontend/app", Kind: "frontend", Indexed: true},
		codeFlows: []CodeFlowRecord{{Kind: "frontend", File: "src/pages/UserPage.tsx"}},
	}
	backend := workspaceIndexProject{
		record:        WorkspaceProjectRecord{Path: "microservices/ms-user", Kind: "backend", Indexed: true},
		endpointFlows: []SpringEndpointFlowRecord{{HTTPMethod: "GET", Path: "/users/{id}", Steps: flows[0].BackendSteps}},
	}

	complete := BuildWorkspaceSymbolAPIUsageCoverage(symbols, matches, flows, []workspaceIndexProject{frontend, backend})
	assertSymbolCoverage(t, complete, "frontend/app", "typescript", symbolAPIRelationKind, CoverageComplete, nil)
	assertSymbolCoverage(t, complete, "microservices/ms-user", "java", symbolAPIRelationKind, CoverageComplete, nil)

	frontend.missingFacts = []string{"flows.json"}
	backend.loadFailures = []string{"microservices/ms-user/endpoint-flows.json: invalid character"}
	degraded := BuildWorkspaceSymbolAPIUsageCoverage(symbols, matches, flows, []workspaceIndexProject{frontend, backend})
	assertSymbolCoverage(t, degraded, "frontend/app", "typescript", symbolAPIRelationKind, CoveragePartial, []string{"flows_missing"})
	assertSymbolCoverage(t, degraded, "microservices/ms-user", "java", symbolAPIRelationKind, CoverageFailed, []string{"endpoint_flows_unreadable"})
}

func TestWorkspaceSymbolAPIUsageCoverageRequiresUniqueJavaProvider(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	frontend := workspaceIndexProject{
		record:    WorkspaceProjectRecord{Path: "frontend/app", Kind: "frontend", Indexed: true},
		codeFlows: []CodeFlowRecord{{Kind: "frontend", File: "src/pages/UserPage.tsx"}},
	}
	backend := workspaceIndexProject{
		record:        WorkspaceProjectRecord{Path: "microservices/ms-user", Kind: "backend", Indexed: true},
		endpointFlows: []SpringEndpointFlowRecord{{HTTPMethod: "GET", Path: "/users/{id}", Steps: flows[0].BackendSteps}},
	}
	projects := []workspaceIndexProject{frontend, backend}

	withoutProvider := symbols
	withoutProvider.Symbols = withoutProvider.Symbols[:len(withoutProvider.Symbols)-1]
	unresolved := BuildWorkspaceSymbolAPIUsageCoverage(withoutProvider, matches, flows, projects)
	for _, want := range []struct {
		project  string
		language string
	}{
		{project: "frontend/app", language: "typescript"},
		{project: "microservices/ms-user", language: "java"},
	} {
		assertSymbolCoverage(t, unresolved, want.project, want.language, symbolAPIRelationKind, CoveragePartial, []string{"java_provider_unresolved"})
		record := findSymbolCoverage(unresolved, want.project, want.language, symbolAPIRelationKind)
		if !strings.Contains(record.Reason, "Java provider") {
			t.Fatalf("unresolved provider coverage = %#v", record)
		}
	}

	ambiguousSymbols := symbols
	duplicate := ambiguousSymbols.Symbols[len(ambiguousSymbols.Symbols)-1]
	duplicate.ID = "symbol:user-service-duplicate"
	ambiguousSymbols.Symbols = append(ambiguousSymbols.Symbols, duplicate)
	ambiguous := BuildWorkspaceSymbolAPIUsageCoverage(ambiguousSymbols, matches, flows, projects)
	for _, want := range []struct {
		project  string
		language string
	}{
		{project: "frontend/app", language: "typescript"},
		{project: "microservices/ms-user", language: "java"},
	} {
		assertSymbolCoverage(t, ambiguous, want.project, want.language, symbolAPIRelationKind, CoveragePartial, []string{"java_provider_ambiguous"})
		record := findSymbolCoverage(ambiguous, want.project, want.language, symbolAPIRelationKind)
		if !strings.Contains(record.Reason, "Java provider") {
			t.Fatalf("ambiguous provider coverage = %#v", record)
		}
	}
}

func TestWorkspaceSymbolAPIUsageCoverageDerivesLanguagesWithoutCanonicalSymbols(t *testing.T) {
	frontend := workspaceIndexProject{
		record:       WorkspaceProjectRecord{Path: "frontend/app", Kind: "frontend", Indexed: true},
		capabilities: []CapabilityRecord{{ID: CapabilitySymbols, Language: "typescript", Coverage: CoverageFailed}},
		loadFailures: []string{"frontend/app/flows.json: invalid character"},
	}
	backend := workspaceIndexProject{
		record:       WorkspaceProjectRecord{Path: "microservices/ms-user", Kind: "backend", Indexed: true},
		capabilities: []CapabilityRecord{{ID: CapabilitySymbols, Language: "java", Coverage: CoverageFailed}},
		loadFailures: []string{"microservices/ms-user/endpoint-flows.json: invalid character"},
	}
	symbols := WorkspaceSymbolIndexRecord{
		Symbols: []CanonicalSymbolRecord{{
			Project: "frontend/app",
			Name:    "malformed-without-language",
		}},
	}

	failed := BuildWorkspaceSymbolAPIUsageCoverage(symbols, nil, nil, []workspaceIndexProject{frontend, backend})
	assertSymbolCoverage(t, failed, "frontend/app", "typescript", symbolAPIRelationKind, CoverageFailed, []string{"flows_unreadable"})
	assertSymbolCoverage(t, failed, "microservices/ms-user", "java", symbolAPIRelationKind, CoverageFailed, []string{"endpoint_flows_unreadable"})

	frontend.capabilities = nil
	frontend.loadFailures = nil
	frontend.missingFacts = []string{"flows.json"}
	frontend.codeFlows = []CodeFlowRecord{{Language: "typescript", Kind: "frontend"}}
	backend.capabilities = nil
	backend.loadFailures = nil
	backend.missingFacts = []string{"endpoint-flows.json"}
	backend.endpointFlows = []SpringEndpointFlowRecord{{HTTPMethod: "GET", Path: "/users/{id}"}}

	partial := BuildWorkspaceSymbolAPIUsageCoverage(symbols, nil, nil, []workspaceIndexProject{frontend, backend})
	assertSymbolCoverage(t, partial, "frontend/app", "typescript", symbolAPIRelationKind, CoveragePartial, []string{"flows_missing"})
	assertSymbolCoverage(t, partial, "microservices/ms-user", "java", symbolAPIRelationKind, CoveragePartial, []string{"endpoint_flows_missing"})
}

func TestWorkspaceSymbolAPIUsageCoverageDistinguishesNoUsageFromMissingEvidence(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	frontend := workspaceIndexProject{
		record:    WorkspaceProjectRecord{Path: "frontend/app", Kind: "frontend", Indexed: true},
		codeFlows: []CodeFlowRecord{{Kind: "frontend", File: "src/pages/UserPage.tsx"}},
	}
	backend := workspaceIndexProject{
		record:        WorkspaceProjectRecord{Path: "microservices/ms-user", Kind: "backend", Indexed: true},
		endpointFlows: []SpringEndpointFlowRecord{{HTTPMethod: "GET", Path: "/users/{id}", Steps: flows[0].BackendSteps}},
	}

	noUsage := BuildWorkspaceSymbolAPIUsageCoverage(symbols, nil, nil, []workspaceIndexProject{frontend, backend})
	for _, want := range []struct {
		project  string
		language string
	}{
		{project: "frontend/app", language: "typescript"},
		{project: "microservices/ms-user", language: "java"},
	} {
		record := findSymbolCoverage(noUsage, want.project, want.language, symbolAPIRelationKind)
		if record.Coverage != CoverageComplete || !strings.Contains(record.Reason, "no resolved HTTP contracts") {
			t.Fatalf("no-usage coverage for %s = %#v", want.project, record)
		}
	}

	flows[0].BackendSteps = nil
	usages := BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, BuildWorkspaceEndpointTraces(matches, flows, nil))
	if len(usages) != 1 ||
		usages[0].Category != SymbolUsageUnresolved ||
		!reflect.DeepEqual(usages[0].Limitations, []string{"backend_implementation_steps_missing"}) {
		t.Fatalf("missing backend steps usages = %#v", usages)
	}
	missing := BuildWorkspaceSymbolAPIUsageCoverage(symbols, matches, flows, []workspaceIndexProject{frontend, backend})
	assertSymbolCoverage(t, missing, "microservices/ms-user", "java", symbolAPIRelationKind, CoveragePartial, []string{"backend_implementation_steps_missing"})
}

func TestWorkspaceSymbolAPIUsageSurfacesMissingAndAmbiguousOrigins(t *testing.T) {
	symbols, matches, flows := workspaceSymbolAPIMinimalFixture()
	missingFlows := BuildWorkspaceSymbolAPIUsages(symbols, matches, nil, WorkspaceEndpointTraceIndexRecord{})
	if len(missingFlows) != 1 ||
		missingFlows[0].Category != SymbolUsageUnresolved ||
		!reflect.DeepEqual(missingFlows[0].Limitations, []string{"workspace_feature_flow_missing"}) {
		t.Fatalf("missing feature flow usages = %#v", missingFlows)
	}

	duplicate := symbols.Symbols[0]
	duplicate.ID = "symbol:user-page-duplicate"
	symbols.Symbols = append(symbols.Symbols, duplicate)
	ambiguousOrigin := BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, BuildWorkspaceEndpointTraces(matches, flows, nil))
	if len(ambiguousOrigin) != 1 ||
		ambiguousOrigin[0].Category != SymbolUsageUnresolved ||
		!reflect.DeepEqual(ambiguousOrigin[0].Limitations, []string{"frontend_origin_unresolved"}) {
		t.Fatalf("ambiguous origin usages = %#v", ambiguousOrigin)
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
