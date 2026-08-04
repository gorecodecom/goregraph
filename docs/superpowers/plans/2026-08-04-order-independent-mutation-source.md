# Order-Independent Mutation Source Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Select the triggering mutation endpoint independently of dependent-symptom word order and fail closed when source evidence is ambiguous.

**Architecture:** Replace the one-direction relation-then-lingering check with clause-aware, order-independent symptom spans. Continue using source-derived ordered route ancestry for clear source selection, and add an ambiguity gate before utility ranking when no source evidence exists.

**Tech Stack:** Go 1.23+ standard library, existing deterministic agent Context ranking, schema 3 test fixtures.

## Global Constraints

- Add no dependencies.
- Add no private repository, service, organization, or business-domain identifiers to production code.
- Do not translate business nouns.
- Preserve the 4,000-token Context budget, one-call guide, fallback, and retry contracts.
- Keep direct dependent-entity mutations unchanged.

---

### Task 1: Reproduce both symptom word orders

**Files:**
- Modify: `internal/agent/context_alias_test.go`

**Interfaces:**
- Consumes: `selectContextEndpoint`, `rankContextFacts`, and the existing neutral catalog/jobs fixture shape.
- Produces: regression coverage for relation-before-lingering and lingering-before-relation queries.

- [ ] **Step 1: Add the failing inverted-order case**

Extend `TestSelectContextEndpointUsesMutationSourceBeforeLingeringDependent` with this literal query and assert `catalog-endpoint`:

```go
"In der Anwendung besteht ein Fehler: Wenn ein Eintrag aus einer Sammlung entfernt wird, " +
    "bleiben die damit verbundenen Aufgaben bestehen. Ermittle den öffentlichen REST-Endpunkt."
```

- [ ] **Step 2: Run the focused test and verify RED**

```bash
go test ./internal/agent -run TestSelectContextEndpointUsesMutationSourceBeforeLingeringDependent -count=1
```

Expected: FAIL because the inverted word order selects a dependent mutation endpoint.

- [ ] **Step 3: Add a short unrelated mutation distractor**

Add an exact production `api_endpoint` fixture for `DELETE /job-management/maintenance`. Give it dependent-language relevance without using private identifiers.

- [ ] **Step 4: Add index-order stability coverage**

Run selection with forward and reversed facts. Assert `catalog-endpoint` for both literal orders.

### Task 2: Implement order-independent symptom spans

**Files:**
- Modify: `internal/agent/context_rank.go:2518-2645`
- Test: `internal/agent/context_alias_test.go`

**Interfaces:**
- Consumes: `contextPrimaryQuery`, `contextOrderedTokens`, `contextEndpointRelatedToken`, `contextEndpointLingeringToken`.
- Produces: a shared clause/span result for source-clause extraction and endpoint ranking.

- [ ] **Step 1: Introduce a focused span type**

```go
type contextEndpointTokenSpan struct {
    start int
    end   int
}
```

- [ ] **Step 2: Detect marker order symmetrically**

Record the first relation and lingering indexes after the mutation. Accept both orders when both markers occur in the immediately following clause, or when a consequence boundary separates them from the mutation target in the same clause. Use the earlier marker as symptom start.

- [ ] **Step 3: Preserve the direct-target guard**

Do not classify `Entferne verbundene Aufgaben, die bestehen bleiben` as source plus symptom: its relation marker belongs to the mutation target, while the following clause has no relation marker.

- [ ] **Step 4: Reuse one span implementation**

Replace the parallel boundary and boolean checks with the shared clause/span result.

- [ ] **Step 5: Verify GREEN**

```bash
go test ./internal/agent -run 'TestSelectContextEndpoint(UsesMutationSourceBeforeLingeringDependent|PrefersTransitionSourceOverTargetUtility)' -count=1
```

Expected: PASS.

### Task 3: Fail closed without source evidence

**Files:**
- Modify: `internal/agent/context_rank.go:2397-2504`
- Test: `internal/agent/context_alias_test.go`

**Interfaces:**
- Consumes: symptom detection, exact anchors, transition-source score, structural source scores, source-clause domain matches.
- Produces: the existing endpoint ambiguity reason when no unique source exists.

- [ ] **Step 1: Add a failing disconnected ambiguity test**

Use neutral action-aligned endpoints with non-overlapping ordered paths and untranslated business nouns. Assert `ok == false` and an ambiguity reason.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/agent -run TestSelectContextEndpointRejectsLingeringMutationWithoutSourceEvidence -count=1
```

Expected: FAIL because an arbitrary short endpoint is selected.

- [ ] **Step 3: Add source-domain evidence scoring**

Count only non-action, non-transport source-clause matches. Keep exact anchors, transition sources, and ordered structural source scores stronger than lexical evidence.

- [ ] **Step 4: Add the ambiguity gate**

Before utility ranking, return `contextEndpointProviderAmbiguityReason` when a dependent symptom exists, multiple mutation candidates remain, and no candidate has unique source evidence.

- [ ] **Step 5: Verify direct-target and ambiguity behavior**

```bash
go test ./internal/agent -run 'TestSelectContextEndpoint(UsesMutationSourceBeforeLingeringDependent|RejectsLingeringMutationWithoutSourceEvidence)' -count=1
```

Expected: PASS.

### Task 4: Verify generality and historical behavior

**Files:**
- Modify only for demonstrated regressions: `internal/agent/context_rank.go`, `internal/agent/context_alias_test.go`

**Interfaces:**
- Consumes: completed endpoint selection behavior.
- Produces: release evidence for the new candidate.

- [ ] **Step 1: Run the complete agent package**

```bash
go test ./internal/agent -count=1
```

- [ ] **Step 2: Run repository tests and Vet**

```bash
go test ./... -timeout 20m -count=1
go vet ./...
```

- [ ] **Step 3: Run the affected race gate**

```bash
go test -race ./internal/agent -timeout 20m -count=1
```

- [ ] **Step 4: Run documentation synchronization**

```bash
go run ./scripts/sync-docs --check
```

- [ ] **Step 5: Run the historical Context assertion**

Build an exact 1.3.0 candidate and use the complete benchmark wording. Require one regulation-removal DELETE entrypoint, MEDIUM confidence, no fallback, no retry, at most 4,000 estimated tokens, and the missing-contract uncertainty.

- [ ] **Step 6: Audit generality**

Confirm the production diff contains no private service or business identifiers and reversed fixture/index order remains deterministic.

### Task 5: Secure and install the candidate

**Files:**
- Commit the spec and plan separately from implementation behavior.

**Interfaces:**
- Consumes: successful verification output.
- Produces: pushed branch, exact local binary, freshly scanned test workspace.

- [ ] **Step 1: Commit and push documentation**

Use an English imperative commit with bullets covering the generic design and executable gates.

- [ ] **Step 2: Commit and push implementation**

Use an English imperative commit with bullets covering symmetric source selection and fail-closed ambiguity.

- [ ] **Step 3: Build and install the exact commit**

Embed version `1.3.0`, full commit SHA, and UTC build timestamp. Verify installed and candidate SHA-256 hashes match.

- [ ] **Step 4: Clean and rescan the historical workspace**

Preview `workspace clean`, remove only listed generated paths, then run `workspace scan-all` with explicit `--workspace` and `--no-update-gitignore`.

- [ ] **Step 5: Run final installed-candidate gates**

Require Doctor success, schema 3, GoreGraph 1.3.0, no stale artifacts, indexed status for all three projects, and a true final Context assertion.
