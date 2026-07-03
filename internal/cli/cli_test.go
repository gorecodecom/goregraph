package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelpPrintsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: goregraph <command>") {
		t.Fatalf("help output missing usage:\n%s", stdout.String())
	}
}

func TestRunScanWritesOutputAndUpdatesGitignore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "goregraph-out", "manifest.json")); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore not written: %v", err)
	}
	if !strings.Contains(string(gitignore), "goregraph-out/") {
		t.Fatalf(".gitignore missing goregraph-out/:\n%s", string(gitignore))
	}
	if !strings.Contains(stdout.String(), "Scanned 1 files") {
		t.Fatalf("stdout missing scan summary:\n%s", stdout.String())
	}
}

func TestRunScanNoUpdateGitignoreLeavesGitignoreUntouched(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", root, "--no-update-gitignore"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf(".gitignore exists after opt-out, err=%v", err)
	}
}

func TestRunUpdateRefreshesCurrentProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	}()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"update", "--no-update-gitignore"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "goregraph-out", "manifest.json")); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
}

func TestRunReportPrintsGeneratedReport(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"report", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# GoreGraph Report") {
		t.Fatalf("report output missing heading:\n%s", stdout.String())
	}
}

func TestRunQuerySearchesGeneratedIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"query", root, "StartServer"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "StartServer") {
		t.Fatalf("query output missing symbol:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "src/main.go") {
		t.Fatalf("query output missing file:\n%s", stdout.String())
	}
}

func TestRunExplainPrintsFileContext(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"explain", root, "src/main.go"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "src/main.go") {
		t.Fatalf("explain output missing file:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "StartServer") {
		t.Fatalf("explain output missing symbol:\n%s", stdout.String())
	}
}

func TestRunScanHelpPrintsScanUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", "help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: goregraph scan") {
		t.Fatalf("scan help missing usage:\n%s", stdout.String())
	}
}

func TestRunUnknownCommandReturnsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"nope"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr missing unknown command:\n%s", stderr.String())
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
