package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/watch"
)

func TestContextWatcherCoverageAndReadOnlyState(t *testing.T) {
	for _, scenario := range []string{"workspace", "direct", "updating", "recovering", "unindexed", "foreign", "invalid", "stopped", "stale_heartbeat", "error"} {
		t.Run(scenario, func(t *testing.T) {
			home, directory := t.TempDir(), t.TempDir()
			t.Setenv("GOREGRAPH_WATCH_HOME", home)
			directory, _ = filepath.EvalSymlinks(directory)
			project := filepath.Join(directory, "app")
			if err := os.Mkdir(project, 0755); err != nil {
				t.Fatal(err)
			}
			workspace, _ := watch.Resolve(directory, true)
			direct, _ := watch.Resolve(project, false)
			registry := scan.WorkspaceRegistryRecord{Root: directory, Projects: []scan.WorkspaceProjectRecord{{Path: "app", Indexed: scenario != "unindexed"}}}
			if scenario == "foreign" {
				registry.Root = t.TempDir()
			}
			writeRuntimeJSON(t, filepath.Join(directory, ".goregraph-workspace", "index", "registry.json"), registry)
			if scenario == "invalid" {
				writeSourceFile(t, directory, ".goregraph-workspace/index/registry.json", "{")
			}
			now := time.Now().UTC()
			state := map[string]any{"token": "test-private-token", "heartbeat": now, "last_check": now, "last_success": now.Add(-24 * time.Hour), "stopped": scenario == "stopped" || scenario == "recovering"}
			if scenario == "stale_heartbeat" {
				state["heartbeat"] = now.Add(-time.Minute)
			}
			if scenario == "updating" {
				state["update_started"] = now
				state["progress"] = watch.Progress{Phase: "validate", Completed: 2, Total: 4}
			}
			if scenario == "error" {
				state["last_error"] = "private connection details"
			}
			writeRuntimeWatcher(t, home, workspace, true, state)
			if scenario == "recovering" {
				writeRuntimeJSON(t, filepath.Join(home, workspace.ID, "supervisor.json"), map[string]any{"upgrading": true, "heartbeat": now})
			}
			if scenario == "direct" {
				writeRuntimeWatcher(t, home, direct, false, state)
			}
			before := snapshotRuntimeFiles(t, home)
			status := inspectContextWatcher(project)
			if !reflect.DeepEqual(before, snapshotRuntimeFiles(t, home)) {
				t.Fatal("read-only status changed watcher files")
			}
			switch scenario {
			case "workspace", "updating", "error", "recovering":
				if status.Coverage != "workspace_project" || status.Root != directory || status.LastCheck == nil || status.LastIndexUpdate == nil {
					t.Fatalf("workspace coverage lost: %+v", status)
				}
				if scenario == "recovering" && status.State != "recovering" {
					t.Fatalf("recovering workspace reported stopped: %+v", status)
				}
				if scenario == "updating" && (status.State != "updating" || status.Phase != "validate" || status.Completed != 2) {
					t.Fatalf("update activity lost: %+v", status)
				}
			case "direct":
				if status.State != "monitoring" || status.Coverage != "direct" || status.Root != project {
					t.Fatalf("direct precedence lost: %+v", status)
				}
			case "invalid", "foreign":
				if status.State != "unknown" || status.Coverage != "unverified" {
					t.Fatalf("invalid registry trusted: %+v", status)
				}
			default:
				if status.State != "not_running" || status.Coverage != "none" {
					t.Fatalf("inactive or uncovered project inherited watcher: %+v", status)
				}
			}
			body, _ := json.Marshal(status)
			if strings.Contains(string(body), "test-private-token") || strings.Contains(string(body), "private connection") {
				t.Fatal("private watcher state leaked")
			}
		})
	}
}

func TestContextWatcherDoesNotChangeFreshnessAndFitsBudgets(t *testing.T) {
	t.Setenv("GOREGRAPH_WATCH_HOME", t.TempDir())
	root := t.TempDir()
	writeSourceFile(t, root, "go.mod", "module example.test/runtime\n\ngo 1.24\n")
	writeSourceFile(t, root, "main.go", "package main\nfunc RunWork() {}\n")
	cfg := config.Defaults()
	cfg.Workspace, cfg.UpdateGitignore = false, false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	initializeSourceReadLocks(t, root)
	for _, budget := range []int{256, 512, 4000} {
		request := ContextRequest{Root: root, Query: "RunWork implementation", BudgetTokens: budget, ProtocolVersion: AdaptiveV2}
		request, err := normalizeContextRequest(request)
		if err != nil {
			t.Fatal(err)
		}
		base, err := buildContext(request)
		if err != nil {
			t.Fatal(err)
		}
		pack, err := BuildContext(request)
		if err != nil {
			t.Fatal(err)
		}
		if pack.Freshness != base.Freshness || !reflect.DeepEqual(pack.Health, base.Health) || !runtimeJSONEqual(pack.CallChain, base.CallChain) || !runtimeJSONEqual(pack.SourceSections, base.SourceSections) {
			t.Fatal("operational status changed evidence or freshness")
		}
		if fits, err := contextPackFitsBudget(pack, budget); err != nil || !fits {
			t.Fatalf("budget %d exceeded: %v", budget, err)
		}
		if budget == 4000 && (pack.Watcher == nil || pack.Watcher.State != "not_running") {
			t.Fatalf("default context has no watcher information: %+v", pack)
		}
	}
}

func writeRuntimeWatcher(t *testing.T, home string, root watch.Root, workspace bool, state any) {
	t.Helper()
	directory := filepath.Join(home, root.ID)
	writeRuntimeJSON(t, filepath.Join(directory, "setting.json"), map[string]any{"root": root.Path, "workspace": workspace, "autostart": true})
	writeRuntimeJSON(t, filepath.Join(directory, "runtime.json"), state)
	writeSourceFile(t, directory, "run.lock/owner", "test-private-token")
}

func writeRuntimeJSON(t *testing.T, name string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeSourceFile(t, filepath.Dir(name), filepath.Base(name), string(body))
}

func snapshotRuntimeFiles(t *testing.T, directory string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(directory, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			body, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			result[name] = string(body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func runtimeJSONEqual(left, right any) bool {
	l, _ := json.Marshal(left)
	r, _ := json.Marshal(right)
	return string(l) == string(r)
}

func TestContextWatcherInspectionDoesNotCreateStateHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "missing-watch-home")
	t.Setenv("GOREGRAPH_WATCH_HOME", home)
	status := inspectContextWatcher(t.TempDir())
	if status.State != "not_running" || status.Coverage != "none" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("status created state home: %v", err)
	}
}
