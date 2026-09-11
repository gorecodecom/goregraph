package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveVerificationPriorityMatchesOmissionRole(t *testing.T) {
	for _, test := range []struct{ kind, role string }{
		{contextConcernHTTPContract, "contract"},
		{contextConcernEntrypoint, "entrypoint"},
		{contextConcernPrimaryPath, "call_chain"},
		{contextConcernSideEffects, "call_chain"},
		{contextConcernConfiguration, "call_chain"},
		{contextConcernAuth, "call_chain"},
		{contextConcernResilience, "call_chain"},
		{contextConcernPersistence, "persistence"},
		{contextConcernTests, "test"},
	} {
		t.Run(test.kind, func(t *testing.T) {
			pack := ContextPack{
				ProtocolVersion: AdaptiveV2,
				Concerns:        []ContextConcern{{Kind: test.kind, Project: "billing"}},
				SourceOmissions: []ContextSourceOmission{
					{Project: "billing", Path: "AInvoice.java", StartLine: 1, EndLine: 4, Role: contextConcernDomainModel, Reason: "missing evidence"},
					{Project: "other", Path: "InvoiceClient.java", StartLine: 2, EndLine: 5, Role: test.role, Reason: "missing evidence"},
					{Project: "billing", Path: "ZInvoiceClient.java", StartLine: 2, EndLine: 5, Role: test.role, Reason: "missing evidence"},
				},
			}
			got := contextVerificationRequests(pack)
			if len(got) != 3 || got[0].Path != "ZInvoiceClient.java" {
				t.Fatalf("uncovered concern promoted unrelated evidence: %+v", got)
			}
			pack.Concerns[0].Covered = true
			if got := contextVerificationRequests(pack); got[0].Path != "AInvoice.java" {
				t.Fatalf("covered concern changed compiler ordering: %+v", got)
			}
		})
	}
}

func TestAdaptiveVerificationPreservesCompilerOrderForTies(t *testing.T) {
	pack := ContextPack{ProtocolVersion: AdaptiveV2}
	for _, name := range []string{"ZPrimaryTest.java", "YRelatedTest.java", "XClientTest.java", "AUnrelatedTest.java"} {
		pack.SourceOmissions = append(pack.SourceOmissions, ContextSourceOmission{
			Project: "billing", Path: name, StartLine: 2, EndLine: 5, Role: "test", Reason: "missing evidence",
		})
	}
	got := contextVerificationRequests(pack)
	if len(got) != 3 || got[0].Path != "ZPrimaryTest.java" || got[1].Path != "YRelatedTest.java" || got[2].Path != "XClientTest.java" {
		t.Fatalf("compiler priority replaced by alphabetical ordering: %+v", got)
	}
	pack.ProtocolVersion = ""
	if legacy := contextVerificationRequests(pack); len(legacy) != 3 || legacy[0].Path != "AUnrelatedTest.java" {
		t.Fatalf("legacy alphabetical ordering changed: %+v", legacy)
	}
}

func TestAdaptiveSourceSelectionRetainsBudgetDisplacedFollowup(t *testing.T) {
	root := t.TempDir()
	const project = "billing"
	query := "Inspect invoice deletion, client configuration, authentication and retry policy."
	pack := ContextPack{Schema: 1, ProtocolVersion: AdaptiveV2, Query: query, Confidence: "EXACT",
		Concerns:              []ContextConcern{{Kind: contextConcernConfiguration}, {Kind: contextConcernAuth}, {Kind: contextConcernResilience}},
		Contracts:             []ContextLocation{{ID: "client", Project: project, Kind: "api_contract", File: "InvoiceClient.java", Line: 2, EndLine: 4}},
		selectedSourceFactIDs: []string{"client"},
	}
	writeSourceFile(t, root, "InvoiceClient.java", "class InvoiceClient {\n  void deleteInvoice() {\n    restClient.delete();\n  }\n}\n")
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "client", Project: project, Kind: "api_contract", Name: "DELETE /invoices/{id}", Qualified: "InvoiceClient.deleteInvoice",
		File: "InvoiceClient.java", Line: 2, EndLine: 4, Confidence: "EXACT",
	}}}
	for _, evidence := range []struct{ kind, name, body string }{
		{"configuration", "InvoiceClientConfig", "  @ConfigurationProperties\n  String baseUrl;"},
		{"authentication", "InvoiceClientAuth", "  void apply() { headers.setBasicAuth(user, password); }"},
		{"resilience", "InvoiceClientRetry", "  @Retryable(maxAttempts = 3)\n  void execute() {}"},
	} {
		content := "class " + evidence.name + " {\n" + evidence.body + "\n"
		for field := range 30 {
			content += fmt.Sprintf("  private String setting%d;\n", field)
		}
		content += "}\n"
		writeSourceFile(t, root, evidence.name+".java", content)
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: evidence.kind, Project: project, Kind: evidence.kind, Name: evidence.name,
			File: evidence.name + ".java", Line: 1, EndLine: strings.Count(content, "\n"), Confidence: "EXACT",
		})
	}
	for _, budget := range []int{900, 950, 4000} {
		t.Run(fmt.Sprint(budget), func(t *testing.T) {
			request := ContextRequest{BudgetTokens: budget, MaxFiles: 4}
			pack.BudgetTokens = budget
			got, err := selectContextSourceOptions(pack, loadedContextIndex{ScopeRoot: root, Index: index}, request)
			if err != nil {
				t.Fatal(err)
			}
			got, err = finalizeContextEstimate(adaptiveContextMetadata(got))
			if err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if len(body) > budget*4 || got.EstimatedTokens > budget {
				t.Fatalf("response exceeds budget: %d bytes, %d tokens, budget %d", len(body), got.EstimatedTokens, budget)
			}
			if contextSourceFileCount(got) > 4 || len(got.SourceSections) > MaxContextSourceSections {
				t.Fatalf("source caps exceeded: %+v", got.SourceSections)
			}
			primaryBody := false
			for _, section := range got.SourceSections {
				if section.Path == "InvoiceClient.java" && strings.Contains(section.Content, "restClient.delete();") && strings.Contains(section.Content, "void deleteInvoice()") {
					primaryBody = true
				}
			}
			if !primaryBody {
				t.Fatalf("primary mutation body lost: %+v", got.SourceSections)
			}
			if budget < 4000 && got.SourceCoverage != "partial" {
				t.Fatalf("fixture did not displace required evidence: %+v", got.Concerns)
			}
			if budget == 4000 && (got.SourceCoverage != "complete" || len(got.SourceSections) != 4) {
				t.Fatalf("follow-up reservation blocked evidence that fits: %+v", got.SourceSections)
			}
			if len(got.VerificationRequests) > 3 {
				t.Fatalf("verification request cap exceeded: %+v", got.VerificationRequests)
			}
			for _, followup := range got.VerificationRequests {
				if followup.Project != project || followup.StartLine < 1 || followup.EndLine > 34 || followup.EndLine < followup.StartLine {
					t.Fatalf("follow-up lost project or bounded source range: %+v", followup)
				}
				for _, section := range got.SourceSections {
					if section.Path == followup.Path && section.StartLine <= followup.EndLine && followup.StartLine <= section.EndLine {
						t.Fatalf("follow-up rereads supplied source: %+v", followup)
					}
				}
			}
			if got.SourceCoverage == "partial" && len(got.VerificationRequests) == 0 {
				t.Fatal("budget-displaced renderable evidence has no bounded follow-up")
			}
		})
	}
}

func TestAdaptiveFollowupReserveSkipsUnavailableAndSuppliedRanges(t *testing.T) {
	pack := ContextPack{ProtocolVersion: AdaptiveV2, SourceSections: []ContextSourceSection{
		{Project: "billing", Path: "Invoice.java", StartLine: 1, EndLine: 8, Content: "void deleteInvoice() { repository.delete(); }"},
	}}
	request := ContextRequest{BudgetTokens: 1000}
	omissions := []ContextSourceOmission{
		{Reason: strings.Repeat("no indexed source candidate ", 1000)},
		{Path: "../outside.java", StartLine: 1, EndLine: 4, Reason: "missing evidence"},
		{Path: "Missing.java", StartLine: 1, EndLine: 4, Reason: "source file is unreadable"},
		{Project: "billing", Path: "Invoice.java", StartLine: 2, EndLine: 7, Reason: "missing evidence"},
	}
	got, err := contextSourceRequestWithFollowupReserve(pack, request, omissions)
	if err != nil {
		t.Fatal(err)
	}
	if got.BudgetTokens != 1000 {
		t.Fatalf("unactionable omissions consumed source budget: %d", got.BudgetTokens)
	}
	omissions = append(omissions, ContextSourceOmission{
		Project: "billing", Path: "Invoice.java", StartLine: 5, EndLine: 12, Reason: "missing body",
	})
	got, err = contextSourceRequestWithFollowupReserve(pack, request, omissions)
	if err != nil {
		t.Fatal(err)
	}
	if got.BudgetTokens >= 1000 {
		t.Fatal("unseen portion of overlapping range did not reserve follow-up space")
	}
	probe := adaptiveContextMetadata(pack)
	before, _ := json.Marshal(probe)
	probe.SourceOmissions = omissions[len(omissions)-1:]
	after, _ := json.Marshal(adaptiveContextMetadata(probe))
	if (1000-got.BudgetTokens)*4 < len(after)-len(before) {
		t.Fatal("reserve did not account for omission and verification serialization")
	}
}

func TestAdaptiveFollowupReserveIsBoundedAndPreservesPrimaryAtMinimumBudget(t *testing.T) {
	pack := ContextPack{ProtocolVersion: AdaptiveV2}
	request := ContextRequest{BudgetTokens: 1000}
	omissions := make([]ContextSourceOmission, 0, 30)
	for index := range 30 {
		omissions = append(omissions, ContextSourceOmission{
			Path: fmt.Sprintf("Source%02d.java", index), StartLine: 2, EndLine: 8, Reason: "missing evidence",
		})
	}
	three, err := contextSourceRequestWithFollowupReserve(pack, request, omissions[:3])
	if err != nil {
		t.Fatal(err)
	}
	all, err := contextSourceRequestWithFollowupReserve(pack, request, omissions)
	if err != nil {
		t.Fatal(err)
	}
	if all.BudgetTokens != three.BudgetTokens || all.BudgetTokens >= 1000 {
		t.Fatalf("unbounded follow-up reserve: three=%d, all=%d", three.BudgetTokens, all.BudgetTokens)
	}
	pack.SourceSections = []ContextSourceSection{{
		Path: "Invoice.java", StartLine: 1, EndLine: 4, Content: "void deleteInvoice() { repository.delete(); }",
	}}
	request.BudgetTokens = 256
	got, err := contextSourceRequestWithFollowupReserve(pack, request, omissions)
	if err != nil {
		t.Fatal(err)
	}
	if got.BudgetTokens != 256 {
		t.Fatalf("minimum source budget shrank: %d", got.BudgetTokens)
	}
}

func TestAdaptiveOmissionCapSkipsWhollySuppliedRanges(t *testing.T) {
	pack := ContextPack{ProtocolVersion: AdaptiveV2, SourceSections: []ContextSourceSection{
		{Project: "billing", Path: "Controller.java", StartLine: 189, EndLine: 207},
		{Project: "billing", Path: "Policy.java", StartLine: 40, EndLine: 45},
	}}
	var concerns []contextConcern
	var candidates []sourceCandidate
	var options []contextSourceOption
	for index, name := range []string{"Controller.java", "Policy.java", "Persistence.java", "Contract.java"} {
		id := fmt.Sprint(index)
		concerns = append(concerns, contextConcern{
			key: id, kind: contextConcernPrimaryPath, required: true, rank: 100 - index,
			candidateFactIDs: []string{id},
		})
		candidate := sourceCandidate{FactID: id, FactIDs: []string{id}, Project: "billing", Path: name, Role: "call_chain", StartLine: 195, EndLine: 195}
		candidates = append(candidates, candidate)
		if index > 0 {
			options = append(options, contextSourceOption{
				candidate: candidate, concernKeys: []string{id},
				section: ContextSourceSection{Project: "billing", Path: name, StartLine: 42, EndLine: 50},
			})
		}
	}
	got := contextSourceEvidenceOmissionsWithOptions(pack, scan.AgentContextIndexRecord{}, concerns, candidates, options, nil, nil)
	if len(got) != 3 || got[0].Path != "Policy.java" || got[1].Path != "Persistence.java" || got[2].Path != "Contract.java" {
		t.Fatalf("supplied range consumed bounded omission slot: %+v", got)
	}
	for _, omission := range got {
		if omission.StartLine != 42 || omission.EndLine != 50 {
			t.Fatalf("rendered option's exact range was changed: %+v", omission)
		}
	}
	pack.SourceOmissions = got
	requests := contextVerificationRequests(pack)
	if len(requests) != 3 || requests[0].StartLine != 46 || requests[0].EndLine != 50 {
		t.Fatalf("useful unseen follow-up lost after omission cap: %+v", requests)
	}
	pack.ProtocolVersion = ""
	legacy := contextSourceEvidenceOmissionsWithOptions(pack, scan.AgentContextIndexRecord{}, concerns, candidates, options, nil, nil)
	if len(legacy) != 3 || legacy[0].Path != "Controller.java" {
		t.Fatalf("legacy omissions changed: %+v", legacy)
	}
	pack.ProtocolVersion = AdaptiveV2
	failures := map[string]string{"0": "source file is unreadable"}
	failed := contextSourceEvidenceOmissionsWithOptions(pack, scan.AgentContextIndexRecord{}, concerns[:1], candidates, options, failures, nil)
	if len(failed) != 1 || failed[0].Reason != "source file is unreadable" {
		t.Fatalf("operational source failure was suppressed: %+v", failed)
	}
	pathless := contextSourceEvidenceOmissionsWithOptions(pack, scan.AgentContextIndexRecord{}, concerns[:1], nil, nil, nil, nil)
	if len(pathless) != 1 || pathless[0].Path != "" || pathless[0].Reason != "required concern has no indexed source candidate" {
		t.Fatalf("pathless uncertainty was suppressed: %+v", pathless)
	}
}

func TestAdaptiveFollowupReserveContinuesPastUnaffordableOmissions(t *testing.T) {
	pack, err := finalizeContextEstimate(ContextPack{
		ProtocolVersion: AdaptiveV2,
		SourceSections: []ContextSourceSection{{
			Path: "Invoice.java", StartLine: 1, EndLine: 4,
			Content: "void deleteInvoice() { repository.delete(); }" + strings.Repeat(" ", 1300),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	omissions := make([]ContextSourceOmission, 0, 4)
	for index := range 3 {
		omissions = append(omissions, ContextSourceOmission{
			Path: fmt.Sprintf("Expensive%d.java", index), StartLine: 2, EndLine: 8,
			Reason: strings.Repeat("missing evidence ", 100),
		})
	}
	omissions = append(omissions, ContextSourceOmission{
		Path: "Policy.java", StartLine: 2, EndLine: 8, Reason: "missing evidence",
	})
	request := ContextRequest{BudgetTokens: 700}
	got, err := contextSourceRequestWithFollowupReserve(pack, request, omissions)
	if err != nil {
		t.Fatal(err)
	}
	if got.BudgetTokens >= 700 {
		t.Fatal("unaffordable omissions hid a later affordable follow-up")
	}
	fits, err := contextSourcePackFits(pack, got)
	if err != nil || !fits {
		t.Fatalf("follow-up reserve displaced primary body: fits=%t, err=%v", fits, err)
	}
}

func TestAdaptiveSourceSelectionAdmitsAffordableOmissionAfterOversizedCandidates(t *testing.T) {
	root := t.TempDir()
	const budget = 700
	pack := ContextPack{
		Schema: 1, ProtocolVersion: AdaptiveV2, Query: "Inspect the selected source evidence.", BudgetTokens: budget,
		Concerns:              []ContextConcern{{Kind: contextConcernEntrypoint}},
		Entrypoints:           []ContextLocation{{ID: "primary", Project: "primary", File: "Invoice.java", Line: 2, EndLine: 4}},
		selectedSourceFactIDs: []string{"primary"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "primary", Project: "primary", Kind: "symbol", Name: "deleteInvoice", Qualified: "Invoice.deleteInvoice",
		File: "Invoice.java", Line: 2, EndLine: 4, Confidence: "EXACT",
	}}}
	writeSourceFile(t, root, "Invoice.java", "class Invoice {\n  void deleteInvoice() {\n    repository.delete(\""+strings.Repeat("x", 1100)+"\");\n  }\n}\n")
	for indexNumber, project := range []string{"a", "b", "c", "z"} {
		path := "Policy.java"
		if indexNumber < 3 {
			path = strings.Repeat(strings.Repeat("x", 180)+"/", 4) + project + "/Policy.java"
		}
		writeSourceFile(t, root, path, "class Policy {\n  void apply() {\n    validate();\n  }\n}\n")
		pack.Concerns = append(pack.Concerns, ContextConcern{Kind: contextConcernProject, Project: project})
		pack.selectedSourceFactIDs = append(pack.selectedSourceFactIDs, project)
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: project, Project: project, Kind: "symbol", Name: "apply", Qualified: "Policy.apply",
			File: path, Line: 2, EndLine: 4, Confidence: "EXACT",
		})
	}
	got, err := selectContextSourceOptions(pack, loadedContextIndex{ScopeRoot: root, Index: index}, ContextRequest{BudgetTokens: budget, MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	got, err = finalizeContextEstimate(adaptiveContextMetadata(got))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.VerificationRequests) != 1 || got.VerificationRequests[0].Path != "Policy.java" || got.VerificationRequests[0].Project != "z" {
		t.Fatalf("oversized omissions hid affordable final follow-up: omissions=%+v, verification=%+v, tokens=%d", got.SourceOmissions, got.VerificationRequests, got.EstimatedTokens)
	}
	if len(got.SourceSections) != 1 || !strings.Contains(got.SourceSections[0].Content, "repository.delete(") {
		t.Fatalf("follow-up displaced primary mutation body: %+v", got.SourceSections)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > budget*4 || got.EstimatedTokens > budget || contextSourceFileCount(got) > 1 {
		t.Fatalf("final response exceeded caps: bytes=%d, tokens=%d, files=%d", len(body), got.EstimatedTokens, contextSourceFileCount(got))
	}
}
