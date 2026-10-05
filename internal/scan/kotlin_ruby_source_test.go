package scan

import "testing"

func TestKotlinTypedCallsNamedDefaultsAndTestMapping(t *testing.T) {
	file := FileRecord{Path: "src/Service.kt", Language: "kotlin"}
	body := `package demo
import org.junit.jupiter.api.Test
class Service {
 fun load(id: String, count: Int = 1) { }
 fun run(service: Service) { service.load(id = "x") }
 @Test
 fun verifies(service: Service) { service.load("x", 2) }
}`
	result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
	if len(result.graph.Edges) != 2 || len(result.tests) != 1 {
		t.Fatal(result.graph.Edges, result.tests, result.facts.Declarations)
	}
}
func TestKotlinOverloadsAndCallbackBodiesStayOpen(t *testing.T) {
	for _, body := range []string{`class Service { fun load(id: String) {} fun load(count: Int) {} fun run(service: Service) { service.load(unknown) } }`, `class Service { fun load() {} fun run(service: Service) { val callback = { service.load() } } }`} {
		file := FileRecord{Path: "Service.kt", Language: "kotlin"}
		result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
		if len(result.graph.Edges) != 0 {
			t.Fatal("unproven Kotlin binding", result.graph.Edges)
		}
	}
}
func TestRubyDeclarationsExplicitSelfCallsAndLiteralSafety(t *testing.T) {
	file := FileRecord{Path: "formula.rb", Language: "ruby"}
	body := `class Formula
 def build()
  self.prepare()
 end
 def prepare()
  value = %q{class Fake; def wrong(); end; end}
  text = <<~TEXT
class FakeAgain
 def wrong()
 end
end
TEXT
 end
end`
	result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
	if len(result.graph.Edges) != 1 || result.graph.Edges[0].To.Method != "prepare" {
		t.Fatal(result.graph.Edges, result.facts.Declarations)
	}
	for _, symbol := range result.facts.Declarations {
		if symbol.Name == "Fake" || symbol.Name == "FakeAgain" || symbol.Name == "wrong" {
			t.Fatal("Ruby literal fabricated a declaration", symbol)
		}
	}
}
