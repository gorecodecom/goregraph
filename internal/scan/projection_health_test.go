package scan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestBuildRejectsInputsChangedDuringAnalysis(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package example\n")
	options := DefaultBuildOptions()
	changed := false
	options.Observer = func(event BuildEvent) {
		if event.Phase == "resolve" && !changed {
			changed = true
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package changed\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cfg := config.Defaults()
	cfg.Workspace = false
	_, err := RunBuildWithOptions(context.Background(), root, cfg, BuildTargetAgent, options)
	if err == nil || !strings.Contains(err.Error(), "source inputs changed") {
		t.Fatalf("expected changed inputs error, got %v", err)
	}
}

func TestProjectionHealthSeparatesStaleAndPartial(t *testing.T) {
	cfg := config.Defaults()
	cfg.Workspace = false
	root := t.TempDir()
	writeFile(t, root, "main.ts", "export const answer = 42;\n")
	if _, err := RunBuild(root, cfg, BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "main.ts", "export const answer = 43;\n")
	options := DefaultBuildOptions()
	options.FileTimeout = time.Nanosecond
	if _, err := RunBuildWithOptions(context.Background(), root, cfg, BuildTargetAgent, options); err != nil {
		t.Fatal(err)
	}
	manifest := readCurrentOutputManifest(NewProjectOutputLayout(filepath.Join(root, cfg.OutputDir)).Manifest)
	dashboard := HealthForProjection(manifest, "dashboard", true)
	agent := HealthForProjection(manifest, "agent", false)
	if dashboard.Integrity != "valid" || dashboard.Freshness != "stale" {
		t.Fatalf("dashboard=%+v", dashboard)
	}
	if agent.Integrity != "valid" || agent.Freshness != "unknown" || agent.Coverage != "partial" {
		t.Fatalf("agent=%+v", agent)
	}
}

func TestProjectionHealthRejectsOutdatedAnalyzerWithoutLiveRead(t *testing.T) {
	identity := CurrentBuildIdentity(config.Defaults(), DefaultBuildOptions(), "ignored", "source")
	identity.ExtractorRevision = "previous"
	manifest := OutputManifest{Tool: ToolName, Schema: SchemaVersion, BuildIdentity: identity, Agent: ProjectionStatus{Complete: true, GeneratedAt: "2026-09-09T00:00:00Z", InputFingerprint: "previous"}}
	health := HealthForProjection(manifest, "agent", false)
	if health.Integrity != "valid" || health.Freshness != "stale" {
		t.Fatalf("health=%+v", health)
	}
}
