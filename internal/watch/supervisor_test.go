package watch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestSupervisorWorkerProcess(t *testing.T) {
	if os.Getenv("GOREGRAPH_SUPERVISOR_TEST_WORKER") != "1" {
		t.Skip("subprocess helper")
	}
	pollInterval, quietPeriod, heartbeatInterval = 20*time.Millisecond, 20*time.Millisecond, 20*time.Millisecond
	root, err := Resolve(os.Getenv("GOREGRAPH_SUPERVISOR_TEST_ROOT"), false)
	if err != nil {
		os.Exit(21)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err = Run(ctx, root, func() error {
		if os.Getenv("GOREGRAPH_SUPERVISOR_TEST_BLOCK") == "1" {
			if err := os.WriteFile(filepath.Join(root.Path, "updating"), nil, 0o600); err != nil {
				return err
			}
			for {
				if _, err := os.Stat(filepath.Join(root.Path, "finish")); err == nil {
					break
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(10 * time.Millisecond):
				}
			}
		}
		return nil
	})
	if err != nil {
		os.Exit(22)
	}
	os.Exit(0)
}

func supervisorFixture(t *testing.T, blocked bool) (Root, string, supervisorOptions, *atomic.Int32) {
	t.Helper()
	root := restartFixture(t)
	executable := filepath.Join(t.TempDir(), "installed")
	if err := os.WriteFile(executable, []byte("version-a"), 0o700); err != nil {
		t.Fatal(err)
	}
	starts := &atomic.Int32{}
	options := supervisorOptions{
		interval: 20 * time.Millisecond, retry: 20 * time.Millisecond,
		command: func(_ string, root Root) *exec.Cmd {
			starts.Add(1)
			command := exec.Command(os.Args[0], "-test.run=^TestSupervisorWorkerProcess$")
			command.Env = append(os.Environ(), "GOREGRAPH_SUPERVISOR_TEST_WORKER=1", "GOREGRAPH_SUPERVISOR_TEST_ROOT="+root.Path)
			if blocked {
				command.Env = append(command.Env, "GOREGRAPH_SUPERVISOR_TEST_BLOCK=1")
			}
			return command
		},
		validate: func(context.Context, string) error { return nil },
	}
	return root, executable, options, starts
}

func startTestSupervisor(t *testing.T, root Root, executable string, options supervisorOptions) <-chan error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- supervise(ctx, root, executable, options); close(done) }()
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(root.Path, "finish"), nil, 0o600)
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("test supervisor cleanup timed out")
		}
	})
	waitFor(t, func() bool {
		status, err := GetStatus(root)
		return err == nil && status.Running && status.Supervised
	})
	return done
}

func stopTestSupervisor(t *testing.T, root Root, done <-chan error) {
	t.Helper()
	if requested, err := RequestStop(root); err != nil || !requested {
		t.Fatalf("stop not requested: %t %v", requested, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("supervisor did not stop")
	}
	status, err := GetStatus(root)
	if err != nil || status.Running || status.Supervised || !status.Autostart {
		t.Fatalf("stop lost login setting or left a process: %+v %v", status, err)
	}
}

func TestSupervisorRecoversCrashedWorkerAndHonorsStop(t *testing.T) {
	root, executable, options, starts := supervisorFixture(t, false)
	done := startTestSupervisor(t, root, executable, options)
	before, _ := GetStatus(root)
	process, err := os.FindProcess(before.PID)
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Kill(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		status, err := GetStatus(root)
		return err == nil && status.Running && status.PID != before.PID
	})
	stopTestSupervisor(t, root, done)
	count := starts.Load()
	time.Sleep(80 * time.Millisecond)
	if count != 2 || starts.Load() != count {
		t.Fatalf("crash recovery or explicit stop launched extra writers: %d -> %d", count, starts.Load())
	}
}

func TestSupervisorReplacementWaitsForCurrentUpdate(t *testing.T) {
	root, executable, options, starts := supervisorFixture(t, true)
	done := startTestSupervisor(t, root, executable, options)
	waitFor(t, func() bool { _, err := os.Stat(filepath.Join(root.Path, "updating")); return err == nil })
	if err := os.WriteFile(executable, []byte("version-b"), 0o700); err != nil {
		t.Fatal(err)
	}
	stopPath, _ := statePath(root, "stop.request")
	waitFor(t, func() bool { _, err := os.Stat(stopPath); return err == nil })
	if starts.Load() != 1 {
		t.Fatal("replacement interrupted the current update")
	}
	if err := os.WriteFile(filepath.Join(root.Path, "finish"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return starts.Load() == 2 })
	waitFor(t, func() bool { status, err := GetStatus(root); return err == nil && status.Running })
	stopTestSupervisor(t, root, done)
}

func TestSupervisorRejectsInvalidReplacementWithoutStoppingWorker(t *testing.T) {
	root, executable, options, starts := supervisorFixture(t, false)
	var validations atomic.Int32
	options.validate = func(context.Context, string) error {
		validations.Add(1)
		return errors.New("replacement invalid")
	}
	done := startTestSupervisor(t, root, executable, options)
	before, _ := GetStatus(root)
	if err := os.WriteFile(executable, []byte("invalid-version"), 0o700); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return validations.Load() > 0 })
	status, err := GetStatus(root)
	if err != nil || !status.Running || status.PID != before.PID || starts.Load() != 1 || status.LastError != "replacement invalid" {
		t.Fatalf("invalid executable displaced a healthy worker: %+v starts=%d err=%v", status, starts.Load(), err)
	}
	stopTestSupervisor(t, root, done)
}

func TestSupervisorAdoptsOrphanWithoutInterruptingUpdate(t *testing.T) {
	root, executable, options, starts := supervisorFixture(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updating, finish := make(chan struct{}), make(chan struct{})
	old := make(chan error, 1)
	go func() { old <- Run(ctx, root, func() error { close(updating); <-finish; return nil }) }()
	<-updating
	supervisorCtx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan error, 1)
	go func() { done <- supervise(supervisorCtx, root, executable, options) }()
	stopPath, _ := statePath(root, "stop.request")
	waitFor(t, func() bool { _, err := os.Stat(stopPath); return err == nil })
	if starts.Load() != 0 {
		t.Fatal("orphan adoption launched a competing writer")
	}
	close(finish)
	if err := <-old; err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { status, err := GetStatus(root); return err == nil && status.Running && starts.Load() == 1 })
	stopTestSupervisor(t, root, done)
}

func TestExitedWorkerCannotReleaseAnotherOwner(t *testing.T) {
	root := restartFixture(t)
	release, token, err := acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	path, _ := statePath(root, "runtime.json")
	if err := writeJSON(path, runtimeState{PID: 123, Token: token, Heartbeat: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := releaseExitedWorker(root, 456); err != nil || !ownsLock(root, token) {
		t.Fatalf("another process's lock was released: %v", err)
	}
}

func TestSupervisorReloadDrainsAndCarriesStopAcrossHandoff(t *testing.T) {
	root, executable, options, starts := supervisorFixture(t, true)
	options.reload = true
	done := startTestSupervisor(t, root, executable, options)
	waitFor(t, func() bool { _, err := os.Stat(filepath.Join(root.Path, "updating")); return err == nil })
	if err := os.WriteFile(executable, []byte("version-b"), 0o700); err != nil {
		t.Fatal(err)
	}
	stopPath, _ := statePath(root, "stop.request")
	waitFor(t, func() bool { _, err := os.Stat(stopPath); return err == nil })
	if starts.Load() != 1 {
		t.Fatal("supervisor reload interrupted the update")
	}
	if err := os.WriteFile(filepath.Join(root.Path, "finish"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, ErrSupervisorUpgrade) {
		t.Fatalf("supervisor did not request replacement: %v", err)
	}
	if requested, err := RequestStop(root); err != nil || !requested {
		t.Fatalf("stop during handoff was lost: %t %v", requested, err)
	}
	if err := supervise(context.Background(), root, executable, options); err != nil {
		t.Fatal(err)
	}
	status, err := GetStatus(root)
	if err != nil || status.Supervised || status.Running || starts.Load() != 1 {
		t.Fatalf("replacement ignored explicit stop: %+v starts=%d err=%v", status, starts.Load(), err)
	}
}

func TestExecutableIdentityFollowsInstallationSymlink(t *testing.T) {
	directory := t.TempDir()
	a, b, alias := filepath.Join(directory, "a"), filepath.Join(directory, "b"), filepath.Join(directory, "current")
	for path, body := range map[string]string{a: "old", b: "new"} {
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(a, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	before, err := readExecutable(alias, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(b, alias); err != nil {
		t.Fatal(err)
	}
	after, err := readExecutable(alias, &before)
	if err != nil || before.hash == after.hash {
		t.Fatalf("installation symlink replacement was missed: %v", err)
	}
}

func TestSupervisorLeaseLossDoesNotOverwriteNewOwner(t *testing.T) {
	root, executable, options, _ := supervisorFixture(t, false)
	done := startTestSupervisor(t, root, executable, options)
	lock, _ := statePath(root, "supervisor.lock")
	path, _ := statePath(root, "supervisor.json")
	const newToken = "replacement-owner"
	if err := os.WriteFile(filepath.Join(lock, "owner"), []byte(newToken), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path, runtimeState{Token: newToken, PID: 99999, Heartbeat: time.Now()}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("displaced supervisor did not drain")
	}
	var live runtimeState
	if err := readJSON(path, &live); err != nil || live.Token != newToken || live.Stopped || !ownsNamedLock(root, newToken, "supervisor.lock") {
		t.Fatalf("old supervisor overwrote or released the new owner's state: %+v %v", live, err)
	}
}

func TestReplacementRejectsLegacySupervisorProtocol(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	path := filepath.Join(t.TempDir(), "legacy-goregraph")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nif [ \"$1\" = version ]; then echo 'goregraph 1.4.3'; exit 0; fi\nexit 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validateReplacement(context.Background(), path); err == nil {
		t.Fatal("an older executable without supervisor support was accepted")
	}
}
