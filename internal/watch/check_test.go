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

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestBlockedInitialFileCheckHonorsControlRequestsWithoutPublishing(t *testing.T) {
	for _, action := range []string{"stop", "cancel", "lease-loss"} {
		t.Run(action, func(t *testing.T) {
			root := restartFixture(t)
			manifest := filepath.Join(root.Path, "published-manifest.json")
			const previous = "previous valid published output"
			if err := os.WriteFile(manifest, []byte(previous), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered, release := make(chan struct{}), make(chan struct{})
			finish := sync.OnceFunc(func() { close(release) })
			defer finish()
			var updates atomic.Int32
			done := make(chan error, 1)
			go func() {
				done <- runWithProgress(ctx, root, func(func(scan.BuildEvent)) error {
					updates.Add(1)
					return os.WriteFile(manifest, []byte("unexpected replacement"), 0o600)
				}, func(context.Context, Root) (string, error) {
					close(entered)
					<-release // Simulate an OS read that ignores cancellation.
					return "snapshot", nil
				})
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("file check never started")
			}
			status, err := GetStatus(root)
			if err != nil || !status.Running || status.CheckStarted.IsZero() || !status.LastCheck.IsZero() || !status.UpdateStarted.IsZero() || !status.LastSuccess.IsZero() {
				t.Fatalf("blocked check was described as verified monitoring: %+v %v", status, err)
			}
			switch action {
			case "stop":
				if requested, err := RequestStop(root); err != nil || !requested {
					t.Fatalf("stop request failed: %v %v", requested, err)
				}
			case "cancel":
				cancel()
			case "lease-loss":
				lock, _ := statePath(root, "run.lock")
				state, _ := statePath(root, "runtime.json")
				if err := os.WriteFile(filepath.Join(lock, "owner"), []byte("new-owner"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := writeJSON(state, runtimeState{Token: "new-owner", PID: 12345, Heartbeat: time.Now()}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if action == "lease-loss" {
					if err == nil || !strings.Contains(err.Error(), "lock changed") || !ownsLock(root, "new-owner") {
						t.Fatalf("lease loss did not preserve the new owner: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("blocked read prevented watcher control request")
			}
			if action != "lease-loss" {
				status, err = GetStatus(root)
				if err != nil || status.Running || !status.CheckStarted.IsZero() {
					t.Fatalf("stopped watcher retained active check metadata: %+v %v", status, err)
				}
			}
			body, err := os.ReadFile(manifest)
			if err != nil || string(body) != previous || updates.Load() != 0 {
				t.Fatalf("control request changed published output: %q updates=%d err=%v", body, updates.Load(), err)
			}
		})
	}
}

func TestBlockedLaterFileCheckPreservesLastSuccessAndStops(t *testing.T) {
	root := restartFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	defer finish()
	var checks, updates atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- runWithProgress(ctx, root, func(func(scan.BuildEvent)) error { updates.Add(1); return nil }, func(context.Context, Root) (string, error) {
			if checks.Add(1) == 1 {
				return "snapshot", nil
			}
			close(entered)
			<-release
			return "snapshot", nil
		})
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("second file check never started")
	}
	before, err := GetStatus(root)
	if err != nil || before.LastSuccess.IsZero() || before.LastCheck.IsZero() || before.CheckStarted.IsZero() || !before.UpdateStarted.IsZero() {
		t.Fatalf("later blocked check lost successful history: %+v %v", before, err)
	}
	if requested, err := RequestStop(root); err != nil || !requested {
		t.Fatalf("stop request failed: %v %v", requested, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("later blocked check prevented stop")
	}
	after, err := GetStatus(root)
	if err != nil || !after.LastSuccess.Equal(before.LastSuccess) || !after.LastCheck.Equal(before.LastCheck) || updates.Load() != 1 {
		t.Fatalf("stopped check invented verification or publication: %+v updates=%d err=%v", after, updates.Load(), err)
	}
}

func TestRecoveredFileCheckClearsOnlyItsOwnError(t *testing.T) {
	for _, publicationFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "successful-publication", true: "failed-publication"}[publicationFails], func(t *testing.T) {
			root := restartFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failed, recovered := make(chan struct{}), make(chan struct{})
			release := make(chan struct{})
			finish := sync.OnceFunc(func() { close(release) })
			defer finish()
			var checks, updates atomic.Int32
			done := make(chan error, 1)
			go func() {
				done <- runWithProgress(ctx, root, func(func(scan.BuildEvent)) error {
					updates.Add(1)
					if publicationFails {
						return errors.New("publication failed")
					}
					return nil
				}, func(ctx context.Context, _ Root) (string, error) {
					switch checks.Add(1) {
					case 2:
						close(failed)
						return "", errors.New("file read failed")
					case 3:
						select {
						case <-release:
						case <-ctx.Done():
							return "", ctx.Err()
						}
					case 4:
						close(recovered)
						<-ctx.Done()
						return "", ctx.Err()
					}
					return "unchanged", nil
				})
			}()
			select {
			case <-failed:
			case <-time.After(3 * time.Second):
				t.Fatal("no failed check")
			}
			waitFor(t, func() bool {
				status, _ := GetStatus(root)
				return strings.Contains(status.LastError, "file read failed")
			})
			before, _ := GetStatus(root)
			if publicationFails && !strings.Contains(before.LastError, "publication failed") {
				t.Fatalf("check erased publication error: %+v", before)
			}
			finish()
			select {
			case <-recovered:
			case <-time.After(3 * time.Second):
				t.Fatal("check did not recover")
			}
			after, err := GetStatus(root)
			expected := ""
			if publicationFails {
				expected = "publication failed"
			}
			if err != nil || after.LastError != expected || !after.LastSuccess.Equal(before.LastSuccess) || !after.LastCheck.After(before.LastCheck) || updates.Load() != 1 {
				t.Fatalf("recovery changed publication history or kept check error: before=%+v after=%+v updates=%d err=%v", before, after, updates.Load(), err)
			}
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("watcher did not exit")
			}
		})
	}
}
