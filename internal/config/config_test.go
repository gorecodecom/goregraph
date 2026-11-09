package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	for _, pattern := range []string{".git/", ".worktrees/", "node_modules/", "target/", "build/", "dist/", "coverage/", ".idea/", ".vscode/", ".gitignore", "goregraph-out/", ".goregraph-workspace/"} {
		if !cfg.HasExclude(pattern) {
			t.Fatalf("default excludes missing %q", pattern)
		}
	}
}

func TestLoadEditorURLTemplate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "goregraph.yml"), []byte("version: 1\neditor_url_template: vscode://file/{file}:{line}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EditorURLTemplate != "vscode://file/{file}:{line}" {
		t.Fatalf("EditorURLTemplate = %q", cfg.EditorURLTemplate)
	}
}

func TestModulePurposesIgnoreInvalidEntriesWithWarnings(t *testing.T) {
	for _, test := range []struct {
		value string
		valid int
	}{
		{"../outside: Retention policy", 0},
		{"/absolute: Retention policy", 0},
		{"C:/absolute: Retention policy", 0},
		{"orders/../billing: Retention policy", 0},
		{"orders: ", 0},
		{"orders: Retention policy\"\n  - \"orders: Disposal policy", 1},
	} {
		cfg := Defaults()
		if err := applyProjectConfig(&cfg, "module_purposes:\n  - \""+test.value+"\"\n"); err != nil {
			t.Fatalf("optional module metadata blocked loading: %v", err)
		}
		if len(cfg.ModulePurposes) != test.valid || len(cfg.Warnings) != 1 {
			t.Fatalf("%q: module purposes=%+v warnings=%+v", test.value, cfg.ModulePurposes, cfg.Warnings)
		}
		if cfg.Warnings[0].Key != "module_purposes" || cfg.Warnings[0].Line < 2 ||
			strings.Contains(cfg.Warnings[0].Message, "Retention policy") {
			t.Fatalf("invalid or unredacted warning: %+v", cfg.Warnings[0])
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

func TestModulePurposeWarningsPreserveOperationalSettings(t *testing.T) {
	cfg := Defaults()
	err := applyProjectConfig(&cfg, "module_purposes:\n  - \"src: Navigation\"\n  - \"../outside: private-value\"\n  -\ninclude:\n  - src/**\nexclude:\n  - generated/**\noutput: .analysis\nfollow_symlinks: false\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ModulePurposes) != 1 || cfg.ModulePurposes[0].Path != "src" || len(cfg.Warnings) != 2 {
		t.Fatalf("purposes=%+v warnings=%+v", cfg.ModulePurposes, cfg.Warnings)
	}
	if cfg.Warnings[0].Line != 3 || cfg.Warnings[1].Line != 4 || cfg.OutputDir != ".analysis" || cfg.FollowSymlinks ||
		!cfg.HasInclude("src/**") || !cfg.HasExclude("generated/**") || !cfg.HasExclude(".git/") {
		t.Fatalf("optional metadata changed scan settings: %+v", cfg)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Warnings") || strings.Contains(string(encoded), "entry ignored") {
		t.Fatalf("diagnostics leaked into serialized config: %s", encoded)
	}
}

func TestModulePurposesWarnOnWrongShapeAndExcessEntries(t *testing.T) {
	cfg := Defaults()
	if err := applyProjectConfig(&cfg, "module_purposes: private-value\noutput: .analysis\n"); err != nil {
		t.Fatal(err)
	}
	if len(cfg.ModulePurposes) != 0 || len(cfg.Warnings) != 1 || cfg.Warnings[0].Line != 1 || cfg.OutputDir != ".analysis" {
		t.Fatalf("unexpected scalar metadata result: %+v", cfg)
	}
	var body strings.Builder
	body.WriteString("module_purposes:\n")
	for index := 0; index < 34; index++ {
		fmt.Fprintf(&body, "  - \"module%d: Navigation\"\n", index)
	}
	cfg = Defaults()
	if err := applyProjectConfig(&cfg, body.String()); err != nil {
		t.Fatal(err)
	}
	if len(cfg.ModulePurposes) != 32 || len(cfg.Warnings) != 2 || cfg.Warnings[0].Line != 34 {
		t.Fatalf("unexpected bounded metadata result: %+v", cfg)
	}
}

func TestUnknownKeysHaveVersionHintRegardlessOfValue(t *testing.T) {
	for _, body := range []string{"new_setting:\n  - private-value\n", "new_setting: private-value\n"} {
		cfg := Defaults()
		err := applyProjectConfig(&cfg, body)
		if err == nil || !strings.Contains(err.Error(), "unsupported key \"new_setting\"") ||
			!strings.Contains(err.Error(), "newer version") || strings.Contains(err.Error(), "private-value") {
			t.Fatalf("unknown key has misleading diagnostic: %v", err)
		}
	}
}

func TestOperationalErrorsRemainFatalAfterOptionalMetadataWarning(t *testing.T) {
	for _, setting := range []string{"output:\n", "output: ../outside\n", "follow_symlinks: invalid\n", "exlcude:\n  - generated/**\n"} {
		cfg := Defaults()
		err := applyProjectConfig(&cfg, "module_purposes:\n  - \"../outside: private-value\"\n"+setting)
		if err == nil || len(cfg.Warnings) != 1 {
			t.Fatalf("operational error suppressed after metadata warning: %q: %v", setting, err)
		}
		if setting == "output:\n" && !strings.Contains(err.Error(), "requires a value") {
			t.Fatalf("known missing value lost its diagnostic: %v", err)
		}
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
