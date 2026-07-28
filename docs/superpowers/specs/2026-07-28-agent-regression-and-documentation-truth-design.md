# Agent Regression and Documentation Truth Design

## Context

GoreGraph's connected-retry candidate produced a mechanically valid external
G1 smoke result after the agent instruction was corrected to show the executable
CLI form:

```text
goregraph context . --query "<focused query>"
```

The immediate pre-change Golden build used two Context calls, including a retry
against a disconnected housekeeping endpoint. The candidate rejected that retry
and used one Context call plus the three exact source omissions returned by the
first pack.

The retained diagnostic result was:

| Metric | Golden | Candidate |
| --- | ---: | ---: |
| End-to-end tokens | 162,916 | 67,392 |
| Tool calls | 6 | 4 |
| Context calls | 2 | 1 |
| Direct Context latency | 2,386 ms | 2,387 ms |
| Direct source reads | 0 | 3 |
| Included-source rereads | 0 | 0 |

The candidate reduced tokens by 58.6 percent and followed the intended bounded
workflow. However, the current analyzer classifies the three explicitly allowed
omission reads as ordinary source reads and raw navigation. That classification
prevents the monotonic regression gate from distinguishing protocol-compliant
evidence reads from unbounded fallback.

A separate audit found drift between the implemented language adapters and the
public documentation:

- `docs/OUTPUTS.md` describes Shell as having deep route, API, and test support,
  although the Shell adapter has no endpoint or test capability;
- the same document describes Rust as best-effort indexing, although Rust has a
  route, call, and test adapter;
- current documents disagree about whether the assisted agent instruction has
  nine or eleven lines;
- README and benchmark documents contain different obsolete diagnostic values;
- language capability tables are maintained manually and are not checked
  against the runtime analyzer registry;
- the implementation branch is two commits behind `main`, including a public
  README update for Codex MCP setup that must be preserved.

## Goals

1. Distinguish bounded omission reads from unauthorized source access without
   hiding their tool or token cost.
2. Keep the normal release benchmark's total-source-read reduction gate intact.
3. Make runtime language capability reporting and public capability claims use
   one code-owned truth model.
4. Correct unsupported or overstated documentation claims even when that makes
   GoreGraph appear less capable.
5. Improve Context Pack omission quality so agents are not directed to
   disconnected operational examples when a requested future transition is
   absent.
6. Keep every behavior change generic, deterministic, and free of private G1
   repository names or paths.
7. Integrate current `main`, preserve its public documentation additions, and
   leave the final release candidate fully tested and benchmarkable.

## Non-goals

- Do not rewrite historical specifications or implementation plans to match the
  current product. They remain dated engineering records.
- Do not generate the entire README or command manual.
- Do not claim runtime completeness from static pattern detection.
- Do not weaken included-source reread, broad-navigation, tool-call, token, or
  answer-quality gates.
- Do not add language analyzers merely to preserve an existing documentation
  claim.
- Do not release, tag, or merge to `main` as part of an individual hypothesis.

## Design principles

Each behavior change is an independent hypothesis, implementation commit, and
verification step:

1. classify bounded omission reads;
2. establish the runtime language capability contract and synchronize current
   documentation;
3. improve Context Pack omission selection.

The next hypothesis starts only after the preceding change passes its focused
tests, the complete local test suite, and deterministic agent benchmark cases.
External G1 evidence is collected only at the explicit smoke or full-benchmark
gate, not after every code edit.

## Bounded omission read classification

### Metrics

The transcript analyzer retains the existing total counts and adds:

- `bounded_omission_read_calls`: terminal tool calls whose source targets are
  entirely contained in concrete omissions from an earlier Context Pack;
- `unauthorized_source_read_calls`: source-reading terminal tool calls that are
  not bounded omission reads and are not Context calls.

`source_read_calls` remains the total number of direct source-reading tool
calls. `raw_navigation_calls`, tool calls, tokens, Context calls, repeated full
packs, and included-source rereads remain visible and are not reduced by the new
classification.

### Allowed omission read

A terminal tool call is a bounded omission read only when all of these
conditions hold:

1. a valid full Context Pack was returned earlier in the same transcript;
2. the pack contains an omission with a non-empty project and path plus positive
   `start_line` and `end_line`;
3. every source target in the command resolves to that omission path;
4. every requested line range is contained within the omission range;
5. the command does not search, inventory, or read another source target;
6. the command does not overlap a range already present in `source_sections`;
7. the terminal tool call occurs after the pack that authorized it.

A subset of an omission range is allowed. A wider range is unauthorized.
Pathless or unbounded omissions grant no source-read permission. A compound
command is bounded only when every source-reading segment is bounded; one
additional or unresolved source target makes the complete terminal call
unauthorized.

### Gates

The matched no-GoreGraph versus assisted release benchmark continues to apply
its existing threshold to total `source_read_calls`.

The Golden-versus-candidate monotonic regression gate:

- compares median `unauthorized_source_read_calls` and rejects an increase;
- limits `bounded_omission_read_calls` to the evaluated contract's
  `max_source_omissions`;
- continues comparing total tool calls, end-to-end tokens, and Context latency;
- continues rejecting included-source rereads and repeated full packs;
- reports total, bounded, and unauthorized source reads together.

This keeps permitted evidence reads measurable while preventing them from being
misreported as broad fallback.

## Runtime language capability contract

### Single source of truth

The scanner gains one code-owned language capability profile used by:

- analyzer inventory;
- generated project and workspace coverage;
- agent-index coverage records;
- dashboard coverage;
- documentation table rendering and drift checks.

Each language profile records:

- adapter level: `full`, `partial`, `index`, or `unavailable`;
- symbols;
- imports and relations;
- calls;
- routes;
- tests;
- API clients;
- persistence;
- messaging and RPC;
- request-to-response data flow;
- exact workspace symbols;
- direct usages;
- HTTP provider reachability;
- HTTP consumer reachability;
- known framework or pattern families;
- limitations that must be stated publicly.

The existing `AnalyzerRecord` booleans remain compatible with generated output,
but they are derived from or validated against the richer profile instead of
being a separate public truth.

### Evidence rule

A capability may be documented as full only when:

1. current production extraction emits normalized file-and-line-backed facts;
2. representative tests prove the extraction and output path;
3. the public description names material static-analysis limitations.

Pattern-backed support is described as pattern-backed. A full adapter does not
mean arbitrary framework, reflection, metaprogramming, runtime configuration,
or dynamic dispatch support.

If production code and tests do not support a current claim, the runtime
capability and documentation are downgraded. Implementation is not expanded
solely to protect marketing wording.

## Context Pack omission quality

### Problem

When the requested future cross-service transition does not exist, lexical
similarity can cause a disconnected housekeeping or maintenance endpoint to
become a source omission. The agent is then invited to read operationally
unrelated code even though the missing transition itself is valid evidence of
the current defect.

### Selection rules

For requested missing transitions:

1. retain the reliable current production entrypoint and reachable call chain;
2. treat the absent future call or contract as an explicit gap, not a fallback
   trigger;
3. do not publish a disconnected maintenance, housekeeping, batch, or cleanup
   endpoint as a call-chain omission unless the query explicitly requests that
   operation;
4. prefer action-aligned domain-model and persistence evidence matching the
   requested models;
5. prefer concise repository or entity declarations over an unrelated endpoint
   body when they cover the requested evidence facet;
6. preserve at most three path-and-line-bounded omissions;
7. leave unavailable future API shape, configuration, compensation, and retry
   behavior as explicit uncertainty.

Disconnected explicitly named projects may still contribute related production
evidence. They must not be fabricated into the current call chain.

### Generalization

Production rules and tests use generic catalog/job or regulation/task fixtures.
No private workspace names, paths, method names, or expected G1 answer strings
are added to the repository.

## Documentation synchronization

### Generated factual blocks

Only drift-prone factual blocks are rendered from code:

- language and framework coverage;
- capability depth;
- exact symbol and usage support;
- HTTP provider and consumer reachability;
- canonical agent instruction;
- current benchmark metric names and gate terminology;
- current version and schema facts where a code-owned value exists.

Narrative documentation remains hand-written.

### Synchronization command

A dependency-free repository command supports:

- `--write`: replace marked factual blocks with canonical rendering;
- `--check`: perform no writes and fail when a block differs.

The check runs in the regular test or pre-release workflow. It must produce a
concise file-and-block mismatch rather than silently rewriting files.

### Audit scope

The implementation audits and synchronizes:

- `README.md`;
- `COMMANDS.md`;
- `SCHEMA.md`;
- `docs/OUTPUTS.md`;
- `docs/BENCHMARKING.md`;
- `docs/RELEASE.md`;
- CLI help;
- MCP server instructions;
- generated agent guide text;
- release workflow, package-manager, version, and schema references.

Historical files under `docs/superpowers/specs/` and
`docs/superpowers/plans/` are excluded from current-product synchronization.

The audit explicitly resolves:

- Shell and Rust capability drift;
- nine-line versus eleven-line instruction wording;
- stale diagnostic benchmark values;
- obsolete CLI syntax;
- version and schema mismatches;
- differences between the command reference and current progressive CLI help;
- the two newer `main` README commits, including Codex MCP setup.

## Integration sequence

1. Commit this design on the existing candidate branch.
2. Integrate current `main` before implementation and resolve README conflicts
   by preserving both the Codex MCP documentation and the candidate's current
   agent workflow.
3. Implement and verify bounded omission metrics as one commit.
4. Implement the runtime language capability contract and documentation
   synchronization as one functional commit, with factual documentation updates
   in a separate documentation commit when practical.
5. Implement Context Pack omission-quality changes as one commit.
6. Run the complete deterministic verification suite.
7. Build a frozen candidate binary from the final commit.
8. Run the full three-pair monotonic regression benchmark and complete its
   independent reviews.
9. If it passes, run the matched three-by-three release benchmark and complete
   the signed twelve-point quality rubric.
10. Only after both evidence sets pass, merge to `main`, install locally, clean
    and rescan the test workspace, run the final user acceptance test, and
    decide whether to publish.

## Testing

### Analyzer tests

Fixtures cover:

- exact omission range;
- subset of an omission range;
- widened range;
- wrong path;
- pathless omission;
- read before authorization;
- included-source overlap;
- compound commands with one unauthorized target;
- duplicate terminal events;
- JSON and Markdown Context Packs.

### Regression runner tests

Tests cover:

- new metric schema and retained total counts;
- contract omission cap;
- unauthorized-read median increase;
- bounded-read allowance without tool or token exemptions;
- unchanged release-benchmark total-source-read gate;
- retained and reviewable external evidence.

### Capability tests

Tests cover:

- every documented language profile;
- agreement between the profile and analyzer inventory;
- representative symbols, relations, calls, routes, tests, API clients,
  persistence, messaging, and data-flow fixtures;
- exact symbols, direct usages, and HTTP reachability only for supported
  languages;
- stable factual block rendering and stale-document detection.

### Context Pack tests

Generic fixtures cover:

- a missing cross-service production transition;
- a lexically similar but disconnected housekeeping endpoint;
- two requested domain-model and persistence families;
- an absent future internal API contract;
- bounded uncertainty without fallback;
- preservation of existing good Java, Go, JavaScript/TypeScript, Python, PHP,
  and Rust selection.

### Complete verification

Before external benchmarking:

```text
gofmt
go vet ./...
go test ./...
benchmark shell harness tests
committed G2-G6 matrix with repeated deterministic runs
documentation synchronization check
```

Any environment-only failure is rerun only with the minimum required local
permission. A quality, accuracy, or efficiency failure is retained and is not
reclassified as infrastructure failure.

## Release decision

The current branch is not release-ready based on one smoke pair. Publication
requires:

- clean integration with current `main`;
- all deterministic checks passing;
- a full three-pair monotonic regression result;
- no unexplained protected pack changes;
- independent completed run reviews;
- a passing matched three-by-three release benchmark;
- a signed quality rubric with assisted quality at least equal to baseline;
- synchronized public documentation;
- successful local installation and clean test-workspace rescan;
- explicit release approval.
