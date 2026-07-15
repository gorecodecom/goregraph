# Safe Git Updates Before Scans

**Status:** Approved design
**Date:** 2026-07-15
**Issue:** [#26](https://github.com/gorecodecom/goregraph/issues/26)

## Goal

Add explicit, conservative Git update commands for one repository and for every
unique Git repository in a GoreGraph workspace. Updating remains separate from
scanning so network access and checkout changes occur only after the user adds
`--execute`.

## Public CLI

The single-repository commands are:

```bash
goregraph git update [path]
goregraph git update [path] --execute
goregraph git update [path] --format text
goregraph git update [path] --format json
```

The workspace commands are:

```bash
goregraph workspace git update [path]
goregraph workspace git update [path] --execute
goregraph workspace git update [path] --format text
goregraph workspace git update [path] --format json
```

`text` is the default format. Any other format is a usage error.

The existing `goregraph update` command keeps its current meaning: refresh the
current project's GoreGraph scan. The explicit `git` namespace prevents the new
network-capable operation from being confused with scanning.

## Safety Invariants

Without `--execute`, the command is a strictly local preview:

- no network access;
- no fetch or remote-reference update;
- no branch or checkout change;
- no index or working-tree change;
- no Git hooks or project code;
- no GoreGraph scan or generated-output write.

Preview commands use only read-only Git inspection and set
`GIT_OPTIONAL_LOCKS=0`. A `would_update` result is based on the currently cached
`origin/<branch>` reference and says so in its reason.

With `--execute`, GoreGraph may fetch, switch branches, create a missing local
tracking branch, and fast-forward the resolved default branch. It must never:

- stash files;
- reset or discard files;
- force-switch or force-update a branch;
- rebase;
- create a merge commit;
- continue or abort an existing Git operation;
- execute repository hooks or project code.

Every Git process is started directly without a shell. Mutating Git commands use
an empty temporary hooks directory through `-c core.hooksPath=<empty-directory>`.
The temporary directory is outside the target repository and is removed after the
operation.

## Repository Discovery and Deduplication

`goregraph git update [path]` resolves the containing Git root. A readable path
that is not inside a Git repository produces a structured `not_git` result.

`goregraph workspace git update [path]` reuses GoreGraph workspace discovery to
obtain project paths, resolves the Git root for each path, and deduplicates roots
by their canonical absolute path. Multiple workspace projects inside one monorepo
therefore produce one Git update result.

Repository processing order is deterministic by canonical Git-root path. A
blocked or failed repository never prevents later repositories from being
inspected or updated.

## Default Branch Resolution

The target branch is resolved independently for every repository:

1. use the branch referenced by `refs/remotes/origin/HEAD` when it points to an
   existing `origin/<branch>` reference;
2. otherwise use `origin/main` or `origin/master` only when exactly one exists;
3. otherwise report `default_branch_unknown` with remediation.

Execution fetches the inspected `origin` URL with an explicit remote-heads
refspec into `refs/remotes/origin/*`, prunes only that namespace, and repeats
default-branch and repository-state inspection before changing the checkout.
Preview never fetches.

When the resolved remote default branch has no local branch, execution may create
an exact tracking branch from `origin/<branch>`. It must not guess another remote
or branch.

## Blocking Conditions

A repository remains unchanged and receives an actionable result when any of
these conditions applies:

- tracked, untracked, staged, or unstaged changes: `dirty`;
- merge, rebase, cherry-pick, or revert in progress: `operation_in_progress`;
- detached HEAD: `detached_head`;
- current branch or target default branch ahead of its relevant remote reference:
  `ahead`;
- current branch or target default branch diverged from its relevant remote
  reference: `diverged`;
- target branch checked out in another worktree: `blocked_worktree`;
- path outside a Git repository: `not_git`;
- missing `origin` remote: `missing_remote`;
- no unambiguous default branch: `default_branch_unknown`;
- authentication, connectivity, or fetch failure: `fetch_failed`.

A clean current branch without an upstream does not by itself block switching to
the resolved default branch. Ahead and diverged checks apply whenever the
relevant comparison reference exists.

## Execution Sequence

For each eligible repository, execution performs:

1. resolve and canonicalize the Git root;
2. inspect the working tree, active operation, HEAD, remotes, branches, commits,
   and worktrees;
3. block without mutation when local inspection finds an unsafe state;
4. fetch the inspected `origin` URL with the explicit refspec
   `+refs/heads/*:refs/remotes/origin/*`, `--prune`, `--no-prune-tags`,
   `--no-tags`, `--no-recurse-submodules`, and `--no-auto-maintenance`, with
   hooks, terminal prompts, and credential-manager interaction disabled, and
   with SSH/scp batch mode and strict host-key checking when applicable;
5. repeat branch resolution and all safety checks against the fetched refs;
6. with submodule recursion disabled, switch to the resolved local default
   branch, or create its exact tracking branch when it does not exist;
7. run `git merge --ff-only origin/<branch>` with hooks disabled;
8. inspect and report the final branch and commit.

The fetch happens before any checkout change. A fetch failure leaves the checkout
unchanged and processing continues with the next workspace repository.

## Result Model

Text and JSON are rendered from the same structured report. Each repository result
contains:

- `path`: requested project or workspace member path;
- `git_root`: canonical repository root;
- `remote`: resolved `origin` URL;
- `branch_before`: branch before execution or preview;
- `branch_after`: resolved target branch for preview, actual branch after execute;
- `commit_before`: checked-out commit before processing;
- `commit_after`: cached target commit in preview, actual checked-out commit after
  execution;
- `status`: stable status value;
- `reason`: concise evidence for the classification;
- `remediation`: concrete next action for blockers;
- `executed`: whether at least one mutating Git process was started for this
  repository. Preflight blockers remain `false`; a started fetch that fails is
  `true`.

The report also contains its mode (`preview` or `execute`), optional workspace
root, deterministic repository results, and counts grouped by status.

Stable successful statuses are:

- `up_to_date`;
- `would_update`;
- `updated`.

Stable blocker or failure statuses are:

- `dirty`;
- `ahead`;
- `diverged`;
- `not_git`;
- `missing_remote`;
- `blocked_worktree`;
- `detached_head`;
- `operation_in_progress`;
- `default_branch_unknown`;
- `fetch_failed`.

Text output names the repository, remote, before/after branch and commit, status,
reason, and remediation. JSON uses the same field values without terminal-only
wording.

## Exit Codes

- `0`: every repository is `up_to_date`, `would_update`, or `updated`;
- `1`: at least one repository is blocked or failed, including partial workspace
  success;
- `2`: invalid command syntax or options.

Workspace execution always processes all deduplicated repositories before
returning the aggregate exit code.

## Internal Architecture

A focused `internal/gitupdate` package owns repository inspection, planning,
execution, structured records, and text/JSON rendering inputs. It does not depend
on CLI writers and does not scan source code.

The CLI layer owns:

- parsing `git update` and `workspace git update` arguments;
- obtaining workspace project paths through existing workspace discovery;
- choosing text or JSON rendering;
- writing stdout/stderr;
- mapping the aggregate result to an exit code.

This boundary keeps all security-critical Git behavior testable without invoking
the complete CLI while still allowing end-to-end CLI coverage.

## Testing

Integration tests use real temporary Git repositories and local bare remotes.
They require no internet access and do not mock Git output. Test repositories set
their own author identity and do not depend on the developer's global Git config.

Coverage includes:

- `origin/HEAD` default-branch resolution;
- unambiguous `origin/main` and `origin/master` fallback;
- ambiguous or missing default branch;
- preview without fetch, reference, checkout, index, or working-tree mutation;
- successful fetch and fast-forward;
- safe creation of a missing local tracking branch;
- dirty tracked and untracked files;
- ahead and diverged history;
- detached HEAD;
- merge, rebase, cherry-pick, and revert state;
- missing origin;
- fetch and authentication-equivalent failure from an unreachable local remote;
- target branch checked out in another worktree;
- workspace Git-root deduplication;
- partial workspace success with continued processing;
- deterministic text and JSON semantics;
- an intentionally failing checkout or merge hook that proves hooks are disabled;
- CLI help, invalid format, and exit-code behavior.

Final verification runs:

```bash
gofmt -l .
go vet ./...
go test ./...
```

## Documentation

`COMMANDS.md` documents both command forms, preview versus execution, output
fields, status values, exit codes, and the safe update-then-scan workflow.

`README.md` adds concise single-project and workspace examples. Its Windows Scoop
section explains how to install Scoop from a regular, non-administrative
PowerShell when it is missing:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression
```

The existing GoreGraph bucket and install commands follow that bootstrap step.
The README links to the
[official Scoop installer documentation](https://github.com/ScoopInstaller/Install).

## Version and Release Boundary

The new public Git commands are backward-compatible functionality, so the source
release target becomes `1.3.0`. The version change is a separate commit after the
feature implementation and updates `internal/version`, the README source target,
and `docs/RELEASE.md`.

This work must not publish a release. It must not create or push a `v1.3.0` tag,
run a release workflow, create a GitHub Release, update Homebrew or Scoop release
artifacts, or publish Winget metadata. Additional work is expected before the
eventual `1.3.0` release.
