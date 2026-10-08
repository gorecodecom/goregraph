package scan

import "testing"

func TestGodotConditionalInitializersRemainUnresolved(t *testing.T) {
	for _, declaration := range []string{
		"var worker = First.new() if use_first else Second.new()",
		"var worker = First.new().choose_second()",
	} {
		t.Run(declaration, func(t *testing.T) {
			result := godotFixture(map[string]string{
				"first.gd": "func tick():\n pass\nfunc choose_second():\n pass\n", "second.gd": "func tick():\n pass\n",
				"main.gd": "const First = preload(\"res://first.gd\")\nconst Second = preload(\"res://second.gd\")\nfunc run(use_first: bool):\n " + declaration + "\n worker.tick()\n",
			})
			for _, edge := range result.graph.Edges {
				if edge.To.Method == "tick" {
					t.Fatalf("compound initializer bound EXACT to %s", edge.To.File)
				}
			}
		})
	}
	result := godotFixture(map[string]string{
		"first.gd": "func tick():\n pass\n", "second.gd": "func tick():\n pass\n",
		"main.gd": "const First = preload(\"res://first.gd\")\nconst Second = preload(\"res://second.gd\")\nvar worker = First.new() if flag else Second.new()\nvar flag = false\nfunc run():\n worker.tick()\n",
	})
	for _, edge := range result.graph.Edges {
		if edge.To.Method == "tick" {
			t.Fatalf("conditional field bound EXACT to %s", edge.To.File)
		}
	}
}

func TestGodotConditionalPreloadAliasDoesNotSelectFirstBranch(t *testing.T) {
	for _, initializer := range []string{
		"preload(\"res://first.gd\") if false else preload(\"res://second.gd\")",
		"preload(\"res://first.gd\").Alternate",
	} {
		result := godotFixture(map[string]string{
			"first.gd": "static func tick():\n pass\nclass Alternate:\n static func tick():\n  pass\n", "second.gd": "static func tick():\n pass\n",
			"main.gd": "const Model = " + initializer + "\nfunc run():\n Model.tick()\n",
		})
		for _, edge := range result.graph.Edges {
			if edge.To.Method == "tick" {
				t.Fatalf("compound preload alias selected an EXACT target: %#v", edge)
			}
		}
	}
}

func TestGodotBlockLocalsDoNotEscapeAndRestoreFields(t *testing.T) {
	result := godotFixture(map[string]string{
		"outer.gd": "func tick():\n pass\n", "inner.gd": "func tick():\n pass\n",
		"main.gd": "const Outer = preload(\"res://outer.gd\")\nconst Inner = preload(\"res://inner.gd\")\nvar worker = Outer.new()\nfunc run(flag: bool):\n if flag:\n  var worker = Inner.new()\n  worker.tick()\n worker.tick()\n",
	})
	foundInside, foundOutside := false, false
	for _, edge := range result.graph.Edges {
		if edge.To.Method != "tick" {
			continue
		}
		if edge.Line == 7 {
			if edge.To.File != "inner.gd" {
				t.Fatalf("wrong block local: %#v", edge)
			}
			foundInside = true
		}
		if edge.Line == 8 {
			if edge.To.File != "outer.gd" {
				t.Fatalf("out-of-scope local escaped: %#v", edge)
			}
			foundOutside = true
		}
	}
	if !foundInside || !foundOutside {
		t.Fatalf("lost proven bindings: inside=%v outside=%v", foundInside, foundOutside)
	}
}

func TestGodotLoopAndSiblingScopesRestoreOuterLocals(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func tick():\n pass\n",
		"main.gd": `const Model = preload("res://model.gd")
func run(flag: bool):
 var actor := Model.new()
 for actor in actors:
  actor.tick()
 actor.tick()
 if flag:
  var other: RefCounted = Model.new()
  other.tick()
 else:
  other.tick()
`,
	})
	lines := map[int]bool{}
	for _, edge := range result.graph.Edges {
		if edge.To.Method == "tick" {
			lines[edge.Line] = true
		}
	}
	if !lines[6] || !lines[9] || lines[5] || lines[11] {
		t.Fatalf("wrong lexical call scope: %v", lines)
	}
}

func TestGodotRelativeLoadAndPreloadUseDifferentBases(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "class_name RootModel\n", "scripts/model.gd": "class_name NestedModel\n",
		"scripts/main.gd": "const Model = preload(\"model.gd\")\nfunc run():\n load(\"model.gd\")\n",
	})
	paths := map[int]string{}
	for _, ref := range result.facts.References {
		if ref.Type == "loads_resource" {
			paths[ref.Line] = ref.To
		}
	}
	if paths[1] != "scripts/model.gd" || paths[3] != "model.gd" {
		t.Fatalf("incorrect resource path bases: %v", paths)
	}
}

func TestGodotQuotedInnerParentRemainsUnresolved(t *testing.T) {
	result := godotFixture(map[string]string{
		"base.gd":    "class_name Outer\nclass Inner:\n pass\n",
		"derived.gd": "extends \"res://base.gd\".Inner\n",
	})
	found := false
	for _, ref := range result.facts.References {
		if ref.Type == "extends" {
			found = true
			if ref.Resolution == SymbolResolutionExact || ref.ToSymbolID != "" {
				t.Fatalf("inner parent bound to outer class: %#v", ref)
			}
		}
	}
	if !found {
		t.Fatal("unresolved inner-class inheritance evidence lost")
	}
}

func TestGodotInlineClassNameExtendsIsRecorded(t *testing.T) {
	result := godotFixture(map[string]string{
		"base.gd":    "class_name Base\n",
		"derived.gd": "class_name Derived extends Base\n",
	})
	base := godotDeclaration(t, result, "base.gd", "class", "Base")
	if !godotExactRef(result, "extends", "derived.gd", base.ID) {
		t.Fatal("documented inline inheritance omitted")
	}
}

func TestGodotLiteralReceiverDoesNotBecomeScriptAlias(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "static func tick():\n pass\n",
		"main.gd":  "const Model = preload(\"res://model.gd\")\nfunc run():\n \"Model\".tick()\n \"Model\".new()\n $Parent/Model.tick()\n $../Model.tick()\n %Model.tick()\n",
	})
	if len(result.graph.Edges) > 0 {
		t.Fatalf("literal receiver was promoted: %#v", result.graph.Edges)
	}
	for _, ref := range result.facts.References {
		if ref.Type == "instantiates" {
			t.Fatalf("literal was interpreted as a class alias: %#v", ref)
		}
	}
}

func TestGodotArithmeticBeforeReceiverRetainsStaticCalls(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "static func value():\n return 1\n",
		"main.gd":  "const Model = preload(\"res://model.gd\")\nfunc run():\n var ratio = 1/Model.value()\n var remainder = 1%Model.value()\n",
	})
	lines := map[int]bool{}
	for _, edge := range result.graph.Edges {
		if edge.To.Method == "value" && edge.To.File == "model.gd" {
			lines[edge.Line] = true
		}
	}
	if len(lines) != 2 || !lines[3] || !lines[4] {
		t.Fatalf("arithmetic was misread as a node path: %v", lines)
	}
}

func TestGodotLoadShadowingRespectsBlocksAndClosures(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func tick():\n pass\n",
		"main.gd": `func run(flag: bool):
 if flag:
  var load = custom_loader
  load("res://model.gd")
 load("res://model.gd")
 for load in loaders:
  load("res://model.gd")
 var callback = func(load):
  load("res://model.gd")
`,
	})
	lines := map[int]bool{}
	for _, ref := range result.facts.References {
		if ref.Type == "loads_resource" && ref.Internal {
			lines[ref.Line] = true
		}
	}
	if !lines[5] || len(lines) != 1 {
		t.Fatalf("shadowed loader produced a resource reference: %v", lines)
	}
}

func TestGodotFieldClosureDoesNotInvokeBuiltinLoader(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func tick():\n pass\n",
		"main.gd": `var callback = func(load):
 load("res://model.gd")
func run():
 load("res://model.gd")
static func create():
 load("res://model.gd")
`,
	})
	lines := map[int]bool{}
	for _, ref := range result.facts.References {
		if ref.Type == "loads_resource" && ref.Internal {
			lines[ref.Line] = true
		}
	}
	if len(lines) != 2 || !lines[4] || !lines[6] {
		t.Fatalf("incorrect loader scope for field closure or method: %v", lines)
	}
}

func TestGodotTypedDeclarationDoesNotConsumeFollowingAssignment(t *testing.T) {
	result := godotFixture(map[string]string{
		"model.gd": "func tick():\n pass\n",
		"main.gd":  "const Model = preload(\"res://model.gd\")\nfunc run():\n var worker: RefCounted\n worker = Model.new()\n worker.tick()\n",
	})
	for _, edge := range result.graph.Edges {
		if edge.To.Method == "tick" {
			t.Fatalf("later assignment was misread as declaration initializer: %#v", edge)
		}
	}
}

func TestGodotSignalReceiversDistinguishNodePathsAndLocalShadowing(t *testing.T) {
	result := godotFixture(map[string]string{
		"main.gd": `extends Node
signal finished
func run():
 $finished.emit()
 %finished.emit()
 $self.finished.emit()
 $self.emit_signal("finished")
 "finished".emit()
 finished.emit()
 self.finished.emit()
 var finished = custom_signal
 finished.emit()
 self.finished.emit()
 var emit_signal = custom_emitter
 emit_signal("finished")
 self.emit_signal("finished")
`,
	})
	lines := map[int]bool{}
	for _, ref := range result.facts.References {
		if ref.Type == "emits_signal" && ref.Resolution == SymbolResolutionExact {
			lines[ref.Line] = true
		}
	}
	if len(lines) != 4 || !lines[9] || !lines[10] || !lines[13] || !lines[16] {
		t.Fatalf("incorrect own-signal receivers: %v", lines)
	}
}

func TestGodotCallableRequiresSelfObjectRatherThanString(t *testing.T) {
	result := godotFixture(map[string]string{
		"main.gd": "signal finished\nfunc done():\n pass\nfunc run():\n finished.connect(Callable(\"self\", \"done\"))\n finished.connect(Callable(self, \"done\"))\n",
	})
	lines := map[int]bool{}
	for _, ref := range result.facts.References {
		if ref.Type == "connects_callback" && ref.Resolution == SymbolResolutionExact {
			lines[ref.Line] = true
		}
	}
	if len(lines) != 1 || !lines[6] {
		t.Fatalf("incorrect Callable object receiver: %v", lines)
	}
}

func TestGodotBitwiseAssignmentsInvalidateConstructedReceivers(t *testing.T) {
	for _, operator := range []string{"&=", "|=", "^=", "<<=", ">>=", "**="} {
		t.Run(operator, func(t *testing.T) {
			result := godotFixture(map[string]string{
				"model.gd": "func tick():\n pass\n",
				"main.gd":  "const Model = preload(\"res://model.gd\")\nvar field = Model.new()\nfunc run(mask):\n var worker = Model.new()\n worker " + operator + " mask\n field " + operator + " mask\n worker.tick()\n field.tick()\n",
			})
			for _, edge := range result.graph.Edges {
				if edge.To.Method == "tick" {
					t.Fatalf("modified receiver retained an exact target: %#v", edge)
				}
			}
		})
	}
}
