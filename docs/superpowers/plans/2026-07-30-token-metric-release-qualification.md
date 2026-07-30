# Token Metric and Release Qualification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Define a prospective, reproducible Codex token metric and use it with the existing structural and quality gates to qualify the stabilized GoreGraph candidate for release.

**Architecture:** Parse every current `turn.completed.usage` component once into a shared strict value type, preserve raw totals for diagnostics, and gate on uncached input plus output tokens. Migrate the transcript analyzer, monotonic runner, and matched release harness to explicit metric columns before any candidate release matrix is run.

**Tech Stack:** Go 1.23, Go standard library, Bash with `set -euo pipefail`, TSV benchmark artifacts, existing agent benchmark runner and documentation synchronizer

## Global Constraints

- This plan starts only after the rendered-evidence stabilization plan passes its complete local acceptance and one authorized private G1 smoke run.
- `effective_tokens = input_tokens - cached_input_tokens + output_tokens`.
- `total_tokens = input_tokens + output_tokens`.
- `reasoning_output_tokens` is recorded separately and remains a subset of `output_tokens`; it is never added again.
- Matched assisted effective-token median must be at most 80% of the matched baseline effective-token median.
- Assisted effective-token median must be at most 116,560.
- Assisted tool-call median must be at most 70% of baseline.
- Assisted total source-read median must be at most 50% of baseline.
- Maximum Context Pack size remains 4,000 estimated tokens, 12 files, and 12 source sections.
- The completed 2026-07-30 release benchmark remains immutable and failed; new metric semantics never rescore it.
- Thresholds and metric names are committed before the final candidate matrix.
- No external private-workspace run starts without active authorization.
- No release, tag, publication workflow, package-manager update, or push is part of this plan.

---

## File Structure

- Create `internal/agentmetrics/token_usage.go`: strict parsing, validation, derived values, TSV header, and TSV row parsing for Codex usage.
- Create `internal/agentmetrics/token_usage_test.go`: table-driven usage semantics and invalid-input tests.
- Modify `internal/agentmetrics/schema.go`: explicit release and regression summary columns.
- Modify `scripts/analyze-agent-context-log.go`: additive `--usage` output while retaining legacy `--tokens`.
- Modify `scripts/analyze-agent-context-log_test.sh`: real current usage shapes, cache arithmetic, and invalid-usage coverage.
- Modify `internal/agentbench/runner.go`: consume one strict usage row and persist all token components.
- Modify `internal/agentbench/review.go`: validate token consistency and compare `effective_tokens`.
- Modify `internal/agentbench/contract.go`: freeze token metric name in monotonic contracts.
- Modify `internal/agentbench/*_test.go`, `scripts/agent-context-regression/main_test.go`, and `testdata/agent-context-regression/*/contract.json`: migrate strict fixtures and expectations.
- Modify `scripts/benchmark-agent-context.sh` and `scripts/benchmark-agent-context_test.sh`: gate the matched release benchmark on explicit effective tokens.
- Create `scripts/calibrate-agent-context-tokens.sh` and `scripts/calibrate-agent-context-tokens_test.sh`: deterministic control report for six retained transcripts.
- Modify `scripts/sync-docs/main.go`, `scripts/sync-docs/main_test.go`, `README.md`, `docs/BENCHMARKING.md`, and `docs/RELEASE.md`: publish the exact prospective metric and truthful release status.

### Task 1: Strict shared token-usage model

**Files:**

- Create: `internal/agentmetrics/token_usage.go`
- Create: `internal/agentmetrics/token_usage_test.go`

**Interfaces:**

- Produces: `type TokenUsage struct`.
- Produces: `ParseTokenUsage(raw []byte) (TokenUsage, error)`.
- Produces: `ParseTokenUsageRow(row string) (TokenUsage, error)`.
- Produces: `func (TokenUsage) TSV() string`.
- Produces constant `TokenUsageHeader`.
- Later analyzer and runner tasks must use these interfaces without duplicating arithmetic.

- [ ] **Step 1: Write failing parser tests with real usage shapes**

Create `internal/agentmetrics/token_usage_test.go`:

```go
package agentmetrics

import (
	"strings"
	"testing"
)

func TestParseTokenUsageDerivesProspectiveMetrics(t *testing.T) {
	raw := []byte(`{
		"input_tokens": 182151,
		"cached_input_tokens": 146944,
		"output_tokens": 6160,
		"reasoning_output_tokens": 3195
	}`)
	got, err := ParseTokenUsage(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := TokenUsage{
		InputTokens:           182151,
		CachedInputTokens:     146944,
		UncachedInputTokens:   35207,
		OutputTokens:          6160,
		ReasoningOutputTokens: 3195,
		TotalTokens:           188311,
		EffectiveTokens:       41367,
	}
	if got != want {
		t.Fatalf("usage = %#v, want %#v", got, want)
	}
	if parsed, err := ParseTokenUsageRow(got.TSV()); err != nil || parsed != want {
		t.Fatalf("TSV round trip = %#v, %v", parsed, err)
	}
}

func TestParseTokenUsageRejectsInvalidCounters(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "missing input", raw: `{"output_tokens":1}`, want: "input_tokens is required"},
		{name: "missing output", raw: `{"input_tokens":1}`, want: "output_tokens is required"},
		{name: "negative cache", raw: `{"input_tokens":1,"cached_input_tokens":-1,"output_tokens":1}`, want: "cached_input_tokens must not be negative"},
		{name: "cache exceeds input", raw: `{"input_tokens":1,"cached_input_tokens":2,"output_tokens":1}`, want: "cached_input_tokens exceeds input_tokens"},
		{name: "reasoning exceeds output", raw: `{"input_tokens":1,"output_tokens":1,"reasoning_output_tokens":2}`, want: "reasoning_output_tokens exceeds output_tokens"},
		{name: "empty total", raw: `{"input_tokens":0,"output_tokens":0}`, want: "token usage is empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseTokenUsage([]byte(test.raw))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests and verify the new API is absent**

```bash
go test ./internal/agentmetrics -count=1
```

Expected: FAIL because `TokenUsage` and its parser are undefined.

- [ ] **Step 3: Implement strict parsing and arithmetic**

Create `internal/agentmetrics/token_usage.go`:

```go
package agentmetrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const TokenUsageHeader = "input_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\teffective_tokens"

type TokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	UncachedInputTokens   int64 `json:"uncached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
	EffectiveTokens       int64 `json:"effective_tokens"`
}

func ParseTokenUsage(raw []byte) (TokenUsage, error) {
	var values map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &values) != nil {
		return TokenUsage{}, errors.New("usage is missing or invalid")
	}
	input, err := requiredTokenField(values, "input_tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	output, err := requiredTokenField(values, "output_tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	cached, err := optionalTokenField(values, "cached_input_tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	reasoning, err := optionalTokenField(values, "reasoning_output_tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	if cached > input {
		return TokenUsage{}, errors.New("cached_input_tokens exceeds input_tokens")
	}
	if reasoning > output {
		return TokenUsage{}, errors.New("reasoning_output_tokens exceeds output_tokens")
	}
	if input > math.MaxInt64-output {
		return TokenUsage{}, errors.New("input_tokens plus output_tokens overflows int64")
	}
	total := input + output
	if total == 0 {
		return TokenUsage{}, errors.New("token usage is empty")
	}
	uncached := input - cached
	return TokenUsage{
		InputTokens:           input,
		CachedInputTokens:     cached,
		UncachedInputTokens:   uncached,
		OutputTokens:          output,
		ReasoningOutputTokens: reasoning,
		TotalTokens:           total,
		EffectiveTokens:       uncached + output,
	}, nil
}

func (usage TokenUsage) TSV() string {
	return fmt.Sprintf(
		"%d\t%d\t%d\t%d\t%d\t%d\t%d",
		usage.InputTokens,
		usage.CachedInputTokens,
		usage.UncachedInputTokens,
		usage.OutputTokens,
		usage.ReasoningOutputTokens,
		usage.TotalTokens,
		usage.EffectiveTokens,
	)
}

func ParseTokenUsageRow(row string) (TokenUsage, error) {
	fields := strings.Split(strings.TrimSpace(row), "\t")
	if len(fields) != 7 {
		return TokenUsage{}, fmt.Errorf("token usage row has %d fields, want 7", len(fields))
	}
	values := make([]int64, len(fields))
	for index, field := range fields {
		value, err := strconv.ParseInt(field, 10, 64)
		if err != nil || value < 0 {
			return TokenUsage{}, fmt.Errorf("token usage field %d is invalid", index+1)
		}
		values[index] = value
	}
	usage := TokenUsage{
		InputTokens:           values[0],
		CachedInputTokens:     values[1],
		UncachedInputTokens:   values[2],
		OutputTokens:          values[3],
		ReasoningOutputTokens: values[4],
		TotalTokens:           values[5],
		EffectiveTokens:       values[6],
	}
	if usage.CachedInputTokens > usage.InputTokens ||
		usage.UncachedInputTokens != usage.InputTokens-usage.CachedInputTokens ||
		usage.ReasoningOutputTokens > usage.OutputTokens ||
		usage.TotalTokens != usage.InputTokens+usage.OutputTokens ||
		usage.EffectiveTokens != usage.UncachedInputTokens+usage.OutputTokens ||
		usage.TotalTokens == 0 {
		return TokenUsage{}, errors.New("token usage row is internally inconsistent")
	}
	return usage, nil
}

func requiredTokenField(
	values map[string]json.RawMessage,
	name string,
) (int64, error) {
	raw, ok := values[name]
	if !ok {
		return 0, fmt.Errorf("%s is required", name)
	}
	return decodeTokenField(raw, name)
}

func optionalTokenField(
	values map[string]json.RawMessage,
	name string,
) (int64, error) {
	raw, ok := values[name]
	if !ok {
		return 0, nil
	}
	return decodeTokenField(raw, name)
}

func decodeTokenField(raw json.RawMessage, name string) (int64, error) {
	var value int64
	if json.Unmarshal(raw, &value) != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s must not be negative", name)
	}
	return value, nil
}
```

- [ ] **Step 4: Add overflow and TSV consistency cases**

Extend the table with:

```go
{
	name: "overflow",
	raw:  `{"input_tokens":9223372036854775807,"output_tokens":1}`,
	want: "overflows int64",
}
```

Add this exact TSV test:

```go
func TestParseTokenUsageRowRejectsInvalidRows(t *testing.T) {
	for _, row := range []string{
		"1\t2",
		"10\t5\t4\t1\t0\t11\t6",
		"10\t5\t5\t1\t2\t11\t6",
	} {
		if _, err := ParseTokenUsageRow(row); err == nil {
			t.Errorf("invalid usage row passed: %q", row)
		}
	}
}
```

- [ ] **Step 5: Format, test, and commit**

```bash
gofmt -w internal/agentmetrics/token_usage.go internal/agentmetrics/token_usage_test.go
go test ./internal/agentmetrics -count=1
git add internal/agentmetrics/token_usage.go internal/agentmetrics/token_usage_test.go
git commit -m "Define prospective Codex token usage" -m "- Parse current input, cached input, output, and reasoning counters strictly
- Derive total and uncached-plus-output metrics without double-counting
- Provide validated TSV serialization for benchmark consumers"
```

### Task 2: Add explicit usage output to the transcript analyzer

**Files:**

- Modify: `scripts/analyze-agent-context-log.go`
- Modify: `scripts/analyze-agent-context-log_test.sh`

**Interfaces:**

- Adds analyzer mode `--usage`.
- Preserves `--tokens` as a legacy diagnostic that follows the existing
  `total_tokens` preference.
- `--usage` requires current input/output counters and prints
  `agentmetrics.TokenUsage.TSV()`.

- [ ] **Step 1: Add failing shell assertions for explicit usage**

In `scripts/analyze-agent-context-log_test.sh`, change the main fixture usage
to:

```json
{"type":"turn.completed","usage":{"input_tokens":182151,"cached_input_tokens":146944,"output_tokens":6160,"reasoning_output_tokens":3195}}
```

Add:

```bash
usage=$(bash "$analyzer" --usage "$temporary_directory/transcript.jsonl")
[ "$usage" = $'182151\t146944\t35207\t6160\t3195\t188311\t41367' ] ||
  fail "usage row = $usage"
```

Add:

```bash
cat >"$temporary_directory/invalid-usage.jsonl" <<'EOF'
{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":11,"output_tokens":1,"reasoning_output_tokens":0}}
EOF
if bash "$analyzer" --usage "$temporary_directory/invalid-usage.jsonl" \
  >"$temporary_directory/invalid-usage.stdout" \
  2>"$temporary_directory/invalid-usage.stderr"; then
  fail "usage with cached input greater than input passed"
fi
grep -q 'cached_input_tokens exceeds input_tokens' \
  "$temporary_directory/invalid-usage.stderr" ||
  fail "invalid usage error was not specific"
```

- [ ] **Step 2: Run the analyzer test and confirm `--usage` is rejected**

```bash
bash scripts/analyze-agent-context-log_test.sh
```

Expected: FAIL because the analyzer accepts only `--header` and `--tokens`.

- [ ] **Step 3: Parse strict and legacy token views independently**

Modify `analysis`:

```go
type analysis struct {
	metrics      metrics
	legacyTokens int64
	usage        agentmetrics.TokenUsage
}
```

For every `turn.completed` event:

```go
legacyTokens, err := legacyTokenUsage(outer.Usage)
if err != nil {
	return analysis{}, fmt.Errorf("turn.completed at line %d: %w", lineNumber, err)
}
usage, err := agentmetrics.ParseTokenUsage(outer.Usage)
if err != nil {
	return analysis{}, fmt.Errorf("turn.completed at line %d: %w", lineNumber, err)
}
result.legacyTokens = legacyTokens
result.usage = usage
seenUsage = true
```

Rename the current `tokenUsage` function to `legacyTokenUsage`; do not alter
its behavior.

- [ ] **Step 4: Add `--usage` argument and output**

Accept `--usage` beside the existing modes:

```go
if len(args) > 0 &&
	(args[0] == "--header" || args[0] == "--tokens" || args[0] == "--usage") {
	mode = strings.TrimPrefix(args[0], "--")
	args = args[1:]
}
```

Update usage text to:

```text
usage: analyze-agent-context-log.go [--header|--tokens|--usage] /absolute/path/to/transcript.jsonl
```

In `main`:

```go
switch mode {
case "tokens":
	fmt.Println(result.legacyTokens)
	return
case "usage":
	fmt.Println(result.usage.TSV())
	return
}
```

- [ ] **Step 5: Prove no navigation metric changed**

Keep every existing expected navigation row byte-for-byte unchanged. Run:

```bash
gofmt -w scripts/analyze-agent-context-log.go
bash scripts/analyze-agent-context-log_test.sh
```

Expected: PASS.

- [ ] **Step 6: Commit additive analyzer usage**

```bash
git add scripts/analyze-agent-context-log.go scripts/analyze-agent-context-log_test.sh
git commit -m "Expose explicit transcript token usage" -m "- Add strict current-counter usage output to the transcript analyzer
- Preserve the legacy token-total mode for retained diagnostics
- Reject inconsistent cache and reasoning counters"
```

### Task 3: Migrate monotonic runner and gate contracts

**Files:**

- Modify: `internal/agentmetrics/schema.go`
- Modify: `internal/agentbench/runner.go`
- Modify: `internal/agentbench/review.go`
- Modify: `internal/agentbench/contract.go`
- Modify: `internal/agentbench/runner_test.go`
- Modify: `internal/agentbench/review_test.go`
- Modify: `internal/agentbench/contract_test.go`
- Modify: `internal/agentbench/matrix_test.go`
- Modify: `scripts/agent-context-regression/main_test.go`
- Modify: `testdata/agent-context-regression/g2-java-missing-contract/contract.json`
- Modify: `testdata/agent-context-regression/g3-go-existing-flow/contract.json`
- Modify: `testdata/agent-context-regression/g4-typescript-consumer-provider/contract.json`
- Modify: `testdata/agent-context-regression/g5-java-persistence-side-effects/contract.json`
- Modify: `testdata/agent-context-regression/g6-ambiguous-entrypoint/contract.json`

**Interfaces:**

- Adds `EfficiencyLimits.TokenMetric string` fixed to
  `uncached_input_plus_output`.
- Adds `RunMetrics.TokenUsage agentmetrics.TokenUsage`.
- Keeps `RunMetrics.Tokens` as the raw `input_tokens + output_tokens`
  diagnostic for JSON compatibility.
- Comparative token gates use `RunMetrics.TokenUsage.EffectiveTokens`.

- [ ] **Step 1: Write failing contract and review tests**

In `contract_test.go`, add:

```go
func TestValidateContractRequiresProspectiveTokenMetric(t *testing.T) {
	contract := validContract()
	contract.Limits.TokenMetric = ""
	if err := ValidateContract(contract); err == nil ||
		!strings.Contains(err.Error(), "token_metric") {
		t.Fatalf("missing token metric error = %v", err)
	}
	contract.Limits.TokenMetric = "total_tokens"
	if err := ValidateContract(contract); err == nil ||
		!strings.Contains(err.Error(), "uncached_input_plus_output") {
		t.Fatalf("wrong token metric error = %v", err)
	}
}
```

In `review_test.go`, add:

```go
func TestEvaluateCaseUsesEffectiveTokens(t *testing.T) {
	contract, golden, candidate, hypothesis, diff := passingCase()
	setUsage := func(
		runs []ReviewedRun,
		input, cached, output int64,
	) {
		for index := range runs {
			usage := agentmetrics.TokenUsage{
				InputTokens:           input,
				CachedInputTokens:     cached,
				UncachedInputTokens:   input - cached,
				OutputTokens:          output,
				ReasoningOutputTokens: 0,
				TotalTokens:           input + output,
				EffectiveTokens:       input - cached + output,
			}
			runs[index].Metrics.Tokens = usage.TotalTokens
			runs[index].Metrics.TokenUsage = usage
		}
	}
	setUsage(golden, 180, 100, 20)
	setUsage(candidate, 190, 110, 20)
	if report := EvaluateCase(contract, golden, candidate, hypothesis, diff); !report.Passed {
		t.Fatalf("equal effective usage failed: %#v", report)
	}

	setUsage(candidate, 190, 110, 26)
	requireGateFailure(
		t,
		EvaluateCase(contract, golden, candidate, hypothesis, diff),
		"candidate median effective tokens 106 exceeds 105 percent of golden median 100",
	)
}
```

Add `github.com/gorecodecom/goregraph/internal/agentmetrics` to the test
imports.

- [ ] **Step 2: Run focused tests and confirm they fail**

```bash
go test ./internal/agentbench -run 'TestValidateContractRequiresProspectiveTokenMetric|TestEvaluateCase.*Token' -count=1
```

Expected: FAIL because contracts and metrics have no explicit token metric.

- [ ] **Step 3: Extend contracts and reviewed metrics**

Add:

```go
const fixedTokenMetric = "uncached_input_plus_output"
```

to `contract.go`, add:

```go
TokenMetric string `json:"token_metric"`
```

to `EfficiencyLimits`, and validate:

```go
if limits.TokenMetric != fixedTokenMetric {
	return fmt.Errorf(
		"limits.token_metric must be %q",
		fixedTokenMetric,
	)
}
```

In `review.go`:

```go
type RunMetrics struct {
	Tokens     int64                   `json:"tokens"`
	TokenUsage agentmetrics.TokenUsage `json:"token_usage"`
	// existing non-token fields remain unchanged
}
```

Set `Tokens` to `TokenUsage.TotalTokens` for newly generated reviews. Extend
`validateMetrics` to call `agentmetrics.ParseTokenUsageRow` on
`metrics.TokenUsage.TSV()` and reject `metrics.Tokens !=
metrics.TokenUsage.TotalTokens`.

- [ ] **Step 4: Migrate runner transcript metrics**

Change:

```go
type transcriptMetrics struct {
	usage agentmetrics.TokenUsage
	// existing fields
}
```

In `analyzeTranscript`, replace the `--tokens` subprocess with `--usage` and:

```go
usage, err := agentmetrics.ParseTokenUsageRow(strings.TrimSpace(string(usageOutput)))
if err != nil {
	return transcriptMetrics{}, nil, fmt.Errorf("parse transcript usage: %w", err)
}
```

Carry `usage` through failure artifacts, `.metrics.tsv`, review templates, and
`summary.tsv`.

- [ ] **Step 5: Make summary columns explicit**

Set the regression header to:

```go
const RegressionSummaryHeader = "case\tquery\tbuild\trun\tattempt\teffective_tokens\tinput_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\ttool_calls\tcontext_calls\trepeated_full_packs\tbroad_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tcontext_millis\tlog"
```

Update `writeRunMetrics` and `appendSummary` in that exact order.

- [ ] **Step 6: Gate on effective tokens**

Change `metricsByRun` to:

```go
type metricsByRun struct {
	effectiveTokens        []int64
	toolCalls              []int64
	unauthorizedSourceReads []int64
	contextMillis          []int64
}
```

Append `runs[run].Metrics.TokenUsage.EffectiveTokens` and update the finding
text to say `effective tokens`.

- [ ] **Step 7: Update strict fixtures**

Change the fake Codex usage in `runner_test.go` to:

```json
{"type":"turn.completed","usage":{"input_tokens":90,"cached_input_tokens":20,"output_tokens":10,"reasoning_output_tokens":4}}
```

Make the fake analyzer handle `--usage`:

```bash
if [ "${1:-}" = "--usage" ]; then
  printf '90\t20\t70\t10\t4\t100\t80\n'
  exit 0
fi
```

Add `"token_metric": "uncached_input_plus_output"` to every committed
regression contract and every in-memory valid contract fixture.

- [ ] **Step 8: Format and run the monotonic suite**

```bash
gofmt -w internal/agentmetrics/schema.go internal/agentbench scripts/agent-context-regression/main_test.go
go test ./internal/agentbench ./scripts/agent-context-regression -count=1
bash scripts/benchmark-agent-context-regression_test.sh
```

Expected: PASS.

- [ ] **Step 9: Commit runner migration**

```bash
git add internal/agentmetrics/schema.go internal/agentbench scripts/agent-context-regression testdata/agent-context-regression
git commit -m "Gate agent regressions on effective tokens" -m "- Persist every raw and derived Codex usage counter in runner artifacts
- Freeze monotonic contracts to uncached input plus output tokens
- Keep raw total tokens as an explicit diagnostic rather than the gate value"
```

### Task 4: Migrate the matched release harness

**Files:**

- Modify: `scripts/benchmark-agent-context.sh`
- Modify: `scripts/benchmark-agent-context_test.sh`
- Modify: `internal/agentmetrics/schema.go`

**Interfaces:**

- Release summary uses the same seven token columns as the regression summary.
- Release gates compare `effective_tokens`.
- The absolute prospective cap remains exactly 116,560.

- [ ] **Step 1: Write a cache-sensitive release harness test**

Change the fake baseline event in `benchmark-agent-context_test.sh` to emit:

```bash
printf '{"type":"turn.completed","usage":{"input_tokens":190000,"cached_input_tokens":90000,"output_tokens":10000,"reasoning_output_tokens":4000}}\n'
```

Change the assisted event to:

```bash
printf '{"type":"turn.completed","usage":{"input_tokens":170000,"cached_input_tokens":110000,"output_tokens":10000,"reasoning_output_tokens":4000}}\n'
```

The raw totals are close, but effective totals are `110000` baseline and
`70000` assisted. Assert the run passes both token gates.

- [ ] **Step 2: Run the release harness test and confirm old totals fail**

```bash
bash scripts/benchmark-agent-context_test.sh
```

Expected: FAIL because the harness still extracts legacy aggregate tokens.

- [ ] **Step 3: Parse one explicit usage row per run**

Replace `extract_tokens` with:

```bash
extract_usage() {
  go run "$analyzer_go" --usage "$1"
}
```

In `run_variant`:

```bash
usage=$(extract_usage "$log_path")
IFS=$'\t' read -r input_tokens cached_input_tokens uncached_input_tokens \
  output_tokens reasoning_output_tokens total_tokens effective_tokens <<EOF
$usage
EOF
[ -n "$effective_tokens" ] || die "cannot extract token usage from $log_path"
```

Write all seven fields to the summary and append only `effective_tokens` to the
median input file.

- [ ] **Step 4: Replace ambiguous summary headers**

Set:

```go
const ReleaseSummaryHeader = "variant\trun\teffective_tokens\tinput_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\ttool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tunique_source_files\tlog"
```

Use this exact order in `benchmark-agent-context.sh` and its expected-header
test.

- [ ] **Step 5: Keep both prospective gates strict**

Rename shell variables to `baseline_effective_median` and
`assisted_effective_median`. Keep:

```bash
[ $((assisted_effective_median * 5)) -le $((baseline_effective_median * 4)) ] ||
  die "assisted effective-token median exceeds 80% of matched baseline"
[ "$assisted_effective_median" -le 116560 ] ||
  die "assisted effective-token median exceeds prospective cap of 116560"
```

Add this case after the relative 80% gate test. The doubled baseline ensures
only the absolute cap is responsible for the failure:

```bash
FAKE_BASELINE_TOKENS=200000
FAKE_ASSISTED_TOKENS=116561
export FAKE_BASELINE_TOKENS FAKE_ASSISTED_TOKENS
if run_harness over-absolute-cap >/dev/null 2>&1; then
  fail "116561 assisted effective tokens passed the absolute cap"
fi
grep -q $'^assisted\tmedian\t116561\t' \
  "$temporary_directory/over-absolute-cap/summary.tsv" ||
  fail "absolute-cap failure did not retain assisted effective-token evidence"
unset FAKE_BASELINE_TOKENS FAKE_ASSISTED_TOKENS
```

- [ ] **Step 6: Run both shell harness suites**

```bash
gofmt -w internal/agentmetrics/schema.go
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
```

Expected: PASS.

- [ ] **Step 7: Commit the release harness migration**

```bash
git add internal/agentmetrics/schema.go scripts/benchmark-agent-context.sh scripts/benchmark-agent-context_test.sh
git commit -m "Gate release benchmarks on effective tokens" -m "- Record raw, cached, uncached, output, reasoning, total, and effective usage
- Compare matched medians using uncached input plus output
- Retain the prospective 116560-token cap and structural gates"
```

### Task 5: Deterministic retained-log calibration report

**Files:**

- Create: `scripts/calibrate-agent-context-tokens.sh`
- Create: `scripts/calibrate-agent-context-tokens_test.sh`

**Interfaces:**

- Command: `scripts/calibrate-agent-context-tokens.sh /absolute/evidence/directory`.
- Requires exactly `baseline-1.log` through `baseline-3.log` and
  `assisted-1.log` through `assisted-3.log`.
- Writes no file; emits a deterministic TSV report to stdout for storage
  outside the repository.

- [ ] **Step 1: Write the calibration script test**

The test creates six minimal transcripts with one terminal tool and one current
usage event, then runs the script. Assert the header:

```text
variant	run	input_tokens	cached_input_tokens	uncached_input_tokens	output_tokens	reasoning_output_tokens	total_tokens	effective_tokens	log
```

Assert footer rows:

```text
baseline	median	-	-	-	-	-	-	110000	-
assisted	median	-	-	-	-	-	-	70000	-
gate	matched_ratio_percent	-	-	-	-	-	-	63	-
gate	absolute_cap	-	-	-	-	-	-	116560	-
```

Add missing-file and malformed-usage cases that must fail.

- [ ] **Step 2: Run the test and verify the command is absent**

```bash
bash scripts/calibrate-agent-context-tokens_test.sh
```

Expected: FAIL because the calibration script does not exist.

- [ ] **Step 3: Implement exact six-log calibration**

Create `scripts/calibrate-agent-context-tokens.sh` with:

```bash
#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
analyzer="$script_dir/analyze-agent-context-log.sh"

[ "$#" -eq 1 ] || {
  printf 'usage: %s /absolute/evidence/directory\n' "$0" >&2
  exit 2
}
evidence=$1
[ "${evidence#/}" != "$evidence" ] || {
  printf 'error: evidence directory must be absolute\n' >&2
  exit 2
}
[ -d "$evidence" ] || {
  printf 'error: evidence directory is missing: %s\n' "$evidence" >&2
  exit 2
}

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/goregraph-token-calibration.XXXXXX")
cleanup() {
  status=$?
  trap - EXIT
  rm -rf -- "$temporary_directory"
  exit "$status"
}
trap cleanup EXIT

printf 'variant\trun\tinput_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\teffective_tokens\tlog\n'
for variant in baseline assisted; do
  values="$temporary_directory/$variant.effective"
  : >"$values"
  for run in 1 2 3; do
    log="$evidence/$variant-$run.log"
    [ -f "$log" ] && [ -r "$log" ] || {
      printf 'error: required transcript is missing: %s\n' "$log" >&2
      exit 2
    }
    usage=$(bash "$analyzer" --usage "$log")
    IFS=$'\t' read -r input cached uncached output reasoning total effective <<EOF
$usage
EOF
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$variant" "$run" "$input" "$cached" "$uncached" "$output" \
      "$reasoning" "$total" "$effective" "$log"
    printf '%s\n' "$effective" >>"$values"
  done
done

median() {
  sort -n "$1" | sed -n '2p'
}
baseline_median=$(median "$temporary_directory/baseline.effective")
assisted_median=$(median "$temporary_directory/assisted.effective")
ratio=$((assisted_median * 100 / baseline_median))
printf 'baseline\tmedian\t-\t-\t-\t-\t-\t-\t%s\t-\n' "$baseline_median"
printf 'assisted\tmedian\t-\t-\t-\t-\t-\t-\t%s\t-\n' "$assisted_median"
printf 'gate\tmatched_ratio_percent\t-\t-\t-\t-\t-\t-\t%s\t-\n' "$ratio"
printf 'gate\tabsolute_cap\t-\t-\t-\t-\t-\t-\t116560\t-\n'
```

- [ ] **Step 4: Test calibration output determinism**

Run the same fixture twice, compare stdout byte-for-byte after replacing the
temporary root path in the test, and assert no file is created in the evidence
directory.

```bash
bash scripts/calibrate-agent-context-tokens_test.sh
```

Expected: PASS.

- [ ] **Step 5: Commit the calibration helper**

```bash
git add scripts/calibrate-agent-context-tokens.sh scripts/calibrate-agent-context-tokens_test.sh
git commit -m "Add deterministic token calibration report" -m "- Recalculate explicit usage for exactly three baseline and three assisted logs
- Report effective-token medians and frozen prospective gates
- Keep retained evidence and calibration output outside the repository"
```

### Task 6: Publish metric and release-status truth

**Files:**

- Modify: `scripts/sync-docs/main.go`
- Modify: `scripts/sync-docs/main_test.go`
- Modify: `README.md`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/RELEASE.md`

**Interfaces:**

- Generated metric blocks use `agentmetrics.ReleaseSummaryHeader` and
  `agentmetrics.RegressionSummaryHeader`.
- Documentation defines effective, total, cached, and reasoning semantics once.
- Release evidence status states that the retained 3×3 result failed and is not
  rescored.

- [ ] **Step 1: Write failing documentation-truth assertions**

In `TestGeneratedFactsDescribeImplementedRuntimeDepth`, require:

```go
for _, want := range []string{
	"effective_tokens",
	"cached_input_tokens",
	"reasoning_output_tokens",
	"uncached input plus output",
	"reasoning output is already part of output",
} {
	if !strings.Contains(metrics, want) {
		t.Fatalf("benchmark metrics are missing %q:\n%s", want, metrics)
	}
}
```

Require release evidence to contain:

```go
"latest controlled three-by-three release benchmark did not pass"
"remains failed"
"prospectively calibrated"
```

- [ ] **Step 2: Run sync-docs tests and confirm stale wording**

```bash
go test ./scripts/sync-docs -count=1
```

Expected: FAIL because generated documentation still exposes an ambiguous
`tokens` column and stale one-pair status.

- [ ] **Step 3: Update generated metric explanation**

Extend `renderAgentBenchmarkMetrics` with:

```text
`effective_tokens` is `input_tokens - cached_input_tokens + output_tokens` and is the prospective comparison metric. `total_tokens` is `input_tokens + output_tokens`. `reasoning_output_tokens` is recorded separately but is already part of `output_tokens`, so it is not added again.
```

Keep all existing source-read classifications.

- [ ] **Step 4: Correct the generated release evidence status**

Set `renderCurrentReleaseEvidenceStatus` to:

```go
return "The latest controlled three-by-three release benchmark did not pass: " +
	"assisted answer quality remained below baseline and its legacy absolute-token " +
	"gate used an aggregate that is not the prospectively frozen metric. " +
	"The retained result remains failed and is not rescored. Publication remains " +
	"blocked until a fresh prospectively calibrated three-by-three run passes " +
	"all gates and receives the required signed 12-point quality review."
```

- [ ] **Step 5: Update hand-written token-gate sections**

In `docs/BENCHMARKING.md`, `README.md`, and `docs/RELEASE.md` state exactly:

- the prospective formula;
- both raw and effective counters are retained;
- the 80% matched threshold uses effective tokens;
- the 116,560 absolute cap uses effective tokens;
- reasoning output is not double-counted;
- the previous 3×3 result remains failed;
- pack `estimated_tokens` remains unrelated to end-to-end usage.

Remove claims that the complete-session aggregate or recorded 145,700-token
baseline is directly comparable without naming its metric.

- [ ] **Step 6: Synchronize generated blocks**

```bash
gofmt -w scripts/sync-docs/main.go scripts/sync-docs/main_test.go
go run ./scripts/sync-docs --write
go run ./scripts/sync-docs --check
go test ./scripts/sync-docs -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit documentation truth**

```bash
git add scripts/sync-docs README.md docs/BENCHMARKING.md docs/RELEASE.md
git commit -m "Document prospective release token metrics" -m "- Define effective, raw, cached, output, and reasoning token semantics
- Correct the retained three-by-three failure status without rescoring it
- Synchronize release and benchmark documentation from shared metric headers"
```

### Task 7: Local verification, control calibration, and release-matrix handoff

**Files:**

- Verify only; no source changes are expected before benchmark evidence exists.

**Interfaces:**

- Consumes: the clean stabilized candidate commit, installed binary, six
  retained control transcripts, frozen prompt/configuration, and active
  external-run authorization.
- Produces: local green verification, an external calibration report, and a
  stop point for the exact final six-run authorization.

- [ ] **Step 1: Run the complete repository verification**

```bash
go test ./... -count=1
go vet ./...
bash scripts/analyze-agent-context-log_test.sh
bash scripts/calibrate-agent-context-tokens_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
go run ./scripts/sync-docs --check
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 2: Verify the metric with retained control evidence**

The executor sets `RETAINED_G1_EVIDENCE` to the absolute directory containing
the completed six transcripts and runs:

```bash
test -n "${RETAINED_G1_EVIDENCE:?set RETAINED_G1_EVIDENCE to the retained six-log directory}"
scripts/calibrate-agent-context-tokens.sh "$RETAINED_G1_EVIDENCE"
```

Store stdout outside the repository beside the retained evidence. For the
known 2026-07-30 logs, independently verify the raw arithmetic:

- baseline effective tokens: 208,258; 182,198; 186,783; median 186,783;
- assisted effective tokens: 41,367; 35,213; 39,505; median 39,505;
- reasoning counters remain included only in output;
- this diagnostic does not alter the failed result.

- [ ] **Step 3: Rebuild and install the exact candidate**

```bash
git status --short
git rev-parse HEAD
go install ./cmd/goregraph
goregraph version
```

Expected: clean worktree and installed candidate identity matching the recorded
commit.

- [ ] **Step 4: Require the product smoke result before final benchmarking**

Confirm the one-run private G1 smoke from the rendered-evidence plan:

- scored 12/12;
- introduced no forbidden or unsupported claim;
- performed no broad navigation or included-source reread;
- stayed within 4,000 tokens, 12 files, and 12 sections;
- did not increase source reads or tool calls beyond accepted assisted
  behavior.

If any condition fails, stop. Do not start a release matrix and do not adjust
token thresholds.

- [ ] **Step 5: Stop for exact final data-sharing authorization**

The earlier exact six-run authorization was consumed by the retained failed
matrix. Request a new authorization for exactly:

- three baseline runs without GoreGraph;
- three assisted runs with the exact installed candidate;
- the same authorized historical workspace;
- the same frozen prompt, model, reasoning, sandbox, approval, and execution
  settings;
- read-only transmission to the external Codex service.

Do not run fewer exploratory trials or extra replacement runs.

- [ ] **Step 6: Execute the frozen 3×3 matrix after authorization**

Use `scripts/benchmark-agent-context.sh` with the externally stored workspace,
prompt, and output paths. Preserve the interleaved order and store:

- six raw JSONL transcripts;
- six stderr files;
- explicit per-run usage metrics;
- `summary.tsv`;
- binary, commit, index, prompt, model, reasoning, Codex version, and argument
  identities;
- the calibration report;
- the unsigned 12-point review template.

- [ ] **Step 7: Apply release gates without reinterpretation**

The matrix passes only if:

- assisted effective-token median is at most 80% of baseline;
- assisted effective-token median is at most 116,560;
- assisted tool-call median is at most 70% of baseline;
- assisted source-read median is at most 50% of baseline;
- repeated full packs and included-source rereads are zero;
- all three assisted answers score 12/12;
- every baseline and assisted run is valid;
- the independent reviewer signs and dates the rubric.

Any failure remains part of the evidence set. Do not replace a valid slow or
inaccurate run.

- [ ] **Step 8: Stop for the separate release decision**

Report the exact gate table, signed-review status, candidate commit, CI status,
and remaining documentation delta. Do not tag, push a release, update package
managers, or change the generated release-status block until the user gives
explicit release direction.
