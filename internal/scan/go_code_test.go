package scan

import "testing"

func TestGoCallsRespectImportsScopesAndArity(t *testing.T) {
	files := map[string]string{
		"internal/watch/watch.go": `package watch
type Worker struct{}
func Resolve(path string, workspace bool) {}
func (w *Worker) Resolve() {}
func Many(values ...int) {}
`,
		"internal/pathutil/resolve.go": `package pathutil
func Resolve(path string) {}
`,
		"version/v2/util.go": `package useful
func Resolve(path string, workspace bool) {}
`,
		"internal/cli/cli.go": `package cli
import (
 observer "example.test/app/internal/watch"
 "example.test/app/version/v2"
 external "external.test/other/internal/watch"
)
type Worker struct{}
func (w Worker) Resolve() {}
func Local() {}
func PackageCaller() { observer.Resolve("path", true) }
func DefaultPackageCaller() { useful.Resolve("path", true) }
func WrongArityCaller() { observer.Resolve("path") }
func ExternalCaller() { external.Resolve("path", true) }
func ShadowCaller() { observer := Worker{}; observer.Resolve() }
func ParameterCaller(worker *observer.Worker) { worker.Resolve() }
func ExpressionCaller(worker *observer.Worker) { (*observer.Worker).Resolve(worker) }
func UnknownCaller(worker interface{ Resolve() }) { worker.Resolve() }
func LocalCaller() { Local() }
func VariableCaller(Local func()) { Local() }
func VariadicCaller(values []int) { observer.Many(); observer.Many(1, 2); observer.Many(values...) }
func CommentsCaller() { /* observer.Resolve("path", true) */; _ = "observer.Resolve(path, true)" }
`,
		"nested/main.go": `package cli
func Local() {}
func NestedCaller() { Local() }
`,
		"internal/cli/external_test.go": `package cli_test
func Local() {}
func ExternalTestCaller() { Local() }
`,
		"internal/dotted/call.go": `package dotted
import . "example.test/app/internal/watch"
func DotCaller() { Resolve("path", true) }
`,
	}
	var code CodeIntelligenceRecord
	for file, body := range files {
		functions, scope := extractGoCodeIntelligence(FileRecord{Path: file, Language: "go"}, body)
		if len(functions) == 0 {
			t.Fatalf("no parsed functions in %s", file)
		}
		code.Functions = append(code.Functions, functions...)
		code.goFiles = append(code.goFiles, scope)
	}
	bindGoCodePackages(&code, []SymbolRecord{{Kind: "module", Name: "module example.test/app", File: "go.mod"}})
	graph := buildGenericCallGraph(code)
	expected := map[string]string{
		"PackageCaller":        "internal/watch/watch.go",
		"DefaultPackageCaller": "version/v2/util.go",
		"ShadowCaller":         "internal/cli/cli.go",
		"ParameterCaller":      "internal/watch/watch.go",
		"ExpressionCaller":     "internal/watch/watch.go",
		"LocalCaller":          "internal/cli/cli.go",
		"NestedCaller":         "nested/main.go",
		"ExternalTestCaller":   "internal/cli/external_test.go",
		"DotCaller":            "internal/watch/watch.go",
		"VariadicCaller":       "internal/watch/watch.go",
	}
	counts := map[string]int{}
	identities := map[string]bool{}
	for _, edge := range graph.Edges {
		if identities[edge.ID] {
			t.Fatalf("duplicate Go call identity: %s", edge.ID)
		}
		identities[edge.ID] = true
		counts[edge.From.Method]++
		if expected[edge.From.Method] != edge.To.File {
			t.Fatalf("wrong Go callee: %+v", edge)
		}
	}
	for caller := range expected {
		want := 1
		if caller == "VariadicCaller" {
			want = 3
		}
		if counts[caller] != want {
			t.Errorf("%s: got %d calls, want %d", caller, counts[caller], want)
		}
	}
	for _, caller := range []string{"WrongArityCaller", "ExternalCaller", "UnknownCaller", "VariableCaller", "CommentsCaller"} {
		if counts[caller] != 0 {
			t.Errorf("%s was guessed into an internal call", caller)
		}
	}
}

func TestGoCallsDoNotCrossNestedModulesOrAmbiguousDefinitions(t *testing.T) {
	var code CodeIntelligenceRecord
	for file, body := range map[string]string{
		"caller/main.go": `package caller
import "example.test/app/local"
func Caller() { local.Resolve() }
func Ambiguous() { Duplicate() }
func Duplicate() {}
`,
		"caller/other.go": `package caller
func Duplicate() {}
`,
		"local/main.go": `package local
func Resolve() {}
`,
	} {
		functions, scope := extractGoCodeIntelligence(FileRecord{Path: file, Language: "go"}, body)
		code.Functions = append(code.Functions, functions...)
		code.goFiles = append(code.goFiles, scope)
	}
	bindGoCodePackages(&code, []SymbolRecord{
		{Kind: "module", Name: "module example.test/app", File: "go.mod"},
		{Kind: "module", Name: "module external.test/module", File: "local/go.mod"},
	})
	if graph := buildGenericCallGraph(code); len(graph.Edges) != 0 {
		t.Fatalf("unsupported or ambiguous Go targets were guessed: %+v", graph.Edges)
	}
}

func TestGoCallsFollowDeclaredFieldsAndInterfacesAcrossFiles(t *testing.T) {
	var code CodeIntelligenceRecord
	for file, body := range map[string]string{
		"contracts/contract.go": `package contracts
type Repository interface { Resolve(path string) }
type SQLRepository struct{}
func (r *SQLRepository) Resolve(path string) {}
`,
		"internal/service/types.go": `package service
import declared "example.test/app/contracts"
type Service struct { repository declared.Repository }
var activeService *Service
`,
		"internal/service/calls.go": `package service
func FieldCaller(service *Service) { service.repository.Resolve("path") }
func GlobalCaller() { activeService.repository.Resolve("path") }
func UnknownFieldCaller(service interface{ Resolve(string) }) { service.Resolve("path") }
`,
		"unrelated/util.go": `package unrelated
func Resolve(path string) {}
`,
	} {
		functions, scope := extractGoCodeIntelligence(FileRecord{Path: file, Language: "go"}, body)
		code.Functions = append(code.Functions, functions...)
		code.goFiles = append(code.goFiles, scope)
	}
	bindGoCodePackages(&code, []SymbolRecord{{Kind: "module", Name: "module example.test/app", File: "go.mod"}})
	graph := buildGenericCallGraph(code)
	counts := map[string]int{}
	for _, edge := range graph.Edges {
		counts[edge.From.Method]++
		if edge.To.Owner != "Repository" || edge.To.File != "contracts/contract.go" || edge.Confidence != "MATCHED" {
			t.Fatalf("interface binding was replaced with a concrete runtime guess: %+v", edge)
		}
	}
	if counts["FieldCaller"] != 1 || counts["GlobalCaller"] != 1 || counts["UnknownFieldCaller"] != 0 {
		t.Fatalf("typed fields or global receiver scopes were lost: %+v", counts)
	}
}
