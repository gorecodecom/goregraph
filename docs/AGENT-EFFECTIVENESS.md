# Agent effectiveness evaluation

This evaluation measures whether an agent completes a coding task correctly and how much end-to-end work that result costs. It is independent of GoreGraph confidence labels and output schema versions. The public development manifest has schema 1, and attempt evidence has its own schema 1.

The historical strict benchmark remains the release baseline. Its approximately 86% token saving is preserved as historical evidence and its existing gate is unchanged. The broader target of at least 25% lower median effective tokens is an additional generality goal for complete tasks; it does not replace, weaken, or re-score the historical gate.

## Public development tasks

[`testdata/agent-effectiveness/development/manifest.json`](../testdata/agent-effectiveness/development/manifest.json) contains 12 original synthetic tasks: four Java/Spring, four TypeScript/React, and four cross-project cases. Every task has equivalent German and English prompts without GoreGraph protocol hints, concrete required and forbidden outcomes, and a meaningful verification contract.

Each case now contains a schema-1 `fixture.json` with explicit project roots, languages, source-file paths, evidence paths, and commands for any executable checks. The project roots contain complete synthetic Java or TypeScript source instead of prose snippets. Cross-project roots include project-local `goregraph.yml` scanner configuration so workspace discovery is reproducible without relying on build-tool heuristics. Stale-source cases name separate indexed and live files: the evaluator temporarily indexes the saved bytes, restores the current source, and queries without rebuilding. This makes the snapshot/source contradiction reproducible.

Change tasks name a visible task test and a separate behavioral acceptance test. An evaluator must run acceptance tests from outside the agent's editable checkout, so editing or deleting a visible test cannot produce a successful result. Investigation tasks use a blinded source-evidence rubric because a build is not evidence that an explanation is correct. The deliberately unanswerable case succeeds only when the agent preserves uncertainty and identifies the missing provider evidence.

The six change fixtures use runnable checks with only local platform tools. Their initial visible checks pass while their evaluator-owned acceptance checks fail on the intended bug: audit ordering and deletion failure, configured header behavior, imported-call shadowing, exact generated-bundle exclusion, post-commit publication and rollback, and the live shipment contract with authorization. Java behavior is compiled and exercised with fake collaborators; TypeScript behavior uses Node's built-in test runner. The six investigation fixtures use full source evidence and rubric scoring rather than pretending a source explanation is an executable behavior test.

The development set is for evaluator development, regressions, and dry runs. It may be inspected by implementers and therefore cannot support a final efficacy claim. Do not add private benchmark names or private WEKA-derived material to production matching rules, fixtures, or documentation.

## Reserved evaluation set

A separate evaluator prepares eight held-out tasks across the same families. The implementer must not receive their answers. Before a final candidate run, the evaluator freezes prompt, source, visible-test, hidden-test, and rubric hashes together with the candidate identities, model, reasoning effort, tool configuration, permissions, timeout, and snapshot ID. Reserved contracts, answers, and raw evidence stay outside this repository. Only approved aggregate synthetic results and methodology may be committed.

For change tasks, run every treatment in an isolated disposable checkout and keep build/test permissions identical. Score the resulting code with acceptance tests held outside that checkout. For investigation tasks, use blinded evidence scoring. Retain timeouts, failed changes, corrections, fallbacks, and missing usage.

## Offline commands

The tool validates manifests and summarizes evidence already produced elsewhere. It never launches an agent, runs fixture code, changes an evaluation checkout, publishes results, or schedules paid work.

```text
go run ./scripts/agent-effectiveness validate --manifest testdata/agent-effectiveness/development/manifest.json
go run ./scripts/agent-effectiveness summarize --attempts C:\evidence\attempts.jsonl --output C:\evidence\summary.json
```

`validate` rejects unknown JSON fields, missing or duplicate task IDs, missing language prompts, empty outcomes or failure criteria, empty verification, mixed visible/acceptance checks, unsafe paths, missing project roots, and every missing source, stale-transition, evidence, or check file. It also requires fixture check names to match the manifest contract. Validation checks structure and file identity; it does not execute fixture code.

`summarize` reads one schema-1 attempt per JSONL line. Each attempt records case, language, treatment, repetition, source snapshot, candidate commit, completion and correctness, rubric score, duration, usage presence and counters, tool/source activity, retries, and corrections. Use treatments `ordinary`, `strict-v1`, and `adaptive-v2` for the planned matrix.

`usage_recorded` is required to distinguish absent usage from a measured zero. When it is false, all token counters must be absent or zero and the report increments `usage_missing`; it never inserts a zero-token observation. When it is true, the existing token parser validates the counters. Total tokens are input plus output. Effective tokens are uncached input plus output:

```text
effective_tokens = input_tokens - cached_input_tokens + output_tokens
```

`reasoning_output_tokens` is a subset of `output_tokens` and is reported separately; it is never added to total or effective tokens again. Cached input is subtracted once. Money must use a recorded pricing version and distinct cached and uncached rates; effective tokens are not billed cost.

## Pairing and gates

Attempts pair by case ID, language, and repetition. Every treatment in a pair must use the same snapshot, and a treatment can appear only once. The report retains every attempt in completion, failure, duration, and activity denominators. Token aggregates include only attempts with recorded usage and disclose the missing count.

Quality passes only when adaptive-v2 does not reduce the all-attempt completion rate or mean rubric score relative to ordinary workflow and does not increase critical incorrect completed edits. Efficiency is evaluated only on pairs where both ordinary and adaptive-v2 completed correctly. Their paired medians must show at least 25% lower effective tokens and 20% lower end-to-end task duration. The report always accompanies those successful-pair comparisons with all-attempt completion and failure rates, per-case regressions, and uncertainty. Missing treatment or pair evidence produces `not_evaluated`, not a pass.

`duration_milliseconds` is agent task time excluding separately recorded index setup/update work. Mark cold starts with `cold_start`; the report then exposes task plus setup/update time separately from pre-indexed task time. Record index refresh in `index_update_milliseconds` whenever a task snapshot evolves. The observed break-even task count is `(setup + update) / (ordinary mean task time - adaptive mean task time)` when the observed adaptive task time is lower. State whether every latency or token claim describes cold-start or pre-indexed use; a warm result is not a first-use result.

## External runs still required

First run a nine-attempt harness pilot: three representative development tasks, the three treatments, and one repetition. This checks isolation, evidence capture, scoring, and cost; it is not an efficacy result. Review its measured runtime and token cost before authorizing further work.

The proposed final matrix is eight held-out tasks × two languages × three repetitions × three treatments, or 144 paid attempts. This repository work does not authorize or perform those runs. Thresholds, prompts, hashes, and eligibility must be frozen before inspecting final results. Existing historical release gates remain additional mandatory evidence.
