package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestRunWritesDeterministicFilesManifestAndReport(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	writeFile(t, root, "src/main.go", "package main\nfunc main() {}\n")
	writeFile(t, root, "dist/bundle.js", "ignored")

	result, err := Run(root, config.Defaults())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 2 {
		t.Fatalf("ScannedFiles = %d, want 2", result.ScannedFiles)
	}

	for _, name := range []string{"manifest.json", "files.json", "report.md"} {
		if _, err := os.Stat(filepath.Join(root, "goregraph-out", name)); err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
	}

	var files []FileRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "files.json"), &files)
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}
	if files[0].Path != "README.md" || files[1].Path != "src/main.go" {
		t.Fatalf("files sorted/filtered incorrectly: %#v", files)
	}
	for _, file := range files {
		if filepath.IsAbs(file.Path) {
			t.Fatalf("file path %q is absolute, want root-relative", file.Path)
		}
	}
}

func TestRunUsesProjectGitignoreAsExclusions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "local/\n*.tmp\n")
	writeFile(t, root, "src/app.go", "package src\n")
	writeFile(t, root, "local/cache.go", "package local\n")
	writeFile(t, root, "scratch.tmp", "tmp")

	result, err := Run(root, config.Defaults())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 1 {
		t.Fatalf("ScannedFiles = %d, want 1", result.ScannedFiles)
	}
}

func TestRunSkipsLargeBinaryAndSymlinkFiles(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.MaxFileSizeBytes = 4
	writeFile(t, root, "src/app.go", "pkg")
	writeFile(t, root, "big.txt", "12345")
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "src/app.go"), filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}

	result, err := Run(root, cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 1 {
		t.Fatalf("ScannedFiles = %d, want 1", result.ScannedFiles)
	}
	if result.SkippedFiles != 3 {
		t.Fatalf("SkippedFiles = %d, want 3", result.SkippedFiles)
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

func readJSON(t *testing.T, path string, dest any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatal(err)
	}
}
