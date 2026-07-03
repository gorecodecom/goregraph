package query

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestSearchFindsMatchingSymbolsAndFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	result, err := Search(root, "StartServer")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if !strings.Contains(result, "StartServer") {
		t.Fatalf("Search result missing symbol:\n%s", result)
	}
	if !strings.Contains(result, "src/main.go") {
		t.Fatalf("Search result missing file:\n%s", result)
	}
}

func TestExplainFileShowsSymbolsAndRelations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nimport \"fmt\"\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	result, err := Explain(root, "src/main.go")
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}

	if !strings.Contains(result, "src/main.go") {
		t.Fatalf("Explain result missing file:\n%s", result)
	}
	if !strings.Contains(result, "StartServer") {
		t.Fatalf("Explain result missing symbol:\n%s", result)
	}
	if !strings.Contains(result, "fmt") {
		t.Fatalf("Explain result missing import relation:\n%s", result)
	}
}

func TestSearchReportsNoMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	result, err := Search(root, "MissingThing")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if !strings.Contains(result, "No matches") {
		t.Fatalf("Search result missing no-match message:\n%s", result)
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
