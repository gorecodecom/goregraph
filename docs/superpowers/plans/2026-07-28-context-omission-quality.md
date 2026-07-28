# Context Omission Quality Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep missing future transitions explicit while using the bounded omission budget for action-aligned domain and persistence evidence instead of disconnected operational endpoints.

**Architecture:** Add a narrow omission-eligibility rule after concern planning and before final omission ranking. The rule uses existing action, query, fact-kind, reachability, role, and requested-domain signals; it does not alter entrypoint selection or fabricate a future call chain.

**Tech Stack:** Go standard library, existing generic catalog/job fixtures, deterministic G2-G6 agent regression matrix.

## Global Constraints

- Existing current production entrypoint and reachable call chain remain unchanged.
- An absent future call, route, or API contract stays an explicit unknown.
- A disconnected maintenance, housekeeping, batch, or cleanup endpoint is not a call-chain omission unless explicitly requested.
- Action-aligned domain model and persistence declarations outrank unrelated endpoint bodies.
- At most three path-and-line-bounded omissions are emitted.
- No private project, path, symbol, or answer string enters production or test code.
- This hypothesis does not change public schema, fallback rules, or retry rules.

---

### Task 1: Reproduce the Disconnected Operational Omission

**Files:**
- Modify: `internal/agent/context_change_analysis_test.go`
- Modify: `internal/agent/context_source_test.go`

**Interfaces:**
- Consumes: generic catalog/job facts with a current catalog deletion path, absent future job-delete contract, disconnected housekeeping route, and two requested job model/repository families.
- Produces: a regression test that fails when the housekeeping endpoint consumes an omission slot.

- [ ] **Step 1: Add a failing end-to-end Context test**

Build a generic fixture where:

```text
current path: DELETE catalog -> CatalogService -> CatalogRepository
missing future path: catalog deletion -> job deletion client/provider
disconnected operation: POST /internal/jobs/housekeeping
requested evidence: regular job model/repository and change-job model/repository
```

Assert the pack keeps the catalog entrypoint, reports the missing job-delete
contract as unknown, has `fallback_required=false` and `retry_allowed=false`,
contains no housekeeping path in `source_omissions`, and uses bounded omissions
for requested model/persistence evidence when those sections do not fit.

- [ ] **Step 2: Add focused eligibility table tests**

Cover:

```text
disconnected housekeeping + not requested -> rejected
disconnected cleanup + explicitly requested -> allowed
reachable operational method -> allowed
domain model declaration -> allowed
persistence declaration -> allowed
ordinary disconnected call-chain evidence -> rejected only for missing-transition planning
```

- [ ] **Step 3: Run focused tests and verify red**

Run:

```bash
go test ./internal/agent -run 'DisconnectedOperationalOmission|MissingTransitionOmission' -count=1
```

Expected: FAIL because the current omission selector admits the lexically
similar disconnected operation.

### Task 2: Filter and Rank Omission Candidates

**Files:**
- Modify: `internal/agent/context_select.go`
- Modify: `internal/agent/context_source.go`
- Modify: `internal/agent/context_change_analysis_test.go`
- Modify: `internal/agent/context_source_test.go`

**Interfaces:**
- Produces: `contextSourceOmissionCandidateAllowed(pack ContextPack, concern contextConcern, candidate sourceCandidate, index scan.AgentContextIndexRecord) bool`.
- Produces: deterministic omission ranking that prefers requested domain and persistence evidence.

- [ ] **Step 1: Implement the minimal eligibility helper**

Return false only when all are true:

1. planning has an explicit missing future transition concern;
2. candidate role is `call_chain` or entrypoint-like;
3. candidate is not reachable from the selected production path;
4. candidate action tokens identify maintenance, housekeeping, batch, cleanup,
   purge, sweep, archive, or repair behavior;
5. the query does not explicitly request those action tokens.

Domain-model, persistence, contract, and test evidence use their existing
concern rules and are not rejected by this helper.

- [ ] **Step 2: Apply eligibility before omission scoring**

Filter `matching` candidates in `contextSourceConcernOmission` before
`contextSourceOmissionFacetScore`. When filtered candidates are empty, keep the
concern pathless and explicit rather than substituting another file.

- [ ] **Step 3: Prefer requested model and repository declarations**

For omission candidates with bounded source, order:

```text
requested domain model
requested persistence owner or repository declaration
requested contract/configuration/auth/retry evidence
other eligible bounded evidence
pathless explicit unknown
```

Reuse existing requested-model and stable-identity helpers. Do not add a second
semantic vocabulary.

- [ ] **Step 4: Run focused tests and verify green**

Run:

```bash
gofmt -w internal/agent/context_select.go internal/agent/context_source.go internal/agent/context_change_analysis_test.go internal/agent/context_source_test.go
go test ./internal/agent -run 'DisconnectedOperationalOmission|MissingTransitionOmission|MissingContract|SourceOmission|SourceConcern' -count=1
```

Expected: PASS without changing the selected current endpoint, fallback, retry,
or three-omission cap.

### Task 3: Protect the Generic Regression Matrix

**Files:**
- Modify: `testdata/agent-context-regression/g2-java-missing-contract/contract.json`
- Modify: `internal/agentbench/matrix_test.go`
- Modify: `internal/agent/context_change_analysis_test.go`

**Interfaces:**
- Consumes: committed G2-G6 matrix.
- Produces: deterministic contract protection against unrelated operational omissions.

- [ ] **Step 1: Tighten the generic G2 contract**

Add the disconnected housekeeping endpoint body to
`forbidden_source_suffixes` only when the query does not request housekeeping.
Keep required adjacent side-effect evidence represented by an action-aligned
generic source fixture so the contract does not suppress a requested facet.

- [ ] **Step 2: Run the committed deterministic matrix**

Run the repository's existing G2-G6 pack generation and verification commands
for English and German query variants, repeated enough to prove byte-stable
packs. Expected: all contracts pass and repeated outputs are identical.

- [ ] **Step 3: Run complete local verification**

Run:

```bash
go vet ./...
go test ./... -count=1
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
go run ./scripts/sync-docs --check
git diff --check
```

Expected: all checks pass.

- [ ] **Step 4: Commit the hypothesis**

```bash
git add internal/agent testdata/agent-context-regression internal/agentbench/matrix_test.go
git commit -m "Prefer action-aligned Context omissions" -m "- Exclude disconnected maintenance operations from missing-transition omission slots.
- Prefer requested domain and persistence declarations for bounded follow-up evidence.
- Preserve explicit future-contract uncertainty, entrypoint selection, and retry behavior."
```

### Task 4: Freeze the Candidate for External Evaluation

**Files:**
- Modify: no production files
- Create outside repository: frozen candidate binary and benchmark evidence directory

**Interfaces:**
- Consumes: final clean candidate commit and prior valid Golden commit.
- Produces: evidence required for a release decision, not a release itself.

- [ ] **Step 1: Verify repository identity**

Run:

```bash
git status --short
git rev-parse HEAD
git log -1 --format=%B
```

Expected: clean tree and one final candidate commit ID.

- [ ] **Step 2: Build the frozen candidate**

Build with version, commit, and build timestamp ldflags into a private temporary
directory. Record checksum and `goregraph version`.

- [ ] **Step 3: Stop before private external execution without authorization**

The external G1 full three-pair regression and matched three-by-three release
benchmark transmit private workspace content. Run them only with a new explicit
count-based authorization. Until then, report the deterministic local result as
ready for external evaluation, not release-ready.
