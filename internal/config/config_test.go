package config

import (
	"os"
	"path/filepath"
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

	for _, pattern := range []string{".git/", "node_modules/", "target/", "build/", "dist/", "coverage/", ".idea/", ".vscode/", ".gitignore", "goregraph-out/", ".goregraph-workspace/"} {
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

func TestLoadMergesProjectConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "goregraph.yml", `version: 1
output: .goregraph
include:
  - src/**
  - tests/**
exclude:
  - generated/**
max_file_size_kb: 128
follow_symlinks: true
use_gitignore: false
update_gitignore: false
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.OutputDir != ".goregraph" {
		t.Fatalf("OutputDir = %q, want .goregraph", cfg.OutputDir)
	}
	if cfg.MaxFileSizeBytes != 128*1024 {
		t.Fatalf("MaxFileSizeBytes = %d, want %d", cfg.MaxFileSizeBytes, 128*1024)
	}
	if !cfg.FollowSymlinks {
		t.Fatal("FollowSymlinks = false, want true")
	}
	if cfg.UseGitignore {
		t.Fatal("UseGitignore = true, want false")
	}
	if cfg.UpdateGitignore {
		t.Fatal("UpdateGitignore = true, want false")
	}
	if !cfg.HasInclude("src/**") || !cfg.HasInclude("tests/**") {
		t.Fatalf("Include missing configured patterns: %#v", cfg.Include)
	}
	if !cfg.HasExclude("generated/**") {
		t.Fatalf("Exclude missing configured pattern: %#v", cfg.Exclude)
	}
	if !cfg.HasExclude("node_modules/") {
		t.Fatalf("Exclude lost default safety patterns: %#v", cfg.Exclude)
	}
}

func TestLoadRejectsUnsafeOutputDirectory(t *testing.T) {
	for _, output := range []string{"", ".", "..", "../out", "/tmp/goregraph-out"} {
		t.Run(output, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "goregraph.yml", "output: "+output+"\n")

			if _, err := Load(dir); err == nil {
				t.Fatalf("Load accepted unsafe output directory %q, want error", output)
			}
		})
	}
}

func TestLoadRejectsMissingProjectRoot(t *testing.T) {
	dir := t.TempDir()
	missing := dir + "/missing"

	if _, err := Load(missing); err == nil {
		t.Fatal("Load missing project root returned nil error, want error")
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
