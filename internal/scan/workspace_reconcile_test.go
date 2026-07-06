package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestScanCreatesWorkspaceRegistryWithIndexedAndNotIndexedProjects(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	task := filepath.Join(workspace, "microservices", "ms-task")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "src/api/cadasterservice.js", "export function loadCadaster(id) {\n"+
		"  return fetch(`/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, cadaster, "README.md", "# ms-cadaster\n")
	writeFile(t, task, "README.md", "# ms-task\n")

	if _, err := Run(frontend, config.Defaults()); err != nil {
		t.Fatalf("Run frontend returned error: %v", err)
	}

	var registry WorkspaceRegistryRecord
	readJSON(t, filepath.Join(workspace, ".goregraph-workspace", "registry.json"), &registry)
	assertWorkspaceProject(t, registry, "frontend/frontend-monorepo", "current", true)
	assertWorkspaceProject(t, registry, "microservices/ms-cadaster", "not_indexed", false)
	assertWorkspaceProject(t, registry, "microservices/ms-task", "not_indexed", false)

	context := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-context.md"))
	if !strings.Contains(context, "frontend/frontend-monorepo") || !strings.Contains(context, "microservices/ms-cadaster") {
		t.Fatalf("workspace context missing discovered projects:\n%s", context)
	}
}

func TestLaterBackendScanRefreshesExistingFrontendWorkspaceOverlay(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "src/api/cadasterservice.js", "export function loadCadaster(id) {\n"+
		"  return fetch(`/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, cadaster, "src/main/java/com/example/CadasterController.java", `package com.example;

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

	if _, err := Run(frontend, config.Defaults()); err != nil {
		t.Fatalf("Run frontend returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("Run cadaster returned error: %v", err)
	}

	frontendMatches := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-contract-matches.md"))
	if !strings.Contains(frontendMatches, "GET `/cadasters/{id}` -> ms-cadaster GET `/cadasters/{cadasterId}`") {
		t.Fatalf("frontend workspace contract overlay missing backend match:\n%s", frontendMatches)
	}

	frontendContext := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-context.md"))
	if !strings.Contains(frontendContext, "This project: `frontend/frontend-monorepo`") || !strings.Contains(frontendContext, "Last refreshed by: `microservices/ms-cadaster`") {
		t.Fatalf("frontend workspace context has misleading project labels:\n%s", frontendContext)
	}

	frontendDiagnostics := readText(t, filepath.Join(frontend, "goregraph-out", "diagnostics.md"))
	if !strings.Contains(frontendDiagnostics, "## Workspace Resolved Contracts") || !strings.Contains(frontendDiagnostics, "ms-cadaster GET `/cadasters/{cadasterId}`") {
		t.Fatalf("frontend diagnostics missing workspace resolved contract:\n%s", frontendDiagnostics)
	}
	if strings.Contains(frontendDiagnostics, "`ms-cadaster` -") {
		t.Fatalf("frontend diagnostics still reports resolved ms-cadaster as unscanned:\n%s", frontendDiagnostics)
	}

	var diagnostics DiagnosticsRecord
	readJSON(t, filepath.Join(frontend, "goregraph-out", "diagnostics.json"), &diagnostics)
	if len(diagnostics.WorkspaceResolvedContracts) == 0 {
		t.Fatalf("diagnostics json missing workspace resolved contracts: %#v", diagnostics)
	}

	backendDiagnostics := readText(t, filepath.Join(cadaster, "goregraph-out", "diagnostics.md"))
	if !strings.Contains(backendDiagnostics, "## Workspace Resolved Contracts") ||
		!strings.Contains(backendDiagnostics, "frontend/frontend-monorepo") ||
		!strings.Contains(backendDiagnostics, "src/api/cadasterservice.js:2") {
		t.Fatalf("backend diagnostics missing incoming workspace contract:\n%s", backendDiagnostics)
	}

	var manifest Manifest
	readJSON(t, filepath.Join(frontend, "goregraph-out", "manifest.json"), &manifest)
	assertGeneratedFile(t, manifest.Generated, "workspace-context.md")
	assertGeneratedFile(t, manifest.Generated, "workspace-contract-matches.md")
	assertGeneratedFile(t, manifest.Generated, "frontend-consumers.md")

	var audit AuditRecord
	readJSON(t, filepath.Join(frontend, "goregraph-out", "audit.json"), &audit)
	assertGeneratedFile(t, audit.Generated, "workspace-context.md")
	assertGeneratedFile(t, audit.Generated, "workspace-contract-matches.md")
	assertGeneratedFile(t, audit.Generated, "frontend-consumers.md")

	consumers := readText(t, filepath.Join(cadaster, "goregraph-out", "frontend-consumers.md"))
	if !strings.Contains(consumers, "frontend/frontend-monorepo") || !strings.Contains(consumers, "src/api/cadasterservice.js") {
		t.Fatalf("backend frontend-consumers overlay missing frontend usage:\n%s", consumers)
	}

	frontendConsumers := readText(t, filepath.Join(frontend, "goregraph-out", "frontend-consumers.md"))
	if !strings.Contains(frontendConsumers, "not applicable for frontend projects") ||
		!strings.Contains(frontendConsumers, "workspace-feature-flows.md") {
		t.Fatalf("frontend frontend-consumers overlay should explain report scope:\n%s", frontendConsumers)
	}

	featureReport := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	if !strings.Contains(featureReport, "no endpoint or backend-step tests matched backend endpoint GET `/cadasters/{cadasterId}`") ||
		!strings.Contains(featureReport, "Endpoints Without Tests") {
		t.Fatalf("feature flow report missing actionable test gap:\n%s", featureReport)
	}

	endpoints := readText(t, filepath.Join(cadaster, "goregraph-out", "endpoints.md"))
	if !strings.Contains(endpoints, "## Frontend Consumers") || !strings.Contains(endpoints, "frontend/frontend-monorepo") {
		t.Fatalf("backend endpoints report missing frontend consumers:\n%s", endpoints)
	}

	var registry WorkspaceRegistryRecord
	readJSON(t, filepath.Join(workspace, ".goregraph-workspace", "registry.json"), &registry)
	assertWorkspaceProject(t, registry, "frontend/frontend-monorepo", "indexed", true)
	assertWorkspaceProject(t, registry, "microservices/ms-cadaster", "current", true)
}

func TestWorkspacePathMatchingDoesNotTreatStaticSegmentsAsRouteParams(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-cadaster", Kind: "backend", Service: "ms-cadaster", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "POST",
					Path:             "/cadasters/cadastertopics",
					File:             "src/api/cadasters.js",
					Line:             19,
					ServiceCandidate: "ms-cadaster",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/cadasters/{cadasterId}", Handler: "CadasterController.get", File: "CadasterController.java", Line: 108},
			},
		},
	})

	if len(matches) != 1 {
		t.Fatalf("matches length = %d, want 1: %#v", len(matches), matches)
	}
	if matches[0].Issue == contractIssueMethodMismatch {
		t.Fatalf("static path was matched too broadly against parameter route: %#v", matches[0])
	}
	if matches[0].Issue != contractIssueMissingRoute {
		t.Fatalf("issue = %q, want %q: %#v", matches[0].Issue, contractIssueMissingRoute, matches[0])
	}
}

func TestWorkspaceFeatureFlowsConnectFrontendBackendServiceAndTests(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "apps/portal/src/api/cadasterservice.js", "export function importCadaster(id) {\n"+
		"  return fetch(`/cadasters/${id}/import`, { method: 'POST' });\n"+
		"}\n")
	writeFile(t, cadaster, "src/main/java/com/example/CadasterController.java", `package com.example;

import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  private final CadasterService cadasterService;

  CadasterController(CadasterService cadasterService) {
    this.cadasterService = cadasterService;
  }

  @PostMapping("/{cadasterId}/import")
  String importFile(@PathVariable String cadasterId) {
    return cadasterService.importFile(cadasterId);
  }
}
`)
	writeFile(t, cadaster, "src/main/java/com/example/CadasterService.java", `package com.example;

import org.springframework.stereotype.Service;

@Service
class CadasterService {
  private final CadasterRepository cadasterRepository;

  CadasterService(CadasterRepository cadasterRepository) {
    this.cadasterRepository = cadasterRepository;
  }

  String importFile(String cadasterId) {
    cadasterRepository.save(cadasterId);
    return cadasterId;
  }
}
`)
	writeFile(t, cadaster, "src/main/java/com/example/CadasterRepository.java", `package com.example;

import org.springframework.stereotype.Repository;

@Repository
class CadasterRepository {
  void save(String cadasterId) {
  }
}
`)
	writeFile(t, cadaster, "src/test/java/com/example/CadasterControllerTest.java", `package com.example;

import org.junit.jupiter.api.Test;
import org.springframework.test.web.servlet.MockMvc;

class CadasterControllerTest {
  private MockMvc mockMvc;

  @Test
  void importsFile() throws Exception {
    mockMvc.perform(post("/cadasters/42/import"));
  }
}
`)

	if _, err := Run(frontend, config.Defaults()); err != nil {
		t.Fatalf("Run frontend returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("Run cadaster returned error: %v", err)
	}

	featureReport := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	for _, want := range []string{
		"apps/portal/src/api/cadasterservice.js:2",
		"CadasterController.importFile",
		"CadasterService.importFile",
		"CadasterRepository.save",
		"CadasterControllerTest",
	} {
		if !strings.Contains(featureReport, want) {
			t.Fatalf("workspace feature flow report missing %q:\n%s", want, featureReport)
		}
	}

	var flows []WorkspaceFeatureFlowRecord
	readJSON(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.json"), &flows)
	if len(flows) != 1 {
		t.Fatalf("feature flow count = %d, want 1: %#v", len(flows), flows)
	}
	if len(flows[0].BackendSteps) < 3 {
		t.Fatalf("feature flow missing backend steps: %#v", flows[0])
	}
	if len(flows[0].Tests) == 0 {
		t.Fatalf("feature flow missing tests: %#v", flows[0])
	}

	workspaceReport := readText(t, filepath.Join(workspace, ".goregraph-workspace", "feature-flows.md"))
	if !strings.Contains(workspaceReport, "frontend/frontend-monorepo") || !strings.Contains(workspaceReport, "microservices/ms-cadaster") {
		t.Fatalf("workspace-level feature flow report missing projects:\n%s", workspaceReport)
	}
}

func TestWorkspaceFeatureFlowsIncludeFrontendRouteComponentAndAPICall(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "apps/portal/src/routes.jsx", `import { Route } from "react-router-dom";
import { CadasterPage } from "./pages/CadasterPage";

export function Routes() {
  return <Route path="/cadasters/:cadasterId" component={CadasterPage} />;
}
`)
	writeFile(t, frontend, "apps/portal/src/pages/CadasterPage.jsx", `import { loadCadaster } from "../api/cadasterservice";

export function CadasterPage(props) {
  return loadCadaster(props.cadasterId);
}
`)
	writeFile(t, frontend, "apps/portal/src/api/cadasterservice.js", "import { GetHelper } from '../utils/requestHelper';\n\n"+
		"export function loadCadaster(dispatch, id) {\n"+
		"  return GetHelper(dispatch, `/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, frontend, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(dispatch, path) { return fetch(path); }
`)
	writeFile(t, cadaster, "src/main/java/com/example/CadasterController.java", `package com.example;

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

	if _, err := Run(frontend, config.Defaults()); err != nil {
		t.Fatalf("Run frontend returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("Run cadaster returned error: %v", err)
	}

	report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	for _, want := range []string{
		"- Frontend route: `portal:/cadasters/:cadasterId` `/cadasters/:cadasterId` -> `CadasterPage`",
		"- Frontend API: `apps/portal/src/api/cadasterservice.js:4` `loadCadaster`",
		"route flow reaches API contract caller",
		"CadasterController.get",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("workspace feature flow report missing %q:\n%s", want, report)
		}
	}

	var flows []WorkspaceFeatureFlowRecord
	readJSON(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.json"), &flows)
	if len(flows) != 1 {
		t.Fatalf("feature flow count = %d, want 1: %#v", len(flows), flows)
	}
	if flows[0].FrontendRouteID != "portal:/cadasters/:cadasterId" {
		t.Fatalf("FrontendRouteID = %q, want portal route: %#v", flows[0].FrontendRouteID, flows[0])
	}
	if flows[0].FrontendComponent != "CadasterPage" {
		t.Fatalf("FrontendComponent = %q, want CadasterPage: %#v", flows[0].FrontendComponent, flows[0])
	}
	if flows[0].FrontendCaller != "loadCadaster" {
		t.Fatalf("FrontendCaller = %q, want loadCadaster: %#v", flows[0].FrontendCaller, flows[0])
	}
}

func TestWorkspaceFeatureFlowsExplainMissingFrontendRouteContext(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "apps/portal/src/api/cadasterservice.js", "import { GetHelper } from '../utils/requestHelper';\n\n"+
		"export function loadCadaster(dispatch, id) {\n"+
		"  return GetHelper(dispatch, `/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, frontend, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(dispatch, path) { return fetch(path); }
`)
	writeFile(t, cadaster, "src/main/java/com/example/CadasterController.java", `package com.example;

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

	if _, err := Run(frontend, config.Defaults()); err != nil {
		t.Fatalf("Run frontend returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("Run cadaster returned error: %v", err)
	}

	report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	if !strings.Contains(report, "Frontend route: none resolved") {
		t.Fatalf("feature flow report should explain missing frontend route context:\n%s", report)
	}
}

func TestWorkspaceStatusDetectsWorkspaceRootItself(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, cadaster, "README.md", "# ms-cadaster\n")

	status, err := WorkspaceStatus(workspace, config.Defaults())
	if err != nil {
		t.Fatalf("WorkspaceStatus returned error: %v", err)
	}

	if !strings.Contains(status, "frontend/frontend-monorepo") || !strings.Contains(status, "microservices/ms-cadaster") {
		t.Fatalf("workspace status missing projects:\n%s", status)
	}
}

func assertWorkspaceProject(t *testing.T, registry WorkspaceRegistryRecord, relPath, status string, indexed bool) {
	t.Helper()
	for _, project := range registry.Projects {
		if project.Path == relPath {
			if project.Status != status || project.Indexed != indexed {
				t.Fatalf("workspace project %q = status %q indexed %v, want status %q indexed %v in %#v", relPath, project.Status, project.Indexed, status, indexed, registry.Projects)
			}
			return
		}
	}
	t.Fatalf("missing workspace project %q in %#v", relPath, registry.Projects)
}

func assertGeneratedFile(t *testing.T, generated []string, name string) {
	t.Helper()
	for _, candidate := range generated {
		if candidate == name {
			return
		}
	}
	t.Fatalf("generated files missing %q in %#v", name, generated)
}
