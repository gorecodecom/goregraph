# Bounded Omission Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Measure exact reads authorized by a preceding Context Pack separately from unauthorized source access while retaining total read, navigation, token, and tool costs.

**Architecture:** Extend the transcript analyzer's event-ordered Context Pack state with bounded omission ranges and classify each terminal source-access command atomically. Carry the two new counters through the regression runner, retained metrics, review JSON, TSV output, and monotonic gate; the existing release benchmark continues to gate total `source_read_calls`.

**Tech Stack:** Go standard library, JSONL transcript fixtures, Bash test harnesses, Go unit tests.

## Global Constraints

- A bounded read requires a preceding full pack, exact project/path, a positive contained line range, no search or inventory, no included-source overlap, and no additional target.
- Pathless or unbounded omissions grant no permission.
- One unauthorized target makes the complete terminal call unauthorized.
- Total source reads, raw navigation, tool calls, tokens, Context calls, and included-source rereads remain visible.
- The standard no-GoreGraph versus assisted release gate continues to use total source reads unchanged.
- No private workspace names, paths, or answer strings enter production code or fixtures.

---

### Task 1: Classify Event-Ordered Omission Reads

**Files:**
- Modify: `scripts/analyze-agent-context-log.go`
- Modify: `scripts/analyze-agent-context-log_test.sh`

**Interfaces:**
- Consumes: JSON and Markdown Context Packs embedded in completed terminal events.
- Produces: analyzer columns `bounded_omission_read_calls` and `unauthorized_source_read_calls` after `source_read_calls`.

- [ ] **Step 1: Add failing transcript fixtures**

Add JSON and Markdown packs followed by exact, subset, widened, wrong-path,
pathless, pre-pack, included-overlap, search, and compound reads. Assert:

```text
exact/subset -> bounded_omission_read_calls
widened/wrong/pathless/pre-pack/overlap/search/compound -> unauthorized_source_read_calls
```

Retain a duplicate completed event for one bounded command and assert that it is
counted once.

- [ ] **Step 2: Run the analyzer harness and verify red**

Run:

```bash
bash scripts/analyze-agent-context-log_test.sh
```

Expected: FAIL because the analyzer header and row do not contain the two new
metrics.

- [ ] **Step 3: Add omission ranges to parsed pack state**

Extend the internal state with:

```go
type parsedContextPack struct {
	contextID       string
	duplicateOf     string
	sourceCoverage  string
	sourceRanges    []sourceRange
	omissionRanges  []sourceRange
}

type metrics struct {
	// existing counters remain
	boundedOmissionReadCalls  int
	unauthorizedSourceReads  int
	authorizedOmissionRanges []sourceRange
}
```

Parse only omissions with non-empty project/path and positive ordered line
bounds. Prefix the project exactly as source sections are prefixed. Parse
Markdown omission bullets through the existing backtick range parser.

- [ ] **Step 4: Classify each terminal command atomically**

Make the command classifier collect source targets before incrementing counters.
A command is bounded only when it has at least one direct read target, every
target is contained by an authorized omission, no target overlaps an included
section, and no segment is `rg`, `grep`, or `find`. Source searches and
inventories count as unauthorized source access. Increment each new counter at
most once per completed terminal item.

- [ ] **Step 5: Run analyzer tests and verify green**

Run:

```bash
gofmt -w scripts/analyze-agent-context-log.go
bash scripts/analyze-agent-context-log_test.sh
```

Expected: PASS with the original total counters unchanged for existing fixtures
and correct new counters for every boundedness fixture.

### Task 2: Carry Metrics Through the Regression Runner

**Files:**
- Modify: `internal/agentbench/runner.go`
- Modify: `internal/agentbench/runner_test.go`
- Modify: `internal/agentbench/review.go`
- Modify: `internal/agentbench/review_test.go`
- Modify: `scripts/agent-context-regression/main_test.go`

**Interfaces:**
- Consumes: the analyzer's eleven metric fields.
- Produces: `RunMetrics.BoundedOmissionReads` and `RunMetrics.UnauthorizedSourceReads`, per-run TSV, summary TSV, retained review JSON, and gate failures.

- [ ] **Step 1: Write failing runner and gate tests**

Add strict assertions for:

```go
type RunMetrics struct {
	BoundedOmissionReads    int64 `json:"bounded_omission_read_calls"`
	UnauthorizedSourceReads int64 `json:"unauthorized_source_read_calls"`
}
```

Cover an eleven-field analyzer row, a malformed old-width row, negative metric
validation, candidate unauthorized median regression, and a candidate bounded
count above `contract.Limits.MaxSourceOmissions`.

- [ ] **Step 2: Run focused tests and verify red**

Run:

```bash
go test ./internal/agentbench ./scripts/agent-context-regression -count=1
```

Expected: FAIL on missing fields, TSV columns, and gate rules.

- [ ] **Step 3: Thread both metrics through runner output**

Add the fields to `transcriptMetrics`, `RunMetrics`, analyzer parsing, run metric
files, summary rows, retained infrastructure review templates, and JSON metric
validation. Preserve `SourceReads` without deriving it from either new field.

- [ ] **Step 4: Replace only the monotonic source gate**

In `appendEfficiencyFailures`, compare median unauthorized reads:

```go
if candidateUnauthorized > goldenUnauthorized {
	*failures = append(*failures, fmt.Sprintf(
		"candidate median unauthorized source reads %d exceeds golden median %d",
		candidateUnauthorized,
		goldenUnauthorized,
	))
}
```

Reject each candidate run whose bounded omission reads exceed
`limits.MaxSourceOmissions`. Do not exempt bounded reads from token or tool-call
comparisons.

- [ ] **Step 5: Run focused tests and verify green**

Run:

```bash
gofmt -w internal/agentbench/runner.go internal/agentbench/runner_test.go internal/agentbench/review.go internal/agentbench/review_test.go scripts/agent-context-regression/main_test.go
go test ./internal/agentbench ./scripts/agent-context-regression -count=1
```

Expected: PASS.

### Task 3: Keep the Release Benchmark Gate Unchanged

**Files:**
- Modify: `scripts/benchmark-agent-context-regression_test.sh`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/RELEASE.md`

**Interfaces:**
- Consumes: the extended regression analyzer schema.
- Produces: documented regression metric semantics while the standard release harness still compares total `source_read_calls`.

- [ ] **Step 1: Add a failing shell regression assertion**

Assert that regression summaries include total, bounded, and unauthorized source
reads, and that `scripts/benchmark-agent-context.sh` still reads and compares
the total `source_read_calls` field.

- [ ] **Step 2: Run shell harness tests and verify red**

Run:

```bash
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
```

Expected: the regression test fails on the old schema; the standard benchmark
test continues to pass.

- [ ] **Step 3: Update current metric documentation**

Document all three counters, event ordering, atomic compound-command behavior,
the omission cap, and the unchanged release comparison against total reads.
Replace “nine-line assisted instruction” with “eleven-line assisted
instruction” wherever it describes the current protocol.

- [ ] **Step 4: Run the complete hypothesis verification**

Run:

```bash
gofmt -w scripts/analyze-agent-context-log.go internal/agentbench/*.go scripts/agent-context-regression/*.go
go vet ./...
go test ./... -count=1
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
git diff --check
```

Expected: all checks pass.

- [ ] **Step 5: Commit the hypothesis**

```bash
git add scripts/analyze-agent-context-log.go scripts/analyze-agent-context-log_test.sh internal/agentbench scripts/agent-context-regression docs/BENCHMARKING.md docs/RELEASE.md
git commit -m "Classify bounded Context omission reads" -m "- Separate exact omission reads from unauthorized source access in event order.
- Carry both metrics through regression artifacts and monotonic gates.
- Preserve total source-read accounting for release benchmarks."
```
