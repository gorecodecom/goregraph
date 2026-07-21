package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const workspaceProgressInterval = 10 * time.Second

type progressTicker interface {
	C() <-chan time.Time
	Stop()
}

type realProgressTicker struct {
	ticker *time.Ticker
}

func (ticker realProgressTicker) C() <-chan time.Time {
	return ticker.ticker.C
}

func (ticker realProgressTicker) Stop() {
	ticker.ticker.Stop()
}

type workspaceProgressClock struct {
	now       func() time.Time
	newTicker func(time.Duration) progressTicker
}

func defaultWorkspaceProgressClock() workspaceProgressClock {
	return workspaceProgressClock{
		now: time.Now,
		newTicker: func(interval time.Duration) progressTicker {
			return realProgressTicker{ticker: time.NewTicker(interval)}
		},
	}
}

type workspaceScanOutcome struct {
	result scan.Result
	err    error
}

func runWorkspaceProjectWithProgress(
	stdout, stderr io.Writer,
	position, total int,
	project string,
	clock workspaceProgressClock,
	run func() (scan.Result, error),
) (scan.Result, error) {
	started := clock.now()
	fmt.Fprintf(stdout, "Scanning [%d/%d] %s ...\n", position, total, project)
	ticker := clock.newTicker(workspaceProgressInterval)
	defer ticker.Stop()

	outcome := make(chan workspaceScanOutcome, 1)
	go func() {
		result, err := run()
		outcome <- workspaceScanOutcome{result: result, err: err}
	}()

	for {
		select {
		case tick := <-ticker.C():
			fmt.Fprintf(stdout, "Still scanning [%d/%d] %s (%s elapsed)\n", position, total, project, tick.Sub(started).Round(time.Second))
		case completed := <-outcome:
			elapsed := clock.now().Sub(started).Round(100 * time.Millisecond)
			if completed.err != nil {
				fmt.Fprintf(stderr, "Failed [%d/%d] %s during scan after %s: %v\n", position, total, project, elapsed, completed.err)
				return completed.result, completed.err
			}
			fmt.Fprintf(stdout, "Completed [%d/%d] %s in %s (%d files, %d skipped)\n", position, total, project, elapsed, completed.result.ScannedFiles, completed.result.SkippedFiles)
			return completed.result, nil
		}
	}
}
