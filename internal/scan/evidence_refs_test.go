package scan

import (
	"encoding/json"
	"testing"
)

func TestLinkEvidenceReferencesConnectsRouteAndCallFacts(t *testing.T) {
	files := []FileRecord{{Path: "src/app.go", Language: "go", Hash: "hash"}}
	routes := []CodeRouteRecord{{Language: "go", Framework: "net/http", File: "src/app.go", Line: 4, Reason: "route registration"}}
	calls := CallGraphRecord{Edges: []CallGraphEdgeRecord{{SourceFile: "src/app.go", Line: 8, Reason: "resolved call"}}}
	evidence := LinkEvidenceReferences("demo", files, nil, &calls, routes, nil, nil)
	if len(evidence) != 2 {
		t.Fatalf("len(evidence) = %d, want 2", len(evidence))
	}
	if len(routes[0].EvidenceIDs) != 1 || len(calls.Edges[0].EvidenceIDs) != 1 {
		t.Fatalf("facts are missing evidence references: routes=%#v calls=%#v", routes, calls)
	}
	known := map[string]bool{}
	for _, record := range evidence {
		known[record.ID] = true
	}
	if !known[routes[0].EvidenceIDs[0]] || !known[calls.Edges[0].EvidenceIDs[0]] {
		t.Fatal("fact references do not resolve to evidence records")
	}
}

func TestLegacyRouteJSONRemainsCompatibleWithoutEvidenceIDs(t *testing.T) {
	var route CodeRouteRecord
	if err := json.Unmarshal([]byte(`{"language":"go","framework":"net/http","kind":"backend","http_method":"GET","path":"/users","file":"app.go","line":1,"confidence":"EXTRACTED"}`), &route); err != nil {
		t.Fatalf("legacy route JSON failed to decode: %v", err)
	}
	if len(route.EvidenceIDs) != 0 {
		t.Fatalf("legacy route unexpectedly contains evidence: %#v", route.EvidenceIDs)
	}
}
