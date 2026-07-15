# Exact Cross-Project Code Explorer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an exact, evidence-backed workspace Code Explorer for canonical Java and JavaScript/TypeScript symbols, verified direct usages, and separately classified HTTP reachability paths.

**Architecture:** Project scans add provenance-rich symbol and reference fields to the existing Schema 2 rich outputs. Workspace reconciliation is the only canonical resolver and writes deterministic `symbol-index.json` and `symbol-usages.json`; dashboard, Query, Explain, MCP, and Doctor read those projections instead of resolving independently. Java resolution is FQN plus module/artifact aware, JavaScript/TypeScript resolution is module/export/package/alias aware, and neither language may promote a name-only match to Exact.

**Tech Stack:** Go 1.23+ standard library, additive Schema 2 JSON, generated standalone offline HTML/CSS/vanilla JavaScript, Go tests, Node syntax validation, browser-based interaction/accessibility/visual verification, GitHub CLI.

## Global Constraints

- Work directly on `main`; do not create a feature branch or worktree.
- Use strict test-driven development for every production behavior: write one focused failing test, run it and observe the expected failure, implement the smallest correct behavior, rerun the focused test, then refactor only while green.
- Keep the source target at unreleased `1.3.0`; do not create a tag, GitHub Release, Homebrew publication, Scoop publication, or Winget publication.
- Preserve Schema 2 and all existing readers; every project-output field and workspace output is additive, and existing field meanings remain unchanged.
- Keep scans offline and deterministic; do not execute scanned source, build tools, dependency installers, applications, hooks, or arbitrary configuration commands.
- Do not add dependencies. Use the Go standard library and the repository's existing generated HTML/CSS/JavaScript approach.
- Canonical symbol IDs use `symbol kind + project + module/artifact/package + language + qualified/export name + declaration file`; file or name alone is insufficient.
- Java exactness requires unique package/FQN plus project/module/artifact/dependency evidence. Duplicate FQNs without unique evidence are `AMBIGUOUS`.
- JavaScript/TypeScript exactness requires unique module path, export name, project/workspace-package provenance, and dependency evidence. Identifier-name equality alone is never Exact.
- Keep `direct_reference`, `reached_through_api`, `ambiguous`, and `unresolved` structurally separate in JSON, counts, Query, MCP, Explain, and the dashboard.
- Implement HTTP reachability in 1.3.0 and retain an explicit `transport` field for later gRPC and messaging support.
- Every usage carries provider, consumer project and consumer symbol when known, language, relation kind, source file, one-based line, confidence, resolution, reason, analyzer, evidence IDs, and dependency/artifact evidence when used.
- Missing results are not proof of no usage. Preserve indexed, missing, partial, unsupported, and failed coverage plus limitations for reflection, generated code, dependency injection, proxies, runtime loading, dynamic imports, computed access, and unindexed dependency artifacts.
- A single unreadable project becomes a structured coverage failure while other project indexes continue reconciling. Duplicate IDs, dangling references, invalid evidence, or output-write failures fail reconciliation.
- Existing Architecture selection/domain context/zoom/pan must be restored exactly after leaving Code Explorer; Code Explorer inventories use semantic HTML at normal scale, never a compressed SVG.
- All keyboard controls expose visible focus and correct pressed/selected state; tooltips work on pointer hover and keyboard focus; reduced motion is respected.
- Verify the Code Explorer at 1280×720, 1440×900, and 1920×1080.
- Change only files listed by the active task. Use English source comments and English commit messages with imperative subjects and concise bullet bodies.
- Issue #25 may be closed only after tests and real Weka acceptance pass, the local `goregraph 1.3.0` binary is installed, `main` is pushed, and `main` equals `origin/main`.

---

## Public Data Contract and File Map

### Additive project facts

- `internal/scan/types.go` adds provenance and symbol-to-symbol fields to `RichSymbolRecord`, `RichRelationRecord`, `CallGraphEdgeRecord`, `JavaTypeRecord`, and `NodePackageRecord`. Existing JSON field names and values remain intact.
- `internal/scan/symbol_facts.go` owns language-neutral raw declaration/reference facts, normalization helpers, stable project declaration IDs, and deterministic sorting.
- `internal/scan/symbol_java.go` owns Java declarations and type/call reference extraction.
- `internal/scan/symbol_script.go` owns JavaScript/TypeScript declarations, imports, re-exports, JSX/type/call references, module resolution, workspace packages, and TypeScript alias resolution.
- `internal/scan/rich_build.go` projects symbol facts into additive `symbols-full.json`, `relations-full.json`, and `callgraph.json` fields.
- `internal/scan/evidence_build.go` gives declarations and symbol references stable source evidence IDs.
- `internal/scan/scan.go` collects symbol facts without executing source and writes the enriched existing outputs.

### Canonical workspace projection

- `internal/scan/symbol_projection.go` owns the public canonical symbol, usage, coverage, API-step, index, and usage-index records plus validation-independent stable-ID functions.
- `internal/scan/workspace_symbols.go` loads project facts, namespaces evidence, resolves exact/ambiguous/unresolved direct references, and creates deterministic `WorkspaceSymbolIndexRecord` and `WorkspaceSymbolUsageIndexRecord` values.
- `internal/scan/workspace_symbol_api.go` derives `reached_through_api` records from existing contracts, feature flows, endpoint traces, and implementation steps.
- `internal/scan/workspace_reconcile.go` loads the enriched project files, writes `.goregraph-workspace/symbol-index.json` and `.goregraph-workspace/symbol-usages.json`, and passes both to dashboard generation.

### Consumers

- `internal/doctor/symbol_projection.go` validates the canonical projection; `internal/doctor/doctor.go` invokes it for project- or workspace-root Doctor runs.
- `internal/agent/service.go`, `internal/query/query.go`, `internal/cli/cli.go`, and `internal/mcp/mcp.go` expose the same task operations and terminology.
- `internal/scan/workspace_dashboard.go`, `internal/scan/workspace_dashboard_template.go`, `internal/scan/workspace_dashboard_script.go`, and `internal/scan/workspace_dashboard_styles.go` render the offline semantic Code Explorer.
- `README.md`, `COMMANDS.md`, `docs/OUTPUTS.md`, and `docs/RELEASE.md` document the feature, exactness boundaries, commands, outputs, integration depth, and unreleased state.

### Exact public Go records

Add these records in `internal/scan/symbol_projection.go`; later tasks must use these field names unchanged:

```go
type SymbolUsageCategory string

const (
	SymbolUsageDirectReference  SymbolUsageCategory = "direct_reference"
	SymbolUsageReachedThroughAPI SymbolUsageCategory = "reached_through_api"
	SymbolUsageAmbiguous        SymbolUsageCategory = "ambiguous"
	SymbolUsageUnresolved       SymbolUsageCategory = "unresolved"
)

type SymbolResolution string

const (
	SymbolResolutionExact      SymbolResolution = "EXACT"
	SymbolResolutionAmbiguous  SymbolResolution = "AMBIGUOUS"
	SymbolResolutionUnresolved SymbolResolution = "UNRESOLVED"
)

type CanonicalSymbolRecord struct {
	ID              string     `json:"id"`
	Project         string     `json:"project"`
	Service         string     `json:"service,omitempty"`
	ProjectKind     string     `json:"project_kind,omitempty"`
	Module          string     `json:"module,omitempty"`
	Package         string     `json:"package,omitempty"`
	Application     string     `json:"application,omitempty"`
	WorkspacePackage string    `json:"workspace_package,omitempty"`
	Artifact        string     `json:"artifact,omitempty"`
	Language        string     `json:"language"`
	Kind            string     `json:"kind"`
	Name            string     `json:"name"`
	QualifiedName   string     `json:"qualified_name"`
	ExportName      string     `json:"export_name,omitempty"`
	DeclarationFile string     `json:"declaration_file"`
	DeclarationLine int        `json:"declaration_line"`
	EvidenceIDs     []string   `json:"evidence_ids,omitempty"`
	Analyzer        string     `json:"analyzer"`
	Confidence      Confidence `json:"confidence"`
	Coverage        Coverage   `json:"coverage"`
	Limitations     []string   `json:"limitations,omitempty"`
}

type SymbolAPIPathStepRecord struct {
	Position    int      `json:"position"`
	Kind        string   `json:"kind"`
	Project     string   `json:"project"`
	SymbolID    string   `json:"symbol_id,omitempty"`
	Label       string   `json:"label"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type CanonicalSymbolUsageRecord struct {
	ID                 string              `json:"id"`
	ProviderSymbolID   string              `json:"provider_symbol_id,omitempty"`
	ConsumerProject    string              `json:"consumer_project"`
	ConsumerSymbolID   string              `json:"consumer_symbol_id,omitempty"`
	Category           SymbolUsageCategory `json:"category"`
	Language           string              `json:"language"`
	RelationKind       string              `json:"relation_kind"`
	TargetQualifiedName string             `json:"target_qualified_name,omitempty"`
	TargetModule       string              `json:"target_module,omitempty"`
	TargetExport       string              `json:"target_export,omitempty"`
	SourceFile         string              `json:"source_file"`
	SourceLine         int                 `json:"source_line"`
	Confidence         Confidence           `json:"confidence"`
	Resolution         SymbolResolution     `json:"resolution"`
	Reason             string               `json:"reason"`
	Analyzer           string               `json:"analyzer"`
	EvidenceIDs        []string             `json:"evidence_ids,omitempty"`
	CandidateSymbolIDs []string             `json:"candidate_symbol_ids,omitempty"`
	DependencyEvidence []string             `json:"dependency_evidence,omitempty"`
	Transport          string               `json:"transport,omitempty"`
	APIPath            []SymbolAPIPathStepRecord `json:"api_path,omitempty"`
	Limitations        []string             `json:"limitations,omitempty"`
}

type SymbolCoverageRecord struct {
	Project     string   `json:"project"`
	Language    string   `json:"language"`
	Capability  string   `json:"capability"`
	Coverage    Coverage `json:"coverage"`
	Reason      string   `json:"reason"`
	Limitations []string `json:"limitations,omitempty"`
}

type WorkspaceSymbolIndexRecord struct {
	SchemaVersion int                     `json:"schema_version"`
	Generated     string                  `json:"generated,omitempty"`
	Root          string                  `json:"root,omitempty"`
	Symbols       []CanonicalSymbolRecord `json:"symbols"`
	Coverage      []SymbolCoverageRecord  `json:"coverage"`
}

type WorkspaceSymbolUsageIndexRecord struct {
	SchemaVersion int                          `json:"schema_version"`
	Generated     string                       `json:"generated,omitempty"`
	Root          string                       `json:"root,omitempty"`
	Usages        []CanonicalSymbolUsageRecord `json:"usages"`
	Coverage      []SymbolCoverageRecord       `json:"coverage"`
}
```

Stable IDs use these exact exported functions:

```go
func StableWorkspaceSymbolID(kind, project, scope, language, qualifiedName, declarationFile string) string
func StableWorkspaceUsageID(providerSymbolID, consumerProject, consumerSymbolID string, category SymbolUsageCategory, relationKind, targetIdentity, sourceFile string, sourceLine int) string
func WorkspaceEvidenceID(project, localEvidenceID string) string
```

`scope` is the first non-empty value from artifact, workspace package, module, package, then application. `WorkspaceEvidenceID` returns `project + "#" + localEvidenceID`; project paths may not contain `#` and reconciliation rejects one that does.

---

### Task 1: Freeze Additive Symbol Contracts and Stable Identities

**Files:**
- Create: `internal/scan/symbol_projection.go`
- Create: `internal/scan/symbol_projection_test.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/rich_build.go`
- Modify: `internal/scan/rich_graph_test.go`

**Interfaces:**
- Consumes: existing `Confidence`, `Coverage`, `RichSymbolRecord`, `RichRelationRecord`, and `CallGraphEdgeRecord`.
- Produces: all public records and stable-ID functions in the Public Data Contract; additive project fields consumed by Tasks 2–6.

Add these fields without renaming existing fields:

```go
// RichSymbolRecord additions
QualifiedName string     `json:"qualified_name,omitempty"`
Package       string     `json:"package,omitempty"`
Module        string     `json:"module,omitempty"`
Application   string     `json:"application,omitempty"`
WorkspacePackage string  `json:"workspace_package,omitempty"`
Artifact      string     `json:"artifact,omitempty"`
ExportName    string     `json:"export_name,omitempty"`
DeclarationID string     `json:"declaration_id,omitempty"`
EvidenceIDs   []string   `json:"evidence_ids,omitempty"`
Analyzer      string     `json:"analyzer,omitempty"`
Confidence    Confidence `json:"symbol_confidence,omitempty"`
Coverage      Coverage   `json:"coverage,omitempty"`
Limitations   []string   `json:"limitations,omitempty"`

// RichRelationRecord additions
FromSymbolID       string           `json:"from_symbol_id,omitempty"`
ToSymbolID         string           `json:"to_symbol_id,omitempty"`
TargetQualifiedName string          `json:"target_qualified_name,omitempty"`
TargetModule       string           `json:"target_module,omitempty"`
TargetExport       string           `json:"target_export,omitempty"`
Resolution         SymbolResolution `json:"resolution,omitempty"`
Reason             string           `json:"reason,omitempty"`
CandidateSymbolIDs []string         `json:"candidate_symbol_ids,omitempty"`
DependencyEvidence []string         `json:"dependency_evidence,omitempty"`

// CallGraphEdgeRecord additions
FromSymbolID       string           `json:"from_symbol_id,omitempty"`
ToSymbolID         string           `json:"to_symbol_id,omitempty"`
TargetQualifiedName string          `json:"target_qualified_name,omitempty"`
Resolution         SymbolResolution `json:"resolution,omitempty"`
CandidateSymbolIDs []string         `json:"candidate_symbol_ids,omitempty"`
```

Define these shared test helpers once in `internal/scan/symbol_facts_test.go` so every assertion named below has a concrete implementation:

```go
func assertRichDeclaration(t *testing.T, records []RichSymbolRecord, kind, qualifiedName, artifact string) RichSymbolRecord {
	t.Helper()
	for _, record := range records {
		if record.Kind == kind && record.QualifiedName == qualifiedName && record.Artifact == artifact {
			return record
		}
	}
	t.Fatalf("missing %s declaration %s in %#v", kind, qualifiedName, records)
	return RichSymbolRecord{}
}

func assertScriptReference(t *testing.T, records []RichRelationRecord, kind, module, exportName string) RichRelationRecord {
	t.Helper()
	for _, record := range records {
		if record.Type == kind && record.TargetModule == module && record.TargetExport == exportName {
			return record
		}
	}
	t.Fatalf("missing %s reference %s#%s in %#v", kind, module, exportName, records)
	return RichRelationRecord{}
}
```

- [ ] **Step 1: Write failing stable-identity and legacy-compatibility tests**

```go
func TestStableWorkspaceSymbolIDUsesEveryCanonicalIdentityPart(t *testing.T) {
	base := StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java")
	cases := []string{
		StableWorkspaceSymbolID("interface", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-task", "com.weka:users", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users-v2", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "typescript", "com.weka.UserService", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService.Inner", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService", "src/test/java/com/weka/UserService.java"),
	}
	for _, candidate := range cases {
		if candidate == base { t.Fatalf("identity input was ignored: %q", candidate) }
	}
	if again := StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java"); again != base {
		t.Fatalf("stable ID changed: %q != %q", again, base)
	}
}

func TestRichSymbolAndRelationRemainLegacyReadable(t *testing.T) {
	legacySymbol := []byte(`{"id":"s","name":"UserService","kind":"class","language":"java","file":"UserService.java","line":3}`)
	legacyRelation := []byte(`{"id":"r","from":"Client.java","to":"UserService.java","type":"imports_internal","confidence":"EXTRACTED","confidence_score":1}`)
	var symbol RichSymbolRecord
	var relation RichRelationRecord
	if err := json.Unmarshal(legacySymbol, &symbol); err != nil { t.Fatal(err) }
	if err := json.Unmarshal(legacyRelation, &relation); err != nil { t.Fatal(err) }
	if symbol.Name != "UserService" || relation.From != "Client.java" { t.Fatalf("legacy fields changed: %#v %#v", symbol, relation) }
}

func TestStableWorkspaceUsageIDSeparatesCategoryTargetAndLocation(t *testing.T) {
	base := StableWorkspaceUsageID("symbol:provider", "frontend/app", "symbol:consumer", SymbolUsageDirectReference, "imports_type", "symbol:provider", "src/app.ts", 7)
	variants := []string{
		StableWorkspaceUsageID("symbol:provider", "frontend/app", "symbol:consumer", SymbolUsageReachedThroughAPI, "http_path", "symbol:provider", "src/app.ts", 7),
		StableWorkspaceUsageID("", "frontend/app", "symbol:consumer", SymbolUsageUnresolved, "imports_type", "@weka/users#UserService", "src/app.ts", 7),
		StableWorkspaceUsageID("symbol:provider", "frontend/app", "symbol:consumer", SymbolUsageDirectReference, "imports_type", "symbol:provider", "src/app.ts", 8),
	}
	for _, candidate := range variants {
		if candidate == base { t.Fatalf("usage identity input was ignored: %q", candidate) }
	}
}
```

- [ ] **Step 2: Run the focused tests and observe RED**

Run: `go test ./internal/scan -run 'TestStableWorkspaceSymbolIDUsesEveryCanonicalIdentityPart|TestRichSymbolAndRelationRemainLegacyReadable' -count=1`

Expected: compilation fails because `StableWorkspaceSymbolID`, `SymbolUsageCategory`, `SymbolResolution`, and the additive fields do not exist.

- [ ] **Step 3: Add the exact public records, constants, validation, and stable hashes**

Implement the records above. Hash normalized slash-separated, trimmed identity parts with SHA-256 and return `symbol:` plus the first 32 lowercase hex characters; usages return `usage:` plus the first 32. `targetIdentity` is the provider ID for exact records, the sorted candidate-ID join for ambiguity, and the attempted qualified/module/export identity for unresolved records, preventing same-line unresolved references from colliding. Reject empty kind/project/language/qualified name/declaration file in projection validation, not by silently changing identity input. Sort candidate IDs, evidence IDs, limitations, and dependency evidence before computing or emitting records.

- [ ] **Step 4: Preserve rich graph behavior while carrying additive fields**

Keep legacy `ID`, `Name`, `From`, `To`, `Type`, and graph-node construction unchanged. Add a test that marshaling an enriched symbol includes `qualified_name`, while marshaling a legacy-only symbol still omits it and still builds the same graph node ID.

- [ ] **Step 5: Run focused and package tests**

Run: `gofmt -w internal/scan/symbol_projection.go internal/scan/symbol_projection_test.go internal/scan/types.go internal/scan/rich_build.go internal/scan/rich_graph_test.go && go test ./internal/scan -run 'TestStableWorkspaceSymbol|TestStableWorkspaceUsage|TestRichSymbol|TestRichGraph' -count=1 && go test ./internal/scan -count=1`

Expected: PASS; existing rich-output tests remain green.

- [ ] **Step 6: Commit the contract**

```bash
git add internal/scan/symbol_projection.go internal/scan/symbol_projection_test.go internal/scan/types.go internal/scan/rich_build.go internal/scan/rich_graph_test.go
git commit -m "Define canonical symbol contracts" -m "- Add stable workspace symbol and usage identities" -m "- Extend Schema 2 rich records without changing legacy fields"
```

### Task 2: Extract Java Canonical Declarations and Reference Facts

**Files:**
- Create: `internal/scan/symbol_facts.go`
- Create: `internal/scan/symbol_facts_test.go`
- Create: `internal/scan/symbol_java.go`
- Create: `internal/scan/symbol_java_test.go`
- Create: `internal/scan/extract_gradle.go`
- Create: `internal/scan/extract_gradle_test.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/extract_java.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/evidence_build.go`

**Interfaces:**
- Consumes: Task 1 additive rich fields and existing `JavaSourceRecord`, Maven packages, statically extractable Gradle metadata, imports, fields, methods, annotations, and calls.
- Produces: `ProjectSymbolFacts`, `ExtractJavaSymbolFacts`, enriched Java `symbols-full.json`, `relations-full.json`, and `callgraph.json` facts for workspace reconciliation.

Define the internal facts exactly:

```go
type ProjectSymbolFacts struct {
	Declarations []RichSymbolRecord
	References   []RichRelationRecord
}

func ExtractJavaSymbolFacts(source JavaSourceRecord, body string, workspace WorkspaceIndex) ProjectSymbolFacts
func MergeProjectSymbolFacts(target *ProjectSymbolFacts, next ProjectSymbolFacts)
func FinalizeProjectSymbolFacts(files []FileRecord, workspace WorkspaceIndex, facts ProjectSymbolFacts) ProjectSymbolFacts
```

Extend `JavaTypeRecord` additively with `Owner string`, `QualifiedName string`, and `EndLine int`. Use a brace-depth owner stack so `Outer.Inner` is stable and top-level declarations after a nested type do not inherit the wrong owner.

Add `GradlePackages []GradlePackageRecord` to `WorkspaceIndex` with this exact record:

```go
type GradlePackageRecord struct {
	Path         string                   `json:"path"`
	Group        string                   `json:"group,omitempty"`
	Artifact     string                   `json:"artifact,omitempty"`
	Dependencies []GradleDependencyRecord `json:"dependencies,omitempty"`
}

type GradleDependencyRecord struct {
	Group    string `json:"group"`
	Artifact string `json:"artifact"`
	Version  string `json:"version,omitempty"`
	Scope    string `json:"scope,omitempty"`
}
```

`extract_gradle.go` accepts only literal Groovy/Kotlin assignments and dependency coordinates: `group = "com.weka"`, `rootProject.name = "users-api"`, `implementation("com.weka:shared:1.2")`, and `implementation 'com.weka:shared:1.2'`. Computed expressions stay absent and add a partial-coverage limitation; the scan never invokes Gradle.

- [ ] **Step 1: Write failing Java declaration tests**

Use one source fixture containing package `com.weka.users`, class `Outer`, nested record `Snapshot`, interface `Port`, enum `State`, and a second top-level class. Assert kinds `class`, `record`, `interface`, `enum`; qualified names `com.weka.users.Outer`, `com.weka.users.Outer.Snapshot`, and `com.weka.users.Port`; declaration files/one-based lines; artifact `com.weka:users-api` from the nearest Maven package; and stable declaration IDs.

```go
facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{MavenPackages: []MavenPackageRecord{{Path: "pom.xml", GroupID: "com.weka", ArtifactID: "users-api"}}})
assertRichDeclaration(t, facts.Declarations, "record", "com.weka.users.Outer.Snapshot", "com.weka:users-api")
```

- [ ] **Step 2: Verify declaration RED**

Run: `go test ./internal/scan -run TestExtractJavaCanonicalDeclarations -count=1`

Expected: compilation fails because `ExtractJavaSymbolFacts` and the new Java type provenance fields are missing.

- [ ] **Step 3: Implement declaration extraction and evidence**

Create one rich declaration per Java class/interface/enum/record. Set `DeclarationID` from kind, language, qualified name, module/artifact scope, and file; set analyzer `java-source`; confidence `EXACT`; coverage `COMPLETE`; add declaration evidence with method `declaration` and reason `java <kind> declaration`. Do not replace existing simple symbols.

- [ ] **Step 4: Write failing Java reference matrix tests**

Create provider declarations and a consumer with exact import, fully qualified annotation, field `List<UserService[]>`, constructor parameter, return `Optional<UserService>`, `extends Base<UserService>`, `implements UserPort`, `new UserService()`, static import, and `userService.find()` where the field type resolves the owner. Assert reference kinds:

```text
imports_type, annotation_type, field_type, parameter_type, return_type,
extends_type, implements_type, instantiates, static_import, calls_method_owner
```

Each record must contain source file/line, `FromSymbolID`, normalized `TargetQualifiedName`, analyzer, reason, and evidence. Generics, arrays, wildcards, annotations, and nested `$`/`.` names normalize without discarding the owning declaration.

Add a focused Gradle fixture and require the Java declaration artifact `com.weka:users-api` plus dependency evidence `gradle:com.weka:users-api -> com.weka:shared` from literal `build.gradle`/`settings.gradle` metadata. Add a computed-coordinate fixture and require `PARTIAL` coverage rather than guessed coordinates.

- [ ] **Step 5: Verify reference RED**

Run: `go test ./internal/scan -run 'TestExtractJavaReferenceMatrix|TestJavaNestedAndGenericTypeNormalization' -count=1`

Expected: FAIL because Java facts contain declarations but no typed reference records.

- [ ] **Step 6: Implement the minimum Java reference extractor**

Resolve simple names only to a unique explicit import, the same package, `java.lang`, or a uniquely declared local type. Retain the normalized target string even when project-local resolution is unavailable. Static imports record the declaring owner, not the member as a class. A call edge gets `ToSymbolID` only when the receiver's declared type uniquely identifies a project declaration; otherwise retain `TargetQualifiedName` and leave workspace resolution for Task 5. Do not match on simple name across projects.

- [ ] **Step 7: Integrate Java facts into existing outputs**

Collect `ProjectSymbolFacts` during `scanProject`, finalize after Maven metadata is known, merge declarations into `richSymbols`, merge references into `richRelations`, and copy resolved class-owner IDs into matching `CallGraphEdgeRecord` values. Extend `LinkEvidenceReferences` to receive `[]RichSymbolRecord` and link declaration evidence before writing JSON.

- [ ] **Step 8: Run Java, evidence, scan, and legacy tests**

Run: `gofmt -w internal/scan && go test ./internal/scan -run 'TestExtractJava|TestJava|TestExtractGradle|TestRunWrites|TestRunExtracts|TestEvidence|TestRich' -count=1 && go test ./internal/doctor -count=1`

Expected: PASS; `symbols.json`, `relations.json`, and existing endpoint/callgraph behavior are unchanged.

- [ ] **Step 9: Commit Java facts**

```bash
git add internal/scan/symbol_facts.go internal/scan/symbol_facts_test.go internal/scan/symbol_java.go internal/scan/symbol_java_test.go internal/scan/extract_gradle.go internal/scan/extract_gradle_test.go internal/scan/types.go internal/scan/extract_java.go internal/scan/scan.go internal/scan/evidence_build.go
git commit -m "Extract exact Java symbol facts" -m "- Record canonical Java declarations and typed references" -m "- Preserve nested, generic, artifact, and source evidence provenance"
```

### Task 3: Extract and Resolve JavaScript/TypeScript Symbol Facts

**Files:**
- Create: `internal/scan/symbol_script.go`
- Create: `internal/scan/symbol_script_test.go`
- Modify: `internal/scan/symbol_facts.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/code_intelligence.go`

**Interfaces:**
- Consumes: `ProjectSymbolFacts`, project file inventory/bodies, `NodePackageRecord`, existing functions/components/calls, and API contract caller evidence.
- Produces: JS/TS class/interface/type/enum/function/component declarations and exact module-backed imports, re-exports, JSX/type/call references.

Add internal public-within-package interfaces:

```go
type ScriptResolutionConfig struct {
	BaseURL string
	Paths   map[string][]string
}

func ExtractScriptSymbolFacts(file FileRecord, body string) ProjectSymbolFacts
func ResolveScriptSymbolFacts(files []FileRecord, packages []NodePackageRecord, configs map[string]ScriptResolutionConfig, facts ProjectSymbolFacts) ProjectSymbolFacts
func ExtractScriptResolutionConfig(path, body string) (ScriptResolutionConfig, bool)
```

Extend `NodePackageRecord` with additive `Exports map[string][]string` and `Types string`. Normalize a string export to a one-element slice; for object/conditional exports retain only statically known string leaves, sort them, and mark multiple surviving targets ambiguous rather than selecting by object order. Extend the scan-only `Index` with source-independent `ScriptConfigs map[string]ScriptResolutionConfig`; do not write raw source bodies into JSON.

- [ ] **Step 1: Write failing declaration and import-shape tests**

Cover exported/default classes, interfaces, type aliases, enums, named functions, exported arrows, React components, named/default/namespace/type-only imports, static-string `import()`, and re-exports. Assert module identity uses the normalized project-relative file without extension plus export name, for example `src/components/UserCard#UserCard`.

```go
facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/components/UserCard.tsx", Language: "typescript"}, body)
assertRichDeclaration(t, facts.Declarations, "component", "src/components/UserCard#UserCard", "")
assertScriptReference(t, facts.References, "imports_type", "@models/user", "User")
```

- [ ] **Step 2: Verify JS/TS extraction RED**

Run: `go test ./internal/scan -run 'TestExtractScriptCanonicalDeclarations|TestExtractScriptImportAndReexportShapes' -count=1`

Expected: compilation fails because script symbol fact extraction and resolution configuration do not exist.

- [ ] **Step 3: Implement declarations and static binding facts**

Implement conservative lexical scanning that ignores line/block comments and string/template contents except statically known module specifiers. Exported declarations get `ExportName`; unexported declarations remain selectable within their project but may only be linked across modules through a proven export/re-export. Default anonymous exports are not selectable unless a stable local declaration name exists. Mark computed imports, runtime aliases, and dynamic property access unresolved with a limitation; never invent an export.

- [ ] **Step 4: Write failing module-resolution tests**

Use a fixture with relative `./UserCard`, directory `./models` resolving `index.ts`, package export `@weka/ui/user`, workspace package dependency, TypeScript `baseUrl`, paths `@models/* -> src/models/*`, and two unrelated `UserService` exports. Assert relative/alias/workspace-package resolution is Exact, re-export chains resolve to the original declaration, and the unrelated same-name export is excluded.

- [ ] **Step 5: Verify resolver RED**

Run: `go test ./internal/scan -run 'TestResolveScriptRelativeAliasAndWorkspaceImports|TestScriptReexportsPreserveOriginalDeclaration|TestScriptSameNameWithoutModuleEvidenceIsNotExact' -count=1`

Expected: FAIL because references still contain unresolved module specifiers and no canonical `ToSymbolID`.

- [ ] **Step 6: Implement deterministic module and export resolution**

For relative imports, try the exact file, `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.cjs`, then `/index` with the same extensions. For aliases, choose the nearest `tsconfig.json`/`jsconfig.json`, substitute one `*`, and resolve with the same file rules. Parse standard JSON plus deterministic comment/trailing-comma removal used only for these config files; malformed configs create `PARTIAL` coverage and do not abort other files. For workspace packages, require the consumer package dependency and provider package `name` plus `exports`, `types`, or source module evidence. Resolve re-export chains with a visited set; cycles become unresolved with reason `cyclic re-export`. Sort all path candidates before deciding uniqueness.

- [ ] **Step 7: Write failing usage-form tests**

Cover explicit TS type references, JSX `<UserCard>`, `new UserService()`, imported `loadUser()`, namespace `api.loadUser()`, and a local call. Assert `type_reference`, `renders_component`, `instantiates`, and `calls_export` relation kinds identify the imported binding's module/export. Assert computed `registry[name]()` and non-static `import(path)` remain unresolved.

- [ ] **Step 8: Implement usage binding and integrate scan outputs**

Bind references to the nearest containing selectable declaration so `FromSymbolID` identifies the consumer component/function/class. Merge script declarations/references into the same project facts as Java. Preserve existing `CodeFunctionRecord`, callgraph, route, and API-contract behavior; add canonical IDs to existing call edges only when the imported/local owner is exact.

- [ ] **Step 9: Run focused, package, and cross-language regression tests**

Run: `gofmt -w internal/scan && go test ./internal/scan -run 'TestExtractScript|TestResolveScript|TestScript|TestRunExtractsSymbolsRelationsAndGraph|TestFrontend|TestCode' -count=1 && go test ./internal/scan -count=1`

Expected: PASS; both Java and JS/TS facts are sorted by stable declaration/reference ID independent of file-walk order.

- [ ] **Step 10: Commit JS/TS facts**

```bash
git add internal/scan/symbol_script.go internal/scan/symbol_script_test.go internal/scan/symbol_facts.go internal/scan/types.go internal/scan/workspace.go internal/scan/scan.go internal/scan/code_intelligence.go
git commit -m "Extract exact JavaScript and TypeScript symbols" -m "- Resolve imports, re-exports, aliases, workspace packages, JSX, and calls" -m "- Keep dynamic and name-only references explicitly unresolved"
```

### Task 4: Reconcile Deterministic Workspace Symbols and Direct Usages

**Files:**
- Create: `internal/scan/workspace_symbols.go`
- Create: `internal/scan/workspace_symbols_test.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/types.go`

**Interfaces:**
- Consumes: enriched `symbols-full.json`, `relations-full.json`, `callgraph.json`, `maven-graph.json`, `package-graph.json`, project registry, capabilities, and evidence.
- Produces: `BuildWorkspaceSymbolProjection(registry WorkspaceRegistryRecord, projects []workspaceIndexProject, generated string) (WorkspaceSymbolIndexRecord, WorkspaceSymbolUsageIndexRecord, error)` and the two root workspace JSON files.

Extend `workspaceIndexProject` with `symbols []RichSymbolRecord`, `relations []RichRelationRecord`, `callGraph CallGraphRecord`, `maven MavenGraphRecord`, `packages PackageGraphRecord`, `evidence []EvidenceRecord`, and `loadFailures []string`. The enriched symbol/module/artifact fields carry package export identity; Maven/package graph edges provide cross-project dependency evidence. A missing optional file produces coverage. Any malformed project output needed by symbol reconciliation appends a project-scoped failure and leaves that project's affected facts empty without discarding other projects; registry/global output failures still fail reconciliation.

- [ ] **Step 1: Write the failing multi-project Java exactness test**

Build an in-memory/provider fixture for `microservices/ms-userservice` declaring `com.weka.wbp.api.userservice.service.UserService`, an exact consumer with Maven dependency `com.weka:users-api`, and `microservices/ms-cadastertask` declaring its own same-simple-name class. Assert one `direct_reference` points to the selected provider and the unrelated declaration produces no usage.

- [ ] **Step 2: Verify Java workspace RED**

Run: `go test ./internal/scan -run TestWorkspaceSymbolsExcludeUnrelatedJavaSameName -count=1`

Expected: compilation fails because `BuildWorkspaceSymbolProjection` does not exist.

- [ ] **Step 3: Implement canonical symbol namespacing and unique direct resolution**

Create canonical symbols from project declarations with `StableWorkspaceSymbolID`. Convert project-local evidence IDs with `WorkspaceEvidenceID`. Build indexes by declaration ID, Java FQN, Java FQN+artifact, JS module+export+project, and workspace package+export. Emit Exact only for one uniquely evidenced provider. When multiple providers survive, emit one `ambiguous` record for each candidate so each selected candidate can disclose the uncertainty; set `ProviderSymbolID` to that candidate and put the complete sorted candidate set in every copy. Emit one unscoped `unresolved` record with an empty `ProviderSymbolID` only when no candidate exists, retaining its attempted `TargetQualifiedName`, `TargetModule`, and `TargetExport`. Uncertainty records never enter default Exact counts.

- [ ] **Step 4: Write failing duplicate-FQN and JS/TS workspace tests**

Assert duplicate Java FQNs resolve by Maven dependency to the correct artifact, become `AMBIGUOUS` without artifact evidence, JS/TS workspace-package imports resolve only through declared package dependency/export evidence, and unrelated same-name exports never become Exact.

- [ ] **Step 5: Implement dependency-aware disambiguation**

Java dependency evidence strings use `maven:<consumer-artifact> -> <provider-artifact>` or `gradle:<consumer-artifact> -> <provider-artifact>`. JS/TS strings use `node:<consumer-package> -> <provider-package>`. If multiple providers remain after dependency filtering, keep every sorted candidate and reason `multiple indexed declarations remain after dependency filtering`. Never fall back to first scan order.

- [ ] **Step 6: Write failing determinism, duplicate-ID, and partial-read tests**

Build the same project slice in forward and reverse order; marshal both projections after blanking `Generated` and require byte equality. Inject duplicate canonical declarations and require an error containing the duplicate ID. Inject one malformed project fact and require valid projects plus a `FAILED` coverage record; inject a root write failure in the reconciliation integration test and require command failure without half-written symbol files.

- [ ] **Step 7: Implement sorting, coverage, and atomic paired output writes**

Sort symbols by ID, usages by ID, coverage by project/language/capability, and every nested string slice. Record `COMPLETE` for supported exact facts, `PARTIAL` with language limitations, `UNAVAILABLE` for unsupported languages, `FAILED` for unreadable project facts, and missing projects from registry as explicit coverage. Use exact limitation keys `reflection`, `dependency_injection`, `runtime_proxy`, `generated_code`, `runtime_loading`, and `unindexed_dependency_artifact` for Java; use `dynamic_import`, `computed_property`, `bundler_only_alias`, `generated_code`, and `unindexed_workspace_package` for JS/TS. Write and validate both projections in one temporary directory, move any old pair to backup names, rename the complete new pair into place, and restore both backups if either final rename fails. Remove staging/backups only after the pair is live; this prevents a mixed old/new pair.

- [ ] **Step 8: Wire reconciliation and generated-file accounting**

Load the new project fields in `loadWorkspaceIndexes`, call the builder after feature flows exist, write `symbol-index.json` and `symbol-usages.json`, and add both to workspace clean planning. Do not add them to per-project `GeneratedFiles`; they are root workspace outputs.

- [ ] **Step 9: Run reconciliation and determinism tests**

Run: `gofmt -w internal/scan && go test ./internal/scan -run 'TestWorkspaceSymbol|TestScanCreatesWorkspace|TestLaterBackendScan|TestWorkspaceClean' -count=1 && go test ./internal/scan -count=1`

Expected: PASS; reversing project discovery order produces identical symbol content except the shared generated timestamp.

- [ ] **Step 10: Commit direct workspace resolution**

```bash
git add internal/scan/workspace_symbols.go internal/scan/workspace_symbols_test.go internal/scan/workspace_reconcile.go internal/scan/scan.go internal/scan/types.go
git commit -m "Reconcile exact workspace symbol usages" -m "- Resolve Java and JavaScript/TypeScript references with dependency evidence" -m "- Preserve deterministic ambiguity and coverage records"
```

### Task 5: Derive Complete HTTP Reachability Paths

**Files:**
- Create: `internal/scan/workspace_symbol_api.go`
- Create: `internal/scan/workspace_symbol_api_test.go`
- Modify: `internal/scan/workspace_symbols.go`
- Modify: `internal/scan/workspace_reconcile.go`

**Interfaces:**
- Consumes: canonical symbols, workspace contract matches, feature flows, endpoint traces, frontend caller/component evidence, and Spring implementation steps.
- Produces: `BuildWorkspaceSymbolAPIUsages(symbols WorkspaceSymbolIndexRecord, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, traces WorkspaceEndpointTraceIndexRecord) []CanonicalSymbolUsageRecord`.

- [ ] **Step 1: Write the failing full-chain test**

Use a TypeScript `UserPage`/`loadUser`, resolved HTTP contract, Spring `UserController.get`, and implementation steps `UserService.find -> UserRepository.find`. Select the canonical `UserService` symbol and assert exactly one `reached_through_api` usage with transport `http` and ordered kinds:

```text
frontend_symbol, api_helper, http_contract, workspace_contract,
spring_route, spring_handler, java_implementation, selected_symbol
```

Assert every available file/line/evidence value survives and the record is not included in the direct-reference count.

- [ ] **Step 2: Verify HTTP RED**

Run: `go test ./internal/scan -run TestWorkspaceSymbolAPIUsageContainsFullFrontendToJavaChain -count=1`

Expected: compilation fails because `BuildWorkspaceSymbolAPIUsages` is missing.

- [ ] **Step 3: Implement Java-provider HTTP reachability**

Join only resolved workspace contracts. Match backend steps to a Java symbol by backend project plus exact declaration file and qualified/simple owner evidence; if the implementation symbol cannot be uniquely selected, emit `ambiguous` or `unresolved`, not `reached_through_api`. Keep the handler and selected implementation step separately even when they share a file.

- [ ] **Step 4: Write failing JS/TS-origin and separation tests**

Select the canonical frontend component/function and assert outbound API paths originate only when the feature flow contains that exact caller/component file and name. Add a project that both directly imports a shared symbol and calls its API; assert two usage IDs in different categories and separate counts.

- [ ] **Step 5: Implement JS/TS-origin paths and deterministic deduplication**

Resolve frontend caller/component to canonical symbols by project, declaration file, and exact name/export evidence. Deduplicate paths by provider symbol, consumer symbol, contract ID, and selected implementation step. Sort paths by usage ID and positions from zero upward. Carry limitations when frontend route context is partial rather than silently upgrading confidence.

Append the returned API records to `WorkspaceSymbolUsageIndexRecord.Usages`, sort/deduplicate the combined direct/API/uncertainty slice once, validate the complete pair, and only then publish both workspace files and dashboard payload. This is the single merge point for the two relationship categories.

- [ ] **Step 6: Run HTTP and existing flow regression tests**

Run: `gofmt -w internal/scan && go test ./internal/scan -run 'TestWorkspaceSymbolAPI|TestBuildWorkspaceEndpointTraces|TestWorkspaceFeatureFlow|TestCanonicalFeatureFlow' -count=1 && go test ./internal/scan -count=1`

Expected: PASS; existing contract, feature-flow, endpoint-trace, and service-map JSON remains compatible.

- [ ] **Step 7: Commit API reachability**

```bash
git add internal/scan/workspace_symbol_api.go internal/scan/workspace_symbol_api_test.go internal/scan/workspace_symbols.go internal/scan/workspace_reconcile.go
git commit -m "Add symbol HTTP reachability paths" -m "- Join frontend callers through contracts and Spring implementation steps" -m "- Keep API reachability separate from direct code references"
```

### Task 6: Validate Symbol Projection Integrity with Doctor

**Files:**
- Create: `internal/doctor/symbol_projection.go`
- Create: `internal/doctor/symbol_projection_test.go`
- Modify: `internal/doctor/doctor.go`

**Interfaces:**
- Consumes: `WorkspaceSymbolIndexRecord`, `WorkspaceSymbolUsageIndexRecord`, registry/project evidence files.
- Produces: `ValidateSymbolProjection(index scan.WorkspaceSymbolIndexRecord, usages scan.WorkspaceSymbolUsageIndexRecord, knownEvidence map[string]bool) error` and workspace-root Doctor support.

- [ ] **Step 1: Write table-driven failing integrity tests**

Create cases for duplicate symbol ID, duplicate usage ID, dangling provider, dangling known consumer, dangling ambiguous candidate, dangling evidence, unknown project/source reference, unordered/non-contiguous API positions, missing selected final symbol, category/resolution mismatch, and Schema version other than 2. Require each error to contain the offending ID and a concrete remediation to clean and rescan.

- [ ] **Step 2: Verify Doctor RED**

Run: `go test ./internal/doctor -run TestValidateSymbolProjectionRejectsInvalidReferences -count=1`

Expected: compilation fails because `ValidateSymbolProjection` is missing.

- [ ] **Step 3: Implement validation and workspace evidence loading**

Validate unique/non-empty IDs, required fields, coverage enum values, symbol references, category/resolution combinations, sorted candidates, evidence namespacing, API step ordering, `transport == "http"` for API records, and the final selected-symbol step. Require a valid provider for direct/API records and candidate-scoped ambiguous records; allow an empty provider only for unresolved records with no candidates. Load each indexed project's `evidence.json`, namespace IDs by registry project path, and reject a reference absent from the owning project.

- [ ] **Step 4: Support Doctor from workspace and project roots**

If `<root>/.goregraph-workspace` exists, validate it directly. Otherwise run existing project checks and, when `scan.WorkspaceRoot` finds a parent projection, validate that projection too. A missing symbol projection on a current 1.3.0 workspace is a failure with `goregraph workspace clean <root> --execute && goregraph workspace scan-all <root>` remediation; legacy project output without a detected workspace remains valid.

- [ ] **Step 5: Run Doctor and compatibility tests**

Run: `gofmt -w internal/doctor && go test ./internal/doctor -count=1 && go test ./internal/scan -run 'TestStableWorkspace|TestWorkspaceSymbol' -count=1`

Expected: PASS; existing project-only Doctor tests remain green.

- [ ] **Step 6: Commit integrity checks**

```bash
git add internal/doctor/symbol_projection.go internal/doctor/symbol_projection_test.go internal/doctor/doctor.go
git commit -m "Validate workspace symbol projections" -m "- Reject duplicate, dangling, and invalid API-chain references" -m "- Extend Doctor to workspace symbol outputs"
```

### Task 7: Add Query, Explain, Agent, CLI, and MCP Parity

**Files:**
- Modify: `internal/agent/service.go`
- Modify: `internal/agent/service_test.go`
- Modify: `internal/query/query.go`
- Modify: `internal/query/query_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/mcp/mcp.go`
- Modify: `internal/mcp/mcp_test.go`

**Interfaces:**
- Consumes: canonical workspace projection only; no consumer re-resolves symbols.
- Produces agent tasks `symbol-inventory`, `symbol-resolve`, `symbol-usages`, `symbol-api-consumers`, `symbol-explain`; matching Query commands; stable-ID Explain; MCP tools `symbol_inventory`, `symbol_resolve`, `symbol_usages`, `symbol_api_consumers`, `symbol_explain`.

- [ ] **Step 1: Write failing agent inventory and candidate-resolution tests**

Write workspace projection fixtures and assert `symbol-inventory --query microservices/ms-user` returns declared symbols grouped through `Item.Data["symbol"]`; `symbol-resolve --query UserService` returns every same-name candidate without choosing; and a unique qualified/export name returns one candidate. Require normal limit/continuation behavior and coverage warnings.

- [ ] **Step 2: Verify agent RED**

Run: `go test ./internal/agent -run 'TestServiceReturnsSymbolInventory|TestServiceResolvesSymbolCandidatesWithoutGuessing' -count=1`

Expected: FAIL with `unknown agent task`.

- [ ] **Step 3: Implement projection loaders and agent tasks**

Add `readWorkspaceOutput(root, name, dest)` that accepts the workspace root or detects the parent workspace from a project root. `symbol-usages` requires a stable symbol ID and returns only `direct_reference`; `symbol-api-consumers` returns only `reached_through_api`; `symbol-explain` accepts a symbol or usage ID and returns classification reason, candidates, dependency evidence, API path, and limitations. `symbol-resolve` is the only operation accepting ambiguous human input.

- [ ] **Step 4: Write failing CLI/Query/Explain tests**

Assert these exact forms:

```bash
goregraph query <workspace> symbol-inventory --query microservices/ms-user --format markdown --limit 20
goregraph query <workspace> symbol-resolve --query com.weka.UserService --format json --limit 20
goregraph query <workspace> symbol-usages --query symbol:<id> --format markdown --limit 20
goregraph query <workspace> symbol-api-consumers --query symbol:<id> --format json --limit 20
goregraph query <workspace> symbol-explain --query usage:<id> --detail full --format json --limit 20
goregraph explain <workspace> symbol:<id>
goregraph explain <workspace> usage:<id>
```

Require syntax errors to exit 2, missing projections to exit 1 with scan remediation, and legacy file/symbol Explain to remain unchanged.

- [ ] **Step 5: Implement CLI registration and Explain dispatch**

Add the five task names to `isAgentQueryTask` and help. In `query.Explain`, detect `symbol:` and `usage:` prefixes and delegate to the shared `symbol-explain` projection formatter; keep existing local file/symbol explain for other targets. Add output aliases `symbol-index` and `symbol-usages-json` for direct generated JSON inspection without colliding with the task name.

- [ ] **Step 6: Write failing MCP list/call parity tests**

List every new tool, call `symbol_usages` with a stable symbol ID, and assert the returned JSON record equals the agent record in category, resolution, evidence, candidates, and API path. Verify bounds and continuation are passed through.

- [ ] **Step 7: Implement MCP mappings through Agent Service**

Use `agentTool` and `agentTaskForTool`; do not add separate resolver logic to MCP. Descriptions must say `exact direct references` versus `HTTP reachability` and state that human text should first use `symbol_resolve`.

- [ ] **Step 8: Run all external-interface tests**

Run: `gofmt -w internal/agent internal/query internal/cli internal/mcp && go test ./internal/agent ./internal/query ./internal/cli ./internal/mcp -count=1`

Expected: PASS; task pagination and existing MCP tools remain unchanged.

- [ ] **Step 9: Commit interface parity**

```bash
git add internal/agent/service.go internal/agent/service_test.go internal/query/query.go internal/query/query_test.go internal/cli/cli.go internal/cli/cli_test.go internal/mcp/mcp.go internal/mcp/mcp_test.go
git commit -m "Expose exact symbol exploration interfaces" -m "- Add bounded inventory, resolve, usage, API, and explain tasks" -m "- Keep Query, Explain, CLI, and MCP terminology aligned"
```

### Task 8: Build the Offline Semantic Code Explorer

**Files:**
- Modify: `internal/scan/workspace_dashboard.go`
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_test.go`
- Modify: `internal/scan/workspace_reconcile.go`

**Interfaces:**
- Consumes: `WorkspaceSymbolIndexRecord` and `WorkspaceSymbolUsageIndexRecord`; does not resolve names in JavaScript.
- Produces: `RenderWorkspaceDashboardHTMLWithCodeExplorer(graph WorkspaceGraphRecord, serviceMap WorkspaceServiceMapRecord, endpointTraces WorkspaceEndpointTraceIndexRecord, symbolIndex WorkspaceSymbolIndexRecord, symbolUsages WorkspaceSymbolUsageIndexRecord) string`.

Keep `RenderWorkspaceDashboardHTMLWithModels` as a compatibility wrapper that passes empty symbol records. Reconciliation calls the new function.

- [ ] **Step 1: Write failing payload and entry-action tests**

Assert the HTML payload contains `symbol_index` and `symbol_usages`, selected-service details render `Explore classes & symbols`, and the action is absent/disabled with an explicit coverage message when the selected project has no supported symbol inventory.

- [ ] **Step 2: Verify dashboard payload RED**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboardEmbedsCanonicalSymbolProjection|TestWorkspaceDashboardOffersCodeExplorerForSelectedService' -count=1`

Expected: FAIL because the payload and action do not exist.

- [ ] **Step 3: Add payload, workbench shell, and exact state restoration**

Add a `code-explorer` view button only through the selected-service action, not as an always-empty global view. Before opening, store this exact object:

```javascript
state.architectureReturn = {
  selected: state.selected,
  domain: state.domainFocus,
  direction: state.directionFocus,
  risk: state.riskFocus,
  zoom: state.zoom,
  panX: state.panX,
  panY: state.panY
};
```

`Back to Architecture` restores every property before rendering and focuses the selected service card. Add state fields `codeProject`, `codeSymbol`, `codeTab`, `codeQuery`, `codeFilters`, and `codeUsage` without altering Architecture coordinates.

- [ ] **Step 4: Write failing inventory/search/count tests**

Assert semantic inventory markup contains grouped package/module headings, buttons/rows for selectable symbols, language/kind, qualified/export name, file:line source action, direct count, API count, confidence, coverage, and search across name/qualified/package/module/file. Include the note `3 unrelated symbols share the name UserService and were excluded.` from canonical same-name inventory, not inferred usages.

- [ ] **Step 5: Implement inventory rendering**

Build maps by stable symbol and provider usage ID once when loading the payload. Group by first non-empty module, package, workspace package, then `(root)`. Use `button` or focusable table rows with `aria-selected`; no inventory item may be SVG. Source actions use the existing safe `sourceHref`/editor template and show disabled explanatory copy when unavailable.

- [ ] **Step 6: Write failing usage tabs/filter/detail tests**

Assert tabs `Direct references`, `Reached through API`, `All`, and conditional `Ambiguous / unresolved`; filters for consumer, category, relation kind, language, confidence; and details showing canonical provider/consumer, reason, evidence, dependency/artifact evidence, ordered API steps, limitations, and source actions. Direct/API counts and lists must remain separate.

- [ ] **Step 7: Implement usage workbench behavior**

Filter only precomputed projection records. `Direct references` includes category `direct_reference`; API includes `reached_through_api`; uncertainty includes only `ambiguous` and `unresolved`. `All` combines them while retaining category badges. When no results exist, render `No verified usages in indexed coverage; this is not proof that the symbol is unused.` plus relevant coverage records.

- [ ] **Step 8: Write failing accessibility/responsive/reduced-motion tests**

Assert keyboard activation for entry/back/inventory/tabs/filters/usages, visible `:focus-visible`, `aria-selected`/`aria-pressed`, focus return, focusable tooltip semantics, reduced-motion CSS, semantic HTML overflow, and explicit layout rules for the three supported desktop viewports plus the existing narrow breakpoint.

- [ ] **Step 9: Implement accessibility and responsive CSS**

Use a two-column inventory/usage grid above 1100px and a single vertical flow below it. Keep inventory and usage lists vertically scrollable with readable minimum row height. Tooltips attach to elements reachable by Tab and use `aria-describedby`. Do not animate view transitions under `prefers-reduced-motion: reduce`.

- [ ] **Step 10: Run dashboard tests and JavaScript syntax validation**

Run:

```bash
gofmt -w internal/scan/workspace_dashboard.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go internal/scan/workspace_reconcile.go
go test ./internal/scan -run 'TestWorkspaceDashboard|TestRenderWorkspaceDashboard|TestDashboard' -count=1
sed -n '/^const workspaceDashboardScript = `/,$p' internal/scan/workspace_dashboard_script.go | sed '1s/^const workspaceDashboardScript = `//' | sed '$s/`$//' > /tmp/goregraph-code-explorer.js
node --check /tmp/goregraph-code-explorer.js
```

Expected: Go tests PASS and Node reports no syntax error. Remove `/tmp/goregraph-code-explorer.js` after validation.

- [ ] **Step 11: Commit Code Explorer UI**

```bash
git add internal/scan/workspace_dashboard.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go internal/scan/workspace_reconcile.go
git commit -m "Add the offline exact Code Explorer" -m "- Render service-scoped symbol inventory, usage tabs, filters, and evidence" -m "- Restore Architecture context and preserve accessible semantic navigation"
```

### Task 9: Document Exact Integration Depth and Unreleased 1.3.0 Scope

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/RELEASE.md`
- Modify: `docs_test.go`
- Modify: `release_files_test.go`

**Interfaces:**
- Consumes: final command names, public record/category names, output names, coverage behavior, and dashboard labels from Tasks 1–8.
- Produces: user-facing 1.3.0 documentation without release publication.

- [ ] **Step 1: Write failing documentation contract tests**

Require README and command/output/release docs to contain:

```text
Explore classes & symbols
Direct references
Reached through API
symbol-index.json
symbol-usages.json
symbol-inventory
symbol-resolve
symbol-usages
symbol-api-consumers
symbol-explain
Java / Spring
JavaScript / TypeScript / Node.js / React
Exact symbols
HTTP reachability
unreleased 1.3.0
```

Require release tests to reject language claiming a published `v1.3.0`, tag, or package-manager update.

- [ ] **Step 2: Verify documentation RED**

Run: `go test . -run 'TestDocumentation|TestReleaseFiles' -count=1`

Expected: FAIL because the Code Explorer outputs, commands, integration-depth columns, and unreleased feature scope are absent.

- [ ] **Step 3: Update README Quick Start and Code Explorer workflow**

Document clean workspace scan, dashboard entry from a selected service, exact symbol selection, separate tabs, candidate resolution before stable-ID queries, coverage warnings, and source actions. State explicitly that direct references are static source/compile relationships and API reachability is not a direct import or runtime request count.

- [ ] **Step 4: Expand the README integration-depth table**

Add columns `Exact symbols`, `Direct usages`, and `HTTP reachability`. Set Java/Spring to `Full`, `Full`, `Provider`; JavaScript/TypeScript/Node.js/React to `Full`, `Full`, `Consumer + provider`; all other languages to `—` for these three columns unless an implemented test proves otherwise. Keep existing Symbols/Imports/Calls/Routes/Tests/API clients/Persistence values unchanged. Explain that `Provider` means a Spring implementation class reached by a proven HTTP chain and `Consumer + provider` covers frontend origins plus supported Node handlers.

- [ ] **Step 5: Update command, output, and release docs**

Document exact CLI examples from Task 7, MCP equivalents, continuation/limit behavior, canonical IDs, category/resolution enums, evidence namespacing, API path steps, coverage honesty, additive rich fields, and Doctor remediation. In `docs/RELEASE.md`, add Issue #25 to current unreleased 1.3.0 scope while retaining every sentence saying tag/release/Homebrew/Scoop/Winget publication is pending.

- [ ] **Step 6: Run docs and CLI help tests**

Run: `go test . ./internal/cli -run 'TestDocumentation|TestReleaseFiles|TestHelp|TestQuery' -count=1 && git diff --check`

Expected: PASS; docs use the exact production operation/category names.

- [ ] **Step 7: Commit documentation**

```bash
git add README.md COMMANDS.md docs/OUTPUTS.md docs/RELEASE.md docs_test.go release_files_test.go
git commit -m "Document exact Code Explorer integration" -m "- Add symbol workflows, commands, outputs, and coverage semantics" -m "- Expand integration depth for Java and JavaScript/TypeScript" -m "- Keep 1.3.0 explicitly unreleased"
```

### Task 10: Full Verification, Local Installation, Clean Weka Rescan, and Issue Closure

**Files:**
- No production edits are expected.
- Generated acceptance outputs: `~/projects/weka/**/goregraph-out/` and `~/projects/weka/.goregraph-workspace/` only.

**Interfaces:**
- Consumes: completed Issue #25 implementation and the separately completed Issue #23 architecture-map implementation on `main`.
- Produces: verified local `goregraph 1.3.0`, clean current Weka outputs, pushed `main`, and closed GitHub Issue #25. Issue #23 is closed by its owning implementation only after the same combined acceptance succeeds.

- [ ] **Step 1: Run fresh source verification**

```bash
test "$(git branch --show-current)" = "main"
test -z "$(gofmt -l .)"
go vet ./...
go test ./... -count=1
git diff --check
go build -o /tmp/goregraph-1.3.0 ./cmd/goregraph
/tmp/goregraph-1.3.0 version
```

Expected: every command exits 0; version output starts `goregraph 1.3.0` and reports schema 2.

- [ ] **Step 2: Validate embedded dashboard JavaScript again**

Extract the embedded script as in Task 8 and run `node --check /tmp/goregraph-code-explorer.js`.

Expected: exit 0 with no syntax error; remove the temporary script afterward.

- [ ] **Step 3: Obtain an independent full-diff review**

Review from the commit before Issue #23/#25 work through `HEAD`, with special attention to false Exact matches, scan safety, Schema 2 compatibility, ID determinism, dangling evidence, API/direct separation, XSS escaping, keyboard behavior, and Weka-scale performance. Resolve every Critical or Important finding using a fresh failing regression test and rerun the focused and full suites. Do not proceed with an unresolved Critical or Important finding.

- [ ] **Step 4: Install the current source locally**

```bash
go install ./cmd/goregraph
command -v goregraph
goregraph version
goregraph query --help
goregraph workspace dashboard --help
```

Expected: command resolves to the active Go bin path (normally `/Users/gorecode/go/bin/goregraph`); version is 1.3.0/schema 2; help lists the symbol operations.

- [ ] **Step 5: Review and execute the generated-output clean plan for Weka**

```bash
cd ~/projects/weka
goregraph workspace clean .
goregraph workspace clean . --execute
```

Expected: the first command lists only generated `goregraph-out` directories and `.goregraph-workspace`; inspect it before execute. The second removes only those generated outputs and does not modify source, Git state, dependencies, or application files.

- [ ] **Step 6: Rescan every discovered Weka project**

```bash
cd ~/projects/weka
goregraph workspace scan-all .
goregraph workspace status .
goregraph doctor .
goregraph workspace dashboard .
```

Expected: `scan-all` exits 0; every discovered project is indexed or has a structured explicit failure; workspace Doctor exits 0; dashboard command prints an existing `.goregraph-workspace/workspace-map.html` path.

- [ ] **Step 7: Verify fresh Weka symbol projections**

```bash
test -s ~/projects/weka/.goregraph-workspace/symbol-index.json
test -s ~/projects/weka/.goregraph-workspace/symbol-usages.json
grep -q '"language": "java"' ~/projects/weka/.goregraph-workspace/symbol-index.json
grep -Eq '"language": "(javascript|typescript)"' ~/projects/weka/.goregraph-workspace/symbol-index.json
grep -q '"category": "direct_reference"' ~/projects/weka/.goregraph-workspace/symbol-usages.json
grep -q '"category": "reached_through_api"' ~/projects/weka/.goregraph-workspace/symbol-usages.json
goregraph query ~/projects/weka symbol-inventory --query microservices --format markdown --limit 5
```

Expected: both non-empty projections contain Java and JS/TS symbols, direct usages, HTTP reachability, and a bounded readable inventory. If real Weka legitimately has no record for one category, inspect coverage and fixture tests instead of inserting synthetic Weka data or weakening acceptance semantics.

- [ ] **Step 8: Verify browser interaction, accessibility, and visuals**

Open the fresh Weka `workspace-map.html` in the local browser. At 1280×720, 1440×900, and 1920×1080 verify: select a service; open `Explore classes & symbols`; search/select Java and JS/TS symbols; switch Direct/API/All/uncertainty tabs; filter and inspect evidence/API steps; activate source actions; Tab through every interactive control; confirm visible focus/tooltips; enable reduced motion; return to Architecture and confirm exact service/domain/direction/risk/zoom/pan restoration. Capture acceptance screenshots outside the repository or under `/tmp`; do not commit generated Weka data or screenshots.

- [ ] **Step 9: Confirm workspace and repository cleanliness**

```bash
cd /Users/gorecode/projects/gorecode/goregraph
git status --short
git log -1 --oneline
```

Expected: no uncommitted GoreGraph source changes. Weka repositories may contain pre-existing user changes, but GoreGraph must not create new tracked source changes; generated outputs remain ignored.

- [ ] **Step 10: Push `main` and verify the remote commit**

```bash
git push origin main
git fetch origin main
test "$(git rev-parse main)" = "$(git rev-parse origin/main)"
```

Expected: push and fetch exit 0 and local/remote commit IDs match. Do not push a tag and do not run a release workflow.

- [ ] **Step 11: Close Issue #25 only after remote and Weka verification**

```bash
gh issue close 25 --repo gorecodecom/goregraph --reason completed --comment "Implemented in unreleased 1.3.0 on main: exact canonical Java and JavaScript/TypeScript symbol inventory, dependency-aware direct usages, separate HTTP reachability, Query/Explain/MCP/Doctor parity, and the offline Code Explorer. Full tests and a clean local Weka workspace rescan passed. No release or package publication was created."
gh issue view 25 --repo gorecodecom/goregraph --json number,state,url
```

Expected: returned state is `CLOSED`. If the push, remote equality, Weka scan, Doctor, or browser acceptance failed, leave the issue open and report the exact blocker.

---

## Final Acceptance Checklist

- [ ] Java class/interface/enum/record, nested declaration, import, generic/array, field/constructor/parameter/return, annotation, inheritance, instantiation, static import, and resolved-owner tests pass.
- [ ] Duplicate Java simple names and duplicate FQNs are excluded, artifact-resolved, or explicitly ambiguous as evidence requires.
- [ ] JavaScript/TypeScript class/interface/type/enum/function/component, import/re-export, workspace package, path alias, JSX, type, and resolved-call tests pass.
- [ ] Same-name Java and JS/TS declarations without module/artifact evidence never appear as Exact.
- [ ] Direct references and HTTP reachability remain separate records, counts, tabs, task results, and MCP results.
- [ ] The complete JS/TS caller → API helper → HTTP contract → workspace contract → Spring route → handler → implementation → Java symbol path is retained.
- [ ] Repeated builds and reversed project order produce identical sorted projection content except generated timestamp.
- [ ] Query, Explain, Agent, MCP, Doctor, generated JSON, and dashboard share category/resolution names and evidence.
- [ ] Doctor rejects duplicate/dangling/evidence/API-chain failures with offending IDs and clean-rescan remediation.
- [ ] The README feature/integration-depth table accurately shows Java and JavaScript/TypeScript exact-symbol and HTTP depth.
- [ ] Full formatting, vet, tests, JS syntax, dashboard interaction, accessibility, and all three visual viewport checks pass.
- [ ] Current source is installed locally as `goregraph 1.3.0`, Weka generated outputs were reviewed/cleaned/rescanned, and fresh projections pass Doctor.
- [ ] `main` equals `origin/main`; Issue #25 is closed; Issue #23 closure remains gated by its own implementation review plus the same combined acceptance.
- [ ] No tag, GitHub Release, Homebrew, Scoop, Winget, or other package publication exists for 1.3.0.
