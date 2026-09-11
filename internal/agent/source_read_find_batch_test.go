package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

type sourceReadBatchFindMetadata struct {
	RequestIndex int                    `json:"request_index"`
	Result       sourceReadFindMetadata `json:"result"`
}

func sourceReadBatchFindMeta(t *testing.T, file SourceReadFileResult) []sourceReadBatchFindMetadata {
	t.Helper()
	body, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Find        *sourceReadFindMetadata       `json:"find"`
		FindResults []sourceReadBatchFindMetadata `json:"find_results"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Find != nil || len(wire.FindResults) < 2 {
		t.Fatalf("ambiguous or absent batch metadata: %s", body)
	}
	return wire.FindResults
}

func TestSourceReadFindBatchIndependentSelectorsAliasesAndReceipts(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "alpha\nbeta\nalpha\nbeta\nalpha\nbeta\ntail")
	writeSourceFile(t, root, "b.go", "other")
	writeContextIndexAt(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"a.go": "old", "b.go": "old"}})
	if err := os.Symlink(filepath.Join(root, "a.go"), filepath.Join(root, "alias.go")); err != nil {
		t.Fatal(err)
	}
	first, err := sourceReadFindJSON(t, root, `{"files":[{"path":"a.go","find":{"pattern":"^alpha$","max_matches":1,"after":1}},{"path":"b.go","ranges":[[1,1]]},{"path":"alias.go","find":{"pattern":"^beta$","start_line":3,"max_matches":2,"before":1}},{"path":"./a.go","find":{"pattern":"missing"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Files) != 2 || first.Files[0].Path != "a.go" || first.Files[1].Path != "b.go" {
		t.Fatalf("canonical grouping: %+v", first)
	}
	wantMeta := []sourceReadBatchFindMetadata{{0, sourceReadFindMetadata{[]int{1}, 3, 2, false}}, {2, sourceReadFindMetadata{[]int{4, 6}, 2, 0, false}}, {3, sourceReadFindMetadata{[]int{}, 0, 0, false}}}
	if got := sourceReadBatchFindMeta(t, first.Files[0]); !reflect.DeepEqual(got, wantMeta) {
		t.Fatalf("independent selectors: got %+v want %+v", got, wantMeta)
	}
	if !reflect.DeepEqual(first.Files[0].Sections, []SourceReadSection{{1, 6, "1\talpha\n2\tbeta\n3\talpha\n4\tbeta\n5\talpha\n6\tbeta"}}) {
		t.Fatalf("union: %+v", first)
	}
	next, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":"./a.go","find":{"pattern":"^alpha$","start_line":2,"max_matches":1},"seen":[%q]},{"path":"alias.go","find":{"pattern":"^beta$","start_line":5,"max_matches":1,"after":1},"seen":[%q]}]}`, first.Files[0].Receipt, first.Files[0].Receipt))
	if err != nil {
		t.Fatal(err)
	}
	wantMeta = []sourceReadBatchFindMetadata{{0, sourceReadFindMetadata{[]int{3}, 2, 4, false}}, {1, sourceReadFindMetadata{[]int{6}, 1, 0, false}}}
	if got := sourceReadBatchFindMeta(t, next.Files[0]); !reflect.DeepEqual(got, wantMeta) {
		t.Fatalf("pagination with seen: %+v", got)
	}
	if !reflect.DeepEqual(next.Files[0].Sections, []SourceReadSection{{7, 7, "7\ttail"}}) || !reflect.DeepEqual(next.Files[0].SkippedRanges, []SourceReadRange{{3, 3}, {6, 6}}) {
		t.Fatalf("receipt subtraction: %+v", next)
	}
	if !strings.HasSuffix(next.Files[0].Receipt, ":1-7") {
		t.Fatalf("cumulative receipt: %s", next.Files[0].Receipt)
	}
}

func TestSourceReadFindBatchMixedSelectorsActionable(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "hit")
	for _, body := range []string{
		`{"files":[{"path":"a.go","find":{"pattern":"hit"}},{"path":"./a.go","ranges":[[1,1]]}]}`,
		`{"files":[{"path":"a.go","ranges":[[1,1]]},{"path":"./a.go","find":{"pattern":"hit"}}]}`,
	} {
		result, err := sourceReadFindJSON(t, root, body)
		if err == nil || len(result.Files) != 0 {
			t.Fatalf("mixed accepted: %+v %v", result, err)
		}
		for _, part := range []string{"a.go", "ranges", "find", "separate"} {
			if !strings.Contains(err.Error(), part) {
				t.Fatalf("missing %q guidance: %v", part, err)
			}
		}
	}
}

func TestSourceReadFindInvalidRegexGuidance(t *testing.T) {
	result, err := sourceReadFindJSON(t, t.TempDir(), `{"files":[{"path":"a.go","find":{"pattern":"hit"}},{"path":"b.go","find":{"pattern":"foo("}}]}`)
	if err == nil || len(result.Files) != 0 {
		t.Fatalf("invalid regex accepted: %+v %v", result, err)
	}
	for _, part := range []string{"files[1]", "b.go", "regexp", "literal", "escape", "JSON"} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("missing %q guidance: %v", part, err)
		}
	}
}

func TestSourceReadFindSingleJSONCompatibility(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "hit\nx\nhit")
	result, err := sourceReadFindJSON(t, root, `{"files":[{"path":"a.go","find":{"pattern":"hit","max_matches":1}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	// The fingerprint depends on the temporary absolute path; normalize only it.
	result.Files[0].Receipt = "RECEIPT"
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"files":[{"path":"a.go","sections":[{"start_line":1,"end_line":1,"content":"1\thit"}],"skipped_ranges":[],"receipt":"RECEIPT","find":{"match_lines":[1],"match_count":2,"next_start_line":2}}]}`
	if string(body) != want {
		t.Fatalf("single-find wire changed:\ngot %s\nwant %s", body, want)
	}
}

func TestSourceReadFindBatchRedaction(t *testing.T) {
	path := "src/main/resources/application.yml"
	root := sourceReadFixture(t, path, "client:\n  password: |\n    hidden-one\n    hidden-two\n  enabled: true")
	result, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":%q,"find":{"pattern":"hidden"}},{"path":%q,"find":{"pattern":"<redacted>","before":1,"after":1}}]}`, path, path))
	if err != nil {
		t.Fatal(err)
	}
	meta := sourceReadBatchFindMeta(t, result.Files[0])
	if meta[0].Result.MatchCount != 0 || meta[1].Result.MatchCount == 0 {
		t.Fatalf("redaction matching: %+v", meta)
	}
	body, _ := json.Marshal(result)
	if strings.Contains(string(body), "hidden") {
		t.Fatalf("secret delivered: %s", body)
	}
}

func TestSourceReadFindBatchCapsAndUnion(t *testing.T) {
	root := sourceReadFixture(t, "a.go", strings.Repeat("hit\n"+strings.Repeat("x\n", 201), 33))
	for _, tc := range []struct{ name, body string }{
		{"merged_ranges", `{"files":[{"path":"a.go","find":{"pattern":"hit","max_matches":32}},{"path":"a.go","find":{"pattern":"hit","start_line":6465,"max_matches":1}}]}`},
		{"merged_lines", `{"files":[{"path":"a.go","find":{"pattern":"hit","max_matches":3,"before":100,"after":100}},{"path":"a.go","find":{"pattern":"hit","start_line":607,"max_matches":3,"before":100,"after":100}}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := sourceReadFindJSON(t, root, tc.body)
			if err != nil || len(result.Files) != 1 {
				t.Fatalf("cap was not paged: %+v %v", result, err)
			}
			limited := false
			for _, find := range sourceReadBatchFindMeta(t, result.Files[0]) {
				limited = limited || find.Result.OutputLimited
			}
			if !limited {
				t.Fatalf("paged result lacks output_limited: %+v", result)
			}
		})
	}
	repeated := `{"path":"a.go","find":{"pattern":"hit","max_matches":32}}`
	result, err := sourceReadFindJSON(t, root, `{"files":[`+repeated+`,`+repeated+`]}`)
	if err != nil {
		t.Fatalf("same windows counted twice: %v", err)
	}
	if len(result.Files[0].Sections) != 32 || len(sourceReadBatchFindMeta(t, result.Files[0])) != 2 {
		t.Fatalf("union: %+v", result)
	}
	entries := make([]SourceReadFileRequest, 17)
	for i := range entries {
		entries[i] = SourceReadFileRequest{Path: "a.go", Find: &SourceReadFindRequest{Pattern: "hit"}}
	}
	result, err = ReadSource(ReadSourceRequest{Root: root, Files: entries})
	if err == nil || len(result.Files) != 0 || !strings.Contains(err.Error(), "16") {
		t.Fatalf("raw entry cap: %+v %v", result, err)
	}
	big := sourceReadFixture(t, "a.go", "hit"+strings.Repeat("x", 24576))
	result, err = sourceReadFindJSON(t, big, `{"files":[`+repeated+`,`+repeated+`]}`)
	if err == nil || len(result.Files) != 0 || !strings.Contains(err.Error(), "24 KiB") {
		t.Fatalf("output cap: %+v %v", result, err)
	}
}

func TestSourceReadFindPagesResultOverflow(t *testing.T) {
	lines := make([]string, 32)
	for i := range lines {
		lines[i] = fmt.Sprintf("hit-%02d-%s", i+1, strings.Repeat("x", 900))
	}
	root := sourceReadFixture(t, "a.go", strings.Join(lines, "\n"))

	first, err := sourceReadFindJSON(t, root, `{"files":[{"path":"a.go","find":{"pattern":"^hit-","max_matches":32}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	meta := sourceReadFindMeta(t, first.Files[0])
	if len(body)+1 > MaxSourceReadResultBytes || !meta.OutputLimited || meta.NextStartLine == 0 || len(meta.MatchLines) == 0 || len(meta.MatchLines) >= len(lines) {
		t.Fatalf("first result page is not bounded and resumable: bytes=%d meta=%+v", len(body)+1, meta)
	}

	second, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":"a.go","find":{"pattern":"^hit-","start_line":%d,"max_matches":32},"seen":[%q]}]}`, meta.NextStartLine, first.Files[0].Receipt))
	if err != nil {
		t.Fatal(err)
	}
	secondMeta := sourceReadFindMeta(t, second.Files[0])
	got := append(append([]int{}, meta.MatchLines...), secondMeta.MatchLines...)
	if len(got) != len(lines) || secondMeta.OutputLimited || secondMeta.NextStartLine != 0 {
		t.Fatalf("continued result page lost matches: first=%+v second=%+v", meta, secondMeta)
	}
	for i, line := range got {
		if line != i+1 {
			t.Fatalf("match %d = %d, want %d", i, line, i+1)
		}
	}
}

func TestSourceReadFindBatchPagesAcrossFiles(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"0.go": "old", "1.go": "old", "2.go": "old", "3.go": "old"}})
	for i := 0; i < 4; i++ {
		lines := make([]string, 4)
		for line := range lines {
			lines[line] = fmt.Sprintf("hit-%d-%d-%s", i, line, strings.Repeat("x", 1600))
		}
		writeSourceFile(t, root, fmt.Sprintf("%d.go", i), strings.Join(lines, "\n"))
	}
	initializeSourceReadLocks(t, root)

	result, err := sourceReadFindJSON(t, root, `{"files":[{"path":"0.go","find":{"pattern":"^hit","max_matches":4}},{"path":"1.go","find":{"pattern":"^hit","max_matches":4}},{"path":"2.go","find":{"pattern":"^hit","max_matches":4}},{"path":"3.go","find":{"pattern":"^hit","max_matches":4}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	limited := 0
	for _, file := range result.Files {
		meta := sourceReadFindMeta(t, file)
		if meta.OutputLimited {
			limited++
			if meta.NextStartLine == 0 {
				t.Fatalf("limited file is not resumable: %+v", meta)
			}
		}
	}
	if len(result.Files) != 4 || len(body)+1 > MaxSourceReadResultBytes || limited == 0 {
		t.Fatalf("batch page: files=%d bytes=%d limited=%d", len(result.Files), len(body)+1, limited)
	}
}

func TestSourceReadPagesWholeRangeFilesWithoutSplittingRanges(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"0.go": "old", "1.go": "old", "2.go": "old"}})
	const lineCount = 8
	for i := 0; i < 3; i++ {
		lines := make([]string, lineCount)
		for line := range lines {
			lines[line] = fmt.Sprintf("file-%d-line-%d-%s", i, line, strings.Repeat("x", 1000))
		}
		writeSourceFile(t, root, fmt.Sprintf("%d.go", i), strings.Join(lines, "\n"))
	}
	initializeSourceReadLocks(t, root)

	files := make([]SourceReadFileRequest, 3)
	for i := range files {
		files[i] = SourceReadFileRequest{Path: fmt.Sprintf("%d.go", i), Ranges: []SourceReadRange{{1, lineCount}}}
	}
	delivered := map[string]int{}
	limitedPages := 0
	for page := 0; page < 3; page++ {
		result, err := ReadSource(ReadSourceRequest{Root: root, Files: files})
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if len(body)+1 > MaxSourceReadResultBytes {
			t.Fatalf("page %d exceeds result limit: %d", page, len(body)+1)
		}
		limited := false
		for i, file := range result.Files {
			if file.OutputLimited {
				limited = true
				if len(file.Sections) != 0 || file.Receipt != "" {
					t.Fatalf("deferred range was partially delivered: %+v", file)
				}
				continue
			}
			for _, section := range file.Sections {
				delivered[file.Path] += section.EndLine - section.StartLine + 1
			}
			if file.Receipt != "" {
				files[i].Seen = []string{file.Receipt}
			}
		}
		if !limited {
			break
		}
		limitedPages++
	}
	if limitedPages == 0 || !reflect.DeepEqual(delivered, map[string]int{"0.go": lineCount, "1.go": lineCount, "2.go": lineCount}) {
		t.Fatalf("range paging incomplete: limited_pages=%d delivered=%v", limitedPages, delivered)
	}
}

func TestSourceReadFindBatchRawReceiptAndRequestCaps(t *testing.T) {
	shortReceipt := "r1:" + strings.Repeat("0", 64) + ":1-1"
	intervals := make([]string, 64)
	for i := range intervals {
		intervals[i] = fmt.Sprintf("%d-%d", 1000001+2*i, 1000001+2*i)
	}
	longReceipt := "r1:" + strings.Repeat("0", 64) + ":" + strings.Join(intervals, ",")
	for _, tc := range []struct {
		name, receipt string
		count         int
		errorPart     string
	}{
		{"raw_receipts", shortReceipt, 65, "64 receipts"},
		{"request_bytes", longReceipt, 64, "64 KiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := []SourceReadFileRequest{{Path: "a.go", Find: &SourceReadFindRequest{Pattern: "hit"}}, {Path: "./a.go", Find: &SourceReadFindRequest{Pattern: "hit"}}}
			for i := 0; i < tc.count; i++ {
				entries[i%2].Seen = append(entries[i%2].Seen, tc.receipt)
			}
			result, err := ReadSource(ReadSourceRequest{Root: t.TempDir(), Files: entries})
			if err == nil || len(result.Files) != 0 || !strings.Contains(err.Error(), tc.errorPart) {
				t.Fatalf("raw cap: %+v %v", result, err)
			}
		})
	}
}
