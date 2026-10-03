package scan

import "testing"

func TestSwiftStructuredCallsLabelsAndTests(t *testing.T) {
	source := parseSwiftSource(FileRecord{Path: "Game.swift"}, `import XCTest
class Service {
 func load(id: Int) {}
 func load(name: String) {}
}
class ServiceTests: XCTestCase {
 let service: Service = Service()
 func testLoads() { service.load(id: 1); service.load(name: "Knight") }
}`)
	result := analyzeSwiftProject([]swiftSource{source})
	if len(source.types) != 2 || len(result.graph.Edges) != 2 || len(result.tests) != 2 {
		t.Fatalf("types=%#v calls=%#v tests=%#v", source.types, result.graph, result.tests)
	}
}

func TestSwiftOverloadAndConditionalUncertainty(t *testing.T) {
	for _, body := range []string{
		`class Service {func run(_ x:Int){};func run(_ x:String){};func call(value: Any){run(value)}}`,
		"#if FEATURE\nclass Service {func run(){};func call(){run()}}\n#endif",
	} {
		result := analyzeSwiftProject([]swiftSource{parseSwiftSource(FileRecord{Path: "Unknown.swift"}, body)})
		if len(result.graph.Edges) != 0 {
			t.Fatalf("uncertainty promoted: %#v", result.graph)
		}
	}
}

func TestSwiftCommentsRawStringsAndExtensions(t *testing.T) {
	source := parseSwiftSource(FileRecord{Path: "Game.swift"}, "/* class Fake { /* nested */ } */\nstruct Knight {}\nextension Knight { func turn(){} }\nlet raw = #\"class Fake2 { func nope(){} }\"#\nactor Engine { func step(){} }\n")
	if len(source.types) != 2 || len(source.members) != 3 {
		t.Fatalf("types=%#v members=%#v", source.types, source.members)
	}
}

func TestSwiftLiteralHTTPAndTypedPersistence(t *testing.T) {
	source := parseSwiftSource(FileRecord{Path: "API.swift"}, `import Foundation
import SwiftData
class API {
 var context: ModelContext
 func load() async throws {
 let url = URL(string: "https://example.test/items")!
 var request = URLRequest(url: url)
 request.httpMethod = "POST"
 let reply = try await URLSession.shared.data(for: request)
 context.save()
 }
}`)
	result := analyzeSwiftProject([]swiftSource{source})
	persistence := 0
	for _, fact := range result.capabilities {
		if fact.Capability == CapabilityPersistence {
			persistence++
		}
	}
	if len(result.code.APIContracts) != 1 || result.code.APIContracts[0].HTTPMethod != "POST" || persistence != 1 {
		t.Fatalf("HTTP=%#v architecture=%#v", result.code.APIContracts, result.capabilities)
	}
}

func TestSwiftFrameworkShadowsAndDynamicURLsStayUnknown(t *testing.T) {
	source := parseSwiftSource(FileRecord{Path: "API.swift"}, `import Foundation
class URLSession {static var shared: URLSession;func data(from x: String){}}
class API {func load(){URLSession.shared.data(from: URL(string: "https://\(host)/items")!)}}
`)
	if result := analyzeSwiftProject([]swiftSource{source}); len(result.code.APIContracts) != 0 {
		t.Fatal(result.code.APIContracts)
	}
}

func TestSwiftUIStateLinksAreSourceEvidence(t *testing.T) {
	source := parseSwiftSource(FileRecord{Path: "View.swift"}, `import SwiftUI
struct CounterView: View {
 @State var count: Int = 0
 var body: some View { Text("Count"); Button("Next") { count = count + 1 } }
}`)
	result := analyzeSwiftProject([]swiftSource{source})
	links := 0
	for _, ref := range result.facts.References {
		if ref.Type == "uses_state" {
			links++
		}
	}
	if links != 2 {
		t.Fatalf("state references: %#v", result.facts)
	}
}

func TestSwiftLocalAndClosureShadowsDoNotBindOuterReceiver(t *testing.T) {
	s := parseSwiftSource(FileRecord{Path: "Scope.swift"}, `class Service {func load() {}}
class Caller {
 let service: Service
 func run() {
 service.load()
 if true { let service = "text"; service.load() }
 service.load()
 items.forEach { service in service.load() }
 service.load()
 }
}`)
	result := analyzeSwiftProject([]swiftSource{s})
	if len(result.graph.Edges) != 3 {
		t.Fatalf("shadowed receiver became exact, or outer scope lost: %#v", result.graph)
	}
}

func TestSwiftDynamicRequestMutationDoesNotKeepLiteralEvidence(t *testing.T) {
	for _, mutation := range []string{`request.httpMethod = method`, `request.url = dynamic`, `request = dynamic`} {
		body := `import Foundation
class API {func load(method: String, dynamic: URLRequest) async throws {
 let url = URL(string: "https://example.test/items")!
 var request = URLRequest(url: url)
 ` + mutation + `
 let response = try await URLSession.shared.data(for: request)
}}
`
		result := analyzeSwiftProject([]swiftSource{parseSwiftSource(FileRecord{Path: "API.swift"}, body)})
		if len(result.code.APIContracts) != 0 {
			t.Fatalf("mutated request retained fabricated static evidence: %#v", result.code.APIContracts)
		}
	}
}

func TestSwiftComparisonsCannotSelectWrongArity(t *testing.T) {
	s := parseSwiftSource(FileRecord{Path: "Compare.swift"}, `struct Service {func test(_ a: Bool){};func run(a: Int,b: Int){test(a < b, b)}}`)
	if result := analyzeSwiftProject([]swiftSource{s}); len(result.graph.Edges) != 0 {
		t.Fatal(result.graph)
	}
}

func TestSwiftInheritedMethodsAndGenericConstraints(t *testing.T) {
	s := parseSwiftSource(FileRecord{Path: "Inheritance.swift"}, `class Parent {func load(id: Int){}}
class Child: Parent {func run(){load(id: 1);super.load(id: 2)}}
struct Container<T> {}
extension Container where T: Equatable {func constrained(){};func run(){constrained()}}
`)
	result := analyzeSwiftProject([]swiftSource{s})
	if len(result.graph.Edges) != 2 {
		t.Fatalf("inheritance absent or constrained extension promoted: %#v", result.graph)
	}
	for _, edge := range result.graph.Edges {
		if edge.To.Owner != "project.Parent" {
			t.Fatal(edge)
		}
	}
}

func TestSwiftPMModulesRequireImportsAndKeepDuplicateNamesAmbiguous(t *testing.T) {
	provider := parseSwiftSource(FileRecord{Path: "Sources/Core/Service.swift"}, `class Service {func load(id: Int){}}`)
	for _, importLine := range []string{"", "import Core\n"} {
		caller := parseSwiftSource(FileRecord{Path: "Sources/App/App.swift"}, importLine+`class App {var service: Service;func run(){service.load(id:1)}}`)
		result := analyzeSwiftProject([]swiftSource{provider, caller})
		want := 0
		if importLine != "" {
			want = 1
		}
		if len(result.graph.Edges) != want {
			t.Fatalf("module import=%q, graph=%#v", importLine, result.graph)
		}
	}
	duplicate := parseSwiftSource(FileRecord{Path: "Packages/Other/Sources/Core/Service.swift"}, `class Service {func load(id: Int){}}`)
	caller := parseSwiftSource(FileRecord{Path: "Sources/App/App.swift"}, "import Core\n"+`class App {var service: Service;func run(){service.load(id:1)}}`)
	if result := analyzeSwiftProject([]swiftSource{provider, duplicate, caller}); len(result.graph.Edges) != 0 {
		t.Fatal(result.graph)
	}
}

func TestSwiftQueryModelsAndProjectedBindings(t *testing.T) {
	s := parseSwiftSource(FileRecord{Path: "View.swift"}, `import SwiftUI
import SwiftData
class Item {}
struct ItemView: View {
 @Query var items: [Item]
 @State var text: String = ""
 var body: some View {TextField("Name",text:$text)}
}`)
	result := analyzeSwiftProject([]swiftSource{s})
	models, state := 0, 0
	for _, ref := range result.facts.References {
		if ref.Type == "uses_model" && ref.Resolution == SymbolResolutionExact {
			models++
		}
		if ref.Type == "uses_state" {
			state++
		}
	}
	if models != 1 || state != 1 {
		t.Fatal(result.facts.References)
	}
}

func TestXcodeProjectMarkersAreDiscoveredWithoutPackageManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "apps/Game/Game.xcodeproj/project.pbxproj", "{}")
	writeFile(t, root, "apps/Game/Game/App.swift", "struct App {}")
	projects, err := discoverWorkspaceProjects(root, root, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Path != "apps/Game" {
		t.Fatal(projects)
	}
}

func TestSwiftScopedImportsDoNotInventDeclarations(t *testing.T) {
	s := parseSwiftSource(FileRecord{Path: "Sources/App/App.swift"}, `import struct Core.Service
struct Caller {func run(){}}
`)
	if len(s.types) != 1 || s.types[0].symbol.Name != "Caller" || len(s.imports) != 1 || s.imports[0] != "Core" {
		t.Fatalf("types=%#v imports=%#v", s.types, s.imports)
	}
}

func TestSwiftNumericLiteralsUseContextAndDefaultTypes(t *testing.T) {
	s := parseSwiftSource(FileRecord{Path: "Numbers.swift"}, `struct Service {
 func rotate(degrees: Double) {}
 func load(_ value: Int) {} func load(_ value: Double) {}
 func run(value: Int) {rotate(degrees: 90);load(1);rotate(degrees: value)}
}`)
	result := analyzeSwiftProject([]swiftSource{s})
	if len(result.graph.Edges) != 2 {
		t.Fatal(result.graph)
	}
	seen := map[string]bool{}
	for _, edge := range result.graph.Edges {
		seen[edge.TargetQualifiedName] = true
	}
	if !seen["project.Service.rotate(degrees:Double)"] || !seen["project.Service.load(_:Int)"] {
		t.Fatal(seen)
	}
}

func TestSwiftRegexAndNestedInterpolationRemainOpaque(t *testing.T) {
	s := parseSwiftSource(FileRecord{Path: "Literals.swift"}, `struct Real {
 func run() {
 let value = "\(render("class Fake { func nope(){} }"))"
 let regex = #/class Fake2 { func nope2(){} }/#
 let bare = /class Fake3 { func nope3(){} }/
 }
}`)
	if len(s.types) != 1 || len(s.members) != 1 {
		t.Fatalf("literal introduced declarations: %#v %#v", s.types, s.members)
	}
	if result := analyzeSwiftProject([]swiftSource{s}); len(result.graph.Edges) != 0 {
		t.Fatal(result.graph)
	}
}
