package config

import (
	"testing"
)

func TestDefaultsMatchMVPSafetyModel(t *testing.T) {
	cfg := Defaults()

	if cfg.OutputDir != "goregraph-out" {
		t.Fatalf("OutputDir = %q, want goregraph-out", cfg.OutputDir)
	}
	if cfg.FollowSymlinks {
		t.Fatal("FollowSymlinks = true, want false")
	}
	if !cfg.UseGitignore {
		t.Fatal("UseGitignore = false, want true")
	}
	if !cfg.UpdateGitignore {
		t.Fatal("UpdateGitignore = false, want true")
	}
	if cfg.MaxFileSizeBytes != 512*1024 {
		t.Fatalf("MaxFileSizeBytes = %d, want %d", cfg.MaxFileSizeBytes, 512*1024)
	}

	for _, pattern := range []string{".git/", "node_modules/", "target/", "build/", "dist/", "coverage/", ".idea/", ".vscode/", ".gitignore", "goregraph-out/"} {
		if !cfg.HasExclude(pattern) {
			t.Fatalf("default excludes missing %q", pattern)
		}
	}
}

func TestLoadUsesDefaultsWhenProjectConfigIsMissing(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.OutputDir != "goregraph-out" {
		t.Fatalf("OutputDir = %q, want default", cfg.OutputDir)
	}
}

func TestLoadRejectsMissingProjectRoot(t *testing.T) {
	dir := t.TempDir()
	missing := dir + "/missing"

	if _, err := Load(missing); err == nil {
		t.Fatal("Load missing project root returned nil error, want error")
	}
}
