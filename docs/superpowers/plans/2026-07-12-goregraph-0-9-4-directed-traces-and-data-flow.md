# GoreGraph 0.9.4 Directed Traces And Data Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace linear endpoint traces with evidence-backed directed subgraphs and add a language-neutral data-flow core accessible through dashboard, Query, and MCP.

**Architecture:** Additive Schema 1 `DirectedTraceRecord` and `DataFlowRecord` reuse 0.9.2 evidence IDs and 0.9.3 agent envelopes. Existing linear trace steps remain compatibility projections of the selected main path.

**Tech Stack:** Go 1.23+, standard library graph algorithms, JSON/Markdown, generated offline SVG/HTML/JS, Query/MCP.

## Global Constraints

- Stable node/edge IDs; deterministic ordering; no absolute paths.
- Node roles: UI route, component, event handler, API client, HTTP/gRPC route, middleware, controller/handler, function/method, validation, transformation, repository, database/table, message producer, channel, consumer, external service, test.
- Edge facts include relation, callsite, evidence IDs, confidence, sync/async, conditionality, and optional mappings.
- Main path is deterministic; branches and cycles are explicit; traversal default maximum 100 nodes and cycle bound 3 visits.
- Selection never auto-centers, zooms, or relayouts.
- Existing `workspace-endpoint-traces.json` and UI remain readable.
- No project-specific names and no fabricated data mappings.

---

### Task 1: Directed Trace Model And Compatibility Projection
- [ ] Write failing model/JSON tests for nodes, edges, entries, exits, main path, branches, cycles, evidence and stable IDs.
- [ ] Implement `internal/scan/directed_trace.go`; project current linear traces into directed graphs and back into legacy steps.
- [ ] Emit project/workspace `directed-traces.json`; validate deterministic ordering; commit `feat: add directed trace model`.

### Task 2: Traversal Algorithms
- [ ] Write failing tests for upstream/downstream sets, shortest entry path, persistence/message/external/test paths, branches, truncation, and bounded cycles.
- [ ] Implement pure graph traversal in `internal/trace`; reject unknown nodes and invalid bounds.
- [ ] Run focused/property-like deterministic tests; commit `feat: add bounded trace traversal`.

### Task 3: Trace From Selection Dashboard
- [ ] Write failing dashboard tests for role labels, full symbol/file/evidence detail, upstream/downstream highlighting, explicit center, paths to persistence/tests, uncertainty, and `Trace from here`.
- [ ] Render stable graph layout; selecting a node changes highlight/detail only; explicit action applies traversal result without viewport mutation.
- [ ] Keep Endpoint method/caller/provider/status filters and return viewport; commit `feat: add directed endpoint trace workbench`.

### Task 4: Query And MCP Trace Tasks
- [ ] Add failing shared-envelope tests for `endpoint-trace`, `symbol-trace`, `trace-from`, and `tests` with bounds/continuation/evidence.
- [ ] Implement agent loaders/traversal and CLI/MCP adapters with strict schemas.
- [ ] Preserve 0.9.3 task contracts; commit `feat: expose directed trace agent tasks`.

### Task 5: Language-Neutral Data Flow
- [ ] Write failing tests for objects/fields, binding, validation, transformation, serialization, persistence, messages, returns, mappings and explicit gaps.
- [ ] Implement additive `DataFlowRecord`/node/edge types; derive only from current DTO/persistence/response facts with evidence and confidence.
- [ ] Emit `data-flows.json` and `data-flows.md`; commit `feat: add language-neutral data flow model`.

### Task 6: Data Flow Dashboard, Query, And MCP
- [ ] Write failing tests for Data Flow view, exact/inferred/weak/missing legend, field mappings, gaps, search, selection stability, `data-flow` Query/MCP.
- [ ] Implement the view and shared agent adapter; no invented mappings; commit `feat: add data flow workbench`.

### Task 7: Doctor, Docs, Version, Acceptance
- [ ] Validate directed graph integrity, evidence refs, role enums, bounds, cycles, data-flow mappings and gaps.
- [ ] Update README, COMMANDS.md, release docs, primary agent reports; set 0.9.4.
- [ ] Run full tests/vet/JS/diff, install locally, clean/rescan 44 WEKA projects, exercise UI statically plus Query/MCP, Doctor, repeat hashes.
- [ ] Do not push, tag, or publish.

## Completion Gate

- [ ] Intermediate selection answers upstream, downstream, persistence/messages/external/tests, evidence and uncertainty.
- [ ] `Trace from here` is explicit and viewport-stable.
- [ ] Cycles/branches/truncation are honest and bounded.
- [ ] Data flow shows known mappings and explicit unknown gaps.
- [ ] Gate D is frozen for language parity releases.
