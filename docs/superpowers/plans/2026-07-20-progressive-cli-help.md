# Progressive CLI Help Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Make the default GoreGraph CLI help short and task-oriented while preserving every existing command through complete help and command-specific help.

**Architecture:** Keep command dispatch unchanged and add one small selector that chooses standard or complete help renderers. Split global and workspace help into compact and full functions, give update dedicated help, and retain COMMANDS.md as the exhaustive reference.

**Tech Stack:** Go 1.26 standard library, existing internal/cli package, Go tests, Markdown documentation tests.

## Global Constraints

- Preserve every existing command, option, output alias, operational exit code, generated file, schema, and MCP tool.
- Add only --all as a global and workspace help selector; invalid selectors return exit code 2.
- Keep standard global help near 35 lines and include agent, dashboard, and diagnosis workflows.
- Keep every hidden command one explicit help --all invocation away.
- Do not add a command framework or dependency.
- Preserve the existing `.gitignore` change exactly, verify that it only adds
  `goregraph-out/`, and include it in the final documentation commit.
- Do not create a release, tag, package publication, or release artifact.

---

### Task 1: Define Progressive Help Behavior with Failing Tests

**Files:**
- Modify: internal/cli/cli_test.go:20-54
- Modify: internal/cli/cli_test.go:1336-1347
- Modify: internal/cli/cli_test.go:1570-1643

**Interfaces:**
- Consumes: Run(args []string, stdout, stderr io.Writer) int
- Produces: behavior contracts for standard help, complete help, invalid selectors, workspace help, and update help

- [ ] **Step 1: Write standard global-help tests**

Add table-driven coverage for no arguments, help, --help, and -h. Require the three workflows, the six core commands, command-specific-help guidance, and the help --all hint. Assert that standard help does not list scan <path>, query <path>, or git update [path] as primary commands.

~~~go
func TestRunHelpUsesProgressiveDisclosure(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"-h"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", args, code, stderr.String())
		}
		text := stdout.String()
		for _, want := range []string{
			"Agent context:",
			"goregraph build agent .",
			"goregraph context . --query",
			"Dashboard:",
			"goregraph dashboard open .",
			"Diagnosis:",
			"goregraph doctor .",
			"goregraph help --all",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%v help missing %q:\n%s", args, want, text)
			}
		}
	}
}
~~~

- [ ] **Step 2: Write complete-help and invalid-selector tests**

Cover help --all, --help --all, and -h --all. Require Core commands:, Manual exploration:, Maintenance:, Compatibility:, and Utility:, every existing top-level command, project/workspace caveats, and MCP expert-mode guidance.

~~~go
func TestRunHelpRejectsUnknownSelector(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"help", "--unknown"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "goregraph help") {
		t.Fatalf("unexpected streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
~~~

- [ ] **Step 3: Write workspace standard/full-help tests**

Require standard workspace help to contain build, dashboard, status, explain, path, impact, and goregraph workspace help --all while omitting scan-all. Require workspace help --all, workspace --help --all, and workspace -h --all to contain every current workspace command, including diff. Invalid workspace selectors must return exit code 2 and point to goregraph workspace help.

- [ ] **Step 4: Write dedicated update-help tests**

Cover update help, update --help, and update -h. Require Usage: goregraph update, --target agent|dashboard|all, and Refreshes the current project's selected projections. Reject Compatibility alias for goregraph build all.

- [ ] **Step 5: Run the focused tests and verify RED**

~~~text
go test ./internal/cli -run "TestRunHelpUsesProgressiveDisclosure|TestRunAllHelp|TestRunHelpRejectsUnknownSelector|TestWorkspaceHelp|TestRunUpdateHelp" -count=1
~~~

Expected: FAIL because --all is ignored, standard help exposes expert commands, workspace help is not progressive, and update help renders scan help.

- [ ] **Step 6: Commit the failing behavior contract**

Commit only internal/cli/cli_test.go:

~~~text
Define progressive CLI help behavior

- Cover compact and complete global and workspace help.
- Require invalid-selector errors and dedicated update guidance.
~~~

### Task 2: Implement Global, Workspace, and Update Help

**Files:**
- Modify: internal/cli/cli.go:33-66
- Modify: internal/cli/cli.go:134-171
- Modify: internal/cli/cli.go:1256-1297
- Modify: internal/cli/cli.go:1388-1557
- Test: internal/cli/cli_test.go

**Interfaces:**
- Consumes: behavior tests from Task 1 and isHelp(arg string) bool
- Produces: runHelpSelector, printHelp, printAllHelp, printWorkspaceHelp, printAllWorkspaceHelp, and printUpdateHelp

- [ ] **Step 1: Add one reusable help selector**

~~~go
func runHelpSelector(
	args []string,
	stdout, stderr io.Writer,
	standard, complete func(io.Writer),
	usage string,
) int {
	switch {
	case len(args) == 0:
		standard(stdout)
		return 0
	case len(args) == 1 && args[0] == "--all":
		complete(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "error: unknown help option %q; run %s\n", args[0], usage)
		return 2
	}
}
~~~

Change global help dispatch so no arguments render standard help, while help, --help, and -h forward remaining arguments to this selector. Do not change operational switch cases.

- [ ] **Step 2: Split global help into compact and complete renderers**

Make printHelp render the product sentence, usage, three workflows, six core commands, command-specific-help hint, and help --all hint near 35 lines. Move the grouped exhaustive catalog and all operational caveats into printAllHelp. Include the previously omitted top-level dashboard command.

- [ ] **Step 3: Split workspace help into compact and complete renderers**

Route workspace help forms through runHelpSelector. Keep the compact renderer focused on build, dashboard, status, explain, path, and impact. Move every existing operation, examples, workspace detection, dashboard editing, cleanup, and compatibility notes into printAllWorkspaceHelp. Include diff.

- [ ] **Step 4: Give update dedicated help without changing execution**

Use this branch at both help points in runScan:

~~~go
if update {
	printUpdateHelp(stdout)
} else {
	printScanHelp(stdout)
}
~~~

Implement printUpdateHelp with --target, --no-update-gitignore, --no-workspace, --workspace, and representative examples. Do not change parsing, defaults, config loading, or projection generation.

- [ ] **Step 5: Format and verify focused GREEN**

~~~text
gofmt -w internal/cli/cli.go internal/cli/cli_test.go
go test ./internal/cli -run "TestRunHelpUsesProgressiveDisclosure|TestRunAllHelp|TestRunHelpRejectsUnknownSelector|TestWorkspaceHelp|TestRunUpdateHelp|TestRunScanHelp" -count=1
~~~

Expected: PASS.

- [ ] **Step 6: Run the complete CLI package**

~~~text
go test ./internal/cli -count=1
~~~

Expected: PASS. Move superseded caveat assertions to complete help rather than weakening them.

- [ ] **Step 7: Commit implementation and tests**

~~~text
Simplify CLI help with progressive disclosure

- Lead standard help with agent, dashboard, and diagnosis workflows.
- Preserve the complete command catalog behind help --all.
- Correct update help without changing command behavior.
~~~

### Task 3: Align the Complete Command Reference

**Files:**
- Modify: COMMANDS.md:1-31
- Modify: COMMANDS.md:188-204
- Modify: docs_test.go:63-86

**Interfaces:**
- Consumes: help names from Task 2
- Produces: quick start and explicit standard, expert, maintenance, and compatibility labels

- [ ] **Step 1: Write a failing documentation contract**

Extend TestCommandsReferenceDocumentsEveryUserCommand with goregraph dashboard, goregraph help --all, and goregraph workspace help --all. Add TestCommandsReferenceExplainsProgressiveHelp requiring Quick start, Standard commands, Manual and expert operations, Maintenance commands, and Compatibility aliases.

- [ ] **Step 2: Verify documentation RED**

~~~text
go test . -run "TestCommandsReferenceDocumentsEveryUserCommand|TestCommandsReferenceExplainsProgressiveHelp" -count=1
~~~

Expected: FAIL because progressive navigation is not documented.

- [ ] **Step 3: Update COMMANDS.md without removing reference content**

Add Quick start after the introduction with the same three workflows as terminal help. Rename the normal and expert headings to the agreed labels, add short maintenance and compatibility navigation blocks, and document help --all plus workspace help --all. Retain every detailed command section and operational caveat.

- [ ] **Step 4: Verify documentation and CLI GREEN**

~~~text
go test . -run "TestCommandsReferenceDocumentsEveryUserCommand|TestCommandsReferenceExplainsProgressiveHelp" -count=1
go test ./internal/cli -count=1
~~~

Expected: both commands PASS.

- [ ] **Step 5: Commit documentation parity**

~~~text
Document progressive CLI workflows

- Add a task-oriented quick start for normal users.
- Keep expert, maintenance, and compatibility commands easy to find.
~~~

### Task 4: Verify, Push, and Install Without Releasing

**Files:**
- Verify only: repository source and documentation
- Install target: C:\Users\goretzkh\go\bin\goregraph.exe

**Interfaces:**
- Consumes: committed implementation and documentation
- Produces: verified main, pushed commits, and a local binary from the pushed commit

- [ ] **Step 1: Run formatting and diff checks**

~~~text
gofmt -d internal/cli/cli.go internal/cli/cli_test.go docs_test.go
git diff --check
~~~

Expected: no formatting diff and no whitespace errors. The user's unstaged .gitignore may emit an existing line-ending warning but must not be staged or altered.

- [ ] **Step 2: Run full verification**

~~~text
go test ./... -count=1
go vet ./...
~~~

Expected: PASS with no test or vet failures.

- [ ] **Step 3: Inspect the final commit range and tree**

~~~text
git status --short --branch
git log --oneline origin/main..HEAD
git diff --cached --name-status
~~~

Expected: no task file is staged and the working tree is clean.

- [ ] **Step 4: Push main without force**

~~~text
git push origin main
~~~

Expected: all verified commits reach origin/main. Do not create a tag, release, package, changelog release entry, or published artifact.

- [ ] **Step 5: Install and verify the pushed source**

~~~text
go install ./cmd/goregraph
C:\Users\goretzkh\go\bin\goregraph.exe version
go version -m C:\Users\goretzkh\go\bin\goregraph.exe
C:\Users\goretzkh\go\bin\goregraph.exe help
C:\Users\goretzkh\go\bin\goregraph.exe help --all
~~~

Expected: goregraph 1.3.0; embedded vcs.revision equals pushed HEAD; standard help is compact; complete help has the full catalog.

## Explicit Non-Goals

- No release or prerelease.
- No Git tag.
- No package manager publication.
- No command deletion or rename.
- No generated schema or MCP behavior change.
