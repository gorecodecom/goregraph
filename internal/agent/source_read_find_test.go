package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type sourceReadFindMetadata struct {
	MatchLines    []int `json:"match_lines"`
	MatchCount    int   `json:"match_count"`
	NextStartLine int   `json:"next_start_line"`
	OutputLimited bool  `json:"output_limited"`
}

func sourceReadFindJSON(t *testing.T, root, body string) (ReadSourceResult, error) {
	t.Helper()
	var request ReadSourceRequest
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatal(err)
	}
	request.Root = root
	before, _ := json.Marshal(request)
	result, err := ReadSource(request)
	after, _ := json.Marshal(request)
	if string(before) != string(after) {
		t.Fatal("reader mutated caller request")
	}
	return result, err
}

func sourceReadFindMeta(t *testing.T, file SourceReadFileResult) sourceReadFindMetadata {
	t.Helper()
	body, _ := json.Marshal(file)
	var wire struct {
		Find *sourceReadFindMetadata `json:"find"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Find == nil {
		t.Fatalf("missing find metadata: %s", body)
	}
	return *wire.Find
}

func TestSourceReadFindAnchorsAlternationAndContext(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "alpha\nx\nbeta\nx\nalphabet\nbeta\n")
	for _, tc := range []struct {
		selector string
		lines    []int
		sections []SourceReadSection
	}{
		{`{"pattern":"^(alpha|beta)$"}`, []int{1, 3, 6}, []SourceReadSection{{1, 1, "1\talpha"}, {3, 3, "3\tbeta"}, {6, 6, "6\tbeta"}}},
		{`{"pattern":"^(alpha|beta)$","before":0,"after":0}`, []int{1, 3, 6}, []SourceReadSection{{1, 1, "1\talpha"}, {3, 3, "3\tbeta"}, {6, 6, "6\tbeta"}}},
		{`{"pattern":"^(alpha|beta)$","before":1,"after":1}`, []int{1, 3, 6}, []SourceReadSection{{1, 7, "1\talpha\n2\tx\n3\tbeta\n4\tx\n5\talphabet\n6\tbeta\n7\t"}}},
		{`{"pattern":"^$","before":0,"after":100}`, []int{7}, []SourceReadSection{{7, 7, "7\t"}}},
		{`{"pattern":"^1\\talpha$"}`, []int{}, []SourceReadSection{}},
		{`{"pattern":"alpha\\nx"}`, []int{}, []SourceReadSection{}},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			result, err := sourceReadFindJSON(t, root, `{"files":[{"path":"a.go","find":`+tc.selector+`}]}`)
			if err != nil {
				t.Fatal(err)
			}
			file := result.Files[0]
			meta := sourceReadFindMeta(t, file)
			if !reflect.DeepEqual(meta.MatchLines, tc.lines) || meta.MatchCount != len(tc.lines) || meta.NextStartLine != 0 || !reflect.DeepEqual(file.Sections, tc.sections) || len(file.EOFRanges) != 0 {
				t.Fatalf("find=%+v file=%+v", meta, file)
			}
			if len(tc.lines) == 0 && file.Receipt != "" {
				t.Fatal("no-match issued receipt")
			}
		})
	}
}

func TestSourceReadFindPaginationAndReceiptInteroperation(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "hit\nx\nhit\nx\nhit")
	first, err := sourceReadFindJSON(t, root, `{"files":[{"path":"a.go","find":{"pattern":"hit","max_matches":1,"after":1}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	meta := sourceReadFindMeta(t, first.Files[0])
	if !reflect.DeepEqual(meta, sourceReadFindMetadata{[]int{1}, 3, 2, false}) {
		t.Fatalf("first: %+v", meta)
	}
	ranged, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: "a.go", Ranges: []SourceReadRange{{2, 3}}, Seen: []string{first.Files[0].Receipt}}}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ranged.Files[0].Sections, []SourceReadSection{{3, 3, "3\thit"}}) {
		t.Fatalf("find to ranges: %+v", ranged)
	}
	next, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":"a.go","find":{"pattern":"hit","start_line":%d,"max_matches":1},"seen":[%q]}]}`, meta.NextStartLine, ranged.Files[0].Receipt))
	if err != nil {
		t.Fatal(err)
	}
	meta = sourceReadFindMeta(t, next.Files[0])
	if !reflect.DeepEqual(meta, sourceReadFindMetadata{[]int{3}, 2, 4, false}) || len(next.Files[0].Sections) != 0 || !reflect.DeepEqual(next.Files[0].SkippedRanges, []SourceReadRange{{3, 3}}) {
		t.Fatalf("ranges to find: %+v %+v", meta, next)
	}
	last, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":"a.go","find":{"pattern":"hit","start_line":%d,"max_matches":1},"seen":[%q]}]}`, meta.NextStartLine, next.Files[0].Receipt))
	if err != nil {
		t.Fatal(err)
	}
	meta = sourceReadFindMeta(t, last.Files[0])
	if !reflect.DeepEqual(meta, sourceReadFindMetadata{[]int{5}, 1, 0, false}) || !reflect.DeepEqual(last.Files[0].Sections, []SourceReadSection{{5, 5, "5\thit"}}) {
		t.Fatalf("last page: %+v %+v", meta, last)
	}
	none, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":"a.go","find":{"pattern":"missing"},"seen":[%q]}]}`, last.Files[0].Receipt))
	if err != nil || none.Files[0].Receipt != last.Files[0].Receipt {
		t.Fatalf("no-match lost receipt: %+v %v", none, err)
	}
	writeSourceFile(t, root, "a.go", "x\nhit")
	changed, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":"a.go","find":{"pattern":"hit"},"seen":[%q]}]}`, last.Files[0].Receipt))
	if err != nil {
		t.Fatal(err)
	}
	if changed.Files[0].IgnoredReceipts != 1 || !reflect.DeepEqual(changed.Files[0].Sections, []SourceReadSection{{2, 2, "2\thit"}}) {
		t.Fatalf("changed: %+v", changed)
	}
}

func TestSourceReadFindDefaultPageSizeAndStartBeyondEOF(t *testing.T) {
	root := sourceReadFixture(t, "a.go", strings.Repeat("hit\n", 11))
	first, err := sourceReadFindJSON(t, root, `{"files":[{"path":"a.go","find":{"pattern":"hit"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	meta := sourceReadFindMeta(t, first.Files[0])
	if !reflect.DeepEqual(meta, sourceReadFindMetadata{[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 11, 11, false}) {
		t.Fatalf("defaults: %+v", meta)
	}
	none, err := sourceReadFindJSON(t, root, `{"files":[{"path":"a.go","find":{"pattern":"hit","start_line":100}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if meta := sourceReadFindMeta(t, none.Files[0]); meta.MatchLines == nil || meta.MatchCount != 0 {
		t.Fatalf("no matches: %+v", meta)
	}
}

func TestSourceReadFindRedactionPrecedesMatching(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"src/main/resources/application.yml", "client:\n  password: |\n    hidden-one\n    hidden-two\n  enabled: true"},
		{"src/main/resources/application.properties", "password=hidden-one\\\n  hidden-two\n123\thidden-three"},
	} {
		root := sourceReadFixture(t, tc.path, tc.body)
		result, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":%q,"find":{"pattern":"hidden"}}]}`, tc.path))
		if err != nil {
			t.Fatal(err)
		}
		meta := sourceReadFindMeta(t, result.Files[0])
		if meta.MatchCount != 0 || len(meta.MatchLines) != 0 || len(result.Files[0].Sections) != 0 {
			t.Fatalf("hidden values discoverable: %+v %+v", meta, result)
		}
		visible, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":%q,"find":{"pattern":"<redacted>"}}]}`, tc.path))
		if err != nil {
			t.Fatal(err)
		}
		if sourceReadFindMeta(t, visible.Files[0]).MatchCount == 0 {
			t.Fatal("redacted content not searchable")
		}
	}
}

func TestSourceReadFindRejectsSelectorsAndAliasesAtomically(t *testing.T) {
	root := sourceReadFixture(t, "a.go", "hit")
	if err := os.Symlink(filepath.Join(root, "a.go"), filepath.Join(root, "alias.go")); err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{`{}`, `{"pattern":"["}`, `{"pattern":"hit","before":-1}`, `{"pattern":"hit","before":101}`, `{"pattern":"hit","after":-1}`, `{"pattern":"hit","after":101}`, `{"pattern":"hit","max_matches":-1}`, `{"pattern":"hit","max_matches":33}`, `{"pattern":"hit","start_line":-1}`, fmt.Sprintf(`{"pattern":"hit","start_line":%d}`, MaxContextSourceFileBytes+2), fmt.Sprintf(`{"pattern":%q}`, strings.Repeat("x", 1025))} {
		result, err := sourceReadFindJSON(t, root, `{"files":[{"path":"a.go","find":`+selector+`}]}`)
		if err == nil || len(result.Files) != 0 {
			t.Fatalf("accepted %s: %+v %v", selector, result, err)
		}
	}
	for _, body := range []string{
		`{"files":[{"path":"a.go","ranges":[[1,1]],"find":{"pattern":"hit"}}]}`,
		`{"files":[{"path":"a.go","ranges":[[1,1]]},{"path":"alias.go","find":{"pattern":"hit"}}]}`,
		`{"files":[{"path":"a.go","find":{"pattern":"hit"}},{"path":"alias.go","ranges":[[1,1]]}]}`,
	} {
		result, err := sourceReadFindJSON(t, root, body)
		if err == nil || len(result.Files) != 0 {
			t.Fatalf("accepted %s: %+v %v", body, result, err)
		}
	}
	for _, path := range []string{"../a.go", "/etc/passwd", "unindexed.go", "goregraph-out/agent/context-index.json"} {
		result, err := sourceReadFindJSON(t, root, fmt.Sprintf(`{"files":[{"path":%q,"find":{"pattern":"hit"}}]}`, path))
		if err == nil || len(result.Files) != 0 {
			t.Fatalf("unsafe %s: %+v %v", path, result, err)
		}
	}
}

func TestSourceReadFindCapsPageWhenPossible(t *testing.T) {
	root := sourceReadFixture(t, "a.go", strings.Repeat("hit\n"+strings.Repeat("x\n", 201), 33))
	for _, body := range []string{
		`{"files":[{"path":"a.go","find":{"pattern":"hit","max_matches":32}},{"path":"b.go","ranges":[[1,1]]}]}`,
		`{"files":[{"path":"a.go","find":{"pattern":"hit","max_matches":6,"before":100,"after":100}}]}`,
	} {
		// Make b.go indexed so failures exercise combined caps, not path rejection.
		writeSourceFile(t, root, "b.go", "x")
		indexPath := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
		data, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		var index map[string]any
		if err := json.Unmarshal(data, &index); err != nil {
			t.Fatal(err)
		}
		index["source_hashes"] = map[string]string{"a.go": "old", "b.go": "old"}
		data, _ = json.Marshal(index)
		if err := os.WriteFile(indexPath, data, 0600); err != nil {
			t.Fatal(err)
		}
		result, err := sourceReadFindJSON(t, root, body)
		if err != nil || len(result.Files) == 0 || !sourceReadFindMeta(t, result.Files[0]).OutputLimited {
			t.Fatalf("cap not paged: %+v %v", result, err)
		}
	}
	result, err := sourceReadFindJSON(t, root, `{"files":[{"path":"a.go","find":{"pattern":"hit","max_matches":1,"after":1}},{"path":"b.go","ranges":[[1,500],[501,999]]}]}`)
	if err != nil || len(result.Files) != 2 || len(result.Files[1].Sections) != 1 || result.Files[1].OutputLimited {
		t.Fatalf("EOF-clamped exact ranges should fit the delivered-line cap: %+v %v", result, err)
	}
	big := sourceReadFixture(t, "a.go", "hit"+strings.Repeat("x", 24576))
	result, err = sourceReadFindJSON(t, big, `{"files":[{"path":"a.go","find":{"pattern":"hit"}}]}`)
	if err == nil || len(result.Files) != 0 {
		t.Fatalf("output cap: %+v %v", result, err)
	}
}
