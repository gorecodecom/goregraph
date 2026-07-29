# Agent Benchmark Causal Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the agent benchmark enforce internal Pack invariants deterministically and classify unchanged-pack efficiency drift as model variance instead of a GoreGraph product regression.

**Architecture:** Extend the existing Pack contract with an optional endpoint-count bound. Pass the runner's semantic `PackDiff` into the case gate, keep quality and per-run safety failures strict, and turn only comparative efficiency breaches into observations when the semantic Pack Diff is unchanged.

**Tech Stack:** Go standard library, existing `internal/agentbench` package, internal benchmark command, JSON fixture contracts, Markdown documentation.

## Global Constraints

- Do not change production Context ranking or scanning behavior.
- Do not add dependencies or change the public GoreGraph CLI.
- Do not rescore retained evidence retroactively.
- Keep answer quality and bounded omission-read safety strict.
- Apply comparative efficiency thresholds unchanged when the semantic Pack Diff changed.
- Do not use private workspace material or external Codex runs for implementation verification.
- Do not merge to `main` or release.

---

### Task 1: Move the G3 representation rule into Pack evaluation

**Files:**
- Modify: `internal/agentbench/contract.go`
- Modify: `internal/agentbench/contract_test.go`
- Modify: `internal/agentbench/pack.go`
- Modify: `internal/agentbench/pack_test.go`
- Modify: `testdata/agent-context-regression/g3-go-existing-flow/contract.json`

**Interfaces:**
- Produces: `PackExpectation.MaxEndpoints *int`
- Consumes: `agent.ContextPack.Endpoints`
- Preserves: existing endpoint matcher and all existing Pack limits

- [ ] **Step 1: Write failing contract-validation tests**

Add tests that accept `max_endpoints: 0` and reject a negative value:

```go
func TestLoadContractAcceptsZeroMaximumEndpoints(t *testing.T) {
	var body map[string]any
	if err := json.Unmarshal(mustJSON(t, validContract()), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	body["pack"].(map[string]any)["max_endpoints"] = 0

	contract, err := LoadContract(writeJSON(t, body))
	if err != nil {
		t.Fatalf("LoadContract returned error: %v", err)
	}
	if contract.Pack.MaxEndpoints == nil || *contract.Pack.MaxEndpoints != 0 {
		t.Fatalf("MaxEndpoints = %#v, want pointer to zero", contract.Pack.MaxEndpoints)
	}
}
```

Add this validation case:

```go
{
	name: "rejects a negative endpoint maximum",
	mutate: func(contract *Contract) {
		value := -1
		contract.Pack.MaxEndpoints = &value
	},
	wantErr: "pack.max_endpoints",
},
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
go test ./internal/agentbench -run 'TestLoadContractAcceptsZeroMaximumEndpoints|TestValidateContract' -count=1
```

Expected: compilation fails because `PackExpectation.MaxEndpoints` does not
exist.

- [ ] **Step 3: Add the minimal contract field and validation**

Add to `PackExpectation`:

```go
MaxEndpoints *int `json:"max_endpoints,omitempty"`
```

Add to `validatePack`:

```go
if pack.MaxEndpoints != nil && *pack.MaxEndpoints < 0 {
	return fmt.Errorf("pack.max_endpoints must not be negative")
}
```

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run the command from Step 2.

Expected: PASS.

- [ ] **Step 5: Write the failing Pack behavior test**

Add a subtest to `TestEvaluatePackReportsExpectationViolations` or a focused
test:

```go
func TestEvaluatePackEnforcesMaximumEndpoints(t *testing.T) {
	maximum := 0
	pack := goldenPack()

	violations := EvaluatePack(pack, PackExpectation{MaxEndpoints: &maximum})

	requireViolation(t, violations, "endpoint_count")
}
```

The production change this catches is omission of the endpoint-count contract
check, which would let a generic route leak into `ContextPack.Endpoints`.

- [ ] **Step 6: Run the Pack test and verify RED**

Run:

```bash
go test ./internal/agentbench -run TestEvaluatePackEnforcesMaximumEndpoints -count=1
```

Expected: FAIL because `EvaluatePack` returns no `endpoint_count` violation.

- [ ] **Step 7: Implement the endpoint-count check**

Add to `EvaluatePack`:

```go
if expectation.MaxEndpoints != nil && len(pack.Endpoints) > *expectation.MaxEndpoints {
	violations = append(violations, Violation{
		Field:  "endpoint_count",
		Reason: fmt.Sprintf("endpoint count %d exceeds maximum %d", len(pack.Endpoints), *expectation.MaxEndpoints),
	})
}
```

- [ ] **Step 8: Update the G3 fixture**

Set:

```json
"max_endpoints": 0
```

Remove `structured-go-endpoint-record` from `answer.explicit_unknowns`, leaving:

```json
"explicit_unknowns": []
```

The existing required `DELETE /orders/{orderId}` entrypoint remains unchanged.

- [ ] **Step 9: Run focused and matrix tests**

Run:

```bash
go test ./internal/agentbench -run 'TestLoadContract|TestValidateContract|TestEvaluatePack|TestCommittedBenchmarkMatrix' -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit Task 1**

```bash
git add internal/agentbench/contract.go internal/agentbench/contract_test.go internal/agentbench/pack.go internal/agentbench/pack_test.go testdata/agent-context-regression/g3-go-existing-flow/contract.json
git commit -m "Enforce G3 endpoint representation in packs" \
  -m "- Add an optional endpoint-count bound to benchmark contracts
- Move the generic Go route representation rule out of answer review
- Verify the G3 invariant in deterministic Pack evaluation"
```

### Task 2: Make efficiency attribution depend on the semantic Pack Diff

**Files:**
- Modify: `internal/agentbench/pack.go`
- Modify: `internal/agentbench/pack_test.go`
- Modify: `internal/agentbench/review.go`
- Modify: `internal/agentbench/review_test.go`

**Interfaces:**
- Produces: `HasSemanticPackChanges(diff PackDiff) bool`
- Changes: `EvaluateCase(contract Contract, golden, candidate []ReviewedRun, hypothesis Hypothesis, diff PackDiff) GateReport`
- Extends: `GateReport.Observations []string`

- [ ] **Step 1: Write failing Pack-Diff identity tests**

Add:

```go
func TestHasSemanticPackChanges(t *testing.T) {
	if HasSemanticPackChanges(DiffPacks(goldenPack(), goldenPack())) {
		t.Fatal("identical semantic packs reported a change")
	}
	diff := DiffPacks(goldenPack(), goldenPack())
	diff.AddedSources = []string{"services/catalog/New.java"}
	if !HasSemanticPackChanges(diff) {
		t.Fatal("added source did not report a semantic change")
	}
}
```

Also cover equal `GoldenRetryAllowed` and `CandidateRetryAllowed` values of
`true`; those raw values must not count as a change when `RetryChanged` is
false.

- [ ] **Step 2: Run the identity test and verify RED**

Run:

```bash
go test ./internal/agentbench -run TestHasSemanticPackChanges -count=1
```

Expected: compilation fails because the function does not exist.

- [ ] **Step 3: Implement semantic change detection**

Add:

```go
func HasSemanticPackChanges(diff PackDiff) bool {
	return diff.EndpointChanged ||
		len(diff.AddedLocations) > 0 ||
		len(diff.RemovedLocations) > 0 ||
		len(diff.AddedSources) > 0 ||
		len(diff.RemovedSources) > 0 ||
		len(diff.AddedOmissions) > 0 ||
		len(diff.RemovedOmissions) > 0 ||
		diff.CoverageChanged ||
		diff.BudgetChanged ||
		diff.FallbackChanged ||
		diff.RetryChanged ||
		diff.UncertaintyChanged
}
```

- [ ] **Step 4: Run the identity test and verify GREEN**

Run the command from Step 2.

Expected: PASS.

- [ ] **Step 5: Write failing unchanged-Pack efficiency tests**

Change `passingCase` to return a semantic diff with one declared source change,
so every existing threshold test keeps its current hard-failure meaning.

Add a focused subtest:

```go
t.Run("retains unchanged-pack efficiency drift as model variance", func(t *testing.T) {
	contract, golden, candidate, hypothesis, _ := passingCase()
	for index := range candidate {
		candidate[index].Metrics.ToolCalls = 11
		candidate[index].Metrics.UnauthorizedSourceReads = 1
		candidate[index].Metrics.Tokens = 106
		candidate[index].Metrics.ContextMillis = 201
	}

	report := EvaluateCase(contract, golden, candidate, hypothesis, PackDiff{})
	if !report.Passed || len(report.Failures) != 0 {
		t.Fatalf("unchanged-Pack report = %#v, want pass", report)
	}
	for _, fragment := range []string{
		"model variance",
		"candidate median tool calls",
		"candidate median unauthorized source reads",
		"candidate median tokens",
		"candidate median Context latency",
		"candidate Context latency",
	} {
		requireObservation(t, report, fragment)
	}
})
```

Add another test proving `BoundedOmissionReads` above the contract remains a
failure with `PackDiff{}`.

- [ ] **Step 6: Run the evaluator tests and verify RED**

Run:

```bash
go test ./internal/agentbench -run TestEvaluateCase -count=1
```

Expected: compilation fails because `EvaluateCase` has no Pack Diff parameter
and `GateReport` has no observations.

- [ ] **Step 7: Split safety from comparative efficiency**

Extend:

```go
type GateReport struct {
	Passed       bool     `json:"passed"`
	Failures     []string `json:"failures"`
	Observations []string `json:"observations,omitempty"`
}
```

Refactor `appendEfficiencyFailures` into:

```go
func appendBoundedReadFailures(candidate map[int]ReviewedRun, limits EfficiencyLimits, failures *[]string)
func comparativeEfficiencyFindings(golden, candidate map[int]ReviewedRun, limits EfficiencyLimits) []string
```

In `EvaluateCase`, always append bounded-read failures. Then:

```go
findings := comparativeEfficiencyFindings(goldenValid, candidateValid, contract.Limits)
if HasSemanticPackChanges(diff) {
	failures = append(failures, findings...)
} else {
	for _, finding := range findings {
		observations = append(observations,
			"unchanged semantic Pack Diff; retained as model variance: "+finding)
	}
}
return gateReport(failures, observations)
```

Keep the exact existing comparative threshold calculations and messages.

- [ ] **Step 8: Run evaluator tests and verify GREEN**

Run the command from Step 6.

Expected: PASS.

- [ ] **Step 9: Run all agentbench tests**

Run:

```bash
go test ./internal/agentbench -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit Task 2**

```bash
git add internal/agentbench/pack.go internal/agentbench/pack_test.go internal/agentbench/review.go internal/agentbench/review_test.go
git commit -m "Attribute unchanged-pack efficiency drift" \
  -m "- Detect whether a semantic Pack Diff contains product changes
- Retain unchanged-pack cost regressions as model-variance observations
- Keep quality, bounded-read, and changed-pack efficiency failures strict"
```

### Task 3: Require Pack Diff evidence in the gate command

**Files:**
- Modify: `scripts/agent-context-regression/main.go`
- Modify: `scripts/agent-context-regression/main_test.go`

**Interfaces:**
- Adds required gate flag: `--pack-diff <absolute path>`
- Consumes: strict JSON `agentbench.PackDiff`
- Preserves: output no-overwrite behavior and evaluation exit codes

- [ ] **Step 1: Write failing command tests**

Add `packDiff string` to `commandFixture`, initialize it, and write
`agentbench.PackDiff{AddedSources: []string{"services/catalog/New.java"}}`.
Add `--pack-diff` to `gateArgs`.

Add a test that removes the flag pair and expects exit code `2`, no output, and
a diagnostic containing `--pack-diff`.

Add a test that writes an unknown field into the Pack Diff JSON and expects
exit code `2` without publishing the gate report.

- [ ] **Step 2: Run command tests and verify RED**

Run:

```bash
go test ./scripts/agent-context-regression -run 'TestRunGate|TestRunRejectsMalformedJSON' -count=1
```

Expected: the new valid gate invocation fails because `--pack-diff` is unknown.

- [ ] **Step 3: Parse and load the Pack Diff**

Add `pack-diff` to the required gate flags and validated input paths:

```go
flags, err := parseFileFlags(
	args,
	"contract",
	"hypothesis",
	"pack-diff",
	"golden-runs",
	"candidate-runs",
	"output",
)
```

Load it before run evaluation:

```go
diff, err := loadStrictJSON[agentbench.PackDiff](flags["pack-diff"])
if err != nil {
	return commandError(stderr, "%v", err)
}
```

Pass `diff` to `EvaluateCase`.

- [ ] **Step 4: Run command tests and verify GREEN**

Run:

```bash
go test ./scripts/agent-context-regression -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 3**

```bash
git add scripts/agent-context-regression/main.go scripts/agent-context-regression/main_test.go
git commit -m "Require Pack Diff evidence for agent gates" \
  -m "- Load the recorded semantic Pack Diff for case evaluation
- Reject missing or malformed causal evidence before publishing a gate report
- Preserve deterministic gate output and no-overwrite behavior"
```

### Task 4: Document and verify the stabilized benchmark

**Files:**
- Modify: `docs/BENCHMARKING.md`

**Interfaces:**
- Documents: deterministic Pack invariants, causal efficiency attribution,
  `--pack-diff`, contract freeze, and retained observations

- [ ] **Step 1: Update benchmark rules**

Document:

- internal Pack representation belongs in `pack`, never the answer rubric;
- unchanged semantic Pack Diffs keep quality gates hard but turn comparative
  efficiency threshold breaches into model-variance observations;
- changed semantic Pack Diffs retain all existing comparative thresholds;
- `gate` requires the runner's case/query `pack-diff.json`;
- contract changes apply only to future frozen runs and never retroactively
  rescore retained evidence.

- [ ] **Step 2: Format and inspect**

Run:

```bash
gofmt -w internal/agentbench/contract.go internal/agentbench/contract_test.go internal/agentbench/pack.go internal/agentbench/pack_test.go internal/agentbench/review.go internal/agentbench/review_test.go scripts/agent-context-regression/main.go scripts/agent-context-regression/main_test.go
git diff --check
```

Expected: no output from `git diff --check`.

- [ ] **Step 3: Run focused and harness verification**

Run:

```bash
go test ./internal/agentbench ./scripts/agent-context-regression -count=1
bash scripts/benchmark-agent-context-regression_test.sh
```

Expected: Go packages PASS and the shell harness prints
`PASS: benchmark-agent-context-regression`.

- [ ] **Step 4: Run full verification**

Run:

```bash
go test ./... -count=1
```

Expected: every package passes with exit code `0`.

- [ ] **Step 5: Review scope and requirements**

Run:

```bash
git status --short
git diff --stat HEAD~3
git diff --check HEAD~3
```

Confirm production scanner/ranking files are unchanged, the G3 answer facet is
absent, `max_endpoints` is present, and the gate requires Pack Diff evidence.

- [ ] **Step 6: Commit documentation**

```bash
git add docs/BENCHMARKING.md
git commit -m "Document causal agent benchmark gates" \
  -m "- Separate deterministic Pack invariants from answer review
- Explain unchanged-pack model-variance observations
- Record the Pack Diff evidence and contract-freeze requirements"
```

- [ ] **Step 7: Push the branch without releasing**

```bash
git push origin chore/monotonic-agent-context-evaluation
```

Expected: the remote branch advances to the verified local HEAD. Do not merge,
tag, publish artifacts, or create a release.
