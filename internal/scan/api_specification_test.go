package scan

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const swaggerSpecificationFixture = `swagger: '2.0'
info: {title: Orders, version: '1'}
basePath: /api
paths:
  /orders/{orderId}:
    get:
      operationId: findOrder
      responses:
        '200':
          schema:
            $ref: '#/definitions/Order'
definitions:
  Order:
    type: object
    required: [id]
    properties:
      id:
        type: string
        example: NEVER_EXPORT_EXAMPLE
      total:
        type: number
`

func TestAPISpecificationExtractsSwaggerStructureAndExactLines(t *testing.T) {
	record, recognized := parseAPISpecification("docs/api.yaml", []byte(swaggerSpecificationFixture))
	if !recognized || record.Status != "parsed" || record.Version != "2.0" || record.Line != 1 {
		t.Fatalf("document = %#v", record)
	}
	if len(record.Operations) != 1 || len(record.Models) != 1 {
		t.Fatalf("structure = %#v", record)
	}
	operation, model := record.Operations[0], record.Models[0]
	if operation.Method != "GET" || operation.Path != "/orders/{orderId}" || operation.Line != 6 || operation.OperationID != "findOrder" || !reflect.DeepEqual(operation.Models, []string{"Order"}) || !reflect.DeepEqual(operation.basePaths, []string{"/api"}) {
		t.Fatalf("operation = %#v", operation)
	}
	if model.Name != "Order" || model.Line != 13 || len(model.Fields) != 2 || model.Fields[0].Line != 17 || model.Fields[0].Required == nil || !*model.Fields[0].Required || model.Fields[1].Required == nil || *model.Fields[1].Required {
		t.Fatalf("model = %#v", model)
	}
	body, _ := json.Marshal(record)
	if strings.Contains(string(body), "NEVER_EXPORT_EXAMPLE") {
		t.Fatal("example value leaked into contract metadata")
	}
}

func TestAPISpecificationOpenAPIJSONKeepsModelsAndServerOverrides(t *testing.T) {
	body := []byte(`{
  "openapi": "3.1.0",
  "servers": [{"url":"https://user:password@example.test/api"}],
  "paths": {
    "/orders": {
      "servers": [{"url":"/v2"}],
      "post": {"servers":[{"url":"/v3"}], "requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Order"}}}}, "responses": {"201":{}}}
    }
  },
  "components": {"schemas":{"Order":{"type":"object","properties":{"id":{"type":"string"}}}}}
}`)
	record, recognized := parseAPISpecification("contract.json", body)
	if !recognized || record.Status != "parsed" || len(record.Operations) != 1 || len(record.Models) != 1 || record.Operations[0].Line != 7 || !reflect.DeepEqual(record.Operations[0].basePaths, []string{"/v3"}) || !reflect.DeepEqual(record.Operations[0].Models, []string{"Order"}) {
		t.Fatalf("OpenAPI = %#v", record)
	}
	exported, _ := json.Marshal(record)
	if strings.Contains(string(exported), "password") || strings.Contains(string(exported), "example.test") {
		t.Fatal("server credentials or hosts leaked into structural metadata")
	}
}

func TestAPISpecificationRejectsInvalidDocumentsWithoutUsableOperations(t *testing.T) {
	for name, body := range map[string]string{
		"syntax":        "openapi: [broken",
		"duplicate":     "swagger: '2.0'\npaths: {}\npaths: {}\n",
		"multiple":      "swagger: '2.0'\npaths: {}\n---\nopenapi: '3.1.0'\npaths: {}\n",
		"two_versions":  "swagger: '2.0'\nopenapi: '3.0.0'\npaths: {}\n",
		"bad_path":      "swagger: '2.0'\npaths:\n  /orders:\n    get: {responses: {}}\n  missing-slash:\n    get: {}\n",
		"bad_operation": "swagger: '2.0'\npaths:\n  /orders:\n    get: []\n",
		"missing_paths": "swagger: '2.0'\ninfo: {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			record, recognized := parseAPISpecification("openapi.yaml", []byte(body))
			if !recognized || record.Status != "invalid" || len(record.Operations) != 0 || len(record.Models) != 0 {
				t.Fatalf("invalid document retained usable evidence: %#v", record)
			}
		})
	}
	record, recognized := parseAPISpecification("openapi.json", []byte("swagger: '2.0'\npaths: {}"))
	if !recognized || record.Status != "invalid" {
		t.Fatalf("non-JSON accepted as JSON: %#v", record)
	}
}

func TestAPISpecificationLimitationsRemainVisible(t *testing.T) {
	for _, tc := range []struct{ name, body, status, limitation string }{
		{"future", "openapi: '4.0.0'\npaths: {}", "unsupported", "unsupported_version"},
		{"malformed_version", "openapi: '3.0.invalid'\npaths: {}", "unsupported", "unsupported_version"},
		{"wrong_version_field", "swagger: '3.0.0'\npaths: {}", "unsupported", "unsupported_version"},
		{"reference", "openapi: '3.0.0'\npaths:\n  /orders:\n    $ref: 'https://example.test/path.yaml'", "partial", "path_references_not_evaluated"},
		{"dynamic", "openapi: '3.0.0'\nservers: [{url: '/{stage}'}]\npaths: {/orders: {get: {responses: {}}}}", "partial", "server_path_unresolved"},
		{"composed", "openapi: '3.1.0'\ncomponents: {schemas: {Order: {allOf: [{$ref: '#/components/schemas/Base'}]}}}", "partial", "composed_models_not_evaluated"},
		{"alias", "swagger: '2.0'\npaths: &paths {}\ndefinitions: *paths", "partial", "aliases_not_evaluated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record, recognized := parseAPISpecification("openapi.yaml", []byte(tc.body))
			if !recognized || record.Status != tc.status || !slices.Contains(record.Limitations, tc.limitation) {
				t.Fatalf("limitation = %#v", record)
			}
			if tc.name == "dynamic" && len(record.Operations[0].basePaths) != 0 {
				t.Fatal("dynamic server prefix admitted route matching")
			}
		})
	}
}

func TestAPISpecificationRecognizesQuotedAndFlowYAMLHeaders(t *testing.T) {
	for _, body := range []string{"'openapi': '3.1.0'\npaths: {}", "{openapi: '3.1.0', paths: {}}"} {
		record, recognized := parseAPISpecification("contract.yaml", []byte(body))
		if !recognized || record.Status != "parsed" {
			t.Fatalf("valid YAML header was missed: %#v", record)
		}
	}
}

func TestAPISpecificationDoesNotInferOptionalFieldsAcrossComposedModels(t *testing.T) {
	body := "openapi: '3.1.0'\ncomponents:\n  schemas:\n    Order:\n      allOf: [{$ref: '#/components/schemas/Base'}]\n      properties:\n        id: {type: string}\n"
	record, _ := parseAPISpecification("contract.yaml", []byte(body))
	if record.Status != "partial" || len(record.Models) != 1 || len(record.Models[0].Fields) != 1 || record.Models[0].Fields[0].Required != nil {
		t.Fatalf("unknown inherited constraints were reported as optional: %#v", record)
	}
}

func TestAPISpecificationDoesNotPromoteOrdinaryConfiguration(t *testing.T) {
	for _, body := range []string{
		`{"dependencies":{"openapi":"1.0"}}`,
		`{"openapi":"NEVER_EXPORT_CONFIGURATION_VALUE"}`,
		"description: |\n  openapi: '3.0.0'\n  paths: {}\n",
		"configuration:\n  swagger: '2.0'\n",
	} {
		if _, recognized := parseAPISpecification("configuration.yaml", []byte(body)); recognized {
			t.Fatalf("configuration was promoted to an API contract: %s", body)
		}
	}
	record, recognized := parseAPISpecification("swagger.yaml", []byte("swagger: NEVER_EXPORT_CONFIGURATION_VALUE\n"))
	exported, _ := json.Marshal(record)
	if !recognized || record.Status != "unsupported" || strings.Contains(string(exported), "NEVER_EXPORT_CONFIGURATION_VALUE") {
		t.Fatal("invalid version field exported an arbitrary configuration value")
	}
}

func TestWorkspaceAPISpecificationLinkingKeepsAmbiguityAndMethods(t *testing.T) {
	makeIndex := func() APISpecificationIndexRecord {
		record, _ := parseAPISpecification("documentation/contracts/orders.yaml", []byte(swaggerSpecificationFixture))
		return APISpecificationIndexRecord{Documents: []APISpecificationRecord{record}}
	}
	provider := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "services/orders"}, routes: []CodeRouteRecord{{Kind: "backend", HTTPMethod: "GET", Path: "/api/orders/{id}", File: "src/Orders.java", Line: 12}}}
	index := makeIndex()
	linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{provider})
	operation := index.Documents[0].Operations[0]
	if operation.LinkStatus != "unique_route" || len(operation.CodeRoutes) != 1 || operation.CodeRoutes[0].Project != "services/orders" || operation.CodeRoutes[0].Line != 12 || index.Documents[0].OwnerProject != "" {
		t.Fatalf("unique code evidence = %#v", index)
	}
	linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{provider})
	if !reflect.DeepEqual(index.Documents[0].Operations[0], operation) {
		t.Fatal("repeated linking changed the result")
	}
	other := provider
	other.record.Path = "services/another"
	linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{provider, other})
	if index.Documents[0].Operations[0].LinkStatus != "ambiguous" || len(index.Documents[0].Operations[0].CodeRoutes) != 2 {
		t.Fatalf("ambiguous provider was assigned: %#v", index)
	}
	other.routes = []CodeRouteRecord{{Kind: "backend", HTTPMethod: "POST", Path: "/api/orders/{id}", File: "src/Other.java", Line: 12}}
	index = makeIndex()
	linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{other})
	if index.Documents[0].Operations[0].LinkStatus != "unlinked" || len(index.Documents[0].Operations[0].CodeRoutes) != 0 {
		t.Fatal("method mismatch created a service link")
	}
}

func TestWorkspaceAPISpecificationServerPrefixCandidates(t *testing.T) {
	makeIndex := func() APISpecificationIndexRecord {
		record, recognized := parseAPISpecification("documentation/swagger-vd.yaml", []byte("swagger: '2.0'\nbasePath: /api/1.0\npaths:\n  /cadasters/{cadasterId}:\n    get: {responses: {}}\n"))
		if !recognized {
			t.Fatal("fixture was not recognized")
		}
		return APISpecificationIndexRecord{Documents: []APISpecificationRecord{record}}
	}
	provider := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "services/cadaster"}, endpoints: []SpringEndpointRecord{{HTTPMethod: "GET", Path: "/cadasters/{id}", File: "src/CadasterController.java", Line: 42}}}
	index := makeIndex()
	linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{provider})
	op := index.Documents[0].Operations[0]
	if op.LinkStatus != "prefix_candidate" || op.MatchBasis != "document_path" || len(op.CodeRoutes) != 1 || op.CodeRoutes[0].Line != 42 || op.ResolvedPaths[0] != "/api/1.0/cadasters/{cadasterId}" {
		t.Fatalf("server-prefix candidate = %#v", op)
	}
	linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{provider})
	if !reflect.DeepEqual(index.Documents[0].Operations[0], op) {
		t.Fatal("repeated candidate linking changed the result")
	}
	other := provider
	other.record.Path = "services/another"
	linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{other, provider})
	op = index.Documents[0].Operations[0]
	if op.LinkStatus != "ambiguous" || op.MatchBasis != "document_path" || len(op.CodeRoutes) != 2 || op.CodeRoutes[0].Project != "services/another" {
		t.Fatalf("candidate ambiguity was hidden: %#v", op)
	}
	full := provider
	full.record.Path = "services/full"
	full.endpoints = []SpringEndpointRecord{{HTTPMethod: "GET", Path: "/api/1.0/cadasters/{id}", File: "src/Full.java", Line: 12}}
	linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{provider, full})
	op = index.Documents[0].Operations[0]
	if op.LinkStatus != "unique_route" || op.MatchBasis != "resolved_path" || len(op.CodeRoutes) != 1 || op.CodeRoutes[0].Project != "services/full" {
		t.Fatalf("full match did not take precedence: %#v", op)
	}
	for _, tc := range []struct{ name, method, route, body string }{
		{"method", "POST", "/cadasters/{id}", "swagger: '2.0'\nbasePath: /api/1.0\npaths: {'/cadasters/{cadasterId}': {get: {responses: {}}}}"},
		{"suffix", "GET", "/cadasters/{id}", "swagger: '2.0'\nbasePath: /api/1.0\npaths: {'/other/cadasters/{cadasterId}': {get: {responses: {}}}}"},
		{"dynamic", "GET", "/cadasters/{id}", "openapi: '3.0.0'\nservers: [{url: '/{stage}'}]\npaths: {'/cadasters/{cadasterId}': {get: {responses: {}}}}"},
		{"alias", "GET", "/cadasters/{id}", "swagger: '2.0'\nbasePath: /api/1.0\npaths: {'/cadasters/{cadasterId}': {get: {responses: &response {}}}}\ndefinitions: {Order: *response}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record, _ := parseAPISpecification("swagger.yaml", []byte(tc.body))
			index := APISpecificationIndexRecord{Documents: []APISpecificationRecord{record}}
			provider.endpoints = []SpringEndpointRecord{{HTTPMethod: tc.method, Path: tc.route, File: "src/Controller.java", Line: 9}}
			linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{provider})
			op := index.Documents[0].Operations[0]
			if op.LinkStatus != "unlinked" || op.MatchBasis != "" || len(op.CodeRoutes) != 0 {
				t.Fatalf("unsafe candidate was linked: %#v", op)
			}
		})
	}
}
