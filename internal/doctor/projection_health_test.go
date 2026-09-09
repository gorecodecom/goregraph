package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func writeDoctorFile(root, relative, body string) error {
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func TestDoctorReportsStaleWorkspaceProjection(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "api")
	if err := writeDoctorFile(project, "go.mod", "module example.test/api\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeDoctorFile(project, "main.go", "package main\n"); err != nil {
		t.Fatal(err)
	}
	projectCfg := config.Defaults()
	projectCfg.Workspace = false
	if _, err := scan.RunBuild(project, projectCfg, scan.BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	workspaceCfg := config.Defaults()
	workspaceCfg.Workspace = true
	workspaceCfg.WorkspaceRoot = workspace
	if _, err := scan.ReconcileWorkspaceTarget(project, workspaceCfg, scan.BuildTargetAll); err != nil {
		t.Fatal(err)
	}

	manifestPath := scan.NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace")).Manifest
	var manifest scan.Manifest
	readTestJSON(t, manifestPath, &manifest)
	manifest.Dashboard.Stale = true
	writeTestJSON(t, manifestPath, manifest)

	result, err := Run(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if !containsLine(result.Lines, "dashboard: projection is readable but stale") {
		t.Fatalf("Doctor did not report stale workspace dashboard: %v", result.Lines)
	}
}

func TestDoctorQualifiesNonDefaultAnalysisPolicy(t *testing.T) {
	root := t.TempDir()
	if err := writeDoctorFile(root, "main.go", "package main\n"); err != nil {
		t.Fatal(err)
	}
	options := scan.DefaultBuildOptions()
	options.FileTimeout = 0
	if _, err := scan.RunBuildWithOptions(context.Background(), root, config.Defaults(), scan.BuildTargetAgent, options); err != nil {
		t.Fatal(err)
	}

	result, err := Run(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsLine(result.Lines, "analysis policy differs from Doctor defaults") {
		t.Fatalf("Doctor did not qualify its default-policy comparison: %v", result.Lines)
	}
	if result.Warnings != 0 {
		t.Fatalf("healthy custom analysis policy produced warnings: %v", result.Lines)
	}
}
