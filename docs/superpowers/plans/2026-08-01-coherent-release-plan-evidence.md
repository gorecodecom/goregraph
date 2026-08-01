# Coherent Release-Plan Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make natural cross-service release plans receive one complete, bounded configuration and production/test file evidence bundle across real scanner fact shapes.

**Architecture:** Extend only exact-inventory matching and its bounded path selection. Recognize generic configuration and repository owner symbols, reserve coherent sibling paths inside a dedicated cap no larger than the public file limit, and leave ordinary evidence planning unchanged.

**Tech Stack:** Go 1.26, Go standard library, existing `internal/agent` selector and table-driven tests

## Global Constraints

- Keep the 4,000-token Context Pack limit.
- Keep the 12-file and 12-source-section limits.
- Keep the three bounded source-omission limit.
- Do not add private identifiers, dependencies, prompt instructions, retries, or fallbacks.
- Do not expose configuration values.
- Preserve deterministic output and mandatory entrypoint/current-path evidence.

---

### Task 1: Reproduce runtime scanner shapes

**Files:**
- Modify: `internal/agent/context_change_analysis_test.go`
- Modify: `internal/agent/context_exact_inventory_review_test.go`

**Interfaces:**
- Consumes: `BuildContext`, `expandContextEvidenceConcerns`, and the existing generic release fixture.
- Produces: a generic runtime-shape release fixture and literal path assertions.

- [ ] **Step 1: Write the failing integration regression**

Change only a dedicated copy of the generic fixture so its config owner and tests are symbol facts, its server policy is endpoint-security evidence, and both repository owners are available. Require one final pack to represent the client/config, server security/controller, both repositories, production/test-profile resources, and controller/service or retry-pattern tests.

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/agent -run 'TestBuildContextKeepsCoherentReleasePlanEvidence|TestExpandContextExactInventoryConcernsAcceptsRuntimeOwnerShapes' -count=1
```

Expected: FAIL because symbol-shaped config owners and persistence sibling paths are not exact-inventory candidates or are displaced by the eight-path cap.

### Task 2: Accept bounded owner shapes and coherent siblings

**Files:**
- Modify: `internal/agent/context_source.go`
- Modify: `internal/agent/context_select.go`
- Test: `internal/agent/context_exact_inventory_review_test.go`

**Interfaces:**
- Produces: `contextExactInventoryConfigurationOwnerFact(scan.AgentContextFactRecord) bool`.
- Produces: `contextExactInventoryPersistenceOwnerFact(scan.AgentContextFactRecord) bool`.
- Produces: `maximumContextExactInventoryCandidates`, bounded by `DefaultContextMaxFiles`.

- [ ] **Step 1: Implement minimal matching**

Accept only exact-confidence, path-backed, production symbols whose file/owner identity ends in conventional configuration suffixes. Accept persistence facts and repository owner symbols for persistence inventory. Preserve the existing domain-anchor filter and deduplicate facts by project/path.

- [ ] **Step 2: Implement bounded coherent selection**

Use the dedicated exact-inventory cap only when exact inventory was requested. Select one path for each requested category first, then prefer a configuration owner/resource scope pair and a second distinct persistence path before ordinary score-ordered fill. Never exceed `DefaultContextMaxFiles`.

- [ ] **Step 3: Verify GREEN**

Run:

```bash
gofmt -w internal/agent/context_source.go internal/agent/context_select.go internal/agent/context_change_analysis_test.go internal/agent/context_exact_inventory_review_test.go
go test ./internal/agent -run 'TestBuildContextKeepsCoherentReleasePlanEvidence|TestExpandContextExactInventoryConcernsAcceptsRuntimeOwnerShapes|TestExpandContextExactInventoryConcernsBalancesKindsAndProjectsAtLimit' -count=1
```

Expected: PASS with deterministic output and unchanged public limits.

### Task 3: Full verification and candidate preparation

**Files:**
- Modify only if required by generated documentation checks.

- [ ] **Step 1: Run the complete local gates**

```bash
go test ./internal/agent -count=1
go test ./... -count=1
go vet ./...
bash scripts/analyze-agent-context-log_test.sh
bash scripts/calibrate-agent-context-tokens_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
go run ./scripts/sync-docs --check
git diff --check
```

- [ ] **Step 2: Commit and push the verified implementation**

Create a separate English implementation commit, push `fix/release-benchmark-metrics`, install the exact commit, clean and rescan the unchanged G1 workspace, and verify `doctor` plus workspace status.

- [ ] **Step 3: Run the authorized release matrix**

Use six of the ten newly authorized read-only external runs for one identical 3×3 matrix. Retain all artifacts and manually score all six answers before considering any of the remaining four runs.

