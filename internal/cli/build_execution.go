package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
)

type buildExecution struct {
	ctx     context.Context
	options scan.BuildOptions
}

func buildCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "scan" || args[0] == "build" || args[0] == "update" {
		return true
	}
	if args[0] != "workspace" || len(args) < 2 {
		return false
	}
	switch args[1] {
	case "build", "update", "scan-all", "scan-missing", "refresh":
		return true
	}
	return false
}

func prepareBuildExecution(args []string, stderr io.Writer) ([]string, buildExecution, func(), error) {
	execution := buildExecution{ctx: context.Background(), options: scan.DefaultBuildOptions()}
	if !buildCommand(args) {
		return args, execution, func() {}, nil
	}
	mode := "auto"
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		if name != "--progress" && name != "--file-timeout" && name != "--project-timeout" {
			filtered = append(filtered, args[i])
			continue
		}
		if !hasValue {
			i++
			if i >= len(args) {
				return nil, execution, func() {}, fmt.Errorf("%s requires a value", name)
			}
			value = args[i]
		}
		if name == "--progress" {
			switch value {
			case "auto", "plain", "json", "off":
				mode = value
			default:
				return nil, execution, func() {}, fmt.Errorf("invalid progress mode %q", value)
			}
			continue
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration < 0 {
			return nil, execution, func() {}, fmt.Errorf("%s requires a nonnegative duration", name)
		}
		if name == "--file-timeout" {
			execution.options.FileTimeout = duration
		} else {
			execution.options.ProjectTimeout = duration
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	execution.ctx = ctx
	if mode == "off" {
		return filtered, execution, cancel, nil
	}
	reporter := &buildReporter{writer: stderr, mode: mode}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				reporter.heartbeat()
			}
		}
	}()
	execution.options.Observer = reporter.observe
	return filtered, execution, func() { close(done); <-stopped; cancel() }, nil
}

type buildReporter struct {
	now         func() time.Time
	mu          sync.Mutex
	writer      io.Writer
	mode        string
	latest      scan.BuildEvent
	lastPrinted time.Time
	lastEvent   time.Time
}

func (reporter *buildReporter) observe(event scan.BuildEvent) {
	reporter.mu.Lock()
	defer reporter.mu.Unlock()
	reporter.latest = event
	reporter.lastEvent = reporter.timeNow()
	if reporter.mode == "json" {
		_ = json.NewEncoder(reporter.writer).Encode(event)
		return
	}
	if reporter.timeNow().Sub(reporter.lastPrinted) >= time.Second || event.Phase == "publish" && event.Outcome == "completed" {
		reporter.print(event)
	}
}

func (reporter *buildReporter) print(event scan.BuildEvent) {
	fmt.Fprintf(reporter.writer, "%s: %s %s %s (%d completed, %s)\n", event.Project, event.Phase, event.File, event.Outcome, event.Completed, event.Elapsed.Round(time.Millisecond))
	reporter.lastPrinted = reporter.timeNow()
}

func (reporter *buildReporter) heartbeat() {
	reporter.mu.Lock()
	defer reporter.mu.Unlock()
	if reporter.latest.Phase == "" || reporter.latest.Phase == "publish" && reporter.latest.Outcome == "completed" {
		return
	}
	event := reporter.latest
	event.Elapsed += reporter.timeNow().Sub(reporter.lastEvent)
	if reporter.mode == "json" {
		event.Outcome = "running"
		_ = json.NewEncoder(reporter.writer).Encode(event)
	} else {
		reporter.print(event)
	}
}

func (reporter *buildReporter) timeNow() time.Time {
	if reporter.now != nil {
		return reporter.now()
	}
	return time.Now()
}
