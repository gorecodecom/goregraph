package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/dashboardeditor"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestRunHelpPrintsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: goregraph <command>") {
		t.Fatalf("help output missing usage:\n%s", stdout.String())
	}
}

func TestRunHelpUsesProgressiveDisclosure(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"help"},
		{"--help"},
		{"-h"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", args, code, stderr.String())
		}
		text := stdout.String()
		for _, want := range []string{
			"Agent context:",
			"goregraph build agent .",
			"goregraph context . --query",
			"Dashboard:",
			"goregraph dashboard open .",
			"Diagnosis:",
			"goregraph doctor .",
			"goregraph help --all",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%v help missing %q:\n%s", args, want, text)
			}
		}
		for _, hidden := range []string{"scan <path>", "query <path>", "git update [path]"} {
			if strings.Contains(text, hidden) {
				t.Fatalf("%v standard help exposes %q:\n%s", args, hidden, text)
			}
		}
		if lines := strings.Count(strings.TrimSuffix(text, "\n"), "\n") + 1; lines > 35 {
			t.Fatalf("%v standard help has %d lines, want at most 35:\n%s", args, lines, text)
		}
	}
}

func TestRunAllHelpPreservesCompleteCommandCatalog(t *testing.T) {
	for _, args := range [][]string{
		{"help", "--all"},
		{"--help", "--all"},
		{"-h", "--all"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", args, code, stderr.String())
		}
		for _, want := range []string{
			"Core commands:",
			"build <target>",
			"context <path>",
			"dashboard",
			"doctor <path>",
			"workspace",
			"mcp",
			"Manual exploration:",
			"query <path>",
			"explain <path>",
			"report <path>",
			"Maintenance:",
			"update",
			"git update [path]",
			"Compatibility:",
			"scan <path>",
			"Utility:",
			"version",
			"help",
			"Project vs workspace builds:",
			"standard MCP exposes only task_context",
		} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("%v complete help missing %q:\n%s", args, want, stdout.String())
			}
		}
	}
}

func TestRunHelpRejectsUnknownSelector(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"help", "--unknown"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "goregraph help") {
		t.Fatalf("unexpected streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunHelpExplainsProjectAndWorkspaceBuildScopes(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"help", "--all"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"Project vs workspace builds:",
		"goregraph build dashboard .",
		"Does not scan sibling projects.",
		"goregraph build dashboard . --no-workspace",
		"Skips workspace discovery and reconciliation.",
		"goregraph workspace build dashboard .",
		"Scans every discovered workspace project.",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunBuildCommandsWriteSelectedProjection(t *testing.T) {
	for _, target := range []string{"agent", "dashboard", "all"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "main.go", "package main\nfunc main() {}\n")
			var stdout, stderr bytes.Buffer

			code := Run([]string{"build", target, root, "--no-workspace", "--no-update-gitignore"}, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
			}
			assertCLIPathExists(t, filepath.Join(root, "goregraph-out", "index", "routes.json"))
			assertCLIProjection(t, root, "agent", target == "agent" || target == "all")
			assertCLIProjection(t, root, "dashboard", target == "dashboard" || target == "all")
		})
	}
}

func TestRunBuildListsWrittenAndPreservedProjections(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\nfunc main() {}\n")
	var initialOut, initialErr bytes.Buffer
	if code := Run([]string{"build", "all", root, "--no-workspace", "--no-update-gitignore"}, &initialOut, &initialErr); code != 0 {
		t.Fatalf("initial build exit code = %d, stderr=%s", code, initialErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"build", "agent", root, "--no-workspace", "--no-update-gitignore"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"Written projections: index, agent",
		"Preserved projections: dashboard",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunBuildRejectsMissingOrUnknownTarget(t *testing.T) {
	for _, args := range [][]string{
		{"build"},
		{"build", "context"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("%v exit code = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "agent, dashboard, all") {
			t.Fatalf("%v error missing accepted targets:\n%s", args, stderr.String())
		}
	}
}

func TestRunWorkspaceBuildCommandsWriteSelectedProjection(t *testing.T) {
	for _, target := range []string{"agent", "dashboard", "all"} {
		t.Run(target, func(t *testing.T) {
			workspace := t.TempDir()
			writeFile(t, workspace, "frontend/web/package.json", `{"name":"web"}`)
			writeFile(t, workspace, "services/api/go.mod", "module example.test/api\n")
			var stdout, stderr bytes.Buffer

			code := Run([]string{"workspace", "build", target, workspace, "--workspace", workspace, "--no-update-gitignore"}, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
			}
			out := filepath.Join(workspace, ".goregraph-workspace")
			assertCLIPathExists(t, filepath.Join(out, "index", "registry.json"))
			assertCLIWorkspaceProjection(t, out, "agent", target == "agent" || target == "all")
			assertCLIWorkspaceProjection(t, out, "dashboard", target == "dashboard" || target == "all")
		})
	}
}

func TestRunUpdateAcceptsBuildTargets(t *testing.T) {
	for _, target := range []string{"agent", "dashboard", "all"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "main.go", "package main\nfunc main() {}\n")
			var stdout, stderr bytes.Buffer

			code := Run([]string{"update", root, "--target", target, "--no-workspace", "--no-update-gitignore"}, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
			}
			assertCLIPathExists(t, filepath.Join(root, "goregraph-out", "manifest.json"))
		})
	}
}

func TestRunScanRejectsTargetOption(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\nfunc main() {}\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", root, "--target", "agent", "--no-update-gitignore"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s; stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--target is not supported by scan") {
		t.Fatalf("error does not explain all-only scan alias:\n%s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "goregraph-out")); !os.IsNotExist(err) {
		t.Fatalf("scan wrote output despite rejected target, err=%v", err)
	}
}

func TestRunWorkspaceRefreshAcceptsBuildTargets(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, workspace, "frontend/web/package.json", `{"name":"web"}`)
	writeFile(t, workspace, "services/api/go.mod", "module example.test/api\n")
	var buildOut, buildErr bytes.Buffer
	if code := Run([]string{"workspace", "build", "all", workspace, "--workspace", workspace, "--no-update-gitignore"}, &buildOut, &buildErr); code != 0 {
		t.Fatalf("workspace build exit code = %d, stderr=%s", code, buildErr.String())
	}

	for _, target := range []string{"agent", "dashboard", "all"} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"workspace", "refresh", workspace, "--target", target, "--workspace", workspace}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("target %s exit code = %d, stderr=%s", target, code, stderr.String())
		}
	}
}

func TestRunWorkspaceRefreshBuildsDashboardWithoutCreatingMissingProjectDashboards(t *testing.T) {
	for _, test := range []struct {
		name        string
		refreshArgs func(string) []string
	}{
		{
			name: "dashboard",
			refreshArgs: func(workspace string) []string {
				return []string{"workspace", "refresh", workspace, "--target", "dashboard", "--workspace", workspace}
			},
		},
		{
			name: "default_all",
			refreshArgs: func(workspace string) []string {
				return []string{"workspace", "refresh", workspace, "--workspace", workspace}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			projects := []string{
				filepath.Join(workspace, "frontend", "web"),
				filepath.Join(workspace, "services", "api"),
			}
			writeFile(t, projects[0], "package.json", `{"name":"web"}`)
			writeFile(t, projects[1], "go.mod", "module example.test/api\n")
			var buildOut, buildErr bytes.Buffer
			if code := Run([]string{"workspace", "build", "agent", workspace, "--workspace", workspace, "--no-update-gitignore"}, &buildOut, &buildErr); code != 0 {
				t.Fatalf("workspace agent build exit code = %d, stderr=%s", code, buildErr.String())
			}
			for _, project := range projects {
				assertCLIPathState(t, filepath.Join(project, "goregraph-out", "dashboard"), false)
			}
			var stdout, stderr bytes.Buffer

			code := Run(test.refreshArgs(workspace), &stdout, &stderr)

			if code != 0 {
				t.Fatalf("workspace refresh exit code = %d, stdout=%s, stderr=%s", code, stdout.String(), stderr.String())
			}
			workspaceOut := filepath.Join(workspace, ".goregraph-workspace")
			assertCLIPathExists(t, filepath.Join(workspaceOut, "dashboard", "workspace-map.html"))
			workspaceManifest := readCLIOutputManifest(t, filepath.Join(workspaceOut, "manifest.json"))
			if !workspaceManifest.Dashboard.Complete {
				t.Fatalf("workspace dashboard is incomplete: %#v", workspaceManifest.Dashboard)
			}
			for _, project := range projects {
				projectOut := filepath.Join(project, "goregraph-out")
				assertCLIPathState(t, filepath.Join(projectOut, "dashboard"), false)
				projectManifest := readCLIOutputManifest(t, filepath.Join(projectOut, "manifest.json"))
				if !projectManifest.Index.Complete {
					t.Fatalf("%s index is incomplete after workspace refresh: %#v", project, projectManifest.Index)
				}
				if !projectManifest.Agent.Complete {
					t.Fatalf("%s agent projection was not preserved: %#v", project, projectManifest.Agent)
				}
				if projectManifest.Dashboard.Complete || len(projectManifest.Dashboard.Files) != 0 {
					t.Fatalf("%s acquired an empty or incomplete dashboard publication: %#v", project, projectManifest.Dashboard)
				}
			}
		})
	}
}

func TestRunWorkspaceRefreshListsWrittenAndPreservedProjections(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, workspace, "frontend/web/package.json", `{"name":"web"}`)
	writeFile(t, workspace, "services/api/go.mod", "module example.test/api\n")
	var buildOut, buildErr bytes.Buffer
	if code := Run([]string{"workspace", "build", "all", workspace, "--workspace", workspace, "--no-update-gitignore"}, &buildOut, &buildErr); code != 0 {
		t.Fatalf("workspace build exit code = %d, stderr=%s", code, buildErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "refresh", workspace, "--target", "agent", "--workspace", workspace}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("refresh exit code = %d, stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"Written projections: index, agent",
		"Preserved projections: dashboard",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, stdout.String())
		}
	}
}

func readCLIOutputManifest(t *testing.T, path string) scan.OutputManifest {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest scan.OutputManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func writeCompleteCLIWorkspaceDashboard(t *testing.T, workspace string) string {
	t.Helper()
	dashboard := filepath.Join(workspace, ".goregraph-workspace", "dashboard", "workspace-map.html")
	writeFile(t, filepath.Dir(dashboard), filepath.Base(dashboard), "<html></html>")
	manifest := scan.OutputManifest{Dashboard: scan.ProjectionStatus{
		Complete: true,
		Files:    []string{"dashboard/workspace-map.html"},
	}}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".goregraph-workspace", "manifest.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	return dashboard
}

func assertCLIProjection(t *testing.T, root, projection string, want bool) {
	t.Helper()
	path := filepath.Join(root, "goregraph-out", projection)
	assertCLIPathState(t, path, want)
}

func assertCLIWorkspaceProjection(t *testing.T, out, projection string, want bool) {
	t.Helper()
	assertCLIPathState(t, filepath.Join(out, projection), want)
}

func assertCLIPathExists(t *testing.T, path string) {
	t.Helper()
	assertCLIPathState(t, path, true)
}

func assertCLIPathState(t *testing.T, path string, want bool) {
	t.Helper()
	_, err := os.Stat(path)
	if want && err != nil {
		t.Fatalf("%s missing: %v", path, err)
	}
	if !want && !os.IsNotExist(err) {
		t.Fatalf("%s exists, err=%v", path, err)
	}
}

func TestWorkspaceHelpExplainsFlatWorkspaceDetection(t *testing.T) {
	for _, args := range [][]string{
		{"help", "--all"},
		{"workspace", "help", "--all"},
		{"workspace", "scan-all", "help"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, want 0; stderr=%s", args, code, stderr.String())
		}
		for _, want := range []string{
			".goregraph-workspace.yml",
			"--workspace",
			"project/build marker",
			".git alone",
			"goregraph.yml",
			"explicit project build",
		} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("%v help output missing %q:\n%s", args, want, stdout.String())
			}
		}
	}
}

func TestWorkspaceScanAllExplainsHowToUseFlatProjectLayout(t *testing.T) {
	root := t.TempDir()
	for _, project := range []string{"service-a", "service-b", "service-common"} {
		writeFile(t, root, filepath.Join(project, "pom.xml"), "<project></project>\n")
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "scan-all", root, "--dry-run"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%s; stderr=%s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"no GoreGraph workspace detected",
		"flat project layout",
		"--workspace",
		".goregraph-workspace.yml",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("workspace detection error missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestRunScanWritesOutputAndUpdatesGitignore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "goregraph-out", "manifest.json")); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore not written: %v", err)
	}
	if !strings.Contains(string(gitignore), "goregraph-out/") {
		t.Fatalf(".gitignore missing goregraph-out/:\n%s", string(gitignore))
	}
	if !strings.Contains(stdout.String(), "Scanned 1 files") {
		t.Fatalf("stdout missing scan summary:\n%s", stdout.String())
	}
}

func TestRunScanNoUpdateGitignoreLeavesGitignoreUntouched(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", root, "--no-update-gitignore"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf(".gitignore exists after opt-out, err=%v", err)
	}
}

func TestRunScanUpdatesWorkspaceGitignore(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	root := filepath.Join(workspace, "frontend", "frontend-monorepo")
	writeFile(t, root, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, filepath.Join(workspace, "microservices", "ms-cadaster"), "README.md", "# ms-cadaster\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	projectGitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("project .gitignore not written: %v", err)
	}
	if !strings.Contains(string(projectGitignore), "goregraph-out/") {
		t.Fatalf("project .gitignore missing goregraph-out/:\n%s", string(projectGitignore))
	}
	workspaceGitignore, err := os.ReadFile(filepath.Join(workspace, ".gitignore"))
	if err != nil {
		t.Fatalf("workspace .gitignore not written: %v", err)
	}
	if !strings.Contains(string(workspaceGitignore), ".goregraph-workspace/") {
		t.Fatalf("workspace .gitignore missing .goregraph-workspace/:\n%s", string(workspaceGitignore))
	}
}

func TestRunScanNoUpdateGitignoreSkipsWorkspaceGitignore(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	root := filepath.Join(workspace, "frontend", "frontend-monorepo")
	writeFile(t, root, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, filepath.Join(workspace, "microservices", "ms-cadaster"), "README.md", "# ms-cadaster\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", root, "--no-update-gitignore"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("project .gitignore exists after opt-out, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("workspace .gitignore exists after opt-out, err=%v", err)
	}
}

func TestRunScanNoWorkspaceSkipsWorkspaceRegistry(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	root := filepath.Join(workspace, "frontend", "frontend-monorepo")
	writeFile(t, root, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, filepath.Join(workspace, "microservices", "ms-cadaster"), "README.md", "# ms-cadaster\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", root, "--no-workspace"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "goregraph-out", "manifest.json")); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".goregraph-workspace", "registry.json")); !os.IsNotExist(err) {
		t.Fatalf("workspace registry exists after --no-workspace, err=%v", err)
	}
}

func TestRunWorkspaceStatusPrintsDetectedProjects(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	root := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, root, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, cadaster, "pom.xml", `<project><artifactId>ms-cadaster</artifactId></project>`)
	writeFile(t, cadaster, "README.md", "# ms-cadaster\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "status", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "frontend/frontend-monorepo") || !strings.Contains(stdout.String(), "microservices/ms-cadaster") {
		t.Fatalf("workspace status missing projects:\n%s", stdout.String())
	}
}

func TestRunWorkspaceScanMissingDryRunShowsPlanWithoutScanning(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	task := filepath.Join(workspace, "microservices", "ms-task")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, task, "pom.xml", `<project><artifactId>ms-task</artifactId></project>`)
	writeFile(t, frontend, "src/api/tasks.js", "export function loadTask(id) {\n"+
		"  return fetch(`/tasks/${id}`);\n"+
		"}\n")
	writeFile(t, task, "README.md", "# ms-task\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", frontend, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "scan-missing", frontend, "--top", "1"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"# GoreGraph Workspace Missing Scan Plan",
		"Dry run: true",
		"microservices/ms-task",
		"goregraph workspace scan-missing",
		"--execute",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("scan-missing dry-run output missing %q:\n%s", want, stdout.String())
		}
	}
	if _, err := os.Stat(filepath.Join(task, "goregraph-out", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not scan ms-task, err=%v", err)
	}
}

func TestRunWorkspaceScanMissingExecuteScansTopMissingService(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	task := filepath.Join(workspace, "microservices", "ms-task")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, task, "pom.xml", `<project><artifactId>ms-task</artifactId></project>`)
	writeFile(t, frontend, "src/api/tasks.js", "export function loadTask(id) {\n"+
		"  return fetch(`/tasks/${id}`);\n"+
		"}\n")
	writeFile(t, task, "src/main/java/com/example/TaskController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/tasks")
class TaskController {
  @GetMapping("/{taskId}")
  String get(@PathVariable String taskId) {
    return taskId;
  }
}
`)
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", frontend, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "scan-missing", frontend, "--top", "1", "--execute", "--no-update-gitignore"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Scanned 1 missing workspace project") || !strings.Contains(stdout.String(), "Completed [1/1] microservices/ms-task") {
		t.Fatalf("execute output missing scan summary:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(task, "goregraph-out", "manifest.json")); err != nil {
		t.Fatalf("execute should scan ms-task: %v", err)
	}
	matches, err := os.ReadFile(filepath.Join(frontend, "goregraph-out", "dashboard", "workspace-contract-matches.md"))
	if err != nil {
		t.Fatalf("reading frontend workspace matches: %v", err)
	}
	if !strings.Contains(string(matches), "ms-task GET `/tasks/{taskId}`") {
		t.Fatalf("frontend workspace matches should include scanned task backend:\n%s", string(matches))
	}
}

func TestRunWorkspaceScanAllScansDiscoveredWorkspaceProjects(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	task := filepath.Join(workspace, "microservices", "ms-task")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, cadaster, "pom.xml", `<project><artifactId>ms-cadaster</artifactId></project>`)
	writeFile(t, task, "pom.xml", `<project><artifactId>ms-task</artifactId></project>`)
	writeFile(t, frontend, ".gitignore", "dist/\n")
	writeFile(t, frontend, "src/api/cadasterservice.js", "export function loadCadaster(id) {\n"+
		"  return fetch(`/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, cadaster, "src/main/java/com/example/CadasterController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @GetMapping("/{cadasterId}")
  String get(@PathVariable String cadasterId) {
    return cadasterId;
  }
}
`)
	writeFile(t, task, "README.md", "# ms-task\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "scan-all", workspace}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	for index, project := range []string{"frontend/frontend-monorepo", "microservices/ms-cadaster", "microservices/ms-task"} {
		if !strings.Contains(stdout.String(), fmt.Sprintf("Completed [%d/3] %s", index+1, project)) {
			t.Fatalf("scan-all output missing progress for %s:\n%s", project, stdout.String())
		}
	}
	for _, out := range []string{
		filepath.Join(frontend, "goregraph-out", "manifest.json"),
		filepath.Join(cadaster, "goregraph-out", "manifest.json"),
		filepath.Join(task, "goregraph-out", "manifest.json"),
		filepath.Join(frontend, "goregraph-out", "index", "workspace-contract-matches.json"),
	} {
		if _, err := os.Stat(out); err != nil {
			t.Fatalf("scan-all should create %s: %v", out, err)
		}
	}
	gitignoreEntries := map[string][]string{
		filepath.Join(frontend, ".gitignore"):  {"dist/", "# GoreGraph local scan output", "goregraph-out/"},
		filepath.Join(cadaster, ".gitignore"):  {"# GoreGraph local scan output", "goregraph-out/"},
		filepath.Join(task, ".gitignore"):      {"# GoreGraph local scan output", "goregraph-out/"},
		filepath.Join(workspace, ".gitignore"): {"# GoreGraph local workspace output", ".goregraph-workspace/"},
	}
	for path, entries := range gitignoreEntries {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for _, entry := range entries {
			if !strings.Contains(string(body), entry) {
				t.Fatalf("%s missing %q:\n%s", path, entry, string(body))
			}
		}
		if !strings.Contains(stdout.String(), "Updated .gitignore: "+path) {
			t.Fatalf("scan-all output missing changed path %s:\n%s", path, stdout.String())
		}
	}

	var secondOut, secondErr bytes.Buffer
	if code := Run([]string{"workspace", "scan-all", workspace}, &secondOut, &secondErr); code != 0 {
		t.Fatalf("second scan-all exit code = %d, stderr=%s", code, secondErr.String())
	}
	if strings.Contains(secondOut.String(), "Updated .gitignore:") {
		t.Fatalf("second scan-all reported unchanged .gitignore:\n%s", secondOut.String())
	}
	for path, entries := range gitignoreEntries {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s after second scan: %v", path, err)
		}
		for _, entry := range entries {
			if count := strings.Count(string(body), entry); count != 1 {
				t.Fatalf("%s entry %q count = %d, want 1:\n%s", path, entry, count, string(body))
			}
		}
	}
}

func TestRunWorkspaceScanAllNoUpdateGitignoreLeavesFilesUntouched(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, ".gitignore", "dist/\n")
	writeFile(t, cadaster, "README.md", "# ms-cadaster\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "scan-all", workspace, "--no-update-gitignore"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	frontendGitignore, err := os.ReadFile(filepath.Join(frontend, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(frontendGitignore) != "dist/\n" {
		t.Fatalf("frontend .gitignore changed despite opt-out:\n%s", string(frontendGitignore))
	}
	for _, path := range []string{filepath.Join(cadaster, ".gitignore"), filepath.Join(workspace, ".gitignore")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s exists after opt-out, err=%v", path, err)
		}
	}
	if strings.Contains(stdout.String(), "Updated .gitignore:") {
		t.Fatalf("opt-out reported .gitignore changes:\n%s", stdout.String())
	}
}

func TestRunWorkspaceScanAllUsesCustomProjectOutputInGitignore(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "goregraph.yml", "output: .goregraph\n")
	writeFile(t, cadaster, "README.md", "# ms-cadaster\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "scan-all", workspace}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	body, err := os.ReadFile(filepath.Join(frontend, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), ".goregraph/") || strings.Contains(string(body), "goregraph-out/") {
		t.Fatalf("frontend .gitignore does not contain only the configured output:\n%s", string(body))
	}
	if !strings.Contains(stdout.String(), "Updated .gitignore: "+filepath.Join(frontend, ".gitignore")) {
		t.Fatalf("scan-all output missing custom project .gitignore path:\n%s", stdout.String())
	}
}

func TestRunWorkspaceCleanDryRunAndExecuteRemovesGeneratedWorkspaceOutput(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, cadaster, "pom.xml", `<project><artifactId>ms-cadaster</artifactId></project>`)
	writeFile(t, cadaster, "README.md", "# ms-cadaster\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"workspace", "scan-all", workspace, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan-all exit code = %d, stderr=%s", code, scanErr.String())
	}
	var dryOut, dryErr bytes.Buffer

	dryCode := Run([]string{"workspace", "clean", workspace}, &dryOut, &dryErr)

	if dryCode != 0 {
		t.Fatalf("dry-run exit code = %d, want 0; stderr=%s", dryCode, dryErr.String())
	}
	if !strings.Contains(dryOut.String(), "Dry run: true") || !strings.Contains(dryOut.String(), "goregraph-out") || !strings.Contains(dryOut.String(), ".goregraph-workspace") {
		t.Fatalf("clean dry-run output missing expected paths:\n%s", dryOut.String())
	}
	if _, err := os.Stat(filepath.Join(frontend, "goregraph-out", "manifest.json")); err != nil {
		t.Fatalf("dry-run should keep frontend output: %v", err)
	}
	var execOut, execErr bytes.Buffer

	execCode := Run([]string{"workspace", "clean", workspace, "--execute"}, &execOut, &execErr)

	if execCode != 0 {
		t.Fatalf("execute exit code = %d, want 0; stderr=%s", execCode, execErr.String())
	}
	for _, path := range []string{
		filepath.Join(frontend, "goregraph-out"),
		filepath.Join(cadaster, "goregraph-out"),
		filepath.Join(workspace, ".goregraph-workspace"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("clean --execute should remove %s, err=%v\noutput:\n%s", path, err, execOut.String())
		}
	}
	if !strings.Contains(execOut.String(), "Removed 3 GoreGraph output path(s)") {
		t.Fatalf("clean execute output missing removal summary:\n%s", execOut.String())
	}
}

func TestRunWorkspaceDiffComparesTwoWorkspaceOutputDirs(t *testing.T) {
	root := t.TempDir()
	before := filepath.Join(root, "before")
	after := filepath.Join(root, "after")
	writeFile(t, before, "index/contract-matches.json", `[{"id":"a","api_http_method":"GET","api_path":"/a","issue":"matched","confidence":"RESOLVED"}]`)
	writeFile(t, before, "index/feature-flows.json", `[{"id":"flow-a","tests":[{"confidence":"MATCHED"}]}]`)
	writeFile(t, after, "index/contract-matches.json", `[{"id":"a","api_http_method":"GET","api_path":"/a","issue":"indexed_backend_route_missing","confidence":"UNRESOLVED"},{"id":"b","api_http_method":"POST","api_path":"/b","issue":"matched","confidence":"RESOLVED"}]`)
	writeFile(t, after, "index/feature-flows.json", `[{"id":"flow-a"}]`)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "diff", "--before", before, "--after", after}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"# GoreGraph Workspace Diff", "New contracts: 1", "Changed contracts: 1", "flow-a lost matched tests"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace diff output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunUpdateRefreshesCurrentProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	}()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"update", "--no-update-gitignore"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "goregraph-out", "manifest.json")); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
}

func TestRunReportPrintsGeneratedReport(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"report", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# GoreGraph Report") {
		t.Fatalf("report output missing heading:\n%s", stdout.String())
	}
}

func TestRunReportUsesConfiguredOutputDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "goregraph.yml", "output: .goregraph\nupdate_gitignore: false\n")
	writeFile(t, root, "README.md", "# Demo\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"report", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# GoreGraph Report") {
		t.Fatalf("report output missing heading:\n%s", stdout.String())
	}
}

func TestRunQuerySearchesGeneratedIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"query", root, "StartServer"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "StartServer") {
		t.Fatalf("query output missing symbol:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "src/main.go") {
		t.Fatalf("query output missing file:\n%s", stdout.String())
	}
}

func TestRunQueryMissingIndexTellsUserToBuildAll(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"query", root, "StartServer"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "goregraph build all") {
		t.Fatalf("stderr missing build guidance:\n%s", stderr.String())
	}
}

func TestRunExplainPrintsFileContext(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"explain", root, "src/main.go"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "src/main.go") {
		t.Fatalf("explain output missing file:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "StartServer") {
		t.Fatalf("explain output missing symbol:\n%s", stdout.String())
	}
}

func TestRunDoctorReportsHealthyIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"doctor", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK   output") {
		t.Fatalf("doctor output missing output check:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "OK   schema") {
		t.Fatalf("doctor output missing schema check:\n%s", stdout.String())
	}
}

func TestRunDoctorReturnsFailureForMissingIndex(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"doctor", root}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "FAIL output") {
		t.Fatalf("doctor output missing output failure:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "goregraph scan") {
		t.Fatalf("doctor output missing scan guidance:\n%s", stdout.String())
	}
}

func TestRunDoctorWarnsForStaleIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc main() {}\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	writeFile(t, root, "src/main.go", "package main\nfunc main() { println(\"changed\") }\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"doctor", root}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "WARN stale") {
		t.Fatalf("doctor output missing stale warning:\n%s", stdout.String())
	}
}

func TestRunMCPHelpPrintsUsage(t *testing.T) {
	want := `Usage: goregraph mcp [--expert-tools]

Starts the read-only MCP stdio server.
Default mode exposes only task_context to prevent query cascades.
--expert-tools exposes legacy diagnostic and exploration tools.
`
	for _, help := range []string{"help", "--help", "-h"} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"mcp", help}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%s exit code = %d, want 0; stderr=%s", help, code, stderr.String())
		}
		if stdout.String() != want {
			t.Fatalf("%s help = %q, want %q", help, stdout.String(), want)
		}
	}
}

func TestRunMCPRejectsUnknownDuplicateAndPositionalOptions(t *testing.T) {
	for _, args := range [][]string{
		{"mcp", "--unknown"},
		{"mcp", "expert"},
		{"mcp", "--expert-tools", "--expert-tools"},
		{"mcp", "--expert-tools", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("%v exit code = %d, want 2; stdout=%s stderr=%s", args, code, stdout.String(), stderr.String())
		}
		if stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage: goregraph mcp [--expert-tools]") {
			t.Fatalf("%v streams: stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}
}

func TestRunMCPPropagatesStandardAndExpertModes(t *testing.T) {
	for _, test := range []struct {
		name      string
		args      []string
		want      string
		forbidden string
	}{
		{name: "standard", args: []string{"mcp"}, want: `"name":"task_context"`, forbidden: `"name":"coverage"`},
		{name: "expert", args: []string{"mcp", "--expert-tools"}, want: `"name":"coverage"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := writer.WriteString(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n"); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			oldStdin := os.Stdin
			os.Stdin = reader
			t.Cleanup(func() {
				os.Stdin = oldStdin
				_ = reader.Close()
			})

			var stdout, stderr bytes.Buffer
			if code := Run(test.args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), test.want) ||
				test.forbidden != "" && strings.Contains(stdout.String(), test.forbidden) {
				t.Fatalf("unexpected MCP tool list:\n%s", stdout.String())
			}
		})
	}
}

func TestRunVersionPrintsBuildMetadata(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"goregraph 1.3.0",
		"commit:",
		"built:",
		"go:",
		"platform:",
		"schema: 3",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("version output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunWorkspaceDashboardPrintsDashboardPath(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(out, "dashboard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "dashboard", "workspace-map.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "dashboard", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "workspace-map.html") {
		t.Fatalf("stdout missing dashboard path:\n%s", stdout.String())
	}
}

func TestRunDashboardUsesWorkspaceDashboardWhenProjectDashboardMissing(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, workspace, ".goregraph-workspace.yml", "")
	dashboard := writeCompleteCLIWorkspaceDashboard(t, workspace)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"dashboard", workspace}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	want, err := filepath.Abs(dashboard)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("dashboard path = %q, want workspace dashboard %q", got, want)
	}
}

func TestRunDashboardPrefersProjectDashboardWhenBothExist(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".goregraph-workspace.yml", "")
	writeFile(t, root, "goregraph-out/dashboard/report.md", "# Project dashboard\n")
	writeCompleteCLIWorkspaceDashboard(t, root)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"dashboard", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	want, err := filepath.Abs(filepath.Join(root, "goregraph-out", "dashboard"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("dashboard path = %q, want project dashboard %q", got, want)
	}
}

func TestRunDashboardRejectsMissingDashboardInsteadOfPrintingHypotheticalPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/project\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"dashboard", root}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%s; stderr=%s", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		"dashboard not found",
		"goregraph build dashboard",
		"goregraph workspace build dashboard",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestDashboardHelpExplainsAutomaticProjectAndWorkspaceResolution(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"dashboard", "help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"path|open|edit",
		"existing project dashboard first",
		"falls back",
		"workspace dashboard",
		"open opens",
		"edit starts",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunDashboardEditUsesSecureWorkspaceEditor(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, workspace, ".goregraph-workspace.yml", "")
	writeFile(t, workspace, "goregraph-out/dashboard/report.md", "# Project dashboard\n")
	dashboard := writeCompleteCLIWorkspaceDashboard(t, workspace)
	previousServe := serveDashboardEditor
	previousOpen := openDashboardEditorURL
	t.Cleanup(func() {
		serveDashboardEditor = previousServe
		openDashboardEditorURL = previousOpen
	})

	var gotRoot, gotDashboard string
	serveDashboardEditor = func(_ context.Context, options dashboardeditor.Options) error {
		gotRoot = options.WorkspaceRoot
		gotDashboard = options.DashboardPath
		options.OnReady("http://127.0.0.1:12345/#token=test")
		return nil
	}
	openDashboardEditorURL = func(string) error { return nil }
	var stdout, stderr bytes.Buffer

	code := Run([]string{"dashboard", "edit", workspace}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if gotRoot != workspace || gotDashboard != dashboard {
		t.Fatalf("editor root=%q dashboard=%q, want root=%q dashboard=%q", gotRoot, gotDashboard, workspace, dashboard)
	}
	if !strings.Contains(stdout.String(), "Dashboard editor: http://127.0.0.1:12345/#token=test") {
		t.Fatalf("editor URL missing from stdout:\n%s", stdout.String())
	}
}

func TestWorkspaceDashboardHelpExplainsStaticAndEditableModes(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)

	if code := Run([]string{"workspace", "dashboard", "help"}, stdout, stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	for _, want := range []string{
		"path|open|edit",
		"path prints the generated static dashboard file",
		"open opens that static read-only file",
		"Only edit starts an authenticated loopback server",
		".goregraph-dashboard.json",
		"Ctrl-C",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout)
		}
	}
}

func TestHelpDistinguishesWorkspaceDashboardPathOpenAndEdit(t *testing.T) {
	for _, args := range [][]string{{"help", "--all"}, {"workspace", "help", "--all"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", args, code, stderr.String())
		}
		for _, want := range []string{
			"workspace dashboard path",
			"workspace dashboard open",
			"workspace dashboard edit",
			"Only edit starts an authenticated loopback server",
		} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("%v help missing %q:\n%s", args, want, stdout.String())
			}
		}
	}
}

func TestRunWorkspaceDashboardEditUsesResolvedWorkspaceAndDashboard(t *testing.T) {
	workspace := t.TempDir()
	dashboard := writeCompleteCLIWorkspaceDashboard(t, workspace)
	previousServe := serveDashboardEditor
	previousOpen := openDashboardEditorURL
	t.Cleanup(func() {
		serveDashboardEditor = previousServe
		openDashboardEditorURL = previousOpen
	})

	var gotRoot, gotDashboard string
	serveDashboardEditor = func(_ context.Context, options dashboardeditor.Options) error {
		gotRoot = options.WorkspaceRoot
		gotDashboard = options.DashboardPath
		options.OnReady("http://127.0.0.1:12345/#token=test")
		if err := options.OpenURL("http://127.0.0.1:12345/#token=test"); err != nil {
			t.Fatalf("opening editor URL: %v", err)
		}
		return nil
	}
	openDashboardEditorURL = func(string) error { return nil }
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "dashboard", "edit", "--workspace", workspace}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if gotRoot != workspace {
		t.Fatalf("workspace root=%q, want %q", gotRoot, workspace)
	}
	if gotDashboard != dashboard {
		t.Fatalf("dashboard path=%q, want %q", gotDashboard, dashboard)
	}
	if !strings.Contains(stdout.String(), "Dashboard editor: http://127.0.0.1:12345/#token=test") {
		t.Fatalf("editor URL missing from stdout:\n%s", stdout.String())
	}
}

func TestRunWorkspaceDashboardEditLeavesURLUsableWhenOpeningFails(t *testing.T) {
	workspace := t.TempDir()
	writeCompleteCLIWorkspaceDashboard(t, workspace)
	previousServe := serveDashboardEditor
	previousOpen := openDashboardEditorURL
	t.Cleanup(func() {
		serveDashboardEditor = previousServe
		openDashboardEditorURL = previousOpen
	})

	serveDashboardEditor = func(_ context.Context, options dashboardeditor.Options) error {
		options.OnReady("http://127.0.0.1:12345/#token=test")
		if err := options.OpenURL("http://127.0.0.1:12345/#token=test"); err == nil {
			t.Fatal("expected browser opener error")
		}
		return nil
	}
	openDashboardEditorURL = func(string) error { return errors.New("browser unavailable") }
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "dashboard", "edit", workspace}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Dashboard editor: http://127.0.0.1:12345/#token=test") {
		t.Fatalf("editor URL missing from stdout:\n%s", stdout.String())
	}
}

func TestRunWorkspaceDashboardEditRequiresCompleteDashboard(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, filepath.Join(workspace, ".goregraph-workspace", "dashboard"), "workspace-map.html", "<html></html>")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "dashboard", "edit", "--workspace", workspace}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code=%d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "dashboard is incomplete") {
		t.Fatalf("missing incomplete dashboard error:\n%s", stderr.String())
	}
}

func TestRunWorkspaceDashboardEditReportsServerFailures(t *testing.T) {
	workspace := t.TempDir()
	writeCompleteCLIWorkspaceDashboard(t, workspace)
	previousServe := serveDashboardEditor
	t.Cleanup(func() { serveDashboardEditor = previousServe })
	serveDashboardEditor = func(context.Context, dashboardeditor.Options) error {
		return errors.New("listen failed")
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "dashboard", "edit", workspace}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code=%d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "error: dashboard editor failed: listen failed") {
		t.Fatalf("missing editor failure:\n%s", stderr.String())
	}
}

func TestRunWorkspaceRefreshRebuildsWorkspaceOverlayWithoutScanningSources(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	frontend := filepath.Join(workspace, "frontend", "app")
	backend := filepath.Join(workspace, "microservices", "ms-user")
	writeFile(t, frontend, "package.json", `{"name":"app"}`)
	writeFile(t, frontend, "src/api/users.ts", "export function getUser(id) {\n  return fetch(`/users/${id}`);\n}\n")
	writeFile(t, backend, "src/main/java/com/example/UserController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RestController;

@RestController
class UserController {
  @GetMapping("/users/{userId}")
  String get(@PathVariable String userId) {
    return userId;
  }
}
`)
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"workspace", "scan-all", workspace, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan-all exit code = %d, stderr=%s", code, scanErr.String())
	}
	dashboard := filepath.Join(workspace, ".goregraph-workspace", "dashboard", "workspace-map.html")
	if err := os.Remove(dashboard); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "refresh", workspace}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("refresh exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(dashboard); err != nil {
		t.Fatalf("refresh should recreate dashboard: %v", err)
	}
	assets, err := filepath.Glob(filepath.Join(workspace, ".goregraph-workspace", "dashboard", "workspace-map-assets", "*.js"))
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) == 0 {
		t.Fatal("refresh should create project-specific dashboard usage assets")
	}
	if !strings.Contains(stdout.String(), "Refreshed workspace overlay") ||
		!strings.Contains(stdout.String(), "workspace-map.html") {
		t.Fatalf("refresh output missing summary:\n%s", stdout.String())
	}
}

func TestRunWorkspaceExplainPathAndImpactUseGeneratedWorkspaceGraph(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	graph := `{"nodes":[{"id":"project:frontend/app","kind":"project","label":"frontend/app"},{"id":"contract:get-users","kind":"contract","label":"GET /users/{userId}","file":"src/api/users.ts"},{"id":"route:ms-user:get:/users/{userid}","kind":"route","label":"UserController.get","symbol":"UserController.get"}],"edges":[{"id":"edge:1","from":"project:frontend/app","to":"contract:get-users","kind":"declares_contract"},{"id":"edge:2","from":"contract:get-users","to":"route:ms-user:get:/users/{userid}","kind":"resolved_by","confidence":"RESOLVED"}]}`
	if err := os.MkdirAll(filepath.Join(out, "index"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "index", "workspace-graph.json"), []byte(graph), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "index", "feature-dossiers.json"), []byte(`[{"id":"feature:get-users","route":"GET /users/{userId}","source_flow_id":"flow:get-users","frontend_project":"frontend/app","backend_project":"services/ms-user"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "index", "feature-flows.json"), []byte(`[{"id":"flow:get-users","frontend_project":"frontend/app","frontend_file":"src/api/users.ts","backend_project":"services/ms-user","backend_file":"UserController.java","http_method":"GET","path":"/users/{userId}"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"workspace", "explain", "GET /users/{userId}", "--workspace", root},
		{"workspace", "path", "--from", "frontend/app", "--to", "UserController.get", "--workspace", root},
		{"workspace", "impact", "--changed-file", "frontend/app/src/api/users.ts", "--workspace", root},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%v exit code = %d, want 0; stderr=%s", args, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "GET /users/{userId}") {
			t.Fatalf("%v output missing route:\n%s", args, stdout.String())
		}
	}
}

func TestRunScanHelpPrintsScanUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"scan", "help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: goregraph scan") {
		t.Fatalf("scan help missing usage:\n%s", stdout.String())
	}
}

func TestRunUpdateHelpDescribesUpdate(t *testing.T) {
	for _, args := range [][]string{
		{"update", "help"},
		{"update", "--help"},
		{"update", "-h"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", args, code, stderr.String())
		}
		for _, want := range []string{
			"Usage: goregraph update",
			"--target agent|dashboard|all",
			"Refreshes the current project's selected projections",
		} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("%v update help missing %q:\n%s", args, want, stdout.String())
			}
		}
		if strings.Contains(stdout.String(), "Compatibility alias for goregraph build all") {
			t.Fatalf("%v update help renders scan guidance:\n%s", args, stdout.String())
		}
	}
}

func TestRunUnknownCommandReturnsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"nope"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr missing unknown command:\n%s", stderr.String())
	}
}

func TestRunQueryDispatchesTaskContext(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	var scanOut, scanErr bytes.Buffer
	if code := Run([]string{"scan", root, "--no-update-gitignore"}, &scanOut, &scanErr); code != 0 {
		t.Fatalf("scan exit code = %d, stderr=%s", code, scanErr.String())
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"query", root, "task-context", "--query", "main.go", "--format", "markdown", "--limit", "5"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# GoreGraph task-context") {
		t.Fatalf("task context was not rendered:\n%s", stdout.String())
	}
}

func TestContextCLIHelpAndGlobalOrdering(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"context", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("context help exit code = %d, stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"Usage: goregraph context <path>",
		"--query <task>",
		"--budget-tokens 4000",
		"--max-files 12",
		"--format markdown|json",
		"--previous-context-id",
		"256-6000",
		"1-20",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("context help missing %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"help", "--all"}, &stdout, &stderr); code != 0 {
		t.Fatalf("global help exit code = %d, stderr=%s", code, stderr.String())
	}
	contextIndex := strings.Index(stdout.String(), "context <path>")
	queryIndex := strings.Index(stdout.String(), "query <path>")
	if contextIndex < 0 || queryIndex < 0 || contextIndex >= queryIndex {
		t.Fatalf("global help does not list context before query:\n%s", stdout.String())
	}
}

func TestCLIContextPassesPreviousContextID(t *testing.T) {
	root := writeCLIContextFixture(t, 0)
	var firstOut, firstErr bytes.Buffer
	if code := Run([]string{"context", root, "--query", "DELETE /users/{id}", "--format", "json"}, &firstOut, &firstErr); code != 0 {
		t.Fatalf("first context exit code = %d, stderr=%s", code, firstErr.String())
	}
	var first agent.ContextPack
	if err := json.Unmarshal(firstOut.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	var secondOut, secondErr bytes.Buffer
	if code := Run([]string{
		"context", root, "--query", "DELETE /users/{id}", "--format", "json",
		"--previous-context-id", first.ContextID,
	}, &secondOut, &secondErr); code != 0 {
		t.Fatalf("second context exit code = %d, stderr=%s", code, secondErr.String())
	}
	var second agent.ContextPack
	if err := json.Unmarshal(secondOut.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second.DuplicateOf != first.ContextID || second.EstimatedTokens > 200 || second.RetryAllowed {
		t.Fatalf("CLI duplicate response = %#v", second)
	}
}

func TestCLIEndpointContextSerialization(t *testing.T) {
	root := writeCLIEndpointContextFixture(t)
	query := "GET /orders authentication"
	expected, err := agent.BuildContext(agent.ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"context", root, "--query", query, "--format", "json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("context exit code = %d, stderr=%s", code, stderr.String())
	}
	var pack agent.ContextPack
	if err := json.Unmarshal(stdout.Bytes(), &pack); err != nil {
		t.Fatal(err)
	}
	assertCLIEndpointContextWireKeys(t, stdout.Bytes())
	if !reflect.DeepEqual(pack.Endpoints, expected.Endpoints) {
		t.Fatalf("CLI endpoints = %#v, want %#v", pack.Endpoints, expected.Endpoints)
	}
	if len(pack.Endpoints) != 1 {
		t.Fatalf("CLI endpoints = %#v", pack.Endpoints)
	}
	endpoint := pack.Endpoints[0]
	if endpoint.Provider != "services/orders" || endpoint.HTTPMethod != "GET" || endpoint.Path != "/orders/{id}" ||
		endpoint.Security != "bearer" || len(endpoint.Consumers) != 8 || endpoint.OmittedConsumers != 5 {
		t.Fatalf("CLI endpoint details = %#v", endpoint)
	}
	if pack.BudgetTokens != agent.DefaultContextBudgetTokens || pack.FallbackRequired {
		t.Fatalf("CLI context bounds/fallback = %#v", pack)
	}
	for _, forbidden := range []string{"api-catalog.json", ".goregraph-dashboard.json", "/invoices"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("CLI context contains %q: %s", forbidden, stdout.String())
		}
	}
}

func TestContextHelpDocumentsBoundedAgentWorkflow(t *testing.T) {
	const assistedInstruction = `Call task_context once before indexed source discovery. Treat source_sections as already read.
Retry only when retry_allowed is true, use one retry_anchor, and pass context_id as previous_context_id.
If duplicate_of is present, use the first pack and do not read more source because of the duplicate response.`

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"context", "help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("context help exit code = %d, stderr=%s", code, stderr.String())
	}
	if count := strings.Count(stdout.String(), assistedInstruction); count != 1 {
		t.Fatalf("context help contains the exact assisted instruction %d times, want 1:\n%s", count, stdout.String())
	}
	for _, want := range []string{
		"goregraph-out/agent/",
		".goregraph-workspace/agent/",
		"Do not read index/, dashboard/",
		"goregraph doctor .",
		"missing or stale output",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("context help missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("context help stderr = %q", stderr.String())
	}
}

func TestQueryAndReportHelpDocumentManualCompatibilityAndDashboardPath(t *testing.T) {
	const sourceBackedGuidance = "Agents should use goregraph context: when source_coverage is complete, continue from source_sections without another navigation read; when source_coverage is partial or none, read only relevant uncovered ranges named by source_omissions or files not represented by source_sections."

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"query", "help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("query help exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), sourceBackedGuidance) {
		t.Fatalf("query help is missing exact source-backed guidance:\n%s", stdout.String())
	}
	for _, want := range []string{
		"Legacy/manual compatibility",
		"not the normal agent workflow",
		"goregraph context",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("query help missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "inspect source after its bounded workflow") {
		t.Fatalf("query help contains obsolete source-reading guidance:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"report", "help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("report help exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "goregraph-out/dashboard/report.md") ||
		strings.Contains(stdout.String(), "goregraph-out/report.md") {
		t.Fatalf("report help uses stale path:\n%s", stdout.String())
	}
}

func TestBuildHelpDocumentsSharedExtractionAndWorkspaceReconciliation(t *testing.T) {
	for _, test := range []struct {
		args []string
		want []string
	}{
		{
			args: []string{"build", "help"},
			want: []string{
				"goregraph build <agent|dashboard|all>",
				"Extracts source once",
				"does not require a workspace marker",
			},
		},
		{
			args: []string{"workspace", "build", "help"},
			want: []string{
				"goregraph workspace build <agent|dashboard|all>",
				"Scans each discovered project once",
				"reconciles the workspace once",
			},
		},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(test.args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", test.args, code, stderr.String())
		}
		for _, want := range test.want {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("%v help missing %q:\n%s", test.args, want, stdout.String())
			}
		}
	}
}

func TestGlobalAndWorkspaceHelpLeadWithCanonicalBuildsAndMarkerRules(t *testing.T) {
	for _, test := range []struct {
		args []string
		want []string
	}{
		{
			args: []string{"help", "--all"},
			want: []string{
				"goregraph build agent .",
				"goregraph build dashboard .",
				"goregraph build all .",
				"goregraph update . --target agent",
				"scan is the compatibility alias for build all",
				"standard MCP exposes only task_context",
				"--expert-tools is for manual diagnostics",
				"Project build commands do not require a workspace marker",
				"does not create .goregraph-workspace.yml",
				".goregraph-workspace/ is removable generated output",
			},
		},
		{
			args: []string{"workspace", "help", "--all"},
			want: []string{
				"goregraph workspace build agent .",
				"goregraph workspace build dashboard .",
				"goregraph workspace build all .",
				"goregraph workspace refresh . --target agent",
				"scan-all is the compatibility alias for workspace build all",
				"Scans each discovered project once and reconciles once",
				"--workspace <path>",
				".goregraph-workspace.yml",
				"does not create .goregraph-workspace.yml",
				".goregraph-workspace/ is removable generated output",
			},
		},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(test.args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", test.args, code, stderr.String())
		}
		for _, want := range test.want {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("%v help missing %q:\n%s", test.args, want, stdout.String())
			}
		}
	}
}

func TestWorkspaceHelpUsesProgressiveDisclosure(t *testing.T) {
	for _, args := range [][]string{
		{"workspace", "help"},
		{"workspace", "--help"},
		{"workspace", "-h"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", args, code, stderr.String())
		}
		for _, want := range []string{
			"build <target>", "dashboard", "status", "explain", "path", "impact",
			"goregraph workspace help --all",
		} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("%v workspace help missing %q:\n%s", args, want, stdout.String())
			}
		}
		if strings.Contains(stdout.String(), "scan-all") {
			t.Fatalf("%v standard workspace help exposes compatibility command:\n%s", args, stdout.String())
		}
	}
}

func TestWorkspaceAllHelpPreservesCompleteCommandCatalog(t *testing.T) {
	for _, args := range [][]string{
		{"workspace", "help", "--all"},
		{"workspace", "--help", "--all"},
		{"workspace", "-h", "--all"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", args, code, stderr.String())
		}
		for _, want := range []string{
			"Core commands:", "build <target>", "status", "dashboard",
			"Exploration:", "explain", "path", "impact", "diff",
			"Maintenance:", "scan-missing", "refresh", "clean", "git update",
			"Compatibility:", "scan-all",
		} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("%v complete workspace help missing %q:\n%s", args, want, stdout.String())
			}
		}
	}
}

func TestWorkspaceHelpRejectsUnknownSelector(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"workspace", "help", "--unknown"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "goregraph workspace help") {
		t.Fatalf("unexpected streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestWorkspaceUsageAndDashboardHelpMatchCanonicalActions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"workspace"}, &stdout, &stderr); code != 2 {
		t.Fatalf("workspace exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "workspace <build|status|") {
		t.Fatalf("workspace usage omits build:\n%s", stderr.String())
	}

	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"dashboard", "help"}, want: "Usage: goregraph dashboard path|open|edit [path]"},
		{args: []string{"workspace", "dashboard", "help"}, want: "Usage: goregraph workspace dashboard path|open|edit [path] [--workspace <path>]"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(test.args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", test.args, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), test.want) {
			t.Fatalf("%v help missing %q:\n%s", test.args, test.want, stdout.String())
		}
	}
}

func TestWorkspaceScanAllHelpDocumentsOneReconciliationAndProjectBoundaries(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"workspace", "scan-all", "help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("workspace scan-all help exit code = %d, stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"Compatibility alias for goregraph workspace build all",
		"Scans each discovered project once and reconciles the workspace once",
		"--workspace <path>",
		".goregraph-workspace.yml",
		"does not create .goregraph-workspace.yml",
		".goregraph-workspace/ is removable generated output",
		"project/build marker",
		".git alone",
		"goregraph.yml",
		"explicit project build",
		"Once a project root is detected",
		"nested manifests remain part of that project",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace scan-all help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceCompleteHelpDocumentsProjectBoundaries(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"workspace", "help", "--all"}, &stdout, &stderr); code != 0 {
		t.Fatalf("complete workspace help exit code = %d, stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"project/build marker",
		".git alone",
		"goregraph.yml",
		"explicit project build",
		"Once a project root is detected",
		"nested manifests remain part of that project",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("complete workspace help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestContextCLIAcceptsBudgetAndMaxFiles(t *testing.T) {
	root := writeCLIContextFixture(t, 1)
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"context", root,
		"--query", "DELETE /users/{id}",
		"--budget-tokens", "900",
		"--max-files", "6",
		"--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var pack agent.ContextPack
	if err := json.Unmarshal(stdout.Bytes(), &pack); err != nil {
		t.Fatalf("direct context output is not a ContextPack: %v\n%s", err, stdout.String())
	}
	if pack.BudgetTokens != 900 || pack.FallbackRequired || len(pack.Files) > 6 {
		t.Fatalf("unexpected direct context pack: %#v", pack)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope["task"]; ok {
		t.Fatalf("direct context JSON returned legacy envelope:\n%s", stdout.String())
	}
	if !strings.HasSuffix(stdout.String(), "\n") || strings.HasSuffix(stdout.String(), "\n\n") {
		t.Fatalf("direct JSON must end in exactly one newline: %q", stdout.String())
	}
}

func TestContextCLIRejectsUsageErrors(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing path", args: []string{"context"}, want: "usage"},
		{name: "missing query", args: []string{"context", root}, want: "requires --query"},
		{name: "missing value", args: []string{"context", root, "--query"}, want: "requires a value"},
		{name: "unknown option", args: []string{"context", root, "--unknown", "value"}, want: "unknown context option"},
		{name: "noninteger budget", args: []string{"context", root, "--query", "route", "--budget-tokens", "many"}, want: "must be an integer"},
		{name: "noninteger max files", args: []string{"context", root, "--query", "route", "--max-files", "many"}, want: "must be an integer"},
		{name: "explicit zero budget", args: []string{"context", root, "--query", "route", "--budget-tokens", "0"}, want: "between 256 and 6000"},
		{name: "explicit zero max files", args: []string{"context", root, "--query", "route", "--max-files", "0"}, want: fmt.Sprintf("between %d and %d", agent.MinContextMaxFiles, agent.MaxContextMaxFiles)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(test.args, &stdout, &stderr); code != 2 {
				t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("unexpected streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestContextCLIParsersShareAgentMaxFilesBounds(t *testing.T) {
	source, err := os.ReadFile("cli.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(source), "agent.MinContextMaxFiles") < 2 {
		t.Fatalf("context CLI parsers do not share the agent max-files minimum")
	}
}

func TestRunQueryRejectsExplicitZeroValuesAsUsage(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		option string
		want   string
	}{
		{option: "--budget-tokens", want: "between 256 and 6000"},
		{option: "--max-files", want: fmt.Sprintf("between %d and %d", agent.MinContextMaxFiles, agent.MaxContextMaxFiles)},
		{option: "--limit", want: "between 1 and 100"},
	} {
		t.Run(test.option, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"query", root, "task-context", "--query", "GET /users",
				test.option, "0",
			}, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("unexpected streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunQueryTaskContextMapsLimitAndMaxFilesBeforeServiceDefaults(t *testing.T) {
	root := writeCLIContextFixture(t, 19)
	tests := []struct {
		name       string
		options    []string
		maxFiles   int
		wantFiles  int
		wantBudget int
	}{
		{name: "default remains bounded", maxFiles: agent.DefaultContextMaxFiles, wantBudget: 4000},
		{name: "explicit limit constrains selection", options: []string{"--limit", "1"}, maxFiles: 1, wantFiles: 1, wantBudget: 4000},
		{name: "limit is capped", options: []string{"--limit", "25"}, maxFiles: agent.MaxContextMaxFiles, wantBudget: 4000},
		{name: "max files wins after limit", options: []string{"--limit", "1", "--max-files", "2", "--budget-tokens", "900"}, maxFiles: 2, wantFiles: 2, wantBudget: 900},
		{name: "max files wins before limit", options: []string{"--max-files", "2", "--limit", "1", "--budget-tokens", "900"}, maxFiles: 2, wantFiles: 2, wantBudget: 900},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := []string{"query", root, "task-context", "--query", "GET /users", "--format", "json"}
			args = append(args, test.options...)
			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
			}
			first := stdout.String()
			stdout.Reset()
			stderr.Reset()
			if code := Run(args, &stdout, &stderr); code != 0 {
				t.Fatalf("repeat exit code = %d, stderr=%s", code, stderr.String())
			}
			if first != stdout.String() {
				t.Fatalf("task context is not deterministic:\nfirst=%s\nsecond=%s", first, stdout.String())
			}
			pack := decodeCLIContextPack(t, stdout.Bytes())
			if len(pack.Files) == 0 || len(pack.Files) > test.maxFiles || pack.BudgetTokens != test.wantBudget {
				t.Fatalf("files/budget = %d/%d, want 1-%d/%d: %#v", len(pack.Files), pack.BudgetTokens, test.maxFiles, test.wantBudget, pack)
			}
			if test.wantFiles != 0 && len(pack.Files) != test.wantFiles {
				t.Fatalf("files = %d, want %d: %#v", len(pack.Files), test.wantFiles, pack)
			}
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if _, ok := envelope["task"]; !ok {
				t.Fatalf("legacy query JSON lost agent.Result envelope:\n%s", stdout.String())
			}
		})
	}
}

func TestRunQueryTaskContextRequiresQueryAsUsage(t *testing.T) {
	root := writeCLIContextFixture(t, 1)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"query", root, "task-context", "--max-files", "6"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "task-context requires --query") {
		t.Fatalf("unexpected streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestQueryHelpExplainsTaskContextBudgets(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"query", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d, stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"--budget-tokens", "--max-files", "task-context", "capped at 20"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("query help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunQueryRecognizesImpactSummaryTask(t *testing.T) {
	if !isAgentQueryTask("impact-summary") {
		t.Fatal("impact-summary must be dispatched through the agent query path")
	}
}

func TestRunQueryAndExplainExposeCanonicalSymbolOperations(t *testing.T) {
	workspace := writeCLISymbolProjectionFixture(t)
	cases := []struct {
		args []string
		want string
	}{
		{
			args: []string{"query", workspace, "symbol-inventory", "--query", "microservices/ms-user", "--format", "markdown", "--limit", "20"},
			want: "# GoreGraph symbol-inventory",
		},
		{
			args: []string{"query", workspace, "symbol-resolve", "--query", "com.weka.UserService", "--format", "json", "--limit", "20"},
			want: `"task": "symbol-resolve"`,
		},
		{
			args: []string{"query", workspace, "symbol-usages", "--query", "symbol:cli-01", "--format", "markdown", "--limit", "20"},
			want: "direct_reference",
		},
		{
			args: []string{"query", workspace, "symbol-api-consumers", "--query", "symbol:cli-01", "--format", "json", "--limit", "20"},
			want: `"category": "reached_through_api"`,
		},
		{
			args: []string{"query", workspace, "symbol-explain", "--query", "usage:cli-direct", "--detail", "full", "--format", "json", "--limit", "20"},
			want: `"reason": "qualified Java reference"`,
		},
		{
			args: []string{"explain", workspace, "symbol:cli-01"},
			want: "# GoreGraph symbol-explain",
		},
		{
			args: []string{"explain", workspace, "usage:cli-direct"},
			want: "qualified Java reference",
		},
	}
	for _, test := range cases {
		var stdout, stderr bytes.Buffer
		if code := Run(test.args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", test.args, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), test.want) {
			t.Fatalf("%v output missing %q:\n%s", test.args, test.want, stdout.String())
		}
	}
}

func TestRunSymbolQuerySyntaxAndMissingProjectionExitCodes(t *testing.T) {
	for _, task := range []string{
		"symbol-inventory",
		"symbol-resolve",
		"symbol-usages",
		"symbol-api-consumers",
		"symbol-explain",
	} {
		if !isAgentQueryTask(task) {
			t.Fatalf("%s must be dispatched through the agent query path", task)
		}
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"query", t.TempDir(), "symbol-inventory", "--query"}, &stdout, &stderr); code != 2 {
		t.Fatalf("syntax exit code = %d, want 2; stderr=%s", code, stderr.String())
	}
	for _, task := range []string{"symbol-resolve", "symbol-usages", "symbol-api-consumers", "symbol-explain"} {
		stdout.Reset()
		stderr.Reset()
		if code := Run([]string{"query", t.TempDir(), task, "--limit", "20"}, &stdout, &stderr); code != 2 {
			t.Fatalf("%s missing-query exit code = %d, want 2; stderr=%s", task, code, stderr.String())
		}
	}

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".goregraph-workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"query", workspace, "symbol-inventory", "--limit", "20"}, &stdout, &stderr); code != 1 {
		t.Fatalf("missing projection exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "workspace build all") {
		t.Fatalf("missing projection remediation:\n%s", stderr.String())
	}
}

func TestQueryHelpListsCanonicalSymbolOperations(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"query", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d, stderr=%s", code, stderr.String())
	}
	for _, task := range []string{
		"symbol-inventory",
		"symbol-resolve",
		"symbol-usages",
		"symbol-api-consumers",
		"symbol-explain",
	} {
		if !strings.Contains(stdout.String(), task) {
			t.Fatalf("query help missing %s:\n%s", task, stdout.String())
		}
	}
}

func writeCLISymbolProjectionFixture(t *testing.T) string {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), "weka")
	writeFile(t, workspace, ".goregraph-workspace/symbol-index.json", `{
  "schema_version": 2,
  "symbols": [{
    "id": "symbol:cli-01",
    "project": "microservices/ms-user",
    "language": "java",
    "kind": "class",
    "name": "UserService",
    "qualified_name": "com.weka.UserService",
    "declaration_file": "src/UserService.java",
    "declaration_line": 10,
    "analyzer": "java-source",
    "confidence": "EXACT",
    "coverage": "COMPLETE"
  }],
  "coverage": []
}`)
	writeFile(t, workspace, ".goregraph-workspace/symbol-usages.json", `{
  "schema_version": 2,
  "usages": [{
    "id": "usage:cli-direct",
    "provider_symbol_id": "symbol:cli-01",
    "consumer_project": "microservices/ms-order",
    "category": "direct_reference",
    "language": "java",
    "relation_kind": "calls_method_owner",
    "source_file": "src/OrderService.java",
    "source_line": 20,
    "confidence": "EXACT",
    "resolution": "EXACT",
    "reason": "qualified Java reference",
    "analyzer": "workspace-symbols"
  }, {
    "id": "usage:cli-api",
    "provider_symbol_id": "symbol:cli-01",
    "consumer_project": "frontend/app",
    "category": "reached_through_api",
    "language": "typescript",
    "relation_kind": "http_reachability",
    "source_file": "src/UserPage.tsx",
    "source_line": 7,
    "confidence": "RESOLVED",
    "resolution": "EXACT",
    "reason": "resolved HTTP contract and implementation",
    "analyzer": "workspace-symbol-api",
    "transport": "http",
    "api_path": [{
      "position": 0,
      "kind": "selected_symbol",
      "project": "microservices/ms-user",
      "symbol_id": "symbol:cli-01",
      "label": "UserService"
    }]
  }],
  "coverage": []
}`)
	return workspace
}

func writeCLIContextFixture(t *testing.T, neighbors int) string {
	t.Helper()
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{{
			ID: "route", Project: "api", Kind: "route", Name: "users route",
			HTTPMethod: "DELETE", Path: "/users/{id}", File: "UserController.java",
			Line: 20, EndLine: 28, Confidence: "EXACT", Search: "GET /users DELETE /users/{id}",
		}},
	}
	for number := 0; number < neighbors; number++ {
		id := "neighbor-" + string(rune('a'+number))
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: id, Project: "api", Kind: "symbol", Name: id,
			File: id + ".go", Confidence: "EXACT",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge-" + id, FromFactID: "route", ToFactID: id,
			FromLabel: "route", ToLabel: id, Kind: "call", Confidence: "EXACT",
		})
	}
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "goregraph-out/agent/context-index.json", string(body))
	return root
}

func decodeCLIContextPack(t *testing.T, body []byte) agent.ContextPack {
	t.Helper()
	var result struct {
		Items []struct {
			Data struct {
				Context agent.ContextPack `json:"context"`
			} `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode legacy query result: %v\n%s", err, body)
	}
	if len(result.Items) != 1 {
		t.Fatalf("legacy query item count = %d, want 1", len(result.Items))
	}
	return result.Items[0].Data.Context
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := cliTestOutputPath(filepath.Join(root, filepath.FromSlash(rel)))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeCLIEndpointContextFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "endpoint:orders", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders/{id}",
				Qualified: "OrderController.get", HTTPMethod: "GET", Path: "/orders/{id}",
				File: "services/orders/src/OrderController.java", Line: 20,
				Summary: "provider orders; security bearer; request OrderRequest; response OrderResponse; 3 consumer call sites omitted",
				Search:  "GET /orders/{id} authentication bearer services orders", Confidence: "EXACT",
			},
			{
				ID: "endpoint:billing", Project: "services/billing", Kind: "api_endpoint", Name: "GET /invoices",
				HTTPMethod: "GET", Path: "/invoices", File: "services/billing/src/InvoiceController.java", Line: 12,
				Summary: "provider billing; security public", Search: "GET /invoices billing public", Confidence: "EXACT",
			},
		},
	}
	for number := 0; number < 10; number++ {
		id := fmt.Sprintf("consumer:%02d", number)
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: id, Project: fmt.Sprintf("frontend/web-%02d", number), Kind: "api_consumer", Name: id,
			File: fmt.Sprintf("frontend/web-%02d/src/api.ts", number), Line: 7,
			Summary: "consumer service web; auth bearer", Confidence: "RESOLVED",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + id, FromFactID: id, ToFactID: "endpoint:orders",
			Kind: "consumes_endpoint", Reason: "catalog consumer auth bearer", Confidence: "RESOLVED",
		})
	}
	for _, metadata := range []struct {
		id   string
		file string
	}{
		{id: "consumer:catalog", file: ".goregraph-workspace/agent/api-catalog.json"},
		{id: "consumer:dashboard", file: ".goregraph-dashboard.json"},
	} {
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: metadata.id, Project: "frontend/metadata", Kind: "api_consumer", Name: metadata.id,
			File: metadata.file, Line: 1, Summary: "consumer service metadata; auth unknown",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + metadata.id, FromFactID: metadata.id, ToFactID: "endpoint:orders", Kind: "consumes_endpoint",
		})
	}
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "goregraph-out/agent/context-index.json", string(body))
	return root
}

func assertCLIEndpointContextWireKeys(t *testing.T, body []byte) {
	t.Helper()
	var contextObject map[string]json.RawMessage
	if err := json.Unmarshal(body, &contextObject); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"endpoints", "budget_tokens", "fallback_required"} {
		if _, ok := contextObject[key]; !ok {
			t.Fatalf("CLI context JSON missing public key %q: %s", key, body)
		}
	}
	for _, alias := range []string{"endpoint", "Endpoints", "budgetTokens", "BudgetTokens", "fallbackRequired", "FallbackRequired"} {
		if _, ok := contextObject[alias]; ok {
			t.Fatalf("CLI context JSON contains alias key %q: %s", alias, body)
		}
	}

	var endpoints []map[string]json.RawMessage
	if err := json.Unmarshal(contextObject["endpoints"], &endpoints); err != nil || len(endpoints) != 1 {
		t.Fatalf("CLI endpoints JSON = %s, error = %v", contextObject["endpoints"], err)
	}
	endpoint := endpoints[0]
	for _, key := range []string{"http_method", "omitted_consumers"} {
		if _, ok := endpoint[key]; !ok {
			t.Fatalf("CLI endpoint JSON missing public key %q: %s", key, contextObject["endpoints"])
		}
	}
	for _, alias := range []string{"method", "httpMethod", "HTTPMethod", "omittedConsumers", "OmittedConsumers"} {
		if _, ok := endpoint[alias]; ok {
			t.Fatalf("CLI endpoint JSON contains alias key %q: %s", alias, contextObject["endpoints"])
		}
	}

	var consumers []map[string]json.RawMessage
	if err := json.Unmarshal(endpoint["consumers"], &consumers); err != nil || len(consumers) == 0 {
		t.Fatalf("CLI consumers JSON = %s, error = %v", endpoint["consumers"], err)
	}
	if _, ok := consumers[0]["authentication"]; !ok {
		t.Fatalf("CLI consumer JSON missing public key %q: %s", "authentication", endpoint["consumers"])
	}
	if _, ok := consumers[0]["auth"]; ok {
		t.Fatalf("CLI consumer JSON contains alias key %q: %s", "auth", endpoint["consumers"])
	}
	if _, ok := consumers[0]["Authentication"]; ok {
		t.Fatalf("CLI consumer JSON contains alias key %q: %s", "Authentication", endpoint["consumers"])
	}
}

func cliTestOutputPath(path string) string {
	dir, name := filepath.Dir(path), filepath.Base(path)
	if name == "manifest.json" {
		return path
	}
	parent := filepath.Base(dir)
	if parent != "goregraph-out" && parent != ".goregraph-workspace" {
		return path
	}
	if name == "agent-guide.md" {
		return filepath.Join(dir, "agent", name)
	}
	if strings.HasSuffix(name, ".json") {
		return filepath.Join(dir, "index", name)
	}
	return filepath.Join(dir, "dashboard", name)
}
