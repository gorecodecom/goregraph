# API Catalog Agent Context and Release Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make relevant API Catalog facts useful to coding agents without increasing hard context budgets, document every dashboard workflow, and complete full local verification for unreleased 1.3.0.

**Architecture:** Workspace context indexing receives the canonical catalog and emits compact searchable endpoint/security/consumer facts. Context compilation selects at most the relevant endpoint neighborhood and bounded consumers, while dashboard configuration and full catalog data remain outside the prompt.

**Tech Stack:** Go 1.23 standard library, existing agent context index and pack compiler, Markdown documentation tests, CLI help tests, local Go install and workspace smoke scan.

## Global Constraints

- Execute after both earlier 2026-07-17 plans.
- Work directly on `main`; use failing-first TDD and focused English commits.
- Keep `DefaultContextBudgetTokens = 1800`, `MaxContextBudgetTokens = 4000`, `DefaultContextMaxFiles = 12`, and `MaxContextMaxFiles = 20` unchanged.
- Never add the full API catalog, dashboard payload, or `.goregraph-dashboard.json` to agent context.
- Prefer productive source entrypoints; tests, reports, generated output, and dashboard files remain excluded or deprioritized by existing policy.
- Keep the source target at unreleased `1.3.0`; push is allowed, publishing is forbidden.
- README, `COMMANDS.md`, CLI help, output docs, and integration-depth table must agree.

## File Structure

- Modify `internal/scan/agent_context_index.go` and tests: catalog facts and edges.
- Modify `internal/scan/workspace_reconcile.go` and tests: pass canonical catalog to workspace agent projection.
- Modify `internal/agent/context.go`, `internal/agent/context_rank.go`, `internal/agent/context_test.go`, and `internal/agent/context_size_test.go`: selected endpoint block and bounded consumers.
- Modify `internal/agent/service_test.go`, `internal/cli/cli_test.go`, and `internal/mcp/mcp_test.go`: serialization regression coverage.
- Modify `README.md`, `COMMANDS.md`, `docs/OUTPUTS.md`, `SCHEMA.md`, and `docs/RELEASE.md`.
- Modify `docs_test.go`, `release_files_test.go`, and `internal/cli/cli_test.go`: documentation/help contracts.

---

### Task 1: Compact Catalog Facts in the Agent Index

**Files:**
- Modify: `internal/scan/agent_context_index.go`
- Modify: `internal/scan/agent_context_index_test.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: `APICatalogRecord` from the canonical catalog plan.
- Changes: `BuildWorkspaceAgentContextIndex(registry WorkspaceRegistryRecord, projectIndexes []AgentContextIndexRecord, matches []WorkspaceContractMatchRecord, dossiers []FeatureDossierRecord, traces WorkspaceEndpointTraceIndexRecord, catalog APICatalogRecord, generated string) AgentContextIndexRecord`.
- Produces fact kinds: `api_endpoint`, `endpoint_security`, and `api_consumer`; edge kinds: `requires_auth` and `consumes_endpoint`.

- [ ] **Step 1: Write failing compact-index tests**

```go
func TestBuildWorkspaceAgentContextIndexAddsCompactCatalogFacts(t *testing.T) {
	catalog := APICatalogRecord{SchemaVersion: SchemaVersion, Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders", HTTPMethod: "GET", Path: "/orders/{id}",
		Handler: "OrderController.get", File: "services/orders/src/OrderController.java", Line: 20,
		Security: []SecurityEvidenceRecord{{Kind: SecurityBearer, Confidence: "EXTRACTED"}},
		Consumers: []APIConsumerRecord{{ID: "consumer:web", Project: "frontend/web", File: "frontend/web/src/api.ts", Line: 7}},
	}}}
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{Path: "services/orders", Indexed: true}, {Path: "frontend/web", Indexed: true}}}
	index := BuildWorkspaceAgentContextIndex(registry, nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed")
	if !hasContextFact(index.Facts, "api_endpoint", "GET /orders/{id}") { t.Fatalf("endpoint fact missing: %#v", index.Facts) }
	if contextIndexContains(index, ".goregraph-dashboard.json") { t.Fatal("dashboard configuration leaked into context") }
}
```

Add a size assertion showing 100 consumers produce bounded summaries/edges rather than embedding expanded evidence, and a reverse-order determinism test.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./internal/scan -run 'AgentContextIndex.*Catalog|CompactCatalog' -count=1`

Expected: FAIL because the builder does not accept catalog data.

- [ ] **Step 3: Add searchable facts without copying the full catalog**

Create one endpoint fact containing provider, method/path, handler, source, normalized security labels, confidence, and compact request/response type names in `Search`/`Summary`. Create separate security facts only when they carry useful explicit or conflicting evidence. Create consumer facts for evidenced call sites and connect them to endpoints, but cap repeated same-service call-site edges deterministically and preserve an omitted count in the endpoint summary. Never copy parameter lists, DTO fields, raw expanded evidence, dashboard fields, or config.

Pass the same already-built workspace catalog to `BuildWorkspaceAgentContextIndex`; do not reread `api-catalog.json` during reconciliation.

- [ ] **Step 4: Run index and reconciliation tests**

Run: `gofmt -w internal/scan/agent_context_index.go internal/scan/agent_context_index_test.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go && go test ./internal/scan -run 'AgentContext|WorkspaceReconcile' -count=1`

Expected: PASS and byte-identical indexes for input permutations.

- [ ] **Step 5: Commit compact catalog indexing**

```bash
git add internal/scan/agent_context_index.go internal/scan/agent_context_index_test.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go
git commit -m "Index compact endpoint context for agents" -m "- Add searchable provider, security, and consumer facts\n- Exclude expanded catalog and dashboard configuration data"
```

### Task 2: Selected Endpoint Context Pack

**Files:**
- Modify: `internal/agent/context.go`
- Modify: `internal/agent/context_rank.go`
- Modify: `internal/agent/context_test.go`
- Modify: `internal/agent/context_size_test.go`

**Interfaces:**
- Produces: `ContextEndpoint`, `ContextEndpointConsumer`, and `ContextPack.Endpoints []ContextEndpoint`.
- Preserves existing context request and budget constants.

- [ ] **Step 1: Write failing relevance, bound, and unknown-auth tests**

```go
func TestBuildContextReturnsOnlyRelevantEndpointSecurityAndBoundedConsumers(t *testing.T) {
	facts := []scan.AgentContextFactRecord{{ID: "endpoint", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders/{id}", Path: "/orders/{id}", HTTPMethod: "GET", File: "services/orders/src/OrderController.java", Line: 20, Summary: "provider services/orders; bearer; handler OrderController.get", Search: "GET /orders/{id} authentication bearer"}}
	edges := make([]scan.AgentContextEdgeRecord, 0, 25)
	for index := 0; index < 25; index++ {
		id := fmt.Sprintf("consumer:%02d", index)
		facts = append(facts, scan.AgentContextFactRecord{ID: id, Project: fmt.Sprintf("frontend/web-%02d", index), Kind: "api_consumer", Name: id, File: fmt.Sprintf("frontend/web-%02d/src/api.ts", index), Line: 7, Summary: "bearer"})
		edges = append(edges, scan.AgentContextEdgeRecord{ID: "edge:"+id, FromFactID: id, ToFactID: "endpoint", Kind: "consumes_endpoint", Reason: "bearer", Confidence: "RESOLVED"})
	}
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion, Facts: facts, Edges: edges})
	pack, err := BuildContext(ContextRequest{Root: root, Query: "who calls GET /orders/{id} and how is it authenticated"})
	if err != nil { t.Fatal(err) }
	if len(pack.Endpoints) != 1 || pack.Endpoints[0].Path != "/orders/{id}" { t.Fatalf("endpoints=%#v", pack.Endpoints) }
	if len(pack.Endpoints[0].Consumers) > 8 || pack.Endpoints[0].OmittedConsumers == 0 { t.Fatalf("consumer bounds=%#v", pack.Endpoints[0]) }
	if pack.EstimatedTokens > pack.BudgetTokens { t.Fatalf("budget exceeded: %#v", pack) }
}
```

Add tests for two same-path providers requiring provider disambiguation, unknown security wording, a query unrelated to APIs producing no endpoint block, productive source preference over tests, deterministic selection, and no dashboard/catalog JSON file in `Files`.

- [ ] **Step 2: Run context tests and verify RED**

Run: `go test ./internal/agent -run 'RelevantEndpoint|EndpointSecurity|BoundedConsumers' -count=1`

Expected: FAIL because `ContextPack` has no structured endpoint selection.

- [ ] **Step 3: Compile a budget-aware endpoint block**

Add:

```go
type ContextEndpointConsumer struct {
	Project string `json:"project"`; File string `json:"file,omitempty"`; Line int `json:"line,omitempty"`
	Authentication string `json:"authentication"`; Confidence string `json:"confidence,omitempty"`
}
type ContextEndpoint struct {
	Provider string `json:"provider"`; HTTPMethod string `json:"http_method"`; Path string `json:"path"`
	Handler string `json:"handler,omitempty"`; File string `json:"file,omitempty"`; Line int `json:"line,omitempty"`
	RequestType string `json:"request_type,omitempty"`; ResponseType string `json:"response_type,omitempty"`
	Security string `json:"security"`; SecurityConfidence string `json:"security_confidence,omitempty"`
	Consumers []ContextEndpointConsumer `json:"consumers,omitempty"`; OmittedConsumers int `json:"omitted_consumers,omitempty"`
	Limitations []string `json:"limitations,omitempty"`
}
```

Rank endpoint facts with the existing production-entrypoint scoring. Require method/path/provider evidence before choosing one among collisions. Add at most one primary endpoint by default and at most eight relevant consumers; compute omitted count from indexed summaries. Add candidates through the same clone-estimate-accept loop used by current pack sections so 1800 tokens and max files remain hard ceilings. `unknown` renders exactly `No auth evidence detected`.

- [ ] **Step 4: Run context size and determinism suites**

Run: `gofmt -w internal/agent/context.go internal/agent/context_rank.go internal/agent/context_test.go internal/agent/context_size_test.go && go test ./internal/agent -count=1`

Expected: PASS with the existing 7200-byte dense-pack ceiling and unchanged constants.

- [ ] **Step 5: Commit selected endpoint context**

```bash
git add internal/agent/context.go internal/agent/context_rank.go internal/agent/context_test.go internal/agent/context_size_test.go
git commit -m "Return relevant API facts in agent context" -m "- Select one production endpoint with bounded consumer authentication\n- Preserve existing token and file ceilings"
```

### Task 3: Task Service and CLI Serialization Regression

**Files:**
- Modify: `internal/agent/service_test.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/mcp/mcp_test.go`.

**Interfaces:**
- Consumes: `ContextPack.Endpoints` from Task 2.
- Preserves: default read-only MCP surface with only `task_context` unless expert tools are explicitly enabled.

- [ ] **Step 1: Write failing end-to-end task-context assertions**

Run `goregraph context <root> --query "GET /orders authentication"` through the CLI test harness and `task_context` through the MCP harness. Assert both serialize the same provider, method/path, security, bounded consumer list, omitted count, budget, and fallback fields. Assert neither response contains `api-catalog.json`, `.goregraph-dashboard.json`, or unrelated endpoints.

- [ ] **Step 2: Run agent/CLI/MCP focused tests and verify RED**

Run: `go test ./internal/agent ./internal/cli ./internal/mcp -run 'Endpoint.*Context|TaskContext' -count=1`

Expected: FAIL only where output shaping drops the new endpoint block.

- [ ] **Step 3: Lock the existing structured serialization path**

Add regression assertions to the existing typed `ContextPack` service item and direct MCP JSON serialization. Do not add a new MCP tool and do not make legacy exploratory tools default. Keep JSON field names identical to `ContextPack` and retain existing continuation and error semantics.

- [ ] **Step 4: Run complete interface tests**

Run: `gofmt -w internal/agent/service_test.go internal/cli/cli_test.go internal/mcp/mcp_test.go && go test ./internal/agent ./internal/cli ./internal/mcp -count=1`

Expected: PASS.

- [ ] **Step 5: Commit serialization regression coverage**

```bash
git add internal/agent/service_test.go internal/cli/cli_test.go internal/mcp/mcp_test.go
git commit -m "Lock endpoint task-context serialization" -m "- Verify compact endpoint facts across service, CLI, and MCP output\n- Keep default agent tooling bounded and read-only"
```

### Task 4: README, Commands, Help, Schema, and Output Documentation

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `SCHEMA.md`
- Modify: `docs/RELEASE.md`
- Modify: `docs_test.go`
- Modify: `release_files_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Documents: static dashboard, editor, grouping persistence, API Catalog vs Endpoints, auth semantics, compact agent context, and unreleased 1.3.0.

- [ ] **Step 1: Write failing documentation and help contract tests**

Extend documentation tests to require all of these exact concepts:

```go
required := []string{
	"goregraph workspace dashboard edit .",
	".goregraph-dashboard.json",
	"API Catalog",
	"Endpoint security",
	"consumer call authentication",
	"No auth evidence detected",
	"index/api-catalog.json",
	"agent/context-index.json",
	"1800",
	"unreleased 1.3.0",
}
```

Require the README integration-depth table to contain separate Java/Spring and JavaScript/TypeScript rows for endpoint inventory, consumers, security/auth, request/response types, dashboard, and agent context. Require help to distinguish `path`, `open`, and `edit` and state that only edit starts a loopback server.

- [ ] **Step 2: Run documentation/help tests and verify RED**

Run: `go test . ./internal/cli -run 'Docs|Documentation|Release|Help|WorkspaceDashboard' -count=1`

Expected: FAIL with missing new workflows.

- [ ] **Step 3: Update all user-facing contracts consistently**

README Quick Start must answer: dashboard workspace scan/build command, static open command, editor command, agent build/context command, and whether `build all` creates both projections. Explain automatic package/module groups, manual drag/drop persistence, new-service auto-placement, stale override Doctor warnings, and static read-only behavior.

Document API Catalog before Endpoints and explain provider inventory versus relationship trace. State that provider security and consumer auth are static evidence, unknown is not public, and runtime enforcement is outside scope. Document catalog and config file schemas, output ownership, agent compact selection, unchanged budgets, and no full-catalog prompting. Update release notes as unreleased 1.3.0 only; do not add publication instructions implying completion.

- [ ] **Step 4: Run docs, CLI, and release-file tests**

Run: `gofmt -w docs_test.go release_files_test.go internal/cli/cli.go internal/cli/cli_test.go && go test . ./internal/cli -count=1`

Expected: PASS and `rg -n 'seven views' README.md COMMANDS.md` returns no stale count.

- [ ] **Step 5: Commit documentation**

```bash
git add README.md COMMANDS.md docs/OUTPUTS.md SCHEMA.md docs/RELEASE.md docs_test.go release_files_test.go internal/cli/cli.go internal/cli/cli_test.go
git commit -m "Document editable dashboards and API context" -m "- Explain grouping, editor persistence, and provider-oriented API Catalog workflows\n- Record compact agent behavior, integration depth, and security limitations"
```

### Task 5: Full Verification, Local Install, and Real Workspace Scan

**Files:**
- Modify only files required by a reproduced failure; never bundle unrelated cleanup.

**Interfaces:**
- Verifies all three plans and installs the locally built unreleased binary for user testing.

- [ ] **Step 1: Run formatting, static checks, and the full test suite**

Run: `gofmt -w cmd/goregraph/*.go internal/**/*.go && go vet ./... && go test ./...`

Expected: all commands exit 0. If a test fails, use `superpowers:systematic-debugging`; add a failing regression test before any fix.

- [ ] **Step 2: Run race checks for mutable/server/context packages**

Run: `go test -race ./internal/dashboardeditor ./internal/agent ./internal/scan`

Expected: PASS.

- [ ] **Step 3: Build and install the local unreleased binary**

Run: `go build ./cmd/goregraph && go install ./cmd/goregraph`

Expected: PASS. Run `goregraph version` and verify it reports source version `1.3.0` without a release tag. Do not invoke GoReleaser, GitHub Release, Homebrew, or Scoop publishing.

- [ ] **Step 4: Clean and rebuild the WEKA workspace projections**

Run:

```bash
goregraph workspace clean /Users/gorecode/projects/weka --execute
goregraph workspace build all /Users/gorecode/projects/weka
goregraph doctor /Users/gorecode/projects/weka
```

Expected: clean/build/Doctor succeed; `.goregraph-workspace/index/api-catalog.json`, dashboard HTML, and compact agent context exist. Preserve `/Users/gorecode/projects/weka/.goregraph-dashboard.json` across generated-output cleanup.

- [ ] **Step 5: Verify actual dashboard behavior visually**

Run `goregraph workspace dashboard edit /Users/gorecode/projects/weka`, inspect at 1280×720, 1440×900, and 1920×1080, and verify:

- package-derived groups are credible and contain no hardcoded organization mapping;
- rename/order/service moves persist after `workspace build dashboard`;
- Code Explorer long paths and actions do not overlap;
- API Catalog service selection, filters, compact rows, expanded details, unknown security, consumer auth, and mismatches render clearly;
- Endpoints trace behavior remains intact;
- static `workspace dashboard open` remains read-only.

- [ ] **Step 6: Compare agent context size and relevance**

Run representative Java/Spring plus JS/TS endpoint queries with the installed `goregraph context`. Record serialized bytes, `estimated_tokens`, selected files, endpoint, consumer count, and omitted count. Verify every pack is at most 1800 tokens by default, contains productive sources, and omits full catalog/config/dashboard files. Compare with the existing pre-change benchmark fixture where available; treat any unexplained token or irrelevant-file regression as a failure.

- [ ] **Step 7: Review changes and commit only focused fixes**

Run: `git diff --check && git status --short && git log --oneline --decorate -15`

Expected: no uncommitted implementation files and no generated WEKA outputs inside the GoreGraph repository. If verification required fixes, each has its own tested English commit.

- [ ] **Step 8: Push main without publishing**

Run: `git push origin main`

Expected: push succeeds. Do not create a tag, GitHub Release, or package-manager publication.
