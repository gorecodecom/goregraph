package agent

import (
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestSourceInventoryKeepsDirectMutationBeforeSupportingFiles(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"InvoiceController.java":   "class InvoiceController {\n public void deleteInvoice(String id) {\n  operations.deleteInvoice(id);\n }\n}\n",
		"InvoiceOperations.java":   "class InvoiceOperations {\n public void deleteInvoice(String id) {\n  repository.deleteById(id);\n }\n}\n",
		"InvoiceClientConfig.java": "@ConfigurationProperties(prefix = \"invoice\")\nclass InvoiceClientConfig {\n private String baseUrl;\n}\n",
		"InvoiceRetryConfig.java":  "@ConfigurationProperties(prefix = \"invoice.retry\")\nclass InvoiceRetryConfig {\n private int maxRetries;\n}\n",
	}
	for path, content := range files {
		writeSourceFile(t, root, path, content)
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "entry", Kind: "route", Name: "DELETE /invoices/{id}", HTTPMethod: "DELETE", Path: "/invoices/{id}", Qualified: "InvoiceController.deleteInvoice", File: "InvoiceController.java", Line: 2, EndLine: 4, Confidence: "EXACT"},
		{ID: "mutation", Kind: "symbol", Name: "deleteInvoice", Qualified: "InvoiceOperations.deleteInvoice", File: "InvoiceOperations.java", Line: 2, EndLine: 4, Confidence: "EXACT"},
		{ID: "config", Kind: "configuration", Name: "InvoiceClientConfig", File: "InvoiceClientConfig.java", Line: 2, EndLine: 4, Confidence: "EXACT"},
		{ID: "retry", Kind: "configuration", Name: "InvoiceRetryConfig", File: "InvoiceRetryConfig.java", Line: 2, EndLine: 4, Confidence: "EXACT"},
	}, Edges: []scan.AgentContextEdgeRecord{{ID: "call", FromFactID: "entry", ToFactID: "mutation", Kind: "call", Confidence: "EXACT"}, {ID: "binding", FromFactID: "entry", ToFactID: "config", Kind: "configuration", Confidence: "EXACT"}, {ID: "retry-binding", FromFactID: "entry", ToFactID: "retry", Kind: "configuration", Confidence: "EXACT"}}}
	pack := ContextPack{Schema: 1, ProtocolVersion: AdaptiveV2, Query: "DELETE /invoices/{id}: show call chain, persistence and exact production configuration files with symbols and source lines.", Confidence: "HIGH", BudgetTokens: 4000,
		Entrypoints:           []ContextLocation{{ID: "entry", File: "InvoiceController.java", Line: 2}},
		Files:                 []ContextFile{{Path: "InvoiceController.java", Role: "entrypoint"}},
		Concerns:              []ContextConcern{{Kind: contextConcernEntrypoint}, {Kind: contextConcernPrimaryPath}, {Kind: contextConcernConfiguration}},
		selectedSourceFactIDs: []string{"entry", "mutation"},
	}
	loaded := loadedContextIndex{ScopeRoot: root, Index: index}
	tight, err := attachContextSource(pack, loaded, ContextRequest{BudgetTokens: 4000, MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, concern := range tight.Concerns {
		if concern.Kind == contextConcernPrimaryPath && concern.Covered {
			t.Error("entrypoint alone claimed a proven primary path")
		}
	}
	decision := adaptiveContextMetadata(tight)
	if !decision.FallbackRequired || decision.FallbackReason != ContextFallbackInsufficientEvidence {
		t.Fatalf("missing primary body did not require source fallback: %+v", decision)
	}
	foundGap := false
	for _, omission := range tight.SourceOmissions {
		foundGap = foundGap || omission.Path == "InvoiceOperations.java" && omission.StartLine == 2 && omission.EndLine == 4
	}
	if !foundGap || contextSourceFileCount(tight) > 1 {
		t.Fatalf("missing bounded primary-body gap: %+v", tight.SourceOmissions)
	}
	got, err := attachContextSource(pack, loaded, ContextRequest{BudgetTokens: 4000, MaxFiles: 2})
	if err != nil {
		t.Fatal(err)
	}
	if contextSourceFileCount(got) > 2 {
		t.Fatal("core reservation exceeded the file limit")
	}
	for _, section := range got.SourceSections {
		if section.Path == "InvoiceOperations.java" && strings.Contains(section.Content, "repository.deleteById(id)") {
			return
		}
	}
	t.Fatalf("direct mutation body displaced by supporting inventory: files=%+v sections=%+v", got.Files, got.SourceSections)
}

func TestCoreInventoryReservationCountsMetadataPathsOnce(t *testing.T) {
	pack := ContextPack{
		Entrypoints: []ContextLocation{{ID: "entry", File: "Entry.java"}},
		Files:       []ContextFile{{Path: "Supporting.java", Role: "call_chain"}},
	}
	options := []contextSourceOption{{candidate: sourceCandidate{FactID: "entry", Path: "Entry.java"}, section: ContextSourceSection{Path: "Entry.java"}}}
	got := reserveContextCoreSourceInventory(pack, options, nil, []contextSourceBoundary{{factID: "entry"}}, 2)
	if len(got.Files) != 1 || got.Files[0].Path != "Supporting.java" {
		t.Fatalf("metadata-only entrypoint counted twice: %+v", got.Files)
	}
}

func TestAdaptiveVerificationPrioritizesMissingPrimaryBody(t *testing.T) {
	primary := newExpandedContextEvidenceConcern(newContextConcern(contextConcernPrimaryPath, "", true, []string{"mutation"}, "primary path"), "primary_declaration:mutation", []string{"mutation"}, "primary declaration body is required")
	concerns := []contextConcern{primary}
	candidates := []sourceCandidate{{FactID: "mutation", Path: "InvoiceOperations.java", StartLine: 2, EndLine: 4, Role: "call_chain"}}
	for _, name := range []string{"ClientConfig.java", "RetryConfig.java", "AppConfig.java"} {
		concern := newExpandedContextEvidenceConcern(newContextConcern(contextConcernConfiguration, "", true, []string{name}, "configuration files"), "exact-file:"+name, []string{name}, "exact file inventory evidence")
		concern.exactInventory = true
		concerns = append(concerns, concern)
		candidates = append(candidates, sourceCandidate{FactID: name, Path: name, StartLine: 1, EndLine: 3, Role: "call_chain"})
	}
	omissions := contextSourceEvidenceOmissionsWithOptions(ContextPack{ProtocolVersion: AdaptiveV2, Query: "exact production configuration files and source lines"}, scan.AgentContextIndexRecord{}, concerns, candidates, nil, nil, nil)
	if len(omissions) == 0 || omissions[0].Path != "InvoiceOperations.java" {
		t.Fatalf("missing primary body displaced by inventory gaps: %+v", omissions)
	}
}

func TestAdaptiveSourceInventoryDoesNotBlockRelatedModelBodies(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"InvoiceController.java":       "class InvoiceController {\n public void deleteInvoice(String id) {\n  repository.deleteById(id);\n }\n}\n",
		"InvoiceTaskEntity.java":       "class InvoiceTaskEntity {\n private String invoiceId;\n private String taskId;\n}\n",
		"InvoiceChangeTaskEntity.java": "class InvoiceChangeTaskEntity {\n private String invoiceId;\n private String taskId;\n private String changeId;\n}\n",
		"InvoiceClientConfig.java":     "class InvoiceClientConfig {\n private String baseUrl;\n}\n",
		"InvoiceRetryConfig.java":      "class InvoiceRetryConfig {\n private int maxRetries;\n}\n",
	}
	for path, content := range files {
		writeSourceFile(t, root, path, content)
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "entry", Kind: "route", Name: "DELETE /invoices/{id}", HTTPMethod: "DELETE", Path: "/invoices/{id}", Qualified: "InvoiceController.deleteInvoice", File: "InvoiceController.java", Line: 2, EndLine: 4, Confidence: "EXACT"},
		{ID: "task", Kind: "symbol", Name: "InvoiceTaskEntity", File: "InvoiceTaskEntity.java", Line: 1, EndLine: 4, Confidence: "EXACT"},
		{ID: "change", Kind: "symbol", Name: "InvoiceChangeTaskEntity", File: "InvoiceChangeTaskEntity.java", Line: 1, EndLine: 5, Confidence: "EXACT"},
		{ID: "config", Kind: "configuration", Name: "InvoiceClientConfig", File: "InvoiceClientConfig.java", Line: 1, EndLine: 3, Confidence: "EXACT"},
		{ID: "retry", Kind: "configuration", Name: "InvoiceRetryConfig", File: "InvoiceRetryConfig.java", Line: 1, EndLine: 3, Confidence: "EXACT"},
	}}
	pack := ContextPack{Schema: 1, ProtocolVersion: AdaptiveV2, Query: "DELETE /invoices/{id}: InvoiceTaskEntity and InvoiceChangeTaskEntity data variants, identity fields and exact production configuration files.", Confidence: "HIGH", BudgetTokens: 4000,
		Entrypoints:           []ContextLocation{{ID: "entry", File: "InvoiceController.java", Line: 2}},
		Files:                 []ContextFile{{Path: "InvoiceController.java", Role: "entrypoint"}, {Path: "InvoiceClientConfig.java", Role: "call_chain"}, {Path: "InvoiceRetryConfig.java", Role: "call_chain"}},
		Concerns:              []ContextConcern{{Kind: contextConcernEntrypoint}, {Kind: contextConcernDomainModel}, {Kind: contextConcernConfiguration}},
		selectedSourceFactIDs: []string{"entry", "task", "change"},
	}
	for i := range index.Facts {
		index.Facts[i].Project = "billing"
	}
	pack.Entrypoints[0].Project = "billing"
	for i := range pack.Files {
		pack.Files[i].Project = "billing"
	}
	got, err := attachContextSource(pack, loadedContextIndex{ScopeRoot: root, Index: index}, ContextRequest{BudgetTokens: 4000, MaxFiles: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"InvoiceTaskEntity.java", "InvoiceChangeTaskEntity.java"} {
		found := false
		for _, section := range got.SourceSections {
			if section.Path == path && strings.Contains(section.Content, "invoiceId") {
				found = true
			}
		}
		if !found {
			t.Errorf("related model body %s blocked by inventory: %+v", path, got.SourceSections)
		}
	}
	if contextSourceFileCount(got) > 3 {
		t.Fatal("source selection exceeded the aggregate file limit")
	}
}

func TestAdaptiveSourceCanReplaceUnrenderedSupportingInventory(t *testing.T) {
	pack := ContextPack{Schema: 1, ProtocolVersion: AdaptiveV2, BudgetTokens: 4000,
		Files:          []ContextFile{{Path: "Entry.java", Role: "entrypoint"}, {Path: "SupportingConfig.java", Role: "call_chain"}},
		SourceSections: []ContextSourceSection{{Path: "Entry.java", Content: "1: void remove() {}", StartLine: 1, EndLine: 1}},
	}
	option := contextSourceOption{candidate: sourceCandidate{FactID: "client", Path: "InvoiceClient.java", Role: "contract"}, section: ContextSourceSection{Path: "InvoiceClient.java", Content: "1: void remove(String id) { transport.delete(id); }", StartLine: 1, EndLine: 1}, concernKeys: []string{"client"}}
	concerns := []contextConcern{{key: "client", kind: contextConcernHTTPContract, required: true, candidateFactIDs: []string{"client"}}}
	request := ContextRequest{BudgetTokens: 4000, MaxFiles: 2}
	state := newContextSourceSelectionState(1, 1)
	fits, err := contextSourceOptionFits(pack, request, option, concerns, state)
	if err != nil || !fits {
		t.Fatalf("unrendered inventory blocked required client evidence: fits=%v err=%v", fits, err)
	}
	got, _, err := addContextSourceOption(pack, request, option, concerns, state)
	if err != nil {
		t.Fatal(err)
	}
	if contextSourceFileCount(got) != 2 || len(got.SourceSections) != 2 {
		t.Fatalf("replacement lost evidence or exceeded file limit: %+v", got)
	}
	for _, file := range got.Files {
		if file.Path == "SupportingConfig.java" {
			t.Fatal("unrendered support still occupies a file slot")
		}
	}
	pack.ProtocolVersion = StrictV1
	fits, err = contextSourceOptionFits(pack, request, option, concerns, state)
	if err != nil || fits {
		t.Fatalf("strict inventory behavior changed: fits=%v err=%v", fits, err)
	}
}

func TestAdaptiveExactInventoryDoesNotReintroduceUnrelatedConfiguration(t *testing.T) {
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "invoice", Project: "shared", Kind: "symbol", Name: "InvoiceClientConfig", File: "InvoiceClientConfig.java", Line: 1, EndLine: 10, Confidence: "EXACT", Search: "invoice baseUrl username password retry timeout"},
		{ID: "license", Project: "shared", Kind: "symbol", Name: "LicenseClientConfig", File: "LicenseClientConfig.java", Line: 1, EndLine: 10, Confidence: "EXACT", Search: "license baseUrl username password retry timeout"},
	}}
	concerns := []contextConcern{newContextConcern(contextConcernConfiguration, "", true, []string{"invoice"}, "requested configuration")}
	pack := ContextPack{ProtocolVersion: AdaptiveV2, Query: "Invoice deletion: exact production configuration file inventory with authentication and retry."}
	got := contextExactInventoryEvidenceConcerns(pack, index, concerns)
	found := false
	for _, concern := range got {
		for _, id := range concern.candidateFactIDs {
			if id == "license" {
				t.Error("generic auth/retry properties reintroduced unrelated license configuration")
			}
			found = found || id == "invoice"
		}
	}
	if !found {
		t.Fatal("relevant invoice configuration was lost")
	}
}

func TestAdaptiveVerificationKeepsMissingProofAheadOfExtraInventory(t *testing.T) {
	model := newExpandedContextEvidenceConcern(newContextConcern(contextConcernDomainModel, "", true, []string{"model"}, "data variants"), "model_identity:model", []string{"model"}, "concrete model identity is missing")
	concerns := []contextConcern{model}
	candidates := []sourceCandidate{{FactID: "model", Path: "InvoiceTaskEntity.java", StartLine: 1, EndLine: 5, Role: "domain_model"}}
	for _, name := range []string{"ApplicationConfig.java", "ClientConfig.java", "RetryConfig.java"} {
		concern := newExpandedContextEvidenceConcern(newContextConcern(contextConcernConfiguration, "", true, []string{name}, "configuration files"), "exact-file:"+name, []string{name}, "exact file inventory evidence")
		concern.exactInventory = true
		concerns = append(concerns, concern)
		candidates = append(candidates, sourceCandidate{FactID: name, Path: name, StartLine: 1, EndLine: 3, Role: "call_chain"})
	}
	pack := ContextPack{ProtocolVersion: AdaptiveV2, Query: "exact production configuration files and data variants"}
	omissions := contextSourceEvidenceOmissionsWithOptions(pack, scan.AgentContextIndexRecord{}, concerns, candidates, nil, nil, nil)
	if len(omissions) == 0 || omissions[0].Path != "InvoiceTaskEntity.java" {
		t.Fatalf("extra inventory displaced missing model proof: %+v", omissions)
	}
}
