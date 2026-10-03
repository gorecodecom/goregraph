package scan

import "testing"

func TestCSharpStructuralDeclarations(t *testing.T) {
	source := parseCSharpSource(FileRecord{Path: "Assets/Game.cs"}, `using System; using Alias = Game.Service;
namespace Game;
// class Fake { void Wrong() {} }
[Route("api/[controller]")]
public partial class Player : MonoBehaviour {
 private Service service;
 public string Name { get; set; }
 public Player(Service value) { service = value; }
 [Test] public void Works() { var text = "class Fake { }"; service.Run(); }
 public T Echo<T>(T value) => value;
 [Route("api/" + Name)] public void Dynamic() { }
 public void Overload(int a) { }
 public void Overload(string a) { }
 public class Nested { public void Tick() {} }
}`)
	if len(source.types) != 2 {
		t.Fatalf("types: %#v", source.types)
	}
	if len(source.members) != 9 {
		t.Fatalf("members: %#v", source.members)
	}
	if source.aliases["Alias"] != "Game.Service" {
		t.Fatal(source.aliases)
	}
	for _, member := range source.members {
		if member.symbol.Name == "Dynamic" && member.attributes[0].literal {
			t.Fatal("dynamic attribute promoted")
		}
	}
}

func TestCSharpLiteralsAreOpaque(t *testing.T) {
	source := parseCSharpSource(FileRecord{Path: "a.cs"}, "class Real { string s = @\"class Fake { \"\"x\"\" }\"; void Run() { var raw = \"\"\"class Fake2 {}\"\"\"; var value = $\"{Call()}\"; } }")
	if len(source.types) != 1 || len(source.members) != 2 {
		t.Fatalf("types=%v members=%v", source.types, source.members)
	}
	if _, balanced := csharpPairs(source.tokens); !balanced {
		t.Fatal("literal braces changed syntax")
	}
}

func TestCSharpNestedInterpolatedStringQuotesRemainOpaque(t *testing.T) {
	s := parseCSharpSource(FileRecord{Path: "Literals.cs"}, `class Real {void Run(){var text=$"{Render("class Fake { void Nope(){} }")}";}}`)
	if len(s.types) != 1 || len(s.members) != 1 {
		t.Fatalf("literal introduced declarations: %#v %#v", s.types, s.members)
	}
	for _, token := range s.tokens {
		if token.text == "Render" || token.text == "Fake" {
			t.Fatal(s.tokens)
		}
	}
}
