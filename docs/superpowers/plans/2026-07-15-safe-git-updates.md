# Safe Git Updates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add explicit, preview-first Git update commands for a project and every unique repository in a GoreGraph workspace.

**Architecture:** A new `internal/gitupdate` package owns read-only repository inspection, deterministic planning, hook-free fetch/switch/fast-forward execution, deduplication, and structured results. A focused CLI file maps the new top-level and workspace command forms to that package and renders the same report as text or JSON.

**Tech Stack:** Go 1.23+, standard library, real local Git repositories in tests, existing GoreGraph workspace discovery.

## Global Constraints

- `goregraph git update [path]` and `goregraph workspace git update [path]` are previews unless `--execute` is present.
- Preview performs no network access and changes no Git or project state.
- Execution never stashes, resets, rebases, force-switches, creates merge commits, executes hooks, or runs project code.
- Workspace Git roots are canonicalized, deduplicated, and processed deterministically; one blocker does not stop later repositories.
- Text and JSON use the same structured status, reason, remediation, branch, commit, remote, and path values.
- No third-party Go dependency is added.
- Work is committed directly on `main` as requested by the user.
- The source target becomes `1.3.0` in a separate commit after the feature and documentation are complete.
- Do not create or push a `v1.3.0` tag, trigger a release workflow, create a GitHub Release, or publish Homebrew, Scoop, or Winget artifacts.

---

## File Structure

- Create `internal/gitupdate/types.go`: public report, result, status, mode, target, and options types plus aggregate exit-code logic.
- Create `internal/gitupdate/git.go`: direct `git` process execution, environment, canonical-root resolution, ref and count helpers.
- Create `internal/gitupdate/inspect.go`: read-only repository-state inspection, default-branch resolution, operation detection, worktree parsing, and classification.
- Create `internal/gitupdate/update.go`: target deduplication, preview planning, safe execution, hook suppression, continuation, and summary construction.
- Create `internal/gitupdate/gitupdate_test.go`: real local Git fixtures and package-level preview/execution/safety tests.
- Create `internal/cli/git_update.go`: argument parsing, single/workspace dispatch, text rendering, JSON encoding, and exit-code mapping.
- Create `internal/cli/git_update_test.go`: end-to-end command, workspace, output, help, and usage tests.
- Modify `internal/cli/cli.go`: register `git` and `workspace git` dispatch and update global/workspace help.
- Modify `COMMANDS.md`: complete command reference, statuses, exit codes, and update-then-scan flow.
- Modify `README.md`: concise Git update examples and Scoop bootstrap instructions.
- Modify `docs/RELEASE.md`, `internal/version/version.go`, `internal/cli/cli_test.go`, and `release_files_test.go`: change only the source target assertions from `1.2.0` to `1.3.0` and describe the unreleased target.

---

### Task 1: Structured Preview and Repository Classification

**Files:**
- Create: `internal/gitupdate/types.go`
- Create: `internal/gitupdate/git.go`
- Create: `internal/gitupdate/inspect.go`
- Create: `internal/gitupdate/update.go`
- Create: `internal/gitupdate/gitupdate_test.go`

**Interfaces:**
- Produces: `func Run(ctx context.Context, targets []Target, options Options) (Report, error)`
- Produces: `type Target struct { Path string }`
- Produces: `type Options struct { Execute bool; WorkspaceRoot string }`
- Produces: `type Report struct { Mode Mode; WorkspaceRoot string; Repositories []RepositoryResult; Summary map[Status]int }`
- Produces: `func (Report) ExitCode() int`
- Produces stable `Mode`, `Status`, and `RepositoryResult` JSON fields consumed by CLI work in Task 4.

- [ ] **Step 1: Write the failing report-model tests**

Add exact status constants and exit-code expectations before creating production types:

```go
func TestReportExitCode(t *testing.T) {
	tests := []struct {
		name    string
		status  Status
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
```

- [ ] **Step 2: Run the model test and verify RED**

Run:

```bash
go test ./internal/gitupdate -run TestReportExitCode -count=1
```

Expected: compilation fails because `Status`, `Report`, and `RepositoryResult` do not exist.

- [ ] **Step 3: Add the minimal stable result model**

Implement these exact exported fields in `types.go`:

```go
type Mode string
type Status string

const (
	ModePreview Mode = "preview"
	ModeExecute Mode = "execute"

	StatusUpToDate          Status = "up_to_date"
	StatusWouldUpdate       Status = "would_update"
	StatusUpdated           Status = "updated"
	StatusDirty             Status = "dirty"
	StatusAhead             Status = "ahead"
	StatusDiverged          Status = "diverged"
	StatusNotGit            Status = "not_git"
	StatusMissingRemote     Status = "missing_remote"
	StatusBlockedWorktree   Status = "blocked_worktree"
	StatusDetachedHead      Status = "detached_head"
	StatusOperationProgress Status = "operation_in_progress"
	StatusDefaultUnknown    Status = "default_branch_unknown"
	StatusFetchFailed       Status = "fetch_failed"
)

type Target struct {
	Path string
}

type Options struct {
	Execute       bool
	WorkspaceRoot string
}

type RepositoryResult struct {
	Path          string `json:"path"`
	GitRoot       string `json:"git_root,omitempty"`
	Remote        string `json:"remote,omitempty"`
	BranchBefore  string `json:"branch_before,omitempty"`
	BranchAfter   string `json:"branch_after,omitempty"`
	CommitBefore  string `json:"commit_before,omitempty"`
	CommitAfter   string `json:"commit_after,omitempty"`
	Status        Status `json:"status"`
	Reason        string `json:"reason"`
	Remediation   string `json:"remediation,omitempty"`
	Executed      bool   `json:"executed"`
}

type Report struct {
	Mode          Mode               `json:"mode"`
	WorkspaceRoot string             `json:"workspace_root,omitempty"`
	Repositories  []RepositoryResult `json:"repositories"`
	Summary       map[Status]int     `json:"summary"`
}
```

`ExitCode` returns `0` only when every result is `up_to_date`, `would_update`, or `updated`; every other status returns `1`.

- [ ] **Step 4: Run the model test and verify GREEN**

Run:

```bash
go test ./internal/gitupdate -run TestReportExitCode -count=1
```

Expected: PASS.

- [ ] **Step 5: Add a real local Git fixture and failing preview tests**

Create a `gitFixture` helper in `gitupdate_test.go` that:

- initializes a bare remote with `git init --bare --initial-branch=main`;
- clones it into `work` and `peer` directories;
- configures `user.name` and `user.email` locally in both clones;
- creates and pushes an initial commit from `work`;
- fetches the commit into `peer`;
- exposes helpers `git(t, dir, args...)`, `commitAndPushFromPeer`, and `revParse`.

Add separate tests with exact assertions:

```go
func TestRunPreviewReportsWouldUpdateWithoutChangingRepository(t *testing.T)
func TestRunPreviewFallsBackToUnambiguousMain(t *testing.T)
func TestRunPreviewBlocksDirtyTrackedAndUntrackedFiles(t *testing.T)
func TestRunPreviewBlocksDetachedHead(t *testing.T)
func TestRunPreviewBlocksOperationInProgress(t *testing.T)
func TestRunPreviewBlocksAheadAndDivergedBranches(t *testing.T)
func TestRunPreviewReportsMissingRemoteAndUnknownDefault(t *testing.T)
func TestRunPreviewBlocksTargetBranchInAnotherWorktree(t *testing.T)
```

The first test records HEAD, current branch, `refs/remotes/origin/main`, status output, and file contents before `Run`, then asserts all five remain byte-for-byte identical afterward. Prepare `would_update` by fetching a peer commit before recording state so no network access is required during `Run`.

- [ ] **Step 6: Run preview tests and verify RED**

Run:

```bash
go test ./internal/gitupdate -run 'TestRunPreview' -count=1
```

Expected: tests fail because `Run` and repository inspection are not implemented.

- [ ] **Step 7: Implement direct read-only Git inspection**

In `git.go`, implement `runGit` with `exec.CommandContext`, `cmd.Dir`, and this environment rule:

```go
cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
```

Never invoke a shell. Return trimmed stdout and an error containing trimmed stderr.

In `inspect.go`, implement:

- canonical root from `git rev-parse --show-toplevel`, `filepath.Abs`, `filepath.EvalSymlinks`, and `filepath.Clean`;
- current branch from `git symbolic-ref --quiet --short HEAD`;
- HEAD from `git rev-parse HEAD`;
- dirty state from non-empty `git status --porcelain=v1 --untracked-files=all`;
- active operations from `git rev-parse --absolute-git-dir` plus `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-apply`, and `rebase-merge`;
- origin URL from `git remote get-url origin`;
- default branch from `refs/remotes/origin/HEAD`, then exactly one of `origin/main` and `origin/master`;
- current/target branch ahead and divergence from `git rev-list --left-right --count <local>...<remote>`;
- target-branch worktree conflicts from `git worktree list --porcelain`.

Return the blocker statuses and remediation strings defined in the design before returning any successful preview status. Use cached remote refs only.

In `update.go`, canonicalize and sort roots, deduplicate them, classify preview as `up_to_date` only when checkout and target commit already match, otherwise `would_update`, and build deterministic summary counts.

- [ ] **Step 8: Run preview tests and verify GREEN**

Run:

```bash
go test ./internal/gitupdate -run 'TestRunPreview|TestReportExitCode' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit the preview unit**

```bash
git add internal/gitupdate
git commit -m "Add safe Git update previews" -m "- Inspect repository state without network or checkout mutations
- Report deterministic update plans and actionable blockers"
```

---

### Task 2: Hook-Free Fetch, Switch, and Fast-Forward Execution

**Files:**
- Modify: `internal/gitupdate/git.go`
- Modify: `internal/gitupdate/inspect.go`
- Modify: `internal/gitupdate/update.go`
- Modify: `internal/gitupdate/gitupdate_test.go`

**Interfaces:**
- Consumes: `Run`, `Options.Execute`, and result types from Task 1.
- Produces: safe execution for existing and missing local default branches.
- Produces: `executed=true` once fetch starts, including `fetch_failed`.

- [ ] **Step 1: Write failing execution tests**

Add these real-repository tests:

```go
func TestRunExecuteFetchesSwitchesAndFastForwards(t *testing.T)
func TestRunExecuteCreatesMissingTrackingBranch(t *testing.T)
func TestRunExecuteReportsUpToDateAfterFetch(t *testing.T)
func TestRunExecuteReportsFetchFailureWithoutCheckoutChange(t *testing.T)
func TestRunExecuteDisablesCheckoutAndMergeHooks(t *testing.T)
func TestRunExecuteRechecksDirtyAheadAndDivergedAfterFetch(t *testing.T)
```

The hook test installs executable `post-checkout` and `post-merge` hooks that write a marker and exit non-zero. Execution must succeed and the marker must not exist. The fetch-failure test points `origin` to a missing local bare repository and asserts branch, HEAD, status, and files are unchanged.

- [ ] **Step 2: Run execution tests and verify RED**

Run:

```bash
go test ./internal/gitupdate -run 'TestRunExecute' -count=1
```

Expected: FAIL because execute still returns preview-only results.

- [ ] **Step 3: Implement hook-free mutation commands**

When `Options.Execute` is true:

1. create one empty temporary hooks directory with `os.MkdirTemp("", "goregraph-git-hooks-")` and defer `os.RemoveAll`;
2. run fetch with `git -c core.hooksPath=<dir> fetch --prune origin` and `GIT_TERMINAL_PROMPT=0`;
3. set `Executed=true` immediately before starting fetch;
4. return `fetch_failed` with remediation on fetch error and continue later targets;
5. repeat the full inspection and blockers after fetch;
6. switch with `git -c core.hooksPath=<dir> switch <branch>` when the local target exists;
7. otherwise create the exact tracking branch with `git -c core.hooksPath=<dir> switch --track -c <branch> origin/<branch>`;
8. fast-forward with `git -c core.hooksPath=<dir> merge --ff-only origin/<branch>`;
9. inspect the final branch and HEAD;
10. return `updated` when branch or commit changed, otherwise `up_to_date`.

Any unexpected switch or merge refusal returns the narrowest existing blocker status supported by fresh inspection; otherwise return `fetch_failed` with the Git error in `reason` and manual remediation. Do not add a force fallback.

- [ ] **Step 4: Run execution tests and verify GREEN**

Run:

```bash
go test ./internal/gitupdate -run 'TestRunExecute|TestRunPreview|TestReportExitCode' -count=1
```

Expected: PASS with no hook marker created.

- [ ] **Step 5: Run the entire package and format it**

```bash
gofmt -w internal/gitupdate
go test ./internal/gitupdate -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit safe execution**

```bash
git add internal/gitupdate
git commit -m "Execute conservative Git updates" -m "- Fetch and fast-forward resolved default branches without hooks
- Preserve repository state for blockers and fetch failures"
```

---

### Task 3: Workspace Deduplication and Partial Success

**Files:**
- Modify: `internal/gitupdate/update.go`
- Modify: `internal/gitupdate/gitupdate_test.go`

**Interfaces:**
- Consumes: multiple `Target` values and `Options.WorkspaceRoot`.
- Produces: one result per canonical Git root, sorted by root.
- Produces: aggregate summaries and exit code after every repository is processed.

- [ ] **Step 1: Write failing multi-target tests**

Add:

```go
func TestRunDeduplicatesCanonicalGitRoots(t *testing.T)
func TestRunProcessesSafeRepositoriesAfterBlocker(t *testing.T)
func TestRunSortsRepositoryResultsByCanonicalRoot(t *testing.T)
```

The deduplication test passes the repository root and two nested project paths from the same checkout and expects exactly one result. The continuation test passes one dirty repository and one behind repository with `Execute: true`, expects `dirty` and `updated`, and expects aggregate exit code `1`.

- [ ] **Step 2: Run multi-target tests and verify RED**

```bash
go test ./internal/gitupdate -run 'TestRun(Deduplicates|ProcessesSafe|Sorts)' -count=1
```

Expected: FAIL until canonical deduplication and continuation are complete.

- [ ] **Step 3: Implement deterministic multi-target processing**

Resolve every requested path independently. Preserve `not_git` results instead of returning early. Group successful roots by canonical absolute root, keep the lexicographically first requested path as `RepositoryResult.Path`, sort canonical roots, process all roots, and build `Summary` only after the final result slice is complete.

Unexpected infrastructure errors that prevent the entire run, such as failure to create the empty hooks directory, return a Go error. Repository-specific errors remain structured results.

- [ ] **Step 4: Run package tests and verify GREEN**

```bash
gofmt -w internal/gitupdate
go test ./internal/gitupdate -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit workspace-safe processing**

```bash
git add internal/gitupdate
git commit -m "Handle workspace Git updates independently" -m "- Deduplicate canonical repository roots in deterministic order
- Continue safe updates when another repository is blocked"
```

---

### Task 4: CLI Dispatch, Text/JSON Output, and Help

**Files:**
- Create: `internal/cli/git_update.go`
- Create: `internal/cli/git_update_test.go`
- Modify: `internal/cli/cli.go`

**Interfaces:**
- Consumes: `gitupdate.Run`, `Target`, `Options`, `Report.ExitCode`.
- Consumes: `scan.WorkspaceProjectScanPlan(root, cfg)` for workspace project paths.
- Produces: `runGit(args, stdout, stderr)` and `runWorkspaceGit(args, stdout, stderr)`.
- Produces: text and indented JSON output for the same `gitupdate.Report`.

- [ ] **Step 1: Write failing single-repository CLI tests**

Add:

```go
func TestRunGitUpdateDefaultsToPreview(t *testing.T)
func TestRunGitUpdateExecuteUpdatesRepository(t *testing.T)
func TestRunGitUpdateJSONMatchesStructuredResult(t *testing.T)
func TestRunGitUpdateRejectsUnknownFormatAndOptions(t *testing.T)
func TestRunGitUpdateHelpDocumentsExecuteAndFormat(t *testing.T)
```

Assert that text output includes mode, root, remote, before/after branch and commit, status, reason, and remediation. Decode JSON into `gitupdate.Report` and assert the same values rather than matching raw JSON text.

- [ ] **Step 2: Write failing workspace CLI tests**

Build a temporary workspace containing two discovered projects in one Git root and one project in a nested independent Git root. Add:

```go
func TestRunWorkspaceGitUpdateDeduplicatesRepositories(t *testing.T)
func TestRunWorkspaceGitUpdateContinuesAfterDirtyRepository(t *testing.T)
func TestRunWorkspaceGitUpdateJSONReportsPartialSuccess(t *testing.T)
func TestRunWorkspaceGitUpdateHelpDocumentsPreview(t *testing.T)
```

Expect two results, continued execution, and exit code `1` for partial success.

- [ ] **Step 3: Run CLI tests and verify RED**

```bash
go test ./internal/cli -run 'TestRun(Git|WorkspaceGit)' -count=1
```

Expected: FAIL because the new dispatch does not exist.

- [ ] **Step 4: Implement parsing and dispatch**

Register top-level `git` in `Run` and nested `git` in `runWorkspace`. Both require the next token `update`; missing or unknown subcommands return `2`.

Parse these exact options in any order:

```text
[path] --execute --format text|json
```

Reject multiple positional paths, missing format values, unknown formats, and unknown flags with exit code `2`.

For workspace mode, load normal config, force workspace discovery, call `scan.WorkspaceProjectScanPlan`, convert every item to `gitupdate.Target`, and pass the plan's workspace root through `Options.WorkspaceRoot`.

Render JSON with:

```go
encoder := json.NewEncoder(stdout)
encoder.SetIndent("", "  ")
```

Render text from the report only; never recalculate status or summary in the renderer. Return `report.ExitCode()` after successful rendering.

- [ ] **Step 5: Update global and workspace help**

Add `git update` to global help, `workspace git update` to workspace help, and examples showing preview before `--execute`.

- [ ] **Step 6: Run focused CLI tests and verify GREEN except documentation**

```bash
gofmt -w internal/cli internal/gitupdate
go test ./internal/cli -run 'TestRun(Git|WorkspaceGit)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit CLI behavior**

```bash
git add internal/cli internal/gitupdate
git commit -m "Expose safe Git update commands" -m "- Add preview-first project and workspace CLI operations
- Render consistent text and JSON results with clear exit codes"
```

---

### Task 5: Command, README, and Scoop Documentation

**Files:**
- Modify: `COMMANDS.md`
- Modify: `README.md`
- Modify: `docs_test.go`

**Interfaces:**
- Documents the exact Task 4 CLI and Task 1 status model.
- Uses current official Scoop bootstrap guidance from `https://github.com/ScoopInstaller/Install`.

- [ ] **Step 1: Extend documentation coverage and verify RED**

Add `goregraph git update` and `goregraph workspace git update` to `TestCommandsReferenceDocumentsEveryUserCommand`, then run:

```bash
go test . -run TestCommandsReferenceDocumentsEveryUserCommand -count=1
```

Expected: FAIL because `COMMANDS.md` does not yet contain both new command strings.

- [ ] **Step 2: Add the full command reference**

Add sections for:

```bash
goregraph git update .
goregraph git update . --execute
goregraph git update . --format json
goregraph workspace git update .
goregraph workspace git update . --execute
goregraph workspace git update . --format json
```

Document strict local preview semantics, cached remote-ref limitations, execution sequence, every stable status, fields, exit codes `0/1/2`, partial success, hook suppression, and the recommended flow:

```bash
goregraph workspace git update .
goregraph workspace git update . --execute
goregraph workspace scan-all .
```

- [ ] **Step 3: Update README usage and Scoop bootstrap**

Add a concise safe-update section near Quick Start and workspace scanning. In the existing Scoop section, add the non-admin PowerShell bootstrap before bucket setup:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression
```

Link `official Scoop installer documentation` to `https://github.com/ScoopInstaller/Install`, then retain:

```powershell
scoop bucket add gorecode https://github.com/gorecodecom/scoop-bucket
scoop install goregraph
```

- [ ] **Step 4: Run documentation and full Go tests**

```bash
go test . -run TestCommandsReferenceDocumentsEveryUserCommand -count=1
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit documentation**

```bash
git add COMMANDS.md README.md docs_test.go
git commit -m "Document safe Git update workflows" -m "- Explain project and workspace preview and execute commands
- Add official Scoop bootstrap guidance for Windows users"
```

---

### Task 6: Set the Unreleased 1.3.0 Source Target

**Files:**
- Modify: `internal/version/version.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`
- Modify: `docs/RELEASE.md`
- Modify: `release_files_test.go`

**Interfaces:**
- Changes default development build output from `1.2.0` to `1.3.0`.
- Does not change Schema 2 or any release workflow configuration.

- [ ] **Step 1: Update version assertions first**

Change the expected development version in `internal/cli/cli_test.go` and the required release target in `release_files_test.go` from `1.2.0` to `1.3.0`.

- [ ] **Step 2: Run version and release-file tests and verify RED**

```bash
go test ./internal/cli -run TestRunVersion -count=1
go test . -run TestMilestone6ReleaseFilesAreConfigured -count=1
```

Expected: FAIL because production version and release documentation still say `1.2.0`.

- [ ] **Step 3: Update the source target without publishing**

Set:

```go
Version = "1.3.0"
```

Update the README current source target to `v1.3.0`. Replace the Current Release section in `docs/RELEASE.md` with an unreleased `v1.3.0` source target that lists safe project/workspace Git updates and explicitly says tags, GitHub Releases, Homebrew, Scoop, and Winget publication remain pending. Preserve the completed `1.2.0` milestone history below it.

- [ ] **Step 4: Run version, release, and full tests**

```bash
gofmt -w internal/version/version.go internal/cli/cli_test.go release_files_test.go
go test ./internal/cli -run TestRunVersion -count=1
go test . -run TestMilestone6ReleaseFilesAreConfigured -count=1
go test ./...
```

Expected: PASS and `goregraph version` development output contains `goregraph 1.3.0`.

- [ ] **Step 5: Commit the source target separately**

```bash
git add internal/version/version.go internal/cli/cli_test.go README.md docs/RELEASE.md release_files_test.go
git commit -m "Set the 1.3.0 source target" -m "- Mark safe Git updates as unreleased 1.3.0 functionality
- Keep release tags and package publication explicitly pending"
```

---

### Task 7: Final Verification, Main Push, and Issue Closure

**Files:**
- Verify all files changed in Tasks 1-6.
- Do not create release files, tags, or artifacts.

**Interfaces:**
- Produces a clean `main` synchronized with `origin/main`.
- Closes GitHub issue #26 only after the push and remote-state verification succeed.

- [ ] **Step 1: Verify the acceptance checklist against the design**

Confirm every status, field, blocker, command form, exit code, documentation section, and no-release constraint from `docs/superpowers/specs/2026-07-15-safe-git-updates-design.md` has a corresponding implementation or test.

- [ ] **Step 2: Run the exact CI checks fresh**

```bash
gofmt -l .
go vet ./...
go test ./...
git diff --check
git status -sb
```

Expected: no formatting output, vet exit `0`, all tests pass, no diff errors, and a clean `main` ahead only by the intended commits.

- [ ] **Step 3: Verify the installed development binary behavior**

```bash
go build -o /tmp/goregraph-1.3.0 ./cmd/goregraph
/tmp/goregraph-1.3.0 version
/tmp/goregraph-1.3.0 git update --help
/tmp/goregraph-1.3.0 workspace git update --help
```

Expected: version `1.3.0`, both help commands exit `0`, and no repository mutation.

- [ ] **Step 4: Push main without publishing a release**

```bash
git push origin main
```

Do not push any tag and do not invoke GitHub release or package-manager publication commands.

- [ ] **Step 5: Verify remote main and close issue #26**

```bash
git fetch origin main
git rev-parse main
git rev-parse origin/main
gh issue close 26 --repo gorecodecom/goregraph --reason completed --comment "Implemented directly on main with preview-first project and workspace Git updates. Verified with gofmt, go vet ./..., go test ./..., and development-binary smoke checks. The 1.3.0 source target is set; no release or package publication was performed."
gh issue view 26 --repo gorecodecom/goregraph --json state,stateReason,url
```

Expected: local and remote commit IDs match; issue state is `CLOSED` with reason `COMPLETED`.
