package scan

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestFailedWorkspacePublicationPreservesEveryCommittedManifest(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
	paths := []string{filepath.Join(workspace, ".goregraph-workspace", "manifest.json")}
	for _, project := range projects {
		paths = append(paths, filepath.Join(project, config.Defaults().OutputDir, "manifest.json"))
	}
	before := make(map[string][]byte)
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = body
	}
	issue := errors.New("injected workspace write failure")
	restore := replaceProjectionWriteHookForTest(func(scope, projection string) error {
		if scope == "workspace" && projection == "agent" {
			return issue
		}
		return nil
	})
	defer restore()
	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace
	if _, err := ReconcileWorkspaceWithOptions(context.Background(), projects[0], cfg, BuildTargetAgent, DefaultBuildOptions()); !errors.Is(err, issue) {
		t.Fatalf("expected stage failure: %v", err)
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(body, before[path]) {
			t.Fatalf("committed manifest changed after failure: %s (%v)", path, err)
		}
	}
}

func TestWorkspacePublicationRetainsProjectBuildIdentity(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	cfg := config.Defaults()
	cfg.Workspace = false
	before := make(map[string]BuildIdentity)
	for _, project := range projects {
		if _, err := RunBuild(project, cfg, BuildTargetAll); err != nil {
			t.Fatal(err)
		}
		manifest, err := readProjectOutputManifest(filepath.Join(project, cfg.OutputDir, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		before[project] = manifest.BuildIdentity
	}
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace
	registry, err := ReconcileWorkspaceWithOptions(context.Background(), projects[0], cfg, BuildTargetAll, DefaultBuildOptions())
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := readProjectOutputManifest(filepath.Join(workspace, ".goregraph-workspace", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := workspaceInputIdentity(workspace, registry.Projects, cfg, DefaultBuildOptions())
	if err != nil {
		t.Fatal(err)
	}
	if manifest.BuildIdentity != identity || manifest.GenerationID == "" {
		t.Fatalf("workspace identity missing or stale: %#v", manifest)
	}
	for _, project := range projects {
		projectManifest, err := readProjectOutputManifest(filepath.Join(project, cfg.OutputDir, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		if projectManifest.BuildIdentity != before[project] {
			t.Fatalf("reconciliation changed project extractor identity: %s", project)
		}
		if projectManifest.GenerationID != manifest.GenerationID {
			t.Fatalf("sibling did not join publication generation: %s", project)
		}
	}
	var serviceMap WorkspaceServiceMapRecord
	readJSON(t, filepath.Join(workspace, ".goregraph-workspace", "index", "workspace-service-map.json"), &serviceMap)
	if serviceMap.Health.GenerationID != manifest.GenerationID || serviceMap.Health.Freshness != "unknown" {
		t.Fatalf("workspace health overclaims live verification: %#v", serviceMap.Health)
	}
}

func TestWorkspaceRejectsChangedInputGenerationBeforePublication(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
	workspaceManifest := filepath.Join(workspace, ".goregraph-workspace", "manifest.json")
	before, err := os.ReadFile(workspaceManifest)
	if err != nil {
		t.Fatal(err)
	}
	changed := false
	restore := replaceProjectionWriteHookForTest(func(scope, projection string) error {
		if scope != "workspace" || projection != "index" || changed {
			return nil
		}
		changed = true
		path := filepath.Join(projects[0], config.Defaults().OutputDir, "manifest.json")
		manifest, err := readProjectOutputManifest(path)
		if err != nil {
			return err
		}
		manifest.GenerationID = "external-generation-change"
		return writeOutputManifestAtomic(path, manifest)
	})
	defer restore()
	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace
	if _, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetAgent); err == nil {
		t.Fatal("changed project generation was published as current workspace input")
	}
	after, err := os.ReadFile(workspaceManifest)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("workspace changed after input conflict: %v", err)
	}
}

func TestDashboardAssetPruningRetainsExactlyPreviousAndCurrentAssets(t *testing.T) {
	layout := NewWorkspaceOutputLayout(t.TempDir())
	root := layout.Dashboard(workspaceDashboardAssetDir)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"previous.js", "current.js", "older.js"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	prefix := "dashboard/" + workspaceDashboardAssetDir + "/"
	if err := pruneWorkspaceDashboardAssets(layout, []string{prefix + "previous.js"}, []string{prefix + "current.js"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"previous.js", "current.js"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "older.js")); !os.IsNotExist(err) {
		t.Fatalf("older unused asset retained: %v", err)
	}
}

func TestWorkspaceAnalysisCoverageKeepsMissingAndLegacyInputsVisible(t *testing.T) {
	projects := []WorkspaceProjectRecord{{Path: "api", AbsPath: "api", OutputDir: ".goregraph", Indexed: true}, {Path: "web", AbsPath: "web", OutputDir: ".goregraph", Indexed: true}}
	manifests := map[string]OutputManifest{filepath.Join("api", ".goregraph"): {AnalysisCoverage: "complete"}, filepath.Join("web", ".goregraph"): {}}
	if coverage, _ := workspaceAnalysisSummary(projects, manifests); coverage != "unknown" {
		t.Fatalf("legacy input coverage = %q", coverage)
	}
	projects[1].Indexed = false
	coverage, issues := workspaceAnalysisSummary(projects, manifests)
	if coverage != "partial" || len(issues) != 1 || issues[0].File != "web" {
		t.Fatalf("missing project coverage = %q %#v", coverage, issues)
	}
}
