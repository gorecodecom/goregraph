package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDoctorRejectsInvalidManifestListedWorkspaceCanonicalJSON(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "api")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.test/api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectCfg := config.Defaults()
	projectCfg.Workspace = false
	if _, err := scan.RunBuild(project, projectCfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	workspaceCfg := config.Defaults()
	workspaceCfg.Workspace = true
	workspaceCfg.WorkspaceRoot = workspace
	if _, err := scan.ReconcileWorkspaceTarget(project, workspaceCfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	contextPath := scan.NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace")).Index("context.json")
	if err := os.WriteFile(contextPath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(workspace)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 {
		t.Fatalf("invalid workspace canonical JSON passed Doctor: %v", result.Lines)
	}
	if !containsLine(result.Lines, "context.json invalid") {
		t.Fatalf("Doctor did not identify the invalid canonical file: %v", result.Lines)
	}
	if containsLine(result.Lines, "symbol projection") {
		t.Fatalf("agent-only workspace unexpectedly ran dashboard symbol validation: %v", result.Lines)
	}
	for _, line := range result.Lines {
		if strings.Contains(line, "context.json valid") {
			t.Fatalf("Doctor reported corrupted context.json as valid: %v", result.Lines)
		}
	}
}
