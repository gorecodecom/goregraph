# Workspace Discovery and Scan Performance Design

**Date:** 2026-07-21

## Goal

Restore a practical one-command workspace scan without losing analysis data.
GoreGraph should discover actual repository or project roots, scan each root
exactly once, remain responsive on large JavaScript and TypeScript monorepos,
and show enough progress that users can distinguish active work from a hang.

## Current Problems

Workspace discovery currently treats every immediate child of conventional
group directories such as `frontend` and `microservices` as a project. This
incorrectly includes infrastructure containers such as `.worktrees`, even
when they contain neither Git metadata nor a supported project marker.

The JavaScript and TypeScript symbol resolver repeatedly traverses the complete
reference collection while resolving individual imports and exports. The
observed `frontend-monorepo` scan produced 24,180 relations, including 18,908
relations from exact script analysis. Repeated whole-collection traversal makes
resolution quadratic or worse for realistic monorepos and kept one CPU core
busy for several minutes.

Workspace progress is printed only after an individual project completes. A
large project therefore produces no terminal feedback while the expensive
analysis runs.

## Design Principles

1. **Project boundaries first:** discover roots, not arbitrary directories.
2. **Scan once:** after recognizing a project root, do not discover nested
   package or build markers as separate workspace projects.
3. **No analysis loss:** preserve generated facts, confidence, ambiguity,
   cycle handling, ordering, schemas, and dashboard and agent projections.
4. **Near-linear lookup:** build indexes once instead of repeatedly searching
   all script references.
5. **Visible progress:** identify the active project before work begins and
   periodically confirm that a long-running scan is still active.
6. **Portable behavior:** use filesystem and path rules that work on Windows,
   macOS, and Linux without shell-specific assumptions.

## Workspace Project Discovery

Discovery first evaluates the workspace root itself. If it is a project root,
it becomes the single discovered project and traversal stops below it.
Otherwise, discovery walks through its organizational directories. For every
candidate directory it applies these rules in order:

1. Do not follow directory symlinks.
2. Skip hidden and generated infrastructure directories, including `.git`,
   `.worktrees`, `node_modules`, `vendor`, `target`, `build`, `dist`,
   `coverage`, `goregraph-out`, and `.goregraph-workspace`.
3. Recognize the directory as a project root when it contains its own `.git`
   directory, its own `.git` file as used by linked worktrees, or a supported
   root project marker.
4. Add a recognized root once using its normalized absolute path and do not
   descend below it.
5. Recurse into an unrecognized organizational directory so that projects may
   be nested more than one level below the workspace root.

This boundary rule means `frontend/frontend-monorepo` is one project. Its
internal `apps/*/package.json` files remain part of that project and do not
create additional workspace entries. A user can still explicitly run a
project command against an internal directory when that narrower scope is
intentional.

The marker set covers the supported ecosystems where a reliable root marker
exists:

- Node.js and JavaScript/TypeScript: `package.json`;
- Java and Kotlin: `pom.xml`, `build.gradle`, `build.gradle.kts`,
  `settings.gradle`, and `settings.gradle.kts`;
- Go: `go.mod`;
- Python: `pyproject.toml`, `requirements.txt`, and `setup.py`;
- Rust: `Cargo.toml`;
- PHP: `composer.json`;
- Scala: `build.sbt`;
- Swift: `Package.swift`;
- Ruby: `Gemfile` and root-level `*.gemspec` files;
- C and C++: `CMakeLists.txt` and `meson.build`;
- C#: root-level `*.sln` and `*.csproj` files;
- explicit GoreGraph projects: `goregraph.yml`.

Languages without a reliable project manifest, such as standalone shell or
documentation projects, are discovered automatically when they are Git
repositories and remain available through explicit project scans otherwise.

Existing workspace role and service classification remains based on the
project's position beneath conventional group directories. Discovery order is
sorted and deterministic after path normalization.

## Script Resolver Performance

The resolver builds immutable lookup indexes from extracted facts before
resolving references:

- declarations by module and export name;
- local declarations by module and local name;
- local exports by module and public export name;
- imports by module and local binding name;
- re-exports by module and public export name;
- references by source module where a grouped traversal is still required.

`resolveExport` uses these indexes instead of scanning every project reference.
Completed export resolutions are memoized by module file, export name, and
required capability. In-progress keys remain separately tracked so cyclic
re-exports retain the current cycle behavior without caching partial results.

The change must preserve exact, unresolved, and ambiguous outcomes, candidate
symbol IDs, reasons, confidence values, alias identities, stable IDs, sorting,
and deduplication. It changes lookup cost, not the analysis contract.

Projects remain sequential initially. Parallel project scans could hide some
latency but would not fix the algorithmic defect and could create excessive CPU
and memory pressure on developer machines.

## Progress and Failure Reporting

Before each project scan, the CLI prints a stable line such as:

```text
Scanning [4/43] frontend/frontend-monorepo ...
```

After completion it prints:

```text
Completed [4/43] frontend/frontend-monorepo in 18.2s (1517 files, 115 skipped)
```

If a scan exceeds ten seconds, a new concise status line is printed every ten
seconds with the project position and elapsed duration. Output uses ordinary
lines rather than terminal cursor control so it behaves consistently in
PowerShell, Command Prompt, macOS terminals, redirected logs, and CI.

On failure, the CLI reports the project position, normalized project path,
elapsed time, and failing phase when known. Workspace builds remain fail-fast:
completed project outputs stay valid, the failed project is not reported as
complete, and the workspace-level manifest is not published from a partial
build.

## Compatibility

No command, option, generated filename, output schema, or projection meaning
changes. `goregraph workspace scan-all .` and
`goregraph workspace build all .` continue to invoke the same complete
workspace workflow. Explicit project scans continue to accept non-Git
directories.

Previously generated valid project indexes remain readable. Discovery may
intentionally remove false project entries such as `.worktrees` and may add
previously missed nested project roots that contain a supported marker.

## Verification

Discovery tests must cover:

1. a normal Git checkout with a `.git` directory;
2. a linked worktree with a `.git` file;
3. a non-Git project with each class of supported root marker;
4. `.worktrees` and other infrastructure directories being excluded;
5. a Git monorepo with nested `package.json` files being discovered once;
6. a non-Git monorepo with a root marker being discovered once;
7. projects nested beneath organizational directories;
8. a workspace root that is itself a project being discovered once;
9. directory symlinks not being followed;
10. deterministic path ordering on supported operating systems.

Resolver verification must cover:

1. existing exact, ambiguous, unresolved, aliased, conditional-export, and
   cyclic-re-export fixtures without expected-output changes;
2. deterministic output across repeated runs;
3. a large synthetic module graph exercising thousands of imports, calls, and
   re-exports;
4. a benchmark demonstrating that growth no longer follows repeated complete
   reference-list scans;
5. unchanged generated project outputs for representative existing fixtures.

CLI tests use an injectable progress clock or ticker so start, heartbeat,
completion, and failure lines are deterministic without real-time sleeps.

Final acceptance includes a complete local scan of the Weka workspace. The
`frontend-monorepo` runtime is recorded and compared with the observed
multi-minute baseline. The practical target is completion in under one minute
on the same machine, without reducing its 1,517-file analysis or omitting
generated facts. Full repository tests, formatting, vetting, and cross-platform
CI must pass before installation.

## Out of Scope

- parallel project scanning;
- changing workspace output schemas;
- reducing analyzer coverage or context detail;
- dashboard redesign;
- releasing or publishing a new version as part of this change.

## Success Criteria

- `.worktrees` is not discovered as a project.
- A real Git checkout or linked worktree is discovered.
- A supported non-Git project root is discovered from its manifest.
- A monorepo is scanned once even when it contains nested manifests.
- Script resolution no longer performs repeated full-reference scans.
- Large projects show immediate and periodic terminal progress.
- The complete Weka workspace scan finishes successfully in a practical time.
- Generated analysis remains deterministic and semantically unchanged.
