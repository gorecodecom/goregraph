package gitupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("GOREGRAPH_TEST_SSH_ARGUMENTS") != "" && filepath.Base(os.Args[0]) == "ssh" {
		recordSSHInvocation()
	}
	if os.Getenv("GOREGRAPH_TEST_GIT_LOG") != "" && filepath.Base(os.Args[0]) == "git" {
		runGitLoggingProcess()
	}
	if os.Getenv("GOREGRAPH_TEST_GIT_BARRIER_DIR") != "" && filepath.Base(os.Args[0]) == "git" {
		runGitBarrierProcess()
	}
	for _, environment := range []string{
		"GOREGRAPH_TEST_FSMONITOR_MARKER",
		"GOREGRAPH_TEST_GIT_COMMAND_MARKER",
		"GOREGRAPH_TEST_LAZY_FETCH_MARKER",
	} {
		if marker := os.Getenv(environment); marker != "" {
			_ = os.WriteFile(marker, []byte("invoked\n"), 0o600)
			os.Exit(97)
		}
	}
	os.Exit(m.Run())
}

func recordSSHInvocation() {
	contents := strings.Join(os.Args[1:], "\n") + "\n" +
		"GIT_TERMINAL_PROMPT=" + os.Getenv("GIT_TERMINAL_PROMPT") + "\n" +
		"GCM_INTERACTIVE=" + os.Getenv("GCM_INTERACTIVE") + "\n"
	if err := os.WriteFile(os.Getenv("GOREGRAPH_TEST_SSH_ARGUMENTS"), []byte(contents), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(98)
	}
	os.Exit(97)
}

func runGitLoggingProcess() {
	log, err := os.OpenFile(os.Getenv("GOREGRAPH_TEST_GIT_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(98)
	}
	if _, err := fmt.Fprintln(log, strings.Join(os.Args[1:], "\x1f")); err != nil {
		_ = log.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(98)
	}
	if err := log.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(98)
	}

	realGit := os.Getenv("GOREGRAPH_TEST_REAL_GIT")
	cmd := exec.Command(realGit, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(98)
	}
	os.Exit(0)
}

func runGitBarrierProcess() {
	realGit := os.Getenv("GOREGRAPH_TEST_REAL_GIT")
	cmd := exec.Command(realGit, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(98)
	}
	if !containsArgument(os.Args[1:], "fetch") {
		os.Exit(0)
	}

	barrierDirectory := os.Getenv("GOREGRAPH_TEST_GIT_BARRIER_DIR")
	if err := os.WriteFile(filepath.Join(barrierDirectory, "ready"), []byte("ready\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(98)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(barrierDirectory, "release")); err == nil {
			break
		} else if !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "timed out waiting for fetch barrier release")
			os.Exit(98)
		}
		time.Sleep(time.Millisecond)
	}
	os.Exit(0)
}

func containsArgument(arguments []string, expected string) bool {
	for _, argument := range arguments {
		if argument == expected {
			return true
		}
	}
	return false
}

func TestReportExitCode(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected int
	}{
		{name: "preview", status: StatusWouldUpdate, expected: 0},
		{name: "current", status: StatusUpToDate, expected: 0},
		{name: "updated", status: StatusUpdated, expected: 0},
		{name: "blocked", status: StatusDirty, expected: 1},
		{name: "failed", status: StatusFetchFailed, expected: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := Report{Repositories: []RepositoryResult{{Status: test.status}}}
			if actual := report.ExitCode(); actual != test.expected {
				t.Fatalf("ExitCode() = %d, want %d", actual, test.expected)
			}
		})
	}
}

type gitFixture struct {
	remote string
	work   string
	peer   string
}

func newGitFixture(t *testing.T) gitFixture {
	t.Helper()

	root := t.TempDir()
	fixture := gitFixture{
		remote: filepath.Join(root, "remote.git"),
		work:   filepath.Join(root, "work"),
		peer:   filepath.Join(root, "peer"),
	}
	git(t, root, "init", "--bare", "--initial-branch=main", fixture.remote)
	git(t, root, "clone", fixture.remote, fixture.work)
	git(t, root, "clone", fixture.remote, fixture.peer)
	configureIdentity(t, fixture.work)
	configureIdentity(t, fixture.peer)

	writeFile(t, filepath.Join(fixture.work, "README.md"), "initial\n")
	git(t, fixture.work, "add", "README.md")
	git(t, fixture.work, "commit", "-m", "initial")
	git(t, fixture.work, "push", "-u", "origin", "main")
	git(t, fixture.work, "remote", "set-head", "origin", "-a")

	git(t, fixture.peer, "fetch", "origin")
	git(t, fixture.peer, "switch", "-c", "main", "--track", "origin/main")
	git(t, fixture.peer, "remote", "set-head", "origin", "-a")

	return fixture
}

func configureIdentity(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "config", "user.name", "GoreGraph Test")
	git(t, dir, "config", "user.email", "goregraph@example.invalid")
}

func git(t *testing.T, dir string, args ...string) string {
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

func (f gitFixture) commitAndPushFromPeer(t *testing.T, contents string) string {
	t.Helper()

	writeFile(t, filepath.Join(f.peer, "README.md"), contents)
	git(t, f.peer, "add", "README.md")
	git(t, f.peer, "commit", "-m", "peer update")
	git(t, f.peer, "push", "origin", "main")
	return revParse(t, f.peer, "HEAD")
}

func revParse(t *testing.T, dir, revision string) string {
	t.Helper()
	return git(t, dir, "rev-parse", revision)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func preview(t *testing.T, targets ...Target) Report {
	t.Helper()

	report, err := Run(context.Background(), targets, Options{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return report
}

func execute(t *testing.T, targets ...Target) Report {
	t.Helper()

	report, err := Run(context.Background(), targets, Options{Execute: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return report
}

func onlyResult(t *testing.T, report Report) RepositoryResult {
	t.Helper()
	if len(report.Repositories) != 1 {
		t.Fatalf("len(Repositories) = %d, want 1", len(report.Repositories))
	}
	return report.Repositories[0]
}

func TestRunDeduplicatesCanonicalGitRoots(t *testing.T) {
	fixture := newGitFixture(t)
	firstNestedPath := filepath.Join(fixture.work, "projects", "alpha")
	secondNestedPath := filepath.Join(fixture.work, "projects", "zeta")
	for _, path := range []string{firstNestedPath, secondNestedPath} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("create nested project %s: %v", path, err)
		}
	}

	report := preview(t,
		Target{Path: secondNestedPath},
		Target{Path: firstNestedPath},
		Target{Path: fixture.work},
	)

	result := onlyResult(t, report)
	if result.Path != fixture.work {
		t.Fatalf("Path = %q, want lexicographically first requested path %q", result.Path, fixture.work)
	}
	if result.Status != StatusUpToDate {
		t.Fatalf("Status = %q, want %q", result.Status, StatusUpToDate)
	}
	if report.Summary[StatusUpToDate] != 1 || len(report.Summary) != 1 {
		t.Fatalf("Summary = %#v, want one up_to_date repository", report.Summary)
	}
}

func TestRunProcessesSafeRepositoriesAfterBlocker(t *testing.T) {
	dirtyFixture := newGitFixture(t)
	updatedFixture := newGitFixture(t)
	writeFile(t, filepath.Join(dirtyFixture.work, "untracked.txt"), "dirty\n")
	dirtyHead := revParse(t, dirtyFixture.work, "HEAD")
	targetCommit := updatedFixture.commitAndPushFromPeer(t, "peer update\n")
	workspaceRoot := t.TempDir()

	report, err := Run(context.Background(), []Target{
		{Path: dirtyFixture.work},
		{Path: updatedFixture.work},
	}, Options{Execute: true, WorkspaceRoot: workspaceRoot})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.WorkspaceRoot != workspaceRoot {
		t.Fatalf("WorkspaceRoot = %q, want %q", report.WorkspaceRoot, workspaceRoot)
	}
	if len(report.Repositories) != 2 {
		t.Fatalf("len(Repositories) = %d, want 2", len(report.Repositories))
	}

	resultsByPath := make(map[string]RepositoryResult, len(report.Repositories))
	for _, result := range report.Repositories {
		resultsByPath[result.Path] = result
	}
	dirtyResult := resultsByPath[dirtyFixture.work]
	if dirtyResult.Status != StatusDirty || dirtyResult.Executed {
		t.Fatalf("dirty result = %#v, want non-executed dirty blocker", dirtyResult)
	}
	updatedResult := resultsByPath[updatedFixture.work]
	if updatedResult.Status != StatusUpdated || !updatedResult.Executed {
		t.Fatalf("safe result = %#v, want executed update", updatedResult)
	}
	if report.Summary[StatusDirty] != 1 || report.Summary[StatusUpdated] != 1 {
		t.Fatalf("Summary = %#v, want one dirty and one updated", report.Summary)
	}
	if report.ExitCode() != 1 {
		t.Fatalf("ExitCode() = %d, want 1 for partial success", report.ExitCode())
	}
	if head := revParse(t, dirtyFixture.work, "HEAD"); head != dirtyHead {
		t.Fatalf("dirty repository HEAD = %s, want unchanged %s", head, dirtyHead)
	}
	if head := revParse(t, updatedFixture.work, "HEAD"); head != targetCommit {
		t.Fatalf("safe repository HEAD = %s, want updated %s", head, targetCommit)
	}
}

func TestRunSortsRepositoryResultsByCanonicalRoot(t *testing.T) {
	firstFixture := newGitFixture(t)
	secondFixture := newGitFixture(t)
	expectedRoots := []string{
		canonicalTestPath(t, firstFixture.work),
		canonicalTestPath(t, secondFixture.work),
	}
	sort.Strings(expectedRoots)

	report := preview(t,
		Target{Path: secondFixture.work},
		Target{Path: firstFixture.work},
	)
	if len(report.Repositories) != len(expectedRoots) {
		t.Fatalf("len(Repositories) = %d, want %d", len(report.Repositories), len(expectedRoots))
	}
	for index, expectedRoot := range expectedRoots {
		if actualRoot := report.Repositories[index].GitRoot; actualRoot != expectedRoot {
			t.Fatalf("Repositories[%d].GitRoot = %q, want %q", index, actualRoot, expectedRoot)
		}
	}
}

func TestRunProcessesSafeRepositoryAfterInspectionFailure(t *testing.T) {
	fixture := newGitFixture(t)
	brokenRepository := filepath.Join(filepath.Dir(fixture.work), "a-broken")
	git(t, filepath.Dir(brokenRepository), "init", "--initial-branch=main", brokenRepository)
	targetCommit := fixture.commitAndPushFromPeer(t, "peer update\n")

	report, err := Run(context.Background(), []Target{
		{Path: fixture.work},
		{Path: brokenRepository},
	}, Options{Execute: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(report.Repositories) != 2 {
		t.Fatalf("len(Repositories) = %d, want 2", len(report.Repositories))
	}

	failed := report.Repositories[0]
	if failed.Path != brokenRepository || failed.GitRoot != canonicalTestPath(t, brokenRepository) {
		t.Fatalf("failed repository identity = path %q, root %q; want %q", failed.Path, failed.GitRoot, brokenRepository)
	}
	if failed.Status != StatusFetchFailed || failed.Executed {
		t.Fatalf("failed result = %#v, want unexecuted fetch_failed", failed)
	}
	if !strings.Contains(failed.Reason, "rev-parse HEAD") || failed.Remediation == "" {
		t.Fatalf("failed result reason/remediation = %q / %q, want precise inspection failure", failed.Reason, failed.Remediation)
	}

	updated := report.Repositories[1]
	if updated.Path != fixture.work || updated.Status != StatusUpdated || !updated.Executed {
		t.Fatalf("safe result = %#v, want executed update after inspection failure", updated)
	}
	if updated.CommitAfter != targetCommit || revParse(t, fixture.work, "HEAD") != targetCommit {
		t.Fatalf("safe repository was not updated to %s: %#v", targetCommit, updated)
	}
	if report.Summary[StatusFetchFailed] != 1 || report.Summary[StatusUpdated] != 1 {
		t.Fatalf("Summary = %#v, want one fetch_failed and one updated", report.Summary)
	}
	if report.ExitCode() != 1 {
		t.Fatalf("ExitCode() = %d, want 1 for partial success", report.ExitCode())
	}
}

func TestRunPropagatesGitResolverInfrastructureFailure(t *testing.T) {
	path := t.TempDir()
	t.Setenv("PATH", t.TempDir())

	report, err := Run(context.Background(), []Target{{Path: path}}, Options{})
	if err == nil {
		t.Fatalf("Run() error = nil, want Git process infrastructure error; report: %#v", report)
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("Run() error = %v, want errors.Is(exec.ErrNotFound)", err)
	}
	if len(report.Repositories) != 0 || len(report.Summary) != 0 {
		t.Fatalf("report = %#v, want no repository classification after global resolver failure", report)
	}
}

func TestRunDeduplicatesSymlinkAliases(t *testing.T) {
	fixture := newGitFixture(t)
	alias := filepath.Join(t.TempDir(), "checkout-alias")
	if err := os.Symlink(fixture.work, alias); err != nil {
		t.Fatalf("create checkout symlink: %v", err)
	}
	expectedPaths := []string{fixture.work, alias}
	sort.Strings(expectedPaths)

	result := onlyResult(t, preview(t,
		Target{Path: fixture.work},
		Target{Path: alias},
	))
	if result.Path != expectedPaths[0] {
		t.Fatalf("Path = %q, want lexicographically first alias %q", result.Path, expectedPaths[0])
	}
	if result.GitRoot != canonicalTestPath(t, fixture.work) {
		t.Fatalf("GitRoot = %q, want canonical checkout root %q", result.GitRoot, canonicalTestPath(t, fixture.work))
	}
}

func canonicalTestPath(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("make %s absolute: %v", path, err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		t.Fatalf("canonicalize %s: %v", path, err)
	}
	return filepath.Clean(canonical)
}

func TestRunPreviewReportsWouldUpdateWithoutChangingRepository(t *testing.T) {
	fixture := newGitFixture(t)
	targetCommit := fixture.commitAndPushFromPeer(t, "peer update\n")
	git(t, fixture.work, "fetch", "origin")

	before := map[string]string{
		"head":       revParse(t, fixture.work, "HEAD"),
		"branch":     git(t, fixture.work, "symbolic-ref", "--short", "HEAD"),
		"remote_ref": revParse(t, fixture.work, "refs/remotes/origin/main"),
		"status":     git(t, fixture.work, "status", "--porcelain=v1", "--untracked-files=all"),
		"contents":   readFile(t, filepath.Join(fixture.work, "README.md")),
	}

	report := preview(t, Target{Path: fixture.work})
	result := onlyResult(t, report)
	if report.Mode != ModePreview {
		t.Fatalf("Mode = %q, want %q", report.Mode, ModePreview)
	}
	if result.Status != StatusWouldUpdate {
		t.Fatalf("Status = %q, want %q", result.Status, StatusWouldUpdate)
	}
	if result.Path != fixture.work {
		t.Fatalf("Path = %q, want %q", result.Path, fixture.work)
	}
	if result.BranchBefore != "main" || result.BranchAfter != "main" {
		t.Fatalf("branches = %q -> %q, want main -> main", result.BranchBefore, result.BranchAfter)
	}
	if result.CommitBefore != before["head"] || result.CommitAfter != targetCommit {
		t.Fatalf("commits = %q -> %q, want %q -> %q", result.CommitBefore, result.CommitAfter, before["head"], targetCommit)
	}
	if result.Executed {
		t.Fatal("Executed = true, want false")
	}
	if report.Summary[StatusWouldUpdate] != 1 {
		t.Fatalf("Summary = %#v, want one would_update", report.Summary)
	}

	after := map[string]string{
		"head":       revParse(t, fixture.work, "HEAD"),
		"branch":     git(t, fixture.work, "symbolic-ref", "--short", "HEAD"),
		"remote_ref": revParse(t, fixture.work, "refs/remotes/origin/main"),
		"status":     git(t, fixture.work, "status", "--porcelain=v1", "--untracked-files=all"),
		"contents":   readFile(t, filepath.Join(fixture.work, "README.md")),
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("repository changed during preview:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}

func TestRunPreviewFallsBackToUnambiguousMain(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")

	result := onlyResult(t, preview(t, Target{Path: fixture.work}))
	if result.Status != StatusUpToDate {
		t.Fatalf("Status = %q, want %q", result.Status, StatusUpToDate)
	}
	if result.BranchAfter != "main" {
		t.Fatalf("BranchAfter = %q, want main", result.BranchAfter)
	}
}

func TestRunPreviewBlocksDirtyTrackedAndUntrackedFiles(t *testing.T) {
	tests := []struct {
		name  string
		dirty func(*testing.T, gitFixture)
	}{
		{
			name: "tracked",
			dirty: func(t *testing.T, fixture gitFixture) {
				writeFile(t, filepath.Join(fixture.work, "README.md"), "dirty\n")
			},
		},
		{
			name: "untracked",
			dirty: func(t *testing.T, fixture gitFixture) {
				writeFile(t, filepath.Join(fixture.work, "untracked.txt"), "dirty\n")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGitFixture(t)
			test.dirty(t, fixture)

			result := onlyResult(t, preview(t, Target{Path: fixture.work}))
			if result.Status != StatusDirty {
				t.Fatalf("Status = %q, want %q", result.Status, StatusDirty)
			}
			if result.Remediation == "" {
				t.Fatal("Remediation is empty")
			}
		})
	}
}

func TestRunPreviewBlocksDetachedHead(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "switch", "--detach")

	result := onlyResult(t, preview(t, Target{Path: fixture.work}))
	if result.Status != StatusDetachedHead {
		t.Fatalf("Status = %q, want %q", result.Status, StatusDetachedHead)
	}
}

func TestRunPreviewBlocksOperationInProgress(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		directory bool
	}{
		{name: "merge", operation: "MERGE_HEAD"},
		{name: "cherry pick", operation: "CHERRY_PICK_HEAD"},
		{name: "revert", operation: "REVERT_HEAD"},
		{name: "rebase apply", operation: "rebase-apply", directory: true},
		{name: "rebase merge", operation: "rebase-merge", directory: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGitFixture(t)
			gitDir := git(t, fixture.work, "rev-parse", "--absolute-git-dir")
			operationPath := filepath.Join(gitDir, test.operation)
			if test.directory {
				if err := os.Mkdir(operationPath, 0o700); err != nil {
					t.Fatalf("create operation directory: %v", err)
				}
			} else {
				writeFile(t, operationPath, revParse(t, fixture.work, "HEAD")+"\n")
			}

			result := onlyResult(t, preview(t, Target{Path: fixture.work}))
			if result.Status != StatusOperationProgress {
				t.Fatalf("Status = %q, want %q", result.Status, StatusOperationProgress)
			}
		})
	}
}

func TestRunPreviewBlocksAheadAndDivergedBranches(t *testing.T) {
	t.Run("ahead", func(t *testing.T) {
		fixture := newGitFixture(t)
		writeFile(t, filepath.Join(fixture.work, "README.md"), "local\n")
		git(t, fixture.work, "add", "README.md")
		git(t, fixture.work, "commit", "-m", "local update")

		result := onlyResult(t, preview(t, Target{Path: fixture.work}))
		if result.Status != StatusAhead {
			t.Fatalf("Status = %q, want %q", result.Status, StatusAhead)
		}
	})

	t.Run("diverged", func(t *testing.T) {
		fixture := newGitFixture(t)
		writeFile(t, filepath.Join(fixture.work, "local.txt"), "local\n")
		git(t, fixture.work, "add", "local.txt")
		git(t, fixture.work, "commit", "-m", "local update")
		fixture.commitAndPushFromPeer(t, "peer update\n")
		git(t, fixture.work, "fetch", "origin")

		result := onlyResult(t, preview(t, Target{Path: fixture.work}))
		if result.Status != StatusDiverged {
			t.Fatalf("Status = %q, want %q", result.Status, StatusDiverged)
		}
	})
}

func TestRunPreviewReportsMissingRemoteAndUnknownDefault(t *testing.T) {
	t.Run("missing remote", func(t *testing.T) {
		fixture := newGitFixture(t)
		git(t, fixture.work, "remote", "remove", "origin")

		result := onlyResult(t, preview(t, Target{Path: fixture.work}))
		if result.Status != StatusMissingRemote {
			t.Fatalf("Status = %q, want %q", result.Status, StatusMissingRemote)
		}
	})

	t.Run("unknown default", func(t *testing.T) {
		fixture := newGitFixture(t)
		git(t, fixture.work, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
		git(t, fixture.work, "update-ref", "refs/remotes/origin/master", "refs/remotes/origin/main")

		result := onlyResult(t, preview(t, Target{Path: fixture.work}))
		if result.Status != StatusDefaultUnknown {
			t.Fatalf("Status = %q, want %q", result.Status, StatusDefaultUnknown)
		}
	})

	t.Run("not git", func(t *testing.T) {
		path := t.TempDir()
		result := onlyResult(t, preview(t, Target{Path: path}))
		if result.Status != StatusNotGit {
			t.Fatalf("Status = %q, want %q", result.Status, StatusNotGit)
		}
	})
}

func TestRunPreviewBlocksTargetBranchInAnotherWorktree(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "switch", "-c", "feature")
	otherWorktree := filepath.Join(filepath.Dir(fixture.work), "other-worktree")
	git(t, fixture.work, "worktree", "add", otherWorktree, "main")

	result := onlyResult(t, preview(t, Target{Path: fixture.work}))
	if result.Status != StatusBlockedWorktree {
		t.Fatalf("Status = %q, want %q", result.Status, StatusBlockedWorktree)
	}
	if result.BranchAfter != "main" {
		t.Fatalf("BranchAfter = %q, want main", result.BranchAfter)
	}
}

func TestRunPreviewDisablesRepositoryFSMonitor(t *testing.T) {
	fixture := newGitFixture(t)
	marker := filepath.Join(t.TempDir(), "fsmonitor-invoked")
	t.Setenv("GOREGRAPH_TEST_FSMONITOR_MARKER", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	git(t, fixture.work, "config", "core.fsmonitor", executable)

	result := onlyResult(t, preview(t, Target{Path: fixture.work}))
	if result.Status != StatusUpToDate {
		t.Fatalf("Status = %q, want %q", result.Status, StatusUpToDate)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("repository fsmonitor executed during preview: %v", err)
	}
}

func TestRunPreviewDisablesLazyPromisorFetch(t *testing.T) {
	fixture := newGitFixture(t)
	marker := filepath.Join(t.TempDir(), "lazy-fetch-invoked")
	t.Setenv("GOREGRAPH_TEST_LAZY_FETCH_MARKER", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}

	tree := revParse(t, fixture.work, "HEAD^{tree}")
	missingCommit := git(t, fixture.work, "commit-tree", tree, "-p", "HEAD", "-m", "promisor update")
	git(t, fixture.work, "update-ref", "refs/remotes/origin/main", missingCommit)
	git(t, fixture.work, "config", "remote.origin.promisor", "true")
	git(t, fixture.work, "config", "remote.origin.partialclonefilter", "blob:none")
	globalConfig := filepath.Join(t.TempDir(), "gitconfig")
	writeFile(t, globalConfig, "[remote \"origin\"]\n\tuploadpack = "+executable+"\n")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)

	gitDir := git(t, fixture.work, "rev-parse", "--absolute-git-dir")
	missingObject := filepath.Join(gitDir, "objects", missingCommit[:2], missingCommit[2:])
	if err := os.Remove(missingObject); err != nil {
		t.Fatalf("remove promised commit %s: %v", missingCommit, err)
	}
	headBefore := revParse(t, fixture.work, "HEAD")
	remoteBefore := revParse(t, fixture.work, "refs/remotes/origin/main")

	result := onlyResult(t, preview(t, Target{Path: fixture.work}))
	if result.Status != StatusDefaultUnknown {
		t.Fatalf("Status = %q, want %q", result.Status, StatusDefaultUnknown)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("promisor remote contacted during preview: %v", err)
	}
	if _, err := os.Stat(missingObject); !os.IsNotExist(err) {
		t.Fatalf("missing promisor object was fetched: %v", err)
	}
	if headAfter := revParse(t, fixture.work, "HEAD"); headAfter != headBefore {
		t.Fatalf("HEAD changed: got %s, want %s", headAfter, headBefore)
	}
	if remoteAfter := revParse(t, fixture.work, "refs/remotes/origin/main"); remoteAfter != remoteBefore {
		t.Fatalf("origin/main changed: got %s, want %s", remoteAfter, remoteBefore)
	}
}

func TestRunExecuteFetchesSwitchesAndFastForwards(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "switch", "-c", "feature")
	targetCommit := fixture.commitAndPushFromPeer(t, "peer update\n")

	report := execute(t, Target{Path: fixture.work})
	result := onlyResult(t, report)
	if report.Mode != ModeExecute {
		t.Fatalf("Mode = %q, want %q", report.Mode, ModeExecute)
	}
	if result.Status != StatusUpdated {
		t.Fatalf("Status = %q, want %q; reason: %s", result.Status, StatusUpdated, result.Reason)
	}
	if !result.Executed {
		t.Fatal("Executed = false, want true")
	}
	if result.BranchBefore != "feature" || result.BranchAfter != "main" {
		t.Fatalf("branches = %q -> %q, want feature -> main", result.BranchBefore, result.BranchAfter)
	}
	if result.CommitAfter != targetCommit || revParse(t, fixture.work, "HEAD") != targetCommit {
		t.Fatalf("CommitAfter = %q and HEAD = %q, want %q", result.CommitAfter, revParse(t, fixture.work, "HEAD"), targetCommit)
	}
	if contents := readFile(t, filepath.Join(fixture.work, "README.md")); contents != "peer update\n" {
		t.Fatalf("README.md = %q, want peer update", contents)
	}
}

func TestRunExecutePreflightBlockerReportsActualCheckout(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "switch", "-c", "feature")
	targetCommit := fixture.commitAndPushFromPeer(t, "peer update\n")
	git(t, fixture.work, "fetch", "origin")
	headBefore := revParse(t, fixture.work, "HEAD")
	if headBefore == targetCommit {
		t.Fatalf("fixture HEAD = target commit %s, want different commits", headBefore)
	}
	writeFile(t, filepath.Join(fixture.work, "README.md"), "dirty\n")
	before := map[string]string{
		"branch":     git(t, fixture.work, "symbolic-ref", "--short", "HEAD"),
		"head":       headBefore,
		"status":     git(t, fixture.work, "status", "--porcelain=v1", "--untracked-files=all"),
		"contents":   readFile(t, filepath.Join(fixture.work, "README.md")),
		"remote_ref": revParse(t, fixture.work, "refs/remotes/origin/main"),
	}

	result := onlyResult(t, execute(t, Target{Path: fixture.work}))
	if result.Status != StatusDirty {
		t.Fatalf("Status = %q, want %q; reason: %s", result.Status, StatusDirty, result.Reason)
	}
	if result.Executed {
		t.Fatal("Executed = true, want false")
	}
	if result.BranchBefore != "feature" || result.BranchAfter != "feature" {
		t.Fatalf("branches = %q -> %q, want feature -> feature", result.BranchBefore, result.BranchAfter)
	}
	if result.CommitBefore != headBefore || result.CommitAfter != headBefore {
		t.Fatalf("commits = %q -> %q, want %q -> %q", result.CommitBefore, result.CommitAfter, headBefore, headBefore)
	}
	if result.Remediation == "" {
		t.Fatal("Remediation is empty")
	}

	after := map[string]string{
		"branch":     git(t, fixture.work, "symbolic-ref", "--short", "HEAD"),
		"head":       revParse(t, fixture.work, "HEAD"),
		"status":     git(t, fixture.work, "status", "--porcelain=v1", "--untracked-files=all"),
		"contents":   readFile(t, filepath.Join(fixture.work, "README.md")),
		"remote_ref": revParse(t, fixture.work, "refs/remotes/origin/main"),
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("repository changed for preflight blocker:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestRunExecuteCreatesMissingTrackingBranch(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "switch", "-c", "feature")
	git(t, fixture.work, "branch", "-D", "main")
	targetCommit := fixture.commitAndPushFromPeer(t, "peer update\n")

	result := onlyResult(t, execute(t, Target{Path: fixture.work}))
	if result.Status != StatusUpdated {
		t.Fatalf("Status = %q, want %q; reason: %s", result.Status, StatusUpdated, result.Reason)
	}
	if result.BranchAfter != "main" || result.CommitAfter != targetCommit {
		t.Fatalf("result = branch %q at %q, want main at %q", result.BranchAfter, result.CommitAfter, targetCommit)
	}
	if upstream := git(t, fixture.work, "rev-parse", "--abbrev-ref", "main@{upstream}"); upstream != "origin/main" {
		t.Fatalf("main upstream = %q, want origin/main", upstream)
	}
}

func TestRunExecuteReportsUpToDateAfterFetch(t *testing.T) {
	fixture := newGitFixture(t)

	result := onlyResult(t, execute(t, Target{Path: fixture.work}))
	if result.Status != StatusUpToDate {
		t.Fatalf("Status = %q, want %q; reason: %s", result.Status, StatusUpToDate, result.Reason)
	}
	if !result.Executed {
		t.Fatal("Executed = false, want true after fetch")
	}
	if result.BranchBefore != "main" || result.BranchAfter != "main" || result.CommitBefore != result.CommitAfter {
		t.Fatalf("unexpected result state: %#v", result)
	}
}

func TestRunExecuteReportsFetchFailureWithoutCheckoutChange(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "switch", "-c", "feature")
	writeFile(t, filepath.Join(fixture.work, "feature.txt"), "feature\n")
	git(t, fixture.work, "add", "feature.txt")
	git(t, fixture.work, "commit", "-m", "feature")
	missingRemote := filepath.Join(filepath.Dir(fixture.remote), "missing.git")
	git(t, fixture.work, "remote", "set-url", "origin", missingRemote)
	before := map[string]string{
		"branch":   git(t, fixture.work, "symbolic-ref", "--short", "HEAD"),
		"head":     revParse(t, fixture.work, "HEAD"),
		"status":   git(t, fixture.work, "status", "--porcelain=v1", "--untracked-files=all"),
		"readme":   readFile(t, filepath.Join(fixture.work, "README.md")),
		"feature":  readFile(t, filepath.Join(fixture.work, "feature.txt")),
		"main_ref": revParse(t, fixture.work, "refs/heads/main"),
	}

	result := onlyResult(t, execute(t, Target{Path: fixture.work}))
	if result.Status != StatusFetchFailed {
		t.Fatalf("Status = %q, want %q; reason: %s", result.Status, StatusFetchFailed, result.Reason)
	}
	if !result.Executed || result.Remediation == "" {
		t.Fatalf("Executed = %t and Remediation = %q, want true and non-empty", result.Executed, result.Remediation)
	}
	if result.BranchAfter != before["branch"] || result.CommitAfter != before["head"] {
		t.Fatalf("reported checkout = %q at %q, want unchanged %q at %q", result.BranchAfter, result.CommitAfter, before["branch"], before["head"])
	}
	after := map[string]string{
		"branch":   git(t, fixture.work, "symbolic-ref", "--short", "HEAD"),
		"head":     revParse(t, fixture.work, "HEAD"),
		"status":   git(t, fixture.work, "status", "--porcelain=v1", "--untracked-files=all"),
		"readme":   readFile(t, filepath.Join(fixture.work, "README.md")),
		"feature":  readFile(t, filepath.Join(fixture.work, "feature.txt")),
		"main_ref": revParse(t, fixture.work, "refs/heads/main"),
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("checkout changed after fetch failure:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestRunExecutePinsFetchToOriginHeadsAndPreservesLocalRefs(t *testing.T) {
	fixture := newGitFixture(t)
	initialHead := revParse(t, fixture.work, "HEAD")

	git(t, fixture.peer, "switch", "-c", "obsolete")
	writeFile(t, filepath.Join(fixture.peer, "obsolete.txt"), "obsolete\n")
	git(t, fixture.peer, "add", "obsolete.txt")
	git(t, fixture.peer, "commit", "-m", "add obsolete branch")
	git(t, fixture.peer, "push", "-u", "origin", "obsolete")
	git(t, fixture.peer, "switch", "main")
	git(t, fixture.work, "fetch", "origin")
	git(t, fixture.peer, "push", "origin", "--delete", "obsolete")

	git(t, fixture.peer, "switch", "-c", "new-remote")
	writeFile(t, filepath.Join(fixture.peer, "remote.txt"), "remote branch\n")
	git(t, fixture.peer, "add", "remote.txt")
	git(t, fixture.peer, "commit", "-m", "add remote branch")
	newRemoteCommit := revParse(t, fixture.peer, "HEAD")
	git(t, fixture.peer, "push", "-u", "origin", "new-remote")
	git(t, fixture.peer, "switch", "main")
	targetCommit := fixture.commitAndPushFromPeer(t, "remote update\n")

	git(t, fixture.work, "branch", "protected", initialHead)
	git(t, fixture.work, "tag", "protected-tag", initialHead)
	git(t, fixture.work, "config", "--replace-all", "remote.origin.fetch", "+refs/heads/main:refs/heads/protected")
	git(t, fixture.work, "config", "fetch.pruneTags", "true")
	git(t, fixture.work, "config", "remote.origin.pruneTags", "true")

	result := onlyResult(t, execute(t, Target{Path: fixture.work}))
	if result.Status != StatusUpdated {
		t.Fatalf("Status = %q, want %q; reason: %s", result.Status, StatusUpdated, result.Reason)
	}
	if protected := revParse(t, fixture.work, "refs/heads/protected"); protected != initialHead {
		t.Fatalf("protected local branch = %s, want unchanged %s", protected, initialHead)
	}
	if protectedTag := revParse(t, fixture.work, "refs/tags/protected-tag"); protectedTag != initialHead {
		t.Fatalf("protected local tag = %s, want unchanged %s", protectedTag, initialHead)
	}
	if originMain := revParse(t, fixture.work, "refs/remotes/origin/main"); originMain != targetCommit {
		t.Fatalf("origin/main = %s, want fetched %s", originMain, targetCommit)
	}
	if originNew := revParse(t, fixture.work, "refs/remotes/origin/new-remote"); originNew != newRemoteCommit {
		t.Fatalf("origin/new-remote = %s, want fetched %s", originNew, newRemoteCommit)
	}
	originRefs := strings.Fields(git(t, fixture.work, "for-each-ref", "--format=%(refname)", "refs/remotes/origin"))
	if containsArgument(originRefs, "refs/remotes/origin/obsolete") {
		t.Fatalf("obsolete origin ref was not pruned: %v", originRefs)
	}
}

func TestRunExecutePinsFetchAndSwitchSafetyFlags(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "switch", "-c", "feature")
	logPath := installGitCommandLog(t)

	result := onlyResult(t, execute(t, Target{Path: fixture.work}))
	if result.Status != StatusUpdated {
		t.Fatalf("Status = %q, want %q; reason: %s", result.Status, StatusUpdated, result.Reason)
	}

	commands := readLoggedGitCommands(t, logPath)
	fetch := loggedGitCommand(t, commands, "fetch")
	for _, expected := range []string{
		"--prune",
		"--no-prune-tags",
		"--no-tags",
		"--no-recurse-submodules",
		"--no-auto-maintenance",
		"+refs/heads/*:refs/remotes/origin/*",
	} {
		if !containsArgument(fetch, expected) {
			t.Fatalf("fetch command does not contain %q: %v", expected, fetch)
		}
	}
	switchCommand := loggedGitCommand(t, commands, "switch")
	if !containsArgument(switchCommand, "--no-recurse-submodules") {
		t.Fatalf("switch command does not disable submodule recursion: %v", switchCommand)
	}
}

func TestRunExecuteUsesBoundedNonInteractiveSSH(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "remote", "set-url", "origin", "git@example.invalid:owner/repository.git")

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	fakeSSHDirectory := t.TempDir()
	if err := os.Symlink(executable, filepath.Join(fakeSSHDirectory, "ssh")); err != nil {
		t.Fatalf("install fake SSH: %v", err)
	}
	argumentsPath := filepath.Join(t.TempDir(), "ssh-arguments")
	t.Setenv("GOREGRAPH_TEST_SSH_ARGUMENTS", argumentsPath)
	t.Setenv("PATH", fakeSSHDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	report, err := Run(ctx, []Target{{Path: fixture.work}}, Options{Execute: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("SSH execution exceeded bounded context: %v", ctx.Err())
	}
	result := onlyResult(t, report)
	if result.Status != StatusFetchFailed || !result.Executed {
		t.Fatalf("result = %#v, want executed fetch failure", result)
	}

	invocation := readFile(t, argumentsPath)
	for _, expected := range []string{
		"-oBatchMode=yes",
		"-oStrictHostKeyChecking=yes",
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	} {
		if !strings.Contains(invocation, expected) {
			t.Fatalf("fake SSH invocation does not contain %q:\n%s", expected, invocation)
		}
	}
}

func TestUsesSSHTransportForSSHAndSCPURLsOnly(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		want      bool
	}{
		{name: "SSH URL", remoteURL: "ssh://git@example.invalid/owner/repository.git", want: true},
		{name: "SCP URL", remoteURL: "git@example.invalid:owner/repository.git", want: true},
		{name: "HTTPS URL", remoteURL: "https://example.invalid/owner/repository.git", want: false},
		{name: "file URL", remoteURL: "file:///tmp/repository.git", want: false},
		{name: "absolute path", remoteURL: "/tmp/repository.git", want: false},
		{name: "relative path with colon", remoteURL: "./directory:name/repository.git", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := usesSSHTransport(test.remoteURL); got != test.want {
				t.Fatalf("usesSSHTransport(%q) = %t, want %t", test.remoteURL, got, test.want)
			}
		})
	}
}

func TestRunExecuteDisablesCheckoutAndMergeHooks(t *testing.T) {
	fixture := newGitFixture(t)
	git(t, fixture.work, "switch", "-c", "feature")
	targetCommit := fixture.commitAndPushFromPeer(t, "peer update\n")
	marker := filepath.Join(t.TempDir(), "hook-invoked")
	hooksDir := filepath.Join(fixture.work, ".git", "hooks")
	for _, hook := range []string{"post-checkout", "post-merge"} {
		hookPath := filepath.Join(hooksDir, hook)
		writeFile(t, hookPath, "#!/bin/sh\nprintf '%s\\n' invoked >> '"+marker+"'\nexit 1\n")
		if err := os.Chmod(hookPath, 0o700); err != nil {
			t.Fatalf("chmod %s: %v", hookPath, err)
		}
	}

	result := onlyResult(t, execute(t, Target{Path: fixture.work}))
	if result.Status != StatusUpdated || result.CommitAfter != targetCommit {
		t.Fatalf("result = %#v, want updated to %s", result, targetCommit)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("repository hook executed: %v", err)
	}
}

func TestRunExecuteRechecksDirtyAheadAndDivergedAfterFetch(t *testing.T) {
	t.Run("dirty after fetch starts", func(t *testing.T) {
		fixture := newGitFixture(t)
		fixture.commitAndPushFromPeer(t, "peer update\n")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		barrierDirectory := installGitFetchBarrier(t)
		type runOutcome struct {
			report Report
			err    error
		}
		outcomes := make(chan runOutcome, 1)
		go func() {
			report, err := Run(ctx, []Target{{Path: fixture.work}}, Options{Execute: true})
			outcomes <- runOutcome{report: report, err: err}
		}()

		if err := waitForBarrier(ctx, filepath.Join(barrierDirectory, "ready")); err != nil {
			t.Fatalf("wait for completed fetch: %v", err)
		}
		writeFile(t, filepath.Join(fixture.work, "late-untracked.txt"), "dirty\n")
		if err := os.WriteFile(filepath.Join(barrierDirectory, "release"), []byte("release\n"), 0o600); err != nil {
			t.Fatalf("release fetch barrier: %v", err)
		}

		var outcome runOutcome
		select {
		case outcome = <-outcomes:
		case <-ctx.Done():
			t.Fatalf("Run() did not finish: %v", ctx.Err())
		}
		if outcome.err != nil {
			t.Fatalf("Run() error = %v", outcome.err)
		}
		result := onlyResult(t, outcome.report)
		if result.Status != StatusDirty || !result.Executed {
			t.Fatalf("result = %#v, want executed dirty blocker", result)
		}
	})

	t.Run("ahead after fetch", func(t *testing.T) {
		fixture := newGitFixture(t)
		initialCommit := revParse(t, fixture.work, "HEAD")
		fixture.commitAndPushFromPeer(t, "peer update\n")
		git(t, fixture.work, "fetch", "origin")
		git(t, fixture.work, "merge", "--ff-only", "origin/main")
		git(t, fixture.peer, "reset", "--hard", initialCommit)
		git(t, fixture.peer, "push", "--force", "origin", "main")

		result := onlyResult(t, execute(t, Target{Path: fixture.work}))
		if result.Status != StatusAhead || !result.Executed {
			t.Fatalf("result = %#v, want executed ahead blocker", result)
		}
		if head := revParse(t, fixture.work, "HEAD"); head != result.CommitBefore {
			t.Fatalf("HEAD = %s, want unchanged %s", head, result.CommitBefore)
		}
		if result.BranchAfter != "main" || result.CommitAfter != result.CommitBefore {
			t.Fatalf("reported checkout = %q at %q, want unchanged main at %q", result.BranchAfter, result.CommitAfter, result.CommitBefore)
		}
	})

	t.Run("diverged after fetch", func(t *testing.T) {
		fixture := newGitFixture(t)
		writeFile(t, filepath.Join(fixture.work, "local.txt"), "local\n")
		git(t, fixture.work, "add", "local.txt")
		git(t, fixture.work, "commit", "-m", "shared tip")
		git(t, fixture.work, "push", "origin", "main")
		git(t, fixture.peer, "fetch", "origin")
		git(t, fixture.peer, "reset", "--hard", "origin/main")
		git(t, fixture.work, "fetch", "origin")
		localCommit := revParse(t, fixture.work, "HEAD")
		git(t, fixture.peer, "reset", "--hard", "HEAD^")
		writeFile(t, filepath.Join(fixture.peer, "remote.txt"), "remote\n")
		git(t, fixture.peer, "add", "remote.txt")
		git(t, fixture.peer, "commit", "-m", "replacement tip")
		git(t, fixture.peer, "push", "--force", "origin", "main")

		result := onlyResult(t, execute(t, Target{Path: fixture.work}))
		if result.Status != StatusDiverged || !result.Executed {
			t.Fatalf("result = %#v, want executed diverged blocker", result)
		}
		if head := revParse(t, fixture.work, "HEAD"); head != localCommit {
			t.Fatalf("HEAD = %s, want unchanged %s", head, localCommit)
		}
		if result.BranchAfter != "main" || result.CommitAfter != localCommit {
			t.Fatalf("reported checkout = %q at %q, want unchanged main at %q", result.BranchAfter, result.CommitAfter, localCommit)
		}
	})
}

func installGitFetchBarrier(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real Git: %v", err)
	}
	barrierDirectory, err := os.MkdirTemp("/tmp", "goregraph-fetch-barrier-")
	if err != nil {
		t.Fatalf("create fetch barrier directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(barrierDirectory) })
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	wrapperDirectory := t.TempDir()
	if err := os.Symlink(executable, filepath.Join(wrapperDirectory, "git")); err != nil {
		t.Fatalf("install Git fetch barrier: %v", err)
	}
	t.Setenv("GOREGRAPH_TEST_GIT_BARRIER_DIR", barrierDirectory)
	t.Setenv("GOREGRAPH_TEST_REAL_GIT", realGit)
	t.Setenv("PATH", wrapperDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	return barrierDirectory
}

func installGitCommandLog(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real Git: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	wrapperDirectory := t.TempDir()
	if err := os.Symlink(executable, filepath.Join(wrapperDirectory, "git")); err != nil {
		t.Fatalf("install Git command logger: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "git-commands")
	t.Setenv("GOREGRAPH_TEST_GIT_LOG", logPath)
	t.Setenv("GOREGRAPH_TEST_REAL_GIT", realGit)
	t.Setenv("PATH", wrapperDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func readLoggedGitCommands(t *testing.T, path string) [][]string {
	t.Helper()
	contents := readFile(t, path)
	commands := make([][]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(contents), "\n") {
		if line != "" {
			commands = append(commands, strings.Split(line, "\x1f"))
		}
	}
	return commands
}

func loggedGitCommand(t *testing.T, commands [][]string, operation string) []string {
	t.Helper()
	for _, command := range commands {
		if containsArgument(command, operation) {
			return command
		}
	}
	t.Fatalf("no logged Git %s command: %v", operation, commands)
	return nil
}

func waitForBarrier(ctx context.Context, path string) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				return nil
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	}
}

func TestRunRejectsExecutableLocalGitConfigWithoutInvokingIt(t *testing.T) {
	t.Run("fetch upload pack", func(t *testing.T) {
		fixture := newGitFixture(t)
		fixture.commitAndPushFromPeer(t, "peer update\n")
		marker := filepath.Join(t.TempDir(), "upload-pack-invoked")
		t.Setenv("GOREGRAPH_TEST_GIT_COMMAND_MARKER", marker)
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("locate test executable: %v", err)
		}
		git(t, fixture.work, "config", "remote.origin.uploadpack", executable)
		headBefore := revParse(t, fixture.work, "HEAD")
		remoteBefore := revParse(t, fixture.work, "refs/remotes/origin/main")

		report, err := Run(context.Background(), []Target{{Path: fixture.work}}, Options{Execute: true})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		result := onlyResult(t, report)
		assertSafetyRefusal(t, result, "remote.origin.uploadpack", false)
		if result.BranchAfter != "main" || result.CommitAfter != headBefore {
			t.Fatalf("reported checkout = %q at %q, want unchanged main at %q", result.BranchAfter, result.CommitAfter, headBefore)
		}
		assertMarkerAbsent(t, marker)
		if head := revParse(t, fixture.work, "HEAD"); head != headBefore {
			t.Fatalf("HEAD = %s, want unchanged %s", head, headBefore)
		}
		if remote := revParse(t, fixture.work, "refs/remotes/origin/main"); remote != remoteBefore {
			t.Fatalf("origin/main = %s, want unchanged %s", remote, remoteBefore)
		}
	})

	t.Run("clean filter during preview", func(t *testing.T) {
		fixture := newGitFixture(t)
		writeFile(t, filepath.Join(fixture.work, ".gitattributes"), "README.md filter=unsafe\n")
		git(t, fixture.work, "add", ".gitattributes")
		git(t, fixture.work, "commit", "-m", "add attributes")
		marker := filepath.Join(t.TempDir(), "clean-filter-invoked")
		t.Setenv("GOREGRAPH_TEST_GIT_COMMAND_MARKER", marker)
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("locate test executable: %v", err)
		}
		git(t, fixture.work, "config", "filter.unsafe.clean", executable)
		writeFile(t, filepath.Join(fixture.work, "README.md"), "force clean filter\n")

		report, err := Run(context.Background(), []Target{{Path: fixture.work}}, Options{})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		assertSafetyRefusal(t, onlyResult(t, report), "filter.unsafe.clean", false)
		assertMarkerAbsent(t, marker)
	})

	for _, key := range []string{
		"filter.unsafe.smudge",
		"filter.unsafe.process",
		"core.askPass",
		"core.attributesFile",
		"core.sshCommand",
		"core.gitProxy",
		"credential.helper",
		"credential.https://example.invalid.helper",
		"remote.origin.vcs",
		"url.ext::unsafe.insteadOf",
	} {
		t.Run(key, func(t *testing.T) {
			fixture := newGitFixture(t)
			git(t, fixture.work, "config", key, "/definitely/not/executed")

			report, err := Run(context.Background(), []Target{{Path: fixture.work}}, Options{Execute: true})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			assertSafetyRefusal(t, onlyResult(t, report), strings.ToLower(key), false)
		})
	}
}

func TestRunExecuteRejectsAdditionalExecutableConfigBeforeFetch(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value func(string) string
	}{
		{
			name: "alternate refs command",
			key:  "core.alternateRefsCommand",
			value: func(executable string) string {
				return executable
			},
		},
		{
			name: "recent objects hook",
			key:  "gc.recentObjectsHook",
			value: func(executable string) string {
				return executable
			},
		},
		{
			name: "quoted command-valued submodule update",
			key:  "submodule.unsafe.update",
			value: func(executable string) string {
				return " !" + executable
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGitFixture(t)
			fixture.commitAndPushFromPeer(t, test.name+"\n")
			marker := filepath.Join(t.TempDir(), "command-invoked")
			t.Setenv("GOREGRAPH_TEST_GIT_COMMAND_MARKER", marker)
			executable, err := os.Executable()
			if err != nil {
				t.Fatalf("locate test executable: %v", err)
			}
			git(t, fixture.work, "config", test.key, test.value(executable))
			headBefore := revParse(t, fixture.work, "HEAD")
			remoteBefore := revParse(t, fixture.work, "refs/remotes/origin/main")

			report, err := Run(context.Background(), []Target{{Path: fixture.work}}, Options{Execute: true})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			result := onlyResult(t, report)
			assertSafetyRefusal(t, result, strings.ToLower(test.key), false)
			assertMarkerAbsent(t, marker)
			if head := revParse(t, fixture.work, "HEAD"); head != headBefore {
				t.Fatalf("HEAD = %s, want unchanged %s", head, headBefore)
			}
			if remote := revParse(t, fixture.work, "refs/remotes/origin/main"); remote != remoteBefore {
				t.Fatalf("origin/main = %s, want unchanged %s", remote, remoteBefore)
			}
		})
	}
}

func TestRunExecuteRejectsTargetFiltersWithoutInvokingThem(t *testing.T) {
	for _, command := range []string{"smudge", "process"} {
		t.Run(command, func(t *testing.T) {
			fixture := newGitFixture(t)
			git(t, fixture.work, "switch", "-c", "feature")
			marker := filepath.Join(t.TempDir(), command+"-filter-invoked")
			executable, err := os.Executable()
			if err != nil {
				t.Fatalf("locate test executable: %v", err)
			}
			globalConfig := filepath.Join(t.TempDir(), "gitconfig")
			writeFile(t, globalConfig, "[filter \"unsafe\"]\n\t"+command+" = "+executable+"\n\trequired = true\n")
			writeFile(t, filepath.Join(fixture.peer, ".gitattributes"), "*.payload filter=unsafe\n")
			writeFile(t, filepath.Join(fixture.peer, "remote.payload"), "remote\n")
			git(t, fixture.peer, "add", ".gitattributes", "remote.payload")
			git(t, fixture.peer, "commit", "-m", "add filtered payload")
			targetCommit := revParse(t, fixture.peer, "HEAD")
			git(t, fixture.peer, "push", "origin", "main")
			t.Setenv("GOREGRAPH_TEST_GIT_COMMAND_MARKER", marker)
			t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
			headBefore := revParse(t, fixture.work, "HEAD")
			mainBefore := revParse(t, fixture.work, "refs/heads/main")

			result := onlyResult(t, execute(t, Target{Path: fixture.work}))
			assertSafetyRefusal(t, result, ".gitattributes", true)
			assertMarkerAbsent(t, marker)
			if head := revParse(t, fixture.work, "HEAD"); head != headBefore {
				t.Fatalf("HEAD = %s, want unchanged %s", head, headBefore)
			}
			if main := revParse(t, fixture.work, "refs/heads/main"); main != mainBefore {
				t.Fatalf("main = %s, want unchanged %s", main, mainBefore)
			}
			if remote := revParse(t, fixture.work, "refs/remotes/origin/main"); remote != targetCommit {
				t.Fatalf("origin/main = %s, want fetched %s", remote, targetCommit)
			}
		})
	}
}

func TestRunExecuteRejectsUnsafeExistingLocalTargetTreeBeforeSwitch(t *testing.T) {
	fixture := newGitFixture(t)
	initialHead := revParse(t, fixture.work, "HEAD")
	git(t, fixture.work, "switch", "-c", "feature")
	git(t, fixture.work, "switch", "main")
	writeFile(t, filepath.Join(fixture.work, ".gitattributes"), "*.payload filter=unsafe\n")
	writeFile(t, filepath.Join(fixture.work, "local.payload"), "unsafe local target\n")
	git(t, fixture.work, "add", ".gitattributes", "local.payload")
	git(t, fixture.work, "commit", "-m", "add unsafe local target tree")
	unsafeLocalMain := revParse(t, fixture.work, "HEAD")
	git(t, fixture.work, "push", "origin", "main")
	git(t, fixture.work, "fetch", "origin")

	git(t, fixture.peer, "fetch", "origin")
	git(t, fixture.peer, "reset", "--hard", "origin/main")
	git(t, fixture.peer, "rm", ".gitattributes", "local.payload")
	git(t, fixture.peer, "commit", "-m", "remove unsafe target attributes")
	safeRemoteMain := revParse(t, fixture.peer, "HEAD")
	git(t, fixture.peer, "push", "origin", "main")
	git(t, fixture.work, "switch", "feature")
	for _, path := range []string{
		filepath.Join(fixture.work, ".gitattributes"),
		filepath.Join(fixture.work, "local.payload"),
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove switched-away target file %s: %v", path, err)
		}
	}
	if status := git(t, fixture.work, "status", "--porcelain=v1", "--untracked-files=all"); status != "" {
		t.Fatalf("feature checkout is not clean before execution: %s", status)
	}

	marker := filepath.Join(t.TempDir(), "local-target-filter-invoked")
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	globalConfig := filepath.Join(t.TempDir(), "gitconfig")
	writeFile(t, globalConfig, "[filter \"unsafe\"]\n\tsmudge = "+executable+"\n\trequired = true\n")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GOREGRAPH_TEST_GIT_COMMAND_MARKER", marker)

	result := onlyResult(t, execute(t, Target{Path: fixture.work}))
	assertSafetyRefusal(t, result, "local branch main", true)
	assertMarkerAbsent(t, marker)
	if branch := git(t, fixture.work, "branch", "--show-current"); branch != "feature" {
		t.Fatalf("current branch = %q, want unchanged feature", branch)
	}
	if head := revParse(t, fixture.work, "HEAD"); head != initialHead {
		t.Fatalf("HEAD = %s, want unchanged %s", head, initialHead)
	}
	if main := revParse(t, fixture.work, "refs/heads/main"); main != unsafeLocalMain {
		t.Fatalf("main = %s, want unchanged %s", main, unsafeLocalMain)
	}
	if remote := revParse(t, fixture.work, "refs/remotes/origin/main"); remote != safeRemoteMain {
		t.Fatalf("origin/main = %s, want fetched %s", remote, safeRemoteMain)
	}
	if _, err := os.Stat(filepath.Join(fixture.work, "local.payload")); !os.IsNotExist(err) {
		t.Fatalf("unsafe payload was checked out or cannot be inspected: %v", err)
	}
}

func TestRunPreviewRejectsCommonWorktreeAttributesWithoutInvokingFilter(t *testing.T) {
	fixture := newGitFixture(t)
	linkedWorktree := filepath.Join(filepath.Dir(fixture.work), "linked-worktree")
	git(t, fixture.work, "worktree", "add", "-b", "feature", linkedWorktree)
	marker := filepath.Join(t.TempDir(), "linked-clean-filter-invoked")
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	globalConfig := filepath.Join(t.TempDir(), "gitconfig")
	writeFile(t, globalConfig, "[filter \"unsafe\"]\n\tclean = "+executable+"\n")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GOREGRAPH_TEST_GIT_COMMAND_MARKER", marker)
	commonDirectory := git(t, linkedWorktree, "rev-parse", "--git-common-dir")
	if !filepath.IsAbs(commonDirectory) {
		commonDirectory = filepath.Join(linkedWorktree, commonDirectory)
	}
	writeFile(t, filepath.Join(commonDirectory, "info", "attributes"), "README.md filter=unsafe\n")
	writeFile(t, filepath.Join(linkedWorktree, "README.md"), "force clean filter\n")

	report, err := Run(context.Background(), []Target{{Path: linkedWorktree}}, Options{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertSafetyRefusal(t, onlyResult(t, report), "info/attributes", false)
	assertMarkerAbsent(t, marker)
}

func TestRunPreviewRejectsNestedIndexAttributesWithoutInvokingFilter(t *testing.T) {
	for _, command := range []string{"clean", "process"} {
		t.Run(command, func(t *testing.T) {
			fixture := newGitFixture(t)
			nestedDirectory := filepath.Join(fixture.work, "nested")
			if err := os.Mkdir(nestedDirectory, 0o700); err != nil {
				t.Fatalf("create nested directory: %v", err)
			}
			attributesPath := filepath.Join(nestedDirectory, ".gitattributes")
			payloadPath := filepath.Join(nestedDirectory, "indexed.payload")
			writeFile(t, attributesPath, "*.payload filter=unsafe\n")
			writeFile(t, payloadPath, "indexed\n")
			git(t, fixture.work, "add", "nested/.gitattributes", "nested/indexed.payload")
			git(t, fixture.work, "commit", "-m", "add indexed attributes")
			git(t, fixture.work, "push", "origin", "main")
			git(t, fixture.work, "fetch", "origin")
			if err := os.Remove(attributesPath); err != nil {
				t.Fatalf("remove worktree attributes: %v", err)
			}
			writeFile(t, payloadPath, "force filter\n")
			marker := filepath.Join(t.TempDir(), command+"-index-filter-invoked")
			executable, err := os.Executable()
			if err != nil {
				t.Fatalf("locate test executable: %v", err)
			}
			globalConfig := filepath.Join(t.TempDir(), "gitconfig")
			writeFile(t, globalConfig, "[filter \"unsafe\"]\n\t"+command+" = "+executable+"\n\trequired = true\n")
			t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
			t.Setenv("GOREGRAPH_TEST_GIT_COMMAND_MARKER", marker)
			headBefore := revParse(t, fixture.work, "HEAD")
			remoteBefore := revParse(t, fixture.work, "refs/remotes/origin/main")

			report, err := Run(context.Background(), []Target{{Path: fixture.work}}, Options{})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			result := onlyResult(t, report)
			assertSafetyRefusal(t, result, "index", false)
			assertMarkerAbsent(t, marker)
			if result.BranchAfter != "main" || result.CommitAfter != headBefore {
				t.Fatalf("reported checkout = %q at %q, want unchanged main at %q", result.BranchAfter, result.CommitAfter, headBefore)
			}
			if head := revParse(t, fixture.work, "HEAD"); head != headBefore {
				t.Fatalf("HEAD = %s, want unchanged %s", head, headBefore)
			}
			if remote := revParse(t, fixture.work, "refs/remotes/origin/main"); remote != remoteBefore {
				t.Fatalf("origin/main = %s, want unchanged %s", remote, remoteBefore)
			}
			if contents := readFile(t, payloadPath); contents != "force filter\n" {
				t.Fatalf("payload = %q, want unchanged", contents)
			}
		})
	}
}

func TestRunExecuteRejectsUnsafeRemoteHelpers(t *testing.T) {
	for _, remoteURL := range []string{"ext::sh -c false", "unknown::payload"} {
		t.Run(remoteURL, func(t *testing.T) {
			fixture := newGitFixture(t)
			git(t, fixture.work, "remote", "set-url", "origin", remoteURL)

			result := onlyResult(t, execute(t, Target{Path: fixture.work}))
			assertSafetyRefusal(t, result, "transport", false)
		})
	}
}

func TestRunExecuteRejectsNonSchemeRemoteHelperWithoutInvokingIt(t *testing.T) {
	fixture := newGitFixture(t)
	marker := filepath.Join(t.TempDir(), "remote-helper-invoked")
	t.Setenv("GOREGRAPH_TEST_GIT_COMMAND_MARKER", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	helperDirectory := t.TempDir()
	if err := os.Symlink(executable, filepath.Join(helperDirectory, "git-remote-1foo")); err != nil {
		t.Fatalf("install remote helper marker: %v", err)
	}
	t.Setenv("PATH", helperDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	git(t, fixture.work, "remote", "set-url", "origin", "1foo::payload")
	headBefore := revParse(t, fixture.work, "HEAD")
	remoteBefore := revParse(t, fixture.work, "refs/remotes/origin/main")

	report, err := Run(context.Background(), []Target{{Path: fixture.work}}, Options{Execute: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	result := onlyResult(t, report)
	assertSafetyRefusal(t, result, "transport", false)
	assertMarkerAbsent(t, marker)
	if result.BranchAfter != "main" || result.CommitAfter != headBefore {
		t.Fatalf("reported checkout = %q at %q, want unchanged main at %q", result.BranchAfter, result.CommitAfter, headBefore)
	}
	if head := revParse(t, fixture.work, "HEAD"); head != headBefore {
		t.Fatalf("HEAD = %s, want unchanged %s", head, headBefore)
	}
	if remote := revParse(t, fixture.work, "refs/remotes/origin/main"); remote != remoteBefore {
		t.Fatalf("origin/main = %s, want unchanged %s", remote, remoteBefore)
	}
}

func TestUnsafeRemoteTransportAllowsStandardIPv6URLs(t *testing.T) {
	for _, remoteURL := range []string{
		"ssh://[2001:db8::1]/repo.git",
		"https://[::1]/repo.git",
	} {
		t.Run(remoteURL, func(t *testing.T) {
			if finding := unsafeRemoteTransport(remoteURL); finding.reason != "" {
				t.Fatalf("unsafeRemoteTransport(%q) reason = %q, want allowed", remoteURL, finding.reason)
			}
		})
	}
}

func TestUnsafeRemoteTransportRejectsHelpersAndUnknownSchemes(t *testing.T) {
	for _, remoteURL := range []string{
		"1foo::payload",
		"ext::sh -c false",
		"unknown::payload",
		"unknown://example.invalid/repo.git",
	} {
		t.Run(remoteURL, func(t *testing.T) {
			if finding := unsafeRemoteTransport(remoteURL); finding.reason == "" {
				t.Fatalf("unsafeRemoteTransport(%q) returned no safety finding", remoteURL)
			}
		})
	}
}

func TestRunExecuteContinuesAfterSafetyRefusal(t *testing.T) {
	first := newGitFixture(t)
	second := newGitFixture(t)
	unsafe, safe := first, second
	if unsafe.work > safe.work {
		unsafe, safe = safe, unsafe
	}
	marker := filepath.Join(t.TempDir(), "unsafe-command-invoked")
	t.Setenv("GOREGRAPH_TEST_GIT_COMMAND_MARKER", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	git(t, unsafe.work, "config", "remote.origin.uploadpack", executable)
	targetCommit := safe.commitAndPushFromPeer(t, "safe update\n")

	report := execute(t, Target{Path: unsafe.work}, Target{Path: safe.work})
	if len(report.Repositories) != 2 {
		t.Fatalf("len(Repositories) = %d, want 2", len(report.Repositories))
	}
	assertSafetyRefusal(t, report.Repositories[0], "remote.origin.uploadpack", false)
	if result := report.Repositories[1]; result.Status != StatusUpdated || result.CommitAfter != targetCommit {
		t.Fatalf("later repository result = %#v, want updated to %s", result, targetCommit)
	}
	assertMarkerAbsent(t, marker)
}

func assertSafetyRefusal(t *testing.T, result RepositoryResult, reasonFragment string, executed bool) {
	t.Helper()
	if result.Status != StatusFetchFailed {
		t.Fatalf("Status = %q, want %q; reason: %s", result.Status, StatusFetchFailed, result.Reason)
	}
	if result.Executed != executed {
		t.Fatalf("Executed = %t, want %t", result.Executed, executed)
	}
	if !strings.Contains(strings.ToLower(result.Reason), strings.ToLower(reasonFragment)) {
		t.Fatalf("Reason = %q, want fragment %q", result.Reason, reasonFragment)
	}
	if result.Remediation == "" {
		t.Fatal("Remediation is empty")
	}
}

func assertMarkerAbsent(t *testing.T, marker string) {
	t.Helper()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("configured command executed: %v", err)
	}
}
