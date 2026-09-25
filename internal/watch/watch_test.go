package watch

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcherUpdatesSelectedFilesAndIgnoresGeneratedOutput(t *testing.T) {
	originalConfigDir, originalPoll, originalQuiet, originalHeartbeat := userConfigDir, pollInterval, quietPeriod, heartbeatInterval
	configPath := t.TempDir()
	userConfigDir = func() (string, error) { return configPath, nil }
	pollInterval, quietPeriod, heartbeatInterval = 20*time.Millisecond, 40*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() {
		userConfigDir, pollInterval, quietPeriod, heartbeatInterval = originalConfigDir, originalPoll, originalQuiet, originalHeartbeat
	})
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := Resolve(rootPath, false)
	if err != nil {
		t.Fatal(err)
	}
	var updates atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, func() error { updates.Add(1); return nil }) }()
	waitFor(t, func() bool {
		status, err := GetStatus(root)
		return err == nil && status.Running && updates.Load() == 1
	})

	output := filepath.Join(rootPath, "goregraph-out")
	if err := os.MkdirAll(output, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "report.md"), []byte("generated"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(140 * time.Millisecond)
	if got := updates.Load(); got != 1 {
		t.Fatalf("generated output triggered %d updates, want 1", got)
	}

	if err := os.WriteFile(filepath.Join(rootPath, "main.go"), []byte("package main\n// changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return updates.Load() == 2 })
	time.Sleep(100 * time.Millisecond)
	if got := updates.Load(); got != 2 {
		t.Fatalf("one source edit triggered %d updates, want 2", got)
	}

	requested, err := RequestStop(root)
	if err != nil || !requested {
		t.Fatalf("stop requested=%t err=%v", requested, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}
	status, err := GetStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if status.Running {
		t.Fatal("stopped watcher still reported as running")
	}
	if status.LastSuccess.IsZero() || status.LastError != "" {
		t.Fatalf("stopped watcher lost its successful update or gained an error: %+v", status)
	}
}

func TestStatusKeepsAutostartSeparateFromLiveness(t *testing.T) {
	original := userConfigDir
	configPath := t.TempDir()
	userConfigDir = func() (string, error) { return configPath, nil }
	t.Cleanup(func() { userConfigDir = original })
	root, err := Resolve(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureStateDir(root); err != nil {
		t.Fatal(err)
	}
	settingPath, err := statePath(root, "setting.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(settingPath, setting{Root: root.Path, Autostart: true, Method: "launchd"}); err != nil {
		t.Fatal(err)
	}
	runtimePath, err := statePath(root, "runtime.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(runtimePath, runtimeState{Token: "dead", Heartbeat: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	status, err := GetStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if status.Running || !status.Autostart {
		t.Fatalf("status = %+v, want stopped with autostart on", status)
	}
	if !strings.Contains(status.LastError, "stale") {
		t.Fatalf("missing stale process hint: %+v", status)
	}
}

func TestWatcherRejectsStateInsideWatchedRoot(t *testing.T) {
	root, err := Resolve(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOREGRAPH_WATCH_HOME", filepath.Join(root.Path, "watch-state"))
	if err := ensureStateDir(root); err == nil {
		t.Fatal("watcher state inside the source tree would trigger self-updates")
	}
}

func TestStaleWatcherCannotRemoveReplacementLock(t *testing.T) {
	original := userConfigDir
	configPath := t.TempDir()
	userConfigDir = func() (string, error) { return configPath, nil }
	t.Cleanup(func() { userConfigDir = original })
	root, err := Resolve(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureStateDir(root); err != nil {
		t.Fatal(err)
	}
	releaseOld, oldToken, err := acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := statePath(root, "run.lock")
	if err != nil {
		t.Fatal(err)
	}
	stale := lock + ".stale"
	if err := os.Rename(lock, stale); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(lock, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lock, "owner"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	releaseOld()
	if ownsLock(root, oldToken) {
		t.Fatal("old watcher still owns the replacement lock")
	}
	if body, err := os.ReadFile(filepath.Join(lock, "owner")); err != nil || string(body) != "replacement" {
		t.Fatalf("replacement lock was removed: %q, %v", body, err)
	}
}

func TestWorkspaceFingerprintTracksProjectChanges(t *testing.T) {
	workspace := t.TempDir()
	orders := filepath.Join(workspace, "services", "orders")
	if err := os.MkdirAll(orders, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orders, "go.mod"), []byte("module example.test/orders\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orders, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := Resolve(workspace, true)
	if err != nil {
		t.Fatal(err)
	}
	before, err := fingerprint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orders, "main.go"), []byte("package main\nconst value = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := fingerprint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("workspace fingerprint ignored a project edit")
	}
	users := filepath.Join(workspace, "services", "users")
	if err := os.MkdirAll(users, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(users, "go.mod"), []byte("module example.test/users\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	added, err := fingerprint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if after == added {
		t.Fatal("workspace fingerprint ignored a new project")
	}
}

func TestWatcherRechecksChangesMadeDuringUpdate(t *testing.T) {
	originalConfigDir, originalPoll, originalQuiet, originalHeartbeat := userConfigDir, pollInterval, quietPeriod, heartbeatInterval
	configPath := t.TempDir()
	userConfigDir = func() (string, error) { return configPath, nil }
	pollInterval, quietPeriod, heartbeatInterval = 20*time.Millisecond, 40*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() {
		userConfigDir, pollInterval, quietPeriod, heartbeatInterval = originalConfigDir, originalPoll, originalQuiet, originalHeartbeat
	})
	path := t.TempDir()
	source := filepath.Join(path, "main.go")
	if err := os.WriteFile(source, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := Resolve(path, false)
	if err != nil {
		t.Fatal(err)
	}
	var updates atomic.Int32
	secondStarted := make(chan struct{})
	releaseSecond := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, root, func() error {
			if updates.Add(1) == 2 {
				close(secondStarted)
				<-releaseSecond
			}
			return nil
		})
	}()
	waitFor(t, func() bool { return updates.Load() == 1 })
	if err := os.WriteFile(source, []byte("package main\nconst first = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("second update did not start")
	}
	if err := os.WriteFile(source, []byte("package main\nconst second = 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	close(releaseSecond)
	waitFor(t, func() bool { return updates.Load() == 3 })
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not exit")
	}
}

func TestHelpSummaryFindsRunningWorkspaceFromNestedProject(t *testing.T) {
	original := userConfigDir
	configPath := t.TempDir()
	userConfigDir = func() (string, error) { return configPath, nil }
	t.Cleanup(func() { userConfigDir = original })
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "orders")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.test/orders\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := Resolve(workspace, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureStateDir(root); err != nil {
		t.Fatal(err)
	}
	settingPath, err := statePath(root, "setting.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(settingPath, setting{Root: root.Path, Workspace: true, Autostart: true}); err != nil {
		t.Fatal(err)
	}
	runtimePath, err := statePath(root, "runtime.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(runtimePath, runtimeState{Token: "active", Heartbeat: time.Now()}); err != nil {
		t.Fatal(err)
	}
	lockPath, err := statePath(root, "run.lock")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lockPath, "owner"), []byte("active"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	summary := HelpSummary()
	if !strings.Contains(summary, "Current root (workspace): running true · autostart true") {
		t.Fatalf("help missed the active parent watcher: %s", summary)
	}
}

func TestMacAutostartIsUserScopedAndRemovable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS LaunchAgent only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	original := userConfigDir
	userConfigDir = func() (string, error) { return filepath.Join(home, "Library", "Application Support"), nil }
	t.Cleanup(func() { userConfigDir = original })
	root, err := Resolve(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetAutostart(root, true, filepath.Join(home, "bin", "goregraph")); err != nil {
		t.Fatal(err)
	}
	status, err := GetStatus(root)
	if err != nil || !status.Autostart || status.Running || status.Method != "launchd" {
		t.Fatalf("autostart status = %+v, err=%v", status, err)
	}
	plist := filepath.Join(home, "Library", "LaunchAgents", "com.gorecode.goregraph.watch."+root.ID+".plist")
	if _, err := os.Stat(plist); err != nil {
		t.Fatalf("LaunchAgent missing: %v", err)
	}
	if err := os.Remove(root.Path); err != nil {
		t.Fatal(err)
	}
	missing, err := Resolve(root.Path, false)
	if err != nil || missing.ID != root.ID {
		t.Fatalf("deleted root is no longer addressable: %+v, %v", missing, err)
	}
	if err := SetAutostart(missing, false, filepath.Join(home, "bin", "goregraph")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Fatalf("LaunchAgent still present: %v", err)
	}
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not reached")
}
