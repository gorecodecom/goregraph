# Build Scope Help Design

## Goal

Make the difference between project and workspace builds immediately clear in
`goregraph help`, `README.md`, and `COMMANDS.md` without changing command
behavior.

## Command contract

The documentation and CLI help use the same three scope descriptions:

1. `goregraph build dashboard .` scans only the selected project, writes its
   shared `goregraph-out/index/` and human-readable `goregraph-out/dashboard/`,
   preserves a valid non-selected agent projection, and refreshes a detected
   workspace overlay from existing sibling indexes. It does not scan sibling
   projects.
2. `goregraph build dashboard . --no-workspace` performs the same project build
   while skipping workspace discovery and reconciliation entirely.
3. `goregraph workspace build dashboard .` scans every discovered workspace
   project and writes the interactive workspace dashboard under
   `.goregraph-workspace/dashboard/`.

The same scope distinction applies to `agent` and `all`. `dashboard` means
human-readable project reports for a project build and the full interactive
workspace view for a workspace build.

## Presentation

- Global help gets a compact "Project vs workspace builds" section with the
  three dashboard commands and their effects.
- `COMMANDS.md` explains the behavior next to both build command references.
- The README Quick Start includes a compact scope table so a first-time user
  can choose the right command without reading the full reference.
- Existing compatibility aliases remain documented but are not promoted as the
  primary workflow.

## Verification

A CLI regression test asserts that global help contains the three commands and
the crucial promises: only the selected project is scanned, sibling projects
are not scanned, `--no-workspace` skips reconciliation, and the workspace build
scans every discovered project. Existing CLI and repository tests must remain
green.
