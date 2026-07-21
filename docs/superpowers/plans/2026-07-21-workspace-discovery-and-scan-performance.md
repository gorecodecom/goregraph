# Workspace Discovery and Scan Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Discover each real repository or manifest-backed project once, remove quadratic script-resolution work, and make long workspace scans visibly progress without losing analysis data.

**Architecture:** Replace directory-name-based discovery with a recursive project-boundary walk that stops below Git or manifest roots. Pre-index script exports, imports, and re-exports and memoize completed non-cyclic export resolutions. Wrap each sequential workspace project scan in a portable progress reporter with deterministic test hooks.

**Tech Stack:** Go 1.26 standard library, existing `internal/scan`, `internal/cli`, and `internal/gitignore` packages, Go tests and benchmarks, Windows/macOS/Linux filesystem semantics.

## Global Constraints

- Preserve every generated fact, confidence, ambiguity, cycle outcome, stable ID, sort order, output schema, dashboard projection, and agent projection.
- Discover a project root from its own `.git` directory, linked-worktree `.git` file, supported root manifest, or existing valid GoreGraph project output.
- Stop discovery below a recognized project root so nested monorepo manifests are not separate projects.
- Do not follow directory symlinks or enter hidden and generated infrastructure directories.
- Keep project scans sequential; do not hide the resolver defect with parallelism.
- Keep `workspace scan-all` and `workspace build all` behavior equivalent.
- Use ordinary terminal lines that work in PowerShell, Command Prompt, macOS terminals, redirected logs, and CI.
- Add no dependency and create no release, tag, or publication.

---

### Task 1: Define Project-Boundary Discovery

**Files:**
- Modify: `internal/scan/workspace_reconcile_test.go:1330-1375`
- Modify: `internal/scan/workspace_reconcile.go:542-580`
- Modify: `internal/scan/workspace_reconcile.go:674-690`

**Interfaces:**
- Consumes: `addWorkspaceProject`, `isWorkspaceGroup`, `hasProjectMarker`, and normalized workspace-relative paths.
- Produces: `walkWorkspaceProjectRoots(workspaceRoot, currentAbs, dir, group, defaultOutput string, projects map[string]WorkspaceProjectRecord) error`, `hasWorkspaceProjectRoot(abs, outputDir string) bool`, and `skipWorkspaceDiscoveryDir(name string) bool`.

- [ ] **Step 1: Write the failing boundary test**

Add a fixture containing a Git monorepo with nested packages, a non-Git project nested below an organizational directory, and a hidden `.worktrees` container that itself contains a manifest:

```go
func TestWorkspaceDiscoveryStopsAtProjectBoundaries(t *testing.T) {
	workspace := t.TempDir()
	monorepo := filepath.Join(workspace, "frontend", "frontend-monorepo")
	if err := os.MkdirAll(filepath.Join(monorepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, monorepo, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, monorepo, "apps/portal/package.json", `{"name":"portal"}`)
	writeFile(t, filepath.Join(workspace, "microservices", "commerce", "orders"), "Cargo.toml", "[package]\nname='orders'\n")
	writeFile(t, filepath.Join(workspace, "frontend", ".worktrees", "temporary"), "package.json", `{"name":"temporary"}`)

	projects, err := discoverWorkspaceProjects(workspace, monorepo, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"frontend/frontend-monorepo",
		"microservices/commerce/orders",
	}, "\n")
	if got := strings.Join(workspaceProjectPaths(projects), "\n"); got != want {
		t.Fatalf("project paths = %q, want %q", got, want)
	}
}
```

Add this test helper and use `strings.Join` for comparisons so no dependency is introduced:

```go
func workspaceProjectPaths(projects []WorkspaceProjectRecord) []string {
	paths := make([]string, 0, len(projects))
	for _, project := range projects {
		paths = append(paths, project.Path)
	}
	return paths
}
```

- [ ] **Step 2: Write Git-file, root-project, and marker-table tests**

Add tests that require a regular `.git` file to identify a linked worktree, require a workspace root with `go.mod` to be returned once, and table-drive all root marker classes:

```go
func TestWorkspaceDiscoveryRecognizesLinkedWorktreeGitFile(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services", "linked")
	writeFile(t, project, ".git", "gitdir: ../.git/worktrees/linked\n")
	writeFile(t, project, "README.md", "# linked\n")

	projects, err := discoverWorkspaceProjects(workspace, project, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(workspaceProjectPaths(projects), "\n"); got != "services/linked" {
		t.Fatalf("project paths = %q, want services/linked", got)
	}
}
```

Use these exact marker cases:

```go
func TestWorkspaceDiscoveryRecognizesProjectMarkers(t *testing.T) {
	markers := []string{
		"package.json", "pom.xml", "build.gradle", "build.gradle.kts",
		"settings.gradle", "settings.gradle.kts", "go.mod", "pyproject.toml",
		"requirements.txt", "setup.py", "Cargo.toml", "composer.json",
		"build.sbt", "Package.swift", "Gemfile", "example.gemspec",
		"CMakeLists.txt", "meson.build", "example.sln", "example.csproj",
		"goregraph.yml",
	}
	for _, marker := range markers {
		t.Run(marker, func(t *testing.T) {
			workspace := t.TempDir()
			project := filepath.Join(workspace, "projects", "app")
			writeFile(t, project, marker, "marker\n")
			projects, err := discoverWorkspaceProjects(workspace, project, "goregraph-out")
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(workspaceProjectPaths(projects), "\n"); got != "projects/app" {
				t.Fatalf("project paths = %q, want projects/app", got)
			}
		})
	}
}

func TestWorkspaceDiscoveryRecognizesWorkspaceRoot(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, workspace, "go.mod", "module example.com/root\n")
	writeFile(t, workspace, "services/nested/package.json", `{"name":"nested"}`)
	projects, err := discoverWorkspaceProjects(workspace, workspace, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].AbsPath != filepath.ToSlash(workspace) {
		t.Fatalf("projects = %#v, want workspace root once", projects)
	}
}
```

- [ ] **Step 3: Run the focused tests and verify RED**

```text
go test ./internal/scan -run "TestWorkspaceDiscoveryStopsAtProjectBoundaries|TestWorkspaceDiscoveryRecognizesLinkedWorktreeGitFile|TestWorkspaceDiscoveryRecognizesProjectMarkers|TestWorkspaceDiscoveryRecognizesWorkspaceRoot" -count=1
```

Expected: FAIL because `.worktrees` is included, recursive projects are missed, Git-only roots are missed, and the added ecosystem markers are incomplete.

- [ ] **Step 4: Implement boundary-first recursive discovery**

Replace the grouped immediate-child loop and flat-root loop with a single boundary walk. Keep `addWorkspaceProject` unchanged and pass the nearest conventional group name through recursion:

```go
func walkWorkspaceProjectRoots(
	workspaceRoot, currentAbs, dir, group, defaultOutput string,
	projects map[string]WorkspaceProjectRecord,
) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() || skipWorkspaceDiscoveryDir(name) {
			continue
		}
		abs := filepath.Join(dir, name)
		nextGroup := group
		if isWorkspaceGroup(name) {
			nextGroup = name
		}
		if hasWorkspaceProjectRoot(abs, defaultOutput) {
			addWorkspaceProject(projects, workspaceRoot, currentAbs, abs, nextGroup, defaultOutput)
			continue
		}
		if err := walkWorkspaceProjectRoots(workspaceRoot, currentAbs, abs, nextGroup, defaultOutput, projects); err != nil {
			continue
		}
	}
	return nil
}
```

Check `hasWorkspaceProjectRoot(workspaceRoot, defaultOutput)` before descending. If true, add only the workspace root and return the sorted result. Use these helpers so both a directory and a regular worktree file qualify without invoking Git:

```go
func hasWorkspaceProjectRoot(abs, outputDir string) bool {
	if info, err := os.Lstat(filepath.Join(abs, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
		return true
	}
	return hasProjectMarker(abs, outputDir)
}

func skipWorkspaceDiscoveryDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch strings.ToLower(name) {
	case "node_modules", "vendor", "target", "build", "dist", "coverage", "goregraph-out":
		return true
	default:
		return false
	}
}
```

Extend `hasProjectMarker` with exact marker names and root-level `filepath.Match` checks for `*.gemspec`, `*.sln`, and `*.csproj`. Preserve recognition of an existing project output manifest:

```go
func hasProjectMarker(abs, outputDir string) bool {
	for _, name := range []string{
		"package.json", "pom.xml", "build.gradle", "build.gradle.kts",
		"settings.gradle", "settings.gradle.kts", "go.mod", "pyproject.toml",
		"requirements.txt", "setup.py", "Cargo.toml", "composer.json",
		"build.sbt", "Package.swift", "Gemfile", "CMakeLists.txt",
		"meson.build", "goregraph.yml",
	} {
		if workspaceFileExists(filepath.Join(abs, name)) {
			return true
		}
	}
	entries, _ := os.ReadDir(abs)
	for _, entry := range entries {
		for _, pattern := range []string{"*.gemspec", "*.sln", "*.csproj"} {
			if matched, _ := filepath.Match(pattern, entry.Name()); matched && !entry.IsDir() {
				return true
			}
		}
	}
	return workspaceFileExists(filepath.Join(abs, outputDir, "manifest.json"))
}
```

- [ ] **Step 5: Run focused and neighboring workspace tests**

```text
gofmt -w internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go
go test ./internal/scan -run "TestWorkspaceDiscovery|TestWorkspaceProjectScanPlan|TestWorkspaceMissingScanPlan" -count=1
```

Expected: PASS. Existing kind and service classification tests remain unchanged.

- [ ] **Step 6: Commit project-boundary discovery**

```text
Discover workspace projects at repository boundaries

- Recognize Git and supported manifest roots recursively.
- Stop below monorepo roots and ignore infrastructure directories.
- Cover linked worktrees, nested services, and cross-platform markers.
```

### Task 2: Index Script References and Memoize Export Resolution

**Files:**
- Modify: `internal/scan/symbol_script_test.go:760-940`
- Modify: `internal/scan/symbol_script.go:1770-2068`

**Interfaces:**
- Consumes: `ProjectSymbolFacts`, `RichRelationRecord`, `scriptFactResolver.resolveModule`, existing declaration maps, alias metadata, and capability filtering.
- Produces: `newScriptReferenceIndex([]RichRelationRecord) scriptReferenceIndex`, `scriptExportMemoKey`, and indexed `scriptFactResolver.resolveExport` behavior with unchanged results.

- [ ] **Step 1: Write a failing reference-index test**

Define the required grouping contract before implementation:

```go
func TestScriptReferenceIndexGroupsExportsImportsAndReexports(t *testing.T) {
	references := []RichRelationRecord{
		{From: "src/index.ts", Type: "exports_local", TargetExport: "local", scriptExportAlias: "public"},
		{From: "src/index.ts", Type: "imports_value", TargetExport: "remote", scriptLocalName: "local"},
		{From: "src/index.ts", Type: "reexports_value", TargetExport: "remote", scriptExportAlias: "public"},
		{From: "src/index.ts", Type: "reexports_all", TargetModule: "./shared"},
	}

	index := newScriptReferenceIndex(references)
	if len(index.localExports["src/index.ts"]["public"]) != 1 {
		t.Fatalf("local export index = %#v", index.localExports)
	}
	if len(index.imports["src/index.ts"]["local"]) != 1 {
		t.Fatalf("import index = %#v", index.imports)
	}
	if len(index.reexports["src/index.ts"]["public"]) != 1 || len(index.starReexports["src/index.ts"]) != 1 {
		t.Fatalf("re-export indexes = %#v / %#v", index.reexports, index.starReexports)
	}
}
```

- [ ] **Step 2: Add a realistic unresolved-reference benchmark**

Add `BenchmarkResolveScriptSymbolFactsLargeUnresolvedProject`. Build one provider file and one consumer file, then generate 1,000 `calls_export` references to distinct missing exports in the provider. Run one resolution per benchmark iteration and retain the result with a package-level benchmark sink.

```go
func BenchmarkResolveScriptSymbolFactsLargeUnresolvedProject(b *testing.B) {
	files := []FileRecord{
		{Path: "src/provider.ts", Language: "typescript"},
		{Path: "src/consumer.ts", Language: "typescript"},
	}
	facts := ProjectSymbolFacts{}
	for i := 0; i < 1000; i++ {
		facts.References = append(facts.References, RichRelationRecord{
			ID: "reference-" + strconv.Itoa(i), From: "src/consumer.ts",
			Type: "calls_export", Language: "typescript",
			TargetModule: "./provider", TargetExport: "Missing" + strconv.Itoa(i),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkScriptFacts = ResolveScriptSymbolFacts(files, nil, nil, facts)
	}
}
```

Declare the sink next to the benchmark:

```go
var benchmarkScriptFacts ProjectSymbolFacts
```

- [ ] **Step 3: Run the new index test and capture the baseline benchmark**

```text
go test ./internal/scan -run TestScriptReferenceIndexGroupsExportsImportsAndReexports -count=1
go test ./internal/scan -run '^$' -bench BenchmarkResolveScriptSymbolFactsLargeUnresolvedProject -benchtime=1x -count=3
```

Expected: the unit test FAILS because `newScriptReferenceIndex` does not exist. Record the three baseline benchmark durations before changing the resolver.

- [ ] **Step 4: Build reference indexes once**

Add focused nested maps to `scriptFactResolver` and populate them while constructing the resolver:

```go
type scriptReferenceIndex struct {
	localExports  map[string]map[string][]RichRelationRecord
	imports       map[string]map[string][]RichRelationRecord
	reexports     map[string]map[string][]RichRelationRecord
	starReexports map[string][]RichRelationRecord
}

func newScriptReferenceIndex(references []RichRelationRecord) scriptReferenceIndex {
	index := scriptReferenceIndex{
		localExports:  map[string]map[string][]RichRelationRecord{},
		imports:       map[string]map[string][]RichRelationRecord{},
		reexports:     map[string]map[string][]RichRelationRecord{},
		starReexports: map[string][]RichRelationRecord{},
	}
	for _, reference := range references {
		switch {
		case reference.Type == "exports_local":
			name := reference.scriptExportAlias
			if name == "" {
				name = reference.TargetExport
			}
			appendScriptReferenceIndex(index.localExports, reference.From, name, reference)
		case strings.HasPrefix(reference.Type, "imports_"):
			appendScriptReferenceIndex(index.imports, reference.From, reference.scriptLocalName, reference)
		case reference.Type == "reexports_all":
			index.starReexports[reference.From] = append(index.starReexports[reference.From], reference)
		case strings.HasPrefix(reference.Type, "reexports_"):
			name := reference.scriptExportAlias
			if name == "" {
				name = reference.TargetExport
			}
			appendScriptReferenceIndex(index.reexports, reference.From, name, reference)
		}
	}
	return index
}
```

Use this helper so empty aliases are never indexed:

```go
func appendScriptReferenceIndex(
	index map[string]map[string][]RichRelationRecord,
	file, name string,
	reference RichRelationRecord,
) {
	if file == "" || name == "" {
		return
	}
	if index[file] == nil {
		index[file] = map[string][]RichRelationRecord{}
	}
	index[file][name] = append(index[file][name], reference)
}
```

Store the result on the resolver as `references scriptReferenceIndex` and remove the now-unnecessary `facts` field after all three full-list loops are replaced.

- [ ] **Step 5: Replace full scans and memoize completed non-cyclic resolutions**

Use indexed slices in `resolveExport`: direct local exports from `localExports[module.file][exportName]`, matching imports from `imports[module.file][localName]`, named re-exports from `reexports[module.file][exportName]`, and wildcard re-exports from `starReexports[module.file]` except for `default`.

Add a memo key and copy helper:

```go
type scriptExportMemoKey struct {
	file       string
	exportName string
	capability scriptSymbolCapability
}

func cloneScriptExportResolution(value scriptExportResolution) scriptExportResolution {
	value.candidates = append([]RichSymbolRecord(nil), value.candidates...)
	return value
}
```

Initialize `exportMemo map[scriptExportMemoKey]scriptExportResolution` on the resolver. At the beginning of `resolveExport`, return a clone of a cached value. Cache a clone before returning only when `cyclic` is false; never cache a result derived from an active cycle path. Continue using the existing per-call `visited` map for cycle detection.

- [ ] **Step 6: Verify semantic parity and benchmark improvement**

```text
gofmt -w internal/scan/symbol_script.go internal/scan/symbol_script_test.go
go test ./internal/scan -run "TestScriptReferenceIndexGroupsExportsImportsAndReexports|Test.*Script.*(Reexport|ReExport|Cycle|Conditional|Alias|Ambiguous)|TestResolveScriptSymbolFacts" -count=1
go test ./internal/scan -run '^$' -bench BenchmarkResolveScriptSymbolFactsLargeUnresolvedProject -benchtime=1x -count=3
```

Expected: all focused tests PASS. The benchmark must show a clear reduction from repeated full-list traversal; investigate rather than relaxing correctness assertions if existing symbol results change.

- [ ] **Step 7: Run the complete scan package**

```text
go test ./internal/scan -count=1
```

Expected: PASS with no fixture or deterministic-output changes.

- [ ] **Step 8: Commit resolver performance work**

```text
Index script export resolution

- Group imports and exports once per project scan.
- Memoize completed non-cyclic export resolutions.
- Preserve exact, ambiguous, and cyclic symbol outcomes.
```

### Task 3: Show Portable Workspace Scan Progress

**Files:**
- Create: `internal/cli/workspace_progress.go`
- Create: `internal/cli/workspace_progress_test.go`
- Modify: `internal/cli/cli.go:730-770`
- Modify: `internal/cli/cli.go:825-875`
- Modify: `internal/cli/cli_test.go:675-765`

**Interfaces:**
- Consumes: `scan.Result`, workspace plan item position and path, sequential scan closures, stdout and stderr writers.
- Produces: `runWorkspaceProjectWithProgress(stdout, stderr io.Writer, position, total int, project string, clock workspaceProgressClock, run func() (scan.Result, error)) (scan.Result, error)`.

- [ ] **Step 1: Write deterministic progress tests**

Define a fake ticker with a buffered channel and a fake clock. Test start, one heartbeat, completion, and failure without sleeping:

```go
func TestRunWorkspaceProjectWithProgressReportsHeartbeatAndCompletion(t *testing.T) {
	start := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	ticker := &fakeProgressTicker{ticks: make(chan time.Time)}
	clock := workspaceProgressClock{
		now: sequenceClock(t, start, start.Add(12*time.Second)),
		newTicker: func(time.Duration) progressTicker { return ticker },
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, err := runWorkspaceProjectWithProgress(&stdout, &stderr, 4, 43, "frontend/frontend-monorepo", clock, func() (scan.Result, error) {
			close(started)
			<-release
			return scan.Result{ScannedFiles: 1517, SkippedFiles: 115}, nil
		})
		done <- err
	}()
	<-started
	ticker.ticks <- start.Add(10 * time.Second)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Scanning [4/43]", "Still scanning [4/43]", "Completed [4/43]", "1517 files", "115 skipped"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("progress missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
```

Add a failure test requiring `Failed [4/43] frontend/frontend-monorepo during scan` on stderr and no `Completed` line.

- [ ] **Step 2: Run the progress tests and verify RED**

```text
go test ./internal/cli -run TestRunWorkspaceProjectWithProgress -count=1
```

Expected: FAIL because the progress types and runner do not exist.

- [ ] **Step 3: Implement the progress runner without concurrent writer access**

Create a ticker abstraction and run the scan closure in a result goroutine while the caller goroutine owns all output:

```go
const workspaceProgressInterval = 10 * time.Second

type progressTicker interface {
	C() <-chan time.Time
	Stop()
}

type realProgressTicker struct{ ticker *time.Ticker }

func (ticker realProgressTicker) C() <-chan time.Time { return ticker.ticker.C }
func (ticker realProgressTicker) Stop()              { ticker.ticker.Stop() }

type workspaceProgressClock struct {
	now       func() time.Time
	newTicker func(time.Duration) progressTicker
}

func defaultWorkspaceProgressClock() workspaceProgressClock {
	return workspaceProgressClock{
		now: time.Now,
		newTicker: func(interval time.Duration) progressTicker {
			return realProgressTicker{ticker: time.NewTicker(interval)}
		},
	}
}
```

Implement the runner with one output-owning goroutine:

```go
type workspaceScanOutcome struct {
	result scan.Result
	err    error
}

func runWorkspaceProjectWithProgress(
	stdout, stderr io.Writer,
	position, total int,
	project string,
	clock workspaceProgressClock,
	run func() (scan.Result, error),
) (scan.Result, error) {
	started := clock.now()
	fmt.Fprintf(stdout, "Scanning [%d/%d] %s ...\n", position, total, project)
	ticker := clock.newTicker(workspaceProgressInterval)
	defer ticker.Stop()
	outcome := make(chan workspaceScanOutcome, 1)
	go func() {
		result, err := run()
		outcome <- workspaceScanOutcome{result: result, err: err}
	}()
	for {
		select {
		case tick := <-ticker.C():
			fmt.Fprintf(stdout, "Still scanning [%d/%d] %s (%s elapsed)\n", position, total, project, tick.Sub(started).Round(time.Second))
		case completed := <-outcome:
			elapsed := clock.now().Sub(started).Round(100 * time.Millisecond)
			if completed.err != nil {
				fmt.Fprintf(stderr, "Failed [%d/%d] %s during scan after %s: %v\n", position, total, project, elapsed, completed.err)
				return completed.result, completed.err
			}
			fmt.Fprintf(stdout, "Completed [%d/%d] %s in %s (%d files, %d skipped)\n", position, total, project, elapsed, completed.result.ScannedFiles, completed.result.SkippedFiles)
			return completed.result, nil
		}
	}
}
```

Define deterministic test helpers without sleeps:

```go
type fakeProgressTicker struct{ ticks chan time.Time }

func (ticker *fakeProgressTicker) C() <-chan time.Time { return ticker.ticks }
func (ticker *fakeProgressTicker) Stop()              {}

func sequenceClock(t *testing.T, values ...time.Time) func() time.Time {
	t.Helper()
	var mu sync.Mutex
	next := 0
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		if next >= len(values) {
			t.Fatalf("clock called %d times, only %d values configured", next+1, len(values))
		}
		value := values[next]
		next++
		return value
	}
}
```


- [ ] **Step 4: Integrate progress into complete and missing workspace scans**

In both project loops, use `index+1` and `len(plan.Items)`. Keep config loading and `.gitignore` handling before the scan wrapper. Replace the existing post-scan `- Scanned` line with the wrapper's completion line:

```go
result, err := runWorkspaceProjectWithProgress(
	stdout,
	stderr,
	index+1,
	len(plan.Items),
	item.Project,
	defaultWorkspaceProgressClock(),
	func() (scan.Result, error) {
		return scan.RunBuild(item.AbsPath, projectCfg, target)
	},
)
if err != nil {
	return 1
}
scanned++
```

Use `scan.Run` in the `scan-missing` closure. Do not change dry-run output, workspace reconciliation order, exit codes, or final summary text.

- [ ] **Step 5: Verify progress and existing CLI behavior**

```text
gofmt -w internal/cli/workspace_progress.go internal/cli/workspace_progress_test.go internal/cli/cli.go internal/cli/cli_test.go
go test ./internal/cli -run "TestRunWorkspaceProjectWithProgress|TestRunWorkspaceScanAll|TestRunWorkspaceScanMissing" -count=1
go test ./internal/cli -count=1
```

Expected: PASS. Tests use fake time only; no test sleeps ten seconds.

- [ ] **Step 6: Commit portable progress reporting**

```text
Report workspace scan progress

- Print project position before and after each scan.
- Emit portable heartbeat lines for long-running projects.
- Preserve sequential execution and fail-fast workspace publication.
```

### Task 4: Document Workspace Boundaries in Help

**Files:**
- Modify: `internal/cli/cli.go:800-820`
- Modify: `internal/cli/cli.go:1500-1560`
- Modify: `internal/cli/cli_test.go:1814-1830`
- Modify: `COMMANDS.md`
- Modify: `docs_test.go`

**Interfaces:**
- Consumes: discovery semantics from Task 1 and existing progressive help structure.
- Produces: user-facing guidance that Git and manifest roots are discovered once and nested monorepo manifests remain within their parent project.

- [ ] **Step 1: Write failing help and documentation assertions**

Require complete workspace help, `workspace scan-all --help`, and `COMMANDS.md` to contain these concepts:

```text
Git repositories and supported project manifests are discovered automatically.
Once a project root is detected, nested manifests remain part of that project.
```

Use semantic substring assertions rather than requiring line wrapping or punctuation.

- [ ] **Step 2: Verify documentation RED**

```text
go test ./internal/cli -run "TestWorkspace.*Help|TestWorkspaceScanAllHelp" -count=1
go test . -run "TestCommandsReference" -count=1
```

Expected: FAIL because current help still describes only conventional grouped layouts.

- [ ] **Step 3: Update help and command reference**

Explain the boundary rule in the existing workspace-detection sections. Keep the concise default help short; put the complete explanation in complete workspace help, scan-all command help, and `COMMANDS.md`. Preserve the visible `workspace scan-all` compatibility-alias explanation.

- [ ] **Step 4: Verify help and documentation GREEN**

```text
go test ./internal/cli -run "TestWorkspace.*Help|TestWorkspaceScanAllHelp" -count=1
go test . -run "TestCommandsReference" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit documentation parity**

```text
Document workspace project boundaries

- Explain Git and manifest-based project discovery.
- Clarify that nested monorepo manifests are scanned with their root.
```

### Task 5: Verify Against the Real Workspace and Install Locally

**Files:**
- Verify: all changed Go source, tests, help, and documentation
- Generated local outputs: `C:\Users\goretzkh\projects\weka\**\goregraph-out`
- Install target: `C:\Users\goretzkh\go\bin\goregraph.exe`

**Interfaces:**
- Consumes: Tasks 1-4 and the real Weka workspace.
- Produces: clean verified commits, measured real-world scan duration, a complete workspace manifest, and a local development binary. No push or release is part of this task.

- [ ] **Step 1: Run formatting, whitespace, unit tests, and vet**

```text
gofmt -d internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/scan/symbol_script.go internal/scan/symbol_script_test.go internal/cli/workspace_progress.go internal/cli/workspace_progress_test.go internal/cli/cli.go internal/cli/cli_test.go
git diff --check
go test ./... -count=1
go vet ./...
```

Expected: no formatting diff, no whitespace errors, all tests PASS, and vet reports no issues.

- [ ] **Step 2: Inspect the real discovery plan without scanning**

```text
go run ./cmd/goregraph workspace build all C:\Users\goretzkh\projects\weka --workspace C:\Users\goretzkh\projects\weka --dry-run --no-update-gitignore
```

Expected: every genuine Git or manifest-backed project is listed once; `.worktrees` is absent; no nested `frontend-monorepo/apps/*` package appears separately.

- [ ] **Step 3: Run and time the real complete workspace build**

```powershell
Measure-Command {
  go run ./cmd/goregraph workspace build all C:\Users\goretzkh\projects\weka `
    --workspace C:\Users\goretzkh\projects\weka `
    --no-update-gitignore
}
```

Expected: the command completes, progress appears for every project, `frontend-monorepo` finishes in under one minute on the same machine, and `C:\Users\goretzkh\projects\weka\.goregraph-workspace\manifest.json` is complete. If the target is missed, capture a CPU profile and return to Task 2 instead of lowering the target or dropping facts.

- [ ] **Step 4: Confirm output completeness and health**

```text
go run ./cmd/goregraph workspace status C:\Users\goretzkh\projects\weka --workspace C:\Users\goretzkh\projects\weka
go run ./cmd/goregraph doctor C:\Users\goretzkh\projects\weka\frontend\frontend-monorepo
```

Expected: discovered projects report indexed state, the workspace manifest exists, and doctor reports complete index, agent, and dashboard projections without schema errors.

- [ ] **Step 5: Install and verify the local binary**

```text
go install ./cmd/goregraph
C:\Users\goretzkh\go\bin\goregraph.exe version
C:\Users\goretzkh\go\bin\goregraph.exe workspace build all C:\Users\goretzkh\projects\weka --workspace C:\Users\goretzkh\projects\weka --dry-run --no-update-gitignore
```

Expected: version remains `1.3.0` with schema 3, and the installed binary produces the verified discovery plan. Do not create a tag, release, or package publication.

- [ ] **Step 6: Inspect final history and working tree**

```text
git status --short --branch
git log --oneline origin/main..HEAD
```

Expected: source and documentation changes are committed, no task file is staged, and only explicitly accepted external generated Weka outputs changed. Do not push until the user explicitly requests it.
