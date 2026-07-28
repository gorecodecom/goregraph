# Language Capability and Documentation Truth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make runtime coverage and current public capability statements derive from one tested code-owned language profile.

**Architecture:** Introduce a richer immutable profile beside the scanner, derive legacy analyzer records and capability coverage from it, and expose a deterministic renderer to a dependency-free documentation synchronization command. Move the canonical eleven-line agent instruction to an import-safe package so CLI help, MCP instructions, generated guides, tests, and factual documentation blocks share one value.

**Tech Stack:** Go standard library, Markdown marker blocks, existing scanner and documentation tests.

## Global Constraints

- Documentation describes only implemented and representative-test-backed behavior.
- Pattern-backed extraction is labeled pattern-backed and never presented as runtime completeness.
- Unsupported claims are downgraded; analyzers are not expanded to protect wording.
- Narrative documentation remains hand-written.
- Historical specifications and plans are not rewritten.
- The command supports deterministic `--write` and read-only `--check`.
- No new dependency is added.

---

### Task 1: Define the Code-Owned Language Profile

**Files:**
- Create: `internal/scan/language_profiles.go`
- Create: `internal/scan/language_profiles_test.go`
- Modify: `internal/scan/analyzers.go`
- Modify: `internal/scan/capabilities.go`
- Modify: `internal/scan/capabilities_test.go`

**Interfaces:**
- Produces: exported `LanguageCapabilityProfiles() []LanguageCapabilityProfile`.
- Produces: internal `languageCapabilityProfile(language string) (LanguageCapabilityProfile, bool)`.
- Preserves: `AnalyzerRecord` JSON shape and output names.

- [ ] **Step 1: Write failing profile truth tests**

Define a profile with:

```go
type LanguageCapabilityProfile struct {
	Language          string
	DisplayName       string
	Level             string
	Scope             string
	Symbols           bool
	Relations         bool
	Calls             bool
	Routes            bool
	Tests             bool
	APIClients        bool
	Persistence       bool
	Messaging         bool
	DataFlow          bool
	ExactSymbols      bool
	DirectUsages      bool
	HTTPProvider      bool
	HTTPConsumer      bool
	PatternFamilies   []string
	Limitations       string
	Outputs           []string
}
```

Assert that Java, JavaScript, TypeScript, Go, PHP, Rust, Python, Shell, Kotlin,
Scala, Swift, Ruby, C, C++, and C# have explicit profiles; Shell has no routes
or tests; Rust has calls, routes, and tests; only Java and JS/TS expose exact
symbols and direct usages; HTTP provider/consumer flags match current tested
workspace symbol behavior.

- [ ] **Step 2: Run scanner tests and verify red**

Run:

```bash
go test ./internal/scan -run 'LanguageProfile|CapabilityInventory|Analyzer' -count=1
```

Expected: FAIL because profiles do not exist and the current broad architecture
capability mapping reports unsupported completeness.

- [ ] **Step 3: Implement profiles and derive legacy analyzers**

Build one sorted profile registry. Make `analyzerCapabilities` project each
profile into the existing `AnalyzerRecord`. Keep metadata/workspace analyzers
explicit but outside the public source-language table.

- [ ] **Step 4: Derive honest capability coverage**

Use profile flags for symbols, relations, calls, routes, and tests. Report
API-client, persistence, messaging, and data-flow support as `PARTIAL` with a
pattern-backed limitation when the profile enables those extractors; report
them unavailable otherwise. Evidence IDs remain linked when facts were emitted.

- [ ] **Step 5: Run focused scanner tests and verify green**

Run:

```bash
gofmt -w internal/scan/language_profiles.go internal/scan/language_profiles_test.go internal/scan/analyzers.go internal/scan/capabilities.go internal/scan/capabilities_test.go
go test ./internal/scan -run 'LanguageProfile|CapabilityInventory|Analyzer' -count=1
```

Expected: PASS.

### Task 2: Centralize the Agent Instruction

**Files:**
- Create: `internal/agentguide/instruction.go`
- Create: `internal/agentguide/instruction_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/mcp/mcp.go`
- Modify: `internal/mcp/mcp_test.go`
- Modify: `internal/scan/agent_reports.go`
- Modify: `internal/scan/agent_reports_test.go`
- Modify: `docs_test.go`

**Interfaces:**
- Produces: `agentguide.AssistedInstruction` and `agentguide.AssistedInstructionLineCount`.
- Consumes: the constant from CLI context help, MCP server instructions, and generated agent guides.

- [ ] **Step 1: Write a failing canonical-value test**

Assert that `AssistedInstructionLineCount == 11`, the first line contains the
executable `goregraph context . --query` form, and the final line prohibits
specialist tools.

- [ ] **Step 2: Run focused tests and verify red**

Run:

```bash
go test ./internal/agentguide ./internal/cli ./internal/mcp ./internal/scan . -run 'Instruction|AgentGuide|ContextHelp' -count=1
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Replace runtime duplicates with the canonical constant**

Compose surrounding help text with `agentguide.AssistedInstruction`. Keep
formatting and one-copy assertions intact. Do not introduce an import cycle:
`internal/agentguide` imports only the standard library.

- [ ] **Step 4: Run focused tests and verify green**

Run:

```bash
gofmt -w internal/agentguide internal/cli/cli.go internal/cli/cli_test.go internal/mcp/mcp.go internal/mcp/mcp_test.go internal/scan/agent_reports.go internal/scan/agent_reports_test.go docs_test.go
go test ./internal/agentguide ./internal/cli ./internal/mcp ./internal/scan . -run 'Instruction|AgentGuide|ContextHelp|Documentation' -count=1
```

Expected: PASS.

### Task 3: Add Deterministic Documentation Synchronization

**Files:**
- Create: `scripts/sync-docs/main.go`
- Create: `scripts/sync-docs/main_test.go`
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/RELEASE.md`
- Modify: `docs_test.go`

**Interfaces:**
- Consumes: `scan.LanguageCapabilityProfiles()`, `agentguide.AssistedInstruction`, `version.Version`, and `scan.SchemaVersion`.
- Produces: `go run ./scripts/sync-docs --write` and `go run ./scripts/sync-docs --check`.

- [ ] **Step 1: Write failing renderer and drift tests**

Use temporary Markdown files with markers:

```text
<!-- goregraph:generated <block-name> start -->
stale
<!-- goregraph:generated <block-name> end -->
```

Assert deterministic replacement, read-only drift failure, unknown or duplicate
marker rejection, missing marker rejection, and no changes outside marker
bounds.

- [ ] **Step 2: Run command tests and verify red**

Run:

```bash
go test ./scripts/sync-docs -count=1
```

Expected: FAIL because the synchronization command does not exist.

- [ ] **Step 3: Implement the standard-library synchronizer**

Accept exactly one of `--write` or `--check`. Render:

- `language-coverage` for the README table and limitations;
- `agent-instruction` for each current protocol copy;
- `agent-regression-metrics` for current metric names and gate semantics;
- `current-contract` for current version and Schema where marked.

In check mode, report `path: block <name> is stale` and exit nonzero without
writing.

- [ ] **Step 4: Mark and regenerate factual blocks**

Add markers only around the factual content. Run:

```bash
go run ./scripts/sync-docs --write
```

Expected: README preserves the newly merged Codex MCP setup; Shell is not
described as having routes/tests; Rust is not described as index-only; current
instruction wording says eleven lines; current metric names agree.

- [ ] **Step 5: Add repository-level drift enforcement**

Add a documentation test that invokes the same in-process block validation or
the command's exported check function without modifying files. It must fail
when any marked block differs.

### Task 4: Audit Current Documentation Against Runtime Truth

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `SCHEMA.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/RELEASE.md`
- Modify: CLI/MCP help only when an ungenerated factual mismatch remains

**Interfaces:**
- Consumes: generated factual blocks and current runtime behavior.
- Produces: truthful current-product documentation without editing dated plans/specs.

- [ ] **Step 1: Run a targeted stale-claim search**

Run:

```bash
rg -n 'nine-line|Shell.*route|Shell.*test|Rust.*best-effort|build \\.|scan-all \\.|1\\.3\\.0|Schema 3|source_read_calls' README.md COMMANDS.md SCHEMA.md docs/OUTPUTS.md docs/BENCHMARKING.md docs/RELEASE.md internal/cli internal/mcp
```

Review every match in its historical or current context.

- [ ] **Step 2: Correct only current-product mismatches**

Keep dated release history as history. Correct current statements about
languages, depth, static limitations, workspace command syntax, current
benchmarks, metric semantics, version, and schema. Preserve the Codex MCP setup
from `main`.

- [ ] **Step 3: Run complete documentation and scanner verification**

Run:

```bash
gofmt -w internal/scan internal/agentguide internal/cli internal/mcp scripts/sync-docs
go run ./scripts/sync-docs --check
go test ./internal/scan ./internal/agentguide ./internal/cli ./internal/mcp ./scripts/sync-docs . -count=1
go vet ./...
go test ./... -count=1
git diff --check
```

Expected: all checks pass and sync check performs no writes.

- [ ] **Step 4: Commit runtime truth separately**

```bash
git add internal/scan internal/agentguide internal/cli internal/mcp scripts/sync-docs docs_test.go
git commit -m "Derive language capabilities from one profile" -m "- Define tested language depth, architecture patterns, exact symbol support, and HTTP reachability.
- Reuse one canonical eleven-line agent instruction across runtime surfaces.
- Add deterministic documentation drift checks without new dependencies."
```

- [ ] **Step 5: Commit factual documentation**

```bash
git add README.md COMMANDS.md SCHEMA.md docs/OUTPUTS.md docs/BENCHMARKING.md docs/RELEASE.md
git commit -m "Synchronize public capability documentation" -m "- Render current language depth and agent protocol from code-owned facts.
- Correct Shell, Rust, metric, CLI, version, and schema drift.
- Preserve historical release notes and Codex MCP setup."
```
