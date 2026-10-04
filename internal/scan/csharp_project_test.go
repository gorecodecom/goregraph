package scan

import "testing"

func TestCSharpStaticCallsAndAmbiguity(t *testing.T) {
	source := parseCSharpSource(FileRecord{Path: "Game.cs"}, `namespace Game;
 class Service { public void Run() {} public void Over(int n){} public void Over(string n){} }
 class Player {
 Service service;
 [Test] public void Tick(object value) {service.Run();service.Over(value);dynamic unknown=service;unknown.Run();External.Run();}
 }`)
	result := analyzeCSharpProject([]csharpSource{source})
	if len(result.graph.Edges) != 1 || len(result.tests) != 1 {
		t.Fatalf("calls=%#v tests=%#v", result.graph, result.tests)
	}
	facts := FinalizeProjectSymbolFacts(nil, WorkspaceIndex{}, result.facts)
	unresolved, ambiguous := 0, 0
	for _, ref := range facts.References {
		if ref.Type != "calls_method_owner" {
			continue
		}
		if ref.Resolution == SymbolResolutionUnresolved {
			unresolved++
		}
		if ref.Resolution == SymbolResolutionAmbiguous {
			ambiguous++
		}
	}
	if unresolved != 2 || ambiguous != 1 {
		t.Fatalf("unresolved=%d ambiguous=%d refs=%#v", unresolved, ambiguous, facts.References)
	}
}

func TestCSharpHTTPAndConditionalSafety(t *testing.T) {
	source := parseCSharpSource(FileRecord{Path: "Controller.cs"}, `using System.Net.Http;
 namespace Game;
 [ApiController][Route("api/[controller]")]
 class PlayersController : ControllerBase {
 HttpClient client;
 [HttpGet("{id}")] public void Get() {client.GetAsync("/items");client.GetAsync($"/{id}");}
 [HttpPost(DynamicRoute)] public void Dynamic() {}
 }`)
	result := analyzeCSharpProject([]csharpSource{source})
	if len(result.code.Routes) != 1 || result.code.Routes[0].Path != "/api/Players/{id}" || len(result.code.APIContracts) != 1 {
		t.Fatalf("code=%#v", result.code)
	}
	conditional := parseCSharpSource(FileRecord{Path: "Maybe.cs"}, "#if MAYBE\nclass Maybe {void A(){B();} void B(){}}\n#endif")
	if calls := analyzeCSharpProject([]csharpSource{conditional}).graph.Edges; len(calls) != 0 {
		t.Fatalf("conditional compilation promoted: %#v", calls)
	}
}

func TestCSharpConditionalProviderCannotBecomeExact(t *testing.T) {
	provider := parseCSharpSource(FileRecord{Path: "Service.cs"}, "#if FEATURE\nnamespace Game; class Service {public void Run(){}}\n#endif")
	caller := parseCSharpSource(FileRecord{Path: "Player.cs"}, "namespace Game; class Player {Service service;void Tick(){service.Run();}}")
	result := analyzeCSharpProject([]csharpSource{provider, caller})
	if len(result.graph.Edges) != 0 {
		t.Fatalf("conditional provider promoted: %#v", result.graph)
	}
}

func TestCSharpAssemblyBoundariesRequireDeclaredDependency(t *testing.T) {
	for _, reference := range []string{"", `"Core"`} {
		metadata := ProjectSymbolFacts{}
		MergeProjectSymbolFacts(&metadata, extractDotnetMetadata(FileRecord{Path: "Assets/Game/Game.asmdef"}, `{"name":"Game","references":[`+reference+`]}`))
		MergeProjectSymbolFacts(&metadata, extractDotnetMetadata(FileRecord{Path: "Assets/Core/Core.asmdef"}, `{"name":"Core"}`))
		metadata = resolveDotnetMetadata(metadata)
		sources := []csharpSource{parseCSharpSource(FileRecord{Path: "Assets/Game/Player.cs"}, "namespace Game; class Player {Service service;void Tick(){service.Run();}}"), parseCSharpSource(FileRecord{Path: "Assets/Core/Service.cs"}, "namespace Game; class Service {public void Run(){}}")}
		assignCSharpModules(sources, metadata)
		result := analyzeCSharpProject(sources)
		expected := 0
		if reference != "" {
			expected = 1
		}
		if len(result.graph.Edges) != expected {
			t.Fatalf("undeclared assembly edge or lost dependency: ref=%s graph=%#v", reference, result.graph)
		}
	}
}

func TestCSharpComparisonArgumentsCannotSelectWrongArity(t *testing.T) {
	source := parseCSharpSource(FileRecord{Path: "Test.cs"}, "class Service {void Call(bool x){} void Tick(){Call(a > 0, b, c);}}")
	result := analyzeCSharpProject([]csharpSource{source})
	if len(result.graph.Edges) != 0 {
		t.Fatalf("comparison syntax selected wrong overload: %#v", result.graph)
	}
}
