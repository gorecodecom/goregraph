# Dashboard acceptance

This document records the bounded synthetic acceptance evidence for the offline workspace dashboard. It covers investigation continuity and projection health. It does not claim that static evidence proves runtime behavior.

## Health labels

The dashboard shows three independent health axes in the navigation panel:

| Axis | Values | Meaning |
| --- | --- | --- |
| Integrity | `valid`, `invalid`, `unavailable`, `unknown` | Whether the published projection passed structural validation. |
| Freshness | `current`, `stale`, `unknown` | Whether the projection inputs still match the selected generation. A timestamp never implies `current`. |
| Coverage | `complete`, `partial`, `unsupported`, `unknown` | Whether the available analyzers represented the requested scope. |

The last successful generation timestamp is displayed separately. Health reasons remain visible in plain language. Legacy service maps without a health record render all three axes as `unknown`; the dashboard does not infer a healthy state from their timestamp.

## Synthetic fixtures

The browser journey test constructs only synthetic records and expects these exact evidence boundaries:

| Fixture | Expected evidence | Expected uncertainty |
| --- | --- | --- |
| Resolved consumer/provider chain | `web/store` `ordersApi.cancel` at `src/ordersApi.ts:18` resolves to `services/orders` `OrderController.cancel` at `src/OrderController.java:42`. | Missing frontend-route, component, and persistence stages remain visible. |
| Ambiguous provider pair | The resolved `services/orders` implementation remains inspectable. | A second `services/orders-legacy` candidate is named and identified as not indexed. |
| Changed symbol with linked tests | `cancelPublishesAfterCommit`, `src/OrderControllerTest.java`, and `mvn -Dtest=OrderControllerTest#cancelPublishesAfterCommit test` remain attached to the selected feature. | Test linkage is static evidence with its recorded confidence. |
| Stale or missing projection | The dashboard labels the synthetic projection `valid`, `stale`, and `partial`, with `projection inputs changed` and `analysis incomplete` reasons. A missing usage shard reports that usage evidence is unavailable and recommends regenerating the dashboard. | A stale, partial, or missing projection is never presented as current or complete. |

## Bounded journeys

1. From **Feature Flow**, select the synthetic `DELETE /api/orders/{id}` flow, inspect its frontend caller and backend endpoint, then open the related endpoint trace. The trace exposes the provider implementation and source path.
2. Use **Back to previous investigation step** to return to the feature. The selected feature, `orders` query, `Resolved` filter, and keyboard focus return with it.
3. Inspect **Changing this may affect** for the direct consumer, dependent test, affected projects, and provider/index uncertainty. Inspect **Linked tests** and **Verification commands** for a bounded static test suggestion.

The same back stack is used by API Catalog to Endpoint, Diagnostic to Endpoint, Feature Flow to Endpoint or Data Flow, Data Flow to Endpoint, and Endpoint to Data Flow links. Choosing a top-level view starts a new investigation and clears that stack.

## Observed browser evidence

On 2026-09-09, the synthetic journeys ran in headless Chromium through Playwright with no unexpected console errors:

| Check | Observed result |
| --- | --- |
| Desktop, 1440 x 900 | Health, missing stages, source identities, linked tests, verification command, impact, and uncertainty rendered without overlap. |
| Narrow desktop, 900 x 800 | The existing stacked responsive layout kept health and feature evidence readable. |
| 200% zoom | Health and feature evidence reflowed into the stacked layout without horizontal page clipping. |
| Keyboard-only forward/back | Enter opened the related Endpoint, focused the journey-back control, returned to Feature Flow, and restored focus to the originating link. |
| Offline open and delayed asset | A `file://` dashboard showed the loading state while a project usage shard was delayed by 180 ms, then rendered its usage evidence. |
| Old tab across rebuild | An already-open synthetic dashboard loaded its original content-addressed shard after a new dashboard and shard were published beside it. |
| Missing asset | A new synthetic tab with its referenced shard removed displayed **Usage evidence unavailable** and a regeneration action. |

Screenshots are optional local test artifacts and are not committed. Set `GOREGRAPH_DASHBOARD_EVIDENCE_DIR` when running the browser tests to capture desktop, narrow, 200%, endpoint/back, retained-old-tab, and missing-shard screens.

The focused browser verification used the bundled Node modules and an existing local Chromium executable. Both tests ran and passed; they did not skip:

```powershell
$env:NODE_PATH = "$env:USERPROFILE\.cache\codex-runtimes\codex-primary-runtime\dependencies\node\node_modules"
$env:GOREGRAPH_PLAYWRIGHT_CHROMIUM = "$env:LOCALAPPDATA\ms-playwright\chromium-1228\chrome-win64\chrome.exe"
$env:GOREGRAPH_DASHBOARD_EVIDENCE_DIR = "$env:TEMP\goregraph-implementation-20260909\dashboard-evidence"
go test ./internal/scan -run 'TestWorkspaceDashboard(CompletesSyntheticInvestigationJourneys|KeepsOldOfflineTabsAndExplainsMissingShard)$' -count=1
```

The broader dashboard-focused Go contract suite passed in the repository's default Node environment:

```powershell
go test ./internal/scan -run 'WorkspaceDashboard' -count=1
```

That broader command did not have `playwright` on its default Node module path, so its optional Playwright cases skipped. The focused command above is the browser evidence for this acceptance record. A later attempt to expose the bundled Playwright module to every historical browser test found that its default browser revision was not installed; no browser download was performed.

## Performance scope

No new dashboard lookup bottleneck was observed in these journeys. Existing endpoint and symbol indexes remain map-backed, API Catalog derived filters remain cached, and symbol usage stays in project-specific lazy assets. There was therefore no speculative B4 performance rewrite and no invented before/after timing claim.

## Not executed

- No private project data, private WEKA data, or production dashboard output was opened for this acceptance run.
- No paid or external agent run, remote browser service, or network-backed dashboard dependency was used.
- No live runtime request was used to validate the static call, impact, or test relationships.
