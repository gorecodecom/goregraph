package scan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestBuildJavaAPIContractsDeclarativeClients(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		serviceCandidate string
	}{
		{
			name: "FeignClient",
			body: `package example;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.DeleteMapping;
@FeignClient(name = "jobs", path = "/job-management")
interface JobClient {
  @DeleteMapping("/catalogs/{catalogId}/items/{itemId}")
  void deleteRelatedJobs(String catalogId, String itemId);
}`,
			serviceCandidate: "jobs",
		},
		{
			name: "HttpExchange",
			body: `package example;
import org.springframework.web.service.annotation.HttpExchange;
import org.springframework.web.service.annotation.DeleteExchange;
@HttpExchange(url = "/job-management")
interface JobClient {
  @DeleteExchange("/catalogs/{catalogId}/items/{itemId}")
  void deleteRelatedJobs(String catalogId, String itemId);
}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const file = "src/main/java/example/JobClient.java"
			source := extractJavaSource(FileRecord{Path: file, Language: "java"}, test.body)
			records := buildJavaAPIContracts([]JavaSourceRecord{source})
			want := APIContractRecord{
				Language: "java", Package: "example", HTTPMethod: "DELETE",
				Path:             "/job-management/catalogs/{catalogId}/items/{itemId}",
				ServiceCandidate: test.serviceCandidate,
				Caller:           "JobClient.deleteRelatedJobs",
				File:             file, Line: 6,
				Confidence: "EXACT", ConfidenceScore: 1,
			}
			if len(records) != 1 {
				t.Fatalf("contracts = %#v, want exactly one", records)
			}
			got := records[0]
			got.Reason = ""
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("contract = %#v, want %#v", got, want)
			}
		})
	}
}

func TestBuildJavaAPIContractsKeepsUnresolvedDeclarativePathsPartial(t *testing.T) {
	tests := []struct {
		name       string
		basePath   string
		wantPath   string
		confidence string
		unsafe     bool
	}{
		{name: "property expression", basePath: `"${client.path}"`, confidence: "PARTIAL", unsafe: true},
		{name: "external constant expression", basePath: "ExternalPaths.ROOT", confidence: "PARTIAL", unsafe: true},
		{name: "literal empty path", basePath: `""`, wantPath: "/items/{itemId}", confidence: "EXACT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := `import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.DeleteMapping;
@FeignClient(name = "jobs", path = ` + test.basePath + `)
interface JobClient {
  @DeleteMapping("/items/{itemId}")
  void deleteRelatedJobs(String itemId);
}`
			source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, body)
			records := buildJavaAPIContracts([]JavaSourceRecord{source})
			if len(records) != 1 {
				t.Fatalf("contracts = %#v, want one", records)
			}
			got := records[0]
			if got.Path != test.wantPath || got.Confidence != test.confidence || got.UnsafeDynamic != test.unsafe {
				t.Fatalf("declarative contract = %#v", got)
			}
			if test.unsafe && got.RawPath == "" {
				t.Fatalf("unresolved declarative expression lost raw path: %#v", got)
			}
		})
	}
}

func TestBuildJavaAPIContractsBoundImperativeClients(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantPath string
		wantRaw  string
	}{
		{
			name: "RestClient literal",
			body: `import org.springframework.web.client.RestClient;
class JobClient {
  private final RestClient restClient;
  void deleteRelatedJobs() {
    restClient
        .delete()
        .uri("/job-management/catalogs/7/items/9")
        .retrieve();
  }
}`,
			wantPath: "/job-management/catalogs/7/items/9",
			wantRaw:  `"/job-management/catalogs/7/items/9"`,
		},
		{
			name: "WebClient constant",
			body: `import org.springframework.web.reactive.function.client.WebClient;
class JobClient {
  private static final String DELETE_PATH = "/job-management/catalogs/7/items/9";
  private final WebClient webClient;
  void deleteRelatedJobs() {
    webClient
        .delete()
        .uri(DELETE_PATH)
        .retrieve();
  }
}`,
			wantPath: "/job-management/catalogs/7/items/9",
			wantRaw:  "DELETE_PATH",
		},
		{
			name: "RestTemplate local variable",
			body: `import org.springframework.web.client.RestTemplate;
class JobClient {
  private final RestTemplate restTemplate;
  void deleteRelatedJobs() {
    String path = "/job-management/catalogs/7/items/9";
    restTemplate.delete(path);
  }
}`,
			wantPath: "/job-management/catalogs/7/items/9",
			wantRaw:  "path",
		},
		{
			name: "local RestClient receiver",
			body: `import org.springframework.web.client.RestClient;
class JobClient {
  void deleteRelatedJobs() {
    RestClient client = RestClient.create();
    client
        .delete()
        .uri("/job-management/catalogs/7/items/9")
        .retrieve();
  }
}`,
			wantPath: "/job-management/catalogs/7/items/9",
			wantRaw:  `"/job-management/catalogs/7/items/9"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const file = "src/main/java/example/JobClient.java"
			source := extractJavaSource(FileRecord{Path: file, Language: "java"}, test.body)
			records := buildJavaAPIContracts([]JavaSourceRecord{source})
			if len(records) != 1 {
				t.Fatalf("contracts = %#v, want exactly one", records)
			}
			got := records[0]
			if got.Language != "java" || got.HTTPMethod != "DELETE" || got.Path != test.wantPath ||
				got.RawPath != test.wantRaw || got.Caller != "JobClient.deleteRelatedJobs" || got.File != file ||
				got.Confidence != "EXACT" || got.ConfidenceScore != 1 || got.UnsafeDynamic {
				t.Fatalf("unexpected contract: %#v", got)
			}
		})
	}
}

func TestBuildJavaAPIContractsInlineFluentClient(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestClient;
class JobClient {
  private final RestClient restClient;
  void deleteRelatedJobs() {
    restClient.delete().uri("/job-management/catalogs/7/items/9").retrieve();
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 || records[0].Path != "/job-management/catalogs/7/items/9" || records[0].Line != 5 {
		t.Fatalf("inline fluent contracts = %#v, want one exact call", records)
	}
}

func TestBuildJavaAPIContractsDefaultAuthorizationHeader(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestClient;
class JobClient {
  private final RestClient restClient;
  void deleteRelatedJobs() {
    restClient.defaultHeader("Authorization", "private-token");
    restClient.delete("/job-management/catalogs/7/items/9");
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	want := []AuthRecord{{Kind: "basic", Source: "spring_client_interceptor", Confidence: "EXTRACTED"}}
	if len(records) != 1 || !reflect.DeepEqual(records[0].Auth, want) {
		t.Fatalf("default-header contracts = %#v, want categorical auth %#v", records, want)
	}
}

func TestBuildJavaAPIContractsIgnoresNonAuthorizationDefaultHeader(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestClient;
class JobClient {
  private final RestClient restClient;
  void deleteRelatedJobs() {
    restClient.defaultHeader("Content-Type", "application/json");
    restClient.delete("/job-management/catalogs/7/items/9");
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 || len(records[0].Auth) != 0 {
		t.Fatalf("non-authorization header produced auth: %#v", records)
	}
}

func TestBuildJavaAPIContractsResolvesUniquePathGetter(t *testing.T) {
	const file = "src/main/java/example/JobClient.java"
	source := extractJavaSource(FileRecord{Path: file, Language: "java"}, `import org.springframework.web.client.RestClient;
class JobClient {
  private final RestClient restClient;
  private final PathProvider pathProvider;
  void deleteRelatedJobs(String catalogId, String itemId) {
    restClient
        .delete()
        .uri(pathProvider.deleteRelatedJobsPath())
        .retrieve();
  }
}
class PathProvider {
  String deleteRelatedJobsPath() {
    return "/job-management/catalogs/" + catalogId + "/items/" + itemId;
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	want := APIContractRecord{
		Language: "java", HTTPMethod: "DELETE",
		Path:    "/job-management/catalogs/{dynamic}/items/{dynamic}",
		RawPath: "pathProvider.deleteRelatedJobsPath()",
		Caller:  "JobClient.deleteRelatedJobs", File: file, Line: 7,
		Confidence: "RESOLVED", ConfidenceScore: 0.9,
		Reason: "spring RestClient receiver with statically resolved path getter",
	}
	if len(records) != 1 || !reflect.DeepEqual(records[0], want) {
		t.Fatalf("contracts = %#v, want %#v", records, want)
	}
}

func TestBuildJavaAPIContractsBindsPathGetterToReceiverType(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestClient;
class JobClient {
  private final RestClient restClient;
  private final PathProvider pathProvider;
  void deleteRelatedJobs() {
    restClient.delete().uri(pathProvider.deletePath()).retrieve();
  }
}
class PathProvider {
  String deletePath() {
    return "/job-management/items/9";
  }
}
class AlternatePathProvider {
  String deletePath() {
    return "/other/items/9";
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 || records[0].Path != "/job-management/items/9" || records[0].Confidence != "RESOLVED" {
		t.Fatalf("typed getter contracts = %#v", records)
	}
}

func TestBuildSpringIndexResolvesQualifiedCrossFileConstants(t *testing.T) {
	routes := extractJavaSource(FileRecord{Path: "src/main/java/example/Routes.java", Language: "java"}, `package example;
final class Routes {
  static final String BASE_PATH = "/job-management";
}`)
	controller := extractJavaSource(FileRecord{Path: "src/main/java/example/JobController.java", Language: "java"}, `package example;
@RestController
@RequestMapping(Routes.BASE_PATH)
final class JobController {
  @GetMapping("/jobs")
  List<Job> listJobs() { return List.of(); }
}`)

	index := buildSpringIndex([]JavaSourceRecord{controller, routes})
	endpoint, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/job-management/jobs")
	if !ok || endpoint.Controller != "JobController" {
		t.Fatalf("qualified cross-file route was not resolved: %#v", index.Endpoints)
	}
}

func TestSpringConstantIndexOmitsAmbiguousSimpleAliases(t *testing.T) {
	first := extractJavaSource(FileRecord{Path: "src/main/java/one/Routes.java", Language: "java"}, `package one;
final class Routes {
  static final String BASE_PATH = "/one";
}`)
	second := extractJavaSource(FileRecord{Path: "src/main/java/two/Paths.java", Language: "java"}, `package two;
final class Paths {
  static final String BASE_PATH = "/two";
}`)

	constants := springConstantIndex([]JavaSourceRecord{second, first})
	if _, exists := constants["BASE_PATH"]; exists {
		t.Fatalf("ambiguous simple constant alias was retained: %#v", constants)
	}
	if constants["Routes.BASE_PATH"] != "/one" || constants["one.Routes.BASE_PATH"] != "/one" ||
		constants["Paths.BASE_PATH"] != "/two" || constants["two.Paths.BASE_PATH"] != "/two" {
		t.Fatalf("qualified constant aliases = %#v", constants)
	}
}

func TestJavaImperativeContractResolvesConfigurationGetterBaseURL(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `package example;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.web.client.RestClient;

@ConfigurationProperties("jobs")
final class JobClientConfig {
  String baseUrl;
  static final String BASE_PATH = "/job-management";
  static final String ALL_JOBS = "/jobs";

  String getAllJobsPath() {
    return baseUrl + BASE_PATH + ALL_JOBS;
  }
}

final class JobClient {
  private final JobClientConfig config;
  private final RestClient restClient;

  void list() {
    restClient.get().uri(config.getAllJobsPath()).retrieve();
  }
}`)

	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 || records[0].Path != "/job-management/jobs" ||
		records[0].Query != "" || records[0].Confidence != "RESOLVED" {
		t.Fatalf("configuration getter contracts = %#v", records)
	}
}

func TestJavaImperativeContractResolvesBoundedTernaryLocal(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `package example;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.web.client.RestClient;

@ConfigurationProperties("jobs")
final class JobClientConfig {
  String baseUrl;
  static final String BASE_PATH = "/job-management";
  static final String ALL_JOBS = "/jobs";
  static final String ACTIVE_JOBS = "/jobs?status=active";

  String getAllJobsPath() {
    return baseUrl + BASE_PATH + ALL_JOBS;
  }

  String getActiveJobsPath() {
    return baseUrl + BASE_PATH + ACTIVE_JOBS;
  }
}

final class JobClient {
  private final JobClientConfig config;
  private final RestClient restClient;

  void list(String status) {
    String url = status.isEmpty()
        ? config.getAllJobsPath()
        : config.getActiveJobsPath();
    restClient.get().uri(url).retrieve();
  }
}`)

	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 2 {
		t.Fatalf("bounded ternary contracts = %#v, want two", records)
	}
	wantQueries := map[string]bool{"": true, "status=active": true}
	for _, record := range records {
		if record.Path != "/job-management/jobs" || record.Confidence != "RESOLVED" ||
			!wantQueries[record.Query] {
			t.Fatalf("bounded ternary contract = %#v", record)
		}
		delete(wantQueries, record.Query)
	}
	if len(wantQueries) != 0 {
		t.Fatalf("missing ternary query alternatives: %#v", wantQueries)
	}
}

func TestJavaImperativeContractResolvesDistinctTernaryPaths(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestClient;
final class JobClient {
  private final RestClient restClient;
  void list(boolean jobs) {
    String url = jobs ? "/jobs" : "/tasks";
    restClient.get().uri(url).retrieve();
  }
}`)

	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 2 || records[0].Path != "/jobs" || records[1].Path != "/tasks" {
		t.Fatalf("distinct ternary paths = %#v", records)
	}
}

func TestJavaImperativeContractRejectsMoreThanFourTernaryAlternatives(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestClient;
final class JobClient {
  private final RestClient restClient;
  void list(boolean one, boolean two, boolean three, boolean four) {
    String url = one ? "/one" : two ? "/two" : three ? "/three" : four ? "/four" : "/five";
    restClient.get().uri(url).retrieve();
  }
}`)

	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 || records[0].Path != "" ||
		records[0].Confidence != "PARTIAL" || !records[0].UnsafeDynamic {
		t.Fatalf("unbounded ternary was resolved: %#v", records)
	}
}

func TestBuildJavaAPIContractsResolvesConstantConcatenation(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestTemplate;
class JobClient {
  private static final String ROOT = "/job-management";
  private final RestTemplate restTemplate;
  void deleteRelatedJobs(String catalogId) {
    restTemplate.delete(ROOT + "/catalogs/" + catalogId + "/items/9");
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 || records[0].Path != "/job-management/catalogs/{dynamic}/items/9" {
		t.Fatalf("constant concatenation contracts = %#v", records)
	}
}

func TestBuildJavaAPIContractsDoesNotDropUnknownConcatenationPrefix(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestTemplate;
class JobClient {
  private final RestTemplate restTemplate;
  void deleteRelatedJobs(String UNKNOWN_BASE_PATH) {
    restTemplate.delete(UNKNOWN_BASE_PATH + "/items/9");
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 || records[0].Path != "" || !records[0].UnsafeDynamic || records[0].Confidence != "PARTIAL" {
		t.Fatalf("unknown prefix was treated as a base URL: %#v", records)
	}
}

func TestBuildJavaAPIContractsRequiresSpringImportForConfigurationBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		valueImport string
		declaration string
		parameter   string
		wantPath    string
		confidence  string
		unsafe      bool
	}{
		{name: "custom field", valueImport: "example.Value", declaration: "  @Value(\"${client.url}\")\n  private String arbitraryPrefix;", confidence: "PARTIAL", unsafe: true},
		{name: "custom parameter", valueImport: "example.Value", parameter: "@Value(\"${client.url}\") String arbitraryPrefix", confidence: "PARTIAL", unsafe: true},
		{name: "Spring field", valueImport: "org.springframework.beans.factory.annotation.Value", declaration: "  @Value(\"${client.url}\")\n  private String arbitraryPrefix;", wantPath: "/items/9", confidence: "RESOLVED"},
		{name: "Spring parameter", valueImport: "org.springframework.beans.factory.annotation.Value", parameter: "@Value(\"${client.url}\") String arbitraryPrefix", wantPath: "/items/9", confidence: "RESOLVED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := `import ` + test.valueImport + `;
import org.springframework.web.client.RestTemplate;
class JobClient {
  private final RestTemplate restTemplate;
` + test.declaration + `
  void deleteRelatedJobs(` + test.parameter + `) {
    restTemplate.delete(arbitraryPrefix + "/items/9");
  }
}`
			source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, body)
			records := buildJavaAPIContracts([]JavaSourceRecord{source})
			if len(records) != 1 || records[0].Path != test.wantPath || records[0].UnsafeDynamic != test.unsafe || records[0].Confidence != test.confidence {
				t.Fatalf("configuration annotation provenance = %#v", records)
			}
		})
	}
}

func TestExtractJavaHTTPCallConfidence(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestClient;
class JobClient {
  private static final String ITEM_PATH = "/items/10";
  private final RestClient restClient;
  void literalPath() {
    restClient.delete("/items/9");
  }
  void resolvedPath(String itemId) {
    restClient.delete("/items/" + itemId);
  }
  void constantPath() {
    restClient.delete(ITEM_PATH);
  }
  void partialPath(Request request) {
    restClient.delete(request.resolvePath());
  }
}`)
	want := map[string]string{
		"literalPath":  "EXACT",
		"resolvedPath": "RESOLVED",
		"constantPath": "RESOLVED",
		"partialPath":  "PARTIAL",
	}
	for _, method := range source.Methods {
		confidence, ok := want[method.Name]
		if !ok {
			continue
		}
		if len(method.HTTPRequests) != 1 || method.HTTPRequests[0].Confidence != confidence {
			t.Fatalf("%s HTTP requests = %#v, want confidence %s", method.Name, method.HTTPRequests, confidence)
		}
		delete(want, method.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing confidence cases: %#v", want)
	}
}

func TestBuildJavaAPIContractsResolvesLocalConcatenation(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestTemplate;
class JobClient {
  private static final String ROOT = "/job-management";
  private final RestTemplate restTemplate;
  void deleteRelatedJobs(String catalogId) {
    String path = ROOT + "/catalogs/" + catalogId + "/items/9";
    restTemplate.delete(path);
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 || records[0].Path != "/job-management/catalogs/{dynamic}/items/9" {
		t.Fatalf("local concatenation contracts = %#v", records)
	}
}

func TestBuildJavaAPIContractsPreservesUnresolvedDynamicExpression(t *testing.T) {
	const file = "src/main/java/example/JobClient.java"
	source := extractJavaSource(FileRecord{Path: file, Language: "java"}, `import org.springframework.web.client.RestClient;
class JobClient {
  private final RestClient restClient;
  void deleteRelatedJobs(Request request) {
    restClient
        .delete()
        .uri(request.resolvePath())
        .retrieve();
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 {
		t.Fatalf("contracts = %#v, want exactly one", records)
	}
	got := records[0]
	if got.Path != "" || got.RawPath != "request.resolvePath()" || !got.UnsafeDynamic || got.Confidence != "PARTIAL" || got.ConfidenceScore >= 0.9 {
		t.Fatalf("unresolved contract = %#v", got)
	}
}

func TestBuildJavaAPIContractsRejectsUnboundDeleteReceiver(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestClient;
class JobClient {
  private final AuditStore auditStore;
  void deleteRelatedJobs() {
    auditStore.delete("/job-management/catalogs/7/items/9");
  }
}`)
	if records := buildJavaAPIContracts([]JavaSourceRecord{source}); len(records) != 0 {
		t.Fatalf("unrelated receiver produced contracts: %#v", records)
	}
}

func TestBuildJavaAPIContractsSortsAndDeduplicatesCallSites(t *testing.T) {
	body := `import org.springframework.web.client.RestTemplate;
class JobClient {
  private final RestTemplate restTemplate;
  void deleteRelatedJobs() {
    restTemplate.delete("/job-management/catalogs/7/items/9");
  }
}`
	later := extractJavaSource(FileRecord{Path: "z/JobClient.java", Language: "java"}, body)
	earlier := extractJavaSource(FileRecord{Path: "a/JobClient.java", Language: "java"}, body)
	records := buildJavaAPIContracts([]JavaSourceRecord{later, earlier, later})
	if len(records) != 2 || records[0].File != "a/JobClient.java" || records[1].File != "z/JobClient.java" {
		t.Fatalf("sorted unique contracts = %#v", records)
	}
}

func TestBuildJavaAPIContractsAuthenticationRetryAndPrivacy(t *testing.T) {
	const file = "src/main/java/example/JobClient.java"
	source := extractJavaSource(FileRecord{Path: file, Language: "java"}, `package example;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.retry.annotation.Retryable;
import org.springframework.web.client.RestClient;
import org.springframework.http.client.support.BasicAuthenticationInterceptor;
class JobClient {
  @Value("${jobs.url}")
  private String baseUrl;
  private final RestClient restClient;
  @Retryable
  void deleteRelatedJobs(String catalogId, String itemId) {
    String credentialUser = "private-user";
    String credentialPassword = "private-password";
    new BasicAuthenticationInterceptor(credentialUser, credentialPassword);
    restClient.defaultHeader("Authorization", "Basic private-token");
    restClient.delete(baseUrl + "/job-management/catalogs/" + catalogId + "/items/" + itemId);
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	if len(records) != 1 {
		t.Fatalf("contracts = %#v, want exactly one", records)
	}
	wantAuth := []AuthRecord{{Kind: "basic", Source: "spring_client_interceptor", Confidence: "EXTRACTED"}}
	if !reflect.DeepEqual(records[0].Auth, wantAuth) {
		t.Fatalf("auth = %#v, want %#v", records[0].Auth, wantAuth)
	}
	if records[0].Path != "/job-management/catalogs/{dynamic}/items/{dynamic}" || !strings.Contains(records[0].Reason, "retryable") {
		t.Fatalf("path/retry provenance missing: %#v", records[0])
	}
	marshaled, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-user", "private-password", "private-token", "${jobs.url}"} {
		if strings.Contains(string(marshaled), secret) {
			t.Fatalf("marshaled contracts leaked %q: %s", secret, marshaled)
		}
	}
}

func TestBuildJavaAPIContractsFindsBareBasicAuthenticationInterceptor(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/JobClient.java", Language: "java"}, `import org.springframework.web.client.RestClient;
import org.springframework.http.client.support.BasicAuthenticationInterceptor;
class JobClient {
  private final RestClient restClient;
  void deleteRelatedJobs() {
    new BasicAuthenticationInterceptor("private-user", "private-password");
    restClient.delete("/job-management/items/9");
  }
}`)
	records := buildJavaAPIContracts([]JavaSourceRecord{source})
	want := []AuthRecord{{Kind: "basic", Source: "spring_client_interceptor", Confidence: "EXTRACTED"}}
	if len(records) != 1 || !reflect.DeepEqual(records[0].Auth, want) {
		t.Fatalf("bare interceptor contracts = %#v, want auth %#v", records, want)
	}
	marshaled, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(marshaled), "private-user") || strings.Contains(string(marshaled), "private-password") {
		t.Fatalf("bare interceptor leaked credentials: %s", marshaled)
	}
}
