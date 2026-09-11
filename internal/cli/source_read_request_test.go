package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestSourceReadCLIShorthandMatchesCanonicalBatch(t *testing.T) {
	root := sourceReadRequestFixture(t)
	canonical := sourceReadRequestOK(t, root, `{"files":[{"path":"a.go","ranges":[[1,2]]},{"path":"b.go","find":{"pattern":"^hit$","start_line":2,"before":1}}]}`)
	for _, shorthand := range []string{
		`{"files":[{"path":"a.go","start_line":1,"end_line":2},{"path":"b.go","find":{"pattern":"^hit$","start_line":2,"before":1}}]}`,
		`{"files":[{"path":"a.go","ranges":[[1,2]]},{"path":"b.go","start_line":2,"find":{"pattern":"^hit$","before":1}}]}`,
		`{"files":[{"path":"a.go","ranges":[],"start_line":1,"end_line":2},{"path":"b.go","ranges":null,"start_line":2,"find":{"pattern":"^hit$","before":1,"start_line":0}}]}`,
	} {
		if got := sourceReadRequestOK(t, root, shorthand); got != canonical {
			t.Fatalf("shorthand changed response or receipts:\ngot %s\nwant %s", got, canonical)
		}
	}
	if !strings.Contains(canonical, `"content":"1\thit\n2\tx"`) || !strings.Contains(canonical, `"match_lines":[3,5]`) {
		t.Fatalf("missing expected source and find results: %s", canonical)
	}
}

func TestSourceReadCLIShorthandPaginationPreservesReceipts(t *testing.T) {
	root := sourceReadRequestFixture(t)
	first := sourceReadRequestOK(t, root, `{"files":[{"path":"a.go","find":{"pattern":"^hit$","max_matches":1,"after":2}}]}`)
	var result agent.ReadSourceResult
	if err := json.Unmarshal([]byte(first), &result); err != nil {
		t.Fatal(err)
	}
	page := result.Files[0]
	if page.Find.NextStartLine != 2 || page.Receipt == "" {
		t.Fatalf("missing page cursor or receipt: %s", first)
	}
	canonical := sourceReadRequestOK(t, root, fmt.Sprintf(`{"files":[{"path":"a.go","find":{"pattern":"^hit$","start_line":2,"max_matches":1,"after":2},"seen":[%q]}]}`, page.Receipt))
	shorthand := sourceReadRequestOK(t, root, fmt.Sprintf(`{"files":[{"path":"a.go","start_line":2,"find":{"pattern":"^hit$","max_matches":1,"after":2},"seen":[%q]}]}`, page.Receipt))
	if shorthand != canonical {
		t.Fatalf("pagination changed response or receipts:\ngot %s\nwant %s", shorthand, canonical)
	}
	if !strings.Contains(shorthand, `"skipped_ranges":[[3,3]]`) || !strings.Contains(shorthand, `"content":"4\tx\n5\thit"`) || !strings.Contains(shorthand, `"next_start_line":4`) {
		t.Fatalf("page did not subtract prior source or advance: %s", shorthand)
	}
	canonical = sourceReadRequestOK(t, root, fmt.Sprintf(`{"files":[{"path":"a.go","ranges":[[1,5]],"seen":[%q]}]}`, page.Receipt))
	shorthand = sourceReadRequestOK(t, root, fmt.Sprintf(`{"files":[{"path":"a.go","start_line":1,"end_line":5,"seen":[%q]}]}`, page.Receipt))
	if shorthand != canonical {
		t.Fatalf("range shorthand changed receipt subtraction:\ngot %s\nwant %s", shorthand, canonical)
	}
}

func TestSourceReadCLIShorthandRejectsMalformedSelectorsAtomically(t *testing.T) {
	root := sourceReadRequestFixture(t)
	for _, selector := range []string{
		`"start_line":1`, `"end_line":2`,
		`"start_line":null,"end_line":2`, `"start_line":1,"end_line":null`,
		`"start_line":1.0,"end_line":2`, `"start_line":1,"end_line":2.5`,
		`"start_line":"1","end_line":2`, `"start_line":1,"end_line":"2"`,
		`"start_line":true,"end_line":2`, `"start_line":1,"end_line":false`,
		`"start_line":0,"end_line":2`, `"start_line":1,"end_line":-2`,
		`"start_line":1e0,"end_line":2`, `"start_line":999999999999999999999999,"end_line":2`,
		`"start_line":1,"end_line":2,"ranges":[[1,2]]`,
		`"start_line":1,"find":{"pattern":"hit"},"ranges":[[1,2]]`,
		`"start_line":1,"end_line":2,"find":{"pattern":"hit"}`,
		`"end_line":2,"find":{"pattern":"hit"}`,
		`"start_line":null,"find":{"pattern":"hit"}`,
		`"start_line":1,"find":{"pattern":"hit","start_line":1}`,
		`"start_line":1,"find":{"pattern":"hit","start_line":2}`,
		`"start_line":1,"find":{"pattern":"hit","start_line":-1}`,
		`"start_line":1,"find":null`,
	} {
		t.Run(selector, func(t *testing.T) {
			request := `{"files":[{"path":"b.go","ranges":[[1,1]]},{"path":"a.go",` + selector + `}]}`
			var out, errs bytes.Buffer
			if code := Run([]string{"read", root, "--request", request}, &out, &errs); code != 2 || out.Len() != 0 {
				t.Fatalf("malformed batch: code=%d out=%s err=%s", code, &out, &errs)
			}
			if !strings.Contains(errs.String(), "ranges") || !strings.Contains(errs.String(), "find") {
				t.Fatalf("missing selector guidance: %s", &errs)
			}
		})
	}
}

func TestSourceReadCLIWireStrictnessAndRawLimit(t *testing.T) {
	valid := `{"files":[{"path":"a.go","start_line":1,"end_line":2}]}`
	for _, request := range []string{
		`{"files":[],"unknown":1}`,
		`{"files":[{"path":"a.go","start_line":1,"end_line":2,"unknown":1}]}`,
		`{"files":[{"path":"a.go","start_line":1,"find":{"pattern":"hit","unknown":1}}]}`,
		valid + ` {}`, valid + ` null`, valid + ` garbage`,
		strings.Repeat(" ", 65537-len(valid)) + valid,
	} {
		var out, errs bytes.Buffer
		if code := Run([]string{"read", ".", "--request", request}, &out, &errs); code != 2 || out.Len() != 0 {
			t.Fatalf("invalid wire request: code=%d out=%s err=%s", code, &out, &errs)
		}
		if len(request) > 65536 && !strings.Contains(errs.String(), "exceeds 64 KiB") {
			t.Fatalf("raw size was not checked first: %s", &errs)
		}
	}
}

func TestSourceReadCLIAcceptsOneRedundantClosingPair(t *testing.T) {
	root := sourceReadRequestFixture(t)
	valid := `{"files":[{"path":"a.go","ranges":[[1,2]]}]}`
	if got, want := sourceReadRequestOK(t, root, valid+`]}`), sourceReadRequestOK(t, root, valid); got != want {
		t.Fatalf("redundant closing pair changed request:\ngot %s\nwant %s", got, want)
	}
	for _, suffix := range []string{`]`, `}`, `]}]}`, `{}", "ignored": true`} {
		var out, errs bytes.Buffer
		if code := Run([]string{"read", root, "--request", valid + suffix}, &out, &errs); code != 2 || out.Len() != 0 {
			t.Fatalf("accepted unsupported suffix %q: code=%d out=%s err=%s", suffix, code, &out, &errs)
		}
	}
}

func TestSourceReadCLIRepairsInvalidJSONRegexEscapes(t *testing.T) {
	root := sourceReadRequestFixture(t)
	invalidJSON := `{"files":[{"path":"a.go","find":{"pattern":"hit\("}}]}`
	validJSON := `{"files":[{"path":"a.go","find":{"pattern":"hit\\("}}]}`
	if got, want := sourceReadRequestOK(t, root, invalidJSON), sourceReadRequestOK(t, root, validJSON); got != want {
		t.Fatalf("regex escape repair changed request:\ngot %s\nwant %s", got, want)
	}
	for _, request := range []string{
		`{"files":[{"path":"a.go","find":{"pattern":"hit\u"}}]}`,
		`{"files":[{"path":"a.go","find":{"pattern":"hit\"}}]}`,
		`{"files":[{"path":"a.go","find":{"pattern":"hit\("},"unknown":1}]}`,
	} {
		var out, errs bytes.Buffer
		if code := Run([]string{"read", root, "--request", request}, &out, &errs); code != 2 || out.Len() != 0 {
			t.Fatalf("accepted unsupported malformed request %q: code=%d out=%s err=%s", request, code, &out, &errs)
		}
	}
}

func TestSourceReadCLIShorthandRetainsAgentValidation(t *testing.T) {
	root := sourceReadRequestFixture(t)
	for _, file := range []string{
		`{"path":"../a.go","start_line":1,"end_line":2}`,
		`{"path":"/a.go","start_line":1,"find":{"pattern":"hit"}}`,
		`{"path":"missing.go","start_line":1,"end_line":2}`,
		`{"path":"a.go","start_line":2,"end_line":1}`,
		`{"path":"a.go","start_line":1,"end_line":501}`,
		`{"path":"a.go","start_line":2097154,"find":{"pattern":"hit"}}`,
	} {
		var out, errs bytes.Buffer
		request := `{"files":[{"path":"b.go","ranges":[[1,1]]},` + file + `]}`
		if code := Run([]string{"read", root, "--request", request}, &out, &errs); code != 1 || out.Len() != 0 {
			t.Fatalf("agent validation bypassed: code=%d out=%s err=%s", code, &out, &errs)
		}
	}
}

func TestSourceReadCLICanonicalEmptyRangesWithFind(t *testing.T) {
	root := sourceReadRequestFixture(t)
	canonical := sourceReadRequestOK(t, root, `{"files":[{"path":"a.go","find":{"pattern":"hit","start_line":2}}]}`)
	for _, ranges := range []string{"[]", "null"} {
		request := `{"files":[{"path":"a.go","ranges":` + ranges + `,"find":{"pattern":"hit","start_line":2}}]}`
		if got := sourceReadRequestOK(t, root, request); got != canonical {
			t.Fatalf("canonical empty ranges compatibility: got %s want %s", got, canonical)
		}
	}
}

func sourceReadRequestOK(t *testing.T, root, request string) string {
	t.Helper()
	var out, errs bytes.Buffer
	if code := Run([]string{"read", root, "--request", request}, &out, &errs); code != 0 {
		t.Fatalf("read failed: code=%d out=%s err=%s", code, &out, &errs)
	}
	return out.String()
}

func sourceReadRequestFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("hit\nx\nhit\nx\nhit"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(root, "goregraph-out", "agent")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"a.go": "old", "b.go": "old"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "context-index.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := scan.WithOutputRead(context.Background(), root, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	return root
}
