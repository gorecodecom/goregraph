package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectConfigLoaderReportsOptionalWarningsOnceWithoutValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "goregraph.yml")
	if err := os.WriteFile(path, []byte("module_purposes:\n  - \"src: Useful description\"\n  - \"../outside: private-value\"\noutput: .analysis\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	loader := newProjectConfigLoader(&stderr)
	for _, alias := range []string{root, filepath.Join(root, ".")} {
		cfg, err := loader.load(alias)
		if err != nil || cfg.OutputDir != ".analysis" || len(cfg.ModulePurposes) != 1 {
			t.Fatalf("metadata warning prevented configuration loading: %+v: %v", cfg, err)
		}
	}
	got := stderr.String()
	if strings.Count(got, "warning:") != 1 || !strings.Contains(got, path+":3: module_purposes:") ||
		!strings.Contains(got, "entry ignored") || strings.Contains(got, "private-value") || strings.Contains(got, "../outside") {
		t.Fatalf("unexpected warning output: %s", got)
	}
}

func TestWorkspacePlanReportsOptionalRootMetadataWarning(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "goregraph.yml"), []byte("module_purposes: private-value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "service")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/service\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "scan-all", root, "--workspace", root, "--dry-run"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stderr.String(), "module_purposes: expected a list") || strings.Contains(stderr.String(), "private-value") ||
		!strings.Contains(stdout.String(), "project `.`") || !strings.Contains(stdout.String(), "Dry run: true") {
		t.Fatalf("plan exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "goregraph-out")); !os.IsNotExist(err) {
		t.Fatalf("dry-run produced scan output or could not verify its absence: %v", err)
	}
}
