package scan

import "testing"

func nativeFixture(language string, bodies map[string]string) supplementaryAnalysis {
	var files []FileRecord
	var sources []supplementarySource
	for name, body := range bodies {
		file := FileRecord{Path: name, Language: language}
		files = append(files, file)
		sources = append(sources, supplementarySource{file, body})
	}
	return analyzeSupplementarySources(sources, files)
}

func TestCFamilyDeepQualifiedCallsAndHeaderChains(t *testing.T) {
	result := nativeFixture("cpp", map[string]string{
		"main.cpp":  "#include \"all.h\"\nint run(){return tools::detail::value(1);}",
		"all.h":     "#include \"value.h\"\n",
		"value.h":   "namespace tools::detail { int value(int count); }",
		"value.cpp": "namespace tools { namespace detail { int value(int count){return count;} } }",
	})
	if len(result.graph.Edges) != 1 || result.graph.Edges[0].To.Owner != "tools::detail" || result.graph.Edges[0].To.File != "value.cpp" {
		t.Fatal(result.graph.Edges, result.facts.Declarations)
	}
}

func TestCFamilyDeepTypedLocalsThisAndNegativeBindings(t *testing.T) {
	result := nativeFixture("cpp", map[string]string{"main.cpp": `class Service { public: void load(){} void run(){this->load();} }; void use(Service &source){source.load(); Service local; local.load(); missing.load();} void load(){} void wrong(){unknown::load();}`})
	if len(result.graph.Edges) != 3 {
		t.Fatal(result.graph.Edges)
	}
	fixtures := []struct {
		body string
		want int
	}{
		{`class Service { public: virtual void load(){} }; void run(Service *s){s->load();}`, 0},
		{`void load(){} void run(){auto load=[](){load();};load();}`, 0},
		{`void load(){} void run(){auto callback=[](){load();};}`, 0},
		{`class Service { public: void load(){} }; void run(){ { Service source; source.load(); } source.load(); }`, 1},
	}
	for _, f := range fixtures {
		result := nativeFixture("cpp", map[string]string{"main.cpp": f.body})
		if len(result.graph.Edges) != f.want {
			t.Fatal("unexpected dynamic/lambda/scope binding", f.body, result.graph.Edges)
		}
	}
}

func TestCFamilyDeepPrototypeTypesMustMatch(t *testing.T) {
	result := nativeFixture("cpp", map[string]string{"main.cpp": "#include \"value.h\"\nvoid run(){value(1);}", "value.h": "void value(int count);", "value.cpp": "void value(double count){}"})
	if len(result.graph.Edges) != 0 {
		t.Fatal("mismatched prototype bound", result.graph.Edges)
	}
}

func TestObjectiveCDeepImportedSelectorsPropertiesNestedMessagesAndSuper(t *testing.T) {
	result := nativeFixture("objectivec", map[string]string{
		"Service.h": `@interface Service
 + (instancetype)shared;
 - (void)load;
 @end`,
		"Service.m": "#import \"Service.h\"\n@implementation Service\n+ (instancetype)shared { return nil; }\n- (void)load {}\n@end",
		"Client.h":  "#import \"Service.h\"\n@interface Client : Service\n@property(nonatomic) Service *service;\n@end",
		"Client.m":  "#import \"Client.h\"\n@implementation Client\n- (void)run:(Service *)source { [source load]; [self.service load]; [[Service shared] load]; [super load]; Service *local = source; [local load]; [unknown load]; }\n@end",
	})
	if len(result.graph.Edges) != 6 {
		t.Fatal(result.graph.Edges, result.facts.Declarations)
	}
	for _, e := range result.graph.Edges {
		if e.To.File != "Service.m" {
			t.Fatal(e)
		}
	}
}

func TestObjectiveCDeepCategoryCollisionsUnknownTypesAndBlocksStayOpen(t *testing.T) {
	result := nativeFixture("objectivec", map[string]string{
		"Service.h":  "@interface Service\n- (void)load;\n@end",
		"Service.m":  "#import \"Service.h\"\n@implementation Service\n- (void)load {}\n@end",
		"Category.m": "#import \"Service.h\"\n@implementation Service (Extra)\n- (void)load {}\n@end",
		"Client.m":   "#import \"Service.h\"\n@implementation Client\n- (void)run:(Service *)source { [source load]; }\n@end",
	})
	if len(result.graph.Edges) != 0 {
		t.Fatal("category collision guessed", result.graph.Edges)
	}
	for _, body := range []string{
		"@implementation Service\n- (void)load {}\n- (void)run:(id)source { [source load]; }\n@end",
		"@implementation Service\n- (void)load {}\n- (void)run { void (^callback)(void) = ^{[self load];}; }\n@end",
		"@implementation Service\n- (void)load {}\n- (void)run { { Service *local=nil; } [local load]; }\n@end",
	} {
		r := nativeFixture("objectivec", map[string]string{"Service.m": body})
		if len(r.graph.Edges) != 0 {
			t.Fatal(body, r.graph.Edges)
		}
	}
}

func TestRubyDeepImplicitSingletonSuffixAndEndlessMethods(t *testing.T) {
	result := nativeFixture("ruby", map[string]string{"service.rb": `class Service
 def prepare()
 end
 def ready?()
 end
 def run()
  prepare
  ready?()
 end
 class << self
  def check()
   self.configure()
  end
  def configure()
  end
 end
 def answer = 42
end`})
	if len(result.graph.Edges) != 3 {
		t.Fatal(result.graph.Edges, result.facts.Declarations)
	}
	found := false
	for _, f := range result.code.Functions {
		if f.Name == "answer" {
			found = true
		}
	}
	if !found {
		t.Fatal("endless method missing")
	}
}

func TestRubyDeepRelativeImportsMixinsInheritanceAndIsolation(t *testing.T) {
	result := nativeFixture("ruby", map[string]string{
		"client.rb":    "require_relative 'library'\nclass Client < Base\n include Helpers\n def run()\n  prepare()\n  inherited()\n end\nend\n",
		"library.rb":   "module Helpers\n def prepare()\n end\nend\nclass Base\n def inherited()\n end\nend\n",
		"unrelated.rb": "module Helpers\n def prepare()\n end\nend\n",
	})
	if len(result.graph.Edges) != 2 {
		t.Fatal(result.graph.Edges, result.facts.References)
	}
	for _, e := range result.graph.Edges {
		if e.To.File != "library.rb" {
			t.Fatal(e)
		}
	}
	imports := 0
	for _, ref := range result.facts.References {
		if ref.Type == "imports_ruby" && ref.Internal && ref.To == "library.rb" {
			imports++
		}
	}
	if imports != 1 {
		t.Fatal(result.facts.References)
	}
}

func TestRubyDeepReopeningDynamicImportsAndShadowingStayOpen(t *testing.T) {
	for _, body := range []string{
		"class Service\n def load()\n end\n def load()\n end\n def run()\n self.load()\n end\nend\n",
		"class Service\n def load()\n end\n alias_method :load, :other\n def run()\n self.load()\n end\nend\n",
		"class Service\n def load()\n end\n def run(load)\n load\n end\nend\n",
		"class Service\n def load()\n end\n def run()\n load = value\n load\n unknown.load()\n end\nend\n",
	} {
		r := nativeFixture("ruby", map[string]string{"service.rb": body})
		if len(r.graph.Edges) != 0 {
			t.Fatal(body, r.graph.Edges)
		}
	}
	r := nativeFixture("ruby", map[string]string{"client.rb": "require_relative \"#{folder}/library\"\nclass Client\n def run()\n Base.load()\n end\nend\n", "library.rb": "class Base\n def self.load()\n end\nend\n"})
	if len(r.graph.Edges) != 0 {
		t.Fatal("dynamic require guessed", r.graph.Edges)
	}
}

func TestNativeDeepCanonicalHeaderGuards(t *testing.T) {
	for _, language := range []string{"cpp", "objectivec"} {
		guarded := "#ifndef SERVICE_H\n#define SERVICE_H\n"
		if language == "cpp" {
			guarded += "void load();\n"
		} else {
			guarded += "@interface Service\n- (void)load;\n@end\n"
		}
		guarded += "#endif\n"
		if !nativePreprocessorSafe(guarded) {
			t.Fatal("canonical guard rejected")
		}
		for _, prefix := range []string{"#define FEATURE 1\n", "#if FEATURE\n", "#pragma pack(1)\n"} {
			if nativePreprocessorSafe(prefix + guarded) {
				t.Fatal("unproven preprocessor accepted", prefix)
			}
		}
	}
	r := nativeFixture("cpp", map[string]string{"main.cpp": "#include \"value.h\"\nvoid run(){load();}", "value.h": "#ifndef VALUE_H\n#define VALUE_H\nvoid load();\n#endif\n", "value.cpp": "void load(){}"})
	if len(r.graph.Edges) != 1 {
		t.Fatal(r.graph.Edges)
	}
}

func TestCFamilyDeepTemplatesRemainOpen(t *testing.T) {
	r := nativeFixture("cpp", map[string]string{"main.cpp": `template<typename T> class Service { public: void load(){} }; void run(Service<int> &s){s.load();}`})
	if len(r.graph.Edges) != 0 {
		t.Fatal(r.graph.Edges)
	}
}

func TestCFamilyDeepLocalDeclarationsAndAnonymousNamespacesDoNotLeak(t *testing.T) {
	for _, bodies := range []map[string]string{
		{"main.cpp": `void load(){} class Service { public: void load(); void run(){load();} };`},
		{"main.cpp": "#include \"value.h\"\nvoid run(){load();}", "value.h": "void load();", "value.cpp": `namespace {void load(){}}`},
	} {
		r := nativeFixture("cpp", bodies)
		if len(r.graph.Edges) != 0 {
			t.Fatal(r.graph.Edges)
		}
	}
}

func TestRubyDeepNamespacedClassCalls(t *testing.T) {
	r := nativeFixture("ruby", map[string]string{"main.rb": "module Outer\n class Service\n  def self.load()\n  end\n end\nend\nclass Client\n def run()\n  Outer::Service.load()\n end\nend\n"})
	if len(r.graph.Edges) != 1 || r.graph.Edges[0].To.Owner != "Outer::Service" {
		t.Fatal(r.graph.Edges)
	}
}

func TestNativeDeepUnitTestMappingsRequireFrameworkEvidence(t *testing.T) {
	fixtures := []struct{ language, file, body string }{
		{"cpp", "service_test.cpp", "#include <gtest/gtest.h>\nvoid load(){}\nTEST(ServiceTest, Loads){load();}\n"},
		{"objectivec", "ServiceTests.m", "#import <XCTest/XCTest.h>\n@interface ServiceTests : XCTestCase\n@end\n@implementation ServiceTests\n- (void)load {}\n- (void)testLoads { [self load]; }\n@end"},
		{"ruby", "service_test.rb", "require 'minitest/autorun'\nclass ServiceTest < Minitest::Test\n def load()\n end\n def test_loads()\n  load()\n end\nend\n"},
	}
	for _, f := range fixtures {
		t.Run(f.language, func(t *testing.T) {
			r := nativeFixture(f.language, map[string]string{f.file: f.body})
			if len(r.tests) != 1 || r.tests[0].TargetMethod != "load" {
				t.Fatal(r.tests, r.graph.Edges, r.code.Functions)
			}
			if len(r.capabilities) != 1 || r.capabilities[0].Capability != CapabilityTests {
				t.Fatal(r.capabilities)
			}
		})
	}
	for _, f := range []struct{ language, body string }{
		{"cpp", "void load(){} TEST(ServiceTest, Loads){load();}"},
		{"objectivec", "@implementation ServiceTests\n- (void)load {}\n- (void)testLoads { [self load]; }\n@end"},
		{"ruby", "class ServiceTest\n def load()\n end\n def test_loads()\n load()\n end\nend\n"},
	} {
		r := nativeFixture(f.language, map[string]string{"test": f.body})
		if len(r.tests) != 0 {
			t.Fatal("unproven test registration", r.tests)
		}
	}
}

func TestNativeDeepDirectiveCommentsAndRawTextDoNotGrantVisibility(t *testing.T) {
	guard := "/* License */\n#ifndef HEADER_H // guard\n#define HEADER_H\nvoid load();\n#endif // guard\n"
	if !nativePreprocessorSafe(guard) {
		t.Fatal("commented guard rejected")
	}
	for _, fake := range []string{"/*\n#include \"value.h\"\n*/\n", "const char *text=R\"(\n#include \"value.h\"\n)\";\n"} {
		r := nativeFixture("cpp", map[string]string{"main.cpp": fake + "void run(){load();}", "value.h": "void load();", "value.cpp": "void load(){}"})
		if len(r.graph.Edges) != 0 {
			t.Fatal("opaque include granted visibility", r.graph.Edges)
		}
	}
}

func TestNativeDeepDeferredTypedLambdaAndMetaprogrammingStayOpen(t *testing.T) {
	cpp := nativeFixture("cpp", map[string]string{"main.cpp": `void load(){} void run(){auto callback=[]() -> int { load(); return 1; };}`})
	if len(cpp.graph.Edges) != 0 {
		t.Fatal("deferred lambda assigned to caller", cpp.graph.Edges)
	}
	for _, body := range []string{
		"class Service\n def self.load()\n end\nend\nService = other\nclass Runner\n def run()\n Service.load()\n end\nend\n",
		"class Service\n def load()\n end\n def run()\n self.load()\n end\nend\nService.class_eval do\n define_method(:load) {}\nend\n",
	} {
		r := nativeFixture("ruby", map[string]string{"main.rb": body})
		if len(r.graph.Edges) != 0 {
			t.Fatal("dynamic Ruby mutation guessed", r.graph.Edges)
		}
	}
}

func TestRubyDeepUnparenthesizedParametersDoNotBecomeCalls(t *testing.T) {
	r := nativeFixture("ruby", map[string]string{"main.rb": "class Service\n def load()\n end\n def run load\n  load\n end\nend\n"})
	if len(r.graph.Edges) != 0 {
		t.Fatal("parameter treated as implicit method", r.graph.Edges)
	}
}

func TestNativeDeepCyclicImportsAndConflictingMixinsTerminateSafely(t *testing.T) {
	r := nativeFixture("ruby", map[string]string{
		"main.rb":    "require_relative 'library'\nclass Client\n include First\n include Second\n def run()\n load()\n end\nend\n",
		"library.rb": "require_relative 'main'\nmodule First\n def load()\n end\nend\nmodule Second\n def load()\n end\nend\n",
	})
	if len(r.graph.Edges) != 0 {
		t.Fatal("conflicting mixins guessed", r.graph.Edges)
	}
	c := nativeFixture("cpp", map[string]string{"main.cpp": "#include \"a.h\"\nvoid run(){load();}", "a.h": "#include \"b.h\"\nvoid load();", "b.h": "#include \"a.h\"\n", "value.cpp": "void load(){}"})
	if len(c.graph.Edges) != 1 {
		t.Fatal(c.graph.Edges)
	}
}

func TestObjectiveCDeepCFunctionPointerShadowStaysOpen(t *testing.T) {
	r := nativeFixture("objectivec", map[string]string{"Bridge.m": "void ready(void){}\nvoid run(void (*ready)(void)){ready();}"})
	if len(r.graph.Edges) != 0 {
		t.Fatal("function pointer bound to C entrypoint", r.graph.Edges)
	}
}

func TestRubyDeepConditionalIncludeDoesNotEstablishMethodTarget(t *testing.T) {
	r := nativeFixture("ruby", map[string]string{"main.rb": "module Helpers\n def load()\n end\nend\nclass Service\n if ready\n include Helpers\n end\n def run()\n load()\n end\nend\n"})
	if len(r.graph.Edges) != 0 {
		t.Fatal("conditional mixin guessed", r.graph.Edges)
	}
}
