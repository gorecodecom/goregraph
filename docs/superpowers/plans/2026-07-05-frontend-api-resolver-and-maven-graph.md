# Frontend API Resolver And Maven Graph Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GoreGraph `v0.7.0` more useful on real WEKA-style frontend monorepos and Java services by extracting API contracts from realistic helper calls, reducing cross-app frontend mis-resolution, and adding Maven dependency graph output.

**Architecture:** Keep the scanner deterministic, local, and dependency-free. Extend the current static heuristics with bounded argument scanning and app/package-aware candidate ranking rather than introducing a full parser yet. Add Maven graph output alongside the existing Node package graph so backend users get package/dependency orientation without making Java/Spring the only deep path.

**Tech Stack:** Go stdlib, existing GoreGraph scan/query/doctor/report architecture, TDD with `go test`, release through GoReleaser tags.

---

### Task 1: Real API Helper Contracts

**Files:**
- Modify: `internal/scan/api_contracts.go`
- Modify: `internal/scan/scan_test.go`

- [ ] **Step 1: Write failing tests**

Add fixtures that prove `api-contracts.json` detects:

```js
return await GetHelper(dispatch, `/productservice/users/${userId}/products`);
const { status } = await PostHelper(
  dispatch,
  `/cadasters/${cadasterId}/users`,
  JSON.stringify(body)
);
await GetHelperWithStatus(dispatch, '/portal/tasks/flyout');
return fetch(url, { method: 'POST' });
```

Expected records:

- `GET /productservice/users/{userId}/products`
- `POST /cadasters/{cadasterId}/users`
- `GET /portal/tasks/flyout`
- fetch calls without literal URL stay omitted unless a literal URL is visible

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/scan -run 'TestRunExtractsRealisticFrontendAPIContracts' -count=1
```

Expected: FAIL because helper calls with `dispatch` as the first argument and multiline calls are not detected.

- [ ] **Step 3: Implement bounded helper call argument scanning**

In `api_contracts.go`:

- detect helper names `GetHelper`, `GetHelperWithStatus`, `PostHelper`, `PutHelper`, `PatchHelper`, `DeleteHelper`
- collect call text until balanced parentheses close or five lines are consumed
- extract the first string or template literal that starts with `/`
- normalize template placeholders from `${name}` to `{name}`
- keep confidence `EXTRACTED` and reason `helper-call-argument`

- [ ] **Step 4: Verify GREEN**

Run:

```bash
go test ./internal/scan -run 'TestRunExtractsRealisticFrontendAPIContracts' -count=1
```

Expected: PASS.

### Task 2: App-Aware Frontend Symbol Resolution

**Files:**
- Modify: `internal/scan/code_flows.go`
- Modify: `internal/scan/scan_test.go`

- [ ] **Step 1: Write failing tests**

Add a fixture with two apps:

```text
apps/portal/src/pages/home/home.jsx
apps/mein-konto/src/pages/home/home.jsx
apps/portal/src/routes.jsx
```

Both apps export `Home`. The portal route must resolve to the portal `Home`, not `mein-konto`.

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/scan -run 'TestRunKeepsFrontendRouteHandlersInsideOwningApp' -count=1
```

Expected: FAIL on current cross-app candidate ordering when the wrong same-name component appears first.

- [ ] **Step 3: Implement candidate ranking**

Rank route handler and generic call candidates:

1. same file
2. same app from `apps/<name>/...`
3. same package from `packages/<name>/...`
4. same language
5. fallback only if no better candidate exists

Keep inferred confidence explicit.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
go test ./internal/scan -run 'TestRunKeepsFrontendRouteHandlersInsideOwningApp' -count=1
```

Expected: PASS.

### Task 3: Maven Dependency Graph

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace.go`
- Create: `internal/scan/maven_graph.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/query/query.go`
- Modify: `internal/doctor/doctor.go`
- Modify: `internal/scan/scan_test.go`

- [ ] **Step 1: Write failing tests**

Add a Maven fixture with parent and dependencies:

```xml
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>service-a</artifactId>
  <version>1.0.0</version>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
      <version>3.5.0</version>
    </dependency>
  </dependencies>
</project>
```

Expected generated files:

- `maven-graph.json`
- `maven-graph.md`

Expected edge:

- `com.example:service-a` -> `org.springframework.boot:spring-boot-starter-web`

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/scan -run 'TestRunGeneratesMavenDependencyGraph' -count=1
```

Expected: FAIL because `maven-graph.json` is missing.

- [ ] **Step 3: Implement Maven graph**

Add Maven dependency records to `MavenPackageRecord`, parse `<dependencies><dependency>`, generate graph nodes/edges, markdown report, query aliases, and doctor JSON validation.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
go test ./internal/scan -run 'TestRunGeneratesMavenDependencyGraph' -count=1
```

Expected: PASS.

### Task 4: Documentation, Version, Release

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `SCHEMA.md`
- Modify: `ROADMAP.md`
- Modify: `AI_INTEGRATION_PLAN.md`
- Modify: `docs/RELEASE.md`
- Modify: `internal/version/version.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Document new outputs and limitations**

Document:

- realistic helper-call API contract extraction
- Maven graph files and query aliases
- app-aware frontend symbol resolution
- remaining limitation: full TS/JS AST and alias resolution is still future work

- [ ] **Step 2: Bump version to `0.7.0`**

Set default development version to `0.7.0` and update release checklist commands.

- [ ] **Step 3: Verify**

Run:

```bash
go test -count=1 ./...
go vet ./...
go build -o /tmp/goregraph-dev ./cmd/goregraph
/tmp/goregraph-dev version
```

Expected: all commands exit `0`, version prints `goregraph 0.7.0`.

- [ ] **Step 4: Commit and release**

Commit:

```bash
git commit -m "feat: improve frontend api and maven graph intelligence"
```

Tag:

```bash
git tag -a v0.7.0 -m "Release v0.7.0"
git push origin main
git push origin v0.7.0
```

Expected: GitHub Release workflow succeeds and Homebrew/Scoop metadata points to `0.7.0`.
