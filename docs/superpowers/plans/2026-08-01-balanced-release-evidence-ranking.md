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

### Task 4A: Correct required model and authentication roles

**Files:**

- Modify: `internal/agent/context_select.go`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Produces: `contextInferredPrimaryProjectModelDuplicate(pack ContextPack, index scan.AgentContextIndexRecord, fact scan.AgentContextFactRecord) bool`.
- Produces: `contextAuthenticationConcernHasRoleEvidence(concern contextConcern, index scan.AgentContextIndexRecord, endpointProjects, contractProjects, modelProjects map[string]bool) bool`.

- [ ] **Step 1: Write failing role-modeling tests**

Add `TestContextRequestedDomainModelIDsExcludeInferredPrimaryDuplicateUnlessExplicit`. A generic cross-service model request must exclude the inferred caller-project duplicate, while a query that explicitly names or compares that caller model must keep it. Add `TestContextSourceConcernsRoleGateCredentialOnlyCallerAuthentication`. Caller configuration containing username/password keys must not create required server authentication; an exact authentication/security fact must keep the concern required.

- [ ] **Step 2: Verify the red state**

```bash
go test ./internal/agent -run 'TestContextRequestedDomainModelIDsExcludeInferredPrimaryDuplicateUnlessExplicit|TestContextSourceConcernsRoleGateCredentialOnlyCallerAuthentication' -count=1
```

Expected: FAIL because model duplicate filtering happens only at option quality and public authentication concerns are retained without role evidence.

- [ ] **Step 3: Implement early bounded filtering**

Reuse the existing compact identity and explicit-query-name checks at fact level. Filter inferred primary-project duplicates while building selected/planned requested-model IDs, then retain any explicitly named caller model. Before authentication expansion, require endpoint/contract/model project role or an exact source-backed authentication/security fact; configuration facts containing credential vocabulary do not qualify.

- [ ] **Step 4: Format and verify**

```bash
gofmt -w internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestContextRequestedDomainModelIDsExcludeInferredPrimaryDuplicateUnlessExplicit|TestContextSourceConcernsRoleGateCredentialOnlyCallerAuthentication|TestContextDomainModelEvidenceConcernsScopeModelsAndLinkedBase|TestBuildContextBalancesBroadReleaseEvidence' -count=1
git diff --check
```

Record the broad residuals without weakening its assertions.

### Task 4B: Add bounded exact-inventory evidence subareas

**Files:**

- Modify: `internal/agent/context_intent.go`
- Modify: `internal/agent/context_select.go`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Extends: `contextConcern.exactInventory bool`.
- Produces: `contextQueryRequestsExactEvidenceInventory(query string) bool`.
- Produces bounded path-specific authentication, configuration, and executable-test concerns after semantic expansion.

- [ ] **Step 1: Write failing exact-inventory tests**

Add `TestExpandContextExactInventoryConcernsCreatesBoundedPathSubareas`. An explicit exact production/test file-inventory query must create one stable subarea per distinct exact source path for authentication, configuration, and executable tests; duplicate facts in a file collapse. A normal category query creates none. Reverse input facts and require byte-equivalent ordered keys. Generate more eligible paths than the existing planning-candidate bound and require the bound.

- [ ] **Step 2: Verify the red state**

```bash
go test ./internal/agent -run TestExpandContextExactInventoryConcernsCreatesBoundedPathSubareas -count=1
```

Expected: FAIL because exact paths still collapse into coarse semantic concerns.

- [ ] **Step 3: Implement the minimal trigger and subareas**

Require exactness, file/path/inventory identity, and production/test scope markers in the normalized query. Use the existing intent token helpers and include equivalent supported German markers; do not key behavior to benchmark-specific class names or full phrases. Create subareas only from exact-confidence, non-empty source paths already belonging to a required authentication, configuration, or test concern. Authentication requires authentication/security kind, configuration requires configuration kind, and tests require executable test evidence. Group by normalized project/path, exclude mandatory entrypoint/contract paths, sort by project/path/line/fact ID, and cap globally at `maximumContextSourcePlanningCandidates`.

Use internal stable keys of the form `<kind>:<project>#exact-file:<normalized-path>` for key/publicKey and `exact-file:<normalized-path>` for facet. Preserve every coarse semantic concern.

- [ ] **Step 4: Format and verify**

```bash
gofmt -w internal/agent/context_intent.go internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestExpandContextExactInventoryConcernsCreatesBoundedPathSubareas|TestContextSourceRequiredPublicProofs|TestContextEvidenceInventoryBalancesPublicAreasBeforeRepeatedFacets|TestMissingTransitionOmissionsPreferUnrepresentedPublicAreas|TestExistingFlowOmissionsPreserveConcernRank|TestBuildContextBalancesBroadReleaseEvidence' -count=1
git diff --check
```

The focused subarea test must pass. Record any remaining broad source-section deficit.

### Task 4C: Reconcile final inventory with source sections once

**Files:**

- Modify: `internal/agent/context_select.go`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Produces: `reconcileContextSourceInventory(pack ContextPack, request ContextRequest, options []contextSourceOption, concerns []contextConcern, coreBoundaries []contextSourceBoundary) (ContextPack, error)` or an equivalent internal helper using the existing selection state.

- [ ] **Step 1: Write the failing reconciliation test**

Add `TestReconcileContextSourceInventoryAddsRepresentedEvidenceOnly`. A pack containing 12 final inventory files and 10 source sections must add two proving sections from already represented paths. Mandatory sections remain byte-identical, reversed options produce identical output, no unrepresented path is added, and all fixed file/source/token bounds remain unchanged.

- [ ] **Step 2: Verify the red state**

```bash
go test ./internal/agent -run TestReconcileContextSourceInventoryAddsRepresentedEvidenceOnly -count=1
```

Expected: FAIL because final inventory changes are not reconciled into source sections.

- [ ] **Step 3: Implement one non-recursive pass**

Call reconciliation exactly once after final `appendContextEvidenceInventory` and before final omission construction. Consider only profiled options proving required concerns whose normalized project/path already appears in `pack.Files`; skip rendered candidates. Reconstruct coverage/selection state from the final sections, sort deterministically by exact-inventory gain, required proof gain, candidate quality, lower token cost, and `contextSourceOptionLess`, then add through the existing bounded option helper. Never replace/remove mandatory or current-path sections, never add inventory files, and stop at the existing limits.

Move final omission construction after this pass so coverage and omissions describe the reconciled output. Do not recurse into `selectContextSourceOptions` or add a fallback loop.

- [ ] **Step 4: Format and run acceptance tests**

```bash
gofmt -w internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestReconcileContextSourceInventoryAddsRepresentedEvidenceOnly|TestBuildContextBalancesBroadReleaseEvidence|TestBuildContextProvesReleaseQualityWithoutPrivateRules' -count=1
go test ./internal/agent -count=1
git diff --check
```

The broad release regression and full agent package must now pass. If either remains red, stop and diagnose rather than changing limits or weakening the contract.

### Task 4D: Preserve profiled cross-project primary routes in inventory

**Files:**

- Modify: `internal/agent/context_proof.go`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Extends exact inventory candidacy with a narrow profiled cross-project route/API-endpoint exception for the required `primary_path` concern.

- [ ] **Step 1: Write the failing inventory-boundary test**

Add `TestContextEvidenceInventoryKeepsProfiledCrossProjectPrimaryRoute`. Build a full inventory with a caller entrypoint route, a profiled provider route/controller, a provider service symbol proving the same shared primary path, and invalid caller/unprofiled/test alternatives. Require only the profiled non-caller route/API endpoint to become a primary-path inventory candidate. When the file cap is full, appending inventory must replace a weaker nonmandatory related-service file with the provider route while retaining all already represented public areas and mandatory files.

- [ ] **Step 2: Verify the red state**

```bash
go test ./internal/agent -run TestContextEvidenceInventoryKeepsProfiledCrossProjectPrimaryRoute -count=1
```

Expected: FAIL because `contextEvidenceInventoryConcernIsExact` excludes every `primary_path` option.

- [ ] **Step 3: Implement the narrow candidacy exception**

Allow a required primary-path concern only when the option is profiled, production source-backed, has a safe canonical path, belongs to a non-empty project different from the selected entrypoint project, and contains an exact route, API-endpoint, or backend-handler fact. Do not admit caller-project routes, symbols/services, tests, pathless options, or unprofiled candidates. Retain the existing shared primary-path facet and deterministic option quality/path ordering; do not create a new public schema or concern.

- [ ] **Step 4: Format and verify**

```bash
gofmt -w internal/agent/context_proof.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestContextEvidenceInventoryKeepsProfiledCrossProjectPrimaryRoute|TestContextEvidenceInventoryBalancesPublicAreasBeforeRepeatedFacets|TestReconcileContextSourceInventoryAddsRepresentedEvidenceOnly|TestBuildContextBalancesBroadReleaseEvidence' -count=1
git diff --check
```

The broad release regression must pass. If another path is displaced, stop and diagnose rather than widening the exception.

### Task 4E: Restore non-exact agent selection compatibility

**Files:**

- Diagnose and minimally modify: `internal/agent/context_intent.go`, `internal/agent/context_select.go`, or `internal/agent/context_source.go` only as proven necessary.
- Test: existing `internal/agent/context_cross_service_test.go` and `internal/agent/context_source_test.go` regressions; add focused counterexamples only when needed.

- [ ] **Step 1: Capture the two existing red contracts**

```bash
go test ./internal/agent -run 'TestContextSourceProductionBeforeTests|TestContextSourceOptionsSelectProjectedClientEvidenceAndReportBudgetOmissions' -count=1 -v
```

Required failures to diagnose: an unscoped cross-service authentication concern remains uncovered, and projected client authentication is absent from both normal source/file selection and bounded budget omissions.

- [ ] **Step 2: Trace the exact regression boundary**

Compare public concern project binding, selected-client concern projection, authentication role gating, candidate fact IDs, option proof keys, source utility, and omission construction. Prove which Task 4A/4B rule changed each behavior. Do not assume the assertions are obsolete and do not enable exact-inventory intent for these ordinary queries.

- [ ] **Step 3: Implement the smallest generic compatibility fix**

Preserve the tightened credential-only caller-auth rejection and cross-project fact scoping. Restore real typed authentication evidence for unscoped and selected-client concerns, and ensure configuration/authentication/resilience siblings remain independently selectable or omittable under the existing file/token caps. Add negative counterexamples for foreign-project auth and same-project audit distractors if the existing tests do not already cover them.

- [ ] **Step 4: Run full agent acceptance**

```bash
gofmt -w internal/agent/context_intent.go internal/agent/context_select.go internal/agent/context_source.go internal/agent/context_cross_service_test.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestContextSourceProductionBeforeTests|TestContextSourceOptionsSelectProjectedClientEvidenceAndReportBudgetOmissions|TestBuildContextBalancesBroadReleaseEvidence|TestContextSourceConcernsRoleGateCredentialOnlyCallerAuthentication' -count=1
go test ./internal/agent -count=1
git diff --check
```

The focused set and complete agent package must pass before Task 5.

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

Preview and execute workspace clean, then scan-all and status. Require all three projects indexed and the approved source digest unchanged; exclude VCS metadata and generated GoreGraph outputs from that identity.

- [ ] **Step 5: Run and review matrix 2**

Use three baseline and three assisted runs with the retained prompt, identical model/reasoning/read-only settings, `--ignore-user-config`, and the same two disabled skill paths. Preserve completed poor runs. Require automatic gates plus assisted median manual quality at least baseline median; otherwise write a failure classification before the next focused fix.
