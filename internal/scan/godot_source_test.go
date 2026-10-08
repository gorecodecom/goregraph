package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

func godotFixture(bodies map[string]string) supplementaryAnalysis {
	var names []string
	for name := range bodies {
		names = append(names, name)
	}
	sort.Strings(names)
	var sources []supplementarySource
	var files []FileRecord
	for _, name := range names {
		file := FileRecord{Path: name, Language: detectLanguage(name)}
		files = append(files, file)
		if supplementaryLanguage(file.Language) {
			sources = append(sources, supplementarySource{file: file, body: bodies[name]})
		}
	}
	return analyzeSupplementarySources(sources, files)
}

func godotDeclaration(t *testing.T, result supplementaryAnalysis, file, kind, name string) RichSymbolRecord {
	t.Helper()
	for _, symbol := range result.facts.Declarations {
		if symbol.File == file && symbol.Kind == kind && symbol.Name == name {
			return symbol
		}
	}
	t.Fatalf("missing %s %s in %s: %#v", kind, name, file, result.facts.Declarations)
	return RichSymbolRecord{}
}

func godotExactRef(result supplementaryAnalysis, kind, fromFile, toID string) bool {
	for _, ref := range result.facts.References {
		if ref.Type == kind && ref.From == fromFile && ref.ToSymbolID == toID && ref.Resolution == SymbolResolutionExact {
			return true
		}
	}
	return false
}

func TestGodotDetectionAndDiscovery(t *testing.T) {
	for file, language := range map[string]string{"scripts/main.gd": "gdscript", "scenes/main.tscn": "godot", "assets/material.tres": "godot", "project.godot": "godot", "export_presets.cfg": "godot", "notes.cfg": "text"} {
		if got := detectLanguage(file); got != language {
			t.Errorf("%s: %s, want %s", file, got, language)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "project.godot"), []byte("config_version=5\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasProjectMarker(root) {
		t.Fatal("Godot root was not discovered")
	}
	for name, body := range map[string]string{"main.gd": "extends Node\nfunc ready():\n pass\n", ".godot/cache.gd": "class_name Generated\n", "project.godot": "[application]\nrun/main_scene=\"res://main.tscn\"\n", "main.tscn": "[gd_scene format=3]\n[node name=\"Root\" type=\"Node\"]\n"} {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	index, _, err := scanProject(root, config.Defaults(), gitignore.Matcher{})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range index.Files {
		if strings.HasPrefix(file.Path, ".godot/") {
			t.Fatalf("engine cache indexed: %s", file.Path)
		}
	}
	if len(index.SymbolFacts.Declarations) == 0 {
		t.Fatal("adapter facts were not merged into project output")
	}
}

func TestGDScriptDeclarationsAndLexicalBoundaries(t *testing.T) {
	result := godotFixture(map[string]string{"scripts/model.gd": `# func imaginary():
extends RefCounted
class_name Model
@export var speed: float = 2.0
const NAME = "func counterfeit():"
signal changed(value: int)
enum State { IDLE, RUNNING }
var note = """
func invented():
 pass
"""
static func create(
 count: int,
 options: Dictionary = {"names": []}
) -> Dictionary:
 return options
func step(delta: float) -> void:
 var local = 1
 changed.emit(delta)
class Nested:
 func not_top_level():
  pass
`})
	for _, pair := range [][2]string{{"class", "Model"}, {"field", "speed"}, {"constant", "NAME"}, {"signal", "changed"}, {"enum", "State"}, {"method", "create"}, {"method", "step"}} {
		godotDeclaration(t, result, "scripts/model.gd", pair[0], pair[1])
	}
	for _, symbol := range result.facts.Declarations {
		if symbol.Name == "imaginary" || symbol.Name == "counterfeit" || symbol.Name == "invented" || symbol.Name == "local" || symbol.Name == "not_top_level" {
			t.Fatalf("non-declaration promoted: %#v", symbol)
		}
	}
	for _, function := range result.code.Functions {
		if function.Name == "create" && (function.Line != 12 || function.EndLine != 16) {
			t.Fatalf("multiline span: %#v", function)
		}
	}
	signal := godotDeclaration(t, result, "scripts/model.gd", "signal", "changed")
	if !godotExactRef(result, "emits_signal", "scripts/model.gd", signal.ID) {
		t.Fatal("modern signal emission is missing")
	}
}

func TestGDScriptExplicitResourcesCallsAndTestHarness(t *testing.T) {
	result := godotFixture(map[string]string{
		"scripts/tool.gd": `extends RefCounted
class_name Tool
static func make():
 return 1
func tick():
 return 2
`,
		"tests/tool_test.gd": `extends SceneTree
const ToolScript = preload("res://scripts/tool.gd")
func _initialize():
 _run()
func _run():
 var actor := ToolScript.new()
 actor.tick()
 ToolScript.make()
 emit_signal("finished")
 signal_source.connect(_run)
 signal_source.connect(Callable(self, "_run"))
signal finished
`,
		"scripts/derived.gd": "extends \"res://scripts/tool.gd\"\n",
	})
	tick := godotDeclaration(t, result, "scripts/tool.gd", "method", "tick")
	makeSymbol := godotDeclaration(t, result, "scripts/tool.gd", "method", "make")
	tool := godotDeclaration(t, result, "scripts/tool.gd", "class", "Tool")
	for _, pair := range []struct{ kind, file, id string }{{"loads_resource", "tests/tool_test.gd", tool.ID}, {"instantiates", "tests/tool_test.gd", tool.ID}, {"calls_method_owner", "tests/tool_test.gd", tick.ID}, {"calls_method_owner", "tests/tool_test.gd", makeSymbol.ID}, {"extends", "scripts/derived.gd", tool.ID}} {
		if !godotExactRef(result, pair.kind, pair.file, pair.id) {
			t.Errorf("missing exact %s -> %s", pair.kind, pair.id)
		}
	}
	if len(result.tests) == 0 {
		t.Fatal("SceneTree test harness has no source target map")
	}
	callbacks := 0
	for _, ref := range result.facts.References {
		if ref.Type == "connects_callback" && ref.Resolution == SymbolResolutionExact {
			callbacks++
		}
	}
	if callbacks != 2 {
		t.Fatalf("callback refs = %d, want 2", callbacks)
	}
	found := false
	for _, mapping := range result.tests {
		if mapping.TargetFile == "scripts/tool.gd" && mapping.TargetMethod == "tick" {
			found = true
		}
	}
	if !found {
		t.Fatal("constructed instance call is missing from test map")
	}
}

func TestGDScriptDoesNotGuessDynamicTargets(t *testing.T) {
	result := godotFixture(map[string]string{
		"a.gd": "class_name Shared\nstatic func tick():\n pass\n",
		"b.gd": "class_name Shared\nstatic func tick():\n pass\n",
		"main.gd": `extends Shared
const Model = preload("res://a.gd")
func local():
 pass
func shadow(local, Model):
 local()
 Model.tick()
func dynamic():
 var actor = Model.new()
 actor = other
 actor.tick()
 load("res://" + dynamic_path)
 load("res://../outside.gd")
 load("uid://unknown")
 other.Model.tick()
 $Model.tick()
func closure():
 var callback = func():
  local()
 local()
func comparison():
 var actor := Model.new()
 if actor == other:
  pass
 actor.tick()
func shadow_loader(load):
 load("res://a.gd")
`})
	for _, edge := range result.graph.Edges {
		if edge.From.Method == "shadow" || edge.From.Method == "dynamic" || edge.From.Method == "closure" && edge.Line == 19 {
			t.Fatalf("dynamic or shadowed target promoted: %#v", edge)
		}
	}
	ambiguous := false
	resourceRefs := 0
	for _, ref := range result.facts.References {
		if ref.Type == "extends" {
			ambiguous = ref.Resolution == SymbolResolutionAmbiguous && len(ref.CandidateSymbolIDs) == 2
		}
		if ref.Type == "loads_resource" {
			resourceRefs++
			if ref.Line > 2 && ref.Internal {
				t.Fatalf("computed/escaping/shadowed load promoted: %#v", ref)
			}
		}
	}
	if !ambiguous {
		t.Fatal("duplicate global class names must remain ambiguous")
	}
	if resourceRefs != 4 {
		t.Fatalf("resource refs %d, want literal preload plus three unresolved loads", resourceRefs)
	}
}

func TestGodotSceneResourceAndSignalBindings(t *testing.T) {
	result := godotFixture(map[string]string{
		"scripts/main.gd":  "extends Control\nfunc _clicked():\n pass\n",
		"assets/image.png": "image",
		"scenes/main.tscn": `[gd_scene load_steps=4 format=3]
[ext_resource type="Script" path="res://scripts/main.gd" id="script"]
[ext_resource type="Texture2D" path="res://assets/image.png" id="image"]
[sub_resource type="StyleBoxFlat" id="style"]
bg_color = Color(1, 0, 0)
[node name="Root" type="Control"]
script = ExtResource("script")
[node name="Button" type="Button" parent="."]
icon = ExtResource("image")
theme_override_styles/normal = SubResource("style")
[connection signal="pressed" from="Button" to="." method="_clicked"]
[connection signal="pressed" from="Missing" to="." method="_clicked"]
`,
		"assets/theme.tres": "[gd_resource type=\"Theme\" format=3]\n[sub_resource type=\"StyleBoxFlat\" id=\"box\"]\n[resource]\nstyle = SubResource(\"box\")\n",
	})
	script := godotDeclaration(t, result, "scripts/main.gd", "class", "main")
	method := godotDeclaration(t, result, "scripts/main.gd", "method", "_clicked")
	style := godotDeclaration(t, result, "scenes/main.tscn", "resource", "style")
	godotDeclaration(t, result, "scenes/main.tscn", "node", "Button")
	for _, pair := range []struct{ kind, id string }{{"attaches_script", script.ID}, {"uses_resource", style.ID}, {"connects_signal", method.ID}} {
		if !godotExactRef(result, pair.kind, "scenes/main.tscn", pair.id) {
			t.Errorf("missing %s", pair.kind)
		}
	}
	unresolved := 0
	for _, ref := range result.facts.References {
		if ref.Type == "connects_signal" && ref.Resolution != SymbolResolutionExact {
			unresolved++
		}
	}
	if unresolved != 1 {
		t.Fatalf("missing source node resolved a signal: unresolved=%d", unresolved)
	}
	if len(result.graph.Edges) != 0 {
		t.Fatal("serialized callbacks are not executed calls")
	}
}

func TestGodotProjectSettingsAndMissingResources(t *testing.T) {
	result := godotFixture(map[string]string{
		"project.godot": `config_version=5
[application]
run/main_scene="res://scenes/main.tscn"
[autoload]
State="*res://scripts/state.gd"
Other="res://missing.gd"
`,
		"scripts/state.gd": "extends Node\n",
		"scenes/main.tscn": "[gd_scene format=3]\n[node name=\"Root\" type=\"Node\"]\n",
	})
	godotDeclaration(t, result, "project.godot", "autoload", "State")
	godotDeclaration(t, result, "project.godot", "autoload", "Other")
	state := godotDeclaration(t, result, "scripts/state.gd", "class", "state")
	if !godotExactRef(result, "autoloads", "project.godot", state.ID) {
		t.Fatal("autoload script not linked")
	}
	main := false
	for _, ref := range result.facts.References {
		if ref.Type == "starts_scene" && ref.Internal && ref.To == "scenes/main.tscn" {
			main = true
		}
		if ref.Type == "autoloads" && ref.To == "res://missing.gd" && ref.Internal {
			t.Fatal("missing autoload was promoted")
		}
	}
	if !main {
		t.Fatal("main scene not linked")
	}
	for _, uri := range []string{"res://../outside.gd", "res:///outside.gd", "../outside.gd", "user://save.tres", "uid://42", "res://a\\b.gd"} {
		if godotResourcePath("main.gd", uri) != "" {
			t.Errorf("unsafe/unsupported URI accepted: %s", uri)
		}
	}
}

func TestGodotDuplicateResourcesAndNodesStayUnresolved(t *testing.T) {
	result := godotFixture(map[string]string{
		"main.gd": "func clicked():\n pass\n",
		"main.tscn": `[gd_scene format=3]
[ext_resource type="Script" path="res://main.gd" id="script"]
[ext_resource type="Script" path="res://missing.gd" id="script"]
[node name="Root" type="Node"]
script = ExtResource("script")
[node name="Button" type="Button" parent="."]
[node name="Button" type="Button" parent="."]
[connection signal="pressed" from="Button" to="." method="clicked"]
`,
	})
	for _, ref := range result.facts.References {
		if (ref.Type == "attaches_script" || ref.Type == "connects_signal") && ref.Resolution == SymbolResolutionExact {
			t.Fatalf("duplicate declaration resolved: %#v", ref)
		}
	}
}

func TestGDScriptConstructedFieldsAndClosureScopes(t *testing.T) {
	result := godotFixture(map[string]string{
		"scripts/simulation.gd": "extends RefCounted\nfunc advance():\n pass\n",
		"main.gd": `extends Control
const Simulation = preload("res://scripts/simulation.gd")
var simulation := Simulation.new()
signal finished
func _ready():
 simulation.advance()
 var callback = func():
  simulation.advance()
 simulation.advance()
func shadowed(simulation):
 simulation.advance()
func shadow_signal():
 var finished = other
 finished.emit()
func defaults(label: String = "(", marker: String = ":") -> void:
 simulation.advance()
`,
	})
	advance := godotDeclaration(t, result, "scripts/simulation.gd", "method", "advance")
	calls := 0
	for _, edge := range result.graph.Edges {
		if edge.ToSymbolID != advance.ID {
			continue
		}
		calls++
		if edge.Line == 8 || edge.From.Method == "shadowed" {
			t.Fatalf("closure/shadowed field promoted: %#v", edge)
		}
	}
	if calls != 3 {
		t.Fatalf("field calls outside closures = %d, want 3", calls)
	}
	for _, ref := range result.facts.References {
		if ref.Type == "emits_signal" && ref.Resolution == SymbolResolutionExact {
			t.Fatalf("shadowed signal promoted: %#v", ref)
		}
	}
	godotDeclaration(t, result, "main.gd", "method", "defaults")
}

func TestGDScriptReassignedFieldIsNotAProvenReceiver(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func step():\n pass\n",
		"main.gd":  "const Model = preload(\"res://model.gd\")\nvar model := Model.new()\nfunc change():\n model = other\nfunc run():\n model.step()\n",
	})
	for _, edge := range result.graph.Edges {
		if edge.To.Method == "step" {
			t.Fatalf("mutable field target promoted: %#v", edge)
		}
	}
}

func TestGodotFinalizationPreservesUnresolvedFacts(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func step():\n pass\n",
		"main.gd":  "func run():\n unknown.step()\n load(\"res://missing.gd\")\n",
	})
	facts := FinalizeProjectSymbolFacts([]FileRecord{{Path: "model.gd", Language: "gdscript"}, {Path: "main.gd", Language: "gdscript"}}, WorkspaceIndex{}, result.facts)
	for _, ref := range facts.References {
		if ref.Resolution == SymbolResolutionExact {
			t.Fatalf("generic finalizer guessed a Godot target: %#v", ref)
		}
	}
}

func TestGDScriptSelfAssignmentAndLoopShadowingStayUnresolved(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func step():\n pass\n",
		"main.gd": `const Model = preload("res://model.gd")
var model := Model.new()
func reset():
 self.model = other
func run():
 model.step()
func loop():
 var actor := Model.new()
 for actor in actors:
  actor.step()
`,
	})
	for _, edge := range result.graph.Edges {
		if edge.To.Method == "step" {
			t.Fatalf("reassigned/shadowed receiver promoted: %#v", edge)
		}
	}
}

func TestGDScriptConflictingPreloadAliasesStayUnresolved(t *testing.T) {
	result := godotFixture(map[string]string{
		"a.gd":    "static func tick():\n pass\n",
		"b.gd":    "static func tick():\n pass\n",
		"main.gd": "const Model = preload(\"res://a.gd\")\nconst Model = preload(\"res://b.gd\")\nfunc run():\n Model.tick()\n",
	})
	if len(result.graph.Edges) != 0 {
		t.Fatalf("conflicting preload aliases resolved: %#v", result.graph.Edges)
	}
}

func TestGodotMixedAdaptersPreserveRepeatedNativeCalls(t *testing.T) {
	result := godotFixture(map[string]string{
		"service.cpp": "void ping() {}\nvoid run() { ping(); ping(); }\n",
		"main.gd":     "func tick():\n pass\nfunc run():\n tick()\n",
	})
	nativeCalls, scriptCalls := 0, 0
	for _, edge := range result.graph.Edges {
		if edge.SourceFile == "service.cpp" {
			nativeCalls++
		}
		if edge.SourceFile == "main.gd" {
			scriptCalls++
		}
	}
	if nativeCalls != 2 || scriptCalls != 1 {
		t.Fatalf("lost call occurrences: native=%d gdscript=%d", nativeCalls, scriptCalls)
	}
}

func TestGDScriptIncompleteHeadersAndNestedInheritanceStayUnbound(t *testing.T) {
	result := godotFixture(map[string]string{
		"base.gd":    "class_name Base\nclass Inner:\n pass\n",
		"derived.gd": "extends Base.Inner\nfunc incomplete()\nfunc real():\n pass\n",
	})
	godotDeclaration(t, result, "derived.gd", "method", "real")
	for _, symbol := range result.facts.Declarations {
		if symbol.Name == "incomplete" {
			t.Fatal("incomplete header consumed the next declaration")
		}
	}
	for _, ref := range result.facts.References {
		if ref.Type == "extends" && ref.Resolution == SymbolResolutionExact {
			t.Fatal("nested parent was bound to its outer class")
		}
	}
}
