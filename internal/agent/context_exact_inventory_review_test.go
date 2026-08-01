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

func TestExpandContextExactInventoryConcernsPreservesAllRequestedCategories(t *testing.T) {
	query := "Provide an exact production file inventory for authentication, configuration, retry, persistence, and side effects."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "auth-config", Project: "services/catalog", Kind: "authentication", Name: "ConfigurationBackedAuth", File: "src/main/java/catalog/ConfigurationBackedAuth.java", Confidence: "EXACT", Search: "authentication configuration retry persistence side effects"},
		{ID: "auth-security", Project: "services/catalog", Kind: "authentication", Name: "SecurityPolicy", File: "src/main/java/catalog/SecurityPolicy.java", Confidence: "EXACT", Search: "authentication security"},
		{ID: "config-auth", Project: "services/catalog", Kind: "configuration", Name: "AuthenticationProperties", File: "src/main/java/catalog/AuthenticationProperties.java", Confidence: "EXACT", Search: "configuration authentication retry persistence side effects"},
		{ID: "config-settings", Project: "services/catalog", Kind: "configuration", Name: "ServiceSettings", File: "src/main/java/catalog/ServiceSettings.java", Confidence: "EXACT", Search: "configuration settings"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{
			newContextConcern(contextConcernAuth, "services/catalog", true, []string{"auth-config", "auth-security"}, "requested authentication"),
			newContextConcern(contextConcernConfiguration, "services/catalog", true, []string{"config-auth", "config-settings"}, "requested configuration"),
		},
	)
	want := map[string]bool{
		"authentication:services/catalog#exact-file:src/main/java/catalog/ConfigurationBackedAuth.java": true,
		"authentication:services/catalog#exact-file:src/main/java/catalog/SecurityPolicy.java":          true,
		"configuration:services/catalog#exact-file:src/main/java/catalog/AuthenticationProperties.java": true,
		"configuration:services/catalog#exact-file:src/main/java/catalog/ServiceSettings.java":          true,
	}
	seen := map[string]bool{}
	for _, concern := range concerns {
		if concern.exactInventory {
			seen[concern.key] = true
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("exact multi-category inventory missing %q from %v", key, seen)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("exact multi-category inventory = %v, want %v", seen, want)
	}
}

func TestExpandContextExactInventoryConcernsPreservesUnscopedAuthenticationSiblings(t *testing.T) {
	query := "Provide an exact production file inventory for client and provider authentication."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "client-auth", Project: "libraries/client", Kind: "authentication", Name: "ClientSecurity", File: "src/main/java/client/ClientSecurity.java", Confidence: "EXACT", Search: "client authentication"},
		{ID: "provider-auth", Project: "services/provider", Kind: "authentication", Name: "ProviderSecurity", File: "src/main/java/provider/ProviderSecurity.java", Confidence: "EXACT", Search: "provider authentication"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{
			newContextConcern(contextConcernAuth, "", true, []string{"client-auth", "provider-auth"}, "requested authentication"),
		},
	)
	want := map[string]bool{
		"authentication:libraries/client#exact-file:src/main/java/client/ClientSecurity.java":      true,
		"authentication:services/provider#exact-file:src/main/java/provider/ProviderSecurity.java": true,
	}
	seen := map[string]bool{}
	for _, concern := range concerns {
		if concern.exactInventory {
			seen[concern.key] = true
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("exact unscoped authentication inventory missing %q from %v", key, seen)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("exact unscoped authentication inventory = %v, want %v", seen, want)
	}
}

func TestExpandContextExactInventoryConcernsPreservesGermanRequestedCategories(t *testing.T) {
	query := "Stelle eine exakte Produktionsdatei-Liste für Authentifizierung, Konfiguration, Wiederholung, Persistenz und Nebenwirkungen bereit."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "auth-config", Project: "services/catalog", Kind: "authentication", Name: "ConfigurationBackedAuth", File: "src/main/java/catalog/ConfigurationBackedAuth.java", Confidence: "EXACT", Search: "authentifizierung konfiguration wiederholung persistenz nebenwirkung"},
		{ID: "auth-security", Project: "services/catalog", Kind: "authentication", Name: "SecurityPolicy", File: "src/main/java/catalog/SecurityPolicy.java", Confidence: "EXACT", Search: "authentifizierung sicherheit"},
		{ID: "config-auth", Project: "services/catalog", Kind: "configuration", Name: "AuthenticationProperties", File: "src/main/java/catalog/AuthenticationProperties.java", Confidence: "EXACT", Search: "konfiguration authentifizierung wiederholung persistenz nebenwirkung"},
		{ID: "config-settings", Project: "services/catalog", Kind: "configuration", Name: "ServiceSettings", File: "src/main/java/catalog/ServiceSettings.java", Confidence: "EXACT", Search: "konfiguration einstellungen"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{
			newContextConcern(contextConcernAuth, "services/catalog", true, []string{"auth-config", "auth-security"}, "requested authentication"),
			newContextConcern(contextConcernConfiguration, "services/catalog", true, []string{"config-auth", "config-settings"}, "requested configuration"),
		},
	)
	want := map[string]bool{
		"authentication:services/catalog#exact-file:src/main/java/catalog/ConfigurationBackedAuth.java": true,
		"authentication:services/catalog#exact-file:src/main/java/catalog/SecurityPolicy.java":          true,
		"configuration:services/catalog#exact-file:src/main/java/catalog/AuthenticationProperties.java": true,
		"configuration:services/catalog#exact-file:src/main/java/catalog/ServiceSettings.java":          true,
	}
	seen := map[string]bool{}
	for _, concern := range concerns {
		if concern.exactInventory {
			seen[concern.key] = true
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("exact German multi-category inventory missing %q from %v", key, seen)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("exact German multi-category inventory = %v, want %v", seen, want)
	}
}
