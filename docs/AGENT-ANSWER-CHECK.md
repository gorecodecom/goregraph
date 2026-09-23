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

Inspect `valid`, `citation_status`, `checked_ranges`, `findings` and `repairs` before
presenting the returned `answer`. `citation_status: no_line_citations_recognized`
means no source-line citation was checked, even when metadata-only file identities
make `valid` true. For a benchmark or workflow that requires source-line citations,
set `require_line_citations: true` in the request; zero recognized line citations
then produce a `no_line_citations` finding. A checked status covers only the
recognized syntax, not all prose. The checker also accepts an unambiguous file
reference followed later on the same non-table line by `Zeilen 10–12` or an
equivalent line marker. Multiple file references on one line are not bound to
a shared prose range. An indented list can also put one source file on a parent
bullet and its numeric ranges on child bullets; these are checked against the
parent file without widening the supplied ledger.

Findings identify answer lines and affected identities/ranges. Missing source
coverage must be resolved through an authorized read or an honest narrowing of
the claim, not by inventing a receipt. Unsupported citation syntax requires
clarification; a successful check is not exhaustive Markdown validation.

Redacted ledger ranges establish visible keys or structure, not hidden values.
`semantic_validity` remains `not_verified`: independently check that proposed
tests agree with selection predicates, isolation boundaries and failure
invariants. A delivered citation for a side effect also does not prove the
answer preserved its guard, skip conditions, or recipients; compare those
statements with the delivered source. In particular, a partial update cannot pass a rollback assertion
merely because the failure was observed.

For measurements, retain the raw response and trace, request ledger, checker
report and finalized response. Include checker and any correction-stage time and
model usage in end-to-end metrics; also report raw-run metrics separately. A
mechanically repaired answer must not be presented as the agent's original output.

See [the schema](../SCHEMA.md#answer-path-and-citation-validation) for limits and
exit codes. The ledger is caller-provided evidence, not authentication or new
source-read authority.
