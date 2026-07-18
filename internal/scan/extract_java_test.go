package scan

import "testing"

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
	source := extractJavaSource(FileRecord{Path: "src/Application.java", Language: "java"}, `@RestController
class Controller {
  @PermitAll
  @GetMapping("/health")
  String health() { return "ok"; }
}

@Configuration
class Security {
  void configure() {
    routes.authenticated();
  }
}`)

	index := buildSpringIndex([]JavaSourceRecord{source})
	endpoint, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/health")
	if !ok || !hasAuthKind(endpoint.Auth, "permit_all") || !hasAuthKind(endpoint.Auth, "authenticated") {
		t.Fatalf("conflicting method/global auth not retained: %#v", endpoint)
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
