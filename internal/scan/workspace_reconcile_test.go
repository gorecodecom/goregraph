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

func TestWorkspaceRootPrefersParentWithFrontendAndMicroservices(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	nestedFrontend := filepath.Join(workspace, "frontend", "frontend")
	nestedFrontends := filepath.Join(workspace, "frontend", "frontends")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "src/api/cadasterservice.js", "export function loadCadaster(id) {\n"+
		"  return fetch(`/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, nestedFrontend, "package.json", `{"name":"nested-frontend"}`)
	writeFile(t, nestedFrontends, "package.json", `{"name":"nested-frontends"}`)
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

	root, ok, err := WorkspaceRoot(frontend, config.Defaults())
	if err != nil {
		t.Fatalf("WorkspaceRoot returned error: %v", err)
	}
	if !ok {
		t.Fatalf("WorkspaceRoot did not detect workspace")
	}
	if !samePath(root, workspace) {
		t.Fatalf("WorkspaceRoot = %q, want parent workspace %q", root, workspace)
	}

	if _, err := Run(frontend, config.Defaults()); err != nil {
		t.Fatalf("Run frontend returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("Run cadaster returned error: %v", err)
	}

	context := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-context.md"))
	if strings.Contains(context, "No GoreGraph workspace detected") ||
		!strings.Contains(context, "Workspace root: `"+filepath.ToSlash(workspace)+"`") ||
		!strings.Contains(context, "microservices/ms-cadaster") {
		t.Fatalf("frontend workspace context should use parent workspace and include backend:\n%s", context)
	}

	matches := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-contract-matches.md"))
	if !strings.Contains(matches, "GET `/cadasters/{id}` -> ms-cadaster GET `/cadasters/{cadasterId}`") {
		t.Fatalf("frontend workspace contract overlay missing backend match:\n%s", matches)
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
	var frontendMatchJSON []WorkspaceContractMatchRecord
	readJSON(t, filepath.Join(frontend, "goregraph-out", "workspace-contract-matches.json"), &frontendMatchJSON)
	if len(frontendMatchJSON) != 1 ||
		frontendMatchJSON[0].Issue != contractIssueMatched ||
		frontendMatchJSON[0].APIProject != "frontend/frontend-monorepo" ||
		frontendMatchJSON[0].BackendProject != "microservices/ms-cadaster" {
		t.Fatalf("frontend workspace contract JSON overlay missing backend match: %#v", frontendMatchJSON)
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
	var backendMatchJSON []WorkspaceContractMatchRecord
	readJSON(t, filepath.Join(cadaster, "goregraph-out", "workspace-contract-matches.json"), &backendMatchJSON)
	if len(backendMatchJSON) != 1 ||
		backendMatchJSON[0].APIProject != "frontend/frontend-monorepo" ||
		backendMatchJSON[0].BackendProject != "microservices/ms-cadaster" {
		t.Fatalf("backend workspace contract JSON overlay missing incoming frontend match: %#v", backendMatchJSON)
	}

	var manifest Manifest
	readJSON(t, filepath.Join(frontend, "goregraph-out", "manifest.json"), &manifest)
	assertGeneratedFile(t, manifest.Generated, "workspace-context.md")
	assertGeneratedFile(t, manifest.Generated, "workspace-contract-matches.md")
	assertGeneratedFile(t, manifest.Generated, "workspace-contract-matches.json")
	assertGeneratedFile(t, manifest.Generated, "frontend-consumers.md")

	var audit AuditRecord
	readJSON(t, filepath.Join(frontend, "goregraph-out", "audit.json"), &audit)
	assertGeneratedFile(t, audit.Generated, "workspace-context.md")
	assertGeneratedFile(t, audit.Generated, "workspace-contract-matches.md")
	assertGeneratedFile(t, audit.Generated, "workspace-contract-matches.json")
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

func TestWorkspaceDiagnosticsDoNotKeepIndexedMissingRoutesAsUnscanned(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	task := filepath.Join(workspace, "microservices", "ms-task")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "src/api/taskservice.js", "export function loadTask(id) {\n"+
		"  return fetch(`/task/${id}`);\n"+
		"}\n")
	writeFile(t, task, "src/main/java/com/example/TaskController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/task")
class TaskController {
  @GetMapping("/overview")
  String overview() {
    return "ok";
  }
}
`)

	if _, err := Run(frontend, config.Defaults()); err != nil {
		t.Fatalf("Run frontend returned error: %v", err)
	}
	if _, err := Run(task, config.Defaults()); err != nil {
		t.Fatalf("Run task returned error: %v", err)
	}

	frontendMatches := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-contract-matches.md"))
	if !strings.Contains(frontendMatches, "indexed_backend_route_missing") {
		t.Fatalf("frontend workspace contract overlay should classify indexed service mismatch as scanned service route gap:\n%s", frontendMatches)
	}

	frontendDiagnostics := readText(t, filepath.Join(frontend, "goregraph-out", "diagnostics.md"))
	if strings.Contains(frontendDiagnostics, "`ms-task` -") {
		t.Fatalf("frontend diagnostics still reports indexed ms-task as unscanned:\n%s", frontendDiagnostics)
	}

	var diagnostics DiagnosticsRecord
	readJSON(t, filepath.Join(frontend, "goregraph-out", "diagnostics.json"), &diagnostics)
	for _, service := range diagnostics.UnscannedServices {
		if service.Service == "ms-task" {
			t.Fatalf("diagnostics json still reports indexed ms-task as unscanned: %#v", diagnostics.UnscannedServices)
		}
	}
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
	if matches[0].Issue != contractIssueIndexedBackendRouteMissing {
		t.Fatalf("issue = %q, want %q: %#v", matches[0].Issue, contractIssueIndexedBackendRouteMissing, matches[0])
	}
}

func TestWorkspaceContractMatchesClassifyGatewayPrefixMatches(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-task", Kind: "backend", Service: "ms-task", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "GET",
					Path:             "/api/tasks/{id}",
					File:             "src/api/task.js",
					Line:             12,
					ServiceCandidate: "ms-task",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/tasks/{taskId}", Handler: "TaskController.get", File: "TaskController.java", Line: 24},
			},
		},
	})

	if len(matches) != 1 {
		t.Fatalf("matches length = %d, want 1: %#v", len(matches), matches)
	}
	if matches[0].Issue != contractIssueGatewayOrProxyPrefix {
		t.Fatalf("issue = %q, want %q: %#v", matches[0].Issue, contractIssueGatewayOrProxyPrefix, matches[0])
	}
	if matches[0].BackendProject != "microservices/ms-task" || matches[0].BackendPath != "/tasks/{taskId}" {
		t.Fatalf("gateway prefix match did not retain backend route context: %#v", matches[0])
	}
}

func TestWorkspaceContractMatchesNormalizeServiceAndConfigBasePrefixes(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-productservice", Kind: "backend", Service: "ms-productservice", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "GET",
					Path:             "/productservice/users/{userId}/products/{baseCode}",
					File:             "src/api/products.js",
					Line:             12,
					ServiceCandidate: "ms-productservice",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/ApplicationConfig.BASE_PATH/users/{userId}/products/{baseCode}", Handler: "ProductController.get", File: "ProductController.java", Line: 28},
			},
		},
	})

	if len(matches) != 1 {
		t.Fatalf("matches length = %d, want 1: %#v", len(matches), matches)
	}
	if matches[0].Issue != contractIssueMatched || matches[0].Confidence != "RESOLVED" {
		t.Fatalf("prefix-normalized route should resolve: %#v", matches[0])
	}
	if matches[0].BackendPath != "/users/{userId}/products/{baseCode}" {
		t.Fatalf("BackendPath = %q, want normalized display path: %#v", matches[0].BackendPath, matches[0])
	}
}

func TestWorkspaceContractMatchesSuggestSimilarBackendRoutes(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-task", Kind: "backend", Service: "ms-task", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "GET",
					Path:             "/tasks/{id}/detail",
					File:             "src/api/task.js",
					Line:             18,
					ServiceCandidate: "ms-task",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/tasks/{taskId}", Handler: "TaskController.get", File: "TaskController.java", Line: 24},
				{Kind: "backend", HTTPMethod: "POST", Path: "/tasks/{taskId}/details", Handler: "TaskController.updateDetails", File: "TaskController.java", Line: 40},
			},
		},
	})

	if len(matches) != 1 {
		t.Fatalf("matches length = %d, want 1: %#v", len(matches), matches)
	}
	if matches[0].Issue != contractIssueIndexedBackendRouteMissing {
		t.Fatalf("issue = %q, want %q: %#v", matches[0].Issue, contractIssueIndexedBackendRouteMissing, matches[0])
	}
	if !strings.Contains(matches[0].Reason, "similar backend routes:") || !strings.Contains(matches[0].Reason, "POST /tasks/{taskId}/details") {
		t.Fatalf("missing similar route hint in reason: %#v", matches[0])
	}
}

func TestWorkspaceContractMatchesNormalizeSimilarBackendRouteHints(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-productservice", Kind: "backend", Service: "ms-productservice", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "GET",
					Path:             "/productservice/users/{userId}/products/{baseCode}/detail",
					File:             "src/api/product.js",
					Line:             18,
					ServiceCandidate: "ms-productservice",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/ApplicationConfig.BASE_PATH/users/{userId}/products/{baseCode}", Handler: "ProductController.get", File: "ProductController.java", Line: 24},
			},
		},
	})

	if len(matches) != 1 {
		t.Fatalf("matches length = %d, want 1: %#v", len(matches), matches)
	}
	if !strings.Contains(matches[0].Reason, "GET /users/{userId}/products/{baseCode}") {
		t.Fatalf("similar route hint was not normalized: %#v", matches[0])
	}
	if strings.Contains(matches[0].Reason, "ApplicationConfig.BASE_PATH") {
		t.Fatalf("similar route hint leaked raw constant: %#v", matches[0])
	}
}

func TestWorkspaceContractMatchesClassifyDynamicEndpointRemainders(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-documenttopic", Kind: "backend", Service: "ms-documenttopic", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "GET",
					Path:             "/documenttopic/modules/{isbn}/{endpoint}",
					File:             "src/api/topic.js",
					Line:             18,
					ServiceCandidate: "ms-documenttopic",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/documenttopic/modules/{isbn}/documents/new", Handler: "TopicController.new", File: "TopicController.java", Line: 24},
			},
		},
	})

	if len(matches) != 1 {
		t.Fatalf("matches length = %d, want 1: %#v", len(matches), matches)
	}
	if matches[0].Issue != contractIssueDynamicEndpointUnresolved {
		t.Fatalf("issue = %q, want %q: %#v", matches[0].Issue, contractIssueDynamicEndpointUnresolved, matches[0])
	}
	if !strings.Contains(matches[0].Reason, "dynamic endpoint segment") {
		t.Fatalf("reason should explain dynamic endpoint segment: %#v", matches[0])
	}
}

func TestWorkspaceContractMatchesClassifyMissingIndexedBackendRoute(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-documentexport", Kind: "backend", Service: "ms-documentexport", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "GET",
					Path:             "/documentexport/modules/{isbn}/documents/{objectId}/availability",
					File:             "src/api/export.js",
					Line:             18,
					ServiceCandidate: "ms-documentexport",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/documentexport/modules/{isbn}/documents/{objectId}/export/{exportId}/status", Handler: "ExportController.status", File: "ExportController.java", Line: 24},
			},
		},
	})

	if len(matches) != 1 {
		t.Fatalf("matches length = %d, want 1: %#v", len(matches), matches)
	}
	if matches[0].Issue != contractIssueIndexedBackendRouteMissing {
		t.Fatalf("issue = %q, want %q: %#v", matches[0].Issue, contractIssueIndexedBackendRouteMissing, matches[0])
	}
	if !strings.Contains(matches[0].Reason, "indexed backend service has no route") {
		t.Fatalf("reason should explain missing route in indexed service: %#v", matches[0])
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

func TestWorkspaceFeatureFlowsResolveRenderedComponentToAPICaller(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, cadaster, "pom.xml", `<project><artifactId>ms-cadaster</artifactId></project>`)
	writeFile(t, frontend, "apps/portal/src/routes.jsx", `import { Route } from "react-router-dom";
import { Home } from "./pages/Home";

export function Routes() {
  return <Route path="/cadaster/:cadasterId" component={Home} />;
}
`)
	writeFile(t, frontend, "apps/portal/src/pages/Home.jsx", `import { CadasterPanel } from "./CadasterPanel";

export function Home() {
  return <CadasterPanel />;
}
`)
	writeFile(t, frontend, "apps/portal/src/pages/CadasterPanel.jsx", `import { loadCadaster } from "../api/cadasterservice";

export function CadasterPanel(props) {
  return loadCadaster(props.dispatch, props.cadasterId);
}
`)
	writeFile(t, frontend, "apps/portal/src/api/cadasterservice.js", "import { GetHelper } from '../utils/requestHelper';\n\n"+
		"export function loadCadaster(dispatch, id) {\n"+
		"  return GetHelper(dispatch, `/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, frontend, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(dispatch, path) { return fetch(path); }
`)
	writeFile(t, cadaster, "src/main/java/com/weka/vd/api/cadaster/controller/CadasterController.java", `package com.weka.vd.api.cadaster.controller;

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
		t.Fatalf("frontend Run returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("cadaster Run returned error: %v", err)
	}

	report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	for _, want := range []string{
		"- Frontend route: `portal:/cadaster/:cadasterId` `/cadaster/:cadasterId` -> `Home`",
		"- Frontend API: `apps/portal/src/api/cadasterservice.js:4` `loadCadaster`",
		"RESOLVED, route flow reaches API contract caller",
		"`CadasterController.get`",
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
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if flows[0].FrontendCaller != "loadCadaster" {
		t.Fatalf("FrontendCaller = %q, want loadCadaster: %#v", flows[0].FrontendCaller, flows[0])
	}
}

func TestWorkspaceFeatureFlowsResolveEffectToAPICaller(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, cadaster, "pom.xml", `<project><artifactId>ms-cadaster</artifactId></project>`)
	writeFile(t, frontend, "apps/portal/src/routes.jsx", `import { Route } from "react-router-dom";
import { CadasterPage } from "./pages/CadasterPage";

export const routes = <Route path="/cadasters/:cadasterId" element={<CadasterPage />} />;
`)
	writeFile(t, frontend, "apps/portal/src/pages/CadasterPage.jsx", `import { useEffect } from "react";
import { loadCadaster } from "../api/cadasterservice";

export function CadasterPage({ cadasterId }) {
  useEffect(() => {
    loadCadaster(cadasterId);
  }, [cadasterId]);
  return <main />;
}
`)
	writeFile(t, frontend, "apps/portal/src/api/cadasterservice.js", "import { GetHelper } from '../utils/requestHelper';\n\n"+
		"export function loadCadaster(id) {\n"+
		"  return GetHelper(null, `/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, frontend, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(dispatch, path) { return fetch(path); }
`)
	writeFile(t, cadaster, "src/main/java/com/weka/vd/api/cadaster/controller/CadasterController.java", `package com.weka.vd.api.cadaster.controller;

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
		t.Fatalf("frontend Run returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("cadaster Run returned error: %v", err)
	}

	report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	for _, want := range []string{
		"- Frontend route: `portal:/cadasters/:cadasterId` `/cadasters/:cadasterId` -> `CadasterPage`",
		"RESOLVED, route flow reaches API contract caller through effect",
		"- Frontend API: `apps/portal/src/api/cadasterservice.js:4` `loadCadaster`",
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
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if flows[0].FrontendReason != "route flow reaches API contract caller through effect" {
		t.Fatalf("FrontendReason = %q, want effect reason: %#v", flows[0].FrontendReason, flows[0])
	}
}

func TestWorkspaceFeatureFlowsResolveEventHandlerToAPICaller(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, cadaster, "pom.xml", `<project><artifactId>ms-cadaster</artifactId></project>`)
	writeFile(t, frontend, "apps/portal/src/routes.jsx", `import { Route } from "react-router-dom";
import { CadasterPage } from "./pages/CadasterPage";

export const routes = <Route path="/cadasters/:cadasterId" element={<CadasterPage />} />;
`)
	writeFile(t, frontend, "apps/portal/src/pages/CadasterPage.jsx", `import { loadCadaster } from "../api/cadasterservice";

export function CadasterPage({ cadasterId }) {
  function handleRefresh() {
    loadCadaster(cadasterId);
  }
  return <button onClick={handleRefresh}>Refresh</button>;
}
`)
	writeFile(t, frontend, "apps/portal/src/api/cadasterservice.js", "import { GetHelper } from '../utils/requestHelper';\n\n"+
		"export function loadCadaster(id) {\n"+
		"  return GetHelper(null, `/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, frontend, "apps/portal/src/utils/requestHelper.js", `export function GetHelper(dispatch, path) { return fetch(path); }
`)
	writeFile(t, cadaster, "src/main/java/com/weka/vd/api/cadaster/controller/CadasterController.java", `package com.weka.vd.api.cadaster.controller;

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
		t.Fatalf("frontend Run returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("cadaster Run returned error: %v", err)
	}

	report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	for _, want := range []string{
		"- Frontend route: `portal:/cadasters/:cadasterId` `/cadasters/:cadasterId` -> `CadasterPage`",
		"RESOLVED, route flow reaches API contract caller through event handler",
		"- Frontend API: `apps/portal/src/api/cadasterservice.js:4` `loadCadaster`",
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
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if flows[0].FrontendReason != "route flow reaches API contract caller through event handler" {
		t.Fatalf("FrontendReason = %q, want event handler reason: %#v", flows[0].FrontendReason, flows[0])
	}
}

func TestWorkspaceFeatureFlowsUseAPIContractCallerWhenRouteIsWeak(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	task := filepath.Join(workspace, "microservices", "ms-task")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, cadaster, "pom.xml", `<project><artifactId>ms-cadaster</artifactId></project>`)
	writeFile(t, task, "pom.xml", `<project><artifactId>ms-task</artifactId></project>`)
	writeFile(t, frontend, "apps/portal/src/routes.jsx", `import { Route } from "react-router-dom";
import { Home } from "./pages/Home";

export const routes = <Route path="/" element={<Home />} />;
`)
	writeFile(t, frontend, "apps/portal/src/pages/Home.jsx", `export function Home() {
  return <main />;
}
`)
	writeFile(t, frontend, "apps/portal/src/api/vd/userService.ts", "import { GetHelper } from '../../utils/requestHelper';\n\n"+
		"export const userService = {\n"+
		"  getCurrentCadaster(cadasterId) {\n"+
		"    return GetHelper(null, `/cadasters/${cadasterId}`);\n"+
		"  }\n"+
		"  getTask(taskId) {\n"+
		"    return GetHelper(null, `/tasks/${taskId}`);\n"+
		"  }\n"+
		"  getCurrentTask() {\n"+
		"    return GetHelper(null, '/tasks/current');\n"+
		"  }\n"+
		"};\n")
	writeFile(t, frontend, "apps/portal/src/utils/requestHelper.ts", `export function GetHelper(dispatch, path) { return fetch(path); }
`)
	writeFile(t, cadaster, "src/main/java/com/weka/vd/api/cadaster/controller/CadasterController.java", `package com.weka.vd.api.cadaster.controller;

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
		t.Fatalf("frontend Run returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("cadaster Run returned error: %v", err)
	}

	report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	for _, want := range []string{
		"- Frontend route: `portal:/` `/` -> `Home`",
		"WEAK_MATCH, frontend route shares app with API contract but no route-flow step reached the API caller",
		"- Frontend API: `apps/portal/src/api/vd/userService.ts:5` `getCurrentCadaster`",
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
	if flows[0].FrontendConfidence != "WEAK_MATCH" {
		t.Fatalf("FrontendConfidence = %q, want WEAK_MATCH: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if flows[0].FrontendCaller != "getCurrentCadaster" {
		t.Fatalf("FrontendCaller = %q, want API contract caller: %#v", flows[0].FrontendCaller, flows[0])
	}

	apiContracts := readText(t, filepath.Join(frontend, "goregraph-out", "api-contracts.md"))
	if !strings.Contains(apiContracts, "caller `getCurrentCadaster`") {
		t.Fatalf("api-contracts report should include API caller:\n%s", apiContracts)
	}

	workspaceMatches := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-contract-matches.md"))
	if !strings.Contains(workspaceMatches, "caller `getCurrentCadaster`") {
		t.Fatalf("workspace contract matches should include API caller:\n%s", workspaceMatches)
	}

	frontendConsumers := readText(t, filepath.Join(cadaster, "goregraph-out", "frontend-consumers.md"))
	if !strings.Contains(frontendConsumers, "caller `getCurrentCadaster`") {
		t.Fatalf("frontend consumers report should include API caller:\n%s", frontendConsumers)
	}

	endpoints := readText(t, filepath.Join(cadaster, "goregraph-out", "endpoints.md"))
	if !strings.Contains(endpoints, "caller `getCurrentCadaster`") {
		t.Fatalf("backend endpoints consumers should include API caller:\n%s", endpoints)
	}

	context := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-context.md"))
	for _, want := range []string{
		"`ms-task` - 2 contracts - project `microservices/ms-task` - not_indexed",
		"cd microservices/ms-task && goregraph scan .",
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("workspace context missing %q:\n%s", want, context)
		}
	}

	nextActions := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-next-actions.md"))
	for _, want := range []string{
		"# GoreGraph Workspace Next Actions",
		"- Projects indexed: 2 / 3",
		"- Referenced services indexed: 1 / 2",
		"- `ms-task` - 2 contracts - project `microservices/ms-task` - not_indexed",
		"`cd microservices/ms-task && goregraph scan .`",
	} {
		if !strings.Contains(nextActions, want) {
			t.Fatalf("workspace next actions missing %q:\n%s", want, nextActions)
		}
	}
}

func TestWorkspaceFeatureFlowsResolveComponentLocalAPICallerWithoutRouteFlow(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-search", Kind: "backend", Service: "ms-search", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "PUT",
					Path:             "/search",
					File:             "apps/portal/src/components/search/search.jsx",
					Line:             81,
					Caller:           "fetchResults",
					ServiceCandidate: "ms-search",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "PUT", Path: "/search", Handler: "SearchController.search", File: "SearchController.java", Line: 24},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "PUT", Path: "/search", Controller: "SearchController", Method: "search"},
			},
		},
	})
	flows := buildWorkspaceFeatureFlows([]workspaceIndexProject{
		{
			record: frontend,
			codeFlows: []CodeFlowRecord{
				{Kind: "frontend", App: "portal", RouteID: "portal:/search", Path: "/search", File: "apps/portal/src/routes.jsx", Line: 5, Handler: "SearchPage"},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "PUT", Path: "/search", Handler: "SearchController.search", File: "SearchController.java", Line: 24},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "PUT", Path: "/search", Controller: "SearchController", Method: "search"},
			},
		},
	}, matches)

	if len(flows) != 1 {
		t.Fatalf("flows length = %d, want 1: %#v", len(flows), flows)
	}
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if !strings.Contains(flows[0].FrontendReason, "component or page file") {
		t.Fatalf("FrontendReason should explain component-local caller: %#v", flows[0])
	}
}

func TestWorkspaceFeatureFlowsResolvePackageUIAPICallerWithoutRouteFlow(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-search", Kind: "backend", Service: "ms-search", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "PUT",
					Path:             "/search",
					File:             "packages/designsystem/src/organisms/search/Search.tsx",
					Line:             42,
					Caller:           "runSearch",
					ServiceCandidate: "ms-search",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "PUT", Path: "/search", Handler: "SearchController.search", File: "SearchController.java", Line: 24},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "PUT", Path: "/search", Controller: "SearchController", Method: "search"},
			},
		},
	})
	flows := buildWorkspaceFeatureFlows([]workspaceIndexProject{
		{record: frontend},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "PUT", Path: "/search", Handler: "SearchController.search", File: "SearchController.java", Line: 24},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "PUT", Path: "/search", Controller: "SearchController", Method: "search"},
			},
		},
	}, matches)

	if len(flows) != 1 {
		t.Fatalf("flows length = %d, want 1: %#v", len(flows), flows)
	}
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if !strings.Contains(flows[0].FrontendReason, "package UI file") {
		t.Fatalf("FrontendReason should explain package UI caller: %#v", flows[0])
	}
}

func TestWorkspaceFeatureFlowsResolvePackageUtilityAPICallerWithoutRouteFlow(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-documentexport", Kind: "backend", Service: "ms-documentexport", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "POST",
					Path:             "/documentexport/modules/{isbn}/documents/{objectId}/export",
					File:             "packages/designsystem/src/utils/export.ts",
					Line:             24,
					Caller:           "generateExport",
					ServiceCandidate: "ms-documentexport",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "POST", Path: "/documentexport/modules/{isbn}/documents/{objectId}/export", Handler: "DocumentExportController.postExport", File: "DocumentExportController.java", Line: 42},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "POST", Path: "/documentexport/modules/{isbn}/documents/{objectId}/export", Controller: "DocumentExportController", Method: "postExport"},
			},
		},
	})
	flows := buildWorkspaceFeatureFlows([]workspaceIndexProject{
		{record: frontend},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "POST", Path: "/documentexport/modules/{isbn}/documents/{objectId}/export", Handler: "DocumentExportController.postExport", File: "DocumentExportController.java", Line: 42},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "POST", Path: "/documentexport/modules/{isbn}/documents/{objectId}/export", Controller: "DocumentExportController", Method: "postExport"},
			},
		},
	}, matches)

	if len(flows) != 1 {
		t.Fatalf("flows length = %d, want 1: %#v", len(flows), flows)
	}
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if !strings.Contains(flows[0].FrontendReason, "package utility file") {
		t.Fatalf("FrontendReason should explain package utility caller: %#v", flows[0])
	}
}

func TestWorkspaceFeatureFlowsResolveAppRootAPICallerWithoutRouteFlow(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-documentinfo", Kind: "backend", Service: "ms-documentinfo", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "GET",
					Path:             "/documentinfo/modules/{isbn}/info",
					File:             "apps/wekapilot/src/Root.tsx",
					Line:             19,
					Caller:           "fetchModuleInfo",
					ServiceCandidate: "ms-documentinfo",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/documentinfo/modules/{isbn}/info", Handler: "InfoController.info", File: "InfoController.java", Line: 31},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "GET", Path: "/documentinfo/modules/{isbn}/info", Controller: "InfoController", Method: "info"},
			},
		},
	})
	flows := buildWorkspaceFeatureFlows([]workspaceIndexProject{
		{
			record: frontend,
			codeFlows: []CodeFlowRecord{
				{Kind: "frontend", App: "wekapilot", RouteID: "wekapilot:/", Path: "/", File: "apps/wekapilot/src/routes.tsx", Line: 5, Handler: "Root"},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/documentinfo/modules/{isbn}/info", Handler: "InfoController.info", File: "InfoController.java", Line: 31},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "GET", Path: "/documentinfo/modules/{isbn}/info", Controller: "InfoController", Method: "info"},
			},
		},
	}, matches)

	if len(flows) != 1 {
		t.Fatalf("flows length = %d, want 1: %#v", len(flows), flows)
	}
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if !strings.Contains(flows[0].FrontendReason, "app root file") {
		t.Fatalf("FrontendReason should explain app root caller: %#v", flows[0])
	}
}

func TestWorkspaceFeatureFlowsResolveFrontendRelationToAPICaller(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-cadaster", Kind: "backend", Service: "ms-cadaster", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "GET",
					Path:             "/cadasters/{cadasterId}/users",
					File:             "apps/portal/src/api/vd/userService.ts",
					Line:             4,
					Caller:           "fetchCadastersUsers",
					ServiceCandidate: "ms-cadaster",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/cadasters/{cadasterId}/users", Handler: "CadasterUserController.getUsers", File: "CadasterUserController.java", Line: 31},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "GET", Path: "/cadasters/{cadasterId}/users", Controller: "CadasterUserController", Method: "getUsers"},
			},
		},
	})
	flows := buildWorkspaceFeatureFlows([]workspaceIndexProject{
		{
			record: frontend,
			codeFlows: []CodeFlowRecord{
				{Kind: "frontend", App: "portal", RouteID: "portal:/tasks", Path: "/tasks", File: "apps/portal/src/routes.tsx", Line: 8, Handler: "TasksPage"},
			},
			relations: []RelationRecord{
				{From: "apps/portal/src/components/modals/vd/taskModal/vdTaskModal.tsx", To: "apps/portal/src/api/vd/userService.ts", Type: "calls", Line: 50},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/cadasters/{cadasterId}/users", Handler: "CadasterUserController.getUsers", File: "CadasterUserController.java", Line: 31},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "GET", Path: "/cadasters/{cadasterId}/users", Controller: "CadasterUserController", Method: "getUsers"},
			},
		},
	}, matches)

	if len(flows) != 1 {
		t.Fatalf("flows length = %d, want 1: %#v", len(flows), flows)
	}
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if !strings.Contains(flows[0].FrontendReason, "frontend relation reaches API contract file") {
		t.Fatalf("FrontendReason should explain relation evidence: %#v", flows[0])
	}
}

func TestWorkspaceFeatureFlowsResolveContainerLocalAPICallerWithoutRouteFlow(t *testing.T) {
	frontend := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Kind: "frontend", Indexed: true}
	backend := WorkspaceProjectRecord{Path: "microservices/ms-cadastertask", Kind: "backend", Service: "ms-cadastertask", Indexed: true}

	matches := buildWorkspaceContractMatches([]workspaceIndexProject{
		{
			record: frontend,
			contracts: []APIContractRecord{
				{
					HTTPMethod:       "GET",
					Path:             "/cadastertask/cadasters/tasks",
					File:             "apps/vorschriftendienst/src/containers/CadasterContainer/tasksService.ts",
					Line:             12,
					ServiceCandidate: "ms-cadastertask",
				},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/cadastertask/cadasters/tasks", Handler: "TaskController.getTasks", File: "TaskController.java", Line: 31},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "GET", Path: "/cadastertask/cadasters/tasks", Controller: "TaskController", Method: "getTasks"},
			},
		},
	})
	flows := buildWorkspaceFeatureFlows([]workspaceIndexProject{
		{
			record: frontend,
			codeFlows: []CodeFlowRecord{
				{Kind: "frontend", App: "vorschriftendienst", RouteID: "vorschriftendienst:/cadasters", Path: "/cadasters", File: "apps/vorschriftendienst/src/routes.js", Line: 5, Handler: "CadasterContainer"},
			},
		},
		{
			record: backend,
			routes: []CodeRouteRecord{
				{Kind: "backend", HTTPMethod: "GET", Path: "/cadastertask/cadasters/tasks", Handler: "TaskController.getTasks", File: "TaskController.java", Line: 31},
			},
			endpointFlows: []SpringEndpointFlowRecord{
				{HTTPMethod: "GET", Path: "/cadastertask/cadasters/tasks", Controller: "TaskController", Method: "getTasks"},
			},
		},
	}, matches)

	if len(flows) != 1 {
		t.Fatalf("flows length = %d, want 1: %#v", len(flows), flows)
	}
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if !strings.Contains(flows[0].FrontendReason, "container file") {
		t.Fatalf("FrontendReason should explain container-local caller: %#v", flows[0])
	}
}

func TestWorkspaceFeatureFlowsResolvePortalServiceMethodToAPICaller(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, cadaster, "pom.xml", `<project><artifactId>ms-cadaster</artifactId></project>`)
	writeFile(t, frontend, "apps/portal/src/routes.jsx", `import { Route } from "react-router-dom";
import { Home } from "./pages/Home";

export const routes = <Route path="/" element={<Home />} />;
`)
	writeFile(t, frontend, "apps/portal/src/pages/Home.jsx", `import { userService } from "../api/vd/userService";

export function Home({ cadasterId }) {
  return userService.getCurrentCadaster(cadasterId);
}
`)
	writeFile(t, frontend, "apps/portal/src/api/vd/userService.ts", "import { GetHelper } from '../../utils/requestHelper';\n\n"+
		"export const userService = {\n"+
		"  getCurrentCadaster(cadasterId) {\n"+
		"    return GetHelper(null, `/cadasters/${cadasterId}`);\n"+
		"  }\n"+
		"};\n")
	writeFile(t, frontend, "apps/portal/src/utils/requestHelper.ts", `export function GetHelper(dispatch, path) { return fetch(path); }
`)
	writeFile(t, cadaster, "src/main/java/com/weka/vd/api/cadaster/controller/CadasterController.java", `package com.weka.vd.api.cadaster.controller;

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
		t.Fatalf("frontend Run returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("cadaster Run returned error: %v", err)
	}

	report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	for _, want := range []string{
		"- Frontend route: `portal:/` `/` -> `Home`",
		"RESOLVED, route flow reaches API contract caller",
		"- Frontend API: `apps/portal/src/api/vd/userService.ts:5` `getCurrentCadaster`",
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
	if flows[0].FrontendConfidence != "RESOLVED" {
		t.Fatalf("FrontendConfidence = %q, want RESOLVED: %#v", flows[0].FrontendConfidence, flows[0])
	}
	if flows[0].FrontendCaller != "getCurrentCadaster" {
		t.Fatalf("FrontendCaller = %q, want API contract caller: %#v", flows[0].FrontendCaller, flows[0])
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

func TestWorkspaceNextActionsDoesNotPrefixBackendMethodWithNoneOwner(t *testing.T) {
	report := renderWorkspaceNextActionsReport(
		WorkspaceContextRecord{},
		nil,
		[]WorkspaceFeatureFlowRecord{
			{
				HTTPMethod:       "GET",
				Path:             "/invoiceservice/users/{userId}/invoices",
				FrontendProject:  "frontend/frontend-monorepo",
				FrontendFile:     "apps/mein-konto/src/api/invoiceservice.js",
				FrontendLine:     27,
				BackendMethod:    "InvoicesController.getInvoicesOfUser",
				BackendProject:   "microservices/ms-invoiceservice",
				BackendService:   "ms-invoiceservice",
				BackendFile:      "src/main/java/InvoicesController.java",
				BackendLine:      59,
				Confidence:       "RESOLVED",
				TestReason:       "no endpoint or backend-step tests matched backend endpoint GET `/users/{userId}/invoices`",
			},
		},
	)

	if strings.Contains(report, "none.InvoicesController.getInvoicesOfUser") {
		t.Fatalf("next actions report leaked none owner prefix:\n%s", report)
	}
	if !strings.Contains(report, "-> `InvoicesController.getInvoicesOfUser`") {
		t.Fatalf("next actions report missing backend method label:\n%s", report)
	}
}

func TestWorkspaceFeatureTestMatchesKnownBasePrefixes(t *testing.T) {
	test := TestMapRecord{
		Type:       "endpoint",
		HTTPMethod: "GET",
		Path:       "/ApplicationConfig.BASE_PATH/cadasters/{cadasterId}/regulations/{objectId}/tasks",
	}
	match := WorkspaceContractMatchRecord{
		BackendHTTPMethod: "GET",
		BackendPath:       "/cadasters/{cadasterId}/regulations/{objectId}/tasks",
	}

	if !workspaceFeatureTestMatches(test, SpringEndpointFlowRecord{}, match) {
		t.Fatalf("workspace feature test should match known base-prefix variants: %#v -> %#v", test, match)
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
