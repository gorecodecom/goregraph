# Cross-Service Context Replacement for GoreGraph 1.3.0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make one GoreGraph Context Pack replace most manual source discovery for cross-service coding tasks while preserving baseline answer quality and measurably reducing total tokens, navigation calls, and source reads.

**Architecture:** Keep the existing compact Schema 3 fact-and-edge index, but fill its largest current gap by extracting Java outbound HTTP contracts and reconciling them with provider routes. Replace the single-seed/two-hop compiler and four-section greedy source attachment with deterministic concern planning, bounded graph-path selection, and coverage-per-token source assembly. Keep the standard MCP surface at one tool, add deterministic context identities for cheap duplicate suppression, and extend the release benchmark with structural navigation metrics so token improvements cannot hide unchanged file exploration.

**Tech Stack:** Go 1.23+ standard library, existing Java/Spring parser, Schema 3 agent context records, deterministic lexical and graph ranking, SHA-256 context identities, Go tests, POSIX shell benchmark harness, local Codex CLI acceptance runs.

## Global Constraints

- Work on the unreleased `1.3.0` source. Do not tag, publish, or run a release workflow.
- Use strict TDD for each production behavior: write one focused failing test, run it and observe the intended failure, implement the smallest behavior, rerun the focused test, then run the owning package.
- Add no runtime dependency, embedding model, vector database, network lookup, or tokenizer dependency.
- Do not encode proprietary repository names, paths, route fragments, symbols, prompt text, or business nouns in production code or committed test fixtures.
- Use synthetic projects named `services/catalog`, `services/jobs`, and `libraries/shared-model` for the cross-project regression.
- Keep query planning, path ranking, and source selection language-neutral. Java is the first outbound-contract extractor, not a Java-only retrieval architecture.
- Preserve the current one-tool standard MCP surface. Expert tools remain opt-in and are not part of the Context workflow.
- Keep the default Context Pack budget at 4,000 estimated tokens, the accepted range at 256..6,000 tokens, the default file limit at 12, and the hard serialized byte ceiling at 24,000 bytes.
- Keep exactly one primary production entrypoint. Other projects contribute connected path facts or explicitly labeled related-production candidates, never competing entrypoints.
- Prefer production source over tests. Tests may be selected only when requested, and only after every coverable required production concern has a source section.
- `source_coverage: complete` means every required concern is represented by current source, not that every ranked candidate was serialized.
- Preserve deterministic, input-order-independent output and stable tie-breaking.
- Keep source contents out of context identity hashes. Hash stable fact IDs, edge IDs, concern keys, and index freshness only.
- A missing or dynamic Java client path is partial evidence. It must not be matched to a provider route without a statically compatible method and normalized path.
- Benchmark prompts must not contain instructions about skills. Isolation belongs in the Codex invocation and the matched treatment configuration.
- Use English source comments, test names, CLI text, documentation, and commit messages.

---

## Last-Run Evidence and Acceptance Contract

The latest matched diagnostic is the regression that motivates this plan:

| Metric | Baseline | Assisted | Required direction |
|---|---:|---:|---|
| End-to-end tokens | 159,739 | 141,259 | Assisted median must reach at least 20% savings |
| Shell/tool executions | 34 | 48 | Assisted median must become at least 30% lower |
| `rg` calls | 14 | 13 | Raw source discovery must become at least 50% lower |
| `nl` calls | 23 | 26 | Included source must not be read again |
| `sed` calls | 31 | 33 | Included source must not be read again |

The assisted run saved 18,480 tokens, or 11.57%, but increased shell/tool executions by 41.18%. Its two approximately 3,090-token Context Packs were almost identical. The first pack found the correct public controller and local operations service, but it did not provide the required production evidence from the two other projects. Codex therefore repeated normal repository discovery.

This plan is complete only when all of these conditions hold together:

1. Median assisted quality is at least the baseline median on the existing twelve-point rubric.
2. Median assisted end-to-end tokens are at most 80% of the matched baseline median.
3. Median assisted navigation tool calls are at most 70% of the matched baseline median.
4. Median assisted raw source-read calls are at most 50% of the matched baseline median.
5. No assisted run returns two full Context Packs with the same `context_id`.
6. The cross-service task receives current production source from all three relevant projects before test source.
7. The same selector passes synthetic Java, Go, TypeScript, and Python graph fixtures without language-specific ranking branches.

---

## File and Interface Map

### New files

- `internal/scan/java_api_contracts.go` owns Java outbound HTTP contract extraction from `JavaSourceRecord` values. It emits existing `APIContractRecord` values and contains no workspace matching logic.
- `internal/scan/java_api_contracts_test.go` owns Feign, Spring HTTP interface, RestClient, WebClient, RestTemplate, dynamic-path, authentication, retry, and determinism tests.
- `internal/agent/context_intent.go` owns language-neutral task concerns, explicit-project concerns, and their deterministic keys.
- `internal/agent/context_paths.go` owns bounded multi-hop graph traversal, connected path scoring, and disconnected related-production candidates.
- `internal/agent/context_select.go` owns render-option costing and coverage-per-token source selection. `context_source.go` remains responsible for safe path resolution and rendering current source.
- `internal/agent/context_cross_service_test.go` owns the synthetic three-project regression and cross-language selector parity tests.
- `scripts/analyze-agent-context-log.sh` validates its arguments and delegates one raw Codex JSONL transcript to the parser.
- `scripts/analyze-agent-context-log.go` decodes terminal Codex JSONL items, classifies tool usage, and emits stable navigation metrics using only the Go standard library.
- `scripts/analyze-agent-context-log_test.sh` tests JSONL lifecycle deduplication, nested tool payloads, compound shell commands, source targets, and failure handling.

### Modified files

- `internal/scan/types.go` adds bounded Java HTTP-call provenance fields without changing existing JSON field meanings.
- `internal/scan/extract_java.go` retains receiver, client kind, unresolved URI expression, authentication, and retry provenance for outbound calls.
- `internal/scan/scan.go` merges Java contracts with existing JavaScript/TypeScript contracts once and passes the merged set to project outputs, contract matching, the API catalog, and the agent index.
- `internal/scan/agent_context_index.go` keeps caller-to-contract edges and stable contract facts for Java consumers.
- `internal/scan/workspace_reconcile.go` preserves caller-to-contract-to-provider paths and adds no duplicate consumer fact when the canonical project contract fact is already available.
- `internal/agent/context.go` adds public concern, context identity, duplicate, and retry metadata and raises only the source-section hard cap.
- `internal/agent/context_rank.go` keeps lexical fact ranking and endpoint disambiguation but delegates concern planning and graph-path selection to focused files.
- `internal/agent/context_source.go` exposes render options to `context_select.go` and stops greedily accepting candidates in role/path order.
- `internal/query/context.go` renders the new compact coverage and retry metadata.
- `internal/cli/cli.go` accepts `--previous-context-id` for a permitted retry.
- `internal/mcp/mcp.go` exposes `previous_context_id` on `task_context` and instructs the agent to retry only when the first pack allows it.
- `internal/scan/agent_reports.go` emits the same retry rule in generated agent guides.
- `scripts/benchmark-agent-context.sh` records structural navigation metrics and enforces the expanded gates.
- `scripts/benchmark-agent-context_test.sh` verifies the new summary columns and boundary conditions.
- `docs/BENCHMARKING.md`, `docs/RELEASE.md`, and `README.md` describe the source-replacement contract and report the latest diagnostic honestly.

---

### Task 1: Freeze the Last-Run Failure as a General Regression

**Files:**
- Create: `internal/agent/context_cross_service_test.go`
- Test: `internal/agent/context_cross_service_test.go`

**Interfaces:**
- Consumes: `BuildContext`, `scan.AgentContextIndexRecord`, existing test helpers that write project source and a workspace agent index.
- Produces: `TestBuildContextReplacesCrossServiceDiscovery` and `TestContextSelectionIsLanguageNeutral`, which later tasks must make pass without proprietary fixtures.

- [ ] **Step 1: Add the failing three-project regression**

Create a synthetic workspace index with this production path and these related candidates:

```go
facts := []scan.AgentContextFactRecord{
    {ID: "route", Project: "services/catalog", Kind: "route", Name: "DELETE /catalogs/{catalogId}/items/{itemId}", Qualified: "CatalogController.deleteItem", HTTPMethod: "DELETE", Path: "/catalogs/{catalogId}/items/{itemId}", File: "src/main/java/example/CatalogController.java", Line: 10, EndLine: 14, Confidence: "EXACT", Search: "delete catalog item"},
    {ID: "operations", Project: "services/catalog", Kind: "symbol", Name: "deleteItem", Qualified: "CatalogOperations.deleteItem", File: "src/main/java/example/CatalogOperations.java", Line: 8, EndLine: 15, Confidence: "EXACT", Search: "delete catalog item operations"},
    {ID: "client", Project: "libraries/shared-model", Kind: "symbol", Name: "deleteRelatedJobs", Qualified: "JobManagementClient.deleteRelatedJobs", File: "src/main/java/example/JobManagementClient.java", Line: 12, EndLine: 24, Confidence: "EXACT", Search: "delete related jobs basic authentication retry configuration"},
    {ID: "contract", Project: "libraries/shared-model", Kind: "api_contract", Name: "DELETE /job-management/catalogs/{catalogId}/items/{itemId}", Qualified: "JobManagementClient.deleteRelatedJobs", HTTPMethod: "DELETE", Path: "/job-management/catalogs/{catalogId}/items/{itemId}", File: "src/main/java/example/JobManagementClient.java", Line: 18, EndLine: 21, Confidence: "RESOLVED", Search: "delete related jobs internal contract"},
    {ID: "provider", Project: "services/jobs", Kind: "route", Name: "DELETE /job-management/catalogs/{catalogId}/items/{itemId}", Qualified: "JobManagementController.deleteRelatedJobs", HTTPMethod: "DELETE", Path: "/job-management/catalogs/{catalogId}/items/{itemId}", File: "src/main/java/example/JobManagementController.java", Line: 20, EndLine: 25, Confidence: "EXACT", Search: "delete related jobs"},
    {ID: "service", Project: "services/jobs", Kind: "symbol", Name: "deleteRelatedJobs", Qualified: "JobService.deleteRelatedJobs", File: "src/main/java/example/JobService.java", Line: 30, EndLine: 45, Confidence: "EXACT", Search: "delete related jobs persistence"},
    {ID: "regular-repository", Project: "services/jobs", Kind: "persistence", Name: "deleteByCatalogIdAndItemId", Qualified: "JobRepository.deleteByCatalogIdAndItemId", File: "src/main/java/example/JobRepository.java", Line: 8, EndLine: 10, Confidence: "EXACT", Search: "regular job catalog item delete persistence"},
    {ID: "change-repository", Project: "services/jobs", Kind: "persistence", Name: "deleteByCatalogIdAndItemId", Qualified: "ChangeJobRepository.deleteByCatalogIdAndItemId", File: "src/main/java/example/ChangeJobRepository.java", Line: 8, EndLine: 10, Confidence: "EXACT", Search: "change job catalog item delete persistence"},
    {ID: "test", Project: "services/catalog", Kind: "test", Name: "deletes item", File: "src/test/java/example/CatalogControllerTest.java", Line: 15, EndLine: 28, Confidence: "EXACT", Search: "delete catalog item test"},
}
edges := []scan.AgentContextEdgeRecord{
    {ID: "e1", FromFactID: "route", ToFactID: "operations", Kind: "call", Confidence: "EXACT"},
    {ID: "e2", FromFactID: "operations", ToFactID: "client", Kind: "call", Confidence: "RESOLVED"},
    {ID: "e3", FromFactID: "client", ToFactID: "contract", Kind: "call", Confidence: "EXACT"},
    {ID: "e4", FromFactID: "contract", ToFactID: "provider", Kind: "http_contract", Confidence: "RESOLVED"},
    {ID: "e5", FromFactID: "provider", ToFactID: "service", Kind: "call", Confidence: "EXACT"},
    {ID: "e6", FromFactID: "service", ToFactID: "regular-repository", Kind: "persistence", Confidence: "RESOLVED"},
    {ID: "e7", FromFactID: "service", ToFactID: "change-repository", Kind: "persistence", Confidence: "RESOLVED"},
    {ID: "e8", FromFactID: "test", ToFactID: "route", Kind: "test_target", Confidence: "EXACT"},
}
```

Write small valid source files for every production fact. Query the complete task in neutral terms, explicitly naming all three synthetic projects and requesting endpoint, current and required chain, contract, authentication, configuration, persistence, and tests.

Add these test-local helpers in the same file:

```go
func contextPackHasProductionSource(pack ContextPack, project string) bool {
    for _, section := range pack.SourceSections {
        if section.Project == project && section.Role != "test" {
            return true
        }
    }
    return false
}

func contextPackHasRelationshipKind(pack ContextPack, kind string) bool {
    for _, relationship := range pack.CallChain {
        if relationship.Kind == kind {
            return true
        }
    }
    return false
}

func contextPackTestPrecedesProduction(pack ContextPack, requiredProjects []string) bool {
    required := make(map[string]bool, len(requiredProjects))
    for _, project := range requiredProjects {
        required[project] = true
    }
    firstTest := len(pack.SourceSections)
    lastRequiredProduction := -1
    for index, section := range pack.SourceSections {
        if section.Role == "test" && firstTest == len(pack.SourceSections) {
            firstTest = index
        }
        if section.Role != "test" && required[section.Project] {
            lastRequiredProduction = index
        }
    }
    return firstTest < lastRequiredProduction
}
```

Require:

```go
if pack.FallbackRequired || len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "route" {
    t.Fatalf("primary entrypoint = %#v", pack)
}
for _, project := range []string{"services/catalog", "libraries/shared-model", "services/jobs"} {
    if !contextPackHasProductionSource(pack, project) {
        t.Fatalf("missing production source for %s: %#v", project, pack.SourceSections)
    }
}
if !contextPackHasRelationshipKind(pack, "http_contract") {
    t.Fatalf("cross-service contract path missing: %#v", pack.CallChain)
}
if contextPackTestPrecedesProduction(pack, []string{"services/catalog", "libraries/shared-model", "services/jobs"}) {
    t.Fatalf("test source displaced required production: %#v", pack.SourceSections)
}
if pack.SourceCoverage != "complete" || pack.EstimatedTokens > 4000 {
    t.Fatalf("source coverage/budget = %q/%d", pack.SourceCoverage, pack.EstimatedTokens)
}
```

- [ ] **Step 2: Verify the regression fails for the observed reasons**

Run:

```bash
go test ./internal/agent -run '^TestBuildContextReplacesCrossServiceDiscovery$' -count=1
```

Expected: `FAIL`; the current compiler stops after the shallow local path or the four-section source limit and does not source-cover all three projects.

- [ ] **Step 3: Add a failing language-neutral selector test**

Create the same fact/edge topology four times using `.java`, `.go`, `.ts`, and `.py` source files. Keep IDs, projects, kinds, confidence, and search terms identical. Require the selected fact IDs, concern keys, project order, render modes, and `source_coverage` to match after file-extension normalization.

- [ ] **Step 4: Verify language parity fails only because the new planner is absent**

Run:

```bash
go test ./internal/agent -run '^TestContextSelectionIsLanguageNeutral$' -count=1
```

Expected: `FAIL`; the required concern and path-selection output does not exist yet.

- [ ] **Step 5: Commit the red regression contract**

```bash
git add internal/agent/context_cross_service_test.go
git commit -m "Add cross-service context regression" -m "- Capture the last benchmark's missing three-project production coverage with synthetic facts.
- Require language-neutral selection and production-before-test ordering."
```

### Task 2: Extract Java Outbound HTTP Contracts

**Files:**
- Create: `internal/scan/java_api_contracts.go`
- Create: `internal/scan/java_api_contracts_test.go`
- Modify: `internal/scan/types.go:616-704`
- Modify: `internal/scan/extract_java.go:31-190,991-1045`
- Modify: `internal/scan/scan.go:350-420,460-475`
- Test: `internal/scan/java_api_contracts_test.go`
- Test: `internal/scan/scan_test.go`

**Interfaces:**
- Consumes: `JavaSourceRecord`, `JavaTypeRecord`, `JavaMethodRecord`, `JavaAnnotationRecord`, `JavaFieldRecord`, current constant and annotation parsing, `normalizeAPIPath`, and existing `APIContractRecord`.
- Produces: `buildJavaAPIContracts(sources []JavaSourceRecord) []APIContractRecord` and one merged `apiContracts` slice in `writeOutputs`.

- [ ] **Step 1: Write exact declarative-client tests**

Add table-driven tests for:

```java
@FeignClient(name = "jobs", path = "/job-management")
interface JobClient {
  @DeleteMapping("/catalogs/{catalogId}/items/{itemId}")
  void deleteRelatedJobs(String catalogId, String itemId);
}
```

and:

```java
@HttpExchange(url = "/job-management")
interface JobClient {
  @DeleteExchange("/catalogs/{catalogId}/items/{itemId}")
  void deleteRelatedJobs(String catalogId, String itemId);
}
```

Require one contract with method `DELETE`, normalized path `/job-management/catalogs/{catalogId}/items/{itemId}`, caller `JobClient.deleteRelatedJobs`, service candidate `jobs` for Feign, exact file/line, and `EXACT` confidence.

- [ ] **Step 2: Write bound imperative-client tests**

Cover `RestClient`, `WebClient`, and `RestTemplate` with literal, constant, and one local path-getter expression. Require framework evidence from an imported client type plus a field/variable receiver. A method named `delete` on an unrelated receiver must produce no contract.

Use this expected record shape:

```go
APIContractRecord{
    Language: "java", HTTPMethod: "DELETE",
    Path: "/job-management/catalogs/{dynamic}/items/{dynamic}",
    RawPath: "pathProvider.deleteRelatedJobsPath()",
    Caller: "JobClient.deleteRelatedJobs",
    File: "src/main/java/example/JobClient.java",
    Confidence: "RESOLVED", ConfidenceScore: 0.9,
    Reason: "spring RestClient receiver with statically resolved path getter",
}
```

For a dynamic expression that cannot be resolved, require `UnsafeDynamic: true`, a non-empty `RawPath`, `PARTIAL` confidence, and no invented `Path`.

- [ ] **Step 3: Write authentication and retry provenance tests**

For `BasicAuthenticationInterceptor`, `defaultHeader("Authorization", ...)`, `@Retryable`, and a configuration-property base URL, require only categorical evidence:

```go
[]AuthRecord{{Kind: "basic", Source: "spring_client_interceptor", Confidence: "EXTRACTED"}}
```

Do not serialize usernames, passwords, header values, or resolved property values. Add one test containing credential-looking literals and assert that none occur in marshaled contracts.

- [ ] **Step 4: Verify the Java tests fail**

Run:

```bash
go test ./internal/scan -run '^TestBuildJavaAPIContracts' -count=1
```

Expected: `FAIL`; Java sources currently produce capability metadata and test HTTP requests, but not outbound `APIContractRecord` values.

- [ ] **Step 5: Extend bounded Java HTTP-call provenance**

Extend `JavaHTTPCallRecord` without removing existing fields:

```go
type JavaHTTPCallRecord struct {
    Receiver       string `json:"receiver,omitempty"`
    ClientKind     string `json:"client_kind,omitempty"`
    HTTPMethod     string `json:"http_method"`
    Path           string `json:"path,omitempty"`
    PathExpression string `json:"path_expression,omitempty"`
    Line           int    `json:"line"`
    Confidence     string `json:"confidence,omitempty"`
}
```

Replace the internal string `PendingHTTP` with a non-serialized record carrying method, receiver, client kind, and start line. Resolve client kind only from imports plus the receiver's declared field/local type. Preserve unresolved URI expressions instead of discarding them.

- [ ] **Step 6: Implement declarative and imperative contract builders**

Create these focused functions:

```go
func buildJavaAPIContracts(sources []JavaSourceRecord) []APIContractRecord
func javaDeclarativeAPIContracts(source JavaSourceRecord) []APIContractRecord
func javaImperativeAPIContracts(source JavaSourceRecord, paths javaPathIndex) []APIContractRecord
func buildJavaPathIndex(sources []JavaSourceRecord) javaPathIndex
func javaClientAuthentication(source JavaSourceRecord, method JavaMethodRecord) []AuthRecord
```

Rules:

1. Declarative mappings combine one type-level base path with each method path.
2. Imperative calls require a receiver proven to be `RestClient`, `WebClient`, or `RestTemplate`.
3. The path index resolves only string constants, direct concatenations, local string variables, and uniquely resolved zero-argument getters returning those expressions.
4. A base URL/property expression is removed only as a dynamic prefix; the route suffix must remain literal and start with `/`.
5. Unresolved expressions remain partial and never participate in exact workspace matching.
6. Sort by file, line, HTTP method, path, caller, and reason; deduplicate byte-identical call sites.

- [ ] **Step 7: Merge Java contracts exactly once in project output**

In `writeOutputs`, create:

```go
apiContracts := append([]APIContractRecord(nil), index.Code.APIContracts...)
apiContracts = append(apiContracts, buildJavaAPIContracts(index.JavaSources)...)
sortAPIContracts(apiContracts)
```

Define the shared deterministic sorter in `java_api_contracts.go` so project output and tests use the same order:

```go
func sortAPIContracts(records []APIContractRecord) {
    sort.Slice(records, func(i, j int) bool {
        left, right := records[i], records[j]
        if left.File != right.File {
            return left.File < right.File
        }
        if left.Line != right.Line {
            return left.Line < right.Line
        }
        if left.HTTPMethod != right.HTTPMethod {
            return left.HTTPMethod < right.HTTPMethod
        }
        if left.Path != right.Path {
            return left.Path < right.Path
        }
        if left.Caller != right.Caller {
            return left.Caller < right.Caller
        }
        return left.Reason < right.Reason
    })
}
```

Pass `apiContracts` to `buildContractMatches`, `BuildProjectAPICatalog`, `BuildProjectAgentContextIndex`, and the `api-contracts.json`/Markdown writers. Keep `buildFrontendUsage` limited to the existing frontend contracts so Java service clients are not mislabeled as frontend usage.

- [ ] **Step 8: Verify focused, package, and scan integration tests**

Run:

```bash
go test ./internal/scan -run '^TestBuildJavaAPIContracts|^TestRunWritesJavaAPIContracts' -count=1
go test ./internal/scan -count=1
```

Expected: `PASS`.

- [ ] **Step 9: Commit Java contract extraction**

```bash
git add internal/scan/java_api_contracts.go internal/scan/java_api_contracts_test.go internal/scan/types.go internal/scan/extract_java.go internal/scan/scan.go internal/scan/scan_test.go
git commit -m "Extract Java outbound API contracts" -m "- Resolve Spring declarative and bound imperative HTTP clients into canonical contracts.
- Preserve dynamic path uncertainty, authentication category, and retry provenance without credentials.
- Feed Java contracts through the existing project API and agent outputs."
```

### Task 3: Preserve Cross-Repository Consumer-to-Provider Paths

**Files:**
- Modify: `internal/scan/agent_context_index.go:639-850`
- Modify: `internal/scan/workspace_reconcile.go:911-1220`
- Test: `internal/scan/agent_context_index_test.go`
- Test: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: Java `APIContractRecord` values from Task 2, project `call` edges, workspace contract matches, and canonical catalog endpoints.
- Produces: one resolvable `caller -> api_contract -> provider route` chain using edge kinds `call` and `http_contract`; catalog consumers remain endpoint metadata and do not replace canonical contract facts.

- [ ] **Step 1: Write the failing project-edge test**

Build a project context index containing `CatalogOperations.deleteItem -> JobClient.deleteRelatedJobs` and a Java contract owned by `JobClient.deleteRelatedJobs`. Require:

```text
CatalogOperations.deleteItem --call--> JobClient.deleteRelatedJobs
JobClient.deleteRelatedJobs --call--> DELETE /job-management/catalogs/{catalogId}/items/{itemId}
```

Require stable IDs and the same result with reversed symbol, relation, and contract input order.

- [ ] **Step 2: Write the failing workspace-bridge test**

Merge `services/catalog`, `libraries/shared-model`, and `services/jobs` project indexes. Supply one resolved `WorkspaceContractMatchRecord` from the library contract to the jobs provider. Require a directed path from the shared-model client through its canonical contract to the jobs provider of at most seven edges, keep catalog production as a separate related-project candidate, and require no generated-metadata file in any fact. Do not invent a catalog-to-shared-model edge when the indexed source graph does not contain one.

- [ ] **Step 3: Verify the tests fail at the missing edge or duplicate-fact boundary**

Run:

```bash
go test ./internal/scan -run '^Test(ProjectAgentContextLinksJavaCallerToContract|WorkspaceAgentContextPreservesCrossRepositoryHTTPPath)$' -count=1
```

Expected: `FAIL` until Java contracts and canonical workspace edge reuse are complete.

- [ ] **Step 4: Reuse canonical contract facts in workspace reconciliation**

Keep `addContractEdges` as the owner of cross-project `http_contract` edges. When `addCatalogFacts` sees a consumer whose project/file/line/caller resolves to one existing `api_contract` fact, link that fact to endpoint metadata rather than creating a second source-bearing consumer fact. Create an `api_consumer` fact only when no canonical contract fact exists.

Do not collapse `api_contract` and provider `route` into one fact: the boundary is needed for source selection and evidence.

- [ ] **Step 5: Verify scan and workspace packages**

Run:

```bash
go test ./internal/scan -run '^Test(ProjectAgentContextLinksJavaCallerToContract|WorkspaceAgentContextPreservesCrossRepositoryHTTPPath)$' -count=1
go test ./internal/scan -count=1
```

Expected: `PASS`.

- [ ] **Step 6: Commit the workspace bridge**

```bash
git add internal/scan/agent_context_index.go internal/scan/agent_context_index_test.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go
git commit -m "Preserve cross-repository HTTP paths" -m "- Link Java caller symbols to canonical outbound contracts.
- Reuse contract facts when matching workspace consumers to provider routes.
- Keep complete directed paths deterministic across project input order."
```

### Task 4: Plan Required Task Concerns

**Files:**
- Create: `internal/agent/context_intent.go`
- Modify: `internal/agent/context.go:12-125`
- Test: `internal/agent/context_test.go`
- Test: `internal/agent/context_cross_service_test.go`

**Interfaces:**
- Consumes: normalized query tokens, project aliases, selected endpoint/entrypoint, facts, edges, and coverage.
- Produces: `planContextConcerns(query string, index scan.AgentContextIndexRecord, seed scan.AgentContextFactRecord) []contextConcern` and public bounded concern results.

- [ ] **Step 1: Write concern-planning tests**

Require these deterministic concern kinds:

```go
const (
    contextConcernEntrypoint   = "entrypoint"
    contextConcernPrimaryPath  = "primary_path"
    contextConcernProject      = "project"
    contextConcernHTTPContract = "http_contract"
    contextConcernAuth         = "authentication"
    contextConcernPersistence  = "persistence"
    contextConcernTests        = "tests"
)
```

Rules tested:

1. `entrypoint` and `primary_path` are always required.
2. Every explicitly named, uniquely resolved project with at least one semantic fact match gets one required `project` concern.
3. A reachable `http_contract` edge creates a required contract concern even when the query does not say “HTTP”.
4. Authentication and persistence become required when requested or when a selected endpoint/path exposes those fact kinds.
5. Tests become required only when the query requests tests.
6. Missing project coverage creates an uncovered concern, never a fake covered concern.
7. At most eight concern records are serialized; required concerns win deterministic ties.

- [ ] **Step 2: Verify concern tests fail**

Run:

```bash
go test ./internal/agent -run '^TestPlanContextConcerns' -count=1
```

Expected: `FAIL`; current packs have no explicit task-coverage contract.

- [ ] **Step 3: Add public compact concern metadata**

Add:

```go
type ContextConcern struct {
    Kind    string `json:"kind"`
    Project string `json:"project,omitempty"`
    Covered bool   `json:"covered"`
    Reason  string `json:"reason,omitempty"`
}
```

Add `Concerns []ContextConcern` to `ContextPack`. Keep the existing compact `ContextRelationship` labels unchanged; stable IDs remain internal to selection and identity calculation. Do not add separate verbose covered/uncovered arrays.

- [ ] **Step 4: Implement deterministic concern planning**

Create a private `contextConcern` with `key`, `kind`, `project`, `required`, `candidateFactIDs`, and `reason`. Derive project aliases with the current exact-path/unique-basename rules. Use generic task vocabulary only for authentication, configuration, persistence, retry, and tests; rely on fact kinds and graph edges for domain concepts.

- [ ] **Step 5: Verify focused and package tests**

Run:

```bash
go test ./internal/agent -run '^TestPlanContextConcerns|^TestBuildContextReplacesCrossServiceDiscovery$' -count=1
go test ./internal/agent -count=1
```

Expected: concern tests `PASS`; the full cross-service test may remain red until Tasks 5 and 6.

- [ ] **Step 6: Commit concern planning**

```bash
git add internal/agent/context.go internal/agent/context_intent.go internal/agent/context_test.go internal/agent/context_cross_service_test.go
git commit -m "Plan context coverage concerns" -m "- Translate tasks and graph evidence into bounded required concerns.
- Represent explicit projects, contracts, authentication, persistence, and requested tests honestly."
```

### Task 5: Replace Shallow Expansion with Bounded Path Planning

**Files:**
- Create: `internal/agent/context_paths.go`
- Modify: `internal/agent/context_rank.go:52-320,1188-1258`
- Test: `internal/agent/context_test.go`
- Test: `internal/agent/context_cross_service_test.go`

**Interfaces:**
- Consumes: one reliable production seed, planned concerns, ranked facts, and all context edges.
- Produces: `selectContextPaths(index scan.AgentContextIndexRecord, seed rankedContextFact, concerns []contextConcern) contextPathSelection` containing selected fact IDs, edge IDs, distances, concern coverage, and related-production facts.

- [ ] **Step 1: Write bounded traversal tests**

Require a seven-edge path to be found through `call`, `http_contract`, and `persistence` edges. Add cycles, a 100-node unrelated fan-out, ambiguous test edges, and reversed inputs. Require:

```go
const (
    maximumContextPathHops     = 7
    maximumContextVisitedFacts = 256
    maximumContextPaths        = 8
    maximumContextEdgesPerNode = 24
)
```

The selected result must be deterministic, cycle-free, and production-only unless tests are a required concern.

- [ ] **Step 2: Write disconnected related-production tests**

Model a requested future change where no call edge exists yet. Put a strongly matching client/configuration fact in an explicitly named library project. Require it as `related_project` with lower confidence, but never insert it into `CallChain` as if connected.

- [ ] **Step 3: Verify path tests fail**

Run:

```bash
go test ./internal/agent -run '^TestSelectContextPaths' -count=1
```

Expected: `FAIL`; `expandContextEdges` currently examines the seed and one production frontier only.

- [ ] **Step 4: Implement deterministic bounded traversal**

Build sorted forward adjacency for these edge kinds:

```go
var contextTraversalCost = map[string]int{
    "call": 1, "http_contract": 1, "persistence": 1,
    "use": 2, "implements": 2, "extends": 2,
    "consumes_endpoint": 2, "requires_auth": 2,
    "test_target": 3,
}
```

Use breadth-first traversal with accumulated cost, hop count, and lexicographic path key. Reject generated metadata, test nodes when tests are not required, cycles, and paths beyond the fixed limits. A candidate path's score is:

```text
1000 × newly covered required concerns
+ 300 × newly covered explicit projects
+ 200 × first cross-project contract boundary
+ lexical score of its terminal fact
- 40 × path hops
- traversal cost
```

Select at most eight paths by marginal score. Add disconnected production facts only through the existing semantic support threshold and label them `related_project`.

- [ ] **Step 5: Replace shallow expansion without removing endpoint metadata**

Keep endpoint security/consumer assembly for `ContextEndpoint`. Replace `expandContextEdges` in `compileContextPack` with the selected path facts and edges. Populate `CallChain` only from connected selected edges. Retain all selected fact IDs for source assembly.

- [ ] **Step 6: Verify focused and package behavior**

Run:

```bash
go test ./internal/agent -run '^TestSelectContextPaths|^TestBuildContextReplacesCrossServiceDiscovery$' -count=1
go test ./internal/agent -count=1
```

Expected: path and concern assertions `PASS`; source coverage may remain partial until Task 6.

- [ ] **Step 7: Commit path planning**

```bash
git add internal/agent/context_paths.go internal/agent/context_rank.go internal/agent/context_test.go internal/agent/context_cross_service_test.go
git commit -m "Select bounded context paths" -m "- Traverse complete production and HTTP-contract paths up to fixed safety limits.
- Score paths by new concern and project coverage.
- Keep disconnected change candidates explicit instead of inventing call-chain edges."
```

### Task 6: Assemble Source by Coverage Gain per Token

**Files:**
- Create: `internal/agent/context_select.go`
- Modify: `internal/agent/context.go:12-24`
- Modify: `internal/agent/context_source.go:20-320`
- Test: `internal/agent/context_source_test.go`
- Test: `internal/agent/context_size_test.go`
- Test: `internal/agent/context_cross_service_test.go`

**Interfaces:**
- Consumes: path-selected facts, concerns, safe source rendering, request token/byte/file limits.
- Produces: `selectContextSourceOptions(pack ContextPack, loaded loadedContextIndex, request ContextRequest) (ContextPack, error)` and concern-based `source_coverage`.

- [ ] **Step 1: Write option-costing and ordering tests**

Require every candidate to produce zero or more pre-costed `body`, `focused`, and `signature` options. Require current source verification before an option is eligible. Require deterministic option order after reversing candidate and fact input.

- [ ] **Step 2: Write production-before-test and project-quota tests**

With a 4,000-token budget, require at least one production section for each coverable required project before any test section. Add a large central method and require `focused` rather than allowing it to consume the remaining pack. Add two nearby ranges in one file and require one merged section when their gap is at most eight lines.

- [ ] **Step 3: Write honest coverage tests**

Require:

- `complete` when every required concern has a current source section even if optional candidates were omitted;
- `partial` when one required project or contract concern lacks source;
- `none` when no current source section was selected;
- an exact `SourceOmission` for each uncovered required concern, including project, file, role, and stable reason;
- optional omissions not to downgrade complete coverage.

- [ ] **Step 4: Verify source-selection tests fail**

Run:

```bash
go test ./internal/agent -run '^TestContextSource(Options|ProductionBeforeTests|ConcernCoverage)' -count=1
```

Expected: `FAIL`; current code takes the first fitting candidate and stops after four sections.

- [ ] **Step 5: Raise only the safety cap and model render options**

Set:

```go
const MaxContextSourceSections = 12
```

Do not reserve twelve sections. The token/byte budget remains authoritative.

Define:

```go
type contextSourceOption struct {
    candidate    sourceCandidate
    section      ContextSourceSection
    estimated    int
    concernKeys  []string
    projectKey   string
    required     bool
    pathDistance int
}
```

- [ ] **Step 6: Implement marginal coverage selection**

First admit the smallest fitting option that covers each mandatory core-path boundary: entrypoint, first local production hop, contract caller, provider route, and one production fact per required project. Then greedily select the option with the greatest deterministic utility:

```text
1200 × newly covered required concerns
+ 300 × newly covered project concerns
+ 150 × newly covered roles
+ 80 × connected-path membership
- estimated tokens
- 25 × path distance
```

Use role, project, path, start line, render mode, and fact ID as final tie-breakers. Tests receive no utility until every coverable required production concern is covered.

- [ ] **Step 7: Replace candidate-count coverage with concern coverage**

Remove `SourceUnrepresented` from coverage decisions; retain it only if compatibility tests require its JSON field. Set each public `ContextConcern.Covered` from selected source evidence. Create omissions only for uncovered required concerns, capped by the existing public omission safety limit after required concerns have been sorted.

- [ ] **Step 8: Verify the main regression and size limits**

Run:

```bash
go test ./internal/agent -run '^TestContextSource|^TestBuildContextReplacesCrossServiceDiscovery$|^TestContextSelectionIsLanguageNeutral$' -count=1
go test ./internal/agent -count=1
```

Expected: all `PASS`; the three-project pack is at most 4,000 estimated tokens and 24,000 serialized bytes.

- [ ] **Step 9: Commit source selection**

```bash
git add internal/agent/context.go internal/agent/context_source.go internal/agent/context_select.go internal/agent/context_source_test.go internal/agent/context_size_test.go internal/agent/context_cross_service_test.go
git commit -m "Select source by coverage per token" -m "- Replace fixed four-section greedy packing with pre-costed source options.
- Cover required production projects and graph boundaries before requested tests.
- Report source completeness against task concerns instead of candidate count."
```

### Task 7: Suppress Identical Context Retries

**Files:**
- Modify: `internal/agent/context.go`
- Modify: `internal/agent/context_rank.go`
- Modify: `internal/query/context.go`
- Modify: `internal/cli/cli.go:1015-1070`
- Modify: `internal/mcp/mcp.go:100-135,230-280`
- Modify: `internal/scan/agent_reports.go`
- Test: `internal/agent/context_test.go`
- Test: `internal/query/context_test.go`
- Test: `internal/cli/cli_test.go`
- Test: `internal/mcp/mcp_test.go`
- Test: `internal/scan/agent_reports_test.go`

**Interfaces:**
- Consumes: selected fact IDs, selected edge IDs, concern keys, index freshness, and optional `ContextRequest.PreviousContextID`.
- Produces: stable `ContextID`, `DuplicateOf`, `RetryAllowed`, `RetryAnchors`, CLI `--previous-context-id`, and MCP `previous_context_id`.

- [ ] **Step 1: Write context identity tests**

Require identical semantic selections to produce the same identity despite fact/edge input order. Require a changed selected fact, edge, concern, or freshness value to change the identity. Require source contents not to affect it.

- [ ] **Step 2: Write duplicate-response tests**

Call `BuildContext` once, then call it with a narrower query and the first `ContextID`. When the semantic selection is unchanged, require:

```go
if second.DuplicateOf != first.ContextID || second.ContextID != first.ContextID {
    t.Fatalf("duplicate identity = %#v", second)
}
if len(second.SourceSections) != 0 || len(second.Files) != 0 || second.EstimatedTokens > 200 {
    t.Fatalf("duplicate response was not minimal: %#v", second)
}
if second.RetryAllowed || second.FallbackRequired {
    t.Fatalf("duplicate response requested more work: %#v", second)
}
```

The duplicate response keeps `Schema`, `Freshness`, `Confidence`, `ContextID`, `DuplicateOf`, `RetryAllowed`, `EstimatedTokens`, and `BudgetTokens`; it sets `Query` to the empty string and omits files, relationships, concerns, source sections, and omissions.

- [ ] **Step 3: Verify duplicate tests fail**

Run:

```bash
go test ./internal/agent -run '^TestContext(Identity|DuplicateResponse)' -count=1
```

Expected: `FAIL`; current packs have no semantic identity.

- [ ] **Step 4: Add compact public retry fields**

Add:

```go
// ContextRequest
PreviousContextID string `json:"previous_context_id,omitempty"`

// ContextPack
ContextID       string   `json:"context_id,omitempty"`
DuplicateOf     string   `json:"duplicate_of,omitempty"`
RetryAllowed    bool     `json:"retry_allowed"`
RetryAnchors    []string `json:"retry_anchors,omitempty"`
```

Compute the identity with SHA-256 and serialize the first 12 bytes as 24 lowercase hexadecimal characters.

- [ ] **Step 5: Determine retry permission from uncovered required concerns**

Allow one retry only when at least one uncovered required concern has a concrete unselected exact route, qualified symbol, or source file anchor. Return at most three anchors. If no new semantic candidate exists, set `RetryAllowed` false even when the pack is partial.

- [ ] **Step 6: Wire CLI, MCP, Markdown, and generated guide behavior**

Add optional `--previous-context-id` and `previous_context_id`. Update standard guidance to:

```text
Call task_context once before indexed source discovery. Treat source_sections as already read.
Retry only when retry_allowed is true, use one retry_anchor, and pass context_id as previous_context_id.
If duplicate_of is present, use the first pack and do not read more source because of the duplicate response.
```

Do not add another standard MCP tool.

- [ ] **Step 7: Verify all user surfaces**

Run:

```bash
go test ./internal/agent ./internal/query ./internal/cli ./internal/mcp ./internal/scan -run 'Context|AgentGuide|MCP' -count=1
```

Expected: `PASS`.

- [ ] **Step 8: Commit duplicate suppression**

```bash
git add internal/agent/context.go internal/agent/context_rank.go internal/agent/context_test.go internal/query/context.go internal/query/context_test.go internal/cli/cli.go internal/cli/cli_test.go internal/mcp/mcp.go internal/mcp/mcp_test.go internal/scan/agent_reports.go internal/scan/agent_reports_test.go
git commit -m "Suppress duplicate context retries" -m "- Identify semantic Context Pack selections independently of source text.
- Return a bounded duplicate response when a permitted retry finds no new context.
- Expose retry permission and exact anchors through CLI, MCP, and agent guides."
```

### Task 8: Measure Navigation Replacement in the Benchmark

**Files:**
- Create: `scripts/analyze-agent-context-log.sh`
- Create: `scripts/analyze-agent-context-log.go`
- Create: `scripts/analyze-agent-context-log_test.sh`
- Modify: `scripts/benchmark-agent-context.sh`
- Modify: `scripts/benchmark-agent-context_test.sh`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/RELEASE.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: one raw Codex JSONL transcript produced by `codex exec --json`, with stderr retained separately by the harness.
- Produces: one TSV data row with `tool_calls`, `goregraph_calls`, `full_context_packs`, `compact_duplicate_packs`, `repeated_full_packs`, `raw_navigation_calls`, `source_read_calls`, and `unique_source_files`.

- [ ] **Step 1: Write synthetic transcript analyzer tests**

Create real Codex JSONL fixtures in the test's temporary directory containing terminal `item.completed` records for command execution, MCP, collaboration, web search, and file changes. Cover `goregraph context`, `rg`, `sed`, `nl`, compound `cd && sed` and pipelines, attached `-e`/`-f` options, lifecycle duplicates, and unrelated build commands. Include full JSON and Markdown Context Packs, compact duplicate responses, and a repeated full pack. Require exact counts and reject malformed JSONL, unknown terminal item types, and conflicting duplicate item IDs rather than silently returning zero.

- [ ] **Step 2: Verify analyzer tests fail**

Run:

```bash
bash scripts/analyze-agent-context-log_test.sh
```

Expected: `FAIL`; the analyzer does not exist.

- [ ] **Step 3: Implement stable transcript classification**

Keep the shell script as the stable entry point and use a small Go standard-library parser for nested JSONL. The analyzer must:

1. count terminal tool items exactly once by item ID;
2. classify command execution, file changes, MCP, collaboration, and web-search items, rejecting unknown completed item types;
3. classify `goregraph context` and `task_context` separately;
4. split unquoted shell control operators while preserving quoted or escaped characters;
5. classify `rg`, `grep`, `find`, `sed`, `nl`, `cat`, `head`, `tail`, and direct source-file reads only when their concrete target has a supported source extension, excluding patterns and script files;
6. extract and deduplicate source paths without storing source content;
7. parse JSON and Markdown Context Packs, count compact duplicates separately, and flag only a full pack repeated for the same context ID;
8. emit a header only with `--header` and one data row otherwise.

- [ ] **Step 4: Extend the harness summary**

Change `summary.tsv` to:

```text
variant run tokens tool_calls goregraph_calls full_context_packs compact_duplicate_packs repeated_full_packs raw_navigation_calls source_read_calls unique_source_files log
```

Calculate medians for tokens, tool calls, raw navigation calls, and source-read calls. Preserve every raw log and analyzer result outside the workspace.

- [ ] **Step 5: Enforce structural gates**

After the existing token gate, enforce:

```bash
[ $((assisted_tool_median * 10)) -le $((baseline_tool_median * 7)) ] ||
  die "assisted tool-call median exceeds 70% of matched baseline"
[ $((assisted_source_read_median * 2)) -le "$baseline_source_read_median" ] ||
  die "assisted source-read median exceeds 50% of matched baseline"
[ "$assisted_repeated_full_packs" -eq 0 ] ||
  die "assisted runs repeated a full Context Pack"
```

Treat zero baseline source reads as an invalid benchmark input because it cannot measure source-replacement savings.

- [ ] **Step 6: Update benchmark and release documentation**

Document the latest 159,739/141,259 diagnostic and its 34/48 tool-call regression. State that it is diagnostic evidence, not release evidence. Keep the twelve-point quality rubric and matched three-by-three requirement. Explain that skills are controlled by the invocation and never by adding “do not use skills” to one treatment prompt.

- [ ] **Step 7: Verify shell syntax and harness boundaries**

Run:

```bash
bash -n scripts/analyze-agent-context-log.sh
bash -n scripts/benchmark-agent-context.sh
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context_test.sh
```

Expected: all `PASS`.

- [ ] **Step 8: Commit benchmark metrics**

```bash
git add scripts/analyze-agent-context-log.sh scripts/analyze-agent-context-log.go scripts/analyze-agent-context-log_test.sh scripts/benchmark-agent-context.sh scripts/benchmark-agent-context_test.sh docs/BENCHMARKING.md docs/RELEASE.md README.md
git commit -m "Gate context on navigation savings" -m "- Measure tool, source-read, and duplicate-pack behavior from retained transcripts.
- Require structural navigation savings alongside tokens and answer quality.
- Record the latest assisted-run regression without overstating release readiness."
```

### Task 9: Verify Generality, Install Locally, Rescan, and Run the Release Gate

**Files:**
- Modify only if verification reveals a defect: files owned by Tasks 2-8
- Test: all Go and shell tests
- External evidence only: prepared benchmark workspaces, prompts, raw transcripts, metric TSV files, and signed quality rubric

**Interfaces:**
- Consumes: completed Tasks 1-8.
- Produces: a locally installed source-backed `goregraph 1.3.0`, fresh benchmark indexes, and retained three-by-three evidence satisfying every gate.

- [ ] **Step 1: Run focused cross-language and cross-service regressions**

```bash
go test ./internal/scan -run 'JavaAPIContracts|WorkspaceAgentContextPreservesCrossRepositoryHTTPPath' -count=1
go test ./internal/agent -run 'BuildContextReplacesCrossServiceDiscovery|ContextSelectionIsLanguageNeutral|ContextDuplicateResponse' -count=1
```

Expected: `PASS`.

- [ ] **Step 2: Run the complete repository verification**

```bash
gofmt -w internal/scan/java_api_contracts.go internal/scan/java_api_contracts_test.go internal/agent/context_intent.go internal/agent/context_paths.go internal/agent/context_select.go internal/agent/context_cross_service_test.go
go test ./... -count=1
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context_test.sh
```

Expected: all `PASS` and no formatting diff after a second `gofmt -d` check.

- [ ] **Step 3: Inspect the final diff and run a strict review**

```bash
git status --short
git diff --check
git diff --stat
```

Expected: only planned files are modified, `git diff --check` is silent, and no generated benchmark data is staged.

- [ ] **Step 4: Install the current source locally**

```bash
go install ./cmd/goregraph
command -v goregraph
goregraph version
go version -m "$(command -v goregraph)"
```

Expected: the command resolves to the local Go bin, reports `goregraph 1.3.0`, and the build metadata points at the current source revision.

- [ ] **Step 5: Clean and rebuild each prepared benchmark workspace**

For every externally prepared workspace, set its path explicitly and run:

```bash
benchmark_workspace=${GOREGRAPH_BENCHMARK_WORKSPACE:?set GOREGRAPH_BENCHMARK_WORKSPACE to one prepared workspace}
goregraph workspace clean "$benchmark_workspace" --execute --workspace "$benchmark_workspace"
goregraph workspace build agent "$benchmark_workspace" --workspace "$benchmark_workspace"
goregraph workspace status "$benchmark_workspace" --workspace "$benchmark_workspace"
```

Expected: every discovered project has a complete, fresh agent projection; the workspace agent index is complete; no dashboard build is required for the benchmark.

- [ ] **Step 6: Run a preflight against the complete task**

```bash
goregraph context "$benchmark_workspace" \
  --query "$(cat "$GOREGRAPH_BENCHMARK_PROMPT")" \
  --budget-tokens 4000 \
  --max-files 12 \
  --format json
```

Expected: exactly one reliable production entrypoint, production source from every relevant project, a visible HTTP-contract boundary or honest related-production candidate, no production concern displaced by tests, and `retry_allowed: false` when no new semantic candidate exists.

- [ ] **Step 7: Run the matched three-by-three benchmark**

```bash
scripts/benchmark-agent-context.sh \
  --workspace "$benchmark_workspace" \
  --prompt "$GOREGRAPH_BENCHMARK_PROMPT" \
  --baseline-instruction "$GOREGRAPH_BASELINE_INSTRUCTION" \
  --assisted-instruction "$GOREGRAPH_ASSISTED_INSTRUCTION" \
  --runs 3 \
  --output "$GOREGRAPH_BENCHMARK_OUTPUT"
```

Expected: token, tool-call, source-read, duplicate-pack, and workflow gates all pass. If any automated gate fails, stop; do not mark 1.3.0 ready.

- [ ] **Step 8: Complete the external quality rubric**

Score all six retained transcripts against the existing twelve questions. Require assisted median quality greater than or equal to baseline. Retain the signed rubric, prompts, exact invocation settings, summary, analyzer rows, raw transcripts, GoreGraph version, and workspace snapshot identifier outside the repository.

- [ ] **Step 9: Record readiness only after every gate passes**

Update the unreleased 1.3.0 section in `docs/RELEASE.md` with aggregate medians, run date, and pass/fail status only; do not include proprietary paths or transcript contents.

- [ ] **Step 10: Commit final verification documentation**

```bash
git add docs/RELEASE.md
git commit -m "Record context release-gate evidence" -m "- Document matched token and navigation medians.
- Record quality parity and retained external evidence.
- Keep 1.3.0 unreleased pending the normal publication decision."
```

---

## Deferred Work Outside 1.3.0

The following items are intentionally excluded until the Java-first design passes the release gate:

- embeddings or vector search;
- persisted community detection;
- full program-dependence graphs;
- automatic Codex hooks that rewrite unrelated tool results;
- cross-repository data flow beyond statically resolved contract boundaries;
- outbound HTTP extractors for Go, TypeScript, Python, PHP, Rust, C#, Kotlin, and Swift.

Those languages already share the new concern, path, and source selectors. A later extractor adds `APIContractRecord` values; it does not fork retrieval behavior.

---

## Plan Self-Review Checklist

- [ ] The last run's token improvement and navigation regression are both represented.
- [ ] The plan fixes the missing Java outbound-contract evidence without benchmark-specific names.
- [ ] The path planner can traverse an existing cross-service chain and label a future/disconnected change candidate honestly.
- [ ] Source selection is driven by required concern coverage per token and is language-neutral.
- [ ] Identical retries become a minimal response and cannot repeat full source payloads.
- [ ] Release acceptance includes quality, tokens, tool calls, source reads, and duplicate packs.
- [ ] Tests cover Java first and selector parity across Java, Go, TypeScript, and Python.
- [ ] No task publishes 1.3.0 or commits proprietary benchmark evidence.
