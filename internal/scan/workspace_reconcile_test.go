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

	consumers := readText(t, filepath.Join(cadaster, "goregraph-out", "frontend-consumers.md"))
	if !strings.Contains(consumers, "frontend/frontend-monorepo") || !strings.Contains(consumers, "src/api/cadasterservice.js") {
		t.Fatalf("backend frontend-consumers overlay missing frontend usage:\n%s", consumers)
	}

	var registry WorkspaceRegistryRecord
	readJSON(t, filepath.Join(workspace, ".goregraph-workspace", "registry.json"), &registry)
	assertWorkspaceProject(t, registry, "frontend/frontend-monorepo", "indexed", true)
	assertWorkspaceProject(t, registry, "microservices/ms-cadaster", "current", true)
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
