package scan

import (
	"os"
	"path/filepath"
	"strings"
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
	assertRichEdge(t, rich.Edges, "contains", "EXTRACTED")

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

	var audit AuditRecord
	readJSON(t, filepath.Join(out, "audit.json"), &audit)
	if audit.NetworkUsed || audit.ExternalCommands {
		t.Fatalf("audit reported unsafe activity: %#v", audit)
	}
}

func TestRunExtractsWekaStyleSpringIntelligence(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pom.xml", `<project><groupId>com.weka</groupId><artifactId>ms-demo</artifactId><version>1.0.0</version></project>`)
	writeFile(t, root, "src/main/java/com/weka/demo/DemoApplication.java", `package com.weka.demo;

import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication(scanBasePackages = "com.weka")
public class DemoApplication {
  public static void main(String[] args) {}
}
`)
	writeFile(t, root, "src/main/java/com/weka/demo/config/ApplicationConfig.java", `package com.weka.demo.config;

import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.context.annotation.Configuration;

@Configuration
@ConfigurationProperties(prefix = "")
public class ApplicationConfig {
  public static final String BASE_PATH = "/cadasters";
}
`)
	writeFile(t, root, "src/main/java/com/weka/demo/controller/CadasterController.java", `package com.weka.demo.controller;

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
`)
	writeFile(t, root, "src/main/java/com/weka/demo/model/CadasterCopyRequest.java", "package com.weka.demo.model;\npublic record CadasterCopyRequest(String name) {}\n")
	writeFile(t, root, "src/main/java/com/weka/demo/service/CadasterService.java", `package com.weka.demo.service;

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
    return cadasterRepository.findAll().stream().map(item -> item).toList();
  }

  public CadasterEntity copyCadaster(final long cadasterId, final CadasterCopyRequest request) {
    final CadasterEntity source = cadasterRepository.findById(cadasterId).orElseThrow();
    return cadasterRepository.save(source.withName(request.name()));
  }
}
`)
	writeFile(t, root, "src/main/java/com/weka/demo/repository/CadasterRepository.java", `package com.weka.demo.repository;

import com.weka.demo.entity.CadasterEntity;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface CadasterRepository extends JpaRepository<CadasterEntity, Long> {
}
`)
	writeFile(t, root, "src/main/java/com/weka/demo/entity/CadasterEntity.java", `package com.weka.demo.entity;

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
`)
	writeFile(t, root, "src/test/java/com/weka/demo/controller/CadasterControllerTest.java", "package com.weka.demo.controller;\nclass CadasterControllerTest {}\n")
	writeFile(t, root, "src/test/java/com/weka/demo/service/CadasterServiceTest.java", "package com.weka.demo.service;\nclass CadasterServiceTest {}\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := filepath.Join(root, "goregraph-out")
	for _, name := range []string{"spring.json", "endpoints.md", "dependencies.md", "workspace.md", "affected.md"} {
		if !fileExists(filepath.Join(out, name)) {
			t.Fatalf("%s was not written", name)
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

	workspace := readText(t, filepath.Join(out, "workspace.md"))
	if !strings.Contains(workspace, "ms-demo") {
		t.Fatalf("workspace report missing Maven metadata:\n%s", workspace)
	}
}

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

func assertNoSymbol(t *testing.T, symbols []SymbolRecord, name, kind string) {
	t.Helper()
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			t.Fatalf("unexpected symbol name=%q kind=%q in %#v", name, kind, symbols)
		}
	}
}

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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
