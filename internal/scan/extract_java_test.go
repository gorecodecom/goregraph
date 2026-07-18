package scan

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractJavaSecurityEvidenceCalls(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/Security.java", Language: "java"}, `class Security {
  void configure() {
    routes.permitAll();
    routes.authenticated();
    routes.hasRole("ADMIN");
    routes.hasAnyRole("ADMIN", "USER");
    routes.hasAuthority("orders:read");
    routes.hasAnyAuthority("orders:read", "orders:write");
    http.httpBasic();
    http.oauth2ResourceServer();
    http.oauth2Login();
    http.formLogin();
    http.x509();
  }
}`)

	want := []string{
		"permit_all", "authenticated", "has_role", "has_any_role", "has_authority", "has_any_authority",
		"http_basic", "oauth2_resource_server", "oauth2_login", "form_login", "x509",
	}
	if len(source.Methods) != 1 {
		t.Fatalf("methods=%#v", source.Methods)
	}
	for _, kind := range want {
		if !hasAuthKind(source.Methods[0].Auth, kind) {
			t.Fatalf("missing %q in auth=%#v", kind, source.Methods[0].Auth)
		}
	}
}

func TestSpringIndexExtractsMethodAnnotationAuth(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/Controller.java", Language: "java"}, `@RestController
class Controller {
  @PermitAll
  @GetMapping("/public")
  String publicEndpoint() { return "ok"; }

  @DenyAll
  @GetMapping("/denied")
  String denied() { return "no"; }

  @PreAuthorize("hasRole('ADMIN')")
  @GetMapping("/pre")
  String pre() { return "ok"; }

  @PostAuthorize("hasAuthority('orders:read')")
  @GetMapping("/post")
  String post() { return "ok"; }

  @Secured({"ROLE_ADMIN", "ROLE_USER"})
  @GetMapping("/secured")
  String secured() { return "ok"; }

  @RolesAllowed("ADMIN")
  @GetMapping("/roles")
  String roles() { return "ok"; }
}`)

	index := buildSpringIndex([]JavaSourceRecord{source})
	want := map[string]string{
		"/public":  "permit_all",
		"/denied":  "deny_all",
		"/pre":     "pre_authorize",
		"/post":    "post_authorize",
		"/secured": "secured",
		"/roles":   "roles_allowed",
	}
	for path, kind := range want {
		endpoint, ok := findSpringEndpointForTest(index.Endpoints, "GET", path)
		if !ok || !hasAuthKind(endpoint.Auth, kind) {
			t.Fatalf("path=%q kind=%q endpoint=%#v endpoints=%#v", path, kind, endpoint, index.Endpoints)
		}
	}
}

func TestSpringIndexInheritsSupportedClassMethodSecurityAnnotations(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/Controllers.java", Language: "java"}, `@RestController
@PreAuthorize("hasRole('ADMIN')")
class PreAuthorizedController {
  @GetMapping("/pre-authorized")
  String get() { return "ok"; }
}

@RestController
@Secured({"ROLE_ADMIN", "ROLE_USER"})
class SecuredController {
  @GetMapping("/secured")
  String get() { return "ok"; }
}

@RestController
@RolesAllowed("ADMIN")
class RolesAllowedController {
  @GetMapping("/roles-allowed")
  String get() { return "ok"; }
}

@RestController
@PermitAll
class PublicController {
  @GetMapping("/public")
  String get() { return "ok"; }
}`)

	index := buildSpringIndex([]JavaSourceRecord{source})
	want := map[string]string{
		"/pre-authorized": "pre_authorize",
		"/secured":        "secured",
		"/roles-allowed":  "roles_allowed",
		"/public":         "permit_all",
	}
	for path, kind := range want {
		endpoint, ok := findSpringEndpointForTest(index.Endpoints, "GET", path)
		if !ok || len(endpoint.Auth) != 1 || endpoint.Auth[0].Kind != kind || endpoint.Auth[0].Source != "class_annotation" {
			t.Fatalf("path=%q kind=%q endpoint=%#v endpoints=%#v", path, kind, endpoint, index.Endpoints)
		}
	}
}

func TestSpringIndexKeepsMethodSecurityOverridesIndependentFromOpenAPI(t *testing.T) {
	definitions := extractJavaSource(FileRecord{Path: "src/main/java/OpenAPIConfig.java", Language: "java"}, `
@SecurityScheme(name = "bearerAuth", type = SecuritySchemeType.HTTP, scheme = "bearer")
class OpenAPIConfig {}
`)
	controller := extractJavaSource(FileRecord{Path: "src/main/java/Controller.java", Language: "java"}, `@RestController
@PreAuthorize("hasRole('ADMIN')")
@SecurityRequirement(name = "bearerAuth")
class Controller {
  @GetMapping("/inherited")
  String inherited() { return "ok"; }

  @PermitAll
  @GetMapping("/method-public")
  String methodPublic() { return "ok"; }

  @Operation(security = {})
  @GetMapping("/openapi-public")
  String openAPIPublic() { return "ok"; }
}`)

	index := buildSpringIndex([]JavaSourceRecord{definitions, controller})
	inherited, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/inherited")
	if !ok || !hasAuthKind(inherited.Auth, "pre_authorize") || !hasAuthKind(inherited.Auth, "bearer") {
		t.Fatalf("class method/OpenAPI security not inherited independently: %#v", inherited)
	}
	methodPublic, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/method-public")
	if !ok || !hasAuthKind(methodPublic.Auth, "permit_all") || hasAuthKind(methodPublic.Auth, "pre_authorize") || !hasAuthKind(methodPublic.Auth, "bearer") {
		t.Fatalf("method security override affected the wrong security model: %#v", methodPublic)
	}
	openAPIPublic, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/openapi-public")
	if !ok || !hasAuthKind(openAPIPublic.Auth, "pre_authorize") || !hasAuthKind(openAPIPublic.Auth, "permit_all") || hasAuthKind(openAPIPublic.Auth, "bearer") {
		t.Fatalf("OpenAPI security override affected method security inheritance: %#v", openAPIPublic)
	}
}

func TestClassMethodSecurityReachesCatalogDashboardAndAgentContext(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/OrderController.java", Language: "java"}, `@RestController
@RolesAllowed("ADMIN")
class OrderController {
  @GetMapping("/orders/{id}")
  String getOrder() { return "ok"; }
}`)
	index := buildSpringIndex([]JavaSourceRecord{source})
	catalog := BuildProjectAPICatalog("services/orders", "fixed", nil, index, nil, nil)
	if len(catalog.Endpoints) != 1 || len(catalog.Endpoints[0].Security) != 1 {
		t.Fatalf("catalog security evidence missing: %#v", catalog.Endpoints)
	}
	security := catalog.Endpoints[0].Security[0]
	if security.Kind != SecurityRole || security.Source != "class_annotation" || security.File != source.File || security.Line == 0 {
		t.Fatalf("class security provenance not preserved in catalog: %#v", security)
	}

	html := renderWorkspaceDashboardHTML(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, catalog,
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion}, WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion}, nil,
	)
	for _, want := range []string{`"kind":"role"`, `"source":"class_annotation"`, `/orders/{id}`} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard payload missing class security evidence %q", want)
		}
	}

	agentIndex := BuildWorkspaceAgentContextIndex(
		WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{Path: "services/orders", Indexed: true}}},
		nil, nil, nil, WorkspaceEndpointTraceIndexRecord{}, catalog, "fixed",
	)
	endpointFact := findContextFact(agentIndex.Facts, "api_endpoint", "GET /orders/{id}")
	securityFact := findContextFact(agentIndex.Facts, "endpoint_security", SecurityRole)
	if endpointFact.ID == "" || securityFact.ID == "" || !hasContextEdge(agentIndex.Edges, endpointFact.ID, securityFact.ID, "requires_auth") {
		t.Fatalf("class security evidence missing from agent context: facts=%#v edges=%#v", agentIndex.Facts, agentIndex.Edges)
	}
}

func TestSpringIndexExtractsExplicitOpenAPISecurityEvidence(t *testing.T) {
	definitions := extractJavaSource(FileRecord{Path: "src/OpenAPIConfig.java", Language: "java"}, `
@SecurityScheme(name = "apiKeyAuth", type = SecuritySchemeType.APIKEY, in = SecuritySchemeIn.HEADER)
@SecurityScheme(name = "bearerAuth", type = SecuritySchemeType.HTTP, scheme = "bearer")
class OpenAPIConfig {}
`)
	controller := extractJavaSource(FileRecord{Path: "src/Controller.java", Language: "java"}, `@RestController
class Controller {
  @SecurityRequirement(name = "apiKeyAuth")
  @GetMapping("/api-key")
  String apiKey() { return "ok"; }

  @Operation(security = @SecurityRequirement(name = "bearerAuth"))
  @GetMapping("/bearer")
  String bearer() { return "ok"; }

  @SecurityRequirement(name = "undefinedAuth")
  @GetMapping("/undefined")
  String undefined() { return "ok"; }
}`)

	index := buildSpringIndex([]JavaSourceRecord{definitions, controller})
	apiKey, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/api-key")
	if !ok || !hasAuthKind(apiKey.Auth, "api_key") {
		t.Fatalf("api-key endpoint=%#v", apiKey)
	}
	bearer, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/bearer")
	if !ok || !hasAuthKind(bearer.Auth, "bearer") {
		t.Fatalf("bearer endpoint=%#v", bearer)
	}
	undefined, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/undefined")
	if !ok || len(undefined.Auth) != 0 {
		t.Fatalf("undefined scheme must not become auth evidence: %#v", undefined)
	}
}

func TestSpringIndexRetainsConflictingMethodAndGlobalSecurityAuth(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/Application.java", Language: "java"}, `import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.web.SecurityFilterChain;

@RestController
class Controller {
  @PermitAll
  @GetMapping("/health")
  String health() { return "ok"; }
}

@Configuration
class Security {
	SecurityFilterChain configure(HttpSecurity http) {
    routes.authenticated();
	return null;
  }
}`)

	index := buildSpringIndex([]JavaSourceRecord{source})
	endpoint, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/health")
	if !ok || !hasAuthKind(endpoint.Auth, "permit_all") || !hasAuthKind(endpoint.Auth, "authenticated") {
		t.Fatalf("conflicting method/global auth not retained: %#v", endpoint)
	}
}

func TestSpringGlobalSecurityAuthRequiresProductionSpringSecurityMethodContext(t *testing.T) {
	ordinary := extractJavaSource(FileRecord{Path: "src/main/java/OrderService.java", Language: "java"}, `class OrderService {
  void update() {
    routes.authenticated();
  }
}`)
	impostor := extractJavaSource(FileRecord{Path: "src/main/java/ImpostorSecurity.java", Language: "java"}, `import org.springframework.security.access.prepost.PreAuthorize;
class ImpostorSecurity {
  void configure(HttpSecurity http) {
    routes.oauth2Login();
  }
}`)
	testConfig := extractJavaSource(FileRecord{Path: "src/test/java/SecurityTest.java", Language: "java"}, `import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.web.SecurityFilterChain;
class SecurityTest {
  SecurityFilterChain configure(HttpSecurity http) {
    routes.permitAll();
    return null;
  }
}`)
	securityConfig := extractJavaSource(FileRecord{Path: "src/main/java/SecurityConfig.java", Language: "java"}, `import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.web.SecurityFilterChain;
class SecurityConfig {
  SecurityFilterChain configure(HttpSecurity http) {
    routes.hasRole("ADMIN");
    return null;
  }
}`)

	records := springGlobalAuthRecords([]JavaSourceRecord{ordinary, impostor, testConfig, securityConfig})
	if len(records) != 1 || records[0].Kind != "has_role" || records[0].File != "src/main/java/SecurityConfig.java" {
		t.Fatalf("global auth=%#v", records)
	}
}

func TestSpringGlobalSecurityAuthRetainsFullProvenanceInCanonicalOrder(t *testing.T) {
	first := extractJavaSource(FileRecord{Path: "src/main/java/AConfig.java", Language: "java"}, `import org.springframework.security.config.annotation.web.builders.HttpSecurity;
class AConfig {
  void configure(HttpSecurity http) {
    routes.authenticated();
  }
}`)
	second := extractJavaSource(FileRecord{Path: "src/main/java/BConfig.java", Language: "java"}, `import org.springframework.security.web.SecurityFilterChain;
class BConfig {
  SecurityFilterChain configure() {
    routes.authenticated();
    return null;
  }
}`)

	left := springGlobalAuthRecords([]JavaSourceRecord{second, first})
	right := springGlobalAuthRecords([]JavaSourceRecord{first, second})
	if len(left) != 2 || !reflect.DeepEqual(left, right) {
		t.Fatalf("global auth is lossy or order-dependent: left=%#v right=%#v", left, right)
	}
	if left[0].File != "src/main/java/AConfig.java" || left[1].File != "src/main/java/BConfig.java" {
		t.Fatalf("global auth not canonically ordered: %#v", left)
	}
}

func TestSpringIndexAppliesClassOpenAPISecurityWithMethodOverrideSemantics(t *testing.T) {
	definitions := extractJavaSource(FileRecord{Path: "src/OpenAPIConfig.java", Language: "java"}, `
@SecurityScheme(name = "apiKeyAuth", type = SecuritySchemeType.APIKEY, in = SecuritySchemeIn.HEADER)
@SecurityScheme(name = "bearerAuth", type = SecuritySchemeType.HTTP, scheme = "bearer")
class OpenAPIConfig {}
`)
	controller := extractJavaSource(FileRecord{Path: "src/Controller.java", Language: "java"}, `@RestController
@SecurityRequirement(name = "bearerAuth")
class Controller {
  @GetMapping("/inherited")
  String inherited() { return "ok"; }

  @SecurityRequirement(name = "apiKeyAuth")
  @GetMapping("/method")
  String methodOverride() { return "ok"; }

  @Operation(security = {})
  @GetMapping("/public")
  String publicOverride() { return "ok"; }
}`)

	index := buildSpringIndex([]JavaSourceRecord{definitions, controller})
	inherited, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/inherited")
	if !ok || !hasAuthKind(inherited.Auth, "bearer") {
		t.Fatalf("class security not inherited: %#v", inherited)
	}
	method, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/method")
	if !ok || !hasAuthKind(method.Auth, "api_key") || hasAuthKind(method.Auth, "bearer") {
		t.Fatalf("method security did not replace class security: %#v", method)
	}
	public, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/public")
	if !ok || !hasAuthKind(public.Auth, "permit_all") || hasAuthKind(public.Auth, "bearer") {
		t.Fatalf("empty method security override not retained honestly: %#v", public)
	}
}

func TestSpringIndexRetainsDuplicateOpenAPISchemeDefinitionsCanonically(t *testing.T) {
	basic := extractJavaSource(FileRecord{Path: "src/BasicScheme.java", Language: "java"}, `@SecurityScheme(name = "sharedAuth", type = SecuritySchemeType.HTTP, scheme = "basic")
class BasicScheme {}`)
	bearer := extractJavaSource(FileRecord{Path: "src/BearerScheme.java", Language: "java"}, `@SecurityScheme(name = "sharedAuth", type = SecuritySchemeType.HTTP, scheme = "bearer")
class BearerScheme {}`)
	controller := extractJavaSource(FileRecord{Path: "src/Controller.java", Language: "java"}, `@RestController
class Controller {
  @SecurityRequirement(name = "sharedAuth")
  @GetMapping("/shared")
  String shared() { return "ok"; }
}`)

	left := buildSpringIndex([]JavaSourceRecord{bearer, controller, basic})
	right := buildSpringIndex([]JavaSourceRecord{basic, controller, bearer})
	leftEndpoint, leftOK := findSpringEndpointForTest(left.Endpoints, "GET", "/shared")
	rightEndpoint, rightOK := findSpringEndpointForTest(right.Endpoints, "GET", "/shared")
	if !leftOK || !rightOK || len(leftEndpoint.Auth) != 2 || !reflect.DeepEqual(leftEndpoint.Auth, rightEndpoint.Auth) {
		t.Fatalf("duplicate schemes lost or order-dependent: left=%#v right=%#v", leftEndpoint.Auth, rightEndpoint.Auth)
	}
	if !hasAuthKind(leftEndpoint.Auth, "http_basic") || !hasAuthKind(leftEndpoint.Auth, "bearer") {
		t.Fatalf("duplicate scheme kinds not retained: %#v", leftEndpoint.Auth)
	}
}

func TestSpringIndexPluralOpenAPISecurityRequirementsOverrideClassSecurity(t *testing.T) {
	definitions := extractJavaSource(FileRecord{Path: "src/OpenAPIConfig.java", Language: "java"}, `
@SecurityScheme(name = "apiKeyAuth", type = SecuritySchemeType.APIKEY, in = SecuritySchemeIn.HEADER)
@SecurityScheme(name = "bearerAuth", type = SecuritySchemeType.HTTP, scheme = "bearer")
class OpenAPIConfig {}
`)
	controller := extractJavaSource(FileRecord{Path: "src/Controller.java", Language: "java"}, `@RestController
@SecurityRequirement(name = "bearerAuth")
class Controller {
  @SecurityRequirements({
      @SecurityRequirement(scopes = {"@SecurityRequirement(name = \"bearerAuth\")"}, name = "apiKeyAuth")
  })
  @GetMapping("/method")
  String methodOverride() { return "ok"; }
}`)

	index := buildSpringIndex([]JavaSourceRecord{definitions, controller})
	method, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/method")
	if !ok || !hasAuthKind(method.Auth, "api_key") || hasAuthKind(method.Auth, "bearer") {
		t.Fatalf("plural method security did not replace class security: %#v", method)
	}
}

func TestSpringIndexNamedEmptyOpenAPISecurityRequirementsOverrideIsPublic(t *testing.T) {
	definitions := extractJavaSource(FileRecord{Path: "src/OpenAPIConfig.java", Language: "java"}, `
@SecurityScheme(name = "bearerAuth", type = SecuritySchemeType.HTTP, scheme = "bearer")
class OpenAPIConfig {}
`)
	controller := extractJavaSource(FileRecord{Path: "src/Controller.java", Language: "java"}, `@RestController
@SecurityRequirement(name = "bearerAuth")
class Controller {
  @SecurityRequirements(value = {})
  @GetMapping("/public")
  String publicOverride() { return "ok"; }
}`)

	index := buildSpringIndex([]JavaSourceRecord{definitions, controller})
	endpoint, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/public")
	if !ok || !hasAuthKind(endpoint.Auth, "permit_all") || hasAuthKind(endpoint.Auth, "bearer") {
		t.Fatalf("named empty plural override not retained as explicit public: %#v", endpoint)
	}
	catalog := BuildProjectAPICatalog("orders", "fixed", nil, index, nil, nil)
	if len(catalog.Endpoints) != 1 || len(catalog.Endpoints[0].Security) != 1 || catalog.Endpoints[0].Security[0].Kind != SecurityPublic {
		t.Fatalf("named empty plural override did not normalize to public: %#v", catalog.Endpoints)
	}
}

func hasAuthKind(records []AuthRecord, kind string) bool {
	for _, record := range records {
		if record.Kind == kind {
			return true
		}
	}
	return false
}
