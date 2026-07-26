# Connected Agent Context Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject disconnected operational retry anchors while retaining connected call-chain and bounded support-evidence retries.

**Architecture:** First make the monotonic evaluator understand a declared directional retry-permission tightening without changing GoreGraph output. Then filter only `call_chain` omission retry candidates through the directed facts reachable from the selected planning seed; leave all other retry eligibility, ranking, Context Pack selection, and rendering unchanged.

**Tech Stack:** Go 1.26, Go standard library, existing `internal/agentbench` semantic evaluator, existing Schema 3 Agent Context compiler, deterministic Go fixtures, and existing shell benchmark harnesses.

## Global Constraints

- Implement exactly one production hypothesis: a disconnected `call_chain` omission cannot authorize a retry.
- Keep `DefaultContextBudgetTokens = 4000`, `DefaultContextMaxFiles = 12`, and `MaxContextSourceOmissions = 3`.
- Do not change Context Pack ranking, source selection, rendering, public schema, or agent instructions.
- Preserve retries for connected call-chain omissions and bounded non-call-chain evidence.
- Add no dependency and no benchmark-specific identifier or production rule.
- Keep evaluator preparation and production behavior in separate commits.
- Run no external Codex benchmark without fresh explicit user authorization.
- Do not release, tag, push, or promote the candidate to Golden.

---

## File and Interface Map

- `internal/agentbench/contract.go`: accepts the `retry_permission` hypothesis change kind.
- `internal/agentbench/contract_test.go`: validates the new hypothesis vocabulary.
- `internal/agentbench/pack.go`: records retry direction and permits only a declared `true` to `false` transition.
- `internal/agentbench/pack_test.go`: proves allowed tightening and rejected widening.
- `internal/agent/context_rank.go`: checks reachability only for retry candidates matching a `call_chain` omission.
- `internal/agent/context_test.go`: reproduces disconnected and connected operational omission behavior with generic fixtures.
- `docs/superpowers/specs/2026-07-26-connected-agent-context-retry-design.md`: approved design and safety boundaries.
- `docs/superpowers/plans/2026-07-26-connected-agent-context-retry.md`: executable TDD plan.

### Task 1: Allow declared retry-permission tightening

**Files:**

- Modify: `internal/agentbench/contract.go`
- Modify: `internal/agentbench/contract_test.go`
- Modify: `internal/agentbench/pack.go`
- Modify: `internal/agentbench/pack_test.go`

**Interfaces:**

- Consumes: `Hypothesis.AllowedPackChanges []string`
- Produces: allowed change kind `retry_permission`
- Produces: `PackDiff.GoldenRetryAllowed bool`
- Produces: `PackDiff.CandidateRetryAllowed bool`
- Preserves: `PackDiff.RetryChanged bool`

- [ ] **Step 1: Write failing hypothesis and Pack Diff tests**

Add `retry_permission` to a valid hypothesis and require validation to accept
it. Add Pack Diff tests with literal Golden and candidate packs:

```go
t.Run("allows declared retry permission tightening", func(t *testing.T) {
	golden := goldenPack()
	golden.RetryAllowed = true
	candidate := golden
	candidate.RetryAllowed = false
	hypothesis := Hypothesis{AllowedPackChanges: []string{"retry_permission"}}

	diff := DiffPacks(golden, candidate)
	if !diff.RetryChanged || !diff.GoldenRetryAllowed || diff.CandidateRetryAllowed {
		t.Fatalf("retry diff = %#v", diff)
	}
	if violations := EvaluatePackDiff(diff, hypothesis); len(violations) != 0 {
		t.Fatalf("declared retry tightening violations = %#v", violations)
	}
})
```

Add one subtest that omits `retry_permission` and requires a
`retry_allowed` violation. Add another that changes `false` to `true` with
`retry_permission` declared and still requires a `retry_allowed` violation.

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
go test ./internal/agentbench -run 'TestValidateHypothesis|TestEvaluatePackDiff' -count=1
```

Expected: FAIL because `retry_permission` is not valid, the directional fields
do not exist, and every retry change is rejected.

- [ ] **Step 3: Implement directional evaluator support**

Add the allowed kind:

```go
var allowedPackChangeKinds = map[string]bool{
	"locations":        true,
	"sources":          true,
	"coverage":         true,
	"omissions":        true,
	"budget":           true,
	"retry_permission": true,
}
```

Extend `PackDiff`:

```go
RetryChanged          bool `json:"retry_changed"`
GoldenRetryAllowed    bool `json:"golden_retry_allowed"`
CandidateRetryAllowed bool `json:"candidate_retry_allowed"`
```

Populate both values from `ProjectPack` in `DiffPacks`. In
`EvaluatePackDiff`, keep undeclared changes rejected and allow only:

```go
if diff.RetryChanged {
	switch {
	case !allowed["retry_permission"]:
		violations = append(violations, Violation{
			Field: "retry_allowed", Reason: "retry permission changes are not allowed",
		})
	case !diff.GoldenRetryAllowed || diff.CandidateRetryAllowed:
		violations = append(violations, Violation{
			Field: "retry_allowed",
			Reason: "retry permission may only tighten from true to false",
		})
	}
}
```

Update the hypothesis validation error to list `retry_permission`.

- [ ] **Step 4: Format and verify GREEN**

Run:

```bash
gofmt -w internal/agentbench/contract.go internal/agentbench/contract_test.go internal/agentbench/pack.go internal/agentbench/pack_test.go
go test ./internal/agentbench -count=1
go test ./scripts/agent-context-regression -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit evaluator preparation**

```bash
git add internal/agentbench/contract.go internal/agentbench/contract_test.go internal/agentbench/pack.go internal/agentbench/pack_test.go
git commit -m "Allow retry permission tightening" \
  -m "- Record Golden and candidate retry values in semantic Pack diffs
- Permit only declared true-to-false retry transitions"
```

### Task 2: Reject disconnected call-chain retry anchors

**Files:**

- Modify: `internal/agent/context_rank.go`
- Modify: `internal/agent/context_test.go`

**Interfaces:**

- Consumes: `reachableContextConcernEvidence(index, seed.ID)`
- Produces: `contextRetryFactExtendsPrimaryPath(pack ContextPack, fact scan.AgentContextFactRecord, reachable map[string]bool) bool`
- Preserves: `contextRetryPermission(pack ContextPack, index scan.AgentContextIndexRecord) (bool, []string)`

- [ ] **Step 1: Write a failing disconnected-flow regression**

Add a generic fixture with:

- selected `DELETE /catalog/items/{id}` route and its service;
- disconnected `DELETE /job-management/marked` housekeeping route;
- a budget omission for the housekeeping controller with role `call_chain`;
- an uncovered explicit `services/jobs` project concern;
- selected IDs only from the catalog flow.

Require:

```go
allowed, anchors := contextRetryPermission(pack, index)
if allowed || len(anchors) != 0 {
	t.Fatalf("disconnected call-chain retry = %v / %#v", allowed, anchors)
}
```

In the same test group, add a connected omitted `deleteCatalogAudit` fact
reachable from the selected catalog route and require its exact qualified
anchor to remain allowed.

- [ ] **Step 2: Run the regression and verify RED**

Run:

```bash
go test ./internal/agent -run 'TestContextRetryPermissionRequiresConnectedCallChainOmission' -count=1
```

Expected: FAIL because the disconnected housekeeping route is currently
accepted as a same-action, omission-matching retry anchor.

- [ ] **Step 3: Implement the minimal reachability filter**

In `contextRetryPermission`, compute:

```go
reachableFactIDs, _ := reachableContextConcernEvidence(index, seed.ID)
```

Before accepting a candidate, require:

```go
contextRetryFactExtendsPrimaryPath(pack, fact, reachableFactIDs)
```

Implement the helper so it:

1. finds omissions matching the fact's normalized project and file;
2. splits comma-separated omission roles;
3. returns `reachable[fact.ID]` when any matching role is exactly
   `call_chain`;
4. returns `true` for matching non-call-chain omissions and facts without a
   matching call-chain omission.

- [ ] **Step 4: Format and verify GREEN**

Run:

```bash
gofmt -w internal/agent/context_rank.go internal/agent/context_test.go
go test ./internal/agent -run 'TestContextRetry' -count=1
go test ./internal/agent -run 'TestBuildContextChangeAnalysisPack' -count=1
```

Expected: PASS, including existing concrete-anchor, support-omission,
unrenderable-omission, and opposite-action regressions.

- [ ] **Step 5: Commit the production hypothesis**

```bash
git add internal/agent/context_rank.go internal/agent/context_test.go
git commit -m "Reject disconnected context retries" \
  -m "- Require omitted call-chain anchors to extend the selected production flow
- Preserve bounded retries for connected and support evidence"
```

### Task 3: Verify deterministic compatibility and G1 behavior

**Files:**

- Modify only if verification finds a defect inside the approved hypothesis.

**Interfaces:**

- Consumes: committed G2–G6 matrix and existing shell harnesses.
- Produces: fresh local verification evidence.
- Does not produce: an external Codex transcript.

- [ ] **Step 1: Run complete repository verification**

Run:

```bash
go test ./... -count=1
go vet ./...
go test -race ./internal/agent ./internal/agentbench ./scripts/agent-context-regression -count=1
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
```

Expected: every command exits zero.

- [ ] **Step 2: Build the candidate binary**

Run:

```bash
go build -o /tmp/goregraph-connected-retry ./cmd/goregraph
```

Expected: exit zero.

- [ ] **Step 3: Verify the direct G1 Context Pack locally**

Run the candidate binary against the already scanned G1 workspace with the
fixed German query, `--token-budget 4000`, and `--max-files 20`. Capture the
JSON outside the repository.

Require with `jq -e`:

```jq
.estimated_tokens <= 4000
and .retry_allowed == false
and ((.retry_anchors // []) | length == 0)
and (.fallback_required == false)
and ((.source_omissions // []) | length <= 3)
```

Compare the direct Pack with the retained candidate Pack and confirm that
protected semantic fields are unchanged and the retry transition is
`true` to `false`.

- [ ] **Step 4: Check the worktree and stop before external execution**

Run:

```bash
git status --short --branch
git log -3 --oneline
```

Expected: no uncommitted implementation changes. Do not run Codex, push,
release, tag, or promote the candidate.
