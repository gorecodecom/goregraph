# Local 1.4.1 acceptance

This candidate implements the scan reliability, output trust, dashboard journey
and opt-in agent protocol changes described in the improvement plan. It is for
local testing on `main`. Source commits are backed up on `main`; no release tag,
release or package update is published.

## Behavior to try

```powershell
goregraph version
goregraph workspace update <workspace> --workspace <workspace> --progress plain
goregraph context <project> --query '<coding task>' --protocol adaptive-v2
```

The first update of older indexes rebuilds them because they have no current
analyzer/input identities. Later unchanged updates preserve output contents and
modification times. Nested ignored generated bundles are excluded consistently
from builds, updates and freshness checks. Progress includes the active file or
phase, with heartbeats during longer operations. Ctrl-C is cooperative; a file
budget expiry records partial analysis, while project cancellation stops the
build and publication retains a recoverable previous result.

The local scan-all fix retries brief Windows publication locks for up to two
seconds per rename. `workspace build` and `workspace scan-all` now continue
through independent project failures and report every failed project with a
nonzero exit code. Workspace reconciliation is skipped on any project failure;
successful local indexes and the previous workspace output are retained.
Cancellation still stops the project loop.

The scan-all regression checks use real Windows file handles: a transient reader
no longer aborts publication, while a persistent reader returns an error with
the previous output intact. A workspace fixture confirms that later projects
are published after an earlier failure, cancellation stops further builds, and
failed runs retain the previous workspace output. Both complete affected Go
packages (`internal/outputstore`, `internal/cli`), scoped vet and documentation
synchronization passed after this fix.

The WEKA follow-up attempted all 43 projects and successfully published 42.
The remaining `frontend/frontends` publication is blocked by the running
`yarn workspace @wbp/local-dev dev` process: Windows handle inspection proved
that its Node watcher holds the staging directory and its `index`, `agent` and
`dashboard` subdirectories open, including for a synthetic 90-file output.
Its `services/local-dev/server.mjs` watch exclusions cover build output but not
GoreGraph output. Excluding `goregraph-out`, `.goregraph-workspace` and
`.goregraph-{stage,backup,journal,lock}-*` at the project root and restarting that
server is the targeted remaining remedy. The live frontend server and its
source configuration have not been changed. No complete WEKA reconciliation is
claimed while that project remains unindexed.

An unresolved route with an explicitly identified provider whose file analysis
was interrupted is labeled `provider_analysis_incomplete`. It does not prove a
missing backend implementation. Structural integrity, freshness and coverage
remain separate throughout Doctor, adaptive context and the dashboard.

`strict-v1` remains the default. The historical approximately 86% token-saving
case is preserved as historical evidence. No new end-to-end token-saving result
is claimed for `adaptive-v2`; broad natural-language retrieval and held-out task
effectiveness need independent outcome evidence.

The new development retrieval gate is **not met**: all 48 original bilingual
strict/adaptive requests returned low-confidence fallbacks without source
sections. See [the complete acceptance result](AGENT-DEVELOPMENT-ACCEPTANCE.md).
Known-entrypoint support and the historical case must not be generalized to
these broader tasks; improving that relevance remains unfinished work.

## Validation scope

Local Windows verification includes Go tests, vet, documentation synchronization,
scoped-ignore comparisons with Git, cancellation/source-change regressions,
transaction failure/restart/concurrent-read cases, and unchanged-update checks.
Two actual Playwright/Chromium tests cover keyboard investigation navigation,
narrow/zoomed rendering, offline loading, retained old-tab assets and missing
assets. See [dashboard acceptance](DASHBOARD-ACCEPTANCE.md).

The final `go test ./... -p 2 -count=1 -timeout 20m` run passed all 20 test
packages: 2,653 test/subtest pass events and no failures. It skipped 64 optional
or platform-dependent cases, including POSIX runner fixtures, symlink/Unix
permission cases and browser cases without the default Node module path. The
two new browser journeys were run separately with the installed Playwright and
Chromium paths and passed without skipping. These results do not constitute a
new external strict/adaptive agent outcome benchmark.

After the final concrete-fixture additions, both affected packages
(`internal/agentbench` and `scripts/agent-effectiveness`) were rerun in full and
passed. The installed Windows executable also passed an isolated synthetic build
and an adaptive named-type query without fallback; private indexes were untouched.

Linux amd64 and macOS arm64 binaries can be cross-compiled locally. Native tests
on those operating systems and race-enabled CI were not run in this Windows
session. A backup push to `main` triggers normal CI, not the tag-only release workflow.

The extraction benchmark is an extraction-only observation, not a complete-scan
or token claim. A profile did not justify a persistent fact cache or a speculative
storage/ranking rewrite. See [scan measurements](SCAN-BENCHMARKING.md) and
[agent evaluation methodology](AGENT-EFFECTIVENESS.md).

## Local installation and rollback

The local 09 September read-only correction opens existing reader lock files
without write access. Windows output and source path resolution now uses the
resolved file handle instead of listing ancestor directories with
`filepath.EvalSymlinks`; source containment checks remain in place. Writer
exclusion and interrupted-publication checks are unchanged. For copied or legacy
outputs, missing lock files (including optional output roots) must be initialized
once in a writable preparation step before entering a read-only sandbox. Readers
never ignore a failed lock to serve an unprotected snapshot.

The fix passed the full `internal/pathutil`, `internal/outputstore`,
`internal/agent`, and `internal/query` tests and targeted `go vet`. The installed
binary was checked with `codex sandbox -P :read-only` against the unchanged
three-project test workspace: source sections are readable without changing
source files or index generations. This is a deterministic tool check, not a
new agent effectiveness or token benchmark.

The subsequent local retrieval fix handles missing lexical anchors with a
bounded search of verified, already indexed mutation handlers. It matches query
terms against their current declaration bodies, without built-in business-domain
translations or index mutation. At most 64 handlers and 16 source files are
considered; unreadable, unrelated and tied matches are not promoted. Existing
metadata matches and explicit ambiguity results keep their previous behavior.
All three original German benchmark queries now return the expected entrypoint,
12 source sections across three projects, and no fallback in direct read-only
sandbox checks. Source coverage remains partial; only new agent runs and manual
quality review can establish effectiveness or token savings.

The validated binary is installed as a separate local Scoop version, with its
local build identity and documentation. The existing 1.4.0 directory and its
binary remain intact. Scoop's `current` link and command shim select the local
1.4.1. This does not upload a Scoop manifest or imply a public release.

To return to the preserved binary:

```powershell
scoop reset goregraph@1.4.0
goregraph version
```

The installation does not rebuild private workspace indexes. Before testing a
new scan against valuable existing output, keep a separate copy of that output
if a downgrade to the older binary and its matching snapshot is needed. Output
schema remains 3, but older clients do not understand new protocol/health fields.
