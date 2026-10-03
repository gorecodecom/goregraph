package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func restartFixture(t *testing.T) Root {
	t.Helper()
	t.Setenv("GOREGRAPH_WATCH_HOME", t.TempDir())
	originalPoll, originalHeartbeat := pollInterval, heartbeatInterval
	pollInterval, heartbeatInterval = 20*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { pollInterval, heartbeatInterval = originalPoll, originalHeartbeat })
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := Resolve(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureStateDir(root); err != nil {
		t.Fatal(err)
	}
	settingPath, _ := statePath(root, "setting.json")
	if err := writeJSON(settingPath, setting{Root: root.Path, Autostart: true, Method: "launchd"}); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRestartWaitsForUpdateAndPreservesAutostart(t *testing.T) {
	root := restartFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updating, finish := make(chan struct{}), make(chan struct{})
	finished := false
	defer func() {
		if !finished {
			close(finish)
		}
	}()
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, root, func() error { close(updating); <-finish; return nil })
	}()
	<-updating
	var starts atomic.Int32
	restarted := make(chan error, 1)
	go func() {
		restartCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		restarted <- restartWithStart(restartCtx, root, func(root Root) error {
			lock, _ := statePath(root, "run.lock")
			if _, err := os.Lstat(lock); !os.IsNotExist(err) {
				return errors.New("new process launched before previous lock release")
			}
			starts.Add(1)
			return nil
		})
	}()
	stopPath, _ := statePath(root, "stop.request")
	waitFor(t, func() bool { _, err := os.Stat(stopPath); return err == nil })
	if starts.Load() != 0 {
		t.Fatal("restart interrupted the running update")
	}
	close(finish)
	finished = true
	if err := <-restarted; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	status, err := GetStatus(root)
	if err != nil || starts.Load() != 1 || !status.Autostart || status.Method != "launchd" {
		t.Fatalf("restart did not preserve autostart: starts=%d status=%+v err=%v", starts.Load(), status, err)
	}
	// The old process's stop token must not stop the replacement process.
	newCtx, stop := context.WithCancel(context.Background())
	defer stop()
	newDone := make(chan error, 1)
	go func() { newDone <- Run(newCtx, root, func() error { return nil }) }()
	waitFor(t, func() bool { status, err := GetStatus(root); return err == nil && status.Running })
	time.Sleep(80 * time.Millisecond)
	if status, err := GetStatus(root); err != nil || !status.Running {
		t.Fatalf("replacement watcher stopped: %+v %v", status, err)
	}
	stop()
	if err := <-newDone; err != nil {
		t.Fatal(err)
	}
}

func TestRestartTimeoutDoesNotStartAnotherProcess(t *testing.T) {
	root := restartFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updating, finish := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, func() error { close(updating); <-finish; return nil }) }()
	<-updating
	restartCtx, stop := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer stop()
	starts := 0
	err := restartWithStart(restartCtx, root, func(Root) error { starts++; return nil })
	close(finish)
	if !errors.Is(err, context.DeadlineExceeded) || starts != 0 {
		t.Fatalf("timeout launched a replacement: starts=%d err=%v", starts, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRestartStartsStoppedRootAndHonorsCancellation(t *testing.T) {
	root := restartFixture(t)
	starts := 0
	start := func(Root) error { starts++; return nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := restartWithStart(ctx, root, start); !errors.Is(err, context.Canceled) || starts != 0 {
		t.Fatalf("canceled restart launched a process: %v", err)
	}
	if err := restartWithStart(context.Background(), root, start); err != nil || starts != 1 {
		t.Fatalf("stopped root did not start: starts=%d err=%v", starts, err)
	}
}
