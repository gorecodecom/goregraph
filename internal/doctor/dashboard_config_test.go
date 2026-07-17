package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDoctorReportsInvalidWorkspaceDashboardConfig(t *testing.T) {
	workspace := t.TempDir()
	out := filepath.Join(workspace, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspace, scan.WorkspaceDashboardConfigName)
	wantConfig := []byte(`{"schema":1,"architecture":{"services":{"services/removed":{"group":"missing"}}}}`)
	if err := os.WriteFile(configPath, wantConfig, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(workspace)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 || !containsLine(result.Lines, scan.WorkspaceDashboardConfigName) {
		t.Fatalf("Doctor did not report invalid dashboard config: %#v", result)
	}
	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Doctor removed dashboard config: %v", err)
	}
	if string(gotConfig) != string(wantConfig) {
		t.Fatalf("Doctor mutated dashboard config: got %s, want %s", gotConfig, wantConfig)
	}
}

func TestDoctorWarnsAboutStaleWorkspaceDashboardService(t *testing.T) {
	workspace := t.TempDir()
	out := filepath.Join(workspace, ".goregraph-workspace")
	if err := os.MkdirAll(filepath.Join(out, "index"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(out, "manifest.json"), scan.OutputManifest{
		Tool: scan.ToolName, Schema: scan.SchemaVersion, Scope: "workspace", OutputDir: ".goregraph-workspace",
		Index: scan.ProjectionStatus{Complete: true, Files: []string{"index/registry.json"}},
	})
	writeTestJSON(t, filepath.Join(out, "index", "registry.json"), scan.WorkspaceRegistryRecord{
		Root:     filepath.ToSlash(workspace),
		Projects: []scan.WorkspaceProjectRecord{{Path: "services/current", Indexed: true, Status: "indexed"}},
	})
	configPath := filepath.Join(workspace, scan.WorkspaceDashboardConfigName)
	wantConfig := []byte(`{"schema":1,"architecture":{"groups":{"custom":{"label":"Custom"}},"services":{"services/removed":{"group":"custom"}}}}`)
	if err := os.WriteFile(configPath, wantConfig, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(workspace)

	if err != nil {
		t.Fatal(err)
	}
	if result.Warnings == 0 || !containsLine(result.Lines, "services/removed") {
		t.Fatalf("Doctor did not report stale dashboard service: %#v", result)
	}
	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Doctor removed dashboard config: %v", err)
	}
	if string(gotConfig) != string(wantConfig) {
		t.Fatalf("Doctor mutated dashboard config: got %s, want %s", gotConfig, wantConfig)
	}
}
