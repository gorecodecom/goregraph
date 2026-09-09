package gitignore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureOutputIgnoredAppendsBlockWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("dist/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureOutputIgnored(dir, "goregraph-out")
	if err != nil {
		t.Fatalf("EnsureOutputIgnored returned error: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "# GoreGraph local scan output\n") {
		t.Fatalf("missing GoreGraph comment in .gitignore:\n%s", text)
	}
	if !strings.Contains(text, "goregraph-out/\n") {
		t.Fatalf("missing goregraph-out/ in .gitignore:\n%s", text)
	}
}

func TestEnsureOutputIgnoredIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("# GoreGraph local scan output\ngoregraph-out/\n.goregraph-lock-*\n.goregraph-journal-*\n.goregraph-stage-*\n.goregraph-backup-*\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureOutputIgnored(dir, "goregraph-out")
	if err != nil {
		t.Fatalf("EnsureOutputIgnored returned error: %v", err)
	}
	if changed {
		t.Fatal("changed = true, want false")
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(body), "goregraph-out/"); count != 1 {
		t.Fatalf("goregraph-out/ count = %d, want 1", count)
	}
}

func TestEnsureWorkspaceIgnoredAppendsWorkspaceOutput(t *testing.T) {
	dir := t.TempDir()

	changed, err := EnsureWorkspaceIgnored(dir)
	if err != nil {
		t.Fatalf("EnsureWorkspaceIgnored returned error: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}

	body, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "# GoreGraph local workspace output\n") {
		t.Fatalf("missing GoreGraph workspace comment in .gitignore:\n%s", text)
	}
	if !strings.Contains(text, ".goregraph-workspace/\n") {
		t.Fatalf("missing .goregraph-workspace/ in .gitignore:\n%s", text)
	}
}

func TestParseMatchesRootRelativeIgnoredPaths(t *testing.T) {
	matcher := Parse("dist/\n*.log\n!important.log\n")

	if !matcher.Ignored("dist/app.js", true) {
		t.Fatal("dist/app.js was not ignored")
	}
	if !matcher.Ignored("debug.log", false) {
		t.Fatal("debug.log was not ignored")
	}
	if matcher.Ignored("important.log", false) {
		t.Fatal("important.log ignored despite negation")
	}
}

func TestEnsureOutputIgnoredMigratesTransactionBookkeeping(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("goregraph-out/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureOutputIgnored(root, "goregraph-out"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".goregraph-lock-123.lock", ".goregraph-journal-123.json", ".goregraph-stage-123/file.json", ".goregraph-backup-123/file.json"} {
		if !Parse(string(body)).Ignored(name, false) {
			t.Errorf("generated bookkeeping is not ignored: %s", name)
		}
	}
	if changed, err := EnsureOutputIgnored(root, "goregraph-out"); err != nil || changed {
		t.Fatalf("repeat changed=%v err=%v", changed, err)
	}
}
