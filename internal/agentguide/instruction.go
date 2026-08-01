// Package agentguide owns the canonical normal-agent workflow text.
package agentguide

// AssistedInstruction is the canonical bounded Context workflow used by every
// user-facing integration surface.
const AssistedInstruction = `Call goregraph context . --query "<focused query>" exactly once before reading indexed source; put the caller's problem statement and requested evidence scope in the query.
Preserve the caller's domain language, identifiers, and requested evidence; exclude workspace setup, tool policy, safety constraints, and output-format instructions. Do not translate or add inferred repository or component responsibilities.
If the context command fails, do not read context-index.json or any generated index; only a missing or stale output error permits goregraph doctor ., otherwise stop using GoreGraph and follow the caller's fallback policy.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path and start_line/end_line ranges listed in source_omissions; do not inspect outside those ranges or other files. Report pathless or unbounded omissions as uncertainty.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
A missing future call, route, or symbol required by the requested fix is evidence of the current gap, not a source-fallback trigger; assess entrypoint reliability from the existing production path.
For change plans, enumerate exact existing production and test paths supplied by files, source_sections, plan_files, or bounded omission reads; treat plan_files as metadata-only existing identities or patterns, never read them unless source_omissions lists the same exact path with a bounded range, do not treat mock_pattern or retry_pattern entries as change targets, do not invent future filenames, and keep future route, authentication, status, lookup implementation, and cross-service transaction ordering as unknown design decisions unless rendered source proves them.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.`

// AssistedInstructionLineCount is the number of non-empty protocol lines in
// AssistedInstruction.
const AssistedInstructionLineCount = 12
