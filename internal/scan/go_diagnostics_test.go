package scan

import (
	"reflect"
	"testing"
)

func TestGoUnresolvedCallsExplainReasonsWithoutInventingEdges(t *testing.T) {
	files := map[string]string{
		"api/api.go": "package api\nfunc Save(value string) {}\n",
		"app/main.go": `package app
import (
 "example.test/diag/api"
 external "external.test/library"
)
type Value struct{}
type Alias = Value
func (v Value) Save() {}
func Local() {}
func Known() { Local(); api.Save("value") }
func FunctionValue(run func()) { run() }
func Receiver(worker interface{ Save() }) { worker.Save() }
func AliasCall(worker Alias) { worker.Save() }
func WrongArguments() { api.Save() }
func External() { external.Save() }
func Duplicate() {}
func Duplicate() {}
func Ambiguous() { Duplicate() }
func Conversion(value int) { _ = Value{}; _ = int(value) }
`,
	}
	var code CodeIntelligenceRecord
	for file, body := range files {
		functions, scope := extractGoCodeIntelligence(FileRecord{Path: file, Language: "go"}, body)
		code.Functions = append(code.Functions, functions...)
		code.goFiles = append(code.goFiles, scope)
	}
	bindGoCodePackages(&code, []SymbolRecord{{Kind: "module", Name: "module example.test/diag", File: "go.mod"}})
	graph := buildGenericCallGraph(code)
	expected := map[string]string{"FunctionValue": "dynamic_function_value", "Receiver": "receiver_type_unresolved", "AliasCall": "unsupported_type_binding", "WrongArguments": "argument_mismatch", "External": "external_target", "Ambiguous": "ambiguous_target"}
	for _, diagnostic := range graph.UnresolvedCalls {
		if expected[diagnostic.Caller] != diagnostic.Reason || diagnostic.File != "app/main.go" || diagnostic.Line < 1 {
			t.Fatalf("unexpected diagnostic: %+v", diagnostic)
		}
		delete(expected, diagnostic.Caller)
	}
	if len(expected) > 0 {
		t.Fatalf("missing explanations: %v", expected)
	}
	for _, edge := range graph.Edges {
		if edge.From.Method != "Known" || (edge.To.Method != "Local" && edge.To.Method != "Save") {
			t.Fatalf("invented call edge: %+v", edge)
		}
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("known edges changed: %+v", graph.Edges)
	}
	merged := mergeCallGraphs(graph, graph)
	if !reflect.DeepEqual(merged.UnresolvedCalls, graph.UnresolvedCalls) {
		t.Fatal("diagnostics lost or duplicated in graph merge")
	}
}

func TestWorkspaceCallDiagnosticsStayWithIndexedProjects(t *testing.T) {
	registry := WorkspaceRegistryRecord{Root: "workspace", Projects: []WorkspaceProjectRecord{{Path: "app", Indexed: true}, {Path: "excluded", Indexed: false}}}
	diagnostic := GoCallDiagnosticRecord{Project: "old", File: "main.go", Line: 8, Caller: "Run", Method: "Save", Reason: "dynamic_function_value"}
	indexes := []AgentContextIndexRecord{{Root: "app", CallDiagnostics: []GoCallDiagnosticRecord{diagnostic}}, {Root: "excluded", CallDiagnostics: []GoCallDiagnosticRecord{diagnostic}}}
	index := BuildWorkspaceAgentContextIndex(registry, indexes, nil, nil, WorkspaceEndpointTraceIndexRecord{}, APICatalogRecord{}, "")
	if len(index.CallDiagnostics) != 1 || index.CallDiagnostics[0].Project != "app" || index.CallDiagnostics[0].File != "main.go" {
		t.Fatalf("incorrect diagnostic ownership: %+v", index.CallDiagnostics)
	}
}
