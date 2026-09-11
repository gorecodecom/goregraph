package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestSourceReadCLIFindAndAtomicFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("hit\nx\nhit"), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "goregraph-out", "agent")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"a.go": "old"}})
	if err := os.WriteFile(filepath.Join(dir, "context-index.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := scan.WithOutputRead(context.Background(), root, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	code := Run([]string{"read", root, "--request", `{"files":[{"path":"a.go","find":{"pattern":"^hit$","max_matches":1,"after":1}}]}`}, &out, &errs)
	if code != 0 || !strings.Contains(out.String(), `"match_lines":[1],"match_count":2,"next_start_line":2`) || !strings.Contains(out.String(), `"content":"1\thit\n2\tx"`) {
		t.Fatalf("find code=%d out=%s err=%s", code, &out, &errs)
	}
	out.Reset()
	errs.Reset()
	code = Run([]string{"read", root, "--request", `{"files":[{"path":"a.go","find":{"pattern":"^hit$","max_matches":1,"after":1}},{"path":"./a.go","find":{"pattern":"^x$"}}]}`}, &out, &errs)
	if code != 0 || !strings.Contains(out.String(), `"find_results":[{"request_index":0,"result":{"match_lines":[1],"match_count":2,"next_start_line":2}},{"request_index":1,"result":{"match_lines":[2],"match_count":1}}]`) || strings.Count(out.String(), `"path":`) != 1 || strings.Contains(out.String(), `"find":`) {
		t.Fatalf("batch find code=%d out=%s err=%s", code, &out, &errs)
	}
	for _, request := range []string{
		`{"files":[{"path":"a.go","find":{"pattern":"["}}]}`,
		`{"files":[{"path":"a.go","find":{"pattern":"hit","unknown":1}}]}`,
		`{"files":[{"path":"a.go","find":{"pattern":"hit","after":1.5}}]}`,
		`{"files":[{"path":"a.go","find":{"pattern":"hit"}},{"path":"a.go","ranges":[[1,1]]}]}`,
	} {
		out.Reset()
		errs.Reset()
		if code := Run([]string{"read", root, "--request", request}, &out, &errs); code == 0 || out.Len() != 0 {
			t.Fatalf("non-atomic rejection: %d %s", code, &out)
		}
		if strings.Contains(request, `"pattern":"["`) {
			for _, guidance := range []string{"files[0]", "a.go", "regexp", "literal", "escape", "JSON"} {
				if !strings.Contains(errs.String(), guidance) {
					t.Fatalf("missing %q guidance: %s", guidance, &errs)
				}
			}
		}
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("hit"+strings.Repeat("x", 24576)), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errs.Reset()
	if code := Run([]string{"read", root, "--request", `{"files":[{"path":"a.go","find":{"pattern":"hit"}}]}`}, &out, &errs); code == 0 || out.Len() != 0 {
		t.Fatalf("non-atomic output cap: %d %s", code, &out)
	}
}
