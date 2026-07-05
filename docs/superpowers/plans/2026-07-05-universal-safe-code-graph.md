# Universal Safe Code Graph Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Build a Graphify-like local code intelligence layer for any codebase, but with an explicit safety model: no hidden network access, no project code execution, no automatic assistant installation, no telemetry, and fully inspectable output.

**Architecture:** Keep GoreGraph a local, deterministic Go CLI that never executes scanned project code. Build a language-neutral graph core first, then plug in language/domain analyzers such as Java, Spring, Go, Python, PHP, Shell, JS/TS, Maven, and frontend workspaces; write additive JSON/Markdown outputs without breaking existing `files.json`, `symbols.json`, `relations.json`, and `graph.json` consumers.

**Tech Stack:** Go stdlib, `go test`, `go vet`, `gofmt`, deterministic JSON/Markdown outputs, language-neutral graph records, adapter-based static source scanning, Maven `pom.xml` metadata scanning, JS/TS package and route/API scanning.

---

## Current Language Baseline

GoreGraph already has shallow scanners for these languages and file types:

- Go
- JavaScript
- TypeScript
- Python
- PHP
- Shell
- Java
- Markdown
- `package.json`
- `composer.json`

This plan must not regress broad language support while adding deeper Java/Spring intelligence. The first implementation pass must normalize all existing language outputs into the same richer core model. Java/Spring depth comes after that shared foundation is in place.

Minimum broad-language behavior for the first safe graph version:

- Every supported source file gets a file node.
- Every detected symbol gets a symbol node.
- Every import/include/source relation gets a rich edge.
- Existing test relations remain present.
- Existing entrypoint detection remains present.
- The new `graph-full.json` includes all supported languages, not only Java.
- Language-specific deeper features are optional per adapter, but the adapter output shape is shared.

## Observed WEKA Baseline

These findings came from local structural inspection of `/Users/gorecode/projects/weka` on 2026-07-05:

- Microservices: 39 `pom.xml` files directly below `/Users/gorecode/projects/weka/microservices`.
- Spring Boot applications: 37 `@SpringBootApplication` hits.
- Main Spring annotations across microservices:
  - `@Autowired`: 1115
  - `@RequiredArgsConstructor`: 623
  - `@Repository`: 314
  - `@Component`: 311
  - `@Entity`: 308
  - `@Table`: 307
  - `@Bean`: 237
  - `@Configuration`: 216
  - `@Service`: 200
  - `@GetMapping`: 146
  - `@RestController`: 82
  - `@RequestMapping`: 77
  - `@PostMapping`: 65
  - `@PutMapping`: 53
  - `@SpringBootTest`: 48
  - `@DeleteMapping`: 20
- Frontend:
  - Yarn/Lerna monorepo under `frontend/frontend-monorepo`.
  - Yarn workspace/Vite/React app under `frontend/frontends/apps/rdbv`.
  - Next app under `frontend/shop-frontend-2024`.
  - 56 frontend test files with `.spec.*` or `.test.*` suffixes.

The real `ms-cadaster` scan showed these GoreGraph gaps:

- Java false positives, including symbols named `for`.
- `entrypoints.md` was empty even though the service has a Spring Boot application and REST controllers.
- `test-map.md` was empty even though Java tests exist.
- No Spring endpoints with HTTP method, path, controller method, request type, and response type.
- No Controller -> Service -> Repository -> Entity chain.
- No constructor/Lombok dependency model.
- No internal/external import distinction.
- Manifest lacks Git revision and dirty state.

## Graphify Reference Findings

Graphify was reviewed as a reference implementation, not as code to copy. The useful ideas for GoreGraph are:

- Graphify has a staged pipeline: detect files, extract structure, build graph, analyze, report, export.
- Code extraction is deterministic and local; AI is reserved for semantic/document extraction.
- The core interchange shape is plain nodes and edges with stable IDs, `source_file`, `source_location`, `relation`, `confidence`, and optional `confidence_score`.
- Relationships distinguish `EXTRACTED`, `INFERRED`, and `AMBIGUOUS`; extracted code facts use confidence `EXTRACTED`.
- The graph report highlights “god nodes”, communities, surprising connections, and suggested questions.
- Incremental runs use content hashes and a cache under the output directory.
- Affected-node queries walk incoming dependency edges from a seed node to answer “what might break if this changes?”.
- SCIP ingestion exists as an optional path for richer symbol graphs, but it is not wired as the normal flow.

Decisions for GoreGraph:

- Keep GoreGraph as a Go CLI with no Python, JDK, Node, Maven, or project runtime requirement.
- Add Graphify-inspired metadata to GoreGraph edges and rich graph output, but keep the existing simple files for compatibility.
- Make the core model language-neutral. Spring facts are domain records produced by the Java/Spring adapter, not the shape of the whole tool.
- Use Java/Spring as the first deep adapter because WEKA gives us real-world pressure, but implement the shared graph core for all currently supported languages first.
- Treat SCIP/tree-sitter-style parsing as a later optional enhancement after the deterministic Go scanner proves useful on real projects.

## Safety Model

GoreGraph should offer the useful part of Graphify without the broad background surface area.

Default behavior:

- No network requests.
- No telemetry.
- No AI calls.
- No background daemon.
- No file watcher unless the user explicitly runs a watch command in a later milestone.
- No MCP server unless the user explicitly runs `goregraph mcp`.
- No assistant/IDE/global installation from `scan`.
- No shelling out to Maven, Gradle, npm, yarn, pnpm, pip, Composer, Java, Node, Docker, GitHub CLI, or cloud CLIs during scan.
- No writes outside `<project>/goregraph-out` and project `.gitignore` updates that the user explicitly requested.
- No reading files ignored by configured excludes and `.gitignore`, except the `.gitignore` files needed to build the exclusion rules.

Allowed by default:

- Read project files under the scan root.
- Read `.git` metadata only to record commit, branch, and dirty status.
- Write deterministic output under `goregraph-out`.
- Update the project-local `.gitignore` with `goregraph-out/` when missing.

Commands that later add larger surface area must be opt-in and visibly named:

- `goregraph mcp`: starts a local MCP server for an existing `goregraph-out`.
- `goregraph watch`: watches local files and reruns scans.
- `goregraph ai`: future optional AI enrichment. It must require explicit config and must never be part of `scan`.
- `goregraph global`: future optional cross-repo index under a GoreGraph-owned user directory.

Every scan should write an audit file:

- `audit.json`: scanner version, command, scan root basename, output directory, started/finished timestamps, files read count, files skipped count, generated files, whether network was used (`false` for normal scan), whether external commands were executed (`false` for normal scan), and optional warnings.

This is the trust contract: a user can inspect `audit.json` and see what GoreGraph did.

## Universal Architecture Rules

Every language/domain analyzer must follow the same contract:

- It receives project-root-relative file paths and file contents.
- It returns normalized symbols, relations, rich graph nodes, rich graph edges, optional domain records, and optional Markdown sections.
- It never shells out to project tooling.
- It never writes files directly; the scan coordinator writes all outputs.
- It is deterministic for the same input files.
- It can be tested with synthetic fixtures that do not include private customer code.

The scanner layers are:

1. **Core inventory:** files, hashes, languages, kind, size, ignored/skipped status.
2. **Language structure:** packages/modules/namespaces, classes/types/functions/methods/imports.
3. **Relation resolution:** internal vs external imports, references, calls, tests, ownership, generated graph edges.
4. **Domain adapters:** Spring, Maven, frontend routes/API usage, future Django/Laravel/Rails/NestJS/etc.
5. **Reports and queries:** human Markdown, structured JSON, MCP resources, `query`, and `affected`.

Initial adapter priorities:

- Existing shallow adapters are upgraded into the shared graph model first: Go, JS/TS, Python, PHP, Shell, Markdown, package/composer metadata.
- Java becomes a deep language adapter.
- Spring becomes a Java domain adapter.
- Maven and Node workspaces become project metadata adapters.
- Future adapters can add depth without changing the core output contract.

## Output Contract

Existing files stay present and backward compatible:

- `manifest.json`
- `files.json`
- `symbols.json`
- `relations.json`
- `graph.json`
- `report.md`
- `modules.md`
- `entrypoints.md`
- `test-map.md`

Add these files:

- `spring.json`: structured Spring components, endpoints, bean dependencies, repositories, entities, and table names.
- `endpoints.md`: human-readable HTTP endpoint inventory.
- `dependencies.md`: human-readable component and bean dependency inventory.
- `workspace.md`: human-readable Maven/package workspace summary.
- `graph-full.json`: additive rich node/edge graph with stable IDs, source locations, confidence metadata, and semantic edge types.
- `symbols-full.json`: additive normalized symbol records for all supported languages with stable IDs, language, kind, source location, and container/owner.
- `relations-full.json`: additive normalized relation records for all supported languages with stable IDs, relation type, confidence, source location, and internal/external resolution where available.
- `affected.md`: human-readable “likely affected areas” report derived from incoming graph edges.
- `audit.json`: machine-readable scan audit showing the operation stayed local and deterministic.

Schema handling:

- Keep `manifest.schema` at `1` until an existing output file changes incompatibly.
- New files are additive and listed in `manifest.generated`.
- Add `manifest.git` and `manifest.generated_at` as additive fields.

## File Structure

Create focused files:

- `internal/scan/java_types.go`: Java and Spring-specific data types.
- `internal/scan/adapter.go`: language-neutral analyzer interface and normalized analyzer result.
- `internal/scan/rich_types.go`: normalized symbols, relations, graph nodes, graph edges, confidence, source locations, and IDs.
- `internal/scan/rich_build.go`: conversion from existing scanner output into rich records for every supported language.
- `internal/scan/extract_java.go`: Java source cleanup, package/import/type/method/annotation extraction.
- `internal/scan/java_resolve.go`: internal Java type/import resolution.
- `internal/scan/spring_extract.go`: Spring component, endpoint, repository, entity, and bean extraction.
- `internal/scan/spring_report.go`: `spring.json`, `endpoints.md`, and `dependencies.md` rendering.
- `internal/scan/workspace_extract.go`: Maven and package workspace discovery.
- `internal/scan/workspace_report.go`: `workspace.md` rendering.
- `internal/scan/rich_graph.go`: Graphify-inspired rich graph model with stable node IDs, source locations, confidence, and edge metadata.
- `internal/scan/affected.go`: affected-node analysis over the rich graph.
- `internal/scan/audit.go`: safe-scan audit records and writer.
- `internal/scan/git_metadata.go`: Git metadata for `manifest.json`.
- `internal/scan/testdata/java_spring_microservice/...`: WEKA-like fixture used by tests.

Modify existing files:

- `internal/scan/types.go`: add metadata fields and top-level result aggregation types.
- `internal/scan/extract.go`: delegate Java logic to `extract_java.go`.
- `internal/scan/scan.go`: assemble all language adapters, Java/Spring/workspace outputs, rich outputs, and reports.
- `internal/scan/graph.go`: add semantic nodes and edges while preserving existing file/dependency/symbol nodes.
- `internal/scan/report.go`: include high-level endpoint/component counts.
- `internal/scan/scan_test.go`: keep existing behavior tests green.
- `internal/mcp/mcp.go`: expose new output files as resources without requiring an LLM.
- `README.md`: describe the new outputs.
- `COMMANDS.md`: document what `scan`, `report`, `query`, and MCP can do with new files.

## Task 1: Create Universal Rich Graph Core For All Current Languages

**Files:**
- Create: `internal/scan/rich_types.go`
- Create: `internal/scan/rich_build.go`
- Create: `internal/scan/rich_graph_test.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/types.go`

- [x] **Step 1: Add broad-language rich graph acceptance test**

Create `internal/scan/rich_graph_test.go`:

```go
package scan

import (
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestRunWritesRichGraphForAllCurrentLanguages(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/demo\n")
	writeFile(t, root, "cmd/api/main.go", "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"ok\") }\n")
	writeFile(t, root, "web/app.ts", "import { api } from './api';\nexport class App {}\nexport function start() { api(); }\n")
	writeFile(t, root, "web/api.js", "export function api() { return fetch('/api'); }\n")
	writeFile(t, root, "python/app.py", "import os\nclass Service:\n    def run(self):\n        return os.getcwd()\n")
	writeFile(t, root, "php/index.php", "<?php\nrequire_once __DIR__ . '/Service.php';\nfunction boot() {}\n")
	writeFile(t, root, "php/Service.php", "<?php\nclass Service {}\n")
	writeFile(t, root, "scripts/deploy.sh", "#!/usr/bin/env bash\nsource ./lib.sh\ndeploy() { echo deploy; }\n")
	writeFile(t, root, "scripts/lib.sh", "helper() { echo helper; }\n")
	writeFile(t, root, "README.md", "# Demo\n")
	writeFile(t, root, "package.json", `{"scripts":{"dev":"vite","test":"vitest"}}`)
	writeFile(t, root, "composer.json", `{"autoload":{"psr-4":{"App\\":"src/"}}}`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := filepath.Join(root, "goregraph-out")
	var rich RichGraph
	readJSON(t, filepath.Join(out, "graph-full.json"), &rich)
	assertRichNode(t, rich.Nodes, "file", "cmd/api/main.go")
	assertRichNode(t, rich.Nodes, "file", "web/app.ts")
	assertRichNode(t, rich.Nodes, "file", "python/app.py")
	assertRichNode(t, rich.Nodes, "file", "php/index.php")
	assertRichNode(t, rich.Nodes, "file", "scripts/deploy.sh")
	assertRichNode(t, rich.Nodes, "symbol", "main")
	assertRichNode(t, rich.Nodes, "symbol", "App")
	assertRichNode(t, rich.Nodes, "symbol", "Service")
	assertRichEdge(t, rich.Edges, "imports", "EXTRACTED")
	assertRichEdge(t, rich.Edges, "sources", "EXTRACTED")

	var symbols []RichSymbolRecord
	readJSON(t, filepath.Join(out, "symbols-full.json"), &symbols)
	assertRichSymbol(t, symbols, "go", "main", "function")
	assertRichSymbol(t, symbols, "typescript", "App", "class")
	assertRichSymbol(t, symbols, "python", "Service", "class")
	assertRichSymbol(t, symbols, "php", "Service", "class")
	assertRichSymbol(t, symbols, "shell", "deploy", "function")

	var relations []RichRelationRecord
	readJSON(t, filepath.Join(out, "relations-full.json"), &relations)
	assertRichRelation(t, relations, "web/app.ts", "./api", "imports")
	assertRichRelation(t, relations, "scripts/deploy.sh", "scripts/lib.sh", "sources")
}
```

- [x] **Step 2: Add rich assertion helpers**

Append to `internal/scan/rich_graph_test.go`:

```go
func assertRichNode(t *testing.T, nodes []RichGraphNode, kind, label string) {
	t.Helper()
	for _, node := range nodes {
		if node.Kind == kind && node.Label == label {
			return
		}
	}
	t.Fatalf("missing rich node kind=%q label=%q in %#v", kind, label, nodes)
}

func assertRichEdge(t *testing.T, edges []RichGraphEdge, relation, confidence string) {
	t.Helper()
	for _, edge := range edges {
		if edge.Relation == relation && edge.Confidence == confidence {
			return
		}
	}
	t.Fatalf("missing rich edge relation=%q confidence=%q in %#v", relation, confidence, edges)
}

func assertRichSymbol(t *testing.T, symbols []RichSymbolRecord, language, name, kind string) {
	t.Helper()
	for _, symbol := range symbols {
		if symbol.Language == language && symbol.Name == name && symbol.Kind == kind {
			return
		}
	}
	t.Fatalf("missing rich symbol language=%q name=%q kind=%q in %#v", language, name, kind, symbols)
}

func assertRichRelation(t *testing.T, relations []RichRelationRecord, from, to, relationType string) {
	t.Helper()
	for _, relation := range relations {
		if relation.From == from && relation.To == to && relation.Type == relationType {
			return
		}
	}
	t.Fatalf("missing rich relation from=%q to=%q type=%q in %#v", from, to, relationType, relations)
}
```

- [x] **Step 3: Run test and confirm it fails**

Run:

```bash
go test -count=1 ./internal/scan -run TestRunWritesRichGraphForAllCurrentLanguages
```

Expected: compile failure because `RichGraph`, `RichGraphNode`, `RichGraphEdge`, `RichSymbolRecord`, and `RichRelationRecord` do not exist yet.

- [x] **Step 4: Define language-neutral rich types**

Create `internal/scan/rich_types.go`:

```go
package scan

type RichSymbolRecord struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Language       string `json:"language"`
	File           string `json:"file"`
	Line           int    `json:"line,omitempty"`
	Owner          string `json:"owner,omitempty"`
	SourceLocation string `json:"source_location,omitempty"`
}

type RichRelationRecord struct {
	ID              string `json:"id"`
	From            string `json:"from"`
	To              string `json:"to"`
	Type            string `json:"type"`
	Language        string `json:"language,omitempty"`
	Line            int    `json:"line,omitempty"`
	SourceLocation  string `json:"source_location,omitempty"`
	Confidence      string `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	Internal        bool   `json:"internal,omitempty"`
}

type RichGraph struct {
	Directed bool            `json:"directed"`
	Nodes    []RichGraphNode `json:"nodes"`
	Edges    []RichGraphEdge `json:"edges"`
}

type RichGraphNode struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Kind           string `json:"kind"`
	Language       string `json:"language,omitempty"`
	SourceFile     string `json:"source_file,omitempty"`
	SourceLocation string `json:"source_location,omitempty"`
}

type RichGraphEdge struct {
	ID              string  `json:"id"`
	Source          string  `json:"source"`
	Target          string  `json:"target"`
	Relation        string  `json:"relation"`
	Confidence      string  `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	SourceFile      string  `json:"source_file,omitempty"`
	SourceLocation  string  `json:"source_location,omitempty"`
}
```

- [x] **Step 5: Convert existing scanner output into rich records**

Create `internal/scan/rich_build.go`:

```go
package scan

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
)

func buildRichSymbols(files []FileRecord, symbols []SymbolRecord) []RichSymbolRecord {
	languageByFile := map[string]string{}
	for _, file := range files {
		languageByFile[file.Path] = file.Language
	}
	var rich []RichSymbolRecord
	for _, symbol := range symbols {
		location := sourceLocation(symbol.Line)
		record := RichSymbolRecord{
			ID:             stableID("symbol", symbol.File, symbol.Kind, symbol.Name, fmt.Sprint(symbol.Line)),
			Name:           symbol.Name,
			Kind:           symbol.Kind,
			Language:       languageByFile[symbol.File],
			File:           symbol.File,
			Line:           symbol.Line,
			SourceLocation: location,
		}
		rich = append(rich, record)
	}
	sort.Slice(rich, func(i, j int) bool { return rich[i].ID < rich[j].ID })
	return rich
}

func buildRichRelations(files []FileRecord, relations []RelationRecord) []RichRelationRecord {
	languageByFile := map[string]string{}
	for _, file := range files {
		languageByFile[file.Path] = file.Language
	}
	var rich []RichRelationRecord
	for _, relation := range relations {
		record := RichRelationRecord{
			ID:              stableID("relation", relation.From, relation.Type, relation.To, fmt.Sprint(relation.Line)),
			From:            relation.From,
			To:              relation.To,
			Type:            relation.Type,
			Language:        languageByFile[relation.From],
			Line:            relation.Line,
			SourceLocation:  sourceLocation(relation.Line),
			Confidence:      "EXTRACTED",
			ConfidenceScore: 1.0,
			Internal:        isInternalRelation(relation),
		}
		rich = append(rich, record)
	}
	sort.Slice(rich, func(i, j int) bool { return rich[i].ID < rich[j].ID })
	return rich
}

func buildRichGraph(files []FileRecord, symbols []RichSymbolRecord, relations []RichRelationRecord) RichGraph {
	nodesByID := map[string]RichGraphNode{}
	for _, file := range files {
		id := stableID("file", file.Path)
		nodesByID[id] = RichGraphNode{ID: id, Label: file.Path, Kind: "file", Language: file.Language, SourceFile: file.Path, SourceLocation: "L1"}
	}
	for _, symbol := range symbols {
		nodesByID[symbol.ID] = RichGraphNode{ID: symbol.ID, Label: symbol.Name, Kind: "symbol", Language: symbol.Language, SourceFile: symbol.File, SourceLocation: symbol.SourceLocation}
	}

	var edges []RichGraphEdge
	for _, symbol := range symbols {
		edges = append(edges, RichGraphEdge{
			ID:              stableID("edge", "contains", symbol.File, symbol.ID),
			Source:          stableID("file", symbol.File),
			Target:          symbol.ID,
			Relation:        "contains",
			Confidence:      "EXTRACTED",
			ConfidenceScore: 1.0,
			SourceFile:      symbol.File,
			SourceLocation:  symbol.SourceLocation,
		})
	}
	for _, relation := range relations {
		edges = append(edges, RichGraphEdge{
			ID:              relation.ID,
			Source:          stableID("file", relation.From),
			Target:          stableRelationTargetID(relation),
			Relation:        relation.Type,
			Confidence:      relation.Confidence,
			ConfidenceScore: relation.ConfidenceScore,
			SourceFile:      relation.From,
			SourceLocation:  relation.SourceLocation,
		})
		if _, ok := nodesByID[stableRelationTargetID(relation)]; !ok {
			nodesByID[stableRelationTargetID(relation)] = RichGraphNode{ID: stableRelationTargetID(relation), Label: relation.To, Kind: "external"}
		}
	}

	var nodes []RichGraphNode
	for _, node := range nodesByID {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	return RichGraph{Directed: true, Nodes: nodes, Edges: edges}
}

func stableID(parts ...string) string {
	hash := sha1.New()
	for _, part := range parts {
		hash.Write([]byte(part))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))[:24]
}

func sourceLocation(line int) string {
	if line <= 0 {
		return ""
	}
	return fmt.Sprintf("L%d", line)
}

func isInternalRelation(relation RelationRecord) bool {
	return relation.Type == "imports_internal" || relation.Type == "tests" || relation.Type == "includes" || relation.Type == "sources"
}

func stableRelationTargetID(relation RichRelationRecord) string {
	if relation.Internal {
		return stableID("file", relation.To)
	}
	return stableID("external", relation.To)
}
```

- [x] **Step 6: Write additive rich outputs**

In `scan.go`, after existing symbols and relations are final:

```go
richSymbols := buildRichSymbols(files, symbols)
richRelations := buildRichRelations(files, relations)
richGraph := buildRichGraph(files, richSymbols, richRelations)
```

Write:

```go
if err := writeJSON(filepath.Join(outDir, "symbols-full.json"), richSymbols); err != nil {
	return Result{}, err
}
if err := writeJSON(filepath.Join(outDir, "relations-full.json"), richRelations); err != nil {
	return Result{}, err
}
if err := writeJSON(filepath.Join(outDir, "graph-full.json"), richGraph); err != nil {
	return Result{}, err
}
```

Add the three filenames to `manifest.generated`.

- [x] **Step 7: Run tests**

Run:

```bash
gofmt -w internal/scan/rich_types.go internal/scan/rich_build.go internal/scan/rich_graph_test.go internal/scan/scan.go
go test -count=1 ./internal/scan
```

Expected: existing scanner tests and broad rich graph test pass.

- [x] **Step 8: Commit universal rich graph core**

```bash
git add internal/scan/rich_types.go internal/scan/rich_build.go internal/scan/rich_graph_test.go internal/scan/scan.go
git commit -m "feat: add universal rich graph outputs"
```

## Task 2: Create WEKA-Like Fixture And Failing Acceptance Test

**Files:**
- Create: `internal/scan/java_spring_test.go`
- Create: `internal/scan/testdata/java_spring_microservice/pom.xml`
- Create: `internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/DemoApplication.java`
- Create: `internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/config/ApplicationConfig.java`
- Create: `internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/controller/CadasterController.java`
- Create: `internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/service/CadasterService.java`
- Create: `internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/repository/CadasterRepository.java`
- Create: `internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/entity/CadasterEntity.java`
- Create: `internal/scan/testdata/java_spring_microservice/src/test/java/com/weka/demo/controller/CadasterControllerTest.java`
- Create: `internal/scan/testdata/java_spring_microservice/src/test/java/com/weka/demo/service/CadasterServiceTest.java`

- [x] **Step 1: Add the failing acceptance test**

Add this test file:

```go
package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestRunExtractsWekaStyleSpringIntelligence(t *testing.T) {
	root := copyScanFixture(t, "java_spring_microservice")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := filepath.Join(root, "goregraph-out")
	for _, name := range []string{"spring.json", "endpoints.md", "dependencies.md", "workspace.md"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(out, "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "CadasterController", "class", "src/main/java/com/weka/demo/controller/CadasterController.java")
	assertHasSymbol(t, symbols, "gets", "method", "src/main/java/com/weka/demo/controller/CadasterController.java")
	assertNoSymbol(t, symbols, "for", "class")

	var relations []RelationRecord
	readJSON(t, filepath.Join(out, "relations.json"), &relations)
	assertHasRelation(t, relations, "src/main/java/com/weka/demo/controller/CadasterController.java", "src/main/java/com/weka/demo/service/CadasterService.java", "imports_internal")
	assertHasRelation(t, relations, "src/main/java/com/weka/demo/controller/CadasterController.java", "org.springframework.web.bind.annotation.GetMapping", "imports_external")
	assertHasRelation(t, relations, "src/test/java/com/weka/demo/controller/CadasterControllerTest.java", "src/main/java/com/weka/demo/controller/CadasterController.java", "tests")

	var spring SpringIndex
	readJSON(t, filepath.Join(out, "spring.json"), &spring)
	assertHasSpringComponent(t, spring.Components, "CadasterController", "rest_controller")
	assertHasSpringComponent(t, spring.Components, "CadasterService", "service")
	assertHasSpringComponent(t, spring.Components, "CadasterRepository", "repository")
	assertHasSpringEntity(t, spring.Entities, "CadasterEntity", "VD_CADASTER")
	assertHasSpringDependency(t, spring.Dependencies, "CadasterController", "CadasterService", "constructor")
	assertHasSpringRepositoryEntity(t, spring.Repositories, "CadasterRepository", "CadasterEntity")
	assertHasSpringEndpoint(t, spring.Endpoints, "GET", "/cadasters", "CadasterController", "gets")
	assertHasSpringEndpoint(t, spring.Endpoints, "POST", "/cadasters/{cadasterId}/copy", "CadasterController", "copy")

	entrypoints := readText(t, filepath.Join(out, "entrypoints.md"))
	if !strings.Contains(entrypoints, "DemoApplication") || !strings.Contains(entrypoints, "Spring Boot application") {
		t.Fatalf("entrypoints report missing Spring Boot application:\n%s", entrypoints)
	}

	testMap := readText(t, filepath.Join(out, "test-map.md"))
	if !strings.Contains(testMap, "CadasterControllerTest") || !strings.Contains(testMap, "CadasterController") {
		t.Fatalf("test-map report missing Java test mapping:\n%s", testMap)
	}

	endpoints := readText(t, filepath.Join(out, "endpoints.md"))
	if !strings.Contains(endpoints, "GET `/cadasters`") || !strings.Contains(endpoints, "POST `/cadasters/{cadasterId}/copy`") {
		t.Fatalf("endpoints report missing expected routes:\n%s", endpoints)
	}
}

func copyScanFixture(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join("testdata", name)
	target := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, body, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func assertNoSymbol(t *testing.T, symbols []SymbolRecord, name, kind string) {
	t.Helper()
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			t.Fatalf("unexpected symbol name=%q kind=%q in %#v", name, kind, symbols)
		}
	}
}
```

- [x] **Step 2: Add assertion helpers**

Append these helpers to `internal/scan/java_spring_test.go`:

```go
func assertHasSpringComponent(t *testing.T, components []SpringComponentRecord, name, kind string) {
	t.Helper()
	for _, component := range components {
		if component.Name == name && component.Kind == kind {
			return
		}
	}
	t.Fatalf("missing Spring component name=%q kind=%q in %#v", name, kind, components)
}

func assertHasSpringEntity(t *testing.T, entities []SpringEntityRecord, name, table string) {
	t.Helper()
	for _, entity := range entities {
		if entity.Name == name && entity.Table == table {
			return
		}
	}
	t.Fatalf("missing Spring entity name=%q table=%q in %#v", name, table, entities)
}

func assertHasSpringDependency(t *testing.T, dependencies []SpringDependencyRecord, from, to, injection string) {
	t.Helper()
	for _, dependency := range dependencies {
		if dependency.From == from && dependency.To == to && dependency.Injection == injection {
			return
		}
	}
	t.Fatalf("missing Spring dependency from=%q to=%q injection=%q in %#v", from, to, injection, dependencies)
}

func assertHasSpringRepositoryEntity(t *testing.T, repositories []SpringRepositoryRecord, repository, entity string) {
	t.Helper()
	for _, record := range repositories {
		if record.Name == repository && record.Entity == entity {
			return
		}
	}
	t.Fatalf("missing Spring repository name=%q entity=%q in %#v", repository, entity, repositories)
}

func assertHasSpringEndpoint(t *testing.T, endpoints []SpringEndpointRecord, method, path, controller, handler string) {
	t.Helper()
	for _, endpoint := range endpoints {
		if endpoint.HTTPMethod == method && endpoint.Path == path && endpoint.Controller == controller && endpoint.Method == handler {
			return
		}
	}
	t.Fatalf("missing Spring endpoint method=%q path=%q controller=%q handler=%q in %#v", method, path, controller, handler, endpoints)
}
```

- [x] **Step 3: Add the fixture source**

Create fixture files with these patterns:

```java
// internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/controller/CadasterController.java
package com.weka.demo.controller;

import com.weka.demo.model.CadasterCopyRequest;
import com.weka.demo.service.CadasterService;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import static com.weka.demo.config.ApplicationConfig.BASE_PATH;

@RestController
@RequestMapping(BASE_PATH)
@RequiredArgsConstructor(onConstructor_ = {@Autowired})
public class CadasterController {
  private final CadasterService cadasterService;

  @GetMapping
  public ResponseEntity<?> gets() {
    return ResponseEntity.ok(cadasterService.getUserCadasters());
  }

  @PostMapping(path = "/{cadasterId}/copy")
  public ResponseEntity<?> copy(@PathVariable("cadasterId") final long cadasterId, @RequestBody final CadasterCopyRequest request) {
    return ResponseEntity.ok(cadasterService.copyCadaster(cadasterId, request));
  }
}
```

```java
// internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/config/ApplicationConfig.java
package com.weka.demo.config;

import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.context.annotation.Configuration;

@Configuration
@ConfigurationProperties(prefix = "")
public class ApplicationConfig {
  public static final String BASE_PATH = "/cadasters";
}
```

```java
// internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/service/CadasterService.java
package com.weka.demo.service;

import com.weka.demo.entity.CadasterEntity;
import com.weka.demo.model.CadasterCopyRequest;
import com.weka.demo.repository.CadasterRepository;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor(onConstructor_ = {@Autowired})
public class CadasterService {
  private final CadasterRepository cadasterRepository;

  public List<CadasterEntity> getUserCadasters() {
    return cadasterRepository.findAll();
  }

  public CadasterEntity copyCadaster(final long cadasterId, final CadasterCopyRequest request) {
    final CadasterEntity source = cadasterRepository.findById(cadasterId).orElseThrow();
    return cadasterRepository.save(source.withName(request.name()));
  }
}
```

```java
// internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/repository/CadasterRepository.java
package com.weka.demo.repository;

import com.weka.demo.entity.CadasterEntity;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface CadasterRepository extends JpaRepository<CadasterEntity, Long> {
}
```

```java
// internal/scan/testdata/java_spring_microservice/src/main/java/com/weka/demo/entity/CadasterEntity.java
package com.weka.demo.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;

@Entity
@Table(name = "VD_CADASTER")
public class CadasterEntity {
  @Id
  @Column(name = "CADASTER_ID")
  private Long cadasterId;

  @Column(name = "NAME")
  private String name;

  public CadasterEntity withName(final String value) {
    this.name = value;
    return this;
  }
}
```

- [x] **Step 4: Run test and confirm it fails for missing types/files**

Run:

```bash
go test -count=1 ./internal/scan -run TestRunExtractsWekaStyleSpringIntelligence
```

Expected: compile failure because `SpringIndex`, `SpringComponentRecord`, `SpringEntityRecord`, `SpringDependencyRecord`, `SpringRepositoryRecord`, and `SpringEndpointRecord` do not exist yet.

- [x] **Step 5: Commit the failing fixture test**

```bash
git add internal/scan/java_spring_test.go internal/scan/testdata/java_spring_microservice
git commit -m "test: add WEKA-style Spring scan fixture"
```

## Task 3: Add Java And Spring Data Types

**Files:**
- Create: `internal/scan/java_types.go`
- Modify: `internal/scan/types.go`

- [x] **Step 1: Define additive Java/Spring records**

Create `internal/scan/java_types.go`:

```go
package scan

type JavaSourceRecord struct {
	File        string                 `json:"file"`
	Package     string                 `json:"package,omitempty"`
	Imports     []JavaImportRecord     `json:"imports,omitempty"`
	Types       []JavaTypeRecord       `json:"types,omitempty"`
	Methods     []JavaMethodRecord     `json:"methods,omitempty"`
	Fields      []JavaFieldRecord      `json:"fields,omitempty"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
	Constants   map[string]string      `json:"constants,omitempty"`
}

type JavaImportRecord struct {
	Name     string `json:"name"`
	Static   bool   `json:"static"`
	Line     int    `json:"line"`
	Internal bool   `json:"internal"`
	File     string `json:"file,omitempty"`
}

type JavaTypeRecord struct {
	Name        string                 `json:"name"`
	Kind        string                 `json:"kind"`
	Package     string                 `json:"package,omitempty"`
	File        string                 `json:"file"`
	Line        int                    `json:"line"`
	Extends     string                 `json:"extends,omitempty"`
	Implements  []string               `json:"implements,omitempty"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
}

type JavaMethodRecord struct {
	Name        string                 `json:"name"`
	File        string                 `json:"file"`
	Line        int                    `json:"line"`
	Owner       string                 `json:"owner,omitempty"`
	Visibility  string                 `json:"visibility,omitempty"`
	ReturnType  string                 `json:"return_type,omitempty"`
	Parameters  []JavaParameterRecord  `json:"parameters,omitempty"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
	Calls       []JavaCallRecord       `json:"calls,omitempty"`
}

type JavaFieldRecord struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	File        string                 `json:"file"`
	Line        int                    `json:"line"`
	Owner       string                 `json:"owner,omitempty"`
	Final       bool                   `json:"final"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
}

type JavaParameterRecord struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
}

type JavaAnnotationRecord struct {
	Name       string            `json:"name"`
	Arguments  string            `json:"arguments,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Line       int               `json:"line"`
}

type JavaCallRecord struct {
	Receiver string `json:"receiver,omitempty"`
	Method   string `json:"method"`
	Line     int    `json:"line"`
}

type SpringIndex struct {
	Applications []SpringApplicationRecord `json:"applications,omitempty"`
	Components   []SpringComponentRecord   `json:"components,omitempty"`
	Endpoints    []SpringEndpointRecord    `json:"endpoints,omitempty"`
	Dependencies []SpringDependencyRecord  `json:"dependencies,omitempty"`
	Repositories []SpringRepositoryRecord  `json:"repositories,omitempty"`
	Entities     []SpringEntityRecord      `json:"entities,omitempty"`
	Beans        []SpringBeanRecord        `json:"beans,omitempty"`
}

type SpringApplicationRecord struct {
	Name             string `json:"name"`
	File             string `json:"file"`
	Line             int    `json:"line"`
	ScanBasePackages string `json:"scan_base_packages,omitempty"`
}

type SpringComponentRecord struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Package     string   `json:"package,omitempty"`
	Annotations []string `json:"annotations,omitempty"`
}

type SpringEndpointRecord struct {
	HTTPMethod  string                `json:"http_method"`
	Path        string                `json:"path"`
	Controller  string                `json:"controller"`
	Method      string                `json:"method"`
	File        string                `json:"file"`
	Line        int                   `json:"line"`
	RequestType string                `json:"request_type,omitempty"`
	ReturnType  string                `json:"return_type,omitempty"`
	Parameters  []JavaParameterRecord `json:"parameters,omitempty"`
}

type SpringDependencyRecord struct {
	From      string `json:"from"`
	To        string `json:"to"`
	FromFile  string `json:"from_file"`
	ToFile    string `json:"to_file,omitempty"`
	Field     string `json:"field,omitempty"`
	Injection string `json:"injection"`
	Line      int    `json:"line"`
}

type SpringRepositoryRecord struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Entity     string `json:"entity,omitempty"`
	EntityFile string `json:"entity_file,omitempty"`
	IDType     string `json:"id_type,omitempty"`
}

type SpringEntityRecord struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Table   string `json:"table,omitempty"`
	Package string `json:"package,omitempty"`
}

type SpringBeanRecord struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	Config     string `json:"config,omitempty"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	MethodName string `json:"method_name,omitempty"`
}
```

- [x] **Step 2: Add manifest metadata types**

Modify `internal/scan/types.go`:

```go
type Manifest struct {
	Tool        string       `json:"tool"`
	Schema      int          `json:"schema"`
	OutputDir   string       `json:"output_dir"`
	Files       int          `json:"files"`
	Skipped     int          `json:"skipped"`
	Generated   []string     `json:"generated"`
	ProjectRoot string       `json:"project_root,omitempty"`
	GeneratedAt string       `json:"generated_at,omitempty"`
	Git         *GitMetadata `json:"git,omitempty"`
}

type GitMetadata struct {
	Commit string `json:"commit,omitempty"`
	Branch string `json:"branch,omitempty"`
	Dirty  bool   `json:"dirty"`
}
```

- [x] **Step 3: Run focused compile**

Run:

```bash
go test -count=1 ./internal/scan -run TestRunExtractsWekaStyleSpringIntelligence
```

Expected: test compiles and fails because `spring.json` and related outputs are not written.

- [x] **Step 4: Commit types**

```bash
git add internal/scan/java_types.go internal/scan/types.go
git commit -m "feat: add Java Spring scan records"
```

## Task 4: Replace Naive Java Regex With Structured Source Extraction

**Files:**
- Create: `internal/scan/extract_java.go`
- Modify: `internal/scan/extract.go`
- Test: `internal/scan/java_spring_test.go`

- [x] **Step 1: Add focused Java extraction tests**

Append:

```go
func TestExtractJavaSourceIgnoresKeywordNoiseAndCapturesMethods(t *testing.T) {
	file := FileRecord{Path: "src/main/java/com/weka/demo/service/CadasterService.java", Language: "java"}
	body := `package com.weka.demo.service;

import java.util.List;

@Service
public class CadasterService {
  private final CadasterRepository cadasterRepository;

  public List<CadasterEntity> getUserCadasters() {
    return cadasterRepository.findAll().stream().map(item -> item).toList();
  }
}`

	source := extractJavaSource(file, body)

	if source.Package != "com.weka.demo.service" {
		t.Fatalf("package = %q", source.Package)
	}
	assertJavaType(t, source.Types, "CadasterService", "class")
	assertJavaMethod(t, source.Methods, "getUserCadasters", "CadasterService")
	assertJavaField(t, source.Fields, "cadasterRepository", "CadasterRepository")
	for _, typ := range source.Types {
		if typ.Name == "for" {
			t.Fatalf("unexpected keyword type in %#v", source.Types)
		}
	}
}
```

- [x] **Step 2: Implement `extractJavaSource`**

Create `internal/scan/extract_java.go` with:

```go
package scan

import (
	"regexp"
	"strings"
)

var (
	javaPackageLineRE    = regexp.MustCompile(`^\s*package\s+([A-Za-z_][A-Za-z0-9_.]*);`)
	javaImportLineRE     = regexp.MustCompile(`^\s*import\s+(static\s+)?([^;]+);`)
	javaTypeLineRE       = regexp.MustCompile(`^\s*(?:public|protected|private|abstract|final|sealed|non-sealed|static|\s)*\s*(class|interface|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)\b(.*)$`)
	javaMethodLineRE     = regexp.MustCompile(`^\s*(public|protected|private)?\s*(?:static\s+)?(?:final\s+)?([A-Za-z_][A-Za-z0-9_<>, ?\[\].]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*(?:throws\s+[^{]+)?\{?\s*$`)
	javaFieldLineRE      = regexp.MustCompile(`^\s*(?:public|protected|private)?\s*(final\s+)?([A-Za-z_][A-Za-z0-9_<>, ?\[\].]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:=.*)?;\s*$`)
	javaAnnotationLineRE = regexp.MustCompile(`^\s*@([A-Za-z_][A-Za-z0-9_.]*)(?:\((.*)\))?\s*$`)
	javaConstantLineRE   = regexp.MustCompile(`^\s*(?:public|protected|private)?\s*static\s+final\s+String\s+([A-Z0-9_]+)\s*=\s*"([^"]*)"\s*;`)
	javaCallRE           = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

func extractJavaSource(file FileRecord, body string) JavaSourceRecord {
	source := JavaSourceRecord{File: file.Path, Constants: map[string]string{}}
	lines := strings.Split(body, "\n")
	var pending []JavaAnnotationRecord
	currentOwner := ""
	braceDepth := 0

	for index, raw := range lines {
		lineNo := index + 1
		line := stripJavaLineNoise(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}

		if match := javaPackageLineRE.FindStringSubmatch(line); len(match) == 2 {
			source.Package = match[1]
			continue
		}
		if match := javaImportLineRE.FindStringSubmatch(line); len(match) == 3 {
			source.Imports = append(source.Imports, JavaImportRecord{Name: strings.TrimSpace(match[2]), Static: strings.TrimSpace(match[1]) == "static", Line: lineNo})
			continue
		}
		if match := javaAnnotationLineRE.FindStringSubmatch(line); len(match) == 3 {
			annotation := JavaAnnotationRecord{Name: shortJavaName(match[1]), Arguments: strings.TrimSpace(match[2]), Attributes: parseJavaAnnotationAttributes(match[2]), Line: lineNo}
			pending = append(pending, annotation)
			source.Annotations = append(source.Annotations, annotation)
			continue
		}
		if match := javaConstantLineRE.FindStringSubmatch(line); len(match) == 3 {
			source.Constants[match[1]] = match[2]
		}
		if match := javaTypeLineRE.FindStringSubmatch(line); len(match) == 4 {
			currentOwner = match[2]
			source.Types = append(source.Types, JavaTypeRecord{
				Name:        match[2],
				Kind:        match[1],
				Package:     source.Package,
				File:        file.Path,
				Line:        lineNo,
				Annotations: pending,
			})
			pending = nil
		} else if match := javaFieldLineRE.FindStringSubmatch(line); len(match) == 4 && currentOwner != "" && !strings.Contains(line, "(") {
			source.Fields = append(source.Fields, JavaFieldRecord{
				Name:        match[3],
				Type:        strings.TrimSpace(match[2]),
				File:        file.Path,
				Line:        lineNo,
				Owner:       currentOwner,
				Final:       strings.TrimSpace(match[1]) == "final",
				Annotations: pending,
			})
			pending = nil
		} else if match := javaMethodLineRE.FindStringSubmatch(line); len(match) == 5 && currentOwner != "" && !isJavaControlLine(line) {
			source.Methods = append(source.Methods, JavaMethodRecord{
				Name:        match[3],
				File:        file.Path,
				Line:        lineNo,
				Owner:       currentOwner,
				Visibility:  strings.TrimSpace(match[1]),
				ReturnType:  strings.TrimSpace(match[2]),
				Parameters:  parseJavaParameters(match[4]),
				Annotations: pending,
				Calls:       extractJavaCalls(line, lineNo),
			})
			pending = nil
		} else if len(source.Methods) > 0 {
			last := &source.Methods[len(source.Methods)-1]
			last.Calls = append(last.Calls, extractJavaCalls(line, lineNo)...)
		}

		braceDepth += strings.Count(line, "{")
		braceDepth -= strings.Count(line, "}")
		if braceDepth <= 0 {
			currentOwner = ""
			braceDepth = 0
		}
	}

	return source
}
```

- [x] **Step 3: Add helper functions**

Add these functions to `extract_java.go`:

```go
func stripJavaLineNoise(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		line = line[:index]
	}
	return strings.TrimSpace(line)
}

func shortJavaName(name string) string {
	name = strings.TrimSpace(name)
	if index := strings.LastIndex(name, "."); index >= 0 {
		return name[index+1:]
	}
	return name
}

func isJavaControlLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{"if ", "for ", "while ", "switch ", "catch ", "return "} {
		if strings.HasPrefix(trimmed, prefix) || strings.HasPrefix(trimmed, strings.TrimSpace(prefix)+"(") {
			return true
		}
	}
	return false
}

func parseJavaAnnotationAttributes(args string) map[string]string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	attrs := map[string]string{}
	for _, part := range strings.Split(args, ",") {
		piece := strings.TrimSpace(part)
		if piece == "" {
			continue
		}
		key := "value"
		value := piece
		if index := strings.Index(piece, "="); index >= 0 {
			key = strings.TrimSpace(piece[:index])
			value = strings.TrimSpace(piece[index+1:])
		}
		attrs[key] = strings.Trim(value, `"`)
	}
	return attrs
}

func parseJavaParameters(params string) []JavaParameterRecord {
	params = strings.TrimSpace(params)
	if params == "" {
		return nil
	}
	var records []JavaParameterRecord
	for _, raw := range strings.Split(params, ",") {
		part := strings.TrimSpace(raw)
		part = strings.ReplaceAll(part, "final ", "")
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		name := fields[len(fields)-1]
		typ := strings.Join(fields[:len(fields)-1], " ")
		records = append(records, JavaParameterRecord{Name: strings.TrimSpace(name), Type: strings.TrimSpace(typ)})
	}
	return records
}

func extractJavaCalls(line string, lineNo int) []JavaCallRecord {
	var calls []JavaCallRecord
	for _, match := range javaCallRE.FindAllStringSubmatch(line, -1) {
		if len(match) == 3 {
			calls = append(calls, JavaCallRecord{Receiver: match[1], Method: match[2], Line: lineNo})
		}
	}
	return calls
}
```

- [x] **Step 4: Delegate Java symbol/relation extraction**

Modify `extractSymbols` in `internal/scan/extract.go` for `case "java"`:

```go
case "java":
	source := extractJavaSource(file, body)
	for _, typ := range source.Types {
		symbols = append(symbols, SymbolRecord{Name: typ.Name, Kind: typ.Kind, File: typ.File, Line: typ.Line})
	}
	for _, method := range source.Methods {
		symbols = append(symbols, SymbolRecord{Name: method.Name, Kind: "method", File: method.File, Line: method.Line})
	}
	return symbols
```

Modify `extractRelations` in `internal/scan/extract.go` for Java:

```go
case "java":
	source := extractJavaSource(file, body)
	for _, imported := range source.Imports {
		relations = append(relations, RelationRecord{From: file.Path, To: imported.Name, Type: "imports", Line: imported.Line})
	}
	return relations
```

- [x] **Step 5: Run tests**

Run:

```bash
gofmt -w internal/scan/extract.go internal/scan/extract_java.go internal/scan/java_spring_test.go
go test -count=1 ./internal/scan
```

Expected: Java extraction tests pass; acceptance test still fails because Spring outputs are absent.

- [x] **Step 6: Commit Java extraction**

```bash
git add internal/scan/extract.go internal/scan/extract_java.go internal/scan/java_spring_test.go
git commit -m "feat: add structured Java extraction"
```

## Task 5: Resolve Internal And External Java Imports

**Files:**
- Create: `internal/scan/java_resolve.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/graph.go`
- Test: `internal/scan/java_spring_test.go`

- [x] **Step 1: Add resolver tests**

Append:

```go
func TestResolveJavaImportsClassifiesInternalAndExternal(t *testing.T) {
	files := []FileRecord{
		{Path: "src/main/java/com/weka/demo/controller/CadasterController.java", Language: "java"},
		{Path: "src/main/java/com/weka/demo/service/CadasterService.java", Language: "java"},
	}
	sources := []JavaSourceRecord{
		{
			File:    files[0].Path,
			Package: "com.weka.demo.controller",
			Imports: []JavaImportRecord{
				{Name: "com.weka.demo.service.CadasterService", Line: 3},
				{Name: "org.springframework.web.bind.annotation.GetMapping", Line: 4},
			},
		},
		{
			File:    files[1].Path,
			Package: "com.weka.demo.service",
			Types:   []JavaTypeRecord{{Name: "CadasterService", Package: "com.weka.demo.service", File: files[1].Path}},
		},
	}

	relations := resolveJavaImportRelations(sources)

	assertHasRelation(t, relations, files[0].Path, files[1].Path, "imports_internal")
	assertHasRelation(t, relations, files[0].Path, "org.springframework.web.bind.annotation.GetMapping", "imports_external")
}
```

- [x] **Step 2: Implement resolver**

Create `internal/scan/java_resolve.go`:

```go
package scan

func resolveJavaImportRelations(sources []JavaSourceRecord) []RelationRecord {
	typesByQualifiedName := map[string]JavaTypeRecord{}
	for _, source := range sources {
		for _, typ := range source.Types {
			qualified := typ.Name
			if typ.Package != "" {
				qualified = typ.Package + "." + typ.Name
			}
			typesByQualifiedName[qualified] = typ
		}
	}

	var relations []RelationRecord
	for _, source := range sources {
		for _, imported := range source.Imports {
			if typ, ok := typesByQualifiedName[imported.Name]; ok {
				relations = append(relations, RelationRecord{From: source.File, To: typ.File, Type: "imports_internal", Line: imported.Line})
			} else {
				relations = append(relations, RelationRecord{From: source.File, To: imported.Name, Type: "imports_external", Line: imported.Line})
			}
		}
	}
	return relations
}
```

- [x] **Step 3: Wire resolver into scan result assembly**

In `internal/scan/scan.go`, collect Java sources while reading files:

```go
var javaSources []JavaSourceRecord
...
if file.Language == "java" {
	javaSources = append(javaSources, extractJavaSource(file, string(body)))
}
```

After ordinary relations are collected:

```go
relations = replaceJavaImportRelations(relations, resolveJavaImportRelations(javaSources))
```

Add helper in `java_resolve.go`:

```go
func replaceJavaImportRelations(existing []RelationRecord, javaRelations []RelationRecord) []RelationRecord {
	var result []RelationRecord
	for _, relation := range existing {
		if relation.Type == "imports" && isJavaSourcePath(relation.From) {
			continue
		}
		result = append(result, relation)
	}
	result = append(result, javaRelations...)
	return result
}

func isJavaSourcePath(path string) bool {
	return strings.HasSuffix(path, ".java")
}
```

Add `strings` import to `java_resolve.go`.

- [x] **Step 4: Update graph edges**

Modify `internal/scan/graph.go` so `imports_internal` creates `file:* -> file:*` edges and `imports_external` creates `file:* -> dependency:*` edges. Keep existing `imports` behavior for non-Java.

- [x] **Step 5: Run tests**

Run:

```bash
gofmt -w internal/scan/java_resolve.go internal/scan/scan.go internal/scan/graph.go internal/scan/java_spring_test.go
go test -count=1 ./internal/scan
```

Expected: resolver tests pass; existing Go/JS/Python/PHP/Shell tests pass.

- [x] **Step 6: Commit resolver**

```bash
git add internal/scan/java_resolve.go internal/scan/scan.go internal/scan/graph.go internal/scan/java_spring_test.go
git commit -m "feat: classify Java imports"
```

## Task 6: Extract Spring Components, Endpoints, Beans, Repositories, And Entities

**Files:**
- Create: `internal/scan/spring_extract.go`
- Modify: `internal/scan/scan.go`
- Test: `internal/scan/java_spring_test.go`

- [x] **Step 1: Add Spring extraction unit tests**

Append:

```go
func TestBuildSpringIndexExtractsEndpointsAndDependencies(t *testing.T) {
	sources := []JavaSourceRecord{
		{
			File:    "src/main/java/com/weka/demo/controller/CadasterController.java",
			Package: "com.weka.demo.controller",
			Types: []JavaTypeRecord{{
				Name: "CadasterController", Kind: "class", File: "src/main/java/com/weka/demo/controller/CadasterController.java", Line: 12,
				Annotations: []JavaAnnotationRecord{{Name: "RestController", Line: 10}, {Name: "RequestMapping", Arguments: "BASE_PATH", Attributes: map[string]string{"value": "BASE_PATH"}, Line: 11}, {Name: "RequiredArgsConstructor", Line: 12}},
			}},
			Fields: []JavaFieldRecord{{Name: "cadasterService", Type: "CadasterService", Owner: "CadasterController", Final: true, Line: 14}},
			Methods: []JavaMethodRecord{{
				Name: "copy", Owner: "CadasterController", ReturnType: "ResponseEntity<?>", File: "src/main/java/com/weka/demo/controller/CadasterController.java", Line: 20,
				Annotations: []JavaAnnotationRecord{{Name: "PostMapping", Attributes: map[string]string{"path": "/{cadasterId}/copy"}, Line: 19}},
				Parameters: []JavaParameterRecord{{Name: "request", Type: "CadasterCopyRequest", Annotations: []JavaAnnotationRecord{{Name: "RequestBody"}}}},
			}},
			Constants: map[string]string{"BASE_PATH": "/cadasters"},
		},
		{
			File:    "src/main/java/com/weka/demo/service/CadasterService.java",
			Package: "com.weka.demo.service",
			Types: []JavaTypeRecord{{
				Name: "CadasterService", Kind: "class", File: "src/main/java/com/weka/demo/service/CadasterService.java", Line: 8,
				Annotations: []JavaAnnotationRecord{{Name: "Service", Line: 7}},
			}},
		},
	}

	index := buildSpringIndex(sources)

	assertHasSpringComponent(t, index.Components, "CadasterController", "rest_controller")
	assertHasSpringComponent(t, index.Components, "CadasterService", "service")
	assertHasSpringEndpoint(t, index.Endpoints, "POST", "/cadasters/{cadasterId}/copy", "CadasterController", "copy")
	assertHasSpringDependency(t, index.Dependencies, "CadasterController", "CadasterService", "constructor")
}
```

- [x] **Step 2: Implement Spring index builder**

Create `internal/scan/spring_extract.go`:

```go
package scan

import (
	"sort"
	"strings"
)

func buildSpringIndex(sources []JavaSourceRecord) SpringIndex {
	typeByName := map[string]JavaTypeRecord{}
	fileByType := map[string]string{}
	for _, source := range sources {
		for _, typ := range source.Types {
			typeByName[typ.Name] = typ
			fileByType[typ.Name] = typ.File
		}
	}

	var index SpringIndex
	for _, source := range sources {
		for _, typ := range source.Types {
			componentKind := springComponentKind(typ.Annotations)
			if hasAnnotation(typ.Annotations, "SpringBootApplication") {
				index.Applications = append(index.Applications, SpringApplicationRecord{Name: typ.Name, File: typ.File, Line: typ.Line, ScanBasePackages: firstAnnotationValue(typ.Annotations, "SpringBootApplication", "scanBasePackages")})
			}
			if componentKind != "" {
				index.Components = append(index.Components, SpringComponentRecord{Name: typ.Name, Kind: componentKind, File: typ.File, Line: typ.Line, Package: typ.Package, Annotations: annotationNames(typ.Annotations)})
			}
			if hasAnnotation(typ.Annotations, "Entity") {
				index.Entities = append(index.Entities, SpringEntityRecord{Name: typ.Name, File: typ.File, Line: typ.Line, Table: firstAnnotationValue(typ.Annotations, "Table", "name"), Package: typ.Package})
			}
			if componentKind == "repository" {
				repository := SpringRepositoryRecord{Name: typ.Name, File: typ.File, Line: typ.Line}
				repository.Entity, repository.IDType = parseRepositoryGeneric(typ.Extends)
				repository.EntityFile = fileByType[repository.Entity]
				index.Repositories = append(index.Repositories, repository)
			}
		}

		for _, method := range source.Methods {
			if hasAnnotation(method.Annotations, "Bean") {
				index.Beans = append(index.Beans, SpringBeanRecord{Name: beanName(method), Type: method.ReturnType, Config: method.Owner, File: method.File, Line: method.Line, MethodName: method.Name})
			}
			endpoints := springEndpointsForMethod(source, method)
			index.Endpoints = append(index.Endpoints, endpoints...)
		}

		for _, field := range source.Fields {
			if field.Owner == "" {
				continue
			}
			if _, ok := typeByName[field.Type]; !ok {
				continue
			}
			injection := "field"
			if field.Final && typeHasAnnotation(sources, field.Owner, "RequiredArgsConstructor") {
				injection = "constructor"
			}
			index.Dependencies = append(index.Dependencies, SpringDependencyRecord{
				From: field.Owner, To: field.Type, FromFile: field.File, ToFile: fileByType[field.Type], Field: field.Name, Injection: injection, Line: field.Line,
			})
		}
	}

	sortSpringIndex(&index)
	return index
}
```

- [x] **Step 3: Add endpoint helpers**

Add to `spring_extract.go`:

```go
func springEndpointsForMethod(source JavaSourceRecord, method JavaMethodRecord) []SpringEndpointRecord {
	controller := ""
	classPath := ""
	for _, typ := range source.Types {
		if typ.Name == method.Owner && hasAnnotation(typ.Annotations, "RestController") {
			controller = typ.Name
			classPath = resolveSpringPath(source, firstMappingAnnotation(typ.Annotations))
			break
		}
	}
	if controller == "" {
		return nil
	}

	annotation := firstMappingAnnotation(method.Annotations)
	if annotation.Name == "" {
		return nil
	}

	httpMethods := springHTTPMethods(annotation)
	paths := splitSpringPaths(resolveSpringPath(source, annotation))
	if len(paths) == 0 {
		paths = []string{""}
	}

	var endpoints []SpringEndpointRecord
	for _, httpMethod := range httpMethods {
		for _, methodPath := range paths {
			endpoints = append(endpoints, SpringEndpointRecord{
				HTTPMethod:  httpMethod,
				Path:        joinSpringPaths(classPath, methodPath),
				Controller:  controller,
				Method:      method.Name,
				File:        method.File,
				Line:        annotation.Line,
				RequestType: requestBodyType(method.Parameters),
				ReturnType:  method.ReturnType,
				Parameters:  method.Parameters,
			})
		}
	}
	return endpoints
}
```

- [x] **Step 4: Add deterministic sorting**

Add:

```go
func sortSpringIndex(index *SpringIndex) {
	sort.Slice(index.Components, func(i, j int) bool { return index.Components[i].Name < index.Components[j].Name })
	sort.Slice(index.Endpoints, func(i, j int) bool {
		if index.Endpoints[i].Path != index.Endpoints[j].Path {
			return index.Endpoints[i].Path < index.Endpoints[j].Path
		}
		return index.Endpoints[i].HTTPMethod < index.Endpoints[j].HTTPMethod
	})
	sort.Slice(index.Dependencies, func(i, j int) bool {
		if index.Dependencies[i].From != index.Dependencies[j].From {
			return index.Dependencies[i].From < index.Dependencies[j].From
		}
		return index.Dependencies[i].To < index.Dependencies[j].To
	})
	sort.Slice(index.Repositories, func(i, j int) bool { return index.Repositories[i].Name < index.Repositories[j].Name })
	sort.Slice(index.Entities, func(i, j int) bool { return index.Entities[i].Name < index.Entities[j].Name })
	sort.Slice(index.Applications, func(i, j int) bool { return index.Applications[i].Name < index.Applications[j].Name })
	sort.Slice(index.Beans, func(i, j int) bool { return index.Beans[i].Name < index.Beans[j].Name })
}
```

- [x] **Step 5: Wire into `Run`**

In `internal/scan/scan.go`, after Java sources are collected:

```go
springIndex := buildSpringIndex(javaSources)
```

Write:

```go
if err := writeJSON(filepath.Join(outDir, "spring.json"), springIndex); err != nil {
	return Result{}, err
}
```

- [x] **Step 6: Run tests**

Run:

```bash
gofmt -w internal/scan/spring_extract.go internal/scan/scan.go internal/scan/java_spring_test.go
go test -count=1 ./internal/scan
```

Expected: `spring.json` assertions pass; report file assertions still fail until reports are added.

- [x] **Step 7: Commit Spring extraction**

```bash
git add internal/scan/spring_extract.go internal/scan/scan.go internal/scan/java_spring_test.go
git commit -m "feat: extract Spring service intelligence"
```

## Task 7: Render Endpoint, Dependency, Entrypoint, And Test Reports

**Files:**
- Create: `internal/scan/spring_report.go`
- Modify: `internal/scan/report.go`
- Modify: `internal/scan/scan.go`
- Test: `internal/scan/java_spring_test.go`

- [x] **Step 1: Add report renderers**

Create `internal/scan/spring_report.go`:

```go
package scan

import (
	"fmt"
	"sort"
	"strings"
)

func renderEndpointsReport(index SpringIndex) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Endpoints\n\n")
	if len(index.Endpoints) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, endpoint := range index.Endpoints {
		b.WriteString(fmt.Sprintf("- %s `%s` - `%s.%s`", endpoint.HTTPMethod, endpoint.Path, endpoint.Controller, endpoint.Method))
		if endpoint.RequestType != "" {
			b.WriteString(fmt.Sprintf(" - request `%s`", endpoint.RequestType))
		}
		if endpoint.ReturnType != "" {
			b.WriteString(fmt.Sprintf(" - returns `%s`", endpoint.ReturnType))
		}
		b.WriteString(fmt.Sprintf(" - %s:%d\n", endpoint.File, endpoint.Line))
	}
	return b.String()
}

func renderDependenciesReport(index SpringIndex) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Dependencies\n\n")
	if len(index.Dependencies) == 0 && len(index.Repositories) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	if len(index.Dependencies) > 0 {
		b.WriteString("## Spring Beans\n\n")
		for _, dependency := range index.Dependencies {
			b.WriteString(fmt.Sprintf("- `%s` -> `%s` (%s", dependency.From, dependency.To, dependency.Injection))
			if dependency.Field != "" {
				b.WriteString(fmt.Sprintf(", field `%s`", dependency.Field))
			}
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}
	if len(index.Repositories) > 0 {
		b.WriteString("## Repositories\n\n")
		for _, repository := range index.Repositories {
			if repository.Entity != "" {
				b.WriteString(fmt.Sprintf("- `%s` -> `%s`", repository.Name, repository.Entity))
				if repository.IDType != "" {
					b.WriteString(fmt.Sprintf(" (id `%s`)", repository.IDType))
				}
				b.WriteString("\n")
			} else {
				b.WriteString(fmt.Sprintf("- `%s`\n", repository.Name))
			}
		}
	}
	return b.String()
}

func springEntrypointLines(index SpringIndex) []string {
	var lines []string
	for _, app := range index.Applications {
		lines = append(lines, fmt.Sprintf("- `%s` - Spring Boot application `%s`", app.File, app.Name))
	}
	sort.Strings(lines)
	return lines
}
```

- [x] **Step 2: Include Spring entrypoints**

Change `renderEntrypointsReport` signature:

```go
func renderEntrypointsReport(files []FileRecord, symbols []SymbolRecord, springIndex SpringIndex) string
```

Before the empty check, append:

```go
for _, app := range springIndex.Applications {
	entrypoints[app.File] = append(entrypoints[app.File], "Spring Boot application "+app.Name)
}
for _, component := range springIndex.Components {
	if component.Kind == "rest_controller" {
		entrypoints[component.File] = append(entrypoints[component.File], "Spring REST controller "+component.Name)
	}
}
```

Update the call in `scan.go` to pass `springIndex`.

- [x] **Step 3: Improve Java test mapping**

Add to `spring_extract.go` or a new small file `java_tests.go`:

```go
func javaTestRelations(files []FileRecord, sources []JavaSourceRecord) []RelationRecord {
	productionByType := map[string]JavaTypeRecord{}
	for _, source := range sources {
		if strings.Contains(source.File, "/src/test/") {
			continue
		}
		for _, typ := range source.Types {
			productionByType[typ.Name] = typ
		}
	}

	var relations []RelationRecord
	for _, source := range sources {
		if !strings.Contains(source.File, "/src/test/") {
			continue
		}
		for _, typ := range source.Types {
			name := strings.TrimSuffix(typ.Name, "Test")
			if target, ok := productionByType[name]; ok {
				relations = append(relations, RelationRecord{From: source.File, To: target.File, Type: "tests", Line: typ.Line})
			}
		}
	}
	return relations
}
```

Append these relations in `scan.go`.

- [x] **Step 4: Write new reports**

In `scan.go`, add generated files:

```go
"spring.json",
"endpoints.md",
"dependencies.md",
"workspace.md",
```

Write:

```go
if err := os.WriteFile(filepath.Join(outDir, "endpoints.md"), []byte(renderEndpointsReport(springIndex)), 0o644); err != nil {
	return Result{}, err
}
if err := os.WriteFile(filepath.Join(outDir, "dependencies.md"), []byte(renderDependenciesReport(springIndex)), 0o644); err != nil {
	return Result{}, err
}
```

- [x] **Step 5: Run tests**

Run:

```bash
gofmt -w internal/scan/report.go internal/scan/spring_report.go internal/scan/spring_extract.go internal/scan/scan.go
go test -count=1 ./internal/scan
```

Expected: `TestRunExtractsWekaStyleSpringIntelligence` passes.

- [x] **Step 6: Commit reports**

```bash
git add internal/scan/report.go internal/scan/spring_report.go internal/scan/spring_extract.go internal/scan/scan.go internal/scan/java_spring_test.go
git commit -m "feat: render Spring intelligence reports"
```

## Task 8: Add Workspace Metadata For Maven And Frontend Monorepos

**Files:**
- Create: `internal/scan/workspace_extract.go`
- Create: `internal/scan/workspace_report.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/scan.go`
- Test: `internal/scan/workspace_test.go`

- [x] **Step 1: Add workspace types**

Add to `internal/scan/types.go`:

```go
type WorkspaceIndex struct {
	MavenPackages []MavenPackageRecord `json:"maven_packages,omitempty"`
	NodePackages  []NodePackageRecord  `json:"node_packages,omitempty"`
}

type MavenPackageRecord struct {
	Path       string `json:"path"`
	GroupID    string `json:"group_id,omitempty"`
	ArtifactID string `json:"artifact_id,omitempty"`
	Version    string `json:"version,omitempty"`
	Parent     string `json:"parent,omitempty"`
}

type NodePackageRecord struct {
	Path         string   `json:"path"`
	Name         string   `json:"name,omitempty"`
	Version      string   `json:"version,omitempty"`
	Private      bool     `json:"private"`
	PackageManager string `json:"package_manager,omitempty"`
	Workspaces    []string `json:"workspaces,omitempty"`
	Scripts       []string `json:"scripts,omitempty"`
}
```

- [x] **Step 2: Add tests**

Create `internal/scan/workspace_test.go`:

```go
package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestRunWritesWorkspaceReportForMavenAndNode(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pom.xml", `<project><groupId>com.weka</groupId><artifactId>ms-demo</artifactId><version>1.0.0</version></project>`)
	writeFile(t, root, "frontend/package.json", `{"name":"demo-ui","private":true,"workspaces":["apps/*"],"scripts":{"dev":"vite dev","test":"vitest"},"packageManager":"yarn@4.0.0"}`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	report := readText(t, filepath.Join(root, "goregraph-out", "workspace.md"))
	if !strings.Contains(report, "ms-demo") || !strings.Contains(report, "demo-ui") || !strings.Contains(report, "dev") {
		t.Fatalf("workspace report missing expected metadata:\n%s", report)
	}
}
```

- [x] **Step 3: Implement workspace extraction**

Create `internal/scan/workspace_extract.go` using `encoding/xml` for `pom.xml` and `encoding/json` for `package.json`. Extract only metadata from already scanned files; do not run Maven, npm, or yarn.

- [x] **Step 4: Render `workspace.md`**

Create `internal/scan/workspace_report.go` with sections:

```markdown
# GoreGraph Workspace

## Maven Packages

- `pom.xml` - `com.weka:ms-demo:1.0.0`

## Node Packages

- `frontend/package.json` - `demo-ui` - package manager `yarn@4.0.0` - scripts `dev`, `test`
```

- [x] **Step 5: Wire output**

Write `workspace.md` from `scan.go` and include it in `manifest.generated`.

- [x] **Step 6: Run tests and commit**

```bash
gofmt -w internal/scan/workspace_extract.go internal/scan/workspace_report.go internal/scan/workspace_test.go internal/scan/types.go internal/scan/scan.go
go test -count=1 ./internal/scan
git add internal/scan
git commit -m "feat: add workspace intelligence"
```

## Task 9: Add Git Metadata To Manifest

**Files:**
- Create: `internal/scan/git_metadata.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/scan_test.go`

- [x] **Step 1: Add no-git-safe unit test**

Add to `internal/scan/scan_test.go`:

```go
func TestRunManifestIncludesGeneratedAtAndNoGitOutsideRepo(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var manifest Manifest
	readJSON(t, filepath.Join(root, "goregraph-out", "manifest.json"), &manifest)
	if manifest.GeneratedAt == "" {
		t.Fatal("GeneratedAt is empty")
	}
	if manifest.Git != nil {
		t.Fatalf("Git = %#v, want nil outside git repo", manifest.Git)
	}
}
```

- [x] **Step 2: Implement Git metadata**

Create `internal/scan/git_metadata.go`:

```go
package scan

import (
	"bytes"
	"os/exec"
	"strings"
)

func readGitMetadata(root string) *GitMetadata {
	commit, ok := gitOutput(root, "rev-parse", "HEAD")
	if !ok {
		return nil
	}
	branch, _ := gitOutput(root, "rev-parse", "--abbrev-ref", "HEAD")
	status, _ := gitOutput(root, "status", "--porcelain")
	return &GitMetadata{Commit: commit, Branch: branch, Dirty: strings.TrimSpace(status) != ""}
}

func gitOutput(root string, args ...string) (string, bool) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return strings.TrimSpace(stdout.String()), true
}
```

- [x] **Step 3: Set manifest metadata**

In `scan.go`, set:

```go
GeneratedAt: time.Now().UTC().Format(time.RFC3339),
Git:         readGitMetadata(root),
```

The existing deterministic manifest golden test must be changed to assert stable fields structurally instead of exact full JSON string.

- [x] **Step 4: Run tests and commit**

```bash
gofmt -w internal/scan/git_metadata.go internal/scan/scan.go internal/scan/scan_test.go
go test -count=1 ./internal/scan
git add internal/scan/git_metadata.go internal/scan/scan.go internal/scan/scan_test.go
git commit -m "feat: include scan manifest metadata"
```

## Task 10: Expose New Intelligence Through Query And MCP

**Files:**
- Modify: `internal/query/query.go`
- Modify: `internal/query/query_test.go`
- Modify: `internal/mcp/mcp.go`
- Modify: `internal/mcp/mcp_test.go`
- Modify: `COMMANDS.md`

- [x] **Step 1: Add query tests**

Add tests that verify:

- `goregraph query <path> graph-full` reads `graph-full.json`.
- `goregraph query <path> symbols-full` reads `symbols-full.json`.
- `goregraph query <path> relations-full` reads `relations-full.json`.
- `goregraph query <path> audit` reads `audit.json`.
- `goregraph query <path> endpoints` reads `endpoints.md`.
- `goregraph query <path> spring` summarizes `spring.json`.
- `goregraph query <path> dependencies` reads `dependencies.md`.

- [x] **Step 2: Implement query aliases**

Map aliases to files:

```go
var queryAliases = map[string]string{
	"files":        "files.json",
	"symbols":      "symbols.json",
	"symbols-full": "symbols-full.json",
	"relations":    "relations.json",
	"relations-full": "relations-full.json",
	"graph":        "graph.json",
	"graph-full":   "graph-full.json",
	"report":       "report.md",
	"modules":      "modules.md",
	"entrypoints":  "entrypoints.md",
	"tests":        "test-map.md",
	"audit":        "audit.json",
	"spring":       "spring.json",
	"endpoints":    "endpoints.md",
	"dependencies": "dependencies.md",
	"workspace":    "workspace.md",
}
```

- [x] **Step 3: Add MCP resources**

Expose these resources in `internal/mcp/mcp.go`:

- `goregraph://graph-full`
- `goregraph://symbols-full`
- `goregraph://relations-full`
- `goregraph://audit`
- `goregraph://spring`
- `goregraph://endpoints`
- `goregraph://dependencies`
- `goregraph://workspace`

- [x] **Step 4: Run tests and commit**

```bash
gofmt -w internal/query/query.go internal/query/query_test.go internal/mcp/mcp.go internal/mcp/mcp_test.go
go test -count=1 ./...
git add internal/query internal/mcp COMMANDS.md
git commit -m "feat: expose code intelligence outputs"
```

## Task 11: Validate Against Real WEKA Projects

**Files:**
- Modify after findings: scanner files touched by failing validation only.
- Do not copy WEKA source into GoreGraph.

- [x] **Step 1: Build local binary**

Run:

```bash
go test -count=1 ./...
go build -o /tmp/goregraph-dev ./cmd/goregraph
```

Expected: tests pass and `/tmp/goregraph-dev` exists.

- [x] **Step 2: Scan `ms-cadaster`**

Run:

```bash
/tmp/goregraph-dev scan /Users/gorecode/projects/weka/microservices/ms-cadaster
```

Expected:

- `goregraph-out/spring.json` exists.
- `goregraph-out/endpoints.md` lists `CadasterController` and `CadasterMgmtController`.
- `goregraph-out/dependencies.md` lists controller/service/repository dependencies.
- `goregraph-out/test-map.md` is not `- none detected`.
- `goregraph-out/symbols.json` does not contain `"name": "for"` with `"kind": "class"`.

- [x] **Step 3: Scan one additional backend service**

Run:

```bash
/tmp/goregraph-dev scan /Users/gorecode/projects/weka/microservices/ms-mailsender
```

Expected:

- Spring Boot application detected.
- Controllers and endpoints detected.
- Entities and repositories detected.
- No generated file contains absolute file paths except `manifest.project_root` basename.

- [x] **Step 4: Scan frontend monorepo root**

Run:

```bash
/tmp/goregraph-dev scan /Users/gorecode/projects/weka/frontend/frontend-monorepo
```

Expected:

- `workspace.md` lists root package and workspace packages.
- Existing JS/TS symbols and relations still work.
- No `node_modules`, `dist`, or `build` content is scanned.

- [x] **Step 5: Commit validation fixes**

If validation required code changes:

```bash
go test -count=1 ./...
git add internal README.md COMMANDS.md
git commit -m "fix: harden scanners against WEKA projects"
```

If validation required no code changes:

```bash
git status --short
```

Expected: clean or only intentional documentation changes.

## Task 12: Update Documentation

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/RELEASE.md`

- [x] **Step 1: Update README output section**

Document that `goregraph scan <path>` now writes:

- `spring.json`: Spring components, endpoints, dependencies, repositories, entities, beans.
- `endpoints.md`: REST endpoint overview for humans and AI assistants.
- `dependencies.md`: bean and repository dependency overview.
- `workspace.md`: Maven/package workspace overview.

- [x] **Step 2: Update command reference**

In `COMMANDS.md`, include examples:

```bash
goregraph scan .
goregraph query . endpoints
goregraph query . spring
goregraph query . dependencies
goregraph report .
```

Explain that `<path>` is the project root to scan or read, for example:

```bash
goregraph scan /Users/gorecode/projects/weka/microservices/ms-cadaster
```

- [x] **Step 3: Update release notes**

Add a release note section for the next version:

```markdown
## Next

- Added Java/Spring intelligence output for Spring Boot services.
- Added endpoint and dependency reports.
- Added Maven/package workspace summary.
- Added manifest Git metadata.
- Hardened Java symbol extraction to avoid keyword false positives.
```

- [x] **Step 4: Run docs smoke check**

Run:

```bash
rg -n "spring.json|endpoints.md|dependencies.md|workspace.md|goregraph query . endpoints" README.md COMMANDS.md docs/RELEASE.md
```

Expected: each new output and command appears in user-facing docs.

- [x] **Step 5: Commit docs**

```bash
git add README.md COMMANDS.md docs/RELEASE.md
git commit -m "docs: document Spring intelligence outputs"
```

## Task 13: Final Verification

**Files:**
- No new files unless verification exposes defects.

- [x] **Step 1: Run complete checks**

```bash
gofmt -l .
go test -count=1 ./...
go vet ./...
```

Expected:

- `gofmt -l .` prints nothing.
- `go test -count=1 ./...` passes.
- `go vet ./...` passes.

- [x] **Step 2: Run CLI smoke test**

```bash
go run ./cmd/goregraph scan internal/scan/testdata/java_spring_microservice
go run ./cmd/goregraph query internal/scan/testdata/java_spring_microservice graph-full
go run ./cmd/goregraph query internal/scan/testdata/java_spring_microservice endpoints
go run ./cmd/goregraph query internal/scan/testdata/java_spring_microservice dependencies
go run ./cmd/goregraph report internal/scan/testdata/java_spring_microservice
```

Expected:

- Scan succeeds.
- Rich graph query prints file and symbol nodes.
- Endpoint query prints `GET /cadasters` and `POST /cadasters/{cadasterId}/copy`.
- Dependency query prints `CadasterController -> CadasterService`.
- Report command succeeds.

- [x] **Step 3: Check working tree**

```bash
git status --short
```

Expected: clean after final commit.

## Implementation Boundaries

- GoreGraph must not execute Maven, Gradle, npm, yarn, pnpm, Java, Node, or project scripts during scanning.
- GoreGraph must not require a JDK or Node runtime to scan source files.
- All paths in generated output must stay project-root-relative, except manifest metadata that intentionally names the scanned project root basename.
- Scanner output must be deterministic for the same input tree, except `manifest.generated_at` and Git dirty state.
- WEKA source files must not be copied into GoreGraph. Fixtures must be synthetic but patterned after observed structure.
- Full language-level type inference is outside this plan. The first useful graph is extracted structure, imports/includes/sources, test relationships, file-symbol containment, and adapter-specific relationships where deterministic extraction is practical.
- Java/Spring adds extra deterministic depth through injected fields and receiver method calls such as `cadasterService.copyCadaster(...)`; this is one adapter-specific enhancement, not a core requirement for every language.

## Definition Of Done

- Existing tests pass.
- `graph-full.json`, `symbols-full.json`, and `relations-full.json` are produced for all currently supported languages: Go, JavaScript, TypeScript, Python, PHP, Shell, Java, Markdown, `package.json`, and `composer.json`.
- Existing shallow language support does not regress while Java/Spring depth is added.
- New WEKA-like fixture test passes.
- Real `ms-cadaster` scan produces non-empty `spring.json`, `endpoints.md`, `dependencies.md`, and `test-map.md`.
- `symbols.json` no longer contains Java keyword false positives such as class `for`.
- `entrypoints.md` detects Spring Boot applications and REST controllers.
- `relations.json` distinguishes Java `imports_internal` and `imports_external`.
- `audit.json` confirms normal scans used no network and executed no external project commands.
- `manifest.json` includes generated timestamp and Git metadata when the scanned path is a Git worktree.
- README and command reference explain all new outputs and commands.
