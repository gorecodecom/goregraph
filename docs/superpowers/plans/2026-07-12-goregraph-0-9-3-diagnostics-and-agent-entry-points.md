# GoreGraph 0.9.3 Diagnostics And Agent Entry Points Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce canonical evidence-backed diagnostics and compact task-oriented Query, MCP, and Markdown entry points that let coding agents orient before reading broad source areas.

**Architecture:** A shared `agent` result envelope loads existing Schema 1 outputs, validates freshness/capability context, bounds results, and exposes stable IDs plus continuation. Query and MCP call the same read-only task service; diagnostics become canonical scan records rather than browser-derived classifications.

**Tech Stack:** Go 1.23+, standard library, JSON/Markdown, MCP stdio, generated offline dashboard.

## Global Constraints

- Preserve all Schema 1 files and aliases; additions are backward compatible.
- Consume 0.9.2 evidence IDs and coverage enums without redefining them.
- Default limit 20; maximum limit 100; opaque deterministic continuation token.
- Detail levels: `summary`, `standard`, `full`; default `standard`.
- Formats: human text, JSON, Markdown.
- Every result includes schema, task, freshness, coverage warnings, result count, truncation, continuation, and suggested next query.
- MCP remains stdio-only, read-only, performs no scan, opens no listener, executes no project code, and writes no project files.
- No full directed trace branches or data-flow graph before 0.9.4.
- No push, tag, or public release.

---

### Task 1: Canonical Diagnostic Records

**Files:** Create `internal/scan/canonical_diagnostics.go`, tests; modify `types.go`, `scan.go`, dashboard script/tests.

- [ ] Write failing tests for stable ID, code, human title, category, severity, confidence, resolution, explanation, impact, evidence IDs, affected artifacts, next checks, and optional configuration guidance.
- [ ] Verify red with `go test ./internal/scan -run TestCanonicalDiagnostic -count=1`.
- [ ] Implement deterministic conversion from existing diagnostics/contracts/coverage; `OUT_OF_SCOPE` defaults to `INFO`; missing capability becomes `missing_scan_coverage`, never a source-absence claim.
- [ ] Emit `diagnostics-canonical.json`; make dashboard consume canonical copy when present and retain legacy fallback.
- [ ] Run scan/dashboard tests and commit `feat: add canonical diagnostics`.

### Task 2: Shared Agent Result Envelope

**Files:** Create `internal/agent/types.go`, `service.go`, `service_test.go`.

- [ ] Write failing tests for defaults, max limit, detail/format validation, deterministic continuation, stale manifest warning, failed/unavailable capability warning, and missing output guidance.
- [ ] Implement `Request{Root,Task,Query,Scope,Format,Detail,Limit,Continuation}` and `Result{Schema,Task,Freshness,CoverageWarnings,Items,Count,Truncated,Continuation,SuggestedNext}`.
- [ ] Implement `Service.Run(Request) (Result,error)` with no writes and no scan invocation.
- [ ] Run `go test ./internal/agent -count=1` and commit `feat: add compact agent result envelope`.

### Task 3: Task-Oriented Query Operations

**Files:** Modify `internal/query/query.go`, tests; modify CLI parsing/tests.

- [ ] Write failing tests for `workspace-summary`, `service-context`, `endpoint-search`, `diagnostics`, `coverage`, `evidence`, `tests`, and `change-context` with `--format`, `--detail`, `--limit`, `--continue`.
- [ ] Preserve old `goregraph query <path> <term>` and output aliases.
- [ ] Route known task names to `agent.Service`; render human, JSON, or Markdown from the same envelope.
- [ ] Verify invalid options are actionable and no-results include coverage warnings.
- [ ] Run query/CLI tests and commit `feat: add task-oriented query operations`.

### Task 4: Compact MCP Tools

**Files:** Modify `internal/mcp/mcp.go`, tests.

- [ ] Write failing `tools/list` and `tools/call` tests for `workspace_summary`, `service_context`, `endpoint_search`, `diagnostics`, `coverage`, `evidence`, `tests`, and `change_context`.
- [ ] Give each tool a strict JSON schema with `additionalProperties:false`, limit/detail/continuation fields, and task-specific required arguments.
- [ ] Delegate calls to `agent.Service`; return compact JSON text and preserve legacy tools.
- [ ] Assert MCP never scans or modifies fixture files.
- [ ] Run MCP tests and commit `feat: expose compact agent MCP tools`.

### Task 5: Primary Markdown Entry Points

**Files:** Create `internal/scan/agent_reports.go`, tests; modify `scan.go`, workspace reconciliation.

- [ ] Write failing tests for `workspace-summary.md`, `architecture.md`, `diagnostics.md`, and `agent-guide.md` with links to evidence, coverage caveats, suggested Query/MCP tasks, freshness guidance, and bounded starting points.
- [ ] Generate the four primary reports while retaining specialized reports as references.
- [ ] Ensure `agent-guide.md` tells agents to query first, then read narrow cited source, and never treats unavailable coverage as absent behavior.
- [ ] Run deterministic report tests and commit `feat: add primary agent markdown entry points`.

### Task 6: Doctor, Documentation, And Version 0.9.3

**Files:** Modify Doctor, README, COMMANDS.md, docs/RELEASE.md, version and tests.

- [ ] Add failing Doctor tests for malformed canonical diagnostics, invalid continuation metadata, missing primary reports, and dangling diagnostic evidence.
- [ ] Implement validations and update documentation with exact commands and limits without claiming 0.9.4 traces.
- [ ] Set version `0.9.3`; run full tests, vet, JS syntax, and diff check.
- [ ] Commit `docs: prepare GoreGraph 0.9.3`.

### Task 7: Local Acceptance

- [ ] Install 0.9.3 in Go bin and existing local default target; verify PATH and version.
- [ ] In `~/projects/weka`, review clean dry-run, execute clean, scan all 44 projects, run status and representative Doctor checks.
- [ ] Exercise every new Query task and MCP tool against fresh outputs; confirm compact bounds, evidence references, warnings, continuation, and no writes.
- [ ] Repeat scan and compare deterministic canonical diagnostics and primary-report hashes.
- [ ] Verify dashboard canonical diagnostics statically; do not open a browser unless authorized.

## Completion Gate

- [ ] Diagnostics are canonical, stable, evidence-backed, and shared by outputs/UI/agents.
- [ ] Query and MCP consume one bounded result envelope.
- [ ] Missing analysis is reported as coverage, not absence.
- [ ] Primary Markdown entry points provide a small orientation surface.
- [ ] 0.9.3 is locally installed and passes clean WEKA acceptance.
- [ ] Gate C is frozen for 0.9.4 directed traces.
