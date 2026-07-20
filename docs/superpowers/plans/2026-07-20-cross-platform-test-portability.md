# Cross-Platform Test Portability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the GoreGraph test and CI workflow reliable on Windows, macOS, and Linux without weakening production safety checks.

**Architecture:** Keep production behavior unchanged because the source-path and dashboard failures were caused by the Codex filesystem sandbox, not Windows itself. Fix platform assumptions in tests, reduce Git fixture process overhead, feed large Node scripts through standard input, and run CI on all supported desktop platforms.

**Tech Stack:** Go 1.23, Git, Node.js, GitHub Actions

## Global Constraints

- Support Windows, macOS, and Linux.
- Do not add dependencies.
- Preserve all GoreGraph command behavior and security checks.
- Use capability checks for optional filesystem features such as symlink creation.
- Keep test assertions independent of checkout line endings.
- Do not create a release or tag.

---

### Task 1: Normalize text assertions

**Files:**
- Modify: `docs_test.go`
- Modify: `internal/agent/context_test.go`

**Interfaces:**
- Consumes: text files checked out with either LF or CRLF line endings
- Produces: assertions over normalized LF text

- [ ] **Step 1: Verify the existing RED failures**

Run:

```powershell
go test . -run 'TestDocumentationUsesSourceBackedAssistedInstruction|TestREADMELaterWorkspaceDashboardReferenceDocumentsEditMode' -count=1
go test ./internal/agent -run TestContextMaxFilesSharesExportedMinimum -count=1
```

Expected: FAIL because the tests search for LF-only multi-line strings in CRLF working-tree files.

- [ ] **Step 2: Normalize file contents before multi-line assertions**

Add this test helper to `docs_test.go` and use it in the two failing tests:

```go
func normalizedFileContents(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}
```

Normalize the `context.go` source string in `TestContextMaxFilesSharesExportedMinimum` with the same `strings.ReplaceAll` operation.

- [ ] **Step 3: Verify GREEN**

Run the two commands from Step 1.

Expected: PASS on Windows; the same assertions remain valid on macOS and Linux.

### Task 2: Treat filesystem capabilities as capabilities

**Files:**
- Modify: `internal/agent/context_source_test.go`
- Modify: `internal/agent/context_test.go`
- Modify: `internal/gitupdate/gitupdate_test.go`

**Interfaces:**
- Consumes: `os.Symlink` and Unix permission-bit behavior when available
- Produces: full security coverage on capable systems and explicit skips on systems that cannot create symlinks or enforce Unix mode bits

- [ ] **Step 1: Verify the existing RED failures**

Run:

```powershell
go test ./internal/agent -run 'TestResolveSourcePathRejectsUnsafePaths|TestResolveSourcePathConfinesWorkspaceCandidatesToProjectRoot|TestBuildContextSourceOperationalFailuresBecomeStableOmissions' -count=1
go test ./internal/gitupdate -run TestRunDeduplicatesSymlinkAliases -count=1
```

Expected: FAIL on Windows without Developer Mode because symlink creation requires a privilege; the unreadable-file case also fails because Windows ignores Unix mode `000`.

- [ ] **Step 2: Skip only unavailable capability cases**

Change the two agent symlink setup failures to `t.Skipf`, matching existing scan and dashboard-editor tests. In the unreadable-file subtest, skip only that subtest when `runtime.GOOS == "windows"`.

Add this helper to `internal/gitupdate/gitupdate_test.go` and use it for every test-created symlink:

```go
func symlinkOrSkip(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink creation is not permitted in this environment: %v", err)
	}
}
```

Replace the hard-coded `/tmp` barrier directory with `t.TempDir()`.

- [ ] **Step 3: Verify GREEN**

Run the commands from Step 1.

Expected: PASS with explicit capability skips on restricted Windows; security assertions execute on macOS, Linux, and Windows systems with symlink support.

### Task 3: Reduce Git fixture overhead

**Files:**
- Modify: `internal/gitupdate/gitupdate_test.go`
- Modify: `internal/cli/git_update_test.go`

**Interfaces:**
- Consumes: local Git executable
- Produces: equivalent work, peer, and bare-remote repositories with fewer subprocesses and deterministic identity/configuration

- [ ] **Step 1: Record the existing RED timing**

Run:

```powershell
go test ./internal/gitupdate -count=1
go test ./internal/cli -run 'GitUpdate|WorkspaceGitUpdate' -count=1
```

Expected on the affected Windows host: `internal/gitupdate` reaches the default ten-minute package timeout; the CLI integration tests consume most of the same limit.

- [ ] **Step 2: Build initialized repositories instead of cloning two empty repositories**

For each fixture helper:

1. Initialize the bare remote with `--initial-branch=main`.
2. Initialize the work repository directly with `--initial-branch=main`.
3. Add `origin` with `git remote add -m main origin <remote>`.
4. Commit and push the initial file.
5. Clone the populated remote once to create the peer.

Set author and committer identity through the test Git command environment so separate `git config user.*` subprocesses are unnecessary. Disable hooks and automatic CRLF conversion for fixture commands with command-local Git configuration.

Create one immutable initialized Git fixture template per test package, copy it into an isolated temporary directory for each test, and use relative `origin` URLs so every copied work, peer, and remote repository stays independent. Keep the tests serial because several cases intentionally mutate process environment variables.

- [ ] **Step 3: Verify behavior and timing**

Run the commands from Step 1.

Expected: PASS within Go's default ten-minute timeout on Windows, with unchanged update classifications and safety behavior.

### Task 4: Execute large Node scripts through standard input

**Files:**
- Create: `internal/scan/node_test.go`
- Modify: `internal/scan/workspace_dashboard_test.go`
- Modify: `internal/scan/workspace_dashboard_architecture_test.go`

**Interfaces:**
- Consumes: Node executable path and JavaScript source
- Produces: `*exec.Cmd` that executes the source from stdin

- [ ] **Step 1: Verify the existing RED failures**

Run:

```powershell
go test ./internal/scan -run 'TestWorkspaceDashboardArchitectureMatrixKeepsEveryDiscoveredService|TestWorkspaceDashboardAPICatalogDetailsRuntimeRendersOneDisclosurePerVisibleEndpoint' -count=1
```

Expected on Windows: FAIL with `The filename or extension is too long` from `node -e <large-source>`.

- [ ] **Step 2: Add and use a stdin-based Node command helper**

Create `internal/scan/node_test.go`:

```go
package scan

import (
	"os/exec"
	"strings"
)

func nodeScriptCommand(node, source string) *exec.Cmd {
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(source)
	return cmd
}
```

Replace every `exec.Command(node, "-e", source)` in the two dashboard test files with `nodeScriptCommand(node, source)` so future fixture growth cannot reintroduce the Windows command-line limit.

- [ ] **Step 3: Verify GREEN**

Run the command from Step 1, then all scan tests:

```powershell
go test ./internal/scan -count=1
```

Expected: PASS without changing the tested JavaScript behavior.

### Task 5: Add supported-platform CI coverage

**Files:**
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: GitHub-hosted Ubuntu, Windows, and macOS runners
- Produces: formatting, vet, and full test results for every supported platform

- [ ] **Step 1: Confirm the current CI coverage gap**

Read `.github/workflows/ci.yml`.

Expected: only `ubuntu-latest` is configured.

- [ ] **Step 2: Add the operating-system matrix**

Use this job strategy and cross-shell formatting check:

```yaml
strategy:
  fail-fast: false
  matrix:
    os: [ubuntu-latest, windows-latest, macos-latest]
runs-on: ${{ matrix.os }}
```

Replace the Bash-only formatting expression with `go fmt ./...` followed by `git diff --exit-code`. Run tests with `go test ./... -timeout 20m` so slow hosted Windows Git process startup does not create a false timeout while package-level fixture optimization remains covered by the local default-timeout test.

- [ ] **Step 3: Final verification**

Run:

```powershell
gofmt -w docs_test.go internal/agent/context_test.go internal/agent/context_source_test.go internal/gitupdate/gitupdate_test.go internal/cli/git_update_test.go internal/scan/node_test.go internal/scan/workspace_dashboard_test.go internal/scan/workspace_dashboard_architecture_test.go
go vet ./...
go test ./... -count=1 -timeout 20m
git diff --check
```

Expected: formatting and vet succeed; the complete Windows suite passes; CI is configured to prove the same on macOS and Linux after push.
