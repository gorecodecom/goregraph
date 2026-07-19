package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDefaultContextPackStaysWithinTokenAndByteBudgets(t *testing.T) {
	const factCount = 220
	root := writeDenseContextIndexFixture(t, factCount)
	request := ContextRequest{
		Root:  root,
		Query: "delete regulation tasks across services",
	}

	pack, err := BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}

	if pack.FallbackRequired || pack.Confidence == "LOW" {
		t.Fatalf("dense fixture produced a fallback or low-confidence pack: %#v", pack)
	}
	if len(pack.Entrypoints) == 0 || len(pack.CallChain) == 0 || len(pack.Files) == 0 {
		t.Fatalf("dense fixture produced a trivial pack: %#v", pack)
	}
	if len(pack.SourceSections) < 1 || len(pack.SourceSections) > MaxContextSourceSections ||
		len(pack.SourceOmissions) > MaxContextSourceOmissions {
		t.Fatalf("source bounds = %#v", pack)
	}
	if pack.SourceCoverage != "complete" && pack.SourceCoverage != "partial" && pack.SourceCoverage != "none" {
		t.Fatalf("source coverage = %q", pack.SourceCoverage)
	}
	if pack.SourceUnrepresented < 0 {
		t.Fatalf("source unrepresented = %d", pack.SourceUnrepresented)
	}
	for _, concern := range pack.Concerns {
		if concern.Covered != (pack.SourceCoverage == "complete") && pack.SourceCoverage != "partial" {
			t.Fatalf("concern/source coverage mismatch: concern=%#v source=%q", concern, pack.SourceCoverage)
		}
	}
	if pack.BudgetTokens != DefaultContextBudgetTokens {
		t.Fatalf("budget tokens = %d, want default %d", pack.BudgetTokens, DefaultContextBudgetTokens)
	}
	estimated, err := EstimateContextTokens(pack)
	if err != nil {
		t.Fatal(err)
	}
	if pack.EstimatedTokens != estimated || pack.EstimatedTokens > DefaultContextBudgetTokens {
		t.Fatalf(
			"estimated tokens = %d, recalculated = %d, budget = %d",
			pack.EstimatedTokens,
			estimated,
			DefaultContextBudgetTokens,
		)
	}
	if len(body) > contextByteBudget(DefaultContextBudgetTokens) {
		t.Fatalf(
			"serialized context = %d bytes, want at most %d (runes=%d estimated=%d files=%d entrypoints=%d relationships=%d contracts=%d persistence=%d tests=%d uncertainties=%d)",
			len(body),
			DefaultContextMaxBytes,
			utf8.RuneCount(body),
			pack.EstimatedTokens,
			len(pack.Files),
			len(pack.Entrypoints),
			len(pack.CallChain),
			len(pack.Contracts),
			len(pack.Persistence),
			len(pack.Tests),
			len(pack.Uncertainties),
		)
	}
	if len(pack.Files) > DefaultContextMaxFiles || len(pack.Uncertainties) > 3 {
		t.Fatalf("pack bounds = %#v", pack)
	}
	if len(body) <= utf8.RuneCount(body) {
		t.Fatalf("dense fixture did not retain multibyte context: bytes=%d runes=%d", len(body), utf8.RuneCount(body))
	}
	t.Logf(
		"dense Context Pack: bytes=%d runes=%d estimated=%d files=%d relationships=%d",
		len(body),
		utf8.RuneCount(body),
		pack.EstimatedTokens,
		len(pack.Files),
		len(pack.CallChain),
	)

	again, err := BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	againBody, err := json.Marshal(again)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, againBody) {
		t.Fatalf("default dense Context Pack is not deterministic:\nfirst:  %s\nsecond: %s", body, againBody)
	}
	if bytes.Contains(body, []byte("selectedSourceFactIDs")) || bytes.Contains(body, []byte("selected_source_fact_ids")) {
		t.Fatalf("private selected fact state leaked into JSON: %s", body)
	}
}

func TestBuildContextEndpointBlockPreservesExistingHardCeilings(t *testing.T) {
	facts := []scan.AgentContextFactRecord{{
		ID: "endpoint", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders/{id}",
		HTTPMethod: "GET", Path: "/orders/{id}", File: "services/orders/src/Orders.java",
		Summary: "provider orders; security bearer", Search: "GET /orders/{id} services orders bearer",
		Confidence: "EXACT",
	}}
	edges := make([]scan.AgentContextEdgeRecord, 0, 80)
	for index := 0; index < 80; index++ {
		id := fmt.Sprintf("consumer:%03d", index)
		facts = append(facts, scan.AgentContextFactRecord{
			ID: id, Project: fmt.Sprintf("frontend/%03d", index), Kind: "api_consumer", Name: id,
			File: fmt.Sprintf("frontend/%03d/src/api.ts", index), Line: index + 1,
			Summary: "consumer service storefront; auth bearer, oauth2", Confidence: "RESOLVED",
		})
		edges = append(edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + id, FromFactID: id, ToFactID: "endpoint",
			Kind: "consumes_endpoint", Reason: "catalog consumer auth bearer, oauth2",
		})
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion, Facts: facts, Edges: edges,
	})

	pack, err := BuildContext(ContextRequest{Root: root, Query: "who calls GET /orders/{id}"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Endpoints) != 1 || len(pack.Endpoints[0].Consumers) > 8 ||
		pack.Endpoints[0].OmittedConsumers == 0 {
		t.Fatalf("endpoint block bounds = %#v", pack.Endpoints)
	}
	if pack.EstimatedTokens > DefaultContextBudgetTokens || len(pack.Files) > DefaultContextMaxFiles || len(body) > DefaultContextMaxBytes {
		t.Fatalf("hard ceilings exceeded: bytes=%d tokens=%d files=%d", len(body), pack.EstimatedTokens, len(pack.Files))
	}
}

func TestBuildContextEndpointTopFactFitsMinimumBudgetWithoutGenericEntrypoint(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "endpoint", Project: "orders", Kind: "api_endpoint", Name: "GET /orders",
			HTTPMethod: "GET", Path: "/orders", Summary: "provider orders; security unknown",
			Search: "GET /orders", Confidence: "EXACT",
		}},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "GET /orders", BudgetTokens: MinContextBudgetTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.Endpoints) != 1 || len(pack.Entrypoints) != 0 {
		t.Fatalf("minimum-budget endpoint pack = %#v", pack)
	}
	if pack.EstimatedTokens > MinContextBudgetTokens {
		t.Fatalf("minimum budget exceeded: %#v", pack)
	}
}

func TestBuildContextSourceMinimumBudgetFallbackAlwaysFits(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion})
	returnedPacks := 0
	deterministicErrors := 0
	for repeats := 1; repeats <= 200; repeats++ {
		request := ContextRequest{
			Root: root, Query: strings.Repeat("Größe ", repeats), BudgetTokens: MinContextBudgetTokens,
		}
		pack, err := BuildContext(request)
		again, againErr := BuildContext(request)
		if fmt.Sprint(err) != fmt.Sprint(againErr) {
			t.Fatalf("repeat %d produced nondeterministic errors: %v != %v", repeats, err, againErr)
		}
		if err != nil {
			deterministicErrors++
			continue
		}
		returnedPacks++
		body, marshalErr := json.Marshal(pack)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		againBody, marshalErr := json.Marshal(again)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if !bytes.Equal(body, againBody) {
			t.Fatalf("repeat %d fallback JSON is not deterministic", repeats)
		}
		estimated, estimateErr := EstimateContextTokens(pack)
		if estimateErr != nil {
			t.Fatal(estimateErr)
		}
		if pack.EstimatedTokens != estimated || estimated > request.BudgetTokens ||
			len(body) > contextByteBudget(request.BudgetTokens) {
			t.Fatalf(
				"repeat %d fallback exceeds budget: bytes=%d/%d tokens=%d/%d pack=%#v",
				repeats, len(body), contextByteBudget(request.BudgetTokens), estimated, request.BudgetTokens, pack,
			)
		}
		if !pack.FallbackRequired || pack.SourceCoverage != "none" || len(pack.SourceSections) != 0 {
			t.Fatalf("repeat %d fallback source state = %#v", repeats, pack)
		}
	}
	if returnedPacks == 0 || deterministicErrors == 0 {
		t.Fatalf("minimum-budget boundary was not exercised: returned=%d errors=%d", returnedPacks, deterministicErrors)
	}
}

func writeDenseContextIndexFixture(t *testing.T, factCount int) string {
	t.Helper()
	if factCount < 2 {
		t.Fatalf("dense fixture fact count = %d, want at least 2", factCount)
	}

	const (
		project = "dienste/vorschriften-änderung"
		query   = "delete regulation tasks across services"
	)
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-17T08:00:00Z",
		Root:          project,
		Facts: []scan.AgentContextFactRecord{{
			ID:         "seed",
			Project:    project,
			Kind:       "symbol",
			Name:       "deleteRegulationTasksAcrossServices",
			Qualified:  "VorschriftenAufgabenService.deleteRegulationTasksAcrossServices",
			File:       "src/main/java/VorschriftenÄnderungsService.java",
			Line:       40,
			EndLine:    68,
			Summary:    "Löscht verknüpfte Vorschriftenaufgaben über mehrere Dienste.",
			Confidence: "EXACT",
			EvidenceIDs: []string{
				"evidence:seed:größenprüfung:001",
				"evidence:seed:änderungsprüfung:002",
			},
			Search: query,
		}},
		Coverage: []scan.AgentContextCoverageRecord{
			{Project: project, Capability: "calls", Coverage: "COMPLETE", Reason: "Aufrufketten vollständig extrahiert"},
			{Project: project, Capability: "api_clients", Coverage: "PARTIAL", Reason: "Dynamische Zieladressen bleiben unvollständig"},
			{Project: project, Capability: "persistence", Coverage: "PARTIAL", Reason: "Native Abfragen bleiben teilweise dynamisch"},
			{Project: project, Capability: "tests", Coverage: "PARTIAL", Reason: "Parametrisierte Tests bleiben teilweise dynamisch"},
		},
	}

	kinds := []struct {
		fact string
		edge string
	}{
		{fact: "symbol", edge: "call"},
		{fact: "api_contract", edge: "http_contract"},
		{fact: "persistence", edge: "persistence"},
		{fact: "test", edge: "test_target"},
	}
	for number := 1; number < factCount; number++ {
		kind := kinds[(number-1)%len(kinds)]
		id := fmt.Sprintf("neighbor-%03d", number)
		name := fmt.Sprintf("ÄnderungsGrößenPrüfung%03d", number)
		fileSlot := (number - 1) % (DefaultContextMaxFiles - 1)
		file := fmt.Sprintf("src/%02d/VorschriftenÄnderungsGrößenPrüfung%02d.java", fileSlot, fileSlot)
		fact := scan.AgentContextFactRecord{
			ID:         id,
			Project:    project,
			Kind:       kind.fact,
			Name:       name,
			Qualified:  "com.weka.vorschriften." + name,
			File:       file,
			Line:       20 + number,
			EndLine:    30 + number,
			Summary:    "Prüft Größe, Änderung, Rückgabe und fachliche Nebenwirkungen der Löschung.",
			Confidence: "RESOLVED",
			Search:     "vorschrift aufgabe änderung größe rückgabe",
		}
		if kind.fact == "api_contract" {
			fact.HTTPMethod = "DELETE"
			fact.Path = fmt.Sprintf("/interne-vorschriften/%03d/aufgaben", number)
		}
		for evidence := 0; evidence < 8; evidence++ {
			fact.EvidenceIDs = append(fact.EvidenceIDs, fmt.Sprintf(
				"evidence:%03d:%02d:präzise-änderungsgrößenprüfung",
				number,
				evidence,
			))
		}
		index.Facts = append(index.Facts, fact)
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID:         "edge-" + id,
			Project:    project,
			FromFactID: "seed",
			ToFactID:   id,
			FromLabel:  "VorschriftenAufgabenService.deleteRegulationTasksAcrossServices",
			ToLabel:    name,
			Kind:       kind.edge,
			File:       file,
			Line:       20 + number,
			Reason: strings.Repeat(
				"präzise Änderung mit Größenprüfung und Rückgabewert; ",
				2,
			),
			Confidence: "RESOLVED",
			EvidenceIDs: []string{
				fmt.Sprintf("edge-evidence:%03d:änderung", number),
				fmt.Sprintf("edge-evidence:%03d:größe", number),
			},
		})
	}

	if len(index.Facts) != factCount || len(index.Edges) != factCount-1 {
		t.Fatalf("dense fixture shape: facts=%d edges=%d", len(index.Facts), len(index.Edges))
	}
	root := writeContextIndexFixture(t, index)
	linesByFile := map[string][]string{}
	for _, fact := range index.Facts {
		lines := linesByFile[fact.File]
		if lines == nil {
			lines = make([]string, 300)
		}
		name := fact.Name
		lines[fact.Line-1] = "public void " + name + "() {}"
		linesByFile[fact.File] = lines
	}
	for path, lines := range linesByFile {
		writeContextSourceFile(t, root, path, strings.Join(lines, "\n"))
	}
	return root
}
