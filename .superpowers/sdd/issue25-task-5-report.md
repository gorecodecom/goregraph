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
- identical-origin flow candidates remain `ambiguous` and are never promoted
  to Exact.

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
reflected in coverage where applicable.

## HTTP Reachability Coverage

`symbol-usages.json` now contains `http_reachability` coverage per supported
project/language.

Frontend JavaScript/TypeScript coverage is tied to:

- readable and present `flows.json`;
- a matching workspace feature flow;
- uniquely selectable frontend origin evidence.

Backend Java coverage is tied to:

- readable and present `endpoint-flows.json`;
- non-empty Spring backend implementation steps.

Coverage meanings:

- `COMPLETE`: required inputs loaded and either verified paths exist or there
  are explicitly no resolved HTTP contracts for the project.
- `PARTIAL`: inputs are missing/empty, a feature flow is absent, origin
  selection is unresolved/ambiguous, or backend steps are missing.
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

## Regression Verification

The final source state was verified with:

```text
go test ./internal/scan \
  -run 'TestWorkspaceSymbolAPI|TestLoadWorkspaceIndexesRecordsHTTPReachabilityFactFailures|TestBuildWorkspaceEndpointTraces|TestWorkspaceFeatureFlow|TestCanonicalFeatureFlow' \
  -count=1
ok github.com/gorecodecom/goregraph/internal/scan 0.919s
```

```text
go test ./internal/scan -count=1
ok github.com/gorecodecom/goregraph/internal/scan 1.344s
```

```text
go test ./... -count=1
PASS: all packages
```

```text
go vet ./internal/scan
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
