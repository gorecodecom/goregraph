# Workspace Project Eligibility Design

Date: 2026-07-22
Status: Proposed

## Context

Workspace discovery currently treats every directory containing a `.git`
directory or file as a scan project. This confuses repository boundaries with
software-project boundaries. In the real Weka workspace, 8 of 51 discovered
projects have `.git` but no supported project marker. They are documentation,
database, template, or shared script repositories and are not intended to be
part of GoreGraph's automatic workspace scan.

All other 43 Weka projects have a supported root marker. Therefore a
marker-first rule removes exactly the unwanted projects in the observed
workspace without requiring machine-specific path configuration.

## Considered Approaches

### 1. Strict marker-first discovery (selected)

Automatically accept a directory only when it contains a supported regular
project marker. Keep `.git` exclusively as repository metadata for Git update
operations.

This is deterministic, portable, inexpensive, and favors precision for bulk
workspace scans. A non-standard project can opt in with a project-local
`goregraph.yml` or be scanned directly by an explicit project command.

### 2. Git plus source-density heuristics (rejected)

Accept Git repositories when they contain enough supported source files.
This would still accept repositories such as `wbp/database` with hundreds of
SQL files and would require arbitrary file-count thresholds. Results could also
change when documentation or generated files are added.

### 3. Workspace-specific include/exclude paths (rejected as the default)

Path filters could solve one installation but would encode local folder names
and leave the incorrect global default unchanged. Explicit filters may be a
separate future feature, but they are not needed for this correction.

## Eligibility Rules

Automatic workspace discovery follows these rules in order:

1. Skip hidden, generated, and symlinked directories using the existing safety
   rules.
2. Accept a directory as a project when it contains a supported regular marker:
   `package.json`, Maven/Gradle markers, `go.mod`, Python markers, Cargo,
   Composer, SBT, Swift, Ruby, CMake, Meson, `goregraph.yml`, or the existing
   solution/project wildcard markers.
3. Once a project root is accepted, stop descending below it. This preserves a
   root-marked monorepo as one GoreGraph project.
4. When a directory has `.git` but no project marker, do not accept it merely
   because it is a repository. Continue traversing visible child directories so
   independently marked nested projects can still be discovered.
5. Do not infer projects from conventional folder names such as `frontend`,
   `microservices`, or `services`. Markerless group-child fallback is removed.
6. Do not infer a project from an existing `goregraph-out/manifest.json`.
   Generated output is evidence of a previous scan, not current project intent.

These rules do not alter explicit project commands. A user who runs
`goregraph build all <path>` or another direct project build has already chosen
that path and may scan it without an automatic-discovery marker.

## Git Operations

Workspace Git update remains independent from scan eligibility. Its recursive
Git-target discovery continues to find repositories by `.git`, including
repositories that are intentionally excluded from GoreGraph workspace scans.
This preserves synchronization behavior while separating it from code-map
generation.

## Existing Generated Output

The eight previously scanned Weka repositories already contain valid
`goregraph-out` manifests. Because generated output no longer qualifies a
directory, these outputs cannot keep a stale project alive in discovery.

The next real workspace reconciliation replaces the workspace registry with
the newly discovered project set. GoreGraph does not automatically delete the
old per-repository output directories; deletion remains an explicit operation.
This avoids destructive migration behavior.

## User Experience and Documentation

Workspace help and command documentation explain the distinction:

- project/build markers determine automatic scan eligibility;
- `.git` determines repository identity for Git operations only;
- `goregraph.yml` is the portable, repository-owned opt-in for non-standard
  projects;
- direct project commands can scan a deliberately selected markerless folder.

No workspace-specific configuration file or path list is introduced.

## Testing and Verification

Automated tests cover:

- Git-only repositories are not automatic projects;
- Git repositories with supported markers are projects;
- marker-based non-Git projects remain supported;
- nested marked projects inside a markerless Git repository are discovered;
- root-marked monorepos remain one project;
- markerless children under conventional group names are not inferred;
- valid generated output alone does not qualify a project;
- project-local `goregraph.yml` provides explicit opt-in;
- workspace Git update still includes Git-only repositories;
- help text describes the new rule.

Real-workspace verification uses a Weka dry-run. Expected result: 43 projects,
with all current frontend and microservice projects retained and the eight
Git-only documentation/database/template/script repositories absent.

The implementation uses Go filesystem APIs and existing path normalization so
the behavior remains consistent on Windows and macOS.
