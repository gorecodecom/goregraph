# Semantic Context Evidence Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GoreGraph preserve the correct primary endpoint while treating operational evidence as complete only when the selected source actually answers every requested authentication, configuration, resilience, persistence, side-effect, and test facet.

**Architecture:** Correct the scanner's Spring Security path scoping first, then carry client authentication and retry signals into the compact agent index. Add internal evidence facets to the existing concern/source selector without changing the public JSON schema. When all facets do not fit the 4,000-token pack, return deterministic path-bound omissions so the agent may perform only the few targeted source reads needed to finish the analysis.

**Tech Stack:** Go 1.26, standard library only, existing Java/Spring scanners, deterministic Context Pack selector, table-driven Go tests, shell benchmark harness.

## Global Constraints

- Keep `DefaultContextBudgetTokens = 4000`, `DefaultContextMaxFiles = 12`, `MaxContextSourceSections = 12`, and `MaxContextSourceOmissions = 3`.
- Preserve `DELETE /cadasters/{cadasterId}/regulations/{objectId:.+}` as the benchmark entrypoint for every German and English query variant.
- Endpoint selection, provider selection, and explicit route/symbol handling are immutable inputs to evidence planning; evidence candidates must never rerank the primary endpoint.
- `source_coverage: complete` means every required internal evidence facet is backed by verified rendered source.
- If a required facet does not fit, return `partial` or `none` with an exact project/path omission whenever an indexed candidate file exists.
- Keep the public Context Pack JSON shape and schema version unchanged.
- Do not hard-code Weka repository names, class names, route fragments, property names, or benchmark wording in production code.
- Preserve deterministic output under reversed fact, edge, contract, and source-candidate order.
- Add no dependencies.
- Make separate English commits for scanner truth, compact-index signals, evidence facets, and bounded omissions.
- Do not release.

---

### Task 1: Correct nested Spring Security matcher scoping

**Files:**

- Modify: `internal/scan/spring_extract.go:1056-1149`
- Test: `internal/scan/extract_java_test.go:367-465`

**Interfaces:**

- Consumes: `javaCallArguments(string, int) []string`, `javaResolvedPathAlternatives(string, map[string]string, map[string]string, map[string]bool, int) []string`
- Produces: `springSecurityMatcherExpressions(string) []string`

- [ ] **Step 1: Write the failing nested-matcher regression test**

Add this test next to `TestBuildSpringIndexScopesSecurityFilterChainsByPath`:

```go
func TestBuildSpringIndexScopesPathPatternSecurityMatchers(t *testing.T) {
	routes := extractJavaSource(FileRecord{
		Path: "src/main/java/example/Routes.java", Language: "java",
	}, `package example;
final class Routes {
  static final String PUBLIC = "/api";
  static final String INTERNAL = "/management";
}`)
	application := extractJavaSource(FileRecord{
		Path: "src/main/java/example/Application.java", Language: "java",
	}, `package example;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.servlet.util.matcher.PathPatternRequestMatcher;

@RestController
class Controller {
  @GetMapping("/api/jobs")
  String jobs() { return "ok"; }

  @DeleteMapping("/management/jobs/{id}")
  void deleteJob() {}
}

@Configuration
class Security {
  @Order(1)
  SecurityFilterChain publicApi(HttpSecurity http) {
    return http
      .securityMatcher(PathPatternRequestMatcher.withDefaults().matcher(Routes.PUBLIC + "/**"))
      .authorizeHttpRequests(authorize -> authorize.anyRequest().authenticated())
      .oauth2ResourceServer(oauth2 -> oauth2.jwt(Customizer.withDefaults()))
      .build();
  }

  @Order(2)
  SecurityFilterChain management(HttpSecurity http) {
    return http
      .securityMatcher(Routes.INTERNAL + "/**")
      .authorizeHttpRequests(authorize -> authorize.anyRequest().hasRole("TECHNICAL_USER"))
      .httpBasic(Customizer.withDefaults())
      .build();
  }
}`)

	index := buildSpringIndex([]JavaSourceRecord{application, routes})
	public, ok := findSpringEndpointForTest(index.Endpoints, "GET", "/api/jobs")
	if !ok || !hasAuthKind(public.Auth, "oauth2_resource_server") ||
		hasAuthKind(public.Auth, "http_basic") {
		t.Fatalf("public endpoint security = %#v", public.Auth)
	}
	internal, ok := findSpringEndpointForTest(index.Endpoints, "DELETE", "/management/jobs/{id}")
	if !ok || !hasAuthKind(internal.Auth, "http_basic") ||
		hasAuthKind(internal.Auth, "oauth2_resource_server") {
		t.Fatalf("management endpoint security = %#v", internal.Auth)
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
go test ./internal/scan -run TestBuildSpringIndexScopesPathPatternSecurityMatchers -count=1
```

Expected: `FAIL`; the nested `PathPatternRequestMatcher...matcher(...)` expression is unresolved and the higher-priority bearer chain leaks onto the management endpoint.

- [ ] **Step 3: Add a conservative matcher-expression unwrapping helper**

Add to `internal/scan/spring_extract.go`:

```go
func springSecurityMatcherExpressions(argument string) []string {
	argument = strings.TrimSpace(argument)
	marker := strings.LastIndex(argument, ".matcher(")
	if marker < 0 {
		return []string{argument}
	}
	open := marker + len(".matcher")
	arguments := javaCallArguments(argument, open)
	if len(arguments) != 1 {
		return []string{argument}
	}
	close := matchingJavaParen(argument, open)
	if close != len(argument)-1 {
		return []string{argument}
	}
	return []string{strings.TrimSpace(arguments[0])}
}
```

This unwraps only a complete, one-argument `.matcher(...)` expression. Dynamic, multi-argument, or partially parsed expressions remain unresolved and therefore retain `PARTIAL` confidence.

- [ ] **Step 4: Resolve unwrapped matcher expressions**

Replace the direct `javaResolvedPathAlternatives(argument, ...)` call inside `springAuthScopes` with:

```go
for _, expression := range springSecurityMatcherExpressions(argument) {
	alternatives := javaResolvedPathAlternatives(expression, constants, nil, nil, 0)
	if len(alternatives) == 0 {
		unresolvedMatcher = true
		continue
	}
	for _, alternative := range alternatives {
		path, ok := springSecurityMatcherPath(alternative)
		if !ok {
			unresolvedMatcher = true
			continue
		}
		scope.Paths = append(scope.Paths, path)
	}
}
```

- [ ] **Step 5: Add negative parser cases**

Add a table test for:

```go
func TestSpringSecurityMatcherExpressionsRemainConservative(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{`Routes.INTERNAL + "/**"`, []string{`Routes.INTERNAL + "/**"`}},
		{`factory.matcher(Routes.INTERNAL + "/**")`, []string{`Routes.INTERNAL + "/**"`}},
		{`factory.matcher(a, b)`, []string{`factory.matcher(a, b)`}},
		{`factory.matcher(dynamic()).orElse(other)`, []string{`factory.matcher(dynamic()).orElse(other)`}},
	}
	for _, test := range tests {
		if got := springSecurityMatcherExpressions(test.input); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%q => %#v, want %#v", test.input, got, test.want)
		}
	}
}
```

Add `reflect` to the test imports.

- [ ] **Step 6: Run focused and package tests**

Run:

```bash
go test ./internal/scan -run 'TestBuildSpringIndexScopes(PathPattern)?SecurityFilterChainsByPath|TestBuildSpringIndexScopesPathPatternSecurityMatchers|TestSpringSecurityMatcherExpressionsRemainConservative' -count=1
go test ./internal/scan -count=1
```

Expected: all tests pass; direct and wrapped matchers remain deterministic.

- [ ] **Step 7: Commit**

Commit:

```text
Resolve nested Spring security matchers

- Unwrap complete PathPatternRequestMatcher expressions
- Keep dynamic matcher scopes partial and conservative
- Prevent higher-priority security chains from leaking across paths
```

---

### Task 2: Preserve client authentication and retry signals in the agent index

**Files:**

- Modify: `internal/scan/agent_context_index.go:642-675`
- Test: `internal/scan/agent_context_index_test.go`

**Interfaces:**

- Consumes: `APIContractRecord.Auth`, `APIContractRecord.Reason`
- Produces: `compactContractAuthKinds([]AuthRecord) []string`
- Preserves: `AgentContextFactRecord` public shape

- [ ] **Step 1: Write a failing compact-index signal test**

Add:

```go
func TestProjectAgentContextIndexPreservesContractOperationalSignals(t *testing.T) {
	index := BuildProjectAgentContextIndex(
		"libraries/jobs",
		"fixed",
		nil,
		nil,
		[]RichSymbolRecord{{
			ID: "client", Name: "JobMgmtClient",
			Qualified: "example.JobMgmtClient",
			Kind: "class", File: "src/JobMgmtClient.java", Line: 10,
		}},
		nil,
		nil,
		[]APIContractRecord{{
			HTTPMethod: "GET",
			Caller:     "JobMgmtClient.getJobs",
			File:       "src/JobMgmtClient.java",
			Line:       42,
			Confidence: "PARTIAL",
			Reason:     "spring RestClient receiver with unresolved dynamic path; retryable method",
			Auth: []AuthRecord{{
				Kind: "basic", Source: "spring_client_interceptor", Confidence: "EXTRACTED",
			}},
		}},
		nil,
		nil,
	)

	fact := findContextFact(index.Facts, "api_contract", "GET /")
	if !strings.Contains(fact.Summary, "auth basic") ||
		!strings.Contains(fact.Summary, "retryable") ||
		!strings.Contains(fact.Search, "basic") ||
		!strings.Contains(fact.Search, "retryable") {
		t.Fatalf("contract operational signals = %#v", fact)
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
go test ./internal/scan -run TestProjectAgentContextIndexPreservesContractOperationalSignals -count=1
```

Expected: `FAIL`; current compact contract facts omit authentication and extraction reason.

- [ ] **Step 3: Add bounded authentication labels**

Add:

```go
func compactContractAuthKinds(auth []AuthRecord) []string {
	kinds := make([]string, 0, len(auth))
	for _, record := range auth {
		kind := strings.ToLower(strings.TrimSpace(record.Kind))
		if compactCatalogConsumerAuthKind(kind) {
			kinds = append(kinds, kind)
		}
	}
	sort.Strings(kinds)
	return orderedContextStrings(kinds)
}
```

- [ ] **Step 4: Enrich compact contract summary and search**

In `addAPIContractFacts`, build the summary deterministically:

```go
summaryParts := []string{}
if contract.ServiceCandidate != "" {
	summaryParts = append(summaryParts, "calls "+contract.ServiceCandidate)
}
if auth := compactContractAuthKinds(contract.Auth); len(auth) > 0 {
	summaryParts = append(summaryParts, "auth "+strings.Join(auth, ", "))
}
reason := compactCatalogValue(strings.TrimSpace(contract.Reason))
if reason != "" {
	summaryParts = append(summaryParts, reason)
}
summary := strings.Join(summaryParts, "; ")
```

Add `summary`, `reason`, and the auth kinds to `compactContextSearch`:

```go
authKinds := compactContractAuthKinds(contract.Auth)
Search: compactContextSearch(
	method,
	contractPath,
	contract.Caller,
	contract.ServiceCandidate,
	strings.Join(authKinds, " "),
	reason,
	contextFileBase(file),
	contextFileStem(file),
),
```

- [ ] **Step 5: Prove credentials never enter the compact index**

Add a contract auth record with:

```go
AuthRecord{
	Kind: "basic", Expression: "service-user,super-secret",
	Source: "spring_client_interceptor", Confidence: "EXTRACTED",
}
```

Assert:

```go
body, err := json.Marshal(index)
if err != nil {
	t.Fatal(err)
}
if strings.Contains(string(body), "service-user") ||
	strings.Contains(string(body), "super-secret") {
	t.Fatalf("compact context leaked credential expressions: %s", body)
}
```

Add `encoding/json` to the test imports.

- [ ] **Step 6: Run focused and package tests**

Run:

```bash
go test ./internal/scan -run 'TestProjectAgentContextIndexPreservesContractOperationalSignals|TestBuildProjectAgentContextIndex' -count=1
go test ./internal/scan -count=1
```

Expected: all tests pass and serialized output contains auth kinds but no credential expressions.

- [ ] **Step 7: Commit**

Commit:

```text
Preserve contract operational signals

- Add bounded authentication kinds to compact contract facts
- Retain retry and resolution reasons for source planning
- Exclude credential expressions from agent context
```

---

### Task 3: Plan evidence facets without changing the public schema

**Files:**

- Modify: `internal/agent/context_intent.go:20-365`
- Modify: `internal/agent/context_select.go:155-272`
- Test: `internal/agent/context_test.go`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Extends: `contextConcern` with `publicKey string` and `facet string`
- Produces: `newContextEvidenceConcern(contextConcern, string, []string, string) contextConcern`
- Produces: `expandContextEvidenceConcerns(ContextPack, scan.AgentContextIndexRecord, []contextConcern) []contextConcern`
- Preserves: `ContextConcern` JSON fields and `contextPublicConcernKey`

- [ ] **Step 1: Write failing public-aggregation tests**

Add:

```go
func TestApplyContextSourceCoverageRequiresEveryInternalFacet(t *testing.T) {
	base := newContextConcern(
		contextConcernPersistence,
		"services/jobs",
		true,
		[]string{"regular-repository", "change-repository"},
		"requested persistence",
	)
	regular := newContextEvidenceConcern(
		base, "model:regular-job", []string{"regular-repository"}, "regular job persistence",
	)
	change := newContextEvidenceConcern(
		base, "model:change-job", []string{"change-repository"}, "change job persistence",
	)
	pack := ContextPack{
		Concerns: []ContextConcern{{
			Kind: contextConcernPersistence, Project: "services/jobs",
		}},
		SourceSections: []ContextSourceSection{{Project: "services/jobs", Path: "RegularRepository.java"}},
	}

	applyContextSourceCoverage(
		&pack,
		[]contextConcern{regular, change},
		map[string]bool{regular.key: true},
	)
	if pack.SourceCoverage != "partial" || pack.Concerns[0].Covered {
		t.Fatalf("one of two persistence facets reported complete: %#v", pack)
	}

	applyContextSourceCoverage(
		&pack,
		[]contextConcern{regular, change},
		map[string]bool{regular.key: true, change.key: true},
	)
	if pack.SourceCoverage != "complete" || !pack.Concerns[0].Covered {
		t.Fatalf("all persistence facets did not aggregate: %#v", pack)
	}
}
```

Also add a language-regression test for the exact evidence vocabulary used by
the benchmark:

```go
func TestContextEvidenceFacetsRecognizeBenchmarkLanguage(t *testing.T) {
	tokens := contextExpandedTokenSet(
		"Retry-Logik und Fehlerbehandlung sowie Protokollierung, E-Mail und Benutzerinformationen",
	)
	for _, test := range []struct {
		kind  string
		facet string
	}{
		{contextConcernResilience, "retry_policy"},
		{contextConcernResilience, "recovery"},
		{contextConcernSideEffects, "mail"},
		{contextConcernSideEffects, "audit"},
		{contextConcernSideEffects, "user_information"},
	} {
		if !contextEvidenceFacetRequested(test.kind, test.facet, tokens) {
			t.Errorf("benchmark vocabulary missed %s#%s", test.kind, test.facet)
		}
	}

	english := contextExpandedTokenSet("mail, audit, and user information")
	if !contextEvidenceFacetRequested(
		contextConcernSideEffects,
		"user_information",
		english,
	) {
		t.Fatal("English user information phrase was not recognized")
	}
}
```

- [ ] **Step 2: Run the aggregation test and verify RED**

Run:

```bash
go test ./internal/agent -run 'TestApplyContextSourceCoverageRequiresEveryInternalFacet|TestContextEvidenceFacetsRecognizeBenchmarkLanguage' -count=1
```

Expected: compile failure because internal facets and `publicKey` do not exist.

- [ ] **Step 3: Extend the internal concern type**

Change the type and constructor:

```go
type contextConcern struct {
	key              string
	publicKey        string
	facet            string
	kind             string
	project          string
	required         bool
	candidateFactIDs []string
	reason           string
	rank             int
}

func newContextConcern(
	kind, project string,
	required bool,
	candidateFactIDs []string,
	reason string,
) contextConcern {
	project = normalizeContextProject(project)
	key := kind
	if project != "" {
		key += ":" + project
	}
	return contextConcern{
		key: key, publicKey: key, kind: kind, project: project,
		required: required,
		candidateFactIDs: orderedContextConcernIDs(candidateFactIDs),
		reason: strings.TrimSpace(reason),
	}
}

func newContextEvidenceConcern(
	base contextConcern,
	facet string,
	candidateFactIDs []string,
	reason string,
) contextConcern {
	base.publicKey = firstNonEmptyContext(base.publicKey, base.key)
	base.facet = strings.TrimSpace(facet)
	base.key = base.publicKey + "#" + base.facet
	base.candidateFactIDs = orderedContextConcernIDs(candidateFactIDs)
	base.reason = strings.TrimSpace(reason)
	return base
}
```

- [ ] **Step 4: Keep public concern serialization aggregated**

In `publicContextConcerns`, deduplicate by `publicKey`, not internal facet key:

```go
appendConcern := func(concern contextConcern) {
	publicKey := firstNonEmptyContext(concern.publicKey, concern.key)
	if len(selected) >= maximumPublicContextConcerns || selectedKeys[publicKey] {
		return
	}
	concern.key = publicKey
	concern.publicKey = publicKey
	concern.facet = ""
	selected = append(selected, concern)
	selectedKeys[publicKey] = true
}
```

The serialized `ContextConcern` remains `{kind, project, covered, reason}`.

- [ ] **Step 5: Add deterministic facet vocabulary**

Add:

```go
var contextEvidenceFacetVocabulary = map[string]map[string][]string{
	contextConcernConfiguration: {
		"binding":  {"config", "configuration", "konfiguration", "properties"},
		"consumer": {"client", "consumer", "vertrag", "contract"},
	},
	contextConcernResilience: {
		"retry_policy": {"retry", "wiederholung"},
		"recovery":     {"error", "exception", "fehler", "fehlerbehandlung", "recover", "recovery"},
	},
	contextConcernSideEffects: {
		"mail":             {"email", "mail", "mails"},
		"audit":            {"audit", "logging", "protocol", "protokollierung", "tracking"},
		"user_information": {"benutzerinformation", "benutzerinformationen", "user_information", "userinfo"},
	},
}

func contextEvidenceFacetRequested(
	kind string,
	facet string,
	queryTokens map[string]bool,
) bool {
	for _, token := range contextEvidenceFacetVocabulary[kind][facet] {
		if queryTokens[token] {
			return true
		}
	}
	return kind == contextConcernSideEffects &&
		facet == "user_information" &&
		queryTokens["user"] &&
		queryTokens["information"]
}
```

Authentication remains project-scoped: selected contract projects represent client transport authentication, while endpoint and requested-model projects represent server policy. Configuration and resilience facets are required only in selected contract projects. English and German aliases already normalized by `contextExpandedTokenSet` must produce identical facet keys.

- [ ] **Step 6: Select task-relevant operational projects**

Add:

```go
func contextEvidenceProjectRoles(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
) (map[string]bool, map[string]bool, map[string]bool) {
	endpointProjects := map[string]bool{}
	for _, endpoint := range pack.Endpoints {
		endpointProjects[normalizeContextProject(endpoint.Provider)] = true
	}
	contractProjects := map[string]bool{}
	for _, contract := range pack.Contracts {
		contractProjects[normalizeContextProject(contract.Project)] = true
	}
	modelProjects := map[string]bool{}
	requestedModels := contextRequestedDomainModelIDs(pack, index)
	for _, fact := range index.Facts {
		if requestedModels[fact.ID] {
			modelProjects[normalizeContextProject(fact.Project)] = true
		}
	}
	return endpointProjects, contractProjects, modelProjects
}

func contextRequiredEvidenceConcern(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	concern contextConcern,
) bool {
	endpointProjects, contractProjects, modelProjects :=
		contextEvidenceProjectRoles(pack, index)
	switch concern.kind {
	case contextConcernAuth:
		return endpointProjects[concern.project] ||
			contractProjects[concern.project] ||
			modelProjects[concern.project]
	case contextConcernConfiguration, contextConcernResilience, contextConcernHTTPContract:
		return contractProjects[concern.project]
	case contextConcernPersistence, contextConcernSideEffects, contextConcernTests:
		return modelProjects[concern.project]
	default:
		return concern.required
	}
}
```

When `contextSourceConcerns` appends a planned concern that was not serialized in `pack.Concerns`, replace `concern.required = false` with:

```go
concern.required = contextRequiredEvidenceConcern(pack, index, concern)
```

This prevents unrelated explicit projects from multiplying required evidence while retaining the client, server, model-owner, and public-entrypoint boundaries needed by the task.

- [ ] **Step 7: Expand authentication, configuration, resilience, persistence, and side-effect facets**

Implement:

```go
func expandContextEvidenceConcerns(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	concerns []contextConcern,
) []contextConcern {
	queryTokens := contextExpandedTokenSet(contextSelectionQuery(pack))
	requestedModels := contextRequestedDomainModelIDs(pack, index)
	endpointProjects, contractProjects, modelProjects :=
		contextEvidenceProjectRoles(pack, index)
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	contractFactIDs := map[string][]string{}
	for _, contract := range pack.Contracts {
		project := normalizeContextProject(contract.Project)
		contractFactIDs[project] = append(contractFactIDs[project], contract.ID)
	}

	result := make([]contextConcern, 0, len(concerns)+len(requestedModels))
	for _, concern := range concerns {
		switch concern.kind {
		case contextConcernAuth:
			added := false
			if contractProjects[concern.project] {
				candidates := orderedContextConcernIDs(append(
					append([]string(nil), concern.candidateFactIDs...),
					contractFactIDs[concern.project]...,
				))
				result = append(result, newContextEvidenceConcern(
					concern, "client_transport", candidates,
					"client transport authentication",
				))
				added = true
			}
			if endpointProjects[concern.project] || modelProjects[concern.project] {
				result = append(result, newContextEvidenceConcern(
					concern, "server_policy", concern.candidateFactIDs,
					"server authentication policy",
				))
				added = true
			}
			if !added {
				result = append(result, concern)
			}
		case contextConcernConfiguration:
			if !contractProjects[concern.project] {
				result = append(result, concern)
				continue
			}
			result = append(
				result,
				newContextEvidenceConcern(
					concern, "binding", concern.candidateFactIDs,
					"client configuration binding",
				),
				newContextEvidenceConcern(
					concern,
					"consumer",
					orderedContextConcernIDs(append(
						append([]string(nil), concern.candidateFactIDs...),
						contractFactIDs[concern.project]...,
					)),
					"client configuration consumption",
				),
			)
		case contextConcernResilience:
			if !contractProjects[concern.project] {
				result = append(result, concern)
				continue
			}
			candidates := orderedContextConcernIDs(append(
				append([]string(nil), concern.candidateFactIDs...),
				contractFactIDs[concern.project]...,
			))
			added := false
			for _, facet := range []string{"retry_policy", "recovery"} {
				if !contextEvidenceFacetRequested(
					contextConcernResilience,
					facet,
					queryTokens,
				) {
					continue
				}
				reason := "client retry policy"
				if facet == "recovery" {
					reason = "client recovery behavior"
				}
				result = append(result, newContextEvidenceConcern(
					concern, facet, candidates, reason,
				))
				added = true
			}
			if !added {
				result = append(result, concern)
			}
		case contextConcernPersistence:
			modelIDs := make([]string, 0, len(requestedModels))
			for modelID := range requestedModels {
				model := factByID[modelID]
				if concern.project == "" ||
					normalizeContextProject(model.Project) == concern.project {
					modelIDs = append(modelIDs, modelID)
				}
			}
			sort.Strings(modelIDs)
			if len(modelIDs) == 0 {
				result = append(result, concern)
				continue
			}
			domainTokens := contextSourceDomainModelTokens(pack, index)
			for _, modelID := range modelIDs {
				candidates := []string{}
				for _, factID := range concern.candidateFactIDs {
					fact, ok := factByID[factID]
					if ok && contextPersistenceMatchesDomainModel(
						index,
						fact,
						domainTokens,
						map[string]bool{modelID: true},
					) {
						candidates = append(candidates, factID)
					}
				}
				result = append(result, newContextEvidenceConcern(
					concern,
					"model:"+modelID,
					candidates,
					"persistence for requested model "+factByID[modelID].Name,
				))
			}
		case contextConcernSideEffects:
			if concern.project != "" && !modelProjects[concern.project] {
				result = append(result, concern)
				continue
			}
			added := false
			facets := contextEvidenceFacetVocabulary[contextConcernSideEffects]
			names := make([]string, 0, len(facets))
			for name := range facets {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				if !contextEvidenceFacetRequested(
					contextConcernSideEffects,
					name,
					queryTokens,
				) {
					continue
				}
				result = append(result, newContextEvidenceConcern(
					concern, name, concern.candidateFactIDs, "requested "+name+" side effects",
				))
				added = true
			}
			if !added {
				result = append(result, concern)
			}
		default:
			result = append(result, concern)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].key < result[j].key })
	return result
}
```

Call it at the end of `contextSourceConcerns`.

- [ ] **Step 8: Aggregate coverage by public key**

Replace `applyContextSourceCoverage` with aggregation that requires all internal facets:

```go
publicCovered := map[string]bool{}
publicSeen := map[string]bool{}
requiredMissing := false
for _, concern := range concerns {
	if !concern.required {
		continue
	}
	publicKey := firstNonEmptyContext(concern.publicKey, concern.key)
	if !publicSeen[publicKey] {
		publicSeen[publicKey] = true
		publicCovered[publicKey] = true
	}
	if !covered[concern.key] {
		publicCovered[publicKey] = false
		requiredMissing = true
	}
}
for index := range pack.Concerns {
	key := contextPublicConcernKey(pack.Concerns[index])
	if publicSeen[key] {
		pack.Concerns[index].Covered = publicCovered[key]
	} else {
		pack.Concerns[index].Covered = covered[key]
	}
}
```

Keep the existing `none`, `partial`, and `complete` decision after this aggregation.

- [ ] **Step 9: Run focused and package tests**

Run:

```bash
go test ./internal/agent -run 'TestApplyContextSourceCoverageRequiresEveryInternalFacet|TestContextEvidenceFacetsRecognizeBenchmarkLanguage|TestPlanContextConcerns|TestPublicContextConcerns' -count=1
go test ./internal/agent -count=1
```

Expected: all tests pass; public JSON remains unchanged and incomplete facets produce `partial`.

- [ ] **Step 10: Commit**

Commit:

```text
Require complete semantic evidence facets

- Track persistence per requested domain model
- Require client/server auth plus configuration and retry/recovery evidence at relevant boundaries
- Track mail, audit, and user-information side effects independently
- Aggregate internal facets into the unchanged public concern schema
```

---

### Task 4: Bind facet coverage to rendered source and exact omissions

**Files:**

- Modify: `internal/agent/context_select.go:847-965`
- Modify: `internal/agent/context_select.go:2178-2257`
- Test: `internal/agent/context_source_test.go`
- Test: `internal/agent/context_change_analysis_test.go`

**Interfaces:**

- Produces: `contextSourceSectionSupportsEvidence(ContextSourceSection, contextConcern) bool`
- Produces: `contextSourceEvidenceOmissions([]contextConcern, []sourceCandidate, map[string]string, map[string]bool) []ContextSourceOmission`
- Preserves: `contextSourceSectionSupportsConcern`

- [ ] **Step 1: Write failing facet-specific rendered-source tests**

Add:

```go
func TestContextSourceSectionSupportsOnlyMatchingEvidenceFacet(t *testing.T) {
	base := newContextConcern(
		contextConcernSideEffects,
		"services/jobs",
		true,
		[]string{"delete-job"},
		"requested side effects",
	)
	mail := newContextEvidenceConcern(base, "mail", []string{"delete-job"}, "mail")
	audit := newContextEvidenceConcern(base, "audit", []string{"delete-job"}, "audit")
	user := newContextEvidenceConcern(base, "user_information", []string{"delete-job"}, "user")
	section := ContextSourceSection{
		Project: "services/jobs",
		Role: "call_chain",
		RenderMode: "declaration_body",
		Content: `void deleteJob() {
  mailService.sendDeletedMail(job);
}`,
	}
	if !contextSourceSectionSupportsEvidence(section, mail) {
		t.Fatal("mail evidence was rejected")
	}
	if contextSourceSectionSupportsEvidence(section, audit) ||
		contextSourceSectionSupportsEvidence(section, user) {
		t.Fatal("mail-only source covered audit or user information")
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
go test ./internal/agent -run TestContextSourceSectionSupportsOnlyMatchingEvidenceFacet -count=1
```

Expected: compile failure because facet-specific rendered evidence is not implemented.

- [ ] **Step 3: Implement facet-specific source markers**

Add:

```go
func contextSourceSectionSupportsEvidence(
	section ContextSourceSection,
	concern contextConcern,
) bool {
	if concern.project != "" &&
		normalizeContextProject(section.Project) != concern.project {
		return false
	}
	if section.Role == "test" && concern.kind != contextConcernTests {
		return false
	}
	if concern.facet == "" {
		return contextSourceSectionSupportsConcern(section, concern)
	}
	content := strings.ToLower(contextSourceSemanticContent(section.Content))
	switch concern.kind + "#" + concern.facet {
	case contextConcernAuth + "#client_transport":
		return contextSourceContainsAny(
			content,
			"basicauthenticationinterceptor",
			"basicauthentication(",
			"defaultheader",
			"authorization",
			"oauth2authorizedclient",
			".setbasicauth(",
		)
	case contextConcernAuth + "#server_policy":
		return contextSourceContainsAny(
			content,
			"securityfilterchain",
			".httpbasic(",
			".oauth2resourceserver(",
			"@securityrequirement",
		)
	case contextConcernConfiguration + "#binding":
		return contextSourceContainsAny(
			content,
			"@configurationproperties",
			"@value(",
			"connecttimeout",
			"readtimeout",
			"maxretries",
		)
	case contextConcernConfiguration + "#consumer":
		return contextSourceContainsAny(
			content,
			"configuration.",
			"config.get",
			"getconfig(",
			"getbaseurl(",
			"getconnecttimeout(",
			"getreadtimeout(",
			"getmaxretries(",
			"getpath(",
		)
	case contextConcernResilience + "#retry_policy":
		return contextSourceContainsAny(content, "@retryable", "maxattempts")
	case contextConcernResilience + "#recovery":
		return contextSourceContainsAny(content, "@recover", "recovering", "recovery")
	case contextConcernSideEffects + "#mail":
		return contextSourceContainsAny(content, "mailservice.", "sendmail", "sendemail")
	case contextConcernSideEffects + "#audit":
		return contextSourceContainsAny(
			content, "protocolservice.", "trackingservice.", "audit", "log.",
		)
	case contextConcernSideEffects + "#user_information":
		return contextSourceContainsAny(
			content, "userservice.", "usermgmt", "getuser", "userinformation",
		)
	default:
		return contextSourceSectionSupportsConcern(section, concern)
	}
}
```

In `contextSourceOptionConcerns`, keep candidate-fact binding mandatory for
facets and prohibit the generic same-project fallback:

```go
if concern.facet != "" {
	if !covered || !contextSourceSectionSupportsEvidence(section, concern) {
		continue
	}
	keys = append(keys, concern.key)
	required = true
	continue
}
```

Place this block after the project check and before the existing
`contextSourceRequiresRenderedConcernEvidence` branch. For non-faceted
concerns, preserve the current generic source fallback. This distinction is
what prevents one repository candidate from covering another requested model
merely because both files contain persistence syntax.

- [ ] **Step 4: Write a failing coalesced-omission test**

Add:

```go
func TestContextSourceEvidenceOmissionsCoalesceFacetsByPath(t *testing.T) {
	base := newContextConcern(
		contextConcernSideEffects,
		"services/jobs",
		true,
		[]string{"service"},
		"requested side effects",
	)
	concerns := []contextConcern{
		newContextEvidenceConcern(base, "mail", []string{"service"}, "mail evidence"),
		newContextEvidenceConcern(base, "audit", []string{"service"}, "audit evidence"),
	}
	candidates := []sourceCandidate{{
		FactID: "service", FactIDs: []string{"service"},
		Project: "services/jobs", Path: "src/JobService.java", Role: "call_chain",
	}}
	got := contextSourceEvidenceOmissions(concerns, candidates, nil, map[string]bool{})
	if len(got) != 1 ||
		got[0].Project != "services/jobs" ||
		got[0].Path != "src/JobService.java" ||
		!strings.Contains(got[0].Reason, "audit evidence") ||
		!strings.Contains(got[0].Reason, "mail evidence") {
		t.Fatalf("coalesced omissions = %#v", got)
	}
}
```

- [ ] **Step 5: Build all missing-facet omissions before applying the cap**

Implement:

```go
func contextSourceEvidenceOmissions(
	concerns []contextConcern,
	candidates []sourceCandidate,
	failures map[string]string,
	covered map[string]bool,
) []ContextSourceOmission {
	grouped := map[string]ContextSourceOmission{}
	reasons := map[string][]string{}
	for _, concern := range concerns {
		if !concern.required || covered[concern.key] {
			continue
		}
		omission := contextSourceConcernOmission(concern, candidates, failures)
		key := normalizeContextProject(omission.Project) + "\x00" +
			contextPackSourceFile(omission.Path) + "\x00" + omission.Role
		if _, exists := grouped[key]; !exists {
			grouped[key] = omission
		}
		reason := strings.TrimSpace(concern.reason)
		if reason == "" {
			reason = omission.Reason
		}
		reasons[key] = append(reasons[key], reason)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]ContextSourceOmission, 0, min(len(keys), MaxContextSourceOmissions))
	for _, key := range keys {
		omission := grouped[key]
		values := orderedContextConcernIDs(reasons[key])
		omission.Reason = "missing evidence: " + strings.Join(values, "; ")
		result = append(result, omission)
		if len(result) == MaxContextSourceOmissions {
			break
		}
	}
	return result
}
```

Replace the per-concern capped omission loop in `selectContextSourceOptions` with this function. Add omissions one at a time only while `contextSourcePackFits` remains true.

- [ ] **Step 6: Add a production-shaped no-false-complete integration test**

Add this alternative endpoint fact to `writeMissingContractContextFixture`:

```go
{
	ID: "update-endpoint", Project: "services/catalog", Kind: "api_endpoint",
	Name: "PUT /catalog/items/{itemId}", Qualified: "CatalogController.updateItem",
	HTTPMethod: "PUT", Path: "/catalog/items/{itemId}",
	File: "src/main/java/example/CatalogController.java",
	Line: 16, EndLine: 20, Confidence: "EXACT",
	Search: "update catalog item save",
},
```

Then add:

```go
func TestBuildContextReportsMissingSideEffectFacetsWithoutRerankingEndpoint(t *testing.T) {
	root := writeMissingContractContextFixture(t)
	query := "When DELETE /catalog/items/{itemId} removes an item in services/catalog, " +
		"plan cleanup of related jobs through libraries/job-client and services/jobs. " +
		"Cover mail, audit, and user information separately."
	pack, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}

	if len(pack.Endpoints) != 1 ||
		pack.Endpoints[0].HTTPMethod != "DELETE" ||
		pack.Endpoints[0].Path != "/catalog/items/{itemId}" {
		t.Fatalf("evidence planning changed primary endpoint: %#v", pack.Endpoints)
	}
	if pack.SourceCoverage == "complete" {
		t.Fatalf("generic side-effect source covered three requested facets: %#v", pack.Concerns)
	}
	var sideEffectOmission *ContextSourceOmission
	for index := range pack.SourceOmissions {
		if strings.HasSuffix(pack.SourceOmissions[index].Path, "JobHousekeeping.java") {
			sideEffectOmission = &pack.SourceOmissions[index]
			break
		}
	}
	if sideEffectOmission == nil {
		t.Fatalf("missing side-effect evidence lacks targeted omission: %#v", pack.SourceOmissions)
	}
	for _, facet := range []string{"mail", "audit", "user_information"} {
		if !strings.Contains(sideEffectOmission.Reason, facet) {
			t.Errorf("coalesced omission lacks %q: %#v", facet, sideEffectOmission)
		}
	}
	if pack.EstimatedTokens > pack.BudgetTokens || pack.FallbackRequired || pack.RetryAllowed {
		t.Fatalf("bounded one-shot contract changed: %#v", pack)
	}
}
```

- [ ] **Step 7: Run focused and package tests**

Run:

```bash
go test ./internal/agent -run 'TestContextSourceSectionSupportsOnlyMatchingEvidenceFacet|TestContextSourceEvidenceOmissionsCoalesceFacetsByPath|TestBuildContextReportsMissingSideEffectFacetsWithoutRerankingEndpoint|TestBuildContextSupportsMissingContractChangeAnalysis' -count=1
go test ./internal/agent -count=1
```

Expected: all tests pass; endpoint selection is unchanged, false completeness is rejected, and missing evidence is path-bound.

- [ ] **Step 8: Commit**

Commit:

```text
Report bounded semantic evidence gaps

- Validate each evidence facet against rendered source
- Coalesce missing facets into deterministic path-bound omissions
- Preserve the selected endpoint and one-shot retry contract
```

---

### Task 5: Document semantics and verify the real benchmark

**Files:**

- Modify: `docs/OUTPUTS.md`
- Modify: `README.md`
- Test: `docs_test.go`
- Verify: `internal/agent`
- Verify: `internal/scan`
- Verify: `scripts/analyze-agent-context-log_test.sh`
- Verify: `scripts/benchmark-agent-context_test.sh`
- Verify: `/Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix`

**Interfaces:**

- Documents: semantic meaning of `concerns[].covered`, `source_coverage`, and coalesced `source_omissions`
- Preserves: the nine-line agent instruction and existing retry/fallback behavior

- [ ] **Step 1: Add failing documentation assertions**

Extend `docs_test.go`:

```go
func TestDocumentationDefinesSemanticEvidenceCoverage(t *testing.T) {
	outputs := normalizedFileContents(t, "docs/OUTPUTS.md")
	for _, phrase := range []string{
		"`source_coverage: complete` requires every requested evidence facet",
		"one repository cannot cover multiple requested domain models",
		"`source_omissions` may combine missing facets for the same project and path",
	} {
		if !strings.Contains(outputs, phrase) {
			t.Errorf("OUTPUTS.md missing semantic coverage contract %q", phrase)
		}
	}
}
```

- [ ] **Step 2: Run the docs test and verify RED**

Run:

```bash
go test . -run TestDocumentation -count=1
```

Expected: `FAIL` until the new coverage contract is documented.

- [ ] **Step 3: Document the exact public behavior**

Add this text to `docs/OUTPUTS.md` and summarize it in `README.md`:

```markdown
`source_coverage: complete` requires every requested evidence facet to be
backed by a verified `source_section`. One repository cannot cover multiple
requested domain models, and one side-effect section cannot implicitly cover
mail, audit, and user-information behavior.

When the bounded pack cannot represent every facet, `source_coverage` is
`partial` or `none`. `source_omissions` may combine missing facets for the
same project and path so an agent can inspect that one indexed file without
widening navigation.
```

Do not change the generated guide because it already permits only the exact paths listed in `source_omissions`.

- [ ] **Step 4: Run complete verification**

Run:

```bash
gofmt -w internal/scan/spring_extract.go internal/scan/extract_java_test.go \
  internal/scan/agent_context_index.go internal/scan/agent_context_index_test.go \
  internal/agent/context_intent.go internal/agent/context_select.go \
  internal/agent/context_test.go internal/agent/context_source_test.go \
  internal/agent/context_change_analysis_test.go
go test ./... -count=1
go vet ./...
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context_test.sh
bash -n scripts/analyze-agent-context-log.sh \
  scripts/analyze-agent-context-log_test.sh \
  scripts/benchmark-agent-context.sh \
  scripts/benchmark-agent-context_test.sh
git diff --check
```

Expected: every command exits `0`.

- [ ] **Step 5: Install and rebuild the benchmark index**

Run:

```bash
go install ./cmd/goregraph
goregraph workspace clean \
  /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --workspace /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix
```

Confirm the dry run lists exactly the three project `goregraph-out` directories and `.goregraph-workspace`. Then execute:

```bash
goregraph workspace clean \
  /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --workspace /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --execute
goregraph workspace build agent \
  /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix \
  --workspace /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix
goregraph doctor \
  /Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix
```

Expected: three indexed projects, no missing services, and a healthy schema-3 agent index.

- [ ] **Step 6: Verify security truth in the regenerated catalog**

Run:

```bash
benchmark_root=/Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix
jq -e '
  [.endpoints[]
   | select(.path | startswith("/cadastertaskmgmt"))
   | .security[].kind] as $kinds
  | ($kinds | index("basic")) != null
    and ($kinds | index("bearer")) == null
' "$benchmark_root/ms-cadastertask/goregraph-out/index/api-catalog.json"
```

Expected: `true`.

- [ ] **Step 7: Verify the exact benchmark Context Pack**

Run the complete benchmark query once with:

```bash
benchmark_root=/Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix
benchmark_tmp=$(mktemp -d)
benchmark_query='Historische, ausschließlich lesende Ursachenanalyse über ms-cadasterregulation, ms-cadastertask und ms-common: Beim Entfernen einer Vorschrift aus einem Kataster bleiben verbundene Aufgaben bestehen. Ermittle den öffentlichen REST-Endpunkt und die bestehende Aufrufkette, die Ursache, die erforderliche neue Aufrufkette über alle drei Repositories, betroffene Aufgabenarten und Suchattribute, den nötigen internen API-Vertrag samt Authentifizierung und Konfiguration, Persistenzoperationen, Nebenwirkungen bei Protokollierung, E-Mail und Benutzerinformationen, alle zu ändernden oder anzulegenden Produktions- und Testdateien sowie Fehlerbehandlung, Retry-Logik und notwendige Tests. Keine Implementierung; jede Aussage mit engem Quellcodebeleg.'
(
  cd "$benchmark_root"
  goregraph context . \
    --query "$benchmark_query" \
    --budget-tokens 4000 \
    --max-files 12 \
    --format json > "$benchmark_tmp/context-pack-1.json"
)
```

The temporary directory keeps generated verification files out of the historical workspace. Assert with `jq`:

```bash
jq -e '
  .endpoints | length == 1
  and .[0].http_method == "DELETE"
  and .[0].path == "/cadasters/{cadasterId}/regulations/{objectId:.+}"
' "$benchmark_tmp/context-pack-1.json"

jq -e '
  .estimated_tokens <= 4000
  and .fallback_required == false
  and .retry_allowed == false
  and (.source_coverage == "complete"
       or (.source_coverage == "partial"
           and (.source_omissions | length) > 0
           and all(.source_omissions[]; .path != "")))
' "$benchmark_tmp/context-pack-1.json"
```

The implementation passes whether every facet fits or whether honest path-bound omissions remain. It fails on false completeness, pathless budget omissions, a changed endpoint, fallback, retry, or budget overflow.

- [ ] **Step 8: Verify determinism**

Run the exact query twice more. Canonicalize each result with `jq -S .` and compare SHA-256 hashes:

```bash
for run in 2 3; do
  (
    cd "$benchmark_root"
    goregraph context . \
      --query "$benchmark_query" \
      --budget-tokens 4000 \
      --max-files 12 \
      --format json > "$benchmark_tmp/context-pack-$run.json"
  )
done
for run in 1 2 3; do
  jq -S . "$benchmark_tmp/context-pack-$run.json" | shasum -a 256
done
```

Expected: all three hashes are identical.

- [ ] **Step 9: Run one assisted agent benchmark**

Use the unchanged benchmark prompt, `gpt-5.6-sol`, `xhigh`, read-only sandbox, no approval, and the regenerated workspace. Acceptance:

- correct DELETE endpoint and handler;
- no PUT cache-area claim;
- at most one initial Context Pack and at most the exact omission-bound source reads;
- no source reads outside `source_omissions`;
- authentication identifies Basic Auth when the omitted/selected source proves it;
- both task models and repositories are either proven or explicitly unknown;
- mail, audit, and user-information conclusions are independently evidenced or independently marked unknown;
- total report remains below 900 words;
- token use remains below the 64,220-token previous-good assisted run.

- [ ] **Step 10: Commit documentation and benchmark assertions**

Commit:

```text
Document semantic source coverage

- Define complete coverage in terms of requested evidence facets
- Explain coalesced path-bound omissions
- Record the real benchmark acceptance contract
```

Do not tag or release.
