# Retrieval correction round — 2026-09-10

This follows [the Mac diagnosis](AGENT-FIX-DIAGNOSIS-2026-09-10.md).
The changes repair local retrieval and evidence-accounting defects. They do not
establish completed-task token savings or satisfy the broader efficacy gate.

## Changes

- Reserve adaptive health metadata before compiling fact metadata. Count concern
  metadata in UTF-8 JSON bytes, and reserve bounded verification requests alongside
  source omissions. An exhausted adaptive metadata budget returns a structured
  fallback rather than the previous intermediate-budget API error. Final token
  and byte checks remain active.
- Recognize generic German compounds for regression tests, test files, data
  variants, lookup attributes and service interfaces. Source-test gates use the
  same expanded vocabulary as concern planning.
- Require configuration fields, consumption or actual configuration composition;
  an annotation, empty class or unrelated `toString` return is insufficient.
  An unrelated client's configuration or retry evidence cannot satisfy an
  explicitly selected candidate. Configuration factories and imports remain valid.
- Discover model candidates without requiring project names. Variant families
  influence ranking without excluding other relevant projects. Newly inferred
  concrete models require their own identity evidence in addition to inherited
  fields. Candidate ownership remains explicitly unproven.
- Expand source candidates around related models to existing clients, providers,
  persistence, side effects and tests. Foreign-project candidates must also match
  the selected route's parent domain. Candidate groups are bounded and ranked
  before selection. This expansion happens during source planning, without adding
  inferred runtime edges to the established call-chain metadata.
- Report adaptive `insufficient_evidence` when requested concerns remain
  uncovered. Entrypoint confidence is preserved. Exact verification requests stay
  bounded, and ordinary source fallback requires the caller's existing authority.

Strict-v1 remains the default. The correction round initially preserved the
installed binaries and prepared workspace as its comparison baseline. The later
authorized installation and completed CLI comparison are recorded below; no
release was created.

## Regression coverage

New synthetic checks cover German compounds, Unicode allocation, adaptive
budgets from 256 to 6,000 tokens, verification metadata allocation, empty and
unrelated configuration declarations, cross-client exclusion, simple and compound
model names, singleton models beside variant families, inherited identities and
the incomplete-task fallback decision.

The separate review found and reproduced additional model-selection and false
configuration-proof cases. These became repository regressions and were fixed.
Two configuration-positive test fixtures now contain actual fields; the committed
benchmark matrix and private workspace were not weakened or changed.

The language-neutral selection test still checks selected facts, concerns,
projects and completeness. It permits render-mode differences caused by source
syntax and byte costs.

Final verification passed with Go 1.26.5 on macOS arm64:
`TMPDIR=/private/tmp GOCACHE=/private/tmp/goregraph-go-build go test ./... -timeout 20m`,
`go vet ./...`, and `git diff --check`. The committed benchmark matrix passes.
The final 15-control verification also passes, using GoreGraph's existing tested
line model, which retains an empty final line after a terminating newline.

## Acceptance limits

The saved control matrix compares 15 unchanged queries across strict/adaptive,
default/maximum budgets, neutral wording and explicitly named positive controls.
These are retrieval controls, not completed Codex tasks or comparable repeated
timing trials. Successful requests alone do not prove evidence completeness.

The candidate completes all 15 controls with zero API errors; the diagnosed build
failed eight. Adaptive incomplete packs retain their entrypoint confidence and
request fallback. Token/byte limits, source/verification range bounds, index
identity and all 374 prepared source-file hashes are checked independently.

The neutral case still needs ordinary source investigation for a complete
cross-service correction plan. The bounded pack can expose additional relevant
models and service extension points, but selection does not reliably include all
model, client/provider, deletion-side-effect and test evidence together. Block 4
of the diagnosis is therefore improved, not fully accepted. Higher budgets alone
do not resolve the remaining selection problem.

A fresh Codex CLI follow-up was prepared with the same base task, model,
reasoning level and arguments as the previous adaptive diagnostic, using an
isolated candidate executable. Automatic approval review rejected execution
because transmitting private workspace source to the Codex backend requires
additional explicit consent. At that stage the run had not started and no new end-to-end result existed.
Subsequent explicit consent enabled the installed comparison recorded below.
The original measurements remain separate.

Local raw controls, frozen candidate, source/index/binary integrity checks and
test logs are retained outside Git under the existing Mac benchmark artifact
directory. The first correction control run is `fix-round-accepted`; the subsequent adaptive CLI
follow-up is `fix-round-cli`.


## Follow-up: preserve the primary mutation

The early file inventory could consume every file slot before the verified
primary operation was rendered. The indexed call edge and readable method body
were present, but supporting configuration references prevented their selection.

Adaptive-v2 now frees supporting inventory slots for the entrypoint and first
local call when needed, using the existing inventory score and protected-file
rules. It counts the same union of paths as the public file limit, including
metadata-only entrypoints, contracts, persistence and tests. Reservation never
creates source coverage. Strict-v1 selection retains its existing contract.

The adaptive primary-path concern requires declaration bodies for those core
boundaries. A signature or entrypoint alone cannot prove the path. When the body
cannot fit, its exact omission takes priority over additional inventory gaps in
the bounded verification requests, and incomplete evidence requires fallback.

Three new regression functions cover saturated file slots, incomplete-path
proof/fallback, metadata-only path accounting and omission priority. Independent
review reproduced the original defect, caught the double-count case, and verified
that the final one-file response retains an exact mutation-body gap. With two
files, the mutation body is retained. No benchmark fixture was changed.

The frozen `fix-round-completeness-03` candidate passes all 15 unchanged Mac
controls and 80 independent integrity/budget/range checks. The three strict-v1
outputs equal the preceding candidate's JSON. The neutral adaptive request now
contains `CadasterRegulationOperationsService.deleteRegulationFromCadaster`
with its body at both 4,000 and 6,000 tokens. These packs use 3,853 and 5,676
estimated tokens respectively and still report incomplete evidence. Source,
index and installed-binary hashes are unchanged.

Validation: `go test ./... -timeout 20m`, `go vet ./...`, the focused regressions,
and `git diff --check` pass. Test logs and the frozen executable are saved with
the controls. This is a retrieval correctness improvement, not a completed-task
token/time comparison. The broader joint selection of models, provider/client,
side effects and tests remains open. The CLI consent block at this stage was
subsequently resolved, as recorded below.


## Authorized local installation

The user authorized updating the local installation, refreshing the test workspace
if needed, and starting the test runs. GoreGraph 1.4.1 with the current local fixes
was built with commit label `d67d1f4ab3c3-dirty` and timestamp
`2026-09-10T13:07:51Z`. Both `~/go/bin/goregraph` and
`/opt/homebrew/bin/goregraph-local` were atomically replaced after backing up the
previous executables. `/opt/homebrew/bin/goregraph` retains its existing symlink.
This is a local development build, not a release.

`workspace update --target agent --dry-run --no-update-gitignore` reports all
three services unchanged. The prepared source hashes still match, and the
existing schema/extractor/agent revisions are compatible. No rescan was needed;
the original source and index state is retained for comparison. Installation
records, binary backups and fresh local controls are in `local-update-20260910`.

The renewed Codex CLI launch was rejected again by automatic approval review:
authorizing test runs was not accepted as explicit consent for transmitting the
private workspace source/tests to the OpenAI Codex backend. That rejected attempt did not start. The user subsequently explicitly authorized
transmitting these private source files and tests to the OpenAI Codex backend;
both the adaptive CLI run and a fresh baseline with the same task, model and
reasoning settings subsequently completed successfully in sequential order.

The [installed comparison](AGENT-INSTALLED-COMPARISON-2026-09-10.md) records
133,299 uncached input plus output tokens in 15:14 with GoreGraph and source
fallback, versus 165,839 in 10:00 without GoreGraph. Both answers cover 12/12
review criteria. This single pair uses 19.6% fewer tokens under that metric but
takes 52.3% longer; it does not establish repeatable efficacy. All 80 integrity
and control checks pass after both runs. Cross-service pack completeness remains
open, since the final adaptive answer depends on ordinary source fallback.
