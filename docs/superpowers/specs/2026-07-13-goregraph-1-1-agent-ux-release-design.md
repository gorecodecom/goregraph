# GoreGraph 1.1 Agent UX Release Design

## Goal

Deliver the eight planned Agent UX issues as GoreGraph 1.1.0 without
changing Schema 2 meanings or pushing any Git state.

## Compatibility

- Existing CLI commands, Query tasks, MCP tools, reports, and Schema 2 facts
  remain valid.
- `task-context`, `workspace-delta`, freshness records, diagnostic families,
  and contract summaries are additive.
- Unknown or incomplete analysis is represented as coverage or uncertainty,
  never as evidence that behavior does not exist.

## Delivery

Implement Issues 1 through 8 in dependency order. Each issue starts with
focused failing tests, updates only its required public surfaces, passes its
focused verification, and is recorded in one local commit. A final local
release-acceptance commit sets the version to 1.1.0 and records supporting
documentation and validation evidence.

## Validation

Run focused tests per issue, then `gofmt -l .`, `go test ./... -count=1`,
`go vet ./...`, and `git diff --check`. Build and install the local 1.1.0
binary, run `goregraph workspace clean . --execute` and
`goregraph workspace scan-all .` from the specified WEKA workspace, and
verify the documented acceptance criteria. Do not push.
