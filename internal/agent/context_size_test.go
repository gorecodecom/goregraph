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
	if len(body) > 7200 {
		t.Fatalf(
			"serialized context = %d bytes, want at most 7200 (runes=%d estimated=%d files=%d entrypoints=%d relationships=%d contracts=%d persistence=%d tests=%d uncertainties=%d)",
			len(body),
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
			Name:       query,
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
	return writeContextIndexFixture(t, index)
}
