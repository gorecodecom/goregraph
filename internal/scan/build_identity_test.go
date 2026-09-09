package scan

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestUpdateRebuildsChangedExtractorIdentity(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "services", "orders")
	writeFile(t, root, "go.mod", "module example.test/orders\n")
	writeFile(t, root, "main.go", "package orders\n")
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := RunBuild(root, cfg, BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	layout := NewProjectOutputLayout(filepath.Join(root, cfg.OutputDir))
	manifest := readCurrentOutputManifest(layout.Manifest)
	manifest.BuildIdentity.ExtractorRevision = "previous"
	if err := writeOutputManifestAtomic(layout.Manifest, manifest); err != nil {
		t.Fatal(err)
	}
	cfg.WorkspaceRoot = workspace
	cfg.Workspace = true
	plan, err := WorkspaceUpdatePlanWithOptions(context.Background(), workspace, cfg, BuildTargetAgent, DefaultBuildOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 || plan.Items[0].Action != WorkspaceUpdateActionBuild || plan.Items[0].Reason != "extractor revision changed" {
		t.Fatalf("plan=%#v", plan)
	}
}

func TestIdentityIgnoresObserverAndIncludesAnalysisBudget(t *testing.T) {
	cfg := config.Defaults()
	options := DefaultBuildOptions()
	before := CurrentBuildIdentity(cfg, options, "ignore", "source")
	options.Observer = func(BuildEvent) {}
	if got := CurrentBuildIdentity(cfg, options, "ignore", "source"); got != before {
		t.Fatal("observer invalidated analysis")
	}
	options.FileTimeout = 0
	if got := CurrentBuildIdentity(cfg, options, "ignore", "source"); got.AnalysisPolicyDigest == before.AnalysisPolicyDigest {
		t.Fatal("budget omitted from identity")
	}
}
