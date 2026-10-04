package watch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestProgressReportsWorkWithoutConfusingHeartbeatsAndClearsFinishedAttempts(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failure], func(t *testing.T) {
			root := restartFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready, next, reported, release := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
			finish := sync.OnceFunc(func() { close(release) })
			defer finish()
			var lateReport func(scan.BuildEvent)
			done := make(chan error, 1)
			go func() {
				done <- RunWithProgress(ctx, root, func(report func(scan.BuildEvent)) error {
					lateReport = report
					report(scan.BuildEvent{Phase: "workspace-project", Project: "services/orders", Outcome: "started", Completed: 1, Total: 3})
					close(ready)
					<-next
					report(scan.BuildEvent{Phase: "analyze", Project: "services/orders", File: "OrderService.go", Completed: 4, Total: 10})
					close(reported)
					<-release
					if failure {
						return errors.New("injected failure")
					}
					return nil
				})
			}()
			select {
			case <-ready:
			case <-time.After(5 * time.Second):
				t.Fatal("watcher did not report progress")
			}
			waitFor(t, func() bool { status, _ := GetStatus(root); return status.Progress != nil })
			first, err := GetStatus(root)
			if err != nil {
				t.Fatal(err)
			}
			runtimePath, err := statePath(root, "runtime.json")
			if err != nil {
				t.Fatal(err)
			}
			var before runtimeState
			if err := readJSON(runtimePath, &before); err != nil {
				t.Fatal(err)
			}
			waitFor(t, func() bool {
				var live runtimeState
				return readJSON(runtimePath, &live) == nil && live.Heartbeat.After(before.Heartbeat)
			})
			idle, err := GetStatus(root)
			if err != nil || !idle.Progress.LastProgress.Equal(first.Progress.LastProgress) {
				t.Fatalf("heartbeat falsely advanced work progress: %+v %v", idle, err)
			}
			close(next)
			<-reported
			waitFor(t, func() bool {
				status, _ := GetStatus(root)
				return status.Progress != nil && status.Progress.File == "OrderService.go"
			})
			active, err := GetStatus(root)
			if err != nil || active.Progress.ProjectsCompleted != 1 || active.Progress.ProjectsTotal != 3 || active.Progress.Completed != 4 || !active.Progress.LastProgress.After(first.Progress.LastProgress) {
				t.Fatalf("progress lost project totals or did not advance: %+v %v", active, err)
			}
			finish()
			waitFor(t, func() bool {
				status, _ := GetStatus(root)
				return status.UpdateStarted.IsZero() && (failure && status.LastError != "" || !failure && !status.LastSuccess.IsZero())
			})
			lateReport(scan.BuildEvent{Phase: "obsolete", Project: "other-project"})
			finished, err := GetStatus(root)
			if err != nil || finished.Progress != nil {
				t.Fatalf("completed or late attempt remained active: %+v %v", finished, err)
			}
			cancel()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}
