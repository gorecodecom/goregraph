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

func TestSourceReadCLIBatch(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("one\ntwo\nthree"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(root, "goregraph-out", "agent")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, SourceHashes: map[string]string{"a.go": "old", "b.go": "old"}})
	if err := os.WriteFile(filepath.Join(dir, "context-index.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := scan.WithOutputRead(context.Background(), root, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	code := Run([]string{"read", root, "--request", `{"files":[{"path":"a.go","ranges":[[1,2]]},{"path":"b.go","ranges":[[2,9]]}]}`}, &out, &errs)
	if code != 0 || !json.Valid(out.Bytes()) || !strings.Contains(out.String(), `"receipt":"r1:`) || !strings.Contains(out.String(), `"end_line":3`) {
		t.Fatalf("batch read: code=%d out=%s err=%s", code, &out, &errs)
	}
}

func TestSourceReadCLIRejectsMalformedOptions(t *testing.T) {
	for _, args := range [][]string{
		{"read"}, {"read", ".", "--request"}, {"read", ".", "--unknown", "{}"},
		{"read", ".", "--request", "{}", "--request", "{}"},
		{"read", ".", "--request", `{"files":[],"unknown":1}`},
		{"read", ".", "--request", `{"files":[]} {}`},
		{"read", ".", "--request", strings.Repeat(" ", 65537)},
		{"read", ".", "--request", `{"files":[{"path":"a","ranges":[[1,2,3]]}]}`},
	} {
		var out, errs bytes.Buffer
		if code := Run(args, &out, &errs); code == 0 || out.Len() != 0 {
			t.Fatalf("accepted %q: code=%d out=%s", args, code, &out)
		}
	}
}

func TestSourceReadCLIRelativeRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("one"), 0600); err != nil {
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
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	})
	var out, errs bytes.Buffer
	if code := Run([]string{"read", ".", "--request", `{"files":[{"path":"a.go","ranges":[[1,10]]}]}`}, &out, &errs); code != 0 {
		t.Fatalf("relative root: %d %s", code, &errs)
	}
}
