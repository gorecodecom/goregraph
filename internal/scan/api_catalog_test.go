package scan

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestAPICatalogStableIdentityIgnoresDiscoveryOrder(t *testing.T) {
	left := StableAPIEndpointID("services/orders", "http", "GET", "/orders/{id}", "OrderController.get", "src/main/java/OrderController.java", 24)
	right := StableAPIEndpointID("services/orders", "http", "get", "/orders/{orderId}", "OrderController.get", "src/main/java/OrderController.java", 24)
	if left != right {
		t.Fatalf("equivalent route IDs differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "endpoint:") {
		t.Fatalf("stable endpoint ID %q has no endpoint prefix", left)
	}
}

func TestStableAPIEndpointIDNormalizesOnlyPathParameterNames(t *testing.T) {
	base := StableAPIEndpointID("services/orders", "http", "GET", "/Orders/{id}/lines/{lineId}", "OrderController.get", "src/OrderController.java", 24)
	equivalent := StableAPIEndpointID("services/orders", "http", "get", "/Orders/{orderId}/lines/{position}", "OrderController.get", "src/OrderController.java", 24)
	if base != equivalent {
		t.Fatalf("path parameter names changed identity: %q != %q", base, equivalent)
	}

	different := []string{
		StableAPIEndpointID("services/billing", "http", "GET", "/Orders/{id}/lines/{lineId}", "OrderController.get", "src/OrderController.java", 24),
		StableAPIEndpointID("services/orders", "grpc", "GET", "/Orders/{id}/lines/{lineId}", "OrderController.get", "src/OrderController.java", 24),
		StableAPIEndpointID("services/orders", "http", "POST", "/Orders/{id}/lines/{lineId}", "OrderController.get", "src/OrderController.java", 24),
		StableAPIEndpointID("services/orders", "http", "GET", "/orders/{id}/lines/{lineId}", "OrderController.get", "src/OrderController.java", 24),
		StableAPIEndpointID("services/orders", "http", "GET", "/Orders/{id}/items/{lineId}", "OrderController.get", "src/OrderController.java", 24),
		StableAPIEndpointID("services/orders", "http", "GET", "/Orders/{id}/lines/{lineId}", "OrderController.list", "src/OrderController.java", 24),
		StableAPIEndpointID("services/orders", "http", "GET", "/Orders/{id}/lines/{lineId}", "OrderController.get", "src/OtherController.java", 24),
		StableAPIEndpointID("services/orders", "http", "GET", "/Orders/{id}/lines/{lineId}", "OrderController.get", "src/OrderController.java", 25),
	}
	for index, candidate := range different {
		if candidate == base {
			t.Fatalf("identity input %d did not affect endpoint ID %q", index, base)
		}
	}

	constrainedLeft := StableAPIEndpointID("services/orders", "http", "GET", `/orders/{id:[0-9]{2}}`, "OrderController.get", "src/OrderController.java", 24)
	constrainedRight := StableAPIEndpointID("services/orders", "http", "GET", `/orders/{orderId:[0-9]{2}}`, "OrderController.get", "src/OrderController.java", 24)
	if constrainedLeft != constrainedRight {
		t.Fatalf("constrained path parameter names changed identity: %q != %q", constrainedLeft, constrainedRight)
	}
	constrainedDifferent := StableAPIEndpointID("services/orders", "http", "GET", `/orders/{id:[a-z]{2}}`, "OrderController.get", "src/OrderController.java", 24)
	if constrainedLeft == constrainedDifferent {
		t.Fatalf("path parameter constraint did not affect endpoint ID %q", constrainedLeft)
	}
}

func TestStableAPIEndpointIDPreservesLiteralPathWhitespace(t *testing.T) {
	base := StableAPIEndpointID("services/orders", "http", "GET", "/orders/{id}", "OrderController.get", "src/OrderController.java", 24)
	for _, routePath := range []string{" /orders/{id}", "/orders/{id} "} {
		candidate := StableAPIEndpointID("services/orders", "http", "GET", routePath, "OrderController.get", "src/OrderController.java", 24)
		if candidate == base {
			t.Fatalf("literal route path whitespace %q did not affect endpoint ID %q", routePath, base)
		}
	}
}

func TestAPICatalogSecurityConstantsUseNormalizedValues(t *testing.T) {
	got := []string{SecurityBasic, SecurityBearer, SecurityOAuth2, SecurityAPIKey, SecuritySession, SecurityMTLS, SecurityRole, SecurityAuthenticated, SecurityPublic, SecurityUnknown}
	want := []string{"basic", "bearer", "oauth2", "api_key", "session", "mtls", "role", "authenticated", "public", "unknown"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("security constant %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestAPICatalogRecordJSONFieldOrder(t *testing.T) {
	record := APICatalogRecord{
		SchemaVersion: 1,
		Generated:     "fixed",
		Root:          "workspace",
		Endpoints: []APIEndpointRecord{{
			ID: "endpoint:1", ProviderProject: "services/orders", ProviderService: "orders", ProviderRole: "backend",
			Transport: "http", HTTPMethod: "POST", Path: "/orders/{id}", RawPath: "/orders/{orderId}", Language: "java", Framework: "spring",
			Controller: "OrderController", Handler: "OrderController.create", File: "src/OrderController.java", Line: 12,
			Parameters:   []APIParameterRecord{{Name: "id", Location: "path", Type: "string", Required: true, Source: "annotation", Confidence: ConfidenceExact}},
			Consumes:     []string{"application/json"},
			Produces:     []string{"application/problem+json"},
			RequestType:  "CreateOrder",
			ResponseType: "Order",
			Security: []SecurityEvidenceRecord{{
				Kind: SecurityBearer, Summary: "Bearer token", Expression: "authenticated()", Source: "security_config", File: "src/Security.java", Line: 8,
				Confidence: ConfidenceExact, Conflicting: true, Limitations: []string{"path scoped"}, EvidenceIDs: []string{"evidence:security"},
			}},
			Consumers: []APIConsumerRecord{{
				ID: "consumer:1", Project: "frontend/web", Service: "web", Role: "frontend", Caller: "createOrder", File: "src/api.ts", Line: 20,
				HTTPMethod: "POST", Path: "/orders/42", CallAuth: []SecurityEvidenceRecord{{Kind: SecurityBearer, Summary: "Authorization header", Confidence: ConfidenceExact}},
				Resolution: "MATCHED", Confidence: ConfidenceResolved, Limitations: []string{"static only"}, EvidenceIDs: []string{"evidence:consumer"},
			}},
			Mismatches: []APIMismatchRecord{{ID: "mismatch:1", Kind: "auth_conflict", Severity: "WARNING", Reason: "conflict", Confidence: ConfidenceInferred, EvidenceIDs: []string{"evidence:mismatch"}}},
			Confidence: ConfidenceExact, Coverage: CoverageComplete, Limitations: []string{"static analysis"}, EvidenceIDs: []string{"evidence:endpoint"},
		}},
	}

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":1,"generated":"fixed","root":"workspace","endpoints":[{"id":"endpoint:1","provider_project":"services/orders","provider_service":"orders","provider_role":"backend","transport":"http","http_method":"POST","path":"/orders/{id}","raw_path":"/orders/{orderId}","language":"java","framework":"spring","controller":"OrderController","handler":"OrderController.create","file":"src/OrderController.java","line":12,"parameters":[{"name":"id","location":"path","type":"string","required":true,"source":"annotation","confidence":"EXACT"}],"consumes":["application/json"],"produces":["application/problem+json"],"request_type":"CreateOrder","response_type":"Order","security":[{"kind":"bearer","summary":"Bearer token","expression":"authenticated()","source":"security_config","file":"src/Security.java","line":8,"confidence":"EXACT","conflicting":true,"limitations":["path scoped"],"evidence_ids":["evidence:security"]}],"consumers":[{"id":"consumer:1","project":"frontend/web","service":"web","role":"frontend","caller":"createOrder","file":"src/api.ts","line":20,"http_method":"POST","path":"/orders/42","call_auth":[{"kind":"bearer","summary":"Authorization header","confidence":"EXACT"}],"resolution":"MATCHED","confidence":"RESOLVED","limitations":["static only"],"evidence_ids":["evidence:consumer"]}],"mismatches":[{"id":"mismatch:1","kind":"auth_conflict","severity":"WARNING","reason":"conflict","confidence":"INFERRED","evidence_ids":["evidence:mismatch"]}],"confidence":"EXACT","coverage":"COMPLETE","limitations":["static analysis"],"evidence_ids":["evidence:endpoint"]}]}`
	if string(data) != want {
		t.Fatalf("catalog JSON field order changed:\n got: %s\nwant: %s", data, want)
	}
}

func TestValidateAPICatalogRejectsDanglingAndDuplicateConsumers(t *testing.T) {
	catalog := validAPICatalogFixture()
	catalog.Endpoints[0].Consumers = append(catalog.Endpoints[0].Consumers, catalog.Endpoints[0].Consumers[0])
	if err := ValidateAPICatalog(catalog); err == nil || !strings.Contains(err.Error(), "duplicate consumer ID") {
		t.Fatalf("error=%v", err)
	}
}

func TestValidateAPICatalogRejectsEmptyAndDuplicateIDs(t *testing.T) {
	tests := []struct {
		name   string
		change func(*APICatalogRecord)
		want   string
	}{
		{name: "empty endpoint", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].ID = " " }, want: "empty endpoint ID"},
		{name: "duplicate endpoint", change: func(catalog *APICatalogRecord) { catalog.Endpoints = append(catalog.Endpoints, catalog.Endpoints[0]) }, want: "duplicate endpoint ID"},
		{name: "empty consumer", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].Consumers[0].ID = "" }, want: "empty consumer ID"},
		{name: "duplicate consumer across endpoints", change: func(catalog *APICatalogRecord) {
			second := catalog.Endpoints[0]
			second.ID = "endpoint:2"
			second.Path = "/orders/second"
			catalog.Endpoints = append(catalog.Endpoints, second)
		}, want: "duplicate consumer ID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := validAPICatalogFixture()
			test.change(&catalog)
			if err := ValidateAPICatalog(catalog); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want message containing %q", err, test.want)
			}
		})
	}
}

func TestValidateAPICatalogRejectsInvalidSourceLocations(t *testing.T) {
	tests := []struct {
		name   string
		change func(*APICatalogRecord)
		want   string
	}{
		{name: "absolute endpoint file", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].File = "/tmp/OrderController.java" }, want: "workspace-relative file"},
		{name: "windows absolute endpoint file", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].File = `C:\src\OrderController.java` }, want: "workspace-relative file"},
		{name: "escaping endpoint file", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].File = "../OrderController.java" }, want: "workspace-relative file"},
		{name: "negative endpoint line", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].Line = -1 }, want: "one-based or absent line"},
		{name: "absolute security file", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].Security[0].File = "/tmp/Security.java" }, want: "workspace-relative file"},
		{name: "negative security line", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].Security[0].Line = -1 }, want: "one-based or absent line"},
		{name: "absolute consumer file", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].Consumers[0].File = "/tmp/api.ts" }, want: "workspace-relative file"},
		{name: "negative consumer line", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].Consumers[0].Line = -1 }, want: "one-based or absent line"},
		{name: "escaping call auth file", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Consumers[0].CallAuth[0].File = "src/../../secret"
		}, want: "workspace-relative file"},
		{name: "negative call auth line", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].Consumers[0].CallAuth[0].Line = -1 }, want: "one-based or absent line"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := validAPICatalogFixture()
			test.change(&catalog)
			if err := ValidateAPICatalog(catalog); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want message containing %q", err, test.want)
			}
		})
	}
}

func TestValidateAPICatalogRejectsUnknownSecurityKinds(t *testing.T) {
	tests := []struct {
		name   string
		change func(*APICatalogRecord)
	}{
		{name: "provider", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].Security[0].Kind = "magic" }},
		{name: "consumer", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].Consumers[0].CallAuth[0].Kind = "magic" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := validAPICatalogFixture()
			test.change(&catalog)
			if err := ValidateAPICatalog(catalog); err == nil || !strings.Contains(err.Error(), "unknown security kind") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestValidateAPICatalogRejectsUnsortedEvidenceIDs(t *testing.T) {
	tests := []struct {
		name   string
		change func(*APICatalogRecord)
	}{
		{name: "endpoint", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].EvidenceIDs = []string{"evidence:z", "evidence:a"}
		}},
		{name: "provider security", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Security[0].EvidenceIDs = []string{"evidence:z", "evidence:a"}
		}},
		{name: "consumer", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Consumers[0].EvidenceIDs = []string{"evidence:z", "evidence:a"}
		}},
		{name: "consumer auth", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Consumers[0].CallAuth[0].EvidenceIDs = []string{"evidence:z", "evidence:a"}
		}},
		{name: "mismatch", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Mismatches[0].EvidenceIDs = []string{"evidence:z", "evidence:a"}
		}},
		{name: "duplicate", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].EvidenceIDs = []string{"evidence:a", "evidence:a"}
		}},
		{name: "empty", change: func(catalog *APICatalogRecord) { catalog.Endpoints[0].EvidenceIDs = []string{""} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := validAPICatalogFixture()
			test.change(&catalog)
			if err := ValidateAPICatalog(catalog); err == nil || !strings.Contains(err.Error(), "evidence IDs") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestSortAPICatalogMakesPermutationsByteStable(t *testing.T) {
	left := permutedAPICatalogFixture(false)
	right := permutedAPICatalogFixture(true)

	SortAPICatalog(&left)
	SortAPICatalog(&right)
	if err := ValidateAPICatalog(left); err != nil {
		t.Fatalf("left catalog invalid after sorting: %v", err)
	}
	if err := ValidateAPICatalog(right); err != nil {
		t.Fatalf("right catalog invalid after sorting: %v", err)
	}
	leftJSON, err := json.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		t.Fatal(err)
	}
	if string(leftJSON) != string(rightJSON) {
		t.Fatalf("sorted catalog JSON differs by discovery order:\nleft:  %s\nright: %s", leftJSON, rightJSON)
	}
}

func TestValidateAPICatalogRejectsNonCanonicalRecordOrdering(t *testing.T) {
	catalog := permutedAPICatalogFixture(false)
	SortAPICatalog(&catalog)
	reverseSlice(catalog.Endpoints)
	if err := ValidateAPICatalog(catalog); err == nil || !strings.Contains(err.Error(), "canonical order") {
		t.Fatalf("error=%v", err)
	}
	SortAPICatalog(&catalog)
	if err := ValidateAPICatalog(catalog); err != nil {
		t.Fatalf("sorted catalog rejected: %v", err)
	}
}

func TestValidateAPICatalogRejectsEveryNonCanonicalNestedOrdering(t *testing.T) {
	tests := []struct {
		name   string
		change func(*APICatalogRecord)
	}{
		{name: "parameters", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Parameters = []APIParameterRecord{{Name: "z", Location: "query"}, {Name: "a", Location: "path"}}
		}},
		{name: "consumes", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Consumes = []string{"text/plain", "application/json"}
		}},
		{name: "produces", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Produces = []string{"text/plain", "application/json"}
		}},
		{name: "endpoint limitations", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Limitations = []string{"z", "a"}
		}},
		{name: "security", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Security = []SecurityEvidenceRecord{
				{Kind: SecurityUnknown, Summary: "unknown", Confidence: ConfidenceUnknown},
				{Kind: SecurityBearer, Summary: "bearer", Confidence: ConfidenceExact},
			}
		}},
		{name: "security limitations", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Security[0].Limitations = []string{"z", "a"}
		}},
		{name: "consumer limitations", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Consumers[0].Limitations = []string{"z", "a"}
		}},
		{name: "call auth", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Consumers[0].CallAuth = []SecurityEvidenceRecord{
				{Kind: SecurityUnknown, Summary: "unknown", Confidence: ConfidenceUnknown},
				{Kind: SecurityBearer, Summary: "bearer", Confidence: ConfidenceExact},
			}
		}},
		{name: "call auth limitations", change: func(catalog *APICatalogRecord) {
			catalog.Endpoints[0].Consumers[0].CallAuth[0].Limitations = []string{"z", "a"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := validAPICatalogFixture()
			test.change(&catalog)
			if err := ValidateAPICatalog(catalog); err == nil || !strings.Contains(err.Error(), "canonical order") {
				t.Fatalf("error=%v", err)
			}
			SortAPICatalog(&catalog)
			if err := ValidateAPICatalog(catalog); err != nil {
				t.Fatalf("sorted catalog rejected: %v", err)
			}
		})
	}
}

func TestSortAPICatalogNormalizesMandatoryNilSlices(t *testing.T) {
	left := APICatalogRecord{
		SchemaVersion: SchemaVersion,
		Endpoints: []APIEndpointRecord{{
			ID: "endpoint:1", ProviderProject: "services/orders", Transport: "http", HTTPMethod: "GET", Path: "/orders",
			Consumers: []APIConsumerRecord{{ID: "consumer:1", Project: "frontend/web"}},
		}},
	}
	right := APICatalogRecord{
		SchemaVersion: SchemaVersion,
		Endpoints: []APIEndpointRecord{{
			ID: "endpoint:1", ProviderProject: "services/orders", Transport: "http", HTTPMethod: "GET", Path: "/orders",
			Security:  []SecurityEvidenceRecord{},
			Consumers: []APIConsumerRecord{{ID: "consumer:1", Project: "frontend/web", CallAuth: []SecurityEvidenceRecord{}}},
		}},
	}
	SortAPICatalog(&left)
	SortAPICatalog(&right)
	leftJSON, err := json.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		t.Fatal(err)
	}
	if string(leftJSON) != string(rightJSON) {
		t.Fatalf("nil and empty mandatory slices differ after sorting:\n nil: %s\nempty: %s", leftJSON, rightJSON)
	}
	for _, want := range []string{`"security":[]`, `"consumers":[`, `"call_auth":[]`} {
		if !strings.Contains(string(leftJSON), want) {
			t.Fatalf("canonical JSON %s missing %s", leftJSON, want)
		}
	}

	empty := APICatalogRecord{SchemaVersion: SchemaVersion}
	SortAPICatalog(&empty)
	emptyJSON, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	wantEmptyJSON := `{"schema_version":` + strconv.Itoa(SchemaVersion) + `,"endpoints":[]}`
	if string(emptyJSON) != wantEmptyJSON {
		t.Fatalf("empty catalog JSON = %s", emptyJSON)
	}
}

func TestSortAPICatalogWritesEmptyConsumerArray(t *testing.T) {
	catalog := APICatalogRecord{SchemaVersion: SchemaVersion, Endpoints: []APIEndpointRecord{{
		ID: "endpoint:1", ProviderProject: "services/orders", Transport: "http", HTTPMethod: "GET", Path: "/orders",
	}}}
	SortAPICatalog(&catalog)
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"security":[],"consumers":[]`) {
		t.Fatalf("endpoint mandatory slices are not empty arrays: %s", data)
	}
}

func TestBuildWorkspaceAPICatalogKeepsProviderInventoryAndAttachesConsumers(t *testing.T) {
	provider := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "services/orders", Service: "orders", Kind: "backend"}, endpoints: []SpringEndpointRecord{
		{HTTPMethod: "GET", Path: "/orders/{id}", Controller: "OrderController", Method: "get", File: "OrderController.java", Line: 10},
		{HTTPMethod: "POST", Path: "/orders", Controller: "OrderController", Method: "create", File: "OrderController.java", Line: 20},
	}}
	consumer := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "frontend/web", Service: "web", Kind: "frontend"}, contracts: []APIContractRecord{{HTTPMethod: "GET", Path: "/orders/42", Caller: "loadOrder", File: "src/api.ts", Line: 7}}}
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{provider.record, consumer.record}}
	matches := []WorkspaceContractMatchRecord{{
		ID: "match:orders", APIProject: "frontend/web", APIHTTPMethod: "GET", APIPath: "/orders/42", APIFile: "src/api.ts", APILine: 7, APICaller: "loadOrder",
		BackendProject: "services/orders", BackendService: "orders", BackendHTTPMethod: "GET", BackendPath: "/orders/{id}",
		BackendHandler: "OrderController.get", BackendFile: "OrderController.java", BackendLine: 10,
		Issue: contractIssueMatched, Confidence: "RESOLVED",
	}}
	flows := []WorkspaceFeatureFlowRecord{
		{ID: "wrong-handler", FrontendProject: "frontend/web", FrontendFile: "src/api.ts", HTTPMethod: "GET", Path: "/orders/{id}", BackendProject: "services/orders", BackendController: "OrderController", BackendMethod: "list", BackendRequestType: "WrongRequest", BackendReturnType: "WrongResponse"},
		{ID: "exact-handler", FrontendProject: "frontend/web", FrontendFile: "src/api.ts", HTTPMethod: "GET", Path: "/orders/{id}", BackendProject: "services/orders", BackendController: "OrderController", BackendMethod: "get", BackendRequestType: "OrderQuery", BackendReturnType: "Order"},
	}

	catalog, err := BuildWorkspaceAPICatalog(registry, []workspaceIndexProject{provider, consumer}, matches, flows, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Endpoints) != 2 {
		t.Fatalf("catalog=%#v", catalog)
	}
	if len(catalog.Endpoints[0].Consumers) != 1 || catalog.Endpoints[0].Consumers[0].Project != "frontend/web" {
		t.Fatalf("matched endpoint consumers=%#v", catalog.Endpoints[0].Consumers)
	}
	if catalog.Endpoints[0].RequestType != "OrderQuery" || catalog.Endpoints[0].ResponseType != "Order" {
		t.Fatalf("matched endpoint types=%q/%q", catalog.Endpoints[0].RequestType, catalog.Endpoints[0].ResponseType)
	}
	if len(catalog.Endpoints[1].Consumers) != 0 {
		t.Fatalf("zero-consumer endpoint was assigned consumers: %#v", catalog.Endpoints[1])
	}
}

func TestBuildWorkspaceAPICatalogGrowthIsSubquadratic(t *testing.T) {
	small := benchmarkWorkspaceAPICatalogBuild(160)
	large := benchmarkWorkspaceAPICatalogBuild(320)
	growth := float64(large.NsPerOp()) / float64(small.NsPerOp())
	t.Logf("catalog build growth: %.2fx (small=%s large=%s)", growth, small, large)
	if growth >= 3.25 {
		t.Fatalf("doubling catalog input grew build time %.2fx; want less than 3.25x (small=%s large=%s)", growth, small, large)
	}
}

func TestWorkspaceCatalogCompatiblePathKeysMirrorRouteMatcher(t *testing.T) {
	paths := []string{
		"/orders/{id}",
		"/orders/{orderId}",
		"/orders/42",
		"/regulations/{type}",
		"/regulations/new",
		"/regulations/archived",
		"/userservice/users/{id}",
		"/users/{id}",
		"/${service.base_path}/users/{id}",
		"/ORDERS/{id}",
		`RegulationChangeBaseController.PATH_BASE + "RegulationChangeBaseController.PATH_FRAGMENT_CHANGES_NEW"`,
	}
	for _, left := range paths {
		for _, right := range paths {
			want := pathsCompatibleWithKnownBasePrefixes(left, right)
			got := workspaceCatalogPathKeysIntersect(
				workspaceCatalogCompatiblePathKeys(left),
				workspaceCatalogCompatiblePathKeys(right),
			)
			if got != want {
				t.Fatalf("indexed compatibility for %q and %q = %t, want %t", left, right, got, want)
			}
		}
	}
}

func workspaceCatalogPathKeysIntersect(left, right []string) bool {
	leftKeys := make(map[string]bool, len(left))
	for _, key := range left {
		leftKeys[key] = true
	}
	for _, key := range right {
		if leftKeys[key] {
			return true
		}
	}
	return false
}

func benchmarkWorkspaceAPICatalogBuild(size int) testing.BenchmarkResult {
	provider := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "services/orders"}}
	consumer := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "frontend/web"}}
	flows := make([]WorkspaceFeatureFlowRecord, 0, size)
	matches := make([]WorkspaceContractMatchRecord, 0, size)
	for index := 0; index < size; index++ {
		route := fmt.Sprintf("/orders/%d/{id}", index)
		consumerRoute := fmt.Sprintf("/orders/%d/42", index)
		handler := fmt.Sprintf("OrderController.get%d", index)
		file := fmt.Sprintf("src/api/%d.ts", index)
		provider.endpoints = append(provider.endpoints, SpringEndpointRecord{
			HTTPMethod: "GET", Path: route, Controller: "OrderController", Method: fmt.Sprintf("get%d", index), File: "OrderController.java", Line: index + 1,
		})
		consumer.contracts = append(consumer.contracts, APIContractRecord{
			HTTPMethod: "GET", Path: consumerRoute, Caller: handler, File: file, Line: index + 1,
		})
		flows = append(flows, WorkspaceFeatureFlowRecord{
			HTTPMethod: "GET", Path: route, BackendProject: "services/orders", BackendController: "OrderController", BackendMethod: fmt.Sprintf("get%d", index), BackendReturnType: "Order",
		})
		matches = append(matches, WorkspaceContractMatchRecord{
			ID: fmt.Sprintf("match:%d", index), APIProject: "frontend/web", APIHTTPMethod: "GET", APIPath: consumerRoute, APIFile: file, APILine: index + 1, APICaller: handler,
			BackendProject: "services/orders", BackendHTTPMethod: "GET", BackendPath: route, BackendHandler: handler, BackendFile: "OrderController.java", BackendLine: index + 1,
			Issue: contractIssueMatched, Confidence: "RESOLVED",
		})
	}
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{provider.record, consumer.record}}
	projects := []workspaceIndexProject{provider, consumer}
	return testing.Benchmark(func(b *testing.B) {
		for iteration := 0; iteration < b.N; iteration++ {
			catalog, err := BuildWorkspaceAPICatalog(registry, projects, matches, flows, "fixed")
			if err != nil {
				b.Fatal(err)
			}
			if len(catalog.Endpoints) != size {
				b.Fatalf("endpoints=%d, want %d", len(catalog.Endpoints), size)
			}
		}
	})
}

func TestBuildWorkspaceAPICatalogReportsSecurityMismatches(t *testing.T) {
	tests := []struct {
		name           string
		providerAuth   []AuthRecord
		consumerAuth   []AuthRecord
		wantKind       string
		wantSeverity   string
		wantReason     string
		unwantedReason string
	}{
		{
			name: "basic versus bearer", providerAuth: []AuthRecord{{Kind: "http_basic", Confidence: "EXACT"}},
			consumerAuth: []AuthRecord{{Kind: "bearer", Confidence: "EXTRACTED"}}, wantKind: "auth_scheme_mismatch", wantSeverity: "WARNING", wantReason: "basic",
		},
		{
			name: "authenticated with no call evidence", providerAuth: []AuthRecord{{Kind: "authenticated", Confidence: "EXACT"}},
			wantKind: "missing_call_auth_evidence", wantSeverity: "WARNING", wantReason: "incomplete static evidence", unwantedReason: "is a proven authentication failure",
		},
		{
			name: "credentials sent to explicit public endpoint", providerAuth: []AuthRecord{{Kind: "permit_all", Confidence: "EXACT"}},
			consumerAuth: []AuthRecord{{Kind: "basic", Confidence: "EXTRACTED"}}, wantKind: "credentials_on_public_endpoint", wantSeverity: "INFO", wantReason: "explicitly public",
		},
		{
			name: "conflicting provider rules", providerAuth: []AuthRecord{{Kind: "permit_all", Confidence: "EXACT"}, {Kind: "authenticated", Confidence: "EXACT"}},
			wantKind: "conflicting_provider_security", wantSeverity: "WARNING", wantReason: "conflicting",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "services/orders"}, endpoints: []SpringEndpointRecord{{
				HTTPMethod: "GET", Path: "/orders/{id}", Controller: "OrderController", Method: "get", File: "OrderController.java", Line: 10, Auth: test.providerAuth,
			}}}
			consumer := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "frontend/web"}, contracts: []APIContractRecord{{
				HTTPMethod: "GET", Path: "/orders/42", File: "src/api.ts", Line: 7, Auth: test.consumerAuth,
			}}}
			match := WorkspaceContractMatchRecord{
				ID: "match:orders", APIProject: "frontend/web", APIHTTPMethod: "GET", APIPath: "/orders/42", APIFile: "src/api.ts", APILine: 7,
				BackendProject: "services/orders", BackendHTTPMethod: "GET", BackendPath: "/orders/{id}", BackendHandler: "OrderController.get", BackendFile: "OrderController.java", BackendLine: 10,
				Issue: contractIssueMatched, Confidence: "RESOLVED",
			}
			catalog, err := BuildWorkspaceAPICatalog(WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{provider.record, consumer.record}}, []workspaceIndexProject{provider, consumer}, []WorkspaceContractMatchRecord{match}, nil, "fixed")
			if err != nil {
				t.Fatal(err)
			}
			mismatch := requireAPIMismatch(t, catalog.Endpoints[0].Mismatches, test.wantKind)
			if mismatch.Severity != test.wantSeverity || !strings.Contains(strings.ToLower(mismatch.Reason), test.wantReason) {
				t.Fatalf("mismatch=%#v", mismatch)
			}
			if test.unwantedReason != "" && strings.Contains(strings.ToLower(mismatch.Reason), test.unwantedReason) {
				t.Fatalf("mismatch overstates static evidence: %#v", mismatch)
			}
			if len(mismatch.EvidenceIDs) == 0 {
				t.Fatalf("mismatch has no evidence IDs: %#v", mismatch)
			}
		})
	}
}

func TestBuildWorkspaceAPICatalogPreservesAmbiguousRouteWithoutAssigningConsumer(t *testing.T) {
	provider := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "services/orders"}, endpoints: []SpringEndpointRecord{
		{HTTPMethod: "GET", Path: "/orders/{id}", Controller: "OrderController", Method: "get", File: "OrderController.java", Line: 10},
		{HTTPMethod: "GET", Path: "/orders/{id}", Controller: "LegacyOrderController", Method: "get", File: "LegacyOrderController.java", Line: 20},
	}}
	consumer := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "frontend/web"}, contracts: []APIContractRecord{{HTTPMethod: "GET", Path: "/orders/42", File: "src/api.ts", Line: 7}}}
	match := WorkspaceContractMatchRecord{
		ID: "match:ambiguous", APIProject: "frontend/web", APIHTTPMethod: "GET", APIPath: "/orders/42", APIFile: "src/api.ts", APILine: 7,
		BackendProject: "services/orders", BackendHTTPMethod: "GET", BackendPath: "/orders/{id}", Issue: "ambiguous_route", Confidence: "AMBIGUOUS",
		EquivalentRouteCandidates: []string{"GET /orders/{id}"},
	}

	catalog, err := BuildWorkspaceAPICatalog(WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{provider.record, consumer.record}}, []workspaceIndexProject{provider, consumer}, []WorkspaceContractMatchRecord{match}, nil, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range catalog.Endpoints {
		if len(endpoint.Consumers) != 0 {
			t.Fatalf("ambiguous route assigned consumer: %#v", endpoint)
		}
		requireAPIMismatch(t, endpoint.Mismatches, "ambiguous_route_match")
	}
}

func TestBuildWorkspaceAPICatalogIsDeterministicAcrossDiscoveryOrder(t *testing.T) {
	provider := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "services/orders", Service: "orders"}, endpoints: []SpringEndpointRecord{
		{HTTPMethod: "POST", Path: "/orders", Controller: "OrderController", Method: "create", File: "OrderController.java", Line: 20},
		{HTTPMethod: "GET", Path: "/orders/{id}", Controller: "OrderController", Method: "get", File: "OrderController.java", Line: 10, Auth: []AuthRecord{{Kind: "authenticated", Confidence: "EXACT"}}},
	}}
	consumer := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "frontend/web", Service: "web"}, contracts: []APIContractRecord{
		{HTTPMethod: "GET", Path: "/orders/42", Caller: "load", File: "src/z.ts", Line: 7},
		{HTTPMethod: "GET", Path: "/orders/7", Caller: "refresh", File: "src/a.ts", Line: 3, Auth: []AuthRecord{{Kind: "bearer", Confidence: "EXTRACTED"}}},
	}}
	matches := []WorkspaceContractMatchRecord{
		{ID: "match:z", APIProject: "frontend/web", APIHTTPMethod: "GET", APIPath: "/orders/42", APIFile: "src/z.ts", APILine: 7, APICaller: "load", BackendProject: "services/orders", BackendService: "orders", BackendHTTPMethod: "GET", BackendPath: "/orders/{id}", BackendHandler: "OrderController.get", BackendFile: "OrderController.java", BackendLine: 10, Issue: contractIssueMatched, Confidence: "RESOLVED"},
		{ID: "match:a", APIProject: "frontend/web", APIHTTPMethod: "GET", APIPath: "/orders/7", APIFile: "src/a.ts", APILine: 3, APICaller: "refresh", BackendProject: "services/orders", BackendService: "orders", BackendHTTPMethod: "GET", BackendPath: "/orders/{id}", BackendHandler: "OrderController.get", BackendFile: "OrderController.java", BackendLine: 10, Issue: contractIssueMatched, Confidence: "RESOLVED"},
	}
	registry := WorkspaceRegistryRecord{Root: "workspace", Projects: []WorkspaceProjectRecord{provider.record, consumer.record}}
	forward, err := BuildWorkspaceAPICatalog(registry, []workspaceIndexProject{provider, consumer}, matches, nil, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	reverseSlice(provider.endpoints)
	reverseSlice(consumer.contracts)
	reverseSlice(matches)
	reverseSlice(registry.Projects)
	reverse, err := BuildWorkspaceAPICatalog(registry, []workspaceIndexProject{consumer, provider}, matches, nil, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	forwardJSON, _ := json.Marshal(forward)
	reverseJSON, _ := json.Marshal(reverse)
	if string(forwardJSON) != string(reverseJSON) {
		t.Fatalf("discovery order changed catalog JSON:\nforward: %s\nreverse: %s", forwardJSON, reverseJSON)
	}
}

func TestFilterWorkspaceAPICatalogKeepsOnlyProjectScopedConsumerMismatches(t *testing.T) {
	catalog := APICatalogRecord{SchemaVersion: SchemaVersion, Generated: "fixed", Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders", Transport: "http", HTTPMethod: "GET", Path: "/orders/{id}",
		Security: []SecurityEvidenceRecord{},
		Consumers: []APIConsumerRecord{
			{ID: "consumer:a", Project: "frontend/a", File: "src/a.ts", Line: 1, CallAuth: []SecurityEvidenceRecord{}, Resolution: "MATCHED", Confidence: ConfidenceResolved, EvidenceIDs: []string{"match:a"}},
			{ID: "consumer:b", Project: "frontend/b", File: "src/b.ts", Line: 2, CallAuth: []SecurityEvidenceRecord{}, Resolution: "MATCHED", Confidence: ConfidenceResolved, EvidenceIDs: []string{"match:b"}},
		},
		Mismatches: []APIMismatchRecord{
			{ID: "mismatch:provider", Kind: "conflicting_provider_security", Severity: "WARNING", Reason: "provider conflict", Confidence: ConfidenceInferred, EvidenceIDs: []string{"provider:evidence"}},
			{ID: "mismatch:a", Kind: "missing_call_auth_evidence", Severity: "WARNING", Reason: "a", ConsumerProject: "frontend/a", ConsumerID: "consumer:a", Confidence: ConfidenceInferred, EvidenceIDs: []string{"match:a"}},
			{ID: "mismatch:b", Kind: "auth_scheme_mismatch", Severity: "WARNING", Reason: "b", ConsumerProject: "frontend/b", ConsumerID: "consumer:b", Confidence: ConfidenceInferred, EvidenceIDs: []string{"match:b"}},
		},
		Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}}}
	SortAPICatalog(&catalog)

	consumerCatalog := filterWorkspaceAPICatalog("frontend/a", catalog)
	if len(consumerCatalog.Endpoints) != 1 || len(consumerCatalog.Endpoints[0].Consumers) != 1 || consumerCatalog.Endpoints[0].Consumers[0].Project != "frontend/a" {
		t.Fatalf("consumer catalog=%#v", consumerCatalog)
	}
	if got := consumerCatalog.Endpoints[0].Mismatches; len(got) != 2 || !hasAPIMismatchID(got, "mismatch:provider") || !hasAPIMismatchID(got, "mismatch:a") {
		t.Fatalf("consumer mismatches=%#v", got)
	}
	consumerJSON, err := json.Marshal(consumerCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(consumerJSON), "consumer:b") || strings.Contains(string(consumerJSON), "match:b") || strings.Contains(string(consumerJSON), "frontend/b") {
		t.Fatalf("consumer catalog leaked another consumer's scope or evidence: %s", consumerJSON)
	}

	providerCatalog := filterWorkspaceAPICatalog("services/orders", catalog)
	if len(providerCatalog.Endpoints) != 1 || len(providerCatalog.Endpoints[0].Consumers) != 2 || len(providerCatalog.Endpoints[0].Mismatches) != 3 {
		t.Fatalf("provider catalog lost consumer or provider-wide details: %#v", providerCatalog)
	}
}

func TestBuildWorkspaceAPICatalogKeepsMultipleAmbiguousCallSitesDeterministically(t *testing.T) {
	spring := workspaceIndexProject{
		record:    WorkspaceProjectRecord{Path: "services/orders-java", Service: "orders"},
		endpoints: []SpringEndpointRecord{{HTTPMethod: "GET", Path: "/orders/{id}", Controller: "OrderController", Method: "get", File: "OrderController.java", Line: 10}},
	}
	script := workspaceIndexProject{
		record: WorkspaceProjectRecord{Path: "services/orders-js", Service: "orders"},
		routes: []CodeRouteRecord{{Language: "typescript", Framework: "Express", Kind: "backend", FrameworkBound: true, HTTPMethod: "GET", Path: "/orders/:id", Handler: "getOrder", File: "src/server.ts", Line: 4}},
	}
	consumerA := workspaceIndexProject{
		record:    WorkspaceProjectRecord{Path: "frontend/a"},
		contracts: []APIContractRecord{{HTTPMethod: "GET", Path: "/orders/{id}", File: "src/a.ts", Line: 7, Caller: "loadA"}},
	}
	consumerB := workspaceIndexProject{
		record:    WorkspaceProjectRecord{Path: "frontend/b"},
		contracts: []APIContractRecord{{HTTPMethod: "GET", Path: "/orders/{id}", File: "src/b.ts", Line: 9, Caller: "loadB"}},
	}
	projects := []workspaceIndexProject{consumerA, spring, consumerB, script}
	registry := WorkspaceRegistryRecord{Root: "workspace", Projects: []WorkspaceProjectRecord{consumerA.record, spring.record, consumerB.record, script.record}}
	matches := buildWorkspaceContractMatches(projects)
	if len(matches) != 2 || matches[0].Issue != "ambiguous_route" || matches[1].Issue != "ambiguous_route" {
		t.Fatalf("matches=%#v", matches)
	}

	forward, err := BuildWorkspaceAPICatalog(registry, projects, matches, nil, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range forward.Endpoints {
		if len(endpoint.Consumers) != 0 || len(endpoint.Mismatches) != 2 {
			t.Fatalf("ambiguous call sites collapsed or attached on %#v", endpoint)
		}
		if endpoint.Mismatches[0].ID == endpoint.Mismatches[1].ID {
			t.Fatalf("ambiguous call sites share mismatch ID: %#v", endpoint.Mismatches)
		}
		for _, mismatch := range endpoint.Mismatches {
			matchID := ambiguousMatchIDForProject(t, matches, mismatch.ConsumerProject)
			if mismatch.ConsumerID == "" || !containsString(mismatch.EvidenceIDs, matchID) {
				t.Fatalf("ambiguous mismatch lost scope/evidence: %#v", mismatch)
			}
			for _, other := range matches {
				if other.ID != matchID && containsString(mismatch.EvidenceIDs, other.ID) {
					t.Fatalf("ambiguous mismatch shared another call site's evidence: %#v", mismatch)
				}
			}
		}
	}
	consumerACatalog := filterWorkspaceAPICatalog("frontend/a", forward)
	if len(consumerACatalog.Endpoints) != 2 {
		t.Fatalf("ambiguous consumer scope did not retain candidate endpoints: %#v", consumerACatalog)
	}
	for _, endpoint := range consumerACatalog.Endpoints {
		if len(endpoint.Consumers) != 0 || len(endpoint.Mismatches) != 1 || endpoint.Mismatches[0].ConsumerProject != "frontend/a" {
			t.Fatalf("ambiguous project catalog leaked or lost call-site scope: %#v", endpoint)
		}
		if containsString(endpoint.Mismatches[0].EvidenceIDs, ambiguousMatchIDForProject(t, matches, "frontend/b")) {
			t.Fatalf("ambiguous project catalog leaked other call-site evidence: %#v", endpoint.Mismatches[0])
		}
	}

	reverseProjects := append([]workspaceIndexProject(nil), projects...)
	reverseSlice(reverseProjects)
	reverseMatches := append([]WorkspaceContractMatchRecord(nil), matches...)
	reverseSlice(reverseMatches)
	reverseRegistry := registry
	reverseRegistry.Projects = append([]WorkspaceProjectRecord(nil), registry.Projects...)
	reverseSlice(reverseRegistry.Projects)
	reverse, err := BuildWorkspaceAPICatalog(reverseRegistry, reverseProjects, reverseMatches, nil, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	forwardJSON, _ := json.Marshal(forward)
	reverseJSON, _ := json.Marshal(reverse)
	if string(forwardJSON) != string(reverseJSON) {
		t.Fatalf("ambiguous call-site order changed catalog JSON:\nforward: %s\nreverse: %s", forwardJSON, reverseJSON)
	}
}

func TestBuildWorkspaceAPICatalogRecoversPrefixCompatibleAmbiguousProviders(t *testing.T) {
	userService := workspaceIndexProject{
		record:    WorkspaceProjectRecord{Path: "services/users", Service: "users"},
		endpoints: []SpringEndpointRecord{{HTTPMethod: "GET", Path: "/userservice/users/{id}", Controller: "UserController", Method: "get", File: "UserController.java", Line: 10}},
	}
	productService := workspaceIndexProject{
		record:    WorkspaceProjectRecord{Path: "services/products", Service: "products"},
		endpoints: []SpringEndpointRecord{{HTTPMethod: "GET", Path: "/productservice/users/{id}", Controller: "ProductUserController", Method: "get", File: "ProductUserController.java", Line: 20}},
	}
	consumer := workspaceIndexProject{
		record:    WorkspaceProjectRecord{Path: "frontend/web"},
		contracts: []APIContractRecord{{HTTPMethod: "GET", Path: "/users/{id}", File: "src/users.ts", Line: 7, Caller: "loadUser"}},
	}
	projects := []workspaceIndexProject{consumer, userService, productService}
	registry := WorkspaceRegistryRecord{Root: "workspace", Projects: []WorkspaceProjectRecord{consumer.record, userService.record, productService.record}}
	matches := buildWorkspaceContractMatches(projects)
	if len(matches) != 1 || matches[0].Issue != "ambiguous_route" || matches[0].Confidence != "AMBIGUOUS" || len(matches[0].EquivalentRouteCandidates) != 2 {
		t.Fatalf("prefix-compatible providers were not preserved by matcher: %#v", matches)
	}

	forward, err := BuildWorkspaceAPICatalog(registry, projects, matches, nil, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	if len(forward.Endpoints) != 2 {
		t.Fatalf("provider inventory=%#v", forward.Endpoints)
	}
	for _, endpoint := range forward.Endpoints {
		if len(endpoint.Consumers) != 0 || len(endpoint.Mismatches) != 1 {
			t.Fatalf("prefix-compatible ambiguity was not attached to candidate endpoint: %#v", endpoint)
		}
		mismatch := endpoint.Mismatches[0]
		if mismatch.Kind != "ambiguous_route_match" || mismatch.ConsumerProject != "frontend/web" || !containsString(mismatch.EvidenceIDs, matches[0].ID) {
			t.Fatalf("prefix-compatible ambiguity lost scope/evidence: %#v", mismatch)
		}
	}
	filtered := filterWorkspaceAPICatalog("frontend/web", forward)
	if len(filtered.Endpoints) != 2 {
		t.Fatalf("consumer catalog lost prefix-compatible candidates: %#v", filtered)
	}

	reverseProjects := append([]workspaceIndexProject(nil), projects...)
	reverseSlice(reverseProjects)
	reverseRegistry := registry
	reverseRegistry.Projects = append([]WorkspaceProjectRecord(nil), registry.Projects...)
	reverseSlice(reverseRegistry.Projects)
	reverseMatches := buildWorkspaceContractMatches(reverseProjects)
	reverse, err := BuildWorkspaceAPICatalog(reverseRegistry, reverseProjects, reverseMatches, nil, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	forwardJSON, _ := json.Marshal(forward)
	reverseJSON, _ := json.Marshal(reverse)
	if string(forwardJSON) != string(reverseJSON) {
		t.Fatalf("prefix-compatible discovery order changed catalog JSON:\nforward: %s\nreverse: %s", forwardJSON, reverseJSON)
	}
}

func ambiguousMatchIDForProject(t *testing.T, matches []WorkspaceContractMatchRecord, project string) string {
	t.Helper()
	for _, match := range matches {
		if match.APIProject == project {
			return match.ID
		}
	}
	t.Fatalf("missing ambiguous match for %q in %#v", project, matches)
	return ""
}

func hasAPIMismatchID(records []APIMismatchRecord, id string) bool {
	for _, record := range records {
		if record.ID == id {
			return true
		}
	}
	return false
}

func requireAPIMismatch(t *testing.T, mismatches []APIMismatchRecord, kind string) APIMismatchRecord {
	t.Helper()
	for _, mismatch := range mismatches {
		if mismatch.Kind == kind {
			return mismatch
		}
	}
	t.Fatalf("missing mismatch %q in %#v", kind, mismatches)
	return APIMismatchRecord{}
}

func validAPICatalogFixture() APICatalogRecord {
	return APICatalogRecord{
		SchemaVersion: SchemaVersion,
		Generated:     "fixed",
		Endpoints: []APIEndpointRecord{{
			ID: "endpoint:1", ProviderProject: "services/orders", Transport: "http", HTTPMethod: "GET", Path: "/orders/{id}",
			File: "src/OrderController.java", Line: 10,
			Security: []SecurityEvidenceRecord{{Kind: SecurityUnknown, Summary: "No auth evidence detected", File: "src/Security.java", Line: 2, Confidence: ConfidenceUnknown, EvidenceIDs: []string{"evidence:security"}}},
			Consumers: []APIConsumerRecord{{
				ID: "consumer:1", Project: "frontend/web", File: "src/api.ts", Line: 3,
				CallAuth:   []SecurityEvidenceRecord{{Kind: SecurityUnknown, Summary: "No auth evidence detected", File: "src/api.ts", Line: 3, Confidence: ConfidenceUnknown, EvidenceIDs: []string{"evidence:call-auth"}}},
				Resolution: "MATCHED", Confidence: ConfidenceResolved, EvidenceIDs: []string{"evidence:consumer"},
			}},
			Mismatches: []APIMismatchRecord{{ID: "mismatch:1", Kind: "missing_auth", Severity: "WARNING", Reason: "Static evidence is incomplete.", Confidence: ConfidenceInferred, EvidenceIDs: []string{"evidence:mismatch"}}},
			Confidence: ConfidenceExact, Coverage: CoverageComplete, EvidenceIDs: []string{"evidence:endpoint"},
		}},
	}
}

func permutedAPICatalogFixture(reverse bool) APICatalogRecord {
	endpointA := validAPICatalogFixture().Endpoints[0]
	endpointA.ID = "endpoint:a"
	endpointA.Parameters = []APIParameterRecord{
		{Name: "query", Location: "query", Confidence: ConfidenceInferred},
		{Name: "id", Location: "path", Confidence: ConfidenceExact},
	}
	endpointA.Consumes = []string{"text/plain", "application/json"}
	endpointA.Produces = []string{"text/plain", "application/json"}
	endpointA.Security = append(endpointA.Security, SecurityEvidenceRecord{Kind: SecurityBearer, Summary: "Bearer token", Confidence: ConfidenceExact, Limitations: []string{"z", "a"}, EvidenceIDs: []string{"evidence:z", "evidence:a"}})
	endpointA.Consumers = append(endpointA.Consumers, APIConsumerRecord{
		ID: "consumer:2", Project: "cli", Caller: "main", CallAuth: []SecurityEvidenceRecord{
			{Kind: SecurityBearer, Summary: "token", Confidence: ConfidenceExact},
			{Kind: SecurityUnknown, Summary: "fallback", Confidence: ConfidenceUnknown},
		}, Resolution: "PARTIAL", Confidence: ConfidenceInferred, Limitations: []string{"z", "a"}, EvidenceIDs: []string{"evidence:z", "evidence:a"},
	})
	endpointA.Mismatches = append(endpointA.Mismatches, APIMismatchRecord{ID: "mismatch:0", Kind: "conflict", Severity: "ERROR", Reason: "conflict", Confidence: ConfidenceExact, EvidenceIDs: []string{"evidence:z", "evidence:a"}})
	endpointA.Limitations = []string{"z", "a"}
	endpointA.EvidenceIDs = []string{"evidence:z", "evidence:a"}

	endpointB := validAPICatalogFixture().Endpoints[0]
	endpointB.ID = "endpoint:b"
	endpointB.ProviderProject = "services/billing"
	endpointB.Path = "/billing"
	endpointB.Consumers[0].ID = "consumer:3"

	if reverse {
		reverseAPIEndpointSlices(&endpointA)
		reverseAPIEndpointSlices(&endpointB)
		return APICatalogRecord{SchemaVersion: SchemaVersion, Generated: "fixed", Endpoints: []APIEndpointRecord{endpointB, endpointA}}
	}
	return APICatalogRecord{SchemaVersion: SchemaVersion, Generated: "fixed", Endpoints: []APIEndpointRecord{endpointA, endpointB}}
}

func reverseAPIEndpointSlices(endpoint *APIEndpointRecord) {
	reverseSlice(endpoint.Parameters)
	reverseSlice(endpoint.Consumes)
	reverseSlice(endpoint.Produces)
	reverseSlice(endpoint.Security)
	reverseSlice(endpoint.Consumers)
	reverseSlice(endpoint.Mismatches)
	reverseSlice(endpoint.Limitations)
	reverseSlice(endpoint.EvidenceIDs)
	for index := range endpoint.Security {
		reverseSlice(endpoint.Security[index].Limitations)
		reverseSlice(endpoint.Security[index].EvidenceIDs)
	}
	for index := range endpoint.Consumers {
		reverseSlice(endpoint.Consumers[index].CallAuth)
		reverseSlice(endpoint.Consumers[index].Limitations)
		reverseSlice(endpoint.Consumers[index].EvidenceIDs)
		for authIndex := range endpoint.Consumers[index].CallAuth {
			reverseSlice(endpoint.Consumers[index].CallAuth[authIndex].Limitations)
			reverseSlice(endpoint.Consumers[index].CallAuth[authIndex].EvidenceIDs)
		}
	}
	for index := range endpoint.Mismatches {
		reverseSlice(endpoint.Mismatches[index].EvidenceIDs)
	}
}

func reverseSlice[T any](values []T) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
