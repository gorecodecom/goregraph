# GoreGraph 1.0.0-rc.1 Contract Freeze Plan

**Goal:** Freeze Schema 2 plus the public CLI, Query, MCP, evidence, coverage, diagnostics, trace, and data-flow contracts for final 1.0 acceptance.

## Tasks

1. Bump generated manifests and schema-bearing workspace indexes to Schema 2; update contract tests and make old-schema Doctor failures provide explicit clean-rescan migration guidance.
2. Freeze documented command names, Query tasks, MCP tools, bounds, continuation, evidence semantics, and additive compatibility rules.
3. Verify every generated JSON file, evidence reference, stable ID, bounded agent response, deterministic repeat, and dashboard embedded schema.
4. Document migration from Schema 1 as local reinstall plus `workspace clean --execute` and `workspace scan-all`; no in-place mutation.
5. Set 1.0.0-rc.1, run tests/vet, install, clean/rescan WEKA, inspect Doctor/Query/MCP/dashboard statically, repeat hashes, and do not publish.
