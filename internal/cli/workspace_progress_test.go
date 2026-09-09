package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestBuildReporterHeartbeatKeepsCurrentFile(t *testing.T) {
	var output bytes.Buffer
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	reporter := buildReporter{writer: &output, mode: "json", now: func() time.Time { return now }}
	reporter.observe(scan.BuildEvent{Phase: "extract", File: "src/large.ts", Outcome: "started"})
	output.Reset()
	now = now.Add(5 * time.Second)
	reporter.heartbeat()
	var event scan.BuildEvent
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.File != "src/large.ts" || event.Outcome != "running" || event.Elapsed != 5*time.Second {
		t.Fatalf("heartbeat=%+v", event)
	}
	reporter.observe(scan.BuildEvent{Phase: "publish", Outcome: "completed"})
	output.Reset()
	reporter.heartbeat()
	if output.Len() != 0 {
		t.Fatal("heartbeat continued after publication")
	}
}

func TestBuildReporterThrottlesHumanProgressButPrintsCompletion(t *testing.T) {
	var output bytes.Buffer
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	reporter := buildReporter{writer: &output, mode: "plain", now: func() time.Time { return now }}
	reporter.observe(scan.BuildEvent{Phase: "extract", File: "a.ts", Outcome: "started"})
	now = now.Add(100 * time.Millisecond)
	reporter.observe(scan.BuildEvent{Phase: "extract", File: "b.ts", Outcome: "started"})
	reporter.observe(scan.BuildEvent{Phase: "publish", Outcome: "completed"})
	if strings.Count(output.String(), "\n") != 2 || strings.Contains(output.String(), "b.ts") {
		t.Fatalf("progress=%s", output.String())
	}
}
