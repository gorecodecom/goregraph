package gitupdate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	for _, environment := range []string{
		"GOREGRAPH_TEST_FSMONITOR_MARKER",
		"GOREGRAPH_TEST_LAZY_FETCH_MARKER",
	} {
		if marker := os.Getenv(environment); marker != "" {
			_ = os.WriteFile(marker, []byte("invoked\n"), 0o600)
			os.Exit(97)
		}
	}
	os.Exit(m.Run())
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
	git(t, fixture.work, "config", "remote.origin.uploadpack", executable)

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
		gitDir := git(t, fixture.work, "rev-parse", "--absolute-git-dir")
		fetchHead := filepath.Join(gitDir, "FETCH_HEAD")
		if err := os.Remove(fetchHead); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove FETCH_HEAD: %v", err)
		}
		dirtied := make(chan error, 1)
		go func() {
			for {
				if _, err := os.Stat(fetchHead); err == nil {
					dirtied <- os.WriteFile(filepath.Join(fixture.work, "late-untracked.txt"), []byte("dirty\n"), 0o600)
					return
				}
				time.Sleep(time.Millisecond)
			}
		}()

		result := onlyResult(t, execute(t, Target{Path: fixture.work}))
		select {
		case err := <-dirtied:
			if err != nil {
				t.Fatalf("create dirty file after fetch started: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("fetch did not create FETCH_HEAD")
		}
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
