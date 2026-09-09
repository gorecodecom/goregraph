package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/outputstore"
)

func TestWithOutputReadConfigLocksExplicitWorkspaceOutputs(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "orders")
	writeFile(t, project, "go.mod", "module example.test/orders\n")
	writeFile(t, project, "goregraph.yml", "output: private-out\n")

	output := filepath.Join(project, "private-out")
	writerStarted := make(chan struct{})
	releaseWriter := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseWriter) }) }
	t.Cleanup(release)
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- outputstore.Update(context.Background(), outputstore.UpdateRequest{
			Root: output,
			Write: func(stage string) error {
				close(writerStarted)
				<-releaseWriter
				return os.WriteFile(filepath.Join(stage, "ready"), []byte("ok\n"), 0o644)
			},
		})
	}()
	select {
	case <-writerStarted:
	case err := <-writerDone:
		t.Fatalf("start writer: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not acquire the explicit workspace output lock")
	}

	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	called := false
	err := WithOutputReadConfig(ctx, t.TempDir(), cfg, func() error {
		called = true
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WithOutputReadConfig error = %v, want deadline while explicit workspace output is being written", err)
	}
	if called {
		t.Fatal("read callback ran without acquiring the explicit workspace output lock")
	}
	release()
	if err := <-writerDone; err != nil {
		t.Fatalf("release writer: %v", err)
	}
}

func TestWorkspaceProjectionCurrentRejectsSourceEditAfterPlan(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace

	plan, err := WorkspaceUpdatePlanWithOptions(context.Background(), workspace, cfg, BuildTargetAll, DefaultBuildOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range plan.Items {
		if item.Action != WorkspaceUpdateActionSkip {
			t.Fatalf("initial plan item = %#v, want skip", item)
		}
	}
	writeFile(t, projects[0], "src/api.ts", "export async function load() { return fetch('/changed'); }\n")

	current, err := WorkspaceProjectionCurrentContext(context.Background(), workspace, cfg, BuildTargetAll, DefaultBuildOptions())
	if err == nil || !strings.Contains(err.Error(), "source inputs changed after planning; rerun workspace update") {
		t.Fatalf("WorkspaceProjectionCurrentContext = (%v, %v), want source-change retry error", current, err)
	}
	if current {
		t.Fatal("workspace reported current after a planned project source changed")
	}
}
