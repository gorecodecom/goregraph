package agent

import (
	"slices"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveEvidenceQueryPlansGermanMechanismConcerns(t *testing.T) {
	const query = "Analysiere Authentifizierungs-, Konfigurations-, Fehler-/Retry- und Testmechanismen."
	seed := scan.AgentContextFactRecord{ID: "route", Project: "service", Kind: "route", Name: "GET /items", Path: "/items"}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{seed}}

	concerns := planContextSourceConcerns(query, index, seed, AdaptiveV2)
	for _, kind := range []string{contextConcernAuth, contextConcernConfiguration, contextConcernResilience, contextConcernTests} {
		if !hasContextConcernKind(concerns, kind) {
			t.Errorf("adaptive concerns omit %q: %#v", kind, concerns)
		}
	}
	strictConcerns := planContextSourceConcerns(query, index, seed, StrictV1)
	for _, kind := range []string{contextConcernAuth, contextConcernConfiguration, contextConcernTests} {
		if hasContextConcernKind(strictConcerns, kind) {
			t.Errorf("strict planning unexpectedly added %q: %#v", kind, strictConcerns)
		}
	}
}

func TestCompileContextPackPreservesPublicQueryAndPlansGermanMechanisms(t *testing.T) {
	const query = "DELETE /cadasters/{cadasterId}/regulations/{objectId}: Analysiere Authentifizierungs-, Konfigurations-, Fehler-/Retry- und Testmechanismen."
	root := writeSourceBackedContextFixture(t, false)
	pack, err := BuildContext(ContextRequest{Root: root, Query: query, ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Query != query {
		t.Fatalf("public query = %q, want original %q", pack.Query, query)
	}
	for _, kind := range []string{contextConcernAuth, contextConcernConfiguration, contextConcernResilience, contextConcernTests} {
		found := false
		for _, concern := range pack.Concerns {
			found = found || concern.Kind == kind
		}
		if !found {
			t.Errorf("compiled concerns omit %q: %#v", kind, pack.Concerns)
		}
	}
	foundTestSource := false
	for _, section := range pack.SourceSections {
		foundTestSource = foundTestSource || section.Path == "ControllerTest.java"
	}
	if !foundTestSource {
		t.Fatalf("German Testmechanismen concern did not retain test source: %#v", pack.SourceSections)
	}
}

func TestAdaptiveEvidenceQueryPlansOrdinaryGermanAndEnglishConcerns(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{name: "German nouns", query: "Prüfe Authentifizierung, Konfiguration, Wiederholung und Tests."},
		{name: "English nouns", query: "Inspect authentication, configuration, error handling, retries, and tests."},
	}
	seed := scan.AgentContextFactRecord{ID: "route", Project: "service", Kind: "route", Name: "GET /items", Path: "/items"}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{seed}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			concerns := planContextSourceConcerns(test.query, index, seed, AdaptiveV2)
			for _, kind := range []string{contextConcernAuth, contextConcernConfiguration, contextConcernResilience, contextConcernTests} {
				if !hasContextConcernKind(concerns, kind) {
					t.Errorf("concerns omit %q: %#v", kind, concerns)
				}
			}
		})
	}
}

func TestContextEvidenceQueryPreservesStrictAndIsIdempotent(t *testing.T) {
	const query = "Authentifizierungs-, Konfigurations- und Testmechanismen"
	if got := contextEvidenceQueryForProtocol(query, StrictV1); got != query {
		t.Fatalf("strict query = %q, want original %q", got, query)
	}

	once := contextEvidenceQueryForProtocol(query, AdaptiveV2)
	if twice := contextEvidenceQueryForProtocol(once, AdaptiveV2); twice != once {
		t.Fatalf("adaptive normalization is not idempotent:\nfirst:  %q\nsecond: %q", once, twice)
	}
}

func TestAdaptiveEvidenceQueryAvoidsSubstringFalsePositives(t *testing.T) {
	cases := []string{
		"Beschreibe die Konfigurationsmanagementplattform.",
		"Explain the contest mechanics and retryable naming convention.",
		"Explain the error loading invoices.",
		"Inspect src/main/AuthConfig.java.",
	}
	for _, query := range cases {
		if got := contextEvidenceQueryForProtocol(query, AdaptiveV2); got != query {
			t.Errorf("unrelated query expanded:\ninput: %q\ngot:   %q", query, got)
		}
	}
}

func TestAdaptiveEvidenceVocabularyDoesNotCreateExplicitProjectScope(t *testing.T) {
	seed := scan.AgentContextFactRecord{ID: "route", Project: "services/billing", Kind: "route", Name: "GET /items", Path: "/items"}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		seed,
		{ID: "configuration-project", Project: "services/configuration", Kind: "symbol", Name: "ConfigurationService"},
		{ID: "tests-project", Project: "services/tests", Kind: "symbol", Name: "TestService"},
		{ID: "source-project", Project: "services/source", Kind: "symbol", Name: "SourceService"},
	}}
	queries := []string{
		"Analysiere Authentifizierungs-, Konfigurations- und Testmechanismen.",
		"Analysiere services/billing Testmechanismen und liefere exakte Quellpfade.",
	}
	for _, query := range queries {
		selectionQuery := contextSelectionQuery(ContextPack{Query: query, ProtocolVersion: AdaptiveV2})
		if explicit := contextExplicitProjects(selectionQuery, contextProjectAliases(index.Facts, nil)); explicit["services/configuration"] || explicit["services/tests"] || explicit["services/source"] {
			t.Errorf("selection query creates project scope for %q: %#v", query, explicit)
		}
		concerns := planContextSourceConcerns(query, index, seed, AdaptiveV2)
		for _, concern := range concerns {
			if concern.kind == contextConcernProject && concern.project != "services/billing" {
				t.Errorf("evidence vocabulary created explicit project %q for %q", concern.project, query)
			}
		}
	}
}

func TestAdaptiveEvidenceVocabularyKeepsRawSeedAndConfigurationScope(t *testing.T) {
	const query = "GET /items: Prüfe Authentifizierungs-, Konfigurations- und Testmechanismen. Liefere exakte Quellpfade."
	seed := scan.AgentContextFactRecord{
		ID: "billing-route", Project: "services/billing", Kind: "route", Name: "GET /items", Path: "/items", HTTPMethod: "GET",
		File: "BillingController.java", Line: 1, Confidence: "EXACT",
	}
	index := configurationNavigationIndex()
	index.Facts = append(index.Facts, seed,
		scan.AgentContextFactRecord{ID: "configuration-symbol", Project: "services/configuration", Kind: "symbol", Name: "ConfigurationService", File: "ConfigurationService.java", Line: 1, Confidence: "EXACT"},
	)
	pack := ContextPack{
		Query: query, selectionQuery: query, ProtocolVersion: AdaptiveV2,
		SourceSections: []ContextSourceSection{{Project: "services/billing", Path: "BillingController.java"}},
	}

	gotSeed, ok := contextConcernPlanningSeed(index, contextSelectionQuery(pack))
	if !ok || gotSeed.ID != seed.ID {
		t.Fatalf("raw concern seed = %#v, found %v", gotSeed, ok)
	}
	resources := contextConfigurationResources(pack, index)
	if len(resources) != 1 || resources[0].Project != "services/billing" {
		t.Fatalf("configuration scope = %#v, want selected billing only", resources)
	}
}

func TestAdaptiveEvidenceQueryRecognizesExactSourcePathWordingConservatively(t *testing.T) {
	queries := []string{
		"Prüfe Konfigurations- und Testmechanismen. Liefere exakte workspace-relative Quellpfade.",
		"Inspect configuration and test mechanisms. Provide exact workspace-relative source paths.",
	}
	for _, query := range queries {
		pack := ContextPack{Query: query, selectionQuery: query, ProtocolVersion: AdaptiveV2}
		selectionQuery := contextEvidenceSelectionQuery(pack)
		if !contextQueryRequestsExactEvidenceInventory(selectionQuery) {
			t.Errorf("exact source identity request was not recognized: %q", selectionQuery)
		}
		for _, transition := range []string{"change", "create", "missing", "transition"} {
			if slices.Contains(strings.Fields(selectionQuery), transition) {
				t.Errorf("source path wording invented %q transition intent: %q", transition, selectionQuery)
			}
		}
		if contextQueryPlansMissingTransition(selectionQuery) {
			t.Errorf("source path wording invented a missing transition: %q", selectionQuery)
		}
	}
}

func hasContextConcernKind(concerns []contextConcern, kind string) bool {
	for _, concern := range concerns {
		if concern.kind == kind {
			return true
		}
	}
	return false
}
