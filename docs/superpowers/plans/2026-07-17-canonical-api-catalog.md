# Canonical API Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete provider-oriented endpoint catalog with separate provider security and consumer call-auth evidence, then expose it as an expandable dashboard view.

**Architecture:** Project builds produce canonical endpoint records from framework analyzers. Workspace reconciliation merges provider inventories with consumer contracts and feature flows, validates stable identities, writes `index/api-catalog.json`, and passes the same model to the dashboard.

**Tech Stack:** Go 1.23 standard library, existing Java/Spring and JavaScript/TypeScript extractors, generated semantic HTML/CSS/vanilla JavaScript, Go tests and optional Playwright.

## Global Constraints

- Execute after `2026-07-17-editable-dashboard-layout-editor.md`.
- Work directly on `main`; use failing-first TDD and focused English commits.
- Keep the source target at unreleased `1.3.0`; do not tag or publish.
- Add no dependency and execute no scanned source or build configuration.
- Keep endpoint security separate from per-consumer call authentication.
- `public` requires explicit evidence; missing evidence is `unknown` and displays `No auth evidence detected`.
- Preserve partial, conflicting, ambiguous, unresolved, and unsupported facts; never guess from names.
- Java/Spring and JavaScript/TypeScript share one language-neutral model while retaining coverage differences.

## File Structure

- Create `internal/scan/api_catalog.go` and `internal/scan/api_catalog_test.go`: canonical records, normalization, stable IDs, validation, sorting.
- Create `internal/scan/security_evidence.go` and `internal/scan/security_evidence_test.go`: normalized security categories and mismatch rules.
- Modify `internal/scan/types.go`: attach outbound auth evidence to API contracts and required parameter metadata.
- Modify `internal/scan/spring_extract.go`, `internal/scan/extract_java.go`, and their tests: provider security evidence.
- Modify `internal/scan/api_contracts.go`, JavaScript/TypeScript extractors, and tests: outbound call-auth evidence.
- Modify `internal/scan/scan.go` and `internal/scan/workspace_reconcile.go`: project/workspace catalog generation and manifests.
- Modify `internal/doctor/doctor.go`; create `internal/doctor/api_catalog_test.go`: catalog integrity.
- Modify dashboard payload/template/script/styles/tests: API Catalog navigation, table, filters, and details.

---

### Task 1: Canonical Catalog Model and Validation

**Files:**
- Create: `internal/scan/api_catalog.go`
- Create: `internal/scan/api_catalog_test.go`

**Interfaces:**
- Produces: `APICatalogRecord`, `APIEndpointRecord`, `APIParameterRecord`, `APIConsumerRecord`, `SecurityEvidenceRecord`, `APIMismatchRecord`.
- Produces: `ValidateAPICatalog(APICatalogRecord) error`, `SortAPICatalog(*APICatalogRecord)`, and `StableAPIEndpointID(provider, transport, method, path, handler, file string, line int) string`.

- [ ] **Step 1: Write failing identity, validation, and ordering tests**

```go
func TestAPICatalogStableIdentityIgnoresDiscoveryOrder(t *testing.T) {
	left := StableAPIEndpointID("services/orders", "http", "GET", "/orders/{id}", "OrderController.get", "src/main/java/OrderController.java", 24)
	right := StableAPIEndpointID("services/orders", "http", "get", "/orders/{orderId}", "OrderController.get", "src/main/java/OrderController.java", 24)
	if left != right { t.Fatalf("equivalent route IDs differ: %q %q", left, right) }
}

func TestValidateAPICatalogRejectsDanglingAndDuplicateConsumers(t *testing.T) {
	catalog := APICatalogRecord{SchemaVersion: SchemaVersion, Endpoints: []APIEndpointRecord{{
		ID: "endpoint:1", ProviderProject: "services/orders", Transport: "http", HTTPMethod: "GET", Path: "/orders/{id}",
		Consumers: []APIConsumerRecord{{ID: "consumer:1", Project: "frontend/web"}, {ID: "consumer:1", Project: "frontend/web"}},
	}}}
	if err := ValidateAPICatalog(catalog); err == nil || !strings.Contains(err.Error(), "duplicate consumer ID") { t.Fatalf("error=%v", err) }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/scan -run 'APICatalog|StableAPIEndpointID' -count=1`

Expected: FAIL because canonical catalog types do not exist.

- [ ] **Step 3: Implement the language-neutral records**

Define these records and keep the fields in this order for deterministic human review:

```go
type APICatalogRecord struct {
	SchemaVersion int `json:"schema_version"`
	Generated string `json:"generated,omitempty"`
	Root string `json:"root,omitempty"`
	Endpoints []APIEndpointRecord `json:"endpoints"`
}
type APIEndpointRecord struct {
	ID string `json:"id"`
	ProviderProject string `json:"provider_project"`
	ProviderService string `json:"provider_service,omitempty"`
	ProviderRole string `json:"provider_role,omitempty"`
	Transport string `json:"transport"`
	HTTPMethod string `json:"http_method"`
	Path string `json:"path"`
	RawPath string `json:"raw_path,omitempty"`
	Language string `json:"language,omitempty"`
	Framework string `json:"framework,omitempty"`
	Controller string `json:"controller,omitempty"`
	Handler string `json:"handler,omitempty"`
	File string `json:"file,omitempty"`
	Line int `json:"line,omitempty"`
	Parameters []APIParameterRecord `json:"parameters,omitempty"`
	Consumes []string `json:"consumes,omitempty"`
	Produces []string `json:"produces,omitempty"`
	RequestType string `json:"request_type,omitempty"`
	ResponseType string `json:"response_type,omitempty"`
	Security []SecurityEvidenceRecord `json:"security"`
	Consumers []APIConsumerRecord `json:"consumers"`
	Mismatches []APIMismatchRecord `json:"mismatches,omitempty"`
	Confidence Confidence `json:"confidence"`
	Coverage Coverage `json:"coverage"`
	Limitations []string `json:"limitations,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}
type APIParameterRecord struct {
	Name string `json:"name"`; Location string `json:"location"`; Type string `json:"type,omitempty"`
	Required bool `json:"required,omitempty"`; Source string `json:"source,omitempty"`; Confidence Confidence `json:"confidence,omitempty"`
}
type SecurityEvidenceRecord struct {
	Kind string `json:"kind"`; Summary string `json:"summary"`; Expression string `json:"expression,omitempty"`
	Source string `json:"source,omitempty"`; File string `json:"file,omitempty"`; Line int `json:"line,omitempty"`
	Confidence Confidence `json:"confidence"`; Conflicting bool `json:"conflicting,omitempty"`
	Limitations []string `json:"limitations,omitempty"`; EvidenceIDs []string `json:"evidence_ids,omitempty"`
}
type APIConsumerRecord struct {
	ID string `json:"id"`; Project string `json:"project"`; Service string `json:"service,omitempty"`; Role string `json:"role,omitempty"`
	Caller string `json:"caller,omitempty"`; File string `json:"file,omitempty"`; Line int `json:"line,omitempty"`
	HTTPMethod string `json:"http_method,omitempty"`; Path string `json:"path,omitempty"`; CallAuth []SecurityEvidenceRecord `json:"call_auth"`
	Resolution string `json:"resolution"`; Confidence Confidence `json:"confidence"`
	Limitations []string `json:"limitations,omitempty"`; EvidenceIDs []string `json:"evidence_ids,omitempty"`
}
type APIMismatchRecord struct {
	ID string `json:"id"`; Kind string `json:"kind"`; Severity string `json:"severity"`; Reason string `json:"reason"`
	Confidence Confidence `json:"confidence"`; EvidenceIDs []string `json:"evidence_ids,omitempty"`
}
```

Define normalized security constants exactly as `basic`, `bearer`, `oauth2`, `api_key`, `session`, `mtls`, `role`, `authenticated`, `public`, and `unknown`.

Normalize only parameter names inside `{...}` for identity while preserving the displayed path. Validate unique non-empty endpoint and consumer IDs, known security kinds, one-based or absent source lines, workspace-relative files, sorted evidence IDs, and deterministic endpoint/consumer/mismatch ordering.

- [ ] **Step 4: Run model tests and verify GREEN**

Run: `gofmt -w internal/scan/api_catalog.go internal/scan/api_catalog_test.go && go test ./internal/scan -run 'APICatalog|StableAPIEndpointID' -count=1`

Expected: PASS and repeated JSON marshaling after input permutation is byte-identical when generated timestamps match.

- [ ] **Step 5: Commit the canonical model**

```bash
git add internal/scan/api_catalog.go internal/scan/api_catalog_test.go
git commit -m "Define canonical API catalog records" -m "- Add stable endpoint, consumer, security, and mismatch identities\n- Validate deterministic language-neutral catalog output"
```

### Task 2: Project Endpoint Catalog Generation

**Files:**
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/scan_test.go`
- Modify: `internal/scan/spring_extract.go`
- Modify: `internal/scan/rich_graph_test.go`

**Interfaces:**
- Consumes: canonical records from Task 1.
- Produces: `BuildProjectAPICatalog(project, generated string, routes []CodeRouteRecord, spring SpringIndex, contracts []APIContractRecord, capabilities []CapabilityRecord) APICatalogRecord`.
- Produces: `goregraph-out/index/api-catalog.json` on every project build target.

- [ ] **Step 1: Write failing complete-inventory test**

```go
func TestBuildProjectAPICatalogIncludesEndpointsWithoutConsumers(t *testing.T) {
	catalog := BuildProjectAPICatalog("orders", "2026-07-17T12:00:00Z", nil, SpringIndex{Endpoints: []SpringEndpointRecord{{
		HTTPMethod: "GET", Path: "/orders/{id}", Controller: "OrderController", Method: "get", File: "src/main/java/OrderController.java", Line: 20,
		RequestType: "OrderQuery", ReturnType: "OrderResponse",
	}}}, nil, nil)
	if len(catalog.Endpoints) != 1 || len(catalog.Endpoints[0].Consumers) != 0 { t.Fatalf("catalog=%#v", catalog) }
}
```

Add an end-to-end scan fixture asserting `manifest.json` lists `index/api-catalog.json` for `agent`, `dashboard`, and `all` targets.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./internal/scan -run 'BuildProjectAPICatalog|APICatalogIncludes' -count=1`

Expected: FAIL because the builder and output file are absent.

- [ ] **Step 3: Map Spring and supported JS/TS provider routes**

Map every `SpringEndpointRecord` to one endpoint, including zero-consumer endpoints. Convert Java parameter annotations into `path`, `query`, `header`, `cookie`, or `body`; retain name, type, required flag, source, and confidence. Map request/return types and consumes. Map supported server `CodeRouteRecord` values for JS/TS only when framework/handler/source evidence is present. Deduplicate identical provider/method/normalized-path/handler identities.

Append `api-catalog.json` to `IndexGeneratedFiles`, build it before manifest creation, write it with other canonical index files, include it in freshness, and add it to Doctor's project JSON decoding list later in Task 6.

- [ ] **Step 4: Run scan and output-layout tests**

Run: `gofmt -w internal/scan/scan.go internal/scan/scan_test.go internal/scan/spring_extract.go internal/scan/rich_graph_test.go && go test ./internal/scan -run 'ProjectAPICatalog|APICatalog|OutputManifest' -count=1`

Expected: PASS for all three build targets.

- [ ] **Step 5: Commit project catalog generation**

```bash
git add internal/scan/scan.go internal/scan/scan_test.go internal/scan/spring_extract.go internal/scan/rich_graph_test.go
git commit -m "Generate project API endpoint catalogs" -m "- Preserve every discovered provider endpoint with typed source evidence\n- Publish the catalog as a canonical index artifact"
```

### Task 3: Provider Security Evidence

**Files:**
- Create: `internal/scan/security_evidence.go`
- Create: `internal/scan/security_evidence_test.go`
- Modify: `internal/scan/spring_extract.go`
- Modify: `internal/scan/extract_java.go`
- Modify: `internal/scan/extract_java_test.go`
- Modify: `internal/scan/rich_graph_test.go`

**Interfaces:**
- Produces: `NormalizeSecurityEvidence(records []AuthRecord) []SecurityEvidenceRecord`.
- Extends: Spring endpoint `Auth` evidence without changing its existing JSON compatibility.

- [ ] **Step 1: Write failing normalization and explicit-public tests**

```go
func TestNormalizeSecurityEvidenceDistinguishesUnknownFromExplicitPublic(t *testing.T) {
	unknown := NormalizeSecurityEvidence(nil)
	if len(unknown) != 1 || unknown[0].Kind != SecurityUnknown { t.Fatalf("unknown=%#v", unknown) }
	public := NormalizeSecurityEvidence([]AuthRecord{{Kind: "permit_all", Source: "security_config_call", Confidence: "EXTRACTED", File: "Security.java", Line: 12}})
	if len(public) != 1 || public[0].Kind != SecurityPublic { t.Fatalf("public=%#v", public) }
}
```

Add table tests for `httpBasic`, bearer/resource-server, OAuth2 login, API-key OpenAPI scheme, form/session login, X.509/mTLS, role/authority, authenticated, and conflicting public/authenticated records.

- [ ] **Step 2: Run security tests and verify RED**

Run: `go test ./internal/scan -run 'SecurityEvidence|ExplicitPublic' -count=1`

Expected: FAIL because normalized security evidence does not exist.

- [ ] **Step 3: Expand extraction while retaining provenance**

Extend Java security call extraction to recognize `permitAll`, `authenticated`, `hasRole`, `hasAnyRole`, `hasAuthority`, `hasAnyAuthority`, `httpBasic`, `oauth2ResourceServer`, `oauth2Login`, `formLogin`, and `x509`. Parse method annotations `PermitAll`, `DenyAll`, `PreAuthorize`, `PostAuthorize`, `Secured`, and `RolesAllowed`. Parse supported OpenAPI `SecurityRequirement` plus `SecurityScheme` evidence only when the scheme type/scheme is explicit.

Normalization maps explicit evidence to the constants from Task 1, keeps expression/source/file/line/confidence, adds a limitation when broader path configuration cannot be tied exactly to one endpoint, returns one `unknown` record for absent evidence, and retains conflicting records instead of collapsing them.

- [ ] **Step 4: Run extractor and catalog tests**

Run: `gofmt -w internal/scan/security_evidence.go internal/scan/security_evidence_test.go internal/scan/spring_extract.go internal/scan/extract_java.go internal/scan/extract_java_test.go internal/scan/rich_graph_test.go && go test ./internal/scan -run 'Security|SpringIndexExtracts.*Auth|APICatalog' -count=1`

Expected: PASS; absence never normalizes to `public`.

- [ ] **Step 5: Commit provider security extraction**

```bash
git add internal/scan/security_evidence.go internal/scan/security_evidence_test.go internal/scan/spring_extract.go internal/scan/extract_java.go internal/scan/extract_java_test.go internal/scan/rich_graph_test.go
git commit -m "Extract endpoint security evidence" -m "- Normalize explicit Spring and OpenAPI security declarations\n- Preserve unknown and conflicting provider requirements honestly"
```

### Task 4: Consumer Call Authentication Evidence

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/api_contracts.go`
- Modify: `internal/scan/api_contracts_test.go`
- Modify: `internal/scan/api_contracts.go`
- Modify: `internal/scan/api_contracts_test.go`

**Interfaces:**
- Extends: `APIContractRecord` with `Auth []AuthRecord json:"auth,omitempty"`.
- Produces: `extractHTTPCallAuth(source string, callStart, callEnd int) []AuthRecord`.

- [ ] **Step 1: Write failing explicit-header tests**

```go
func TestAPIContractsExtractExplicitConsumerAuthentication(t *testing.T) {
	source := `export const load = () => axios.get('/orders', { headers: { Authorization: 'Bearer ' + token, 'X-API-Key': apiKey } });`
	contracts := extractAPIContracts(FileRecord{Path: "src/api/orders.ts", Language: "typescript"}, strings.Split(source, "\n"), nil)
	kinds := map[string]bool{}
	for _, record := range contracts[0].Auth { kinds[record.Kind] = true }
	if !kinds["bearer"] || !kinds["api_key"] {
		t.Fatalf("auth=%#v", contracts[0].Auth)
	}
}
```

Add cases for Basic authorization, `credentials: 'include'`/session, supported OAuth client helpers, an interceptor whose association is partial, and a request without auth that must retain an empty raw list rather than claiming public.

- [ ] **Step 2: Run extractor tests and verify RED**

Run: `go test ./internal/scan -run 'ConsumerAuthentication|APIContractsExtractExplicit' -count=1`

Expected: FAIL because contracts do not carry authentication evidence.

- [ ] **Step 3: Associate explicit call configuration with the contract**

Inspect only the statically bounded call/config expression and supported imported client/interceptor declarations. Emit `AuthRecord` entries for explicit Basic/Bearer authorization, API-key headers/query configuration, credentials/session, and supported OAuth helpers. Preserve source line, extractor source, and `EXTRACTED` or `PARTIAL` confidence. Never store token/header values; store only the scheme and sanitized expression name. Do not infer auth from variable names outside the associated call path.

- [ ] **Step 4: Run script, contract, and privacy regressions**

Run: `gofmt -w internal/scan/types.go internal/scan/api_contracts.go internal/scan/api_contracts_test.go && go test ./internal/scan -run 'APIContract|ConsumerAuthentication|Auth' -count=1`

Expected: PASS and serialized fixtures contain no sample token or API-key value.

- [ ] **Step 5: Commit consumer auth evidence**

```bash
git add internal/scan/types.go internal/scan/api_contracts.go internal/scan/api_contracts_test.go
git commit -m "Capture outbound call authentication evidence" -m "- Associate explicit JS and TypeScript auth configuration with HTTP contracts\n- Keep credential values out of generated output"
```

### Task 5: Workspace Catalog Reconciliation and Mismatches

**Files:**
- Modify: `internal/scan/api_catalog.go`
- Modify: `internal/scan/api_catalog_test.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_reconcile_test.go`
- Modify: `internal/scan/scan.go`

**Interfaces:**
- Produces: `BuildWorkspaceAPICatalog(registry WorkspaceRegistryRecord, projects []workspaceIndexProject, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, generated string) (APICatalogRecord, error)`.
- Produces: `.goregraph-workspace/index/api-catalog.json` and project reconciliation copies.

- [ ] **Step 1: Write failing inventory, consumer, mismatch, and permutation tests**

```go
func TestBuildWorkspaceAPICatalogKeepsProviderInventoryAndAttachesConsumers(t *testing.T) {
	provider := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "services/orders"}, endpoints: []SpringEndpointRecord{
		{HTTPMethod: "GET", Path: "/orders/{id}", Controller: "OrderController", Method: "get", File: "OrderController.java", Line: 10},
		{HTTPMethod: "POST", Path: "/orders", Controller: "OrderController", Method: "create", File: "OrderController.java", Line: 20},
	}}
	consumer := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "frontend/web"}, contracts: []APIContractRecord{{HTTPMethod: "GET", Path: "/orders/42", File: "src/api.ts", Line: 7}}}
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{provider.record, consumer.record}}
	matches := []WorkspaceContractMatchRecord{{
		APIProject: "frontend/web", APIHTTPMethod: "GET", APIPath: "/orders/42", APIFile: "src/api.ts", APILine: 7,
		BackendProject: "services/orders", BackendHTTPMethod: "GET", BackendPath: "/orders/{id}",
		BackendHandler: "OrderController.get", BackendFile: "OrderController.java", BackendLine: 10,
		Issue: "matched", Confidence: "RESOLVED",
	}}
	catalog, err := BuildWorkspaceAPICatalog(registry, []workspaceIndexProject{provider, consumer}, matches, nil, "fixed")
	if err != nil { t.Fatal(err) }
	if len(catalog.Endpoints) != 2 || len(catalog.Endpoints[0].Consumers) != 1 { t.Fatalf("catalog=%#v", catalog) }
}
```

Add table tests for Basic-vs-Bearer mismatch, authenticated-with-no-call-evidence warning, credentials-to-explicit-public informational warning, conflicting provider rules, ambiguous route match, and reverse project discovery order producing identical JSON.

- [ ] **Step 2: Run workspace catalog tests and verify RED**

Run: `go test ./internal/scan -run 'BuildWorkspaceAPICatalog' -count=1`

Expected: FAIL because workspace aggregation is absent.

- [ ] **Step 3: Merge providers first, then attach evidenced consumers**

Build every provider endpoint from indexed `endpoints` and supported provider `routes` before processing matches. Attach consumers by the canonical `WorkspaceContractMatchRecord` resolution, preserving ambiguous candidates rather than assigning them. Use feature-flow details for request/response types only when they match the same provider method/path/handler.

Normalize provider security and consumer call auth separately. Mismatch rules create warning records with kind, severity, reason, confidence, and evidence IDs; missing call auth is worded as incomplete static evidence, not a proven failure. Validate and sort before writing. Add `api-catalog.json` to workspace index files for every build target because both dashboard and agent projections consume the canonical model, and write filtered project reconciliation catalogs to each project index.

- [ ] **Step 4: Run reconciliation, manifest, and deterministic-output tests**

Run: `gofmt -w internal/scan/api_catalog.go internal/scan/api_catalog_test.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/scan/scan.go && go test ./internal/scan -run 'APICatalog|WorkspaceReconcile|Manifest' -count=1`

Expected: PASS with zero-consumer endpoints retained.

- [ ] **Step 5: Commit workspace reconciliation**

```bash
git add internal/scan/api_catalog.go internal/scan/api_catalog_test.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/scan/scan.go
git commit -m "Reconcile workspace API consumers" -m "- Join complete provider inventories with evidenced consumer call sites\n- Report authentication mismatches without runtime certainty"
```

### Task 6: Catalog Integrity in Doctor

**Files:**
- Modify: `internal/doctor/doctor.go`
- Create: `internal/doctor/api_catalog_test.go`

**Interfaces:**
- Consumes: `scan.ValidateAPICatalog`.
- Produces: project/workspace Doctor validation for manifest-listed catalogs.

- [ ] **Step 1: Write failing duplicate, invalid-security, and dangling-evidence tests**

Create a valid project and workspace fixture, corrupt `api-catalog.json` in one distinct way per subtest, run Doctor, and assert the failure begins with `api-catalog` and names the endpoint/consumer ID.

- [ ] **Step 2: Run Doctor tests and verify RED**

Run: `go test ./internal/doctor -run 'APICatalog' -count=1`

Expected: FAIL because Doctor only decodes generic JSON.

- [ ] **Step 3: Validate semantic catalog integrity**

Decode catalog files only when listed by the active manifest, require `SchemaVersion == scan.SchemaVersion`, call `scan.ValidateAPICatalog`, verify catalog evidence IDs against project evidence where available, and verify each workspace provider/consumer project exists in registry. Report one clear failure and stop catalog validation to avoid cascades.

- [ ] **Step 4: Run Doctor suites**

Run: `gofmt -w internal/doctor/doctor.go internal/doctor/api_catalog_test.go && go test ./internal/doctor -count=1`

Expected: PASS.

- [ ] **Step 5: Commit integrity checks**

```bash
git add internal/doctor/doctor.go internal/doctor/api_catalog_test.go
git commit -m "Validate API catalog integrity" -m "- Reject invalid identities, security categories, and project references\n- Include canonical catalog health in Doctor results"
```

### Task 7: API Catalog Dashboard Payload and Navigation

**Files:**
- Modify: `internal/scan/workspace_dashboard.go`
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_test.go`
- Modify: call sites in `internal/scan/workspace_reconcile.go`.

**Interfaces:**
- Consumes: `APICatalogRecord` from Task 5.
- Extends: `workspaceDashboardPayload` with `APICatalog APICatalogRecord json:"api_catalog"`.
- Produces navigation order `Architecture`, `API Catalog`, `Endpoints`, `Feature Flow`, `Data Flow`, `Code Explorer`, `Diagnostics`, `Coverage`.

- [ ] **Step 1: Write failing payload and navigation tests**

```go
func TestWorkspaceDashboardEmbedsCompleteAPICatalogAndKeepsEndpoints(t *testing.T) {
	catalogFixture := APICatalogRecord{SchemaVersion: SchemaVersion, Endpoints: []APIEndpointRecord{{
		ID: "endpoint:orders", ProviderProject: "services/orders", Transport: "http", HTTPMethod: "POST", Path: "/orders",
		Security: []SecurityEvidenceRecord{{Kind: SecurityUnknown, Summary: "No auth evidence detected"}},
	}}}
	html := renderWorkspaceDashboardHTML(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, catalogFixture,
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion}, WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion}, nil,
	)
	architecture := strings.Index(html, `data-view-mode="architecture"`)
	catalog := strings.Index(html, `data-view-mode="api-catalog"`)
	endpoints := strings.Index(html, `data-view-mode="endpoints"`)
	if !(architecture < catalog && catalog < endpoints) { t.Fatalf("navigation order incorrect") }
	for _, want := range []string{`"api_catalog"`, `POST`, `/orders`, `No auth evidence detected`} {
		if !strings.Contains(html, want) { t.Fatalf("payload missing %q", want) }
	}
}
```

- [ ] **Step 2: Run dashboard payload tests and verify RED**

Run: `go test ./internal/scan -run 'CompleteAPICatalog|APICatalog.*Navigation' -count=1`

Expected: FAIL because payload and view button are absent.

- [ ] **Step 3: Pass the canonical catalog through unchanged**

Add the payload field and function parameters through `buildWorkspaceDashboardArtifacts`, renderer helpers, and reconciliation. The internal renderer signature becomes `renderWorkspaceDashboardHTML(graph WorkspaceGraphRecord, serviceMap WorkspaceServiceMapRecord, endpointTraces WorkspaceEndpointTraceIndexRecord, apiCatalog APICatalogRecord, symbolIndex WorkspaceSymbolIndexRecord, symbolUsages WorkspaceSymbolUsageIndexRecord, codeUsageAssets map[string]string) string`. Do not derive rows from endpoint traces. Add the API Catalog button second, its provider selector/filter shell, and distinct mode help. Keep existing Endpoints IDs, state, filters, and trace behavior unchanged.

- [ ] **Step 4: Run payload, offline, and existing Endpoints tests**

Run: `gofmt -w internal/scan/workspace_dashboard.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_test.go internal/scan/workspace_reconcile.go && go test ./internal/scan -run 'WorkspaceDashboard.*(APICatalog|Endpoint|Offline|Payload)' -count=1`

Expected: PASS and the standalone HTML still requires no network resource.

- [ ] **Step 5: Commit the payload boundary**

```bash
git add internal/scan/workspace_dashboard.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_test.go internal/scan/workspace_reconcile.go
git commit -m "Add API Catalog dashboard view" -m "- Pass the canonical endpoint inventory into the offline dashboard\n- Keep the existing trace-oriented Endpoints view intact"
```

### Task 8: Expandable API Catalog Workbench

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `renderAPICatalog`, `filteredAPICatalogEndpoints`, `apiCatalogSecurityLabel`, and `apiCatalogConsumerSummary` JavaScript functions.

- [ ] **Step 1: Write failing semantic table, filters, and expansion tests**

Assert generated HTML/JS includes a searchable provider `<select>`, text/method/security/consumer/status filters, one `<details>` row per visible endpoint, separate `Endpoint security` and `Consumer call authentication` headings, source actions, parameter/request/response sections, explicit empty/partial states, and no guessed public label.

Add Playwright behavior that selects a provider, filters `GET`, expands a row, verifies consumer/auth cells, clears filters, and confirms existing Endpoints trace navigation still works.

- [ ] **Step 2: Run workbench tests and verify RED**

Run: `go test ./internal/scan -run 'APICatalog.*(Workbench|Filter|Details)' -count=1`

Expected: FAIL because the workbench renderer is absent.

- [ ] **Step 3: Implement compact inventory with Swagger-like details**

Add state fields `apiCatalogService`, `apiCatalogQuery`, method/security/consumer/status sets, and expanded endpoint IDs. Render provider options from catalog endpoints. Collapsed rows show method/path, handler, endpoint security, consumer count/names, consumer auth summary, confidence/status, and warnings. Expanded `<details>` renders parameters grouped by location, media types, request/response identities, handler/source, provider security evidence, individual consumer call sites/auth, mismatches, and coverage limitations.

Use semantic table-like CSS grid at desktop and labeled stacked cells below 820px. Keep row text at normal browser scale, wrap paths, maintain 44px controls on compact widths, add focus-visible styling, and use `aria-live` only for the filter result summary.

- [ ] **Step 4: Run JS parse, dashboard, and responsive browser tests**

Run: `gofmt -w internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go && go test ./internal/scan -run 'WorkspaceDashboard.*(APICatalog|Endpoint)|EmbeddedJavaScript' -count=1`

Expected: PASS at 1280×720, 1440×900, and 1920×1080 without page-level horizontal scrolling.

- [ ] **Step 5: Commit the workbench**

```bash
git add internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "Render expandable API Catalog details" -m "- Add provider-first endpoint filtering and compact consumer summaries\n- Separate endpoint security from each consumer authentication path"
```

### Task 9: API Catalog Integration Verification

**Files:**
- Modify only focused files when a verified regression is found.

- [ ] **Step 1: Run extractor, reconciliation, Doctor, and dashboard suites**

Run: `go test ./internal/scan ./internal/doctor -count=1`

Expected: PASS.

- [ ] **Step 2: Run the complete race-free regression suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Inspect a representative Java/Spring plus JS/TS fixture**

Verify the generated JSON contains every Spring endpoint, the selected JS/TS consumer, separate normalized auth arrays, raw evidence without credentials, deterministic IDs, and explicit unknown/coverage records.

- [ ] **Step 4: Visually inspect both endpoint views**

At 1280, 1440, and 1920 widths, confirm API Catalog is provider-oriented and expandable while Endpoints remains caller-to-provider trace-oriented. Verify long paths, many consumers, unknown security, conflict warnings, and empty consumer lists.

- [ ] **Step 5: Commit only verified corrections**

Do not create an empty commit. Any correction gets a focused English commit naming the regression and its test.
