# GoreGraph 1.0 Implementation Program

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement each release plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the approved GoreGraph 1.0 product, analyzer parity, dashboard, evidence, data-flow, Query, and MCP design through independently installable and testable releases.

**Architecture:** Treat each `0.9.x` release as a separate implementation project with a frozen acceptance boundary. Later release plans are written only after the preceding release establishes and verifies the shared interfaces they consume, preventing speculative cross-release signatures and Spring- or WEKA-specific core abstractions.

**Tech Stack:** Go 1.23+, Go standard library, generated offline HTML/CSS/JavaScript, JSON/Markdown outputs, read-only MCP stdio, browser-based UI verification, repository fixtures and golden files.

## Global Constraints

- Current baseline is `0.9.0`; public target is `1.0.0`.
- Canonical design: `docs/superpowers/specs/2026-07-12-goregraph-1-0-product-and-language-parity-design.md`.
- Keep scans local, deterministic, inspectable, and free of project-code execution, telemetry, and network requirements.
- Preserve Schema 1 meanings throughout `0.9.x`; additions and deprecations must be explicit.
- Do not push, tag, or publish a release without explicit user authorization.
- Do not embed WEKA paths, names, domains, services, or helper conventions in generic analyzers.
- Use focused capability interfaces and framework adapters rather than one universal analyzer interface.
- Use TDD, focused fixtures, golden tests, full Go verification, local installation, clean real-workspace rescan, output inspection, and browser UI verification.
- Real acceptance workspace: `~/projects/weka/`.
- `goregraph workspace clean .` is a required dry-run; destructive cleanup uses `goregraph workspace clean . --execute` only after reviewing the listed generated paths.
- Confirm `command -v goregraph` and `goregraph version` before every real-workspace acceptance run.
- A release is incomplete until a freshly installed binary generates the outputs under test.

---

## Release Plan Sequence

| Release | Independent deliverable | Plan gate |
|---|---|---|
| `0.9.1` | Architecture-first dashboard foundation, integrated service focus, Endpoints/Diagnostics navigation, stable viewport, real Fit, explanations, design system | Detailed plan approved and executed first |
| `0.9.2` | Shared capability, evidence, confidence, resolution, severity, and coverage model | Write after `0.9.1` renderer/state interfaces are verified |
| `0.9.3` | Canonical diagnostics plus task-oriented Query, MCP, and primary Markdown entry points | Write after `0.9.2` record IDs and evidence references are frozen |
| `0.9.4` | Directed endpoint subgraphs, upstream/downstream traversal, branches, cycles, and common data-flow core | Write after Query/MCP pagination and evidence contracts are verified |
| `0.9.5` | Full Java/Spring and JS/TS/Node/React parity | Write after common trace and data-flow interfaces are frozen |
| `0.9.6` | Full Go and PHP parity | Write after cross-language acceptance harness proves the reference ecosystems |
| `0.9.7` | Full Rust parity and cross-language messaging architecture | Write after synchronous route and persistence parity is verified |
| `0.9.8` | Cross-language parity, configuration, performance, cross-platform, Python/Shell integration, real-workspace hardening | Write after all full-support language adapters exist |
| `1.0.0-rc.1` | Schema 2 and public CLI/Query/MCP freeze, migration and platform acceptance | Write after `0.9.8` passes all parity gates |
| `1.0.0` | Stable public contract and final release approval | Fixes only after RC; public release requires explicit authorization |

## Required Plan Artifacts Per Release

Each release receives its own `docs/superpowers/plans/YYYY-MM-DD-goregraph-<version>-<scope>.md` containing:

1. Exact files created and modified.
2. Exact public Go interfaces and JSON fields introduced in that release.
3. Failing tests before production changes.
4. Small, independently reviewable tasks with focused commits.
5. Backward-compatibility and migration assertions.
6. Fixture and golden-file acceptance.
7. Full-suite and deterministic-repeat commands.
8. Local `go install ./cmd/goregraph` verification.
9. WEKA clean dry-run, reviewed `--execute`, full scan, doctor, and workspace status.
10. Generated JSON/Markdown/Query/MCP inspection as applicable.
11. Browser verification of every changed dashboard behavior.

## Cross-Release Interface Gates

### Gate A — Dashboard Shell (`0.9.1`)

Freeze only dashboard view identifiers, viewport-state behavior, renderer boundaries, and design tokens. Do not introduce speculative Evidence or Data Flow records.

### Gate B — Evidence And Capabilities (`0.9.2`)

Freeze stable evidence IDs, capability identifiers, confidence/resolution/severity/coverage enums, and additive Schema 1 JSON records. Later Query, MCP, diagnostics, traces, and language adapters consume these exact types.

### Gate C — Agent Contract (`0.9.3`)

Freeze compact result envelopes, result limits, continuation, evidence references, freshness warnings, and primary task names. Do not expose raw internal maps as public MCP contracts.

### Gate D — Trace And Data Flow (`0.9.4`)

Freeze node/edge roles, branch and cycle semantics, main-path selection, upstream/downstream traversal, field mappings, and missing-flow representation.

### Gate E — Reference Language Parity (`0.9.5`)

Prove the common contracts against a React/Node consumer and Java/Spring or Node backend before porting them. Reject interfaces that require Spring naming to function.

### Gate F — Full Language Parity (`0.9.6`–`0.9.8`)

Every advertised language must answer the same acceptance questions with language-appropriate evidence. Capability absence must never be reported as source-code absence.

### Gate G — Public Freeze (`1.0.0-rc.1`)

No new features after the RC. Only correctness, compatibility, performance, documentation, accessibility, and packaging fixes may enter `1.0.0`.

## Program Completion Criteria

- All release plans are executed and their independent acceptance records pass.
- Java, JS/TS/Node/React, Go, PHP, and Rust pass the shared end-to-end fixture suite.
- Python and Shell participate in the shared evidence and agent model without losing existing support.
- Index-only languages are labeled honestly and never presented as full parity.
- Every public relationship has evidence and one shared confidence meaning.
- Architecture, Endpoints, Data Flow, Diagnostics, and Coverage answer distinct developer questions.
- Query and MCP let agents navigate with compact results before reading source broadly.
- Schema 2, CLI, Query, and MCP contracts are stable and documented.
- A clean local install and fresh scan of `~/projects/weka/` passes output and UI acceptance.

