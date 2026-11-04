package agent

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestScopeProductionIndexKeepsExplicitAndLinkedEvidence(t *testing.T) {
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		Facts: []scan.AgentContextFactRecord{
			{ID: "local", Project: "apps/keeper"},
			{ID: "linked", Project: "apps/provider"},
			{ID: "unrelated-provider", Project: "apps/provider"},
			{ID: "noise", Project: "apps/neighbor"},
			{ID: "inferred", Project: "apps/other"},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "resolved", FromFactID: "local", ToFactID: "linked", Kind: "http_contract", Confidence: "RESOLVED"},
			{ID: "guess", FromFactID: "local", ToFactID: "inferred", Kind: "call", Confidence: "INFERRED"},
		},
	}
	loaded := loadedContextIndex{Index: index, Workspace: true, ScopeRoot: root}
	for _, test := range []struct {
		query string
		want  []string
	}{
		{"Trace the write route", []string{"local", "linked"}},
		{"Compare the write route with apps/neighbor", []string{"local", "linked", "noise"}},
	} {
		got, err := scopeProductionIndex(loaded, ContextRequest{Root: filepath.Join(root, "apps", "keeper"), Query: test.query})
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, fact := range got.Index.Facts {
			ids = append(ids, fact.ID)
		}
		if !slices.Equal(ids, test.want) || len(got.Index.Edges) != 1 || got.Index.Edges[0].ID != "resolved" {
			t.Fatalf("%q: facts=%v, edges=%v", test.query, ids, got.Index.Edges)
		}
	}
}
