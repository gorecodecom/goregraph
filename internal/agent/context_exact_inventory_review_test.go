package agent

import (
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestContextQueryRequestsExactEvidenceInventoryRequiresInventoryOrFileNoun(t *testing.T) {
	for _, test := range []struct {
		query string
		want  bool
	}{
		{query: "Analyze the exact production path for authentication.", want: false},
		{query: "Analysiere den exakten Produktions-Pfad für Authentifizierung.", want: false},
		{query: "Provide an exact production path inventory for authentication.", want: true},
		{query: "Provide an exact production file for authentication.", want: true},
		{query: "Stelle ein Verzeichnis mit exaktem Produktionsdatei-Inventar für Authentifizierung bereit.", want: true},
		{query: "Provide an exact file inventory mentioning TestimonialClient.", want: false},
		{query: "Stelle ein exaktes Datei-Inventar für den Produktionsauftrag bereit.", want: false},
	} {
		if got := contextQueryRequestsExactEvidenceInventory(test.query); got != test.want {
			t.Errorf("exact inventory trigger for %q = %t, want %t", test.query, got, test.want)
		}
	}
}

func TestExpandContextExactInventoryConcernsUsesPathSpecificPublicKeys(t *testing.T) {
	query := "Provide an exact production file inventory for configuration."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "config-class", Project: "services/catalog", Kind: "configuration", File: "src/main/java/catalog/CatalogConfig.java", Confidence: "EXACT"},
		{ID: "config-resource", Project: "services/catalog", Kind: "configuration", File: "src/main/resources/application.yml", Confidence: "EXACT"},
	}}
	base := newContextConcern(
		contextConcernConfiguration,
		"services/catalog",
		true,
		[]string{"config-class", "config-resource"},
		"requested configuration",
	)

	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{base},
	)
	want := map[string]bool{
		"configuration:services/catalog#exact-file:src/main/java/catalog/CatalogConfig.java": true,
		"configuration:services/catalog#exact-file:src/main/resources/application.yml":       true,
	}
	seen := map[string]bool{}
	for _, concern := range concerns {
		if !concern.exactInventory {
			continue
		}
		if concern.publicKey != concern.key {
			t.Errorf("exact concern public key = %q, want path-specific key %q", concern.publicKey, concern.key)
		}
		if !want[concern.key] {
			t.Errorf("unexpected exact concern key %q", concern.key)
		}
		seen[concern.key] = true
	}
	if len(seen) != len(want) {
		t.Fatalf("path-specific exact concerns = %v, want %v", seen, want)
	}
}
