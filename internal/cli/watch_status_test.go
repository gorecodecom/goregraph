package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/watch"
)

func TestWatchActivityShowsObservedProgressOnlyDuringAnActiveUpdate(t *testing.T) {
	now := time.Now()
	status := watch.Status{Running: true, UpdateStarted: now, Progress: &watch.Progress{Phase: "analyze", Project: "services/orders", File: "OrderService.go", Completed: 4, Total: 10, ProjectsCompleted: 1, ProjectsTotal: 3, LastProgress: now}}
	var output bytes.Buffer
	printWatchActivity(&output, status)
	for _, want := range []string{"Update phase: analyze", "Current project: services/orders", "Current file: OrderService.go", "Phase progress: 4/10", "Projects prepared: 1/3", "Last observed progress:"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q in status: %s", want, output.String())
		}
	}
	status.UpdateStarted = time.Time{}
	status.LastCheck = now
	output.Reset()
	printWatchActivity(&output, status)
	if strings.Contains(output.String(), "Update phase:") || strings.Contains(output.String(), "Projects prepared:") {
		t.Fatalf("idle status exposed a previous attempt's progress: %s", output.String())
	}
}

func TestRunWatchStatusReportsCoveringWorkspace(t *testing.T) {
	home, workspace, project := watchStatusWorkspaceFixture(t)
	writeWatchStatusState(t, home, workspace, true, true, "update failed")
	before := watchStatusStateFiles(t, home)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"watch", "status", project.Path}, &stdout, &stderr); code != 0 {
		t.Fatalf("status exit code = %d: %s", code, stderr.String())
	}
	for _, want := range []string{
		"Requested root: " + project.Path,
		"Direct watcher running: false",
		"Coverage: indexed project in active workspace watcher",
		"Project output: " + filepath.Join(project.Path, "goregraph-out"),
		"Root: " + workspace.Path,
		"Mode: workspace\nRunning: true\nAutostart: true",
		"Target output: " + filepath.Join(workspace.Path, ".goregraph-workspace"),
		"Last successful index update:",
		"Last error: update failed",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status missing %q:\n%s", want, stdout.String())
		}
	}
	if after := watchStatusStateFiles(t, home); !reflect.DeepEqual(before, after) {
		t.Fatal("status changed watcher state")
	}

	stdout.Reset()
	if code := Run([]string{"watch", "stop", project.Path}, &stdout, &stderr); code != 0 {
		t.Fatalf("direct stop exit code = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Watcher is not running.") {
		t.Fatalf("project stop targeted the covering workspace: %s", stdout.String())
	}
	if after := watchStatusStateFiles(t, home); !reflect.DeepEqual(before, after) {
		t.Fatal("project stop changed the covering workspace watcher")
	}
}

func TestWatchActivityDistinguishesIdleChecksFromIndexUpdates(t *testing.T) {
	checked := time.Date(2026, 10, 4, 20, 0, 0, 0, time.UTC)
	updated := checked.Add(-2 * time.Hour)
	for _, updating := range []bool{false, true} {
		status := watch.Status{Running: true, LastCheck: checked, LastSuccess: updated}
		if updating {
			status.UpdateStarted = checked.Add(time.Second)
		}
		var stdout bytes.Buffer
		printWatchActivity(&stdout, status)
		for _, want := range []string{"Last successful file check: 2026-10-04 20:00:00 UTC", "Last successful index update: 2026-10-04 18:00:00 UTC"} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("activity missing %q: %s", want, stdout.String())
			}
		}
		if updating && !strings.Contains(stdout.String(), "Activity: updating index\nUpdate started:") {
			t.Fatal(stdout.String())
		}
		if !updating && !strings.Contains(stdout.String(), "Activity: monitoring files") {
			t.Fatal(stdout.String())
		}
		if strings.Contains(stdout.String(), "stale") || strings.Contains(stdout.String(), "Last successful watcher check") {
			t.Fatalf("idle index was incorrectly described: %s", stdout.String())
		}
	}
}

func TestRunWatchStatusDoesNotInferUnverifiedWorkspaceCoverage(t *testing.T) {
	for _, scenario := range []string{"unindexed", "unregistered", "excluded_directory", "project_watcher", "stopped", "stale_heartbeat", "missing_registry", "invalid_registry", "foreign_registry", "explicit_workspace"} {
		t.Run(scenario, func(t *testing.T) {
			home, workspace, project := watchStatusWorkspaceFixture(t)
			writeWatchStatusState(t, home, workspace, scenario != "project_watcher", scenario != "stopped", "")
			registryPath := filepath.Join(workspace.Path, ".goregraph-workspace", "index", "registry.json")
			registry := scan.WorkspaceRegistryRecord{Root: workspace.Path, Projects: []scan.WorkspaceProjectRecord{{Path: "apps/web", Indexed: true}}}
			switch scenario {
			case "unindexed":
				registry.Projects[0].Indexed = false
				writeWatchStatusJSON(t, registryPath, registry)
			case "unregistered":
				registry.Projects[0].Path = "apps/other"
				writeWatchStatusJSON(t, registryPath, registry)
			case "excluded_directory":
				writeFile(t, workspace.Path, "node_modules/example/package.json", `{}`)
				var err error
				project, err = watch.Resolve(filepath.Join(workspace.Path, "node_modules", "example"), false)
				if err != nil {
					t.Fatal(err)
				}
			case "stale_heartbeat":
				writeWatchStatusJSON(t, filepath.Join(home, workspace.ID, "runtime.json"), map[string]any{"token": "fixture", "heartbeat": time.Now().Add(-time.Hour)})
			case "missing_registry":
				if err := os.Remove(registryPath); err != nil {
					t.Fatal(err)
				}
			case "invalid_registry":
				writeFile(t, workspace.Path, ".goregraph-workspace/index/registry.json", "invalid")
			case "foreign_registry":
				registry.Root = filepath.Join(workspace.Path, "other")
				writeWatchStatusJSON(t, registryPath, registry)
			}
			args := []string{"watch", "status", project.Path}
			if scenario == "explicit_workspace" {
				args = append(args, "--workspace")
			}
			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr); code != 0 {
				t.Fatalf("status exit code = %d: %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Root: "+project.Path) || !strings.Contains(stdout.String(), "Running: false") || strings.Contains(stdout.String(), "Coverage: indexed") {
				t.Fatalf("unverified path inherited workspace liveness:\n%s", stdout.String())
			}
			if strings.Contains(scenario, "registry") && !strings.Contains(stdout.String(), "Coverage warning:") {
				t.Fatalf("missing coverage warning:\n%s", stdout.String())
			}
		})
	}
}

func TestRunWatchStatusPrefersDirectProjectWatcher(t *testing.T) {
	home, workspace, project := watchStatusWorkspaceFixture(t)
	writeWatchStatusState(t, home, workspace, true, true, "")
	writeWatchStatusState(t, home, project, false, true, "")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"watch", "status", project.Path}, &stdout, &stderr); code != 0 {
		t.Fatalf("status exit code = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Root: "+project.Path) || !strings.Contains(stdout.String(), "Mode: project\nRunning: true") || strings.Contains(stdout.String(), "Requested root:") {
		t.Fatalf("direct project watcher lost precedence:\n%s", stdout.String())
	}
}

func watchStatusWorkspaceFixture(t *testing.T) (string, watch.Root, watch.Root) {
	t.Helper()
	home, directory := t.TempDir(), t.TempDir()
	t.Setenv("GOREGRAPH_WATCH_HOME", home)
	writeFile(t, directory, "apps/web/package.json", `{}`)
	workspace, err := watch.Resolve(directory, true)
	if err != nil {
		t.Fatal(err)
	}
	project, err := watch.Resolve(filepath.Join(directory, "apps", "web"), false)
	if err != nil {
		t.Fatal(err)
	}
	writeWatchStatusJSON(t, filepath.Join(workspace.Path, ".goregraph-workspace", "index", "registry.json"), scan.WorkspaceRegistryRecord{
		Root: workspace.Path, Projects: []scan.WorkspaceProjectRecord{{Path: "apps/web", Indexed: true}},
	})
	return home, workspace, project
}

func writeWatchStatusState(t *testing.T, home string, root watch.Root, workspace, running bool, lastError string) {
	t.Helper()
	writeWatchStatusJSON(t, filepath.Join(home, root.ID, "setting.json"), map[string]any{
		"root": root.Path, "workspace": workspace, "autostart": true,
	})
	writeWatchStatusJSON(t, filepath.Join(home, root.ID, "runtime.json"), map[string]any{
		"token": "fixture", "stopped": !running, "heartbeat": time.Now(), "last_success": time.Now(), "last_error": lastError,
	})
	if running {
		writeFile(t, home, filepath.Join(root.ID, "run.lock", "owner"), "fixture")
	}
}

func writeWatchStatusJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Dir(path), filepath.Base(path), string(body))
}

func watchStatusStateFiles(t *testing.T, directory string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err == nil {
			files[path] = string(body)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
