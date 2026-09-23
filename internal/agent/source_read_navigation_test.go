package agent

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestSourceReadNextRequestPreservesIndependentSelectors(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "alpha\nbeta\nalpha\nbeta\nalpha\nbeta\ntail")
	request := ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{
		{Path: "a.go", Find: &SourceReadFindRequest{Pattern: "alpha", MaxMatches: 1, After: 1}},
		{Path: "./a.go", Find: &SourceReadFindRequest{Pattern: "beta", MaxMatches: 2}},
		{Path: "a.go", Find: &SourceReadFindRequest{Pattern: "missing"}},
	}}
	seen := map[int]bool{}
	for pages := 0; ; pages++ {
		if pages > 4 {
			t.Fatal("continuation did not finish")
		}
		result, err := ReadSource(request)
		if err != nil {
			t.Fatal(err)
		}
		for _, section := range result.Files[0].Sections {
			for line := section.StartLine; line <= section.EndLine; line++ {
				if seen[line] {
					t.Fatalf("line %d delivered twice", line)
				}
				seen[line] = true
			}
		}
		if result.NextRequest == nil {
			break
		}
		if err := validateSourceReadRequest(*result.NextRequest); err != nil {
			t.Fatalf("invalid continuation: %v", err)
		}
		body, err := json.Marshal(result.NextRequest)
		if err != nil {
			t.Fatal(err)
		}
		request = ReadSourceRequest{Root: root}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatal(err)
		}
		for i, file := range request.Files {
			wantReceipts := 0
			if i == 0 {
				wantReceipts = 1
			}
			if file.Path != "a.go" || len(file.Seen) != wantReceipts || file.Find.Pattern == "missing" {
				t.Fatalf("unexpected continuation: %+v", file)
			}
		}
	}
	if len(seen) != 6 {
		t.Fatalf("missing requested lines: %+v", seen)
	}
}

func TestSourceReadCitationsNeverBridgeUnseenGaps(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "one\ntwo\nthree\nfour\nfive")
	first, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{3, 3}}}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 9}}, Seen: []string{first.Files[0].Receipt}}}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.go:1-2", "a.go:4-5"}
	if !reflect.DeepEqual(result.Files[0].Citations, want) || result.NextRequest != nil {
		t.Fatalf("citations include skipped or nonexistent lines: %+v", result)
	}
}

func TestSourceReadNextRequestCompletesBoundedRangeAndFindBatches(t *testing.T) {
	for _, find := range []bool{false, true} {
		t.Run(fmt.Sprintf("find=%t", find), func(t *testing.T) {
			root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"0.go": "old", "1.go": "old", "2.go": "old"}})
			request := ReadSourceRequest{Root: root}
			for i := 0; i < 3; i++ {
				path := fmt.Sprintf("%d.go", i)
				writeSourceFile(t, root, path, strings.Repeat("hit "+strings.Repeat("x", 1100)+"\n", 8))
				file := SourceReadFileRequest{Path: path, Ranges: []SourceReadRange{{1, 8}}}
				if find {
					file.Ranges = nil
					file.Find = &SourceReadFindRequest{Pattern: "^hit", MaxMatches: 8}
				}
				request.Files = append(request.Files, file)
			}
			initializeSourceReadLocks(t, root)
			delivered := map[string]bool{}
			for page := 0; ; page++ {
				if page > 4 {
					t.Fatal("continuation did not finish")
				}
				result, err := ReadSource(request)
				if err != nil {
					t.Fatal(err)
				}
				body, err := json.Marshal(result)
				if err != nil || len(body)+1 > MaxSourceReadResultBytes {
					t.Fatalf("output exceeds limit: bytes=%d err=%v", len(body), err)
				}
				for _, file := range result.Files {
					for _, section := range file.Sections {
						for line := section.StartLine; line <= section.EndLine; line++ {
							key := fmt.Sprintf("%s:%d", file.Path, line)
							if delivered[key] {
								t.Fatalf("duplicate %s", key)
							}
							delivered[key] = true
						}
					}
				}
				if result.NextRequest == nil {
					break
				}
				request = *result.NextRequest
				request.Root = root
			}
			if len(delivered) != 24 {
				t.Fatalf("missing requested source: %d lines", len(delivered))
			}
		})
	}
}

func TestSourceReadDeferredContinuationDoesNotMultiplyReceiptLimit(t *testing.T) {
	receipts := make([]string, 64)
	for i := range receipts {
		receipts[i] = makeSourceReadReceipt(strings.Repeat("a", 64), []SourceReadRange{{1, 1}})
	}
	batch := []*sourceReadBatchFile{{path: "a.go", deferred: true, seen: receipts, finds: []sourceReadBatchFind{
		{request: SourceReadFindRequest{Pattern: "first", MaxMatches: 1}, limit: 1},
		{request: SourceReadFindRequest{Pattern: "second", MaxMatches: 1}, limit: 1},
	}}}
	next := sourceReadNextRequest(batch, ReadSourceResult{Files: []SourceReadFileResult{{Path: "a.go", OutputLimited: true}}})
	if next == nil || len(next.Files) != 2 {
		t.Fatalf("missing deferred selectors: %+v", next)
	}
	if err := validateSourceReadRequest(*next); err != nil {
		t.Fatalf("continuation multiplied valid receipt count: %v", err)
	}
	if len(next.Files[0].Seen) != 64 || len(next.Files[1].Seen) != 0 {
		t.Fatalf("receipts must be carried once for the canonical file: %+v", next)
	}
}

func TestSourceReadOptionalNavigationCannotBlockMinimumPage(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "hit"+strings.Repeat("x", 9000)+"\nhit")
	request := ReadSourceRequest{Root: root}
	for i := 0; i < 16; i++ {
		request.Files = append(request.Files, SourceReadFileRequest{Path: "a.go", Find: &SourceReadFindRequest{
			Pattern: "hit|" + strings.Repeat("z", 980) + fmt.Sprint(i), MaxMatches: 1,
		}})
	}
	result, err := ReadSource(request)
	if err != nil {
		t.Fatalf("optional navigation blocked source: %v", err)
	}
	if result.NextRequest != nil || len(result.Files) != 1 || len(result.Files[0].Sections) != 1 || len(result.Files[0].FindResults) != 16 {
		t.Fatalf("minimum page lost source or legacy continuation metadata: %+v", result)
	}
	for _, selector := range result.Files[0].FindResults {
		if selector.Result.NextStartLine != 2 {
			t.Fatalf("manual continuation unavailable: %+v", selector)
		}
	}
}

func TestSourceReadCitationsCannotBlockPreviouslyFittingExactRange(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "x")
	request := ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{1, 1}}}}}
	small, err := ReadSource(request)
	if err != nil {
		t.Fatal(err)
	}
	small.Files[0].Citations = nil
	body, err := json.Marshal(small)
	if err != nil {
		t.Fatal(err)
	}
	// Replace the single x so the legacy JSON plus newline fits exactly.
	content := strings.Repeat("x", MaxSourceReadResultBytes-len(body))
	writeSourceFile(t, root, "a.go", content)
	result, err := ReadSource(request)
	if err != nil {
		t.Fatalf("optional citations blocked a valid exact range: %v", err)
	}
	body, err = json.Marshal(result)
	if err != nil || len(body)+1 != MaxSourceReadResultBytes || len(result.Files[0].Citations) != 0 || result.Files[0].Sections[0].Content != "1\t"+content {
		t.Fatalf("exact-limit source changed: bytes=%d err=%v", len(body)+1, err)
	}
}
