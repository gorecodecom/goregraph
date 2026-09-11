package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func sourceReadFixture(t *testing.T, path, content string) string {
	t.Helper()
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{path: "old"}})
	writeSourceFile(t, root, path, content)
	initializeSourceReadLocks(t, root)
	return root
}

func TestSourceReadUnionSubtractionAndChangedContent(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "one\ntwo\nthree\nfour\nfive\nsix")
	first, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{2, 3}, {3, 4}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Files) != 1 || len(first.Files[0].Sections) != 1 || first.Files[0].Sections[0].Content != "2\ttwo\n3\tthree\n4\tfour" {
		t.Fatalf("union: %#v", first)
	}
	next, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "./a.go", Ranges: []SourceReadRange{{1, 5}}, Seen: []string{first.Files[0].Receipt}}, {Path: "a.go", Ranges: []SourceReadRange{{5, 20}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := next.Files[0]
	if len(next.Files) != 1 || len(f.Sections) != 2 || f.Sections[0].StartLine != 1 || f.Sections[0].EndLine != 1 || f.Sections[1].StartLine != 5 || f.Sections[1].EndLine != 6 || !reflect.DeepEqual(f.SkippedRanges, []SourceReadRange{{2, 4}}) || !reflect.DeepEqual(f.EOFRanges, []SourceReadRange{{7, 20}}) {
		t.Fatalf("subtraction: %#v", f)
	}
	last, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 6}}, Seen: []string{f.Receipt}}}})
	if err != nil || len(last.Files[0].Sections) != 0 || !reflect.DeepEqual(last.Files[0].SkippedRanges, []SourceReadRange{{1, 6}}) {
		t.Fatalf("cumulative: %#v %v", last, err)
	}
	writeSourceFile(t, root, "a.go", "changed")
	changed, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 6}}, Seen: []string{f.Receipt}}}})
	if err != nil || len(changed.Files[0].Sections) != 1 || changed.Files[0].IgnoredReceipts != 1 {
		t.Fatalf("changed: %#v %v", changed, err)
	}
}

func TestSourceReadRejectsUnsafeUnindexedAndGeneratedPaths(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "safe")
	writeSourceFile(t, root, "unindexed.go", "secret")
	outside := t.TempDir()
	writeSourceFile(t, outside, "a.go", "outside")
	if err := os.Symlink(filepath.Join(outside, "a.go"), filepath.Join(root, "escape.go")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../a.go", "/etc/passwd", `C:\a.go`, `a\..\a.go`, "unindexed.go", "escape.go", ".git/config", "goregraph-out/agent/context-index.json"} {
		result, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: path, Ranges: []SourceReadRange{{1, 1}}}}})
		if err == nil || len(result.Files) != 0 {
			t.Fatalf("accepted %q: %#v %v", path, result, err)
		}
	}
	for _, path := range []string{".git/config", "goregraph-out/agent/other.json", ".goregraph-workspace/agent/other.json"} {
		malicious := sourceReadFixture(t, path, "secret")
		if _, err := ReadSource(ReadSourceRequest{Root: malicious, Files: []SourceReadFileRequest{{Path: path, Ranges: []SourceReadRange{{1, 1}}}}}); err == nil {
			t.Fatalf("accepted indexed generated %q", path)
		}
	}
}

func TestSourceReadRedactsWholeConfigurationBeforeSlicing(t *testing.T) {
	for _, fixture := range []struct {
		path, body string
		line       int
	}{
		{"src/main/resources/application.yml", "client:\n  password: |\n    secret-one\n    secret-two\n  enabled: true", 4},
		{"src/main/resources/application.properties", "password=secret\\\n  continued-secret\\\n  final-secret", 3},
	} {
		root := sourceReadFixture(t, fixture.path, fixture.body)
		result, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: fixture.path, Ranges: []SourceReadRange{{fixture.line, fixture.line}}}}})
		if err != nil || strings.Contains(result.Files[0].Sections[0].Content, "secret") || !strings.Contains(result.Files[0].Sections[0].Content, "<redacted>") {
			t.Fatalf("redaction: %#v %v", result, err)
		}
	}
}

func TestSourceReadRejectsMalformedAndExcessiveRequests(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "one\ntwo")
	first, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := strings.Split(first.Files[0].Receipt, ":")[1]
	for _, receipt := range []string{"", "1-2", "r2:" + fingerprint + ":1-2", "r1:" + fingerprint + ":0-1", "r1:" + fingerprint + ":1-3", "r1:" + fingerprint + ":2-1", "r1:" + fingerprint + ":" + strings.Repeat("1-1,", 64) + "1-1", strings.Repeat("x", 4097)} {
		if _, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 1}}, Seen: []string{receipt}}}}); err == nil {
			t.Fatalf("accepted receipt %q", receipt)
		}
	}
	for _, ranges := range [][]SourceReadRange{nil, {{0, 1}}, {{2, 1}}, {{1, 501}}, {{1, 500}, {1, 500}, {1, 1}}} {
		if _, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: ranges}}}); err == nil {
			t.Fatalf("accepted ranges %#v", ranges)
		}
	}
	big := sourceReadFixture(t, "a.go", strings.Repeat("X", 24576))
	if result, err := ReadSource(ReadSourceRequest{Root: big, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 1}}}}}); err == nil || len(result.Files) != 0 {
		t.Fatalf("output overflow claimed delivery: %#v %v", result, err)
	}
	invalid := sourceReadFixture(t, "a.go", string([]byte{0xff}))
	if _, err := ReadSource(ReadSourceRequest{Root: invalid, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 1}}}}}); err == nil {
		t.Fatal("accepted non-UTF8")
	}
}

func TestSourceReadAdaptiveContextReceiptReusesOnlyDeliveredLines(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "Payment.go", "package app\nfunc capturePayment() {\n ledger.recordPayment()\n}\n")
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, Facts: []scan.AgentContextFactRecord{{ID: "payment", Kind: "symbol", Name: "capturePayment", File: "Payment.go", Line: 2, EndLine: 4, Confidence: "EXACT"}}})
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Trace payment capture behavior.", ProtocolVersion: AdaptiveV2})
	if err != nil || len(pack.SourceSections) == 0 {
		t.Fatalf("context fixture: %#v %v", pack, err)
	}
	section := pack.SourceSections[0]
	if section.ReadReceipt == "" {
		t.Fatal("adaptive source lacks delivery receipt")
	}
	result, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: section.Path, Ranges: []SourceReadRange{{section.StartLine, section.EndLine}}, Seen: []string{section.ReadReceipt}}}})
	if err != nil || len(result.Files[0].Sections) != 0 {
		t.Fatalf("context reread: %#v %v", result, err)
	}
	body, _ := json.Marshal(pack)
	if len(body) > MaxContextBytes || pack.EstimatedTokens > DefaultContextBudgetTokens {
		t.Fatalf("receipt escaped context budget")
	}
	strict, err := BuildContext(ContextRequest{Root: root, Query: "Trace payment capture behavior."})
	if err != nil {
		t.Fatal(err)
	}
	strictBody, _ := json.Marshal(strict)
	if strings.Contains(string(strictBody), "read_receipt") {
		t.Fatal("strict output changed")
	}
}

func TestSourceReadWorkspaceAliasesAndRootConfinement(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "services/a/a.go", "one\ntwo")
	writeSourceFile(t, root, "services/b/b.go", "other")
	if err := os.Symlink(filepath.Join(root, "services/a"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"services/a/a.go": "old", "services/b/b.go": "old"}})
	initializeSourceReadLocks(t, root)
	result, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "alias/a.go", Ranges: []SourceReadRange{{1, 1}}}, {Path: "services/a/a.go", Ranges: []SourceReadRange{{1, 2}}}}})
	if err != nil || len(result.Files) != 1 || result.Files[0].Path != "services/a/a.go" || len(result.Files[0].Sections) != 1 {
		t.Fatalf("project aliases: %#v %v", result, err)
	}
	subroot := filepath.Join(root, "services/a")
	initializeSourceReadLocks(t, subroot)
	local, err := ReadSource(ReadSourceRequest{Root: subroot, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 2}}, Seen: []string{result.Files[0].Receipt}}}})
	if err != nil || len(local.Files[0].Sections) != 0 || local.Files[0].Path != "a.go" {
		t.Fatalf("root-relative reuse: %#v %v", local, err)
	}
	if err := os.Symlink(filepath.Join(root, "services/b/b.go"), filepath.Join(subroot, "escape.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSource(ReadSourceRequest{Root: subroot, Files: []SourceReadFileRequest{{Path: "escape.go", Ranges: []SourceReadRange{{1, 1}}}}}); err == nil {
		t.Fatal("read escaped requested project into workspace peer")
	}
}

func TestSourceReadRejectsWorkspaceCustomOutputEvenIfIndexed(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "services/a/goregraph.yml", "output: custom-output\n")
	writeSourceFile(t, root, "services/a/custom-output/agent/leak.json", "secret")
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"services/a/custom-output/agent/leak.json": "old"}})
	initializeSourceReadLocks(t, root)
	if _, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "services/a/custom-output/agent/leak.json", Ranges: []SourceReadRange{{1, 1}}}}}); err == nil {
		t.Fatal("custom project output accepted as source")
	}
}

func TestSourceReadRequestAndReceiptCountBounds(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "one")
	entry := SourceReadFileRequest{Path: "a.go", Ranges: []SourceReadRange{{1, 1}}}
	first, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{entry}})
	if err != nil {
		t.Fatal(err)
	}
	files := make([]SourceReadFileRequest, 17)
	for i := range files {
		files[i] = entry
	}
	manyRanges := entry
	manyRanges.Ranges = make([]SourceReadRange, 33)
	for i := range manyRanges.Ranges {
		manyRanges.Ranges[i] = SourceReadRange{1, 1}
	}
	manyReceipts := entry
	manyReceipts.Seen = make([]string, 65)
	for i := range manyReceipts.Seen {
		manyReceipts.Seen[i] = first.Files[0].Receipt
	}
	for _, request := range []ReadSourceRequest{{Root: root, Files: files}, {Root: root, Files: []SourceReadFileRequest{manyRanges}}, {Root: root, Files: []SourceReadFileRequest{manyReceipts}}} {
		if _, err := ReadSource(request); err == nil {
			t.Fatal("accepted excessive request")
		}
	}
	tooLarge := sourceReadFixture(t, "a.go", strings.Repeat("x", MaxContextSourceFileBytes+1))
	if _, err := ReadSource(ReadSourceRequest{Root: tooLarge, Files: []SourceReadFileRequest{entry}}); err == nil {
		t.Fatal("accepted oversized file")
	}
}

func TestSourceReadAdaptiveEntrypointReceiptAndReadOnlyState(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "PaymentController.java", "class PaymentController {\n @PostMapping(\"/payments\")\n void capturePayment() {\n  ledger.recordPayment();\n }\n}\n")
	indexPath := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
	writeContextIndexAt(t, indexPath, scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, Facts: []scan.AgentContextFactRecord{
		{ID: "payment-endpoint", Kind: "api_endpoint", Name: "POST /payments", Qualified: "PaymentController.capturePayment", HTTPMethod: "POST", Path: "/payments", File: "PaymentController.java", Line: 2, EndLine: 5, Confidence: "EXACT"},
		{ID: "payment-handler", Kind: "symbol", Name: "capturePayment", Qualified: "PaymentController.capturePayment", File: "PaymentController.java", Line: 3, EndLine: 5, Confidence: "EXACT"},
	}, Edges: []scan.AgentContextEdgeRecord{{ID: "handler", FromFactID: "payment-endpoint", ToFactID: "payment-handler", Kind: "call", Confidence: "EXACT"}}})
	pack, err := BuildContext(ContextRequest{Root: root, Query: "POST /payments", ProtocolVersion: AdaptiveV2})
	if err != nil || pack.FallbackRequired || len(pack.SourceSections) == 0 {
		t.Fatalf("normal source fixture: %#v %v", pack, err)
	}
	// Existing context reads initialize legacy output lock files as needed.
	before := sourceReadDiskSnapshot(t, root)
	for _, section := range pack.SourceSections {
		if section.ReadReceipt == "" {
			t.Fatal("normal adaptive section missing receipt")
		}
		result, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: section.Path, Ranges: []SourceReadRange{{section.StartLine, section.EndLine}}, Seen: []string{section.ReadReceipt}}}})
		if err != nil || len(result.Files[0].Sections) != 0 || len(result.Files[0].SkippedRanges) == 0 {
			t.Fatalf("seeded read: %#v %v", result, err)
		}
	}
	after := sourceReadDiskSnapshot(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("source read wrote workspace state: before=%v after=%v", before, after)
	}
}

func sourceReadDiskSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func initializeSourceReadLocks(t *testing.T, root string) {
	t.Helper()
	if err := scan.WithOutputRead(context.Background(), root, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestSourceReadMissingLockLeavesAllFilesUnchanged(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"a.go": "old"}})
	writeSourceFile(t, root, "a.go", "one")
	before := sourceReadDiskSnapshot(t, root)
	result, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 1}}}}})
	if err == nil || !strings.Contains(err.Error(), "lock is missing") || len(result.Files) != 0 {
		t.Fatalf("missing lock did not fail closed: %#v %v", result, err)
	}
	if after := sourceReadDiskSnapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("missing-lock read wrote state: before=%v after=%v", before, after)
	}
}

func TestSourceReadDisjointRangesAndOtherFileReceipt(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12")
	first, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{8, 12}, {2, 3}, {5, 5}}}}})
	if err != nil {
		t.Fatal(err)
	}
	next, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 10}}, Seen: []string{first.Files[0].Receipt}}}})
	if err != nil {
		t.Fatal(err)
	}
	var missing []SourceReadRange
	for _, section := range next.Files[0].Sections {
		missing = append(missing, SourceReadRange{section.StartLine, section.EndLine})
	}
	if !reflect.DeepEqual(missing, []SourceReadRange{{1, 1}, {4, 4}, {6, 7}}) || !reflect.DeepEqual(next.Files[0].SkippedRanges, []SourceReadRange{{2, 3}, {5, 5}, {8, 10}}) {
		t.Fatalf("disjoint subtraction: %#v", next)
	}
	other := sourceReadFixture(t, "a.go", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12")
	copied, err := ReadSource(ReadSourceRequest{Root: other, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{2, 3}}, Seen: []string{first.Files[0].Receipt}}}})
	if err != nil || copied.Files[0].IgnoredReceipts != 1 || len(copied.Files[0].Sections) != 1 || len(copied.Files[0].SkippedRanges) != 0 {
		t.Fatalf("same-content different-path receipt suppressed source: %#v %v", copied, err)
	}
}

func TestSourceReadRedactsNumericPropertiesKeysWithTabs(t *testing.T) {
	root := sourceReadFixture(t, "src/main/resources/application.properties", "123\tsecret-value\n456\tsecond-secret\nplain=third-secret")
	result, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "src/main/resources/application.properties", Ranges: []SourceReadRange{{1, 3}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || len(result.Files[0].Sections) != 1 {
		t.Fatalf("missing property section: %#v", result)
	}
	content := result.Files[0].Sections[0].Content
	if strings.Contains(content, "secret") || content != "1\t123\t<redacted>\n2\t456\t<redacted>\n3\tplain=<redacted>" {
		t.Fatalf("numeric property values escaped redaction or line numbers changed: %q", content)
	}
}
