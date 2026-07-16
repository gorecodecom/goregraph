package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestRichSymbolAdditionsPreserveLegacyGraphIdentity(t *testing.T) {
	legacy := RichSymbolRecord{
		ID:       "legacy-symbol-id",
		Name:     "UserService",
		Kind:     "class",
		Language: "java",
		File:     "src/UserService.java",
		Line:     3,
	}
	enriched := legacy
	enriched.QualifiedName = "com.weka.UserService"

	legacyJSON, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	enrichedJSON, err := json.Marshal(enriched)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(legacyJSON), `"qualified_name"`) {
		t.Fatalf("legacy-only symbol emitted additive field: %s", legacyJSON)
	}
	if !strings.Contains(string(enrichedJSON), `"qualified_name":"com.weka.UserService"`) {
		t.Fatalf("enriched symbol omitted qualified name: %s", enrichedJSON)
	}

	legacyGraph := buildRichGraph(nil, []RichSymbolRecord{legacy}, nil)
	enrichedGraph := buildRichGraph(nil, []RichSymbolRecord{enriched}, nil)
	if len(legacyGraph.Nodes) != 1 || len(enrichedGraph.Nodes) != 1 {
		t.Fatalf("unexpected graph nodes: legacy=%#v enriched=%#v", legacyGraph.Nodes, enrichedGraph.Nodes)
	}
	if legacyGraph.Nodes[0].ID != legacy.ID || enrichedGraph.Nodes[0].ID != legacy.ID {
		t.Fatalf("additive fields changed graph identity: legacy=%#v enriched=%#v", legacyGraph.Nodes[0], enrichedGraph.Nodes[0])
	}
}

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
	assertRichEdgeType(t, rich.Edges, "imports")

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

func TestJavaCallGraphMarksTypeOnlyTargetsAsExtracted(t *testing.T) {
	graph := buildJavaCallGraph([]JavaSourceRecord{
		{
			File: "src/main/java/com/example/ExampleService.java",
			Types: []JavaTypeRecord{
				{Name: "ExampleService", Kind: "class", File: "src/main/java/com/example/ExampleService.java", Line: 1},
			},
			Fields: []JavaFieldRecord{
				{Name: "repository", Type: "ExampleRepository", Owner: "ExampleService", File: "src/main/java/com/example/ExampleService.java", Line: 2},
			},
			Methods: []JavaMethodRecord{
				{
					Name:  "load",
					Owner: "ExampleService",
					File:  "src/main/java/com/example/ExampleService.java",
					Line:  4,
					Calls: []JavaCallRecord{{Receiver: "repository", Method: "findById", Line: 5}},
				},
			},
		},
		{
			File: "src/main/java/com/example/ExampleRepository.java",
			Types: []JavaTypeRecord{
				{Name: "ExampleRepository", Kind: "interface", File: "src/main/java/com/example/ExampleRepository.java", Line: 1},
			},
		},
	})

	assertHasJavaCallGraphEdgeConfidence(t, graph.Edges, "ExampleService", "load", "ExampleRepository", "findById", "EXTRACTED")
}

func TestParseJavaMethodSignatureWithAnnotatedMultipartParameters(t *testing.T) {
	line := `public ResponseEntity<?> importFile(@RequestPart(name = "csvFile") MultipartFile multipartFile, @RequestPart(name = "ownerUserId") String ownerUserId) throws Exception {`
	name, returnType, params, visibility, ok := parseJavaMethodSignature(line)
	if !ok {
		t.Fatalf("parseJavaMethodSignature returned false")
	}
	if name != "importFile" || returnType != "ResponseEntity<?>" || visibility != "public" {
		t.Fatalf("unexpected signature parts name=%q returnType=%q visibility=%q", name, returnType, visibility)
	}
	parsed := parseJavaParameters(params)
	if len(parsed) != 2 {
		t.Fatalf("parameter count = %d, want 2: %#v", len(parsed), parsed)
	}
	if parsed[0].Name != "multipartFile" || parsed[0].Type != "MultipartFile" || !hasAnnotation(parsed[0].Annotations, "RequestPart") {
		t.Fatalf("unexpected first parameter: %#v", parsed[0])
	}
	if parsed[1].Name != "ownerUserId" || parsed[1].Type != "String" || !hasAnnotation(parsed[1].Annotations, "RequestPart") {
		t.Fatalf("unexpected second parameter: %#v", parsed[1])
	}
}

func TestSpringEndpointPathMatchesKnownBasePrefixes(t *testing.T) {
	if !springEndpointPathMatches("/ApplicationConfig.BASE_PATH/cadasters/{cadasterId}/regulations/{objectId}/tasks", "/cadastertask/cadasters/101/regulations/10/tasks") {
		t.Fatal("spring endpoint path matcher should treat config base and service prefixes as compatible")
	}
}

func TestExtractJavaSourceFindsHelperHTTPRequests(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/test/java/DemoTest.java", Language: "java"}, `class DemoTest {
  @Test
  void updatesStateWithHelperMethod() throws Exception {
    mockMvc.perform(putStateRequestFor(CADASTER_ID));
  }

  @Test
  void updatesStateNoAuthIsUnauthorized() throws Exception {
    mockMvc.perform(putStateRequestFor(CADASTER_ID));
  }

  private MockHttpServletRequestBuilder putStateRequestFor(final long cadasterId) {
    return MockMvcRequestBuilders.put("/cadasters/%d/state".formatted(cadasterId));
  }
}`)

	var testMethod JavaMethodRecord
	var helper JavaMethodRecord
	for _, method := range source.Methods {
		switch method.Name {
		case "updatesStateWithHelperMethod":
			testMethod = method
		case "putStateRequestFor":
			helper = method
		}
	}
	if !hasJavaCall(testMethod.Calls, "putStateRequestFor") {
		t.Fatalf("test method did not record helper call: %#v", testMethod.Calls)
	}
	if len(helper.HTTPRequests) != 1 {
		t.Fatalf("helper HTTP request count = %d, want 1: %#v", len(helper.HTTPRequests), helper.HTTPRequests)
	}
	if helper.HTTPRequests[0].HTTPMethod != "PUT" || helper.HTTPRequests[0].Path != "/cadasters/{dynamic}/state" {
		t.Fatalf("unexpected helper HTTP request: %#v", helper.HTTPRequests[0])
	}
}

func TestSpringIndexExtractsDTOFieldsAndEndpointAuth(t *testing.T) {
	controller := extractJavaSource(FileRecord{Path: "src/main/java/com/example/CadasterController.java", Language: "java"}, `package com.example;

import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @PreAuthorize("hasAuthority('CADASTER_WRITE')")
  @PostMapping("/{cadasterId}/copy")
  CadasterDto copy(@RequestBody CadasterCopyRequest request) {
    return new CadasterDto();
  }
}
`)
	request := extractJavaSource(FileRecord{Path: "src/main/java/com/example/CadasterCopyRequest.java", Language: "java"}, `package com.example;

import jakarta.validation.constraints.NotBlank;

class CadasterCopyRequest {
  @NotBlank
  private String name;
}
`)
	response := extractJavaSource(FileRecord{Path: "src/main/java/com/example/CadasterDto.java", Language: "java"}, `package com.example;

class CadasterDto {
  private Long id;
}
`)

	spring := buildSpringIndex([]JavaSourceRecord{controller, request, response})
	endpoint, ok := findSpringEndpointForTest(spring.Endpoints, "POST", "/cadasters/{cadasterId}/copy")
	if !ok {
		t.Fatalf("missing endpoint in %#v", spring.Endpoints)
	}
	if len(endpoint.Auth) != 1 || endpoint.Auth[0].Expression != "hasAuthority('CADASTER_WRITE')" {
		t.Fatalf("missing endpoint auth: %#v", endpoint)
	}
	assertHasDTOField(t, spring.DTOs, "CadasterCopyRequest", "name", true)
	assertHasDTOField(t, spring.DTOs, "CadasterDto", "id", false)
}

func TestSpringIndexKeepsMappingAfterMultilineOpenAPIOperation(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/example/CadasterController.java", Language: "java"}, `package com.example;

import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @Operation(
      summary = "Get all cadasters of user",
      responses = {@ApiResponse(responseCode = "200", content = @Content(array = @ArraySchema(schema = @Schema(implementation = CadasterResponse.class)))),
          @ApiResponse(responseCode = "204", content = @Content)},
      security = @SecurityRequirement(name = "bearerAuth"))
  @GetMapping(produces = MediaType.APPLICATION_JSON_VALUE)
  public ResponseEntity<?> gets() {
    return ResponseEntity.ok();
  }
}
`)

	spring := buildSpringIndex([]JavaSourceRecord{source})
	endpoint, ok := findSpringEndpointForTest(spring.Endpoints, "GET", "/cadasters")
	if !ok {
		t.Fatalf("missing endpoint after multiline Operation; methods=%#v endpoints=%#v", source.Methods, spring.Endpoints)
	}
	if endpoint.ReturnType != "CadasterResponse" {
		t.Fatalf("return type = %q, want CadasterResponse: %#v", endpoint.ReturnType, endpoint)
	}
}

func TestExtractJavaSourceKeepsApplicationConfigBasePathPrefix(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/test/java/DemoTest.java", Language: "java"}, `class DemoTest {
  @Test
  void getsTasks() throws Exception {
    mockMvc.perform(get(ApplicationConfig.BASE_PATH + "/services/" + KNOWN_MODULE + "/tasks"));
  }
}`)

	if len(source.Methods) != 1 {
		t.Fatalf("method count = %d, want 1: %#v", len(source.Methods), source.Methods)
	}
	requests := source.Methods[0].HTTPRequests
	if len(requests) != 1 {
		t.Fatalf("HTTP request count = %d, want 1: %#v", len(requests), requests)
	}
	if requests[0].HTTPMethod != "GET" || requests[0].Path != "/ApplicationConfig.BASE_PATH/services/{dynamic}/tasks" {
		t.Fatalf("unexpected request: %#v", requests[0])
	}
	if !springEndpointPathMatches("/ApplicationConfig.BASE_PATH/services/{serviceCode}/tasks", requests[0].Path) {
		t.Fatalf("request path should match unresolved ApplicationConfig endpoint: %s", requests[0].Path)
	}
}

func TestExtractJavaSourceStripsHTTPTestQueryString(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/test/java/DemoTest.java", Language: "java"}, `class DemoTest {
  @Test
  void autocompletes() throws Exception {
    mockMvc.perform(get("/search/autocomplete?search=was&authorisation=RDBV"));
  }
}`)

	requests := source.Methods[0].HTTPRequests
	if len(requests) != 1 {
		t.Fatalf("HTTP request count = %d, want 1: %#v", len(requests), requests)
	}
	if requests[0].Path != "/search/autocomplete" {
		t.Fatalf("request path = %q, want /search/autocomplete", requests[0].Path)
	}
}

func TestExtractJavaSourceReadsStringFormatHTTPPath(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/test/java/DemoTest.java", Language: "java"}, `class DemoTest {
  @Test
  void downloadsBinary() throws Exception {
    mockMvc.perform(get(String.format(ApplicationConfig.BASE_PATH + "/modules/%s/downloads/binary/%s", isbn, objectId)));
  }
}`)

	requests := source.Methods[0].HTTPRequests
	if len(requests) != 1 {
		t.Fatalf("HTTP request count = %d, want 1: %#v", len(requests), requests)
	}
	if requests[0].Path != "/ApplicationConfig.BASE_PATH/modules/{dynamic}/downloads/binary/{dynamic}" {
		t.Fatalf("request path = %q", requests[0].Path)
	}
}

func TestBuildJavaTestMapMatchesRegulationChangeBaseControllerConstants(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/test/java/RegulationChangesSaveRelevantForControllerTest.java", Language: "java"}, `class RegulationChangesSaveRelevantForControllerTest {
  @Test
  public void savesRelevantFor() throws Exception {
    mockMvc.perform(buildPut(cadasterId, lra, objectId));
  }

  private MockHttpServletRequestBuilder buildPut(final Long cadasterId, final Timestamp lra, final Long objectId)
      throws JacksonException {
    return put("/cadasters/" + cadasterId + "/regulations/" + objectId + "/changes/" + lra.getTime());
  }
}`)
	endpoints := []SpringEndpointRecord{{
		HTTPMethod: "PUT",
		Path:       "/RegulationChangeBaseController.PATH_BASE/{cadasterId}/regulations/{objectId}/changes/{lraTimestamp}",
		Controller: "RegulationChangesController",
		Method:     "editRegulationChangeDetails",
		File:       "src/main/java/RegulationChangesController.java",
	}}
	var testMethod JavaMethodRecord
	var helper JavaMethodRecord
	for _, method := range source.Methods {
		switch method.Name {
		case "savesRelevantFor":
			testMethod = method
		case "buildPut":
			helper = method
		}
	}
	if !hasJavaCall(testMethod.Calls, "buildPut") {
		t.Fatalf("test method did not record helper call: %#v", testMethod.Calls)
	}
	if len(helper.HTTPRequests) != 1 {
		t.Fatalf("helper HTTP request count = %d, want 1: %#v", len(helper.HTTPRequests), helper.HTTPRequests)
	}
	if !springEndpointPathMatches(endpoints[0].Path, helper.HTTPRequests[0].Path) {
		t.Fatalf("endpoint path %q did not match helper path %q", endpoints[0].Path, helper.HTTPRequests[0].Path)
	}

	records := buildJavaTestMap([]JavaSourceRecord{source}, endpoints)
	assertHasEndpointTestMap(t, records, "RegulationChangesSaveRelevantForControllerTest", "savesRelevantFor", "PUT", endpoints[0].Path)
}

func TestBuildJavaTestMapPropagatesInheritedHTTPHelper(t *testing.T) {
	base := extractJavaSource(FileRecord{Path: "src/test/java/ControllerBaseTest.java", Language: "java"}, `class ControllerBaseTest {
  protected MockHttpServletRequestBuilder getForUser(final String isbn, final Optional<Long> objectId, final String suffix) {
    return get("/documentinfo/modules/" + isbn + "/documents/" + objectId.orElseThrow() + suffix);
  }
}`)
	test := extractJavaSource(FileRecord{Path: "src/test/java/DocumentAuthorsControllerTest.java", Language: "java"}, `class DocumentAuthorsControllerTest extends ControllerBaseTest {
  @Test
  void getsAuthors() throws Exception {
    mockMvc.perform(getForUser(KNOWN_MODULE, Optional.of(KNOWN_OBJECT), "/authors"));
  }
}`)
	endpoints := []SpringEndpointRecord{{
		HTTPMethod: "GET",
		Path:       "/documentinfo/modules/{isbn}/documents/{objectId}/authors",
		Controller: "DocumentController",
		Method:     "getAuthors",
		File:       "src/main/java/DocumentController.java",
	}}
	var testMethod JavaMethodRecord
	for _, method := range test.Methods {
		if method.Name == "getsAuthors" {
			testMethod = method
		}
	}
	if !hasJavaCall(testMethod.Calls, "getForUser") {
		t.Fatalf("test method did not record getForUser call: %#v", testMethod.Calls)
	}
	var helperCall JavaCallRecord
	for _, call := range testMethod.Calls {
		if call.Method == "getForUser" {
			helperCall = call
		}
	}
	var helper JavaMethodRecord
	for _, method := range base.Methods {
		if method.Name == "getForUser" {
			helper = method
		}
	}
	if len(helper.HTTPRequests) != 1 {
		t.Fatalf("helper HTTP request count = %d, want 1: %#v", len(helper.HTTPRequests), helper.HTTPRequests)
	}
	specialized := specializeJavaHelperHTTPRequests(helper.HTTPRequests, helperCall.Arguments)
	if len(specialized) != 1 || specialized[0].Path != "/documentinfo/modules/{dynamic}/documents/{dynamic}/authors" {
		t.Fatalf("unexpected specialized helper requests: %#v from args %#v", specialized, helperCall.Arguments)
	}
	if !springEndpointPathMatches(endpoints[0].Path, specialized[0].Path) {
		t.Fatalf("endpoint path %q did not match specialized helper path %q", endpoints[0].Path, specialized[0].Path)
	}

	records := buildJavaTestMap([]JavaSourceRecord{base, test}, endpoints)
	assertHasEndpointTestMap(t, records, "DocumentAuthorsControllerTest", "getsAuthors", "GET", endpoints[0].Path)
}

func TestBuildJavaTestMapDoesNotPropagateGenericHTTPHelperAcrossClasses(t *testing.T) {
	binary := extractJavaSource(FileRecord{Path: "src/test/java/BinaryControllerTest.java", Language: "java"}, `class BinaryControllerTest {
  private RequestBuilder get(final String objectId, final String isbn) {
    return MockMvcRequestBuilders.get(String.format(ApplicationConfig.BASE_PATH + "/modules/%s/downloads/binary/%s", isbn, objectId));
  }
}`)
	search := extractJavaSource(FileRecord{Path: "src/test/java/SearchControllerTest.java", Language: "java"}, `class SearchControllerTest {
  @Test
  void searches() throws Exception {
    mockMvc.perform(get(KNOWN_MODULE, 800, Optional.of("query")));
  }
}`)
	endpoints := []SpringEndpointRecord{{
		HTTPMethod: "GET",
		Path:       "/ApplicationConfig.BASE_PATH/modules/{isbn}/downloads/binary/{objectId}",
		Controller: "DocumentDownloadBinaryController",
		Method:     "getBinary",
		File:       "src/main/java/DocumentDownloadBinaryController.java",
	}}

	records := buildJavaTestMap([]JavaSourceRecord{binary, search}, endpoints)
	for _, record := range records {
		if record.TestClass == "SearchControllerTest" {
			t.Fatalf("generic cross-class get helper was incorrectly propagated: %#v", records)
		}
	}
}

func TestExtractJavaSourceDoesNotTreatCatchAsMethod(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/SecurityConfig.java", Language: "java"}, `class SecurityConfig {
    void configure() {
        try {
            run();
        } catch (Exception ex) {
            recover();
        }
    }
}`)

	for _, method := range source.Methods {
		if method.Name == "catch" {
			t.Fatalf("catch block was extracted as method: %#v", source.Methods)
		}
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
import com.weka.demo.model.ImportResult;
import com.weka.demo.service.CadasterService;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RequestPart;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.multipart.MultipartFile;

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

  @GetMapping("/{cadasterId}")
  public ResponseEntity<?> get(@PathVariable("cadasterId") final long cadasterId) {
    return ResponseEntity.ok(cadasterService.getCadaster(cadasterId));
  }

  @PutMapping("/{cadasterId}/copy")
  public ResponseEntity<?> updateCopy(@PathVariable("cadasterId") final long cadasterId, @RequestBody final CadasterCopyRequest request) {
    return ResponseEntity.ok(cadasterService.copyCadaster(cadasterId, request));
  }

  @PutMapping("/{cadasterId}/state")
  public ResponseEntity<?> updateState(@PathVariable("cadasterId") final long cadasterId) {
    return ResponseEntity.ok(cadasterService.getCadaster(cadasterId));
  }

  @PostMapping(path = "/{cadasterId}/copy")
  public ResponseEntity<?> copy(@PathVariable("cadasterId") final long cadasterId, @RequestBody final CadasterCopyRequest request) {
    return ResponseEntity.ok(cadasterService.copyCadaster(cadasterId, request));
  }

  @Operation(
      summary = "Import CSV to create a cadaster",
      responses = {@ApiResponse(responseCode = "200", content = @Content(schema = @Schema(implementation = ImportResult.class)))})
  @PostMapping(path = "/{cadasterId}/import", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)
  public ResponseEntity<ImportResult> importFile(
      @PathVariable("cadasterId") final long cadasterId,
      @RequestPart("file") final MultipartFile file,
      @RequestParam("source") final String source) {
    if (source == null) {
      throw new ForbiddenException("missing");
    }
    return ResponseEntity.ok(cadasterService.importFile(cadasterId, file, source));
  }
}
`)
	writeFile(t, root, "src/main/java/com/weka/demo/model/CadasterCopyRequest.java", "package com.weka.demo.model;\npublic record CadasterCopyRequest(String name) {}\n")
	writeFile(t, root, "src/main/java/com/weka/demo/model/ImportResult.java", "package com.weka.demo.model;\npublic record ImportResult(long imported) {}\n")
	writeFile(t, root, "src/main/java/com/weka/demo/service/CadasterService.java", `package com.weka.demo.service;

import com.weka.demo.entity.CadasterEntity;
import com.weka.demo.model.CadasterCopyRequest;
import com.weka.demo.model.ImportResult;
import com.weka.demo.repository.CadasterRepository;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.web.multipart.MultipartFile;

@Service
@RequiredArgsConstructor(onConstructor_ = {@Autowired})
public class CadasterService {
  private final CadasterRepository cadasterRepository;

  public List<CadasterEntity> getUserCadasters() {
    return cadasterRepository.findAll().stream().map(item -> item).toList();
  }

  public CadasterEntity getCadaster(final long cadasterId) {
    return cadasterRepository.findById(cadasterId).orElseThrow();
  }

  public CadasterEntity copyCadaster(final long cadasterId, final CadasterCopyRequest request) {
    final CadasterEntity source = cadasterRepository.findById(cadasterId).orElseThrow();
    return cadasterRepository.save(source.withName(request.name()));
  }

  public ImportResult importFile(final long cadasterId, final MultipartFile file, final String source) {
    final CadasterEntity target = cadasterRepository.findById(cadasterId).orElseThrow();
    cadasterRepository.save(target);
    return new ImportResult(1);
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
	writeFile(t, root, "src/test/java/com/weka/demo/controller/CadasterControllerTest.java", `package com.weka.demo.controller;

import org.junit.jupiter.api.Test;
import org.springframework.test.web.reactive.server.WebTestClient;
import org.springframework.test.web.servlet.request.MockHttpServletRequestBuilder;
import org.springframework.test.web.servlet.request.MockMvcRequestBuilders;
import org.springframework.test.web.servlet.MockMvc;

class CadasterControllerTest {
  private MockMvc mockMvc;
  private WebTestClient client;

  @Test
  void importsFile() throws Exception {
    mockMvc.perform(post("/cadasters/42/import"));
  }

  @Test
  void importsFileFromUriVariable() throws Exception {
    final String uri = "/cadasters/42/import";
    mockMvc.perform(post(uri));
  }

  @Test
  void getsDetails() throws Exception {
    mockMvc.perform(get("/cadasters/" + CADASTER_ID));
  }

  @Test
  void updatesCopyWithBuilderHelper() throws Exception {
    mockMvc.perform(buildRequest(MockMvcRequestBuilders::put, "/cadasters/" + CADASTER_ID + "/copy"));
  }

  @Test
  void updatesCopyWithWebTestClient() {
    client
        .put()
        .uri("/cadasters/42/copy")
        .exchange()
        .expectStatus().isOk();
  }

  @Test
  void updatesStateWithHelperMethod() throws Exception {
    mockMvc.perform(putStateRequestFor(CADASTER_ID));
  }

  @Test
  void updatesStateNoAuthIsUnauthorized() throws Exception {
    mockMvc.perform(putStateRequestFor(CADASTER_ID));
  }

  private MockHttpServletRequestBuilder buildRequest(final RequestBuilderFactory factory, final String uri) {
    return factory.apply(uri);
  }

  private MockHttpServletRequestBuilder putStateRequestFor(final long cadasterId) {
    return MockMvcRequestBuilders.put("/cadasters/%d/state".formatted(cadasterId));
  }
}
`)
	writeFile(t, root, "src/test/java/com/weka/demo/service/CadasterServiceTest.java", `package com.weka.demo.service;

import org.junit.jupiter.api.Test;

class CadasterServiceTest {
  @Test
  void importsFile() {
    new CadasterService(null).importFile(1L, null, "manual");
  }
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := filepath.Join(root, "goregraph-out")
	for _, name := range []string{"spring.json", "endpoints.md", "dependencies.md", "workspace.md", "affected.md", "callgraph.json", "callgraph.md", "endpoint-flows.json", "endpoint-flows.md", "test-map.json", "analyzers.json", "analyzers.md"} {
		if !fileExists(filepath.Join(out, name)) {
			t.Fatalf("%s was not written", name)
		}
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(out, "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "CadasterController", "class", "src/main/java/com/weka/demo/controller/CadasterController.java")
	assertHasSymbol(t, symbols, "gets", "method", "src/main/java/com/weka/demo/controller/CadasterController.java")
	assertNoSymbol(t, symbols, "for", "class")
	assertNoSymbol(t, symbols, "ForbiddenException", "method")
	assertNoSymbol(t, symbols, "catch", "method")

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
	assertHasSpringEndpoint(t, spring.Endpoints, "GET", "/cadasters/{cadasterId}", "CadasterController", "get")
	assertHasSpringEndpoint(t, spring.Endpoints, "PUT", "/cadasters/{cadasterId}/copy", "CadasterController", "updateCopy")
	assertHasSpringEndpoint(t, spring.Endpoints, "PUT", "/cadasters/{cadasterId}/state", "CadasterController", "updateState")
	assertHasSpringEndpoint(t, spring.Endpoints, "POST", "/cadasters/{cadasterId}/copy", "CadasterController", "copy")
	assertHasSpringEndpoint(t, spring.Endpoints, "POST", "/cadasters/{cadasterId}/import", "CadasterController", "importFile")
	assertHasSpringEndpointRequest(t, spring.Endpoints, "POST", "/cadasters/{cadasterId}/import", "multipart", "MultipartFile")

	var callGraph CallGraphRecord
	readJSON(t, filepath.Join(out, "callgraph.json"), &callGraph)
	assertHasCallGraphEdge(t, callGraph.Edges, "CadasterController", "copy", "CadasterService", "copyCadaster")
	assertHasCallGraphEdge(t, callGraph.Edges, "CadasterController", "importFile", "CadasterService", "importFile")
	assertHasCallGraphEdge(t, callGraph.Edges, "CadasterService", "importFile", "CadasterRepository", "findById")
	assertHasJavaCallGraphEdgeConfidence(t, callGraph.Edges, "CadasterService", "importFile", "CadasterRepository", "findById", "EXTRACTED")
	assertHasCallGraphEdge(t, callGraph.Edges, "CadasterService", "importFile", "CadasterRepository", "save")

	var flows []SpringEndpointFlowRecord
	readJSON(t, filepath.Join(out, "endpoint-flows.json"), &flows)
	assertHasEndpointFlowStep(t, flows, "POST", "/cadasters/{cadasterId}/import", "CadasterController", "importFile")
	assertHasEndpointFlowStep(t, flows, "POST", "/cadasters/{cadasterId}/import", "CadasterService", "importFile")
	assertHasEndpointFlowStep(t, flows, "POST", "/cadasters/{cadasterId}/import", "CadasterRepository", "save")

	var testMapRecords []TestMapRecord
	readJSON(t, filepath.Join(out, "test-map.json"), &testMapRecords)
	assertHasMethodTestMap(t, testMapRecords, "CadasterServiceTest", "importsFile", "CadasterService", "importFile")
	assertHasEndpointTestMap(t, testMapRecords, "CadasterControllerTest", "importsFile", "POST", "/cadasters/{cadasterId}/import")
	assertHasEndpointTestMap(t, testMapRecords, "CadasterControllerTest", "importsFileFromUriVariable", "POST", "/cadasters/{cadasterId}/import")
	assertHasEndpointTestMap(t, testMapRecords, "CadasterControllerTest", "getsDetails", "GET", "/cadasters/{cadasterId}")
	assertHasEndpointTestMap(t, testMapRecords, "CadasterControllerTest", "updatesCopyWithBuilderHelper", "PUT", "/cadasters/{cadasterId}/copy")
	assertHasEndpointTestMap(t, testMapRecords, "CadasterControllerTest", "updatesCopyWithWebTestClient", "PUT", "/cadasters/{cadasterId}/copy")
	assertHasEndpointTestMap(t, testMapRecords, "CadasterControllerTest", "updatesStateWithHelperMethod", "PUT", "/cadasters/{cadasterId}/state")
	assertEndpointTestMapConfidence(t, testMapRecords, "CadasterControllerTest", "updatesStateWithHelperMethod", "MATCHED")
	assertEndpointTestMapCase(t, testMapRecords, "CadasterControllerTest", "updatesStateNoAuthIsUnauthorized", "auth_error", "401")

	var analyzers []AnalyzerRecord
	readJSON(t, filepath.Join(out, "analyzers.json"), &analyzers)
	assertHasAnalyzer(t, analyzers, "java", true, true, true)
	assertHasAnalyzer(t, analyzers, "maven", false, false, false)

	entrypoints := readText(t, filepath.Join(out, "entrypoints.md"))
	if !strings.Contains(entrypoints, "DemoApplication") || !strings.Contains(entrypoints, "Spring Boot application") {
		t.Fatalf("entrypoints report missing Spring Boot application:\n%s", entrypoints)
	}

	testMap := readText(t, filepath.Join(out, "test-map.md"))
	if !strings.Contains(testMap, "CadasterControllerTest") || !strings.Contains(testMap, "CadasterController") {
		t.Fatalf("test-map report missing Java test mapping:\n%s", testMap)
	}

	endpoints := readText(t, filepath.Join(out, "endpoints.md"))
	if !strings.Contains(endpoints, "GET `/cadasters`") || !strings.Contains(endpoints, "POST `/cadasters/{cadasterId}/copy`") || !strings.Contains(endpoints, "POST `/cadasters/{cadasterId}/import`") {
		t.Fatalf("endpoints report missing expected routes:\n%s", endpoints)
	}
	if !strings.Contains(endpoints, "multipart") || !strings.Contains(endpoints, "MultipartFile") {
		t.Fatalf("endpoints report missing multipart metadata:\n%s", endpoints)
	}

	callGraphReport := readText(t, filepath.Join(out, "callgraph.md"))
	if !strings.Contains(callGraphReport, "CadasterController.importFile") || !strings.Contains(callGraphReport, "CadasterService.importFile") {
		t.Fatalf("callgraph report missing expected call chain:\n%s", callGraphReport)
	}

	flowReport := readText(t, filepath.Join(out, "endpoint-flows.md"))
	if !strings.Contains(flowReport, "POST `/cadasters/{cadasterId}/import`") || !strings.Contains(flowReport, "CadasterRepository.save") {
		t.Fatalf("endpoint flow report missing expected flow:\n%s", flowReport)
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

func assertRichEdgeType(t *testing.T, edges []RichGraphEdge, edgeType string) {
	t.Helper()
	for _, edge := range edges {
		if edge.Type == edgeType {
			return
		}
	}
	t.Fatalf("missing rich edge type=%q in %#v", edgeType, edges)
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

func assertHasSpringEndpointRequest(t *testing.T, endpoints []SpringEndpointRecord, method, path, requestKind, requestType string) {
	t.Helper()
	for _, endpoint := range endpoints {
		if endpoint.HTTPMethod == method && endpoint.Path == path && endpoint.RequestKind == requestKind && endpoint.RequestType == requestType {
			return
		}
	}
	t.Fatalf("missing Spring endpoint request method=%q path=%q requestKind=%q requestType=%q in %#v", method, path, requestKind, requestType, endpoints)
}

func findSpringEndpointForTest(endpoints []SpringEndpointRecord, method, path string) (SpringEndpointRecord, bool) {
	for _, endpoint := range endpoints {
		if endpoint.HTTPMethod == method && endpoint.Path == path {
			return endpoint, true
		}
	}
	return SpringEndpointRecord{}, false
}

func assertHasDTOField(t *testing.T, dtos []DTORecord, dtoName, fieldName string, required bool) {
	t.Helper()
	for _, dto := range dtos {
		if dto.Name != dtoName {
			continue
		}
		for _, field := range dto.Fields {
			if field.Name == fieldName && field.Required == required {
				return
			}
		}
		t.Fatalf("DTO %q missing field %q required=%v in %#v", dtoName, fieldName, required, dto)
	}
	t.Fatalf("missing DTO %q in %#v", dtoName, dtos)
}

func assertHasCallGraphEdge(t *testing.T, edges []CallGraphEdgeRecord, fromOwner, fromMethod, toOwner, toMethod string) {
	t.Helper()
	for _, edge := range edges {
		if edge.From.Owner == fromOwner && edge.From.Method == fromMethod && edge.To.Owner == toOwner && edge.To.Method == toMethod {
			return
		}
	}
	t.Fatalf("missing call graph edge %s.%s -> %s.%s in %#v", fromOwner, fromMethod, toOwner, toMethod, edges)
}

func assertHasJavaCallGraphEdgeConfidence(t *testing.T, edges []CallGraphEdgeRecord, fromOwner, fromMethod, toOwner, toMethod, confidence string) {
	t.Helper()
	for _, edge := range edges {
		if edge.From.Owner == fromOwner && edge.From.Method == fromMethod && edge.To.Owner == toOwner && edge.To.Method == toMethod && edge.Confidence == confidence {
			return
		}
	}
	t.Fatalf("missing call graph edge %s.%s -> %s.%s confidence=%q in %#v", fromOwner, fromMethod, toOwner, toMethod, confidence, edges)
}

func assertHasEndpointFlowStep(t *testing.T, flows []SpringEndpointFlowRecord, method, path, owner, handler string) {
	t.Helper()
	for _, flow := range flows {
		if flow.HTTPMethod != method || flow.Path != path {
			continue
		}
		for _, step := range flow.Steps {
			if step.Owner == owner && step.Method == handler {
				return
			}
		}
	}
	t.Fatalf("missing endpoint flow step %s %s -> %s.%s in %#v", method, path, owner, handler, flows)
}

func hasJavaCall(calls []JavaCallRecord, method string) bool {
	for _, call := range calls {
		if call.Method == method {
			return true
		}
	}
	return false
}

func assertHasMethodTestMap(t *testing.T, records []TestMapRecord, testClass, testMethod, targetClass, targetMethod string) {
	t.Helper()
	for _, record := range records {
		if record.TestClass == testClass && record.TestMethod == testMethod && record.TargetClass == targetClass && record.TargetMethod == targetMethod {
			return
		}
	}
	t.Fatalf("missing method test map %s.%s -> %s.%s in %#v", testClass, testMethod, targetClass, targetMethod, records)
}

func assertHasEndpointTestMap(t *testing.T, records []TestMapRecord, testClass, testMethod, httpMethod, path string) {
	t.Helper()
	for _, record := range records {
		if record.TestClass == testClass && record.TestMethod == testMethod && record.HTTPMethod == httpMethod && record.Path == path {
			return
		}
	}
	t.Fatalf("missing endpoint test map %s.%s -> %s %s in %#v", testClass, testMethod, httpMethod, path, records)
}

func assertEndpointTestMapConfidence(t *testing.T, records []TestMapRecord, testClass, testMethod, confidence string) {
	t.Helper()
	for _, record := range records {
		if record.TestClass == testClass && record.TestMethod == testMethod && record.Type == "endpoint" {
			if record.Confidence != confidence {
				t.Fatalf("endpoint test map confidence = %q, want %q for %#v", record.Confidence, confidence, record)
			}
			return
		}
	}
	t.Fatalf("missing endpoint test map for confidence assertion %s.%s in %#v", testClass, testMethod, records)
}

func assertEndpointTestMapCase(t *testing.T, records []TestMapRecord, testClass, testMethod, testCase, status string) {
	t.Helper()
	for _, record := range records {
		if record.TestClass == testClass && record.TestMethod == testMethod && record.Type == "endpoint" {
			if record.TestCase != testCase || record.StatusExpectation != status {
				t.Fatalf("endpoint test case = %q/%q, want %q/%q for %#v", record.TestCase, record.StatusExpectation, testCase, status, record)
			}
			return
		}
	}
	t.Fatalf("missing endpoint test map for case assertion %s.%s in %#v", testClass, testMethod, records)
}

func assertHasAnalyzer(t *testing.T, records []AnalyzerRecord, language string, calls, endpoints, tests bool) {
	t.Helper()
	for _, record := range records {
		if record.Language == language && record.Calls == calls && record.Endpoints == endpoints && record.Tests == tests {
			return
		}
	}
	t.Fatalf("missing analyzer language=%q calls=%t endpoints=%t tests=%t in %#v", language, calls, endpoints, tests, records)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
