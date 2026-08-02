# Compact Production Plan Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve exact existing provider-contract and primary-persistence file identities alongside configuration, test, and uncertainty evidence in a deterministic 4,000-token Context Pack.

**Architecture:** Group configuration resources by project and identical key groups to reclaim repeated metadata bytes. Add a compact, value-free production-plan group selected only for exact missing-transition file inventories; keep source-reading authority exclusively in `source_sections` and bounded `source_omissions`.

**Tech Stack:** Go 1.23+, standard library, Bash regression harnesses, GoReleaser v2, Schema 3 Context Packs.

## Global Constraints

- Keep `BudgetTokens <= 4000`, `MaxFiles <= 12`, existing source-section ceilings, and bounded omission limits unchanged.
- Never expose configuration values or authorize reads through metadata-only fields.
- Use no private benchmark identifiers, paths, prompts, or language-specific names in production code or committed tests.
- Preserve future route, method, status, lookup implementation, dependent persistence, cascade behavior, and cross-service ordering as explicit unknowns unless rendered source proves them.
- Add no dependencies and retain Go 1.23 compatibility plus Darwin, Linux, and Windows builds.
- Do not release, tag, merge, or modify historical workspace source files.

---

### Task 1: Reproduce the Tight-Budget Inventory Regression

**Files:**
- Modify: `internal/agent/context_change_analysis_test.go`
- Test: `internal/agent/context_change_analysis_test.go`

**Interfaces:**
- Consumes: `runtimeShapeReleaseQualityMissingContractIndex`, `BuildContext`, `contextPackRepresentsSourcePath`.
- Produces: `TestBuildContextKeepsCompactProductionPlanEvidence`, which fails until grouped configuration and production-plan metadata exist.

- [ ] **Step 1: Extend the generic fixture with supplemental configuration profiles and caller test patterns**

Append exact, value-free configuration facts for production and test profiles in both caller and provider projects. Use literal generic paths and key-group names:

```go
scan.AgentContextFactRecord{
    ID: "catalog-technical-user-production", Project: "services/catalog",
    Kind: "configuration", Name: "technical-user",
    File: "src/main/resources/application.properties",
    Confidence: "EXACT", Search: "technical user authentication configuration production",
}
```

Add matching provider resources and `application-test.properties` facts. Fixture file contents may contain sentinel values, but the final encoded pack must not contain them.

Also append generic caller-side mock and retry-test symbol facts, matching the
existing compact plan-file fixture, so the regression can require both provider
tests and both caller patterns without benchmark-specific identities.

- [ ] **Step 2: Write the failing end-to-end assertions**

Build the broad missing-transition query twice with `BudgetTokens: 4000` and `MaxFiles: 12`. Assert byte-identical JSON, exact budget bounds, four configuration resource identities, one provider contract, two primary persistence paths, dependent-persistence uncertainty, and all four plan-file identities:

```go
want := ContextProductionPlanFiles{
    Project: "services/jobs",
    ProviderContract: "src/main/java/example/JobManagementController.java",
    PrimaryPersistence: []string{
        "src/main/java/example/CatalogChangeJobRepository.java",
        "src/main/java/example/CatalogJobRepository.java",
    },
}
if len(pack.ProductionPlanFiles) != 1 ||
    !reflect.DeepEqual(pack.ProductionPlanFiles[0], want) {
    t.Fatalf("production plan = %#v, want %#v", pack.ProductionPlanFiles, want)
}
```

- [ ] **Step 3: Run RED verification**

```bash
go test ./internal/agent -run TestBuildContextKeepsCompactProductionPlanEvidence -count=1
```

Expected: compile failure because `ProductionPlanFiles` and `ContextProductionPlanFiles` do not exist.

---

### Task 2: Group Configuration Resource Metadata

**Files:**
- Modify: `internal/agent/context.go`
- Modify: `internal/agent/context_configuration_resources.go`
- Modify: `internal/agent/context_rank.go`
- Modify: `internal/agent/context_change_analysis_test.go`

**Interfaces:**
- Produces:

```go
type ContextConfigurationResource struct {
    Path    string `json:"path"`
    Profile string `json:"profile"`
}

type ContextConfigurationResourceGroup struct {
    Project   string                         `json:"project,omitempty"`
    KeyGroups []string                       `json:"key_groups,omitempty"`
    Resources []ContextConfigurationResource `json:"resources"`
}

func contextConfigurationResources(
    ContextPack,
    scan.AgentContextIndexRecord,
) []ContextConfigurationResourceGroup
```

- [ ] **Step 1: Add focused failing grouping tests**

Assert that identical project/key-group resources share one group, differing key groups do not merge, resources are sorted by profile/path, represented paths are excluded, and total resources never exceeds six.

- [ ] **Step 2: Verify the grouping tests fail**

```bash
go test ./internal/agent -run 'TestContextConfigurationResourcesAddsOnlyUnrepresentedProfiles|TestContextConfigurationResourcesGroupsOnlyEquivalentKeySets' -count=1
```

Expected: compile or deep-equality failure against the old flat structure.

- [ ] **Step 3: Implement deterministic grouping**

Keep the existing fact eligibility and scoring. Select at most six individual resources first, then group by normalized project plus sorted key groups. Deep-copy `KeyGroups` and `Resources` in `cloneContextPack`.

- [ ] **Step 4: Verify GREEN and budget reduction**

```bash
go test ./internal/agent -run 'TestContextConfigurationResources|TestBuildContextBalancesBroadReleaseEvidence' -count=1
```

Expected: PASS; no sentinel configuration values appear in encoded packs.

---

### Task 3: Select Compact Production Plan Identities

**Files:**
- Modify: `internal/agent/context.go`
- Create: `internal/agent/context_production_plan_files.go`
- Create: `internal/agent/context_production_plan_files_test.go`
- Modify: `internal/agent/context_rank.go`
- Modify: `internal/agent/context_change_analysis_test.go`

**Interfaces:**
- Produces:

```go
type ContextProductionPlanFiles struct {
    Project            string   `json:"project,omitempty"`
    ProviderContract   string   `json:"provider_contract,omitempty"`
    PrimaryPersistence []string `json:"primary_persistence,omitempty"`
}

func contextProductionPlanFiles(
    pack ContextPack,
    index scan.AgentContextIndexRecord,
) []ContextProductionPlanFiles
```

- [ ] **Step 1: Write failing selector tests**

Use generic caller/client/provider facts and assert no result for ordinary or non-inventory queries; one internal provider route/controller; at most two exact primary repository-owner paths; rejection of dependent/comment, generic, test, foreign-project, absolute, and represented paths; and input-order independence.

- [ ] **Step 2: Run RED verification**

```bash
go test ./internal/agent -run 'TestContextProductionPlanFiles|TestBuildContextKeepsCompactProductionPlanEvidence' -count=1
```

Expected: compile failure because the production selector does not exist.

- [ ] **Step 3: Implement provider selection**

Reuse `contextPlanFileEntrypointProject`, `contextPlanFileProviderProjects`, `contextExactInventoryInternalInterfaceFact`, exact confidence checks, action-family overlap, normalized path validation, and represented-path deduplication. Group facts by path and select the highest-scoring internal production route/controller per provider project.

- [ ] **Step 4: Implement primary-persistence selection**

Reuse exact repository-owner recognition, source-domain tokens, stable identity matching, and dependency classification. Exclude `contextDomainModelDependencyFact` candidates and keep at most two sorted distinct paths.

- [ ] **Step 5: Attach metadata during final source decision**

Set `pack.ProductionPlanFiles = contextProductionPlanFiles(pack, index)` after source selection and before final estimate. Deep-copy all nested path slices in `cloneContextPack`.

- [ ] **Step 6: Run GREEN verification**

```bash
gofmt -w internal/agent/context.go internal/agent/context_configuration_resources.go internal/agent/context_production_plan_files.go internal/agent/context_production_plan_files_test.go internal/agent/context_change_analysis_test.go internal/agent/context_rank.go
go test ./internal/agent -run 'TestContextConfigurationResources|TestContextProductionPlanFiles|TestBuildContextKeepsCompactProductionPlanEvidence|TestBuildContextBalancesBroadReleaseEvidence' -count=1
```

Expected: PASS, deterministic packs, `EstimatedTokens <= 4000`, and unchanged source/file/omission ceilings.

- [ ] **Step 7: Commit the model and selector**

```bash
git add internal/agent/context.go internal/agent/context_configuration_resources.go internal/agent/context_production_plan_files.go internal/agent/context_production_plan_files_test.go internal/agent/context_change_analysis_test.go internal/agent/context_rank.go
git commit -m "Preserve compact production plan evidence" -m $'- Group equivalent configuration resource identities\n- Surface exact provider and primary persistence file plans within existing budgets'
```

---

### Task 4: Render and Enforce the Metadata Contract

**Files:**
- Modify: `internal/query/context.go`
- Modify: `internal/query/context_test.go`
- Modify: `internal/agentguide/instruction.go`
- Modify: `internal/agentguide/instruction_test.go`
- Modify: `scripts/benchmark-agent-context.sh`
- Modify: `scripts/benchmark-agent-context_test.sh`
- Modify: `scripts/sync-docs/main.go`
- Modify: `scripts/sync-docs/main_test.go`
- Modify: `README.md`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/RELEASE.md`

**Interfaces:**
- Produces heading `## Production plan file identities (metadata only; do not read)`.
- Extends `agentguide.AssistedInstruction` to require every `production_plan_files` identity in the production-file inventory with its role.

- [ ] **Step 1: Write failing renderer tests**

Assert sorted grouped configuration entries and sorted production groups. Reject blank/unsafe paths. Verify both metadata headings appear between `Files to inspect` and `Source sections` and occur once.

- [ ] **Step 2: Write the failing instruction/harness expectation**

Add this exact clause to the test fixture first:

```text
name every supplied production_plan_files identity in the production-file inventory with its role because naming metadata is not reading source
```

Run the harness and require the RED error `assisted instruction does not match docs/BENCHMARKING.md`.

- [ ] **Step 3: Implement rendering and canonical instruction**

Render full `project/path` references without line ranges. List `provider_contract` and `primary_persistence` roles explicitly. Update the canonical instruction, then update the release harness's accepted value.

- [ ] **Step 4: Synchronize generated documentation**

```bash
go run ./scripts/sync-docs --write
go run ./scripts/sync-docs --check
```

Document both metadata shapes, value-free/read-prohibited semantics, caps, and release-quality purpose. Do not publish a passing benchmark claim before a passing matrix exists.

- [ ] **Step 5: Verify GREEN**

```bash
go test ./internal/query ./internal/agentguide ./scripts/sync-docs -count=1
bash scripts/benchmark-agent-context_test.sh
git diff --check
```

- [ ] **Step 6: Commit integration and documentation**

```bash
git add internal/query internal/agentguide scripts README.md docs/BENCHMARKING.md docs/OUTPUTS.md docs/RELEASE.md
git commit -m "Document production plan identities" -m $'- Render metadata-only production and grouped configuration resources\n- Synchronize agent guidance, benchmark validation, and release documentation'
```

---

### Task 5: Verify, Install, Rescan, and Requalify

**Files:**
- Verify: entire repository.
- Write external evidence only below new `/private/tmp/goregraph-*` directories.
- Modify historical workspace generated outputs only: three `goregraph-out` directories and `.goregraph-workspace`.

**Interfaces:**
- Consumes: committed candidate, frozen base prompt, exact local binary metadata, approved historical workspace digest.
- Produces: local gate evidence, installed candidate, clean G1 scan, retained external smoke/matrix evidence, and release-readiness classification.

- [ ] **Step 1: Run complete repository gates**

```bash
go test ./... -count=1
go vet ./...
go test -race ./... -count=1
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
bash scripts/calibrate-agent-context-tokens_test.sh
go run ./scripts/sync-docs --check
git diff --check
```

- [ ] **Step 2: Verify compatibility and release packaging**

Run the full suite with `GOTOOLCHAIN=go1.23.12`, then build Darwin amd64/arm64, Linux amd64/arm64, and Windows amd64. Run `goreleaser check` and a snapshot build with a temporary `GOCACHE`; inspect archive/checksum outputs and remove only the exact generated `dist` directory afterward.

- [ ] **Step 3: Push and install the exact candidate**

Push `fix/release-benchmark-metrics`. Build `/Users/gorecode/go/bin/goregraph` with exact `Commit` and UTC `Built` ldflags. Verify `goregraph version`, `command -v goregraph`, and SHA-256.

- [ ] **Step 4: Clean and rebuild historical generated outputs**

Preview `goregraph workspace clean`, execute only the four confirmed generated paths, then run `goregraph workspace build all <root> --workspace <root>`. Require three indexed projects, workspace Doctor `FAIL=0`, each project Doctor `FAIL=0`, and unchanged source digest `549e6f2af8a030fb62353b9a4a8f2f7954861a731b656578089b1f8952e6148f`.

- [ ] **Step 5: Verify the real Context Pack locally**

Build the frozen query twice. Require byte-identical packs, `EstimatedTokens <= 4000`, all four configuration profiles, one provider contract, two primary repositories, four plan-file identities, dependent-persistence/cascade uncertainty, cross-service-ordering uncertainty, and bounded omissions only.

- [ ] **Step 6: Run external fail-fast qualification**

Run one read-only assisted smoke with the exact installed candidate, frozen prompt/settings, zero external skill reads, and unchanged source digest. Manually score all twelve items. Only a 12/12 smoke may start three baseline and three assisted release runs with identical settings. Retain every transcript, metric, signed review, and failure classification.

- [ ] **Step 7: Audit release readiness**

Require all automatic gates, assisted median quality at least baseline median, complete documentation truth, clean branch/upstream state, and no unresolved release blocker. Report readiness without releasing, tagging, merging, or changing historical sources.
