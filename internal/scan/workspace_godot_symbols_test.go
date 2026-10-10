package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestReconcileWorkspaceWithGodotAndGDScriptBindings(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "game")
	writeFile(t, project, "project.godot", "config_version=5\n[application]\nrun/main_scene=\"res://main.tscn\"\n")
	writeFile(t, project, "main.gd", "extends Node\nfunc ready():\n tick()\nfunc tick():\n pass\n")
	writeFile(t, project, "main.tscn", `[gd_scene load_steps=2 format=3]
[ext_resource type="Script" path="res://main.gd" id="1"]
[node name="Main" type="Node"]
script = ExtResource("1")
[node name="Button" type="Button" parent="."]
[connection signal="pressed" from="Button" to="." method="tick"]
`)
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
	byID := map[string]CanonicalSymbolRecord{}
	for _, symbol := range symbols.Symbols {
		byID[symbol.ID] = symbol
	}
	found := map[string]bool{}
	for _, usage := range usages.Usages {
		if usage.Resolution != SymbolResolutionExact {
			continue
		}
		owner, ownerExists := byID[usage.ConsumerSymbolID]
		target, targetExists := byID[usage.ProviderSymbolID]
		if !ownerExists || !targetExists {
			t.Fatalf("dangling workspace binding: %+v", usage)
		}
		found[owner.Language+"->"+target.Language] = true
	}
	for _, binding := range []string{"godot->gdscript", "gdscript->gdscript"} {
		if !found[binding] {
			t.Fatalf("workspace lost %s binding: %+v", binding, usages)
		}
	}
	if err := validateWorkspaceSymbolProjectionPair(symbols, usages); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, name := range []string{"symbol-index.json", "symbol-usages.json"} {
		body, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = body
	}
	var references []RichRelationRecord
	relations := filepath.Join(project, cfg.OutputDir, "index", "relations-full.json")
	readJSON(t, relations, &references)
	corrupted := false
	for i := range references {
		if references[i].Language == "godot" && references[i].FromSymbolID != "" {
			references[i].FromSymbolID = "missing-godot-owner"
			corrupted = true
			break
		}
	}
	if !corrupted {
		t.Fatal("fixture did not emit a Godot owner reference")
	}
	if err := writeJSON(relations, references); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileWorkspace(project, cfg); err == nil {
		t.Fatal("workspace accepted a dangling Godot owner identity")
	}
	for name, body := range before {
		after, err := os.ReadFile(filepath.Join(out, name))
		if err != nil || string(after) != string(body) {
			t.Fatalf("failed reconciliation replaced valid %s: %v", name, err)
		}
	}
}

func TestGodotWorkspaceUsageCoverageNeedsOnlyApplicableFacts(t *testing.T) {
	for _, language := range []string{"gdscript", "godot"} {
		project := workspaceIndexProject{missingFacts: []string{"maven-graph.json", "package-graph.json"}}
		if workspaceSymbolCapabilityHasIssues(project, language, "direct_usages") {
			t.Fatalf("%s usage coverage required unrelated build-system facts", language)
		}
		project.missingFacts = append(project.missingFacts, "relations-full.json")
		if !workspaceSymbolCapabilityHasIssues(project, language, "direct_usages") {
			t.Fatalf("%s usage coverage ignored missing reference facts", language)
		}
	}
}
