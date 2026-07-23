# Agent Context Change-Analysis V2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make one GoreGraph Context Pack materially reduce agent exploration for cross-project change tasks where the requested integration does not exist yet, while clearly separating the current call chain, adjacent implementation evidence, and the missing contract.

**Architecture:** Keep the existing Schema 3 fact-and-edge index and bounded 4,000-token Context Pack. Extend concern planning with canonical multilingual tokens and project-scoped operational concerns, then select a bounded role-balanced set of adjacent production facts when no connected target path exists. Improve Java path and Spring security extraction at the source, render compact verified declaration bodies, and decide coverage, fallback, and retry only after source rendering. Preserve the primary call chain as observed truth; adjacent patterns remain explicitly related evidence and never become fabricated future edges.

**Tech Stack:** Go 1.23+ standard library, existing Java/Spring static extractor, Schema 3 agent context records, deterministic lexical and graph ranking, Go tests, POSIX shell benchmark harness, local Codex CLI acceptance runs.

## Global Constraints

- Work on the unreleased source. Do not tag, publish, or run a release workflow.
- Keep the CLI short-help changes in `internal/cli/cli.go` and `internal/cli/cli_test.go` in their earlier dedicated commit; do not amend or rewrite them as part of this plan.
- Use strict TDD for each production behavior: write a focused failing test, run it and observe the intended failure, implement the smallest behavior, rerun the focused test, then run the owning package.
- Add no runtime dependency, embedding model, vector database, network lookup, or tokenizer dependency.
- Do not encode proprietary repository names, absolute paths, route fragments, symbols, prompt text, or business nouns in production code or committed test fixtures.
- Use only the synthetic projects `services/catalog`, `libraries/job-client`, and `services/jobs` in the new change-analysis fixture.
- Keep the default Context Pack budget at 4,000 estimated tokens, the accepted range at 256..6,000 tokens, the default file limit at 12, the source-section cap at 12, and the serialized byte ceiling at 24,000 bytes.
- Keep exactly one primary production entrypoint and only evidence-backed relationships in `call_chain`.
- Mark disconnected production facts as adjacent or related evidence through concern reasons, file roles, source roles, and uncertainties. Never create a call edge or HTTP-contract edge to represent code that should be added in the future.
- Prefer production source over tests. Requested tests may be selected only after every coverable required production concern has current source.
- `source_coverage: complete` means that every required concern is represented by verified current source, including an explicit evidence-backed absence statement for a requested contract that does not exist.
- `fallback_required: false` is valid for change analysis when the current path and adjacent patterns are sufficiently evidenced even though the requested future contract is absent.
- `retry_allowed: true` is valid only when a concrete unselected fact can cover an actual post-render omission. It is not a generic invitation to search again.
- Preserve deterministic, input-order-independent output and stable tie-breaking.
- Keep query planning and support selection language-neutral. Language-specific parsing stays under `internal/scan`.
- Keep source contents out of context identity hashes.
- Use English source comments, test names, CLI text, documentation, and commit messages.

---

## Last-Run Evidence and Acceptance Contract

The latest external diagnostic pair is substantially below the existing release gates:

| Metric | Baseline | Assisted | Result |
|---|---:|---:|---:|
| End-to-end tokens | 169,913 | 166,833 | 1.81% reduction |
| Shell executions | 28 | 31 | 10.71% increase |
| Approximate elapsed time | 4:39 | 4:19 | 20 seconds faster |

The assisted pack found the public endpoint and local persistence path, but it selected only shallow or generic evidence from the supporting projects. It spent source budget on broad 50-61-line windows, reported mutually contradictory endpoint security as exact, reduced a composed Java client URL to `GET /`, omitted a repository fact because the inherited method had no declaration, and suggested unrelated create-method retry anchors. The agent therefore resumed normal source discovery and nearly erased the token benefit.

This is diagnostic evidence only. It is not a controlled three-by-three release result.

The implementation is complete only when all conditions below hold:

1. The synthetic missing-contract fixture yields exactly one primary endpoint and preserves only its observed current call chain.
2. The pack includes verified production source from all three explicit projects without inventing the requested future HTTP contract.
3. The client project contributes an adjacent client contract, authentication, configuration, and retry pattern; the provider project contributes its adjacent controller/service path, a domain-specific repository method, side-effect evidence, and requested tests.
4. Generic inherited CRUD facts such as `findAll` do not displace a declared domain-specific finder in the same project and concern.
5. The absence of the requested contract appears as an explicit uncertainty; it does not silently become `GET /`, a fabricated edge, or a low-confidence fallback.
6. A complete pack has `fallback_required: false`, `retry_allowed: false`, `source_coverage: complete`, at most 4,000 estimated tokens, at most 12 source sections, and at most 12 files.
7. The German and English forms of the same synthetic task produce identical concern keys and selected fact IDs.
8. A Spring route constant referenced as `Type.CONSTANT` resolves across source files.
9. A Java client URL selected by a local ternary between two configuration getters resolves to bounded real paths; a configuration base URL is removed only when provenance proves it is a base URL.
10. Spring filter-chain authentication is applied only to compatible route scopes. Conflicting matched rules are reported as partial/conflicting evidence, never as one exact list of public, basic, bearer, authenticated, and role requirements.
11. Markdown renders coverage and endpoint security, so the default output exposes the same decision-critical metadata as JSON.
12. The official matched three-by-three benchmark passes quality parity, assisted token median at most 80% of baseline, tool-call median at most 70%, source-read median at most 50%, zero repeated full packs, and zero re-reads of files from a complete Context Pack.

---

## File and Interface Map

### New file

- `internal/agent/context_change_analysis_test.go` owns the synthetic three-project missing-contract regression and its English/German parity assertion.

### Modified files

- `internal/agent/context_intent.go` owns canonical concern vocabulary, explicit-project operational concerns, and the twelve-concern public cap.
- `internal/agent/context_rank.go` owns canonical query tokens, structural support roles, domain-specific persistence preference, and retry-candidate ranking.
- `internal/agent/context_paths.go` owns bounded disconnected production support selection while preserving the observed primary path.
- `internal/agent/context_select.go` owns project/concern source boundaries, inherited-owner render alternatives, and post-render concern coverage.
- `internal/agent/context_source.go` owns compact declaration-body ranges and verified inherited-owner source sections.
- `internal/agent/context.go` finalizes fallback, uncertainty, and retry metadata after source attachment.
- `internal/agent/context_test.go` owns intent, support-ranking, absence, fallback, and retry unit regressions.
- `internal/agent/context_source_test.go` owns compact rendering and inherited-owner regressions.
- `internal/scan/spring_extract.go` owns qualified Spring constant aliases and path-scoped filter-chain application.
- `internal/scan/java_api_contracts.go` owns configuration-getter provenance and bounded ternary path resolution.
- `internal/scan/java_api_contracts_test.go` owns getter/base-URL/ternary contract regressions.
- `internal/scan/extract_java_test.go` owns retained `securityMatcher` call provenance.
- `internal/query/context.go` renders coverage concerns and endpoint metadata.
- `internal/query/context_test.go` owns Markdown regressions.
- `scripts/analyze-agent-context-log.go` records source files included by complete packs and later re-reads.
- `scripts/analyze-agent-context-log_test.sh` owns analyzer lifecycle and path-normalization regressions.
- `scripts/benchmark-agent-context.sh` carries the new metric into `summary.tsv` and fails assisted re-reads.
- `scripts/benchmark-agent-context_test.sh` owns summary and gate regressions.
- `docs/BENCHMARKING.md` and `docs/RELEASE.md` document the latest diagnostic and the added structural gate.

---

## Task 1: Add a Missing-Contract Regression Fixture

**Files:**

- Create: `internal/agent/context_change_analysis_test.go`
- Test: `internal/agent/context_change_analysis_test.go`

**Interfaces:**

- Reuse `writeContextIndexFixture`, `writeContextIndexAt`, and the existing source-fixture helpers from package `agent`.
- Add `writeMissingContractContextFixture(t *testing.T) string`.
- Add `contextSelectedFactSet(pack ContextPack) map[string]bool`.
- Add `contextHasUncertainty(pack ContextPack, scope string) bool`.

- [ ] **Step 1: Write the synthetic current and adjacent production sources**

Create small valid Java files for this topology:

```text
services/catalog
  CatalogController.deleteItem
    -> CatalogOperations.deleteItem
      -> CatalogRepository.deleteById

libraries/job-client
  JobClient.listJobs
    -> GET /job-management/jobs
  JobClientConfig.getAllJobsPath
  JobClientAuth.basicAuthentication
  JobClientRetry.retryPolicy

services/jobs
  JobManagementController.listJobs
    -> JobService.listJobs
      -> JobRepository.findByCatalogIdAndItemId
  JobRepository.findAll              (inherited, deliberately absent in source)
  JobHousekeeping.publishDeletion
  JobManagementControllerTest.listJobs
```

Do not add a cleanup client method, cleanup provider route, cleanup contract fact, or edge from `CatalogOperations.deleteItem` into `libraries/job-client`. The missing target integration is the central test condition.

- [ ] **Step 2: Write the complete fact and edge fixture**

Use exact fact roles and confidence:

```go
facts := []scan.AgentContextFactRecord{
    {ID: "catalog-route", Project: "services/catalog", Kind: "route", Name: "DELETE /catalog/items/{itemId}", Qualified: "CatalogController.deleteItem", HTTPMethod: "DELETE", Path: "/catalog/items/{itemId}", File: "src/main/java/example/CatalogController.java", Line: 8, EndLine: 12, Confidence: "EXACT", Search: "delete catalog item"},
    {ID: "catalog-operations", Project: "services/catalog", Kind: "symbol", Name: "deleteItem", Qualified: "CatalogOperations.deleteItem", File: "src/main/java/example/CatalogOperations.java", Line: 9, EndLine: 14, Confidence: "EXACT", Search: "delete catalog item operations"},
    {ID: "catalog-repository", Project: "services/catalog", Kind: "persistence", Name: "deleteById", Qualified: "CatalogRepository.deleteById", File: "src/main/java/example/CatalogRepository.java", Line: 5, EndLine: 7, Confidence: "RESOLVED", Search: "delete catalog item persistence"},

    {ID: "job-client", Project: "libraries/job-client", Kind: "symbol", Name: "listJobs", Qualified: "JobClient.listJobs", File: "src/main/java/example/JobClient.java", Line: 14, EndLine: 20, Confidence: "EXACT", Search: "job task client configuration authentication retry"},
    {ID: "job-contract", Project: "libraries/job-client", Kind: "api_contract", Name: "GET /job-management/jobs", Qualified: "JobClient.listJobs", HTTPMethod: "GET", Path: "/job-management/jobs", File: "src/main/java/example/JobClient.java", Line: 16, EndLine: 18, Confidence: "RESOLVED", Search: "job task client contract"},
    {ID: "job-config", Project: "libraries/job-client", Kind: "configuration", Name: "getAllJobsPath", Qualified: "JobClientConfig.getAllJobsPath", File: "src/main/java/example/JobClientConfig.java", Line: 10, EndLine: 13, Confidence: "EXACT", Search: "job task client configuration base url"},
    {ID: "job-auth", Project: "libraries/job-client", Kind: "authentication", Name: "basicAuthentication", Qualified: "JobClientAuth.basicAuthentication", File: "src/main/java/example/JobClientAuth.java", Line: 7, EndLine: 10, Confidence: "EXACT", Search: "job task client basic authentication"},
    {ID: "job-retry", Project: "libraries/job-client", Kind: "resilience", Name: "retryPolicy", Qualified: "JobClientRetry.retryPolicy", File: "src/main/java/example/JobClientRetry.java", Line: 7, EndLine: 12, Confidence: "EXACT", Search: "job task retry exception handling resilience"},

    {ID: "jobs-route", Project: "services/jobs", Kind: "route", Name: "GET /job-management/jobs", Qualified: "JobManagementController.listJobs", HTTPMethod: "GET", Path: "/job-management/jobs", File: "src/main/java/example/JobManagementController.java", Line: 10, EndLine: 14, Confidence: "EXACT", Search: "job task management endpoint"},
    {ID: "jobs-service", Project: "services/jobs", Kind: "symbol", Name: "listJobs", Qualified: "JobService.listJobs", File: "src/main/java/example/JobService.java", Line: 12, EndLine: 18, Confidence: "EXACT", Search: "job task management service side effect"},
    {ID: "jobs-finder", Project: "services/jobs", Kind: "persistence", Name: "findByCatalogIdAndItemId", Qualified: "JobRepository.findByCatalogIdAndItemId", File: "src/main/java/example/JobRepository.java", Line: 7, EndLine: 8, Confidence: "EXACT", Search: "job task catalog item finder persistence"},
    {ID: "jobs-find-all", Project: "services/jobs", Kind: "persistence", Name: "findAll", Qualified: "JobRepository.findAll", File: "src/main/java/example/JobRepository.java", Line: 6, EndLine: 6, Confidence: "RESOLVED", Search: "job repository inherited find all persistence"},
    {ID: "jobs-side-effect", Project: "services/jobs", Kind: "symbol", Name: "publishDeletion", Qualified: "JobHousekeeping.publishDeletion", File: "src/main/java/example/JobHousekeeping.java", Line: 8, EndLine: 13, Confidence: "EXACT", Search: "job task deletion logging mail user information side effect"},
    {ID: "jobs-test", Project: "services/jobs", Kind: "test", Name: "listJobs", Qualified: "JobManagementControllerTest.listJobs", File: "src/test/java/example/JobManagementControllerTest.java", Line: 10, EndLine: 18, Confidence: "EXACT", Search: "job task management test"},
}
edges := []scan.AgentContextEdgeRecord{
    {ID: "current-1", FromFactID: "catalog-route", ToFactID: "catalog-operations", Kind: "call", Confidence: "EXACT"},
    {ID: "current-2", FromFactID: "catalog-operations", ToFactID: "catalog-repository", Kind: "persistence", Confidence: "RESOLVED"},
    {ID: "adjacent-1", FromFactID: "job-client", ToFactID: "job-contract", Kind: "call", Confidence: "EXACT"},
    {ID: "adjacent-2", FromFactID: "job-contract", ToFactID: "jobs-route", Kind: "http_contract", Confidence: "RESOLVED"},
    {ID: "adjacent-3", FromFactID: "jobs-route", ToFactID: "jobs-service", Kind: "call", Confidence: "EXACT"},
    {ID: "adjacent-4", FromFactID: "jobs-service", ToFactID: "jobs-finder", Kind: "persistence", Confidence: "RESOLVED"},
    {ID: "adjacent-5", FromFactID: "jobs-service", ToFactID: "jobs-side-effect", Kind: "call", Confidence: "RESOLVED"},
    {ID: "adjacent-test", FromFactID: "jobs-test", ToFactID: "jobs-route", Kind: "test_target", Confidence: "EXACT"},
}
```

- [ ] **Step 3: Add the final integration assertions**

Use an English task and its German equivalent:

```go
english := "When DELETE /catalog/items/{itemId} removes an item in services/catalog, plan cleanup of related jobs through libraries/job-client and services/jobs. Cover the current path, missing HTTP contract, authentication, configuration, retry behavior, persistence, side effects, and tests."
german := "Wenn DELETE /catalog/items/{itemId} einen Eintrag in services/catalog löscht, plane das Entfernen verbundener Aufgaben über libraries/job-client und services/jobs. Berücksichtige aktuellen Pfad, fehlenden HTTP-Vertrag, Authentifizierung, Konfiguration, Wiederholung, Persistenz, Nebenwirkungen und Tests."
```

Require:

```go
if len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "catalog-route" {
    t.Fatalf("primary entrypoint = %#v", pack.Entrypoints)
}
for _, relationship := range pack.CallChain {
    if relationship.From == "CatalogOperations.deleteItem" &&
        strings.Contains(relationship.To, "Job") {
        t.Fatalf("fabricated future relationship: %#v", relationship)
    }
}
for _, factID := range []string{
    "catalog-route", "catalog-operations", "catalog-repository",
    "job-client", "job-contract", "job-config", "job-auth", "job-retry",
    "jobs-route", "jobs-service", "jobs-finder", "jobs-side-effect", "jobs-test",
} {
    if !contextSelectedFactSet(pack)[factID] {
        t.Errorf("required evidence %q not selected", factID)
    }
}
if contextSelectedFactSet(pack)["jobs-find-all"] {
    t.Error("generic inherited findAll displaced the declared finder")
}
if !contextHasUncertainty(pack, "requested_http_contract") {
    t.Fatalf("missing contract uncertainty = %#v", pack.Uncertainties)
}
if pack.FallbackRequired || pack.RetryAllowed || pack.SourceCoverage != "complete" {
    t.Fatalf("pack decision = fallback %v retry %v coverage %q", pack.FallbackRequired, pack.RetryAllowed, pack.SourceCoverage)
}
if pack.EstimatedTokens > 4000 || len(pack.SourceSections) > 12 || len(pack.Files) > 12 {
    t.Fatalf("pack bounds = tokens %d sections %d files %d", pack.EstimatedTokens, len(pack.SourceSections), len(pack.Files))
}
```

Also compare the sorted `selectedFactIDs`, `selectedConcernKeys`, entrypoint ID, source roles, fallback, retry, and source coverage for the two queries.

- [ ] **Step 4: Verify the new regression fails for the observed reasons**

Run:

```bash
go test ./internal/agent -run '^TestBuildContextSupportsMissingContractChangeAnalysis$' -count=1
```

Expected: `FAIL`; the current selector omits operational support roles, may choose `findAll`, leaves source coverage partial, and permits an unrelated retry.

- [ ] **Step 5: Keep the red integration test uncommitted**

Do not commit a deliberately failing repository state. Keep this file in the working tree through Tasks 2-7 and stage it only when the complete regression passes.

---

## Task 2: Make Intent and Concerns Multilingual and Project-Scoped

**Files:**

- Modify: `internal/agent/context_intent.go`
- Modify: `internal/agent/context_rank.go`
- Modify: `internal/agent/context_test.go`
- Test: `internal/agent/context_test.go`

**Interfaces:**

- Add concern kinds `configuration`, `resilience`, and `side_effects`.
- Raise `maximumPublicContextConcerns` from 8 to 12.
- Add `contextExpandedTokenSet(value string) map[string]bool`.
- Add `contextExplicitProjectConcernCandidates(...) []string`.
- Preserve `ContextConcern` JSON fields and use the existing `kind:project` key shape.

- [ ] **Step 1: Add failing canonical-token tests**

Add table-driven tests that require these canonical expansions:

```go
tests := map[string][]string{
    "Aufgaben":          {"job", "jobs", "task", "tasks"},
    "Authentifizierung": {"auth", "authentication"},
    "Konfiguration":     {"config", "configuration"},
    "Wiederholung":      {"retry", "resilience"},
    "Persistenz":        {"persistence", "repository"},
    "Nebenwirkungen":    {"side_effect", "side_effects"},
    "Fehlerbehandlung":  {"exception", "resilience"},
}
```

Require `contextProjectSemanticQueryTokens` and `contextValueRequestsConcern` to use the same expansion path. Add a parity test proving that the English and German fixture queries plan identical concern keys.

Run:

```bash
go test ./internal/agent -run '^(TestContextExpandedTokenSet|TestPlanContextConcernsMatchesGermanOperationalTask)$' -count=1
```

Expected: `FAIL`; existing project semantic matching and concern vocabulary use raw tokens.

- [ ] **Step 2: Centralize alias expansion**

Implement:

```go
func contextExpandedTokenSet(value string) map[string]bool {
    result := make(map[string]bool)
    for _, token := range contextQueryTokens(value) {
        result[token] = true
    }
    return result
}
```

Extend `contextQueryTokenAliases` with generic language mappings only:

```go
"aufgabe":            {"job", "jobs", "task", "tasks"},
"aufgaben":           {"job", "jobs", "task", "tasks"},
"authentifizierung":  {"auth", "authentication"},
"konfiguration":      {"config", "configuration"},
"wiederholung":       {"retry", "resilience"},
"wiederholungen":     {"retry", "resilience"},
"fehlerbehandlung":   {"exception", "resilience"},
"persistenz":         {"persistence", "repository"},
"nebenwirkung":       {"side_effect", "side_effects"},
"nebenwirkungen":     {"side_effect", "side_effects"},
"protokollierung":    {"logging", "side_effects"},
"benutzerinformation":{"user_information", "side_effects"},
```

Use `contextExpandedTokenSet` in `contextProjectSemanticQueryTokens` and `contextValueRequestsConcern`. Pass the expanded query-token set into `semanticContextProjectFacts`, but keep its indexed fact tokens on `contextTokenSet`; compare canonical query tokens against normalized fact tokens and generic concern vocabulary.

- [ ] **Step 3: Add project-scoped operational concerns**

Add:

```go
const (
    contextConcernConfiguration = "configuration"
    contextConcernResilience    = "resilience"
    contextConcernSideEffects   = "side_effects"
)
```

Extend `contextConcernVocabulary` and `normalizedContextConcernKind`:

```go
contextConcernConfiguration: {"config", "configuration", "properties", "property"},
contextConcernResilience:    {"exception", "retry", "retries", "resilience", "timeout"},
contextConcernSideEffects:   {"event", "logging", "mail", "notification", "side_effect", "side_effects", "user_information"},
```

For each explicit project, create a required concern only when both conditions hold:

1. the query requests that concern; and
2. the project contains at least one semantically relevant candidate for that concern.

Keep the existing global reachable concerns for tasks that do not name projects. For tests, collect test facts whose `test_target` reaches a selected or adjacent production candidate in an explicit project.

When explicit projects are present, do not also add an equivalent global concern for the same kind. Sort required concerns by entrypoint, primary path, explicit project, then operational kind before applying the twelve-concern cap.

- [ ] **Step 4: Run focused and package tests**

```bash
gofmt -w internal/agent/context_intent.go internal/agent/context_rank.go internal/agent/context_test.go
go test ./internal/agent -run '^(TestContextExpandedTokenSet|TestPlanContextConcernsMatchesGermanOperationalTask|TestPlanContextConcernsScopesOperationalEvidenceToExplicitProjects)$' -count=1
go test ./internal/agent -count=1
```

Expected: focused tests pass. The package may still fail only at `TestBuildContextSupportsMissingContractChangeAnalysis` because ranking and source rendering are not complete.

- [ ] **Step 5: Commit the intent change without the still-red integration file**

```bash
git add internal/agent/context_intent.go internal/agent/context_rank.go internal/agent/context_test.go
git commit -m "Improve project-scoped context intent" \
  -m "- Canonicalize multilingual operational terms through one token path
- Plan configuration, resilience, side-effect, persistence, and test concerns per explicit project"
```

---

## Task 3: Select Role-Balanced Adjacent Production Evidence

**Files:**

- Modify: `internal/agent/context_rank.go`
- Modify: `internal/agent/context_paths.go`
- Modify: `internal/agent/context_select.go`
- Modify: `internal/agent/context_test.go`
- Test: `internal/agent/context_test.go`

**Interfaces:**

- Replace the two-facts-per-project rule with bounded structural role slots.
- Add `contextSupportRole(index, fact) string`.
- Add `contextGenericPersistenceFact(fact) bool`.
- Keep `maximumContextSupportingProjects = 2`.
- Set `maximumContextSupportFactsPerProject = 5`. Tests remain last and consume a slot only in projects that have fewer than five higher-priority requested production facts.

- [ ] **Step 1: Add failing support-role tests**

Build a ranked support list containing client, contract, provider, service, configuration, authentication, resilience, declared persistence, inherited `findAll`, side effect, and test facts in two explicit support projects.

Require:

```go
wantRoles := map[string][]string{
    "libraries/job-client": {"client", "contract", "configuration", "authentication", "resilience"},
    "services/jobs":        {"provider", "service", "persistence", "side_effects", "tests"},
}
```

Require the declared finder to outrank the inherited generic method regardless of input order.

Run:

```bash
go test ./internal/agent -run '^(TestSelectContextSupportFactsBalancesRequestedRoles|TestRankContextSupportFactsPrefersDeclaredPersistence|TestSelectRelatedContextProductionIsDeterministic)$' -count=1
```

Expected: `FAIL`; the current two-fact cap selects shallow or duplicate roles.

- [ ] **Step 2: Classify structural roles deterministically**

Implement a role classifier with this precedence:

```go
func contextSupportRole(index scan.AgentContextIndexRecord, fact scan.AgentContextFactRecord) string {
    switch normalizedContextConcernKind(fact.Kind) {
    case contextConcernHTTPContract:
        return "contract"
    case contextConcernConfiguration:
        return "configuration"
    case contextConcernAuth:
        return "authentication"
    case contextConcernResilience:
        return "resilience"
    case contextConcernPersistence:
        return "persistence"
    case contextConcernSideEffects:
        return "side_effects"
    case contextConcernTests:
        return "tests"
    }
    identifier := strings.ToLower(strings.Join([]string{fact.Name, fact.Qualified, fact.Summary}, " "))
    switch {
    case strings.Contains(identifier, "client"):
        return "client"
    case strings.EqualFold(fact.Kind, "route"), strings.EqualFold(fact.Kind, "api_endpoint"):
        return "provider"
    case contextValueRequestsConcern(identifier, contextConcernSideEffects):
        return "side_effects"
    }
    if contextFactHasOutgoingOperationalEdge(index, fact.ID) {
        return "service"
    }
    return ""
}
```

Use edge kinds as stronger evidence than identifier suffixes. The identifier check is a generic architecture convention, not a project-specific noun.

- [ ] **Step 3: Prefer semantic declared persistence**

Implement:

```go
func contextGenericPersistenceFact(fact scan.AgentContextFactRecord) bool {
    name := strings.ToLower(strings.TrimSpace(fact.Name))
    switch name {
    case "count", "deleteall", "existsbyid", "findall", "findbyid", "save", "saveall":
        return true
    default:
        return strings.Contains(strings.ToLower(fact.Summary), "inherited")
    }
}
```

When two persistence candidates have equal project affinity and concern coverage:

1. prefer non-generic over generic;
2. prefer `EXACT` over `RESOLVED` over `INFERRED`;
3. prefer higher semantic matches;
4. retain the existing stable project/file/line/ID tie-breakers.

- [ ] **Step 4: Select role slots before duplicate facts**

Change `selectContextSupportFacts` to make two deterministic passes:

1. select one candidate for each requested production role in concern priority order;
2. fill remaining per-project capacity by score without duplicating a non-persistence role;
3. add at most one requested test after all coverable production roles.

Use this priority:

```go
var contextSupportRoleOrder = []string{
    "client",
    "contract",
    "provider",
    "service",
    "configuration",
    "authentication",
    "resilience",
    "persistence",
    "side_effects",
    "tests",
}
```

Keep selected adjacent facts out of `pack.CallChain` unless their relationships are existing index edges on a separately evidenced adjacent path. Their `ContextFile.Role` and source role must start with `related_`.

- [ ] **Step 5: Run focused and package tests**

```bash
gofmt -w internal/agent/context_rank.go internal/agent/context_paths.go internal/agent/context_select.go internal/agent/context_test.go
go test ./internal/agent -run '^(TestSelectContextSupportFactsBalancesRequestedRoles|TestRankContextSupportFactsPrefersDeclaredPersistence|TestSelectRelatedContextProductionIsDeterministic)$' -count=1
go test ./internal/agent -run '^(TestBuildContextReplacesCrossServiceDiscovery|TestContextSelectionIsLanguageNeutral|TestSelectContextPaths)' -count=1
```

Expected: all focused tests pass. Existing connected cross-service behavior remains unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/context_rank.go internal/agent/context_paths.go internal/agent/context_select.go internal/agent/context_test.go
git commit -m "Balance adjacent context evidence by role" \
  -m "- Select requested client, service, operational, persistence, and test evidence within fixed bounds
- Prefer declared domain-specific persistence over generic inherited CRUD facts"
```

---

## Task 4: Render Compact Declaration Bodies and Inherited Owners

**Files:**

- Modify: `internal/agent/context_source.go`
- Modify: `internal/agent/context_select.go`
- Modify: `internal/agent/context_source_test.go`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Add render mode `declaration_body`.
- Add `sourceDeclarationBodyRange(path string, lines, codeLines []string, declaration sourceOccurrence) (int, int, bool)`.
- Add `contextInheritedOwnerCandidate(index, candidate) (sourceCandidate, bool)`.
- Add `sourceCandidate.SourceState string`.
- Use `inherited_owner_current` only after verifying the owner declaration in current source.

- [ ] **Step 1: Add failing compact-body tests**

Test:

- a Java method with annotations and a 14-line body;
- a Go function with a nested composite literal;
- a TypeScript method with braces inside strings and comments;
- a Python method whose body ends at the next declaration with equal indentation.

Require `declaration_body` to include the annotation/signature and complete body, exclude adjacent declarations, and remain smaller than the existing 61-line `focused` window.

Run:

```bash
go test ./internal/agent -run '^TestRenderSourceCandidateUsesCompactDeclarationBody$' -count=1
```

Expected: `FAIL`; the render mode does not exist.

- [ ] **Step 2: Implement balanced declaration ranges**

For brace-based files:

1. begin at `sourceAnnotationStart`;
2. find the first declaration-level `{` after the verified declaration;
3. count braces in `codeLines`, where comments and strings are already masked;
4. return at the matching closing brace;
5. reject a body over 120 lines.

For `.py`:

1. begin at decorators immediately above the declaration;
2. record the declaration indentation;
3. continue through blank/comment lines and deeper indentation;
4. stop before the next nonblank line at equal or lower indentation.

Fall back to the current `body`, `focused`, and `signature` modes only when a declaration body cannot be proven.

Change the render order to:

```go
for _, mode := range []string{"declaration_body", "body", "focused", "signature"} {
    // existing option construction
}
```

- [ ] **Step 3: Add failing inherited-owner tests**

Create an interface source containing only:

```java
interface JobRepository extends CrudRepository<JobEntity, Long> {
    List<JobEntity> findByCatalogIdAndItemId(long catalogId, long itemId);
}
```

Index both `JobRepository.findAll` and the `JobRepository` type. Require:

```go
if section.SourceState != "inherited_owner_current" {
    t.Fatalf("source state = %q", section.SourceState)
}
if !strings.Contains(section.Content, "interface JobRepository") {
    t.Fatalf("owner declaration missing: %s", section.Content)
}
if len(pack.SourceOmissions) != 0 {
    t.Fatalf("verified inherited owner became an omission: %#v", pack.SourceOmissions)
}
```

Also require that the inherited-owner alternative is not used when the owner fact is absent, ambiguous, in another file, or from another project.

Run:

```bash
go test ./internal/agent -run '^TestContextSourceUsesVerifiedInheritedOwner$' -count=1
```

Expected: `FAIL`; the current renderer reports that the indexed method is absent.

- [ ] **Step 4: Implement the inherited-owner alternative**

Derive the owner from the qualified name before the final separator. Select an owner fact only when project and file match and exactly one fact has that exact qualified owner. Copy the original candidate fact IDs into the owner alternative so concern coverage remains attached to the inherited operation.

Set:

```go
ownerCandidate.Name = ownerShortName
ownerCandidate.Qualified = ownerQualified
ownerCandidate.StartLine = ownerFact.Line
ownerCandidate.EndLine = ownerFact.EndLine
ownerCandidate.SourceState = "inherited_owner_current"
```

In `verifiedContextSourceFactIDs`, accept the original inherited fact only when the verified section contains that exact owner declaration and the candidate project/file match. Do not claim that the inherited method body exists.

- [ ] **Step 5: Run tests and compare source size**

```bash
gofmt -w internal/agent/context_source.go internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run '^(TestRenderSourceCandidateUsesCompactDeclarationBody|TestContextSourceUsesVerifiedInheritedOwner|TestContextSourceOptions)' -count=1
go test ./internal/agent -count=1
```

Expected: source tests pass; the missing-contract integration may still fail on scanner metadata or final fallback/retry decisions.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/context_source.go internal/agent/context_select.go internal/agent/context_source_test.go
git commit -m "Render compact verified context source" \
  -m "- Prefer complete declaration bodies over broad fixed source windows
- Represent inherited methods through verified current owner declarations"
```

---

## Task 5: Resolve Qualified Spring Constants and Java Client Paths

**Files:**

- Modify: `internal/scan/spring_extract.go`
- Modify: `internal/scan/java_api_contracts.go`
- Modify: `internal/scan/java_api_contracts_test.go`
- Test: `internal/scan/java_api_contracts_test.go`

**Interfaces:**

- Add `springConstantIndex(sources []JavaSourceRecord) map[string]string`.
- Extend `javaIndexedPathExpression` with `baseURLs map[string]bool`.
- Add `javaResolvedPathAlternatives(...) []string`.
- Keep unresolved alternatives partial and bounded to four normalized paths.

- [ ] **Step 1: Add a failing qualified-constant route test**

Create two Java sources:

```java
final class Routes {
    static final String BASE_PATH = "/job-management";
}

@RequestMapping(Routes.BASE_PATH)
final class JobController {
    @GetMapping("/jobs")
    List<Job> listJobs() { return List.of(); }
}
```

Require the Spring endpoint path `/job-management/jobs`, not `/Routes.BASE_PATH/jobs`.

Run:

```bash
go test ./internal/scan -run '^TestBuildSpringIndexResolvesQualifiedCrossFileConstants$' -count=1
```

Expected: `FAIL`; constants are currently indexed only by simple names.

- [ ] **Step 2: Build safe constant aliases**

Index each constant under:

```text
CONSTANT
Type.CONSTANT
package.Type.CONSTANT
```

Retain the simple `CONSTANT` alias only when it is unique across all sources. Always retain qualified aliases. Sort source/type traversal before insertion so input order cannot affect collision handling.

- [ ] **Step 3: Add failing getter/base-URL/ternary tests**

Use:

```java
@ConfigurationProperties
final class JobClientConfig {
    String baseUrl;
    static final String BASE_PATH = "/job-management";
    static final String ALL_JOBS = "/jobs";
    static final String ACTIVE_JOBS = "/jobs?status=active";

    String getAllJobsPath() {
        return baseUrl + BASE_PATH + ALL_JOBS;
    }

    String getActiveJobsPath() {
        return baseUrl + BASE_PATH + ACTIVE_JOBS;
    }
}

final class JobClient {
    void list(String status) {
        String url = status.isEmpty()
            ? config.getAllJobsPath()
            : config.getActiveJobsPath();
        restClient.get().uri(url).retrieve();
    }
}
```

Require two bounded contracts with `Path: "/job-management/jobs"`: one with an empty `Query`, one with `Query: "status=active"`, both `RESOLVED`, and no `GET /` record.

Add negative tests proving that:

- an arbitrary unknown prefix is not discarded as if it were a base URL;
- more than four ternary alternatives remain partial;
- different HTTP paths remain separate contracts;
- alternatives that differ only by a query string remain separate until existing normalization deliberately merges them.

Run:

```bash
go test ./internal/scan -run '^(TestJavaImperativeContractResolvesConfigurationGetterBaseURL|TestJavaImperativeContractResolvesBoundedTernaryLocal)$' -count=1
```

Expected: `FAIL`; getter resolution drops base-URL provenance and the expression resolver does not branch on ternaries.

- [ ] **Step 4: Retain getter provenance**

Change:

```go
type javaIndexedPathExpression struct {
    expression string
    constants  map[string]string
    baseURLs   map[string]bool
}
```

Populate `baseURLs` from configuration-annotated fields owned by the getter's type. Pass it to `javaResolvedPathExpression` instead of `nil`.

- [ ] **Step 5: Resolve bounded alternatives**

Implement:

```go
func javaResolvedPathAlternatives(
    expression string,
    constants, locals map[string]string,
    baseURLs map[string]bool,
    depth int,
) []string
```

Rules:

1. resolve a local before parsing the expression;
2. split a top-level ternary into true/false branches;
3. resolve each branch recursively;
4. flatten, normalize, sort, and deduplicate;
5. return no alternatives when recursion exceeds eight or the result exceeds four;
6. accept a skipped prefix only when `baseURLs[expression]` is true;
7. never synthesize `/` from an unresolved getter.

Change imperative contract extraction to emit one record per resolved alternative. Normalize each raw alternative with `normalizeAPIPathDetails` and populate `Path`, `Query`, and `QueryParams`. Include `Query` and normalized query parameters in Java contract sorting and deduplication. Preserve caller, auth, file, line, and retry provenance on every record.

- [ ] **Step 6: Run scan tests**

```bash
gofmt -w internal/scan/spring_extract.go internal/scan/java_api_contracts.go internal/scan/java_api_contracts_test.go
go test ./internal/scan -run '^(TestBuildSpringIndexResolvesQualifiedCrossFileConstants|TestJavaImperativeContractResolves)' -count=1
go test ./internal/scan -count=1
```

Expected: all scanner tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/scan/spring_extract.go internal/scan/java_api_contracts.go internal/scan/java_api_contracts_test.go
git commit -m "Resolve composed Java API paths" \
  -m "- Index qualified Spring constants without ambiguous simple-name collisions
- Resolve proven configuration base URLs and bounded ternary client paths"
```

---

## Task 6: Scope Spring Security Evidence to Matching Filter Chains

**Files:**

- Modify: `internal/scan/spring_extract.go`
- Modify: `internal/scan/extract_java_test.go`
- Modify: `internal/scan/java_api_contracts_test.go`
- Test: `internal/scan/extract_java_test.go`
- Test: `internal/scan/java_api_contracts_test.go`

**Interfaces:**

- Replace `springGlobalAuthRecords` with `springAuthScopes`.
- Add internal type `springAuthScope`.
- Add `applyScopedSpringAuth(index *SpringIndex, scopes []springAuthScope)`.
- Reuse `JavaMethodRecord.Calls` and resolved constant expressions; do not add a new public JSON type.

- [ ] **Step 1: Add failing multi-chain tests**

Use three filter chains:

```java
@Bean
@Order(1)
SecurityFilterChain publicApi(HttpSecurity http) {
    http.securityMatcher("/public/**").authorizeHttpRequests(routes -> routes.anyRequest().permitAll());
    return http.build();
}

@Bean
@Order(2)
SecurityFilterChain internalApi(HttpSecurity http) {
    http.securityMatcher(Routes.INTERNAL + "/**")
        .authorizeHttpRequests(routes -> routes.anyRequest().authenticated())
        .httpBasic();
    return http.build();
}

@Bean
SecurityFilterChain fallback(HttpSecurity http) {
    http.authorizeHttpRequests(routes -> routes.anyRequest().hasRole("USER"))
        .oauth2ResourceServer(server -> server.jwt());
    return http.build();
}
```

Create endpoints under `/public`, `/internal`, and `/other`. Require:

- public endpoint: public only;
- internal endpoint: authenticated/basic only;
- other endpoint: role/bearer only;
- no endpoint contains all three chains;
- an unresolved matcher produces partial/unknown evidence instead of globally exact evidence.

Run:

```bash
go test ./internal/scan -run '^TestBuildSpringIndexScopesSecurityFilterChainsByPath$' -count=1
```

Expected: `FAIL`; all filter-chain auth is currently appended to every endpoint.

- [ ] **Step 2: Retain and resolve chain matchers**

Add:

```go
type springAuthScope struct {
    Paths      []string
    Auth       []AuthRecord
    Order      int
    File       string
    Line       int
    Confidence string
}
```

For every production `SecurityFilterChain` method:

1. collect `securityMatcher` calls from `method.Calls`;
2. resolve every argument through the qualified constant index;
3. normalize `/**` to a route-prefix matcher;
4. read `@Order` from method annotations, defaulting to the lowest precedence;
5. attach only that method's `Auth` records;
6. use an empty `Paths` slice for a proven fallback chain;
7. mark the scope partial when any matcher is unresolved.

- [ ] **Step 3: Apply the most specific compatible scope**

For each endpoint:

1. select scopes whose resolved prefix matches the normalized endpoint path;
2. choose the lowest explicit order, then longest matching prefix;
3. use a fallback scope only if no path-specific scope matches;
4. if equally specific selected scopes disagree, retain the conflicting records but lower their confidence to `PARTIAL`;
5. never append auth from an unrelated scope.

Keep method/controller security annotations as endpoint-local evidence and merge them after filter-chain selection.

- [ ] **Step 4: Run scanner and API-catalog security tests**

```bash
gofmt -w internal/scan/spring_extract.go internal/scan/extract_java_test.go internal/scan/java_api_contracts_test.go
go test ./internal/scan -run '^(TestBuildSpringIndexScopesSecurityFilterChainsByPath|TestExtractJava|TestNormalizeSecurityEvidence)' -count=1
go test ./internal/scan -count=1
```

Expected: all tests pass; no endpoint receives unrelated public and authenticated rules.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/spring_extract.go internal/scan/extract_java_test.go internal/scan/java_api_contracts_test.go
git commit -m "Scope Spring security evidence" \
  -m "- Associate filter-chain authentication with resolved security matcher prefixes
- Report unresolved or conflicting matched rules as partial evidence"
```

---

## Task 7: Finalize Coverage, Retry Semantics, and Markdown Honestly

**Files:**

- Modify: `internal/agent/context.go`
- Modify: `internal/agent/context_rank.go`
- Modify: `internal/agent/context_select.go`
- Modify: `internal/agent/context_test.go`
- Modify: `internal/query/context.go`
- Modify: `internal/query/context_test.go`
- Test: `internal/agent/context_change_analysis_test.go`
- Test: `internal/agent/context_test.go`
- Test: `internal/query/context_test.go`

**Interfaces:**

- Add `finalizeContextSourceDecision(pack ContextPack, index scan.AgentContextIndexRecord) ContextPack`.
- Add `contextRequestedContractGap(...) *ContextUncertainty`.
- Add `rankContextRetryCandidates(...) []scan.AgentContextFactRecord`.
- Add Markdown sections `## Coverage` and `## Endpoints`.

- [ ] **Step 1: Add failing post-render decision tests**

Cover these cases:

| Case | Fallback | Retry | Coverage |
|---|---:|---:|---|
| Entrypoint source cannot be verified | true | false | partial/none |
| Required support concern has a renderable unselected candidate | false | true | partial |
| Requested future contract is absent but adjacent patterns are complete | false | false | complete |
| All required concerns have verified source | false | false | complete |
| Only unrelated create facts remain for a delete task | false | false | complete |

For a retryable omission, require the first anchor to be the omitted fact's exact file or qualified symbol. Do not accept an arbitrary alphabetically first method.

Run:

```bash
go test ./internal/agent -run '^(TestFinalizeContextSourceDecision|TestContextRetryPermissionRanksActualOmissions|TestContextRetryPermissionRejectsOppositeAction)$' -count=1
```

Expected: `FAIL`; fallback is decided before source attachment and retry scans pre-render candidate lists.

- [ ] **Step 2: Finalize only after source selection**

After `attachContextSource`, call:

```go
pack = finalizeContextSourceDecision(pack, loaded.Index)
pack.RetryAllowed, pack.RetryAnchors = contextRetryPermission(pack, loaded.Index)
```

Decision rules:

1. missing verified entrypoint or primary-path source sets `fallback_required: true`;
2. another required concern with a renderable alternative stays partial and permits one retry;
3. a requested contract with no exact action/path candidate becomes an explicit covered absence uncertainty when adjacent client/provider evidence is verified;
4. an absence uncertainty never adds a fact ID or edge;
5. retry candidates must cover an uncovered concern, match its project, remain unselected, have current source or an explicit source omission, and share the task action tokens;
6. rank omission path, exact qualified name, semantic score, confidence, then stable ID;
7. return at most three anchors.

Use this stable uncertainty:

```go
ContextUncertainty{
    Scope:  "requested_http_contract",
    Reason: "no indexed HTTP contract matches the requested operation; included contracts are adjacent current implementation evidence",
}
```

- [ ] **Step 3: Add failing Markdown tests**

Require:

```text
## Coverage
- entrypoint — covered
- configuration [libraries/job-client] — covered

## Endpoints
- DELETE /catalog/items/{itemId} — CatalogController.deleteItem
  Security: public
  Confidence: EXACT
```

For conflicting or partial security, require the rendered limitation/confidence. Verify control characters and backticks still pass through the existing escaping helpers.

Run:

```bash
go test ./internal/query -run '^TestRenderContextMarkdownIncludesCoverageAndEndpoints$' -count=1
```

Expected: `FAIL`; Markdown currently omits `pack.Concerns` and `pack.Endpoints`.

- [ ] **Step 4: Render coverage and endpoints compactly**

Add `appendContextConcernSection` and `appendContextEndpointSection`. Keep one line per concern and at most three detail lines per endpoint. Render consumers only as a count; detailed consumers remain available in JSON.

Order sections:

```text
Coverage
Entrypoints
Endpoints
Call chain
Contracts
Persistence
Tests
Files to inspect
Source sections
Source omissions
Uncertainties
Fallback
```

- [ ] **Step 5: Make the missing-contract integration pass**

Run:

```bash
gofmt -w internal/agent/context.go internal/agent/context_rank.go internal/agent/context_select.go internal/agent/context_test.go internal/agent/context_change_analysis_test.go internal/query/context.go internal/query/context_test.go
go test ./internal/agent -run '^TestBuildContextSupportsMissingContractChangeAnalysis$' -count=1
go test ./internal/agent -run '^TestMissingContractChangeAnalysisEnglishGermanParity$' -count=1
go test ./internal/query -run '^TestRenderContextMarkdownIncludesCoverageAndEndpoints$' -count=1
go test ./internal/agent ./internal/query -count=1
```

Expected: all pass. The integration pack is complete, has no retry, and contains no fabricated future relationship.

- [ ] **Step 6: Commit the final integration contract and decision logic**

```bash
git add internal/agent/context.go internal/agent/context_rank.go internal/agent/context_select.go internal/agent/context_test.go internal/agent/context_change_analysis_test.go internal/query/context.go internal/query/context_test.go
git commit -m "Finalize source-backed context decisions" \
  -m "- Decide coverage, fallback, missing-contract uncertainty, and retries after verified source selection
- Expose concern coverage and endpoint security in default Markdown output"
```

---

## Task 8: Measure Included-Source Re-reads and Run Release Gates

**Files:**

- Modify: `scripts/analyze-agent-context-log.go`
- Modify: `scripts/analyze-agent-context-log_test.sh`
- Modify: `scripts/benchmark-agent-context.sh`
- Modify: `scripts/benchmark-agent-context_test.sh`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/RELEASE.md`
- Test: `scripts/analyze-agent-context-log_test.sh`
- Test: `scripts/benchmark-agent-context_test.sh`

**Interfaces:**

- Add analyzer metric `included_source_rereads`.
- Extend the TSV schema after `source_read_calls`.
- Fail the official assisted run when the sum of `included_source_rereads` is nonzero for complete packs.

- [ ] **Step 1: Add failing analyzer fixtures**

Add JSONL fixtures for:

1. a complete JSON Context Pack followed by `sed` on an included file;
2. a complete Markdown Context Pack followed by `rg` on an absolute path ending in the included relative path;
3. a partial pack followed by a read of an omitted file;
4. a read before the Context Pack;
5. duplicate terminal lifecycle events for the same read.

Require:

```text
tool_calls goregraph_calls full_context_packs compact_duplicate_packs repeated_full_packs raw_navigation_calls source_read_calls included_source_rereads unique_source_files
```

Only cases 1 and 2 increment `included_source_rereads`. Count once per terminal tool item, not once per repeated path argument.

Run:

```bash
./scripts/analyze-agent-context-log_test.sh
```

Expected: `FAIL`; the metric and complete-pack path state do not exist.

- [ ] **Step 2: Track complete-pack source paths in event order**

Extend:

```go
type metrics struct {
    toolCalls, goregraphCalls, fullPacks, compactPacks, repeatedPacks int
    navigationCalls, sourceReadCalls, includedSourceRereads         int
    sourcePaths, completePackSourcePaths                            map[string]struct{}
}
```

When `recordContextPack` sees `source_coverage: complete`, collect `project/path` for every JSON source section with a project and the plain path otherwise. For Markdown, collect the existing combined project/path code references under `## Source sections` and strip trailing line numbers.

Normalize paths with `filepath.ToSlash`, remove leading `./`, and compare:

```go
func sameSourcePath(readPath, includedPath string) bool {
    readPath = normalizeRecordedSourcePath(readPath)
    includedPath = normalizeRecordedSourcePath(includedPath)
    return readPath == includedPath ||
        strings.HasSuffix(readPath, "/"+includedPath)
}
```

Refactor command classification to return one `includedReread` boolean per tool item. Increment the metric once after all command segments are classified.

- [ ] **Step 3: Extend the benchmark gate**

Add the new column to per-run and median rows. Collect the assisted total and fail with:

```bash
[ "$assisted_included_source_rereads" -eq 0 ] ||
  die "assisted runs re-read source files already included by complete Context Packs"
```

Keep the existing 80% token, recorded-baseline, 70% tool-call, 50% source-read, and repeated-pack gates unchanged.

- [ ] **Step 4: Update documentation with the latest diagnostic**

Replace the previous single-pair diagnostic in both documents with:

```text
The latest diagnostic pair recorded 169,913 baseline tokens and 166,833
assisted tokens, a 3,080-token reduction (1.81%). The assisted run made
31 shell executions versus 28 baseline executions, a 10.71% increase.
This is diagnostic evidence only, not controlled three-by-three release proof.
```

Document `included_source_rereads`, its event-order semantics, and the zero-tolerance gate. Do not commit the external prompts, transcripts, workspace path, or score sheet.

- [ ] **Step 5: Run script tests**

```bash
gofmt -w scripts/analyze-agent-context-log.go
./scripts/analyze-agent-context-log_test.sh
./scripts/benchmark-agent-context_test.sh
```

Expected: both scripts print their success messages and exit zero.

- [ ] **Step 6: Commit**

```bash
git add scripts/analyze-agent-context-log.go scripts/analyze-agent-context-log_test.sh scripts/benchmark-agent-context.sh scripts/benchmark-agent-context_test.sh docs/BENCHMARKING.md docs/RELEASE.md
git commit -m "Gate context packs against source rereads" \
  -m "- Measure reads of files already supplied by complete Context Packs
- Refresh diagnostic evidence and enforce zero assisted rereads"
```

- [ ] **Step 7: Run full repository verification**

```bash
go test ./...
go vet ./...
temporary_gobin=$(mktemp -d)
GOBIN="$temporary_gobin" go install ./cmd/goregraph
"$temporary_gobin/goregraph" --help
```

Expected: every command exits zero and help includes the workspace guidance from the earlier dedicated CLI commit.

- [ ] **Step 8: Rebuild the external diagnostic workspace**

Keep all external evidence outside the repository:

```bash
benchmark_workspace=${GOREGRAPH_BENCHMARK_WORKSPACE:?set GOREGRAPH_BENCHMARK_WORKSPACE to the immutable prepared workspace}
benchmark_prompt=${GOREGRAPH_BENCHMARK_PROMPT:?set GOREGRAPH_BENCHMARK_PROMPT to the neutral task file}
benchmark_query=$(cat "$benchmark_prompt")
"$temporary_gobin/goregraph" workspace status "$benchmark_workspace" --workspace "$benchmark_workspace"
"$temporary_gobin/goregraph" workspace scan-all "$benchmark_workspace" --workspace "$benchmark_workspace"
"$temporary_gobin/goregraph" context "$benchmark_workspace" --query "$benchmark_query" --format json
```

Expected:

- one reliable production entrypoint;
- current primary chain only;
- verified source from all three relevant external projects;
- explicit `requested_http_contract` uncertainty when the desired route is absent;
- no contradictory exact security;
- no `GET /` artifact from a composed client URL;
- `fallback_required: false`;
- `retry_allowed: false`;
- `source_coverage: complete`;
- no source omission for a verifiable inherited repository owner;
- at most 4,000 estimated tokens and 12 source sections.

- [ ] **Step 9: Run the controlled three-by-three gate**

```bash
benchmark_prompt=${GOREGRAPH_BENCHMARK_PROMPT:?set GOREGRAPH_BENCHMARK_PROMPT}
baseline_instruction=${GOREGRAPH_BASELINE_INSTRUCTION:?set GOREGRAPH_BASELINE_INSTRUCTION}
assisted_instruction=${GOREGRAPH_ASSISTED_INSTRUCTION:?set GOREGRAPH_ASSISTED_INSTRUCTION}
benchmark_output=${GOREGRAPH_BENCHMARK_OUTPUT:?set GOREGRAPH_BENCHMARK_OUTPUT to a new absolute directory}

./scripts/benchmark-agent-context.sh \
  --workspace "$benchmark_workspace" \
  --prompt "$benchmark_prompt" \
  --baseline-instruction "$baseline_instruction" \
  --assisted-instruction "$assisted_instruction" \
  --runs 3 \
  --output "$benchmark_output"
```

Expected:

- assisted quality score is at least the matched baseline score on the twelve-point rubric;
- assisted median tokens are at most 80% of matched baseline and at most the recorded-baseline ceiling;
- assisted median tool calls are at most 70% of baseline;
- assisted median source reads are at most 50% of baseline;
- `repeated_full_packs = 0`;
- `included_source_rereads = 0`.

Do not claim the improvement complete from a single diagnostic pair. Retain the raw JSONL transcripts and completed rubric outside the repository.

---

## Final Self-Review Checklist

- [ ] Search production and committed fixtures for external repository names, absolute paths, route fragments, and task-specific business nouns.
- [ ] Verify the synthetic fixture contains no desired cleanup contract or edge.
- [ ] Verify no selected related fact is serialized as part of the primary call chain without a real index path.
- [ ] Reverse fact, edge, source, and filter-chain input order in determinism tests.
- [ ] Verify concern keys are identical for the English and German synthetic tasks.
- [ ] Verify exact route security never combines unrelated filter chains.
- [ ] Verify unresolved constants, dynamic URLs, and security matchers remain partial.
- [ ] Verify inherited-owner source states prove only the owner declaration, not a nonexistent method body.
- [ ] Verify `source_coverage: complete` is computed after rendering.
- [ ] Verify retry anchors correspond to actual uncovered concerns and the requested action.
- [ ] Verify Markdown and JSON expose equivalent coverage, uncertainty, endpoint, fallback, and retry decisions.
- [ ] Verify all source/file/token/byte limits remain enforced.
- [ ] Verify the CLI help changes remain isolated in their earlier dedicated commit.
- [ ] Verify `git diff --check`, `go test ./...`, `go vet ./...`, and both script test suites pass.
- [ ] Review every commit against the repository's English imperative commit format and ensure unrelated changes are absent.
