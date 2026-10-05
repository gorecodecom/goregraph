package scan

import (
	"strings"
	"testing"
)

func TestDartClosuresAndConditionalImportsDoNotFabricateCalls(t *testing.T) {
	for _, body := range []string{
		`import 'service.dart'; void run(Service service) { values.forEach((service) { service.load(); }); }`,
		`import 'service.dart' if (dart.library.io) 'alternate.dart'; void run(Service service) { service.load(); }`,
		`import 'service.dart'; void run() { dynamic Service; Service(); }`,
		`import 'service.dart'; void run(Service service) { final callback=(Service service) => service.load(); }`,
	} {
		result := dartTestProject(map[string]string{"lib/service.dart": `class Service { Service(); void load() {} }`, "lib/alternate.dart": `class Service { void load() {} }`, "lib/main.dart": body})
		for _, e := range result.graph.Edges {
			if e.To.Method == "load" && e.From.Method == "run" {
				t.Fatal("unproven closure or conditional import", body, e)
			}
		}
	}
}

func TestDartNavigationUsesTheSameLexicalRules(t *testing.T) {
	body := "/* class Fake { /* nested */ void wrong(){} } */\nclass API {\n Future<void> load({required String id}) async {\n final message=\"${render('}')}\";\n }\n int count() => 1;\n}\n"
	declarations := DartSourceDeclarations("lib/api.dart", body)
	found := false
	for _, d := range declarations {
		if d.Name == "load" {
			found = true
			if d.Line != 3 || d.EndLine != 5 {
				t.Fatal(d)
			}
		}
	}
	if !found {
		t.Fatal(declarations)
	}
	mask := DartSourceCodeMask(body)
	if strings.Contains(mask, "Fake") || strings.Contains(mask, "render") {
		t.Fatal(mask)
	}
	if len(DartSourceDeclarations("bad.dart", "class Missing {")) != 0 {
		t.Fatal("malformed navigation verified")
	}
}

func TestDartPrimaryConstructorsAndNamedLibraryParts(t *testing.T) {
	result := dartTestProject(map[string]string{
		"lib/a.dart":   `library example.models; part 'a.g.dart'; class Service { void load() {} } class Holder.named(final Service service); class Person({required final String _name}); void run(Service service) { Holder.named(service); Person(name:'Test'); generated(); }`,
		"lib/a.g.dart": `part of example.models; void generated() {}`,
	})
	for _, name := range []string{"Holder.named", "Person", "generated"} {
		if !dartHasCall(result, "run", name) {
			t.Error("missing primary/part binding", name, result.graph.Edges)
		}
	}
}
func TestDartInferredReceiverAssignmentsRemainOpen(t *testing.T) {
	result := dartTestProject(map[string]string{"lib/a.dart": `class Service { Service(); void load() {} } void run() { var service=Service(); service=unknown(); service.load(); }`})
	if dartHasCall(result, "run", "load") {
		t.Fatal("stale inferred receiver type", result.graph.Edges)
	}
}
