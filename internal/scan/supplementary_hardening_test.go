package scan

import "testing"

func TestCFamilyFunctionPointersAndConditionalTargetsStayOpen(t *testing.T) {
	for _, body := range []string{
		`void load() {} void run(void (*load)()) { load(); }`,
		`void load() {} void run() { void (*load)() = other; load(); }`,
		"#if FEATURE\nvoid load() {}\n#endif\nvoid run() { load(); }",
	} {
		file := FileRecord{Path: "bridge.cpp", Language: "cpp"}
		result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
		if len(result.graph.Edges) != 0 {
			t.Fatal("unproven pointer or conditional binding", result.graph.Edges)
		}
	}
}
func TestObjectiveCClassAndInstanceSelectorsStayDistinct(t *testing.T) {
	file := FileRecord{Path: "Bridge.m", Language: "objectivec"}
	body := `@implementation Bridge
+ (void)load { }
- (void)run { [self load]; }
@end`
	result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
	if len(result.graph.Edges) != 0 {
		t.Fatal("instance message bound to class selector", result.graph.Edges)
	}
}

func TestRubyRegexAndDeferredBlocksDoNotInventDeclarations(t *testing.T) {
	file := FileRecord{Path: "Formula.rb", Language: "ruby"}
	body := "class Formula\n def run()\n  pattern = /class Fake\n def invented()\n end\nend/\n end\nend\n"
	result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
	for _, symbol := range result.facts.Declarations {
		if symbol.Name == "Fake" || symbol.Name == "invented" {
			t.Fatal(symbol)
		}
	}
}
func TestObjectiveCHeaderAndLauncherDetection(t *testing.T) {
	if got := detectSourceLanguage("Bridge.h", "@interface Bridge\n- (void)run;\n@end"); got != "objectivec" {
		t.Fatal(got)
	}
	if got := detectSourceLanguage("Bridge.h", "// @interface Fake\nvoid run(void);"); got != "c" {
		t.Fatal(got)
	}
	if detectLanguage("launch.command") != "shell" || detectLanguage("pubspec.lock") != "yaml" {
		t.Fatal("native build and launcher formats missing")
	}
}

func TestRubyDocumentationAndDeferredBlockCallsStayOpen(t *testing.T) {
	file := FileRecord{Path: "Formula.rb", Language: "ruby"}
	body := "=begin\nclass Fake\n def wrong()\n end\nend\n=end\nclass Formula\n def prepare()\n end\n def run()\n  items.each do\n   self.prepare()\n  end\n end\nend\n"
	result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
	if len(result.graph.Edges) != 0 {
		t.Fatal("deferred body assigned to immediate caller", result.graph.Edges)
	}
	for _, symbol := range result.facts.Declarations {
		if symbol.Name == "Fake" || symbol.Name == "wrong" {
			t.Fatal(symbol)
		}
	}
}
func TestCSSResourcesDoNotExportCredentialsOrOpaquePayloads(t *testing.T) {
	file := FileRecord{Path: "styles.css", Language: "css"}
	body := `@import "https://user:secret@example.test/styles.css";
.board { background: url('data:image/svg+xml;base64,private'); mask: url('https://example.test/icon.svg?token=secret'); }`
	result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
	if len(result.facts.References) != 1 || result.facts.References[0].To != "https://example.test/icon.svg" {
		t.Fatal(result.facts.References)
	}
}

func TestKotlinLocalReceiverScopesDoNotLeak(t *testing.T) {
	file := FileRecord{Path: "Service.kt", Language: "kotlin"}
	body := `class Service {
 fun load() {}
 fun run(source: Any) {
  if (ready) { val source = Service(); source.load() }
  source.load()
 }
}`
	result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
	if len(result.graph.Edges) != 1 || result.graph.Edges[0].Line != 4 {
		t.Fatal("block-local receiver escaped its scope", result.graph.Edges)
	}
}

func TestNativeAdaptersAcceptTruncatedSourceWithoutPanicking(t *testing.T) {
	bodies := []string{"", "companion object", "@property (", "class", "fun f(", "class A { fun f(x: Any) }", "@interface A", "def", "class A\n def run\n", "void f(int (*callback)(", "<form action=\"", ".board { --color:"}
	for _, language := range []string{"dart", "kotlin", "objectivec", "c", "cpp", "ruby", "html", "css", "batch"} {
		for _, body := range bodies {
			t.Run(language+"/"+body, func(t *testing.T) {
				file := FileRecord{Path: "truncated", Language: language}
				if language == "dart" {
					analyzeDartProject([]dartSource{parseDartSource(file, body)}, nil)
				} else {
					analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
				}
			})
		}
	}
}
