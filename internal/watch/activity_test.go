package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIdleFileChecksDoNotClaimNewIndexUpdates(t *testing.T) {
	root := restartFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var updates atomic.Int32
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, func() error { updates.Add(1); return nil }) }()
	waitFor(t, func() bool {
		status, _ := GetStatus(root)
		return !status.LastSuccess.IsZero() && !status.LastCheck.IsZero()
	})
	before, err := GetStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { status, _ := GetStatus(root); return status.LastCheck.After(before.LastCheck) })
	after, err := GetStatus(root)
	if err != nil || !after.Running || !after.UpdateStarted.IsZero() || !after.LastSuccess.Equal(before.LastSuccess) || updates.Load() != 1 {
		t.Fatalf("idle file check changed index-update state: before=%+v after=%+v updates=%d err=%v", before, after, updates.Load(), err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestWatcherReportsActiveUpdateAndRetainsLastSuccessfulSnapshotOnFailure(t *testing.T) {
	for _, fails := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fails], func(t *testing.T) {
			root := restartFixture(t)
			previousQuiet := quietPeriod
			quietPeriod = 20 * time.Millisecond
			t.Cleanup(func() { quietPeriod = previousQuiet })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready, release := make(chan struct{}), make(chan struct{})
			finish := sync.OnceFunc(func() { close(release) })
			defer finish()
			var updates atomic.Int32
			done := make(chan error, 1)
			go func() {
				done <- Run(ctx, root, func() error {
					attempt := updates.Add(1)
					if attempt == 1 {
						return nil
					}
					if attempt == 2 {
						close(ready)
						<-release
					}
					if fails {
						return errors.New("injected update failure")
					}
					return nil
				})
			}()
			waitFor(t, func() bool { status, _ := GetStatus(root); return !status.LastSuccess.IsZero() })
			before, err := GetStatus(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root.Path, "main.go"), []byte("package main\nconst changed = 1\n"), 0600); err != nil {
				t.Fatal(err)
			}
			select {
			case <-ready:
			case <-time.After(3 * time.Second):
				t.Fatal("changed source did not trigger update")
			}
			active, err := GetStatus(root)
			if err != nil || !active.Running || active.UpdateStarted.IsZero() || !active.LastCheck.After(before.LastCheck) || !active.LastSuccess.Equal(before.LastSuccess) {
				t.Fatalf("active update status is inaccurate: %+v %v", active, err)
			}
			finish()
			waitFor(t, func() bool {
				status, _ := GetStatus(root)
				return status.UpdateStarted.IsZero() && ((fails && strings.Contains(status.LastError, "injected update failure")) || (!fails && status.LastSuccess.After(before.LastSuccess)))
			})
			after, err := GetStatus(root)
			if err != nil || (fails && !after.LastSuccess.Equal(before.LastSuccess)) || (!fails && after.LastError != "") {
				t.Fatalf("completed update status is inaccurate: %+v %v", after, err)
			}
			cancel()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}
