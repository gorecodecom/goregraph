package agentbench

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
)

func TestProjectPackProducesStableSemanticEvidence(t *testing.T) {
	projection := ProjectPack(goldenPack())

	if projection.Endpoint != "catalog|DELETE|/catalog/{itemId}" {
		t.Fatalf("Endpoint = %q", projection.Endpoint)
	}
	if !slices.Contains(projection.Persistence, "persistence|services/catalog|repository|deleteById|CatalogRepository.java|42") {
		t.Fatalf("Persistence = %#v", projection.Persistence)
	}
	if !slices.Contains(projection.Sources, "services/catalog|src/CatalogRepository.java|40|48|persistence|deletion query") {
		t.Fatalf("Sources = %#v", projection.Sources)
	}
}

func TestEvaluatePackReportsExpectationViolations(t *testing.T) {
	violations := EvaluatePack(candidatePack(), validContract().Pack)

	requireViolation(t, violations, "endpoint")
	requireViolation(t, violations, "source_omission")
}

func TestDiffPacksReportsSemanticChanges(t *testing.T) {
	diff := DiffPacks(goldenPack(), candidatePack())
	if !diff.EndpointChanged {
		t.Fatal("endpoint change was not reported")
	}
	if !slices.Contains(diff.RemovedLocations, "persistence|services/catalog|deleteById") {
		t.Fatalf("removed locations = %#v", diff.RemovedLocations)
	}
	if !slices.Contains(diff.AddedSources, "libraries/job-client/src/JobClientConfig.java") {
		t.Fatalf("added sources = %#v", diff.AddedSources)
	}
}

func TestEvaluatePackDiffRejectsProtectedAndUndeclaredChanges(t *testing.T) {
	t.Run("rejects protected fields regardless of allowed change categories", func(t *testing.T) {
		hypothesis := Hypothesis{AllowedPackChanges: []string{"locations", "sources", "coverage", "omissions", "budget"}}

		violations := EvaluatePackDiff(DiffPacks(goldenPack(), candidatePack()), hypothesis)

		requireViolation(t, violations, "endpoint")
		requireViolation(t, violations, "persistence")
	})

	t.Run("rejects call chain changes regardless of allowed change categories", func(t *testing.T) {
		hypothesis := Hypothesis{AllowedPackChanges: []string{"locations", "sources", "coverage", "omissions", "budget"}}
		candidate := goldenPack()
		candidate.CallChain[0].To = "CatalogService.getByID"

		violations := EvaluatePackDiff(DiffPacks(goldenPack(), candidate), hypothesis)

		requireViolation(t, violations, "call_chain")
	})

	t.Run("rejects actual change categories absent from the hypothesis", func(t *testing.T) {
		hypothesis := Hypothesis{AllowedPackChanges: []string{"locations"}}
		candidate := goldenPack()
		candidate.Files = append(candidate.Files, agent.ContextFile{
			Project: "libraries/job-client", Path: "src/JobClientConfig.java", StartLine: 1, EndLine: 10, Role: "contract", Reason: "configuration",
		})

		violations := EvaluatePackDiff(DiffPacks(goldenPack(), candidate), hypothesis)

		requireViolation(t, violations, "sources")
	})
}

func TestProjectPackAndDiffPacksAreDeterministicAcrossPublicSliceOrder(t *testing.T) {
	golden := goldenPack()
	candidate := candidatePack()
	addOrderingEvidence(&golden)
	addOrderingEvidence(&candidate)
	reversedGolden := reversePublicPackSlices(golden)
	reversedCandidate := reversePublicPackSlices(candidate)

	assertSameJSON(t, ProjectPack(golden), ProjectPack(reversedGolden))
	assertSameJSON(t, DiffPacks(golden, candidate), DiffPacks(reversedGolden, reversedCandidate))
}

func addOrderingEvidence(pack *agent.ContextPack) {
	pack.Concerns = append(pack.Concerns, agent.ContextConcern{Kind: "contract", Project: "services/catalog", Covered: true, Reason: "selected"})
	pack.Entrypoints = append(pack.Entrypoints, agent.ContextLocation{Project: "services/catalog", Kind: "controller", Label: "getItem", File: "CatalogController.java", Line: 28, EvidenceIDs: []string{"b", "a"}})
	pack.Endpoints = append(pack.Endpoints, agent.ContextEndpoint{Provider: "catalog", HTTPMethod: "POST", Path: "/catalog", Consumers: []agent.ContextEndpointConsumer{{Project: "web", Authentication: "session"}, {Project: "cli", Authentication: "token"}}, Limitations: []string{"second", "first"}})
	pack.CallChain = append(pack.CallChain, agent.ContextRelationship{From: "CatalogService.deleteById", To: "CatalogRepository.deleteById", Kind: "calls", Reason: "persistence"})
	pack.Contracts = append(pack.Contracts, agent.ContextLocation{Project: "services/catalog", Kind: "api_contract", Label: "CatalogItem", File: "CatalogApi.java", Line: 20, EvidenceIDs: []string{"b", "a"}})
	pack.Persistence = append(pack.Persistence, agent.ContextLocation{Project: "services/catalog", Kind: "repository", Label: "findById", File: "CatalogRepository.java", Line: 28, EvidenceIDs: []string{"b", "a"}})
	pack.Tests = append(pack.Tests, agent.ContextLocation{Project: "services/catalog", Kind: "test", Label: "gets catalog item", File: "CatalogControllerTest.java", Line: 52, EvidenceIDs: []string{"b", "a"}})
	pack.Files = append(pack.Files, agent.ContextFile{Project: "services/catalog", Path: "src/CatalogService.java", StartLine: 10, EndLine: 20, Role: "call_chain", Reason: "service flow"})
	pack.Uncertainties = append(pack.Uncertainties, agent.ContextUncertainty{Scope: "authorization", Reason: "not verified"})
	pack.SourceSections = append(pack.SourceSections, agent.ContextSourceSection{Project: "services/catalog", Path: "src/CatalogService.java", StartLine: 10, EndLine: 20, Role: "call_chain", RenderMode: "excerpt", SourceState: "present", Content: "delete"})
	pack.SourceOmissions = append(pack.SourceOmissions, agent.ContextSourceOmission{Project: "services/catalog", Path: "src/CatalogAudit.java", StartLine: 1, EndLine: 2, Role: "persistence", Reason: "budget"})
	pack.RetryAnchors = append(pack.RetryAnchors, "repository")
}

func goldenPack() agent.ContextPack {
	return agent.ContextPack{
		Entrypoints:     []agent.ContextLocation{{Project: "services/catalog", Kind: "controller", Label: "deleteItem", File: "CatalogController.java", Line: 18}},
		Endpoints:       []agent.ContextEndpoint{{Provider: "catalog", HTTPMethod: "DELETE", Path: "/catalog/{itemId}", Handler: "deleteItem", File: "CatalogController.java", Line: 18, Consumers: []agent.ContextEndpointConsumer{{Project: "web", File: "catalog.ts", Line: 9, Authentication: "session"}}, Limitations: []string{"request body omitted"}}},
		CallChain:       []agent.ContextRelationship{{From: "CatalogController.deleteItem", To: "CatalogService.deleteById", Kind: "calls", Reason: "deletion flow"}},
		Contracts:       []agent.ContextLocation{{Project: "services/catalog", Kind: "api_contract", Label: "DeleteCatalogItem", File: "CatalogApi.java", Line: 11}},
		Persistence:     []agent.ContextLocation{{Project: "services/catalog", Kind: "repository", Label: "deleteById", File: "CatalogRepository.java", Line: 42}},
		Tests:           []agent.ContextLocation{{Project: "services/catalog", Kind: "test", Label: "deletes catalog item", File: "CatalogControllerTest.java", Line: 72}},
		Files:           []agent.ContextFile{{Project: "services/catalog", Path: "src/CatalogRepository.java", StartLine: 40, EndLine: 48, Role: "persistence", Reason: "deletion query"}},
		Uncertainties:   []agent.ContextUncertainty{{Scope: "audit", Reason: "job client not indexed"}},
		SourceSections:  []agent.ContextSourceSection{{Project: "services/catalog", Path: "src/CatalogRepository.java", StartLine: 40, EndLine: 48, Role: "persistence", RenderMode: "excerpt", SourceState: "present", Content: "delete"}},
		SourceOmissions: []agent.ContextSourceOmission{{Project: "services/catalog", Path: "src/LegacyCatalog.java", StartLine: 1, EndLine: 2, Role: "persistence", Reason: "budget"}},
		Concerns:        []agent.ContextConcern{{Kind: "persistence", Project: "services/catalog", Covered: true, Reason: "selected"}},
		SourceCoverage:  "partial", EstimatedTokens: 1000, FallbackRequired: false, RetryAllowed: true, RetryAnchors: []string{"catalog"},
	}
}

func candidatePack() agent.ContextPack {
	pack := goldenPack()
	pack.Endpoints = []agent.ContextEndpoint{{Provider: "catalog", HTTPMethod: "GET", Path: "/catalog/{itemId}", Handler: "getItem", File: "CatalogController.java", Line: 28}}
	pack.Persistence = nil
	pack.Files = append(pack.Files, agent.ContextFile{Project: "libraries/job-client", Path: "src/JobClientConfig.java", StartLine: 1, EndLine: 12, Role: "contract", Reason: "configuration source"})
	pack.SourceOmissions = append(pack.SourceOmissions, agent.ContextSourceOmission{Project: "libraries/job-client", Path: "", Role: "contract", Reason: "pathless omission"})
	return pack
}

func reversePublicPackSlices(pack agent.ContextPack) agent.ContextPack {
	slices.Reverse(pack.Concerns)
	slices.Reverse(pack.Entrypoints)
	slices.Reverse(pack.Endpoints)
	slices.Reverse(pack.CallChain)
	slices.Reverse(pack.Contracts)
	slices.Reverse(pack.Persistence)
	slices.Reverse(pack.Tests)
	slices.Reverse(pack.Files)
	slices.Reverse(pack.Uncertainties)
	slices.Reverse(pack.SourceSections)
	slices.Reverse(pack.SourceOmissions)
	slices.Reverse(pack.RetryAnchors)
	for index := range pack.Endpoints {
		slices.Reverse(pack.Endpoints[index].Consumers)
		slices.Reverse(pack.Endpoints[index].Limitations)
	}
	reverseLocationEvidence(pack.Entrypoints)
	reverseLocationEvidence(pack.Contracts)
	reverseLocationEvidence(pack.Persistence)
	reverseLocationEvidence(pack.Tests)
	return pack
}

func reverseLocationEvidence(locations []agent.ContextLocation) {
	for index := range locations {
		slices.Reverse(locations[index].EvidenceIDs)
	}
}

func assertSameJSON(t *testing.T, got, want any) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("JSON differs:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func requireViolation(t *testing.T, violations []Violation, field string) {
	t.Helper()
	for _, violation := range violations {
		if violation.Field == field {
			return
		}
	}
	t.Fatalf("violations = %#v, want field %q", violations, field)
}
