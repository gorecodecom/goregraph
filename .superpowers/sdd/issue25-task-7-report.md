# Issue #25 Task 7 Report: Symbol Interface Parity

## Outcome

Task 7 exposes the canonical workspace symbol projections through Agent,
Query, Explain, CLI, and MCP without adding another resolver.

The public operations are:

- Agent and Query:
  - `symbol-inventory`
  - `symbol-resolve`
  - `symbol-usages`
  - `symbol-api-consumers`
  - `symbol-explain`
- MCP:
  - `symbol_inventory`
  - `symbol_resolve`
  - `symbol_usages`
  - `symbol_api_consumers`
  - `symbol_explain`
- Explain:
  - `goregraph explain <workspace> symbol:<id>`
  - `goregraph explain <workspace> usage:<id>`

All operations read `.goregraph-workspace/symbol-index.json` or
`.goregraph-workspace/symbol-usages.json`. They accept either the workspace
root or an indexed project root whose parent workspace can be detected.

## Canonical Behavior

### Inventory and resolution

`symbol-inventory` searches canonical declaration fields and returns the full
`CanonicalSymbolRecord` through `Item.Data["symbol"]`.

`symbol-resolve` is the only operation that accepts ambiguous human symbol
text. It matches stable ID, simple name, qualified name, or export name
exactly, case-insensitively. It returns every canonical candidate in stable-ID
order and does not select a winner.

### Direct references and HTTP reachability

`symbol-usages` requires a stable `symbol:` ID and returns only records whose
canonical category is `direct_reference`.

`symbol-api-consumers` requires a stable `symbol:` ID and returns only records
whose canonical category is `reached_through_api`.

Neither operation re-resolves provider names. The returned
`CanonicalSymbolUsageRecord` is preserved through `Item.Data["usage"]`,
including:

- category and resolution;
- reason and confidence;
- evidence IDs;
- candidate symbol and path IDs;
- dependency evidence;
- ordered API path;
- limitations.

### Explain

`symbol-explain` requires a stable `symbol:` or `usage:` ID. Stable-ID CLI
Explain delegates to this same task. Legacy local file and symbol Explain
remains unchanged for targets without those prefixes.

JSON retains the complete canonical projection record. Markdown and text show
the stable item ID so users and agents can chain inventory, resolution, and
usage output directly into Explain.

## Bounds and Coverage

Items use the existing limit and continuation envelope:

- default limit: 20;
- maximum limit: 100;
- continuation tokens remain task-specific;
- MCP forwards limit and continuation to Agent Service.

Coverage warnings are built from canonical symbol coverage records.

- Inventory and resolution warnings are scoped to returned declaration
  projects.
- Direct-usage queries include every non-complete `direct_usages` coverage gap
  across possible workspace consumer projects and languages.
- API queries include every non-complete `http_reachability` gap across
  possible workspace consumer projects and languages.
- Missing usage records therefore never suppress partial, unavailable, or
  failed coverage.
- Symbol coverage warnings are returned completely and deterministically.
  They do not use the generic twelve-warning compaction or suggest a
  non-existent coverage-query continuation.

This preserves the rule that no matching usage is not proof that a symbol is
unused.

## CLI and Output Parity

The CLI registers all five Query tasks and documents their exact operation
names. Missing option values and missing required `--query` arguments are
syntax errors with exit code 2. Missing workspace projections are runtime
errors with exit code 1 and `goregraph workspace scan-all` remediation.

Direct projection aliases are:

- `symbol-index` -> `symbol-index.json`
- `symbol-usages-json` -> `symbol-usages.json`

These aliases work from workspace and detected project roots without
colliding with the `symbol-usages` task name.

MCP lists the five equivalent tools and maps them only through
`agentTaskForTool`. Its descriptions distinguish exact direct references from
HTTP reachability and direct human text through `symbol_resolve`.

## Strict TDD Evidence

### Agent RED

The initial focused test was:

```text
GOCACHE=/tmp/goregraph-go-cache go test ./internal/agent -run 'TestServiceReturnsSymbolInventory|TestServiceResolvesSymbolCandidatesWithoutGuessing' -count=1
```

It failed with:

```text
unknown agent task "symbol-inventory"
unknown agent task "symbol-resolve"
```

After projection loading and task implementation, the same tests passed.

### Query, Explain, and CLI RED

Stable-ID Explain and projection aliases initially failed because they entered
the legacy local index path. CLI tests failed because the five task names and
help entries were absent.

After dispatch and alias implementation, focused Query and CLI tests passed,
including unchanged legacy file Explain.

### MCP RED

The MCP tests initially failed with:

```text
symbol_inventory tool is not listed
unknown tool: symbol_usages
```

After adding only Agent-backed tool declarations and mappings, list, call,
record parity, bounds, and continuation tests passed.

### Review follow-up RED: stable IDs and consumer coverage

Markdown inventory output omitted the stable ID:

```text
- com.weka.UserService — java class declared in src/UserService.java:10
```

Usage coverage was filtered by projects that had returned records, so a
partial consumer project with no record disappeared. Regression tests were
added before the fixes. Text rows now contain backticked stable IDs, and
direct/API warnings are capability-scoped across the whole workspace.

### Review follow-up RED: more than twelve coverage gaps

A fixture with fifteen relevant `direct_usages` gaps returned only twelve real
warnings plus:

```text
3 additional coverage gaps omitted; continue the coverage query for details
```

That guidance was invalid because there is no paginated symbol-coverage
operation. The symbol task warning path now retains all fifteen sorted gaps;
generic warning compaction remains unchanged for existing non-symbol tasks.

## Verification

Focused verification covered:

```text
GOCACHE=/tmp/goregraph-go-cache go test ./internal/agent -count=1
GOCACHE=/tmp/goregraph-go-cache go test ./internal/query -count=1
GOCACHE=/tmp/goregraph-go-cache go test ./internal/cli -count=1
GOCACHE=/tmp/goregraph-go-cache go test ./internal/mcp -count=1
```

Repository-wide completion verification uses:

```text
test -z "$(gofmt -l .)"
git diff --check
GOCACHE=/tmp/goregraph-go-cache go test ./... -count=1
GOCACHE=/tmp/goregraph-go-cache go vet ./...
```

All commands passed after the interface and review fixes.

## Commits

- `e15af6c` — `Expose exact symbol exploration interfaces`
- `7ca2a95` — `Preserve symbol query chaining context`
- Final coverage-warning and report follow-up: committed with this report.

## Remaining Concerns

None.
