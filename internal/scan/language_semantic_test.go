package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

func TestSemanticReportReplacesOnlyVerifiedSourceCalls(t *testing.T) {
	body := "class Service { void Target(){} void Run(){ Target(); } }"
	file := fileRecord("Service.cs", int64(len(body)), []byte(body))
	column := strings.Index(body, "Target();") + 1
	report := languageSemanticReport{SchemaVersion: 1, Language: "csharp", Producer: "Roslyn fixture", Inputs: map[string]string{file.Path: file.Hash}, CoveredFiles: []string{file.Path}, Declarations: []semanticDeclaration{
		{USR: "target", Name: "Target", Kind: "method", File: file.Path, Line: 1, Column: strings.Index(body, "Target()") + 1},
		{USR: "run", Name: "Run", Kind: "method", File: file.Path, Line: 1, Column: strings.Index(body, "Run()") + 1},
	}, References: []semanticReference{{USR: "target", Caller: "run", Name: "Target", File: file.Path, Line: 1, Column: column, Kind: "calls_method_owner"}}}
	index := Index{Files: []FileRecord{file}, CSharp: analyzeCSharpProject([]csharpSource{parseCSharpSource(file, body)})}
	original := len(index.CSharp.graph.Edges)
	if original != 1 {
		t.Fatal(index.CSharp.graph)
	}
	data, _ := json.Marshal(report)
	applyLanguageSemanticSources(&index, []assetExportSource{{file: FileRecord{Path: "Report.goregraph-csharp.json"}, body: string(data)}}, map[string]string{file.Path: body})
	if len(index.AnalysisIssues) != 0 || len(index.CSharp.graph.Edges) != 1 || !strings.Contains(index.CSharp.graph.Edges[0].Reason, "compiler symbol") {
		t.Fatalf("%#v %#v", index.AnalysisIssues, index.CSharp.graph)
	}
	for _, change := range []string{"stale", "added_source", "bad_location", "metadata", "duplicate", "wrong_language"} {
		t.Run(change, func(t *testing.T) {
			copy := index
			copy.CSharp = analyzeCSharpProject([]csharpSource{parseCSharpSource(file, body)})
			var candidate languageSemanticReport
			json.Unmarshal(data, &candidate)
			sources := []assetExportSource{{file: FileRecord{Path: "Report.goregraph-csharp.json"}}}
			switch change {
			case "stale":
				candidate.Inputs[file.Path] = "old"
			case "added_source":
				copy.Files = append(append([]FileRecord{}, copy.Files...), fileRecord("Other.cs", 1, []byte("x")))
			case "metadata":
				copy.Files = append(append([]FileRecord{}, copy.Files...), fileRecord("Game.csproj", 1, []byte("x")))
			case "bad_location":
				candidate.References[0].Column = 1
			case "duplicate":
				sources = append(sources, assetExportSource{file: FileRecord{Path: "Other.goregraph-csharp.json"}})
			case "wrong_language":
				candidate.Language = "swift"
			}
			encoded, _ := json.Marshal(candidate)
			for i := range sources {
				sources[i].body = string(encoded)
			}
			applyLanguageSemanticSources(&copy, sources, map[string]string{file.Path: body})
			if len(copy.AnalysisIssues) == 0 || len(copy.CSharp.graph.Edges) != original || strings.Contains(copy.CSharp.graph.Edges[0].Reason, "compiler symbol") {
				t.Fatal(copy.AnalysisIssues, copy.CSharp.graph)
			}
		})
	}
}

func TestSemanticReferencesDoNotTurnMethodValuesIntoCalls(t *testing.T) {
	facts := ProjectSymbolFacts{}
	report := languageSemanticReport{Language: "swift", CoveredFiles: []string{"File.swift"}, Declarations: []semanticDeclaration{{USR: "target", Name: "target", Kind: "function", File: "File.swift", Line: 1}, {USR: "caller", Name: "caller", Kind: "function", File: "File.swift", Line: 2}}, References: []semanticReference{{USR: "target", Caller: "caller", Name: "target", File: "File.swift", Line: 3, Kind: "uses_symbol"}}}
	graph := CallGraphRecord{}
	applySemanticReport(report, &facts, &graph)
	if len(graph.Edges) != 0 || len(facts.References) != 1 || facts.References[0].Resolution != SymbolResolutionExact {
		t.Fatal(facts, graph)
	}
}

func TestSemanticLocationsCannotUseAnIdentifierPrefix(t *testing.T) {
	body := "class Service {void Runner(){}}"
	file := fileRecord("Service.cs", int64(len(body)), []byte(body))
	report := languageSemanticReport{SchemaVersion: 1, Language: "csharp", Producer: "fixture", Inputs: map[string]string{file.Path: file.Hash}, CoveredFiles: []string{file.Path}, Declarations: []semanticDeclaration{{USR: "run", Name: "Run", Kind: "method", File: file.Path, Line: 1, Column: strings.Index(body, "Runner") + 1}}}
	if err := validateSemanticReport(report, []FileRecord{file}, map[string]string{file.Path: body}); err == nil {
		t.Fatal("prefix was accepted as a declaration")
	}
}

func TestSemanticSameLineOverloadsCannotShareAStaticIdentity(t *testing.T) {
	facts := ProjectSymbolFacts{Declarations: []RichSymbolRecord{{ID: "one-static-symbol", File: "File.cs", Line: 1, Name: "Pick", Kind: "method"}}}
	report := languageSemanticReport{Language: "csharp", Declarations: []semanticDeclaration{
		{USR: "Pick(int)", File: "File.cs", Line: 1, Column: 10, Name: "Pick", Kind: "method"},
		{USR: "Pick(string)", File: "File.cs", Line: 1, Column: 40, Name: "Pick", Kind: "method"},
	}, References: []semanticReference{{USR: "Pick(int)", File: "File.cs", Line: 2, Column: 1, Kind: "uses_symbol"}, {USR: "Pick(string)", File: "File.cs", Line: 3, Column: 1, Kind: "uses_symbol"}}}
	applySemanticReport(report, &facts, &CallGraphRecord{})
	if len(facts.References) != 2 || facts.References[0].ToSymbolID == facts.References[1].ToSymbolID {
		t.Fatal("compiler overload identities were collapsed", facts)
	}
}

func TestCompilerTestMapsReplaceCoveredStaticTargetsOnly(t *testing.T) {
	existing := []TestMapRecord{{TestFile: "Tests.cs", TargetMethod: "Wrong"}, {TestFile: "Other.cs", TargetMethod: "Retained"}}
	code := CodeIntelligenceRecord{Functions: []CodeFunctionRecord{{Kind: "test", File: "Tests.cs", Line: 2, Name: "Check"}}}
	graph := CallGraphRecord{Edges: []CallGraphEdgeRecord{{SourceFile: "Tests.cs", From: MethodRefRecord{File: "Tests.cs", Line: 2, Method: "Check"}, To: MethodRefRecord{File: "Service.cs", Method: "Right"}}}}
	result := semanticTestMaps(languageSemanticReport{CoveredFiles: []string{"Tests.cs"}}, existing, code, graph)
	if len(result) != 2 || result[0].TargetMethod != "Retained" || result[1].TargetMethod != "Right" || result[1].Type != "compiler-call" {
		t.Fatal(result)
	}
}

// Reports produced by real compilers are supplied explicitly, never generated here.
func TestRealSemanticReports(t *testing.T) {
	base := os.Getenv("GOREGRAPH_SEMANTIC_SMOKE_ROOT")
	if base == "" {
		t.Skip("real semantic reports not explicitly supplied")
	}
	for _, language := range []string{"csharp", "swift"} {
		t.Run(language, func(t *testing.T) {
			root := filepath.Join(base, language)
			index, _, err := scanProject(root, config.Defaults(), gitignore.Matcher{})
			if err != nil {
				t.Fatal(err)
			}
			if len(index.AnalysisIssues) != 0 {
				t.Fatal(index.AnalysisIssues)
			}
			graph := index.CSharp.graph
			if language == "swift" {
				graph = index.Swift.graph
			}
			if len(graph.Edges) < 2 {
				t.Fatalf("missing generic/extension compiler bindings: %#v", graph)
			}
			for _, edge := range graph.Edges {
				if !strings.Contains(edge.Reason, "compiler symbol") {
					t.Fatal(edge)
				}
			}
			if language == "csharp" {
				operator, constructor := false, false
				for _, edge := range graph.Edges {
					operator = operator || strings.Contains(edge.TargetQualifiedName, "op_Addition")
					constructor = constructor || edge.To.Method == "Counter"
					if edge.To.Method == "!" {
						t.Fatal("null suppression was interpreted as a call", edge)
					}
				}
				if !operator || !constructor {
					t.Fatal("record constructor or overloaded operator binding absent", graph)
				}
			}
		})
	}
}
