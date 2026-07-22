package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gorecodecom/goregraph/internal/gitupdate"
	"github.com/gorecodecom/goregraph/internal/scan"
)

var cliGitFixtureTemplate struct {
	sync.Once
	root string
	err  error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if cliGitFixtureTemplate.root != "" {
		_ = os.RemoveAll(cliGitFixtureTemplate.root)
	}
	os.Exit(code)
}

type cliGitFixture struct {
	remote string
	work   string
	peer   string
}

func newCLIGitFixture(t *testing.T) cliGitFixture {
	t.Helper()
	return newCLIGitFixtureAt(t, filepath.Join(t.TempDir(), "work"))
}

func newCLIGitFixtureAt(t *testing.T, work string) cliGitFixture {
	t.Helper()

	root := t.TempDir()
	fixture := cliGitFixture{
		remote: filepath.Join(root, "remote.git"),
		work:   work,
		peer:   filepath.Join(root, "peer"),
	}
	if err := os.MkdirAll(filepath.Dir(work), 0o755); err != nil {
		t.Fatalf("create work parent: %v", err)
	}
	templateRoot := cliGitFixtureTemplateRoot(t)
	for _, directory := range []struct {
		source string
		target string
	}{
		{source: filepath.Join(templateRoot, "remote.git"), target: fixture.remote},
		{source: filepath.Join(templateRoot, "work"), target: fixture.work},
		{source: filepath.Join(templateRoot, "peer"), target: fixture.peer},
	} {
		if err := os.CopyFS(directory.target, os.DirFS(directory.source)); err != nil {
			t.Fatalf("copy CLI Git fixture %s: %v", filepath.Base(directory.source), err)
		}
	}
	rewriteCLIFixtureOrigin(t, fixture.work, fixture.remote)
	rewriteCLIFixtureOrigin(t, fixture.peer, fixture.remote)
	return fixture
}

func cliGitFixtureTemplateRoot(t *testing.T) string {
	t.Helper()
	cliGitFixtureTemplate.Do(func() {
		cliGitFixtureTemplate.root, cliGitFixtureTemplate.err = createCLIGitFixtureTemplate()
	})
	if cliGitFixtureTemplate.err != nil {
		t.Fatalf("create CLI Git fixture template: %v", cliGitFixtureTemplate.err)
	}
	return cliGitFixtureTemplate.root
}

func createCLIGitFixtureTemplate() (root string, resultErr error) {
	root, err := os.MkdirTemp("", "goregraph-cli-git-fixture-")
	if err != nil {
		return "", err
	}
	defer func() {
		if resultErr != nil {
			_ = os.RemoveAll(root)
		}
	}()

	remote := filepath.Join(root, "remote.git")
	work := filepath.Join(root, "work")
	peer := filepath.Join(root, "peer")
	for _, command := range []struct {
		dir  string
		args []string
	}{
		{dir: root, args: []string{"init", "--bare", "--initial-branch=main", remote}},
		{dir: root, args: []string{"init", "--initial-branch=main", work}},
		{dir: work, args: []string{"remote", "add", "-m", "main", "origin", "../remote.git"}},
	} {
		if _, err := runCLIFixtureGit(command.dir, command.args...); err != nil {
			return root, err
		}
	}
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("initial\n"), 0o600); err != nil {
		return root, err
	}
	for _, command := range [][]string{
		{"add", "README.md"},
		{"commit", "-m", "initial"},
		{"push", "-u", "origin", "main"},
	} {
		if _, err := runCLIFixtureGit(work, command...); err != nil {
			return root, err
		}
	}
	if _, err := runCLIFixtureGit(root, "clone", "remote.git", "peer"); err != nil {
		return root, err
	}
	if _, err := runCLIFixtureGit(peer, "remote", "set-url", "origin", "../remote.git"); err != nil {
		return root, err
	}
	return root, nil
}

func rewriteCLIFixtureOrigin(t *testing.T, repository, remote string) {
	t.Helper()
	configPath := filepath.Join(repository, ".git", "config")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read CLI Git fixture config: %v", err)
	}
	updated := strings.Replace(string(contents), "../remote.git", filepath.ToSlash(remote), 1)
	if updated == string(contents) {
		t.Fatalf("CLI Git fixture config has no relative origin: %s", configPath)
	}
	if err := os.WriteFile(configPath, []byte(updated), 0o600); err != nil {
		t.Fatalf("write CLI Git fixture config: %v", err)
	}
}

func (fixture cliGitFixture) commitAndPushFromPeer(t *testing.T, contents string) string {
	t.Helper()
	writeFile(t, fixture.peer, "README.md", contents)
	cliGit(t, fixture.peer, "add", "README.md")
	cliGit(t, fixture.peer, "commit", "-m", "peer update")
	cliGit(t, fixture.peer, "push", "origin", "main")
	return cliGit(t, fixture.peer, "rev-parse", "HEAD")
}

func cliGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := runCLIFixtureGit(dir, args...)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return output
}

func runCLIFixtureGit(dir string, args ...string) (string, error) {
	gitArgs := append([]string{"-c", "core.autocrlf=false", "-c", "core.hooksPath="}, args...)
	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_AUTHOR_NAME=GoreGraph CLI Test",
		"GIT_AUTHOR_EMAIL=goregraph-cli@example.invalid",
		"GIT_COMMITTER_NAME=GoreGraph CLI Test",
		"GIT_COMMITTER_EMAIL=goregraph-cli@example.invalid",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w\n%s", strings.Join(args, " "), dir, err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func canonicalCLIPath(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("absolute path for %s: %v", path, err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		t.Fatalf("canonical path for %s: %v", path, err)
	}
	return filepath.Clean(canonical)
}

func TestRunGitUpdateDefaultsToPreview(t *testing.T) {
	fixture := newCLIGitFixture(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"git", "update", fixture.work}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	for _, expected := range []string{
		"Mode: preview",
		"Git root: " + canonicalCLIPath(t, fixture.work),
		"Remote: " + filepath.ToSlash(fixture.remote),
		"Branch before: main",
		"Branch after: main",
		"Commit before:",
		"Commit after:",
		"Status: up_to_date",
		"Reason:",
		"Remediation: -",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("text output missing %q:\n%s", expected, stdout.String())
		}
	}
}

func TestRunGitUpdateExecuteUpdatesRepository(t *testing.T) {
	fixture := newCLIGitFixture(t)
	wantCommit := fixture.commitAndPushFromPeer(t, "updated\n")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"git", "update", "--execute", fixture.work}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if got := cliGit(t, fixture.work, "rev-parse", "HEAD"); got != wantCommit {
		t.Fatalf("HEAD = %q, want %q", got, wantCommit)
	}
	for _, expected := range []string{"Mode: execute", "Status: updated", "Executed: true"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("text output missing %q:\n%s", expected, stdout.String())
		}
	}
}

func TestRunGitUpdateJSONMatchesStructuredResult(t *testing.T) {
	fixture := newCLIGitFixture(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"git", "update", "--format", "json", fixture.work}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	var report gitupdate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON report: %v\n%s", err, stdout.String())
	}
	if report.Mode != gitupdate.ModePreview {
		t.Fatalf("Mode = %q, want %q", report.Mode, gitupdate.ModePreview)
	}
	if len(report.Repositories) != 1 {
		t.Fatalf("len(Repositories) = %d, want 1", len(report.Repositories))
	}
	result := report.Repositories[0]
	if result.Path != fixture.work || result.GitRoot != canonicalCLIPath(t, fixture.work) || result.Remote != filepath.ToSlash(fixture.remote) {
		t.Fatalf("repository identity = %#v", result)
	}
	if result.BranchBefore != "main" || result.BranchAfter != "main" {
		t.Fatalf("branches = %q -> %q, want main -> main", result.BranchBefore, result.BranchAfter)
	}
	if result.CommitBefore == "" || result.CommitAfter != result.CommitBefore {
		t.Fatalf("commits = %q -> %q, want the same non-empty commit", result.CommitBefore, result.CommitAfter)
	}
	if result.Status != gitupdate.StatusUpToDate || result.Reason == "" || result.Remediation != "" || result.Executed {
		t.Fatalf("structured result = %#v", result)
	}
	if report.Summary[gitupdate.StatusUpToDate] != 1 {
		t.Fatalf("Summary = %#v, want one up_to_date result", report.Summary)
	}
}

func TestRunGitUpdateRejectsUnknownFormatAndOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing subcommand", args: []string{"git"}},
		{name: "unknown subcommand", args: []string{"git", "status"}},
		{name: "multiple paths", args: []string{"git", "update", "first", "second"}},
		{name: "missing format", args: []string{"git", "update", "--format"}},
		{name: "unknown format", args: []string{"git", "update", "--format", "yaml"}},
		{name: "unknown option", args: []string{"git", "update", "--force"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(test.args, &stdout, &stderr); code != 2 {
				t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("stderr is empty, want a usage error")
			}
		})
	}
}

func TestRunGitUpdateHelpDocumentsExecuteAndFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"git", "update", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	for _, expected := range []string{"Usage: goregraph git update", "preview", "--execute", "--format text|json"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("help output missing %q:\n%s", expected, stdout.String())
		}
	}

	stdout.Reset()
	if code := Run([]string{"help", "--all"}, &stdout, &stderr); code != 0 {
		t.Fatalf("global help exit code = %d, want 0", code)
	}
	if preview := strings.Index(stdout.String(), "goregraph git update .\n"); preview < 0 {
		t.Fatalf("global help missing preview example:\n%s", stdout.String())
	} else if execute := strings.Index(stdout.String(), "goregraph git update . --execute"); execute < preview {
		t.Fatalf("global help must show preview before execute:\n%s", stdout.String())
	}
}

type cliWorkspaceFixture struct {
	workspace string
	outer     cliGitFixture
	nested    cliGitFixture
}

func newCLIWorkspaceFixture(t *testing.T) cliWorkspaceFixture {
	t.Helper()

	workspace := filepath.Join(t.TempDir(), "weka")
	outer := newCLIGitFixtureAt(t, workspace)
	writeFile(t, workspace, "frontend/frontend-monorepo/package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, workspace, "microservices/ms-cadaster/pom.xml", "<project/>\n")
	nested := newCLIGitFixtureAt(t, filepath.Join(workspace, "microservices", "ms-task"))
	writeFile(t, nested.work, "go.mod", "module example.invalid/ms-task\n")
	cliGit(t, nested.work, "add", "go.mod")
	cliGit(t, nested.work, "commit", "-m", "add project marker")
	cliGit(t, nested.work, "push", "origin", "main")
	cliGit(t, nested.peer, "fetch", "origin")
	cliGit(t, nested.peer, "reset", "--hard", "origin/main")

	cliGit(t, workspace, "add", ".")
	cliGit(t, workspace, "commit", "-m", "add workspace projects")
	cliGit(t, workspace, "push", "origin", "main")
	return cliWorkspaceFixture{workspace: workspace, outer: outer, nested: nested}
}

func runWorkspaceGitJSON(t *testing.T, fixture cliWorkspaceFixture, execute bool) (int, gitupdate.Report, string) {
	t.Helper()
	args := []string{"workspace", "git", "update", fixture.workspace, "--format", "json"}
	if execute {
		args = append(args, "--execute")
	}
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	var report gitupdate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode workspace JSON report: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	return code, report, stderr.String()
}

func TestWorkspaceGitTargetsDiscoversNestedRepositories(t *testing.T) {
	workspace := t.TempDir()
	scanProject := filepath.Join(workspace, "projects", "service")
	linkedRepository := filepath.Join(workspace, "linked")
	for _, directory := range []string{
		filepath.Join(workspace, ".git"),
		filepath.Join(workspace, "nested", ".git"),
		filepath.Join(workspace, ".worktrees", "checkout", ".git"),
		filepath.Join(workspace, "node_modules", "dependency", ".git"),
		scanProject,
		linkedRepository,
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatalf("create %s: %v", directory, err)
		}
	}
	if err := os.WriteFile(filepath.Join(linkedRepository, ".git"), []byte("gitdir: ../metadata\n"), 0o600); err != nil {
		t.Fatalf("create regular .git file: %v", err)
	}
	for _, repository := range []string{workspace, filepath.Join(workspace, "nested"), linkedRepository} {
		for _, marker := range []string{"package.json", "pom.xml", "go.mod", "goregraph.yml"} {
			if _, err := os.Stat(filepath.Join(repository, marker)); !os.IsNotExist(err) {
				t.Fatalf("Git-only repository %s unexpectedly has marker %s: %v", repository, marker, err)
			}
		}
	}

	plan := scan.WorkspaceProjectScanPlanRecord{
		WorkspaceRoot: filepath.ToSlash(workspace),
		Items: []scan.WorkspaceProjectScanItemRecord{
			{AbsPath: filepath.ToSlash(scanProject)},
		},
	}
	targets, err := workspaceGitTargets(plan)
	if err != nil {
		t.Fatalf("workspace Git targets: %v", err)
	}

	want := []string{
		filepath.ToSlash(workspace),
		filepath.ToSlash(linkedRepository),
		filepath.ToSlash(filepath.Join(workspace, "nested")),
		filepath.ToSlash(scanProject),
	}
	if len(targets) != len(want) {
		t.Fatalf("targets = %#v, want paths %#v", targets, want)
	}
	for index, target := range targets {
		if target.Path != want[index] {
			t.Fatalf("targets[%d].Path = %q, want %q; targets=%#v", index, target.Path, want[index], targets)
		}
	}
}

func TestRunWorkspaceGitUpdateDeduplicatesRepositories(t *testing.T) {
	fixture := newCLIWorkspaceFixture(t)

	code, report, stderr := runWorkspaceGitJSON(t, fixture, false)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}
	if report.WorkspaceRoot != filepath.ToSlash(fixture.workspace) {
		t.Fatalf("WorkspaceRoot = %q, want %q", report.WorkspaceRoot, filepath.ToSlash(fixture.workspace))
	}
	if len(report.Repositories) != 2 {
		t.Fatalf("len(Repositories) = %d, want 2; report=%#v", len(report.Repositories), report)
	}
	wantOuterRoot := canonicalCLIPath(t, fixture.outer.work)
	wantNestedRoot := canonicalCLIPath(t, fixture.nested.work)
	if report.Repositories[0].GitRoot != wantOuterRoot || report.Repositories[1].GitRoot != wantNestedRoot {
		t.Fatalf("Git roots = %q, %q; want %q, %q", report.Repositories[0].GitRoot, report.Repositories[1].GitRoot, wantOuterRoot, wantNestedRoot)
	}
}

func TestRunWorkspaceGitUpdateContinuesAfterDirtyRepository(t *testing.T) {
	fixture := newCLIWorkspaceFixture(t)
	writeFile(t, fixture.outer.work, "dirty.txt", "dirty\n")
	wantNestedCommit := fixture.nested.commitAndPushFromPeer(t, "updated nested\n")

	code, report, stderr := runWorkspaceGitJSON(t, fixture, true)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr)
	}
	if len(report.Repositories) != 2 {
		t.Fatalf("len(Repositories) = %d, want 2", len(report.Repositories))
	}
	if report.Repositories[0].Status != gitupdate.StatusDirty || report.Repositories[1].Status != gitupdate.StatusUpdated {
		t.Fatalf("statuses = %q, %q; want dirty, updated", report.Repositories[0].Status, report.Repositories[1].Status)
	}
	if got := cliGit(t, fixture.nested.work, "rev-parse", "HEAD"); got != wantNestedCommit {
		t.Fatalf("nested HEAD = %q, want %q", got, wantNestedCommit)
	}
}

func TestRunWorkspaceGitUpdateJSONReportsPartialSuccess(t *testing.T) {
	fixture := newCLIWorkspaceFixture(t)
	writeFile(t, fixture.outer.work, "dirty.txt", "dirty\n")

	code, report, stderr := runWorkspaceGitJSON(t, fixture, false)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr)
	}
	if len(report.Repositories) != 2 {
		t.Fatalf("len(Repositories) = %d, want 2", len(report.Repositories))
	}
	if report.Summary[gitupdate.StatusDirty] != 1 || report.Summary[gitupdate.StatusUpToDate] != 1 {
		t.Fatalf("Summary = %#v, want one dirty and one up_to_date", report.Summary)
	}
	if report.Repositories[0].Reason == "" || report.Repositories[0].Remediation == "" {
		t.Fatalf("dirty result lacks reason or remediation: %#v", report.Repositories[0])
	}
}

func TestRunWorkspaceGitUpdateHelpDocumentsPreview(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"workspace", "git", "update", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	for _, expected := range []string{"Usage: goregraph workspace git update", "preview", "--execute", "--format text|json"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("help output missing %q:\n%s", expected, stdout.String())
		}
	}

	stdout.Reset()
	if code := Run([]string{"workspace", "--help", "--all"}, &stdout, &stderr); code != 0 {
		t.Fatalf("workspace help exit code = %d, want 0", code)
	}
	if preview := strings.Index(stdout.String(), "goregraph workspace git update .\n"); preview < 0 {
		t.Fatalf("workspace help missing preview example:\n%s", stdout.String())
	} else if execute := strings.Index(stdout.String(), "goregraph workspace git update . --execute"); execute < preview {
		t.Fatalf("workspace help must show preview before execute:\n%s", stdout.String())
	}
}
