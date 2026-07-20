# Progressive CLI Help Design

**Date:** 2026-07-20

## Goal

Make the GoreGraph CLI approachable for normal users without removing,
renaming, or changing the behavior of any existing command, option, output
alias, or automation path.

The default help should lead with the few workflows most users need. Complete
manual, compatibility, maintenance, and diagnostic functionality must remain
available through progressive disclosure.

## Current Problem

The global help presents thirteen top-level commands and more than thirty
examples with equal visual weight. The workspace help adds another large
command tree. Several concepts overlap:

- `build`, `scan`, and `update` all create or refresh projections;
- project and workspace scopes repeat build, scan, dashboard, explain, and Git
  update operations;
- `query` combines search, task operations, canonical symbol operations, and
  direct output aliases;
- the top-level `dashboard` command is executable but absent from the global
  command list;
- `update --help` incorrectly prints the compatibility help for `scan`.

The capabilities are useful, but the default presentation makes expert and
compatibility paths appear mandatory.

## Design Principles

1. **Progressive disclosure:** show the normal workflow first and the complete
   catalog on explicit request.
2. **No compatibility break:** preserve every existing invocation and exit
   behavior outside the new help-selection syntax.
3. **Task-first language:** lead with outcomes users recognize rather than the
   internal projection model.
4. **One-step discoverability:** every hidden command must be reachable from a
   single visible `help --all` hint.
5. **Complete command help:** command-specific `help`, `--help`, and `-h`
   output remains authoritative and detailed.
6. **Documentation parity:** terminal help and `COMMANDS.md` describe the same
   standard and expert split.

## Global Help

`goregraph help`, `goregraph --help`, `goregraph -h`, and invocation without
arguments print a compact standard view. It should remain close to one terminal
screen, with a target maximum of approximately 35 lines.

The standard view contains:

1. the product sentence and usage line;
2. three copyable workflows:
   - agent context: `build agent` followed by `context`;
   - dashboard: `build dashboard` followed by `dashboard open`;
   - health diagnosis: `doctor`;
3. the core commands `build`, `context`, `dashboard`, `doctor`, `workspace`,
   and `mcp`;
4. a direct hint to `goregraph help --all` for every other command;
5. a direct pointer to command-specific help.

The standard help does not list compatibility aliases, manual query surfaces,
raw report access, safe Git maintenance, or version metadata as primary
workflow choices.

## Complete Global Help

`goregraph help --all` prints the full catalog. For ergonomic consistency,
`goregraph --help --all` and `goregraph -h --all` produce the same output.

The complete view groups commands by purpose instead of presenting one flat
list:

- **Core:** `build`, `context`, `dashboard`, `doctor`, `workspace`, `mcp`;
- **Manual exploration:** `query`, `explain`, `report`;
- **Maintenance:** `update`, `git update`;
- **Compatibility:** `scan`;
- **Utility:** `version`, `help`.

It retains the important project-versus-workspace explanation, dashboard edit
behavior, MCP expert-mode warning, workspace detection rules, and relevant
examples. Repetition should be reduced, but no operational caveat should be
dropped.

Unknown global help options fail with exit code 2 and a concise error that
points to `goregraph help`.

## Workspace Help

`goregraph workspace help`, `goregraph workspace --help`, and
`goregraph workspace -h` print a compact workspace view.

The standard workspace view leads with:

- `build` for scanning all discovered workspace projects;
- `dashboard` for human inspection;
- `status` for checking existing generated state;
- `explain`, `path`, and `impact` as the primary generated-output exploration
  tools.

It includes a visible `goregraph workspace help --all` hint.

`goregraph workspace help --all` prints every existing workspace operation,
grouped as core, exploration, maintenance, and compatibility commands. It keeps
the existing workspace detection, safe cleanup, dashboard editing, and scan
semantics discoverable.

Unknown workspace help options fail with exit code 2 and point to
`goregraph workspace help`.

## Command-Specific Help

Every existing command-specific help path remains available and complete.
This includes expert and compatibility commands omitted from standard help.

`goregraph update help`, `goregraph update --help`, and
`goregraph update -h` receive dedicated output describing:

- refresh of the current project;
- `--target agent|dashboard|all`;
- `--no-update-gitignore`, `--no-workspace`, and `--workspace`;
- representative update examples.

The implementation must no longer route update help through `printScanHelp`.
The `scan` help remains unchanged as the explicit compatibility alias for
`build all`.

## Compatibility Contract

The change is presentation-only except for adding the `--all` help selector
and correcting update help text. It must not:

- remove or rename commands;
- alter command dispatch;
- change existing non-help flags;
- change generated files or schemas;
- change successful or failing exit codes for operational commands;
- change MCP tool exposure;
- change direct output aliases;
- require migration of scripts or documentation links.

`COMMANDS.md` remains the comprehensive reference. It gains a concise quick
start near the beginning and clear labels distinguishing standard workflow,
manual/expert operations, maintenance, and compatibility aliases. Existing
technical details remain in place.

## Implementation Boundaries

Help rendering should use focused functions for standard and complete views.
Argument validation for the help selector should be isolated so global and
workspace help apply the same rules without changing normal command parsing.

No command framework or new dependency is introduced. The existing standard
library rendering style remains appropriate for static deterministic help.

## Error Handling

- Standard and complete help return exit code 0.
- A help selector other than `--all` returns exit code 2.
- Errors are written to stderr; help content is written to stdout.
- Operational command errors and their streams remain unchanged.

## Verification

Tests must establish that:

1. standard global help contains the three primary workflows and all core
   commands;
2. standard global help points to `help --all` and does not enumerate manual,
   maintenance, or compatibility commands as primary choices;
3. complete global help contains every existing top-level command and preserves
   the important scope and MCP caveats;
4. the no-argument, `help`, `--help`, and `-h` forms select the expected view;
5. invalid global help selectors return exit code 2;
6. standard and complete workspace help expose the intended command sets;
7. invalid workspace help selectors return exit code 2;
8. all three update help forms describe update rather than scan;
9. scan remains an executable compatibility alias;
10. existing operational CLI tests continue to pass.

Completion verification runs:

```text
gofmt -d internal/cli/cli.go internal/cli/cli_test.go
go test ./internal/cli -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

## Success Criteria

- A new user can identify the agent, dashboard, and diagnosis workflows from
  the first help screen without reading expert material.
- The standard global help remains approximately one terminal screen.
- Every prior capability remains executable and documented.
- Full help is one explicit command away.
- Workspace help follows the same progressive-disclosure model.
- `update --help` accurately describes update behavior.
