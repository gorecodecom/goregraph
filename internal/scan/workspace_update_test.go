package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestWorkspaceUpdatePlanSkipsUnchangedAndSelectsModifiedProjects(t *testing.T) {
	workspace := t.TempDir()
	orders := filepath.Join(workspace, "services", "orders")
	users := filepath.Join(workspace, "services", "users")
	writeFile(t, orders, "go.mod", "module example.test/orders\n")
	writeFile(t, orders, "main.go", "package main\nconst version = 1\n")
	writeFile(t, users, "go.mod", "module example.test/users\n")
	writeFile(t, users, "main.go", "package main\nconst version = 1\n")

	projectConfig := config.Defaults()
	projectConfig.Workspace = false
	projectConfig.UpdateGitignore = false
	for _, project := range []string{orders, users} {
		if _, err := RunBuild(project, projectConfig, BuildTargetAll); err != nil {
			t.Fatalf("initial build for %s: %v", project, err)
		}
	}
	writeFile(t, orders, "main.go", "package main\nconst version = 2\n")

	workspaceConfig := config.Defaults()
	workspaceConfig.Workspace = true
	workspaceConfig.WorkspaceRoot = workspace
	plan, err := WorkspaceUpdatePlan(workspace, workspaceConfig, BuildTargetAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 2 {
		t.Fatalf("items = %#v, want two discovered projects", plan.Items)
	}
	items := workspaceUpdateItemsByProject(plan.Items)
	if got := items["services/orders"]; got.Action != WorkspaceUpdateActionBuild || got.Reason != "source files changed" || got.Modified != 1 || got.Added != 0 || got.Deleted != 0 {
		t.Fatalf("orders item = %#v, want one modified source file", got)
	}
	if got := items["services/users"]; got.Action != WorkspaceUpdateActionSkip || got.Reason != "source files unchanged" || got.Modified != 0 || got.Added != 0 || got.Deleted != 0 {
		t.Fatalf("users item = %#v, want unchanged project", got)
	}
}

func TestWorkspaceUpdatePlanDetectsAddedAndDeletedFiles(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "orders")
	writeFile(t, project, "go.mod", "module example.test/orders\n")
	writeFile(t, project, "keep.go", "package orders\n")
	writeFile(t, project, "remove.go", "package orders\n")

	projectConfig := config.Defaults()
	projectConfig.Workspace = false
	projectConfig.UpdateGitignore = false
	if _, err := RunBuild(project, projectConfig, BuildTargetAll); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	if err := os.Remove(filepath.Join(project, "remove.go")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, project, "added.go", "package orders\n")

	workspaceConfig := config.Defaults()
	workspaceConfig.WorkspaceRoot = workspace
	plan, err := WorkspaceUpdatePlan(workspace, workspaceConfig, BuildTargetAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("items = %#v, want one project", plan.Items)
	}
	item := plan.Items[0]
	if item.Action != WorkspaceUpdateActionBuild || item.Reason != "source files changed" || item.Added != 1 || item.Modified != 0 || item.Deleted != 1 {
		t.Fatalf("item = %#v, want one added and one deleted file", item)
	}
}

func TestWorkspaceUpdatePlanBuildsProjectsWithMissingOrIncompatibleOutputs(t *testing.T) {
	tests := []struct {
		name        string
		build       bool
		buildTarget BuildTarget
		planTarget  BuildTarget
		mutate      func(*testing.T, string)
		wantReason  string
	}{
		{
			name:       "missing index",
			planTarget: BuildTargetAll,
			wantReason: "project index is missing or invalid",
		},
		{
			name:        "old schema",
			build:       true,
			buildTarget: BuildTargetAll,
			planTarget:  BuildTargetAll,
			mutate: func(t *testing.T, project string) {
				layout := NewProjectOutputLayout(filepath.Join(project, "goregraph-out"))
				manifest := readCurrentOutputManifest(layout.Manifest)
				manifest.Schema = SchemaVersion - 1
				if err := writeOutputManifestAtomic(layout.Manifest, manifest); err != nil {
					t.Fatal(err)
				}
			},
			wantReason: "output schema changed",
		},
		{
			name:        "missing agent projection",
			build:       true,
			buildTarget: BuildTargetDashboard,
			planTarget:  BuildTargetAgent,
			wantReason:  "agent projection is missing or incomplete",
		},
		{
			name:        "missing dashboard projection",
			build:       true,
			buildTarget: BuildTargetAgent,
			planTarget:  BuildTargetDashboard,
			wantReason:  "dashboard projection is missing or incomplete",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			project := filepath.Join(workspace, "services", "orders")
			writeFile(t, project, "go.mod", "module example.test/orders\n")
			writeFile(t, project, "main.go", "package main\n")
			projectConfig := config.Defaults()
			projectConfig.Workspace = false
			projectConfig.UpdateGitignore = false
			if test.build {
				if _, err := RunBuild(project, projectConfig, test.buildTarget); err != nil {
					t.Fatalf("initial build: %v", err)
				}
			}
			if test.mutate != nil {
				test.mutate(t, project)
			}

			workspaceConfig := config.Defaults()
			workspaceConfig.WorkspaceRoot = workspace
			plan, err := WorkspaceUpdatePlan(workspace, workspaceConfig, test.planTarget)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Items) != 1 {
				t.Fatalf("items = %#v, want one project", plan.Items)
			}
			item := plan.Items[0]
			if item.Action != WorkspaceUpdateActionBuild || item.Reason != test.wantReason {
				t.Fatalf("item = %#v, want build because %q", item, test.wantReason)
			}
		})
	}
}

func TestWorkspaceUpdatePlanRejectsSemanticallyInvalidFileIndex(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "orders")
	writeFile(t, project, "go.mod", "module example.test/orders\n")
	writeFile(t, project, "main.go", "package main\n")
	projectConfig := config.Defaults()
	projectConfig.Workspace = false
	projectConfig.UpdateGitignore = false
	if _, err := RunBuild(project, projectConfig, BuildTargetAll); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	layout := NewProjectOutputLayout(filepath.Join(project, "goregraph-out"))
	if err := os.WriteFile(layout.Index("files.json"), []byte(`[{"path":"main.go","hash":""}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	workspaceConfig := config.Defaults()
	workspaceConfig.WorkspaceRoot = workspace
	plan, err := WorkspaceUpdatePlan(workspace, workspaceConfig, BuildTargetAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("items = %#v, want one project", plan.Items)
	}
	item := plan.Items[0]
	if item.Action != WorkspaceUpdateActionBuild || item.Reason != "project file index is missing or invalid" {
		t.Fatalf("item = %#v, want invalid file index rebuild", item)
	}
}

func TestWorkspaceUpdatePlanBuildsWhenIndexArtifactIsMissing(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "orders")
	writeFile(t, project, "go.mod", "module example.test/orders\n")
	writeFile(t, project, "main.go", "package main\n")
	projectConfig := config.Defaults()
	projectConfig.Workspace = false
	projectConfig.UpdateGitignore = false
	if _, err := RunBuild(project, projectConfig, BuildTargetAll); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	layout := NewProjectOutputLayout(filepath.Join(project, "goregraph-out"))
	if err := os.Remove(layout.Index("routes.json")); err != nil {
		t.Fatal(err)
	}

	workspaceConfig := config.Defaults()
	workspaceConfig.WorkspaceRoot = workspace
	plan, err := WorkspaceUpdatePlan(workspace, workspaceConfig, BuildTargetAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("items = %#v, want one project", plan.Items)
	}
	item := plan.Items[0]
	if item.Action != WorkspaceUpdateActionBuild || item.Reason != "project index is missing or invalid" {
		t.Fatalf("item = %#v, want missing index artifact rebuild", item)
	}
}

func TestWorkspaceUpdatePlanBuildsWhenIndexManifestIsIncomplete(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "orders")
	writeFile(t, project, "go.mod", "module example.test/orders\n")
	writeFile(t, project, "main.go", "package main\n")
	projectConfig := config.Defaults()
	projectConfig.Workspace = false
	projectConfig.UpdateGitignore = false
	if _, err := RunBuild(project, projectConfig, BuildTargetAll); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	layout := NewProjectOutputLayout(filepath.Join(project, "goregraph-out"))
	manifest := readCurrentOutputManifest(layout.Manifest)
	manifest.Index.Files = []string{"index/files.json"}
	if err := writeOutputManifestAtomic(layout.Manifest, manifest); err != nil {
		t.Fatal(err)
	}

	workspaceConfig := config.Defaults()
	workspaceConfig.WorkspaceRoot = workspace
	plan, err := WorkspaceUpdatePlan(workspace, workspaceConfig, BuildTargetAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("items = %#v, want one project", plan.Items)
	}
	item := plan.Items[0]
	if item.Action != WorkspaceUpdateActionBuild || item.Reason != "project index is missing or invalid" {
		t.Fatalf("item = %#v, want incomplete index manifest rebuild", item)
	}
}

func TestWorkspaceUpdatePlanBuildsWhenDashboardManifestIsIncomplete(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "orders")
	writeFile(t, project, "go.mod", "module example.test/orders\n")
	writeFile(t, project, "main.go", "package main\n")
	projectConfig := config.Defaults()
	projectConfig.Workspace = false
	projectConfig.UpdateGitignore = false
	if _, err := RunBuild(project, projectConfig, BuildTargetAll); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	layout := NewProjectOutputLayout(filepath.Join(project, "goregraph-out"))
	manifest := readCurrentOutputManifest(layout.Manifest)
	manifest.Dashboard.Files = []string{"dashboard/report.md"}
	if err := writeOutputManifestAtomic(layout.Manifest, manifest); err != nil {
		t.Fatal(err)
	}

	workspaceConfig := config.Defaults()
	workspaceConfig.WorkspaceRoot = workspace
	plan, err := WorkspaceUpdatePlan(workspace, workspaceConfig, BuildTargetDashboard)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("items = %#v, want one project", plan.Items)
	}
	item := plan.Items[0]
	if item.Action != WorkspaceUpdateActionBuild || item.Reason != "dashboard projection is missing or incomplete" {
		t.Fatalf("item = %#v, want incomplete dashboard manifest rebuild", item)
	}
}

func workspaceUpdateItemsByProject(items []WorkspaceUpdateItemRecord) map[string]WorkspaceUpdateItemRecord {
	result := make(map[string]WorkspaceUpdateItemRecord, len(items))
	for _, item := range items {
		result[item.Project] = item
	}
	return result
}
