# Checking an agent answer against delivered evidence

Use `goregraph answer-check` as an explicit finalization step after an agent has
produced Markdown. The command validates supported file/citation syntax and can
expand unambiguous path abbreviations. It does not automatically intercept Codex
answers or run another model.

An orchestrator records exact discovered paths and numbered source lines from
successful context, reader and authorized search outputs. Metadata-only files
establish identity; they do not establish source coverage. Do not populate delivery
ranges from receipts, match counts, skipped ranges or EOF metadata. If source
changes, invalidate or rebuild the affected evidence before checking the answer.

Save the original answer separately and supply a JSON request:

```json
{
  "answer": "See `Handler.java:10-12`.",
  "root": "/workspace",
  "files": [
    {"path": "service/src/Handler.java", "ranges": [[10,12]]},
    {"path": "service/src/test/HandlerTest.java"}
  ],
  "repair_paths": true
}
```

```sh
goregraph answer-check --request-file answer-request.json
```

The report expands `Handler.java` to the exact ledger path when there is only one
match. When several files share that name and the citation includes line ranges,
it also expands the path if exactly one candidate covers every cited range in the
supplied ledger. Metadata-only citations and ranges covered by several candidates
remain ambiguous and unchanged. It does not search the filesystem to guess a
replacement. A Markdown link can retain a short display label when its destination
is complete.

Inspect `valid`, `findings` and `repairs` before presenting the returned `answer`.
Findings identify answer lines and affected identities/ranges. Missing source
coverage must be resolved through an authorized read or an honest narrowing of
the claim, not by inventing a receipt. Unsupported citation syntax requires
clarification; a successful check is not exhaustive Markdown validation.

Redacted ledger ranges establish visible keys or structure, not hidden values.
`semantic_validity` remains `not_verified`: independently check that proposed
tests agree with selection predicates, isolation boundaries and failure
invariants. In particular, a partial update cannot pass a rollback assertion
merely because the failure was observed.

For measurements, retain the raw response and trace, request ledger, checker
report and finalized response. Include checker and any correction-stage time and
model usage in end-to-end metrics; also report raw-run metrics separately. A
mechanically repaired answer must not be presented as the agent's original output.

See [the schema](../SCHEMA.md#answer-path-and-citation-validation) for limits and
exit codes. The ledger is caller-provided evidence, not authentication or new
source-read authority.
