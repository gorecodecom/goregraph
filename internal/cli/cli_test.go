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
	matches, err := os.ReadFile(filepath.Join(frontend, "goregraph-out", "workspace-contract-matches.md"))
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
		filepath.Join(frontend, "goregraph-out", "workspace-contract-matches.json"),
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
	writeFile(t, before, "contract-matches.json", `[{"id":"a","api_http_method":"GET","api_path":"/a","issue":"matched","confidence":"RESOLVED"}]`)
	writeFile(t, before, "feature-flows.json", `[{"id":"flow-a","tests":[{"confidence":"MATCHED"}]}]`)
	writeFile(t, after, "contract-matches.json", `[{"id":"a","api_http_method":"GET","api_path":"/a","issue":"indexed_backend_route_missing","confidence":"UNRESOLVED"},{"id":"b","api_http_method":"POST","api_path":"/b","issue":"matched","confidence":"RESOLVED"}]`)
	writeFile(t, after, "feature-flows.json", `[{"id":"flow-a"}]`)
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

func TestRunQueryMissingIndexTellsUserToScan(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"query", root, "StartServer"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "goregraph scan") {
		t.Fatalf("stderr missing scan guidance:\n%s", stderr.String())
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
		"goregraph 1.2.0",
		"commit:",
		"built:",
		"go:",
		"platform:",
		"schema: 2",
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
	if err := os.WriteFile(filepath.Join(out, "workspace-map.html"), []byte("<html></html>"), 0o644); err != nil {
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
	dashboard := filepath.Join(workspace, ".goregraph-workspace", "workspace-map.html")
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
	if err := os.WriteFile(filepath.Join(out, "workspace-graph.json"), []byte(graph), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "feature-dossiers.json"), []byte(`[{"id":"feature:get-users","route":"GET /users/{userId}","source_flow_id":"flow:get-users","frontend_project":"frontend/app","backend_project":"services/ms-user"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "feature-flows.json"), []byte(`[{"id":"flow:get-users","frontend_project":"frontend/app","frontend_file":"src/api/users.ts","backend_project":"services/ms-user","backend_file":"UserController.java","http_method":"GET","path":"/users/{userId}"}]`), 0o644); err != nil {
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

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
