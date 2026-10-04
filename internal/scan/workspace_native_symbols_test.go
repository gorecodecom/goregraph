package scan

import (
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestWorkspaceNativeSymbolProjectionKeepsLocalBindings(t *testing.T) {
	for _, language := range []string{"csharp", "swift"} {
		t.Run(language, func(t *testing.T) {
			caller := RichSymbolRecord{ID: "caller", Name: "show", Kind: "method", Language: language, File: "Panel.source", Line: 2, QualifiedName: "Panel.show", Analyzer: language + "-source", Confidence: ConfidenceExact, Coverage: CoveragePartial}
			target := caller
			target.ID, target.Name, target.QualifiedName, target.Line = "target", "present", "Panel.present", 3
			project := workspaceIndexProject{
				record:  WorkspaceProjectRecord{Path: "apps/panel", Indexed: true},
				symbols: []RichSymbolRecord{caller, target},
				relations: []RichRelationRecord{
					{ID: "local-call", From: caller.File, To: target.File, Type: "calls_method_owner", Language: language, Line: 2, FromSymbolID: caller.ID, ToSymbolID: target.ID, TargetQualifiedName: target.QualifiedName, Resolution: SymbolResolutionExact},
					{ID: "external-call", From: caller.File, To: target.QualifiedName, Type: "calls_method_owner", Language: language, Line: 4, FromSymbolID: caller.ID, TargetQualifiedName: target.QualifiedName, Resolution: SymbolResolutionUnresolved, NonPromotable: true},
				},
				callGraph: CallGraphRecord{Edges: []CallGraphEdgeRecord{{ID: "local-edge", FromSymbolID: caller.ID, ToSymbolID: target.ID, TargetQualifiedName: target.QualifiedName, SourceFile: caller.File, Line: 2, Type: "calls", Resolution: SymbolResolutionExact}}},
			}
			unrelated := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "apps/unrelated", Indexed: true}, symbols: []RichSymbolRecord{target}}
			registry := workspaceSymbolRegistry(project.record, unrelated.record)
			symbols, usages, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{project, unrelated}, registry.Generated)
			if err != nil {
				t.Fatal(err)
			}
			if len(symbols.Symbols) != 3 || len(usages.Usages) != 3 {
				t.Fatalf("native declarations or references were omitted: %#v, %#v", symbols, usages)
			}
			for _, usage := range usages.Usages {
				if usage.Language != language || usage.ConsumerSymbolID == "" {
					t.Fatalf("native caller provenance was lost: %#v", usage)
				}
				if usage.SourceLine == 4 {
					if usage.Resolution != SymbolResolutionUnresolved || usage.ProviderSymbolID != "" {
						t.Fatalf("unverified same-name target was promoted: %#v", usage)
					}
					continue
				}
				if usage.Resolution != SymbolResolutionExact {
					t.Fatalf("local static binding was lost: %#v", usage)
				}
				for _, symbol := range symbols.Symbols {
					if symbol.ID == usage.ProviderSymbolID && symbol.Project != project.record.Path {
						t.Fatalf("local reference selected another repository: %#v", usage)
					}
				}
			}
			assertSymbolCoverage(t, symbols.Coverage, project.record.Path, language, "declarations", CoveragePartial, []string{})
			assertSymbolCoverage(t, usages.Coverage, project.record.Path, language, "direct_usages", CoveragePartial, nil)
			project.relations[0].FromSymbolID = "missing-native-caller"
			if _, _, err := BuildWorkspaceSymbolProjection(registry, []workspaceIndexProject{project, unrelated}, registry.Generated); err == nil {
				t.Fatal("workspace accepted a dangling native caller identity")
			}
		})
	}
}

func TestReconcileWorkspaceWithSwiftAndCSharpSourceBindings(t *testing.T) {
	workspace := t.TempDir()
	swift := filepath.Join(workspace, "swift-app")
	csharp := filepath.Join(workspace, "csharp-app")
	writeFile(t, swift, "Package.swift", `// swift-tools-version: 5.9
import PackageDescription
let package = Package(name: "Panel", targets: [.target(name: "Panel")])
`)
	writeFile(t, swift, "Sources/Panel/Panel.swift", `class Panel {
    func show() { present() }
    func present() {}
}
`)
	writeFile(t, csharp, "Panel.csproj", `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`)
	writeFile(t, csharp, "Panel.cs", `class Panel {
    void Show() { Present(); }
    void Present() {}
}
`)
	cfg := config.Defaults()
	cfg.Workspace, cfg.WorkspaceRoot = true, workspace
	for _, project := range []string{swift, csharp} {
		if _, err := Run(project, cfg); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReconcileWorkspace(swift, cfg); err != nil {
		t.Fatal(err)
	}
	var symbols WorkspaceSymbolIndexRecord
	var usages WorkspaceSymbolUsageIndexRecord
	out := filepath.Join(workspace, ".goregraph-workspace", "index")
	readJSON(t, filepath.Join(out, "symbol-index.json"), &symbols)
	readJSON(t, filepath.Join(out, "symbol-usages.json"), &usages)
	for _, language := range []string{"swift", "csharp"} {
		found := false
		for _, usage := range usages.Usages {
			if usage.Language == language && usage.Resolution == SymbolResolutionExact && usage.ConsumerSymbolID != "" && usage.ProviderSymbolID != "" {
				found = true
			}
		}
		if !found {
			t.Fatalf("workspace lost %s source binding: %#v", language, usages)
		}
	}
	if err := validateWorkspaceSymbolProjectionPair(symbols, usages); err != nil {
		t.Fatal(err)
	}
}
