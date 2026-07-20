package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/gitupdate"
)

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
	cliGit(t, root, "init", "--bare", "--initial-branch=main", fixture.remote)
	cliGit(t, root, "clone", fixture.remote, fixture.work)
	cliGit(t, root, "clone", fixture.remote, fixture.peer)
	configureCLIGitIdentity(t, fixture.work)
	configureCLIGitIdentity(t, fixture.peer)

	writeFile(t, fixture.work, "README.md", "initial\n")
	cliGit(t, fixture.work, "add", "README.md")
	cliGit(t, fixture.work, "commit", "-m", "initial")
	cliGit(t, fixture.work, "push", "-u", "origin", "main")
	cliGit(t, fixture.work, "remote", "set-head", "origin", "-a")

	cliGit(t, fixture.peer, "fetch", "origin")
	cliGit(t, fixture.peer, "switch", "-c", "main", "--track", "origin/main")
	cliGit(t, fixture.peer, "remote", "set-head", "origin", "-a")
	return fixture
}

func (fixture cliGitFixture) commitAndPushFromPeer(t *testing.T, contents string) string {
	t.Helper()
	writeFile(t, fixture.peer, "README.md", contents)
	cliGit(t, fixture.peer, "add", "README.md")
	cliGit(t, fixture.peer, "commit", "-m", "peer update")
	cliGit(t, fixture.peer, "push", "origin", "main")
	return cliGit(t, fixture.peer, "rev-parse", "HEAD")
}

func configureCLIGitIdentity(t *testing.T, dir string) {
	t.Helper()
	cliGit(t, dir, "config", "user.name", "GoreGraph CLI Test")
	cliGit(t, dir, "config", "user.email", "goregraph-cli@example.invalid")
}

func cliGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, output)
	}
	return strings.TrimSpace(string(output))
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
		"Remote: " + fixture.remote,
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
	if result.Path != fixture.work || result.GitRoot != canonicalCLIPath(t, fixture.work) || result.Remote != fixture.remote {
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
