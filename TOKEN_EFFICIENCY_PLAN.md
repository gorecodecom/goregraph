# Source-Backed Context Packs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce end-to-end agent tokens by making the existing `task_context` response replace the first source reads for the selected production path instead of only pointing agents at files.

**Architecture:** Keep the current deterministic agent index, ranking, workspace reconciliation, and one-tool MCP surface. After the existing compiler selects an evidence-backed flow, a new read-only source projection resolves those selected files inside their project roots, verifies or safely relocates the indexed symbol range, and appends a small number of current, line-numbered source sections within one total response budget. MCP initialization instructions tell agents to treat included sections as already read and to inspect source directly only for explicit omissions, uncovered locations reported by partial/absent coverage, or fallback packs.

**Tech Stack:** Go 1.23+ standard library, existing Schema 3 agent index, JSON-RPC/MCP over stdio, existing lexical ranking and evidence model, Go tests, current shell benchmark harness, local Codex CLI acceptance runs.

## Global Constraints

- Work on the current branch selected by the repository owner; do not create or switch branches as part of this plan.
- Use strict TDD: add one focused failing test, run it and observe the failure, implement the smallest behavior, then rerun the focused and package tests.
- Add no runtime dependency, parser dependency, tokenizer dependency, daemon, watcher, telemetry, network request, hook, or agent-configuration write.
- Reuse `agent/context-index.json`, fact IDs, line ranges, call-chain ranking, confidence, evidence, workspace paths, and fallback behavior.
- Do not modify source extraction, language coverage, dashboard behavior, workspace reconciliation semantics, or the generated agent-index schema in this iteration.
- Standard MCP continues to expose exactly one tool, `task_context`; expert tools remain opt-in and unchanged.
- Source access is read-only. Reject absolute fact paths, traversal, symlink escapes, directories, non-regular files, and files larger than 2 MiB.
- Never execute indexed project code or invoke its build tools while compiling a Context Pack.
- Include source only when `fallback_required` is false and confidence is not `LOW`.
- Read included source from the current working tree at request time. If the indexed symbol is absent, call-only, ambiguous, unreadable, or not UTF-8 text, omit its content with an explicit reason.
- Keep `files` for compatibility. Add only optional `source_sections`, `source_omissions`, `source_coverage`, and `source_unrepresented` fields.
- The default total budget is 4,000 estimated tokens and 16,000 serialized UTF-8 bytes. Accept `256..6000` tokens with an absolute 24,000-byte ceiling.
- Reserve at most 1,100 estimated tokens for metadata under the default budget. Do not fill spare space with extra evidence IDs.
- Include at most four source sections and three detailed source omissions. Set `source_unrepresented` to the exact number of selected source candidates that have neither an included section nor a detailed omission. Prefer the production entrypoint and call-chain code over contracts, persistence, and tests.
- Include test bodies only when the query explicitly asks about tests; otherwise tests remain locations or signatures.
- If a body does not fit, retry as a focused window and then a signature before omitting it. Never truncate in the middle of a source line.
- Preserve byte-identical serialized output for an unchanged index and working tree. Private selected-fact state must not be serialized.
- Optimize the complete agent session, not Context Pack size. A larger pack is acceptable only when matched A/B runs reduce total tokens without reducing evidence quality.
- Keep the existing release gate: assisted median at least 20% below its paired baseline with no lower twelve-point evidence score.
- Add a second gate: source-backed assisted median at least 15% below a metadata-only assisted median captured from the mandatory pre-source checkpoint with the same indexed workspace snapshot, task prompt, Codex version, model, reasoning, sandbox, and approvals. Record the distinct GoreGraph commits and binaries used for the control and treatment.
- Use English source comments, CLI text, documentation, tests, and commit messages.

---

## Why This Is the First Efficiency Slice

The current compiler already loads one compact index, ranks routes and symbols, selects a production entrypoint, expands a bounded chain, and returns exact files and ranges. The missing step is delivery: the agent must still call `Read`, `Grep`, or shell commands for those files.

This plan keeps the existing analysis and makes the selected path self-sufficient. It deliberately does not copy CodeGraph's broad parser, SQLite index, watcher, installer, or language matrix. Those are independent investments and should not obscure the highest-leverage hypothesis: current source bodies plus precise MCP instructions eliminate repeated navigation.

## Public Contract and File Map

### Create

- `internal/agent/context_source.go`: project/workspace source resolution, containment, current-file loading, range verification, safe relocation, line-number rendering, and render fallbacks.
- `internal/agent/context_source_test.go`: project/workspace/security/rendering tests.

### Modify

- `internal/agent/context.go`: additive source records, total budget constants, and orchestration.
- `internal/agent/context_load.go`: return the selected index path and source scope.
- `internal/agent/context_rank.go`: reserve metadata space, stop evidence backfill, exact query anchors, and source priority.
- `internal/agent/context_test.go`: ranking, compatibility, attachment, fallback, and determinism.
- `internal/agent/context_size_test.go`: total token and byte ceilings.
- `internal/agent/context_workspace_test.go`: source from multiple projects.
- `internal/mcp/mcp.go`: initialization instructions, source-aware tool description, and bounds.
- `internal/mcp/mcp_test.go`: instructions, one-tool surface, bounds, and source response.
- `README.md`, `COMMANDS.md`, `docs/OUTPUTS.md`, `docs/BENCHMARKING.md`: source-backed workflow.
- `scripts/benchmark-agent-context.sh`, `scripts/benchmark-agent-context_test.sh`: exact assisted instruction.

### Additive records

Add to `internal/agent/context.go`:

```go
const (
	DefaultContextBudgetTokens         = 4000
	MinContextBudgetTokens             = 256
	MaxContextBudgetTokens             = 6000
	DefaultContextMetadataBudgetTokens = 1100
	DefaultContextMaxBytes             = 16000
	MaxContextBytes                    = 24000
	DefaultContextMaxFiles             = 12
	MaxContextMaxFiles                 = 20
	MaxContextSourceSections           = 4
	MaxContextSourceOmissions          = 3
	MaxContextSourceFileBytes          = 2 << 20
)

type ContextSourceSection struct {
	Project     string `json:"project,omitempty"`
	Path        string `json:"path"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	Role        string `json:"role"`
	RenderMode  string `json:"render_mode"`
	SourceState string `json:"source_state"`
	Content     string `json:"content"`
}

type ContextSourceOmission struct {
	Project string `json:"project,omitempty"`
	Path    string `json:"path"`
	Role    string `json:"role"`
	Reason  string `json:"reason"`
}
```

Append to `ContextPack`:

```go
SourceSections  []ContextSourceSection  `json:"source_sections,omitempty"`
SourceOmissions []ContextSourceOmission `json:"source_omissions,omitempty"`
SourceCoverage  string                  `json:"source_coverage,omitempty"`
SourceUnrepresented int                 `json:"source_unrepresented,omitempty"`

selectedSourceFactIDs []string
```

`selectedSourceFactIDs` is private compiler state. `cloneContextPack` must copy it, and JSON serialization must never expose it.

Allowed `source_coverage` values:

- `complete`: every selected source candidate is present in `source_sections`.
- `partial`: at least one source section is present, but one or more selected candidates are omitted or unrepresented.
- `none`: no selected source candidate could be included. Fallback and low-confidence packs use `none` and contain no source bodies.

An agent must never infer complete coverage from an empty omission list. `source_unrepresented > 0` means the bounded omission detail was exhausted; the remaining uncovered locations are still available through `files`.

Allowed `render_mode` values: `body`, `focused`, `signature`.

Allowed `source_state` values:

- `indexed_range_current`: the current file still contains exactly one declaration-like occurrence of the indexed symbol in the indexed range.
- `relocated_current`: the range moved, but the exact identifier has exactly one declaration-like occurrence in the current file.

Stale or ambiguous content is never emitted. Stable omission reasons are:

- `source file is missing`
- `source path escapes project root`
- `source file is not a regular file`
- `source file exceeds 2097152 bytes`
- `source file is unreadable`
- `source file is not UTF-8 text`
- `indexed symbol is absent from current source`
- `indexed symbol is ambiguous in current source`
- `indexed symbol has no unique declaration-like occurrence`
- `source section does not fit the response budget`

### Private loaded-index contract

```go
type loadedContextIndex struct {
	Index     scan.AgentContextIndexRecord
	IndexPath string
	ScopeRoot string
	Workspace bool
}

type contextIndexCandidate struct {
	Path      string
	ScopeRoot string
	Workspace bool
}

func loadContextIndex(request ContextRequest) (loadedContextIndex, error)
```

`loadContextIndex` computes an absolute request root once and constructs each `contextIndexCandidate` with the exact root already known at discovery time. A project candidate uses the absolute requested project root even when `cfg.OutputDir` contains multiple path segments. A workspace candidate uses the absolute root returned by workspace discovery. Never derive `ScopeRoot` by counting parents above `context-index.json`.

Project source resolves as `<ScopeRoot>/<fact.File>`. Workspace source resolves as `<ScopeRoot>/<fact.Project>/<fact.File>`.

---

### Task 1: Teach MCP clients to reuse included source

**Files:**

- Modify: `internal/mcp/mcp.go:74-128`
- Modify: `internal/mcp/mcp_test.go:204-247`

**Produces:** `serverInstructions() string` and `initialize.result.instructions`.

- [ ] **Step 1: Write the failing initialization test**

```go
func TestInitializeInstructsAgentsToReuseIncludedSource(t *testing.T) {
	result := handle(request{JSONRPC: "2.0", ID: 1, Method: "initialize"}, Options{})
	body, err := json.Marshal(result.Result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"Call task_context before Read or Grep",
		"Treat source_sections as current source already read",
		"Do not re-read or grep included ranges",
		"If source_coverage is absent, partial, or none, inspect only relevant uncovered ranges from source_omissions or files",
		"At most one narrower task_context retry",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("initialize instructions missing %q: %s", want, text)
		}
	}
}
```

- [ ] **Step 2: Verify the test fails**

Run: `go test ./internal/mcp -run TestInitializeInstructsAgentsToReuseIncludedSource -count=1`

Expected: `FAIL`; initialize has no `instructions` field.

- [ ] **Step 3: Add deterministic instructions**

```go
func serverInstructions() string {
	return "Call task_context before Read or Grep for indexed code questions. " +
		"Treat source_sections as current source already read. " +
		"Do not re-read or grep included ranges. " +
		"If source_coverage is absent, partial, or none, inspect only relevant uncovered ranges from source_omissions or files. " +
		"If fallback_required is true, stop using GoreGraph and inspect source directly. " +
		"At most one narrower task_context retry may use an exact route, qualified symbol, or file returned by the first call."
}
```

Add `"instructions": serverInstructions()` to the `initialize` result. Do not write agent configuration files.

- [ ] **Step 4: Update the tool description exactly**

```go
"description": "Return one evidence-backed Context Pack with current, line-numbered source for the central coding path. Treat source_sections as already read. When source_coverage is absent, partial, or none, inspect only relevant uncovered ranges. If fallback_required is true, inspect source directly. Call at most twice per task.",
```

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/mcp -count=1`

Expected: `PASS` after updating the existing exact-description assertion.

```bash
git add internal/mcp/mcp.go internal/mcp/mcp_test.go
git commit -m "Guide agents to reuse context source" -m "- Add deterministic initialization instructions for included and uncovered source.
- Keep the standard MCP surface limited to task_context."
```

### Task 2: Reserve the response budget for source

**Files:** `internal/agent/context.go`, `internal/agent/context_rank.go`, `internal/agent/context_test.go`, `internal/agent/context_size_test.go`, `internal/mcp/mcp.go`, `internal/mcp/mcp_test.go`

**Produces:** `contextMetadataBudget(total int) int`, `contextByteBudget(tokens int) int`, updated bounds, one retained evidence ID per selected location, and metadata that leaves source capacity.

- [ ] **Step 1: Write failing budget tests**

```go
func TestContextBudgetsReserveSpaceForSource(t *testing.T) {
	if got := contextMetadataBudget(DefaultContextBudgetTokens); got != DefaultContextMetadataBudgetTokens {
		t.Fatalf("metadata budget = %d, want %d", got, DefaultContextMetadataBudgetTokens)
	}
	if got := contextMetadataBudget(700); got != 700 {
		t.Fatalf("small metadata budget = %d, want 700", got)
	}
	if got := contextByteBudget(DefaultContextBudgetTokens); got != DefaultContextMaxBytes {
		t.Fatalf("default byte budget = %d, want %d", got, DefaultContextMaxBytes)
	}
	if got := contextByteBudget(MaxContextBudgetTokens); got != MaxContextBytes {
		t.Fatalf("maximum byte budget = %d, want %d", got, MaxContextBytes)
	}
}
```

Update invalid-bound tests to reject `6001` and MCP schema tests to expect default `4000`, maximum `6000`.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/agent ./internal/mcp -run 'TestContextBudgetsReserveSpaceForSource|TestBuildContextRejectsInvalidBounds|TestDefaultMCPTaskContextSchemaAndInstructions|TestMCPTaskContextAcceptsExactBounds' -count=1`

Expected: `FAIL`; new constants and helpers do not exist.

- [ ] **Step 3: Add budget helpers**

```go
func contextMetadataBudget(total int) int {
	if total < DefaultContextMetadataBudgetTokens {
		return total
	}
	return DefaultContextMetadataBudgetTokens
}

func contextByteBudget(tokens int) int {
	bytes := tokens * 4
	if bytes > MaxContextBytes {
		return MaxContextBytes
	}
	return bytes
}
```

Serialized checks use `len(json.Marshal(pack))`, not rune count.

- [ ] **Step 4: Retain one evidence ID without backfilling spare space**

Add a focused test with a selected fact containing three evidence IDs. Require the resulting entrypoint to retain exactly the first indexed ID:

```go
func retainFirstContextEvidence(fact scan.AgentContextFactRecord) scan.AgentContextFactRecord {
	if len(fact.EvidenceIDs) > 1 {
		fact.EvidenceIDs = append([]string(nil), fact.EvidenceIDs[:1]...)
	} else {
		fact.EvidenceIDs = append([]string(nil), fact.EvidenceIDs...)
	}
	return fact
}
```

Call `retainFirstContextEvidence(fact)` in `tryAddContextLocation` instead of clearing `compacted.EvidenceIDs`. Remove `backfillContextEvidence`, `maximizeContextEvidencePrefix`, and their maximal-fill tests. Do not add evidence IDs to locations that did not already originate from an indexed fact.

- [ ] **Step 5: Compile metadata under the reserved ceiling**

```go
metadataRequest := request
metadataRequest.BudgetTokens = contextMetadataBudget(request.BudgetTokens)
pack, err := compileContextPack(index, metadataRequest)
if err != nil {
	return ContextPack{}, err
}
pack.BudgetTokens = request.BudgetTokens
return finalizeContextEstimate(pack)
```

- [ ] **Step 6: Update the size test**

Require `EstimatedTokens <= 4000`, serialized bytes `<= 16000`, unchanged semantic fields, and byte-identical repeated output. Remove the old 7,200-byte ceiling.

- [ ] **Step 7: Verify and commit**

Run: `go test ./internal/agent ./internal/mcp -count=1`

Expected: `PASS`; the metadata pack retains useful facts but leaves most of the total budget unused.

```bash
git add internal/agent/context.go internal/agent/context_rank.go internal/agent/context_test.go internal/agent/context_size_test.go internal/mcp/mcp.go internal/mcp/mcp_test.go
git commit -m "Reserve context capacity for source" -m "- Raise and split the total and metadata response budgets.
- Retain one original evidence ID per selected location without backfilling."
```

## Mandatory Metadata-Only Benchmark Checkpoint

Complete this checkpoint after Task 2 and before creating `internal/agent/context_source.go`. Task 9 consumes this control; it must not attempt to reconstruct it after source attachment exists.

- [ ] Record the Task 2 GoreGraph commit, binary digest, GoreGraph version, Codex version, model, reasoning, sandbox, approval policy, indexed-workspace commit, index digest, task-prompt digest, and the exact metadata-only assisted instruction.
- [ ] Run three metadata-only assisted trials against one unchanged indexed workspace snapshot. Use one integration surface consistently for control and treatment: the existing harness uses `goregraph context`, while an MCP-specific run may use `task_context` only when both variants provision the same MCP configuration. The instruction must tell the agent to call the chosen context surface once before source reads and then use the returned `files` and ranges as navigation metadata. Do not mention `source_sections`, because the control binary cannot return them.
- [ ] Store raw transcripts, token totals, the median, and the twelve-point rubric outside the repository. Record only non-proprietary digests and aggregate results in any repository documentation.
- [ ] Do not start Task 3 until all three control runs and their signed rubric are present.

Expected: three metadata-only assisted transcripts, one signed rubric, and enough recorded provenance to replay the control without changing the indexed workspace under test.

### Task 3: Resolve source paths safely

**Files:** create `internal/agent/context_source.go`, create `internal/agent/context_source_test.go`, modify `internal/agent/context_load.go`, `internal/agent/context.go`, `internal/agent/context_test.go`.

**Produces:** `loadedContextIndex`, `sourceCandidate`, `resolveSourcePath`, and `readSourceFile`.

```go
type sourceCandidate struct {
	FactID     string
	Project    string
	Path       string
	StartLine  int
	EndLine    int
	Role       string
	Kind       string
	Name       string
	Qualified  string
	Priority   int
}

type sourceFile struct {
	Path  string
	Lines []string
}
```

- [ ] **Step 1: Write project/workspace tests**

Create real files below `t.TempDir()`. Assert project resolution uses `<ScopeRoot>/<Path>` and workspace resolution uses `<ScopeRoot>/<Project>/<Path>`. Add a configured project output such as `build/generated/goregraph`; require `ScopeRoot` to remain the project root rather than three parents above the index.

```go
tests := []struct {
	name   string
	loaded loadedContextIndex
	file   sourceCandidate
	want   string
}{
	{name: "project", loaded: loadedContextIndex{ScopeRoot: projectRoot}, file: sourceCandidate{Path: "src/UserService.java"}, want: projectFile},
	{name: "workspace", loaded: loadedContextIndex{ScopeRoot: workspaceRoot, Workspace: true}, file: sourceCandidate{Project: "services/users", Path: "src/UserService.java"}, want: workspaceFile},
}
```

- [ ] **Step 2: Write security tests**

Reject `/etc/passwd`, `../../outside.java`, workspace project `../../outside`, a symlink escaping the root, a directory, an unreadable regular file, a non-UTF-8 file, and a 2 MiB plus one byte file. Assert the stable reasons from the public contract. Skip only the unreadable-permission assertion on platforms where the test process can still open a mode-`000` file; all other cases remain mandatory.

- [ ] **Step 3: Verify failure**

Run: `go test ./internal/agent -run 'TestResolveSourcePath|TestReadSourceFile' -count=1`

Expected: `FAIL`; the resolver does not exist.

- [ ] **Step 4: Return source scope from index loading**

Build candidates with their exact roots before probing the index files:

```go
requestRoot, err := filepath.Abs(request.Root)
if err != nil {
	return loadedContextIndex{}, fmt.Errorf("resolve context root: %w", err)
}
candidates := []contextIndexCandidate{
	{
		Path:      scan.NewWorkspaceOutputLayout(filepath.Join(requestRoot, ".goregraph-workspace")).Agent("context-index.json"),
		ScopeRoot: requestRoot,
		Workspace: true,
	},
}
```

Append a detected parent-workspace candidate with the absolute discovered workspace root and a project candidate with `Path: filepath.Join(requestRoot, cfg.OutputDir, "agent", "context-index.json")` and `ScopeRoot: requestRoot`. On successful decode, copy the selected candidate's `Path`, `ScopeRoot`, and `Workspace` into `loadedContextIndex`. Add tests for nested configured project output, a workspace requested directly, and a detected parent workspace.

- [ ] **Step 5: Implement containment**

Reject empty/absolute fact paths; reject empty/absolute workspace project paths; clean and join; use `filepath.Rel` to reject `..`; resolve root and candidate symlinks; repeat `filepath.Rel`; require `Mode().IsRegular()`. Return and subsequently open the resolved candidate path, not the unresolved symlink path.

- [ ] **Step 6: Implement bounded reads**

Open the resolved file, map permission and other open/read failures to `source file is unreadable`, reject `Size() > MaxContextSourceFileBytes`, read with `io.LimitReader(file, MaxContextSourceFileBytes+1)`, reject overflow, require `utf8.Valid(body)`, normalize CRLF to LF, and split into physical lines.

- [ ] **Step 7: Verify and commit**

Run: `go test ./internal/agent -count=1`

Expected: `PASS`; packs do not include content yet.

```bash
git add internal/agent/context.go internal/agent/context_load.go internal/agent/context_source.go internal/agent/context_source_test.go internal/agent/context_test.go
git commit -m "Resolve context source safely" -m "- Carry exact project and workspace roots with index candidates.
- Reject traversal, unsafe files, unreadable content, and non-UTF-8 source."
```

### Task 4: Verify, relocate, and render current ranges

**Files:** `internal/agent/context_source.go`, `internal/agent/context_source_test.go`

**Produces:** `renderSourceCandidate(candidate sourceCandidate, file sourceFile, mode string) (ContextSourceSection, error)`.

- [ ] **Step 1: Write failing renderer tests**

Cover: indexed declaration current; unique declaration relocation after inserted lines; absent identifier; ambiguous declaration; a renamed declaration with one remaining old call; Java `Owner.method`, TypeScript `src/module#method`, and PHP-style `Owner::method` identifiers; body over 120 lines; focused maximum 61 lines; signature with annotations maximum 12 lines; every output line prefixed with one-based line number and tab.

```go
if section.Content != "10\t    public void deleteUser() {\n11\t        repository.delete();\n12\t    }" {
	t.Fatalf("rendered content:\n%s", section.Content)
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/agent -run TestRenderSourceCandidate -count=1`

Expected: `FAIL`; renderer is undefined.

- [ ] **Step 3: Extract an exact source identifier**

```go
func contextIdentifier(candidate sourceCandidate) string {
	value := strings.TrimSpace(candidate.Name)
	if candidate.Kind == "route" || candidate.Kind == "api_endpoint" || value == "" {
		value = strings.TrimSpace(candidate.Qualified)
	}
	separatorIndex, separatorWidth := -1, 0
	for _, separator := range []string{".", "#", "::"} {
		if index := strings.LastIndex(value, separator); index > separatorIndex {
			separatorIndex, separatorWidth = index, len(separator)
		}
	}
	if separatorIndex >= 0 {
		value = value[separatorIndex+separatorWidth:]
	}
	if index := strings.IndexAny(value, "( "); index >= 0 {
		value = value[:index]
	}
	return strings.Trim(value, "`*&#")
}
```

Route and API-endpoint facts use their handler/qualified name, not their URL, as source identifier. Other facts prefer the indexed `Name`, which avoids treating module-qualified JavaScript and TypeScript names as literal source tokens.

- [ ] **Step 4: Implement conservative relocation**

An identifier occurrence is whole-token only: surrounding runes are not letters, digits, `_`, or `$`. An occurrence is eligible only when `declarationLikeOccurrence` recognizes a declaration, never merely a call. The helper must:

- accept explicit declaration keywords such as `class`, `interface`, `record`, `enum`, `type`, `func`, `function`, `def`, `fn`, and `fun`;
- accept Java/Kotlin/C#/Go-style callable declarations when non-control declaration text precedes the identifier;
- reject empty-prefix calls and prefixes containing assignment operators, member-access operators, `return`, `throw`, `if`, `for`, `while`, `switch`, or `case`;
- remain language-neutral beyond these lexical checks and prefer false-negative omissions over emitting the wrong source.

Keep the clamped indexed range only when it contains exactly one declaration-like occurrence. Otherwise relocate only when the entire file contains exactly one declaration-like occurrence. If the identifier exists only in calls, return `indexed symbol has no unique declaration-like occurrence`. If multiple declaration-like occurrences exist, return `indexed symbol is ambiguous in current source`. Missing `EndLine` uses declaration plus 28 lines.

- [ ] **Step 5: Implement rendering modes**

- `body`: the complete verified range when it is at most 120 lines; a larger range makes this mode unavailable rather than labeling a partial range as `body`.
- `focused`: declaration plus 28 lines before and 32 after, maximum 61 lines.
- `signature`: contiguous annotations above the declaration plus the complete declaration through its first `{`, `:`, `;`, or `=>`, maximum 12 lines. If no declaration terminator is present within 12 lines, this mode is unavailable.

Render with `strconv.Itoa(lineNumber) + "\t" + sourceLine`; do not add Markdown fences.

- [ ] **Step 6: Verify and commit**

Run: `go test ./internal/agent -count=1`

Expected: `PASS` with deterministic ranges and content.

```bash
git add internal/agent/context_source.go internal/agent/context_source_test.go
git commit -m "Render verified context source ranges" -m "- Validate declaration-like indexed and relocated symbol occurrences.
- Render bounded body, focused, and signature views with line numbers."
```

### Task 5: Attach source for the central flow

**Files:** `internal/agent/context.go`, `internal/agent/context_rank.go`, `internal/agent/context_source.go`, `internal/agent/context_test.go`, `internal/agent/context_size_test.go`, `internal/agent/context_workspace_test.go`

**Produces:** deterministic private selected-fact tracking, `contextSourceCandidates`, and `attachContextSource`.

- [ ] **Step 1: Write an end-to-end project test**

Create controller, service, repository, and test files matching a context-index fixture. Call `BuildContext` for an exact DELETE route and require at least the entrypoint and first call-chain body:

```go
if pack.FallbackRequired || len(pack.SourceSections) < 2 {
	t.Fatalf("source-backed pack = %#v", pack)
}
if pack.SourceSections[0].Role != "entrypoint" || pack.SourceSections[0].RenderMode != "body" {
	t.Fatalf("first section = %#v", pack.SourceSections[0])
}
if !strings.Contains(pack.SourceSections[0].Content, "deleteFromCadaster") ||
	!strings.Contains(pack.SourceSections[1].Content, "removeRegulation") {
	t.Fatalf("central source path is missing: %#v", pack.SourceSections)
}
```

- [ ] **Step 2: Add workspace/fallback/test cases**

Prove: two workspace project roots resolve; fallback and low confidence contain no sections and report `source_coverage: "none"`; production queries do not include test bodies; test queries may include one test body after production; missing/unreadable/non-UTF-8 source becomes an omission; more candidates than the section and omission-detail caps produce `source_coverage: "partial"` and an exact positive `source_unrepresented`; complete central paths report `source_coverage: "complete"`; private selected fact IDs never appear in JSON; reversed fact/edge order yields byte-identical JSON.

- [ ] **Step 3: Verify failure**

Run: `go test ./internal/agent -run 'TestBuildContext.*Source|TestWorkspaceContext.*Source' -count=1`

Expected: `FAIL`; no source is attached.

- [ ] **Step 4: Carry selected fact IDs without reconstructing merged files**

The compiler already maintains `includedFactIDs`. Before the successful non-fallback return, copy its keys into `pack.selectedSourceFactIDs` and sort them. Do not derive candidates from `pack.Files`: files are merged by project/path and may contain combined ranges and roles.

```go
func retainSelectedSourceFactIDs(pack *ContextPack, included map[string]bool) {
	pack.selectedSourceFactIDs = pack.selectedSourceFactIDs[:0]
	for id := range included {
		pack.selectedSourceFactIDs = append(pack.selectedSourceFactIDs, id)
	}
	sort.Strings(pack.selectedSourceFactIDs)
}
```

Update `cloneContextPack` with:

```go
pack.selectedSourceFactIDs = append([]string(nil), pack.selectedSourceFactIDs...)
```

`contextSourceCandidates` looks up these exact IDs in the index. It assigns `entrypoint` to facts represented by `Entrypoints` or the selected `Endpoint`, preserves `contract`, `persistence`, and explicitly requested `test` roles, and assigns remaining production selections to `call_chain`. Ignore facts with empty or generated-metadata file paths. Sort and deduplicate using:

```go
var contextSourceRolePriority = map[string]int{
	"entrypoint":  0,
	"call_chain":  1,
	"contract":    2,
	"persistence": 3,
	"test":        4,
}
```

Then project, path, start line, end line, fact ID. Merge overlapping ranges in one file and retain the stronger role plus the stronger candidate's `Kind`, `Name`, and `Qualified`. Add a regression test where two selected facts in one file were merged into one `ContextFile`; both exact selected fact IDs must still participate in deterministic candidate construction.

- [ ] **Step 5: Attach with graceful fallback**

Before attempting content, set `SourceCoverage` to `complete` and `SourceUnrepresented` to the number of selected candidates. This reserves the longest coverage value and the full count in every subsequent budget check. Attempt `body`, then `focused`, then `signature`. Clone the pack, append one section, decrement `SourceUnrepresented`, recompute tokens, marshal, and accept only when:

```go
candidate.EstimatedTokens <= request.BudgetTokens
len(serializedCandidate) <= contextByteBudget(request.BudgetTokens)
```

If none fit, append one bounded omission when it fits and decrement `SourceUnrepresented`. Stop adding sections after four and detailed omissions after three, but continue counting all remaining candidates as unrepresented. Operational file errors use only the stable omission reasons; invalid request/index/serialization errors remain command errors.

After all candidates:

```go
switch {
case len(pack.SourceSections) == 0:
	pack.SourceCoverage = "none"
case uncoveredCandidates > 0:
	pack.SourceCoverage = "partial"
default:
	pack.SourceCoverage = "complete"
}
```

`uncoveredCandidates` counts both detailed omissions and unrepresented candidates. Recompute and revalidate the final token and byte ceilings. Because the initial reservation used `complete` and the maximum `SourceUnrepresented`, finalization may only shrink the serialized envelope.

- [ ] **Step 6: Update `BuildContext` orchestration**

```go
func BuildContext(request ContextRequest) (ContextPack, error) {
	request, err := normalizeContextRequest(request)
	if err != nil {
		return ContextPack{}, err
	}
	loaded, err := loadContextIndex(request)
	if err != nil {
		return ContextPack{}, err
	}
	metadataRequest := request
	metadataRequest.BudgetTokens = contextMetadataBudget(request.BudgetTokens)
	pack, err := compileContextPack(loaded.Index, metadataRequest)
	if err != nil {
		return ContextPack{}, err
	}
	pack.BudgetTokens = request.BudgetTokens
	if pack.FallbackRequired || pack.Confidence == "LOW" {
		pack.SourceCoverage = "none"
		return finalizeContextEstimate(pack)
	}
	pack, err = attachContextSource(pack, loaded, request)
	if err != nil {
		return ContextPack{}, err
	}
	return finalizeContextEstimate(pack)
}
```

- [ ] **Step 7: Update size/determinism coverage**

Require real source, one to four sections, zero to three detailed omissions, an exact non-negative unrepresented count, valid coverage state, tokens within request, bytes within `contextByteBudget`, and byte-identical repeated JSON. Marshal the pack and assert that no selected fact ID or private-field spelling is present.

- [ ] **Step 8: Verify and commit**

Run: `go test ./internal/agent -count=1`

Expected: `PASS`; default packs contain useful current source under both ceilings.

```bash
git add internal/agent/context.go internal/agent/context_rank.go internal/agent/context_source.go internal/agent/context_test.go internal/agent/context_size_test.go internal/agent/context_workspace_test.go
git commit -m "Include source in task context" -m "- Carry selected fact IDs into deterministic source candidates.
- Attach bounded source sections with explicit coverage and omission state."
```

### Task 6: Force exact query anchors before broad matches

**Files:** `internal/agent/context_rank.go`, `internal/agent/context_test.go`

**Produces:** `contextQueryAnchors(query string) []string` and exact embedded seed scores.

- [ ] **Step 1: Write failing ranking tests**

Embed each anchor in a long task with a broad high-scoring distractor:

```text
DELETE /cadasters/{cadasterId}/regulations/{objectId}
CadasterRegulationOperationsService.removeRegulation
src/main/java/example/CadasterRegulationController.java
`removeRegulation`
```

Require the anchored route, qualified symbol, file, or name to be the first production seed.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/agent -run TestBuildContextRanksEmbeddedExact -count=1`

Expected: `FAIL`; current exact scoring generally requires the whole normalized query to equal the fact.

- [ ] **Step 3: Extract at most eight anchors**

In first-appearance order, extract HTTP method plus `/path`, tokens with supported source extensions, qualified identifiers containing `.`, `#`, or `::`, and backtick-delimited values. Cap each at 256 runes. Do not add fuzzy synonyms.

- [ ] **Step 4: Match anchors before lexical scores**

Compare normalized anchors with method/path, `Qualified`, `File`, and `Name`. Give embedded exact matches a score above `scoreExactRoute` and reasons `embedded exact route`, `embedded exact qualified name`, `embedded exact file`, or `embedded exact name`. Existing production/test filtering remains authoritative.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/agent -count=1`

Expected: `PASS`; long tasks select explicitly named code without a search retry.

```bash
git add internal/agent/context_rank.go internal/agent/context_test.go
git commit -m "Prioritize exact context anchors" -m "- Extract bounded route, symbol, and file anchors from long tasks.
- Rank explicit anchors ahead of broad lexical matches."
```

### Task 7: Lock the source-bearing MCP contract

**Files:** `internal/mcp/mcp.go`, `internal/mcp/mcp_test.go`

- [ ] **Step 1: Extend the MCP fixture**

Create the indexed controller/service source under the temporary root with matching ranges.

- [ ] **Step 2: Require useful compact source JSON**

Extend `TestMCPTaskContextPassesBudgetsToCompilerAsBareCompactJSON`:

```go
if len(pack.SourceSections) == 0 || !strings.Contains(pack.SourceSections[0].Content, "deleteUser") {
	t.Fatalf("MCP source is not useful: %#v", pack.SourceSections)
}
if pack.SourceCoverage != "complete" || pack.SourceUnrepresented != 0 {
	t.Fatalf("MCP source coverage = %q / %d", pack.SourceCoverage, pack.SourceUnrepresented)
}
if strings.Contains(text, "```") {
	t.Fatalf("compact JSON contains Markdown fences: %s", text)
}
```

- [ ] **Step 3: Share bounds with the agent package**

Construct and validate MCP schema bounds using exported agent constants so package values cannot drift.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/mcp ./internal/query ./internal/cli -count=1`

Expected: `PASS`; CLI and MCP serialize the same additive pack.

```bash
git add internal/mcp/mcp.go internal/mcp/mcp_test.go
git commit -m "Lock the source-backed MCP contract" -m "- Verify compact source-bearing task_context responses.
- Share request bounds with the agent package to prevent drift."
```

### Task 8: Update guidance and benchmark instructions

**Files:** `README.md`, `COMMANDS.md`, `docs/OUTPUTS.md`, `docs/BENCHMARKING.md`, `docs/RELEASE.md`, `docs_test.go`, `release_files_test.go`, `scripts/benchmark-agent-context.sh`, `scripts/benchmark-agent-context_test.sh`

- [ ] **Step 1: Replace the assisted instruction everywhere**

```text
Call goregraph context once with the complete task before reading indexed source.
Treat source_sections as current source already read; do not re-read or grep included ranges.
If source_coverage is complete, continue from the included source without another navigation read.
If source_coverage is partial or none, read only relevant uncovered ranges named by source_omissions or files not represented by source_sections.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
At most one narrower retry may use an exact route, qualified symbol, or file returned by the first call; never use a call-chain label.
Do not use specialist GoreGraph queries or expert MCP tools.
```

The harness rejects content differences while tolerating one final newline.

- [ ] **Step 2: Update the harness unit test**

Change the fake-agent assisted marker to `Treat source_sections as current source already read`. Preserve alternating order `baabba`, token parsing, injection safety, and the 80% gate tests.

- [ ] **Step 3: Verify shell behavior**

Run:

```bash
bash -n scripts/benchmark-agent-context.sh
bash -n scripts/benchmark-agent-context_test.sh
bash scripts/benchmark-agent-context_test.sh
```

Expected final line: `PASS: benchmark-agent-context harness`.

- [ ] **Step 4: Document behavior**

State that source sections replace reads of included ranges; `source_coverage` is authoritative; omissions are the normal reason to read afterward; `source_unrepresented` reports bounded omission detail; uncovered entries in `files` remain navigation metadata when coverage is partial or none; complete-session tokens are the target; and GoreGraph remains offline, explicit, dependency-free, and watcher-free. Update the 1.3.0 release contract and its documentation tests while preserving every statement that the tag, GitHub Release, Homebrew, Scoop, and Winget publication remain pending.

- [ ] **Step 5: Verify and commit**

Run: `go test ./...`

Expected: `PASS` for packages and documentation contract tests.

```bash
git add README.md COMMANDS.md docs/OUTPUTS.md docs/BENCHMARKING.md docs/RELEASE.md docs_test.go release_files_test.go scripts/benchmark-agent-context.sh scripts/benchmark-agent-context_test.sh
git commit -m "Define the source-backed agent workflow" -m "- Align user guidance and benchmark instructions with source coverage.
- Document when uncovered source may be read after task_context."
```

### Task 9: Prove savings before expanding scope

**Files:** modify `docs/BENCHMARKING.md` only when a measured contract changes. Do not commit proprietary prompts, paths, transcripts, tokens, or source.

- [ ] **Step 1: Validate the mandatory metadata-only control**

Load the three trials captured at the checkpoint after Task 2. Verify that all provenance fields are present and that the indexed-workspace commit, index digest, task-prompt digest, Codex version, model, reasoning, sandbox, and approvals match the source-backed treatment. The GoreGraph control commit and binary must be the recorded Task 2 artifacts; the treatment uses the final source-backed commit and binary. If the workspace or prompt digest differs, the comparison is invalid and the release gate remains blocked until a new matched control is captured from the Task 2 artifact.

Expected: three valid metadata-only assisted transcripts, one signed rubric, and an explicit control-versus-treatment provenance table stored outside the repository.

- [ ] **Step 2: Run the paired source-backed benchmark**

Use the same indexed workspace snapshot, prompt, Codex version, model, reasoning, sandbox, approvals, and three-run alternation. Record the distinct GoreGraph commits and binaries. Only the documented assisted instruction and GoreGraph implementation may differ between metadata-only control and source-backed treatment; the paired baseline uses no GoreGraph assistance.

Expected: three baseline and three assisted transcripts plus `summary.tsv`.

- [ ] **Step 3: Apply all gates**

```text
source-backed assisted median <= 0.80 * paired baseline median
source-backed assisted median <= 0.85 * metadata-only assisted median
source-backed assisted quality median >= paired baseline quality median
source-backed assisted quality median >= metadata-only assisted quality median
every assisted run calls task_context no more than twice
no assisted run re-reads or greps an included source range
```

Round thresholds down to whole tokens. A failed gate blocks the source-backed MCP claim, not unrelated dashboard functionality.

- [ ] **Step 4: Classify avoidable follow-up calls**

Use exactly one category per call:

```text
missing entrypoint body
missing call-chain body
section too narrow
wrong ranked seed
stale or ambiguous location
agent ignored server instructions
question required non-source evidence
```

Tune only the dominant measured category and only one dimension per cycle: instructions, ranking, section count, window size, or total budget.

- [ ] **Step 5: Run final verification**

```bash
gofmt -w internal/agent/context.go internal/agent/context_load.go internal/agent/context_rank.go internal/agent/context_source.go internal/agent/context_test.go internal/agent/context_size_test.go internal/agent/context_workspace_test.go internal/agent/context_source_test.go internal/mcp/mcp.go internal/mcp/mcp_test.go
go test ./...
git diff --check
git status --short
```

Expected: tests pass; `git diff --check` is silent; status contains only intentional files if changes are not committed.

---

## Delivery Gates

1. **Guidance:** Tasks 1-2 are independently measurable. Benchmark once after Task 1 to isolate instruction effects.
2. **Metadata-only control:** Complete the mandatory checkpoint after Task 2. Source implementation must not begin without its three matched control runs.
3. **Minimum viable source path:** Tasks 3-5 are the central deliverable. Benchmark before adding ranking work.
4. **Retrieval hardening:** Tasks 6-8 improve long-task selection and contract consistency without new tools or stores.
5. **Release decision:** Task 9 determines whether the behavior is a real end-to-end optimization. Do not market pack-size or single-run percentages.

## Explicit Non-Goals

- Tree-sitter, Java compiler APIs, SQLite, FTS, daemons, or watchers.
- Broad new language/framework support.
- Automatic MCP installation or edits to Codex, Claude, Cursor, or IDE configuration.
- Telemetry or background update checks.
- Removing evidence, confidence, uncertainty, diagnostics, or workspace contracts.
- Returning whole files when a selected method or focused range is sufficient.
- Copying CodeGraph payload ceilings without Codex-host measurements.

## Follow-Up Only After the Gates Pass

1. Store one hash per unique indexed file if relocation omissions dominate failures.
2. Add explicit foreground `goregraph update --changed` if stale indexes remain common; do not introduce a watcher first.
3. Add Java/Spring-specific body boundaries if current `EndLine` data is measurably insufficient.
4. Collapse interface-family siblings to signatures if repeated implementations consume material payload.
5. Evaluate a compact persistent search index only if warm latency, rather than tokens, becomes limiting.

## Self-Review Checklist

- [ ] Every behavior uses GoreGraph's existing index and trust model.
- [ ] Every production change starts with a focused failing test.
- [ ] Standard MCP still lists only `task_context`.
- [ ] Source cannot escape its project root through traversal or symlinks.
- [ ] Nested configured output directories preserve the exact project root.
- [ ] Fallback and low-confidence packs contain no source bodies.
- [ ] Included source is valid current UTF-8 disk content and never emitted for absent, call-only, or ambiguous identifiers.
- [ ] JavaScript/TypeScript `#`, PHP-style `::`, and dotted qualified names resolve to the indexed fact name.
- [ ] Selected source facts are carried privately and never reconstructed from merged `files`.
- [ ] `source_coverage` and `source_unrepresented` account for every selected source candidate.
- [ ] Default output stays below 4,000 estimated tokens and 16,000 bytes.
- [ ] Every selected location retains at most one original evidence ID, and unused capacity is not filled with more.
- [ ] Exact routes, symbols, files, and backtick anchors outrank broad distractors.
- [ ] Tests remain supporting context unless explicitly requested.
- [ ] Documentation and benchmark instructions use the same exact behavior.
- [ ] Release claims use the pre-source metadata control, matched end-to-end medians, recorded distinct GoreGraph binaries, and equal-or-better evidence quality.
