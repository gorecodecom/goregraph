package agent

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveCoreBoundariesKeepEverySelectedLocalCall(t *testing.T) {
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "entry", Project: "billing", Kind: "backend_handler", File: "Controller.java"},
		{ID: "helper", Project: "billing", Kind: "symbol", File: "Controller.java"},
		{ID: "service", Project: "billing", Kind: "symbol", File: "Service.java"},
		{ID: "store", Project: "billing", Kind: "symbol", File: "Store.java"},
		{ID: "unrelated", Project: "billing", Kind: "symbol", File: "Unrelated.java"},
		{ID: "test", Project: "billing", Kind: "symbol", File: "src/test/java/ServiceTest.java"},
		{ID: "remote", Project: "shipping", Kind: "symbol", File: "Remote.java"},
	}, Edges: []scan.AgentContextEdgeRecord{
		{FromFactID: "entry", ToFactID: "helper", Kind: "call"},
		{FromFactID: "helper", ToFactID: "service", Kind: "call"},
		{FromFactID: "service", ToFactID: "store", Kind: "call"},
		{FromFactID: "store", ToFactID: "helper", Kind: "call"},
		{FromFactID: "unrelated", ToFactID: "unrelated", Kind: "call"},
		{FromFactID: "entry", ToFactID: "unrelated", Kind: "configuration"},
		{FromFactID: "entry", ToFactID: "unrelated", Kind: "use"},
		{FromFactID: "entry", ToFactID: "test", Kind: "call"},
		{FromFactID: "entry", ToFactID: "remote", Kind: "call"},
	}}
	pack := ContextPack{ProtocolVersion: AdaptiveV2, Entrypoints: []ContextLocation{{ID: "entry"}}, selectedSourceFactIDs: []string{"store", "unrelated", "remote", "helper", "entry", "test", "service"}}
	distances := map[string]int{"entry": 0, "helper": 1, "service": 2, "store": 3, "unrelated": 1, "remote": 1, "test": 1}
	got := contextCoreSourceBoundariesWithRelated(pack, index, distances, false)
	want := []contextSourceBoundary{{factID: "entry"}, {factID: "helper"}, {factID: "service"}, {factID: "store"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("local chain = %+v, want %+v", got, want)
	}
	pack.ProtocolVersion = StrictV1
	strict := contextCoreSourceBoundariesWithRelated(pack, index, distances, false)
	if len(strict) != 2 {
		t.Fatalf("strict first-local-call behavior changed: %+v", strict)
	}
}

func TestAdaptiveContextRendersLocalChainThroughPersistence(t *testing.T) {
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{}
	pack := ContextPack{Schema: 1, ProtocolVersion: AdaptiveV2, Query: "DELETE /invoices/{id}: show call chain and persistence with exact production and configuration files.", Confidence: "HIGH", BudgetTokens: 4000,
		Entrypoints: []ContextLocation{{ID: "entry", File: "Controller.java", Line: 2}},
		Files:       []ContextFile{{Path: "Controller.java", Role: "entrypoint"}},
		Concerns:    []ContextConcern{{Kind: contextConcernEntrypoint}, {Kind: contextConcernPrimaryPath}, {Kind: contextConcernPersistence}},
	}
	ids := []string{"entry", "service", "store"}
	files := []string{"Controller.java", "Service.java", "Store.java"}
	bodies := []string{"service.remove(id);", "store.remove(id);", "repository.deleteById(id);"}
	for i, id := range ids {
		owner := strings.TrimSuffix(files[i], ".java")
		writeSourceFile(t, root, files[i], fmt.Sprintf("class %s {\n public void remove(String id) {\n  %s\n }\n}\n", owner, bodies[i]))
		fact := scan.AgentContextFactRecord{ID: id, Kind: "symbol", Name: "remove", Qualified: owner + ".remove", File: files[i], Line: 2, EndLine: 4, Confidence: "EXACT"}
		if i == 0 {
			fact.Kind, fact.HTTPMethod, fact.Path = "route", "DELETE", "/invoices/{id}"
		}
		if i == len(ids)-1 {
			fact.Kind = "persistence"
		}
		index.Facts = append(index.Facts, fact)
		pack.selectedSourceFactIDs = append(pack.selectedSourceFactIDs, id)
		if i > 0 {
			kind := "call"
			if fact.Kind == "persistence" {
				kind = "persistence"
			}
			index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{ID: ids[i-1] + id, FromFactID: ids[i-1], ToFactID: id, Kind: kind, Reason: "flow", Confidence: "EXACT"})
		}
	}
	got, err := attachContextSource(pack, loadedContextIndex{ScopeRoot: root, Index: index}, ContextRequest{BudgetTokens: 4000, MaxFiles: 3})
	if err != nil {
		t.Fatal(err)
	}
	for i, path := range files {
		found := false
		for _, section := range got.SourceSections {
			found = found || section.Path == path && strings.Contains(section.Content, bodies[i])
		}
		if !found {
			t.Errorf("missing executable chain evidence in %s: %+v", path, got.SourceSections)
		}
	}
	if got.EstimatedTokens > 4000 || contextSourceFileCount(got) > 3 {
		t.Fatalf("selection exceeded caller limits: %+v", got)
	}
}
