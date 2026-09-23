# Adaptive MCP comparison, September 2026

This Windows-host development measurement used a frozen three-service Java workspace,
normal Codex MCP access, `gpt-5.6-sol` at high reasoning, and effective tokens
(`input_tokens - cached_input_tokens + output_tokens`). It measures one static
diagnosis task, not general savings or runtime speed. Raw prompts, transcripts,
source ledgers, and scoring records remain outside the repository.

| Comparable pair | Without GoreGraph | With GoreGraph MCP | Effective-token saving | Static quality without / with |
| --- | ---: | ---: | ---: | ---: |
| 1 | 177,169 | 123,946 | 30.04% | 9/12 / 11/12 |
| 2 | 212,366 | 137,150 | 35.42% | 9/12 / 10/12 |

The two unchanged pairs together saved **32.97% effective tokens**. Both
control runs and both MCP runs used the regular available tools. The extra
control run (185,312 versus 103,762; 44.01%) required disabling GoreGraph MCP
in the control process after two earlier control attempts violated the no-GoreGraph
rule. Its tool availability differed, so it is excluded from the directly
comparable figure. Setup and invalid runs are excluded too.

The measured MCP binary was from commit `c597805`. A later local confirmation
of commit `2642c9b` used 138,555 effective tokens and scored 11/12 with all
seven required test-file identities. That single run does not establish a new
savings percentage. Subsequent answer-check and agent-instruction changes are
outside the measured series and need a fresh matched comparison before a new
performance claim.

The answers still had quality gaps. All primary answers missed the exact
condition for an existing deletion notification. Two assisted runs did not
receive the mail implementation's relevant source lines; two other assisted
runs did receive them but omitted the condition in the final text. That is a
mixed evidence-selection and answer-composition problem, not proof of one
specific retrieval defect. Several cited ranges also crossed undelivered source
lines. The source-line checker used for that series recognized only part of the
answer syntax, so its raw report alone must not be treated as exhaustive.
A later checker re-evaluation of the unchanged answers recognized 65, 73, and
51 line ranges in the three assisted main runs, 43 in the reminder diagnosis,
and 62 in the final confirmation run. The reminder ranges all had delivered
source coverage. The other runs still have cited gaps or ambiguous paths; this
re-evaluation does not repair their original answers.

Two subsequent normal-MCP quality-only runs used 144,257 and 130,286 effective
tokens. The first still omitted the deletion-mail condition despite receiving
both relevant source sections. After the finalization instruction was made
more explicit, the second answer stated the guard, the no-recipient skip, and
the optional CC condition, supported by delivered source. Its 73 recognized
line ranges still include three ambiguous or incomplete path citations. These
are individual quality observations without paired baselines, so they do not
change the 32.97% saving claim.
The older **52.08%** figure (165,839 without versus 79,464 with GoreGraph)
comes from a separate macOS 1.4.1 development experiment with a previously
saved reference, not these newly paired runs. Its model, build, timing and
protocol must remain attached to that historical result. See the
[original development follow-up](AGENT-READER-AUTOPAGE-2026-09-11.md).
Neither figure is a general guarantee, and the two series must not be pooled.
