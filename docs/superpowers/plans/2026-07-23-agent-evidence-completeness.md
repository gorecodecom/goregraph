# Agent Evidence Completeness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `source_coverage: complete` mean that every requested cross-cutting concern is backed by actionable rendered source, prefer those actionable variants within the existing budget, and prevent agents from re-reading indexed source after a complete Context Pack.

**Architecture:** Keep GoreGraph's deterministic concern planning and bounded source-option pipeline. Tighten concern attribution at the point where verified source is already rendered, then make mandatory fact selection prefer the smallest option that adds required evidence before falling back to a signature. Synchronize one stricter navigation contract across the generated guide, CLI, MCP, documentation, and benchmark assertions.

**Tech Stack:** Go 1.26, standard library, table-driven Go tests, shell contract tests, existing deterministic Context Pack fixtures.

## Global Constraints

- Keep the default Context Pack budget at exactly 4,000 tokens and the default file limit at exactly 12.
- Preserve deterministic ordering and all existing byte, token, file, section, and omission limits.
- A Fact ID alone must not mark authentication, configuration, resilience, persistence, side effects, or tests covered when its rendered source does not contain evidence for that concern.
- A rendered test signature or annotation without an executable statement must not satisfy the tests concern.
- Prefer actionable evidence only when it fits the requested budget; otherwise retain honest partial coverage and an exact omission.
- Do not hard-code benchmark repository, class, method, route, or property names.
- Do not add dependencies.
- Keep all implementation work on `main`, as previously authorized after the safety push.
- Do not release.

---

### Task 1: Require actionable rendered concern evidence

**Files:**

- Modify: `internal/agent/context_select.go`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Consumes: `contextSourceOptionConcerns`, `contextSourceSectionSupportsConcern`, `ContextSourceSection.RenderMode`
- Produces: `contextSourceSectionHasExecutableTest(ContextSourceSection, string) bool`

- [ ] **Step 1: Add failing exact-Fact coverage tests**

Add tests showing that an exact configuration Fact ID rendered only as `public class JobConfig {` does not cover configuration, while a declaration body containing `configuration.getJobsPath()` does:

Add `"reflect"` to the existing test imports.

```go
func TestContextSourceOptionConcernsRequireRenderedCrossCuttingEvidence(t *testing.T) {
	concern := newContextConcern(
		contextConcernConfiguration,
		"libraries/jobs",
		true,
		[]string{"config"},
		"requested configuration",
	)
	candidate := sourceCandidate{
		FactID: "config", FactIDs: []string{"config"},
		Project: "libraries/jobs", Role: "call_chain",
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "config", Project: "libraries/jobs", Kind: contextConcernConfiguration,
	}}}

	signature := ContextSourceSection{
		Project: "libraries/jobs", RenderMode: "signature",
		Content: "public class JobConfig {",
	}
	if keys, _ := contextSourceOptionConcerns(candidate, signature, []contextConcern{concern}, index); len(keys) != 0 {
		t.Fatalf("type-only exact fact covered configuration: %v", keys)
	}

	body := ContextSourceSection{
		Project: "libraries/jobs", RenderMode: "declaration_body",
		Content: "String path = configuration.getJobsPath();",
	}
	if keys, required := contextSourceOptionConcerns(candidate, body, []contextConcern{concern}, index); !required || !reflect.DeepEqual(keys, []string{concern.key}) {
		t.Fatalf("actionable configuration evidence = %v, required %v", keys, required)
	}
}
```

- [ ] **Step 2: Add failing executable-test tests**

Add a table-driven test proving that a `signature` containing only `@Test` and the method declaration is rejected, while a `declaration_body` containing a real assertion or request statement is accepted:

```go
func TestContextSourceTestsRequireExecutableRenderedBody(t *testing.T) {
	concern := newContextConcern(contextConcernTests, "services/jobs", true, nil, "")
	signature := ContextSourceSection{
		Project: "services/jobs", Role: "test", RenderMode: "signature",
		Content: "@Test\nvoid deletesJob() {",
	}
	if contextSourceSectionSupportsConcern(signature, concern) {
		t.Fatal("test signature counted as executable evidence")
	}

	body := ContextSourceSection{
		Project: "services/jobs", Role: "test", RenderMode: "declaration_body",
		Content: "@Test\nvoid deletesJob() {\n  mockMvc.perform(delete(\"/jobs/1\")).andExpect(status().isNoContent());\n}",
	}
	if !contextSourceSectionSupportsConcern(body, concern) {
		t.Fatal("executable test body was rejected")
	}
}
```

- [ ] **Step 3: Run the focused tests and verify RED**

Run:

```bash
go test ./internal/agent -run 'TestContextSourceOptionConcernsRequireRenderedCrossCuttingEvidence|TestContextSourceTestsRequireExecutableRenderedBody' -count=1
```

Expected: both tests fail because exact Fact IDs bypass rendered semantics and `@Test` alone currently counts as coverage.

- [ ] **Step 4: Require rendered semantics for every cross-cutting concern**

In `contextSourceOptionConcerns`, replace the exact-kind exception with an unconditional rendered-source check for cross-cutting concerns:

```go
if covered && contextSourceCrossCuttingFamily(concern.kind) {
	covered = contextSourceSectionSupportsConcern(section, concern)
}
```

Keep the existing project equality check and the test-role exclusion.

- [ ] **Step 5: Require an executable statement for test coverage**

Change the tests branch to require both a test marker and an executable statement from a non-signature render:

```go
case contextConcernTests:
	return contextSourceContainsAny(content, "@test", "describe(", "it(", "test(") &&
		contextSourceSectionHasExecutableTest(section, semanticContent)
```

Implement the helper without parsing a specific language:

```go
func contextSourceSectionHasExecutableTest(section ContextSourceSection, content string) bool {
	if section.RenderMode == "signature" {
		return false
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "@") ||
			line == "{" || line == "}" || strings.HasSuffix(line, "{") {
			continue
		}
		if strings.HasSuffix(line, ";") ||
			strings.HasPrefix(line, "assert ") ||
			strings.Contains(line, "assert(") ||
			strings.Contains(line, "expect(") ||
			strings.Contains(line, ".andexpect(") ||
			strings.HasPrefix(line, "await ") {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: Run focused and package tests and verify GREEN**

Run:

```bash
go test ./internal/agent -run 'TestContextSourceOptionConcerns|TestContextSourceSection|TestContextSourceTests' -count=1
go test ./internal/agent -count=1
```

Expected: all selected tests and all `internal/agent` tests pass.

- [ ] **Step 7: Commit the task**

Commit with:

```text
Require rendered concern evidence

- Validate cross-cutting coverage against the selected source section
- Reject annotation-only test signatures without executable statements
```

---

### Task 2: Prefer actionable fact-bound source variants

**Files:**

- Modify: `internal/agent/context_select.go`
- Test: `internal/agent/context_source_test.go`
- Test: `internal/agent/context_change_analysis_test.go`

**Interfaces:**

- Consumes: `smallestFittingContextSourceOption`, `contextConcern.candidateFactIDs`, `contextSourceOption.concernKeys`
- Produces: `contextSourceBoundaryConcernGain(contextSourceOption, contextSourceBoundary, []contextConcern, contextSourceSelectionState) int`

- [ ] **Step 1: Add a failing boundary-selection test**

Create a test with two fitting options for the same mandatory configuration fact: a cheaper signature with no concern key and a slightly larger declaration body with the required configuration key. Assert that the declaration body wins:

```go
func TestSmallestFittingContextSourceOptionPrefersActionableFactEvidence(t *testing.T) {
	concern := newContextConcern(
		contextConcernConfiguration,
		"libraries/jobs",
		true,
		[]string{"config"},
		"requested configuration",
	)
	candidate := sourceCandidate{
		FactID: "config", FactIDs: []string{"config"},
		Project: "libraries/jobs", Path: "JobConfig.java",
	}
	signature := contextSourceOption{
		candidate: candidate,
		section: ContextSourceSection{Project: "libraries/jobs", Path: "JobConfig.java", RenderMode: "signature", Content: "class JobConfig {"},
		estimated: 10, projectKey: "libraries/jobs",
	}
	body := contextSourceOption{
		candidate: candidate,
		section: ContextSourceSection{Project: "libraries/jobs", Path: "JobConfig.java", RenderMode: "declaration_body", Content: "String path = configuration.getJobsPath();"},
		estimated: 30, projectKey: "libraries/jobs",
		concernKeys: []string{concern.key}, required: true,
	}
	state := contextSourceSelectionState{
		selectedCandidates:       map[string]bool{},
		selectedFactIDs:          map[string]bool{},
		selectedProjects:         map[string]bool{},
		coveredConcerns:          map[string]bool{},
		coveredRoles:             map[string]bool{},
		selectedEvidenceFamilies: map[string]int{},
	}
	got, found, err := smallestFittingContextSourceOption(
		ContextPack{Schema: 1, Query: "jobs configuration", BudgetTokens: DefaultContextBudgetTokens},
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens, MaxFiles: DefaultContextMaxFiles},
		[]contextSourceOption{signature, body},
		[]contextConcern{concern},
		state,
		contextSourceBoundary{factID: "config"},
	)
	if err != nil || !found || got.section.RenderMode != "declaration_body" {
		t.Fatalf("mandatory actionable option = %#v, found %v, err %v", got, found, err)
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
go test ./internal/agent -run TestSmallestFittingContextSourceOptionPrefersActionableFactEvidence -count=1
```

Expected: FAIL because the current selector chooses the ten-token signature.

- [ ] **Step 3: Rank new required concern evidence before byte cost**

Implement:

```go
func contextSourceBoundaryConcernGain(
	option contextSourceOption,
	boundary contextSourceBoundary,
	concerns []contextConcern,
	state contextSourceSelectionState,
) int {
	if boundary.factID == "" {
		return 0
	}
	optionKeys := make(map[string]bool, len(option.concernKeys))
	for _, key := range option.concernKeys {
		optionKeys[key] = true
	}
	gain := 0
	for _, concern := range concerns {
		if !concern.required || state.coveredConcerns[concern.key] ||
			!optionKeys[concern.key] ||
			!slices.Contains(concern.candidateFactIDs, boundary.factID) {
			continue
		}
		gain++
	}
	return gain
}
```

Add the standard-library `slices` import. In `smallestFittingContextSourceOption`, compare `contextSourceBoundaryConcernGain` before estimated tokens for fact boundaries. Preserve `betterContextProjectBoundaryOption` for project-only boundaries and preserve deterministic tie-breaking.

- [ ] **Step 4: Add an integration assertion against false completeness**

Extend the missing-contract fixture test so every covered cross-cutting public concern has at least one same-project rendered section for which `contextSourceSectionSupportsConcern` returns true. Also assert that every selected test section has `RenderMode != "signature"` and contains an executable statement.

```go
for _, concern := range pack.Concerns {
	if !concern.Covered || !contextSourceCrossCuttingFamily(concern.Kind) {
		continue
	}
	supported := false
	internalConcern := newContextConcern(
		concern.Kind,
		normalizeContextProject(concern.Project),
		true,
		nil,
		concern.Reason,
	)
	for _, section := range pack.SourceSections {
		if contextSourceSectionSupportsConcern(section, internalConcern) {
			supported = true
			break
		}
	}
	if !supported {
		t.Errorf("covered concern lacks actionable rendered source: %#v", concern)
	}
}
```

- [ ] **Step 5: Run focused and package tests and verify GREEN**

Run:

```bash
go test ./internal/agent -run 'TestSmallestFittingContextSourceOptionPrefersActionableFactEvidence|TestBuildContextSupportsMissingContractChangeAnalysis' -count=1
go test ./internal/agent -count=1
```

Expected: all tests pass within the unchanged 4,000-token and 12-file limits.

- [ ] **Step 6: Commit the task**

Commit with:

```text
Prefer actionable source variants

- Rank required rendered concern gains ahead of compact signatures
- Verify complete packs contain same-project operational evidence
```

---

### Task 3: Make the no-re-read guide operationally explicit

**Files:**

- Modify: `internal/scan/agent_reports.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/mcp/mcp.go`
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/RELEASE.md`
- Test: `internal/scan/agent_reports_test.go`
- Test: `internal/cli/cli_test.go`
- Test: `internal/mcp/mcp_test.go`
- Test: `docs_test.go`
- Test: `scripts/benchmark-agent-context_test.sh`

**Interfaces:**

- Consumes: generated Agent Guide, CLI `context --help`, MCP server instructions
- Produces: one identical source-reuse policy across every user-facing surface

- [ ] **Step 1: Change the contract tests first**

Replace the existing three source-reuse lines in the test constants with:

```text
Call goregraph context once with the complete task before reading indexed source.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path entries listed in source_omissions; do not inspect other files or widen ranges. Report pathless omissions as uncertainty.
```

Update the benchmark shell assertions to require `run no source-reading commands` and `mark details absent from them as unknown`.

- [ ] **Step 2: Run contract tests and verify RED**

Run:

```bash
go test ./internal/scan ./internal/cli ./internal/mcp . -run 'AgentGuide|ContextHelp|Instructions|Documentation' -count=1
bash scripts/benchmark-agent-context_test.sh
```

Expected: failures show the old guide text is still emitted.

- [ ] **Step 3: Update every production and documentation copy**

Apply the exact four-line policy verbatim to:

- `internal/scan/agent_reports.go`
- `internal/cli/cli.go`
- `internal/mcp/mcp.go`
- `README.md`
- `COMMANDS.md`
- `docs/BENCHMARKING.md`
- `docs/OUTPUTS.md`
- `docs/RELEASE.md`

Keep the retry, fallback, doctor, and generated-output restrictions unchanged.

- [ ] **Step 4: Run guide contract tests and verify GREEN**

Run:

```bash
go test ./internal/scan ./internal/cli ./internal/mcp . -count=1
bash scripts/benchmark-agent-context_test.sh
```

Expected: all tests pass and all surfaces contain the identical stricter policy.

- [ ] **Step 5: Commit the task**

Commit with:

```text
Forbid source re-reads after complete context

- Make complete Context Packs the sole indexed-source input
- Require unknown reporting instead of widening or repeating source reads
```

---

### Task 4: Verify, install, and prepare the benchmark workspace

**Files:**

- Verify: all Go packages
- Verify: `scripts/analyze-agent-context-log_test.sh`
- Verify: `scripts/benchmark-agent-context_test.sh`
- Verify: `/Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix`

**Interfaces:**

- Consumes: verified repository HEAD
- Produces: locally installed `goregraph` and a clean, freshly indexed three-project benchmark workspace

- [ ] **Step 1: Format and run the full product verification**

Run:

```bash
gofmt -w internal/agent/context_select.go internal/agent/context_source_test.go internal/agent/context_change_analysis_test.go
go test ./... -count=1
go vet ./...
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context_test.sh
git diff --check
```

Expected: every command exits zero with no test failures or vet findings.

- [ ] **Step 2: Generate the unchanged benchmark query before installation**

Run the repository CLI against the prepared index with exactly:

```bash
go run ./cmd/goregraph context /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --query 'Historische, ausschließlich lesende Ursachenanalyse über ms-cadasterregulation, ms-cadastertask und ms-common: Beim Entfernen einer Vorschrift aus einem Kataster bleiben verbundene Aufgaben bestehen. Ermittle den öffentlichen REST-Endpunkt, die aktuelle Aufrufkette und Ursache, die minimal notwendige produktionsreife neue Aufrufkette über alle drei Repositories, betroffene Aufgabenarten und Suchattribute, internen API-Vertrag, Authentifizierung und Konfiguration, Persistenzoperationen, Nebenwirkungen für Protokollierung/E-Mail/Benutzerinformationen, zu ändernde oder anzulegende Produktions- und Testdateien sowie Fehlerbehandlung, Retry-Logik und erforderliche Tests. Nicht implementieren.' \
  --budget-tokens 4000 \
  --max-files 12 \
  --format json
```

Verify:

- `estimated_tokens <= 4000`
- `len(source_sections) <= 12`
- every covered cross-cutting concern has matching rendered source
- a test concern is covered only by a non-signature section with an executable statement
- repeated runs return the same Context ID

- [ ] **Step 3: Run an independent whole-change review**

Review the complete diff from the pre-plan commit through HEAD for:

- false `complete` coverage
- budget regressions
- language-specific assumptions
- inconsistent guide copies
- missing RED/GREEN evidence

Resolve every Critical or Important finding and re-run its focused tests.

- [ ] **Step 4: Install the verified CLI locally**

Run the repository's documented local install command and verify:

```bash
/Users/gorecode/go/bin/goregraph version
```

Expected: GoreGraph `1.3.0`, Go `1.26.5`, `darwin/arm64`, schema `3`.

- [ ] **Step 5: Preview and execute the benchmark cleanup**

Preview:

```bash
/Users/gorecode/go/bin/goregraph workspace clean \
  /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --workspace /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix
```

Confirm only these generated paths are targeted:

- `ms-cadasterregulation/goregraph-out`
- `ms-cadastertask/goregraph-out`
- `ms-common/goregraph-out`
- `.goregraph-workspace`

Then execute with `--execute`.

- [ ] **Step 6: Re-scan and verify the workspace**

Run:

```bash
/Users/gorecode/go/bin/goregraph workspace scan-all \
  /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --workspace /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix

/Users/gorecode/go/bin/goregraph workspace status \
  /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --workspace /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix
```

Expected: all three projects are indexed and referenced-but-missing services are `none`.

- [ ] **Step 7: Record the completion state**

Report:

- commits created
- full verification commands and results
- benchmark Context ID, token count, coverage, and rendered test mode
- installed CLI version
- cleaned paths and freshly indexed projects
- any remaining evidence limitations
