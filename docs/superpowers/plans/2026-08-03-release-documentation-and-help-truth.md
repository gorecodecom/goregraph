# Release Documentation and Help Truth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the GoreGraph 1.3.0 CLI help and public documentation exactly match the installed runtime, release workflow, dashboard, commands, and language capability profiles.

**Architecture:** Keep runtime behavior unchanged except for honoring the already documented `-h` help alias on every workspace subcommand. Keep language and release claims source-backed, repair the hand-written command and dashboard descriptions, and guard the public command inventory with existing Go tests.

**Tech Stack:** Go 1.23, standard library CLI, Go tests, Markdown, existing `scripts/sync-docs` generator, GoReleaser v2.15.2.

## Global Constraints

- GoreGraph remains version 1.3.0 with output Schema 3 and remains unreleased until a separate explicit release action.
- Do not change Context Pack behavior, scanning behavior, language analyzers, benchmark metrics, dependencies, or generated output schemas.
- Every public command must accept `help`, `--help`, and `-h` as documented.
- Public documentation must distinguish Full, Integration, and Index language depth exactly as represented by `scan.LanguageCapabilityProfiles`.
- Keep changes minimal, comments in English, and commit CLI behavior separately from documentation corrections.

---

### Task 1: Make workspace short help consistent

**Files:**
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/cli/cli.go`

**Interfaces:**
- Consumes: `cli.Run(args, stdout, stderr)` and the existing command-specific help text.
- Produces: exit code `0`, empty stderr, and a `Usage: goregraph ...` line for `-h` on every public workspace subcommand.

- [ ] **Step 1: Write the failing test**

Add a table-driven `TestWorkspaceSubcommandsSupportShortHelp` covering `refresh`, `dashboard`, `explain`, `path`, `impact`, `diff`, `status`, `scan-missing`, `scan-all`, and `clean`. Invoke each as `workspace <command> -h`; assert exit code `0`, empty stderr, and output containing `Usage: goregraph workspace <command>`.

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
GOTOOLCHAIN=go1.23.12 go test ./internal/cli -run TestWorkspaceSubcommandsSupportShortHelp -count=1
```

Expected: FAIL because the affected subcommands currently treat `-h` as an unknown option.

- [ ] **Step 3: Implement the minimal fix**

Add `"-h"` to the existing help switch case in each affected workspace subcommand. Do not change help text or command behavior.

- [ ] **Step 4: Run focused and package tests**

Run:

```bash
GOTOOLCHAIN=go1.23.12 go test ./internal/cli -run TestWorkspaceSubcommandsSupportShortHelp -count=1
GOTOOLCHAIN=go1.23.12 go test ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```text
Honor short help for workspace commands

- Support -h consistently across every workspace subcommand
- Cover the public short-help contract with a table-driven CLI test
```

### Task 2: Align the public command and README documentation

**Files:**
- Modify: `COMMANDS.md`
- Modify: `README.md`
- Modify: `docs_test.go`

**Interfaces:**
- Consumes: installed CLI command catalog, Go 1.23 release workflows, `scan.LanguageCapabilityProfiles`, and the eight-view dashboard contract.
- Produces: complete, internally consistent release documentation without broken relative links or unsupported language-depth claims.

- [ ] **Step 1: Strengthen the existing command inventory guard**

Add `goregraph workspace diff`, `goregraph workspace explain`, `goregraph workspace path`, and `goregraph workspace impact` to the existing `TestCommandsReferenceDocumentsEveryUserCommand` literal command list.

- [ ] **Step 2: Run the documentation test to verify it fails**

Run:

```bash
GOTOOLCHAIN=go1.23.12 go test . -run TestCommandsReferenceDocumentsEveryUserCommand -count=1
```

Expected: FAIL because `COMMANDS.md` does not yet document `workspace diff`.

- [ ] **Step 3: Correct COMMANDS.md**

- Add a complete `goregraph workspace diff --before <workspace-output> --after <workspace-output>` section next to the other workspace exploration commands.
- Include Rust in the `callgraph.json` and `routes.json` language descriptions.
- Change the version output example from `go1.26.x` to the release-compatible `go1.23.x`.

- [ ] **Step 4: Correct README.md**

- Point the output-contract link to `docs/OUTPUTS.md`.
- Add the four public workspace exploration commands to the command overview.
- Change the stale seven-view statement to eight views and insert API Catalog between Architecture and Endpoints.
- Preserve the generated language-coverage block unchanged unless `scripts/sync-docs --check` requires regeneration.

- [ ] **Step 5: Run documentation verification**

Run:

```bash
GOTOOLCHAIN=go1.23.12 go test . -count=1
GOTOOLCHAIN=go1.23.12 go run ./scripts/sync-docs --check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```text
Align release documentation with the CLI

- Document every public workspace command and the complete dashboard view set
- Correct runtime version examples, links, and Rust capability descriptions
- Extend the command inventory guard for manual workspace operations
```

### Task 3: Prove and install the final release candidate

**Files:**
- Verify: all tracked source and documentation
- Install: `/Users/gorecode/go/bin/goregraph`

**Interfaces:**
- Consumes: committed branch HEAD after Tasks 1 and 2.
- Produces: a clean pushed branch and a locally installed 1.3.0 binary embedding that exact commit.

- [ ] **Step 1: Run complete source and documentation gates**

Run Go formatting checks, `go test ./... -count=1`, `go vet ./...`, `go test -race ./... -count=1`, `scripts/sync-docs --check`, the repository shell verification scripts, and GoReleaser configuration/snapshot checks.

- [ ] **Step 2: Cross-build release platforms**

Build macOS amd64/arm64, Linux amd64/arm64, and Windows amd64 using Go 1.23.12. Expected: all builds succeed.

- [ ] **Step 3: Push the clean branch**

Push `fix/release-benchmark-metrics` only after all gates pass. Do not create or push a release tag.

- [ ] **Step 4: Install the exact committed candidate**

Build `/Users/gorecode/go/bin/goregraph` with release ldflags for version 1.3.0, the exact HEAD commit, and a UTC build timestamp.

- [ ] **Step 5: Audit the installed binary**

Verify `goregraph version`, all `help`, `--help`, and `-h` forms, the complete command catalog including `workspace diff`, and byte-for-byte help equality between the installed binary and a fresh source build.

- [ ] **Step 6: Confirm repository state**

Verify branch HEAD equals its upstream, the worktree is clean, no `v1.3.0` tag exists, and no release was published.
