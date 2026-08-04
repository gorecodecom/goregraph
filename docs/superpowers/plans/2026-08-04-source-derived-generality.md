# Source-Derived Generality Implementation Plan

> **For Codex:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace private benchmark heuristics with conservative source-derived service and provider resolution, then prove transfer on an unrelated three-repository workspace before declaring 1.3.0 release-ready.

**Architecture:** A shared canonical identity helper derives exact service keys from types, imports, paths, and workspace registry records. Project scans retain neutral evidence and resolution keys; workspace reconciliation is the only layer allowed to turn those keys into concrete project or service ownership. Exact HTTP route evidence wins over name-derived resolution, and ambiguous or absent evidence remains unresolved.

**Tech Stack:** Go 1.24, standard library, existing GoreGraph scanner/agent/CLI packages, shell release checks

---

## Global constraints

- Follow strict red/green TDD for every behavioral change.
- Do not add dependencies.
- Do not copy private workspace source, prompts, names, routes, or expected answers into the repository.
- Preserve Schema 3 compatibility; new JSON fields must be additive and `omitempty`.
- Keep ordering deterministic and ambiguity explicit.
- Show each intended diff before applying it.
- Use English commit messages with a concise imperative subject and bullet body.
- Do not tag, release, or publish packages in this plan.

### Task 1: Add canonical service identity and unique workspace resolution

**Files:**
- Create: `internal/scan/service_identity.go`
- Create: `internal/scan/service_identity_test.go`
- Modify: `internal/scan/types.go`

**Step 1: Write the failing canonicalization tests**

Add table-driven tests for these exact cases:

```go
func TestCanonicalServiceIdentityVariants(t *testing.T) {
    tests := []struct {
        input string
        want  []string
    }{
        {input: "InventoryMgmtService", want: []string{"inventory"}},
        {input: "services/inventory-service", want: []string{"inventory"}},
        {input: "OrderCatalogClient", want: []string{"ordercatalog"}},
        {input: "order-catalog-api", want: []string{"ordercatalog"}},
        {input: "UserDetailsService", want: []string{"userdetail", "userdetails"}},
    }
    // Compare sorted unique variants.
}
```

Add registry-resolution tests proving:

```go
func TestResolveWorkspaceProjectByServiceKey(t *testing.T) {
    projects := []WorkspaceProjectRecord{
        {Name: "order-service", Service: "orders-api", Path: "services/order-service"},
        {Name: "inventory-service", Service: "inventory-service", Path: "services/inventory-service"},
    }
    project, candidates, ok := resolveWorkspaceProjectByServiceKey(projects, "InventoryMgmtService")
    // Assert services/inventory-service, one stable candidate path, and ok.
}
```

Also prove that `UserDetailsService` does not match `user-service`, and that two projects with the same canonical identity return `ok == false` with both sorted candidate paths.

**Step 2: Run the focused test and verify RED**

Run: `go test ./internal/scan -run 'TestCanonicalServiceIdentityVariants|TestResolveWorkspaceProjectByServiceKey' -count=1`

Expected: FAIL because the helpers do not exist.

**Step 3: Add additive contract metadata**

Extend `APIContractRecord` in `internal/scan/types.go` with:

```go
ServiceResolutionKey string `json:"service_resolution_key,omitempty"`
```

Do not rename or remove `ServiceCandidate`.

**Step 4: Implement the minimal identity helper**

In `internal/scan/service_identity.go`:

- split camel case before normalizing punctuation;
- split path, dot, dash, and underscore boundaries;
- lowercase tokens;
- remove only the wrapper tokens defined by the approved design;
- retain an original and conservative terminal singular variant;
- return sorted unique joined variants;
- collect project variants from `Service`, `Name`, and the base name of `Path`;
- resolve only when exactly one project ID matches;
- return stable candidate project paths for diagnostics;
- never use substring, edit-distance, or fixed aliases.

Keep the helpers unexported until another package needs them.

**Step 5: Run focused and package tests and verify GREEN**

Run:

```sh
gofmt -w internal/scan/service_identity.go internal/scan/service_identity_test.go internal/scan/types.go
go test ./internal/scan -run 'TestCanonicalServiceIdentityVariants|TestResolveWorkspaceProjectByServiceKey' -count=1
go test ./internal/scan -count=1
```

Expected: PASS.

**Step 6: Commit**

```sh
git add internal/scan/service_identity.go internal/scan/service_identity_test.go internal/scan/types.go
git commit -m "Resolve canonical workspace service identities

- derive exact service keys from source and registry names
- preserve ambiguous candidates without inventing ownership
- add optional contract resolution metadata"
```

### Task 2: Replace fixed Java dependency mappings with imported-type evidence

**Files:**
- Modify: `internal/scan/service_dependencies.go`
- Modify: `internal/scan/service_dependencies_test.go`
- Modify: `internal/scan/workspace_service_map.go`
- Modify: `internal/scan/workspace_service_map_test.go`

**Step 1: Write failing project-extraction tests**

Add a Java source fixture containing:

```java
import com.acme.platform.inventory.InventoryMgmtService;

final class OrderCancellationService {
    private final InventoryMgmtService inventory;
}
```

Assert the extracted dependency has:

```go
ResolutionKey == "inventory"
ToService == ""
ToProject == ""
Kind == "java_client_import"
```

Add negative cases for `java.util.ServiceLoader`, an imported local `OrderService`, and a type without a client-boundary suffix. Remove or rewrite tests whose expected value depends on a private fixed mapping.

**Step 2: Write failing workspace-resolution tests**

Add service-map tests proving:

- `InventoryMgmtService` resolves to the actual service identity of a unique `inventory-service` registry record;
- duplicate inventory identities add no dependency edge;
- `UserDetailsService` adds no edge to `user-service`;
- pre-existing explicit `ToProject` or `ToService` evidence still works.

**Step 3: Run focused tests and verify RED**

Run: `go test ./internal/scan -run 'TestBuildServiceDependencies|TestWorkspaceServiceMap.*Resolution' -count=1`

Expected: FAIL on current hardcoded behavior.

**Step 4: Implement neutral Java extraction**

Replace the fixed switch in `buildServiceDependencies` with source-derived extraction:

- parse imported Java type names;
- accept only imported type names ending with `Client`, `Service`, `Gateway`, `Connector`, or `Api`;
- require matching field or constructor evidence for that imported type;
- reject known Java/framework namespaces and types local to the scanned project;
- derive `ResolutionKey` via the shared helper;
- retain the raw import/type in `Evidence`;
- leave `ToProject` and `ToService` empty in project scope.

Do not add an organization or package-name allowlist.

**Step 5: Resolve dependencies only in workspace scope**

In `workspace_service_map.go`, before adding a dependency edge:

- honor explicit project/service evidence first;
- otherwise resolve `ResolutionKey` against the registry;
- fill the edge with the registry's real project ID and service name only for one unique match;
- skip zero or ambiguous matches;
- preserve deterministic edge ordering.

**Step 6: Run tests and verify GREEN**

Run:

```sh
gofmt -w internal/scan/service_dependencies.go internal/scan/service_dependencies_test.go internal/scan/workspace_service_map.go internal/scan/workspace_service_map_test.go
go test ./internal/scan -run 'TestBuildServiceDependencies|TestWorkspaceServiceMap.*Resolution' -count=1
go test ./internal/scan -count=1
```

Expected: PASS.

**Step 7: Commit**

```sh
git add internal/scan/service_dependencies.go internal/scan/service_dependencies_test.go internal/scan/workspace_service_map.go internal/scan/workspace_service_map_test.go
git commit -m "Derive Java service dependencies from imports

- emit neutral resolution keys instead of private service mappings
- resolve only unique workspace registry matches
- keep ambiguous and local types out of cross-project edges"
```

### Task 3: Generalize request extraction and provider ownership

**Files:**
- Modify: `internal/scan/api_contracts.go`
- Modify: `internal/scan/api_contracts_test.go`
- Modify: `internal/scan/contract_matches.go`
- Modify: `internal/scan/contract_matches_test.go`

**Step 1: Write failing generic request tests**

Add these fixtures:

```ts
transport.request("DELETE", "inventory/items/{id}")
gateway.request("GET", "health")
transport.request(method, "inventory/items/{id}")
transport.request("GET", "https://example.invalid/items")
```

Assert that the first two produce contracts independent of receiver name, the dynamic method is unsafe/unresolved according to existing semantics, and the absolute URL is rejected as a local API path. Assert project-scope records have an empty `ServiceCandidate` and a non-empty source-derived `ServiceResolutionKey`.

**Step 2: Write failing route-precedence and ambiguity tests**

Create workspace contract fixtures proving:

- an exact `DELETE /inventory/items/{id}` provider route wins even when its project name does not resemble `inventory`;
- if no exact route exists, a unique canonical project identity may resolve ownership;
- duplicate canonical identities stay ambiguous and do not select the first project;
- a placeholder route does not exactly match an unrelated fixed segment;
- candidate order is stable.

**Step 3: Run focused tests and verify RED**

Run: `go test ./internal/scan -run 'Test.*Request|Test.*Provider|Test.*Ambiguous.*Contract' -count=1`

Expected: FAIL because request extraction and provider fallback are receiver/domain specific.

**Step 4: Implement receiver-neutral request parsing**

In `api_contracts.go`:

- replace the receiver-specific request expression with a generic member-call expression;
- parse only supported literal HTTP methods;
- validate the second argument structurally;
- allow one static segment only in a recognized HTTP call;
- reject schemes, whitespace, and complex unresolved expressions;
- set `ServiceResolutionKey` from the first stable path segment;
- leave `ServiceCandidate` empty at project scan time;
- remove fixed business endpoint and service mapping tables.

Keep current `fetch` and method-specific client handling intact.

**Step 5: Implement workspace provider resolution**

In `contract_matches.go`:

- preserve exact method/path route matching as the primary source;
- report all sorted candidates on multiple matches;
- only when no provider route matches, resolve `ServiceResolutionKey` through the workspace registry;
- record the actual project/service identity only for a unique match;
- keep project-only results unresolved when ownership is not scanned.

Thread registry context through the narrowest existing workspace reconciliation boundary; do not add global state.

**Step 6: Run tests and verify GREEN**

Run:

```sh
gofmt -w internal/scan/api_contracts.go internal/scan/api_contracts_test.go internal/scan/contract_matches.go internal/scan/contract_matches_test.go
go test ./internal/scan -run 'Test.*Request|Test.*Provider|Test.*Ambiguous.*Contract' -count=1
go test ./internal/scan -count=1
```

Expected: PASS.

**Step 7: Commit**

```sh
git add internal/scan/api_contracts.go internal/scan/api_contracts_test.go internal/scan/contract_matches.go internal/scan/contract_matches_test.go
git commit -m "Resolve HTTP providers from generic request evidence

- extract request calls without receiver-specific adapters
- prefer exact provider routes over canonical project identities
- preserve unresolved and ambiguous ownership"
```

### Task 4: Remove private route, ranking, and dashboard branches and guard purity

**Files:**
- Modify: `internal/scan/contract_matches.go`
- Modify: `internal/scan/contract_matches_test.go`
- Modify: `internal/agent/context_rank.go`
- Modify: `internal/agent/context_rank_test.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_test.go`
- Create: `internal/scan/production_purity_test.go`

**Step 1: Add the failing production purity guard**

Create a test that locates the repository root with `runtime.Caller`, scans non-test `.go` and `.sh` files under `cmd/`, `internal/`, and `scripts/`, and fails with path and line for the audited private identifier set. Include the historical organization label, private dashboard label, fixed benchmark service prefixes, fixed route class, private noun aliases, and receiver-specific adapter name. Do not scan fixtures, tests, docs, `.git`, or `.worktrees`.

The forbidden set must use exact audited strings rather than broad generic terms such as `user`, `task`, or `regulation`.

**Step 2: Add focused behavior tests**

Add or update tests proving:

- owner-qualified and fully qualified Spring constants resolve from declarations in indexed Java source;
- an absent constant remains unresolved;
- a placeholder does not receive a fixed static substitution;
- generic `base_path`, `service`, `svc`, `api`, and `mgmt` prefixes retain structural handling;
- private business-noun translations are no longer injected into query tokens;
- generic delete/authentication/configuration/persistence intent aliases remain;
- diagnostic group labels are derived from existing family/route/project/status data only.

**Step 3: Run focused tests and verify RED**

Run:

```sh
go test ./internal/scan -run 'TestProductionPurity|Test.*Spring.*Constant|Test.*Placeholder|Test.*DiagnosticGroup' -count=1
go test ./internal/agent -run 'Test.*Query.*Alias|Test.*Intent.*Alias' -count=1
```

Expected: FAIL and list the current private production branches.

**Step 4: Remove private production behavior**

- Delete the fixed route-constant replacement map.
- Use the existing source-derived Spring constant index for owner-qualified and fully qualified constants.
- Delete fixed placeholder-to-static values.
- Replace fixed service-prefix entries with suffix/syntax-based recognition only.
- Remove private and organization-specific business-noun aliases while retaining generic technical/intent aliases.
- Delete the private dashboard route-family branch and use the canonical generic grouping path.
- Do not replace any removed rule with a renamed equivalent.

**Step 5: Run the guard and package tests and verify GREEN**

Run:

```sh
gofmt -w internal/scan/contract_matches.go internal/scan/contract_matches_test.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go internal/scan/production_purity_test.go internal/agent/context_rank.go internal/agent/context_rank_test.go
go test ./internal/scan -run 'TestProductionPurity|Test.*Spring.*Constant|Test.*Placeholder|Test.*DiagnosticGroup' -count=1
go test ./internal/agent -run 'Test.*Query.*Alias|Test.*Intent.*Alias' -count=1
go test ./internal/scan ./internal/agent -count=1
```

Expected: PASS with zero private production identifiers.

**Step 6: Audit production text explicitly**

Run:

```sh
rg -n -i 'weka|rdbv|cadaster|vorschrift|RegulationChangeBaseController|weka\.request|0442483' cmd internal scripts -g '*.go' -g '*.sh' -g '!**/*_test.go'
```

Expected: no output. If a generic false positive appears, narrow the audited expression only with written justification in the purity test.

**Step 7: Commit**

```sh
git add internal/scan/contract_matches.go internal/scan/contract_matches_test.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go internal/scan/production_purity_test.go internal/agent/context_rank.go internal/agent/context_rank_test.go
git commit -m "Remove private scanner and ranking heuristics

- rely on indexed constants and structural route evidence
- keep only generic intent aliases and dashboard grouping
- prevent private production identifiers from returning"
```

### Task 5: Add an unrelated three-repository acceptance workspace

**Files:**
- Create: `internal/cli/testdata/generality-workspace/.goregraph-workspace.yml`
- Create: `internal/cli/testdata/generality-workspace/order-service/pom.xml`
- Create: `internal/cli/testdata/generality-workspace/order-service/src/main/java/example/orders/OrderController.java`
- Create: `internal/cli/testdata/generality-workspace/order-service/src/main/java/example/orders/OrderCancellationService.java`
- Create: `internal/cli/testdata/generality-workspace/order-service/src/test/java/example/orders/OrderCancellationServiceTest.java`
- Create: `internal/cli/testdata/generality-workspace/inventory-service/pom.xml`
- Create: `internal/cli/testdata/generality-workspace/inventory-service/src/main/java/example/inventory/InventoryController.java`
- Create: `internal/cli/testdata/generality-workspace/inventory-service/src/main/java/example/inventory/ReservationCleanupService.java`
- Create: `internal/cli/testdata/generality-workspace/inventory-service/src/main/java/example/inventory/StockReservationRepository.java`
- Create: `internal/cli/testdata/generality-workspace/inventory-service/src/main/java/example/inventory/AllocationReservationRepository.java`
- Create: `internal/cli/testdata/generality-workspace/platform-clients/pom.xml`
- Create: `internal/cli/testdata/generality-workspace/platform-clients/src/main/java/example/clients/InventoryMgmtService.java`
- Create: `internal/cli/generality_acceptance_test.go`

**Step 1: Write the failing acceptance test before fixture implementation**

The test must copy the fixture into `t.TempDir()`, build the workspace through the public CLI entry point, and then request a Context Pack for:

```text
Trace cancellation from the public order endpoint to inventory reservation cleanup. Identify the missing client call, both persistence repositories, authentication and configuration evidence, side effects, and tests. Report uncertainty when source evidence is absent.
```

Assert:

- all three project indexes are current;
- workspace reconciliation succeeds;
- Doctor exits successfully;
- the pack uses one entrypoint and no fallback;
- the entrypoint is the public cancellation route;
- `inventory-service` and `platform-clients` appear through source-derived evidence;
- both reservation repositories, the client boundary, auth/configuration, persistence, side effects, and tests are represented;
- the missing order-to-inventory call is not falsely reported as an existing call;
- the pack is at most 4,000 estimated tokens and 12 files;
- repeated builds and packs are byte-for-byte deterministic after normalizing temporary absolute roots;
- no private audited term occurs in fixture or output.

**Step 2: Run the acceptance test and verify RED**

Run: `go test ./internal/cli -run TestSourceDerivedGeneralityWorkspace -count=1 -v`

Expected: FAIL because the fixture and/or generic relationships are incomplete.

**Step 3: Add the minimal unrelated fixture**

Implement source-only evidence with these relationships:

- `DELETE /orders/{orderId}` is the public entrypoint.
- `OrderCancellationService` contains the imported `InventoryMgmtService` boundary but intentionally omits the deletion call.
- `InventoryController` exposes the matching internal cleanup route.
- `ReservationCleanupService` deletes stock and allocation reservations and saves cleanup state.
- both repositories are explicit Spring Data-style interfaces.
- `InventoryMgmtService` contains the shared authenticated HTTP client contract and configuration property evidence.
- tests mention success, failure, retry, and side-effect expectations without implementing a production workaround.

Use fictional package and configuration names only.

**Step 4: Make the acceptance assertions pass without fixture-specific production code**

Adjust only generic selection/ranking weights or relationship plumbing if the Context Pack misses required evidence. Every adjustment must have a focused unit test using a second unrelated identifier. Do not test or branch on `order`, `inventory`, `stock`, `allocation`, or `platform` in production.

**Step 5: Run acceptance and regression tests and verify GREEN**

Run:

```sh
gofmt -w internal/cli/generality_acceptance_test.go
go test ./internal/cli -run TestSourceDerivedGeneralityWorkspace -count=1 -v
go test ./internal/scan ./internal/agent ./internal/cli -count=1
```

Expected: PASS.

**Step 6: Commit**

```sh
git add internal/cli/generality_acceptance_test.go internal/cli/testdata/generality-workspace
git add internal/agent internal/scan
git commit -m "Prove transfer on an unrelated workspace

- add a synthetic three-repository cross-service fixture
- verify bounded deterministic Context and Doctor behavior
- prevent fixture vocabulary from influencing production logic"
```

### Task 6: Align release documentation with evidence

**Files:**
- Modify: `README.md`
- Modify: `docs/RELEASE.md`
- Modify: `docs/BENCHMARKING.md`
- Modify: `docs/COMMANDS.md` only if command/help text changed
- Modify: generated documentation sources identified by `scripts/check-docs.sh`, if required

**Step 1: Write or update documentation assertions first**

Inspect `scripts/check-docs.sh` and existing documentation tests. Add assertions that:

- README describes source-derived exact/unique resolution and explicit ambiguity;
- language-depth claims remain generated from current capability metadata;
- historical token/time results are labeled as one frozen case, not a universal average;
- release instructions include the unrelated blind fixture gate;
- no documentation claims fuzzy, guaranteed, or organization-specific resolution.

**Step 2: Run documentation checks and verify RED**

Run the repository's documented docs synchronization/check commands, including `./scripts/check-docs.sh` when present.

Expected: FAIL until prose and generated sections agree.

**Step 3: Update source-of-truth documentation**

Document:

- exact source-derived service identity matching;
- route-first provider resolution;
- unresolved and ambiguous behavior;
- supported language depth only at the evidence level currently generated by the product;
- platform support as verified by cross-build/release checks, not as an unqualified runtime guarantee;
- benchmark measurements with candidate, case, run count, and scope.

Do not publish a cross-project token-saving average unless the checked-in evidence contains matched baseline and assisted samples sufficient to calculate it.

**Step 4: Synchronize and verify GREEN**

Run:

```sh
./scripts/check-docs.sh
go test ./internal/cli -run 'Test.*Help|Test.*Docs' -count=1
git diff --check
```

If a synchronization script has a different name, use the repository's source-of-truth command and record it in the final report.

Expected: PASS and no generated drift.

**Step 5: Commit**

```sh
git add README.md docs scripts
git commit -m "Document source-derived cross-service resolution

- describe exact route and identity evidence with ambiguity limits
- add the unrelated transfer gate to release validation
- scope benchmark and platform claims to verified evidence"
```

### Task 7: Run complete release gates, install, and rescan

**Files:**
- Modify only files required to fix failures revealed by the gates; use a separate focused commit for each logically independent fix.

**Step 1: Verify formatting, tests, race safety, and vet**

Run:

```sh
gofmt -l .
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
./scripts/check-docs.sh
```

Expected: all pass and `gofmt -l .` prints nothing.

**Step 2: Run benchmark and shell test suites**

Discover the exact checked-in benchmark test commands with:

```sh
rg -n 'benchmark|shellcheck|bats|goreleaser' Makefile Taskfile.yml scripts docs .github .gitlab-ci.yml
```

Run every repository-provided local benchmark harness and shell test. Do not send private data externally in this step.

Expected: all pass.

**Step 3: Cross-build the supported targets**

Build the CLI for:

```text
darwin/amd64
darwin/arm64
linux/amd64
linux/arm64
windows/amd64
```

Use isolated output paths under `t.TempDir()` or `/tmp`, and assert every binary exists and is non-empty.

Expected: all builds succeed.

**Step 4: Run the pinned GoReleaser snapshot**

Use the repository's pinned GoReleaser invocation without publishing. Verify archive names, platforms, checksums, and version metadata.

Expected: snapshot succeeds with all expected artifacts.

**Step 5: Verify the exact candidate tree and local installation**

- Require a clean worktree.
- Record `git rev-parse HEAD`.
- Install from that exact commit using the repository's supported local install command.
- Verify `goregraph version`, top-level help, workspace help, `build`, and `scan-all` short help against `docs/COMMANDS.md`.

Expected: installed binary reports the intended 1.3.0 candidate and help text includes required `--workspace` guidance wherever the positional root is insufficient.

**Step 6: Rebuild the blind workspace and historical workspace locally**

- Clean generated GoreGraph state only, never source or user files.
- Build and reconcile the unrelated acceptance workspace twice.
- Run Doctor and Context gates.
- Clean and rescan the historical G1 workspace using the exact installed candidate.
- Confirm one entrypoint, no fallback/retry, unchanged budgets, and no private rule dependency.

Expected: all local gates pass. If the historical result changes, report the source-derived difference instead of adding a name rule.

**Step 7: Re-audit the final diff and history**

Run:

```sh
git status --short
git diff origin/main...HEAD --check
git log --oneline --decorate origin/main..HEAD
rg -n -i 'weka|rdbv|cadaster|vorschrift|RegulationChangeBaseController|weka\.request|0442483' cmd internal scripts -g '*.go' -g '*.sh' -g '!**/*_test.go'
```

Expected: clean worktree, no whitespace errors, intentional commits only, and no audited production identifier.

**Step 8: Push the verified branch without releasing**

```sh
git push origin fix/release-benchmark-metrics
```

Record the exact commit, all gate results, installed version, blind-workspace result, historical local result, and remaining evidence limitations in the final report. Do not call 1.3.0 release-ready unless every local gate above passes.
