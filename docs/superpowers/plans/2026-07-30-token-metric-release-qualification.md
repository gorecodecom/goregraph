# Deterministic Release Benchmark Metrics and Skill Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make future GoreGraph release benchmarks use strict effective-token accounting and reject uncontrolled external skill reads without changing normal skill-enabled workflows.

**Architecture:** Parse every current `turn.completed.usage` component once into a shared strict value type, preserve raw totals for diagnostics, and gate on uncached input plus output tokens. Add a plugin-agnostic path classifier and ordered analyzer evidence for external skill reads, then make the controlled release harness record Codex configuration and stop after its first contaminated run. Migrate regression and release artifacts before any new candidate matrix is run.

**Tech Stack:** Go 1.23, Go standard library, Bash with `set -euo pipefail`, TSV benchmark artifacts, existing agent benchmark runner and documentation synchronizer

## Global Constraints

- The approved design is `docs/superpowers/specs/2026-07-31-release-benchmark-metric-and-skill-isolation-design.md`.
- `effective_tokens = input_tokens - cached_input_tokens + output_tokens`.
- `total_tokens = input_tokens + output_tokens`.
- `reasoning_output_tokens` is recorded separately and remains a subset of `output_tokens`; it is never added again.
- Matched assisted effective-token median must be at most 80% of the matched baseline effective-token median.
- Assisted effective-token median must be at most 116,560.
- Assisted tool-call median must be at most 70% of baseline.
- Assisted total source-read median must be at most 50% of baseline.
- Maximum Context Pack size remains 4,000 estimated tokens, 12 files, and 12 source sections.
- The completed 2026-07-30 release benchmark remains immutable and failed; new metric semantics never rescore it.
- The retained candidate matrix recorded raw total-token medians of 2,551,495 baseline and 147,212 assisted; its 116,560 absolute gate failed.
- Prospective offline effective-token medians of 164,295 baseline and 39,180 assisted are diagnostic only and do not alter that verdict.
- Controlled baseline and assisted release runs each require `external_skill_read_calls = 0` across the complete transcript.
- Skill detection is based on normalized command targets outside the benchmark workspace and never on plugin names, skill names, or prompt wording.
- Normal GoreGraph use remains compatible with task-scoped Brainstorming, TDD, debugging, and review skills.
- The harness records plugin inventory but never changes global Codex or plugin configuration.
- A contaminated run is retained and fails the matrix immediately; it is never removed, retried, or replaced automatically.
- macOS, Linux, and Windows-shaped paths must produce deterministic classification results.
- Language-support and analysis-depth claims remain derived from implemented scanners; this benchmark-only change must not broaden them.
- Thresholds and metric names are committed before the final candidate matrix.
- No external private-workspace run starts without active authorization.
- No release, tag, publication workflow, or package-manager update is part of this plan.
- Execute implementation in an isolated linked worktree for `fix/release-benchmark-metrics`; return the primary checkout to `main` before creating it.

---

## File Structure

- Create `internal/agentmetrics/token_usage.go`: strict parsing, validation, derived values, TSV header, and TSV row parsing for Codex usage.
- Create `internal/agentmetrics/token_usage_test.go`: table-driven usage semantics and invalid-input tests.
- Create `internal/agentmetrics/skill_path.go`: cross-platform external skill-target normalization and classification.
- Create `internal/agentmetrics/skill_path_test.go`: Unix-, Windows-, relative-, in-workspace-, and ambiguous-path cases.
- Modify `internal/agentmetrics/schema.go`: explicit release and regression token and contamination columns.
- Modify `scripts/analyze-agent-context-log.go`: additive `--usage`, `--workspace`, and `--skill-reads` output while retaining legacy `--tokens`.
- Modify `scripts/analyze-agent-context-log_test.sh`: current usage shapes, cache arithmetic, invalid usage, ordered skill evidence, and generic path coverage.
- Modify `internal/agentbench/runner.go`: consume one strict usage row and persist all token and contamination components.
- Modify `internal/agentbench/review.go`: validate token consistency and compare `effective_tokens`.
- Modify `internal/agentbench/contract.go`: freeze token metric name in monotonic contracts.
- Modify `internal/agentbench/*_test.go`, `scripts/agent-context-regression/main_test.go`, and `testdata/agent-context-regression/*/contract.json`: migrate strict fixtures and expectations.
- Modify `scripts/benchmark-agent-context.sh` and `scripts/benchmark-agent-context_test.sh`: gate the matched release benchmark on explicit effective tokens and zero external skill reads, record plugin inventory, and stop after the first contamination.
- Create `scripts/calibrate-agent-context-tokens.sh` and `scripts/calibrate-agent-context-tokens_test.sh`: deterministic control report for six retained transcripts.
- Modify `scripts/sync-docs/main.go`, `scripts/sync-docs/main_test.go`, `README.md`, `docs/BENCHMARKING.md`, `docs/RELEASE.md`, and `docs/OUTPUTS.md`: publish exact metric semantics, normal skill compatibility, controlled isolation requirements, and truthful release status.

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

### Task 2: Cross-platform external skill-target classification

**Files:**

- Create: `internal/agentmetrics/skill_path.go`
- Create: `internal/agentmetrics/skill_path_test.go`

**Interfaces:**

- Produces: `ClassifyExternalSkillTarget(workspace, commandDirectory, target string) (string, bool)`.
- Returns a normalized target only when the resolved target is outside the benchmark workspace and is either `SKILL.md`, a skill directory, or below a `skills` path component.
- Does not inspect plugin names, skill names, prompt prose, or command output.
- Treats Windows drive paths case-insensitively while preserving case-sensitive Unix path comparison.

- [ ] **Step 1: Write failing table-driven classifier tests**

Create `internal/agentmetrics/skill_path_test.go`:

```go
package agentmetrics

import "testing"

func TestClassifyExternalSkillTarget(t *testing.T) {
	tests := []struct {
		name             string
		workspace        string
		commandDirectory string
		target           string
		want             string
		matched          bool
	}{
		{
			name:             "macOS skill file",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/Users/me/.codex/skills/brainstorming/SKILL.md",
			want:             "/Users/me/.codex/skills/brainstorming/SKILL.md",
			matched:          true,
		},
		{
			name:             "Linux plugin reference",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/opt/codex/plugins/vendor/skills/tdd/references/guide.md",
			want:             "/opt/codex/plugins/vendor/skills/tdd/references/guide.md",
			matched:          true,
		},
		{
			name:             "relative target from external skill directory",
			workspace:        "/work/repo",
			commandDirectory: "/opt/codex/plugins/vendor/skills/review",
			target:           "references/checklist.md",
			want:             "/opt/codex/plugins/vendor/skills/review/references/checklist.md",
			matched:          true,
		},
		{
			name:             "Windows skill file",
			workspace:        `C:\\work\\repo`,
			commandDirectory: `C:\\work\\repo`,
			target:           `C:\\Users\\Me\\.codex\\skills\\TDD\\SKILL.md`,
			want:             "c:/users/me/.codex/skills/tdd/skill.md",
			matched:          true,
		},
		{
			name:             "workspace-local skill fixture",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/work/repo/testdata/skills/example/SKILL.md",
			matched:          false,
		},
		{
			name:             "ordinary external source",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/opt/source/Service.java",
			matched:          false,
		},
		{
			name:             "similar component is not skill directory",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/opt/skills-old/review/guide.md",
			matched:          false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, matched := ClassifyExternalSkillTarget(
				test.workspace,
				test.commandDirectory,
				test.target,
			)
			if got != test.want || matched != test.matched {
				t.Fatalf("classification = %q, %v; want %q, %v", got, matched, test.want, test.matched)
			}
		})
	}
}

func TestClassifyExternalSkillTargetRejectsAmbiguousRoots(t *testing.T) {
	for _, values := range [][3]string{
		{"work/repo", "/work/repo", "/opt/skills/tdd/SKILL.md"},
		{"/work/repo", "work/repo", "SKILL.md"},
		{"/work/repo", "/work/repo", ""},
	} {
		if got, ok := ClassifyExternalSkillTarget(values[0], values[1], values[2]); ok || got != "" {
			t.Fatalf("ambiguous classification = %q, %v for %#v", got, ok, values)
		}
	}
}
```

- [ ] **Step 2: Run the focused test and verify the API is absent**

```bash
go test ./internal/agentmetrics -run 'TestClassifyExternalSkillTarget' -count=1
```

Expected: FAIL because `ClassifyExternalSkillTarget` is undefined.

- [ ] **Step 3: Implement normalization and classification**

Create `internal/agentmetrics/skill_path.go`:

```go
package agentmetrics

import (
	pathpkg "path"
	"strings"
	"unicode"
)

func ClassifyExternalSkillTarget(
	workspace string,
	commandDirectory string,
	target string,
) (string, bool) {
	workspacePath, workspaceWindows, ok := normalizeAbsolutePath(workspace)
	if !ok {
		return "", false
	}
	commandPath, commandWindows, ok := normalizeAbsolutePath(commandDirectory)
	if !ok || commandWindows != workspaceWindows {
		return "", false
	}
	target = strings.TrimSpace(strings.ReplaceAll(target, `\`, "/"))
	if target == "" {
		return "", false
	}
	if !isAbsoluteComparablePath(target) {
		target = pathpkg.Join(commandPath, target)
	}
	targetPath, targetWindows, ok := normalizeAbsolutePath(target)
	if !ok || targetWindows != workspaceWindows || pathWithin(targetPath, workspacePath) {
		return "", false
	}
	if !isSkillBundlePath(targetPath) {
		return "", false
	}
	return targetPath, true
}

func normalizeAbsolutePath(value string) (string, bool, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	if !isAbsoluteComparablePath(value) {
		return "", false, false
	}
	windows := len(value) >= 3 && unicode.IsLetter(rune(value[0])) && value[1] == ':'
	value = pathpkg.Clean(value)
	if windows {
		value = strings.ToLower(value)
	}
	return value, windows, true
}

func isAbsoluteComparablePath(value string) bool {
	if strings.HasPrefix(value, "/") {
		return true
	}
	return len(value) >= 3 && unicode.IsLetter(rune(value[0])) && value[1] == ':' && value[2] == '/'
}

func pathWithin(candidate, root string) bool {
	return candidate == root || strings.HasPrefix(candidate, strings.TrimSuffix(root, "/")+"/")
}

func isSkillBundlePath(value string) bool {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	for _, part := range parts {
		if strings.EqualFold(part, "SKILL.md") || strings.EqualFold(part, "skills") {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Format, test, and commit the classifier**

```bash
gofmt -w internal/agentmetrics/skill_path.go internal/agentmetrics/skill_path_test.go
go test ./internal/agentmetrics -count=1
git add internal/agentmetrics/skill_path.go internal/agentmetrics/skill_path_test.go
git commit -m "Classify external skill read targets" -m "- Normalize Unix and Windows-shaped command targets deterministically
- Exclude workspace-local and ambiguous paths from contamination counts
- Detect skill bundles without coupling metrics to plugin or skill names"
```

### Task 3: Add usage and skill-read output to the transcript analyzer

**Files:**

- Modify: `scripts/analyze-agent-context-log.go`
- Modify: `scripts/analyze-agent-context-log_test.sh`

**Interfaces:**

- Adds analyzer mode `--usage`.
- Adds optional `--workspace /absolute/path` and JSON mode `--skill-reads`.
- Appends `external_skill_read_calls` to `agentmetrics.AnalyzerHeader` and the normal metrics row.
- `--skill-reads` emits an ordered JSON array with `event_order`, `item_id`, `command`, and normalized `target` fields.
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

Keep `included-rereads.jsonl` as a `total_tokens`-only fixture and add:

```bash
legacy_tokens=$(bash "$analyzer" --tokens "$temporary_directory/included-rereads.jsonl")
[ "$legacy_tokens" = "100" ] || fail "legacy tokens = $legacy_tokens"
```

This proves strict prospective parsing is additive and does not make retained
legacy diagnostics unreadable.

- [ ] **Step 2: Add failing generic skill-read evidence assertions**

Create the workspace and fixture with these terminal commands in this order:

```bash
mkdir -p "$temporary_directory/workspace/testdata/skills/example"
cat >"$temporary_directory/skill-reads.jsonl" <<EOF
{"type":"item.completed","item":{"id":"skill-one","type":"command_execution","command":"cat /Users/me/.codex/skills/brainstorming/SKILL.md","exit_code":0}}
{"type":"item.completed","item":{"id":"ordinary-external","type":"command_execution","command":"cat /opt/source/config.json","exit_code":0}}
{"type":"item.completed","item":{"id":"workspace-skill","type":"command_execution","command":"cat $temporary_directory/workspace/testdata/skills/example/SKILL.md","exit_code":0}}
{"type":"item.completed","item":{"id":"skill-two","type":"command_execution","command":"rg -n Rule /opt/codex/plugins/vendor/skills/tdd/references/guide.md","exit_code":0}}
{"type":"item.completed","item":{"id":"skill-two-targets","type":"command_execution","command":"cat /opt/codex/plugins/vendor/skills/review/SKILL.md /opt/codex/plugins/vendor/skills/review/references/checklist.md","exit_code":0}}
{"type":"turn.completed","usage":{"input_tokens":20,"cached_input_tokens":5,"output_tokens":5,"reasoning_output_tokens":1}}
EOF
```

Assert:

```bash
header=$(bash "$analyzer" --header "$temporary_directory/transcript.jsonl")
case "$header" in
  *$'\texternal_skill_read_calls') ;;
  *) fail "analyzer header lacks external_skill_read_calls: $header" ;;
esac

skill_row=$(bash "$analyzer" \
  --workspace "$temporary_directory/workspace" \
  "$temporary_directory/skill-reads.jsonl")
[ "${skill_row##*$'\t'}" = "3" ] || fail "skill-read count row = $skill_row"

skill_evidence=$(bash "$analyzer" \
  --workspace "$temporary_directory/workspace" \
  --skill-reads "$temporary_directory/skill-reads.jsonl")
printf '%s\n' "$skill_evidence" | grep -q '"event_order":1' ||
  fail "first skill event order missing"
printf '%s\n' "$skill_evidence" | grep -q '/Users/me/.codex/skills/brainstorming/SKILL.md' ||
  fail "first generic skill target missing"
printf '%s\n' "$skill_evidence" | grep -q '"event_order":4' ||
  fail "second skill event order missing"
printf '%s\n' "$skill_evidence" | grep -q '/opt/codex/plugins/vendor/skills/tdd/references/guide.md' ||
  fail "second generic skill target missing"
case "$skill_evidence" in
  *ordinary-external*|*workspace-skill*) fail "non-contaminating target entered evidence" ;;
esac
target_count=$(printf '%s\n' "$skill_evidence" | grep -o '/opt/codex/plugins/vendor/skills/review[^" ]*' | wc -l | tr -d ' ')
[ "$target_count" = "2" ] || fail "multi-target skill evidence = $skill_evidence"
```

- [ ] **Step 3: Run the analyzer test and confirm the new modes are rejected**

```bash
bash scripts/analyze-agent-context-log_test.sh
```

Expected: FAIL because the analyzer accepts neither `--usage`, `--workspace`,
nor `--skill-reads`.

- [ ] **Step 4: Parse strict and legacy token views independently**

Modify `analysis`:

```go
type analysis struct {
	metrics      metrics
	legacyTokens int64
	usage        agentmetrics.TokenUsage
	usageErr     error
}
```

For every `turn.completed` event:

```go
legacyTokens, err := legacyTokenUsage(outer.Usage)
if err != nil {
	return analysis{}, fmt.Errorf("turn.completed at line %d: %w", lineNumber, err)
}
usage, usageErr := agentmetrics.ParseTokenUsage(outer.Usage)
result.legacyTokens = legacyTokens
result.usage = usage
result.usageErr = usageErr
seenUsage = true
```

Rename the current `tokenUsage` function to `legacyTokenUsage`; do not alter
its behavior. Structural metrics, `--tokens`, and `--skill-reads` continue to
accept a legacy event when `legacyTokenUsage` succeeds. In the `usage` output
case, return the saved strict error before printing:

```go
case "usage":
	if result.usageErr != nil {
		die(result.usageErr)
	}
	fmt.Println(result.usage.TSV())
	return
```

- [ ] **Step 5: Add analyzer configuration and output modes**

Replace positional mode parsing with one pass that accepts `--workspace` once
and exactly one of `--header`, `--tokens`, `--usage`, or `--skill-reads` in any
order. Use this configuration:

```go
type analyzerConfig struct {
	mode      string
	workspace string
	path      string
}
```

Update usage text to:

```text
usage: analyze-agent-context-log.go [--header|--tokens|--usage|--skill-reads] [--workspace /absolute/path] /absolute/path/to/transcript.jsonl
```

In `main`:

```go
switch mode {
case "tokens":
	fmt.Println(result.legacyTokens)
	return
case "usage":
	if result.usageErr != nil {
		die(result.usageErr)
	}
	fmt.Println(result.usage.TSV())
	return
case "skill-reads":
	if result.metrics.workspace == "" {
		die(errors.New("--skill-reads requires --workspace"))
	}
	if err := json.NewEncoder(os.Stdout).Encode(result.metrics.skillReads); err != nil {
		die(err)
	}
	return
}
```

- [ ] **Step 6: Record ordered command-target evidence**

Add:

```go
type skillReadEvidence struct {
	EventOrder int    `json:"event_order"`
	ItemID     string `json:"item_id"`
	Command    string `json:"command"`
	Target     string `json:"target"`
}
```

Extend `metrics` with `workspace`, `commandDirectory`, `eventOrder`, current
item/command fields, `skillReads []skillReadEvidence`, and
`skillReadEvents map[int]struct{}`. Initialize the command directory from
`--workspace`. Increment `eventOrder` once for every unique completed item
before classifying it. For every target already parsed
by `recordSearchTargets`, `recordFindTargets`, and `recordReadTargets`, call:

```go
func recordCommandTarget(target string, metrics *metrics) {
	normalized, ok := agentmetrics.ClassifyExternalSkillTarget(
		metrics.workspace,
		metrics.commandDirectory,
		target,
	)
	if !ok {
		return
	}
	metrics.skillReads = append(metrics.skillReads, skillReadEvidence{
		EventOrder: metrics.eventOrder,
		ItemID:     metrics.currentItemID,
		Command:    metrics.currentCommand,
		Target:     normalized,
	})
	metrics.skillReadEvents[metrics.eventOrder] = struct{}{}
}
```

Call it before source-extension filtering so `SKILL.md` and skill references
are visible without changing source-read metrics. Do not call it for
`file_change`. When a compound-command segment is exactly `cd <absolute-path>`,
set `commandDirectory` to that path for subsequent segments in the same
command. Reset it to the benchmark workspace before the next terminal item.
Deduplicate identical normalized targets within one terminal item before
appending evidence; two distinct skill targets retain two evidence entries but
the event map still counts the terminal call once.

Append `len(result.metrics.skillReadEvents)` to the normal metric row and
`external_skill_read_calls` to `agentmetrics.AnalyzerHeader`.

- [ ] **Step 7: Prove navigation metrics are unchanged and evidence is stable**

Keep the first eleven fields of every existing expected navigation row
unchanged, append the expected zero contamination count, and run:

```bash
gofmt -w scripts/analyze-agent-context-log.go
bash scripts/analyze-agent-context-log_test.sh
```

Expected: PASS.

- [ ] **Step 8: Commit additive analyzer evidence**

```bash
git add scripts/analyze-agent-context-log.go scripts/analyze-agent-context-log_test.sh
git commit -m "Expose deterministic transcript benchmark evidence" -m "- Add strict current-counter usage output to the transcript analyzer
- Preserve the legacy token-total mode for retained diagnostics
- Record ordered external skill reads without plugin-specific rules"
```

### Task 4: Migrate monotonic runner and gate contracts

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
- Adds `RunMetrics.ExternalSkillReadCalls int64` and persists it for diagnosis.
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
	Tokens                  int64                   `json:"tokens"`
	TokenUsage              agentmetrics.TokenUsage `json:"token_usage"`
	ExternalSkillReadCalls int64                   `json:"external_skill_read_calls"`
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
	usage                  agentmetrics.TokenUsage
	externalSkillReadCalls int64
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

Invoke the normal analyzer metrics mode with `--workspace plan.workspace`,
parse the appended `external_skill_read_calls` field as a non-negative integer,
and carry both values through failure artifacts, `.metrics.tsv`, review
templates, and `summary.tsv`.

- [ ] **Step 5: Make summary columns explicit**

Set the regression header to:

```go
const RegressionSummaryHeader = "case\tquery\tbuild\trun\tattempt\teffective_tokens\tinput_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\texternal_skill_read_calls\ttool_calls\tcontext_calls\trepeated_full_packs\tbroad_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tcontext_millis\tlog"
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

Make the fake analyzer normal row end in `0`, assert the regression summary's
`external_skill_read_calls` column is zero, and add a fixture row ending in `2`
to prove the value is retained as evidence without becoming a monotonic gate.

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
- Keep raw totals and external skill reads as explicit diagnostics"
```

### Task 5: Migrate and isolate the matched release harness

**Files:**

- Modify: `scripts/benchmark-agent-context.sh`
- Modify: `scripts/benchmark-agent-context_test.sh`
- Modify: `internal/agentmetrics/schema.go`

**Interfaces:**

- Release summary uses the same seven token columns as the regression summary.
- Release summary adds `external_skill_read_calls` after `total_tokens`.
- Release gates compare `effective_tokens`.
- The absolute prospective cap remains exactly 116,560.
- Both variants require zero external skill reads over their entire transcript.
- The harness captures `codex plugin list --json` before the first external run and never changes plugin state.
- The first contaminated run is retained and terminates the matrix without retry or replacement.

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

- [ ] **Step 2: Add fail-first contamination and inventory tests**

Extend the fake `codex` before its normal execution path:

```bash
if [ "${1:-}" = "plugin" ] && [ "${2:-}" = "list" ] && [ "${3:-}" = "--json" ]; then
  [ "${FAKE_PLUGIN_LIST_FAIL:-0}" = "0" ] || exit 9
  printf '[{"id":"workflow-tools","enabled":true}]\n'
  exit 0
fi
```

In the baseline branch, emit a generic external skill read when requested:

```bash
if [ "${FAKE_BASELINE_SKILL_READ:-0}" = "1" ]; then
  emit_command 'cat /opt/codex/plugins/vendor/skills/brainstorming/SKILL.md'
fi
```

Forward both environment variables through `run_harness`, then add:

```bash
FAKE_BASELINE_SKILL_READ=1
export FAKE_BASELINE_SKILL_READ
if run_harness contaminated >/dev/null 2>&1; then
  fail "contaminated baseline passed"
fi
unset FAKE_BASELINE_SKILL_READ
[ "$(tr -d '\n' <"$temporary_directory/contaminated.order")" = "b" ] ||
  fail "harness did not stop after first contaminated run"
grep -q $'^baseline\t1\t.*\t1\t' "$temporary_directory/contaminated/summary.tsv" ||
  fail "contaminated run was not retained in summary"
[ -s "$temporary_directory/contaminated/baseline-1.log.skill-reads.json" ] ||
  fail "ordered skill evidence was not retained"
[ ! -e "$temporary_directory/contaminated/assisted-1.log" ] ||
  fail "harness launched a run after contamination"

FAKE_PLUGIN_LIST_FAIL=1
export FAKE_PLUGIN_LIST_FAIL
if run_harness plugin-inventory-failure >/dev/null 2>&1; then
  fail "missing plugin inventory passed"
fi
unset FAKE_PLUGIN_LIST_FAIL
[ ! -s "$temporary_directory/plugin-inventory-failure.order" ] ||
  fail "Codex run started after plugin inventory failure"
```

- [ ] **Step 3: Run the release harness test and confirm old behavior fails**

```bash
bash scripts/benchmark-agent-context_test.sh
```

Expected: FAIL because the harness still extracts legacy aggregate tokens,
does not capture plugin inventory, and does not reject skill reads.

- [ ] **Step 4: Parse one explicit usage row per run**

Replace `extract_tokens` with:

```bash
extract_usage() {
  bash "$analyzer" --workspace "$workspace" --usage "$1"
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

- [ ] **Step 5: Replace ambiguous summary headers**

Set:

```go
const ReleaseSummaryHeader = "variant\trun\teffective_tokens\tinput_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\texternal_skill_read_calls\ttool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tunique_source_files\tlog"
```

Use this exact order in `benchmark-agent-context.sh` and its expected-header
test.

- [ ] **Step 6: Keep both prospective gates strict**

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

- [ ] **Step 7: Capture configuration and stop after contamination**

Before `codex --version` and before any prompt execution, add:

```bash
if ! codex plugin list --json \
  >"$output/codex-plugins.json" \
  2>"$output/codex-plugins.stderr"; then
  die "cannot capture Codex plugin inventory; no benchmark run started"
fi
[ -s "$output/codex-plugins.json" ] ||
  die "Codex plugin inventory is empty; no benchmark run started"
```

Keep the existing exact `codex-args.txt` and `codex-version.txt` artifacts. In
`run_variant`, invoke normal metrics with `--workspace "$workspace"`, parse
the appended `external_skill_read_calls`, and write ordered evidence before
appending the summary row:

```bash
bash "$analyzer" --workspace "$workspace" --skill-reads "$log_path" \
  >"$log_path.skill-reads.json"
```

After the complete run row and evidence files are safely written, enforce:

```bash
[ "$external_skill_read_calls" -eq 0 ] ||
  die "$variant run $run_number read $external_skill_read_calls external skill files; matrix stopped and evidence retained"
```

Do not add a retry loop or replacement-run counter. This check remains inside
`run_variant`, so the alternating outer loop cannot start the next variant.

- [ ] **Step 8: Run both shell harness suites**

```bash
gofmt -w internal/agentmetrics/schema.go
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
```

Expected: PASS.

- [ ] **Step 9: Commit the release harness migration**

```bash
git add internal/agentmetrics/schema.go scripts/benchmark-agent-context.sh scripts/benchmark-agent-context_test.sh
git commit -m "Gate release benchmarks on effective tokens" -m "- Record raw, cached, uncached, output, reasoning, total, and effective usage
- Reject generic external skill reads in either controlled variant
- Capture plugin inventory and retain the first contaminated run"
```

### Task 6: Deterministic retained-log calibration report

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

### Task 7: Publish metric, skill policy, and release-status truth

**Files:**

- Modify: `scripts/sync-docs/main.go`
- Modify: `scripts/sync-docs/main_test.go`
- Modify: `README.md`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/RELEASE.md`
- Modify: `docs/OUTPUTS.md`

**Interfaces:**

- Generated metric blocks use `agentmetrics.ReleaseSummaryHeader` and
  `agentmetrics.RegressionSummaryHeader`.
- Documentation defines effective, total, cached, and reasoning semantics once.
- Documentation distinguishes normal skill compatibility from the controlled
  release matrix's zero-skill-read requirement.
- Release evidence status states that the retained 3×3 result failed and is not
  rescored.

- [ ] **Step 1: Write failing documentation-truth assertions**

In `TestGeneratedFactsDescribeImplementedRuntimeDepth`, require:

```go
for _, want := range []string{
	"effective_tokens",
	"cached_input_tokens",
	"reasoning_output_tokens",
	"external_skill_read_calls",
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
"zero external skill reads"
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

`external_skill_read_calls` counts transcript-observed read or search targets outside the benchmark workspace that resolve to a skill bundle. Controlled baseline and assisted release runs require zero; normal GoreGraph workflows may continue to use task-scoped skills.
```

Keep all existing source-read classifications.

- [ ] **Step 4: Correct the generated release evidence status**

Set `renderCurrentReleaseEvidenceStatus` to:

```go
return "The latest controlled three-by-three release benchmark did not pass: " +
	"its raw total-token medians were 2551495 baseline and 147212 assisted, so " +
	"the assisted result exceeded the legacy 116560 absolute cap. The retained " +
	"result remains failed and is not rescored. A prospective offline calculation " +
	"produced effective-token medians of 164295 and 39180, but both variants also " +
	"contained external skill reads. Publication remains blocked until a fresh " +
	"prospectively calibrated matrix has zero external skill reads and receives " +
	"the required signed 12-point quality review."
```

- [ ] **Step 5: Update hand-written token-gate sections**

In `docs/BENCHMARKING.md`, `README.md`, `docs/RELEASE.md`, and
`docs/OUTPUTS.md` state exactly:

- the prospective formula;
- both raw and effective counters are retained;
- the 80% matched threshold uses effective tokens;
- the 116,560 absolute cap uses effective tokens;
- reasoning output is not double-counted;
- the previous 3×3 result remains failed;
- `external_skill_read_calls` is plugin-agnostic transcript evidence;
- both controlled variants require zero external skill reads across the complete transcript;
- normal use remains compatible with Brainstorming, TDD, debugging, and review skills;
- `--ignore-user-config` is not a skill-isolation guarantee;
- the harness records plugin state but never mutates it;
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
git add scripts/sync-docs README.md docs/BENCHMARKING.md docs/RELEASE.md docs/OUTPUTS.md
git commit -m "Document deterministic release benchmark policy" -m "- Define effective, raw, cached, output, and reasoning token semantics
- Correct the retained three-by-three failure status without rescoring it
- Distinguish normal skill compatibility from controlled zero-skill qualification"
```

### Task 8: Local verification, control calibration, and release-matrix handoff

**Files:**

- Verify only; no source changes are expected before benchmark evidence exists.

**Interfaces:**

- Consumes: the implementation commits and the six retained control transcripts.
- Produces: local green verification, an offline calibration report, an
  installed candidate, and a hard stop before any new external run.

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
retained candidate evidence in
`/private/tmp/goregraph-release-3x3-c8d0afc-20260731`, independently verify:

- baseline effective-token median: 164,295;
- assisted effective-token median: 39,180;
- assisted share: 23.85%, with 76.15% savings;
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

- [ ] **Step 4: Audit retained skill evidence without executing Codex**

Run the updated analyzer against all six retained transcripts with the retained
workspace path and write the audit beside that evidence. Confirm:

- baseline external skill reads are `4`, `2`, and `0`;
- assisted external skill reads are `2`, `5`, and `4`;
- every assisted skill read precedes its first GoreGraph context call;
- the audit does not alter or replace the failed matrix verdict.

If the new analyzer cannot reproduce this retained evidence, stop and fix the
classifier before requesting another matrix.

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
- `codex-plugins.json` and `codex-plugins.stderr`;
- six ordered `*.skill-reads.json` files;
- the calibration report;
- the unsigned 12-point review template.

- [ ] **Step 7: Apply release gates without reinterpretation**

The matrix passes only if:

- assisted effective-token median is at most 80% of baseline;
- assisted effective-token median is at most 116,560;
- assisted tool-call median is at most 70% of baseline;
- assisted source-read median is at most 50% of baseline;
- repeated full packs and included-source rereads are zero;
- `external_skill_read_calls` is zero for every baseline and assisted run;
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
