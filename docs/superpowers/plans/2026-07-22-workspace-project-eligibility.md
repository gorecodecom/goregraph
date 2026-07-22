# Workspace Project Eligibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make automatic workspace scans discover only marker-backed software projects while keeping explicit project scans and Git repository updates intact.

**Architecture:** Workspace scan eligibility is reduced to supported regular project markers. Git repository discovery remains a separate CLI concern, and previously generated GoreGraph output no longer bootstraps automatic discovery. Accepted roots still stop traversal so root-marked monorepos remain one project.

**Tech Stack:** Go 1.26, standard library filesystem APIs, existing GoreGraph CLI and scan packages, Go tests.

## Global Constraints

- Do not add workspace-specific folder names, include lists, or exclude lists.
- Do not add dependencies.
- Keep direct project builds able to scan an explicitly selected markerless directory.
- Keep workspace Git update able to find Git-only repositories.
- Preserve Windows and macOS path behavior through `filepath` and existing helpers.
- Do not delete existing project output automatically.
- Keep the reported application version at 1.3.0; do not create a release or Git tag.

---

### Task 1: Enforce marker-only automatic project eligibility

**Files:**
- Modify: `internal/scan/workspace_reconcile.go:543-772`
- Modify: `internal/scan/workspace_reconcile_test.go:1330-1580`

**Interfaces:**
- Consumes: existing supported marker list and `workspaceRegularFileExists(path string) bool`.
- Produces: `hasWorkspaceProjectRoot(abs string) bool` and `hasProjectMarker(abs string) bool` with marker-only semantics.

- [ ] **Step 1: Write failing discovery regressions**

Replace the Git-file-only expectation, markerless group fallback expectation, and valid-output-only expectation with tests equivalent to:

```go
func TestWorkspaceDiscoveryIgnoresGitOnlyRepositories(t *testing.T) {
	for _, gitEntry := range []string{"directory", "file"} {
		t.Run(gitEntry, func(t *testing.T) {
			workspace := t.TempDir()
			project := filepath.Join(workspace, "repositories", "documentation")
			if gitEntry == "directory" {
				if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, project, ".git", "gitdir: ../metadata\n")
			}
			writeFile(t, project, "README.md", "# documentation\n")

			projects, err := discoverWorkspaceProjects(workspace, workspace, "goregraph-out")
			if err != nil {
				t.Fatal(err)
			}
			if len(projects) != 0 {
				t.Fatalf("projects = %#v, want none", projects)
			}
		})
	}
}

func TestWorkspaceDiscoveryFindsMarkedProjectInsideGitOnlyRepository(t *testing.T) {
	workspace := t.TempDir()
	repository := filepath.Join(workspace, "repositories", "container")
	if err := os.MkdirAll(filepath.Join(repository, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repository, "apps", "api"), "go.mod", "module example.test/api\n")

	projects, err := discoverWorkspaceProjects(workspace, workspace, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(workspaceProjectPaths(projects), "\n"); got != "repositories/container/apps/api" {
		t.Fatalf("project paths = %q, want nested API", got)
	}
}

func TestWorkspaceDiscoveryDoesNotInferMarkerlessGroupChildren(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, filepath.Join(workspace, "microservices", "legacy-service"), "README.md", "# legacy\n")
	writeFile(t, filepath.Join(workspace, "microservices", "orders"), "Cargo.toml", "[package]\nname='orders'\n")

	projects, err := discoverWorkspaceProjects(workspace, workspace, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(workspaceProjectPaths(projects), "\n"); got != "microservices/orders" {
		t.Fatalf("project paths = %q, want only marked project", got)
	}
}

func TestWorkspaceDiscoveryIgnoresValidGeneratedOutputAsProjectMarker(t *testing.T) {
	workspace := t.TempDir()
	container := filepath.Join(workspace, "repositories", "documentation")
	manifest := OutputManifest{Tool: ToolName, Schema: SchemaVersion, Scope: "project", Index: ProjectionStatus{Complete: true}}
	if err := writeJSON(NewProjectOutputLayout(filepath.Join(container, "goregraph-out")).Manifest, manifest); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(container, "examples", "app"), "package.json", `{"name":"app"}`)

	projects, err := discoverWorkspaceProjects(workspace, workspace, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(workspaceProjectPaths(projects), "\n"); got != "repositories/documentation/examples/app" {
		t.Fatalf("project paths = %q, want nested marked project", got)
	}
}
```

Retain the marker matrix test, including `goregraph.yml`, and the root-marked monorepo boundary test.

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
go test ./internal/scan -run '^TestWorkspaceDiscovery(IgnoresGitOnlyRepositories|FindsMarkedProjectInsideGitOnlyRepository|DoesNotInferMarkerlessGroupChildren|IgnoresValidGeneratedOutputAsProjectMarker)$' -count=1
```

Expected: FAIL because `.git`, markerless conventional group children, and valid generated output are still accepted.

- [ ] **Step 3: Implement marker-only discovery**

Change the discovery predicates to:

```go
func hasWorkspaceProjectRoot(abs string) bool {
	return hasProjectMarker(abs)
}

func hasProjectMarker(abs string) bool {
	for _, name := range []string{
		"package.json", "pom.xml", "build.gradle", "build.gradle.kts",
		"settings.gradle", "settings.gradle.kts", "go.mod", "pyproject.toml",
		"requirements.txt", "setup.py", "Cargo.toml", "composer.json",
		"build.sbt", "Package.swift", "Gemfile", "CMakeLists.txt",
		"meson.build", "goregraph.yml",
	} {
		if workspaceRegularFileExists(filepath.Join(abs, name)) {
			return true
		}
	}
	entries, _ := os.ReadDir(abs)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		for _, pattern := range []string{"*.gemspec", "*.sln", "*.csproj"} {
			if matched, _ := filepath.Match(pattern, entry.Name()); matched {
				return true
			}
		}
	}
	return false
}
```

Update both callers of `hasWorkspaceProjectRoot` to omit `outputDir`. Remove the markerless group-child fallback block from `walkWorkspaceProjectRootsFound`. Keep `defaultOutput` for project output/status calculation after a marker-backed root is accepted. Keep `validProjectOutput` for determining whether an accepted project already has a usable index.

- [ ] **Step 4: Run focused and full scan tests**

Run:

```powershell
gofmt -w internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go
go test ./internal/scan -run '^TestWorkspaceDiscovery' -count=1
go test ./internal/scan -count=1 -timeout 10m
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit Task 1**

```powershell
git add -- internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go
git commit -m "Require project markers for workspace scans" -m "- Stop treating Git metadata and generated output as project evidence`n- Remove folder-name fallback for markerless projects`n- Preserve nested marker discovery and monorepo boundaries"
```

---

### Task 2: Preserve Git update behavior and explain the new rule

**Files:**
- Modify: `internal/cli/git_update_test.go:360-410`
- Modify: `internal/cli/cli.go:1540-1665`
- Modify: `internal/cli/cli_test.go:420-475`
- Modify: `README.md:380-405`
- Modify: `COMMANDS.md:1140-1220`

**Interfaces:**
- Consumes: `workspaceGitTargets(plan scan.WorkspaceProjectScanPlanRecord) ([]gitupdate.Target, error)`.
- Produces: unchanged Git target behavior and user-facing marker-first discovery documentation.

- [ ] **Step 1: Strengthen Git-only repository compatibility test**

Extend `TestWorkspaceGitTargetsDiscoversNestedRepositories` so the nested and linked repositories contain no project markers, and keep the exact expected target list:

```go
want := []string{
	filepath.ToSlash(workspace),
	filepath.ToSlash(linkedRepository),
	filepath.ToSlash(filepath.Join(workspace, "nested")),
	filepath.ToSlash(scanProject),
}
```

The scan plan contributes `scanProject`; recursive `.git` discovery contributes the markerless workspace, nested repository, and linked repository.

- [ ] **Step 2: Add help-text regression**

Add assertions to the existing workspace help test:

```go
for _, want := range []string{
	"project/build marker",
	".git alone",
	"goregraph.yml",
	"explicit project build",
} {
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("%v help output missing %q:\n%s", args, want, stdout.String())
	}
}
```

Run the help test before changing production help. Expected: FAIL because the explanation is absent.

- [ ] **Step 3: Update help and command documentation**

Add this behavior statement to full workspace help and scan-all help, with equivalent wording in `README.md` and `COMMANDS.md`:

```text
Automatic workspace scans require a project/build marker. A .git directory
alone identifies a repository for Git operations but is not a scan project.
Use a project-local goregraph.yml to opt in a non-standard project. An explicit
project build can still scan a deliberately selected markerless directory.
```

Do not introduce workspace include/exclude examples or machine-specific paths.

- [ ] **Step 4: Run CLI and documentation tests**

Run:

```powershell
gofmt -w internal/cli/git_update_test.go internal/cli/cli.go internal/cli/cli_test.go
go test ./internal/cli -run '^(TestWorkspaceGitTargetsDiscoversNestedRepositories|TestWorkspaceHelpExplainsFlatWorkspaceDetection)$' -count=1
go test ./internal/cli -count=1 -timeout 10m
go test . -count=1
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit Task 2**

```powershell
git add -- internal/cli/git_update_test.go internal/cli/cli.go internal/cli/cli_test.go README.md COMMANDS.md
git commit -m "Explain marker-first workspace discovery" -m "- Keep Git-only repositories available to workspace Git updates`n- Document automatic project eligibility and explicit opt-in behavior`n- Cover the distinction in CLI help tests"
```

---

### Task 3: Verify Weka, reinstall 1.3.0, and publish the branch

**Files:**
- No tracked source changes expected.
- Create ignored verification binary: `.superpowers/sdd/goregraph-project-eligibility.exe`

**Interfaces:**
- Consumes: final branch executable and Weka workspace at `C:/Users/goretzkh/projects/weka`.
- Produces: verified 43-project dry-run and local `goregraph 1.3.0` installation.

- [ ] **Step 1: Run final formatting, vet, and test verification**

Run:

```powershell
$files = @(git diff --name-only main..HEAD -- '*.go')
$unformatted = @(& gofmt -l @files)
if ($unformatted.Count -gt 0) { $unformatted; exit 1 }
git diff --check main..HEAD
go vet ./...
go test -count=1 -p 1 -timeout 10m ./...
```

Expected: no formatting or diff output; vet and all packages exit 0. If the Windows runner cannot execute the aggregate suite, run the same package list serially and record the exact successful commands rather than claiming the failed wrapper passed.

- [ ] **Step 2: Build and verify against the real Weka workspace**

Run:

```powershell
go build -o .superpowers/sdd/goregraph-project-eligibility.exe ./cmd/goregraph
$binary = (Resolve-Path '.superpowers/sdd/goregraph-project-eligibility.exe').Path
Push-Location 'C:\Users\goretzkh\projects\weka'
try {
	$output = @(& $binary workspace build all . --dry-run 2>&1)
	if ($LASTEXITCODE -ne 0) { $output; exit $LASTEXITCODE }
	$projects = @($output | Where-Object { $_ -match '^\d+\. project ' })
	if ($projects.Count -ne 43) { $output; throw "expected 43 projects, got $($projects.Count)" }
	foreach ($excluded in @('hekate/common','hekate/documentation','hekate/email-templates','ita/documentation','ita/testleitfaden','wbp/common','wbp/database','wbp/documentation')) {
		if ($output -match [regex]::Escape($excluded)) { throw "unexpected project: $excluded" }
	}
	$output
} finally {
	Pop-Location
}
```

Expected: exit 0, exactly 43 projects, all eight Git-only repositories absent, and `frontend/frontend-monorepo` still present exactly once.

- [ ] **Step 3: Install and verify GoreGraph 1.3.0 locally**

Run:

```powershell
go install ./cmd/goregraph
$installed = 'C:\Users\goretzkh\go\bin\goregraph.exe'
& $installed version
$shim = Get-Content -Raw -LiteralPath 'C:\Users\goretzkh\scoop\shims\goregraph.shim'
if ($shim -notmatch [regex]::Escape($installed)) { throw 'Scoop shim does not target the installed binary' }
```

Expected: `goregraph 1.3.0`, Windows/amd64, schema 3, and the Scoop shim targets the installed executable.

- [ ] **Step 4: Review final repository state**

Run:

```powershell
git status --short --branch
git log --oneline main..HEAD
```

Expected: clean tracked worktree and the design plus implementation commits on `fix/workspace-project-eligibility`.

- [ ] **Step 5: Push without releasing**

Run:

```powershell
git push -u origin fix/workspace-project-eligibility
git tag --points-at HEAD
```

Expected: branch push succeeds and the tag command prints nothing. Do not run release tooling, do not create a Git tag, and do not merge into `main` without a separate user request.
