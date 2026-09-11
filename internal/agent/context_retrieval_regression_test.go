package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestRelatedEvidenceRetainsInternalProviderAndMutationImplementation(t *testing.T) {
	seed := scan.AgentContextFactRecord{ID: "entry", Project: "billing", Kind: "route", Name: "DELETE /invoices/{id}", Path: "/invoices/{id}"}
	model := scan.AgentContextFactRecord{ID: "model", Project: "ledger", Kind: "symbol", Name: "InvoiceEntryEntity", File: "InvoiceEntryEntity.java", Line: 1}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{seed, model,
		{ID: "provider", Project: "ledger", Kind: "symbol", Name: "InvoiceEntryMgmtController", Qualified: "InvoiceEntryMgmtController", File: "InvoiceEntryMgmtController.java", Line: 1},
		{ID: "mutation", Project: "ledger", Kind: "symbol", Name: "deleteEntry", Qualified: "InvoiceEntryService.deleteEntry", File: "InvoiceEntryService.java", Line: 10},
	}}
	for i := range 20 {
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{ID: fmt.Sprintf("route-%02d", i), Project: "ledger", Kind: "route", Name: "DELETE /invoices/{id}/entries/{entryId}", Path: "/invoices/{id}/entries/{entryId}", HTTPMethod: "DELETE", Qualified: "InvoiceEntryController.deleteEntry", File: "InvoiceEntryController.java", Line: i + 1})
	}
	concerns := []contextConcern{
		newContextConcern(contextConcernDomainModel, "", true, []string{"model"}, "data variants"),
		newContextConcern(contextConcernHTTPContract, "", true, nil, "internal interfaces"),
		newContextConcern(contextConcernSideEffects, "", true, nil, "deletion side effects"),
	}
	got := relatedModelContextConcerns("invoice deletion: internal API contracts and side effects", index, seed, concerns, AdaptiveV2)
	want := map[string]string{contextConcernHTTPContract: "provider", contextConcernSideEffects: "mutation"}
	for kind, id := range want {
		found := false
		for _, concern := range got {
			if concern.kind == kind && slices.Contains(concern.candidateFactIDs, id) {
				found = true
			}
		}
		if !found {
			t.Errorf("route duplicates displaced %s evidence %s", kind, id)
		}
	}
}

func TestRelatedModelProjectDoesNotAdmitOtherParentDomains(t *testing.T) {
	seed := scan.AgentContextFactRecord{ID: "entry", Project: "billing", Kind: "route", Name: "DELETE /invoices/{id}", Path: "/invoices/{id}"}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "model", Project: "shared", Kind: "symbol", Name: "InvoiceEntryEntity", File: "InvoiceEntryEntity.java", Line: 1},
		{ID: "invoice", Project: "shared", Kind: "symbol", Name: "InvoiceEntryClient", File: "InvoiceEntryClient.java", Line: 1},
		{ID: "parcel", Project: "shared", Kind: "symbol", Name: "ParcelEntryClient", File: "ParcelEntryClient.java", Line: 1},
	}}
	concerns := []contextConcern{
		newContextConcern(contextConcernDomainModel, "", true, []string{"model"}, "data variants"),
		newContextConcern(contextConcernHTTPContract, "", true, nil, "internal clients"),
	}
	got := relatedModelContextConcerns("invoice deletion: internal clients", index, seed, concerns, AdaptiveV2)
	found := false
	for _, concern := range got {
		if slices.Contains(concern.candidateFactIDs, "parcel") {
			t.Error("shared project membership admitted a different parent domain")
		}
		found = found || slices.Contains(concern.candidateFactIDs, "invoice")
	}
	if !found {
		t.Fatal("related invoice client was lost")
	}
}

func TestNeutralRequestDiscoversRelatedModelsWithoutProjectNames(t *testing.T) {
	seed := scan.AgentContextFactRecord{ID: "route", Project: "billing", Kind: "route", Name: "DELETE /invoices/{invoiceId}", HTTPMethod: "DELETE", Path: "/invoices/{invoiceId}", File: "Invoices.java", Line: 2}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		seed,
		{ID: "entry", Project: "ledger", Kind: "symbol", Name: "InvoiceEntryEntity", File: "InvoiceEntryEntity.java", Line: 1},
		{ID: "adjustment", Project: "ledger", Kind: "symbol", Name: "InvoiceEntryChangeEntity", File: "InvoiceEntryChangeEntity.java", Line: 1},
		{ID: "unrelated", Project: "shipping", Kind: "symbol", Name: "ParcelEntity", File: "ParcelEntity.java", Line: 1},
		{ID: "client", Project: "shared", Kind: "symbol", Name: "InvoiceEntryClient", File: "InvoiceEntryClient.java", Line: 1},
		{ID: "repository", Project: "ledger", Kind: "persistence", Name: "InvoiceEntryRepository", File: "InvoiceEntryRepository.java", Line: 1},
	}}
	concerns := planContextConcerns("Nach dem Entfernen einer Rechnung bleiben Informationen sichtbar. Nenne Datenvarianten, Zuordnungsmerkmale, interne Schnittstellen und Persistenz.", index, seed)
	for _, concern := range concerns {
		if concern.kind == contextConcernDomainModel && len(concern.candidateFactIDs) > 0 {
			t.Fatal("inferred model candidates can displace the established path in metadata selection")
		}
	}
	var models []string
	concerns = planContextSourceConcerns("Datenvarianten, Zuordnungsmerkmale, interne Schnittstellen und Persistenz", index, seed, AdaptiveV2)
	for _, concern := range concerns {
		if concern.kind == contextConcernDomainModel {
			models = concern.candidateFactIDs
		}
	}
	if len(models) != 2 || !slices.Contains(models, "entry") || !slices.Contains(models, "adjustment") {
		t.Fatalf("related model candidates = %v", models)
	}
	for _, want := range []string{"client", "repository"} {
		found := false
		for _, concern := range relatedModelContextConcerns("models, contracts and persistence", index, seed, concerns, AdaptiveV2) {
			found = found || slices.Contains(concern.candidateFactIDs, want)
		}
		if !found {
			t.Errorf("missing related evidence candidate %s", want)
		}
	}
	if len(index.Edges) != 0 {
		t.Fatal("discovery fabricated a runtime relationship")
	}
}

func TestConcernMetadataReserveAccountsForUTF8Bytes(t *testing.T) {
	concerns := []ContextConcern{{Kind: contextConcernProject, Project: strings.Repeat("ä", 64)}}
	reserve, err := contextConcernMetadataTokens(concerns)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(ContextPack{})
	after, _ := json.Marshal(ContextPack{Concerns: concerns, SourceCoverage: "complete"})
	if reserve*4 < len(after)-len(before) {
		t.Fatal("concern reserve undercounts UTF-8 bytes")
	}
}

func TestAdaptiveOmissionReserveIncludesVerificationMetadata(t *testing.T) {
	pack := ContextPack{ProtocolVersion: AdaptiveV2}
	omissions := []ContextSourceOmission{{Project: "ledger", Path: "InvoiceClient.java", StartLine: 4, EndLine: 18, Role: "contract", Reason: "required client evidence does not fit"}}
	request, err := contextSourceRequestWithOmissionReserve(pack, ContextRequest{BudgetTokens: 4000}, omissions)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(pack)
	pack.SourceOmissions = omissions
	after, _ := json.Marshal(adaptiveContextMetadata(pack))
	if (4000-request.BudgetTokens)*4 < len(after)-len(before) {
		t.Fatal("adaptive verification metadata is not reserved")
	}
}

func TestRequestedEvidenceGermanCompounds(t *testing.T) {
	cases := []struct{ name, query, kind string }{
		{"tests", "Nenne vorhandene Regressionstests und Testdateien.", contextConcernTests},
		{"models", "Nenne betroffene Datenvarianten und Zuordnungsmerkmale.", contextConcernDomainModel},
		{"contracts", "Prüfe interne Schnittstellen über Projektgrenzen.", contextConcernHTTPContract},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if !contextQueryRequestsConcern(test.query, test.kind) {
				t.Errorf("requested evidence %q not recognized", test.kind)
			}
			if test.kind == contextConcernTests && !contextQueryRequestsTests(test.query) {
				t.Error("source gate rejects requested tests")
			}
		})
	}
}

func TestConfigurationSignatureDoesNotProveFields(t *testing.T) {
	section := ContextSourceSection{Path: "TaskConfig.java", Role: "call_chain", RenderMode: "signature",
		Content: "@ConfigurationProperties(prefix = \"taskmgmt\")\npublic class TaskConfig {"}
	if contextSourceSectionSupportsConcern(section, contextConcern{kind: contextConcernConfiguration}) {
		t.Fatal("annotation without configuration fields counted as proof")
	}
}

func TestUnrelatedClientCannotCoverRequestedConfigurationOrRetry(t *testing.T) {
	for _, kind := range []string{contextConcernConfiguration, contextConcernResilience} {
		concern := newContextConcern(kind, "shared", true, []string{"ledger-client"}, "ledger client evidence")
		candidate := sourceCandidate{FactID: "shipping-client", Project: "shared", Path: "ShippingClient.java"}
		section := ContextSourceSection{Project: "shared", Path: candidate.Path, RenderMode: "declaration_body", Content: "public ShippingClient(Config config) { configure(config.readTimeout, config.maxRetries); }"}
		keys, _ := contextSourceOptionConcernsWithAction(candidate, section, []contextConcern{concern}, scan.AgentContextIndexRecord{}, true, false)
		if len(keys) != 0 {
			t.Errorf("unrelated client covered %s", kind)
		}
	}
}

func TestAdaptiveMetadataBudgetRetainsBoundedResponse(t *testing.T) {
	root := writeSourceBackedContextFixture(t, false)
	manifest := scan.OutputManifest{Tool: scan.ToolName, Schema: scan.SchemaVersion,
		GenerationID: strings.Repeat("a", 32), AnalysisCoverage: "partial",
		BuildIdentity: scan.CurrentBuildIdentity(config.Defaults(), scan.DefaultBuildOptions(), "ignore", "source"),
		Agent:         scan.ProjectionStatus{Complete: true, InputFingerprint: "fingerprint", GeneratedAt: "2026-09-10T00:00:00Z"}}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "goregraph-out", "manifest.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, budget := range []int{256, 768, 1100, 4000, 6000} {
		pack, err := BuildContext(ContextRequest{Root: root, ProtocolVersion: AdaptiveV2, BudgetTokens: budget,
			Query: "DELETE /cadasters/{cadasterId}/regulations/{objectId}. Explain authentication, configuration, persistence, side effects and existing regression tests."})
		if err != nil {
			t.Errorf("budget %d returned an API error: %v", budget, err)
			continue
		}
		fits, err := contextPackFitsBudget(pack, budget)
		if err != nil || !fits {
			t.Errorf("budget %d exceeded: %v", budget, err)
		}
		if pack.Health == nil || pack.ProtocolVersion != AdaptiveV2 {
			t.Errorf("budget %d dropped adaptive health", budget)
		}
		if pack.FallbackReason == ContextFallbackIndexMissing || pack.FallbackReason == ContextFallbackUnsupportedAnalysis {
			t.Fatalf("budget test did not exercise valid index: %+v", pack)
		}
	}
}

func TestAdaptiveIncompleteTaskPreservesEntrypointConfidenceAndRequiresFallback(t *testing.T) {
	pack := adaptiveContextMetadata(ContextPack{ProtocolVersion: AdaptiveV2, Confidence: "EXACT", SourceCoverage: "partial",
		Concerns: []ContextConcern{{Kind: contextConcernEntrypoint, Covered: true}, {Kind: contextConcernDomainModel, Covered: false}},
	})
	if !pack.FallbackRequired || pack.Confidence != "EXACT" || pack.FallbackReason != "insufficient_evidence" {
		t.Fatalf("incomplete task decision = %+v", pack)
	}
}

func TestInheritedFieldsDoNotProveConcreteModelIdentity(t *testing.T) {
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "entry", Project: "ledger", Kind: "symbol", Name: "InvoiceEntryEntity", File: "InvoiceEntryEntity.java", Line: 1},
		{ID: "base", Project: "ledger", Kind: "symbol", Name: "BaseEntryEntity", File: "BaseEntryEntity.java", Line: 1},
	}, Edges: []scan.AgentContextEdgeRecord{{FromFactID: "entry", ToFactID: "base", Kind: "extends"}}}
	base := newContextConcern(contextConcernDomainModel, "", true, []string{"entry"}, "requested model")
	base.requireIdentity = true
	concerns := contextDomainModelEvidenceConcerns(base, index, map[string]bool{"entry": true})
	candidate := sourceCandidate{FactID: "base", Project: "ledger", Path: "BaseEntryEntity.java"}
	section := ContextSourceSection{Project: "ledger", Path: candidate.Path, RenderMode: "declaration_body", Role: "domain_model", Content: "class BaseEntryEntity { long invoiceId; }"}
	keys, _ := contextSourceOptionConcernsWithAction(candidate, section, concerns, index, true, false)
	if len(keys) == len(concerns) {
		t.Fatal("base fields alone prove the concrete model")
	}
}

func TestRelatedRetrievalSimpleDomainFindsClient(t *testing.T) {
	seed := scan.AgentContextFactRecord{ID: "route", Project: "entry", Kind: "route", Name: "DELETE /invoices/{invoiceId}", HTTPMethod: "DELETE", Path: "/invoices/{invoiceId}", File: "Invoices.java", Line: 2}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{seed,
		{ID: "model", Project: "ledger", Kind: "symbol", Name: "InvoiceEntity", File: "InvoiceEntity.java", Line: 1},
		{ID: "client", Project: "shared", Kind: "symbol", Name: "InvoiceClient", File: "InvoiceClient.java", Line: 1},
		{ID: "repo", Project: "ledger", Kind: "persistence", Name: "InvoiceRepository", File: "InvoiceRepository.java", Line: 1},
	}}
	concerns := planContextSourceConcerns("DELETE /invoices/{invoiceId}. Explain models, contracts and persistence.", index, seed, AdaptiveV2)
	found := false
	for _, c := range concerns {
		t.Logf("%s %s: %v", c.kind, c.facet, c.candidateFactIDs)
		found = found || slices.Contains(c.candidateFactIDs, "client")
	}
	if !found {
		t.Fatal("related InvoiceClient missing despite exact model/domain stem")
	}
}
func TestRelatedRetrievalSingletonModelNotLostToVariantPair(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{ID: "primary", Project: "entry", Kind: "symbol", Name: "InvoiceEntity", File: "InvoiceEntity.java", Line: 1},
		{ID: "related", Project: "ledger", Kind: "symbol", Name: "InvoiceEntryEntity", File: "InvoiceEntryEntity.java", Line: 1},
		{ID: "related-change", Project: "ledger", Kind: "symbol", Name: "InvoiceEntryChangeEntity", File: "InvoiceEntryChangeEntity.java", Line: 1},
	}
	ids := contextDomainModelConcernCandidates("Invoice models and attributes", nil, nil, facts)
	if !slices.Contains(ids, "primary") {
		t.Fatalf("direct model dropped despite spare cap: %v", ids)
	}
}
func TestRelatedRetrievalConfigEmptyBodyDoesNotProveFields(t *testing.T) {
	section := ContextSourceSection{Path: "TaskConfig.java", RenderMode: "declaration_body", Content: "@ConfigurationProperties(prefix = \"taskmgmt\")\npublic class TaskConfig {}"}
	if contextSourceSectionSupportsConcern(section, contextConcern{kind: contextConcernConfiguration}) {
		t.Fatal("annotation with empty body counted as behavior proof")
	}
}

func TestConfigurationUnrelatedReturnDoesNotProveBehavior(t *testing.T) {
	section := ContextSourceSection{Path: "TaskConfig.java", RenderMode: "declaration_body", Content: "@ConfigurationProperties(prefix = \"taskmgmt\")\npublic class TaskConfig { public String toString() { return \"empty\"; } }"}
	if contextSourceSectionSupportsConcern(section, contextConcern{kind: contextConcernConfiguration}) {
		t.Fatal("unrelated toString return counted as configuration behavior")
	}
}
