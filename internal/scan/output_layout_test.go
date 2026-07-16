package scan

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

func TestOutputLayoutSeparatesIndexAgentAndDashboard(t *testing.T) {
	layout := NewProjectOutputLayout("/tmp/project/goregraph-out")
	assertOutputPath(t, layout.Manifest, "/tmp/project/goregraph-out/manifest.json")
	assertOutputPath(t, layout.Index("routes.json"), "/tmp/project/goregraph-out/index/routes.json")
	assertOutputPath(t, layout.Agent("context-index.json"), "/tmp/project/goregraph-out/agent/context-index.json")
	assertOutputPath(t, layout.Dashboard("report.md"), "/tmp/project/goregraph-out/dashboard/report.md")
}

func TestWorkspaceOutputLayoutSeparatesIndexAgentAndDashboard(t *testing.T) {
	layout := NewWorkspaceOutputLayout("/tmp/workspace/.goregraph-workspace")
	assertOutputPath(t, layout.Manifest, "/tmp/workspace/.goregraph-workspace/manifest.json")
	assertOutputPath(t, layout.Index("registry.json"), "/tmp/workspace/.goregraph-workspace/index/registry.json")
	assertOutputPath(t, layout.Agent("context-index.json"), "/tmp/workspace/.goregraph-workspace/agent/context-index.json")
	assertOutputPath(t, layout.Dashboard("workspace-map.html"), "/tmp/workspace/.goregraph-workspace/dashboard/workspace-map.html")
}

func TestParseBuildTargetRejectsUnknownValues(t *testing.T) {
	for _, value := range []string{"", "context", "contextai", "reports", "everything"} {
		if _, err := ParseBuildTarget(value); err == nil {
			t.Fatalf("accepted target %q", value)
		}
	}
}

func TestParseBuildTargetAcceptsPublicTargets(t *testing.T) {
	for _, value := range []string{"agent", "dashboard", "all"} {
		target, err := ParseBuildTarget(value)
		if err != nil {
			t.Fatalf("ParseBuildTarget(%q): %v", value, err)
		}
		if string(target) != value {
			t.Fatalf("ParseBuildTarget(%q) = %q", value, target)
		}
	}
}

func TestProjectBuildAgentDoesNotWriteDashboard(t *testing.T) {
	root := writeBuildFixture(t)
	cfg := config.Defaults()
	cfg.Workspace = false

	if _, err := RunBuild(root, cfg, BuildTargetAgent); err != nil {
		t.Fatal(err)
	}

	assertOutputExists(t, filepath.Join(root, "goregraph-out", "index", "routes.json"))
	assertOutputExists(t, filepath.Join(root, "goregraph-out", "agent", "agent-guide.md"))
	assertOutputNotExists(t, filepath.Join(root, "goregraph-out", "dashboard"))
}

func TestProjectBuildDashboardDoesNotWriteAgent(t *testing.T) {
	root := writeBuildFixture(t)
	cfg := config.Defaults()
	cfg.Workspace = false

	if _, err := RunBuild(root, cfg, BuildTargetDashboard); err != nil {
		t.Fatal(err)
	}

	assertOutputExists(t, filepath.Join(root, "goregraph-out", "index", "routes.json"))
	assertOutputExists(t, filepath.Join(root, "goregraph-out", "dashboard", "report.md"))
	assertOutputNotExists(t, filepath.Join(root, "goregraph-out", "agent"))
}

func TestProjectBuildAllExtractsSourceOnce(t *testing.T) {
	root := writeBuildFixture(t)
	cfg := config.Defaults()
	cfg.Workspace = false
	extractions := 0
	restore := replaceProjectExtractorForTest(func(root string, cfg config.Config, matcher gitignore.Matcher) (Index, int, error) {
		extractions++
		return scanProject(root, cfg, matcher)
	})
	defer restore()

	if _, err := RunBuild(root, cfg, BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	if extractions != 1 {
		t.Fatalf("source extractions = %d, want 1", extractions)
	}
}

func TestWorkspaceBuildAgentWritesOnlyAgentProjection(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAgent)

	layout := NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace"))
	assertOutputExists(t, layout.Index("registry.json"))
	assertOutputExists(t, layout.Agent("agent-guide.md"))
	assertOutputNotExists(t, filepath.Join(layout.Root, "dashboard"))
	assertOutputNotExists(t, layout.Index("symbol-index.json"))
	assertOutputNotExists(t, layout.Index("symbol-usages.json"))
}

func TestWorkspaceBuildDashboardWritesOnlyDashboardProjection(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetDashboard)

	layout := NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace"))
	assertOutputExists(t, layout.Index("registry.json"))
	assertOutputExists(t, layout.Index("symbol-index.json"))
	assertOutputExists(t, layout.Index("symbol-usages.json"))
	assertOutputExists(t, layout.Dashboard("workspace-map.html"))
	assertOutputNotExists(t, filepath.Join(layout.Root, "agent"))
}

func TestProjectBuildRejectsLegacyLayout(t *testing.T) {
	root := writeBuildFixture(t)
	legacyPath := filepath.Join(root, "goregraph-out", "routes.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = false

	_, err := RunBuild(root, cfg, BuildTargetAll)

	if err == nil {
		t.Fatal("legacy flat output was silently mixed with the current layout")
	}
	for _, want := range []string{"legacy", "clean", "build all"} {
		if !strings.Contains(strings.ToLower(err.Error()), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func TestWorkspaceBuildRejectsLegacyLayout(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	for _, project := range projects {
		cfg := config.Defaults()
		cfg.Workspace = false
		if _, err := RunBuild(project, cfg, BuildTargetAll); err != nil {
			t.Fatal(err)
		}
	}
	legacyPath := filepath.Join(workspace, ".goregraph-workspace", "workspace-map.html")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace

	_, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetAll)

	if err == nil {
		t.Fatal("legacy flat workspace output was silently mixed with the current layout")
	}
	for _, want := range []string{"legacy", "clean", "build all"} {
		if !strings.Contains(strings.ToLower(err.Error()), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func TestProjectBuildAgentPreservesValidDashboard(t *testing.T) {
	root := writeBuildFixture(t)
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := RunBuild(root, cfg, BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	report := NewProjectOutputLayout(filepath.Join(root, cfg.OutputDir)).Dashboard("report.md")
	wantTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(report, wantTime, wantTime); err != nil {
		t.Fatal(err)
	}

	if _, err := RunBuild(root, cfg, BuildTargetAgent); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(report)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(wantTime) {
		t.Fatalf("dashboard report modification time = %v, want %v", info.ModTime(), wantTime)
	}
	var manifest OutputManifest
	readJSON(t, filepath.Join(root, cfg.OutputDir, "manifest.json"), &manifest)
	if !manifest.Dashboard.Complete {
		t.Fatalf("valid dashboard projection was not preserved: %#v", manifest)
	}
}

func TestWorkspaceRefreshTargetsPreserveUnselectedProjection(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
	layout := NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace"))
	var before OutputManifest
	readJSON(t, layout.Manifest, &before)
	dashboard := layout.Dashboard("workspace-map.html")
	agent := layout.Agent("agent-guide.md")
	dashboardTime := time.Unix(1_700_000_100, 0)
	agentTime := time.Unix(1_700_000_200, 0)
	if err := os.Chtimes(dashboard, dashboardTime, dashboardTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(agent, agentTime, agentTime); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace

	if _, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	assertOutputModTime(t, dashboard, dashboardTime)
	var afterAgentRefresh OutputManifest
	readJSON(t, layout.Manifest, &afterAgentRefresh)
	if !afterAgentRefresh.Dashboard.Complete {
		t.Fatalf("dashboard status was not preserved: %#v", afterAgentRefresh.Dashboard)
	}
	if strings.Join(afterAgentRefresh.Dashboard.Files, "\n") != strings.Join(before.Dashboard.Files, "\n") {
		t.Fatalf("dashboard files changed:\nbefore=%v\nafter=%v", before.Dashboard.Files, afterAgentRefresh.Dashboard.Files)
	}
	for _, name := range []string{"index/symbol-index.json", "index/symbol-usages.json"} {
		if !containsGeneratedFile(afterAgentRefresh.Index.Files, name) {
			t.Fatalf("preserved dashboard index file %q missing from %v", name, afterAgentRefresh.Index.Files)
		}
	}
	if err := os.Chtimes(agent, agentTime, agentTime); err != nil {
		t.Fatal(err)
	}

	if _, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetDashboard); err != nil {
		t.Fatal(err)
	}
	assertOutputModTime(t, agent, agentTime)
}

func TestWorkspaceReconcileRepublishesAffectedProjectManifests(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
	projectLayout := NewProjectOutputLayout(filepath.Join(projects[0], config.Defaults().OutputDir))
	var before OutputManifest
	readJSON(t, projectLayout.Manifest, &before)
	oldTime := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(projectLayout.Manifest, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace

	registry, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetAgent)

	if err != nil {
		t.Fatal(err)
	}
	var after OutputManifest
	readJSON(t, projectLayout.Manifest, &after)
	if after.Index.GeneratedAt != registry.Generated {
		t.Fatalf("project index generated_at = %q, want reconciliation time %q", after.Index.GeneratedAt, registry.Generated)
	}
	if after.Dashboard.GeneratedAt != before.Dashboard.GeneratedAt {
		t.Fatalf("unselected dashboard generated_at changed from %q to %q", before.Dashboard.GeneratedAt, after.Dashboard.GeneratedAt)
	}
	manifestInfo, err := os.Stat(projectLayout.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	overlayInfo, err := os.Stat(projectLayout.Index("workspace-graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	if manifestInfo.ModTime().Before(overlayInfo.ModTime()) {
		t.Fatalf("project manifest modification time %v precedes reconciled overlay %v", manifestInfo.ModTime(), overlayInfo.ModTime())
	}
	if manifestInfo.ModTime().Equal(oldTime) {
		t.Fatalf("project manifest was not republished after reconciliation")
	}
}

func TestFailedSelectedProjectionRewritePublishesIncompleteManifest(t *testing.T) {
	t.Run("project", func(t *testing.T) {
		root := writeBuildFixture(t)
		cfg := config.Defaults()
		cfg.Workspace = false
		if _, err := RunBuild(root, cfg, BuildTargetAll); err != nil {
			t.Fatal(err)
		}
		restore := replaceProjectionWriteHookForTest(func(scope, projection string) error {
			if scope == "project" && projection == "dashboard" {
				return errors.New("injected dashboard write failure")
			}
			return nil
		})
		defer restore()

		if _, err := RunBuild(root, cfg, BuildTargetDashboard); err == nil {
			t.Fatal("project build succeeded despite injected selected projection failure")
		}

		var manifest OutputManifest
		readJSON(t, NewProjectOutputLayout(filepath.Join(root, cfg.OutputDir)).Manifest, &manifest)
		if manifest.Dashboard.Complete {
			t.Fatalf("failed dashboard rewrite left an old complete status: %#v", manifest.Dashboard)
		}
		if !manifest.Agent.Complete {
			t.Fatalf("valid unselected agent projection was not preserved: %#v", manifest.Agent)
		}
	})

	t.Run("workspace", func(t *testing.T) {
		workspace, projects := writeWorkspaceBuildFixture(t)
		buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
		restore := replaceProjectionWriteHookForTest(func(scope, projection string) error {
			if scope == "workspace" && projection == "agent" {
				return errors.New("injected workspace agent write failure")
			}
			return nil
		})
		defer restore()
		cfg := config.Defaults()
		cfg.Workspace = true
		cfg.WorkspaceRoot = workspace

		if _, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetAgent); err == nil {
			t.Fatal("workspace refresh succeeded despite injected selected projection failure")
		}

		var manifest OutputManifest
		readJSON(t, NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace")).Manifest, &manifest)
		if manifest.Agent.Complete {
			t.Fatalf("failed workspace agent rewrite left an old complete status: %#v", manifest.Agent)
		}
		if !manifest.Dashboard.Complete {
			t.Fatalf("valid unselected workspace dashboard was not preserved: %#v", manifest.Dashboard)
		}
	})
}

func containsGeneratedFile(files []string, want string) bool {
	for _, name := range files {
		if name == want {
			return true
		}
	}
	return false
}

func assertOutputModTime(t *testing.T, path string, want time.Time) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(want) {
		t.Fatalf("%s modification time = %v, want %v", path, info.ModTime(), want)
	}
}

func writeWorkspaceBuildFixture(t *testing.T) (string, []string) {
	t.Helper()
	workspace := t.TempDir()
	frontend := filepath.Join(workspace, "frontend", "web")
	backend := filepath.Join(workspace, "services", "api")
	writeFile(t, frontend, "package.json", `{"name":"web"}`)
	writeFile(t, frontend, "src/api.ts", "export async function load() { return fetch('/api'); }\n")
	writeFile(t, backend, "go.mod", "module example.test/api\n")
	writeFile(t, backend, "main.go", "package main\nfunc main() {}\n")
	return workspace, []string{frontend, backend}
}

func buildWorkspaceProjects(t *testing.T, workspace string, projects []string, target BuildTarget) {
	t.Helper()
	for _, project := range projects {
		cfg := config.Defaults()
		cfg.Workspace = false
		if _, err := RunBuild(project, cfg, target); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace
	if _, err := ReconcileWorkspaceTarget(projects[0], cfg, target); err != nil {
		t.Fatal(err)
	}
}

func writeBuildFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Build fixture\n")
	writeFile(t, root, "main.go", "package main\nfunc main() {}\n")
	return root
}

func assertOutputExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s does not exist: %v", path, err)
	}
}

func assertOutputNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists, err=%v", path, err)
	}
}

func assertOutputPath(t *testing.T, got, want string) {
	t.Helper()
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
