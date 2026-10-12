# Direct agent-guide A/B measurement, 2026-09-24

This experiment tested one proposed GoreGraph agent-guide instruction: locate
behavior-specific test methods first, read bounded setup/action/assertion ranges,
and avoid class-wide generic `assert|verify|status` searches unless a narrower
selector fails. The candidate was measured before deciding whether to ship it.
It was rejected and reverted.

The frozen task was to diagnose a cross-service cleanup defect in a private, before-fix Java workspace. All six runs used Codex CLI with
`gpt-5.6-sol` at high reasoning, the same prompt, and the normal GoreGraph MCP
workflow. The old binary was built from repository HEAD `8d981aa`; the new
binary was built from the candidate worktree. Run order was
old/new/new/old/old/new. Every attempt completed technically, called
`task_context` first, and left the source and index unchanged.

Effective tokens are input tokens minus cached input tokens plus output tokens.

| Pair | Old | New | New saving | Static quality old/new | Citation findings old/new |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1 | 164,437 | 128,974 | 21.57% | 11/12 / 12/12 | 7 of 59 / 2 of 95 checked ranges |
| 2 | 126,812 | 149,112 | -17.59% | 12/12 / 12/12 | 0 of 10 / 0 of 13 checked ranges |
| 3 | 127,615 | 135,726 | -6.36% | 12/12 / 10/12 | 1 of 57 / 6 of 128 checked ranges |
| Total | 418,864 | 413,812 | **1.21%** | 35/36 / 34/36 | Counts depend on answer style |

The candidate missed the predeclared decision threshold of approximately 5%
pooled token saving without worse answer quality. In new run 3, the answer
omitted the conditional notification rules and did not explicitly
assert both earlier-write restoration and unchanged later state in its failure
test. The mechanical citation checker also found six undelivered-range
citations in that answer. These checks do not prove semantic correctness,
and the number of recognized citations varies by answer style.

Both versions avoided broad generic test-source searches. The candidate made
six context calls and 40 reader calls, compared with four and 26 for the old
version. Its test-source line count did not materially fall. The intended
instruction therefore did not reliably reduce work; extra context and reader
calls are the next evidence-backed area to investigate, without relaxing
source coverage or answer quality.

The citation collector was applied uniformly to all six runs. It strips a
trailing `EXIT_CODE` marker from model-visible reader JSON because the old-2
runner appended it; before that correction, old-2 had seven false
undelivered-range findings.

A separate three-pair GoreGraph-versus-no-GoreGraph development comparison on
the same frozen diagnosis task saved 34.03% effective tokens, with 11/12
static criteria for each assisted run and 9/12 for each control. That
comparison cannot establish that this particular guide change helped. The
historical 52.08% macOS 1.4.1 result is another separate series. Do not pool
these percentages or treat them as a general guarantee.

No agent-guide change, release, local installation, MCP restart, or workspace
rescan resulted from this experiment. The private raw attempts and their
metrics, answers, quality reviews, source ledgers, and citation audits remain
in the original measurement workspace; this document carries the reviewable
decision across clients.
