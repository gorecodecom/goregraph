package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		{"help"},
		{"workspace", "help"},
		{"workspace", "scan-all", "help"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, want 0; stderr=%s", args, code, stderr.String())
		}
		for _, want := range []string{".goregraph-workspace.yml", "--workspace"} {
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
	writeFile(t, root, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, filepath.Join(workspace, "microservices", "ms-cadaster"), "README.md", "# ms-cadaster\n")
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
	if !strings.Contains(stdout.String(), "Scanned 1 missing workspace project") || !strings.Contains(stdout.String(), "microservices/ms-task") {
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
	for _, project := range []string{"frontend/frontend-monorepo", "microservices/ms-cadaster", "microservices/ms-task"} {
		if !strings.Contains(stdout.String(), "Scanned `"+project+"`") {
			t.Fatalf("scan-all output missing %s:\n%s", project, stdout.String())
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
	var stdout, stderr bytes.Buffer

	code := Run([]string{"mcp", "help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: goregraph mcp") {
		t.Fatalf("mcp help missing usage:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "stdio") {
		t.Fatalf("mcp help missing stdio note:\n%s", stdout.String())
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
