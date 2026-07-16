package scan

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestWorkspaceSymbolsExcludeUnrelatedJavaSameName(t *testing.T) {
	providerProject := WorkspaceProjectRecord{
		Path:        "microservices/ms-userservice",
		Kind:        "backend",
		Service:     "ms-userservice",
		BuildSystem: "maven",
		Indexed:     true,
	}
	consumerProject := WorkspaceProjectRecord{
		Path:        "microservices/ms-consumer",
		Kind:        "backend",
		Service:     "ms-consumer",
		BuildSystem: "maven",
		Indexed:     true,
	}
	unrelatedProject := WorkspaceProjectRecord{
		Path:        "microservices/ms-cadastertask",
		Kind:        "backend",
		Service:     "ms-cadastertask",
		BuildSystem: "maven",
		Indexed:     true,
	}
	provider := RichSymbolRecord{
		ID:            "local-provider",
		Name:          "UserService",
		Kind:          "interface",
		Language:      "java",
		File:          "src/main/java/com/weka/wbp/api/userservice/service/UserService.java",
		Line:          7,
		QualifiedName: "com.weka.wbp.api.userservice.service.UserService",
		Package:       "com.weka.wbp.api.userservice.service",
		Artifact:      "com.weka:users-api",
		Analyzer:      "java-source",
		Confidence:    ConfidenceExact,
		Coverage:      CoverageComplete,
	}
	consumer := RichSymbolRecord{
		ID:            "local-consumer",
		Name:          "UserClient",
		Kind:          "class",
		Language:      "java",
		File:          "src/main/java/com/weka/consumer/UserClient.java",
		Line:          5,
		QualifiedName: "com.weka.consumer.UserClient",
		Package:       "com.weka.consumer",
		Artifact:      "com.weka:consumer",
		Analyzer:      "java-source",
		Confidence:    ConfidenceExact,
		Coverage:      CoverageComplete,
	}
	unrelated := RichSymbolRecord{
		ID:            "local-unrelated",
		Name:          "UserService",
		Kind:          "class",
		Language:      "java",
		File:          "src/main/java/com/weka/cadastertask/UserService.java",
		Line:          4,
		QualifiedName: "com.weka.cadastertask.UserService",
		Package:       "com.weka.cadastertask",
		Artifact:      "com.weka:cadaster-task",
		Analyzer:      "java-source",
		Confidence:    ConfidenceExact,
		Coverage:      CoverageComplete,
	}
	reference := RichRelationRecord{
		ID:                  "local-reference",
		From:                consumer.File,
		Type:                "field_type",
		Language:            "java",
		Analyzer:            "java-source",
		Line:                9,
		Confidence:          string(ConfidenceNormalized),
		ConfidenceScore:     0.8,
		FromSymbolID:        consumer.ID,
		TargetQualifiedName: provider.QualifiedName,
		Resolution:          SymbolResolutionUnresolved,
		EvidenceIDs:         []string{"evidence:consumer-reference"},
	}
	registry := WorkspaceRegistryRecord{
		Root:      "/workspace",
		Generated: "2026-07-16T10:00:00Z",
		Projects:  []WorkspaceProjectRecord{providerProject, consumerProject, unrelatedProject},
	}
	projects := []workspaceIndexProject{
		{
			record:  providerProject,
			symbols: []RichSymbolRecord{provider},
		},
		{
			record:    consumerProject,
			symbols:   []RichSymbolRecord{consumer},
			relations: []RichRelationRecord{reference},
			maven: MavenGraphRecord{Edges: []MavenEdgeRecord{{
				From: "com.weka:consumer",
				To:   "com.weka:users-api",
				Type: "depends_on",
			}}},
		},
		{
			record:  unrelatedProject,
			symbols: []RichSymbolRecord{unrelated},
		},
	}

	symbols, usages, err := BuildWorkspaceSymbolProjection(registry, projects, registry.Generated)
	if err != nil {
		t.Fatalf("BuildWorkspaceSymbolProjection returned error: %v", err)
	}
	if len(symbols.Symbols) != 3 {
		t.Fatalf("symbols = %#v, want three canonical declarations", symbols.Symbols)
	}
	if len(usages.Usages) != 1 {
		t.Fatalf("usages = %#v, want one direct usage", usages.Usages)
	}

	wantProviderID := StableWorkspaceSymbolID(
		provider.Kind,
		providerProject.Path,
		provider.Artifact,
		provider.Language,
		provider.QualifiedName,
		provider.File,
	)
	usage := usages.Usages[0]
	if usage.Category != SymbolUsageDirectReference ||
		usage.Resolution != SymbolResolutionExact ||
		usage.ProviderSymbolID != wantProviderID {
		t.Fatalf("usage = %#v, want exact provider %q", usage, wantProviderID)
	}
	if !reflect.DeepEqual(usage.DependencyEvidence, []string{"maven:com.weka:consumer -> com.weka:users-api"}) {
		t.Fatalf("dependency evidence = %#v", usage.DependencyEvidence)
	}
}

func TestWorkspaceSymbolsConsumeProjectCallGraphReferences(t *testing.T) {
	provider := workspaceJavaProject(
		"microservices/ms-users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "users", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", QualifiedName: "com.weka.UserService",
			Artifact: "com.weka:users", Analyzer: "java-source",
			Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	consumer := workspaceJavaConsumer(
		"microservices/ms-consumer",
		"com.weka:consumer",
		"com.weka.UserService",
	)
	consumer.relations = nil
	consumer.callGraph = CallGraphRecord{Edges: []CallGraphEdgeRecord{{
		ID: "call", From: MethodRefRecord{Owner: "Consumer", Method: "run"},
		To:   MethodRefRecord{Owner: "UserService", Method: "load"},
		Type: "calls_method_owner", SourceFile: consumer.symbols[0].File, Line: 12,
		FromSymbolID: consumer.symbols[0].ID, TargetQualifiedName: "com.weka.UserService",
		Resolution: SymbolResolutionUnresolved, EvidenceIDs: []string{"evidence:call"},
	}}}
	consumer.maven = MavenGraphRecord{Edges: []MavenEdgeRecord{{
		From: "com.weka:consumer", To: "com.weka:users", Type: "depends_on",
	}}}
	registry := workspaceSymbolRegistry(provider.record, consumer.record)

	_, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{provider, consumer}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages.Usages) != 1 {
		t.Fatalf("call graph usages = %#v", usages.Usages)
	}
	usage := usages.Usages[0]
	if usage.Resolution != SymbolResolutionExact ||
		!reflect.DeepEqual(usage.EvidenceIDs, []string{consumer.record.Path + "#evidence:call"}) {
		t.Fatalf("call graph usage = %#v", usage)
	}
}

func TestWorkspaceSymbolsAttachProjectEvidenceBySourceLocation(t *testing.T) {
	project := workspaceJavaProject(
		"microservices/ms-users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "users", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", Line: 7, QualifiedName: "com.weka.UserService",
			Artifact: "com.weka:users", Analyzer: "java-source",
			Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	project.evidence = []EvidenceRecord{{
		ID: "evidence:source", Project: "ms-users", File: "src/UserService.java",
		Start: EvidenceLocation{Line: 7}, Analyzer: "java", Method: "syntax",
		Reason: "class declaration",
	}}
	registry := workspaceSymbolRegistry(project.record)

	symbols, _, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{project}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols.Symbols) != 1 ||
		!reflect.DeepEqual(symbols.Symbols[0].EvidenceIDs, []string{project.record.Path + "#evidence:source"}) {
		t.Fatalf("canonical evidence = %#v", symbols.Symbols)
	}
}

func TestWorkspaceSymbolsResolveDuplicateJavaFQNByMavenArtifact(t *testing.T) {
	fqn := "com.weka.wbp.api.userservice.service.UserService"
	providerA := workspaceJavaProject(
		"microservices/ms-users-api",
		"com.weka:users-api",
		RichSymbolRecord{
			ID:            "provider-a",
			Name:          "UserService",
			Kind:          "interface",
			Language:      "java",
			File:          "src/main/java/com/weka/wbp/api/userservice/service/UserService.java",
			QualifiedName: fqn,
			Artifact:      "com.weka:users-api",
			Analyzer:      "java-source",
			Confidence:    ConfidenceExact,
			Coverage:      CoverageComplete,
		},
	)
	providerB := workspaceJavaProject(
		"microservices/ms-users-legacy",
		"com.weka:users-legacy",
		RichSymbolRecord{
			ID:            "provider-b",
			Name:          "UserService",
			Kind:          "interface",
			Language:      "java",
			File:          "src/main/java/com/weka/wbp/api/userservice/service/UserService.java",
			QualifiedName: fqn,
			Artifact:      "com.weka:users-legacy",
			Analyzer:      "java-source",
			Confidence:    ConfidenceExact,
			Coverage:      CoverageComplete,
		},
	)
	consumer := workspaceJavaConsumer("microservices/ms-consumer", "com.weka:consumer", fqn)
	consumer.relations[0].FromSymbolID = ""
	consumer.maven = MavenGraphRecord{Edges: []MavenEdgeRecord{{
		From: "com.weka:consumer",
		To:   "com.weka:users-api",
		Type: "depends_on",
	}}}
	registry := workspaceSymbolRegistry(providerA.record, providerB.record, consumer.record)

	_, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{providerA, providerB, consumer}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages.Usages) != 1 {
		t.Fatalf("usages = %#v, want one exact usage", usages.Usages)
	}
	want := StableWorkspaceSymbolID(
		providerA.symbols[0].Kind,
		providerA.record.Path,
		providerA.symbols[0].Artifact,
		providerA.symbols[0].Language,
		fqn,
		providerA.symbols[0].File,
	)
	got := usages.Usages[0]
	if got.Resolution != SymbolResolutionExact || got.ProviderSymbolID != want {
		t.Fatalf("usage = %#v, want Maven-selected provider %q", got, want)
	}
	if !reflect.DeepEqual(got.DependencyEvidence, []string{"maven:com.weka:consumer -> com.weka:users-api"}) {
		t.Fatalf("dependency evidence = %#v", got.DependencyEvidence)
	}
}

func TestWorkspaceSymbolsRequireJavaDependencyEvidenceForCrossProjectExact(t *testing.T) {
	fqn := "com.weka.UserService"
	provider := workspaceJavaProject(
		"microservices/ms-users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "provider", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", QualifiedName: fqn, Artifact: "com.weka:users",
			Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	consumer := workspaceJavaConsumer("microservices/ms-consumer", "com.weka:consumer", fqn)
	consumer.maven.Edges = nil
	registry := workspaceSymbolRegistry(provider.record, consumer.record)

	_, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{provider, consumer}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages.Usages) != 1 ||
		usages.Usages[0].Resolution != SymbolResolutionUnresolved ||
		usages.Usages[0].ProviderSymbolID != "" {
		t.Fatalf("unique cross-project FQN without dependency became Exact: %#v", usages.Usages)
	}
}

func TestWorkspaceSymbolsDoNotPromoteNonPromotableJavaReference(t *testing.T) {
	fqn := "com.weka.UserService"
	provider := workspaceJavaProject(
		"microservices/ms-users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "provider", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", QualifiedName: fqn, Artifact: "com.weka:users",
			Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	consumer := workspaceJavaConsumer("microservices/ms-consumer", "com.weka:consumer", fqn)
	consumer.maven = MavenGraphRecord{Edges: []MavenEdgeRecord{{
		From: "com.weka:consumer", To: "com.weka:users", Type: "depends_on",
	}}}
	consumer.relations[0].NonPromotable = true
	consumer.relations = jsonRoundTrip[[]RichRelationRecord](t, consumer.relations)
	registry := workspaceSymbolRegistry(provider.record, consumer.record)

	_, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{provider, consumer}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages.Usages) != 1 ||
		usages.Usages[0].Resolution != SymbolResolutionUnresolved ||
		usages.Usages[0].ProviderSymbolID != "" {
		t.Fatalf("non-promotable reference became Exact: %#v", usages.Usages)
	}
}

func TestWorkspaceSymbolsDoNotPromoteProjectAmbiguousJavaReference(t *testing.T) {
	fqn := "com.weka.UserService"
	provider := workspaceJavaProject(
		"microservices/ms-users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "provider", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", QualifiedName: fqn, Artifact: "com.weka:users",
			Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	consumer := workspaceJavaConsumer("microservices/ms-consumer", "com.weka:consumer", fqn)
	consumer.maven = MavenGraphRecord{Edges: []MavenEdgeRecord{{
		From: "com.weka:consumer", To: "com.weka:users", Type: "depends_on",
	}}}
	consumer.relations[0].Resolution = SymbolResolutionAmbiguous
	registry := workspaceSymbolRegistry(provider.record, consumer.record)

	_, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{provider, consumer}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages.Usages) != 1 ||
		usages.Usages[0].Resolution == SymbolResolutionExact ||
		usages.Usages[0].ProviderSymbolID != "" {
		t.Fatalf("project-ambiguous reference became Exact: %#v", usages.Usages)
	}
}

func TestWorkspaceSymbolsPreserveEveryAmbiguousJavaCandidate(t *testing.T) {
	fqn := "com.weka.shared.UserService"
	providerA := workspaceJavaProject(
		"microservices/ms-a",
		"com.weka:a",
		RichSymbolRecord{
			ID: "a", Name: "UserService", Kind: "class", Language: "java",
			File: "src/a/UserService.java", QualifiedName: fqn, Artifact: "com.weka:a",
			Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	providerB := workspaceJavaProject(
		"microservices/ms-b",
		"com.weka:b",
		RichSymbolRecord{
			ID: "b", Name: "UserService", Kind: "class", Language: "java",
			File: "src/b/UserService.java", QualifiedName: fqn, Artifact: "com.weka:b",
			Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	consumer := workspaceJavaConsumer("microservices/ms-consumer", "com.weka:consumer", fqn)
	registry := workspaceSymbolRegistry(providerA.record, providerB.record, consumer.record)

	_, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{consumer, providerB, providerA}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages.Usages) != 2 {
		t.Fatalf("usages = %#v, want one ambiguity record per provider", usages.Usages)
	}
	candidates := []string{
		StableWorkspaceSymbolID("class", providerA.record.Path, "com.weka:a", "java", fqn, "src/a/UserService.java"),
		StableWorkspaceSymbolID("class", providerB.record.Path, "com.weka:b", "java", fqn, "src/b/UserService.java"),
	}
	if candidates[0] > candidates[1] {
		candidates[0], candidates[1] = candidates[1], candidates[0]
	}
	for _, usage := range usages.Usages {
		if usage.Category != SymbolUsageAmbiguous ||
			usage.Resolution != SymbolResolutionAmbiguous ||
			!reflect.DeepEqual(usage.CandidateSymbolIDs, candidates) ||
			usage.Reason != "multiple indexed declarations remain after dependency filtering" {
			t.Fatalf("ambiguous usage = %#v", usage)
		}
	}
	if usages.Usages[0].ProviderSymbolID == usages.Usages[1].ProviderSymbolID {
		t.Fatalf("ambiguous providers were collapsed: %#v", usages.Usages)
	}
}

func TestWorkspaceSymbolsResolveJavaScriptWorkspacePackageDependency(t *testing.T) {
	provider := workspaceScriptProject(
		"frontend/packages/users",
		"@weka/users",
		RichSymbolRecord{
			ID: "users-export", Name: "UserService", ExportName: "UserService",
			Kind: "class", Language: "typescript", File: "src/index.ts",
			QualifiedName: "src/index#UserService", Module: "src/index",
			WorkspacePackage: "@weka/users", Analyzer: "typescript-source",
			Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	unrelated := workspaceScriptProject(
		"frontend/packages/cadaster",
		"@weka/cadaster",
		RichSymbolRecord{
			ID: "cadaster-export", Name: "UserService", ExportName: "UserService",
			Kind: "class", Language: "typescript", File: "src/index.ts",
			QualifiedName: "src/index#UserService", Module: "src/index",
			WorkspacePackage: "@weka/cadaster", Analyzer: "typescript-source",
			Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	consumerRecord := WorkspaceProjectRecord{
		Path: "frontend/apps/admin", Kind: "frontend", BuildSystem: "node", Indexed: true,
	}
	consumerSymbol := RichSymbolRecord{
		ID: "app", Name: "App", ExportName: "App", Kind: "function", Language: "typescript",
		File: "src/App.ts", QualifiedName: "src/App#App", Module: "src/App",
		WorkspacePackage: "@weka/admin", Analyzer: "typescript-source",
		Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}
	consumer := workspaceIndexProject{
		record:  consumerRecord,
		symbols: []RichSymbolRecord{consumerSymbol},
		relations: []RichRelationRecord{{
			ID: "import-users", From: "src/App.ts",
			Type: "imports_export", Language: "typescript", Analyzer: "typescript-source", Line: 3,
			TargetModule: "@weka/users", TargetExport: "UserService",
			Resolution: SymbolResolutionUnresolved,
		}},
		packages: PackageGraphRecord{Edges: []PackageEdgeRecord{{
			From: "@weka/admin", To: "@weka/users", Type: "depends_on",
		}}},
	}
	registry := workspaceSymbolRegistry(provider.record, unrelated.record, consumer.record)

	_, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{unrelated, consumer, provider}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages.Usages) != 1 {
		t.Fatalf("usages = %#v, want one exact workspace-package usage", usages.Usages)
	}
	want := StableWorkspaceSymbolID(
		provider.symbols[0].Kind,
		provider.record.Path,
		"@weka/users",
		"typescript",
		provider.symbols[0].QualifiedName,
		provider.symbols[0].File,
	)
	got := usages.Usages[0]
	if got.Resolution != SymbolResolutionExact || got.ProviderSymbolID != want {
		t.Fatalf("usage = %#v, want workspace-package provider %q", got, want)
	}
	if !reflect.DeepEqual(got.DependencyEvidence, []string{"node:@weka/admin -> @weka/users"}) {
		t.Fatalf("dependency evidence = %#v", got.DependencyEvidence)
	}
}

func TestWorkspaceSymbolsRespectNodePackageEntrypoints(t *testing.T) {
	providerRecord := WorkspaceProjectRecord{
		Path: "frontend/packages/users", Kind: "frontend", BuildSystem: "node", Indexed: true,
	}
	provider := workspaceIndexProject{
		record: providerRecord,
		symbols: []RichSymbolRecord{
			{
				ID: "root", Name: "RootService", ExportName: "RootService",
				Kind: "class", Language: "typescript", File: "src/index.ts",
				QualifiedName: "src/index#RootService", Module: "src/index",
				WorkspacePackage: "@weka/users", Analyzer: "typescript-source",
				Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
			{
				ID: "user", Name: "UserService", ExportName: "UserService",
				Kind: "class", Language: "typescript", File: "src/user.ts",
				QualifiedName: "src/user#UserService", Module: "src/user",
				WorkspacePackage: "@weka/users", Analyzer: "typescript-source",
				Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
			{
				ID: "internal", Name: "InternalService", ExportName: "InternalService",
				Kind: "class", Language: "typescript", File: "src/internal.ts",
				QualifiedName: "src/internal#InternalService", Module: "src/internal",
				WorkspacePackage: "@weka/users", Analyzer: "typescript-source",
				Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
		},
	}
	if err := json.Unmarshal([]byte(`{
		"nodes": [{
			"name": "@weka/users",
			"path": "package.json",
			"kind": "package",
			"types": "./src/index.ts",
			"exports": {
				".": ["./src/index.ts"],
				"./user": ["./src/user.ts"]
			}
		}],
		"edges": []
	}`), &provider.packages); err != nil {
		t.Fatal(err)
	}

	consumerRecord := WorkspaceProjectRecord{
		Path: "frontend/apps/admin", Kind: "frontend", BuildSystem: "node", Indexed: true,
	}
	consumerSymbol := RichSymbolRecord{
		ID: "app", Name: "App", ExportName: "App", Kind: "function", Language: "typescript",
		File: "src/App.ts", QualifiedName: "src/App#App", Module: "src/App",
		WorkspacePackage: "@weka/admin", Analyzer: "typescript-source",
		Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}
	consumer := workspaceIndexProject{
		record:  consumerRecord,
		symbols: []RichSymbolRecord{consumerSymbol},
		relations: []RichRelationRecord{
			{
				ID: "root-exact", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "typescript-source", Line: 3,
				TargetModule: "@weka/users", TargetExport: "RootService",
				Resolution: SymbolResolutionUnresolved,
			},
			{
				ID: "subpath-exact", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "typescript-source", Line: 4,
				TargetModule: "@weka/users/user", TargetExport: "UserService",
				Resolution: SymbolResolutionUnresolved,
			},
			{
				ID: "subpath-not-root", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "typescript-source", Line: 5,
				TargetModule: "@weka/users", TargetExport: "UserService",
				Resolution: SymbolResolutionUnresolved,
			},
			{
				ID: "internal-not-root", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "typescript-source", Line: 6,
				TargetModule: "@weka/users", TargetExport: "InternalService",
				Resolution: SymbolResolutionUnresolved,
			},
		},
		packages: PackageGraphRecord{Edges: []PackageEdgeRecord{{
			From: "@weka/admin", To: "@weka/users", Type: "depends_on",
		}}},
	}
	registry := workspaceSymbolRegistry(provider.record, consumer.record)

	symbols, usages, err := BuildWorkspaceSymbolProjection(
		registry,
		[]workspaceIndexProject{consumer, provider},
		registry.Generated,
	)
	if err != nil {
		t.Fatal(err)
	}
	symbolIDByName := map[string]string{}
	for _, symbol := range symbols.Symbols {
		symbolIDByName[symbol.Name] = symbol.ID
	}
	usageByLine := map[int]CanonicalSymbolUsageRecord{}
	for _, usage := range usages.Usages {
		usageByLine[usage.SourceLine] = usage
	}
	for line, wantProvider := range map[int]string{
		3: symbolIDByName["RootService"],
		4: symbolIDByName["UserService"],
	} {
		if usageByLine[line].Resolution != SymbolResolutionExact ||
			usageByLine[line].ProviderSymbolID != wantProvider {
			t.Fatalf("line %d usage = %#v, want exact provider %q", line, usageByLine[line], wantProvider)
		}
	}
	for _, line := range []int{5, 6} {
		if usageByLine[line].Resolution != SymbolResolutionUnresolved ||
			usageByLine[line].ProviderSymbolID != "" {
			t.Fatalf("line %d usage = %#v, want unresolved unexposed module", line, usageByLine[line])
		}
	}
}

func TestPackageGraphPreservesNodeEntrypoints(t *testing.T) {
	graph := buildPackageGraph(WorkspaceIndex{NodePackages: []NodePackageRecord{{
		Path:  "packages/users/package.json",
		Name:  "@weka/users",
		Types: "./src/index.ts",
		Exports: map[string][]string{
			".":      {"./src/index.ts"},
			"./user": {"./src/user.d.ts", "./src/user.ts"},
		},
	}}})
	body, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"types":"./src/index.ts"`,
		`"exports":{".":["./src/index.ts"],"./user":["./src/user.d.ts","./src/user.ts"]}`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("package graph = %s, missing %s", body, want)
		}
	}
}

func TestWorkspaceSymbolsSelectStaticImportCondition(t *testing.T) {
	providerPackage, ok := extractNodePackage("package.json", `{
		"name": "@weka/users",
		"exports": {
			".": {
				"import": "./src/esm.ts",
				"require": "./src/cjs.ts",
				"default": "./src/default.ts"
			}
		}
	}`)
	if !ok {
		t.Fatal("conditional package fixture was not extracted")
	}
	providerRecord := WorkspaceProjectRecord{
		Path: "frontend/packages/users", Kind: "frontend", BuildSystem: "node", Indexed: true,
	}
	provider := workspaceIndexProject{
		record: providerRecord,
		symbols: []RichSymbolRecord{
			{
				ID: "esm", Name: "EsmOnly", ExportName: "EsmOnly",
				Kind: "class", Language: "typescript", File: "src/esm.ts",
				QualifiedName: "src/esm#EsmOnly", Module: "src/esm",
				WorkspacePackage: "@weka/users", Analyzer: "typescript-source",
				Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
			{
				ID: "cjs", Name: "CjsOnly", ExportName: "CjsOnly",
				Kind: "class", Language: "typescript", File: "src/cjs.ts",
				QualifiedName: "src/cjs#CjsOnly", Module: "src/cjs",
				WorkspacePackage: "@weka/users", Analyzer: "typescript-source",
				Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
			{
				ID: "default", Name: "DefaultOnly", ExportName: "DefaultOnly",
				Kind: "class", Language: "typescript", File: "src/default.ts",
				QualifiedName: "src/default#DefaultOnly", Module: "src/default",
				WorkspacePackage: "@weka/users", Analyzer: "typescript-source",
				Confidence: ConfidenceExact, Coverage: CoverageComplete,
			},
		},
		packages: jsonRoundTrip(
			t,
			buildPackageGraph(WorkspaceIndex{NodePackages: []NodePackageRecord{providerPackage}}),
		),
	}
	consumerRecord := WorkspaceProjectRecord{
		Path: "frontend/apps/admin", Kind: "frontend", BuildSystem: "node", Indexed: true,
	}
	consumerSymbol := RichSymbolRecord{
		ID: "app", Name: "App", ExportName: "App", Kind: "function", Language: "typescript",
		File: "src/App.ts", QualifiedName: "src/App#App", Module: "src/App",
		WorkspacePackage: "@weka/admin", Analyzer: "typescript-source",
		Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}
	consumer := workspaceIndexProject{
		record:  consumerRecord,
		symbols: []RichSymbolRecord{consumerSymbol},
		relations: []RichRelationRecord{
			{
				ID: "esm-import", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "typescript-source", Line: 3,
				TargetModule: "@weka/users", TargetExport: "EsmOnly",
				Resolution: SymbolResolutionUnresolved,
			},
			{
				ID: "cjs-import", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "typescript-source", Line: 4,
				TargetModule: "@weka/users", TargetExport: "CjsOnly",
				Resolution: SymbolResolutionUnresolved,
			},
			{
				ID: "condition-unknown", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "calls_export", Language: "typescript", Analyzer: "typescript-source", Line: 5,
				TargetModule: "@weka/users", TargetExport: "CjsOnly",
				Resolution: SymbolResolutionUnresolved,
			},
			{
				ID: "default-must-not-fill-import-gap", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "typescript-source", Line: 6,
				TargetModule: "@weka/users", TargetExport: "DefaultOnly",
				Resolution: SymbolResolutionUnresolved,
			},
		},
		packages: PackageGraphRecord{Edges: []PackageEdgeRecord{{
			From: "@weka/admin", To: "@weka/users", Type: "depends_on",
		}}},
	}
	registry := workspaceSymbolRegistry(provider.record, consumer.record)

	symbols, usages, err := BuildWorkspaceSymbolProjection(
		registry,
		[]workspaceIndexProject{consumer, provider},
		registry.Generated,
	)
	if err != nil {
		t.Fatal(err)
	}
	symbolIDByName := map[string]string{}
	for _, symbol := range symbols.Symbols {
		symbolIDByName[symbol.Name] = symbol.ID
	}
	usageByLine := map[int]CanonicalSymbolUsageRecord{}
	for _, usage := range usages.Usages {
		usageByLine[usage.SourceLine] = usage
	}
	if usageByLine[3].Resolution != SymbolResolutionExact ||
		usageByLine[3].ProviderSymbolID != symbolIDByName["EsmOnly"] {
		t.Fatalf("ESM workspace import = %#v", usageByLine[3])
	}
	for _, line := range []int{4, 5, 6} {
		if usageByLine[line].Resolution == SymbolResolutionExact ||
			usageByLine[line].ProviderSymbolID != "" {
			t.Fatalf("line %d selected an unproven conditional export: %#v", line, usageByLine[line])
		}
	}
}

func TestWorkspaceSymbolsUseOnlyEvidencedScriptExportsAndPreserveAliases(t *testing.T) {
	files := []FileRecord{
		{Path: "src/index.ts", Language: "typescript"},
		{Path: "src/core.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
class PrivateService {}
class LocalService {}
export { LocalService as Service };
export { CoreService as ReexportedService } from "./core";
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `
export class CoreService {}
`))
	facts = ResolveScriptSymbolFacts(files, nil, nil, facts)
	facts = FinalizeProjectSymbolFacts(files, WorkspaceIndex{NodePackages: []NodePackageRecord{{
		Path: "package.json", Name: "@weka/users",
	}}}, facts)

	providerRecord := WorkspaceProjectRecord{
		Path: "frontend/packages/users", Kind: "frontend", BuildSystem: "node", Indexed: true,
	}
	provider := workspaceIndexProject{
		record:    providerRecord,
		symbols:   jsonRoundTrip[[]RichSymbolRecord](t, facts.Declarations),
		relations: jsonRoundTrip[[]RichRelationRecord](t, facts.References),
		packages: PackageGraphRecord{Nodes: []PackageNodeRecord{{
			Name: "@weka/users", Path: "package.json", Kind: "package",
			Exports: map[string][]string{".": {"./src/index.ts"}},
		}}},
	}
	consumerRecord := WorkspaceProjectRecord{
		Path: "frontend/apps/admin", Kind: "frontend", BuildSystem: "node", Indexed: true,
	}
	consumerSymbol := RichSymbolRecord{
		ID: "app", Name: "App", ExportName: "App", Kind: "function", Language: "typescript",
		File: "src/App.ts", QualifiedName: "src/App#App", Module: "src/App",
		WorkspacePackage: "@weka/admin", Analyzer: "script-source",
		Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}
	consumer := workspaceIndexProject{
		record:  consumerRecord,
		symbols: []RichSymbolRecord{consumerSymbol},
		relations: []RichRelationRecord{
			{
				ID: "local-alias", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "script-source", Line: 3,
				TargetModule: "@weka/users", TargetExport: "Service",
				Resolution: SymbolResolutionUnresolved,
			},
			{
				ID: "reexport-alias", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "script-source", Line: 4,
				TargetModule: "@weka/users", TargetExport: "ReexportedService",
				Resolution: SymbolResolutionUnresolved,
			},
			{
				ID: "private", From: "src/App.ts", FromSymbolID: consumerSymbol.ID,
				Type: "imports_value", Language: "typescript", Analyzer: "script-source", Line: 5,
				TargetModule: "@weka/users", TargetExport: "PrivateService",
				Resolution: SymbolResolutionUnresolved,
			},
		},
		packages: PackageGraphRecord{Edges: []PackageEdgeRecord{{
			From: "@weka/admin", To: "@weka/users", Type: "depends_on",
		}}},
	}
	registry := workspaceSymbolRegistry(provider.record, consumer.record)

	symbols, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{consumer, provider}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	originalByName := map[string]string{}
	for _, symbol := range symbols.Symbols {
		originalByName[symbol.Name] = symbol.ID
	}
	usageByExport := map[string]CanonicalSymbolUsageRecord{}
	for _, usage := range usages.Usages {
		usageByExport[usage.TargetExport] = usage
	}
	if usageByExport["Service"].Resolution != SymbolResolutionExact ||
		usageByExport["Service"].ProviderSymbolID != originalByName["LocalService"] {
		t.Fatalf("local alias usage = %#v, symbols = %#v", usageByExport["Service"], symbols.Symbols)
	}
	if usageByExport["ReexportedService"].Resolution != SymbolResolutionExact ||
		usageByExport["ReexportedService"].ProviderSymbolID != originalByName["CoreService"] {
		t.Fatalf("reexport alias usage = %#v, symbols = %#v", usageByExport["ReexportedService"], symbols.Symbols)
	}
	if usageByExport["PrivateService"].Resolution != SymbolResolutionUnresolved ||
		usageByExport["PrivateService"].ProviderSymbolID != "" {
		t.Fatalf("private declaration became an implicit export: %#v", usageByExport["PrivateService"])
	}
}

func TestWorkspaceSymbolsRetainUnresolvedJavaAndJavaScriptTargets(t *testing.T) {
	javaConsumer := workspaceJavaConsumer(
		"microservices/ms-consumer",
		"com.weka:consumer",
		"com.external.MissingService",
	)
	scriptRecord := WorkspaceProjectRecord{
		Path: "frontend/apps/admin", Kind: "frontend", BuildSystem: "node", Indexed: true,
	}
	scriptConsumer := RichSymbolRecord{
		ID: "app", Name: "App", ExportName: "App", Kind: "function", Language: "javascript",
		File: "src/App.js", QualifiedName: "src/App#App", Module: "src/App",
		WorkspacePackage: "@weka/admin", Analyzer: "javascript-source",
		Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}
	scriptProject := workspaceIndexProject{
		record:  scriptRecord,
		symbols: []RichSymbolRecord{scriptConsumer},
		relations: []RichRelationRecord{{
			ID: "missing-import", From: "src/App.js", FromSymbolID: scriptConsumer.ID,
			Type: "imports_export", Language: "javascript", Analyzer: "javascript-source", Line: 4,
			TargetModule: "@weka/missing", TargetExport: "loadUser",
			Resolution: SymbolResolutionUnresolved,
		}},
	}
	registry := workspaceSymbolRegistry(javaConsumer.record, scriptProject.record)

	_, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{scriptProject, javaConsumer}, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages.Usages) != 2 {
		t.Fatalf("usages = %#v, want two unresolved usages", usages.Usages)
	}
	for _, usage := range usages.Usages {
		if usage.Category != SymbolUsageUnresolved ||
			usage.Resolution != SymbolResolutionUnresolved ||
			usage.ProviderSymbolID != "" {
			t.Fatalf("unresolved usage = %#v", usage)
		}
		switch usage.Language {
		case "java":
			if usage.TargetQualifiedName != "com.external.MissingService" {
				t.Fatalf("Java target was lost: %#v", usage)
			}
		case "javascript":
			if usage.TargetModule != "@weka/missing" || usage.TargetExport != "loadUser" {
				t.Fatalf("JavaScript target was lost: %#v", usage)
			}
		default:
			t.Fatalf("unexpected usage language: %#v", usage)
		}
	}
}

func TestWorkspaceSymbolProjectionIsDeterministicAcrossProjectOrder(t *testing.T) {
	javaProvider := workspaceJavaProject(
		"microservices/ms-users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "users", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", QualifiedName: "com.weka.UserService",
			Artifact: "com.weka:users", EvidenceIDs: []string{"evidence:z", "evidence:a"},
			Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
			Limitations: []string{"runtime_proxy", "reflection"},
		},
	)
	javaConsumer := workspaceJavaConsumer(
		"microservices/ms-consumer",
		"com.weka:consumer",
		"com.weka.UserService",
	)
	javaConsumer.maven = MavenGraphRecord{Edges: []MavenEdgeRecord{{
		From: "com.weka:consumer", To: "com.weka:users", Type: "depends_on",
	}}}
	scriptProvider := workspaceScriptProject(
		"frontend/packages/users",
		"@weka/users",
		RichSymbolRecord{
			ID: "load-user", Name: "loadUser", ExportName: "loadUser", Kind: "function",
			Language: "typescript", File: "src/index.ts", QualifiedName: "src/index#loadUser",
			Module: "src/index", WorkspacePackage: "@weka/users",
			Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	registry := workspaceSymbolRegistry(javaProvider.record, javaConsumer.record, scriptProvider.record)
	forward := []workspaceIndexProject{javaProvider, javaConsumer, scriptProvider}
	reverse := []workspaceIndexProject{scriptProvider, javaConsumer, javaProvider}

	forwardSymbols, forwardUsages, err := BuildWorkspaceSymbolProjection(registry, forward, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	reverseSymbols, reverseUsages, err := BuildWorkspaceSymbolProjection(registry, reverse, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	forwardSymbols.Generated = ""
	forwardUsages.Generated = ""
	reverseSymbols.Generated = ""
	reverseUsages.Generated = ""
	forwardJSON, err := json.Marshal(struct {
		Symbols WorkspaceSymbolIndexRecord
		Usages  WorkspaceSymbolUsageIndexRecord
	}{forwardSymbols, forwardUsages})
	if err != nil {
		t.Fatal(err)
	}
	reverseJSON, err := json.Marshal(struct {
		Symbols WorkspaceSymbolIndexRecord
		Usages  WorkspaceSymbolUsageIndexRecord
	}{reverseSymbols, reverseUsages})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(forwardJSON, reverseJSON) {
		t.Fatalf("reversed project order changed projection:\nforward %s\nreverse %s", forwardJSON, reverseJSON)
	}
}

func TestWorkspaceSymbolProjectionRejectsDuplicateCanonicalID(t *testing.T) {
	project := workspaceJavaProject(
		"microservices/ms-users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "first", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", QualifiedName: "com.weka.UserService",
			Artifact: "com.weka:users", Analyzer: "java-source",
			Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	duplicate := project.symbols[0]
	duplicate.ID = "second-local-id"
	project.symbols = append(project.symbols, duplicate)
	wantID := StableWorkspaceSymbolID(
		duplicate.Kind,
		project.record.Path,
		duplicate.Artifact,
		duplicate.Language,
		duplicate.QualifiedName,
		duplicate.File,
	)

	_, _, err := BuildWorkspaceSymbolProjection(
		workspaceSymbolRegistry(project.record),
		[]workspaceIndexProject{project},
		"generated",
	)
	if err == nil {
		t.Fatal("duplicate canonical declaration did not fail")
	}
	assertContains(t, err.Error(), wantID)
}

func TestWorkspaceSymbolProjectionRejectsUnknownLocalReferenceIDs(t *testing.T) {
	fqn := "com.weka.UserService"
	provider := workspaceJavaProject(
		"microservices/ms-users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "provider", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", QualifiedName: fqn, Artifact: "com.weka:users",
			Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	consumer := workspaceJavaConsumer("microservices/ms-consumer", "com.weka:consumer", fqn)
	consumer.maven = MavenGraphRecord{Edges: []MavenEdgeRecord{{
		From: "com.weka:consumer", To: "com.weka:users", Type: "depends_on",
	}}}
	registry := workspaceSymbolRegistry(provider.record, consumer.record)

	tests := []struct {
		name  string
		field string
		value string
		apply func(*RichRelationRecord)
	}{
		{
			name: "from symbol", field: "from_symbol_id", value: "missing-consumer",
			apply: func(reference *RichRelationRecord) {
				reference.FromSymbolID = "missing-consumer"
			},
		},
		{
			name: "to symbol", field: "to_symbol_id", value: "missing-provider",
			apply: func(reference *RichRelationRecord) {
				reference.ToSymbolID = "missing-provider"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			broken := consumer
			broken.relations = append([]RichRelationRecord(nil), consumer.relations...)
			test.apply(&broken.relations[0])

			_, _, err := BuildWorkspaceSymbolProjection(
				registry,
				[]workspaceIndexProject{provider, broken},
				registry.Generated,
			)
			if err == nil {
				t.Fatalf("unknown %s did not fail projection", test.field)
			}
			for _, want := range []string{
				consumer.record.Path,
				consumer.relations[0].ID,
				test.field,
				test.value,
			} {
				assertContains(t, err.Error(), want)
			}
		})
	}
}

func TestWorkspaceSymbolProjectionRejectsProjectPathContainingEvidenceSeparator(t *testing.T) {
	project := workspaceJavaProject(
		"microservices/ms#users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "users", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", QualifiedName: "com.weka.UserService",
			Artifact: "com.weka:users", Analyzer: "java-source",
			Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)

	_, _, err := BuildWorkspaceSymbolProjection(
		workspaceSymbolRegistry(project.record),
		[]workspaceIndexProject{project},
		"generated",
	)

	if err == nil {
		t.Fatal("project path containing # did not fail projection")
	}
	for _, want := range []string{project.record.Path, "#", "evidence"} {
		assertContains(t, err.Error(), want)
	}
}

func TestReconcileWorkspaceRejectsDanglingEvidenceBeforeReplacingProjection(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	project := workspaceProjectOnDisk(workspace, "microservices/ms-users")
	project.Name = "ms-users"
	writeFile(t, project.AbsPath, "pom.xml", "<project></project>")
	writeWorkspaceProjectFacts(t, project, []RichSymbolRecord{{
		ID: "users", Name: "UserService", Kind: "class", Language: "java",
		File: "src/UserService.java", QualifiedName: "com.weka.UserService",
		Artifact: "com.weka:users", Analyzer: "java-source",
		Confidence: ConfidenceExact, Coverage: CoverageComplete,
		EvidenceIDs: []string{"evidence:missing"},
	}}, nil)

	workspaceOut := filepath.Join(workspace, ".goregraph-workspace")
	oldSymbols := WorkspaceSymbolIndexRecord{
		SchemaVersion: SchemaVersion,
		Generated:     "old",
		Root:          filepath.ToSlash(workspace),
		Symbols:       []CanonicalSymbolRecord{},
		Coverage:      []SymbolCoverageRecord{},
	}
	oldUsages := WorkspaceSymbolUsageIndexRecord{
		SchemaVersion: SchemaVersion,
		Generated:     "old",
		Root:          filepath.ToSlash(workspace),
		Usages:        []CanonicalSymbolUsageRecord{},
		Coverage:      []SymbolCoverageRecord{},
	}
	if err := writeWorkspaceSymbolProjectionPair(workspaceOut, oldSymbols, oldUsages); err != nil {
		t.Fatal(err)
	}
	beforeSymbols, err := os.ReadFile(filepath.Join(workspaceOut, "symbol-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	beforeUsages, err := os.ReadFile(filepath.Join(workspaceOut, "symbol-usages.json"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Defaults()
	_, err = ReconcileWorkspace(project.AbsPath, cfg)

	if err == nil {
		t.Fatal("reconciliation published a dangling canonical evidence ID")
	}
	for _, want := range []string{project.Path + "#evidence:missing", "unknown evidence"} {
		assertContains(t, err.Error(), want)
	}
	afterSymbols, readErr := os.ReadFile(filepath.Join(workspaceOut, "symbol-index.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	afterUsages, readErr := os.ReadFile(filepath.Join(workspaceOut, "symbol-usages.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !reflect.DeepEqual(afterSymbols, beforeSymbols) || !reflect.DeepEqual(afterUsages, beforeUsages) {
		t.Fatal("invalid reconciliation replaced the live symbol projection")
	}
}

func TestReconcileWorkspaceRejectsEvidenceOwnedByAnotherProject(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	project := workspaceProjectOnDisk(workspace, "microservices/ms-users")
	project.Name = "ms-users"
	writeFile(t, project.AbsPath, "pom.xml", "<project></project>")
	symbol := RichSymbolRecord{
		ID: "users", Name: "UserService", Kind: "class", Language: "java",
		File: "src/UserService.java", Line: 4, QualifiedName: "com.weka.UserService",
		Artifact: "com.weka:users", Analyzer: "java-source",
		Confidence: ConfidenceExact, Coverage: CoverageComplete,
		EvidenceIDs: []string{"evidence:users"},
	}
	writeWorkspaceProjectFacts(t, project, []RichSymbolRecord{symbol}, nil)
	if err := writeJSON(
		filepath.Join(project.AbsPath, project.OutputDir, "evidence.json"),
		[]EvidenceRecord{{
			ID: "evidence:users", Project: "ms-orders", File: symbol.File,
			Start: EvidenceLocation{Line: symbol.Line}, Analyzer: "java-source",
			Method: "declaration", Reason: "java class declaration",
		}},
	); err != nil {
		t.Fatal(err)
	}

	_, err := ReconcileWorkspace(project.AbsPath, config.Defaults())

	if err == nil {
		t.Fatal("reconciliation accepted evidence owned by another project")
	}
	for _, want := range []string{"evidence:users", "ms-orders", "ms-users"} {
		assertContains(t, err.Error(), want)
	}
}

func TestReconcileWorkspaceRejectsUnknownLocalReferenceID(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	project := filepath.Join(workspace, "microservices", "ms-users")
	writeFile(t, project, "pom.xml", `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.weka</groupId>
  <artifactId>users</artifactId>
  <version>1</version>
</project>`)
	writeFile(t, project, "src/main/java/com/weka/UserService.java", `package com.weka;
class UserService {}
`)
	writeFile(t, project, "src/main/java/com/weka/Consumer.java", `package com.weka;
class Consumer {
  UserService service;
}
`)
	cfg := config.Defaults()
	if _, err := Run(project, cfg); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(project, cfg.OutputDir)
	var relations []RichRelationRecord
	readJSON(t, filepath.Join(out, "relations-full.json"), &relations)
	if len(relations) == 0 {
		t.Fatal("fixture did not produce rich relations")
	}
	relations[0].FromSymbolID = "missing-consumer"
	if err := writeJSON(filepath.Join(out, "relations-full.json"), relations); err != nil {
		t.Fatal(err)
	}

	_, err := ReconcileWorkspace(project, cfg)
	if err == nil {
		t.Fatal("reconciliation accepted unknown local reference ID")
	}
	for _, want := range []string{
		"microservices/ms-users",
		relations[0].ID,
		"from_symbol_id",
		"missing-consumer",
	} {
		assertContains(t, err.Error(), want)
	}
}

func TestWorkspaceSymbolProjectionReportsCoverageAndMissingProjects(t *testing.T) {
	java := workspaceJavaProject(
		"microservices/ms-users",
		"com.weka:users",
		RichSymbolRecord{
			ID: "users", Name: "UserService", Kind: "class", Language: "java",
			File: "src/UserService.java", QualifiedName: "com.weka.UserService",
			Artifact: "com.weka:users", Analyzer: "java-source",
			Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	script := workspaceScriptProject(
		"frontend/packages/users",
		"@weka/users",
		RichSymbolRecord{
			ID: "load-user", Name: "loadUser", ExportName: "loadUser", Kind: "function",
			Language: "typescript", File: "src/index.ts", QualifiedName: "src/index#loadUser",
			Module: "src/index", WorkspacePackage: "@weka/users",
			Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		},
	)
	missing := WorkspaceProjectRecord{
		Path: "microservices/ms-missing", Kind: "backend", Indexed: false, Status: "not_indexed",
	}
	registry := workspaceSymbolRegistry(java.record, script.record, missing)

	symbols, usages, err := BuildWorkspaceSymbolProjection(
		registry,
		[]workspaceIndexProject{script, java},
		registry.Generated,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertSymbolCoverage(t, symbols.Coverage, java.record.Path, "java", "declarations", CoverageComplete, nil)
	assertSymbolCoverage(
		t,
		usages.Coverage,
		java.record.Path,
		"java",
		"direct_usages",
		CoveragePartial,
		[]string{
			"dependency_injection",
			"generated_code",
			"reflection",
			"runtime_loading",
			"runtime_proxy",
			"unindexed_dependency_artifact",
		},
	)
	assertSymbolCoverage(
		t,
		usages.Coverage,
		script.record.Path,
		"typescript",
		"direct_usages",
		CoveragePartial,
		[]string{
			"bundler_only_alias",
			"computed_property",
			"dynamic_import",
			"generated_code",
			"unindexed_workspace_package",
		},
	)
	assertSymbolCoverage(t, symbols.Coverage, missing.Path, "unknown", "declarations", CoverageUnavailable, nil)
}

func TestLoadWorkspaceIndexesKeepsValidProjectsWhenSymbolFactsAreMalformed(t *testing.T) {
	root := t.TempDir()
	valid := workspaceProjectOnDisk(root, "microservices/ms-valid")
	broken := workspaceProjectOnDisk(root, "microservices/ms-broken")
	validSymbol := RichSymbolRecord{
		ID: "valid", Name: "Valid", Kind: "class", Language: "java",
		File: "src/Valid.java", QualifiedName: "com.weka.Valid", Artifact: "com.weka:valid",
		Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}
	writeWorkspaceProjectFacts(t, valid, []RichSymbolRecord{validSymbol}, []RichRelationRecord{})
	writeWorkspaceProjectFacts(t, broken, []RichSymbolRecord{}, []RichRelationRecord{})
	if err := os.WriteFile(
		filepath.Join(broken.AbsPath, broken.OutputDir, "symbols-full.json"),
		[]byte("{"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadWorkspaceIndexes([]WorkspaceProjectRecord{broken, valid})
	if err != nil {
		t.Fatalf("malformed project symbol facts aborted reconciliation: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded projects = %#v", loaded)
	}
	registry := workspaceSymbolRegistry(broken, valid)
	symbols, usages, err := BuildWorkspaceSymbolProjection(registry, loaded, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateWorkspaceSymbolProjectionEvidence(symbols, usages, loaded); err != nil {
		t.Fatalf("unreadable project without canonical evidence references failed integrity validation: %v", err)
	}
	if len(symbols.Symbols) != 1 || symbols.Symbols[0].QualifiedName != validSymbol.QualifiedName {
		t.Fatalf("valid project facts were discarded: %#v", symbols.Symbols)
	}
	assertSymbolCoverage(t, symbols.Coverage, broken.Path, "unknown", "declarations", CoverageFailed, nil)
}

func TestLoadWorkspaceIndexesReportsMissingOptionalSymbolFactsAsPartialCoverage(t *testing.T) {
	root := t.TempDir()
	project := workspaceProjectOnDisk(root, "microservices/ms-old-index")
	writeFile(t, project.AbsPath, filepath.Join(project.OutputDir, "manifest.json"), `{"tool":"goregraph","schema":2}`)

	loaded, err := loadWorkspaceIndexes([]WorkspaceProjectRecord{project})
	if err != nil {
		t.Fatal(err)
	}
	registry := workspaceSymbolRegistry(project)
	symbols, usages, err := BuildWorkspaceSymbolProjection(registry, loaded, registry.Generated)
	if err != nil {
		t.Fatal(err)
	}
	declarations := findSymbolCoverage(symbols.Coverage, project.Path, "unknown", "declarations")
	if declarations.Coverage != CoveragePartial ||
		!strings.Contains(declarations.Reason, "symbols-full.json") {
		t.Fatalf("missing declaration facts coverage = %#v", declarations)
	}
	direct := findSymbolCoverage(usages.Coverage, project.Path, "unknown", "direct_usages")
	if direct.Coverage != CoveragePartial ||
		!strings.Contains(direct.Reason, "relations-full.json") ||
		!strings.Contains(direct.Reason, "callgraph.json") {
		t.Fatalf("missing usage facts coverage = %#v", direct)
	}
}

func TestWorkspaceSymbolCoverageScopesDependencyAndCallGraphReadIssues(t *testing.T) {
	tests := []struct {
		name          string
		malformedFile string
		missingFile   string
		wantUsage     Coverage
	}{
		{
			name:          "malformed Maven graph",
			malformedFile: "maven-graph.json",
			wantUsage:     CoverageFailed,
		},
		{
			name:          "malformed call graph",
			malformedFile: "callgraph.json",
			wantUsage:     CoverageFailed,
		},
		{
			name:        "missing Maven graph",
			missingFile: "maven-graph.json",
			wantUsage:   CoveragePartial,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			project := workspaceProjectOnDisk(root, "microservices/ms-users")
			symbol := RichSymbolRecord{
				ID: "users", Name: "UserService", Kind: "class", Language: "java",
				File: "src/UserService.java", QualifiedName: "com.weka.UserService",
				Artifact: "com.weka:users", Analyzer: "java-source",
				Confidence: ConfidenceExact, Coverage: CoverageComplete,
			}
			writeWorkspaceProjectFacts(t, project, []RichSymbolRecord{symbol}, []RichRelationRecord{})
			if test.malformedFile != "" {
				if err := os.WriteFile(
					filepath.Join(project.AbsPath, project.OutputDir, test.malformedFile),
					[]byte("{"),
					0o644,
				); err != nil {
					t.Fatal(err)
				}
			}
			if test.missingFile != "" {
				if err := os.Remove(filepath.Join(project.AbsPath, project.OutputDir, test.missingFile)); err != nil {
					t.Fatal(err)
				}
			}

			loaded, err := loadWorkspaceIndexes([]WorkspaceProjectRecord{project})
			if err != nil {
				t.Fatal(err)
			}
			registry := workspaceSymbolRegistry(project)
			symbols, usages, err := BuildWorkspaceSymbolProjection(registry, loaded, registry.Generated)
			if err != nil {
				t.Fatal(err)
			}
			declarations := findSymbolCoverage(symbols.Coverage, project.Path, "java", "declarations")
			if declarations.Coverage != CoverageComplete {
				t.Fatalf("declaration coverage = %#v, want COMPLETE", declarations)
			}
			direct := findSymbolCoverage(usages.Coverage, project.Path, "java", "direct_usages")
			if direct.Coverage != test.wantUsage {
				t.Fatalf("direct usage coverage = %#v, want %s", direct, test.wantUsage)
			}
			affected := test.malformedFile
			if affected == "" {
				affected = test.missingFile
			}
			assertContains(t, direct.Reason, affected)
			if strings.Contains(declarations.Reason, affected) {
				t.Fatalf("declaration coverage was degraded by %s: %#v", affected, declarations)
			}
		})
	}
}

func TestWorkspaceDeclarationCoverageAggregatesCapabilityAndSymbolCoverage(t *testing.T) {
	tests := []struct {
		name               string
		capabilityCoverage Coverage
		symbolCoverage     Coverage
		want               Coverage
	}{
		{
			name:               "partial symbol",
			capabilityCoverage: CoverageComplete,
			symbolCoverage:     CoveragePartial,
			want:               CoveragePartial,
		},
		{
			name:               "partial capability",
			capabilityCoverage: CoveragePartial,
			symbolCoverage:     CoverageComplete,
			want:               CoveragePartial,
		},
		{
			name:               "failed capability",
			capabilityCoverage: CoverageFailed,
			symbolCoverage:     CoverageComplete,
			want:               CoverageFailed,
		},
		{
			name:               "failed symbol",
			capabilityCoverage: CoverageComplete,
			symbolCoverage:     CoverageFailed,
			want:               CoverageFailed,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := workspaceJavaProject(
				"microservices/ms-users",
				"com.weka:users",
				RichSymbolRecord{
					ID: "users", Name: "UserService", Kind: "class", Language: "java",
					File: "src/UserService.java", QualifiedName: "com.weka.UserService",
					Artifact: "com.weka:users", Analyzer: "java-source",
					Confidence: ConfidenceExact, Coverage: test.symbolCoverage,
				},
			)
			project.capabilities = []CapabilityRecord{{
				ID: CapabilitySymbols, Language: "java", Coverage: test.capabilityCoverage,
				Reason: "fixture symbol capability", StatusReason: "fixture symbol capability",
			}}
			registry := workspaceSymbolRegistry(project.record)
			symbols, _, err := BuildWorkspaceSymbolProjection(
				registry,
				[]workspaceIndexProject{project},
				registry.Generated,
			)
			if err != nil {
				t.Fatal(err)
			}
			declarations := findSymbolCoverage(symbols.Coverage, project.record.Path, "java", "declarations")
			if declarations.Coverage != test.want {
				t.Fatalf("declaration coverage = %#v, want %s", declarations, test.want)
			}
		})
	}
}

func TestWorkspaceDeclarationCoverageOverlaysCapabilitiesInputStatus(t *testing.T) {
	tests := []struct {
		name      string
		malformed bool
		want      Coverage
	}{
		{name: "missing", want: CoveragePartial},
		{name: "malformed", malformed: true, want: CoverageFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			project := workspaceProjectOnDisk(root, "microservices/ms-users")
			symbol := RichSymbolRecord{
				ID: "users", Name: "UserService", Kind: "class", Language: "java",
				File: "src/UserService.java", QualifiedName: "com.weka.UserService",
				Artifact: "com.weka:users", Analyzer: "java-source",
				Confidence: ConfidenceExact, Coverage: CoverageComplete,
			}
			writeWorkspaceProjectFacts(t, project, []RichSymbolRecord{symbol}, nil)
			capabilitiesPath := filepath.Join(project.AbsPath, project.OutputDir, "capabilities.json")
			if test.malformed {
				if err := os.WriteFile(capabilitiesPath, []byte("{"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Remove(capabilitiesPath); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}

			loaded, err := loadWorkspaceIndexes([]WorkspaceProjectRecord{project})
			if err != nil {
				t.Fatal(err)
			}
			registry := workspaceSymbolRegistry(project)
			symbols, _, err := BuildWorkspaceSymbolProjection(registry, loaded, registry.Generated)
			if err != nil {
				t.Fatal(err)
			}
			declarations := findSymbolCoverage(symbols.Coverage, project.Path, "java", "declarations")
			if declarations.Coverage != test.want ||
				!strings.Contains(declarations.Reason, "capabilities.json") {
				t.Fatalf("declaration coverage = %#v, want %s capabilities overlay", declarations, test.want)
			}
		})
	}
}

func TestLoadWorkspaceIndexesRecordsHTTPReachabilityFactFailures(t *testing.T) {
	root := t.TempDir()
	frontend := workspaceProjectOnDisk(root, "frontend/app")
	frontend.Kind = "frontend"
	backend := workspaceProjectOnDisk(root, "microservices/ms-user")
	writeWorkspaceProjectFacts(t, frontend, []RichSymbolRecord{{
		ID: "page", Name: "UserPage", Kind: "function", Language: "typescript",
		File: "src/UserPage.tsx", QualifiedName: "UserPage", ExportName: "UserPage",
		Analyzer: "typescript-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}}, nil)
	writeWorkspaceProjectFacts(t, backend, []RichSymbolRecord{{
		ID: "service", Name: "UserService", Kind: "class", Language: "java",
		File: "src/UserService.java", QualifiedName: "com.example.UserService",
		Analyzer: "java-source", Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}}, nil)
	if err := os.WriteFile(filepath.Join(frontend.AbsPath, frontend.OutputDir, "flows.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backend.AbsPath, backend.OutputDir, "endpoint-flows.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadWorkspaceIndexes([]WorkspaceProjectRecord{frontend, backend})

	if err != nil {
		t.Fatalf("loadWorkspaceIndexes returned error instead of structured coverage failure: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded projects = %#v", loaded)
	}
	byPath := map[string]workspaceIndexProject{}
	for _, project := range loaded {
		byPath[project.record.Path] = project
	}
	if len(workspaceSymbolFactFailures(byPath[frontend.Path].loadFailures, []string{"flows.json"})) != 1 {
		t.Fatalf("frontend flow failures = %#v", byPath[frontend.Path].loadFailures)
	}
	if len(workspaceSymbolFactFailures(byPath[backend.Path].loadFailures, []string{"endpoint-flows.json"})) != 1 {
		t.Fatalf("backend endpoint-flow failures = %#v", byPath[backend.Path].loadFailures)
	}
}

func TestWorkspaceSymbolProjectionPairRejectsInvalidUsage(t *testing.T) {
	out := t.TempDir()
	symbols := WorkspaceSymbolIndexRecord{
		SchemaVersion: SchemaVersion,
		Symbols:       []CanonicalSymbolRecord{},
		Coverage:      []SymbolCoverageRecord{},
	}
	usages := WorkspaceSymbolUsageIndexRecord{
		SchemaVersion: SchemaVersion,
		Usages: []CanonicalSymbolUsageRecord{{
			ID:              "usage:invalid",
			ConsumerProject: "frontend/app",
			Category:        SymbolUsageDirectReference,
			Language:        "typescript",
			RelationKind:    "imports_export",
			SourceFile:      "src/App.ts",
			Resolution:      SymbolResolution("INVALID"),
		}},
		Coverage: []SymbolCoverageRecord{},
	}

	err := writeWorkspaceSymbolProjectionPair(out, symbols, usages)
	if err == nil {
		t.Fatal("invalid workspace symbol usage was written")
	}
	assertContains(t, err.Error(), "INVALID")
	if workspaceFileExists(filepath.Join(out, "symbol-index.json")) ||
		workspaceFileExists(filepath.Join(out, "symbol-usages.json")) {
		t.Fatal("invalid pair left root symbol files behind")
	}
}

func TestReconcileWorkspaceRestoresSymbolPairWhenSecondRenameFails(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	project := filepath.Join(workspace, "microservices", "ms-users")
	writeFile(t, project, "pom.xml", `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.weka</groupId>
  <artifactId>users</artifactId>
  <version>1</version>
</project>`)
	writeFile(t, project, "src/main/java/com/weka/UserService.java", `package com.weka;
class UserService {}
`)
	cfg := config.Defaults()
	if _, err := Run(project, cfg); err != nil {
		t.Fatalf("initial Run returned error: %v", err)
	}
	out := filepath.Join(workspace, ".goregraph-workspace")
	oldSymbols, err := os.ReadFile(filepath.Join(out, "symbol-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	oldUsages, err := os.ReadFile(filepath.Join(out, "symbol-usages.json"))
	if err != nil {
		t.Fatal(err)
	}

	originalRename := workspaceSymbolRename
	workspaceSymbolRename = func(oldPath, newPath string) error {
		if filepath.Base(newPath) == "symbol-usages.json" &&
			strings.Contains(filepath.Base(filepath.Dir(oldPath)), ".symbol-projection-") {
			return errors.New("injected second rename failure")
		}
		return originalRename(oldPath, newPath)
	}
	t.Cleanup(func() {
		workspaceSymbolRename = originalRename
	})

	if _, err := ReconcileWorkspace(project, cfg); err == nil {
		t.Fatal("ReconcileWorkspace succeeded despite injected root write failure")
	}
	gotSymbols, err := os.ReadFile(filepath.Join(out, "symbol-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	gotUsages, err := os.ReadFile(filepath.Join(out, "symbol-usages.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotSymbols, oldSymbols) || !reflect.DeepEqual(gotUsages, oldUsages) {
		t.Fatal("failed paired write left a mixed old/new symbol projection")
	}
}

func TestWorkspaceSymbolFilesAreRootOutputsAndExplicitCleanItems(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	project := filepath.Join(workspace, "microservices", "ms-users")
	writeFile(t, project, "pom.xml", `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.weka</groupId>
  <artifactId>users</artifactId>
  <version>1</version>
</project>`)
	writeFile(t, project, "src/main/java/com/weka/UserService.java", `package com.weka;
class UserService {}
`)
	cfg := config.Defaults()
	if _, err := Run(project, cfg); err != nil {
		t.Fatal(err)
	}
	workspaceOut := filepath.Join(workspace, ".goregraph-workspace")
	for _, name := range []string{"symbol-index.json", "symbol-usages.json"} {
		if !workspaceFileExists(filepath.Join(workspaceOut, name)) {
			t.Fatalf("missing workspace root output %s", name)
		}
	}
	var manifest Manifest
	readJSON(t, filepath.Join(project, cfg.OutputDir, "manifest.json"), &manifest)
	for _, generated := range manifest.Generated {
		if generated == "symbol-index.json" || generated == "symbol-usages.json" {
			t.Fatalf("root workspace output leaked into per-project GeneratedFiles: %#v", manifest.Generated)
		}
	}
	plan, err := WorkspaceCleanPlan(project, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"symbol-index.json", "symbol-usages.json"} {
		want := filepath.ToSlash(filepath.Join(workspaceOut, name))
		found := false
		for _, item := range plan.Items {
			if item.Reason != "workspace overlay output" {
				continue
			}
			for _, included := range item.Includes {
				if included == want {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("clean plan missing explicit workspace symbol file %s: %#v", want, plan.Items)
		}
	}
}

func workspaceJavaProject(path, artifact string, symbol RichSymbolRecord) workspaceIndexProject {
	return workspaceIndexProject{
		record: WorkspaceProjectRecord{
			Path: path, Kind: "backend", BuildSystem: "maven", Indexed: true,
		},
		symbols: []RichSymbolRecord{symbol},
		maven: MavenGraphRecord{Nodes: []MavenNodeRecord{{
			ID: artifact, Kind: "module",
		}}},
	}
}

func workspaceJavaConsumer(path, artifact, target string) workspaceIndexProject {
	symbol := RichSymbolRecord{
		ID: "consumer", Name: "Consumer", Kind: "class", Language: "java",
		File: "src/main/java/com/weka/Consumer.java", QualifiedName: "com.weka.Consumer",
		Artifact: artifact, Analyzer: "java-source",
		Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}
	return workspaceIndexProject{
		record:  WorkspaceProjectRecord{Path: path, Kind: "backend", BuildSystem: "maven", Indexed: true},
		symbols: []RichSymbolRecord{symbol},
		relations: []RichRelationRecord{{
			ID: "reference", From: symbol.File, FromSymbolID: symbol.ID,
			Type: "field_type", Language: "java", Analyzer: "java-source", Line: 8,
			TargetQualifiedName: target, Resolution: SymbolResolutionUnresolved,
		}},
		maven: MavenGraphRecord{Nodes: []MavenNodeRecord{{
			ID: artifact, Kind: "module",
		}}},
	}
}

func workspaceScriptProject(path, packageName string, symbol RichSymbolRecord) workspaceIndexProject {
	return workspaceIndexProject{
		record:  WorkspaceProjectRecord{Path: path, Kind: "frontend", BuildSystem: "node", Indexed: true},
		symbols: []RichSymbolRecord{symbol},
		packages: PackageGraphRecord{Nodes: []PackageNodeRecord{{
			Name: packageName, Path: "package.json", Kind: "package", Types: "src/index.ts",
		}}},
	}
}

func workspaceSymbolRegistry(projects ...WorkspaceProjectRecord) WorkspaceRegistryRecord {
	return WorkspaceRegistryRecord{
		Root:      "/workspace",
		Generated: "2026-07-16T10:00:00Z",
		Projects:  projects,
	}
}

func workspaceProjectOnDisk(root, path string) WorkspaceProjectRecord {
	return WorkspaceProjectRecord{
		Name:      filepath.Base(path),
		Path:      path,
		AbsPath:   filepath.Join(root, filepath.FromSlash(path)),
		Kind:      "backend",
		Indexed:   true,
		Status:    "indexed",
		OutputDir: "goregraph-out",
	}
}

func writeWorkspaceProjectFacts(t *testing.T, project WorkspaceProjectRecord, symbols []RichSymbolRecord, relations []RichRelationRecord) {
	t.Helper()
	out := filepath.Join(project.AbsPath, project.OutputDir)
	writeFile(t, project.AbsPath, filepath.Join(project.OutputDir, "manifest.json"), `{"tool":"goregraph","schema":2}`)
	languages := map[string]bool{}
	for _, symbol := range symbols {
		if symbol.Language != "" {
			languages[symbol.Language] = true
		}
	}
	capabilities := make([]CapabilityRecord, 0, len(languages))
	for language := range languages {
		capabilities = append(capabilities, CapabilityRecord{
			ID: CapabilitySymbols, Language: language, Coverage: CoverageComplete,
			Reason: "fixture symbols capability", StatusReason: "fixture symbols capability",
		})
	}
	values := map[string]any{
		"symbols-full.json":   symbols,
		"relations-full.json": relations,
		"callgraph.json":      CallGraphRecord{},
		"maven-graph.json":    MavenGraphRecord{},
		"package-graph.json":  PackageGraphRecord{},
		"evidence.json":       []EvidenceRecord{},
		"capabilities.json":   capabilities,
	}
	for name, value := range values {
		if err := writeJSON(filepath.Join(out, name), value); err != nil {
			t.Fatal(err)
		}
	}
}

func assertSymbolCoverage(t *testing.T, records []SymbolCoverageRecord, project, language, capability string, want Coverage, limitations []string) {
	t.Helper()
	for _, record := range records {
		if record.Project != project || record.Language != language || record.Capability != capability {
			continue
		}
		if record.Coverage != want || !reflect.DeepEqual(record.Limitations, limitations) {
			t.Fatalf("coverage = %#v, want %s with %#v", record, want, limitations)
		}
		return
	}
	t.Fatalf("missing coverage for %s/%s/%s in %#v", project, language, capability, records)
}

func findSymbolCoverage(records []SymbolCoverageRecord, project, language, capability string) SymbolCoverageRecord {
	for _, record := range records {
		if record.Project == project && record.Language == language && record.Capability == capability {
			return record
		}
	}
	return SymbolCoverageRecord{}
}

func jsonRoundTrip[T any](t *testing.T, value T) T {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertContains(t *testing.T, value, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("%q does not contain %q", value, want)
	}
}
