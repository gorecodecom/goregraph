package agent

import (
	"reflect"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveCoreBoundariesIncludeTypedPersistenceFlow(t *testing.T) {
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "entry", Kind: "backend_handler", File: "Controller.java"},
		{ID: "service", Kind: "symbol", File: "Service.java"},
		{ID: "helper", Kind: "persistence", File: "Service.java"},
		{ID: "store", Kind: "persistence", File: "Store.java"},
		{ID: "related", Kind: "persistence", File: "Related.java"},
	}, Edges: []scan.AgentContextEdgeRecord{
		{FromFactID: "entry", ToFactID: "service", Kind: "call", Reason: "flow"},
		{FromFactID: "service", ToFactID: "helper", Kind: "persistence", Reason: "flow"},
		{FromFactID: "helper", ToFactID: "store", Kind: "persistence", Reason: "flow"},
		{FromFactID: "service", ToFactID: "related", Kind: "persistence", Reason: "shared model"},
	}}
	pack := ContextPack{ProtocolVersion: AdaptiveV2, Entrypoints: []ContextLocation{{ID: "entry"}}, selectedSourceFactIDs: []string{"related", "store", "helper", "service", "entry"}}
	distances := map[string]int{"entry": 0, "service": 1, "helper": 2, "store": 3, "related": 2}
	want := []contextSourceBoundary{{factID: "entry"}, {factID: "service"}, {factID: "helper"}, {factID: "store"}}
	if got := contextCoreSourceBoundariesWithRelated(pack, index, distances, false); !reflect.DeepEqual(got, want) {
		t.Fatalf("typed persistence chain = %+v, want %+v", got, want)
	}
	pack.ProtocolVersion = StrictV1
	want = want[:2]
	if got := contextCoreSourceBoundariesWithRelated(pack, index, distances, false); !reflect.DeepEqual(got, want) {
		t.Fatalf("strict chain changed: %+v, want %+v", got, want)
	}
}
