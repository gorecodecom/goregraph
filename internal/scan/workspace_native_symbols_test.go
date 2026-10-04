package scan

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestWorkspaceSymbolEvidenceIndexPreservesLocationRules(t *testing.T) {
	project := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "game"}, evidence: []EvidenceRecord{
		{ID: "whole-file", File: "Scene.unity"},
		{ID: "unknown-line", File: "Scene.unity", Start: EvidenceLocation{Line: -1}},
		{ID: "line-five", File: "Scene.unity", Start: EvidenceLocation{Line: 5}},
		{ID: "line-six", File: "Scene.unity", Start: EvidenceLocation{Line: 6}},
		{ID: "other-file", File: "Other.unity", Start: EvidenceLocation{Line: 5}},
		{File: "Scene.unity", Start: EvidenceLocation{Line: 5}},
	}}
	indexed := project
	indexed.symbolEvidence = indexWorkspaceSymbolEvidence(project.evidence)
	for _, file := range []string{"Scene.unity", "Other.unity", "Missing.unity"} {
		for _, line := range []int{-1, 0, 5, 6, 7} {
			local := []string{"explicit", "line-five"}
			want := workspaceFactEvidenceIDs(project, local, file, line)
			got := workspaceFactEvidenceIDs(indexed, local, file, line)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s:%d: indexed=%v, linear=%v", file, line, got, want)
			}
		}
	}
}

func TestWorkspaceSymbolProjectionFromExistingIndexes(t *testing.T) {
	root := os.Getenv("GOREGRAPH_WORKSPACE_SYMBOL_SMOKE_ROOT")
	if root == "" {
		t.Skip("existing workspace indexes not explicitly supplied")
	}
	var previous WorkspaceRegistryRecord
	readJSON(t, filepath.Join(root, ".goregraph-workspace", "index", "registry.json"), &previous)
	paths := map[string]bool{}
	for _, project := range previous.Projects {
		paths[filepath.Join(root, filepath.FromSlash(project.Path))] = true
	}
	indexes, err := filepath.Glob(filepath.Join(root, "*", "goregraph-out", "index", "symbols-full.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		paths[filepath.Dir(filepath.Dir(filepath.Dir(index)))] = true
	}
	registry := WorkspaceRegistryRecord{Root: root}
	var projects []workspaceIndexProject
	for path := range paths {
		out := filepath.Join(path, "goregraph-out", "index")
		if _, err := os.Stat(filepath.Join(out, "symbols-full.json")); os.IsNotExist(err) {
			continue
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		project := workspaceIndexProject{record: WorkspaceProjectRecord{Name: filepath.Base(path), Path: filepath.ToSlash(relative), Indexed: true}}
		readJSON(t, filepath.Join(out, "symbols-full.json"), &project.symbols)
		readJSON(t, filepath.Join(out, "relations-full.json"), &project.relations)
		readJSON(t, filepath.Join(out, "callgraph.json"), &project.callGraph)
		readJSON(t, filepath.Join(out, "evidence.json"), &project.evidence)
		projects = append(projects, project)
		registry.Projects = append(registry.Projects, project.record)
	}
	started := time.Now()
	symbols, usages, err := BuildWorkspaceSymbolProjection(registry, projects, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateWorkspaceSymbolProjectionEvidence(symbols, usages, projects); err != nil {
		t.Fatal(err)
	}
	if err := validateWorkspaceSymbolProjectionPair(symbols, usages); err != nil {
		t.Fatal(err)
	}
	t.Logf("validated %d project indexes, %d symbols and %d usages in %s", len(projects), len(symbols.Symbols), len(usages.Usages), time.Since(started))
}

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

func TestReconcileWorkspaceWithUnityAndBlenderAssetBindings(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "game")
	writeFile(t, project, "ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 6000.6.4f1")
	writeFile(t, project, "Packages/manifest.json", "{}")
	writeFile(t, project, "Assets/Player.cs", "namespace Game; public class Player { public void Tick(){} }")
	writeFile(t, project, "Assets/Player.cs.meta", "guid: 11111111111111111111111111111111\n")
	writeFile(t, project, "Assets/Player.prefab", `%YAML 1.1
%TAG !u! tag:unity3d.com,2011:
--- !u!1 &100
GameObject:
  m_Name: Knight
  m_Component:
  - component: {fileID: 200}
--- !u!114 &200
MonoBehaviour:
  m_GameObject: {fileID: 100}
  m_Script: {fileID: 11500000, guid: 11111111111111111111111111111111, type: 3}
`)
	blend := "BLENDER-v\x00fixture"
	writeFile(t, project, "Art/Knight.blend", blend)
	report := map[string]any{
		"schema_version": 1, "engine": "blender", "producer_version": "5.2.2", "source": "Art/Knight.blend", "source_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(blend))),
		"objects": []map[string]any{
			{"id": "object:Knight", "name": "Knight", "kind": "mesh", "references": []map[string]any{{"property": "material", "target": "material:Armor"}}},
			{"id": "material:Armor", "name": "Armor", "kind": "material"},
		},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, project, "Knight.goregraph-blender.json", string(body))
	cfg := config.Defaults()
	cfg.Workspace, cfg.WorkspaceRoot = true, workspace
	if _, err := Run(project, cfg); err != nil {
		t.Fatal(err)
	}
	var symbols WorkspaceSymbolIndexRecord
	var usages WorkspaceSymbolUsageIndexRecord
	out := filepath.Join(workspace, ".goregraph-workspace", "index")
	readJSON(t, filepath.Join(out, "symbol-index.json"), &symbols)
	readJSON(t, filepath.Join(out, "symbol-usages.json"), &usages)
	languages := map[string]string{}
	for _, symbol := range symbols.Symbols {
		languages[symbol.ID] = symbol.Language
	}
	found := map[string]bool{}
	for _, usage := range usages.Usages {
		if usage.Resolution != SymbolResolutionExact || usage.ConsumerSymbolID == "" {
			continue
		}
		found[usage.Language+"->"+languages[usage.ProviderSymbolID]] = true
	}
	for _, binding := range []string{"unity->unity", "unity->csharp", "blender->blender"} {
		if !found[binding] {
			t.Fatalf("workspace lost %s asset binding: %#v", binding, usages)
		}
	}
	if err := validateWorkspaceSymbolProjectionPair(symbols, usages); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(out, "symbol-usages.json"))
	if err != nil {
		t.Fatal(err)
	}
	var references []RichRelationRecord
	projectIndex := filepath.Join(project, cfg.OutputDir, "index")
	readJSON(t, filepath.Join(projectIndex, "relations-full.json"), &references)
	for index := range references {
		if references[index].Language == "unity" && references[index].FromSymbolID != "" {
			references[index].FromSymbolID = "missing-serialized-object"
			break
		}
	}
	if err := writeJSON(filepath.Join(projectIndex, "relations-full.json"), references); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileWorkspace(project, cfg); err == nil {
		t.Fatal("workspace accepted a dangling serialized-object identity")
	}
	after, err := os.ReadFile(filepath.Join(out, "symbol-usages.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("failed asset reconciliation replaced the valid workspace projection")
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
