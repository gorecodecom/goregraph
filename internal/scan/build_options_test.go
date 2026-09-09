package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestCancelledBuildDoesNotPublish(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunBuildWithOptions(ctx, root, config.Defaults(), BuildTargetAll, BuildOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "goregraph-out")); !os.IsNotExist(err) {
		t.Fatalf("created output: %v", err)
	}
}

func TestBuildEventsIdentifyFilesAndPhases(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.ts", "export const count = 1\n")
	cfg := config.Defaults()
	cfg.Workspace = false
	var events []BuildEvent
	_, err := RunBuildWithOptions(context.Background(), root, cfg, BuildTargetAgent, BuildOptions{Observer: func(event BuildEvent) { events = append(events, event) }})
	if err != nil {
		t.Fatal(err)
	}
	fileSeen, publishSeen := false, false
	for _, event := range events {
		if event.Phase == "extract" && event.File == "main.ts" {
			fileSeen = true
		}
		if event.Phase == "publish" && event.Outcome == "completed" {
			publishSeen = true
		}
	}
	if !fileSeen || !publishSeen {
		t.Fatalf("events=%#v", events)
	}
}
