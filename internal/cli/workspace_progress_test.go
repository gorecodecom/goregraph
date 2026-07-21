package cli

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestRunWorkspaceProjectWithProgressReportsHeartbeatAndCompletion(t *testing.T) {
	start := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	ticker := &fakeProgressTicker{ticks: make(chan time.Time)}
	clock := workspaceProgressClock{
		now:       sequenceClock(t, start, start.Add(12*time.Second)),
		newTicker: func(time.Duration) progressTicker { return ticker },
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)

	go func() {
		_, err := runWorkspaceProjectWithProgress(&stdout, &stderr, 4, 43, "frontend/frontend-monorepo", clock, func() (scan.Result, error) {
			close(started)
			<-release
			return scan.Result{ScannedFiles: 1517, SkippedFiles: 115}, nil
		})
		done <- err
	}()

	<-started
	ticker.ticks <- start.Add(10 * time.Second)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Scanning [4/43]", "Still scanning [4/43]", "Completed [4/43]", "1517 files", "115 skipped"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("progress missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunWorkspaceProjectWithProgressReportsFailure(t *testing.T) {
	start := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	clock := workspaceProgressClock{
		now:       sequenceClock(t, start, start.Add(time.Second)),
		newTicker: func(time.Duration) progressTicker { return &fakeProgressTicker{ticks: make(chan time.Time)} },
	}
	var stdout, stderr bytes.Buffer

	_, err := runWorkspaceProjectWithProgress(&stdout, &stderr, 4, 43, "frontend/frontend-monorepo", clock, func() (scan.Result, error) {
		return scan.Result{}, errors.New("scan failed")
	})

	if err == nil || err.Error() != "scan failed" {
		t.Fatalf("error = %v, want scan failed", err)
	}
	if !strings.Contains(stdout.String(), "Scanning [4/43] frontend/frontend-monorepo") {
		t.Fatalf("start missing:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Completed") {
		t.Fatalf("unexpected completion:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Failed [4/43] frontend/frontend-monorepo during scan") {
		t.Fatalf("failure missing:\n%s", stderr.String())
	}
}

type fakeProgressTicker struct {
	ticks chan time.Time
}

func (ticker *fakeProgressTicker) C() <-chan time.Time {
	return ticker.ticks
}

func (ticker *fakeProgressTicker) Stop() {}

func sequenceClock(t *testing.T, values ...time.Time) func() time.Time {
	t.Helper()
	var mu sync.Mutex
	next := 0
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		if next >= len(values) {
			t.Fatalf("clock called %d times, only %d values configured", next+1, len(values))
		}
		value := values[next]
		next++
		return value
	}
}
