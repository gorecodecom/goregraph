# Storybook audit regression fixture

This synthetic source-only project tests GoreGraph's scanner and real JSON-RPC `task_context` entrypoint. It is not an application to install or run. No LLM, npm installation, Storybook/browser run, network access or real Weka index is involved.

```sh
go test ./internal/mcp -run 'Test(Storybook|CompleteAudit)' -v -count=1
```

The complete acceptance gate is mandatory in the normal suite. Both protocols use explicit `mode: "audit"`, 20 files and 6000 tokens. All seven independent groups in `expectations.json` must be delivered as actual source: configuration, disabled automatic A11y, component/story/fixture references, Chromium interaction runner, non-blocking CI integration, visual-test preparation and documented execution limits. The fixture contains 15 input files; 13 are required evidence. The unrelated production function and unreferenced CI file are negative controls.

Positive and negative oracle controls ensure that omissions do not count as delivered evidence, a complete label cannot hide missing source, the addon cannot replace the preview settings, and an unrelated CI file cannot replace the root include and referenced job. Changing the A11y activation or CI failure policy invalidates the original source expectation. Expected states in the report describe the fixture specification, not an automatically inferred runtime result.

The normal production workflow is independently tested in both protocols. Without audit mode, adaptive fallback can supply conventional Storybook configuration and one story with direct unambiguous declaration imports, bounded to five files; strict production-entrypoint requirements remain unchanged. Tests retain named-import aliases, missing/ambiguous targets, barrel exclusions in production fallback, changed sources, duplicates, named stories in later sentences and file/token limits.

Complete-audit boundary tests additionally cover the full Unicode query over CLI/MCP, minimum budgets, old metadata, nested/cyclic CI includes, stale CI links, external/dynamic/missing includes, 100 additional stories and a separate real-index multi-project deployment-trigger fixture. See [tooling audits](../../../../docs/TOOLING-AUDITS.md) for the exact scope and limits.

Optional evidence report (PowerShell):

```powershell
$env:GOREGRAPH_STORYBOOK_AUDIT_REPORT = Join-Path $PWD 'output/storybook-audit-report.json'
go test ./internal/mcp -run '^TestStorybookAuditContextEvidence$' -v -count=1
Remove-Item Env:GOREGRAPH_STORYBOOK_AUDIT_REPORT
```

The JSON report includes protocol, original query, source coverage, fallback reason, expected fixture states, missing paths, bounded omissions and independent fixture completeness. Global audit source coverage deliberately stays partial even when all seven fixture groups pass. Temporary projects are removed by Go test cleanup. Prepared visual tests are never represented as approved reference images or successful execution.
