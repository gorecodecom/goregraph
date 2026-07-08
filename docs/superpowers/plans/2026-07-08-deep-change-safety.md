# GoreGraph Deep Change Safety Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn GoreGraph from a strong code-map and impact tool into a deeper change-safety system that explains unresolved contracts, compares request/response shapes, links auth and persistence paths, and supports scan diffs.

**Architecture:** Keep the current scanner pipeline intact and add focused enrichment records beside existing outputs. Prefer deterministic extraction from Java/Spring, JS/TS API usage, tests, and workspace overlays; avoid speculative LLM-style inference. Each new capability must appear in JSON first, then Markdown reports, so the future website/code-map can consume stable structured data.

**Tech Stack:** Go scanner, Go tests, Java/Spring source extraction, JS/TS regex-based extraction, workspace overlay JSON/Markdown, `jq`-verified WEKA workspace scans.

## Global Constraints

- Stay on version `0.8.9` until explicitly told otherwise.
- Work TDD: failing test first, verify red, implement, verify green.
- No new dependencies unless a task proves the standard library is insufficient.
- Keep generated WEKA outputs out of commits.
- Do not push unless explicitly requested.
- Preserve existing output compatibility; add fields rather than renaming existing fields.
- Use stable IDs for any new record that will be useful in website deep links or scan diffs.

---

## Council Review Results

This plan was reviewed against the current GoreGraph codebase with five lenses: risk, first principles, upside, outside readability, and implementation practicality.

### Skeptic Finding

The main risk is building a second analysis layer beside the existing scanner model. GoreGraph already has `WorkspaceContractMatchRecord`, `WorkspaceFeatureFlowRecord`, `SpringEndpointRecord`, `TestMapRecord`, `SpringRepositoryRecord`, and `SpringEntityRecord`. New capabilities must extend these records or join them in workspace reconciliation instead of creating parallel truth sources.

### First-Principles Finding

The correct goal is not "make every unresolved contract disappear." The correct goal is "make change impact defensible." A genuine backend gap, a frontend method mismatch, or a dynamic endpoint can remain unresolved if the report tells the user exactly why, who likely owns it, which files are involved, and what changed since the last scan.

### Vision Finding

The largest product gain is not another percentage point of route matching. It is a compact feature dossier and scan diff that can answer: "What changed, what can break, who consumes it, what tests cover it, what auth/data path is involved?" That becomes directly useful for the website code map and for assistant-driven changes.

### Outside-Reader Finding

The current plan is technically strong but too implementation-heavy before it defines the user-facing artifact. A fresh reader needs an output contract first: which JSON/Markdown files exist, what questions each file answers, and which signals are hard evidence versus conservative hints.

### Maker Finding

The plan is implementable, but only if the first step creates fixture-based quality gates. Without a small stable workspace fixture and JSON assertions, repeated WEKA scans will be too slow and too noisy to guide TDD. WEKA scans should remain milestone validation, not the only feedback loop.

## Plan Corrections From Council

- Treat the remaining 4 open WEKA contracts as explainability targets, not necessarily as routes that must become `RESOLVED`.
- Add a baseline fixture and output contract before deeper extraction work.
- Extend existing record types first; introduce new files only for isolated concerns such as scan diffs or dossier rendering.
- Prioritize feature dossiers and scan diffs earlier because they convert existing data into practical value.
- Make every new signal carry `source`, `confidence`, and stable IDs where it can affect decisions.
- Keep Markdown concise; JSON remains the primary product surface for the future website/code-map.

---

## Milestone 0: Baseline Output Contract And Quality Gates

Before adding deeper analysis, freeze the expected behavior of the current strong state. This prevents later DTO/auth/diff work from regressing route matching, frontend flow extraction, and test matching.

### Task 0.1: Add A Minimal Workspace Fixture For Change-Safety Tests

**Files:**
- Add: `internal/scan/testdata/workspace_change_safety/frontend-app/...`
- Add: `internal/scan/testdata/workspace_change_safety/microservices/ms-cadaster/...`
- Add: `internal/scan/testdata/workspace_change_safety/microservices/ms-documentexport/...`
- Modify: `internal/scan/workspace_reconcile_test.go`

**Fixture must include:**
- one fully resolved frontend-to-backend contract
- one method mismatch with same backend path and different method
- one dynamic endpoint with two candidate values
- one missing neighbor route such as `/availability` next to `/export`
- one endpoint with request DTO, response DTO, auth annotation, repository call, entity, table, and matched backend test

- [ ] **Step 1: Write failing fixture test**

Create a test that scans/reconciles the fixture and asserts the current baseline:

```go
func TestWorkspaceChangeSafetyFixtureBaseline(t *testing.T) {
	// expected:
	// - resolved contract count >= 1
	// - one MISMATCH method conflict
	// - one UNRESOLVED dynamic endpoint with candidates
	// - one UNRESOLVED neighbor missing route
	// - frontend and backend flow steps are EXTRACTED
	// - matched endpoint test is MATCHED
}
```

- [ ] **Step 2: Verify red**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestWorkspaceChangeSafetyFixtureBaseline -count=1
```

- [ ] **Step 3: Add the smallest fixture files needed**

Use compact Java and TS/JS source snippets. Do not copy WEKA code.

- [ ] **Step 4: Verify green**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestWorkspaceChangeSafetyFixtureBaseline -count=1
```

### Task 0.2: Define The Public Output Contract

**Files:**
- Add or Modify: `docs/OUTPUTS.md`
- Modify: `docs/RELEASE.md`

**Document:**
- `.goregraph-workspace/context.json`
- `.goregraph-workspace/contract-matches.json`
- `.goregraph-workspace/feature-flows.json`
- planned `.goregraph-workspace/feature-dossiers.json`
- planned `.goregraph-workspace/workspace-diff.json`
- confidence semantics: `RESOLVED`, `MISMATCH`, `PARTIAL_MATCH`, `UNRESOLVED`, `OUT_OF_SCOPE`, `MATCHED`, `EXTRACTED`
- rule: fields may be added, existing field meanings must not silently change

- [ ] **Step 1: Write the doc first**

This is the reference for implementation and website consumption.

- [ ] **Step 2: Add a doc smoke test if existing test style supports it**

At minimum, include release notes that state the new output contract is additive.

---

## File Structure

- `internal/scan/types.go`: add new structured record fields for DTOs, auth, persistence, scan diffs, and feature dossiers.
- `internal/scan/extract_java.go`: extract Java DTO fields, Spring security annotations, query/status hints, repository/entity/table metadata already present or nearby.
- `internal/scan/api_contracts.go`: enrich frontend contracts with request-body and response-field usage where deterministic.
- `internal/scan/java_callgraph.go`: enrich test maps with status assertions and case classification beyond method-name heuristics.
- `internal/scan/workspace_reconcile.go`: join frontend contracts, backend endpoints, DTO metadata, auth, persistence, tests, and diff records into workspace outputs.
- `internal/scan/code_reports.go`, `internal/scan/report.go`, `internal/scan/diagnostics.go`: render concise Markdown sections for the new structured data.
- `internal/scan/workspace_diff.go`: new focused file for comparing two workspace output directories.
- `internal/scan/feature_dossier.go`: optional focused renderer/joiner only if `workspace_reconcile.go` would become less readable with dossier-specific logic.
- `internal/scan/*_test.go`: add narrow unit tests for each extractor/joiner plus at least one end-to-end workspace fixture.
- `docs/OUTPUTS.md`: public output contract and confidence semantics.
- `docs/RELEASE.md`: document feature checks for the 0.8.9 local release line.

---

## Milestone 1: Finish The Four Open Contracts

### Task 1: Add Contract Resolution Classification

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: `WorkspaceContractMatchRecord`
- Produces:
  - `ResolutionClass string json:"resolution_class,omitempty"`
  - `ResolutionEvidence []string json:"resolution_evidence,omitempty"`

- [ ] **Step 1: Write the failing test**

Add a test that verifies:

```go
func TestWorkspaceContractMatchesClassifyLikelyFrontendMethodBug(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-cadasterregulation", Kind: "backend", Service: "ms-cadasterregulation", Indexed: true}
	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{{
				HTTPMethod:       "PUT",
				Path:             "/cadasters/{cadasterId}/regulations/{objectId}",
				File:             "apps/vorschriftendienst/src/api/note.js",
				Line:             33,
				ServiceCandidate: "ms-cadaster",
			}},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{{
				Kind:       "backend",
				HTTPMethod: "GET",
				Path:       "/cadasters/{cadasterId}/regulations/{objectId}",
				Handler:    "CadasterRegulationMgmtController.getRegulationDetails",
				File:       "CadasterRegulationMgmtController.java",
				Line:       100,
			}},
		},
	})
	if matches[0].ResolutionClass != "method_conflict" {
		t.Fatalf("ResolutionClass = %q, want method_conflict: %#v", matches[0].ResolutionClass, matches[0])
	}
	if !containsString(matches[0].ResolutionEvidence, "same_path_backend_method=GET") {
		t.Fatalf("missing method evidence: %#v", matches[0])
	}
}
```

- [ ] **Step 2: Verify red**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestWorkspaceContractMatchesClassifyLikelyFrontendMethodBug -count=1
```

Expected: build failure or assertion failure because `ResolutionClass` does not exist.

- [ ] **Step 3: Implement minimal code**

Add fields to `WorkspaceContractMatchRecord`. In `workspaceContractMatch`, set:

```go
record.ResolutionClass = "method_conflict"
record.ResolutionEvidence = []string{
	"same_path_backend_method=" + route.route.HTTPMethod,
	"frontend_method=" + contract.HTTPMethod,
}
```

- [ ] **Step 4: Verify green**

Run the focused test, then:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'WorkspaceContractMatches' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/scan/types.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go
git commit -m "Add contract resolution classification"
```

### Task 2: Resolve Dynamic Endpoint Values From Frontend Constants More Precisely

**Files:**
- Modify: `internal/scan/api_contracts.go`
- Test: `internal/scan/scan_test.go`

**Interfaces:**
- Consumes: `APIContractRecord.DynamicEndpointCandidates`
- Produces: more precise candidates from local object literals, arrays, ternaries, and switch cases.

- [ ] **Step 1: Write failing tests**

Add tests for:

```go
const endpoints = {
  focus: "documents/focus",
  newest: "documents/new",
};
```

and:

```go
switch (type) {
  case "news": endpoint = "news"; break;
  case "topics": endpoint = "topics/firstletters"; break;
}
```

Expected candidates:

```go
[]string{"documents/focus", "documents/new", "news", "topics/firstletters"}
```

- [ ] **Step 2: Verify red**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestRunExtractsRealisticFrontendAPIContracts -count=1
```

- [ ] **Step 3: Implement minimal code**

Extend `dynamicEndpointCandidatesForLine` to collect:

```go
codeStringValueRE.FindAllStringSubmatch(...)
```

from the containing function body, but only include slash-containing endpoint fragments and explicitly allow single-segment values when the placeholder is the last path segment and the value matches a backend suffix later in workspace reconciliation.

- [ ] **Step 4: Verify with WEKA open case**

After install/scan, verify:

```bash
jq -r '.[] | select(.issue=="dynamic_endpoint_unresolved") | .dynamic_endpoint_candidates[]' /Users/gorecode/projects/weka/.goregraph-workspace/contract-matches.json
```

Expected: values include the currently listed backend-compatible suffixes.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/api_contracts.go internal/scan/scan_test.go
git commit -m "Improve dynamic endpoint candidate extraction"
```

### Task 3: Add Missing-Route Equivalence Hints

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Produces:
  - `EquivalentRouteCandidates []string json:"equivalent_route_candidates,omitempty"`
  - `MissingRouteKind string json:"missing_route_kind,omitempty"`

- [ ] **Step 1: Write failing test**

For `/documentexport/modules/{isbn}/documents/{objectId}/availability`, with backend routes `/export`, `/export/{exportId}/status`, `/export/{exportId}/download`, assert:

```go
matches[0].MissingRouteKind == "neighbor_resource"
containsString(matches[0].EquivalentRouteCandidates, "POST /modules/{isbn}/documents/{objectId}/export")
```

- [ ] **Step 2: Verify red**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestWorkspaceContractMatchesClassifyMissingIndexedBackendRoute -count=1
```

- [ ] **Step 3: Implement minimal code**

Use existing `similarWorkspaceRouteHints` and add a simple classifier:

```go
if samePrefixUntilPlaceholderDepth(contract.Path, route.Path, 4) {
	return "neighbor_resource"
}
```

- [ ] **Step 4: Verify green**

Run focused workspace reconciliation tests.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/scan/types.go
git commit -m "Add missing route equivalence hints"
```

---

## Milestone 2: Deep Request/Response Contract

### Task 4: Extract Java DTO Field Shapes

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/extract_java.go`
- Test: `internal/scan/rich_graph_test.go`

**Interfaces:**
- Produces:
  - `DTORecord{Name, File, Fields, Confidence}`
  - `DTOFieldRecord{Name, Type, Required, Source, Line}`

- [ ] **Step 1: Write failing test**

Fixture:

```java
public record CadasterCopyRequest(String name, boolean includeUsers) {}
public class UserUpdateRequest {
  @NotNull private String firstName;
  private String lastName;
}
```

Assert:

```go
assertHasDTOField(t, spring.DTOs, "CadasterCopyRequest", "name", "String", true)
assertHasDTOField(t, spring.DTOs, "UserUpdateRequest", "firstName", "String", true)
assertHasDTOField(t, spring.DTOs, "UserUpdateRequest", "lastName", "String", false)
```

- [ ] **Step 2: Verify red**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestRunExtractsWekaStyleSpringIntelligence -count=1
```

- [ ] **Step 3: Implement minimal parser**

Handle:
- Java records
- private fields
- `@NotNull`, `@NotBlank`, `@NotEmpty`

- [ ] **Step 4: Wire DTOs into `spring.json`**

Add `DTOs []DTORecord` to `SpringIndex`.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/types.go internal/scan/extract_java.go internal/scan/rich_graph_test.go
git commit -m "Extract Java DTO field shapes"
```

### Task 5: Attach Request DTO Shapes To Backend Endpoints

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/code_reports.go`
- Test: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: `SpringEndpointRecord.RequestType`, `SpringIndex.DTOs`
- Produces:
  - `BackendRequestFields []DTOFieldRecord json:"backend_request_fields,omitempty"`

- [ ] **Step 1: Write failing test**

Build a workspace feature flow where backend endpoint has `RequestType: "CadasterCopyRequest"` and DTO fields `name`, `includeUsers`.

Assert:

```go
len(flows[0].BackendRequestFields) == 2
```

and report includes:

```text
Request fields: `name` String required, `includeUsers` boolean required
```

- [ ] **Step 2: Verify red**

Run focused workspace feature flow test.

- [ ] **Step 3: Implement join**

Load DTOs from `spring.json` into `workspaceIndexProject`, then match by simple type name.

- [ ] **Step 4: Verify green**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'WorkspaceFeatureFlows.*Request' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/scan/types.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go
git commit -m "Attach request DTO fields to feature flows"
```

### Task 6: Extract Frontend Response Field Usage

**Files:**
- Modify: `internal/scan/api_contracts.go`
- Test: `internal/scan/scan_test.go`

**Interfaces:**
- Produces:
  - `ResponseFields []string json:"response_fields,omitempty"` on `APIContractRecord`

- [ ] **Step 1: Write failing test**

Fixture:

```ts
export async function loadUser() {
  const response = await GetHelper(dispatch, `/userservice/users/${userId}/info`);
  return {
    name: response.firstName,
    image: response.profile.imageUrl,
  };
}
```

Assert:

```go
assertHasAPIContractResponseFields(t, api, "GET", "/userservice/users/{userId}/info", "firstName", "profile.imageUrl")
```

- [ ] **Step 2: Verify red**

Run focused API contract test.

- [ ] **Step 3: Implement conservative extractor**

Only support fields read from the variable assigned from the API helper result in the same function body.

- [ ] **Step 4: Verify green**

Run API contract tests and full scan tests.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/api_contracts.go internal/scan/scan_test.go internal/scan/types.go
git commit -m "Extract frontend response field usage"
```

### Task 7: Add DTO-vs-Frontend Field Risk Signals

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/diagnostics.go`
- Test: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Produces:
  - `FieldRiskRecord{Kind, Field, Reason, Confidence}`
  - on feature flow: `FieldRisks []FieldRiskRecord`

- [ ] **Step 1: Write failing tests**

Cases:
- frontend uses `profile.imageUrl`, backend DTO has no `profile`
- backend returns `unusedField`, frontend never reads it

Expected:

```go
field_used_by_frontend_not_seen_in_backend
backend_field_not_seen_in_frontend
```

- [ ] **Step 2: Verify red**

Run focused test.

- [ ] **Step 3: Implement minimal comparison**

Compare exact field names first. Mark nested frontend fields as higher risk if backend has no first segment.

- [ ] **Step 4: Render diagnostics**

Add `Breaking-change risk` section.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/workspace_reconcile.go internal/scan/diagnostics.go internal/scan/workspace_reconcile_test.go
git commit -m "Add response field risk signals"
```

---

## Milestone 3: Auth, DB, E2E, Diff

### Task 8: Extract Backend Auth And Permission Context

**Files:**
- Modify: `internal/scan/extract_java.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/rich_graph_test.go`

**Interfaces:**
- Produces:
  - `AuthRecord{Kind, Expression, File, Line}`
  - `SpringEndpointRecord.Auth []AuthRecord`
  - `WorkspaceFeatureFlowRecord.Auth []AuthRecord`

- [ ] **Step 1: Write failing test**

Fixture:

```java
@PreAuthorize("hasAuthority('CADASTER_WRITE')")
@PutMapping("/{cadasterId}/state")
public ResponseEntity<?> updateState() {}
```

Assert endpoint auth contains `CADASTER_WRITE`.

- [ ] **Step 2: Verify red**

Run Spring intelligence test.

- [ ] **Step 3: Implement extraction**

Support:
- `@PreAuthorize`
- `@Secured`
- `@RolesAllowed`
- obvious method calls like `permissionService.check...`

- [ ] **Step 4: Render feature flow auth**

Markdown:

```text
Auth: PreAuthorize hasAuthority('CADASTER_WRITE')
```

- [ ] **Step 5: Commit**

```bash
git add internal/scan/extract_java.go internal/scan/types.go internal/scan/workspace_reconcile.go internal/scan/rich_graph_test.go
git commit -m "Extract backend auth context"
```

### Task 9: Add Persistence Path To Feature Flows

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/types.go`
- Test: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: existing `SpringRepositoryRecord`, `SpringEntityRecord`, endpoint backend steps.
- Produces:
  - `PersistencePath []PersistenceStepRecord`

- [ ] **Step 1: Write failing test**

Given backend steps include `CadasterRepository.save` and Spring index maps `CadasterRepository -> CadasterEntity -> VD_CADASTER`, assert:

```go
PersistencePath[0].Repository == "CadasterRepository"
PersistencePath[0].Entity == "CadasterEntity"
PersistencePath[0].Table == "VD_CADASTER"
```

- [ ] **Step 2: Verify red**

Run focused test.

- [ ] **Step 3: Implement join**

For each backend step where `Kind == "repository"`, join repository metadata to entity/table.

- [ ] **Step 4: Render report**

Add:

```text
Persistence: CadasterRepository -> CadasterEntity -> VD_CADASTER
```

- [ ] **Step 5: Commit**

```bash
git add internal/scan/types.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go
git commit -m "Add persistence path to feature flows"
```

### Task 10: Link Playwright/E2E Tests To Feature Flows

**Files:**
- Modify: `internal/scan/code_intelligence.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/scan_test.go`
- Test: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Produces:
  - `E2ETestRecord{File, Title, Route, Confidence}`
  - `WorkspaceFeatureFlowRecord.E2ETests []E2ETestRecord`

- [ ] **Step 1: Write failing extractor test**

Fixture:

```ts
test("3.1 Overview", async ({ page }) => {
  await page.goto("/portal");
  await page.getByRole("button", { name: "Export" }).click();
});
```

Assert route `/portal` and title `3.1 Overview`.

- [ ] **Step 2: Verify red**

Run scan test.

- [ ] **Step 3: Implement extraction**

Support:
- `test("...")`
- `page.goto("...")`
- `getByRole(...).click()` as action labels

- [ ] **Step 4: Workspace join**

Match E2E route to frontend route path/app and attach to feature flow.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/code_intelligence.go internal/scan/workspace_reconcile.go internal/scan/*_test.go
git commit -m "Link Playwright tests to feature flows"
```

### Task 11: Add Workspace Scan Diff Mode

**Files:**
- Create: `internal/scan/workspace_diff.go`
- Modify: `internal/cli/cli.go`
- Test: `internal/scan/workspace_diff_test.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- CLI:

```bash
goregraph workspace diff --before /path/to/old/.goregraph-workspace --after /path/to/new/.goregraph-workspace
```

- Produces:
  - `workspace-diff.json`
  - `workspace-diff.md`

- [ ] **Step 1: Write failing unit test**

Given two contract match arrays, assert:
- new contract detected
- removed contract detected
- confidence changed detected
- backend path changed detected

- [ ] **Step 2: Verify red**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestWorkspaceDiffDetectsContractChanges -count=1
```

- [ ] **Step 3: Implement diff engine**

Match by stable `id` first, then fallback key:

```go
api_project + api_file + api_line + api_http_method + api_path
```

- [ ] **Step 4: Add CLI command**

Add `workspace diff` to CLI parser and tests.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/workspace_diff.go internal/cli/cli.go internal/scan/workspace_diff_test.go internal/cli/cli_test.go
git commit -m "Add workspace scan diff command"
```

### Task 12: Generate Compact Feature Dossiers

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Create: `internal/scan/feature_dossier.go`
- Test: `internal/scan/feature_dossier_test.go`

**Interfaces:**
- Produces:
  - `.goregraph-workspace/feature-dossiers.json`
  - `.goregraph-workspace/feature-dossiers.md`

- [ ] **Step 1: Write failing test**

Given one feature flow with frontend route, API, backend, request, return type, tests, auth, persistence, assert dossier contains sections:

```text
Route
User Entry
API Contract
Backend
Request/Response
Tests
Auth
Persistence
Risks
```

- [ ] **Step 2: Verify red**

Run feature dossier test.

- [ ] **Step 3: Implement dossier builder**

Use existing `WorkspaceFeatureFlowRecord` and contract match records. Do not re-scan.

- [ ] **Step 4: Write outputs in reconcile**

Add files to workspace overlay and project overlay where relevant.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/feature_dossier.go internal/scan/workspace_reconcile.go internal/scan/feature_dossier_test.go
git commit -m "Add compact feature dossiers"
```

---

## Execution Validation

After every 2-3 tasks:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./...
```

After each milestone:

```bash
GOCACHE=/private/tmp/goregraph-gocache go build -o /private/tmp/goregraph ./cmd/goregraph
```

Then install locally and scan WEKA:

```bash
goregraph workspace clean . --execute
goregraph workspace scan-all .
```

Expected final checks:

```bash
jq -r '[.[]?.field_risks[]?.kind] | group_by(.)[] | "\(length) \(.[0])"' /Users/gorecode/projects/weka/.goregraph-workspace/feature-flows.json
jq -r '[.[]?.auth[]?.kind] | group_by(.)[] | "\(length) \(.[0])"' /Users/gorecode/projects/weka/.goregraph-workspace/feature-flows.json
jq -r '[.[]?.persistence_path[]?.table] | map(select(. != null and . != "")) | length' /Users/gorecode/projects/weka/.goregraph-workspace/feature-flows.json
```

---

## Recommended Order

1. Task 0.1
2. Task 0.2
3. Task 1
4. Task 2
5. Task 3
6. Task 11
7. Task 12
8. Task 4
9. Task 5
10. Task 6
11. Task 7
12. Task 8
13. Task 9
14. Task 10

Reason: lock the current strong behavior first, then improve explainability of the remaining open contracts, then add diff/dossier outputs so the existing data becomes more actionable. DTO, auth, persistence, and E2E depth come after the product surface is stable.

## Self-Review

- Spec coverage: covers unresolved contracts, dynamic endpoints, request/response, DTO field matching, test semantics, auth, persistence, feature dossiers, stable IDs, and scan diffs.
- Placeholder scan: no `TBD` or `TODO`; each task has concrete files, interfaces, tests, commands, and commit shape.
- Type consistency: new fields are introduced before downstream tasks consume them.
- Scope check: this is large but decomposed into independently testable tasks. If time is limited, execute Milestone 1 and Task 11 first for immediate utility.
