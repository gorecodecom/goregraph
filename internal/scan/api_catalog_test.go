package scan

import (
	"encoding/json"
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
