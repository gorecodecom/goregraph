# Monotonic Agent Context Evaluation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a monotonic evaluation system that rejects GoreGraph Agent Context changes when they lose any capability demonstrated by the current Golden Build.

**Architecture:** Add a standard-library-only `internal/agentbench` package for benchmark contracts, semantic Context Pack projection, review scorecards, and strict gates. Add a separate regression command and harness that scan isolated Golden and candidate workspace copies, interleave paired Codex runs, retain every artifact, and reuse the existing transcript analyzer without changing the release benchmark.

**Tech Stack:** Go 1.26, Go standard library, existing `internal/agent`, `internal/cli`, and scan pipeline, POSIX shell, existing Codex JSONL analyzer, deterministic JSON fixtures, and external signed review scorecards.

## Global Constraints

- Treat commit `1bc4408` as the initial Golden Build even though the evaluation documentation is committed later.
- Keep the real G1 workspace, prompts, transcripts, contracts, scorecards, and proprietary identifiers outside the repository.
- Commit only generic G2-G6 workspaces, prompts, and contracts.
- Represent quality as required facets, forbidden outcomes, and explicit unknowns; never compare exact answer prose.
- Keep the existing no-GoreGraph versus GoreGraph release benchmark unchanged.
- Keep `DefaultContextBudgetTokens = 4000`, `DefaultContextMaxFiles = 12`, and `MaxContextSourceOmissions = 3`.
- Require every previously passing G1 facet in every candidate run.
- Reject every new unsupported claim, wrong entrypoint, pathless omission, broad source search, included-source reread, or unapproved retry.
- Require the declared target facet to improve deterministically and in at least two of three candidate runs.
- Do not allow median source reads or median tool calls to increase.
- Allow median end-to-end tokens to increase by at most 5 percent.
- Allow median direct Context latency to increase by at most 10 percent and reject an unexplained paired latency above 2x.
- Fix contracts and thresholds before a candidate run. Never replace a valid slow or inaccurate run with another run.
- Add no dependency, network lookup, model judge, embedding model, tokenizer, or benchmark-specific production rule.
- Use English comments, identifiers, fixture source, documentation, and commit messages. German parity query files are the only non-English committed text.
- Do not release, tag, publish, or push during implementation unless the user requests it separately.

---

## File and Interface Map

### New benchmark package

- `internal/agentbench/contract.go`: strict JSON contract, matrix, and hypothesis types plus validation.
- `internal/agentbench/contract_test.go`: schema, unknown-field, identity, and threshold validation.
- `internal/agentbench/pack.go`: public Context Pack projection, semantic expectation evaluation, and stable Pack Diff.
- `internal/agentbench/pack_test.go`: invariant, bounded-omission, deterministic-order, and allowed-change tests.
- `internal/agentbench/review.go`: signed run reviews, metrics, medians, invalid-run records, and gate reports.
- `internal/agentbench/review_test.go`: per-facet, target-improvement, efficiency, and no-cherry-picking regressions.
- `internal/agentbench/matrix_test.go`: scans copied G2-G6 workspaces and verifies all committed contracts.

### New command and harness

- `scripts/agent-context-regression/main.go`: `verify-pack`, `diff-pack`, `gate`, and `run` subcommands.
- `scripts/agent-context-regression/main_test.go`: CLI validation and stable JSON output tests.
- `scripts/benchmark-agent-context-regression.sh`: repository-root-safe wrapper around the Go command.
- `scripts/benchmark-agent-context-regression_test.sh`: fake Codex and fake GoreGraph end-to-end harness tests.

### New generic benchmark data

- `testdata/agent-context-regression/matrix.json`: committed G2-G6 matrix.
- `testdata/agent-context-regression/g2-java-missing-contract/`: Java/Spring missing-contract and competing-endpoint case.
- `testdata/agent-context-regression/g3-go-existing-flow/`: Go current-flow case.
- `testdata/agent-context-regression/g4-typescript-consumer-provider/`: TypeScript consumer/provider case.
- `testdata/agent-context-regression/g5-java-persistence-side-effects/`: Java persistence and side-effect case.
- `testdata/agent-context-regression/g6-ambiguous-entrypoint/`: ambiguity, fallback, and budget-pressure case.

Every case directory contains:

```text
contract.json
query.en.txt
query.de.txt                 # only when language parity is required
workspace/
  .goregraph-workspace.yml
  services/ or frontend/
```

### Documentation changes

- `docs/BENCHMARKING.md`: document the regression benchmark separately from the release benchmark.
- `README.md`: link to the monotonic evaluation workflow.

### Explicitly unchanged

- `scripts/benchmark-agent-context.sh`
- `scripts/benchmark-agent-context_test.sh`
- GoreGraph Context selection, ranking, scanning, and rendering production code

---

### Task 1: Define Strict Benchmark Contracts

**Files:**

- Create: `internal/agentbench/contract.go`
- Create: `internal/agentbench/contract_test.go`

**Interfaces:**

- Produces: `LoadContract(path string) (Contract, error)`
- Produces: `LoadMatrix(path string) (Matrix, error)`
- Produces: `LoadHypothesis(path string) (Hypothesis, error)`
- Produces: `ValidateContract(Contract) error`
- Produces: `ValidateMatrix(Matrix) error`
- Produces: `ValidateHypothesis(Hypothesis, Matrix) error`

- [ ] **Step 1: Write contract schema tests**

Add table-driven tests that accept one complete contract and reject unknown
fields, duplicate facet IDs, empty evidence descriptions, unsupported schema
versions, a budget other than `4000`, more than three omissions, missing query
variants, and overlapping required/forbidden location matchers.

Use this complete fixture in `contract_test.go`:

```go
func validContract() Contract {
	fallback := false
	return Contract{
		Schema: 1,
		ID:     "g2-java-missing-contract",
		Queries: []QueryVariant{
			{ID: "en", Language: "en", File: "query.en.txt", EndToEnd: true},
			{ID: "de", Language: "de", File: "query.de.txt"},
		},
		Pack: PackExpectation{
			Endpoint: &EndpointExpectation{
				HTTPMethod: "DELETE",
				Path:       "/catalog/{itemId}",
			},
			RequiredLocations: []LocationExpectation{
				{Section: "persistence", Project: "services/catalog", LabelContains: "deleteById"},
			},
			ForbiddenLocations: []LocationExpectation{
				{Section: "entrypoints", Project: "services/jobs", LabelContains: "listJobs"},
			},
			FallbackRequired:    &fallback,
			MaxEstimatedTokens:  4000,
			MaxSourceOmissions:  3,
			RequireBoundedSource: true,
		},
		Answer: AnswerExpectation{
			RequiredFacets: []FacetDefinition{
				{ID: "current-path", Description: "Explains the observed catalog deletion path."},
				{ID: "missing-contract", Description: "Marks the requested job deletion contract as absent."},
			},
			ForbiddenOutcomes: []FacetDefinition{
				{ID: "invented-edge", Description: "Claims that the missing deletion call already exists."},
			},
		},
		Limits: EfficiencyLimits{
			ContextTokens:              4000,
			MaxSourceOmissions:         3,
			MaxTokenIncreasePercent:    5,
			MaxLatencyIncreasePercent:  10,
			MaxPairedLatencyMultiplier: 2,
		},
	}
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run:

```bash
go test ./internal/agentbench -run 'TestLoadContract|TestValidateContract|TestValidateMatrix|TestValidateHypothesis' -count=1
```

Expected: FAIL because package `internal/agentbench` and its types do not exist.

- [ ] **Step 3: Implement strict schema types and loaders**

Define the public types exactly as follows:

```go
type Contract struct {
	Schema  int               `json:"schema"`
	ID      string            `json:"id"`
	Queries []QueryVariant    `json:"queries"`
	Pack    PackExpectation   `json:"pack"`
	Answer  AnswerExpectation `json:"answer"`
	Limits  EfficiencyLimits  `json:"limits"`
}

type QueryVariant struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	File     string `json:"file"`
	EndToEnd bool   `json:"end_to_end"`
}

type PackExpectation struct {
	Endpoint               *EndpointExpectation  `json:"endpoint,omitempty"`
	RequiredLocations      []LocationExpectation `json:"required_locations"`
	ForbiddenLocations     []LocationExpectation `json:"forbidden_locations"`
	RequiredSourceSuffixes []string              `json:"required_source_suffixes"`
	ForbiddenSourceSuffixes []string             `json:"forbidden_source_suffixes"`
	RequiredUnknowns       []string              `json:"required_unknowns"`
	FallbackRequired       *bool                 `json:"fallback_required,omitempty"`
	RetryAllowed           *bool                 `json:"retry_allowed,omitempty"`
	MaxEstimatedTokens     int                   `json:"max_estimated_tokens"`
	MaxSourceOmissions     int                   `json:"max_source_omissions"`
	RequireBoundedSource   bool                  `json:"require_bounded_source"`
}

type EndpointExpectation struct {
	ProviderContains string `json:"provider_contains,omitempty"`
	HTTPMethod       string `json:"http_method"`
	Path             string `json:"path"`
}

type LocationExpectation struct {
	Section       string `json:"section"`
	Project       string `json:"project,omitempty"`
	Kind          string `json:"kind,omitempty"`
	LabelContains string `json:"label_contains"`
}

type AnswerExpectation struct {
	RequiredFacets   []FacetDefinition `json:"required_facets"`
	ForbiddenOutcomes []FacetDefinition `json:"forbidden_outcomes"`
	ExplicitUnknowns []FacetDefinition `json:"explicit_unknowns"`
}

type FacetDefinition struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type EfficiencyLimits struct {
	ContextTokens              int `json:"context_tokens"`
	MaxSourceOmissions         int `json:"max_source_omissions"`
	MaxTokenIncreasePercent    int `json:"max_token_increase_percent"`
	MaxLatencyIncreasePercent  int `json:"max_latency_increase_percent"`
	MaxPairedLatencyMultiplier int `json:"max_paired_latency_multiplier"`
}

type Matrix struct {
	Schema int          `json:"schema"`
	Cases  []MatrixCase `json:"cases"`
}

type MatrixCase struct {
	ID        string `json:"id"`
	Directory string `json:"directory"`
	External  bool   `json:"external"`
}

type Hypothesis struct {
	Schema              int      `json:"schema"`
	ID                  string   `json:"id"`
	TargetFacet         string   `json:"target_facet"`
	TargetCases         []string `json:"target_cases"`
	AllowedPackChanges  []string `json:"allowed_pack_changes"`
	ProtectedPackFields []string `json:"protected_pack_fields"`
}
```

Load JSON with `json.Decoder.DisallowUnknownFields()`, require one JSON value,
reject trailing data, normalize no values, and return field-specific errors.
Require exactly one `EndToEnd` query per contract. Additional query variants
are deterministic language-parity inputs and never multiply the 36 full
end-to-end runs.
Require facet IDs to be unique across required facets, forbidden outcomes, and
explicit unknowns. `ValidateHypothesis` may target reserved external case `g1`
or a committed matrix case; the `gate` command must additionally prove that
`TargetFacet` exists in the required facets of every target case contract.
Allowed change categories are exactly:

```go
var allowedPackChangeKinds = map[string]bool{
	"locations": true,
	"sources":   true,
	"coverage":  true,
	"omissions": true,
	"budget":    true,
}
```

Protected fields are exactly `endpoint`, `entrypoints`, `call_chain`,
`contracts`, and `persistence`.

- [ ] **Step 4: Run the focused and package tests**

Run:

```bash
gofmt -w internal/agentbench/contract.go internal/agentbench/contract_test.go
go test ./internal/agentbench -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agentbench/contract.go internal/agentbench/contract_test.go
git commit -m "Define agent benchmark contracts" -m $'- Validate strict benchmark, matrix, and hypothesis schemas\n- Fix monotonic quality and efficiency limits before execution'
```

---

### Task 2: Add Semantic Context Pack Projection and Diffing

**Files:**

- Create: `internal/agentbench/pack.go`
- Create: `internal/agentbench/pack_test.go`

**Interfaces:**

- Consumes: `agent.ContextPack`
- Consumes: `PackExpectation` and `Hypothesis` from Task 1
- Produces: `ProjectPack(pack agent.ContextPack) PackProjection`
- Produces: `EvaluatePack(pack agent.ContextPack, expectation PackExpectation) []Violation`
- Produces: `DiffPacks(golden, candidate agent.ContextPack) PackDiff`
- Produces: `EvaluatePackDiff(diff PackDiff, hypothesis Hypothesis) []Violation`

- [ ] **Step 1: Write failing projection and invariant tests**

Build two in-memory packs where the candidate replaces a protected DELETE
endpoint with a richer GET endpoint, removes one persistence location, adds a
concrete configuration source, and adds a pathless omission.

Assert:

```go
diff := DiffPacks(golden, candidate)
if !diff.EndpointChanged {
	t.Fatal("endpoint change was not reported")
}
if !slices.Contains(diff.RemovedLocations, "persistence|services/catalog|deleteById") {
	t.Fatalf("removed locations = %#v", diff.RemovedLocations)
}
if !slices.Contains(diff.AddedSources, "libraries/job-client/src/JobClientConfig.java") {
	t.Fatalf("added sources = %#v", diff.AddedSources)
}
violations := EvaluatePack(candidate, validContract().Pack)
requireViolation(t, violations, "endpoint")
requireViolation(t, violations, "source_omission")
```

Add a determinism test that reverses every public Context Pack slice and
requires byte-identical JSON projections and diffs.

- [ ] **Step 2: Run the tests and verify RED**

Run:

```bash
go test ./internal/agentbench -run 'TestProjectPack|TestEvaluatePack|TestDiffPacks|TestEvaluatePackDiff' -count=1
```

Expected: FAIL because the projection and diff functions do not exist.

- [ ] **Step 3: Implement stable public projections**

Define:

```go
type PackProjection struct {
	Endpoint         string   `json:"endpoint"`
	Entrypoints      []string `json:"entrypoints"`
	CallChain        []string `json:"call_chain"`
	Contracts        []string `json:"contracts"`
	Persistence      []string `json:"persistence"`
	Tests            []string `json:"tests"`
	Sources          []string `json:"sources"`
	Omissions        []string `json:"omissions"`
	Uncertainties    []string `json:"uncertainties"`
	SourceCoverage   string   `json:"source_coverage"`
	EstimatedTokens  int      `json:"estimated_tokens"`
	FallbackRequired bool     `json:"fallback_required"`
	RetryAllowed     bool     `json:"retry_allowed"`
}

type PackDiff struct {
	EndpointChanged      bool     `json:"endpoint_changed"`
	GoldenEndpoint       string   `json:"golden_endpoint"`
	CandidateEndpoint    string   `json:"candidate_endpoint"`
	AddedLocations       []string `json:"added_locations"`
	RemovedLocations     []string `json:"removed_locations"`
	AddedSources         []string `json:"added_sources"`
	RemovedSources       []string `json:"removed_sources"`
	AddedOmissions       []string `json:"added_omissions"`
	RemovedOmissions     []string `json:"removed_omissions"`
	CoverageChanged      bool     `json:"coverage_changed"`
	BudgetChanged        bool     `json:"budget_changed"`
	FallbackChanged      bool     `json:"fallback_changed"`
	RetryChanged         bool     `json:"retry_changed"`
}

type Violation struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
```

Canonical location keys are `section|project|kind|label|file|line`. Canonical
source keys are `project|path|start_line|end_line|role|reason`. Canonical
omission keys include the same bounded range fields. Sort and deduplicate every
projection slice before comparison.

`EvaluatePackDiff` must reject changes to every protected field regardless of
the allowed change categories. It must also reject any actual change category
not listed in `AllowedPackChanges`.

- [ ] **Step 4: Run focused tests and all agent tests**

Run:

```bash
gofmt -w internal/agentbench/pack.go internal/agentbench/pack_test.go
go test ./internal/agentbench -count=1
go test ./internal/agent -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agentbench/pack.go internal/agentbench/pack_test.go
git commit -m "Add semantic Context Pack diffs" -m $'- Project public Context Pack evidence into stable semantic keys\n- Reject protected-field and undeclared candidate changes'
```

---

### Task 3: Enforce Per-Facet Quality and Efficiency Gates

**Files:**

- Create: `internal/agentbench/review.go`
- Create: `internal/agentbench/review_test.go`

**Interfaces:**

- Consumes: `Contract` from Task 1
- Produces: `LoadReview(path string) (RunReview, error)`
- Produces: `EvaluateCase(contract Contract, golden, candidate []ReviewedRun, hypothesis Hypothesis) GateReport`
- Produces: `Median(values []int64) (int64, error)`

- [ ] **Step 1: Write failing quality gate tests**

Use three Golden and three candidate runs. All candidate runs pass the existing
facets, two improve the target facet, and metrics remain inside limits. Then
mutate one condition at a time and require rejection for:

- one previously passing G1 facet failing once;
- one forbidden outcome present once;
- the target improving in only one candidate run;
- median tool calls increasing by one;
- median source reads increasing by one;
- candidate token median at 106 percent;
- candidate latency median at 111 percent;
- one paired candidate latency above 2x;
- an inaccurate run marked invalid;
- a fourth run appended after a valid failure.

The central assertion is:

```go
report := EvaluateCase(contract, golden, candidate, hypothesis)
if !report.Passed || len(report.Failures) != 0 {
	t.Fatalf("passing report = %#v", report)
}

candidate[1].Review.Facets["current-path"] = FacetResult{
	Status:   "fail",
	Evidence: "The answer selected the unrelated list endpoint.",
}
report = EvaluateCase(contract, golden, candidate, hypothesis)
requireGateFailure(t, report, "current-path failed in candidate run 2")
```

- [ ] **Step 2: Run the tests and verify RED**

Run:

```bash
go test ./internal/agentbench -run 'TestLoadReview|TestEvaluateCase|TestMedian' -count=1
```

Expected: FAIL because review and gate types do not exist.

- [ ] **Step 3: Implement strict reviews and integer gates**

Define:

```go
type RunReview struct {
	Schema            int                    `json:"schema"`
	CaseID            string                 `json:"case_id"`
	Build             string                 `json:"build"`
	Run               int                    `json:"run"`
	Attempt           int                    `json:"attempt"`
	Reviewer           string                 `json:"reviewer"`
	ReviewedAt         string                 `json:"reviewed_at"`
	Signature          string                 `json:"signature"`
	Facets             map[string]FacetResult `json:"facets"`
	ForbiddenOutcomes map[string]FacetResult `json:"forbidden_outcomes"`
}

type FacetResult struct {
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

type RunMetrics struct {
	Tokens                int64 `json:"tokens"`
	ToolCalls             int64 `json:"tool_calls"`
	SourceReads           int64 `json:"source_reads"`
	IncludedSourceRereads int64 `json:"included_source_rereads"`
	RepeatedFullPacks     int64 `json:"repeated_full_packs"`
	ContextCalls          int64 `json:"context_calls"`
	ContextMillis         int64 `json:"context_millis"`
	BroadNavigationCalls  int64 `json:"broad_navigation_calls"`
}

type InvalidRun struct {
	InfrastructureFailure bool   `json:"infrastructure_failure"`
	Reason                string `json:"reason"`
	RetainedLog           string `json:"retained_log"`
}

type ReviewedRun struct {
	Review  RunReview   `json:"review"`
	Metrics RunMetrics  `json:"metrics"`
	Invalid *InvalidRun `json:"invalid,omitempty"`
}

type GateReport struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures"`
}
```

Allow only `pass` and `fail` review states. Require nonblank evidence for every
state. A forbidden outcome passes only when its status is `pass` and its
evidence explains why the outcome is absent.

`RunReview.Facets` contains both required-facet IDs and explicit-unknown IDs.
Every non-target required facet and every explicit unknown must pass in every
candidate run. The target facet may fail in Golden runs but must pass in at
least two of three candidate runs. Before execution, create a hypothesis
contract by moving exactly the chosen target from `ExplicitUnknowns` to
`RequiredFacets`; never alter that contract after any run starts.

Use integer comparisons:

```go
candidateTokens*100 <= goldenTokens*105
candidateLatency*100 <= goldenLatency*110
pairedCandidateLatency <= pairedGoldenLatency*2
```

Require exactly the predeclared set of logical run numbers. Each logical run
must have exactly one valid attempt. An invalid attempt is acceptable only when
`InfrastructureFailure` is true, `Reason` is nonblank, and `RetainedLog` is
nonblank; it remains in the run sequence and its replacement uses the same run
number with the next attempt number. Reject extra logical run numbers and
multiple valid attempts for one logical run.

- [ ] **Step 4: Run focused and package tests**

Run:

```bash
gofmt -w internal/agentbench/review.go internal/agentbench/review_test.go
go test ./internal/agentbench -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agentbench/review.go internal/agentbench/review_test.go
git commit -m "Enforce monotonic agent quality gates" -m $'- Require every retained facet and forbid unsupported outcomes per run\n- Gate tool use, source reads, tokens, and Context latency without cherry-picking'
```

---

### Task 4: Add Java/Spring Accuracy Fixtures

**Files:**

- Create: `testdata/agent-context-regression/matrix.json`
- Create: `testdata/agent-context-regression/g2-java-missing-contract/contract.json`
- Create: `testdata/agent-context-regression/g2-java-missing-contract/query.en.txt`
- Create: `testdata/agent-context-regression/g2-java-missing-contract/query.de.txt`
- Create: `testdata/agent-context-regression/g2-java-missing-contract/workspace/.goregraph-workspace.yml`
- Create: Java source and `pom.xml` files below G2 `workspace/services/catalog`, `workspace/libraries/job-client`, and `workspace/services/jobs`
- Create: `testdata/agent-context-regression/g5-java-persistence-side-effects/contract.json`
- Create: `testdata/agent-context-regression/g5-java-persistence-side-effects/query.en.txt`
- Create: `testdata/agent-context-regression/g5-java-persistence-side-effects/query.de.txt`
- Create: G5 workspace marker, `pom.xml`, Java production, and Java test files
- Create: `internal/agentbench/matrix_test.go`

**Interfaces:**

- Consumes: Tasks 1 and 2
- Produces: committed cases `g2-java-missing-contract` and `g5-java-persistence-side-effects`
- Produces: `copyBenchmarkWorkspace(t *testing.T, source string) string`

- [ ] **Step 1: Write the matrix integration test and verify RED**

Add G2 and G5 to `matrix.json` before creating their case directories. Copy each
declared workspace to `t.TempDir()`, run:

```go
code := cli.Run([]string{
	"workspace", "scan-all", root,
	"--workspace", root,
	"--no-update-gitignore",
}, &stdout, &stderr)
```

Then build each query:

```go
pack, err := agent.BuildContext(agent.ContextRequest{
	Root:         root,
	Query:        string(query),
	BudgetTokens: 4000,
	MaxFiles:     12,
})
```

Load the contract, call `EvaluatePack`, and require no violations. For English
and German variants, require identical endpoint, entrypoint, call-chain,
contract, persistence, source, coverage, fallback, and retry projections.

Run:

```bash
go test ./internal/agentbench -run 'TestCommittedBenchmarkMatrix/G2|TestCommittedBenchmarkMatrix/G5' -count=1
```

Expected: FAIL because the declared case directories do not exist.

- [ ] **Step 2: Write G2 as a generic three-project missing-contract workspace**

Use this topology:

```text
services/catalog
  DELETE /catalog/{itemId}
  CatalogController.remove -> CatalogService.remove -> CatalogRepository.deleteById
libraries/job-client
  GET /internal/jobs
  JobClient, JobClientConfig, JobClientAuth, JobClientRetry
services/jobs
  GET /internal/jobs
  JobController.list -> JobService.list -> JobRepository.findByCatalogIdAndItemId
  JobHousekeeping.publishRemoval
```

The English query must say:

```text
Plan the smallest production change that removes jobs when a catalog item is deleted. Show the current public deletion path, prove that the future job deletion contract is absent, and provide adjacent client configuration, authentication, retry, provider persistence, side-effect, and test evidence. Do not invent the missing call or route.
```

The German query must express the same domain request without adding project
responsibilities:

```text
Plane die kleinste produktionsreife Änderung, durch die beim Löschen eines Katalogeintrags auch die zugehörigen Aufgaben entfernt werden. Zeige den aktuellen öffentlichen Löschpfad, belege das Fehlen des zukünftigen Aufgaben-Löschvertrags und liefere angrenzende Belege zu Client-Konfiguration, Authentifizierung, Retry, Provider-Persistenz, Nebenwirkungen und Tests. Erfinde weder den fehlenden Aufruf noch die fehlende Route.
```

The contract requires the catalog DELETE endpoint and
`deleteById`, requires the concrete client config/auth/retry sources, requires
an uncertainty for the missing delete contract, and forbids the existing jobs
GET endpoint as the primary entrypoint.

Use source bodies with direct calls, for example:

```java
@RestController
@RequestMapping("/catalog")
final class CatalogController {
  private final CatalogService service;

  @DeleteMapping("/{itemId}")
  void remove(@PathVariable String itemId) {
    service.remove(itemId);
  }
}

final class CatalogService {
  private final CatalogRepository repository;

  void remove(String itemId) {
    repository.deleteById(itemId);
  }
}
```

Do not add a DELETE route or delete client method to the jobs projects; absence
is part of the contract.

- [ ] **Step 3: Write G5 as a persistence and side-effect workspace**

Use one Java/Spring project with this exact observed flow:

```java
@DeleteMapping("/accounts/{accountId}")
void remove(@PathVariable String accountId) {
  accountService.remove(accountId);
}

void remove(String accountId) {
  Account account = accountRepository.findByAccountId(accountId);
  accountRepository.delete(account);
  auditLog.recordAccountRemoval(accountId);
  mailSender.sendAccountRemoved(account.ownerEmail());
  userDirectory.invalidate(account.ownerId());
}
```

Add `AccountRepository`, `AuditLog`, `MailSender`, `UserDirectory`,
`AccountControllerTest`, and `AccountServiceTest`. Add a decoy inherited
`findAll` reference and a create method so the contract can forbid both as
substitutes for the concrete finder and deletion path.

The German parity query is:

```text
Analysiere das Löschen eines Kontos. Belege den öffentlichen Endpunkt, die aktuelle Aufrufkette, den konkreten Repository-Finder und die Löschoperation sowie alle Nebenwirkungen für Audit, E-Mail und Benutzerinformationen. Verwende keine Create- oder generische Find-All-Evidenz als Ersatz.
```

- [ ] **Step 4: Run the completed Java fixture tests**

Run:

```bash
gofmt -w internal/agentbench/matrix_test.go
go test ./internal/agentbench -run 'TestCommittedBenchmarkMatrix/G2|TestCommittedBenchmarkMatrix/G5' -count=1
go test ./internal/agentbench -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agentbench/matrix_test.go testdata/agent-context-regression/matrix.json testdata/agent-context-regression/g2-java-missing-contract testdata/agent-context-regression/g5-java-persistence-side-effects
git commit -m "Add Java agent accuracy fixtures" -m $'- Cover missing-contract endpoint competition with generic Spring services\n- Preserve concrete persistence and business side-effect evidence'
```

---

### Task 5: Add Go, TypeScript, and Ambiguity Fixtures

**Files:**

- Modify: `testdata/agent-context-regression/matrix.json`
- Create: `testdata/agent-context-regression/g3-go-existing-flow/`
- Create: `testdata/agent-context-regression/g4-typescript-consumer-provider/`
- Create: `testdata/agent-context-regression/g6-ambiguous-entrypoint/`
- Modify: `internal/agentbench/matrix_test.go`

**Interfaces:**

- Consumes: matrix integration helper from Task 4
- Produces: committed cases G3, G4, and G6

- [ ] **Step 1: Extend the matrix test and verify RED**

Add G3, G4, and G6 to `matrix.json` before creating their directories.

Run:

```bash
go test ./internal/agentbench -run 'TestCommittedBenchmarkMatrix/G3|TestCommittedBenchmarkMatrix/G4|TestCommittedBenchmarkMatrix/G6' -count=1
```

Expected: FAIL because the three newly declared case directories do not exist.

- [ ] **Step 2: Add a Go current-flow case**

Create a Go module under `workspace/services/orders` with:

```go
func routes(router *Router) {
	router.DELETE("/orders/{orderId}", deleteOrder)
}

func deleteOrder(w http.ResponseWriter, request *http.Request) {
	orderID := request.PathValue("orderId")
	orderService.Remove(request.Context(), orderID)
}

func (service *OrderService) Remove(ctx context.Context, orderID string) error {
	return service.repository.DeleteByOrderID(ctx, orderID)
}
```

Add a repository implementation and test. The contract requires the DELETE
route, handler, service, and `DeleteByOrderID` source; it forbids an unrelated
`GET /orders` list route as the entrypoint. This case has only `query.en.txt`
because language parity is already exercised by G2 and G5.

- [ ] **Step 3: Add a TypeScript consumer/provider case**

Create `workspace/frontend/storefront` and `workspace/services/orders`, each
with `package.json`. Use:

```typescript
export async function removeOrder(orderId: string): Promise<void> {
  await fetch(`/api/orders/${orderId}`, {
    method: "DELETE",
    headers: authorizationHeaders(),
  });
}
```

and:

```typescript
import express from "express";

const application = express();
application.delete("/api/orders/:orderId", removeOrder);

async function removeOrder(request: Request, response: Response): Promise<void> {
  await orderService.remove(request.params.orderId);
  response.sendStatus(204);
}
```

The contract requires the consumer, DELETE provider endpoint, authorization
helper, provider handler, and service source. It forbids a decoy
`application.get("/api/orders", listOrders)` route as the primary endpoint.

- [ ] **Step 4: Add an ambiguity and fallback case**

Create two Java/Spring services that both expose:

```java
@DeleteMapping("/jobs/{jobId}")
void remove(@PathVariable String jobId) {
  service.remove(jobId);
}
```

Give the services different project paths and equally strong source evidence.
The query says only “Explain how deleting a job works” and names neither
service. The contract requires:

```json
{
  "fallback_required": true,
  "max_estimated_tokens": 4000,
  "max_source_omissions": 0,
  "require_bounded_source": true
}
```

It forbids either service being presented as the unique reliable entrypoint.
Add several richer create/list decoys to exercise budget competition without
changing the expected fallback.

- [ ] **Step 5: Run the new cases and verify their contracts**

Run:

```bash
go test ./internal/agentbench -run 'TestCommittedBenchmarkMatrix/G3|TestCommittedBenchmarkMatrix/G4|TestCommittedBenchmarkMatrix/G6' -count=1
go test ./internal/agentbench -count=1
```

Expected: PASS without modifying agent production code. If a current
cross-language limitation prevents a strong required facet, record it as an
explicit unknown in the case contract rather than fabricating coverage.

- [ ] **Step 6: Commit**

```bash
git add internal/agentbench/matrix_test.go testdata/agent-context-regression/matrix.json testdata/agent-context-regression/g3-go-existing-flow testdata/agent-context-regression/g4-typescript-consumer-provider testdata/agent-context-regression/g6-ambiguous-entrypoint
git commit -m "Add cross-language agent guardrails" -m $'- Preserve Go and TypeScript endpoint and call-flow evidence\n- Require safe fallback for unresolved entrypoint ambiguity'
```

---

### Task 6: Expose Deterministic Verification and Gate Commands

**Files:**

- Create: `scripts/agent-context-regression/main.go`
- Create: `scripts/agent-context-regression/main_test.go`
- Create: `scripts/benchmark-agent-context-regression.sh`

**Interfaces:**

- Consumes: `internal/agentbench`
- Produces:
  - `verify-pack --contract <absolute> --pack <absolute>`
  - `diff-pack --golden-pack <absolute> --candidate-pack <absolute> --hypothesis <absolute> --output <absolute>`
  - `gate --contract <absolute> --hypothesis <absolute> --golden-runs <absolute> --candidate-runs <absolute> --output <absolute>`

- [ ] **Step 1: Write failing command tests**

Test each subcommand through an injected `run(args []string, stdout, stderr
io.Writer) int`. Require exit `0` for pass, `1` for a valid gate failure, and
`2` for malformed input. Require absolute paths, reject an existing output
path, and require stable indented JSON ending in one newline.

Central test:

```go
code := run([]string{
	"diff-pack",
	"--golden-pack", goldenPath,
	"--candidate-pack", candidatePath,
	"--hypothesis", hypothesisPath,
	"--output", outputPath,
}, &stdout, &stderr)
if code != 1 {
	t.Fatalf("protected endpoint diff exit = %d, stderr=%s", code, stderr.String())
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
go test ./scripts/agent-context-regression -count=1
```

Expected: FAIL because the command does not exist.

- [ ] **Step 3: Implement the command**

Decode Context Packs directly into `agent.ContextPack`. `verify-pack` prints a
`GateReport`; `diff-pack` always writes the complete semantic diff plus its
violations; `gate` loads reviewed runs and evaluates the declared target.

The wrapper is exactly:

```bash
#!/usr/bin/env bash

set -euo pipefail
script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
repo_root=$(cd -P -- "$script_dir/.." && pwd -P)
exec go run "$repo_root/scripts/agent-context-regression" "$@"
```

- [ ] **Step 4: Run command and wrapper tests**

Run:

```bash
gofmt -w scripts/agent-context-regression/main.go scripts/agent-context-regression/main_test.go
go test ./scripts/agent-context-regression ./internal/agentbench -count=1
bash -n scripts/benchmark-agent-context-regression.sh
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/agent-context-regression scripts/benchmark-agent-context-regression.sh
git commit -m "Add agent regression evaluation commands" -m $'- Verify Context Packs and emit stable semantic diffs\n- Evaluate reviewed Golden and candidate runs with strict exit codes'
```

---

### Task 7: Run Isolated Golden-Versus-Candidate Benchmarks

**Files:**

- Create: `internal/agentbench/runner.go`
- Create: `internal/agentbench/runner_test.go`
- Modify: `scripts/agent-context-regression/main.go`
- Modify: `scripts/agent-context-regression/main_test.go`
- Create: `scripts/benchmark-agent-context-regression_test.sh`

**Interfaces:**

- Produces: `RunRegression(ctx context.Context, config RunnerConfig) error`
- Adds command:
  - `run --matrix <absolute> --external-case <absolute> --golden-binary <absolute> --golden-commit <40-hex> --candidate-binary <absolute> --candidate-commit <40-hex> --instruction <absolute> --phase <smoke|full> --target-case <id> --runs <count> --output <absolute>`
- Reuses: `scripts/analyze-agent-context-log.sh`

- [ ] **Step 1: Write runner order and isolation tests**

Use fake `codex`, Golden, and candidate executables. Require smoke order:

```text
g1 golden 1
g1 candidate 1
target golden 1
target candidate 1
```

Require full three-run order per case:

```text
golden 1
candidate 1
candidate 2
golden 2
golden 3
candidate 3
```

Only the query with `end_to_end: true` runs through Codex. English/German
parity variants remain deterministic Pack checks, so the full six-case matrix
contains exactly 36 Codex executions. When the smoke target is G1, deduplicate
the target list and run one Golden/candidate G1 pair.

Assert that Golden and candidate commands receive different copied workspace
paths, each copy is scanned by only its own binary, and each Codex process sees
the matching binary as `goregraph` at the front of `PATH`.

- [ ] **Step 2: Write artifact, identity, and failure tests**

Require these retained files:

```text
inputs/matrix.json
inputs/external-case/contract.json
inputs/instruction.txt
inputs/prompt-digests.tsv
identity/golden-binary.sha256
identity/candidate-binary.sha256
identity/golden-commit.txt
identity/candidate-commit.txt
identity/codex-version.txt
identity/codex-args.txt
identity/run-order.tsv
cases/<case>/<query>/golden-pack.json
cases/<case>/<query>/candidate-pack.json
cases/<case>/<query>/pack-diff.json
cases/<case>/<query>/golden-workspace.sha256
cases/<case>/<query>/candidate-workspace.sha256
cases/<case>/<query>/golden-index.sha256
cases/<case>/<query>/candidate-index.sha256
runs/<case>/<query>/<build>-<run>-<attempt>.jsonl
runs/<case>/<query>/<build>-<run>-<attempt>.stderr
runs/<case>/<query>/<build>-<run>-<attempt>.metrics.tsv
reviews/<case>/<query>/<build>-<run>-<attempt>.json
summary.tsv
```

Simulate a Codex exit failure. Require the JSONL and stderr to remain, require
an invalid-run record with `infrastructure_failure: true`, and require the
harness to stop before any later run. Simulate a semantically bad but
successful run and require it to remain valid.

- [ ] **Step 3: Run tests and verify RED**

Run:

```bash
go test ./internal/agentbench ./scripts/agent-context-regression -run 'TestRunRegression|TestRunCommand' -count=1
bash scripts/benchmark-agent-context-regression_test.sh
```

Expected: FAIL because runner support does not exist.

- [ ] **Step 4: Implement isolated preparation and execution**

Define:

```go
type RunnerConfig struct {
	MatrixPath      string
	ExternalCase    string
	GoldenBinary    string
	GoldenCommit    string
	CandidateBinary string
	CandidateCommit string
	InstructionPath string
	Phase           string
	TargetCase      string
	Runs            int
	Output          string
	CodexArgs       []string
	AnalyzerPath    string
}
```

Require `Runs == 1` for `--phase smoke` and an odd `Runs >= 3` for
`--phase full`. Load G1 from the absolute `ExternalCase` directory and G2-G6
from the committed matrix. Reject missing, duplicate, or unexpected case IDs.
Require lowercase 40-character hexadecimal commit identities. Hash binaries,
case source trees, prompts, the instruction, and generated
`.goregraph-workspace/agent/context-index.json` files with SHA-256. Record the
validated Codex argument vector and exact execution order before the first
Codex process starts.

Copy fixture sources with `fs.WalkDir`, rejecting symlinks and paths that escape
the case workspace. Never copy `goregraph-out`, `.goregraph-workspace`, `.git`,
`node_modules`, `target`, `build`, or `dist`.

For each build, run:

```text
<binary> workspace scan-all <copy> --workspace <copy> --no-update-gitignore
<binary> context <copy> --query <query> --budget-tokens 4000 --max-files 12 --format json
```

Measure direct Context duration with `time.Now()` around the second command.
Alternate Golden and candidate direct Context calls with the same order as the
Codex runs.

Validate `CODEX_BENCHMARK_ARGS` with the same fixed requirements documented in
`docs/BENCHMARKING.md`: explicit model and reasoning, approval `never`, sandbox
`read-only`, `--ephemeral`, `--skip-git-repo-check`, `--ignore-user-config`,
`--ignore-rules`, and color `never`. Invoke `exec.CommandContext` with an
argument slice; never evaluate shell text.

Create a per-run review template containing every required facet, forbidden
outcome, and explicit unknown with status `fail` and evidence
`"Review required before gating."`. The gate must reject these templates until
an independent reviewer replaces every evidence value and records their name
timestamp, and signature.

- [ ] **Step 5: Parse existing transcript metrics and write summaries**

Invoke:

```text
bash scripts/analyze-agent-context-log.sh <absolute-jsonl>
```

Write this header:

```text
case	query	build	run	attempt	tokens	tool_calls	context_calls	repeated_full_packs	broad_navigation_calls	source_read_calls	included_source_rereads	context_millis	log
```

Do not alter the existing analyzer schema or release harness. Convert its
`goregraph_calls` to `context_calls` only in the new regression summary.

- [ ] **Step 6: Run the fake end-to-end harness**

Run:

```bash
gofmt -w internal/agentbench/runner.go internal/agentbench/runner_test.go scripts/agent-context-regression/main.go scripts/agent-context-regression/main_test.go
go test ./internal/agentbench ./scripts/agent-context-regression -count=1
bash scripts/benchmark-agent-context-regression_test.sh
```

Expected: PASS with no network or real Codex invocation.

- [ ] **Step 7: Commit**

```bash
git add internal/agentbench/runner.go internal/agentbench/runner_test.go scripts/agent-context-regression scripts/benchmark-agent-context-regression_test.sh
git commit -m "Add paired agent regression runner" -m $'- Scan isolated Golden and candidate workspace copies\n- Interleave retained Codex runs and measure direct Context latency\n- Stop on infrastructure failures without replacing valid outcomes'
```

---

### Task 8: Freeze G1 Externally and Document the Workflow

**Files:**

- Modify: `docs/BENCHMARKING.md`
- Modify: `README.md`
- External only: G1 case directory, Golden binary, prompt, contract, reviews, and transcripts

**Interfaces:**

- Consumes: regression command from Tasks 6 and 7
- Produces: reproducible external G1 baseline bound to commit `1bc4408`

- [ ] **Step 1: Document the two benchmark purposes**

Add a “Monotonic regression benchmark” section before the release decision in
`docs/BENCHMARKING.md`. State explicitly:

```text
The release benchmark compares no GoreGraph with one accepted GoreGraph build.
The monotonic regression benchmark compares the frozen Golden GoreGraph build
with one candidate build. Passing one benchmark does not imply passing the
other.
```

Document smoke and full phases, the six-case matrix, external G1 ownership,
independent per-run reviews, thresholds, invalid-run rules, and artifact
retention.

- [ ] **Step 2: Prepare G1 outside the repository**

Require `GOREGRAPH_G1_WORKSPACE` and `GOREGRAPH_G1_REFERENCE_TRANSCRIPT` to be
absolute, readable external paths supplied by the operator. Copy neither value
nor either file into repository output.

Create an external G1 contract whose required facets preserve:

```text
public-endpoint
current-oracle-path
missing-task-transition
both-task-types
direct-task-delete
repository-delete
mail-side-effect
user-information-side-effect
protocol-side-effect
individual-delete-not-reused
one-context-call
three-bounded-omissions
no-broad-navigation
```

Its forbidden outcomes are:

```text
create-path-selected
unrelated-endpoint-selected
future-call-presented-as-current
included-source-reread
unbounded-source-search
unsupported-cross-service-atomicity
```

Record the known gaps as explicit unknowns, not passing requirements:

```text
precise-internal-delete-client-pattern
concrete-client-configuration
exact-repository-finders-and-entity-fields
comment-or-foreign-key-dependencies
task-side-test-inventory
cross-service-compensation-or-retry-strategy
```

For a later accuracy candidate, copy the frozen external contract before any
run, move exactly the declared `Hypothesis.TargetFacet` from
`explicit_unknowns` to `required_facets`, and retain both contract digests. All
other explicit unknowns remain mandatory uncertainty disclosures.

Do not copy this contract, prompt, transcript, or workspace into the repository.

- [ ] **Step 3: Build and hash the Golden binary**

At execution time, use `superpowers:using-git-worktrees` to create a temporary
detached worktree at `1bc4408`, then run:

```bash
GOBIN="$GOREGRAPH_BENCHMARK_EVIDENCE/bin/golden" go install ./cmd/goregraph
shasum -a 256 "$GOREGRAPH_BENCHMARK_EVIDENCE/bin/golden/goregraph" >"$GOREGRAPH_BENCHMARK_EVIDENCE/bin/golden.sha256"
```

Build the candidate from the implementation worktree into a different absolute
directory. Do not replace the user’s normal local installation.

- [ ] **Step 4: Validate the smoke workflow**

Run the new harness with `--phase smoke`, `--external-case` pointing at the G1
directory, G1 as the target case, one run, and a new external output directory.
The runner must produce Golden and candidate Pack Diffs and retained review
templates. Complete both reviews from source evidence, then run `gate`.

Expected: because this plan changes only benchmark infrastructure, Golden and
candidate semantic packs are identical and all preserved G1 facets pass.

- [ ] **Step 5: Document rejected-candidate classification**

Require one external `failure-classification.json` before another hypothesis
may start after a failed gate:

```json
{
  "schema": 1,
  "hypothesis": "concrete-client-configuration",
  "failed_case": "g1",
  "failed_run": 2,
  "category": "ranking",
  "evidence": "The candidate replaced the protected public endpoint while admitting configuration evidence."
}
```

Allow exactly `scanner_truth`, `intent`, `ranking`, `budget`, `rendering`, or
`agent_behavior`. The file is external evidence, not an input that can waive a
failed gate.

- [ ] **Step 6: Commit documentation**

```bash
git add docs/BENCHMARKING.md README.md
git commit -m "Document monotonic agent evaluation" -m $'- Separate candidate regression gates from the release benchmark\n- Define external G1 evidence, review, and retention requirements'
```

---

### Task 9: Verify the Complete Evaluation System

**Files:**

- Modify only when a failure proves a defect in files created by Tasks 1-8.

**Interfaces:**

- Consumes: all previous tasks
- Produces: a verified, unreleased evaluation framework ready for the first single-hypothesis accuracy change

- [ ] **Step 1: Run formatting and static checks**

Run:

```bash
gofmt -w internal/agentbench/*.go scripts/agent-context-regression/*.go
git diff --check
go vet ./...
```

Expected: PASS.

- [ ] **Step 2: Run focused benchmark tests**

Run:

```bash
go test ./internal/agentbench ./scripts/agent-context-regression -count=1
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
```

Expected: PASS, including the unchanged release harness regression.

- [ ] **Step 3: Run the complete suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS. If one of the three known machine-dependent performance tests
fails, run the identical test against `1bc4408` in the Golden worktree. Treat it
as pre-existing only when the unchanged Golden build fails the same test under
the same command and environment; retain both outputs.

- [ ] **Step 4: Verify the full synthetic matrix without Codex**

Run:

```bash
go test ./internal/agentbench -run TestCommittedBenchmarkMatrix -count=3
```

Expected: all G2-G6 projections and English/German parity checks pass in all
three repetitions.

- [ ] **Step 5: Run one fake full regression**

Run:

```bash
bash scripts/benchmark-agent-context-regression_test.sh
```

Expected: all six fake cases use the declared interleaving, every artifact is
retained, and the gate rejects one injected semantic regression without
executing a replacement run.

- [ ] **Step 6: Review scope**

Run:

```bash
git status --short
git diff --stat 1bc4408..HEAD
git log --oneline --decorate 1bc4408..HEAD
```

Confirm that production Context selection, ranking, scanner, and rendering
files are unchanged, the existing release harness remains byte-identical, no
proprietary G1 artifact is tracked, and no release tag exists.

- [ ] **Step 7: Stop before accuracy tuning**

Report the framework results and the external G1 smoke result. Do not implement
the concrete configuration/client-pattern accuracy hypothesis in this plan.
That hypothesis receives its own failing contract change, implementation plan,
and Golden-versus-candidate run after this framework is accepted.
