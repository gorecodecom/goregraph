# Token-Efficient Agent Context for GoreGraph 1.3.0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the existing offline dashboard as the complete human exploration surface while adding a physically separate, deterministic, budgeted AI context surface that measurably reduces end-to-end agent tokens.

**Architecture:** Source extraction produces one shared canonical index per project and one reconciled index per workspace. Independent projection writers then produce either the full human-facing dashboard/reports, the compact AI-facing context index and guide, or both. Project output is split into `goregraph-out/index/`, `goregraph-out/agent/`, and `goregraph-out/dashboard/`; workspace output is split into `.goregraph-workspace/index/`, `.goregraph-workspace/agent/`, and `.goregraph-workspace/dashboard/`. `goregraph context` and the standard MCP `task_context` tool read only `agent/context-index.json`. Dashboard generation reads only the shared `index/` tree and never consumes `agent/`; agent generation reads the in-memory/shared index and never consumes `dashboard/`. `build all` performs source extraction once and writes both projections.

**Tech Stack:** Go 1.23+ standard library, output Schema 3 JSON, existing scan/workspace records, deterministic lexical ranking, Go tests, shell-based benchmark harness, local Codex CLI acceptance runs.

## Global Constraints

- Work directly on `main`; do not create a feature branch or worktree.
- Use strict TDD for every production behavior: add one focused failing test, run it and observe the expected failure, implement the smallest behavior, rerun the focused test, then refactor only while green.
- Keep the source target at unreleased `1.3.0`; do not create a tag, GitHub Release, Homebrew publication, Scoop publication, or Winget publication.
- Preserve the current workspace dashboard, all seven dashboard views, Code Explorer, and existing human-readable reports. Move them into the new `dashboard/` projection without changing their behavior.
- Do not invent a second interactive project dashboard in 1.3.0. For a single project, the `dashboard` target means the existing full human-readable Markdown reports; the interactive HTML dashboard remains a workspace feature.
- Build targets are exactly `agent`, `dashboard`, and `all`. `agent` must not write `dashboard/`; `dashboard` must not write `agent/`; `all` must perform one source extraction and then write both projections.
- `goregraph scan <path>` remains a compatibility alias for `goregraph build all <path>`.
- `goregraph workspace scan-all <path>` remains a compatibility alias for `goregraph workspace build all <path>`.
- `goregraph update` and `goregraph workspace refresh` accept `--target agent|dashboard|all`, defaulting to `all`.
- Existing output directories from pre-1.3.0 flat layouts are never silently mixed with the new layout. Doctor must identify the legacy layout and prescribe `clean` plus a rebuild.
- Bump generated-output `SchemaVersion` from `2` to `3` because the manifest and file layout are intentionally incompatible. Keep the application version at unreleased `1.3.0`.
- Preserve existing CLI query commands for compatibility, but remove them from the default agent guidance and default MCP tool list.
- Add no runtime dependency and no tokenizer dependency. Token estimates must be deterministic and implemented with the Go standard library.
- `goregraph context` must perform one index read and one ranking pass. It must not recursively invoke GoreGraph commands or emit a `Suggested next` query chain.
- Default limits are exactly `1800` estimated tokens and `12` source files. Accepted ranges are `256..4000` tokens and `1..20` files.
- A context result contains at most three scoped uncertainties. It must never repeat global Maven, text, YAML, or unsupported-language coverage tables.
- Low-confidence or empty retrieval returns `fallback_required: true` with one reason. Agents must then stop using GoreGraph and inspect source directly.
- Standard MCP mode exposes exactly one tool: `task_context`. `goregraph mcp --expert-tools` retains the existing specialist tools for diagnostics and manual exploration.
- One agent task may call `task_context` at most twice: once with the user task, and once more only with a narrower exact route or symbol returned by the first call.
- The existing benchmark scenario is a release gate. Across three runs per variant with the same model, neutral base prompt, workspace, sandbox, and reasoning settings, only the treatment instruction may differ: baseline forbids GoreGraph, assisted permits one bounded Context Pack workflow. The assisted median must use at least 20% fewer total tokens than the baseline median and must not score lower on the twelve-question evidence rubric.
- For the recorded 145,700-token baseline, an assisted run is not acceptable above 116,560 median tokens.
- The serialized default Context Pack must remain at or below 7,200 UTF-8 bytes and report at most 1,800 estimated tokens.
- The merged Weka `.goregraph-workspace/agent/context-index.json` must remain at or below 16 MiB. It must not reproduce the full workspace `index/symbol-usages.json` projection under another name.
- On the real Weka workspace, a warm `goregraph context` invocation must complete in at most one second median and two seconds maximum across 20 invocations.
- Use English source comments, CLI text, documentation, and commit messages.

---

## Public Contract and File Map

### Generated output layout

Project output:

```text
goregraph-out/
├── manifest.json
├── index/
│   ├── freshness.json
│   ├── files.json
│   ├── symbols.json
│   ├── relations.json
│   ├── graph.json
│   ├── symbols-full.json
│   ├── relations-full.json
│   ├── graph-full.json
│   ├── callgraph.json
│   ├── endpoint-flows.json
│   ├── test-map.json
│   ├── routes.json
│   ├── flows.json
│   ├── api-contracts.json
│   ├── architecture-capabilities.json
│   ├── service-dependencies.json
│   ├── frontend-usage.json
│   ├── contract-matches.json
│   ├── diagnostics.json
│   ├── diagnostics-canonical.json
│   ├── diagnostic-families.json
│   ├── package-graph.json
│   ├── maven-graph.json
│   ├── analyzers.json
│   ├── evidence.json
│   ├── capabilities.json
│   ├── coverage.json
│   ├── spring.json
│   ├── audit.json
│   └── workspace-*.json
├── agent/
│   ├── context-index.json
│   └── agent-guide.md
└── dashboard/
    ├── report.md
    ├── navigation.md
    ├── architecture.md
    ├── endpoints.md
    ├── flows.md
    ├── test-map.md
    └── <all other existing human-readable project reports>
```

Workspace output:

```text
.goregraph-workspace/
├── manifest.json
├── index/
│   ├── registry.json
│   ├── context.json
│   ├── contract-matches.json
│   ├── feature-flows.json
│   ├── data-flows.json
│   ├── feature-dossiers.json
│   ├── workspace-graph.json
│   ├── workspace-service-map.json
│   ├── workspace-endpoint-traces.json
│   ├── directed-traces.json
│   ├── freshness.json
│   ├── symbol-index.json
│   └── symbol-usages.json
├── agent/
│   ├── context-index.json
│   └── agent-guide.md
└── dashboard/
    ├── workspace-map.html
    ├── workspace-map-assets/
    ├── workspace-context.md
    ├── contract-matches.md
    ├── feature-flows.md
    ├── feature-dossiers.md
    └── next-actions.md
```

- `internal/scan/output_layout.go` owns output-path helpers, target parsing, projection status, and the top-level manifest contract.
- `internal/scan/agent_context_index.go` owns compact public index records, stable IDs, project index construction, workspace index merging, normalization, and deterministic sorting.
- `goregraph-out/agent/context-index.json` contains one project's agent-navigation facts.
- `.goregraph-workspace/agent/context-index.json` contains merged project facts plus workspace contracts, feature dossiers, and endpoint traces.
- `index/` contains complete machine-readable graph data and may be large. It is an internal GoreGraph data surface, not recommended AI prompt input.
- The tree above is the `all` superset. Target-specific builds write only index components required by that target; notably, workspace symbol index/usages are omitted from an agent-only build.
- `dashboard/` contains full human-facing Markdown/HTML projections. The workspace dashboard continues to provide Architecture, Endpoints, Feature Flow, Data Flow, Code Explorer, Diagnostics, and Coverage.
- `agent/` contains the only files recommended for AI use.

### Build target and manifest contract

Add:

```go
type BuildTarget string

const (
	BuildTargetAgent     BuildTarget = "agent"
	BuildTargetDashboard BuildTarget = "dashboard"
	BuildTargetAll       BuildTarget = "all"
)

type ProjectionStatus struct {
	GeneratedAt string   `json:"generated_at,omitempty"`
	Complete    bool     `json:"complete"`
	Files       []string `json:"files,omitempty"`
}

type OutputManifest struct {
	Tool        string           `json:"tool"`
	Schema      int              `json:"schema"`
	Scope       string           `json:"scope"`
	OutputDir   string           `json:"output_dir"`
	ProjectRoot string           `json:"project_root,omitempty"`
	Files       int              `json:"files,omitempty"`
	Skipped     int              `json:"skipped,omitempty"`
	Index       ProjectionStatus `json:"index"`
	Agent       ProjectionStatus `json:"agent"`
	Dashboard   ProjectionStatus `json:"dashboard"`
	Git         *GitMetadata     `json:"git,omitempty"`
}
```

Rules:

1. `ParseBuildTarget` accepts only `agent`, `dashboard`, and `all`.
2. Every successful build writes `manifest.json` last via atomic rename.
3. `index.complete` is true only after every canonical JSON file required by every currently complete projection is durable; `index.files` records that exact set.
4. A target rebuild updates only its own projection status and preserves the other projection directory and status when they are already valid.
5. `all` invokes source extraction exactly once, then writes `index/`, `agent/`, and `dashboard/`.
6. A failed projection leaves its `complete` value false and must not publish a success manifest.
7. Query, expert-query, MCP, report, dashboard, workspace path/impact, Doctor, diff, and refresh resolve files through layout helpers rather than hard-coded flat paths.
8. Workspace `symbol-index.json` and `symbol-usages.json` are dashboard/expert index components. An agent-only build must not materialize or refresh them.

### Context compiler

- `internal/agent/context.go` owns the public `ContextRequest`, `ContextPack`, location, relationship, file, and uncertainty records.
- `internal/agent/context_rank.go` owns query tokenization, lexical scoring, seed selection, graph expansion, deduplication, budget enforcement, confidence, and fallback decisions.
- `internal/agent/context_load.go` loads exactly one project or workspace `agent/context-index.json`.
- `internal/query/context.go` renders compact JSON or Markdown without generic warnings or suggested follow-up queries.

### User surfaces

- `internal/cli/cli.go` adds `goregraph build`, `goregraph workspace build`, and `goregraph context`.
- Existing `goregraph query <path> task-context` delegates to the same compiler for compatibility.
- `internal/mcp/mcp.go` exposes only `task_context` by default and accepts `--expert-tools` through CLI-provided server options.
- `internal/scan/agent_reports.go` replaces the query cascade in `agent/agent-guide.md` with one bounded context call and a fallback rule.
- `README.md`, `COMMANDS.md`, `docs/OUTPUTS.md`, and `docs/RELEASE.md` explain the split between the human dashboard and AI Context Pack.

### Benchmark and release gate

- `scripts/benchmark-agent-context.sh` runs matched baseline and assisted Codex executions and writes raw logs plus a TSV summary outside generated scan output.
- `docs/BENCHMARKING.md` documents the matched-prompt protocol and twelve-question quality rubric.
- The benchmark script accepts prompt and workspace paths as arguments; proprietary benchmark source and prompts are not committed.
- Treat the recorded 145,700/212,616 pair as regression evidence, not as a perfectly controlled A/B result: the baseline transcript explicitly forbids GoreGraph while the assisted transcript permits a query workflow. The new release gate therefore uses one neutral base prompt and explicit per-variant treatment instructions.

### Exact public Go records

Add these records to `internal/scan/agent_context_index.go` and keep their field names stable:

```go
type AgentContextFactRecord struct {
	ID           string   `json:"id"`
	Project      string   `json:"project,omitempty"`
	Kind         string   `json:"kind"`
	Name         string   `json:"name"`
	Qualified    string   `json:"qualified,omitempty"`
	HTTPMethod   string   `json:"http_method,omitempty"`
	Path         string   `json:"path,omitempty"`
	File         string   `json:"file,omitempty"`
	Line         int      `json:"line,omitempty"`
	EndLine      int      `json:"end_line,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Confidence   string   `json:"confidence,omitempty"`
	EvidenceIDs  []string `json:"evidence_ids,omitempty"`
	Search       string   `json:"search,omitempty"`
}

type AgentContextEdgeRecord struct {
	ID          string   `json:"id"`
	Project     string   `json:"project,omitempty"`
	FromFactID  string   `json:"from_fact_id,omitempty"`
	ToFactID    string   `json:"to_fact_id,omitempty"`
	FromLabel   string   `json:"from_label"`
	ToLabel     string   `json:"to_label"`
	Kind        string   `json:"kind"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type AgentContextCoverageRecord struct {
	Project    string `json:"project,omitempty"`
	Capability string `json:"capability"`
	Coverage   string `json:"coverage"`
	Reason     string `json:"reason"`
}

type AgentContextIndexRecord struct {
	SchemaVersion int                          `json:"schema_version"`
	Generated     string                       `json:"generated,omitempty"`
	Root          string                       `json:"root,omitempty"`
	Facts         []AgentContextFactRecord     `json:"facts"`
	Edges         []AgentContextEdgeRecord     `json:"edges"`
	Coverage      []AgentContextCoverageRecord `json:"coverage,omitempty"`
}
```

Add these records to `internal/agent/context.go` and keep their field names stable:

```go
const (
	DefaultContextBudgetTokens = 1800
	MinContextBudgetTokens     = 256
	MaxContextBudgetTokens     = 4000
	DefaultContextMaxFiles     = 12
	MaxContextMaxFiles         = 20
)

type ContextRequest struct {
	Root         string `json:"root,omitempty"`
	Query        string `json:"query"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
	MaxFiles     int    `json:"max_files,omitempty"`
}

type ContextLocation struct {
	ID          string   `json:"id"`
	Project     string   `json:"project,omitempty"`
	Kind        string   `json:"kind"`
	Label       string   `json:"label"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	EndLine     int      `json:"end_line,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type ContextRelationship struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Reason     string `json:"reason,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

type ContextFile struct {
	Project    string `json:"project,omitempty"`
	Path       string `json:"path"`
	StartLine  int    `json:"start_line,omitempty"`
	EndLine    int    `json:"end_line,omitempty"`
	Role       string `json:"role"`
	Reason     string `json:"reason"`
	Confidence string `json:"confidence,omitempty"`
}

type ContextUncertainty struct {
	Scope  string `json:"scope"`
	Reason string `json:"reason"`
}

type ContextPack struct {
	Schema            int                   `json:"schema"`
	Query             string                `json:"query"`
	Freshness         string                `json:"freshness,omitempty"`
	Confidence        string                `json:"confidence"`
	FallbackRequired  bool                  `json:"fallback_required"`
	FallbackReason    string                `json:"fallback_reason,omitempty"`
	Entrypoints       []ContextLocation     `json:"entrypoints,omitempty"`
	CallChain         []ContextRelationship `json:"call_chain,omitempty"`
	Contracts         []ContextLocation     `json:"contracts,omitempty"`
	Persistence       []ContextLocation     `json:"persistence,omitempty"`
	Tests             []ContextLocation     `json:"tests,omitempty"`
	Files             []ContextFile         `json:"files,omitempty"`
	Uncertainties     []ContextUncertainty  `json:"uncertainties,omitempty"`
	EstimatedTokens   int                   `json:"estimated_tokens"`
	BudgetTokens      int                   `json:"budget_tokens"`
}
```

The compiler entrypoint is:

```go
func BuildContext(request ContextRequest) (ContextPack, error)
```

The deterministic estimator is:

```go
func EstimateContextTokens(value any) (int, error)
```

It JSON-marshals `value`, counts UTF-8 runes, and returns `(runes + 3) / 4`. This is an estimate, not a model-specific tokenizer; the serialized byte ceiling and end-to-end benchmark are the stronger gates.

---

### Task 1: Separate Canonical Index, Agent, and Dashboard Builds

**Files:**
- Create: `internal/scan/output_layout.go`
- Create: `internal/scan/output_layout_test.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/scan_test.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_reconcile_test.go`
- Modify: `internal/scan/workspace_symbols.go`
- Modify: `internal/scan/workspace_dashboard.go`
- Modify: `internal/scan/workspace_diff.go`
- Modify: `internal/scan/workspace_graph.go`
- Modify: `internal/scan/workspace_impact.go`
- Modify: `internal/doctor/doctor.go`
- Modify: `internal/doctor/doctor_test.go`
- Modify: `internal/query/query.go`
- Modify: `internal/agent/service.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `release_files_test.go`

**Interfaces:**
- Consumes: the current single-pass `scanProject` result and current workspace reconciliation records.
- Produces: target-aware project/workspace builds, the new directory layout, atomic projection manifests, and compatibility aliases.

- [ ] **Step 1: Write failing layout, isolation, and one-pass tests**

Add table-driven path tests:

```go
func TestOutputLayoutSeparatesIndexAgentAndDashboard(t *testing.T) {
	layout := NewProjectOutputLayout("/tmp/project/goregraph-out")
	assertPath(t, layout.Manifest, "/tmp/project/goregraph-out/manifest.json")
	assertPath(t, layout.Index("routes.json"), "/tmp/project/goregraph-out/index/routes.json")
	assertPath(t, layout.Agent("context-index.json"), "/tmp/project/goregraph-out/agent/context-index.json")
	assertPath(t, layout.Dashboard("report.md"), "/tmp/project/goregraph-out/dashboard/report.md")
}

func TestParseBuildTargetRejectsUnknownValues(t *testing.T) {
	for _, value := range []string{"", "context", "contextai", "reports", "everything"} {
		if _, err := ParseBuildTarget(value); err == nil {
			t.Fatalf("accepted target %q", value)
		}
	}
}
```

Add project build tests:

```go
func TestProjectBuildAgentDoesNotWriteDashboard(t *testing.T) {
	root := writeBuildFixture(t)
	if _, err := RunBuild(root, config.Defaults(), BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	assertExists(t, filepath.Join(root, "goregraph-out", "index", "routes.json"))
	assertExists(t, filepath.Join(root, "goregraph-out", "agent", "agent-guide.md"))
	assertNotExists(t, filepath.Join(root, "goregraph-out", "dashboard"))
}

func TestProjectBuildDashboardDoesNotWriteAgent(t *testing.T) {
	root := writeBuildFixture(t)
	if _, err := RunBuild(root, config.Defaults(), BuildTargetDashboard); err != nil {
		t.Fatal(err)
	}
	assertExists(t, filepath.Join(root, "goregraph-out", "index", "routes.json"))
	assertExists(t, filepath.Join(root, "goregraph-out", "dashboard", "report.md"))
	assertNotExists(t, filepath.Join(root, "goregraph-out", "agent"))
}

func TestProjectBuildAllExtractsSourceOnce(t *testing.T) {
	root := writeBuildFixture(t)
	extractions := 0
	restore := replaceProjectExtractorForTest(func(...) (...) {
		extractions++
		return scanProject(...)
	})
	defer restore()

	if _, err := RunBuild(root, config.Defaults(), BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	if extractions != 1 {
		t.Fatalf("source extractions = %d, want 1", extractions)
	}
}
```

Use a narrow package-level extractor seam only for the counter test; do not redesign the scan architecture around dependency injection.

Add workspace tests asserting:

- `workspace build agent` creates project/workspace `index/` and `agent/`, but no `dashboard/`; in Task 1 the agent projection contains `agent-guide.md`, while Tasks 2–3 add `context-index.json`;
- `workspace build agent` does not create workspace `index/symbol-index.json` or `index/symbol-usages.json`;
- `workspace build dashboard` creates project/workspace `index/` and `dashboard/`, but no `agent/`;
- `workspace build all` scans each discovered project once and reconciles the workspace once after the project loop, rather than rebuilding the workspace after every project;
- the workspace HTML path is `.goregraph-workspace/dashboard/workspace-map.html`;
- Code Explorer reads `index/symbol-index.json` and `index/symbol-usages.json`;
- `workspace refresh --target agent` does not touch dashboard modification times;
- `workspace refresh --target dashboard` does not touch agent modification times.

Add CLI tests for these exact commands:

```text
goregraph build agent .
goregraph build dashboard .
goregraph build all .
goregraph workspace build agent .
goregraph workspace build dashboard .
goregraph workspace build all .
goregraph update --target agent|dashboard|all
goregraph workspace refresh . --target agent|dashboard|all
```

Also assert:

```text
goregraph scan .                    == goregraph build all .
goregraph workspace scan-all .      == goregraph workspace build all .
```

The compatibility test compares the generated manifest and directory set after normalizing timestamps.

- [ ] **Step 2: Run focused tests and observe RED**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/scan ./internal/cli ./internal/doctor ./internal/query ./internal/agent . -run 'TestOutputLayout|TestParseBuildTarget|TestProjectBuild|TestWorkspaceBuild|TestRunBuild|TestRunWorkspaceBuild|Test.*LegacyLayout' -count=1
```

Expected: compilation and assertion failures because target-aware builds and the separated paths do not exist.

- [ ] **Step 3: Add layout helpers and the target-aware manifest**

Implement:

```go
type OutputLayout struct {
	Root     string
	Manifest string
}

func NewProjectOutputLayout(root string) OutputLayout
func NewWorkspaceOutputLayout(root string) OutputLayout
func (l OutputLayout) Index(name string) string
func (l OutputLayout) Agent(name string) string
func (l OutputLayout) Dashboard(name string) string
func ParseBuildTarget(value string) (BuildTarget, error)
func (t BuildTarget) IncludesAgent() bool
func (t BuildTarget) IncludesDashboard() bool
```

Replace `Manifest` with the `OutputManifest` contract from the public section. Keep a private legacy manifest record only for Doctor's migration diagnosis; production readers must not treat it as current output.

Set `scan.SchemaVersion` to `3`. Context Pack's own `schema` field remains independently versioned as defined later; do not use it to infer the generated-output directory schema.

Partition current `GeneratedFiles` into:

```go
var IndexGeneratedFiles []string
var AgentGeneratedFiles []string
var DashboardGeneratedFiles []string
```

`manifest.json` remains the only generated file at the output root.

- [ ] **Step 4: Refactor project scanning into extraction plus projections**

Add:

```go
func RunBuild(root string, cfg config.Config, target BuildTarget) (Result, error)
```

`RunBuild` must:

1. validate the root and target;
2. call `scanProject` exactly once;
3. finish all derived records in memory exactly once;
4. write the canonical JSON set under `index/`;
5. write `agent/` only when `target.IncludesAgent()`;
6. write `dashboard/` only when `target.IncludesDashboard()`;
7. write `manifest.json` last through a temporary file and atomic rename;
8. reconcile a workspace only when invoked as a single-project command, not during each item of a workspace build loop.

Keep:

```go
func Run(root string, cfg config.Config) (Result, error) {
	return RunBuild(root, cfg, BuildTargetAll)
}
```

This preserves API compatibility while making `all` explicit.

Move all existing JSON writes, including per-project workspace overlays, to `index/`. Move all Markdown writes except `agent-guide.md` to `dashboard/`. Write the compact guide to `agent/agent-guide.md`.

- [ ] **Step 5: Make workspace reconciliation target-aware**

Add:

```go
func ReconcileWorkspaceTarget(currentRoot string, cfg config.Config, target BuildTarget) (*WorkspaceRegistryRecord, error)
```

Keep `ReconcileWorkspace` as an `all` wrapper.

Workspace reconciliation must:

- load project facts from each project's `index/`;
- write registry, contracts, flows, dossiers, traces, and freshness to workspace `index/` for either target;
- write workspace symbol index/usages and other Code Explorer-only full projections to `index/` only when the target includes `dashboard`;
- render HTML/assets and Markdown only for the dashboard target;
- render the compact workspace context index and guide only for the agent target;
- write `.goregraph-workspace/manifest.json` last;
- never require the dashboard to build the agent projection;
- never require the agent projection to build the dashboard;
- avoid rebuilding `symbol-usages.json` for `refresh --target agent` when the compact index can be merged from project `agent/context-index.json` plus lightweight workspace contract/trace records;
- preserve a valid non-selected projection and its manifest status.

For workspace-wide source builds, add one orchestration function that scans every project with workspace reconciliation disabled and calls `ReconcileWorkspaceTarget` exactly once after all project indexes exist.

- [ ] **Step 6: Add CLI build commands and compatibility aliases**

Exact grammar:

```text
goregraph build <agent|dashboard|all> [path] [--no-update-gitignore] [--no-workspace]
goregraph workspace build <agent|dashboard|all> [path] [--dry-run] [--workspace <path>] [--no-update-gitignore]
goregraph update [path] [--target agent|dashboard|all]
goregraph workspace refresh [path] [--target agent|dashboard|all] [--workspace <path>]
```

Defaults:

- `build` requires an explicit target;
- `update` and `workspace refresh` default to `all`;
- `scan` and `workspace scan-all` remain documented compatibility aliases for `all`;
- unknown/missing targets return exit code `2` and print the accepted values;
- build output states which projections were written and which valid projection was preserved.

Keep `goregraph workspace dashboard [path]` as the compatibility path command. Add:

```text
goregraph dashboard path [path]
goregraph dashboard open [path]
goregraph workspace dashboard path [path]
goregraph workspace dashboard open [path]
```

`open` is a CLI convenience only and is not used by tests or agent guidance. It must fail clearly in headless environments. `workspace dashboard [path]` without `path|open` continues to print the HTML path.

For a project, `dashboard path` prints the `goregraph-out/dashboard/` directory and `dashboard open` opens its primary `report.md`; it does not imply an interactive project HTML dashboard. For a workspace, both commands resolve `.goregraph-workspace/dashboard/workspace-map.html`.

- [ ] **Step 7: Migrate every reader and maintenance command**

Replace hard-coded flat paths in:

- project report/query/explain operations;
- workspace explain/path/impact/diff;
- workspace dashboard path resolution;
- symbol and usage expert operations;
- freshness/evidence/capability validation;
- MCP specialist tools;
- project/workspace discovery;
- clean plans and release-file assertions.

Doctor behavior:

- validates `manifest.json` and only the selected complete projections;
- validates every canonical index file under `index/`;
- validates `agent/context-index.json` only when `agent.complete`;
- validates dashboard HTML/assets/reports only when `dashboard.complete`;
- reports a clear migration error when it sees pre-1.3.0 files such as root-level `routes.json`, `report.md`, `workspace-map.html`, or `context-index.json`;
- prints `goregraph workspace clean . --execute` plus `goregraph workspace build all .` for a workspace legacy layout, or the corresponding project commands for a project layout.

Clean behavior removes the entire configured project output and `.goregraph-workspace` trees, so both current and legacy layouts are covered.

- [ ] **Step 8: Run focused and regression tests**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/scan ./internal/cli ./internal/doctor ./internal/query ./internal/agent -count=1
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./... -run 'TestRunScan|TestRunWorkspaceScanAll|TestRunWorkspaceRefresh|TestRunWorkspaceDashboard|TestRunReport|TestRunQuery|TestWorkspace(Symbol|Dashboard|Path|Impact|Diff)' -count=1
git diff --check
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/scan internal/cli/cli.go internal/cli/cli_test.go internal/doctor internal/query/query.go internal/agent/service.go release_files_test.go
git commit -m "Separate agent and dashboard outputs" -m "- Add target-aware project and workspace builds
- Write canonical indexes, compact AI context, and human reports to isolated trees
- Preserve scan commands as all-target compatibility aliases
- Migrate maintenance and query readers to the new layout"
```

---

### Task 2: Freeze the Compact Context Index Contract

**Files:**
- Create: `internal/scan/agent_context_index.go`
- Create: `internal/scan/agent_context_index_test.go`
- Modify: `internal/scan/scan.go`
- Modify: `release_files_test.go`

**Interfaces:**
- Consumes: existing `RichSymbolRecord`, `RichRelationRecord`, `CodeRouteRecord`, `CodeFlowRecord`, `TestMapRecord`, `APIContractRecord`, `EvidenceRecord`, and `CapabilityRecord`.
- Produces: `AgentContextIndexRecord`, stable fact/edge IDs, and `BuildProjectAgentContextIndex`.

- [ ] **Step 1: Write failing stable-contract and deterministic-order tests**

Add:

```go
func TestBuildProjectAgentContextIndexIsCompactAndDeterministic(t *testing.T) {
	routes := []CodeRouteRecord{{
		RouteID: "route:delete", HTTPMethod: "DELETE",
		Path: "/cadasters/{cadasterId}/regulations/{objectId}",
		Handler: "CadasterRegulationController.deleteFromCadaster",
		File: "src/CadasterRegulationController.java", Line: 182,
		Confidence: "EXACT", EvidenceIDs: []string{"evidence:route"},
	}}
	symbols := []RichSymbolRecord{{
		ID: "symbol:operations", Name: "deleteRegulationFromCadaster",
		QualifiedName: "CadasterRegulationOperationsService.deleteRegulationFromCadaster",
		Kind: "method", Language: "java",
		File: "src/CadasterRegulationOperationsService.java", Line: 45,
		Confidence: ConfidenceExact, EvidenceIDs: []string{"evidence:symbol"},
	}, {
		ID: "symbol:local-helper", Name: "formatDebugMessage",
		QualifiedName: "CadasterRegulationOperationsService.formatDebugMessage",
		Kind: "method", Language: "java",
		File: "src/CadasterRegulationOperationsService.java", Line: 90,
		Confidence: ConfidenceExact,
	}}
	relations := []RichRelationRecord{{
		ID: "relation:delete", From: "src/CadasterRegulationController.java",
		To: "CadasterRegulationOperationsService.deleteRegulationFromCadaster",
		Type: "call", Line: 201, Confidence: "EXACT",
		Reason: "qualified method call", EvidenceIDs: []string{"evidence:call"},
	}}

	first := BuildProjectAgentContextIndex("ms-cadasterregulation", "2026-07-16T00:00:00Z",
		routes, nil, symbols, relations, nil, nil, nil, nil)
	second := BuildProjectAgentContextIndex("ms-cadasterregulation", "2026-07-16T00:00:00Z",
		routes, nil, symbols, relations, nil, nil, nil, nil)

	if diff := cmpJSON(first, second); diff != "" {
		t.Fatalf("context index is not deterministic: %s", diff)
	}
	if len(first.Facts) != 2 || len(first.Edges) != 1 {
		t.Fatalf("context index = %#v", first)
	}
	if hasContextFact(first.Facts, "symbol", "formatDebugMessage") {
		t.Fatalf("isolated local helper leaked into compact context: %#v", first.Facts)
	}
	for _, fact := range first.Facts {
		if fact.Search == "" {
			t.Fatalf("fact missing compact search aliases: %#v", fact)
		}
	}
}

func TestContextIndexOutputIsPartOfReleaseContract(t *testing.T) {
	if !slices.Contains(AgentGeneratedFiles, "context-index.json") {
		t.Fatal("agent/context-index.json is not registered as agent output")
	}
}
```

Use the repository's existing JSON comparison helper pattern; if no shared helper exists, define this test-local helper:

```go
func cmpJSON(left, right any) string {
	leftBody, _ := json.Marshal(left)
	rightBody, _ := json.Marshal(right)
	if string(leftBody) == string(rightBody) {
		return ""
	}
	return string(leftBody) + " != " + string(rightBody)
}
```

- [ ] **Step 2: Run the focused tests and observe RED**

Run:

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/scan . -run 'TestBuildProjectAgentContextIndexIsCompactAndDeterministic|TestContextIndexOutputIsPartOfReleaseContract' -count=1
```

Expected: compilation failure because the context index records and builder do not exist.

- [ ] **Step 3: Implement the public records, normalization, stable IDs, and project builder**

Implement:

```go
func BuildProjectAgentContextIndex(
	project string,
	generated string,
	routes []CodeRouteRecord,
	flows []CodeFlowRecord,
	symbols []RichSymbolRecord,
	relations []RichRelationRecord,
	tests []TestMapRecord,
	contracts []APIContractRecord,
	evidence []EvidenceRecord,
	capabilities []CapabilityRecord,
) AgentContextIndexRecord
```

Rules:

1. Route facts use kind `route`, name `METHOD path`, qualified handler, route source location, route evidence, and one deduplicated space-separated `Search` string built from method, path segments, handler, file base name, and camel-case aliases.
2. Keep type-level declarations needed for class and usage navigation: Java/Kotlin classes, interfaces, records, enums, and annotations; JS/TS exported classes, components, hooks, and named exported types.
3. Keep method/function declarations only when they are a route handler, API caller, flow step, persistence step, test target, or one endpoint of a cross-file/cross-project semantic relation. Exclude isolated local helpers.
4. Test facts use kind `test` and target method/path aliases. API client facts use kind `api_contract`. Flow steps containing `repository`, `persistence`, `entity`, `database`, or `store` become kind `persistence`.
5. Store evidence IDs only. Never embed evidence reasons, snippets, source hashes, or complete evidence records in the compact index.
6. Retain an edge only when both endpoint facts are retained and it represents `call`, `use`, `implements`, `extends`, HTTP contract, persistence, test target, or an existing semantic flow transition. Exclude import-only, lexical-only, duplicate, and unresolved same-file relations.
7. Prefer `FromSymbolID` and `ToSymbolID` over label matching. Keep cross-file usage edges so a class/symbol query can answer where the symbol is used without loading `symbol-usages.json`.
8. Capability records become compact coverage entries only for `routes`, `calls`, `tests`, `api_clients`, and `persistence`; exclude Maven/text/YAML language rows.
9. Stable IDs use the existing `stableID` helper with every identity field.
10. Sort facts by `Project`, `Kind`, `Qualified`, `Name`, `File`, `Line`, then `ID`. Sort edges by `FromLabel`, `ToLabel`, `Kind`, `File`, `Line`, then `ID`.

Add `"context-index.json"` to `AgentGeneratedFiles` and the release-file assertions. Do not add it to the shared `index/` or dashboard contracts.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the Step 2 command.

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/agent_context_index.go internal/scan/agent_context_index_test.go internal/scan/scan.go release_files_test.go
git commit -m "Add compact agent context index contract" -m "- Define deterministic navigation facts and relationships
- Register agent/context-index.json in the Schema 3 agent projection
- Cover compact search terms and stable ordering"
```

---

### Task 3: Generate Project and Workspace Agent Context Indexes

**Files:**
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/scan_test.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_reconcile_test.go`
- Modify: `internal/doctor/doctor.go`
- Create: `internal/doctor/context_index_test.go`

**Interfaces:**
- Consumes: `BuildProjectAgentContextIndex` from Task 2 and existing workspace registry, contract matches, feature dossiers, and endpoint traces.
- Produces: `goregraph-out/agent/context-index.json` and `.goregraph-workspace/agent/context-index.json`.

- [ ] **Step 1: Write failing scan and workspace output tests**

Add a project scan assertion:

```go
func TestScanWritesCompactAgentContextIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/UserController.java", `
@RestController
class UserController {
  @DeleteMapping("/users/{id}")
  void deleteUser() {}
}`)

	if _, err := RunBuild(root, config.Defaults(), BuildTargetAgent); err != nil {
		t.Fatal(err)
	}

	var index AgentContextIndexRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "agent", "context-index.json"), &index)
	if !hasContextFact(index.Facts, "route", "DELETE /users/{id}") {
		t.Fatalf("context facts = %#v", index.Facts)
	}
}
```

Add a workspace merge assertion:

```go
func TestWorkspaceReconciliationMergesProjectAndCrossProjectContext(t *testing.T) {
	workspace, frontend, backend := writeWorkspaceFixture(t)
	writeProjectContextIndex(t, frontend, AgentContextIndexRecord{
		SchemaVersion: SchemaVersion,
		Facts: []AgentContextFactRecord{{
			ID: "frontend:api", Project: "frontend/app", Kind: "api_contract",
			Name: "DELETE /users/{id}", File: "src/api/users.ts", Line: 8,
		}},
	})
	writeProjectContextIndex(t, backend, AgentContextIndexRecord{
		SchemaVersion: SchemaVersion,
		Facts: []AgentContextFactRecord{{
			ID: "backend:route", Project: "services/users", Kind: "route",
			Name: "DELETE /users/{id}", File: "UserController.java", Line: 20,
		}},
	})

	if _, err := ReconcileWorkspaceTarget(frontend, config.Config{Workspace: true, WorkspaceRoot: workspace}, BuildTargetAgent); err != nil {
		t.Fatal(err)
	}

	var index AgentContextIndexRecord
	readJSON(t, filepath.Join(workspace, ".goregraph-workspace", "agent", "context-index.json"), &index)
	if !hasContextEdge(index.Edges, "frontend:api", "backend:route", "http") {
		t.Fatalf("workspace edges = %#v", index.Edges)
	}
}
```

Add Doctor assertions that malformed JSON, duplicate fact IDs, and dangling `FromFactID`/`ToFactID` fail.

- [ ] **Step 2: Run the focused tests and observe RED**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/scan ./internal/doctor -run 'TestScanWritesCompactAgentContextIndex|TestWorkspaceReconciliationMergesProjectAndCrossProjectContext|TestDoctor.*ContextIndex' -count=1
```

Expected: FAIL because scans and reconciliation do not write the index.

- [ ] **Step 3: Write the project index during scanning**

In `scan.RunBuild`, after rich symbols, relations, routes, flows, tests, contracts, evidence, and capabilities are finalized, call this only when the selected target includes `agent`:

```go
contextIndex := BuildProjectAgentContextIndex(
	projectName,
	generated,
	routes,
	flows,
	richSymbols,
	richRelations,
	testMap,
	apiContracts,
	evidence,
	capabilities,
)
```

Write it with the output layout helper:

```go
if err := writeJSON(layout.Agent("context-index.json"), contextIndex); err != nil {
	return Result{}, err
}
```

- [ ] **Step 4: Merge workspace facts without reading dashboard artifacts**

Add:

```go
func BuildWorkspaceAgentContextIndex(
	registry WorkspaceRegistryRecord,
	projectIndexes []AgentContextIndexRecord,
	matches []WorkspaceContractMatchRecord,
	dossiers []FeatureDossierRecord,
	traces WorkspaceEndpointTraceIndexRecord,
	generated string,
) AgentContextIndexRecord
```

Merge all already-pruned project facts and edges after prefixing project-local IDs with the project path. Add:

- `http_contract` edges from matched `WorkspaceContractMatchRecord` values;
- dossier facts for backend handlers, authentication, persistence, and tests;
- endpoint-trace edges for frontend/API/backend step transitions;
- workspace coverage only for relevant projects/capabilities;
- deterministic deduplication by stable ID.

Do not read or reference:

```text
.goregraph-workspace/dashboard/workspace-map.html
.goregraph-workspace/dashboard/workspace-map-assets/
.goregraph-workspace/index/symbol-usages.json
```

Write `.goregraph-workspace/agent/context-index.json` in `ReconcileWorkspaceTarget` only when the target includes `agent`. The implementation may use in-memory reconciliation records and project `agent/context-index.json`; it must not load the dashboard or the full workspace symbol-usage projection.

- [ ] **Step 5: Validate the context index in Doctor**

Doctor checks:

- schema equals `scan.SchemaVersion`;
- all fact and edge IDs are non-empty and unique;
- every referenced fact ID exists;
- files are workspace-relative or project-relative, never absolute;
- line values are non-negative;
- facts and edges are deterministically sorted.

- [ ] **Step 6: Run focused tests and verify GREEN**

Run the Step 2 command.

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/scan/scan.go internal/scan/scan_test.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/doctor/doctor.go internal/doctor/context_index_test.go
git commit -m "Generate compact context indexes" -m "- Write project navigation facts during scans
- Merge workspace contracts, dossiers, and traces
- Validate context indexes without loading dashboard artifacts"
```

---

### Task 4: Build the Budgeted Context Compiler

**Files:**
- Create: `internal/agent/context.go`
- Create: `internal/agent/context_load.go`
- Create: `internal/agent/context_rank.go`
- Create: `internal/agent/context_test.go`
- Modify: `internal/agent/types.go`
- Modify: `internal/agent/service.go`

**Interfaces:**
- Consumes: one `AgentContextIndexRecord`.
- Produces: `BuildContext(ContextRequest) (ContextPack, error)` and compatibility delegation from `task-context`.

- [ ] **Step 1: Write failing validation, ranking, fallback, and budget tests**

Add:

```go
func TestBuildContextRejectsInvalidBounds(t *testing.T) {
	for _, request := range []ContextRequest{
		{Root: t.TempDir(), Query: "delete user", BudgetTokens: 255},
		{Root: t.TempDir(), Query: "delete user", BudgetTokens: 4001},
		{Root: t.TempDir(), Query: "delete user", MaxFiles: 21},
		{Root: t.TempDir(), Query: "   "},
	} {
		if _, err := BuildContext(request); err == nil {
			t.Fatalf("accepted invalid request: %#v", request)
		}
	}
}

func TestBuildContextRanksExactRouteAndExpandsCallChain(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated: "2026-07-16T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "regulation", Kind: "route", Name: "DELETE /cadasters/{cadasterId}/regulations/{objectId}", Qualified: "CadasterRegulationController.deleteFromCadaster", File: "Controller.java", Line: 182, Confidence: "EXACT", Search: "delete cadaster regulation"},
			{ID: "service", Project: "regulation", Kind: "symbol", Name: "deleteRegulationFromCadaster", Qualified: "CadasterRegulationOperationsService.deleteRegulationFromCadaster", File: "OperationsService.java", Line: 45, Confidence: "EXACT", Search: "delete regulation cadaster"},
			{ID: "test", Project: "regulation", Kind: "test", Name: "testDeleteFromCadaster_okay", File: "ControllerDeleteTest.java", Line: 59, Confidence: "RESOLVED", Search: "delete cadaster regulation test"},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "edge", FromFactID: "route", ToFactID: "service",
			FromLabel: "CadasterRegulationController.deleteFromCadaster",
			ToLabel: "CadasterRegulationOperationsService.deleteRegulationFromCadaster",
			Kind: "calls", Confidence: "EXACT",
		}},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "remove regulation from cadaster",
		BudgetTokens: 1800, MaxFiles: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.Entrypoints) == 0 {
		t.Fatalf("pack = %#v", pack)
	}
	if pack.Entrypoints[0].ID != "route" || len(pack.CallChain) != 1 {
		t.Fatalf("ranked pack = %#v", pack)
	}
	if pack.EstimatedTokens > pack.BudgetTokens || len(pack.Files) > 12 {
		t.Fatalf("unbounded pack = %#v", pack)
	}
}

func TestBuildContextFallsBackWithoutQueryCascade(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{{
			ID: "unrelated", Kind: "symbol", Name: "InvoiceService",
			File: "InvoiceService.java", Line: 10,
			Search: "invoice service",
		}},
	})

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "remove regulation tasks",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || pack.FallbackReason == "" {
		t.Fatalf("fallback pack = %#v", pack)
	}
	if len(pack.Uncertainties) > 1 || pack.EstimatedTokens > 256 {
		t.Fatalf("fallback must be tiny: %#v", pack)
	}
}
```

- [ ] **Step 2: Run the focused tests and observe RED**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/agent -run 'TestBuildContext' -count=1
```

Expected: compilation failure because the Context Pack compiler does not exist.

- [ ] **Step 3: Implement exact request normalization and one-file loading**

`normalizeContextRequest` must:

```go
func normalizeContextRequest(request ContextRequest) (ContextRequest, error) {
	if strings.TrimSpace(request.Root) == "" {
		request.Root = "."
	}
	request.Query = strings.TrimSpace(request.Query)
	if request.Query == "" {
		return ContextRequest{}, fmt.Errorf("context query is required")
	}
	if request.BudgetTokens == 0 {
		request.BudgetTokens = DefaultContextBudgetTokens
	}
	if request.MaxFiles == 0 {
		request.MaxFiles = DefaultContextMaxFiles
	}
	if request.BudgetTokens < MinContextBudgetTokens || request.BudgetTokens > MaxContextBudgetTokens {
		return ContextRequest{}, fmt.Errorf("budget-tokens must be between %d and %d", MinContextBudgetTokens, MaxContextBudgetTokens)
	}
	if request.MaxFiles < 1 || request.MaxFiles > MaxContextMaxFiles {
		return ContextRequest{}, fmt.Errorf("max-files must be between 1 and %d", MaxContextMaxFiles)
	}
	return request, nil
}
```

`loadContextIndex` checks, in order:

1. `<root>/.goregraph-workspace/agent/context-index.json`;
2. detected workspace root `.goregraph-workspace/agent/context-index.json`;
3. configured project output `agent/context-index.json`.

It reads exactly one successful file.

- [ ] **Step 4: Implement deterministic lexical ranking**

Tokenization rules:

- lowercase Unicode letters and digits;
- split punctuation, paths, dots, slashes, underscores, and hyphens;
- split camel-case before lowercasing;
- discard tokens shorter than two characters except HTTP verbs;
- retain full normalized route and qualified-name strings as exact terms;
- tokenize the compact `Search` aliases together with `Name`, `Qualified`, `HTTPMethod`, `Path`, and `Summary`;
- no stemming, embeddings, model calls, or network access.

Scoring:

```go
const (
	scoreExactRoute       = 1000
	scoreExactQualified   = 900
	scoreExactName        = 800
	scoreAllTerms         = 500
	scorePerMatchedTerm   = 60
	scoreRouteKind        = 80
	scoreSymbolKind       = 60
	scoreTestKind         = 20
	scoreExactConfidence  = 30
	scoreResolvedConfidence = 15
)
```

Select at most three seed facts with score at least `180`. Expand one edge hop in both directions, then add tests, contracts, persistence, and files connected to selected IDs. Never broaden to unrelated facts merely to fill the budget.

Confidence:

- `EXACT` when the top fact is an exact route, qualified name, or exact name and has `EXACT` confidence;
- `HIGH` when all normalized query terms match the top fact and at least one relationship is expanded;
- `MEDIUM` when the top score is at least `240`;
- `LOW` otherwise.

Set `fallback_required` when confidence is `LOW`, no entrypoint exists, or every selected fact comes from incomplete/failed scoped coverage.

- [ ] **Step 5: Implement budget enforcement and scoped uncertainty**

Add `EstimateContextTokens` exactly as specified in the public contract.

Build order:

1. mandatory envelope;
2. top entrypoint;
3. direct call-chain relationships;
4. contracts and persistence;
5. tests;
6. remaining files;
7. at most three uncertainties.

Before appending each optional item, clone the pack, append the item, estimate tokens, and keep it only if it remains within `BudgetTokens`. Deduplicate files by `Project + Path`; merge line bounds and roles rather than adding duplicates.

Only include coverage uncertainty whose project and capability intersect the selected facts. Compress it to:

```text
<project>/<capability>: <coverage> — <reason>
```

- [ ] **Step 6: Delegate legacy task-context to BuildContext**

Add `BudgetTokens` and `MaxFiles` to `agent.Request`. In `loadTask`, replace the old `loadTaskContext` branch with a compatibility wrapper around `BuildContext`. The wrapped `Result`:

- contains one `task_context` item;
- stores the complete `ContextPack` under `Data["context"]`;
- has no global `CoverageWarnings`;
- has no `SuggestedNext`;
- uses Context Pack freshness.

Delete the old `TaskContextRecord` and `loadTaskContext` after all focused tests have been updated.

- [ ] **Step 7: Run focused tests and verify GREEN**

Run the Step 2 command plus:

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/agent -run 'TestServiceBuildsBoundedTaskContext|TestTaskContext' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/agent/context.go internal/agent/context_load.go internal/agent/context_rank.go internal/agent/context_test.go internal/agent/types.go internal/agent/service.go internal/agent/service_test.go
git commit -m "Compile budgeted agent context" -m "- Rank one compact context index for a natural-language task
- Enforce token, file, uncertainty, and fallback bounds
- Route legacy task-context through the same compiler"
```

---

### Task 5: Add Compact Rendering and the Context CLI

**Files:**
- Create: `internal/query/context.go`
- Create: `internal/query/context_test.go`
- Modify: `internal/query/query.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `agent.BuildContext`.
- Produces: `query.RunContext` and `goregraph context`.

- [ ] **Step 1: Write failing JSON, Markdown, CLI help, and byte-budget tests**

Add:

```go
func TestRenderContextMarkdownIsCompactAndActionable(t *testing.T) {
	pack := agent.ContextPack{
		Schema: 2, Query: "delete user", Confidence: "EXACT",
		Entrypoints: []agent.ContextLocation{{
			ID: "route", Kind: "route", Label: "DELETE /users/{id}",
			File: "UserController.java", Line: 20, Confidence: "EXACT",
		}},
		Files: []agent.ContextFile{{
			Path: "UserController.java", StartLine: 20, EndLine: 28,
			Role: "entrypoint", Reason: "exact route match",
		}},
		EstimatedTokens: 120, BudgetTokens: 1800,
	}
	body := RenderContextMarkdown(pack)
	for _, want := range []string{
		"# GoreGraph Context",
		"Confidence: EXACT",
		"`UserController.java:20-28`",
		"Fallback required: no",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("markdown missing %q:\n%s", want, body)
		}
	}
	for _, forbidden := range []string{"WARNING:", "Suggested next:", "maven /", "text /", "yaml /"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("markdown contains %q:\n%s", forbidden, body)
		}
	}
}

func TestContextCLIAcceptsBudgetAndMaxFiles(t *testing.T) {
	root := writeCLIContextFixture(t)
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"context", root,
		"--query", "DELETE /users/{id}",
		"--budget-tokens", "900",
		"--max-files", "6",
		"--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{`"budget_tokens": 900`, `"fallback_required": false`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] **Step 2: Run focused tests and observe RED**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/query ./internal/cli -run 'TestRenderContext|TestContextCLI' -count=1
```

Expected: compilation failure because rendering and CLI command do not exist.

- [ ] **Step 3: Implement `query.RunContext` and compact renderers**

Add:

```go
type ContextOptions struct {
	Root         string
	Query        string
	Format       string
	BudgetTokens int
	MaxFiles     int
}

func RunContext(options ContextOptions) (string, error) {
	pack, err := agent.BuildContext(agent.ContextRequest{
		Root: options.Root, Query: options.Query,
		BudgetTokens: options.BudgetTokens, MaxFiles: options.MaxFiles,
	})
	if err != nil {
		return "", err
	}
	if options.Format == "" || options.Format == "markdown" {
		return RenderContextMarkdown(pack), nil
	}
	if options.Format != "json" {
		return "", fmt.Errorf("context format must be json or markdown")
	}
	body, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return "", err
	}
	return string(body) + "\n", nil
}
```

Markdown sections are emitted only when non-empty and in this order:

1. heading and envelope;
2. entrypoints;
3. call chain;
4. contracts;
5. persistence;
6. tests;
7. files to inspect;
8. uncertainties;
9. fallback reason.

- [ ] **Step 4: Implement `goregraph context`**

Usage:

```text
goregraph context <path> --query <task> [--budget-tokens 1800] [--max-files 12] [--format markdown|json]
```

Add it to global help before `query`. Parse every option explicitly. Unknown options and missing values return exit code `2`; compiler failures return exit code `1`.

Extend agent-query option parsing so:

```text
goregraph query <path> task-context --query ... --budget-tokens ... --max-files ...
```

delegates to the same compiler. Existing `--limit` remains accepted for other tasks; for `task-context`, it is ignored when `--max-files` is supplied and otherwise maps to `MaxFiles` capped at 20 for compatibility.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run the Step 2 command and:

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/query ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/query/context.go internal/query/context_test.go internal/query/query.go internal/cli/cli.go internal/cli/cli_test.go
git commit -m "Add the budgeted context command" -m "- Render compact JSON and Markdown Context Packs
- Expose explicit token and file budgets in the CLI
- Keep task-context as a compatibility alias"
```

---

### Task 6: Make MCP Minimal by Default

**Files:**
- Modify: `internal/mcp/mcp.go`
- Modify: `internal/mcp/mcp_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `agent.BuildContext` and `ContextRequest`.
- Produces: standard MCP with exactly `task_context`, and expert MCP with the legacy tool set.

- [ ] **Step 1: Write failing default/expert tool-list and bounded-call tests**

Add:

```go
func TestDefaultMCPListsOnlyTaskContext(t *testing.T) {
	listed := tools(Options{})
	if len(listed) != 1 || listed[0]["name"] != "task_context" {
		t.Fatalf("default tools = %#v", listed)
	}
}

func TestExpertMCPRetainsLegacyTools(t *testing.T) {
	listed := tools(Options{ExpertTools: true})
	names := toolNames(listed)
	for _, want := range []string{
		"task_context", "coverage", "diagnostics",
		"symbol_resolve", "symbol_usages", "symbol_api_consumers",
	} {
		if !slices.Contains(names, want) {
			t.Fatalf("expert tools missing %s: %#v", want, names)
		}
	}
}

func TestMCPTaskContextPassesBudgetsToCompiler(t *testing.T) {
	root := writeMCPContextFixture(t)
	text, err := callTool(Options{}, "task_context", map[string]any{
		"root": root, "query": "DELETE /users/{id}",
		"budget_tokens": float64(700), "max_files": float64(5),
	})
	if err != nil {
		t.Fatal(err)
	}
	var pack agent.ContextPack
	if err := json.Unmarshal([]byte(text), &pack); err != nil {
		t.Fatal(err)
	}
	if pack.BudgetTokens != 700 || len(pack.Files) > 5 || pack.EstimatedTokens > 700 {
		t.Fatalf("pack = %#v", pack)
	}
}
```

- [ ] **Step 2: Run focused tests and observe RED**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/mcp ./internal/cli -run 'TestDefaultMCP|TestExpertMCP|TestMCPTaskContext|TestMCPHelp' -count=1
```

Expected: compilation or assertion failure because MCP has no options and lists all tools.

- [ ] **Step 3: Add explicit server options**

Add:

```go
type Options struct {
	ExpertTools bool
}

func Serve(input io.Reader, output io.Writer) error {
	return ServeWithOptions(input, output, Options{})
}

func ServeWithOptions(input io.Reader, output io.Writer, options Options) error
```

Pass `options` through `handle`, `tools`, and `callTool`.

Default tool schema:

```go
func taskContextTool() map[string]any {
	return map[string]any{
		"name": "task_context",
		"description": "Return one compact, budgeted Context Pack for a coding task. If fallback_required is true, stop using GoreGraph and inspect source directly. Call at most twice per task.",
		"inputSchema": map[string]any{
			"type": "object",
			"additionalProperties": false,
			"required": []string{"query"},
			"properties": map[string]any{
				"root": map[string]any{"type": "string"},
				"query": map[string]any{"type": "string", "minLength": 1},
				"budget_tokens": map[string]any{"type": "integer", "minimum": 256, "maximum": 4000, "default": 1800},
				"max_files": map[string]any{"type": "integer", "minimum": 1, "maximum": 20, "default": 12},
			},
		},
	}
}
```

`task_context` calls `agent.BuildContext` directly and returns the `ContextPack` JSON. It does not wrap the pack in `agent.Result`.

Expert mode returns `task_context` first, then all current legacy tools. Legacy calls continue to use existing implementations.

- [ ] **Step 4: Add `goregraph mcp --expert-tools`**

Help text:

```text
Usage: goregraph mcp [--expert-tools]

Starts the read-only MCP stdio server.
Default mode exposes only task_context to prevent query cascades.
--expert-tools exposes legacy diagnostic and exploration tools.
```

Unknown options return exit code `2`.

- [ ] **Step 5: Run focused and full MCP tests**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/mcp ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/mcp.go internal/mcp/mcp_test.go internal/cli/cli.go internal/cli/cli_test.go
git commit -m "Make task context the default MCP surface" -m "- Expose one bounded tool in standard mode
- Keep specialist tools behind explicit expert mode
- Pass token and file budgets directly to the compiler"
```

---

### Task 7: Replace the Query-Cascade Agent Guide

**Files:**
- Modify: `internal/scan/agent_reports.go`
- Modify: `internal/scan/agent_reports_test.go`
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/RELEASE.md`

**Interfaces:**
- Consumes: the Context CLI and MCP contract.
- Produces: generated agent guidance that allows one normal call and one exact retry only.

- [ ] **Step 1: Write failing agent-guide contract tests**

Add:

```go
func TestAgentGuideUsesOneBoundedContextWorkflow(t *testing.T) {
	guide := renderAgentGuideEntry()
	for _, want := range []string{
		"goregraph context . --query",
		"--budget-tokens 1800",
		"--max-files 12",
		"MCP: `task_context`",
		"fallback_required",
		"at most one narrower retry",
	} {
		if !strings.Contains(guide, want) {
			t.Fatalf("agent guide missing %q:\n%s", want, guide)
		}
	}
	for _, forbidden := range []string{
		"goregraph query . coverage",
		"goregraph query . service-context",
		"goregraph query . tests",
		"goregraph query . diagnostics",
		"goregraph query . data-flow",
		"goregraph query . evidence",
	} {
		if strings.Contains(guide, forbidden) {
			t.Fatalf("agent guide still promotes query cascade %q:\n%s", forbidden, guide)
		}
	}
}
```

- [ ] **Step 2: Run the focused test and observe RED**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/scan -run TestAgentGuideUsesOneBoundedContextWorkflow -count=1
```

Expected: FAIL because the current guide advertises the full query cascade.

- [ ] **Step 3: Replace the generated guide**

The guide must state:

````markdown
# GoreGraph Agent Guide

Use GoreGraph once to obtain a small navigation pack, then verify the cited source.

```bash
goregraph context . --query "<current coding task>" --budget-tokens 1800 --max-files 12
```

MCP: `task_context`

- Read only the cited file ranges required to verify the answer.
- If `fallback_required` is true, stop using GoreGraph and inspect source directly.
- If confidence is not low and the result returns one exact route or symbol, at most one narrower retry with that exact value is allowed.
- Do not call coverage, diagnostics, tests, data-flow, evidence, or symbol tools in sequence.
- Read generated AI context only from `goregraph-out/agent/` or `.goregraph-workspace/agent/`.
- Do not read `index/`, `dashboard/`, dashboard assets, or `index/symbol-usages.json` as AI context.
- Run `goregraph doctor .` only when the context command reports missing or stale output.
````

- [ ] **Step 4: Document dashboard/AI separation**

Documentation must clearly say:

- Dashboard remains the full human exploration surface.
- `agent/context-index.json`, `agent/agent-guide.md`, and bounded Context Packs are the only recommended AI input.
- `index/` is GoreGraph's complete shared machine index and is not intended for direct prompt ingestion.
- `dashboard/` is the human projection; Code Explorer remains available there.
- Single-project `build dashboard` produces the existing human-readable reports. The interactive dashboard remains workspace-only in 1.3.0.
- Document `build agent|dashboard|all`, both workspace forms, target-aware update/refresh, and the two compatibility aliases.
- Document the exact output trees and the rule that source extraction is shared rather than repeated.
- Explain that single-project `goregraph build ... .` commands do not require a workspace marker. Workspace-wide commands require either an auto-detectable grouped layout, explicit `--workspace <root>`, or `.goregraph-workspace.yml` at the workspace root; scanning once does not create a persistent marker implicitly.
- Keep the improved flat-layout error and help text aligned with that marker rule.
- Standard MCP has one tool; expert mode exists for manual diagnostics.
- Context token estimates are approximate; benchmark totals are authoritative.
- Existing specialist CLI queries remain supported but are not part of the normal agent workflow.

Update the README integration-depth table so the AI row describes `Context Pack` rather than “all generated outputs”.

- [ ] **Step 5: Run documentation and guide tests**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/scan . -run 'TestAgentGuideUsesOneBoundedContextWorkflow|TestReleaseFiles' -count=1
git diff --check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scan/agent_reports.go internal/scan/agent_reports_test.go README.md COMMANDS.md docs/OUTPUTS.md docs/RELEASE.md
git commit -m "Replace the agent query cascade" -m "- Guide agents through one bounded Context Pack
- Define immediate fallback and one exact retry
- Separate dashboard exploration from AI context"
```

---

### Task 8: Add Reproducible Token and Quality Benchmarks

**Files:**
- Create: `scripts/benchmark-agent-context.sh`
- Create: `docs/BENCHMARKING.md`
- Create: `internal/agent/context_size_test.go`
- Modify: `README.md`
- Modify: `docs/RELEASE.md`

**Interfaces:**
- Consumes: external prepared workspace, one base prompt file, one GoreGraph instruction file, and installed `codex`/`goregraph`.
- Produces: raw run logs and `summary.tsv` containing token totals and medians.

- [ ] **Step 1: Write the deterministic Context Pack size test**

Add:

```go
func TestDefaultContextPackStaysWithinTokenAndByteBudgets(t *testing.T) {
	root := writeDenseContextIndexFixture(t, 200)
	pack, err := BuildContext(ContextRequest{
		Root: root,
		Query: "delete regulation tasks across services",
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if pack.EstimatedTokens > DefaultContextBudgetTokens {
		t.Fatalf("estimated tokens = %d", pack.EstimatedTokens)
	}
	if len(body) > 7200 {
		t.Fatalf("serialized context = %d bytes", len(body))
	}
	if len(pack.Files) > DefaultContextMaxFiles || len(pack.Uncertainties) > 3 {
		t.Fatalf("pack bounds = %#v", pack)
	}
}
```

- [ ] **Step 2: Run the focused size test and observe RED if bounds are not enforced**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/agent -run TestDefaultContextPackStaysWithinTokenAndByteBudgets -count=1
```

Expected before final budget tuning: FAIL when the dense fixture exceeds a bound.

- [ ] **Step 3: Add the benchmark harness**

Usage:

```text
scripts/benchmark-agent-context.sh \
  --workspace /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --prompt /private/tmp/goregraph-benchmark/base-prompt.txt \
  --baseline-instruction /private/tmp/goregraph-benchmark/baseline-instruction.txt \
  --assisted-instruction /private/tmp/goregraph-benchmark/context-instruction.txt \
  --runs 3 \
  --output /private/tmp/goregraph-benchmark/results
```

The script must:

1. use `set -euo pipefail`;
2. reject non-absolute input paths;
3. require `codex`, `goregraph`, `awk`, `sed`, and `sort`;
4. run baseline and assisted variants alternately to reduce temporal bias;
5. pass the exact same neutral base prompt to both variants;
6. append only the supplied baseline instruction to baseline runs and only the supplied Context Pack instruction to assisted runs;
7. preserve the same Codex model, reasoning, sandbox, approval, and workspace arguments supplied through `CODEX_BENCHMARK_ARGS`;
8. save every raw transcript;
9. extract the final `tokens used` value;
10. write:

```text
variant	run	tokens	log
baseline	1	145700	...
assisted	1	...
```

11. calculate integer medians;
12. exit non-zero when assisted median is greater than 80% of baseline median.

The script does not score quality automatically. It prints the location of `docs/BENCHMARKING.md` and requires the reviewer to complete the rubric before release.

- [ ] **Step 4: Document the matched-prompt protocol and quality rubric**

`docs/BENCHMARKING.md` must require:

- identical workspace snapshot;
- identical model and reasoning setting;
- identical neutral base prompt with no statement that requires or forbids GoreGraph;
- no network, Git history, builds, tests, or writes when the benchmark forbids them;
- three independent baseline and assisted runs;
- alternated run order;
- median token comparison;
- raw transcript retention;
- baseline prompt may add only:

```text
Do not use the goregraph CLI, MCP tools, goregraph-out, or .goregraph-workspace files.
```

- assisted prompt may add only:

```text
Call goregraph context once with the task and its default budget.
Read only cited source needed for verification.
If fallback_required is true, stop using GoreGraph.
At most one narrower exact retry is allowed.
```

Score one point for each correctly evidenced answer:

1. public endpoint;
2. current call chain;
3. root cause;
4. required cross-repository call chain;
5. task variants;
6. lookup attributes;
7. internal API contract;
8. authentication/configuration;
9. persistence operations;
10. business side effects;
11. production/test files;
12. error, retry, and test strategy.

Release requires assisted score greater than or equal to baseline score.

- [ ] **Step 5: Tune only selection/budget logic until the size test is GREEN**

Do not remove required envelope fields. Reduce optional facts in reverse priority order: uncertainties beyond the first, secondary tests, secondary persistence facts, secondary contracts, remaining files, then lower-ranked entrypoints.

Run:

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./internal/agent -run TestDefaultContextPackStaysWithinTokenAndByteBudgets -count=1
bash -n scripts/benchmark-agent-context.sh
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add scripts/benchmark-agent-context.sh docs/BENCHMARKING.md internal/agent/context_size_test.go README.md docs/RELEASE.md
git commit -m "Gate agent context on token savings" -m "- Add matched-prompt benchmark automation
- Define a twelve-point evidence quality rubric
- Enforce default Context Pack token and byte ceilings"
```

---

### Task 9: Complete 1.3.0 Acceptance Without Publishing

**Files:**
- Modify only if acceptance finds documentation defects: `README.md`, `COMMANDS.md`, `docs/OUTPUTS.md`, `docs/RELEASE.md`, `docs/BENCHMARKING.md`
- No version change is expected: `internal/version/version.go` must remain `1.3.0`.

**Interfaces:**
- Consumes: all prior tasks.
- Produces: verified local binary, refreshed test workspaces, benchmark evidence, and pushed `main`; no public release.

- [ ] **Step 1: Run formatting, static analysis, and the full test suite**

```bash
gofmt -w internal/scan/output_layout.go internal/scan/output_layout_test.go internal/scan/agent_context_index.go internal/scan/agent_context_index_test.go internal/agent/context.go internal/agent/context_load.go internal/agent/context_rank.go internal/agent/context_test.go internal/agent/context_size_test.go internal/query/context.go internal/query/context_test.go internal/mcp/mcp.go internal/mcp/mcp_test.go internal/cli/cli.go internal/cli/cli_test.go
test -z "$(gofmt -l .)"
env GOCACHE=/private/tmp/goregraph-gocache-context go test ./... -count=1
env GOCACHE=/private/tmp/goregraph-gocache-context go vet ./...
git diff --check
```

Expected: all commands exit `0`.

- [ ] **Step 2: Install and verify the local 1.3.0 binary**

```bash
env GOCACHE=/private/tmp/goregraph-gocache-context go install ./cmd/goregraph
goregraph version
goregraph build help
goregraph workspace build help
goregraph context help
goregraph mcp help
```

Expected:

- version is `1.3.0`;
- build help documents `agent`, `dashboard`, and `all`;
- scan help identifies `scan` and `workspace scan-all` as compatibility aliases for `all`;
- context help shows the 1,800-token and 12-file defaults;
- MCP help explains standard and expert modes.

- [ ] **Step 3: Rebuild the real Weka workspace**

```bash
cd /Users/gorecode/projects/weka
goregraph workspace clean . --execute
goregraph workspace build all .
goregraph doctor .
test "$(stat -f %z .goregraph-workspace/agent/context-index.json)" -le 16777216
```

Expected:

- every indexed project has `goregraph-out/index/`, `goregraph-out/agent/context-index.json`, and `goregraph-out/dashboard/`;
- `.goregraph-workspace/index/`, `.goregraph-workspace/agent/context-index.json`, and `.goregraph-workspace/dashboard/workspace-map.html` exist;
- project and workspace manifests report generated-output schema `3`;
- `.goregraph-workspace/agent/context-index.json` is no larger than 16 MiB;
- Doctor reports the context index valid;
- the existing dashboard still opens with all seven views and Code Explorer data from `.goregraph-workspace/index/`;
- no legacy root-level JSON, Markdown, HTML, or asset files remain below either output root.

Verify target isolation on a temporary copy or fixture:

```bash
goregraph workspace clean /private/tmp/goregraph-target-fixture --execute
goregraph workspace build agent /private/tmp/goregraph-target-fixture --workspace /private/tmp/goregraph-target-fixture
test -f /private/tmp/goregraph-target-fixture/.goregraph-workspace/agent/context-index.json
test ! -e /private/tmp/goregraph-target-fixture/.goregraph-workspace/dashboard
test ! -e /private/tmp/goregraph-target-fixture/.goregraph-workspace/index/symbol-usages.json
goregraph workspace build dashboard /private/tmp/goregraph-target-fixture --workspace /private/tmp/goregraph-target-fixture
test -f /private/tmp/goregraph-target-fixture/.goregraph-workspace/dashboard/workspace-map.html
test -f /private/tmp/goregraph-target-fixture/.goregraph-workspace/agent/context-index.json
```

Record the `agent/` modification times before the dashboard build and verify they remain unchanged. Repeat in the opposite order on a clean fixture.

- [ ] **Step 4: Verify context performance and forbidden-file independence**

Run 20 warm invocations:

```bash
for i in $(seq 1 20); do
  /usr/bin/time -p goregraph context /Users/gorecode/projects/weka \
    --query "remove regulation from cadaster and clean related tasks" \
    --format json >/private/tmp/goregraph-context-$i.json
done
```

Create a context-only fixture from the generated compact index and compare it with the real workspace:

```bash
context_fixture="$(mktemp -d /private/tmp/goregraph-context-only.XXXXXX)"
mkdir -p "$context_fixture/.goregraph-workspace/agent"
cp /Users/gorecode/projects/weka/.goregraph-workspace/agent/context-index.json \
  "$context_fixture/.goregraph-workspace/agent/context-index.json"
goregraph context /Users/gorecode/projects/weka \
  --query "remove regulation from cadaster and clean related tasks" \
  --format json >/private/tmp/goregraph-context-real.json
goregraph context "$context_fixture" \
  --query "remove regulation from cadaster and clean related tasks" \
  --format json >/private/tmp/goregraph-context-only.json
cmp /private/tmp/goregraph-context-real.json /private/tmp/goregraph-context-only.json
```

Acceptance:

- median real time at most `1.0` second;
- maximum real time at most `2.0` seconds;
- every output reports at most `1800` estimated tokens and at most `12` files;
- a context-only temporary fixture containing only `.goregraph-workspace/agent/context-index.json`, but no `index/` or `dashboard/`, returns the same Context Pack as the real workspace for the same query.

- [ ] **Step 5: Run the three-by-three matched benchmark**

Use the historical three-service workspace and the same base prompt from the recorded 145,700/212,616 comparison:

Before running, create these three uncommitted files outside the repository:

- `/private/tmp/goregraph-benchmark/base-prompt.txt`: the exact first `user` message from `/Users/gorecode/.codex/attachments/0d55d570-b04a-4b06-af78-489edede1c12/pasted-text.txt`, without transcript metadata or the following Codex response, and with only the GoreGraph-specific prohibition bullet removed so that the base prompt is tool-neutral;
- `/private/tmp/goregraph-benchmark/baseline-instruction.txt`: exactly the one-line baseline instruction specified in `docs/BENCHMARKING.md`;
- `/private/tmp/goregraph-benchmark/context-instruction.txt`: exactly the four-line GoreGraph instruction specified in `docs/BENCHMARKING.md`.

Stop acceptance if any file is absent, the base prompt contains a GoreGraph requirement/prohibition, or the base prompt differs between variants.

```bash
scripts/benchmark-agent-context.sh \
  --workspace /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --prompt /private/tmp/goregraph-benchmark/base-prompt.txt \
  --baseline-instruction /private/tmp/goregraph-benchmark/baseline-instruction.txt \
  --assisted-instruction /private/tmp/goregraph-benchmark/context-instruction.txt \
  --runs 3 \
  --output /private/tmp/goregraph-benchmark/results
```

Acceptance:

- assisted median is at most 80% of baseline median;
- assisted median is at most 116,560 when compared directly with the recorded 145,700-token baseline;
- assisted run uses at most two Context Pack calls;
- no assisted run invokes specialist GoreGraph queries;
- assisted quality rubric score is at least the baseline score;
- raw transcripts and completed rubric are retained outside the repository.

If this gate fails, do not release 1.3.0. Keep the dashboard, disable the standard MCP integration from release documentation, and decide whether to ship dashboard-only or continue context-ranking work in a later version.

- [ ] **Step 6: Verify the dashboard visually**

Open:

```text
/Users/gorecode/projects/weka/.goregraph-workspace/dashboard/workspace-map.html
```

Verify at 1280×720, 1440×900, and 1920×1080:

- Architecture, Endpoints, Feature Flow, Data Flow, Code Explorer, Diagnostics, and Coverage still load;
- service selection and Code Explorer usages remain interactive;
- no console errors;
- no layout regression;
- no dashboard content is mentioned in Context Pack output.

- [ ] **Step 7: Commit acceptance-only documentation corrections if needed**

Do not change production code during acceptance. If docs needed correction:

```bash
git add README.md COMMANDS.md docs/OUTPUTS.md docs/RELEASE.md docs/BENCHMARKING.md
git commit -m "Document 1.3.0 context acceptance" -m "- Record token and quality release gates
- Clarify dashboard and agent integration boundaries
- Preserve the unreleased source state"
```

- [ ] **Step 8: Push `main` without publishing**

```bash
git status --short
git push origin main
git rev-parse HEAD
git rev-parse origin/main
```

Expected:

- clean worktree;
- local `HEAD` equals `origin/main`;
- no tag, GitHub Release, Homebrew update, Scoop publication, or Winget publication.

---

## Final Acceptance Checklist

- [ ] Project and workspace outputs are physically separated into `index/`, `agent/`, and `dashboard/`, with only `manifest.json` at the output root.
- [ ] Generated-output schema is `3`; application version remains unreleased `1.3.0`.
- [ ] `build agent` and `build dashboard` do not create or rewrite the other projection.
- [ ] Agent-only workspace builds omit full workspace symbol index/usages and other dashboard-only heavy index components.
- [ ] `build all` performs one source extraction per project and one final workspace reconciliation.
- [ ] `scan` and `workspace scan-all` remain compatible aliases for `build all`.
- [ ] Update and workspace refresh support `--target agent|dashboard|all` and preserve non-selected projections.
- [ ] Dashboard behavior and all seven views remain unchanged under `dashboard/`.
- [ ] Project and workspace agent builds write compact `agent/context-index.json`.
- [ ] Context compilation reads exactly one compact index.
- [ ] Context compilation does not depend on `index/`, dashboard HTML/assets, or `index/symbol-usages.json`.
- [ ] The merged Weka context index is at most 16 MiB and does not duplicate the full usage projection.
- [ ] Default Context Pack is at most 1,800 estimated tokens, 7,200 bytes, 12 files, and three uncertainties.
- [ ] Low confidence produces immediate `fallback_required: true`.
- [ ] Agent guide promotes one call and at most one narrower retry.
- [ ] Standard MCP lists exactly `task_context`.
- [ ] Expert MCP retains legacy diagnostic tools.
- [ ] Existing CLI query commands remain compatible.
- [ ] Doctor detects and explains legacy flat output rather than mixing it with the new layout.
- [ ] Clean removes both new and legacy output trees safely.
- [ ] README, COMMANDS, help, OUTPUTS, and release docs describe build targets, aliases, output ownership, and the workspace marker requirement consistently.
- [ ] Full Go tests and `go vet` pass.
- [ ] Real Weka context performance meets the one-second median/two-second maximum gate.
- [ ] Three-by-three assisted benchmark saves at least 20% median end-to-end tokens.
- [ ] Assisted benchmark quality is not lower than baseline.
- [ ] GoreGraph remains local, deterministic, read-only for query/MCP operations, and dependency-free.
- [ ] Version remains unreleased `1.3.0`.
- [ ] `main` is pushed without publishing a release.
