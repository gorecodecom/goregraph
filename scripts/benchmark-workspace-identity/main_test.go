package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashWorkspaceIgnoresGeneratedAndVCSDirectories(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "service/src/main.go", "package main\n")
	writeTestFile(t, root, ".git/index", "first index")
	writeTestFile(t, root, ".goregraph-workspace/manifest.json", `{"scan":1}`)
	writeTestFile(t, root, "service/goregraph-out/index/summary.json", `{"scan":1}`)

	want, err := hashWorkspace(root)
	if err != nil {
		t.Fatalf("hash workspace: %v", err)
	}

	writeTestFile(t, root, ".git/index", "second index")
	writeTestFile(t, root, ".goregraph-workspace/manifest.json", `{"scan":2}`)
	writeTestFile(t, root, "service/goregraph-out/index/summary.json", `{"scan":2}`)

	got, err := hashWorkspace(root)
	if err != nil {
		t.Fatalf("hash workspace after generated changes: %v", err)
	}
	if got != want {
		t.Fatalf("generated or VCS changes altered workspace identity: got %s, want %s", got, want)
	}
}

func TestHashWorkspaceIncludesSourceChanges(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "service/src/main.go", "package main\n")

	before, err := hashWorkspace(root)
	if err != nil {
		t.Fatalf("hash workspace: %v", err)
	}

	writeTestFile(t, root, "service/src/main.go", "package service\n")
	after, err := hashWorkspace(root)
	if err != nil {
		t.Fatalf("hash workspace after source change: %v", err)
	}
	if after == before {
		t.Fatalf("source change did not alter workspace identity: %s", after)
	}
}

func writeTestFile(t *testing.T, root, relativePath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent directory for %s: %v", relativePath, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", relativePath, err)
	}
}
