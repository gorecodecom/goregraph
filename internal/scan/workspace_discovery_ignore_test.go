package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceDiscoverySkipsIgnoredUnityAndBlenderWorkspaces(t *testing.T) {
	workspace := t.TempDir()
	game := filepath.Join(workspace, "game")
	writeFile(t, game, ".gitignore", "/work/\n")
	for _, project := range []string{"Unity", "work/checkpoint/Unity"} {
		writeFile(t, game, project+"/ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 6000.0.0f1\n")
		writeFile(t, game, project+"/Packages/manifest.json", "{}\n")
	}
	writeFile(t, game, "Art/Blender/character.blend", "BLENDER")
	writeFile(t, game, "work/checkpoint/Art/Blender/character.blend", "BLENDER")
	writeFile(t, workspace, "other/work/api/go.mod", "module example.com/api\n")

	originalReadDir := workspaceReadDir
	workspaceReadDir = func(path string) ([]os.DirEntry, error) {
		if samePath(path, filepath.Join(game, "work")) {
			t.Fatalf("discovery entered ignored workspace %s", path)
		}
		return originalReadDir(path)
	}
	t.Cleanup(func() { workspaceReadDir = originalReadDir })

	projects, err := discoverWorkspaceProjects(workspace, workspace, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	want := "game/Art/Blender\ngame/Unity\nother/work/api"
	if got := strings.Join(workspaceProjectPaths(projects), "\n"); got != want {
		t.Fatalf("project paths = %q, want %q", got, want)
	}
}

func TestWorkspaceDiscoveryAppliesScopedIgnoreRulesAndNegation(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, workspace, ".gitignore", "/archives/\n")
	writeFile(t, workspace, "archives/old/package.json", "{}\n")
	writeFile(t, workspace, "repositories/.gitignore", "/*\n!/active/\n")
	writeFile(t, workspace, "repositories/active/package.json", "{}\n")
	writeFile(t, workspace, "repositories/old/package.json", "{}\n")
	writeFile(t, workspace, "other/old/package.json", "{}\n")

	projects, err := discoverWorkspaceProjects(workspace, workspace, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	want := "other/old\nrepositories/active"
	if got := strings.Join(workspaceProjectPaths(projects), "\n"); got != want {
		t.Fatalf("project paths = %q, want %q", got, want)
	}
}

func TestWorkspaceDiscoveryReportsUnreadableIgnoreRules(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, workspace, "services/api/go.mod", "module example.com/api\n")

	if _, err := discoverWorkspaceProjects(workspace, workspace, "goregraph-out"); err == nil {
		t.Fatal("discovery accepted unreadable ignore rules")
	}
}

func TestWorkspaceDiscoveryCanDisableGitignore(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, workspace, "goregraph.yml", "use_gitignore: false\n")
	writeFile(t, workspace, ".gitignore", "/archives/\n")
	writeFile(t, workspace, "archives/api/go.mod", "module example.com/api\n")

	projects := map[string]WorkspaceProjectRecord{}
	if err := walkWorkspaceProjectRoots(workspace, workspace, workspace, "", "goregraph-out", projects); err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects["archives/api"].Path != "archives/api" {
		t.Fatalf("projects = %#v, want archives/api", projects)
	}
}
