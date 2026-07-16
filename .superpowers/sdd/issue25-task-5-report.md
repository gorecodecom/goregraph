# Issue #25 Task 5 Report: HTTP Symbol Reachability

## Outcome

Task 5 adds deterministic, evidence-backed HTTP reachability records to the
canonical workspace symbol projection. API-mediated usages remain structurally
separate from direct references:

- `direct_reference` represents a verified code-level symbol reference.
- `reached_through_api` represents a uniquely resolved HTTP path.
- `ambiguous` represents multiple surviving provider or frontend-origin
  candidates.
- `unresolved` represents a resolved contract whose static path cannot be
  completed uniquely.

The workspace reconciliation merge point combines direct, API, ambiguous, and
unresolved records once, validates the complete projection, and only then
publishes `symbol-index.json` and `symbol-usages.json`.

## Implemented Path Contract

A verified HTTP usage carries `transport: "http"` and the ordered path:

1. `frontend_symbol`
2. `api_helper`
3. `http_contract`
4. `workspace_contract`
5. `spring_route`
6. `spring_handler`
7. `java_implementation`
8. `selected_symbol`

Positions are contiguous from zero. Available project, symbol ID, file, line,
and namespaced evidence IDs are preserved. The Spring handler and selected
implementation step remain separate even when they share a source file.

## Exactness and Ambiguity Rules

### Contract-to-flow join

The join uses the complete available call-site identity:

- frontend project
- backend project
- API file
- API line when present
- HTTP method
- HTTP path
- caller name when present

Candidate flows are sorted by stable identity before use. Discovery order does
not select a winner.

When more than one distinct flow survives:

- distinct canonical frontend origins produce `ambiguous` usages;
- every selected origin is disclosed through sorted candidate symbol IDs;
- every surviving flow is disclosed through sorted `candidate_path_ids`;
- unresolved origins, missing backend steps, and ambiguous Java providers retain
  the complete path candidate set independently of symbol resolution;
- identical-origin flow candidates remain `ambiguous` and are never promoted
  to Exact.

Every non-exact API usage carries the complete surviving
`candidate_path_ids` set even when exactly one flow survives. The
`feature_flow_join_ambiguous` limitation and multiple-flow reason are added
only when at least two distinct paths survive.

### Frontend origin

A JS/TS origin is selected only by:

- exact project;
- exact declaration file;
- exact component/caller name or export evidence.

Component evidence may come from an exact frontend step or the feature flow's
route-level component file and line. A declared component whose evidenced file
does not match is not substituted with another frontend step.

### Java provider

A Spring implementation step selects a Java declaration only by:

- exact backend project;
- exact declaration file;
- exact qualified or simple owner evidence.

No candidate produces `unresolved`; multiple candidates produce `ambiguous`;
one candidate produces `reached_through_api`.

## Non-Vanishing Uncertainty

Resolved contracts no longer disappear when required evidence is absent:

- no matching workspace feature flow:
  `workspace_feature_flow_missing`;
- no uniquely selectable frontend origin:
  `frontend_origin_unresolved`;
- no Spring implementation steps:
  `backend_implementation_steps_missing`;
- multiple surviving flows without distinct canonical origins:
  `feature_flow_join_ambiguous`.

These are emitted as unresolved or ambiguous usage records and are also
reflected in coverage where applicable. Surviving path candidates are retained
on every unresolved or ambiguous record, so deduplication cannot erase
independent flow alternatives.

## HTTP Reachability Coverage

`symbol-usages.json` now contains `http_reachability` coverage per supported
project/language.

Frontend JavaScript/TypeScript coverage is tied to:

- readable and present `flows.json`;
- a matching workspace feature flow;
- uniquely selectable frontend origin evidence;
- uniquely selectable feature-flow path evidence;
- exactly one indexed Java provider for every selected implementation step.

Backend Java coverage is tied to:

- readable and present `endpoint-flows.json`;
- non-empty Spring backend implementation steps;
- uniquely selectable frontend origin and feature-flow evidence;
- exactly one indexed Java provider for every selected implementation step.

Project languages are derived from canonical symbols, raw symbol/relation
facts, symbol capabilities, frontend route/flow facts, and Java project/Spring
facts. Coverage failures therefore remain visible when canonical symbols are
empty or malformed.

Unresolved usage languages use the same evidence-first approach: selected
consumer and helper symbols, frontend steps, API-file declarations, projected
API file extension, and finally project capability coverage. Exact `.js`/`.jsx`
and `.ts`/`.tsx` evidence therefore wins in mixed JavaScript/TypeScript
projects. JavaScript facts remain `javascript`; there is no TypeScript default.

Coverage meanings:

- `COMPLETE`: required inputs loaded and either verified paths exist or there
  are explicitly no resolved HTTP contracts for the project. A verified path
  requires one frontend origin, one path, and one Java provider.
- `PARTIAL`: inputs are missing/empty, a feature flow is absent, origin
  selection is unresolved/ambiguous, backend steps are missing, or Java
  provider selection has zero or multiple candidates.
- `FAILED`: a required reachability fact file is malformed or unreadable.

Limitations include:

- `flows_missing`
- `flows_empty`
- `flows_unreadable`
- `endpoint_flows_missing`
- `endpoint_flows_empty`
- `endpoint_flows_unreadable`
- `workspace_feature_flow_missing`
- `frontend_origin_unresolved`
- `feature_flow_join_ambiguous`
- `backend_implementation_steps_missing`
- `java_provider_unresolved`
- `java_provider_ambiguous`

This makes verified zero usage distinguishable from missing evidence.

## Strict TDD Evidence

### Initial full HTTP chain

RED:

```text
internal/scan/workspace_symbol_api_test.go:127:12:
undefined: BuildWorkspaceSymbolAPIUsages
```

GREEN:

```text
go test ./internal/scan \
  -run TestWorkspaceSymbolAPIUsageContainsFullFrontendToJavaChain \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Exact frontend origin

RED:

```text
consumer = "symbol:prepare-user", want exact frontend component
```

GREEN:

```text
go test ./internal/scan \
  -run TestWorkspaceSymbolAPIUsageStartsAtExactFrontendComponent \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Direct/API category separation

RED:

```text
undefined: mergeWorkspaceSymbolUsageRecords
```

GREEN:

```text
go test ./internal/scan \
  -run TestWorkspaceSymbolAPIUsagesRemainSeparateFromDirectReferences \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Full call-site identity and order independence

RED:

```text
call-site join ... selected "symbol:user-page"
instead of "symbol:user-details-page"
```

GREEN:

```text
go test ./internal/scan \
  -run 'TestWorkspaceSymbolAPIUsageJoinsFullCallSiteIdentityOrderIndependently|TestWorkspaceSymbolAPIUsageMarksMultipleSurvivingFlowsAmbiguous' \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Reachability coverage

RED:

```text
undefined: BuildWorkspaceSymbolAPIUsageCoverage
```

GREEN:

```text
go test ./internal/scan \
  -run 'TestWorkspaceSymbolAPIUsageCoverage|TestWorkspaceSymbolAPIUsageSurfacesMissingAndAmbiguousOrigins' \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Malformed reachability facts

RED:

```text
loadWorkspaceIndexes returned error instead of structured coverage failure:
unexpected end of JSON input
```

GREEN:

```text
go test ./internal/scan \
  -run TestLoadWorkspaceIndexesRecordsHTTPReachabilityFactFailures \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Signoff 3: candidate paths survive uncertainty

RED:

```text
unresolved origin usage had candidate_path_ids: nil
provider/flow ambiguity collapsed four records to two and omitted path IDs
missing backend-step usages had candidate_path_ids: nil
```

GREEN:

```text
go test ./internal/scan \
  -run 'TestWorkspaceSymbolAPIUsagePreservesAllCandidatePaths|TestWorkspaceSymbolAPIUsageKeepsSameOriginFlowCandidatesAmbiguous|TestWorkspaceSymbolAPIUsageMarksMultipleSurvivingFlowsAmbiguous' \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Signoff 3: unique Java provider completeness

RED:

```text
frontend/typescript HTTP coverage was COMPLETE when no indexed Java provider existed
backend/java coverage disappeared with the missing canonical provider symbol
```

GREEN:

```text
go test ./internal/scan \
  -run 'TestWorkspaceSymbolAPIUsageCoverageRequiresUniqueJavaProvider' \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Signoff 3: language derivation without canonical symbols

RED:

```text
missing coverage for frontend/app/typescript/http_reachability
in []scan.SymbolCoverageRecord(nil)
```

GREEN:

```text
go test ./internal/scan \
  -run 'TestWorkspaceSymbolAPIUsageCoverageDerivesLanguagesWithoutCanonicalSymbols' \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Final signoff: single surviving path on non-exact providers

RED:

```text
single-path unresolved Java provider usage had candidate_path_ids: nil
single-path ambiguous Java provider usages had candidate_path_ids: nil
```

GREEN:

```text
go test ./internal/scan \
  -run 'TestWorkspaceSymbolAPIUsagePreservesSingleCandidatePathForNonExactProviders' \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

The fixture also verifies that one surviving path does not add
`feature_flow_join_ambiguous` or a multiple-flow reason.

### Final signoff: unresolved JavaScript language

RED:

```text
missing-origin helper/API facts produced language "typescript"
project JavaScript capability produced language "typescript"
```

GREEN:

```text
go test ./internal/scan \
  -run 'TestWorkspaceSymbolAPIUsagePreservesJavaScriptLanguageWhenUnresolved' \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

### Mixed-language API extension precedence

RED:

```text
mixed JavaScript/TypeScript project coverage selected "javascript"
for the exact API file src/api/users.ts
```

GREEN:

```text
go test ./internal/scan \
  -run 'TestWorkspaceSymbolAPIUsagePreservesJavaScriptLanguageWhenUnresolved/API_extension_before_mixed_project_capability' \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan
```

## Regression Verification

The final source state was verified with:

```text
go test ./internal/scan -count=1
ok github.com/gorecodecom/goregraph/internal/scan 1.333s
```

```text
go test ./... -count=1
PASS: all packages
```

```text
go vet ./...
PASS
```

```text
gofmt -d <changed Go files>
PASS: no output
```

```text
git diff --check
PASS: no output
```

## Files

- `internal/scan/workspace_symbol_api.go`
- `internal/scan/workspace_symbol_api_test.go`
- `internal/scan/symbol_projection.go`
- `internal/scan/workspace_symbols.go`
- `internal/scan/workspace_symbols_test.go`
- `internal/scan/workspace_reconcile.go`
- `.superpowers/sdd/issue25-task-5-report.md`
