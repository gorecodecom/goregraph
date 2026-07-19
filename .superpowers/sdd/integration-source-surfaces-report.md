# Integration Source Surfaces Report

## Scope

Updated the Query, CLI, and MCP integration tests after coverage-first source selection changed the selected source surface from a full body to the smallest useful signature.

## RED Reproduction

Before the changes, `go test ./internal/query ./internal/cli ./internal/mcp -count=1` failed because tests expected:

- a three-line body where the selector now returns the useful one-line signature;
- source-control escaping on a line no longer selected for rendering;
- dense relevance fixtures to fill every requested file slot; and
- MCP source range `20-22` instead of the selected signature range `20-20`.

## Changes

- Query Markdown tests now assert the selected useful signature and put escaped controls in that selected source line.
- Query and CLI task-context tests prove `limit=1` constrains selection, explicit `max_files=2` wins over `limit=1`, and default/capped cases stay bounded and deterministic without requiring irrelevant fan-out.
- CLI checks precedence in both option orders.
- MCP retains budget, useful-source, complete-coverage, bare compact JSON, and deterministic expert/default assertions while expecting the signature range.

No production code changed.

## Verification

- Focused query, CLI, and MCP regression tests: passed.
- `go test ./internal/query ./internal/cli ./internal/mcp -count=1`: passed.
- `go test ./... -count=1`: passed.
- `go vet ./...`: passed.
- `gofmt` and `git diff --check`: passed.

## Concerns

None. The revised assertions preserve Task 5/6 behavior: selected source remains useful, sanitizer coverage is exercised on rendered content, and bounded selection is not mistaken for mandatory saturation.
