# Balanced Release Evidence Ranking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Balance a bounded Context Pack across every explicitly requested evidence area so authentication, configuration, contracts, and exact test inventory are not displaced by repeated model or persistence facets.

**Architecture:** Add bounded, value-free agent facts for conventional Spring configuration resources and mask their values during rendering. Then extend proof-aware selection and inventory scores with a leading count of fully represented public concern keys, keep internal-facet proof and identity checks as secondary criteria, and direct bounded omissions to required public areas that remain unrepresented. No public schema, CLI, prompt, or limit changes are required.

**Tech Stack:** Go 1.23, Go standard library, existing `internal/agent` Context selector, table-driven Go tests, existing shell benchmark harness

## Global Constraints

- Keep the 4,000-token Context Pack limit.
- Keep the 12-file and 12-source-section limits.
- Keep the three bounded source-omission limit.
- Preserve mandatory entrypoint and current-path evidence.
- Preserve deterministic output and bounded candidate discovery/substitution.
- Do not add retries, fallbacks, dependencies, prompt instructions, or private identifiers.
- Do not place configuration values in the agent index or rendered Context Pack.
- Do not relax token, source-read, tool-call, workspace, skill-read, or manual quality gates.

---

## File Structure

- Modify `internal/agent/context_change_analysis_test.go`: broad release-shaped regression using the existing generic Java/Spring fixture.
- Modify `internal/agent/context_substitute.go`: public-area source selection score.
- Modify `internal/agent/context_proof.go`: public-area scoring for exact evidence inventory.
- Modify `internal/agent/context_select.go`: dynamic omission representation and priority.
- Modify `internal/agent/context_source_test.go`: focused unit tests for the three ranking decisions.
- Create `internal/scan/agent_context_configuration.go`: bounded Spring resource facts containing paths, key groups, and line ranges only.
- Create `internal/scan/agent_context_configuration_test.go`: deterministic extraction and value-exclusion tests.
- Modify `internal/scan/scan.go`: add configuration-resource facts to the project agent projection.
- Modify `internal/scan/types.go`: retain extracted internal configuration facts while scanning.
- Modify `internal/agent/context_source.go`: mask property/YAML values before source serialization.

### Task 0: Safe configuration-resource evidence

**Files:**

- Create: `internal/scan/agent_context_configuration.go`
- Create: `internal/scan/agent_context_configuration_test.go`
- Modify: `internal/scan/scan.go:260-285,400-414`
- Modify: `internal/scan/types.go:9-20`
- Modify: `internal/agent/context_source.go:785-850`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Produces: `extractAgentContextConfigurationFacts(file FileRecord, body string) []AgentContextFactRecord`.
- Produces: `appendAgentContextConfigurationFacts(index AgentContextIndexRecord, project string, facts []AgentContextFactRecord) AgentContextIndexRecord`.
- Produces: `redactContextConfigurationValues(path, content string) string`.

- [ ] **Step 1: Write failing extraction and redaction tests**

Use production and test-profile `.properties` fixtures containing client URL, username, password, timeouts, and retries. Require deterministic configuration facts for the file and key group; assert that fact names, summaries, and search text contain no values. Render a bounded property section and a YAML section containing sentinel credentials; assert keys and line numbers remain while both sentinels are absent.

- [ ] **Step 2: Verify the red state**

```bash
go test ./internal/scan -run 'TestExtractAgentContextConfigurationFacts|TestAppendAgentContextConfigurationFacts' -count=1
go test ./internal/agent -run TestRenderSourceCandidateRedactsConfigurationValues -count=1
```

Expected: FAIL because the extraction, append, and redaction helpers do not exist.

- [ ] **Step 3: Implement bounded value-free facts**

Recognize only `application` and `bootstrap` resources with `.properties`, `.yml`, or `.yaml` suffixes. Emit one file-level fact plus at most 32 sorted key-group facts per file. Parse keys and contiguous line ranges without retaining values. Set kind `configuration`, exact confidence, portable relative path, and stable IDs through the existing `stableID` helper.

Collect facts while file contents are already in memory, append them to the project agent index, and re-sort with `sortAgentContextFacts`. Do not change general file, symbol, or workspace schemas.

- [ ] **Step 4: Implement renderer masking**

Before constructing a final source section for a recognized configuration resource, replace each non-comment `.properties` value after its delimiter and each YAML scalar value after `:` with `<redacted>`. Preserve indentation, keys, list structure, tabs carrying rendered line numbers, and blank/comment lines.

- [ ] **Step 5: Format and verify**

```bash
gofmt -w internal/scan/agent_context_configuration.go internal/scan/agent_context_configuration_test.go internal/scan/scan.go internal/scan/types.go internal/agent/context_source.go internal/agent/context_source_test.go
go test ./internal/scan -run 'TestExtractAgentContextConfigurationFacts|TestAppendAgentContextConfigurationFacts' -count=1
go test ./internal/agent -run TestRenderSourceCandidateRedactsConfigurationValues -count=1
```

### Task 1: Broad release-quality regression

**Files:**

- Modify: `internal/agent/context_change_analysis_test.go:15-19,328-396`

**Interfaces:**

- Consumes: `writeReleaseQualityMissingContractFixture`, `BuildContext`, and final source/inventory/omission metadata.
- Produces: `broadReleaseQualityQuery` and `contextPackRepresentsSourcePath(ContextPack, string, string) bool` for tests.

- [ ] **Step 1: Write the failing broad-query test**

Use this query, which names evidence categories but not implementation classes:

```go
const broadReleaseQualityQuery = "When DELETE /catalog/items/{itemId} removes an item in services/catalog, analyze the current and required cross-service cleanup through libraries/job-client and services/jobs. Cover the public and internal HTTP contracts, both job types and lookup attributes, client and provider authentication and configuration, retry and failure behavior, persistence, side effects, and the exact production and executable test files required for a release-ready change."
```

The test must accept a path only when it appears in final source, exact file inventory, or a line-bounded omission. Require the base model, client config/auth, provider security and management controller, both repositories, both provider tests, and production/test-profile configuration resources in the calling service. Reject the wrong-project duplicate and any sentinel configuration value. Assert 4,000 tokens, 12 aggregate files, 12 sections, three omissions, and byte-stability across three builds.

- [ ] **Step 2: Run the focused test and verify the red state**

```bash
go test ./internal/agent -run TestBuildContextBalancesBroadReleaseEvidence -count=1
```

Expected: FAIL because provider auth/config/controller or adjacent test inventory is missing.

### Task 2: Public-area-first source substitution

**Files:**

- Modify: `internal/agent/context_substitute.go:10-15,209-223,380-409`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Produces: `contextSourceRequiredPublicProofs(concerns []contextConcern, covered map[string]bool) int`.
- Extends: `contextSourceSelectionScore.requiredPublicProofs int`.

- [ ] **Step 1: Write focused score tests**

Create required concerns where two internal facets share one public key. Verify the helper counts that public area only when both facets are covered. Compare equal-internal-proof scores and require the score with one completed public area to win:

```go
if !betterContextSourceSelection(
    contextSourceSelectionScore{requiredPublicProofs: 1, requiredProofs: 2},
    contextSourceSelectionScore{requiredPublicProofs: 0, requiredProofs: 2},
) {
    t.Fatal("complete requested public evidence area did not win")
}
```

- [ ] **Step 2: Verify the red state**

```bash
go test ./internal/agent -run 'TestContextSourceRequiredPublicProofs|TestBetterContextSourceSelectionPrefersCompletePublicEvidence' -count=1
```

Expected: FAIL because the field/helper do not exist.

- [ ] **Step 3: Implement the minimal score**

Group only required concerns by `firstNonEmptyContext(concern.publicKey, concern.key)`. Mark a group incomplete when any member is absent from `covered`. Set the result in `contextSourceSelectionScoreFor` and compare it before `requiredProofs`. Preserve proof protection and all later tie-breakers.

- [ ] **Step 4: Format and test**

```bash
gofmt -w internal/agent/context_substitute.go internal/agent/context_source_test.go internal/agent/context_change_analysis_test.go
go test ./internal/agent -run 'TestContextSourceRequiredPublicProofs|TestBetterContextSourceSelectionPrefersCompletePublicEvidence|TestBuildContextBalancesBroadReleaseEvidence' -count=1
```

The focused score tests must pass. Record any broad-test paths still missing.

### Task 3: Balanced exact evidence inventory

**Files:**

- Modify: `internal/agent/context_proof.go:12-31,149-190,281-390`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Extends: `contextEvidenceInventoryCandidate.publicFacets map[string]bool`.
- Extends: `contextEvidenceInventoryScore.publicAreas int`.

- [ ] **Step 1: Write the failing inventory test**

Build profiled options where two paths prove separate persistence facets under one public key and another path proves an unrepresented authentication key. With two file slots, require persistence plus authentication. With three slots, require the second repository to remain eligible.

- [ ] **Step 2: Verify the red state**

```bash
go test ./internal/agent -run TestContextEvidenceInventoryBalancesPublicAreasBeforeRepeatedFacets -count=1
```

Expected: FAIL because internal production facets currently win first.

- [ ] **Step 3: Implement public-area inventory scoring**

For every matched required concern, retain `concern.key` and `firstNonEmptyContext(concern.publicKey, concern.key)`. Count unique represented public keys across production and tests. Compare `publicAreas` before `productionFacets`; preserve every existing weak-evidence, path, quality, and deterministic-key comparison.

- [ ] **Step 4: Format and test**

```bash
gofmt -w internal/agent/context_proof.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestContextEvidenceInventoryBalancesPublicAreasBeforeRepeatedFacets|TestBuildContextBalancesBroadReleaseEvidence' -count=1
```

### Task 4: Dynamic bounded omission balance

**Files:**

- Modify: `internal/agent/context_select.go:4079-4195`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Produces: `contextSourcePublicAreaRendered(pack ContextPack, concern contextConcern, concerns []contextConcern, options []contextSourceOption) bool`.
- Extends the internal omission-priority call with concerns/options; public output is unchanged.

- [ ] **Step 1: Write omission ordering tests**

Construct a missing-transition pack with rendered domain and persistence evidence while auth, config, side effects, and tests remain uncovered with exact options. Assert repeated domain/persistence paths do not displace three completely unrepresented public areas. Keep `TestExistingFlowOmissionsPreserveConcernRank` unchanged.

- [ ] **Step 2: Verify the red state**

```bash
go test ./internal/agent -run 'TestMissingTransitionOmissionsPreferUnrepresentedPublicAreas|TestExistingFlowOmissionsPreserveConcernRank' -count=1
```

Expected: new test FAILS; existing-flow test PASSES.

- [ ] **Step 3: Implement final-representation priority**

Consider a public area rendered when a final source section corresponds to an option proving any required concern with the same public key. For missing-transition plans only, add a leading fixed bonus to unrepresented public areas before existing kind priority and rank. Keep pathless priority zero and non-missing-transition ordering identical.

- [ ] **Step 4: Format and run agent tests**

```bash
gofmt -w internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestMissingTransitionOmissionsPreferUnrepresentedPublicAreas|TestExistingFlowOmissionsPreserveConcernRank|TestBuildContextBalancesBroadReleaseEvidence' -count=1
go test ./internal/agent -count=1
```

- [ ] **Step 5: Commit the implementation**

```bash
git add internal/scan/agent_context_configuration.go internal/scan/agent_context_configuration_test.go internal/scan/scan.go internal/scan/types.go internal/agent/context_source.go internal/agent/context_change_analysis_test.go internal/agent/context_substitute.go internal/agent/context_proof.go internal/agent/context_select.go internal/agent/context_source_test.go
git commit -m "Balance requested release evidence" -m "- Complete requested public evidence areas before repeated internal facets
- Index conventional configuration resources without exposing their values
- Preserve exact auth, configuration, contract, and test inventory under fixed limits
- Direct bounded omissions to still-unrepresented evidence areas"
```

### Task 5: Candidate verification and release matrix 2

**Files:**

- Verify: entire repository and historical benchmark workspace.
- Write evidence only to a new `/private/tmp/goregraph-release-3x3-<commit>-20260801-m2` directory.

- [ ] **Step 1: Run all local gates**

```bash
go test ./... -count=1
go vet ./...
bash scripts/analyze-agent-context-log_test.sh
bash scripts/calibrate-agent-context-tokens_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
go run ./scripts/sync-docs --check
git diff --check
```

- [ ] **Step 2: Review the focused diff independently**

Check private identifiers, hard-coded prompt wording, limit drift, proof inflation, determinism, and generic regressions. Fix valid findings and rerun Step 1.

- [ ] **Step 3: Push and install the exact commit**

Push the branch, install with exact `Commit`/UTC `Built` ldflags, and verify `goregraph version` plus binary SHA-256.

- [ ] **Step 4: Clean and freshly scan the historical workspace**

Preview and execute workspace clean, then scan-all and status. Require all three projects indexed and the approved workspace digest unchanged.

- [ ] **Step 5: Run and review matrix 2**

Use three baseline and three assisted runs with the retained prompt, identical model/reasoning/read-only settings, `--ignore-user-config`, and the same two disabled skill paths. Preserve completed poor runs. Require automatic gates plus assisted median manual quality at least baseline median; otherwise write a failure classification before the next focused fix.
