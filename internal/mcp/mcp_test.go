package mcp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestServeHandlesToolsListAndQueryTool(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"query_code_map","arguments":{"root":"` + filepath.ToSlash(root) + `","term":"StartServer"}}}` + "\n",
	)
	var output bytes.Buffer

	if err := Serve(input, &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "query_code_map") {
		t.Fatalf("tools/list response missing query_code_map:\n%s", got)
	}
	if !strings.Contains(got, "StartServer") {
		t.Fatalf("tools/call response missing query result:\n%s", got)
	}
}

func TestServeReadsGeneratedOutputTool(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_output","arguments":{"root":"` + filepath.ToSlash(root) + `","name":"graph-full"}}}` + "\n",
	)
	var output bytes.Buffer

	if err := Serve(input, &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "graph-full") && !strings.Contains(got, "StartServer") {
		t.Fatalf("get_output response missing rich graph content:\n%s", got)
	}
}

func TestServeRejectsUnknownTool(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_file","arguments":{}}}` + "\n")
	var output bytes.Buffer

	if err := Serve(input, &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	if !strings.Contains(output.String(), "unknown tool") {
		t.Fatalf("unknown tool response missing error:\n%s", output.String())
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
