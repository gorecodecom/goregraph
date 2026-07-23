# Agent Context Source Quality Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve the fast, single-pack GoreGraph workflow while making the twelve selected source sections answer the requested domain questions instead of spending slots on short generic signatures.

**Architecture:** Add an explicit, language-neutral `domain_model` intent for requests about types, entities, identifiers, and attributes. Carry that intent into support-fact ranking and source-option profiling. Keep core route, current call-chain, selected-contract, token, file, and section boundaries mandatory; within the remaining budget, select by required concern coverage, domain evidence diversity, stable identity relevance, and information content. Do not blacklist benchmark filenames or synthesize a missing future contract.

**Tech Stack:** Go 1.x, existing `internal/agent` context compiler and source renderer, table-driven Go tests, shell benchmark harness, Markdown documentation.

## Global Constraints

- Keep `MaxContextSourceSections = 12`, `DefaultContextBudgetTokens = 4000`, and `DefaultContextMaxFiles = 12`.
- Keep one initial Context Pack, at most one explicitly permitted retry, and the existing no-broad-navigation agent guidance.
- Never infer the requested future DELETE contract from a neighboring GET or single-item DELETE route.
- Base relevance on normalized fact identity (`Name`, `Qualified`, source basename, HTTP method/path), graph evidence, and rendered information content. `Search` may recall candidates but must not establish identity relevance by itself.
- Do not add Weka-specific class names, paths, repository names, or route fragments to production selection code.
- Do not alter scanner output or the public JSON shape except for the new documented concern/role value `domain_model`.
- Keep English/German intent parity and deterministic output under reversed fact and edge order.
- Keep the implementation and its tests in focused English commits. Do not tag or release.

---

## Task 1: Lock the Observed Quality Regression Into a Production-Shaped Fixture

**Files:**

- Modify: `internal/agent/context_change_analysis_test.go`
- Test: `internal/agent/context_change_analysis_test.go`

**Interfaces:**

- Extend `writeMissingContractContextFixture`.
- Add `contextSourcePathSet(pack ContextPack) map[string]bool`.
- Add `contextSourceContainsStableIdentity(pack ContextPack, value string) bool`.

- [ ] **Step 1: Extend the synthetic task with the missing evidence classes**

Add two task models, their distinct repositories, comment dependencies, and generic distractors to the existing fixture. Keep names generic so the regression proves reusable behavior:

```go
{ID: "regular-job-model", Project: "services/jobs", Kind: "symbol",
    Name: "CatalogJobEntity", Qualified: "example.CatalogJobEntity",
    File: "src/main/java/example/CatalogJobEntity.java", Line: 8, EndLine: 24,
    Confidence: "EXACT", Search: "regular job task model catalogId itemId"},
{ID: "change-job-model", Project: "services/jobs", Kind: "symbol",
    Name: "CatalogChangeJobEntity", Qualified: "example.CatalogChangeJobEntity",
    File: "src/main/java/example/CatalogChangeJobEntity.java", Line: 8, EndLine: 26,
    Confidence: "EXACT", Search: "change job task model catalogId itemId changeId"},
{ID: "regular-job-repository", Project: "services/jobs", Kind: "persistence",
    Name: "findByCatalogIdAndItemId", Qualified: "CatalogJobRepository.findByCatalogIdAndItemId",
    File: "src/main/java/example/CatalogJobRepository.java", Line: 7, EndLine: 9,
    Confidence: "EXACT", Search: "regular job task catalog item persistence"},
{ID: "change-job-repository", Project: "services/jobs", Kind: "persistence",
    Name: "findByCatalogIdAndItemId", Qualified: "CatalogChangeJobRepository.findByCatalogIdAndItemId",
    File: "src/main/java/example/CatalogChangeJobRepository.java", Line: 7, EndLine: 9,
    Confidence: "EXACT", Search: "change job task catalog item persistence"},
{ID: "regular-comment-repository", Project: "services/jobs", Kind: "persistence",
    Name: "findByJobIdOrderByCreated", Qualified: "CatalogJobCommentRepository.findByJobIdOrderByCreated",
    File: "src/main/java/example/CatalogJobCommentRepository.java", Line: 7, EndLine: 8,
    Confidence: "EXACT", Search: "regular job comment dependency persistence"},
{ID: "generic-mail-properties", Project: "libraries/job-client", Kind: "configuration",
    Name: "MailProperties", Qualified: "example.MailProperties",
    File: "src/main/java/example/MailProperties.java", Line: 8, EndLine: 8,
    Confidence: "EXACT", Search: "mail configuration"},
{ID: "generic-async-handler", Project: "libraries/job-client", Kind: "resilience",
    Name: "AsyncExceptionHandler", Qualified: "example.AsyncExceptionHandler",
    File: "src/main/java/example/AsyncExceptionHandler.java", Line: 8, EndLine: 8,
    Confidence: "EXACT", Search: "exception handling resilience"},
{ID: "unrelated-topic-repository", Project: "services/catalog", Kind: "persistence",
    Name: "findTopic", Qualified: "CatalogTopicRepository.findTopic",
    File: "src/main/java/example/CatalogTopicRepository.java", Line: 7, EndLine: 8,
    Confidence: "EXACT", Search: "catalog topic persistence"},
```

Give both entity fixtures concrete declaration bodies containing their identifiers:

```java
class CatalogJobEntity {
  long catalogId;
  long itemId;
}
```

```java
class CatalogChangeJobEntity extends CatalogJobEntity {
  long changeId;
}
```

After the fixture's generic source-generation loop, overwrite only these two model files with `writeContextSourceFile` and the literal declarations above. Remove the existing accidental duplicate `jobs-service` fact while touching the fixture; retain one fact with that ID.

Keep the future bulk-delete route absent. The fixture must continue to express uncertainty about the missing HTTP contract rather than accidentally proving the proposed design.

- [ ] **Step 2: Strengthen the query and source-level assertions**

Extend both existing English and German queries with task types and lookup attributes:

```go
const missingContractEnglishQuery = "When DELETE /catalog/items/{itemId} removes an item in services/catalog, plan cleanup of related jobs through libraries/job-client and services/jobs. Cover the current path, missing HTTP contract, task types and lookup attributes, authentication, configuration, retry behavior, persistence, side effects, and tests."
const missingContractGermanQuery = "Wenn DELETE /catalog/items/{itemId} einen Eintrag in services/catalog löscht, plane das Entfernen verbundener Aufgaben über libraries/job-client und services/jobs. Berücksichtige aktuellen Pfad, fehlenden HTTP-Vertrag, Aufgabenarten und Suchattribute, Authentifizierung, Konfiguration, Wiederholung, Persistenz, Nebenwirkungen und Tests."
```

Require the selected source paths to contain the two distinct model declarations and the two declared repositories:

```go
paths := contextSourcePathSet(pack)
for _, path := range []string{
    "src/main/java/example/CatalogJobEntity.java",
    "src/main/java/example/CatalogChangeJobEntity.java",
    "src/main/java/example/CatalogJobRepository.java",
    "src/main/java/example/CatalogChangeJobRepository.java",
} {
    if !paths[path] {
        t.Errorf("required domain evidence %q missing from %#v", path, pack.SourceSections)
    }
}
for _, identity := range []string{"catalogId", "itemId", "changeId"} {
    if !contextSourceContainsStableIdentity(pack, identity) {
        t.Errorf("lookup identity %q missing from source sections", identity)
    }
}
for _, path := range []string{
    "src/main/java/example/MailProperties.java",
    "src/main/java/example/AsyncExceptionHandler.java",
    "src/main/java/example/CatalogTopicRepository.java",
} {
    if paths[path] {
        t.Errorf("generic distractor %q displaced domain evidence", path)
    }
}
```

Retain the existing assertions for one entrypoint, current-chain integrity, missing-contract uncertainty, `fallback_required: false`, `retry_allowed: false`, and all token/file/section bounds.

- [ ] **Step 3: Add an explicit source-quality parity snapshot**

Extend `missingContractContextSnapshot` with sorted source paths and render modes:

```go
SourcePaths []string
SourceModes []string
```

The English and German packs must select the same domain models, repositories, and render modes. This prevents the German compound words from passing metadata intent tests while diverging during source selection.

- [ ] **Step 4: Run the focused test and confirm the intended red state**

```bash
go test ./internal/agent \
  -run '^(TestBuildContextSupportsMissingContractChangeAnalysis|TestMissingContractChangeAnalysisEnglishGermanParity)$' \
  -count=1
```

Expected before implementation: `FAIL` because the selector has no domain-model concern/role and short generic concern signatures can occupy the remaining section slots.

Do not commit the deliberately failing state.

---

## Task 2: Represent Requested Types and Attributes as Domain-Model Intent

**Files:**

- Modify: `internal/agent/context_intent.go`
- Modify: `internal/agent/context_rank.go`
- Modify: `internal/agent/context_test.go`
- Test: `internal/agent/context_test.go`

**Interfaces:**

- Add `contextConcernDomainModel = "domain_model"`.
- Raise `maximumPublicContextConcerns` from `12` to `14`.
- Add `contextDomainModelFact(fact scan.AgentContextFactRecord, domainTokens map[string]bool) bool`.
- Add `contextStableFactIdentityMatchCount(fact scan.AgentContextFactRecord, tokens map[string]bool) int`.
- Add `contextDomainModelQueryTokens(query string, aliases map[string][]string, explicitProjects map[string]bool) map[string]bool`.
- Add `contextDomainModelConcernCandidates(query string, aliases map[string][]string, explicitProjects map[string]bool, facts []scan.AgentContextFactRecord) []string`.
- Change `contextSupportOperationalScore(utility *contextForwardUtility, fact scan.AgentContextFactRecord, query string)` to accept `domainTokens map[string]bool`.
- Change `contextSupportRole(index scan.AgentContextIndexRecord, fact scan.AgentContextFactRecord)` to accept `domainTokens map[string]bool`.
- Add `domain_model` to `normalizedContextConcernKind`, concern vocabulary, support-role ordering, and source concern roles.

- [ ] **Step 1: Add failing multilingual intent tests**

Extend `TestContextExpandedTokenSet` with:

```go
"Aufgabenarten":   {"task_type", "task_types", "type", "types"},
"Suchattribute":   {"attribute", "attributes", "field", "fields", "identifier", "identifiers"},
"task types":      {"task_type", "task_types", "type", "types"},
"lookup attributes": {"attribute", "attributes", "field", "fields", "identifier", "identifiers"},
```

Add the corresponding aliases in `contextIntentTokenAliases`:

```go
"aufgabenart":    {"task_type", "task_types", "type", "types"},
"aufgabenarten":  {"task_type", "task_types", "type", "types"},
"suchattribut":   {"attribute", "attributes", "field", "fields", "identifier", "identifiers"},
"suchattribute":  {"attribute", "attributes", "field", "fields", "identifier", "identifiers"},
"attribute":      {"attributes", "field", "fields", "identifier", "identifiers"},
"attributes":     {"attribute", "field", "fields", "identifier", "identifiers"},
"type":           {"task_type", "task_types", "types"},
"types":          {"task_type", "task_types", "type"},
```

Add `TestPlanContextConcernsSelectsRequestedDomainModels`. Use model symbols in an explicitly named support project plus `MailProperties` and `AsyncExceptionHandler` distractors. Assert that the global `domain_model` concern contains both model IDs and neither generic ID.

- [ ] **Step 2: Implement stable domain-model classification**

Use a compact, language-neutral type-name suffix check plus task-domain identity overlap:

```go
var contextDomainModelSuffixes = []string{
    "dto", "entity", "model", "payload", "projection", "record", "request", "response",
}

func contextDomainModelFact(
    fact scan.AgentContextFactRecord,
    domainTokens map[string]bool,
) bool {
    if contextFactUsesTestSource(fact) || contextPackSourceFile(fact.File) == "" {
        return false
    }
    identity := compactContextIdentifier(firstNonEmptyContext(fact.Qualified, fact.Name))
    modelShape := false
    for _, suffix := range contextDomainModelSuffixes {
        if strings.HasSuffix(identity, suffix) {
            modelShape = true
            break
        }
    }
    if !modelShape {
        return false
    }
    return contextStableFactIdentityMatchCount(fact, domainTokens) > 0
}
```

`contextStableFactIdentityMatchCount` must inspect only `Name`, `Qualified`, `filepath.Base(File)`, `HTTPMethod`, and `Path`; do not use `Search`. Normalize CamelCase and separators through the existing token helpers, and use compact-identifier containment only for domain tokens of at least four runes.

Rank domain-model candidates by:

1. stable domain-token match count;
2. concrete type shape (`entity`, `model`, `record`, `projection`) before transport shape (`dto`, `payload`, `request`, `response`);
3. concrete declarations before names prefixed with `Base` or `Abstract`;
4. confidence;
5. normalized project, file, line, and fact ID.

Keep at most four domain-model candidates, matching the existing per-concern source candidate bound. This makes the two concrete task types eligible before generic base classes and transport DTOs without encoding any business-specific names.

Build domain tokens from the query after removing:

- project aliases;
- concern vocabulary;
- generic action words such as `analyze`, `cover`, `delete`, `remove`, `plan`;
- generic type words such as `entity`, `model`, `type`, `attribute`, and `identifier`.

If no domain token remains, do not create the concern. This keeps a generic “show model types” query from pulling arbitrary DTOs across the workspace.

- [ ] **Step 3: Add the global required concern without multiplying it per project**

In `planContextConcerns`, add at most one `domain_model` concern:

```go
if contextQueryRequestsConcern(query, contextConcernDomainModel) {
    candidates := contextDomainModelConcernCandidates(
        query,
        aliases,
        explicitProjects,
        index.Facts,
    )
    if len(candidates) > 0 {
        concerns = append(concerns, newContextConcern(
            contextConcernDomainModel,
            "",
            true,
            candidates,
            "requested domain types and lookup attributes",
        ))
    }
}
```

Do not create one domain-model concern per repository. The request asks for the domain shapes as a whole, while the existing `project` concerns continue to enforce repository representation.

Add vocabulary:

```go
contextConcernDomainModel: {
    "attribute", "attributes", "entity", "entities", "field", "fields",
    "identifier", "identifiers", "model", "models", "task_type", "task_types",
    "type", "types",
},
```

Raise the public cap to fourteen and make `publicContextConcerns` select non-core concerns in two passes:

1. the first deterministic concern for each requested kind in this order: `domain_model`, `http_contract`, `authentication`, `configuration`, `resilience`, `persistence`, `side_effects`, `tests`;
2. remaining project-scoped duplicates in existing `contextConcernLess` order until the cap.

Core and explicit-project concerns remain first. Add a reversed-input test proving the public list is deterministic and contains every requested concern kind before a second project-scoped copy of any kind. Add a metadata-budget regression proving the complete representative set still fits and the final pack remains at most 4,000 tokens.

- [ ] **Step 4: Carry the concern into support ranking**

In `rankContextSupportFacts`, compute domain tokens once:

```go
domainTokens := contextDomainModelQueryTokens(query, aliases, explicitProjects)
```

Pass them to `contextSupportOperationalScore` and `contextSupportRole`. Recognize a stable domain-model fact only when the query requests the concern:

```go
if contextQueryRequestsConcern(query, contextConcernDomainModel) &&
    contextDomainModelFact(fact, domainTokens) {
    score += 180
    operational = true
}
```

Update direct unit-test call sites with an explicit domain-token map. Return `domain_model` from `contextSupportRole`, and place it after `service` and before cross-cutting configuration:

```go
var contextSupportRoleOrder = []string{
    "client",
    "contract",
    "provider",
    "service",
    contextConcernDomainModel,
    contextConcernConfiguration,
    contextConcernAuth,
    contextConcernResilience,
    contextConcernPersistence,
    contextConcernSideEffects,
    contextConcernTests,
}
```

Do not increase `maximumContextSupportCandidatesPerProject`; the existing cap of eight is sufficient once the requested model role competes explicitly.

- [ ] **Step 5: Verify intent and support ranking**

```bash
go test ./internal/agent \
  -run '^(TestContextExpandedTokenSet|TestPlanContextConcernsMatchesGermanOperationalTask|TestPlanContextConcernsSelectsRequestedDomainModels|TestRankContextSupportFactsPrefersRequestedDomainModels)$' \
  -count=1
```

Expected: `PASS`, with English/German concern keys identical and both model facts ranked before generic cross-cutting types.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/context_intent.go internal/agent/context_rank.go internal/agent/context_test.go
git commit -m "Represent requested domain model evidence" \
  -m "- Recognize task types and lookup attributes in English and German
- Rank stable model identities without using search text as proof
- Preserve long-request concern visibility within the context budget"
```

---

## Task 3: Profile Source Options by Information Value and Evidence Family

**Files:**

- Modify: `internal/agent/context_select.go`
- Modify: `internal/agent/context_source.go`
- Modify: `internal/agent/context_source_test.go`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Add `quality int` and `evidenceFamily string` to `contextSourceOption`.
- Add `selectedEvidenceFamilies map[string]int` to `contextSourceSelectionState`.
- Add `contextSourceOptionQuality(pack ContextPack, index scan.AgentContextIndexRecord, option contextSourceOption) int`.
- Add `contextSourceEvidenceFamily(pack ContextPack, index scan.AgentContextIndexRecord, option contextSourceOption) string`.
- Add `contextSourceEvidenceFamilyLimit(family string) int`.

- [ ] **Step 1: Add failing unit tests for the exact bad tradeoff**

Add `TestContextSourceOptionQualityPrefersDomainEvidenceOverGenericSignatures` with four options:

1. a `domain_model` declaration body containing `catalogId` and `itemId`;
2. a declared `persistence` finder containing both identifiers;
3. a signature-only configuration enum with no domain identity;
4. a signature-only exception handler with no domain identity.

Assert:

```go
if model.quality <= mailProperties.quality {
    t.Fatalf("model quality %d <= generic configuration %d", model.quality, mailProperties.quality)
}
if finder.quality <= asyncHandler.quality {
    t.Fatalf("finder quality %d <= generic resilience %d", finder.quality, asyncHandler.quality)
}
```

Add `TestSmallestFittingContextSourceOptionUsesQualityForProjectBoundary`:

- an exact-fact boundary must still select the smallest fitting rendering of that fact;
- a project-only boundary must select the highest-quality fact, then the smallest rendering of that same fact;
- the final deterministic tie-break remains `contextSourceOptionLess`.

Add `TestContextSourceUtilityAllowsTwoDistinctDomainModels` and `TestContextSourceUtilityAllowsTwoDistinctRepositories`. The second candidate in each family must retain a diversity bonus; the third must not.

- [ ] **Step 2: Compute quality from stable, bounded signals**

Populate the new fields once in `appendContextSourceCandidateOptions`:

```go
option := contextSourceOption{
    candidate: optionCandidate,
    section: section,
    estimated: estimated,
    concernKeys: concernKeys,
    projectKey: projectKey,
    required: required,
    pathDistance: contextSourceCandidateDistance(optionCandidate, distances),
}
option.evidenceFamily = contextSourceEvidenceFamily(pack, index, option)
option.quality = contextSourceOptionQuality(pack, index, option)
```

Use these fixed scoring rules:

```text
+260 domain_model evidence with stable domain identity
+220 non-generic declared persistence
+200 action-aligned route, handler, or contract
+180 declaration_body or body rendering
+100 focused rendering
  +60 per stable domain-token match, capped at three matches
  +40 EXACT confidence; +20 RESOLVED/EXTRACTED confidence
-120 path distance beyond maximumContextPathHops
-320 signature-only cross-cutting evidence with zero stable domain-token matches
-180 generic inherited persistence
```

Clamp the result to `[-500, 1000]`. Search text and summaries must not contribute to the stable-domain match count.

Use the following evidence families:

```text
domain_model
persistence
action
contract
authentication
configuration
resilience
side_effects
tests
other
```

Key family counts by normalized project plus family. Give `domain_model` and `persistence` a limit of two; every other family has a limit of one. This supports two distinct task models and two distinct task repositories without allowing repeated generic classes to consume the pack.

- [ ] **Step 3: Expose the model role in source sections**

Return `domain_model` from `contextSourceRole` when the candidate is part of the planned domain-model concern. Add it to `contextSourceRolePriority` between `call_chain` and `contract`:

```go
var contextSourceRolePriority = map[string]int{
    "entrypoint":   0,
    "call_chain":   1,
    "domain_model": 2,
    "contract":     3,
    "persistence":  4,
    "test":         5,
}
```

Update only tests that compare the documented role ordering. Do not change core-boundary membership or source renderers.

- [ ] **Step 4: Verify source profiling**

```bash
go test ./internal/agent \
  -run '^(TestContextSourceOptionQuality|TestSmallestFittingContextSourceOptionUsesQualityForProjectBoundary|TestContextSourceUtilityAllowsTwoDistinct)' \
  -count=1
```

Expected: all new quality, boundary, and diversity tests pass.

---

## Task 4: Apply Quality-Aware Selection Without Weakening Hard Boundaries

**Files:**

- Modify: `internal/agent/context_select.go`
- Modify: `internal/agent/context_change_analysis_test.go`
- Modify: `internal/agent/context_source_test.go`
- Test: `internal/agent/context_change_analysis_test.go`
- Test: `internal/agent/context_source_test.go`

- [ ] **Step 1: Make project-only mandatory selection quality-aware**

Keep exact-fact boundaries cost-first. For project-only boundaries, compare:

1. higher `quality`;
2. lower estimated tokens;
3. `contextSourceOptionLess`.

This prevents a one-line generic type in an explicitly requested repository from satisfying the project boundary ahead of a concrete service, model, or repository declaration.

Implement the comparison in a named helper:

```go
func betterContextProjectBoundaryOption(left, right contextSourceOption) bool {
    if left.quality != right.quality {
        return left.quality > right.quality
    }
    if left.estimated != right.estimated {
        return left.estimated < right.estimated
    }
    return contextSourceOptionLess(left, right)
}
```

- [ ] **Step 2: Add quality and bounded diversity to greedy utility**

Initialize and update `selectedEvidenceFamilies` together with the existing selected candidate, fact, project, concern, and role maps.

Use:

```go
familyBonus := 0
familyKey := option.projectKey + "\x00" + option.evidenceFamily
if state.selectedEvidenceFamilies[familyKey] <
    contextSourceEvidenceFamilyLimit(option.evidenceFamily) {
    familyBonus = 260
}

utility := 1200*newConcerns +
    300*newProjects +
    150*newRoles +
    80*connected +
    familyBonus +
    option.quality -
    option.estimated -
    25*option.pathDistance
```

The existing production-before-tests gate remains authoritative. A test source must not displace coverable production evidence.

Increment the family count only in `addContextSourceOption`; speculative fit checks must remain side-effect free.

- [ ] **Step 3: Protect core enrichment and tight-budget behavior**

Add or retain regressions proving:

- route and first service hop remain mandatory;
- selected contracts and selected related-project files remain boundaries;
- exact-fact boundaries may start as signatures when only the signature fits;
- `enrichContextCoreSourceOptions` upgrades all core boundaries fairly before optional source;
- no pack exceeds 4,000 tokens, 12 files, or 12 source sections;
- source omissions and `SourceUnrepresented` still count missing required concerns, not optional discarded candidates.

- [ ] **Step 4: Run the production-shaped regression**

```bash
go test ./internal/agent \
  -run '^(TestBuildContextSupportsMissingContractChangeAnalysis|TestMissingContractChangeAnalysisEnglishGermanParity|TestContextSourceOptions|TestContextCoreSourceBoundaries|TestEnrichContextCoreSourceOptions)' \
  -count=1
```

Expected:

- both task model declarations are present;
- both declared repositories are present;
- the core DELETE route and operations body remain present;
- the existing client method and current provider behavior remain present;
- generic mail/configuration, exception-handler, and unrelated-topic signatures do not displace domain evidence;
- the missing future HTTP contract remains an uncertainty;
- fallback and retry decisions remain false;
- English and German snapshots are identical;
- the pack stays within all hard bounds.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/context_select.go internal/agent/context_source.go internal/agent/context_source_test.go internal/agent/context_change_analysis_test.go
git commit -m "Prefer informative context source evidence" \
  -m "- Score source options by stable domain identity and rendered information
- Preserve two model and persistence families within the fixed section budget
- Keep core boundaries, fallback, retry, and determinism invariants covered"
```

---

## Task 5: Document and Gate the New Selection Contract

**Files:**

- Modify: `docs/OUTPUTS.md`
- Modify: `docs/BENCHMARKING.md`
- Modify: `README.md`
- Verify: `scripts/analyze-agent-context-log_test.sh`
- Verify: `scripts/benchmark-agent-context_test.sh`
- Test: `scripts/analyze-agent-context-log_test.sh`
- Test: `scripts/benchmark-agent-context_test.sh`

- [ ] **Step 1: Document behavior, not benchmark-specific names**

Document:

- `domain_model` means current source for explicitly requested types, entities, payloads, identifiers, or lookup attributes;
- source selection prefers declaration bodies with stable domain identity over unrelated one-line cross-cutting signatures;
- up to two distinct domain-model and persistence families may be selected per project;
- `source_coverage: complete` still means every required concern has current source, not that every candidate was serialized;
- the hard 4,000-token, 12-file, and 12-source-section limits are unchanged;
- complete coverage still forbids reconstructive repository reads outside supplied source sections.

Do not mention `MailProperties`, `AsyncExceptionHandler`, `Cadaster*`, Weka repositories, or the historical benchmark route in general product documentation.

- [ ] **Step 2: Keep benchmark automation environment-neutral**

Do not add historical filenames or content scoring to the transcript analyzer. Keep its schema and efficiency gates unchanged:

```text
assisted median tokens <= 80% of matched baseline
assisted median tool calls <= 70% of baseline
assisted median source reads <= 50% of baseline
repeated_full_packs = 0
included_source_rereads = 0
```

Content quality is enforced by the deterministic Go regression rather than by teaching the generic transcript analyzer historical filenames.

- [ ] **Step 3: Run documentation and harness tests**

```bash
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context_test.sh
```

Expected: both scripts print `PASS` and exit zero.

- [ ] **Step 4: Commit**

```bash
git add README.md docs/OUTPUTS.md docs/BENCHMARKING.md
git commit -m "Document context source quality guarantees" \
  -m "- Explain domain-model evidence and bounded source-family diversity
- Keep transcript efficiency gates independent of benchmark-specific names"
```

---

## Task 6: Verify, Install Locally, and Rebuild the Historical Workspace

**Files:**

- Verify only: all tracked files
- External generated output: `/Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix`

- [ ] **Step 1: Run formatting and repository checks**

```bash
gofmt -w internal/agent/context_intent.go internal/agent/context_rank.go internal/agent/context_select.go internal/agent/context_source.go internal/agent/context_test.go internal/agent/context_source_test.go internal/agent/context_change_analysis_test.go
git diff --check
go test ./... -count=1
go vet ./...
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context_test.sh
```

Expected: every command exits zero; both shell suites print `PASS`.

- [ ] **Step 2: Install the verified binary locally**

```bash
go install ./cmd/goregraph
goregraph version
```

Expected: installation succeeds and `goregraph version` reports the repository version. Do not create a tag, release artifact, or release commit.

- [ ] **Step 3: Clean and rescan the exact benchmark workspace**

```bash
benchmark_workspace=/Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix
goregraph workspace clean "$benchmark_workspace" --execute --workspace "$benchmark_workspace"
goregraph workspace scan-all "$benchmark_workspace" --workspace "$benchmark_workspace"
goregraph workspace status "$benchmark_workspace" --workspace "$benchmark_workspace"
```

Expected: clean removes only generated GoreGraph output, scan-all succeeds for all three prepared projects, and status reports a complete current workspace index.

- [ ] **Step 4: Inspect one fresh Context Pack before the end-to-end agent run**

```bash
goregraph context "$benchmark_workspace" \
  --query 'Im Vorschriftendienst bleibt beim Entfernen einer Vorschrift aus einem Kataster die verbundene Aufgabe bestehen. Analysiere repository-übergreifend in ms-cadasterregulation, ms-cadastertask und ms-common: öffentlicher REST-Endpunkt, aktuelle Aufrufkette und Ursache; erforderliche neue Aufrufkette; betroffene Aufgabenarten und Suchattribute; interner API-Vertrag; Authentifizierung und Konfiguration; Persistenzoperationen; Nebenwirkungen für Protokollierung, E-Mail und Benutzerinformationen; notwendige Produktions- und Testdateien; Fehlerbehandlung, Retry-Logik und Tests. Nur Analyse, keine Implementierung.' \
  --budget-tokens 4000 \
  --max-files 12
```

Expected:

- `fallback_required: false`;
- `retry_allowed: false`;
- exactly one reliable production entrypoint;
- current regulation controller and operations bodies are present;
- `CadasterTaskMgmtService` current client/retry/configuration pattern is present;
- current task provider behavior is present;
- source evidence distinguishes `CadasterRegTaskEntity` from `CadasterRegChangeTaskEntity`;
- source evidence exposes `cadasterId`, `objectId`, and the change-only `lra`;
- both `CadasterRegTaskRepository` and `CadasterRegChangeTaskRepository` are represented;
- generic one-line mail properties, async exception handler, protocol repository, and unrelated topic repository do not consume source slots unless their bodies carry stronger task-domain evidence;
- the absent bulk-delete contract remains explicitly uncertain;
- estimated tokens are at most 4,000 and source sections/files are at most twelve.

- [ ] **Step 5: Run the matched three-by-three end-to-end benchmark**

Use the unchanged historical prompt, baseline instruction, assisted instruction, model, reasoning effort, sandbox, and approval mode. Store raw JSONL and stderr outside the repository:

```bash
benchmark_prompt=${GOREGRAPH_BENCHMARK_PROMPT:?set GOREGRAPH_BENCHMARK_PROMPT to the unchanged historical prompt file}
baseline_instruction=${GOREGRAPH_BASELINE_INSTRUCTION:?set GOREGRAPH_BASELINE_INSTRUCTION to the unchanged baseline instruction file}
assisted_instruction=${GOREGRAPH_ASSISTED_INSTRUCTION:?set GOREGRAPH_ASSISTED_INSTRUCTION to the unchanged assisted instruction file}
benchmark_output=${GOREGRAPH_BENCHMARK_OUTPUT:?set GOREGRAPH_BENCHMARK_OUTPUT to a new absolute result directory}

./scripts/benchmark-agent-context.sh \
  --workspace "$benchmark_workspace" \
  --prompt "$benchmark_prompt" \
  --baseline-instruction "$baseline_instruction" \
  --assisted-instruction "$assisted_instruction" \
  --runs 3 \
  --output "$benchmark_output"
```

Required acceptance:

- the assisted answer identifies both regular regulation tasks and regulation-change tasks;
- it identifies `cadasterId` and `objectId` as shared lookup attributes and `lra` as change-task-specific;
- it preserves the current Oracle deletion call and its order in the existing chain;
- it distinguishes current behavior from the proposed missing internal DELETE contract;
- it covers both task repositories plus dependent comment cleanup without inventing existing delete methods;
- it treats mail, protocol, and user-information effects as evidence-backed behavior or explicit uncertainty;
- it names concrete production and test files from supplied source;
- assisted median tokens remain at most 80% of baseline;
- assisted median tool calls remain at most 70% of baseline;
- assisted median source reads remain at most 50% of baseline;
- `repeated_full_packs = 0` and `included_source_rereads = 0`.

Do not accept a single fast run as sufficient. All three assisted runs must satisfy the content rubric, and the median must satisfy the structural efficiency gates.

---

## Final Self-Review Checklist

- [ ] No production selector contains benchmark repository names, historical route fragments, or Weka class names.
- [ ] The fixture still omits the desired future bulk-delete API and graph edge.
- [ ] `Search` recalls candidates but does not prove stable identity relevance or earn identity quality.
- [ ] A project-only boundary prefers informative domain evidence; exact-fact boundaries remain cost-first.
- [ ] Two distinct domain models and two distinct persistence owners can earn diversity, but a third same-family option cannot.
- [ ] Generic cross-cutting signature penalties never exclude a required exact boundary.
- [ ] Production remains ahead of tests while requested test evidence is still eligible.
- [ ] English and German tasks produce identical semantic and source-selection snapshots.
- [ ] Reversing fact and edge input order does not change output.
- [ ] Missing contracts remain uncertainty and never appear in the current call chain.
- [ ] Fallback and retry decisions remain based on usable source omissions, not optional discarded candidates.
- [ ] Markdown and JSON expose equivalent concern, role, coverage, omission, fallback, and retry decisions.
- [ ] The final pack remains within 4,000 tokens, 12 files, 12 source sections, and three source omissions.
- [ ] `git diff --check`, `go test ./... -count=1`, `go vet ./...`, and both shell suites pass.
- [ ] Local installation and external workspace regeneration happen only after repository verification.
- [ ] No tag, release, or release artifact is created.
