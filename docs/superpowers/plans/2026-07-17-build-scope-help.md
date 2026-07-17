# Build Scope Help Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Explain project-only, project-with-workspace-refresh, and full-workspace builds consistently in the CLI help and primary documentation.

**Architecture:** Keep command behavior unchanged and treat global CLI help as a tested public contract. Mirror the same concise scope matrix in `README.md` and the detailed command reference in `COMMANDS.md`.

**Tech Stack:** Go CLI tests, Markdown documentation, existing GoreGraph build commands.

## Global Constraints

- Do not change scan or reconciliation behavior.
- Do not introduce new commands, flags, or dependencies.
- Use the existing terms project projection, workspace overlay, agent, dashboard, and all.
- State explicitly that a project build never scans sibling projects.

---

### Task 1: Test and update global help

**Files:**
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/cli/cli.go`

**Interfaces:**
- Consumes: `Run([]string{"help"}, stdout, stderr)`.
- Produces: Global help containing the three build-scope examples and guarantees.

- [ ] **Step 1: Write the failing test**

Add `TestRunHelpExplainsProjectAndWorkspaceBuildScopes` and assert that global
help contains:

```text
goregraph build dashboard .
goregraph build dashboard . --no-workspace
goregraph workspace build dashboard .
Does not scan sibling projects.
Skips workspace discovery and reconciliation.
Scans every discovered workspace project.
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
GOCACHE=/private/tmp/goregraph-go-cache go test ./internal/cli -run TestRunHelpExplainsProjectAndWorkspaceBuildScopes -count=1
```

Expected: FAIL because global help does not yet contain the scope section.

- [ ] **Step 3: Add the minimal help section**

Add one compact `Project vs workspace builds` section to `printHelp` that
contains the tested command examples and descriptions. Do not change parsing or
execution code.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run the command from Step 2. Expected: PASS.

### Task 2: Align README and command reference

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`

**Interfaces:**
- Consumes: The help contract from Task 1.
- Produces: Matching scope guidance for quick-start and reference readers.

- [ ] **Step 1: Update the README Quick Start**

Add a scope table that distinguishes project dashboard, strict project-only
dashboard, and full workspace dashboard. Explain the two dashboard output
directories and state that the same scope rules apply to `agent` and `all`.

- [ ] **Step 2: Update both command sections in COMMANDS.md**

Under `goregraph build`, document project scanning, optional overlay refresh,
and `--no-workspace`. Under `goregraph workspace build`, explicitly contrast the
full project loop and interactive dashboard output.

- [ ] **Step 3: Check documentation consistency**

Run:

```bash
rg -n "Does not scan sibling projects|--no-workspace|Scans every discovered workspace project|goregraph workspace build dashboard" README.md COMMANDS.md internal/cli/cli.go
```

Expected: all three surfaces contain equivalent scope guidance.

### Task 3: Verify the complete change

**Files:**
- Verify: `internal/cli/cli.go`
- Verify: `internal/cli/cli_test.go`
- Verify: `README.md`
- Verify: `COMMANDS.md`

**Interfaces:**
- Consumes: Tasks 1 and 2.
- Produces: A tested, formatting-clean documentation clarification.

- [ ] **Step 1: Run the full test suite**

```bash
GOCACHE=/private/tmp/goregraph-go-cache go test ./... -count=1
```

Expected: all packages PASS.

- [ ] **Step 2: Run static and formatting checks**

```bash
GOCACHE=/private/tmp/goregraph-go-cache go vet ./...
git diff --check
test -z "$(gofmt -l .)"
```

Expected: all commands exit successfully without output indicating a problem.

- [ ] **Step 3: Inspect installed-style help output**

```bash
go run ./cmd/goregraph help
```

Expected: the project/workspace scope section is readable and contains all
three commands.
