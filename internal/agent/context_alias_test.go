package agent

import (
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestRankContextFactsPromotesUniqueRouteInExplicitProjects(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{
			ID: "order-delete", Project: "services/orders", Kind: "route",
			HTTPMethod: "DELETE", Path: "/orders/{id}", Search: "delete order",
			Confidence: "EXACT",
		},
		{
			ID: "unrelated", Project: "services/shipping", Kind: "route",
			HTTPMethod: "GET", Path: "/shipping/{id}", Search: "read shipping",
			Confidence: "EXACT",
		},
	}
	query := "Analysiere services/orders und libraries/shared: Beim Entfernen bleiben abhängige Einträge bestehen. Ermittle den REST-Endpunkt."

	ranked := rankContextFacts(facts, query)
	if len(ranked) == 0 || ranked[0].fact.ID != "order-delete" || ranked[0].score < minimumContextMediumScore {
		t.Fatalf("unique route in explicit projects was not promoted: %#v", ranked)
	}
}

func TestRankContextFactsDoesNotPromoteAmbiguousExplicitProjectRoutes(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{
			ID: "order-delete", Project: "services/orders", Kind: "route",
			HTTPMethod: "DELETE", Path: "/orders/{id}", Search: "delete order",
			Confidence: "EXACT",
		},
		{
			ID: "shipment-delete", Project: "services/shipping", Kind: "route",
			HTTPMethod: "DELETE", Path: "/shipping/{id}", Search: "delete shipping",
			Confidence: "EXACT",
		},
	}
	query := "Analysiere services/orders und services/shipping: Beim Entfernen bleiben abhängige Einträge bestehen. Ermittle den REST-Endpunkt."

	for _, candidate := range rankContextFacts(facts, query) {
		if candidate.score >= minimumContextMediumScore {
			t.Fatalf("ambiguous explicit routes received a confidence floor: %#v", candidate)
		}
	}
}

func TestSelectContextPathsKeepsBoundedSourceCallChainWithoutDomainAliases(t *testing.T) {
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "route", Project: "services/orders", Kind: "route",
				HTTPMethod: "DELETE", Path: "/orders/{id}", Search: "delete order",
				Confidence: "EXACT",
			},
			{ID: "controller", Project: "services/orders", Kind: "symbol", Name: "cancel", Confidence: "EXACT"},
			{ID: "service", Project: "services/orders", Kind: "symbol", Name: "archive", Confidence: "EXACT"},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "route-controller", FromFactID: "route", ToFactID: "controller", Kind: "call", Confidence: "EXACT"},
			{ID: "controller-service", FromFactID: "controller", ToFactID: "service", Kind: "call", Confidence: "EXACT"},
		},
	}
	query := "Analysiere services/orders: Beim Entfernen bleiben abhängige Einträge bestehen. Ermittle den REST-Endpunkt und die bestehende Aufrufkette."
	ranked := rankContextFacts(index.Facts, query)
	if len(ranked) == 0 || ranked[0].fact.ID != "route" {
		t.Fatalf("route was not selected: %#v", ranked)
	}
	concerns := planContextConcerns(query, index, ranked[0].fact)
	selection := selectContextPaths(index, ranked[0], concerns)
	selected := map[string]bool{}
	for _, factID := range selection.factIDs {
		selected[factID] = true
	}
	for _, factID := range []string{"route", "controller", "service"} {
		if !selected[factID] {
			t.Errorf("bounded source call chain omitted %q: %#v", factID, selection)
		}
	}
}

func TestContextAliasesDoNotTranslateBusinessNouns(t *testing.T) {
	tokens := contextExpandedTokenSet("vorschrift kataster katalogeintrag konto")
	for _, forbidden := range []string{"account", "cadaster", "cadasters", "catalog", "item", "regulation", "regulations"} {
		if tokens[forbidden] {
			t.Errorf("business noun introduced built-in alias %q: %#v", forbidden, tokens)
		}
	}
}

func TestContextAliasesRetainTechnicalIntentVocabulary(t *testing.T) {
	tokens := contextExpandedTokenSet("löschen authentifizierung konfiguration persistenz")
	for _, required := range []string{"authentication", "config", "delete", "persistence", "remove", "repository"} {
		if !tokens[required] {
			t.Errorf("technical intent alias %q is missing: %#v", required, tokens)
		}
	}
}
