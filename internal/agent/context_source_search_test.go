package agent

import (
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestContextSourceSearchFindsBusinessTermInVerifiedHandler(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "Billing.java", "class Billing {\n void deleteEntry() {\n  audit(\"Buchung\", \"entfernt\");\n }\n}\n")
	writeSourceFile(t, root, "Delivery.java", "class Delivery {\n void deleteEntry() {\n  audit(\"Lieferung\", \"entfernt\");\n }\n}\n")
	facts := []scan.AgentContextFactRecord{
		{ID: "billing", Kind: "api_endpoint", HTTPMethod: "DELETE", Path: "/billing/{id}", Name: "DELETE /billing/{id}", Qualified: "Billing.deleteEntry", File: "Billing.java", Line: 2, Confidence: "EXTRACTED"},
		{ID: "delivery", Kind: "api_endpoint", HTTPMethod: "DELETE", Path: "/delivery/{id}", Name: "DELETE /delivery/{id}", Qualified: "Delivery.deleteEntry", File: "Delivery.java", Line: 2, Confidence: "EXTRACTED"},
	}
	query := "Das Entfernen einer Buchung wird erfolgreich abgeschlossen. Danach bleiben Informationen sichtbar. Ermittle den HTTP-Einstiegspunkt."
	loaded := loadedContextIndex{ScopeRoot: root, Index: scan.AgentContextIndexRecord{Facts: facts}}
	got := withContextSourceSearch(loaded, query)
	selected, ok, reason := selectContextEndpoint(got.Index, rankContextFacts(got.Index.Facts, query), query)
	if !ok || selected.fact.ID != "billing" || got.sourceSearchID != "billing" {
		t.Fatalf("verified business term did not select billing: %s, %v, %s", selected.fact.ID, ok, reason)
	}
	if facts[0].Search != "" {
		t.Fatal("source search mutated the loaded index in place")
	}
	pack, err := compileContextPack(got.Index, ContextRequest{
		Query: query, BudgetTokens: DefaultContextBudgetTokens, MaxFiles: DefaultContextMaxFiles,
		sourceSearchID: got.sourceSearchID,
	})
	if err != nil || pack.FallbackRequired || pack.Confidence == "LOW" {
		t.Fatalf("verified source anchor did not produce usable metadata: %+v, %v", pack, err)
	}

	t.Run("unrelated query stays unresolved", func(t *testing.T) {
		got := withContextSourceSearch(loaded, "Das Entfernen einer unbekannten Sache wird erfolgreich abgeschlossen.")
		if got.sourceSearchID != "" {
			t.Fatal("query without a source match gained a seed")
		}
	})
	t.Run("handler search is bounded", func(t *testing.T) {
		large := loaded
		large.Index.Facts = make([]scan.AgentContextFactRecord, maxContextSourceSearchHandlers+1)
		for i := range large.Index.Facts {
			large.Index.Facts[i] = facts[0]
		}
		if got := withContextSourceSearch(large, query); got.sourceSearchID != "" {
			t.Fatal("handler search exceeded its limit")
		}
	})

	t.Run("neighboring declaration does not supply evidence", func(t *testing.T) {
		writeSourceFile(t, root, "Billing.java", "class Billing {\n void deleteEntry() {}\n void unrelated() { audit(\"Buchung\"); }\n}\n")
		got := withContextSourceSearch(loaded, query)
		if strings.Contains(got.Index.Facts[0].Search, "buchung") {
			t.Fatal("unrelated neighboring method promoted the endpoint")
		}
	})
	t.Run("ambiguous source matches remain unresolved", func(t *testing.T) {
		for _, name := range []string{"Billing", "Delivery"} {
			writeSourceFile(t, root, name+".java", "class "+name+" {\n void deleteEntry() { audit(\"Buchung\"); }\n}\n")
		}
		got := withContextSourceSearch(loaded, query)
		for _, fact := range got.Index.Facts {
			if fact.Search != "" {
				t.Fatal("tied business term selected an arbitrary endpoint")
			}
		}
	})
}
