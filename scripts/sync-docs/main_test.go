package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agentguide"
)

func TestSynchronizeDocumentsChecksThenWritesOnlyGeneratedBlocks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "README.md")
	original := "hand-written before\n" +
		"<!-- goregraph:generated example start -->\n" +
		"stale\n" +
		"<!-- goregraph:generated example end -->\n" +
		"hand-written after\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	specs := []documentSpec{{
		Path: "README.md",
		Blocks: []generatedBlock{{
			Name: "example",
			Body: "fresh line one\nfresh line two",
		}},
	}}

	err := synchronizeDocuments(root, specs, false)
	if err == nil || !strings.Contains(err.Error(), "README.md: block example is stale") {
		t.Fatalf("check error = %v, want stale block", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != original {
		t.Fatalf("check mode changed file:\n%s", body)
	}

	if err := synchronizeDocuments(root, specs, true); err != nil {
		t.Fatalf("write mode: %v", err)
	}
	want := "hand-written before\n" +
		"<!-- goregraph:generated example start -->\n" +
		"fresh line one\nfresh line two\n" +
		"<!-- goregraph:generated example end -->\n" +
		"hand-written after\n"
	body, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("written document:\n%s\nwant:\n%s", body, want)
	}
	if err := synchronizeDocuments(root, specs, false); err != nil {
		t.Fatalf("check after write: %v", err)
	}
}

func TestSynchronizeDocumentsAcceptsCurrentCRLFBlocks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "README.md")
	body := "hand-written before\r\n" +
		"<!-- goregraph:generated example start -->\r\n" +
		"fresh line one\r\nfresh line two\r\n" +
		"<!-- goregraph:generated example end -->\r\n" +
		"hand-written after\r\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	specs := []documentSpec{{
		Path: "README.md",
		Blocks: []generatedBlock{{
			Name: "example",
			Body: "fresh line one\nfresh line two",
		}},
	}}

	if err := synchronizeDocuments(root, specs, false); err != nil {
		t.Fatalf("CRLF check: %v", err)
	}
}

func TestGeneratedFactsDescribeImplementedRuntimeDepth(t *testing.T) {
	languageCoverage := renderLanguageCoverage()
	for _, want := range []string{
		"| Java / Spring | Full | Full | Full | Full | Full | Full | Pattern-backed | Pattern-backed | Pattern-backed | Pattern-backed | Full | Full | Provider |",
		"| JavaScript / TypeScript / Node.js / React | Full | Full | Full | Full | Full | Full | Pattern-backed | Pattern-backed | Pattern-backed | Pattern-backed | Full | Full | Consumer + provider |",
		"| Rust | Full | Full | Full | Full | Full | Full | Pattern-backed | Pattern-backed | Pattern-backed | Pattern-backed | — | — | — |",
		"| Shell | Integration | Integration | Integration | Integration | — | — | — | — | — | — | — | — | — |",
		"runtime-generated behavior",
		"Shell integration does not provide routes, tests, or architecture capabilities",
	} {
		if !strings.Contains(languageCoverage, want) {
			t.Fatalf("language coverage is missing %q:\n%s", want, languageCoverage)
		}
	}
	if strings.Contains(languageCoverage, "| Shell | Full |") {
		t.Fatalf("language coverage overstates Shell:\n%s", languageCoverage)
	}

	languageInventory := renderLanguageInventorySummary()
	for _, want := range []string{
		"full adapters for Go, Java / Spring, JavaScript / TypeScript / Node.js / React, PHP, Python, and Rust",
		"Shell integration provides symbols, imports, and calls",
		"does not provide routes, tests, or architecture facts",
		"Index adapters for C, C++, C#, Kotlin, Ruby, Scala, and Swift",
	} {
		if !strings.Contains(languageInventory, want) {
			t.Fatalf("language inventory is missing %q:\n%s", want, languageInventory)
		}
	}
	for _, forbidden := range []string{
		"deep route/API/test analyzers for Go, Java/Spring, JavaScript/TypeScript/Node.js/React, Python, PHP, and Shell",
		"best-effort symbols and imports for Rust",
	} {
		if strings.Contains(languageInventory, forbidden) {
			t.Fatalf("language inventory contains stale claim %q:\n%s", forbidden, languageInventory)
		}
	}

	instruction := renderAgentInstruction()
	wantInstruction := "```text\n" + agentguide.AssistedInstruction + "\n```"
	if instruction != wantInstruction {
		t.Fatalf("instruction block:\n%s\nwant:\n%s", instruction, wantInstruction)
	}

	currentContract := renderCurrentContract()
	for _, want := range []string{"1.3.0", "Schema 3", "unreleased"} {
		if !strings.Contains(currentContract, want) {
			t.Fatalf("current contract is missing %q: %s", want, currentContract)
		}
	}

	releaseEvidence := renderCurrentReleaseEvidenceStatus()
	for _, want := range []string{
		"latest controlled three-by-three release benchmark did not pass",
		"candidate efc21f3",
		"effective-token medians of 160317 baseline and 20715 assisted",
		"87.08% reduction",
		"zero external skill reads",
		"quality medians were 12 baseline and 9 assisted",
		"authentication/configuration and exact production/test-file inventory",
		"Publication remains blocked",
	} {
		if !strings.Contains(releaseEvidence, want) {
			t.Fatalf("release evidence is missing %q: %s", want, releaseEvidence)
		}
	}
	for _, staleValue := range []string{"2551495", "147212", "164295", "39180"} {
		if strings.Contains(releaseEvidence, staleValue) {
			t.Fatalf("release evidence contains stale diagnostic value %q: %s", staleValue, releaseEvidence)
		}
	}

	metrics := renderAgentBenchmarkMetrics()
	for _, want := range []string{
		"effective_tokens",
		"cached_input_tokens",
		"reasoning_output_tokens",
		"external_skill_read_calls",
		"uncached input plus output",
		"reasoning output is already part of output",
		"source_read_calls",
		"bounded_omission_read_calls",
		"unauthorized_source_read_calls",
		"continues to compare total `source_read_calls`",
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("benchmark metrics are missing %q:\n%s", want, metrics)
		}
	}
}

func TestSynchronizeDocumentsRejectsInvalidMarkers(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "unknown marker",
			body: "<!-- goregraph:generated other start -->\nold\n" +
				"<!-- goregraph:generated other end -->\n",
			wantErr: "unknown generated block other",
		},
		{
			name: "duplicate marker",
			body: "<!-- goregraph:generated example start -->\na\n" +
				"<!-- goregraph:generated example end -->\n" +
				"<!-- goregraph:generated example start -->\nb\n" +
				"<!-- goregraph:generated example end -->\n",
			wantErr: "duplicate generated block example",
		},
		{
			name:    "missing end marker",
			body:    "<!-- goregraph:generated example start -->\nold\n",
			wantErr: "missing end marker for generated block example",
		},
		{
			name:    "missing required marker",
			body:    "hand-written only\n",
			wantErr: "missing generated block example",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(
				filepath.Join(root, "README.md"),
				[]byte(test.body),
				0o644,
			); err != nil {
				t.Fatal(err)
			}
			err := synchronizeDocuments(root, []documentSpec{{
				Path:   "README.md",
				Blocks: []generatedBlock{{Name: "example", Body: "fresh"}},
			}}, false)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestRepositoryDocumentationIsSynchronized(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := synchronizeDocuments(root, repositoryDocumentSpecs(), false); err != nil {
		t.Fatalf("repository documentation drift: %v", err)
	}
}
