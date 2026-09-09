package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestWorkspaceScanAllContinuesAfterProjectPublicationFailure(t *testing.T) {
	workspace := t.TempDir()
	for _, project := range []string{"frontend/a", "frontend/b", "microservices/c"} {
		writeFile(t, filepath.Join(workspace, project), "package.json", `{"name":"example"}`)
	}
	workspaceOutput := filepath.Join(workspace, ".goregraph-workspace")
	if err := os.MkdirAll(workspaceOutput, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceOutput, "previous"), []byte("previous workspace output"), 0644); err != nil {
		t.Fatal(err)
	}
	options := scan.DefaultBuildOptions()
	options.Observer = func(event scan.BuildEvent) {
		if event.Project == "b" && event.Phase == "project" {
			writeFile(t, workspace, "frontend/b/goregraph-out", "output path blocked")
		}
	}
	var stdout, stderr bytes.Buffer
	code := runWorkspaceScanAll([]string{workspace, "--no-update-gitignore"}, &stdout, &stderr, buildExecution{ctx: context.Background(), options: options})
	if code != 1 {
		t.Fatalf("failed project must produce exit 1, got %d: %s", code, stderr.String())
	}
	for _, project := range []string{"frontend/a", "microservices/c"} {
		if _, err := os.Stat(filepath.Join(workspace, project, "goregraph-out", "manifest.json")); err != nil {
			t.Errorf("independent project %s was not published: %v; stderr=%s", project, err, stderr.String())
		}
	}
	if !strings.Contains(stdout.String(), "Completed [3/3] microservices/c") || !strings.Contains(stdout.String(), "2 succeeded, 1 failed") {
		t.Errorf("missing complete attempt summary: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "frontend/b") || !strings.Contains(stderr.String(), "reconciliation skipped") {
		t.Errorf("missing failure/reconciliation diagnostic: %s", stderr.String())
	}
	previous, err := os.ReadFile(filepath.Join(workspace, ".goregraph-workspace", "previous"))
	if err != nil || string(previous) != "previous workspace output" {
		t.Fatalf("previous workspace output changed: %q %v", previous, err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".goregraph-workspace", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("failed scan must not publish a reconciled workspace: %v", err)
	}
}

func TestWorkspaceScanAllStopsOnCancellation(t *testing.T) {
	workspace := t.TempDir()
	for _, project := range []string{"frontend/a", "microservices/b"} {
		writeFile(t, filepath.Join(workspace, project), "package.json", `{"name":"example"}`)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	options := scan.DefaultBuildOptions()
	options.Observer = func(event scan.BuildEvent) {
		if event.Project == "a" && event.Phase == "discover" {
			cancel()
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runWorkspaceScanAll([]string{workspace, "--no-update-gitignore"}, &stdout, &stderr, buildExecution{ctx: ctx, options: options}); code == 0 {
		t.Fatal("canceled scan returned success")
	}
	if _, err := os.Stat(filepath.Join(workspace, "microservices/b/goregraph-out")); !os.IsNotExist(err) {
		t.Fatalf("canceled scan started another project: %v", err)
	}
}
