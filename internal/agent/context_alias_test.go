package agent

import (
	"strings"
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

func TestRankContextFactsMatchesCancellationToSourceCancelHandler(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{
			ID: "subscription-route", Project: "services/subscription-service", Kind: "route",
			Name: "DELETE /subscriptions/{id}", Qualified: "SubscriptionController.cancel",
			HTTPMethod: "DELETE", Path: "/subscriptions/{id}", File: "SubscriptionController.java", Confidence: "EXACT",
		},
		{
			ID: "session-route", Project: "services/session-service", Kind: "route",
			Name: "DELETE /sessions/{id}", Qualified: "SessionController.cleanup",
			HTTPMethod: "DELETE", Path: "/sessions/{id}", File: "SessionController.java", Confidence: "EXACT",
		},
	}

	ranked := rankContextFacts(facts, "Trace cancellation from the public subscription endpoint to session cleanup.")
	if len(ranked) != 2 || ranked[0].fact.ID != "subscription-route" {
		t.Fatalf("cancellation did not follow the source cancel handler: %#v", ranked)
	}
}

func TestSelectContextEndpointPrefersTransitionSourceOverTargetUtility(t *testing.T) {
	query := "Trace cancellation from the public subscription endpoint to session cleanup."
	facts := []scan.AgentContextFactRecord{
		{
			ID: "subscription-endpoint", Project: "services/subscription-service", Kind: "api_endpoint",
			Name: "DELETE /subscriptions/{id}", Qualified: "SubscriptionController.cancel",
			HTTPMethod: "DELETE", Path: "/subscriptions/{id}", File: "SubscriptionController.java",
			Summary: "provider subscription-service; security role", Confidence: "EXACT",
		},
		{
			ID: "session-endpoint", Project: "services/session-service", Kind: "api_endpoint",
			Name: "DELETE /sessions/{id}", Qualified: "SessionController.cleanup",
			HTTPMethod: "DELETE", Path: "/sessions/{id}", File: "SessionController.java",
			Summary: "provider session-service; security unknown", Confidence: "EXACT",
		},
		{ID: "session-store-1", Project: "services/session-service", Kind: "persistence", Name: "SessionStore.delete", Confidence: "EXTRACTED"},
		{ID: "session-store-2", Project: "services/session-service", Kind: "persistence", Name: "SessionAudit.save", Confidence: "EXTRACTED"},
		{ID: "session-effect", Project: "services/session-service", Kind: "side_effect", Name: "SessionEvents.publish", Confidence: "EXTRACTED"},
	}
	index := scan.AgentContextIndexRecord{
		Facts: facts,
		Edges: []scan.AgentContextEdgeRecord{
			{FromFactID: "session-endpoint", ToFactID: "session-store-1", Kind: "persistence"},
			{FromFactID: "session-endpoint", ToFactID: "session-store-2", Kind: "persistence"},
			{FromFactID: "session-endpoint", ToFactID: "session-effect", Kind: "call"},
		},
	}

	selected, ok, reason := selectContextEndpoint(index, rankContextFacts(facts, query), query)
	if !ok || selected.fact.ID != "subscription-endpoint" {
		t.Fatalf("transition source endpoint = %#v, ok=%v, reason=%q", selected, ok, reason)
	}
}

func TestSelectContextEndpointUsesMutationSourceBeforeLingeringDependent(t *testing.T) {
	query := "Analysiere repositoryübergreifend, warum beim Entfernen eines Eintrags aus einer Sammlung " +
		"verbundene Aufgaben bestehen bleiben. Ermittle den öffentlichen REST-Endpunkt und die " +
		"bestehende Aufrufkette über services/catalog, services/jobs und libraries/shared."
	facts := []scan.AgentContextFactRecord{
		{
			ID: "catalog-endpoint", Project: "services/catalog", Kind: "api_endpoint",
			Name: "DELETE /collections/{collectionId}/entries/{entryId}", Qualified: "CatalogController.deleteEntry",
			HTTPMethod: "DELETE", Path: "/collections/{collectionId}/entries/{entryId}", File: "CatalogController.java",
			Summary: "provider catalog; security role", Confidence: "EXACT",
		},
		{
			ID: "jobs-endpoint", Project: "services/jobs", Kind: "api_endpoint",
			Name:      "DELETE /job-management/collections/{collectionId}/entries/{entryId}/changes/{changeId}/jobs/{jobId}",
			Qualified: "JobController.deleteChangeJob", HTTPMethod: "DELETE",
			Path: "/job-management/collections/{collectionId}/entries/{entryId}/changes/{changeId}/jobs/{jobId}",
			File: "JobController.java", Summary: "provider jobs; security role", Confidence: "EXACT",
		},
		{ID: "jobs-service", Project: "services/jobs", Kind: "symbol", Name: "deleteChangeJob", Confidence: "EXTRACTED"},
		{ID: "jobs-store", Project: "services/jobs", Kind: "persistence", Name: "JobStore.delete", Confidence: "EXTRACTED"},
		{ID: "jobs-audit", Project: "services/jobs", Kind: "side_effect", Name: "JobAudit.record", Confidence: "EXTRACTED"},
	}
	index := scan.AgentContextIndexRecord{
		Facts: facts,
		Edges: []scan.AgentContextEdgeRecord{
			{FromFactID: "jobs-endpoint", ToFactID: "jobs-service", Kind: "call"},
			{FromFactID: "jobs-service", ToFactID: "jobs-store", Kind: "persistence"},
			{FromFactID: "jobs-service", ToFactID: "jobs-audit", Kind: "call"},
		},
	}
	primaryAction, ok := contextEndpointPrimaryActionClause(query)
	if !ok || strings.Contains(primaryAction, "aufgaben") {
		t.Fatalf("primary mutation clause = %q, ok=%v", primaryAction, ok)
	}
	sourceScore := contextEndpointPrimaryActionScore(facts[0], primaryAction)
	dependentScore := contextEndpointPrimaryActionScore(facts[1], primaryAction)
	if sourceScore <= dependentScore {
		t.Fatalf("mutation source score = %d, dependent score = %d", sourceScore, dependentScore)
	}

	ranked := rankContextFacts(facts, query)
	selected, ok, reason := selectContextEndpoint(index, ranked, query)
	if !ok || selected.fact.ID != "catalog-endpoint" {
		t.Fatalf(
			"mutation source endpoint = %#v, ok=%v, reason=%q, ranked=%#v, sourceEligible=%v, sourceRouteMatch=%v",
			selected, ok, reason, ranked, eligibleContextEndpoint(facts[0]),
			contextEndpointRouteMatchesQuery(facts[0], query),
		)
	}

	directTargetQuery := "Analysiere services/jobs: Entferne verbundene Aufgaben, die bestehen bleiben. " +
		"Ermittle den öffentlichen REST-Endpunkt."
	selected, ok, reason = selectContextEndpoint(
		index,
		rankContextFacts(facts, directTargetQuery),
		directTargetQuery,
	)
	if !ok || selected.fact.ID != "jobs-endpoint" {
		t.Fatalf("direct dependent target endpoint = %#v, ok=%v, reason=%q", selected, ok, reason)
	}
	if contextEndpointPathStrictSubset(
		"/collections/{collectionId}/entries/{entryId}",
		"/jobs/entries/{entryId}/collections/{collectionId}",
	) {
		t.Fatal("reordered route tokens were treated as a parent path")
	}
}

func TestSelectSecondaryContextContractPathFollowsUniqueResolvedProvider(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{ID: "primary", Project: "services/subscription-service", Kind: "route", Name: "DELETE /subscriptions/{id}", Qualified: "SubscriptionController.cancel", HTTPMethod: "DELETE", Path: "/subscriptions/{id}", File: "SubscriptionController.java", Confidence: "EXACT"},
		{ID: "contract", Project: "libraries/membership-client", Kind: "api_contract", Name: "DELETE /sessions/{id}", Qualified: "MembershipClient.purgeSession", HTTPMethod: "DELETE", Path: "/sessions/{id}", File: "MembershipClient.java", Confidence: "RESOLVED"},
		{ID: "provider", Project: "services/session-provider", Kind: "route", Name: "DELETE /sessions/{id}", Qualified: "SessionController.purge", HTTPMethod: "DELETE", Path: "/sessions/{id}", File: "SessionController.java", Confidence: "EXACT"},
		{ID: "service", Project: "services/session-provider", Kind: "symbol", Name: "SessionPurgeService.purge", Qualified: "SessionPurgeService.purge", File: "SessionPurgeService.java", Confidence: "EXACT"},
		{ID: "active", Project: "services/session-provider", Kind: "persistence", Name: "ActiveSessionRepository.delete", Qualified: "ActiveSessionRepository.delete", File: "ActiveSessionRepository.java", Confidence: "EXTRACTED"},
		{ID: "archive", Project: "services/session-provider", Kind: "persistence", Name: "ArchivedSessionRepository.delete", Qualified: "ArchivedSessionRepository.delete", File: "ArchivedSessionRepository.java", Confidence: "EXTRACTED"},
		{ID: "event", Project: "services/session-provider", Kind: "side_effect", Name: "SessionEventPublisher.publish", Qualified: "SessionEventPublisher.publish", File: "SessionEventPublisher.java", Confidence: "EXTRACTED"},
	}
	index := scan.AgentContextIndexRecord{
		Facts: facts,
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "contract-provider", FromFactID: "contract", ToFactID: "provider", Kind: "http_contract", Confidence: "RESOLVED"},
			{ID: "provider-service", FromFactID: "provider", ToFactID: "service", Kind: "call", Confidence: "EXTRACTED"},
			{ID: "service-active", FromFactID: "service", ToFactID: "active", Kind: "persistence", Confidence: "EXTRACTED"},
			{ID: "service-archive", FromFactID: "service", ToFactID: "archive", Kind: "persistence", Confidence: "EXTRACTED"},
			{ID: "service-event", FromFactID: "service", ToFactID: "event", Kind: "call", Confidence: "EXTRACTED"},
		},
	}
	query := "Trace cancellation from the public subscription endpoint to session cleanup through libraries/membership-client. Identify the missing client call, both persistence repositories, side effects, and tests."
	primary := contextPathSelection{relatedProductionFacts: []scan.AgentContextFactRecord{facts[1]}}

	top, selected, ok := selectSecondaryContextContractPath(index, query, facts[0], primary)
	if !ok || top.fact.ID != "contract" {
		t.Fatalf("secondary contract seed = %#v, ok=%v", top, ok)
	}
	selectedIDs := map[string]bool{}
	for _, factID := range selected.factIDs {
		selectedIDs[factID] = true
	}
	for _, want := range []string{"contract", "provider", "service", "active", "archive", "event"} {
		if !selectedIDs[want] {
			t.Errorf("secondary contract path missing %q: %#v", want, selected.factIDs)
		}
	}
}

func TestSelectSecondaryContextContractPathRejectsDifferentPrimaryAction(t *testing.T) {
	primary := scan.AgentContextFactRecord{
		ID: "primary", Project: "services/catalog", Kind: "route",
		Name: "DELETE /catalog/items/{id}", HTTPMethod: "DELETE", Path: "/catalog/items/{id}",
		File: "CatalogController.java", Confidence: "EXACT",
	}
	contract := scan.AgentContextFactRecord{
		ID: "contract", Project: "libraries/job-client", Kind: "api_contract",
		Name: "GET /jobs", Qualified: "JobClient.list", HTTPMethod: "GET", Path: "/jobs",
		File: "JobClient.java", Confidence: "RESOLVED", Search: "job cleanup client",
	}
	provider := scan.AgentContextFactRecord{
		ID: "provider", Project: "services/jobs", Kind: "route",
		Name: "GET /jobs", HTTPMethod: "GET", Path: "/jobs",
		File: "JobController.java", Confidence: "EXACT",
	}
	index := scan.AgentContextIndexRecord{
		Facts: []scan.AgentContextFactRecord{primary, contract, provider},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "contract-provider", FromFactID: contract.ID, ToFactID: provider.ID,
			Kind: "http_contract", Confidence: "RESOLVED",
		}},
	}
	selection := contextPathSelection{relatedProductionFacts: []scan.AgentContextFactRecord{contract}}

	if _, _, ok := selectSecondaryContextContractPath(
		index,
		"Trace deletion from the catalog endpoint to job cleanup and identify the missing HTTP contract.",
		primary,
		selection,
	); ok {
		t.Fatal("GET side contract must not become a continuation of a DELETE operation")
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

func TestRankContextFactsPromotesOnlyRouteMatchingGermanMutationIntent(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{ID: "delete-route", Project: "services/profile", Kind: "route", Name: "DELETE /profiles/{id}", Qualified: "ProfileController.delete", HTTPMethod: "DELETE", Path: "/profiles/{id}", File: "ProfileController.java", Confidence: "EXACT"},
		{ID: "create-route", Project: "services/profile", Kind: "route", Name: "POST /profiles", Qualified: "ProfileController.create", HTTPMethod: "POST", Path: "/profiles", File: "ProfileController.java", Confidence: "EXACT"},
	}

	ranked := rankContextFacts(facts, "Analysiere das Löschen eines unbekannten Objekts.")
	if len(ranked) == 0 || ranked[0].fact.ID != "delete-route" || ranked[0].score < minimumContextSeedScore {
		t.Fatalf("unique mutation route was not promoted: %#v", ranked)
	}
}

func TestRankContextFactsPrefersPublicRouteForGermanPublicEndpointRequest(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{ID: "public-route", Project: "services/profile", Kind: "route", Name: "DELETE /profiles/{id}", Qualified: "ProfileController.delete", HTTPMethod: "DELETE", Path: "/profiles/{id}", File: "ProfileController.java", Confidence: "EXACT"},
		{ID: "internal-route", Project: "services/maintenance", Kind: "route", Name: "DELETE /internal/profiles/expired", Qualified: "ProfileMaintenanceController.deleteExpired", HTTPMethod: "DELETE", Path: "/internal/profiles/expired", File: "ProfileMaintenanceController.java", Confidence: "EXACT"},
		{ID: "target-route", Project: "services/worker", Kind: "route", Name: "GET /worker/tasks", Qualified: "WorkerController.list", HTTPMethod: "GET", Path: "/worker/tasks", File: "WorkerController.java", Confidence: "EXACT"},
	}

	ranked := rankContextFacts(facts, "Plane das Löschen eines unbekannten Objekts über services/worker. Zeige den aktuellen öffentlichen Endpunkt.")
	if len(ranked) == 0 || ranked[0].fact.ID != "public-route" || ranked[0].score < minimumContextSeedScore {
		t.Fatalf("public mutation route was not promoted: %#v", ranked)
	}
}
