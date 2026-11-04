package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/pathutil"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveNavigationUsesAuthoredPurposesOnlyAsHints(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"alpha", "beta", "missing"} {
		writeSourceFile(t, root, directory+"/handler.go", "package handler\nfunc Process() {}\n")
	}
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "alpha", Kind: "symbol", Name: "Process", File: "alpha/handler.go", Line: 2, EndLine: 2},
			{ID: "beta", Kind: "symbol", Name: "Process", File: "beta/handler.go", Line: 2, EndLine: 2},
		},
	}
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), index)
	request := ContextRequest{Root: root, ProtocolVersion: AdaptiveV2, Query: "Where are retention, disposal and notification decided?"}
	baseline, err := BuildContext(request)
	if err != nil || len(baseline.NavigationCandidates) != 0 || len(baseline.SourceSections) != 0 {
		t.Fatalf("unexpected navigation without configured purposes: %+v, %v", baseline, err)
	}
	writeSourceFile(t, root, "goregraph.yml", "module_purposes:\n"+
		"  - \"beta: retention and disposal\"\n"+
		"  - \"alpha: retention, disposal and notification\"\n"+
		"  - \"missing: retention, disposal and notification\"\n")
	pack, err := BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.NavigationCandidates) != 2 || pack.NavigationCandidates[0].Path != "alpha" ||
		pack.NavigationCandidates[1].Path != "beta" || len(pack.NavigationCandidates[0].MatchedTerms) != 3 {
		t.Fatalf("unexpected ranked module hints: %+v", pack.NavigationCandidates)
	}
	if !pack.FallbackRequired || len(pack.Entrypoints) != 0 || len(pack.SourceSections) != 0 {
		t.Fatalf("authored hints became source evidence: %+v", pack)
	}
	request.PreviousContextID = pack.ContextID
	duplicate, err := BuildContext(request)
	if err != nil || duplicate.DuplicateOf != pack.ContextID || len(duplicate.NavigationCandidates) != 0 {
		t.Fatalf("unchanged navigation was replayed: %+v, %v", duplicate, err)
	}
	request.ProtocolVersion, request.PreviousContextID = StrictV1, ""
	strict, err := BuildContext(request)
	if err != nil || len(strict.NavigationCandidates) != 0 || strict.NextRead != nil {
		t.Fatalf("strict mode gained navigation metadata: %+v, %v", strict, err)
	}
}

func TestContextNextReadUsesOnlyUnseenInScopeVerification(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "service")
	fileName := "handler.go"
	writeSourceFile(t, project, fileName, "package handler\n\nfunc Process() {}\n")
	writeSourceFile(t, root, "neighbor/handler.go", "package neighbor\nfunc Process() {}\n")
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "handler", Project: "service", Kind: "symbol", Name: "Process", File: fileName, Line: 3, EndLine: 3},
			{ID: "neighbor", Project: "neighbor", Kind: "symbol", Name: "Process", File: fileName, Line: 2, EndLine: 2},
		},
	}
	indexPath := filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json")
	writeContextIndexAt(t, indexPath, index)
	initializeSourceReadLocks(t, project)
	canonical, err := pathutil.Resolve(project)
	if err != nil {
		t.Fatal(err)
	}
	source, err := readSourceFile(filepath.Join(canonical, fileName))
	if err != nil {
		t.Fatal(err)
	}
	section := ContextSourceSection{Project: "service", Path: fileName, StartLine: 1, EndLine: 1}
	attachSourceReadReceipt(&section, source)
	pack := adaptiveContextMetadata(ContextPack{
		ProtocolVersion: AdaptiveV2, BudgetTokens: 4000,
		SourceSections: []ContextSourceSection{section},
		SourceOmissions: []ContextSourceOmission{
			{Project: "service", Path: fileName, StartLine: 1, EndLine: 3, Reason: "additional candidate evidence exceeds the inline source limit"},
			{Project: "neighbor", Path: fileName, StartLine: 2, EndLine: 2, Reason: "additional candidate evidence exceeds the inline source limit"},
		},
	})
	loaded := loadedContextIndex{Index: index, Path: indexPath, ScopeRoot: root, Workspace: true}
	if _, err := resolveSourceReadPath(canonical, loaded, fileName); err != nil {
		t.Fatal(err)
	}
	pack, err = attachContextNextRead(pack, loaded, ContextRequest{Root: project, BudgetTokens: pack.BudgetTokens})
	if err != nil || pack.NextRead == nil || len(pack.NextRead.Files) != 1 {
		t.Fatalf("expected one in-scope next read: %+v, %v; requests: %+v", pack.NextRead, err, pack.VerificationRequests)
	}
	entry := pack.NextRead.Files[0]
	if entry.Path != fileName || len(entry.Ranges) != 1 || entry.Ranges[0] != (SourceReadRange{2, 3}) ||
		len(entry.Seen) != 1 || entry.Seen[0] != section.ReadReceipt {
		t.Fatalf("unexpected reader payload: %+v", entry)
	}
	read := *pack.NextRead
	read.Root = project
	result, err := ReadSource(read)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || len(result.Files[0].Sections) != 1 ||
		result.Files[0].Sections[0].StartLine != 2 || !strings.Contains(result.Files[0].Sections[0].Content, "func Process") {
		t.Fatalf("reader did not deliver the missing evidence: %+v", result)
	}
}

func BenchmarkContextIndexLoad(b *testing.B) {
	root := os.Getenv("GOREGRAPH_BENCH_INDEX_ROOT")
	if root == "" {
		b.Skip("set GOREGRAPH_BENCH_INDEX_ROOT to a stable existing index root")
	}
	request := ContextRequest{Root: root}
	loaded, err := loadContextIndex(request)
	if err != nil {
		b.Fatal(err)
	}
	info, err := os.Stat(loaded.Path)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(info.Size())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := loadContextIndex(request); err != nil {
			b.Fatal(err)
		}
	}
}
