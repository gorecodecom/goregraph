# Adaptive evidence follow-up — 2026-09-10

Implements the [improvement plan](superpowers/plans/2026-09-10-adaptive-evidence-followup.md)
following the [latency diagnosis](AGENT-LATENCY-FIX-2026-09-10.md).

## Corrected behavior

- Adaptive inferred model selection uses the terminal resource in declaration
  names. Package names and parent-only overlap no longer establish a model's
  relevance. Generic resource models remain eligible alongside parent-qualified
  variants. Singular/plural forms and unambiguous identifier abbreviations use
  general rules, without private domain aliases.
- Unrelated response/request/DTO/payload models no longer become required inferred
  models merely to fill another project's slot when concrete resource models
  exist. Explicit names, requested payload-type evidence and known primary-path
  dependencies remain eligible.
- The compiler reserves bounded follow-up metadata after selecting primary
  bodies and before filling optional source sections. It considers renderable
  evidence displaced by the response budget, not only unavailable evidence.
  Unaffordable candidates cannot hide a later affordable range.
- Verification priority matches the actual omission role and keeps compiler
  ordering among equal-priority adaptive candidates. Wholly supplied budget
  omissions are removed before the omission cap. Final adaptive admission tries
  candidates in priority order until three actually fit, rather than truncating
  the candidate list before checking fit.

Strict-v1 output and instructions, response limits, safe-path checks, operational
failure reporting and source-proof requirements remain intact. No runtime edge,
missing API or database cascade is inferred from model names. No dependencies
were added and no private service source was modified.

## Verification

Synthetic tests reproduced the failures before implementation. They cover nested
resources, unrelated package matches, generic and qualified model coexistence,
explicit payload requests, primary-path dependencies, ambiguous abbreviations,
role/project priority, saturated source budgets, preservation of primary bodies,
fully supplied ranges, and an affordable fourth omission behind three oversized
ones. Review caught the nested-resource and reserve/admission edge cases; their
regressions now pass. Final scoped review found no further issues.

Two preliminary candidates and their local controls are retained separately.
The final candidate passes all 17 fixed queries, 117 integrity/budget/range and
targeted evidence checks, the complete Go suite (`go test ./... -timeout 20m`),
`go vet ./...`, formatting and `git diff --check`. All three historical strict
JSON controls remain unchanged. Post-install verification also passes.

The actual-entry replays now provide the task model and change variant through
supplied source or bounded verification, without requiring unrelated cachet,
protocol or details-response models. This does not make the full task complete
from the initial pack: repository implementations, tests and some configuration
still require source fallback. The adaptive-only CLI measurement is complete.

## Measurement protocol

The no-GoreGraph reference is reused: `local-update-baseline-cli`, 600.300493
seconds, 165839 uncached input plus output tokens, 12/12 coverage criteria.
No baseline was rerun. One new adaptive-only CLI analysis completed with the
same frozen workspace, base task, model and reasoning setting. It uses the
canonical adaptive instruction, including the previously corrected retry syntax.

Raw artifacts remain outside Git under the existing Mac benchmark directory:
`evidence-followup-20260910` and `evidence-followup-reviewed` are preliminary;
`evidence-followup-final` is the final candidate; `evidence-followup-cli` holds
the adaptive-only run, measured usage, command trace, citation checks and quality
assessment.

## Completed measurement

| Metric | Saved no-GoreGraph reference | New adaptive run |
|---|---:|---:|
| Elapsed seconds | 600.300493 | 754.945442 |
| Input tokens | 2258257 | 671592 |
| Cached input tokens | 2108928 | 588544 |
| Output tokens | 16510 | 22355 |
| Uncached input plus output tokens | 165839 | 105403 |
| Total input plus output tokens | 2274767 | 693947 |
| First entrypoint evidence, seconds | 44.250668 | 21.609457 |
| Last command response, seconds | 370.796301 | 503.956980 |
| Command executions | 23 | 57 |
| Existing coverage criteria | 12/12 | 12/12 |

The new run uses **36.4% fewer uncached input plus output tokens**, but takes
**25.8% longer** than the saved reference. Reasoning output is included in output
tokens, not added again. These token counts are not a monetary cost calculation.
The latency acceptance gate remains open.

The run completed with exit 0 and one successful GoreGraph context call. It read
all three bounded verification requests before ordinary source fallback. Six
search commands returned no matches (exit 1); none exited with code 2 or above.
No backend stream disconnection is recorded; the CLI model-cache warning remains
in stderr. Earlier GoreGraph runs took 888.70 and 903.10 seconds but included
stream interruptions, so their elapsed differences do not isolate this fix.

All 33 inventory paths and their numeric source ranges exist and are in bounds.
The answer covers the same twelve criteria, including both task families,
project roles, existing authentication/configuration, side effects and concrete
regression cases. The executing reviewer assessed coverage; this is not a blind
human review or approval of a service implementation. Cross-service atomicity,
database behavior and the actual later UI symptom remain explicitly unresolved.

Remaining work is visible in the trace: the agent still searches for repository,
configuration and test evidence, and rereads some already supplied model ranges.
It also spends 250.99 seconds between the last command response and completion.
That interval cannot be attributed solely to GoreGraph or split reliably into
reasoning, output generation and backend overhead. Command counts are not network
round-trip counts. Further latency work should target the missing evidence and
repeated reads while retaining the unchanged task and quality rubric. One run
against an earlier baseline does not establish general performance.

Post-run verification again passes all 117 checks, including installed binary,
source and index integrity. The code fixes and installation are complete; a
faster complete task has not yet been demonstrated.

## Local installation

Installed 1.4.1 development build `d67d1f4ab3c3-dirty`, built
`2026-09-10T15:18:44Z`, at `/Users/gorecode/go/bin/goregraph` and
`/opt/homebrew/bin/goregraph-local`. SHA-256:
`c7e1ae18c590159ed2e8a99530cdfef409c2e07e720d7af0fb1b5287a276ec6f`.
Both prior binaries are backed up under `evidence-followup-final`; the Homebrew
symlink is preserved. All 374 frozen source files and existing indices retain
their hashes; no rescan or release was required.
