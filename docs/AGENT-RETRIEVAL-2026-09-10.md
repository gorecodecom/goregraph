# Retrieval fixes and development rerun — 2026-09-10

The broader C3 efficacy gate remains unmet. This follow-up fixes reproducible
retrieval defects and repeats the 48 context requests from the 12 development
fixtures. It does not measure completed agent tasks or validate token savings.

## Implemented corrections

- Source planning retains distinct proving declarations in one file after first
  representing different files, within the existing eight-candidate planning cap.
- Exported ordinary JavaScript/TypeScript functions and their existing call edges
  enter the compact agent projection. Unrelated private functions remain excluded.
- Adaptive low-confidence fallback preserves bounded, verified candidate source
  instead of necessarily returning an empty pack. Lexical relevance is explicitly
  insufficient to prove a unique handler or runtime owner. Additional candidates
  can be supplied as bounded verification requests.
- Low relevance has its own `insufficient_relevance` code; it no longer implies
  ambiguous entrypoints or unsupported language analysis.
- New agent indexes retain per-file source hashes, scoped by project in workspaces.
  Adaptive queries check selected and concern-expanded files before duplicate
  suppression. Changed or missing source invalidates the old indexed answer.
- JavaScript/TypeScript test facts can resolve a literal `test`/`it` registration
  by its title. Comment-only, missing and ambiguous registrations are rejected.
  Uncertain JavaScript regular-expression boundaries are rejected conservatively.

Strict-v1 remains the default. Source-planning, exported-function and test-rendering
fixes also benefit strict requests; candidate fallback and snapshot conflict
handling belong to opt-in adaptive-v2. Existing agent projections should be rebuilt
to receive the new functions and hashes (agent build revision 2).

## Retrieval-only rerun

The original German and English prompts were used unchanged, with both protocols
and the default 4,000-token/12-file limits. Fixtures were copied to disposable
directories. The stale-controller case indexed the saved old source, restored the
provided current source and queried without rebuilding. No paid agents ran.

| Measure | strict-v1 | adaptive-v2 |
| --- | ---: | ---: |
| Requests | 24 | 24 |
| API errors | 0 | 0 |
| Packs with source sections | 1 | 17 |
| Fallbacks | 23 | 23 |
| Estimated tokens, minimum–maximum | 101–459 | 196–603 |

The baseline build in the same runner returned zero source-bearing packs in all
48 requests. Evidence counts measure retrieval, not answer correctness. Most
adaptive packs still explicitly require fallback and inspection.

The English stale-controller request now reports `evidence_conflict` and cites
the live source. The English configuration request includes redacted YAML and
client/properties declarations. The English focused-test request includes the
store and both relevant test registrations; additional source is bounded through
verification requests. The German variants remain uneven: seven German requests
return no source sections. No exact-name replacement prompts or case-specific
business vocabulary translations were introduced.

Local diagnostic packs and aggregate counts are retained under
`/private/tmp/goregraph-retrieval-acceptance/`; the disposable runner is
`/private/tmp/goregraph-evaluate-retrieval.py`. These are local diagnostics, not a
published benchmark dataset.

## Verification

On macOS arm64 with Go 1.26.5, the final
`TMPDIR=/private/tmp GOCACHE=/tmp/goregraph-go-build go test ./... -timeout 20m`
passed across all packages. The suite ran outside the sandbox so dashboard tests
could bind local HTTP servers. `go vet ./...` and `git diff --check` also passed.
The regression tests cover source selection, exports, test registrations,
configuration redaction, path containment, workspace hash scope, changed/deleted
source, bounded evidence, and duplicate identities including swapped file bodies.

## Remaining work

- Broad natural-language selection, especially German prompts against English
  identifiers, still needs better retrieval. Candidate bodies do not by themselves
  establish complete dependency chains, side-effect ordering or task completion.
- A subsequent six-run Mac CLI comparison found incomplete strict-v1
  client/provider/test coverage despite lower runtime and token consumption.
  The [diagnostic follow-up](AGENT-FIX-DIAGNOSIS-2026-09-10.md) identifies
  reproducible language, adaptive-budget and evidence-selection defects.
  Savings at equal completed-task quality remain unverified.
- The external `@wbp/local-dev` watcher described in the handoff is outside this
  repository; its restart configuration cannot be corrected here without its source.
- Runtime ownership and unsupported decorator expansion remain explicit evidence
  gaps when provider/deployment or generated source is absent.
