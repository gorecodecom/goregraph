# Compact Plan-File Evidence Design

**Status:** Approved through the user's instruction to continue with the recommended release-readiness improvement on 2026-08-01

## Purpose

Close the remaining release-benchmark inventory gap without weakening causal source evidence. Exact cross-service change plans currently use the complete 12-file source identity budget for production flow, configuration, persistence, and one current test. Existing provider controller/service tests and caller-side mock/retry patterns can therefore disappear even though their indexed paths are useful for a production-ready test plan.

## Root cause

`files`, `source_sections`, and selected metadata share the hard 12-file limit. Raising that limit to 20 was tested against the historical workspace: it exposed more file names but displaced the Oracle deletion body and therefore weakened the cause analysis. Reordering the same 12 paths would trade one requested evidence area for another and repeat the selection oscillation seen in earlier matrices.

The missing paths have a different purpose from source evidence. They identify existing executable tests or test-support patterns that a change plan should reuse; they do not authorize source reads and do not prove future behavior.

## Considered approaches

1. **Raise `max-files` to 20.** Rejected because the fixed token budget then displaces causal source bodies.
2. **Re-rank the existing 12 files and three omissions.** Rejected because every slot already proves a requested production area and the result would remain unstable.
3. **Add a compact metadata-only plan-file projection.** Selected because it represents the missing test inventory without changing source permissions or public source limits.

## Design

An optional `plan_files` array is emitted only when a query:

- requests an exact production/test file inventory;
- plans a missing transition; and
- requests tests.

Each entry contains only `project`, normalized relative `path`, and `use`. Allowed uses are:

- `provider_test`: an existing provider controller or service test relevant to the requested internal change;
- `test_support`: an existing provider test-support class relevant to the requested service;
- `retry_pattern`: an existing caller-side retry test belonging to an outbound-client test pair;
- `mock_pattern`: the matching caller-side mock from that same pair.

The projection contains at most four entries. Every path must come from an exact-confidence indexed fact under a test source root. Provider entries must belong to a requested non-entrypoint project and match the query domain or internal-interface vocabulary. Caller pattern entries must form a deterministic pair after removing the `Mock` and `RetryableTest` suffixes; an isolated or unrelated file is not eligible. Paths already represented by `files`, `source_sections`, or bounded `source_omissions` are omitted.

`plan_files` is identity evidence only. It is not included in the 12-source-file count, is not a retry anchor, does not affect source coverage, and never authorizes reading. The generated agent instruction continues to permit reads only from `source_sections` and exact `source_omissions`. Change plans may name `plan_files` as existing tests or patterns, but must not claim their unrendered implementation details and must not invent a future filename.

## Budgeting and determinism

The existing final-decision reserve measures `plan_files` on the metadata probe before source selection. This reserves their compact serialized cost inside the unchanged 4,000-token and 16,000-byte defaults. Final source selection may therefore render slightly less optional source, but mandatory entrypoint and current-path evidence remain protected.

Selection is sorted by use priority, relevance score, project, path, line, and fact ID. If fewer than four entries fit or qualify, only the qualified prefix is emitted. Repeated builds over the same index and query remain byte-identical.

## Safety and compatibility

- Keep the 4,000-token, 12-source-file, 12-source-section, and three-omission limits.
- Add no dependency, retry, fallback, prompt exception, private identifier, or workspace-specific rule.
- Never expose source contents or configuration values through `plan_files`.
- Existing JSON consumers remain compatible because the field is optional and additive under Schema 3.
- Ordinary exploration, existing-flow, and non-inventory queries emit no `plan_files`.

## Verification

TDD uses generic Java/Spring-shaped facts for a caller, shared client, and provider. The failing tests require a provider management test, provider service test/support, and a paired caller mock/retry pattern while the normal 12-file source selection remains unchanged. Negative cases cover unmatched mocks, non-test paths, unrelated projects, non-exact facts, existing-flow requests, and already represented paths.

Renderer tests prove compact deterministic Markdown and explicitly label the section as non-readable metadata. Integration tests enforce the unchanged token, source-file, section, and omission bounds. Full Go, vet, benchmark harness, documentation-sync, installation, historical-workspace scan, and release-matrix gates remain mandatory.
