# Adaptive scope evidence — 2026-09-10

Continues the [previous evidence round](AGENT-EVIDENCE-FOLLOWUP-2026-09-10.md)
under the [scope evidence plan](superpowers/plans/2026-09-10-adaptive-scope-evidence.md).

## Trace diagnosis

The completed `evidence-followup-cli` run contains one GoreGraph request, three
bounded verification reads and 53 ordinary fallback commands. Fourteen commands
mention configuration, eleven persistence and six tests; these categories
overlap. The first ordinary fallback occurs at 61.78 seconds. The last command
response arrives at 503.96 seconds and completion at 754.95 seconds.

The 250.99 seconds after the last command cannot be attributed entirely to
retrieval. The event log does not split reasoning, generation and backend delay.
Similarly, 57 command executions are not 57 measured network round trips.

The first context lacks authentication, configuration and test concerns despite
the German query explicitly requesting them. Tokenization preserves German
linking forms and compounds, while the existing evidence vocabulary recognizes
their ordinary noun forms. Thus these requested candidates are missed before
budget selection. A diagnostic replay on the old installed binary with only
canonical evidence words appended restores the three concerns and selects the
internal management controller. That replay is a controlled diagnosis, not an
end-to-end benchmark result or a replacement for the user's original query.

Configuration resource navigation additionally requires explicit project names,
although the initial context has already selected projects. Existing change-plan
inventories deliberately require a change-plan request; those semantics remain
outside this fix. Ordinary fallback also rereads some supplied model ranges and
repeats equivalent negative searches.

## Implementation and validation

The round preserves strict-v1, the public query, domain identifiers, source-proof
requirements and existing budgets. It adds adaptive evidence vocabulary
recognition, bounded selected-project configuration navigation and clearer source
reuse guidance. Original queries determine project scope, seed ranking, actions
and domain identity; a separate evidence query determines requested concerns and
evidence relevance. This separation fixes the review-discovered collision where
an appended word such as `configuration` could otherwise become a project name.

New optional navigation uses remaining response space. If adding it makes the
final pack too large, resources are trimmed before repeating expensive source
selection. Explicit project requests and strict-v1 keep their previous behavior.
The selected source and verification ranges remain intact. A preliminary large
query rose from the stored 15.87 seconds to 106.76 seconds; the corrected isolated
replay takes 16.60 seconds. The preliminary candidate was not installed.

Synthetic regressions reproduce the missing concerns, project-name collision,
missing configuration identities and budget pressure. Focused tests pass; final
review has no open findings. The final candidate is built at 2026-09-10T16:08:27Z,
SHA-256 `99cb87374e85a84eb0b1c17ac3de89deac4b3775b4f6fb03c45056f1a3a1b929`.
Its isolated 18-query replay passes all 127 checks, including the latest query's
requested scope and primary mutation body. The complete Go suite, vet, formatting
and diff checks pass. All three historical strict controls remain unchanged.

The 1.4.1 development build is installed at `/Users/gorecode/go/bin/goregraph`
and `/opt/homebrew/bin/goregraph-local`, with both previous binaries backed up
under `scope-evidence-final`. The Homebrew symlink is preserved. All 374 frozen
service files and existing indices retain their hashes; no rescan or release was
needed. Verification after installation also passes.

The adaptive-only CLI run completed with the unchanged base task and settings.
The latency gate failed; this candidate is not an end-to-end performance win.

| Measurement | Saved no-GoreGraph | Previous adaptive | Scope evidence adaptive |
|---|---:|---:|---:|
| Task seconds | 600.30 | 754.95 | 895.69 |
| Uncached input + output tokens | 165839 | 105403 | 126234 |
| Command invocations | 23 | 57 | 36 |
| Core coverage rubric | 12/12 | 12/12 | 12/12 |

The new run takes 18.64% longer and uses 19.76% more effective tokens than the
previous adaptive run. Relative to the stored baseline, it saves 23.88% of these
tokens but takes 49.21% longer. These are individual runs, not causal estimates
or a repeated statistical sample. The baseline was not rerun.

All 37 cited file paths exist and all 57 numeric table ranges are in bounds.
Core claims match the stored reference and selected citations were checked against
source; this is not an exhaustive semantic audit of every range. The answer
omits two relevant mail-test files and initially overstates what the unknown
Oracle function deletes, although it explicitly acknowledges that uncertainty
later. Thus 12/12 core coverage does not mean a complete inventory or a verified
implementation. No service builds or tests were performed.

The reduced invocation count hides much larger outputs. A malformed generated
AWK batch puts its print action in a separate unconditional rule: it prints 2182
numbered lines from 11 complete files plus 597 duplicated requested lines. Six
subsequent commands reread 265 of those lines. A broad fallback search returns
1173 matches across 147 files. Those two responses total 366756 UTF-8 bytes,
exceeding all 270979 bytes returned by the previous run. Overall returned output
rises to 579355 bytes. This is concrete avoidable output, not proof that it caused
the entire latency increase. The separate conservative read parser misses some
batch shapes, so its lower-bound counts must not be compared as complete totals.

The last command completes at 601.02 seconds; another 294.67 seconds elapse before
the final answer. Events do not separate reasoning, answer generation and backend
delay. No stream disconnect was recorded; the known model-cache warning remains.
Post-run checks preserve the installed binary, all frozen sources and indices.

## Bounded-reader follow-up

The trace justifies one narrowly controlled follow-up: replace vague batching
advice with independently bounded per-file readers and a tested range/action
example. Discover filenames before printing broad source matches, starting with
known relevant scope and widening when needed. Preserve verification limits,
caller permissions, raw retrieval inputs and strict behavior. Test the reader
example on synthetic files; then freeze, install and measure one further adaptive
run. Keep the failed scope-evidence run as evidence. The instruction-only follow-up passes synthetic exact-range checks, existing
instruction/protocol consumers and root tests; scoped review has no findings.
Candidate `5f2ca6eca7240ed147467ad361b0fe1130c0dddd37ed9dc528dffeac1575ce6e`,
built 2026-09-10T16:42:38Z, is installed with backups. Only the adaptive guide
changed since the full-suite-tested candidate. Sources and indices still match.
The separate `scope-reader-cli` run completed; the latency gate remains failed.

| Measurement | Scope evidence | Bounded reader follow-up |
|---|---:|---:|
| Task seconds | 895.69 | 872.11 |
| Uncached input + output tokens | 126234 | 105833 |
| Input tokens, including cached | 2313065 | 1276303 |
| Cached input tokens | 2212224 | 1195520 |
| Output tokens, including reasoning | 25393 | 25050 |
| Command invocations | 36 | 21 |
| Returned command-output bytes | 579355 | 200776 |
| Last command response, seconds | 601.02 | 499.28 |
| Completion after last command, seconds | 294.67 | 372.83 |
| Core coverage rubric | 12/12 | 12/12 |
| Existing test files inventoried | 7 | 3 |

The follow-up reduces returned output by 65.35% and effective tokens by 16.16%
against the failed scope-evidence run, but elapsed time falls only 2.63% (23.58
seconds). Against the earlier adaptive run, effective tokens are almost equal
(+0.41%) and time is 15.52% longer. Against the saved no-GoreGraph reference,
effective tokens fall 36.18% while time rises 45.28%. Cached input is shown
separately; effective tokens are not all processed tokens or a monetary estimate.
These single-run observations do not isolate model variability or backend delay.
The base task/settings are unchanged, but the model-generated retrieval queries
naturally differ.

The tested per-file AWK pattern is used and the demonstrated full-file dispatch
bug does not recur. The exact-output checker verifies 43 readers across eight
pure batches and 1705 numbered lines. It finds 17 repeated lines; manual exact
comparisons in skipped mixed batches establish another 20. Thus at least 37
lines are reread. This is a lower bound, not a global repetition rate. The two
exit-1 commands end in ordinary no-match searches, with no recorded command-error
diagnostics. A scoped no-match search is not proof of workspace-wide absence.

The final answer cites 31 unique existing paths (61 distinct path/citation tokens)
and all 51 numeric table ranges are in bounds. Core diagnosis, both task families,
three project roles, local-versus-distributed consistency and concrete test
scenarios remain covered. However, its existing test inventory falls to three
files: public delete, management API and housekeeping. It omits existing individual
delete tests, mail tests and retry/mock reference patterns. Core 12/12 therefore
must not conceal this separate completeness regression. The executing reviewer
compares core claims with the saved reference and checks selected source ranges;
this is neither a blinded human review nor a complete semantic citation audit.

No stream disconnect was recorded; the known model-cache warning remains. All
374 frozen files, index hashes and installed binary/source hashes pass post-run
checks. No rescan, service changes, baseline rerun or release occurred.

## Remaining work

The instruction-only change addresses demonstrated output inflation; it does not
meet the end-to-end latency/completeness gate. The initial pack still has six
source sections and three verification requests, no production/test plan-file
inventory, and uncovered authentication, configuration, persistence and test
concerns. The generated `Wiederholungsverhalten` phrase also still fails to
request a resilience concern. Source fallback reconstructs the missing evidence,
and instruction-based reuse still permits repeated reads.

The next product work should target a reproducible missing-evidence selection
case and a reliable inventory of relevant existing tests, with source reuse
supported by explicit range data. Preserve proof and response budgets; do not
advertise broader completeness by merely weakening concern gates. First verify
those changes on local controls and the stored traces. Another full CLI run is
useful only after a concrete tested change; keep the saved baseline. The long
post-tool phase remains separately measured and cannot be explained away as
GoreGraph retrieval time.

## Measurement protocol

Reuse `local-update-baseline-cli`: 600.300493 seconds, 165839 uncached input plus
output tokens, 12/12 coverage criteria. Never rerun it for this fix. Preserve the
prior adaptive run as a separate comparison: 754.945442 seconds and 105403 tokens.
Only a completed new adaptive run with the same base task and settings can supply
new elapsed-time and usage claims. Compiler tests do not close the latency gate.

Private artifacts remain outside Git in the existing Mac benchmark directory:
`scope-evidence-round` contains the command audit, before-source snapshot and
preliminary controls; `scope-evidence-final` contains the reviewed candidate and
final controls; `scope-evidence-cli` contains the completed adaptive-only run and its audits.
The new CLI run uses only the installed final candidate. The saved no-GoreGraph
reference remains untouched.
