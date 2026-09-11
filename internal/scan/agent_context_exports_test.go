package scan

import "testing"

func TestAgentProjectionKeepsExportedScriptFunctionsAndTheirCalls(t *testing.T) {
	for _, language := range []string{"typescript", "javascript", "tsx", "jsx"} {
		t.Run(language, func(t *testing.T) {
			symbols := []RichSymbolRecord{
				{ID: "submit", Name: "submitOrder", QualifiedName: "checkout#submitOrder", Kind: "function", Language: language, ExportName: "submitOrder", File: "checkout.ts", Line: 2},
				{ID: "save", Name: "saveOrder", QualifiedName: "api#saveOrder", Kind: "function", Language: language, ExportName: "saveOrder", File: "api.ts", Line: 1},
				{ID: "private", Name: "formatDebug", QualifiedName: "api#formatDebug", Kind: "function", Language: language, File: "api.ts", Line: 10},
			}
			relations := []RichRelationRecord{{ID: "call", FromSymbolID: "submit", ToSymbolID: "save", From: "checkout#submitOrder", To: "api#saveOrder", Type: "calls_function", Confidence: "EXACT"}}
			index := BuildProjectAgentContextIndex("store", "", nil, nil, symbols, relations, nil, nil, nil, nil)
			for _, name := range []string{"submitOrder", "saveOrder"} {
				if !hasContextFact(index.Facts, "symbol", name) {
					t.Errorf("exported function %s missing from agent projection", name)
				}
			}
			if hasContextFact(index.Facts, "symbol", "formatDebug") {
				t.Fatal("unrelated private function leaked into compact projection")
			}
			if len(index.Edges) != 1 || index.Edges[0].Kind != "call" {
				t.Fatalf("exported caller/provider edge missing: %#v", index.Edges)
			}
		})
	}
}
