package scan

import "testing"

func TestCSharpKnownLiteralOverloadsAndInheritedMethods(t *testing.T) {
	s := parseCSharpSource(FileRecord{Path: "Example.cs"}, `class Base {public void Load(int value){} public void Load(string value){} public void Load(char value){} }
class Player: Base {void Tick(){Load(1);Load("one");Load('x');}}`)
	result := analyzeCSharpProject([]csharpSource{s})
	if len(result.graph.Edges) != 3 {
		t.Fatalf("typed inherited calls absent: %#v", result.graph)
	}
	seen := map[string]bool{}
	for _, edge := range result.graph.Edges {
		seen[edge.TargetQualifiedName] = true
	}
	if !seen["Base.Load(int)"] || !seen["Base.Load(string)"] || !seen["Base.Load(char)"] {
		t.Fatal(seen)
	}
}

func TestCSharpNamedOptionalAndRefArguments(t *testing.T) {
	s := parseCSharpSource(FileRecord{Path: "Args.cs"}, `class Service {
 void Load(int id, string label = "default") {}
 void Change(int id) {} void Change(ref int id) {}
 void Run(int value) {Load(id: 1); Change(value); Change(ref value);}
}`)
	result := analyzeCSharpProject([]csharpSource{s})
	if len(result.graph.Edges) != 3 {
		t.Fatal(result.graph)
	}
	seen := map[string]bool{}
	for _, edge := range result.graph.Edges {
		seen[edge.TargetQualifiedName] = true
	}
	if !seen["Service.Load(int,string)"] || !seen["Service.Change(int)"] || !seen["Service.Change(ref int)"] {
		t.Fatal(seen)
	}
}

func TestCSharpRepeatedCallsHaveDistinctEvidence(t *testing.T) {
	s := parseCSharpSource(FileRecord{Path: "Repeat.cs"}, `class Service {void Load(){} void Run(){Load();Load();}}`)
	result := analyzeCSharpProject([]csharpSource{s})
	if len(result.graph.Edges) != 2 || result.graph.Edges[0].ID == result.graph.Edges[1].ID {
		t.Fatal(result.graph)
	}
}

func TestCSharpConstantConversionsDoNotChooseWrongWideOverload(t *testing.T) {
	s := parseCSharpSource(FileRecord{Path: "Constants.cs"}, `class Service {void Load(short value){} void Load(long value){} void Run(){Load(1);Load(50000);}}`)
	result := analyzeCSharpProject([]csharpSource{s})
	if len(result.graph.Edges) != 2 || result.graph.Edges[0].TargetQualifiedName != "Service.Load(short)" || result.graph.Edges[1].TargetQualifiedName != "Service.Load(long)" {
		t.Fatal(result.graph)
	}
}
