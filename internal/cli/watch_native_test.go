package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/watch"
)

// This opt-in test creates only a temporary project and user-session service.
func TestNativeMacWatcherLifecycle(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("GOREGRAPH_NATIVE_WATCHER_SMOKE") != "1" {
		t.Skip("set GOREGRAPH_NATIVE_WATCHER_SMOKE=1 for the native macOS lifecycle test")
	}
	base := t.TempDir()
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("GOREGRAPH_WATCH_HOME", filepath.Join(base, "watch"))
	rootPath := filepath.Join(base, "project")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, rootPath, "go.mod", "module fixture\n\ngo 1.23\n")
	writeFile(t, rootPath, "main.go", "package fixture\nfunc Example() {}\n")
	root, err := watch.Resolve(rootPath, false)
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(base, "goregraph")
	replacement := filepath.Join(base, "replacement")
	for path, commit := range map[string]string{executable: "native-fixture-a", replacement: "native-fixture-b"} {
		command := exec.Command("go", "build", "-trimpath", "-ldflags", "-X github.com/gorecodecom/goregraph/internal/version.Commit="+commit, "-o", path, "../../cmd/goregraph")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build native fixture: %v: %s", err, output)
		}
	}
	command := func(args ...string) {
		t.Helper()
		if output, err := exec.Command(executable, args...).CombinedOutput(); err != nil {
			t.Fatalf("native watcher command %v: %v: %s", args, err, output)
		}
	}
	status := func() watch.Status {
		t.Helper()
		value, err := watch.GetStatus(root)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	wait := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(time.Minute)
		for time.Now().Before(deadline) {
			if condition() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("native watcher condition timed out: %+v", status())
	}
	t.Cleanup(func() {
		_ = exec.Command(executable, "watch", "stop", rootPath).Run()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			value, err := watch.GetStatus(root)
			if err != nil || (!value.Running && !value.Supervised) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		service := "gui/" + strconv.Itoa(os.Getuid()) + "/com.gorecode.goregraph.watch." + root.ID
		_ = exec.Command("launchctl", "bootout", service).Run()
	})
	command("watch", "start", rootPath, "--autostart", "off")
	wait(func() bool {
		value := status()
		return value.Running && value.Supervised && value.Commit == "native-fixture-a"
	})
	before := status()
	process, err := os.FindProcess(before.PID)
	if err != nil || process.Kill() != nil {
		t.Fatalf("kill fixture worker: %v", err)
	}
	wait(func() bool { value := status(); return value.Running && value.PID != before.PID })
	if err := os.Rename(replacement, executable); err != nil {
		t.Fatal(err)
	}
	wait(func() bool { return status().Commit == "native-fixture-b" })
	path := filepath.Join(os.Getenv("GOREGRAPH_WATCH_HOME"), root.ID, "supervisor.json")
	var supervisor struct{ Commit string }
	body, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(body, &supervisor) != nil || supervisor.Commit != "native-fixture-b" {
		t.Fatalf("supervisor did not update with its worker: %+v %v", supervisor, err)
	}
	before = status()
	process, err = os.FindProcess(before.SupervisorPID)
	if err != nil || process.Kill() != nil {
		t.Fatalf("kill fixture supervisor: %v", err)
	}
	wait(func() bool {
		value := status()
		return value.Running && value.Supervised && value.SupervisorPID != before.SupervisorPID
	})
	command("watch", "stop", rootPath)
	wait(func() bool { value := status(); return !value.Running && !value.Supervised })
	time.Sleep(12 * time.Second)
	if value := status(); value.Running || value.Supervised {
		t.Fatalf("explicit stop was undone: %+v", value)
	}
	command("watch", "autostart", "on", rootPath)
	wait(func() bool { value := status(); return value.Running && value.Supervised && value.Autostart })
	command("watch", "stop", rootPath)
	wait(func() bool { value := status(); return !value.Running && !value.Supervised && value.Autostart })
}
