package agent

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveFallbackCandidateRelevance(t *testing.T) {
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion}
	for _, source := range []struct{ project, name string }{
		{"apps/atlas", "AtlasExporter"},
		{"apps/atlas", "RetryWorker"},
		{"apps/beacon", "BeaconRetryWorker"},
		{"apps/atlas-old", "ArchivedRetryWorker"},
		{"apps/beacon", "ShipmentWorker"},
		{"apps/beacon", "behavior"},
	} {
		file := source.name + ".java"
		writeContextSourceFile(t, root, filepath.Join(source.project, file), "class "+source.name+" { void execute() {} }\n")
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: source.name, Project: source.project, Kind: "symbol", Name: source.name,
			Qualified: source.name, File: file, Line: 1, EndLine: 1, Confidence: "EXACT",
		})
	}
	loaded := loadedContextIndex{Index: index, ScopeRoot: root, Workspace: true}
	for _, test := range []struct {
		name, scope, query string
		want               []string
	}{
		{"project", "apps/atlas", "Explain retry behavior.", []string{"RetryWorker.java"}},
		{"later_sentence", "apps/atlas", "Explain the behavior. Inspect retry handling.", []string{"RetryWorker.java"}},
		{"explicit_neighbor", "apps/atlas", "Explain retry behavior in apps/beacon.", []string{"BeaconRetryWorker.java"}},
		{"named_neighbor", "apps/atlas", "Explain the behavior. Inspect BeaconRetryWorker.", []string{"BeaconRetryWorker.java"}},
		{"workspace", "", "Explain retry behavior.", []string{"ArchivedRetryWorker.java", "BeaconRetryWorker.java", "RetryWorker.java"}},
		{"project_group", "apps", "Explain retry behavior.", []string{"ArchivedRetryWorker.java", "BeaconRetryWorker.java", "RetryWorker.java"}},
		{"missing_local_evidence", "apps/atlas", "Explain shipment behavior.", nil},
		{"project_name_only", "apps/atlas", "Welche Qualitätslücken ergeben sich bei Atlas?", nil},
		{"product_comparison", "apps/atlas", "Vergleiche Atlas mit RTK. Wie reduziert das Werkzeug den Tokenverbrauch?", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := normalizeContextRequest(ContextRequest{
				Root: filepath.Join(root, test.scope), Query: test.query, ProtocolVersion: AdaptiveV2,
			})
			if err != nil {
				t.Fatal(err)
			}
			pack, err := fallbackContextPack(index, request, ContextFallbackInsufficientRelevance, nil)
			if err != nil {
				t.Fatal(err)
			}
			pack, err = attachAdaptiveFallbackEvidence(pack, loaded, request)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, section := range pack.SourceSections {
				got = append(got, section.Path)
				if section.Role != "candidate" || section.ReadReceipt == "" {
					t.Fatalf("candidate lost its evidence metadata: %+v", section)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, test.want) || len(pack.SourceOmissions) != 0 || len(pack.VerificationRequests) != 0 {
				t.Fatalf("candidate sources = %v, want %v; omissions=%+v verification=%+v", got, test.want, pack.SourceOmissions, pack.VerificationRequests)
			}
			if !pack.FallbackRequired || pack.Confidence != "LOW" || len(pack.Entrypoints) != 0 {
				t.Fatalf("fallback candidates became verified context: %+v", pack)
			}
		})
	}
}

func TestAdaptiveBroadProjectQuestionDoesNotBorrowWorkspaceEvidence(t *testing.T) {
	root := t.TempDir()
	writeContextSourceFile(t, root, ".goregraph-workspace.yml", "projects: []\n")
	index := scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion}
	for _, project := range []string{"apps/atlas", "apps/neighbor"} {
		writeContextSourceFile(t, root, filepath.Join(project, "AtlasExporter.java"), "class AtlasExporter { void execute() {} }\n")
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: project, Project: project, Kind: "symbol", Name: "AtlasExporter", Qualified: "AtlasExporter",
			File: "AtlasExporter.java", Line: 1, EndLine: 1, Confidence: "EXACT",
		})
	}
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), index)
	pack, err := BuildContext(ContextRequest{
		Root: filepath.Join(root, "apps/atlas"), Query: "Welche Qualitätslücken ergeben sich bei Atlas?", ProtocolVersion: AdaptiveV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || pack.FallbackReason != ContextFallbackInsufficientRelevance || len(pack.SourceSections) != 0 || len(pack.VerificationRequests) != 0 {
		t.Fatalf("broad project question returned unrelated code: %+v", pack)
	}
}

func TestAdaptiveBroadStandaloneProjectQuestionDoesNotMatchProjectName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "atlas")
	writeSourceFile(t, root, "AtlasExporter.java", "class AtlasExporter { void execute() {} }\n")
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "exporter", Kind: "symbol", Name: "AtlasExporter", Qualified: "AtlasExporter",
			File: "AtlasExporter.java", Line: 1, EndLine: 1, Confidence: "EXACT",
		}},
	})
	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "Welche Qualitätslücken ergeben sich bei Atlas?", ProtocolVersion: AdaptiveV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || len(pack.SourceSections) != 0 || len(pack.VerificationRequests) != 0 {
		t.Fatalf("standalone project name became source evidence: %+v", pack)
	}
}

func TestAdaptiveBroadCompoundProjectQuestionDoesNotMatchProjectName(t *testing.T) {
	root := t.TempDir()
	writeContextSourceFile(t, root, ".goregraph-workspace.yml", "projects: []\n")
	writeContextSourceFile(t, root, "atlasgraph/AtlasGraphExporter.java", "class AtlasGraphExporter { void execute() {} }\n")
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "exporter", Project: "atlasgraph", Kind: "symbol", Name: "AtlasGraphExporter", Qualified: "AtlasGraphExporter",
			File: "AtlasGraphExporter.java", Line: 1, EndLine: 1, Confidence: "EXACT",
		}},
	})
	pack, err := BuildContext(ContextRequest{
		Root: filepath.Join(root, "atlasgraph"), Query: "Welche Qualitätslücken ergeben sich bei AtlasGraph?", ProtocolVersion: AdaptiveV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || len(pack.SourceSections) != 0 || len(pack.VerificationRequests) != 0 {
		t.Fatalf("compound project name became source evidence: %+v", pack)
	}
}

func TestAdaptiveFallbackRanksLaterSentenceBeforeReadBudget(t *testing.T) {
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion}
	for i := range maxContextSourceSearchHandlers + 1 {
		name := fmt.Sprintf("Noise%03d", i)
		writeSourceFile(t, root, name+".java", "class "+name+" { void execute() {} }\n")
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: name, Kind: "symbol", Name: name, Qualified: name, File: name + ".java",
			Line: 1, EndLine: 1, Confidence: "EXACT",
		})
	}
	writeSourceFile(t, root, "RetryWorker.java", "class RetryWorker { void execute() {} }\n")
	index.Facts = append(index.Facts, scan.AgentContextFactRecord{
		ID: "RetryWorker", Kind: "symbol", Name: "RetryWorker", Qualified: "RetryWorker",
		File: "RetryWorker.java", Line: 1, EndLine: 1, Confidence: "EXACT",
	})
	request, err := normalizeContextRequest(ContextRequest{
		Root: root, Query: "Explain the behavior. Inspect retry handling.", ProtocolVersion: AdaptiveV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	pack, err := fallbackContextPack(index, request, ContextFallbackInsufficientRelevance, nil)
	if err != nil {
		t.Fatal(err)
	}
	pack, err = attachAdaptiveFallbackEvidence(pack, loadedContextIndex{Index: index, ScopeRoot: root}, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.SourceSections) != 1 || pack.SourceSections[0].Path != "RetryWorker.java" {
		t.Fatalf("later task terms lost behind unrelated read candidates: %+v", pack.SourceSections)
	}
}
