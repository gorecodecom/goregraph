package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/doctor"
	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/testresults"
)

func TestResultsImportRefreshesDashboardWithoutChangingAgent(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "frontend", "app")
	if err := os.MkdirAll(filepath.Join(root, ".storybook"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"app","scripts":{"test:storybook":"vitest run"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".storybook", "main.ts"), []byte(`export default { stories: ['../src/**/*.stories.tsx'] };`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
	before, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatal(err)
	}
	reportDir := filepath.Join(root, "output")
	if err := os.MkdirAll(reportDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reportDir, "interactions.xml"), []byte(`<testsuite timestamp="2026-09-25T06:00:00Z"><testcase name="works"/></testsuite>`), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"results", "import", root, "--suite", "storybook-interactions", "--from", "output/interactions.xml"}
	if code := Run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("import code %d: %s", code, stderr.String())
	}
	record, err := testresults.Load(filepath.Join(root, "goregraph-out"))
	if err != nil || len(record.Runs) != 1 || record.Runs[0].Status != "passed" {
		t.Fatalf("saved result: %+v, %v", record, err)
	}
	after, err := os.ReadFile(agentPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("agent projection changed during result import")
	}
	dashboard, err := os.ReadFile(filepath.Join(workspace, ".goregraph-workspace", "dashboard", "workspace-map.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"results":{"frontend/app"`, `"suite":"storybook-interactions"`, `Testergebnisse`, `Kein Ergebnis importiert`} {
		if !strings.Contains(string(dashboard), want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
	if err := os.WriteFile(filepath.Join(reportDir, "invalid.xml"), []byte(`<html/>`), 0600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"results", "import", root, "--suite", "storybook-interactions", "--from", "output/invalid.xml"}, &stdout, &stderr); code == 0 {
		t.Fatal("invalid JUnit was accepted")
	}
	retained, err := testresults.Load(filepath.Join(root, "goregraph-out"))
	if err != nil || retained.Runs[0].Status != "passed" {
		t.Fatalf("invalid import replaced valid evidence: %+v, %v", retained, err)
	}
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAll); err != nil {
		t.Fatalf("normal rebuild after import: %v", err)
	}
	retained, err = testresults.Load(filepath.Join(root, "goregraph-out"))
	if err != nil || len(retained.Runs) != 1 || retained.Runs[0].Status != "passed" {
		t.Fatalf("normal rebuild lost imported evidence: %+v, %v", retained, err)
	}
	health, err := doctor.Run(root)
	if err != nil || health.Failures != 0 || health.Warnings != 0 {
		t.Fatalf("import damaged output health: %+v, %v", health, err)
	}
}
