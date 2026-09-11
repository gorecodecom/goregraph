package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveFallbackRetainsBothVerifiedCandidates(t *testing.T) {
	root := t.TempDir()
	facts := []scan.AgentContextFactRecord{}
	for _, name := range []string{"CardPaymentController", "TransferPaymentController"} {
		writeSourceFile(t, root, name+".java", "class "+name+" {\n void capture() { ledger.recordPayment(); }\n}\n")
		facts = append(facts, scan.AgentContextFactRecord{ID: name, Kind: "symbol", Name: name, Qualified: name, File: name + ".java", Line: 1, EndLine: 3, Confidence: "EXACT"})
	}
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, Facts: facts})
	query := "Trace the payment capture handler and describe any ambiguity."
	pack, err := BuildContext(ContextRequest{Root: root, Query: query, ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || pack.Confidence != "LOW" || len(pack.Entrypoints) != 0 || len(pack.Endpoints) != 0 {
		t.Fatalf("ambiguous candidates became a verified entrypoint: %#v", pack)
	}
	if len(pack.SourceSections) != 2 || pack.SourceCoverage != "partial" {
		t.Fatalf("fallback discarded candidate source: %#v", pack)
	}
	for _, section := range pack.SourceSections {
		if !strings.Contains(section.Content, "capture()") || section.Role != "candidate" {
			t.Fatalf("unverified or mislabeled candidate: %#v", section)
		}
	}
	strict, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil || len(strict.SourceSections) != 0 || !strict.FallbackRequired {
		t.Fatalf("strict fallback changed: %#v, %v", strict, err)
	}
	bounded, err := BuildContext(ContextRequest{Root: root, Query: query, ProtocolVersion: AdaptiveV2, BudgetTokens: MinContextBudgetTokens, MaxFiles: 1})
	if err != nil || bounded.EstimatedTokens > MinContextBudgetTokens || len(bounded.SourceSections) > 1 {
		t.Fatalf("fallback exceeded its budget: %#v, %v", bounded, err)
	}
	writeSourceFile(t, root, "CardPaymentController.java", "class CardPaymentController {\n void capture() { ledger.recordChangedPayment(); }\n}\n")
	changed, err := BuildContext(ContextRequest{Root: root, Query: query, ProtocolVersion: AdaptiveV2, PreviousContextID: pack.ContextID})
	if err != nil || changed.DuplicateOf != "" || changed.ContextID == pack.ContextID {
		t.Fatalf("changed candidate source was hidden as a duplicate: %#v, %v", changed, err)
	}
}

func TestAdaptiveFallbackIdentityPreservesFileContentAssociation(t *testing.T) {
	root := t.TempDir()
	paths := []string{"CardPayment.ts", "TransferPayment.ts"}
	bodies := []string{
		"export function capturePayment() { ledger.recordCard(); }\n",
		"export function capturePayment() { ledger.recordTransfer(); }\n",
	}
	var facts []scan.AgentContextFactRecord
	for i, path := range paths {
		writeSourceFile(t, root, path, bodies[i])
		facts = append(facts, scan.AgentContextFactRecord{ID: path, Kind: "symbol", Name: "capturePayment", File: path, Line: 1, EndLine: 1, Confidence: "EXACT"})
	}
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, Facts: facts})
	request := ContextRequest{Root: root, Query: "Trace payment capture behavior and ambiguity.", ProtocolVersion: AdaptiveV2}
	before, err := BuildContext(request)
	if err != nil || len(before.SourceSections) != 2 || !before.FallbackRequired {
		t.Fatalf("candidate source unavailable: %#v, %v", before, err)
	}
	for i, path := range paths {
		writeSourceFile(t, root, path, bodies[1-i])
	}
	request.PreviousContextID = before.ContextID
	after, err := BuildContext(request)
	if err != nil || after.ContextID == before.ContextID || after.DuplicateOf != "" || len(after.SourceSections) != 2 {
		t.Fatalf("swapped file contents were suppressed as a duplicate: %#v, %v", after, err)
	}
}

func TestAdaptiveContextDetectsChangedRouteBeforeDuplicateSuppression(t *testing.T) {
	root := t.TempDir()
	old := "@RestController\nclass OrderController {\n @PostMapping(\"/v1/orders\")\n public void create() {}\n}\n"
	writeSourceFile(t, root, "OrderController.java", old)
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	request := ContextRequest{Root: root, Query: "POST /v1/orders", ProtocolVersion: AdaptiveV2}
	before, err := BuildContext(request)
	if err != nil || before.FallbackRequired || len(before.SourceSections) == 0 {
		t.Fatalf("initial route not retrieved: %#v, %v", before, err)
	}
	writeSourceFile(t, root, "OrderController.java", strings.ReplaceAll(old, "/v1/orders", "/v2/orders"))
	request.PreviousContextID = before.ContextID
	after, err := BuildContext(request)
	if err != nil || !after.FallbackRequired || after.FallbackReason != "evidence_conflict" || after.DuplicateOf != "" {
		t.Fatalf("changed route reused stale metadata: %#v, %v", after, err)
	}
	if len(after.Endpoints) != 0 || after.Health == nil || after.Health.Freshness != "stale" {
		t.Fatalf("stale route still presented as current: %#v", after)
	}
	for budget := MinContextBudgetTokens; budget <= 700; budget++ {
		boundedRequest := request
		boundedRequest.BudgetTokens = budget
		bounded, err := BuildContext(boundedRequest)
		// Some budgets cannot fit the required endpoint metadata before source selection.
		if err != nil && strings.Contains(err.Error(), "required context concerns exceed metadata budget") {
			continue
		}
		if err != nil || bounded.EstimatedTokens > budget {
			t.Fatalf("changed-source fallback exceeds budget %d: %#v, %v", budget, bounded, err)
		}
	}
	for _, section := range after.SourceSections {
		if strings.Contains(section.Content, "/v2/orders") {
			return
		}
	}
	t.Fatalf("current route missing from conflict evidence: %#v", after.SourceSections)
}

func TestAdaptiveLowRelevanceDoesNotClaimUnsupportedAnalysis(t *testing.T) {
	for _, reason := range []string{"no sufficiently relevant context fact found", "context confidence is low; inspect source directly"} {
		pack := adaptiveContextMetadata(ContextPack{ProtocolVersion: AdaptiveV2, FallbackRequired: true, FallbackReason: reason})
		if pack.FallbackReason != "insufficient_relevance" {
			t.Errorf("low relevance mislabeled as %q", pack.FallbackReason)
		}
	}
}

func TestAdaptiveContextDetectsDeletedSourceBeforeDuplicateSuppression(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "OrderController.java", "@RestController\nclass OrderController {\n @PostMapping(\"/orders\")\n public void create() {}\n}\n")
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	request := ContextRequest{Root: root, Query: "POST /orders", ProtocolVersion: AdaptiveV2}
	before, err := BuildContext(request)
	if err != nil || before.FallbackRequired {
		t.Fatalf("initial context unavailable: %#v, %v", before, err)
	}
	if err := os.Remove(filepath.Join(root, "OrderController.java")); err != nil {
		t.Fatal(err)
	}
	request.PreviousContextID = before.ContextID
	after, err := BuildContext(request)
	if err != nil || after.DuplicateOf != "" || !after.FallbackRequired || after.FallbackReason != ContextFallbackSourceUnreadable || len(after.Endpoints) != 0 {
		t.Fatalf("deleted source reused as a valid duplicate: %#v, %v", after, err)
	}
}

func TestAdaptiveFallbackKeepsConfigurationVocabularyAndRedactsValues(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "application.yml", "inventory:\n  service-token: do-not-disclose\n  base-url: https://internal.invalid\n")
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts:         []scan.AgentContextFactRecord{{ID: "config", Kind: "configuration", Name: "inventory", File: "application.yml", Line: 1, EndLine: 3, Confidence: "EXACT"}},
	})
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Add configured service authentication to inventory requests.", ProtocolVersion: AdaptiveV2})
	if err != nil || len(pack.SourceSections) != 1 {
		t.Fatalf("inventory configuration evidence missing: %#v, %v", pack, err)
	}
	if strings.Contains(pack.SourceSections[0].Content, "do-not-disclose") || strings.Contains(pack.SourceSections[0].Content, "internal.invalid") {
		t.Fatal("configuration values leaked in fallback evidence")
	}
}

func TestAdaptiveFallbackOffersBoundedVerificationForOmittedCandidates(t *testing.T) {
	root := t.TempDir()
	facts := []scan.AgentContextFactRecord{}
	for i := range 5 {
		name := fmt.Sprintf("Payment%d", i)
		writeSourceFile(t, root, name+".java", "class "+name+" {\n void capture() {}\n}\n")
		facts = append(facts, scan.AgentContextFactRecord{ID: name, Kind: "symbol", Name: name, Qualified: name, File: name + ".java", Line: 1, EndLine: 3, Confidence: "EXACT"})
	}
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, Facts: facts})
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain payment capture ambiguity.", ProtocolVersion: AdaptiveV2})
	if err != nil || len(pack.SourceSections) != 3 || len(pack.VerificationRequests) != 2 {
		t.Fatalf("candidate omissions lack bounded verification: %#v, %v", pack, err)
	}
	for _, request := range pack.VerificationRequests {
		if request.StartLine != 1 || request.EndLine != 3 || !strings.HasPrefix(request.Path, "Payment") {
			t.Fatalf("verification widened beyond the rendered candidate: %#v", request)
		}
		for _, section := range pack.SourceSections {
			if section.Path == request.Path {
				t.Fatal("already supplied source was requested again")
			}
		}
	}
}

func TestAdaptiveFallbackRejectsUnindexedAndEscapingSources(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "UnindexedPayment.java", "class UnindexedPayment { void capture() {} }")
	outside := t.TempDir()
	writeSourceFile(t, outside, "Payment.java", "class Payment { void capture() {} }")
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts:         []scan.AgentContextFactRecord{{ID: "unsafe", Kind: "symbol", Name: "Payment", File: filepath.Join(outside, "Payment.java"), Line: 1}},
	})
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain payment capture ambiguity.", ProtocolVersion: AdaptiveV2})
	if err != nil || len(pack.SourceSections) != 0 || len(pack.VerificationRequests) != 0 {
		t.Fatalf("unindexed or escaping candidate exposed: %#v, %v", pack, err)
	}
}
