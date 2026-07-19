# General Context Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct generic route extraction and make broad change-analysis Context Packs retain one reliable primary path while adding a bounded, honest view of semantically relevant projects before source bodies are attached.

**Architecture:** Fix Java annotation literal handling at its shared parser boundary, without special-casing Spring routes. Preserve the primary-query seed and bounded connected call chain, then run a separate full-query support pass that may add files—but never entrypoints—from explicitly named or strongly relevant projects. Workspace indexes represent missing project projections as unavailable coverage so the compiler cannot silently imply completeness.

**Tech Stack:** Go 1.23+ standard library, existing Java/Spring extraction, Schema 3 agent context records, deterministic lexical ranking, Go tests.

## Global Constraints

- Use strict TDD: write one focused failing test, run it and observe the expected failure, implement the smallest behavior, then rerun focused and package tests.
- Add no dependency and do not change Schema 3 JSON field names or the standard one-tool MCP surface.
- Do not hardcode proprietary repository names, service names, route fragments, class names, symbols, German business nouns, benchmark prompt text, or benchmark paths in production code or new tests.
- Keep exactly one primary production entrypoint. Supporting facts must never appear in `entrypoints` and must never displace the accepted primary path or its bounded chain.
- Prefer connected graph facts. Add independent supporting facts only through a separate bounded full-query pass.
- Treat tests and generated metadata as ineligible supporting production facts.
- Project matching must use deterministic normalized exact aliases. A basename alias is usable only when unique in the loaded index; ambiguous aliases must not select a project.
- A project name alone is not semantic relevance. An explicitly named project still requires at least one non-project query-token match; an unnamed project requires at least two.
- Add at most one independent supporting fact per project and at most two supporting projects. Admit supports after the primary chain through the existing byte/token and file-count gates.
- Preserve byte-identical output for identical inputs and input-order independence.
- Missing secondary project coverage produces uncertainty, not fallback, when the primary entrypoint remains reliable.
- Context support ranking is language- and extension-neutral. Mixed-language fact fixtures must prove that no source language is privileged.
- Use synthetic `services/catalog`, `services/jobs`, `libraries/shared-model`, and `services/reporting` fixtures in tests. These names describe generic architecture only.
- Use English source comments, tests, documentation, and commit messages.

---

### Task 1: Preserve quoted Java annotation braces

**Files:**
- Modify: `internal/scan/extract_java.go:1250-1257`
- Modify: `internal/scan/spring_extract.go:853-875`
- Test: `internal/scan/extract_java_test.go`

**Interfaces:**
- Consumes: `parseJavaAnnotationAttributes`, `splitSpringPaths`, and `isSpringPathArray`.
- Produces: `trimJavaValue(value string) string` that removes quotes from one literal but leaves annotation-array braces for its domain consumer, annotation continuation handling that retains literal-only lines, and OpenAPI empty-container handling at the Spring extraction boundary.

- [ ] **Step 1: Write the failing regression**

Add `TestSpringIndexPreservesPathVariablesAndMappingArrays`. Extract this synthetic controller and build its Spring index:

```go
source := extractJavaSource(FileRecord{Path: "src/main/java/CatalogController.java", Language: "java"}, `@RestController
@RequestMapping("/catalog/")
class CatalogController {
  @DeleteMapping(path = "{catalogId}/entries/{entryId:.+}")
  void deleteOne() {}

  @DeleteMapping(path = {
      "{catalogId}/entries/{entryId:.+}",
      "{catalogId}/entries/by-key/{entryId:[a-z]+}"
  })
  void deleteMany() {}
}`)
index := buildSpringIndex([]JavaSourceRecord{source})
```

Collect every `DELETE` endpoint path and require exactly these two unique paths:

```go
want := []string{
    "/catalog/{catalogId}/entries/{entryId:.+}",
    "/catalog/{catalogId}/entries/by-key/{entryId:[a-z]+}",
}
```

- [ ] **Step 2: Verify the regression fails for the parser bug**

Run:

```bash
go test ./internal/scan -run '^TestSpringIndexPreservesPathVariablesAndMappingArrays$' -count=1
```

Expected: `FAIL`; the first path is missing its outer path-variable braces.

- [ ] **Step 3: Make literal, array, and continuation ownership explicit**

Change `trimJavaValue` so it trims whitespace and surrounding double quotes only:

```go
func trimJavaValue(value string) string {
    return strings.Trim(strings.TrimSpace(value), `"`)
}
```

Do not add Spring/path detection here. `splitSpringPaths` and `isSpringPathArray` remain responsible for unwrapping `{...}` annotation arrays.

While `annotationSignature` is active, process the cleaned non-empty source line before applying the `lexicalLine == ""` skip. Java's lexical sanitizer intentionally blanks string contents, so a quoted continuation can otherwise disappear even though it belongs to the active annotation. Preserve the existing annotation-boundary fallthrough and do not change empty-line handling outside an active annotation.

Because empty annotation arrays now remain `{}`, update `explicitEmptyOpenAPISecurity` to delegate `SecurityRequirements.value` and `Operation.security` to `emptyOpenAPIContainerExpression`. Preserve the existing explicit `@SecurityRequirement()` forms and do not classify non-empty arrays as public.

- [ ] **Step 4: Verify focused and package behavior**

Run:

```bash
go test ./internal/scan -run '^TestSpringIndexPreservesPathVariablesAndMappingArrays$' -count=1
go test ./internal/scan -count=1
```

Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/extract_java.go internal/scan/spring_extract.go internal/scan/extract_java_test.go
git commit -m "Preserve braces in Java annotation strings" -m "- Keep path-variable braces in quoted annotation values.
- Retain array-valued Spring mappings through the dedicated path splitter.
- Interpret empty OpenAPI security containers at the OpenAPI extraction boundary."
```

### Task 1b: Prove parameterized route parity

**Files:**
- Test: `internal/scan/rich_graph_test.go`

**Interfaces:**
- Consumes: the existing Go, PHP, JavaScript/TypeScript, Python, and Rust route extractors.
- Produces: characterization coverage proving that supported literal route syntaxes preserve parameter and regex braces.

- [ ] **Step 1: Add a table-driven route-literal parity test**

Add `TestExtractCodeRoutesPreservesParameterizedLiteralsAcrossLanguages`. For each supported extractor, call `extractCodeIntelligence` with one parameterized route in syntax that extractor already recognizes:

- Go router: `/records/{recordId:[0-9]{2,4}}`
- PHP/Laravel: `/records/{record}`
- TypeScript/Express: `/records/:recordId([0-9]{2,4})`
- Python/FastAPI: `/records/{record_id:path}`
- Rust/Actix or Rocket: `/records/{record_id:[0-9]{2,4}}`

Require exactly one route, the extractor's current framework label, and byte-identical `Path` for every case. Assert `FrameworkBound` only for the TypeScript/Express case, where imports and factory initialization provide actual binding evidence; require it to remain false for syntax-classified Go, PHP, Python, and Rust cases. Name the Rust case `Rust attribute syntax`, because its shared `Actix/Rocket` label does not prove one specific framework. Do not imply multiline or framework coverage that does not exist.

- [ ] **Step 2: Run the focused characterization**

```bash
go test ./internal/scan -run '^TestExtractCodeRoutesPreservesParameterizedLiteralsAcrossLanguages$' -count=1
```

Expected: `PASS`. If a case fails, fix only an actual shared literal-corruption path and rerun the full scan package. If every case passes, make no production change.

- [ ] **Step 3: Commit**

```bash
git add internal/scan/rich_graph_test.go
git commit -m "Cover parameterized routes across extractors" -m "- Prove supported language extractors preserve route and regex braces.
- Keep parity coverage limited to syntax the scanners actually support."
```

### Task 2: Represent unavailable workspace projections

**Files:**
- Modify: `internal/scan/workspace_reconcile.go:1075-1158,1868-1895`
- Test: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: `WorkspaceRegistryRecord`, `workspaceAgentContextBuilder.projects`, `projectFactIDs`, `contextAgentCapability`, and `CoverageUnavailable`.
- Produces: deterministic `UNAVAILABLE` records for each agent capability when an indexed registry project has no merged agent projection.

- [ ] **Step 1: Write the failing workspace coverage test**

Add `TestBuildWorkspaceAgentContextIndexMarksMissingProjectProjectionUnavailable`. Build a registry with indexed `services/catalog` and `services/jobs`, but pass a project context index only for `services/catalog`. Require `services/jobs` to contain exactly one `UNAVAILABLE` coverage record for each of:

```go
[]string{"api_clients", "calls", "persistence", "routes", "tests"}
```

Every record must use reason `project agent context projection unavailable`. Reverse the registry projects and require `reflect.DeepEqual` output.

- [ ] **Step 2: Verify the coverage test fails**

Run:

```bash
go test ./internal/scan -run '^TestBuildWorkspaceAgentContextIndexMarksMissingProjectProjectionUnavailable$' -count=1
```

Expected: `FAIL`; the missing project currently emits no coverage.

- [ ] **Step 3: Add missing-projection coverage**

Add a deterministic helper called from `workspaceAgentContextBuilder.index` before materializing `coverage`:

```go
func (builder *workspaceAgentContextBuilder) addUnavailableProjectCoverage() {
    capabilities := []CapabilityID{
        CapabilityAPIClients,
        CapabilityCalls,
        CapabilityPersistence,
        CapabilityRoutes,
        CapabilityTests,
    }
    projects := make([]string, 0, len(builder.projects))
    for project := range builder.projects {
        projects = append(projects, project)
    }
    sort.Strings(projects)
    for _, project := range projects {
        if _, merged := builder.projectFactIDs[project]; merged {
            continue
        }
        for _, capability := range capabilities {
            key := project + "\x00" + string(capability)
            builder.coverageByKey[key] = AgentContextCoverageRecord{
                Project: project, Capability: string(capability),
                Coverage: string(CoverageUnavailable),
                Reason: "project agent context projection unavailable",
            }
        }
    }
}
```

`mergeProjectIndex` already initializes `projectFactIDs[project]` before merging facts, so a valid empty index is not mislabeled missing.

- [ ] **Step 4: Verify focused and package behavior**

Run:

```bash
go test ./internal/scan -run '^TestBuildWorkspaceAgentContextIndexMarksMissingProjectProjectionUnavailable$' -count=1
go test ./internal/scan -count=1
```

Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go
git commit -m "Expose unavailable workspace context projections" -m "- Record missing indexed project projections as unavailable capabilities.
- Keep workspace coverage deterministic across registry order."
```

### Task 3: Add bounded project-aware supporting facts

**Files:**
- Modify: `internal/agent/context_rank.go`
- Test: `internal/agent/context_test.go`

**Interfaces:**
- Consumes: the existing primary `rankContextFacts`, retained seed, connected edge expansion, `tryContextPack`, `mergeContextFile`, `ContextUncertainty`, facts, and coverage.
- Produces: `contextProjectAliases`, `rankContextSupportFacts`, and `selectContextSupportFacts`; at most one support per project and two supporting projects.

- [ ] **Step 1: Write the failing explicitly named-project test**

Add `TestBuildContextAddsSupportingFactsFromNamedProjects`. Use a synthetic index containing:

```go
catalogRoute := scan.AgentContextFactRecord{
    ID: "catalog-route", Project: "services/catalog", Kind: "route",
    Name: "DELETE /catalog/{catalogId}/entries/{entryId}", HTTPMethod: "DELETE",
    Path: "/catalog/{catalogId}/entries/{entryId}", File: "CatalogController.java",
    Confidence: "EXACT", Search: "delete catalog entry",
}
catalogService := scan.AgentContextFactRecord{
    ID: "catalog-service", Project: "services/catalog", Kind: "symbol",
    Name: "deleteEntry", File: "CatalogService.java", Confidence: "EXACT",
}
jobsClient := scan.AgentContextFactRecord{
    ID: "jobs-client", Project: "services/jobs", Kind: "symbol",
    Name: "deleteEntryJobs", File: "JobsClient.go", Confidence: "EXACT",
    Search: "delete entry jobs internal client authentication retry",
}
sharedModel := scan.AgentContextFactRecord{
    ID: "shared-model", Project: "libraries/shared-model", Kind: "symbol",
    Name: "JobReference", File: "JobReference.ts", Confidence: "EXACT",
    Search: "entry job catalog identifier persistence",
}
reportingNoise := scan.AgentContextFactRecord{
    ID: "reporting", Project: "services/reporting", Kind: "symbol",
    Name: "deleteReport", File: "Reporting.py", Confidence: "EXACT",
    Search: "delete entry retry persistence",
}
```

Connect only `catalog-route -> catalog-service`. Query:

```text
When a catalog entry is deleted, related jobs remain. Analyze services/catalog, services/jobs, and libraries/shared-model for the public endpoint, internal client authentication and retry, identifiers, persistence, and tests.
```

Require exactly one entrypoint (`catalog-route`), the unchanged primary chain, files from all three named projects, no `Reporting.go`, and no test-source support.

- [ ] **Step 2: Write the failing unnamed-but-strongly-relevant test**

Add `TestBuildContextAddsOnlyStrongCrossProjectSupportWhenProjectsAreUnnamed`. Query only the catalog-entry/jobs failure without project names. Use a supporting project whose full path and unique basename do not occur in the query, such as `services/worker`, while its facts match at least two non-project semantic terms such as `job`, `retry`, or `persistence`. Exclude a different project whose best fact matches only `retry`, and retain exactly one entrypoint. Do not use `services/jobs` for this case: the query token `jobs` would make that unique basename an explicit project alias.

- [ ] **Step 3: Write coverage, budget, ambiguity, and determinism regressions**

Add table-driven cases that require:

- an explicitly named project still needs one semantic token after its project-name tokens are removed;
- duplicate basenames such as `services/jobs` and `libraries/jobs` make bare `jobs` ambiguous, while full paths remain exact;
- a named project represented only by `UNAVAILABLE` coverage adds uncertainty without setting `fallback_required` when the central path is reliable;
- `BudgetTokens: 256` may drop all optional supports but must retain the central entrypoint;
- reversing facts, edges, and coverage yields `reflect.DeepEqual` packs.

- [ ] **Step 4: Verify the new tests fail for missing support selection**

Run:

```bash
go test ./internal/agent -run 'TestBuildContext(AddsSupportingFactsFromNamedProjects|AddsOnlyStrongCrossProjectSupportWhenProjectsAreUnnamed|.*Project.*Coverage|.*Project.*Ambigu)' -count=1
```

Expected: `FAIL`; independent project facts are not currently selected and missing named projects are silent.

- [ ] **Step 5: Implement a separate support pass**

Keep `rankContextFacts` and `selectContextSeeds` unchanged for the primary entrypoint. After connected call-chain and neighbor expansion:

1. Build unique normalized aliases from all non-empty `fact.Project` and `coverage.Project` values. Full project paths are always aliases; basenames are aliases only when unique.
2. Detect exact aliases in the normalized full query.
3. Rank eligible non-test, non-generated facts against `contextQueryTokens(request.Query)`, excluding every token belonging to that fact's project aliases from the semantic match count.
4. First select at most one qualifying fact from each explicitly named, unrepresented project with at least one semantic match. Then fill remaining capacity from unrepresented projects with at least two semantic matches. Sort by explicitness, semantic matches descending, score descending, project, kind, qualified name, file, line, and ID.
5. Admit at most two projects and one fact per project through `tryContextPack`. Add only `ContextFile{Role: "related_project", Reason: "full task project match"}` and the fact ID to `includedFactIDs`; never append a support to `Entrypoints` or `CallChain`.
6. For an explicitly named project with no accepted qualifying fact, add a bounded `ContextUncertainty` with scope `<project>/project_context` and reason taken from its strongest incomplete coverage record, or `no relevant production fact selected` when no explicit coverage failure exists.

Use constants:

```go
const maximumContextSupportingProjects = 2
```

The support pass is optional: failure to fit it must not change the primary pack or primary fallback decision.

- [ ] **Step 6: Verify focused, package, and deterministic behavior**

Run:

```bash
go test ./internal/agent -run 'TestBuildContext(AddsSupportingFactsFromNamedProjects|AddsOnlyStrongCrossProjectSupportWhenProjectsAreUnnamed|.*Project.*Coverage|.*Project.*Ambigu|KeepsOnlyHighestRankedProductionSeed|UsesProductionEntrypointsForLongAnalysisRequests)' -count=1
go test ./internal/agent -count=1
```

Expected: `PASS`.

- [ ] **Step 7: Commit**

```bash
git add internal/agent/context_rank.go internal/agent/context_test.go
git commit -m "Add bounded cross-project context support" -m "- Preserve one primary path while selecting semantically relevant project files.
- Surface missing named-project context without benchmark-specific ranking rules."
```

### Task 4: Verify the corrected metadata-only checkpoint

**Files:**
- Modify only if a generic contract assertion requires it: `TOKEN_EFFICIENCY_PLAN.md`

**Interfaces:**
- Consumes: Tasks 1-3 and the existing benchmark harness.
- Produces: one review-clean pre-source commit and binary suitable for the mandatory metadata-only three-run control.

- [ ] **Step 1: Run formatting and focused static checks**

```bash
gofmt -w internal/scan/extract_java.go internal/scan/extract_java_test.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/agent/context_rank.go internal/agent/context_test.go
go vet ./internal/agent ./internal/scan
```

- [ ] **Step 2: Run the full suite**

```bash
go test ./... -count=1
```

Expected: `PASS`.

- [ ] **Step 3: Inspect a generic synthetic workspace**

Create no committed proprietary fixture. Use a temporary three-project workspace with catalog, jobs, and shared-model names. Verify one correct brace-preserving route, one primary entrypoint, the bounded connected chain, at most two supporting projects, honest uncertainty for a missing projection, and byte-identical repeated JSON output.

- [ ] **Step 4: Record checkpoint provenance outside the repository**

Record the commit, binary SHA-256, version, Go version, platform, index digest, prompt digest, Codex version, model, reasoning, sandbox, approvals, and exact benchmark instructions. The historical proprietary workspace is acceptance evidence only and must not alter implementation rules or committed tests.

## Self-Review Checklist

- [ ] No production or test rule names a proprietary service, repository, route, class, symbol, prompt, or path.
- [ ] Quoted annotation strings keep braces; annotation path arrays still expand.
- [ ] Existing Go, PHP, TypeScript, Python, and Rust route extractors preserve parameterized literals.
- [ ] Missing indexed workspace projections are explicit `UNAVAILABLE` coverage.
- [ ] Primary ranking still uses the first substantive problem statement and returns exactly one reliable production entrypoint.
- [ ] Supporting facts use the full query, remain bounded, never become entrypoints, and never displace the primary chain.
- [ ] Explicit project names are unique exact aliases and still require semantic relevance.
- [ ] Unnamed project support requires at least two semantic matches.
- [ ] Tests and generated metadata cannot become production supports.
- [ ] Secondary coverage gaps create uncertainty without discarding a reliable central pack.
- [ ] Output remains deterministic and within all existing budgets.
