package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestFreshnessDetectsNewEligibleFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "added.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var result Result
	checkStaleFiles(root, scan.Manifest{OutputDir: cfg.OutputDir}, &result)
	if result.Warnings == 0 {
		t.Fatal("new eligible source was not reported stale")
	}
}
