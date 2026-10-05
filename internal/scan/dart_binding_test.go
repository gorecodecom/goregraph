package scan

import "testing"

func dartTestProject(files map[string]string, manifests ...DartPackageRecord) dartAnalysis {
	var sources []dartSource
	for file, body := range files {
		sources = append(sources, parseDartSource(FileRecord{Path: file}, body))
	}
	assignDartPackages(sources, manifests)
	return analyzeDartProject(sources, manifests)
}
func dartHasCall(result dartAnalysis, from, to string) bool {
	for _, edge := range result.graph.Edges {
		if edge.From.Method == from && edge.To.Method == to {
			return true
		}
	}
	return false
}

func TestDartTypedCallsImportsBarrelsAndParts(t *testing.T) {
	result := dartTestProject(map[string]string{
		"lib/service.dart":   `part 'service.g.dart'; class Service { void load({required String id, int count = 1}) {} void _secret() {} } void shared() { generated(); }`,
		"lib/service.g.dart": `part of 'service.dart'; void generated() { Service()._secret(); }`,
		"lib/api.dart":       `export 'service.dart' show Service, shared;`,
		"lib/main.dart":      `import 'api.dart' as api; void run(api.Service service) { service.load(id: 'x'); api.shared(); }`,
	})
	if !dartHasCall(result, "run", "load") || !dartHasCall(result, "run", "shared") || !dartHasCall(result, "shared", "generated") {
		t.Fatalf("calls=%#v", result.graph.Edges)
	}
	for _, ref := range result.facts.References {
		if ref.Type == "calls_method_owner" && ref.TargetQualifiedName == "load" && ref.Resolution != SymbolResolutionExact {
			t.Fatal(ref)
		}
	}
}

func TestDartPrivacyAmbiguityAndArgumentSafety(t *testing.T) {
	for _, body := range []string{
		`import 'a.dart'; void run(Service service) { service._secret(); service.load(); service.load(id:'x', unknown:1); }`,
		`import 'a.dart'; import 'b.dart'; void run(Service service) { service.load(id:'x'); }`,
		`import 'a.dart' hide Service; void run(Service service) { service.load(id:'x'); }`,
		`import 'a.dart' deferred as a; void run(a.Service service) { service.load(id:'x'); }`,
		`void run(dynamic service) { service.load(id:'x'); }`,
		`import 'a.dart'; void run(Function load) { load(id:'x'); }`,
	} {
		result := dartTestProject(map[string]string{"lib/a.dart": `class Service { void load({required String id}) {} void _secret() {} } void load({required String id}) {}`, "lib/b.dart": `class Service { void load({required String id}) {} }`, "lib/main.dart": body})
		if len(result.graph.Edges) != 0 {
			t.Fatalf("unsafe binding for %s: %#v", body, result.graph.Edges)
		}
		facts := result.facts
		facts = FinalizeProjectSymbolFacts(nil, WorkspaceIndex{}, facts)
		for _, ref := range facts.References {
			if ref.Type == "calls_method_owner" && ref.Resolution == SymbolResolutionExact {
				t.Fatal("finalization promoted unresolved binding", ref)
			}
		}
	}
}

func TestDartPackagesDoNotMergeUnrelatedNames(t *testing.T) {
	packages := []DartPackageRecord{extractDartPackage("app/pubspec.yaml", "name: app\ndependencies:\n  service:\n    path: ../service\n"), extractDartPackage("service/pubspec.yaml", "name: service\n"), extractDartPackage("other/pubspec.yaml", "name: service\n")}
	result := dartTestProject(map[string]string{"app/lib/main.dart": `import 'package:service/service.dart'; void run(Service s) { s.load(); }`, "service/lib/service.dart": `class Service { void load() {} }`, "other/lib/service.dart": `class Service { void load() {} }`}, packages...)
	if len(result.graph.Edges) != 1 || result.graph.Edges[0].To.File != "service/lib/service.dart" {
		t.Fatal(result.graph.Edges)
	}
}

func TestDartLocalInferenceInheritanceAndScopes(t *testing.T) {
	result := dartTestProject(map[string]string{"lib/main.dart": `class Base { void work(int count, {String? tag}) {} } mixin Cache { void clear() {} } class Child extends Base with Cache { Child(); void run() { final child=Child(); child.work(1,tag:'ok'); child.clear(); { dynamic child; child.work(1); } child.work(2); } }`})
	count := 0
	for _, e := range result.graph.Edges {
		if e.To.Method == "work" {
			count++
		}
	}
	if count != 2 || !dartHasCall(result, "run", "clear") {
		t.Fatal(result.graph.Edges)
	}
}

func TestDartPubWorkspaceBindingsRequireMembershipAndVersions(t *testing.T) {
	manifests := []DartPackageRecord{
		extractDartPackage("pubspec.yaml", "name: _\nworkspace:\n  - packages/*\n"),
		extractDartPackage("packages/app/pubspec.yaml", "name: app\nresolution: workspace\ndependencies:\n  service: ^1.0.0\n"),
		extractDartPackage("packages/service/pubspec.yaml", "name: service\nversion: 1.2.0\nresolution: workspace\n"),
		extractDartPackage("unrelated/pubspec.yaml", "name: service\nversion: 1.2.0\nresolution: workspace\n"),
	}
	files := map[string]string{"packages/app/lib/app.dart": `import 'package:service/service.dart'; void run(Service s) { s.load(); }`, "packages/service/lib/service.dart": `class Service { void load() {} }`, "unrelated/lib/service.dart": `class Service { void load() {} }`}
	result := dartTestProject(files, manifests...)
	if len(result.graph.Edges) != 1 || result.graph.Edges[0].To.File != "packages/service/lib/service.dart" {
		t.Fatal(result.graph, manifests)
	}
	manifests[2].Version = "2.0.0"
	if result = dartTestProject(files, manifests...); len(result.graph.Edges) != 0 {
		t.Fatal("incompatible workspace version promoted", result.graph)
	}
}
