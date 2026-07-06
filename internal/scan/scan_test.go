package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestRunWritesDeterministicFilesManifestAndReport(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	writeFile(t, root, "src/main.go", "package main\nfunc main() {}\n")
	writeFile(t, root, "dist/bundle.js", "ignored")

	result, err := Run(root, config.Defaults())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 2 {
		t.Fatalf("ScannedFiles = %d, want 2", result.ScannedFiles)
	}

	for _, name := range []string{"manifest.json", "files.json", "symbols.json", "relations.json", "graph.json", "report.md", "modules.md", "entrypoints.md", "test-map.md"} {
		if _, err := os.Stat(filepath.Join(root, "goregraph-out", name)); err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
	}

	var files []FileRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "files.json"), &files)
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}
	if files[0].Path != "README.md" || files[1].Path != "src/main.go" {
		t.Fatalf("files sorted/filtered incorrectly: %#v", files)
	}
	for _, file := range files {
		if filepath.IsAbs(file.Path) {
			t.Fatalf("file path %q is absolute, want root-relative", file.Path)
		}
	}
}

func TestRunExtractsSymbolsRelationsAndGraph(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/demo\n")
	writeFile(t, root, "src/main.go", `package main

import "fmt"

type Server struct{}

func main() {
	fmt.Println("hello")
}
`)
	writeFile(t, root, "web/app.ts", `import { api } from "./api";

export class App {}

export function start() {
  api();
}
`)
	writeFile(t, root, "README.md", "# Demo\n\n## Usage\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "module example.test/demo", "module", "go.mod")
	assertHasSymbol(t, symbols, "Server", "type", "src/main.go")
	assertHasSymbol(t, symbols, "main", "package", "src/main.go")
	assertHasSymbol(t, symbols, "main", "function", "src/main.go")
	assertHasSymbol(t, symbols, "App", "class", "web/app.ts")
	assertHasSymbol(t, symbols, "start", "function", "web/app.ts")
	assertHasSymbol(t, symbols, "Demo", "heading", "README.md")

	var relations []RelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations.json"), &relations)
	assertHasRelation(t, relations, "src/main.go", "fmt", "imports")
	assertHasRelation(t, relations, "web/app.ts", "./api", "imports")

	var graph Graph
	readJSON(t, filepath.Join(root, "goregraph-out", "graph.json"), &graph)
	if len(graph.Nodes) == 0 {
		t.Fatal("graph has no nodes")
	}
	if len(graph.Edges) == 0 {
		t.Fatal("graph has no edges")
	}
}

func TestRunExtractsGoSymbolsWithParser(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/server.go", `package service

type Server interface {
	Start() error
}

func NewServer() Server {
	return nil
}

func (s *serverImpl) Start() error {
	return nil
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "service", "package", "src/server.go")
	assertHasSymbol(t, symbols, "Server", "type", "src/server.go")
	assertHasSymbol(t, symbols, "NewServer", "function", "src/server.go")
	assertHasSymbol(t, symbols, "Start", "method", "src/server.go")
}

func TestRunResolvesLocalGoImports(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/demo\n")
	writeFile(t, root, "cmd/api/main.go", `package main

import (
	"fmt"
	"example.test/demo/internal/service"
)

func main() {
	fmt.Println(service.Name)
}
`)
	writeFile(t, root, "internal/service/service.go", "package service\nconst Name = \"demo\"\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var relations []RelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations.json"), &relations)
	assertHasRelation(t, relations, "cmd/api/main.go", "internal/service/service.go", "imports")
	assertHasRelation(t, relations, "cmd/api/main.go", "fmt", "imports")

	var graph Graph
	readJSON(t, filepath.Join(root, "goregraph-out", "graph.json"), &graph)
	assertHasGraphNode(t, graph, "dependency:fmt")
	assertHasGraphEdge(t, graph, "file:cmd/api/main.go", "file:internal/service/service.go", "imports")
	assertHasGraphEdge(t, graph, "file:cmd/api/main.go", "dependency:fmt", "imports")
}

func TestRunExtractsPythonSymbolsRelationsAndEntryPoint(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/service.py", `import os
from app.utils import helper

class Service:
    def run(self):
        return helper()

def main():
    Service().run()

if __name__ == "__main__":
    main()
`)
	writeFile(t, root, "app/utils.py", "def helper():\n    return 'ok'\n")
	writeFile(t, root, "tests/test_service.py", "def test_service():\n    assert True\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "Service", "class", "app/service.py")
	assertHasSymbol(t, symbols, "run", "method", "app/service.py")
	assertHasSymbol(t, symbols, "main", "function", "app/service.py")
	assertHasSymbol(t, symbols, "test_service", "test", "tests/test_service.py")

	var relations []RelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations.json"), &relations)
	assertHasRelation(t, relations, "app/service.py", "os", "imports")
	assertHasRelation(t, relations, "app/service.py", "app/utils.py", "imports")

	entrypoints := readText(t, filepath.Join(root, "goregraph-out", "entrypoints.md"))
	if !strings.Contains(entrypoints, "app/service.py") || !strings.Contains(entrypoints, "Python main guard") {
		t.Fatalf("entrypoints report missing Python main guard:\n%s", entrypoints)
	}
}

func TestRunExtractsPHPSymbolsRelationsAndEntrypoint(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "composer.json", `{"autoload":{"psr-4":{"App\\":"src/"}}}`)
	writeFile(t, root, "public/index.php", `<?php
require_once __DIR__ . '/../src/Service.php';

use App\Service;

function boot() {}
`)
	writeFile(t, root, "src/Service.php", `<?php
namespace App;

interface Contract {}
trait Logger {}
class Service {
    public function run() {}
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "App", "namespace", "src/Service.php")
	assertHasSymbol(t, symbols, "Contract", "interface", "src/Service.php")
	assertHasSymbol(t, symbols, "Logger", "trait", "src/Service.php")
	assertHasSymbol(t, symbols, "Service", "class", "src/Service.php")
	assertHasSymbol(t, symbols, "run", "method", "src/Service.php")
	assertHasSymbol(t, symbols, "boot", "function", "public/index.php")

	var relations []RelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations.json"), &relations)
	assertHasRelation(t, relations, "public/index.php", "src/Service.php", "imports")
	assertHasRelation(t, relations, "public/index.php", "src/Service.php", "includes")

	entrypoints := readText(t, filepath.Join(root, "goregraph-out", "entrypoints.md"))
	if !strings.Contains(entrypoints, "public/index.php") {
		t.Fatalf("entrypoints report missing PHP public index:\n%s", entrypoints)
	}
}

func TestRunExtractsShellSymbolsRelationsAndEntrypoint(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "scripts/deploy.sh", `#!/usr/bin/env bash
source ./lib.sh

deploy() {
  echo deploy
}

function rollback() {
  echo rollback
}
`)
	writeFile(t, root, "scripts/lib.sh", "helper() { echo helper; }\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "deploy", "function", "scripts/deploy.sh")
	assertHasSymbol(t, symbols, "rollback", "function", "scripts/deploy.sh")

	var relations []RelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations.json"), &relations)
	assertHasRelation(t, relations, "scripts/deploy.sh", "scripts/lib.sh", "sources")

	entrypoints := readText(t, filepath.Join(root, "goregraph-out", "entrypoints.md"))
	if !strings.Contains(entrypoints, "scripts/deploy.sh") || !strings.Contains(entrypoints, "shell script") {
		t.Fatalf("entrypoints report missing shell script:\n%s", entrypoints)
	}
}

func TestRunProducesDeterministicManifestGoldenOutput(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var manifest Manifest
	readJSON(t, filepath.Join(root, "goregraph-out", "manifest.json"), &manifest)
	if manifest.Tool != "goregraph" {
		t.Fatalf("Tool = %q, want goregraph", manifest.Tool)
	}
	if manifest.Schema != 1 {
		t.Fatalf("Schema = %d, want 1", manifest.Schema)
	}
	if manifest.OutputDir != "goregraph-out" {
		t.Fatalf("OutputDir = %q, want goregraph-out", manifest.OutputDir)
	}
	if manifest.Files != 1 || manifest.Skipped != 0 {
		t.Fatalf("manifest counts = files %d skipped %d, want 1/0", manifest.Files, manifest.Skipped)
	}
	if manifest.ProjectRoot != filepath.Base(root) {
		t.Fatalf("ProjectRoot = %q, want %q", manifest.ProjectRoot, filepath.Base(root))
	}
	if manifest.GeneratedAt == "" {
		t.Fatal("GeneratedAt is empty")
	}
	for _, name := range GeneratedFiles {
		if !containsString(manifest.Generated, name) {
			t.Fatalf("Generated missing %q in %#v", name, manifest.Generated)
		}
	}
}

func TestRunUsesIncludePatternsFromConfig(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.Include = []string{"src/**"}
	writeFile(t, root, "README.md", "# Demo\n")
	writeFile(t, root, "src/app.go", "package src\n")
	writeFile(t, root, "tools/tool.go", "package tools\n")

	result, err := Run(root, cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 1 {
		t.Fatalf("ScannedFiles = %d, want 1", result.ScannedFiles)
	}
	var files []FileRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "files.json"), &files)
	if len(files) != 1 || files[0].Path != "src/app.go" {
		t.Fatalf("files = %#v, want only src/app.go", files)
	}
}

func TestRunWritesProjectIntelligenceReports(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/demo\n")
	writeFile(t, root, "cmd/api/main.go", "package main\nfunc main() {}\n")
	writeFile(t, root, "internal/service/service.go", "package service\nfunc StartServer() {}\n")
	writeFile(t, root, "internal/service/service_test.go", "package service\nfunc TestStartServer() {}\n")
	writeFile(t, root, "package.json", `{"scripts":{"dev":"vite --host 0.0.0.0","test":"vitest"}}`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	modules := readText(t, filepath.Join(root, "goregraph-out", "modules.md"))
	if !strings.Contains(modules, "`cmd/`") || !strings.Contains(modules, "`internal/`") {
		t.Fatalf("modules report missing top-level modules:\n%s", modules)
	}

	entrypoints := readText(t, filepath.Join(root, "goregraph-out", "entrypoints.md"))
	if !strings.Contains(entrypoints, "cmd/api/main.go") {
		t.Fatalf("entrypoints report missing Go main file:\n%s", entrypoints)
	}
	if !strings.Contains(entrypoints, "package.json") || !strings.Contains(entrypoints, "dev") {
		t.Fatalf("entrypoints report missing package scripts:\n%s", entrypoints)
	}

	testMap := readText(t, filepath.Join(root, "goregraph-out", "test-map.md"))
	if !strings.Contains(testMap, "internal/service/service_test.go") || !strings.Contains(testMap, "internal/service/service.go") {
		t.Fatalf("test map missing source/test association:\n%s", testMap)
	}
}

func TestRunDetectsTestSymbolsAndRelations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "internal/service/service.go", "package service\nfunc StartServer() {}\n")
	writeFile(t, root, "internal/service/service_test.go", "package service\nfunc TestStartServer() {}\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "service", "package", "internal/service/service.go")
	assertHasSymbol(t, symbols, "TestStartServer", "test", "internal/service/service_test.go")

	var relations []RelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations.json"), &relations)
	assertHasRelation(t, relations, "internal/service/service_test.go", "internal/service/service.go", "tests")
}

func TestRunUsesProjectGitignoreAsExclusions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "local/\n*.tmp\n")
	writeFile(t, root, "src/app.go", "package src\n")
	writeFile(t, root, "local/cache.go", "package local\n")
	writeFile(t, root, "scratch.tmp", "tmp")

	result, err := Run(root, config.Defaults())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 1 {
		t.Fatalf("ScannedFiles = %d, want 1", result.ScannedFiles)
	}
}

func TestRunExtractsUniversalLanguageIntelligence(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/universal\n")
	writeFile(t, root, "cmd/server/main.go", `package main

import "net/http"

func main() {
	http.HandleFunc("/health", healthHandler)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	checkHealth()
}

func checkHealth() {}
`)
	writeFile(t, root, "cmd/server/main_test.go", `package main

func TestHealthHandler(t *testing.T) {
	healthHandler(nil, nil)
}
`)
	writeFile(t, root, "routes/web.php", `<?php
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\UserController;

Route::get('/users', [UserController::class, 'index']);
`)
	writeFile(t, root, "app/Http/Controllers/UserController.php", `<?php
namespace App\Http\Controllers;

class UserController {
    public function index() {
        return $this->service->listUsers();
    }
}
`)
	writeFile(t, root, "tests/UserControllerTest.php", `<?php
class UserControllerTest {
    public function testIndex() {
        $controller = new UserController();
        $controller->index();
    }
}
`)
	writeFile(t, root, "src/App.tsx", `import { Route } from "react-router-dom";
import { loadUsers } from "./api";

export function App() {
  return <Route path="/users" element={<UsersPage />} />;
}

export function UsersPage() {
  loadUsers();
  return null;
}
`)
	writeFile(t, root, "src/Router.jsx", `import { Fragment } from "@weka/redux-little-router";
import { Route } from "react-router-dom";

export function Router() {
  return <>
    // <Fragment forRoute="/commented" />
    <Route exact path="/search" component={SearchContainer} />
    <Fragment forRoute="/kataster/:id" />
  </>;
}
`)
	writeFile(t, root, "src/api.ts", `export function loadUsers() {
  return fetch("/api/users");
}
`)
	writeFile(t, root, "src/server.ts", `import express from "express";
const app = express();

app.get("/api/users", listUsers);

export function listUsers(req, res) {
  loadUsers();
}
`)
	writeFile(t, root, "src/App.test.tsx", `import { UsersPage } from "./App";

test("users page loads users", () => {
  UsersPage();
});
`)
	writeFile(t, root, "app/main.py", `from fastapi import FastAPI

app = FastAPI()

@app.get("/status")
def status():
    return compute_status()

def compute_status():
    return {"ok": True}
`)
	writeFile(t, root, "tests/test_main.py", `from app.main import status

def test_status():
    status()
`)
	writeFile(t, root, "scripts/deploy.sh", `#!/usr/bin/env bash
source ./lib.sh

deploy() {
  build_image
}
`)
	writeFile(t, root, "scripts/lib.sh", `build_image() {
  echo build
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	for _, name := range []string{"routes.json", "routes.md", "flows.json", "flows.md", "navigation.md"} {
		if _, err := os.Stat(filepath.Join(root, "goregraph-out", name)); err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
	}

	var routes []map[string]any
	readJSON(t, filepath.Join(root, "goregraph-out", "routes.json"), &routes)
	assertHasRoute(t, routes, "go", "GET", "/health", "healthHandler")
	assertHasRoute(t, routes, "php", "GET", "/users", "UserController.index")
	assertHasRoute(t, routes, "typescript", "ROUTE", "/users", "UsersPage")
	assertHasRoute(t, routes, "javascript", "ROUTE", "/search", "SearchContainer")
	assertHasRoute(t, routes, "javascript", "ROUTE", "/kataster/:id", "Fragment")
	assertNoRoute(t, routes, "/commented")
	assertHasRoute(t, routes, "typescript", "GET", "/api/users", "listUsers")
	assertHasRoute(t, routes, "python", "GET", "/status", "status")

	var callGraph CallGraphRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "callgraph.json"), &callGraph)
	assertHasAnyCallGraphEdge(t, callGraph, "healthHandler", "checkHealth")
	assertHasAnyCallGraphEdge(t, callGraph, "UsersPage", "loadUsers")
	assertHasAnyCallGraphEdge(t, callGraph, "status", "compute_status")
	assertHasAnyCallGraphEdge(t, callGraph, "deploy", "build_image")

	var testMap []TestMapRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "test-map.json"), &testMap)
	assertHasTestMapTarget(t, testMap, "TestHealthHandler", "healthHandler")
	assertHasTestMapTarget(t, testMap, "test_status", "status")
	assertHasTestMapTarget(t, testMap, "users page loads users", "UsersPage")

	routesReport := readText(t, filepath.Join(root, "goregraph-out", "routes.md"))
	if !strings.Contains(routesReport, "React Router") || !strings.Contains(routesReport, "FastAPI") || !strings.Contains(routesReport, "Laravel") {
		t.Fatalf("routes report missing framework context:\n%s", routesReport)
	}

	flowsReport := readText(t, filepath.Join(root, "goregraph-out", "flows.md"))
	if !strings.Contains(flowsReport, "UsersPage") || !strings.Contains(flowsReport, "loadUsers") || !strings.Contains(flowsReport, "healthHandler") {
		t.Fatalf("flows report missing useful flow steps:\n%s", flowsReport)
	}

	navigationReport := readText(t, filepath.Join(root, "goregraph-out", "navigation.md"))
	if !strings.Contains(navigationReport, "Where To Start") || !strings.Contains(navigationReport, "Most Connected Files") || !strings.Contains(navigationReport, "src/App.tsx") {
		t.Fatalf("navigation report missing orientation content:\n%s", navigationReport)
	}
}

func TestRunHardensFrontendMonorepoIntelligence(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"name":"workspace-root","private":true,"workspaces":["apps/*","packages/*"]}`)
	writeFile(t, root, "apps/portal/package.json", `{"name":"@demo/portal","dependencies":{"@demo/ui":"workspace:*","react":"18.0.0"}}`)
	writeFile(t, root, "packages/ui/package.json", `{"name":"@demo/ui","dependencies":{"@demo/shared":"workspace:*"}}`)
	writeFile(t, root, "packages/shared/package.json", `{"name":"@demo/shared"}`)
	writeFile(t, root, "apps/portal/src/components/routes/router.jsx", `import { Route } from "react-router-dom";
import { Home } from "../pages/Home";
import { TasksPage } from "../pages/TasksPage";

export function Router() {
  return <>
    <Route exact path="/" component={Home} />
    <Route path="/tasks" render={() => <TasksPage />} />
  </>;
}
`)
	writeFile(t, root, "apps/portal/src/Root.tsx", `import { Fragment } from "@weka/redux-little-router";
import ConnectedEdit from "./containers/Edit";

export function Root() {
  return <Fragment forRoute="/editieren"><ConnectedEdit /></Fragment>;
}
`)
	writeFile(t, root, "apps/portal/src/pages/Home.jsx", `import { GetHelper } from "../utils/requestHelper";
import { Button } from "@demo/ui";

export function Home() {
  return Button();
}

export async function loadHome() {
  return GetHelper("/api/home");
}
`)
	writeFile(t, root, "apps/portal/src/pages/TasksPage.tsx", `import { PostHelper } from "../utils/requestHelper";

export function TasksPage() {
  return PostHelper("/api/tasks/export");
}
`)
	writeFile(t, root, "apps/portal/src/containers/Edit.jsx", `export default function ConnectedEdit() {
  return null;
}
`)
	writeFile(t, root, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(path) {
  return fetch(path);
}

export function PostHelper(path) {
  return fetch(path, { method: "POST" });
}
`)
	writeFile(t, root, "packages/redux-little-router/index.d.ts", `export function push(path: string): void;
export function block(path: string): void;
`)
	writeFile(t, root, ".storybook.old/archive.jsx", `export function ArchivedStory() {
  return <Route path="/archive" component={Archive} />;
}
`)
	writeFile(t, root, "apps/portal/src/pages/Home.test.jsx", `import { Home } from "./Home";

test("renders home", () => {
  const wrapper = shallow(<Home />);
  wrapper.find("button").text();
});
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var routes []CodeRouteRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "routes.json"), &routes)
	assertHasRouteID(t, routes, "portal:/", "Home")
	assertHasRouteID(t, routes, "portal:/tasks", "TasksPage")
	assertHasRouteID(t, routes, "portal:/editieren", "ConnectedEdit")
	assertNoRouteID(t, routes, "workspace-root:/archive")

	var graph CallGraphRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "callgraph.json"), &graph)
	assertNoAnyCallGraphTarget(t, graph, "find")
	assertNoAnyCallGraphTarget(t, graph, "text")
	assertNoAnyCallGraphTarget(t, graph, "push")
	assertNoAnyCallGraphTarget(t, graph, "block")

	var packages PackageGraphRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "package-graph.json"), &packages)
	assertHasPackageEdge(t, packages, "@demo/portal", "@demo/ui")
	assertHasPackageEdge(t, packages, "@demo/ui", "@demo/shared")

	var api []APIContractRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "api-contracts.json"), &api)
	assertHasAPIContract(t, api, "GET", "/api/home", "apps/portal/src/pages/Home.jsx")
	assertHasAPIContract(t, api, "POST", "/api/tasks/export", "apps/portal/src/pages/TasksPage.tsx")

	routesReport := readText(t, filepath.Join(root, "goregraph-out", "routes.md"))
	if !strings.Contains(routesReport, "portal:/editieren") || !strings.Contains(routesReport, "ConnectedEdit") {
		t.Fatalf("routes report missing app-specific rendered component:\n%s", routesReport)
	}

	packageReport := readText(t, filepath.Join(root, "goregraph-out", "package-graph.md"))
	if !strings.Contains(packageReport, "@demo/portal") || !strings.Contains(packageReport, "@demo/ui") {
		t.Fatalf("package graph report missing workspace dependency:\n%s", packageReport)
	}

	apiReport := readText(t, filepath.Join(root, "goregraph-out", "api-contracts.md"))
	if !strings.Contains(apiReport, "GET `/api/home`") || !strings.Contains(apiReport, "POST `/api/tasks/export`") {
		t.Fatalf("api contract report missing helper calls:\n%s", apiReport)
	}
}

func TestRunExtractsRealisticFrontendAPIContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "apps/portal/package.json", `{"name":"@demo/portal"}`)
	writeFile(t, root, "apps/portal/src/api/productsservice.js", "import { GetHelper, PostHelper, GetHelperWithStatus } from \"../utils/requestHelper\";\n\n"+
		"export async function fetchProducts(dispatch, userId) {\n"+
		"  return await GetHelper(dispatch, `/productservice/users/${userId}/products`);\n"+
		"}\n\n"+
		"export async function inviteUser(dispatch, cadasterId, body) {\n"+
		"  const { status } = await PostHelper(\n"+
		"    dispatch,\n"+
		"    `/cadasters/${cadasterId}/users`,\n"+
		"    JSON.stringify(body)\n"+
		"  );\n"+
		"  return status;\n"+
		"}\n\n"+
		"export async function flyout(dispatch) {\n"+
		"  return GetHelperWithStatus(dispatch, '/portal/tasks/flyout');\n"+
		"}\n\n"+
		"export async function dynamicFetch(url) {\n"+
		"  return fetch(url, { method: 'POST' });\n"+
		"}\n")
	writeFile(t, root, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(dispatch, path) { return fetch(path); }
export function GetHelperWithStatus(dispatch, path) { return fetch(path); }
export function PostHelper(dispatch, path) { return fetch(path, { method: "POST" }); }
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var api []APIContractRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "api-contracts.json"), &api)
	assertHasAPIContract(t, api, "GET", "/productservice/users/{userId}/products", "apps/portal/src/api/productsservice.js")
	assertHasAPIContract(t, api, "POST", "/cadasters/{cadasterId}/users", "apps/portal/src/api/productsservice.js")
	assertHasAPIContract(t, api, "GET", "/portal/tasks/flyout", "apps/portal/src/api/productsservice.js")
	assertNoAPIContract(t, api, "POST", "/url")

	apiReport := readText(t, filepath.Join(root, "goregraph-out", "api-contracts.md"))
	if !strings.Contains(apiReport, "GET `/productservice/users/{userId}/products`") || !strings.Contains(apiReport, "POST `/cadasters/{cadasterId}/users`") {
		t.Fatalf("api contract report missing realistic helper calls:\n%s", apiReport)
	}
}

func TestRunKeepsFrontendRouteHandlersInsideOwningApp(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "apps/mein-konto/src/pages/home/home.jsx", `export function Home() {
  return null;
}
`)
	writeFile(t, root, "apps/portal/src/pages/home/home.jsx", `export function Home() {
  return loadPortalHome();
}

export function loadPortalHome() {
  return null;
}
`)
	writeFile(t, root, "apps/portal/src/routes.jsx", `import { Route } from "react-router-dom";
import { Home } from "./pages/home/home";

export function Routes() {
  return <Route path="/" component={Home} />;
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var flows []CodeFlowRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "flows.json"), &flows)
	for _, flow := range flows {
		if flow.RouteID != "portal:/" {
			continue
		}
		if len(flow.Steps) == 0 || flow.Steps[0].File != "apps/portal/src/pages/home/home.jsx" {
			t.Fatalf("portal route resolved to wrong handler step: %#v", flow)
		}
		return
	}
	t.Fatalf("missing portal route flow in %#v", flows)
}

func TestRunGeneratesMavenDependencyGraph(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pom.xml", `<project>
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
</project>`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var graph MavenGraphRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "maven-graph.json"), &graph)
	assertHasMavenEdge(t, graph, "com.example:service-a", "org.springframework.boot:spring-boot-starter-web")

	report := readText(t, filepath.Join(root, "goregraph-out", "maven-graph.md"))
	if !strings.Contains(report, "com.example:service-a") || !strings.Contains(report, "org.springframework.boot:spring-boot-starter-web") {
		t.Fatalf("maven graph report missing dependency:\n%s", report)
	}
}

func TestRunMatchesFrontendAPIContractsToBackendRoutes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "apps/portal/src/api/cadasterservice.js", "import { GetHelper, PostHelper, DeleteHelper } from '../utils/requestHelper';\n\n"+
		"export function loadCadaster(dispatch, id) {\n"+
		"  return GetHelper(dispatch, `/cadasters/${id}`);\n"+
		"}\n\n"+
		"export function createCadaster(dispatch, id) {\n"+
		"  return PostHelper(dispatch, `/cadasters/${id}`);\n"+
		"}\n\n"+
		"export function deleteCadaster(dispatch, id) {\n"+
		"  return DeleteHelper(dispatch, `/cadasters/${id}`);\n"+
		"}\n\n"+
		"export function dynamicCadaster(dispatch, stateName, id) {\n"+
		"  return GetHelper(dispatch, `/cadasters/${stateName ? 'draft' : 'active'}/${id}`);\n"+
		"}\n")
	writeFile(t, root, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(dispatch, path) { return fetch(path); }
export function PostHelper(dispatch, path) { return fetch(path, { method: "POST" }); }
export function DeleteHelper(dispatch, path) { return fetch(path, { method: "DELETE" }); }
`)
	writeFile(t, root, "src/main/java/com/example/CadasterController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @GetMapping("/{cadasterId}")
  String get(@PathVariable String cadasterId) {
    return cadasterId;
  }
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var api []APIContractRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "api-contracts.json"), &api)
	assertHasAPIContract(t, api, "GET", "/cadasters/{id}", "apps/portal/src/api/cadasterservice.js")
	assertHasUnsafeAPIContract(t, api, "GET", "/cadasters/{dynamic}/{id}")

	var matches []ContractMatchRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "contract-matches.json"), &matches)
	assertHasContractMatch(t, matches, "GET", "/cadasters/{id}", "GET", "/cadasters/{cadasterId}", "RESOLVED")
	assertHasContractIssue(t, matches, "POST", "/cadasters/{id}", "method_mismatch")
	assertHasContractIssue(t, matches, "DELETE", "/cadasters/{id}", "method_mismatch")
	assertHasContractIssue(t, matches, "GET", "/cadasters/{dynamic}/{id}", "unsafe_dynamic")

	report := readText(t, filepath.Join(root, "goregraph-out", "contract-matches.md"))
	if !strings.Contains(report, "GET `/cadasters/{id}` -> GET `/cadasters/{cadasterId}`") {
		t.Fatalf("contract match report missing resolved match:\n%s", report)
	}
	broken := readText(t, filepath.Join(root, "goregraph-out", "potentially-broken-contracts.md"))
	if !strings.Contains(broken, "method_mismatch") || !strings.Contains(broken, "unsafe_dynamic") {
		t.Fatalf("broken contract report missing issues:\n%s", broken)
	}
}

func TestRunClassifiesContractsForUnscannedServices(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "apps/portal/src/api/services.js", "import { GetHelper } from '../utils/requestHelper';\n\n"+
		"export function loadCadaster(dispatch, id) {\n"+
		"  return GetHelper(dispatch, `/cadasters/${id}`);\n"+
		"}\n\n"+
		"export function loadTask(dispatch, id) {\n"+
		"  return GetHelper(dispatch, `/tasks/${id}`);\n"+
		"}\n\n"+
		"export function loadMissingCadasterRoute(dispatch) {\n"+
		"  return GetHelper(dispatch, '/cadasters/missing/detail');\n"+
		"}\n")
	writeFile(t, root, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(dispatch, path) { return fetch(path); }
`)
	writeFile(t, root, "src/main/java/com/example/CadasterController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @GetMapping("/{cadasterId}")
  String get(@PathVariable String cadasterId) {
    return cadasterId;
  }
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var matches []ContractMatchRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "contract-matches.json"), &matches)
	assertHasContractIssue(t, matches, "GET", "/tasks/{id}", "unscanned_service")
	assertHasContractIssue(t, matches, "GET", "/cadasters/missing/detail", "missing_backend_route")

	report := readText(t, filepath.Join(root, "goregraph-out", "contract-matches.md"))
	if !strings.Contains(report, "unscanned_service") || !strings.Contains(report, "ms-task was not scanned") {
		t.Fatalf("contract match report missing unscanned service context:\n%s", report)
	}
}

func TestRunWritesDiagnosticsReports(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "apps/portal/src/routes.jsx", `import { Route } from "react-router-dom";
import { Home } from "./Home";

export function Routes() {
  return <Route path="/" component={Home} />;
}
`)
	writeFile(t, root, "apps/portal/src/Home.jsx", "import { GetHelper } from \"./utils/requestHelper\";\n\n"+
		"export function Home() {\n"+
		"  return loadTask();\n"+
		"}\n\n"+
		"export function loadTask(dispatch, id) {\n"+
		"  return GetHelper(dispatch, `/tasks/${id}`);\n"+
		"}\n")
	writeFile(t, root, "apps/portal/src/Home.test.jsx", `import { Home } from "./Home";

test("renders home", () => {
  Home();
});
`)
	writeFile(t, root, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(dispatch, path) { return fetch(path); }
`)
	writeFile(t, root, "src/main/java/com/example/CadasterController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @GetMapping("/{cadasterId}")
  String get() {
    return "";
  }
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var diagnostics map[string]any
	readJSON(t, filepath.Join(root, "goregraph-out", "diagnostics.json"), &diagnostics)
	for _, key := range []string{"entrypoints", "risky_contracts", "unscanned_services", "endpoints_without_tests", "weak_flows", "likely_tests"} {
		if _, ok := diagnostics[key]; !ok {
			t.Fatalf("diagnostics.json missing key %q in %#v", key, diagnostics)
		}
	}

	report := readText(t, filepath.Join(root, "goregraph-out", "diagnostics.md"))
	for _, want := range []string{"# GoreGraph Diagnostics", "Top Entry Points", "Risky Contracts", "Unscanned Services", "Endpoints Without Tests", "Weak Flows", "Likely Tests"} {
		if !strings.Contains(report, want) {
			t.Fatalf("diagnostics report missing %q:\n%s", want, report)
		}
	}
	if !strings.Contains(report, "ms-task") || !strings.Contains(report, "GET `/cadasters/{cadasterId}`") {
		t.Fatalf("diagnostics report missing service and endpoint context:\n%s", report)
	}
}

func TestRunAffectedReportPrioritizesLocalDiagnosisOverExternalImports(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/affected\n")
	writeFile(t, root, "cmd/app/main.go", `package main

import "example.test/affected/internal/home"

func main() {
	home.Render()
}
`)
	writeFile(t, root, "internal/home/home.go", `package home

func Render() {}
`)
	writeFile(t, root, "package.json", `{"name":"portal"}`)
	for _, rel := range []string{"src/App.jsx", "src/Home.jsx", "src/Search.jsx"} {
		writeFile(t, root, rel, `import React from "react";
import { Button } from "@weka/designsystem";

export function Component() {
  return React.createElement(Button);
}
`)
	}
	writeFile(t, root, "src/routes.jsx", `import { Route } from "react-router-dom";
import { Component } from "./Home";

export function Routes() {
  return <Route path="/" component={Component} />;
}
`)
	writeFile(t, root, "src/Home.test.jsx", `import { Component } from "./Home";

test("renders", () => {
  Component();
});
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	affected := readText(t, filepath.Join(root, "goregraph-out", "affected.md"))
	if strings.Contains(affected, "`react`") || strings.Contains(affected, "`@weka/designsystem`") {
		t.Fatalf("affected report should not prioritize external imports:\n%s", affected)
	}
	if !strings.Contains(affected, "internal/home/home.go") {
		t.Fatalf("affected report missing local diagnosis target:\n%s", affected)
	}
}

func assertHasSymbol(t *testing.T, symbols []SymbolRecord, name, kind, file string) {
	t.Helper()
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind && symbol.File == file {
			return
		}
	}
	t.Fatalf("missing symbol name=%q kind=%q file=%q in %#v", name, kind, file, symbols)
}

func assertHasRelation(t *testing.T, relations []RelationRecord, from, to, kind string) {
	t.Helper()
	for _, relation := range relations {
		if relation.From == from && relation.To == to && relation.Type == kind {
			return
		}
	}
	t.Fatalf("missing relation from=%q to=%q type=%q in %#v", from, to, kind, relations)
}

func assertHasGraphNode(t *testing.T, graph Graph, id string) {
	t.Helper()
	for _, node := range graph.Nodes {
		if node.ID == id {
			return
		}
	}
	t.Fatalf("missing graph node id=%q in %#v", id, graph.Nodes)
}

func assertHasGraphEdge(t *testing.T, graph Graph, from, to, kind string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.From == from && edge.To == to && edge.Type == kind {
			return
		}
	}
	t.Fatalf("missing graph edge from=%q to=%q type=%q in %#v", from, to, kind, graph.Edges)
}

func assertHasRoute(t *testing.T, routes []map[string]any, language, method, path, handler string) {
	t.Helper()
	for _, route := range routes {
		if route["language"] == language && route["http_method"] == method && route["path"] == path && route["handler"] == handler {
			return
		}
	}
	t.Fatalf("missing route language=%q method=%q path=%q handler=%q in %#v", language, method, path, handler, routes)
}

func assertNoRoute(t *testing.T, routes []map[string]any, path string) {
	t.Helper()
	for _, route := range routes {
		if route["path"] == path {
			t.Fatalf("unexpected route path=%q in %#v", path, routes)
		}
	}
}

func assertHasRouteID(t *testing.T, routes []CodeRouteRecord, routeID, handler string) {
	t.Helper()
	for _, route := range routes {
		if route.RouteID == routeID && (route.Handler == handler || containsString(route.RenderedComponents, handler)) {
			return
		}
	}
	t.Fatalf("missing route id=%q handler=%q in %#v", routeID, handler, routes)
}

func assertNoRouteID(t *testing.T, routes []CodeRouteRecord, routeID string) {
	t.Helper()
	for _, route := range routes {
		if route.RouteID == routeID {
			t.Fatalf("unexpected route id=%q in %#v", routeID, routes)
		}
	}
}

func assertHasAnyCallGraphEdge(t *testing.T, graph CallGraphRecord, fromMethod, toMethod string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.From.Method == fromMethod && edge.To.Method == toMethod {
			return
		}
	}
	t.Fatalf("missing callgraph edge %q -> %q in %#v", fromMethod, toMethod, graph.Edges)
}

func assertNoAnyCallGraphTarget(t *testing.T, graph CallGraphRecord, target string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.To.Method == target {
			t.Fatalf("unexpected callgraph target %q in %#v", target, graph.Edges)
		}
	}
}

func assertHasPackageEdge(t *testing.T, graph PackageGraphRecord, from, to string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.From == from && edge.To == to {
			return
		}
	}
	t.Fatalf("missing package edge %q -> %q in %#v", from, to, graph.Edges)
}

func assertHasAPIContract(t *testing.T, records []APIContractRecord, method, path, file string) {
	t.Helper()
	for _, record := range records {
		if record.HTTPMethod == method && record.Path == path && record.File == file {
			return
		}
	}
	t.Fatalf("missing api contract method=%q path=%q file=%q in %#v", method, path, file, records)
}

func assertHasUnsafeAPIContract(t *testing.T, records []APIContractRecord, method, path string) {
	t.Helper()
	for _, record := range records {
		if record.HTTPMethod == method && record.Path == path && record.UnsafeDynamic {
			return
		}
	}
	t.Fatalf("missing unsafe api contract method=%q path=%q in %#v", method, path, records)
}

func assertNoAPIContract(t *testing.T, records []APIContractRecord, method, path string) {
	t.Helper()
	for _, record := range records {
		if record.HTTPMethod == method && record.Path == path {
			t.Fatalf("unexpected api contract method=%q path=%q in %#v", method, path, records)
		}
	}
}

func assertHasContractMatch(t *testing.T, records []ContractMatchRecord, apiMethod, apiPath, backendMethod, backendPath, confidence string) {
	t.Helper()
	for _, record := range records {
		if record.APIHTTPMethod == apiMethod && record.APIPath == apiPath && record.BackendHTTPMethod == backendMethod && record.BackendPath == backendPath && record.Confidence == confidence {
			return
		}
	}
	t.Fatalf("missing contract match api=%s %q backend=%s %q confidence=%q in %#v", apiMethod, apiPath, backendMethod, backendPath, confidence, records)
}

func assertHasContractIssue(t *testing.T, records []ContractMatchRecord, apiMethod, apiPath, issue string) {
	t.Helper()
	for _, record := range records {
		if record.APIHTTPMethod == apiMethod && record.APIPath == apiPath && record.Issue == issue {
			return
		}
	}
	t.Fatalf("missing contract issue api=%s %q issue=%q in %#v", apiMethod, apiPath, issue, records)
}

func assertHasMavenEdge(t *testing.T, graph MavenGraphRecord, from, to string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.From == from && edge.To == to {
			return
		}
	}
	t.Fatalf("missing maven edge %q -> %q in %#v", from, to, graph.Edges)
}

func assertHasTestMapTarget(t *testing.T, records []TestMapRecord, testName, targetName string) {
	t.Helper()
	for _, record := range records {
		if record.TestMethod == testName && record.TargetMethod == targetName {
			return
		}
	}
	t.Fatalf("missing test map test=%q target=%q in %#v", testName, targetName, records)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestRunSkipsLargeBinaryAndSymlinkFiles(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.MaxFileSizeBytes = 4
	writeFile(t, root, "src/app.go", "pkg")
	writeFile(t, root, "big.txt", "12345")
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "src/app.go"), filepath.Join(root, "link.go")); err != nil {
		t.Skipf("symlink creation is not permitted in this environment: %v", err)
	}

	result, err := Run(root, cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 1 {
		t.Fatalf("ScannedFiles = %d, want 1", result.ScannedFiles)
	}
	if result.SkippedFiles != 3 {
		t.Fatalf("SkippedFiles = %d, want 3", result.SkippedFiles)
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string, dest any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatal(err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
