package scan

import (
	"strings"
	"testing"
)

func TestGodotAnnotatedMethodsAndConstructedFieldsRemainTopLevel(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func tick():\n pass\n",
		"main.gd": `extends Node
const Model = preload("res://model.gd")
@onready var worker = Model.new()
@warning_ignore("unused_parameter") func run(argument):
 worker.tick()
 load("res://model.gd")
func invoke():
 run(null)
`,
	})
	run := godotDeclaration(t, result, "main.gd", "method", "run")
	godotDeclaration(t, result, "main.gd", "field", "worker")
	foundCall, foundResource, foundInvoke := false, false, false
	for _, edge := range result.graph.Edges {
		foundCall = foundCall || edge.Line == 5 && edge.To.File == "model.gd" && edge.To.Method == "tick"
		foundInvoke = foundInvoke || edge.From.Method == "invoke" && edge.ToSymbolID == run.ID
	}
	for _, ref := range result.facts.References {
		foundResource = foundResource || ref.Line == 6 && ref.Type == "loads_resource" && ref.Internal
	}
	if !foundCall || !foundResource || !foundInvoke {
		t.Fatalf("annotated declarations lost calls or loader scope: call=%v resource=%v invoke=%v", foundCall, foundResource, foundInvoke)
	}
}

func TestGodotImplicitLineJoiningDoesNotTruncateMethodBody(t *testing.T) {
	result := godotFixture(map[string]string{
		"main.gd": "func tick():\n pass\nfunc run():\n print(\n  1\n)\n tick()\n",
	})
	found := false
	for _, edge := range result.graph.Edges {
		found = found || edge.From.Method == "run" && edge.To.Method == "tick" && edge.Line == 7
	}
	if !found {
		t.Fatal("closing delimiter at column zero truncated the current method")
	}
	for _, function := range result.code.Functions {
		if function.Name == "run" && function.EndLine != 7 {
			t.Fatalf("incomplete method span: %#v", function)
		}
	}
}

func TestGodotNamedLambdaInArrayIsNotScriptMethod(t *testing.T) {
	result := godotFixture(map[string]string{
		"main.gd": "var callbacks = [\nfunc callback(): return 1\n]\nfunc run():\n callbacks[0].call()\n",
	})
	for _, symbol := range result.facts.Declarations {
		if symbol.Kind == "method" && symbol.Name == "callback" {
			t.Fatalf("anonymous function was promoted to a script method: %#v", symbol)
		}
	}
	for _, edge := range result.graph.Edges {
		if edge.To.Method == "callback" {
			t.Fatalf("named lambda produced an exact class-method target: %#v", edge)
		}
	}
}

func TestGodotMatchPatternBindingsDoNotEscapeTheirBranch(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func tick():\n pass\n",
		"main.gd": `const Model = preload("res://model.gd")
func run(value):
 var worker = Model.new()
 match value:
  [var worker]:
   worker.tick()
  _:
   worker.tick()
 worker.tick()
`,
	})
	lines := map[int]bool{}
	for _, edge := range result.graph.Edges {
		if edge.To.Method == "tick" && edge.To.File == "model.gd" {
			lines[edge.Line] = true
		}
	}
	if lines[6] || !lines[8] || !lines[9] || len(lines) != 2 {
		t.Fatalf("match capture leaked into another branch: %v", lines)
	}
}

func TestGodotNestedClassLoaderAndFieldsDoNotBecomeOuterBindings(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func tick():\n pass\n",
		"main.gd": `const Model = preload("res://model.gd")
var worker = Model.new()
class Inner:
 var worker = null
 var load = Callable(self, "custom_loader")
 var resource = load("res://model.gd")
 func custom_loader(_path):
  return null
func run():
 worker.tick()
 load("res://model.gd")
`,
	})
	loaderLines := map[int]bool{}
	for _, ref := range result.facts.References {
		if ref.Type == "loads_resource" && ref.Internal {
			loaderLines[ref.Line] = true
		}
	}
	if len(loaderLines) != 2 || !loaderLines[1] || !loaderLines[11] {
		t.Fatalf("nested custom loader became an outer builtin call: %v", loaderLines)
	}
	found := false
	for _, edge := range result.graph.Edges {
		found = found || edge.Line == 10 && edge.To.Method == "tick" && edge.To.File == "model.gd"
	}
	if !found {
		t.Fatal("nested-class field assignment invalidated unrelated outer field")
	}
}

func TestGodotAnnotatedGlobalClassInheritanceAndStaticReceiver(t *testing.T) {
	result := godotFixture(map[string]string{
		"base.gd":    "class_name Base\n",
		"icon.svg":   "<svg/>\n",
		"derived.gd": "@icon(\"res://icon.svg\") class_name Derived extends Base\n@warning_ignore(\"unused_parameter\") static func own(_unused = null):\n pass\n",
		"main.gd":    "const Child = preload(\"res://derived.gd\")\nfunc run():\n Child.own()\n",
	})
	base := godotDeclaration(t, result, "base.gd", "class", "Base")
	child := godotDeclaration(t, result, "derived.gd", "class", "Derived")
	method := godotDeclaration(t, result, "derived.gd", "method", "own")
	if !godotExactRef(result, "extends", "derived.gd", base.ID) || !godotExactRef(result, "loads_resource", "main.gd", child.ID) || !godotExactRef(result, "calls_method_owner", "main.gd", method.ID) {
		t.Fatal("annotated class, inheritance and static method no longer agree")
	}
}

func TestGodotNestedResourceArraysDoNotTruncateScriptOrConnections(t *testing.T) {
	result := godotFixture(map[string]string{
		"main.gd": "extends Node\nfunc done():\n pass\n",
		"main.tscn": `[gd_scene load_steps=2 format=3]
[ext_resource type="Script" path="res://main.gd" id="1"]
[node name="Root" type="Node"]
metadata/nested = [
[1, 2],
[3]
]
script = ExtResource("1")
[node name="Button" type="Button" parent="."]
[connection signal="pressed" from="Button" to="." method="done"]
`,
	})
	script := godotDeclaration(t, result, "main.gd", "class", "main")
	method := godotDeclaration(t, result, "main.gd", "method", "done")
	if !godotExactRef(result, "attaches_script", "main.tscn", script.ID) || !godotExactRef(result, "connects_signal", "main.tscn", method.ID) {
		t.Fatal("nested serialized arrays hid the attached script or callback")
	}
}

func TestGodotInstanceInGroupedNodeHeaderKeepsResourceUse(t *testing.T) {
	for _, groups := range []string{"", " groups=[\"group\"]", " groups=[\n\"group\"\n]"} {
		result := godotFixture(map[string]string{
			"child.tscn": "[gd_scene format=3]\n[node name=\"ChildRoot\" type=\"Node\"]\n",
			"main.tscn": `[gd_scene load_steps=2 format=3]
[ext_resource type="PackedScene" path="res://child.tscn" id="1"]
[node name="Root" type="Node"]
[node name="Child" parent="."` + groups + ` instance=ExtResource("1")]
`,
		})
		node := godotDeclaration(t, result, "main.tscn", "node", "Child")
		found := false
		for _, ref := range result.facts.References {
			found = found || ref.Type == "uses_resource" && ref.FromSymbolID == node.ID && ref.To == "child.tscn" && ref.Internal
		}
		if !found {
			t.Fatalf("PackedScene instance in node header lost its resource use: %q", groups)
		}
	}
}

func TestGodotUnterminatedHeadersRemainUnbound(t *testing.T) {
	tokens := godotTokens(strings.Repeat("[node name=\"unfinished\"\n", 4000), true)
	if ends := godotBracketEnds(tokens); len(ends) != 0 {
		t.Fatalf("unterminated headers acquired matches: %v", ends)
	}
	if sections := godotSections(tokens); len(sections) != 0 {
		t.Fatalf("unterminated headers produced sections: %d", len(sections))
	}
}

func TestGodotBracketPairingIgnoresLiteralDelimiters(t *testing.T) {
	tokens := godotTokens("[node name=\"[literal]\" groups=[\"a\"]]\n", true)
	ends := godotBracketEnds(tokens)
	end, ok := godotSectionHeaderEnd(tokens, 0, ends)
	if !ok || end != len(tokens)-1 || len(ends) != 2 {
		t.Fatalf("incorrect header bracket pairing: %v end=%d", ends, end)
	}
	sections := godotSections(tokens)
	if len(sections) != 1 || sections[0].attrs["name"].text != "[literal]" {
		t.Fatalf("literal brackets changed section: %+v", sections)
	}
}
