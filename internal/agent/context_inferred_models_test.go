package agent

import (
	"reflect"
	"slices"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveInferredModelsUseNestedResourceIdentity(t *testing.T) {
	seed := scan.AgentContextFactRecord{ID: "route", Project: "gateway", Kind: "route", Name: "DELETE /portfolios/{portfolioId}/invoices/{invoiceId}", Path: "/portfolios/{portfolioId}/invoices/{invoiceId}", HTTPMethod: "DELETE"}
	facts := []scan.AgentContextFactRecord{
		inferredModelFixture("entry", "ledger", "PortfolioInvoiceEntryEntity"),
		inferredModelFixture("change", "ledger", "PortfolioInvoiceEntryChangeEntity"),
		inferredModelFixture("cache", "archive", "PortfolioCacheEntity"),
		inferredModelFixture("cache-change", "archive", "PortfolioCacheChangeEntity"),
		inferredModelFixture("protocol", "audit", "PortfolioProtocolEntity"),
	}
	for i := range facts {
		facts[i].Qualified = "example.portfolioinvoice." + facts[i].Name
	}
	query := "Explain models, configuration, tests and protocol side effects."
	index := scan.AgentContextIndexRecord{Facts: facts}
	got := inferredModelIDs(planContextSourceConcerns(query, index, seed, AdaptiveV2))
	assertInferredModelIDs(t, got, []string{"entry", "change"})
	if len(index.Edges) != 0 {
		t.Fatal("model discovery fabricated runtime edges")
	}
	t.Run("explicit additional model", func(t *testing.T) {
		got := inferredModelIDs(planContextSourceConcerns(query+" Include PortfolioProtocolEntity.", index, seed, AdaptiveV2))
		assertInferredModelIDs(t, got, []string{"entry", "change", "protocol"})
	})
	t.Run("strict unchanged", func(t *testing.T) {
		got := inferredModelIDs(planContextSourceConcerns(query, index, seed, StrictV1))
		want := contextDomainModelConcernCandidates(query+" "+seed.Name+" "+seed.Path, nil, nil, facts)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("strict candidates changed: got %v, want %v", got, want)
		}
	})
	t.Run("explicit project unchanged", func(t *testing.T) {
		query := "Explain archive PortfolioCacheEntity models and attributes."
		got := inferredModelIDs(planContextSourceConcerns(query, index, seed, AdaptiveV2))
		want := inferredModelIDs(planContextConcerns(query, index, seed))
		if !reflect.DeepEqual(got, want) || len(got) == 0 {
			t.Fatalf("explicit project candidates changed: got %v, want %v", got, want)
		}
	})
}

func TestAdaptiveInferredModelsNormalizeResourceTokens(t *testing.T) {
	for _, test := range []struct {
		name, path string
		models     []string
		want       []string
	}{
		{"simple plural", "/invoices/{invoiceId}", []string{"InvoiceEntity", "InvoiceChangeEntity", "InventoryEntity"}, []string{"InvoiceEntity", "InvoiceChangeEntity"}},
		{"singular", "/invoice/{id}", []string{"InvoiceEntity", "InvoiceChangeEntity", "InventoryEntity"}, []string{"InvoiceEntity", "InvoiceChangeEntity"}},
		{"ies plural", "/companies/{companyId}", []string{"CompanyEntity", "CompanyChangeEntity", "CompartmentEntity"}, []string{"CompanyEntity", "CompanyChangeEntity"}},
		{"abbreviated nested resource", "/portfolios/{portfolioId}/documents/{documentId}", []string{"PortfolioDocEntity", "PortfolioDocChangeEntity", "PortfolioDoctorEntity", "PortfolioProtocolEntity"}, []string{"PortfolioDocEntity", "PortfolioDocChangeEntity"}},
		{"abbreviated single resource", "/documents/{id}", []string{"DocEntity", "DocChangeEntity", "DoctorEntity"}, []string{"DocEntity", "DocChangeEntity"}},
		{"ambiguous abbreviation", "/doctors/{id}/documents/{id}", []string{"DocEntity", "DocChangeEntity", "DoctorDocumentEntity"}, []string{"DoctorDocumentEntity"}},
		{"short abbreviation", "/documents/{id}", []string{"DoEntity", "DoChangeEntity", "DocumentEntity"}, []string{"DocumentEntity"}},
		{"package only", "/invoice/{id}", []string{"UnrelatedEntity", "OtherEntity"}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			seed := scan.AgentContextFactRecord{ID: "route", Kind: "route", Project: "gateway", Name: "DELETE " + test.path, Path: test.path, HTTPMethod: "DELETE"}
			var facts []scan.AgentContextFactRecord
			for _, name := range test.models {
				fact := inferredModelFixture(name, "storage", name)
				fact.Qualified = "example.portfolioinvoice." + name
				facts = append(facts, fact)
			}
			got := inferredModelIDs(planContextSourceConcerns("Explain models and attributes.", scan.AgentContextIndexRecord{Facts: facts}, seed, AdaptiveV2))
			assertInferredModelIDs(t, got, test.want)
		})
	}
}

func TestAdaptiveInferredModelsRequireTerminalResource(t *testing.T) {
	seed := scan.AgentContextFactRecord{ID: "route", Project: "gateway", Kind: "route", Name: "DELETE /portfolios/{id}/invoices/{id}", Path: "/portfolios/{id}/invoices/{id}", HTTPMethod: "DELETE"}
	for _, project := range []string{"archive", "ledger"} {
		t.Run("parent model in "+project, func(t *testing.T) {
			index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
				inferredModelFixture("invoice", "ledger", "InvoiceEntity"),
				inferredModelFixture("cache", project, "PortfolioCacheEntity"),
			}}
			got := inferredModelIDs(planContextSourceConcerns("Explain models and attributes.", index, seed, AdaptiveV2))
			assertInferredModelIDs(t, got, []string{"invoice"})
		})
	}
	t.Run("generic and parent-qualified resource models", func(t *testing.T) {
		index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
			inferredModelFixture("invoice", "ledger", "InvoiceEntity"),
			inferredModelFixture("invoice-change", "ledger", "InvoiceChangeEntity"),
			inferredModelFixture("portfolio-invoice", "portfolio-storage", "PortfolioInvoiceEntity"),
		}}
		got := inferredModelIDs(planContextSourceConcerns("Explain models and attributes.", index, seed, AdaptiveV2))
		assertInferredModelIDs(t, got, []string{"invoice", "invoice-change", "portfolio-invoice"})
	})
}

func TestAdaptiveInferredModelsDoNotRequireUnrelatedPayloads(t *testing.T) {
	seed := scan.AgentContextFactRecord{ID: "route", Project: "gateway", Kind: "route", Name: "DELETE /invoices/{id}", Path: "/invoices/{id}", HTTPMethod: "DELETE"}
	for _, suffix := range []string{"Response", "Request", "DTO", "Payload"} {
		t.Run(suffix, func(t *testing.T) {
			name := "InvoiceDetails" + suffix
			index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
				seed,
				inferredModelFixture("invoice", "ledger", "InvoiceEntity"),
				inferredModelFixture("invoice-change", "ledger", "InvoiceChangeEntity"),
				inferredModelFixture("payload", "shared", name),
			}}
			query := "Explain models and attributes."
			got := inferredModelIDs(planContextSourceConcerns(query, index, seed, AdaptiveV2))
			assertInferredModelIDs(t, got, []string{"invoice", "invoice-change"})
			t.Run("explicit declaration", func(t *testing.T) {
				got := inferredModelIDs(planContextSourceConcerns(query+" Include "+name+".", index, seed, AdaptiveV2))
				assertInferredModelIDs(t, got, []string{"invoice", "invoice-change", "payload"})
			})
			t.Run("explicit payload type", func(t *testing.T) {
				got := inferredModelIDs(planContextSourceConcerns(query+" Include "+suffix+" types.", index, seed, AdaptiveV2))
				assertInferredModelIDs(t, got, []string{"invoice", "invoice-change", "payload"})
			})
			t.Run("direct primary dependency", func(t *testing.T) {
				index.Edges = []scan.AgentContextEdgeRecord{{ID: "route-payload", FromFactID: seed.ID, ToFactID: "payload", Kind: "uses", Confidence: "EXACT"}}
				got := inferredModelIDs(planContextSourceConcerns(query, index, seed, AdaptiveV2))
				assertInferredModelIDs(t, got, []string{"invoice", "invoice-change", "payload"})
			})
			t.Run("no concrete entity", func(t *testing.T) {
				index.Facts = []scan.AgentContextFactRecord{seed, inferredModelFixture("payload", "shared", name)}
				index.Edges = nil
				got := inferredModelIDs(planContextSourceConcerns(query, index, seed, AdaptiveV2))
				assertInferredModelIDs(t, got, []string{"payload"})
			})
		})
	}
}

func inferredModelFixture(id, project, name string) scan.AgentContextFactRecord {
	return scan.AgentContextFactRecord{ID: id, Project: project, Kind: "symbol", Name: name, Qualified: name, File: name + ".java", Line: 1}
}

func inferredModelIDs(concerns []contextConcern) []string {
	for _, concern := range concerns {
		if concern.kind == contextConcernDomainModel {
			return concern.candidateFactIDs
		}
	}
	return nil
}

func assertInferredModelIDs(t *testing.T, got, want []string) {
	t.Helper()
	got = slices.Clone(got)
	want = slices.Clone(want)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("inferred models = %v, want %v", got, want)
	}
}
