package agent

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestContextQueryRequestsExactEvidenceInventoryRequiresInventoryOrFileNoun(t *testing.T) {
	for _, test := range []struct {
		query string
		want  bool
	}{
		{query: "Analyze the exact production path for authentication.", want: false},
		{query: "Analysiere den exakten Produktions-Pfad für Authentifizierung.", want: false},
		{query: "Provide an exact production path inventory for authentication.", want: true},
		{query: "Provide an exact production file for authentication.", want: true},
		{query: "Stelle ein Verzeichnis mit exaktem Produktionsdatei-Inventar für Authentifizierung bereit.", want: true},
		{query: "Identify production and test files to change/create.", want: true},
		{query: "Welche Produktions- und Testdateien müssen geändert oder angelegt werden?", want: true},
		{query: "Analyze production behavior and tests.", want: false},
		{query: "Open the production file and run tests.", want: false},
		{query: "Provide an exact file inventory mentioning TestimonialClient.", want: false},
		{query: "Stelle ein exaktes Datei-Inventar für den Produktionsauftrag bereit.", want: false},
	} {
		if got := contextQueryRequestsExactEvidenceInventory(test.query); got != test.want {
			t.Errorf("exact inventory trigger for %q = %t, want %t", test.query, got, test.want)
		}
	}
}

func TestExpandContextExactInventoryConcernsUsesPathSpecificPublicKeys(t *testing.T) {
	query := "Provide an exact production file inventory for configuration."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "config-class", Project: "services/catalog", Kind: "configuration", File: "src/main/java/catalog/CatalogConfig.java", Confidence: "EXACT"},
		{ID: "config-resource", Project: "services/catalog", Kind: "configuration", File: "src/main/resources/application.yml", Confidence: "EXACT"},
	}}
	base := newContextConcern(
		contextConcernConfiguration,
		"services/catalog",
		true,
		[]string{"config-class", "config-resource"},
		"requested configuration",
	)

	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{base},
	)
	want := map[string]bool{
		"configuration:services/catalog#exact-file:src/main/java/catalog/CatalogConfig.java": true,
		"configuration:services/catalog#exact-file:src/main/resources/application.yml":       true,
	}
	seen := map[string]bool{}
	for _, concern := range concerns {
		if !concern.exactInventory {
			continue
		}
		if concern.publicKey != concern.key {
			t.Errorf("exact concern public key = %q, want path-specific key %q", concern.publicKey, concern.key)
		}
		if !want[concern.key] {
			t.Errorf("unexpected exact concern key %q", concern.key)
		}
		seen[concern.key] = true
	}
	if len(seen) != len(want) {
		t.Fatalf("path-specific exact concerns = %v, want %v", seen, want)
	}
}

func TestExpandContextExactInventoryConcernsPreservesAllRequestedCategories(t *testing.T) {
	query := "Provide an exact production file inventory for authentication, configuration, retry, persistence, and side effects."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "auth-config", Project: "services/catalog", Kind: "authentication", Name: "ConfigurationBackedAuth", File: "src/main/java/catalog/ConfigurationBackedAuth.java", Confidence: "EXACT", Search: "authentication configuration retry persistence side effects"},
		{ID: "auth-security", Project: "services/catalog", Kind: "authentication", Name: "SecurityPolicy", File: "src/main/java/catalog/SecurityPolicy.java", Confidence: "EXACT", Search: "authentication security"},
		{ID: "config-auth", Project: "services/catalog", Kind: "configuration", Name: "AuthenticationProperties", File: "src/main/java/catalog/AuthenticationProperties.java", Confidence: "EXACT", Search: "configuration authentication retry persistence side effects"},
		{ID: "config-settings", Project: "services/catalog", Kind: "configuration", Name: "ServiceSettings", File: "src/main/java/catalog/ServiceSettings.java", Confidence: "EXACT", Search: "configuration settings"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{
			newContextConcern(contextConcernAuth, "services/catalog", true, []string{"auth-config", "auth-security"}, "requested authentication"),
			newContextConcern(contextConcernConfiguration, "services/catalog", true, []string{"config-auth", "config-settings"}, "requested configuration"),
		},
	)
	want := map[string]bool{
		"authentication:services/catalog#exact-file:src/main/java/catalog/ConfigurationBackedAuth.java": true,
		"authentication:services/catalog#exact-file:src/main/java/catalog/SecurityPolicy.java":          true,
		"configuration:services/catalog#exact-file:src/main/java/catalog/AuthenticationProperties.java": true,
		"configuration:services/catalog#exact-file:src/main/java/catalog/ServiceSettings.java":          true,
	}
	seen := map[string]bool{}
	for _, concern := range concerns {
		if concern.exactInventory {
			seen[concern.key] = true
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("exact multi-category inventory missing %q from %v", key, seen)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("exact multi-category inventory = %v, want %v", seen, want)
	}
}

func TestExpandContextExactInventoryConcernsPreservesUnscopedAuthenticationSiblings(t *testing.T) {
	query := "Provide an exact production file inventory for client and provider authentication."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "client-auth", Project: "libraries/client", Kind: "authentication", Name: "ClientSecurity", File: "src/main/java/client/ClientSecurity.java", Confidence: "EXACT", Search: "client authentication"},
		{ID: "provider-auth", Project: "services/provider", Kind: "authentication", Name: "ProviderSecurity", File: "src/main/java/provider/ProviderSecurity.java", Confidence: "EXACT", Search: "provider authentication"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{
			newContextConcern(contextConcernAuth, "", true, []string{"client-auth", "provider-auth"}, "requested authentication"),
		},
	)
	want := map[string]bool{
		"authentication:libraries/client#exact-file:src/main/java/client/ClientSecurity.java":      true,
		"authentication:services/provider#exact-file:src/main/java/provider/ProviderSecurity.java": true,
	}
	seen := map[string]bool{}
	for _, concern := range concerns {
		if concern.exactInventory {
			seen[concern.key] = true
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("exact unscoped authentication inventory missing %q from %v", key, seen)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("exact unscoped authentication inventory = %v, want %v", seen, want)
	}
}

func TestExpandContextExactInventoryConcernsPreservesGermanRequestedCategories(t *testing.T) {
	query := "Stelle eine exakte Produktionsdatei-Liste für Authentifizierung, Konfiguration, Wiederholung, Persistenz und Nebenwirkungen bereit."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "auth-config", Project: "services/catalog", Kind: "authentication", Name: "ConfigurationBackedAuth", File: "src/main/java/catalog/ConfigurationBackedAuth.java", Confidence: "EXACT", Search: "authentifizierung konfiguration wiederholung persistenz nebenwirkung"},
		{ID: "auth-security", Project: "services/catalog", Kind: "authentication", Name: "SecurityPolicy", File: "src/main/java/catalog/SecurityPolicy.java", Confidence: "EXACT", Search: "authentifizierung sicherheit"},
		{ID: "config-auth", Project: "services/catalog", Kind: "configuration", Name: "AuthenticationProperties", File: "src/main/java/catalog/AuthenticationProperties.java", Confidence: "EXACT", Search: "konfiguration authentifizierung wiederholung persistenz nebenwirkung"},
		{ID: "config-settings", Project: "services/catalog", Kind: "configuration", Name: "ServiceSettings", File: "src/main/java/catalog/ServiceSettings.java", Confidence: "EXACT", Search: "konfiguration einstellungen"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{
			newContextConcern(contextConcernAuth, "services/catalog", true, []string{"auth-config", "auth-security"}, "requested authentication"),
			newContextConcern(contextConcernConfiguration, "services/catalog", true, []string{"config-auth", "config-settings"}, "requested configuration"),
		},
	)
	want := map[string]bool{
		"authentication:services/catalog#exact-file:src/main/java/catalog/ConfigurationBackedAuth.java": true,
		"authentication:services/catalog#exact-file:src/main/java/catalog/SecurityPolicy.java":          true,
		"configuration:services/catalog#exact-file:src/main/java/catalog/AuthenticationProperties.java": true,
		"configuration:services/catalog#exact-file:src/main/java/catalog/ServiceSettings.java":          true,
	}
	seen := map[string]bool{}
	for _, concern := range concerns {
		if concern.exactInventory {
			seen[concern.key] = true
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("exact German multi-category inventory missing %q from %v", key, seen)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("exact German multi-category inventory = %v, want %v", seen, want)
	}
}

func TestExpandContextExactInventoryConcernsAcceptsIndexedRuntimeShapes(t *testing.T) {
	query := "Identify the production and test files to change or create for authentication and configuration."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "provider-security", Project: "services/jobs", Kind: "endpoint_security", Name: "role", File: "src/main/java/example/JobSecurity.java", Confidence: "EXACT", Search: "job management technical role security"},
		{ID: "management-controller", Project: "services/jobs", Kind: "api_endpoint", Name: "GET /job-management/jobs", File: "src/main/java/example/JobManagementController.java", Confidence: "EXACT", Search: "job management endpoint"},
		{ID: "management-test-class", Project: "services/jobs", Kind: "symbol", Name: "JobManagementControllerTest", Qualified: "example.JobManagementControllerTest", File: "src/test/java/example/JobManagementControllerTest.java", Confidence: "EXACT", Search: "job management controller test"},
		{ID: "technical-user", Project: "services/jobs", Kind: "configuration", Name: "technical-user", File: "src/main/resources/application.properties", Confidence: "EXACT", Search: "technical user configuration"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{
			newContextConcern(contextConcernAuth, "services/jobs", true, []string{"provider-security"}, "requested authentication"),
			newContextConcern(contextConcernHTTPContract, "services/jobs", true, []string{"management-controller"}, "requested internal contract"),
			newContextConcern(contextConcernTests, "services/jobs", true, []string{"management-test-class"}, "requested tests"),
			newContextConcern(contextConcernConfiguration, "services/jobs", true, []string{"technical-user"}, "requested configuration"),
		},
	)
	want := map[string]bool{
		"authentication:services/jobs#exact-file:src/main/java/example/JobSecurity.java":            true,
		"http_contract:services/jobs#exact-file:src/main/java/example/JobManagementController.java": true,
		"tests:services/jobs#exact-file:src/test/java/example/JobManagementControllerTest.java":     true,
		"configuration:services/jobs#exact-file:src/main/resources/application.properties":          true,
	}
	seen := map[string]bool{}
	for _, concern := range concerns {
		if concern.exactInventory {
			seen[concern.key] = true
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("indexed runtime inventory shape missing %q from %v", key, seen)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("indexed runtime inventory = %v, want %v", seen, want)
	}
}

func TestExpandContextExactInventoryConcernsDiscoversProjectTestClass(t *testing.T) {
	query := "Identify the exact production and executable test files to change or create."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "test-target", Project: "services/jobs", Kind: "test_target", Name: "JobManagementController", File: "src/main/java/example/JobManagementController.java", Confidence: "EXACT"},
		{ID: "test-class", Project: "services/jobs", Kind: "symbol", Name: "JobManagementControllerTest", Qualified: "example.JobManagementControllerTest", File: "src/test/java/example/JobManagementControllerTest.java", Confidence: "EXACT", Search: "job management controller test"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{newContextConcern(
			contextConcernTests,
			"services/jobs",
			true,
			[]string{"test-target"},
			"requested tests",
		)},
	)
	want := "tests:services/jobs#exact-file:src/test/java/example/JobManagementControllerTest.java"
	for _, concern := range concerns {
		if concern.exactInventory && concern.key == want {
			return
		}
	}
	t.Fatalf("project test class %q was not discovered from %#v", want, concerns)
}

func TestExpandContextExactInventoryConcernsBalancesKindsAndProjectsAtLimit(t *testing.T) {
	query := "Identifiziere in services/catalog und services/jobs zu ändernde oder anzulegende Produktions- und Testdateien für Authentifizierung, Konfiguration und HTTP-Verträge."
	projects := []string{"services/catalog", "services/jobs"}
	facts := []scan.AgentContextFactRecord{}
	concerns := []contextConcern{}
	for _, project := range projects {
		for _, kind := range []string{
			contextConcernAuth,
			contextConcernConfiguration,
			contextConcernHTTPContract,
			contextConcernTests,
		} {
			id := project + "-" + kind
			file := fmt.Sprintf("src/main/java/example/%sEvidence.java", kind)
			factKind := kind
			name := kind + "Evidence"
			if kind == contextConcernTests {
				file = "src/test/java/example/WorkflowTest.java"
				factKind = "symbol"
				name = "WorkflowTest"
			}
			facts = append(facts, scan.AgentContextFactRecord{
				ID: id, Project: project, Kind: factKind, Name: name,
				File: file, Confidence: "EXACT", Search: kind + " workflow",
			})
			concerns = append(concerns, newContextConcern(
				kind, project, true, []string{id}, "requested "+kind,
			))
			if kind == contextConcernConfiguration {
				facts = append(facts, scan.AgentContextFactRecord{
					ID: id + "-test-profile", Project: project, Kind: contextConcernConfiguration,
					Name: "technical-user", File: "src/main/resources/application-UNITTEST.properties",
					Confidence: "EXACT", Search: "authentication technical user configuration",
				})
			}
		}
	}
	for index := 0; index < maximumContextSourcePlanningCandidates; index++ {
		id := fmt.Sprintf("catalog-extra-auth-%02d", index)
		facts = append(facts, scan.AgentContextFactRecord{
			ID: id, Project: "services/catalog", Kind: contextConcernAuth,
			Name: "ExtraAuth", File: fmt.Sprintf("src/main/java/aaa/Auth%02d.java", index),
			Confidence: "EXACT", Search: "authentication catalog",
		})
		concerns = append(concerns, newContextConcern(
			contextConcernAuth, "services/catalog", true, []string{id}, "requested authentication",
		))
	}

	expanded := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		scan.AgentContextIndexRecord{Facts: facts},
		concerns,
	)
	seen := map[string]bool{}
	keys := map[string]bool{}
	count := 0
	for _, concern := range expanded {
		if !concern.exactInventory {
			continue
		}
		seen[concern.kind+":"+concern.project] = true
		keys[concern.key] = true
		count++
	}
	if count != maximumContextSourcePlanningCandidates {
		t.Fatalf("exact inventory groups = %d, want %d", count, maximumContextSourcePlanningCandidates)
	}
	for _, project := range projects {
		for _, kind := range []string{
			contextConcernAuth,
			contextConcernConfiguration,
			contextConcernHTTPContract,
			contextConcernTests,
		} {
			key := kind + ":" + project
			if !seen[key] {
				t.Errorf("bounded exact inventory omitted requested bucket %q from %v", key, seen)
			}
		}
	}
	for _, project := range projects {
		want := "configuration:" + project + "#exact-file:src/main/java/example/configurationEvidence.java"
		if !keys[want] {
			t.Errorf("production configuration %q missing from %v", want, keys)
		}
	}
}

func TestExpandContextExactInventoryConcernsPrioritizesInternalInterfaceFiles(t *testing.T) {
	query := "Across services/jobs, identify the exact production and executable test files to change or create for the internal HTTP contract."
	facts := []scan.AgentContextFactRecord{
		{ID: "config", Project: "services/jobs", Kind: "configuration", Name: "JobConfig", File: "src/main/java/example/JobConfig.java", Confidence: "EXACT"},
		{ID: "public-controller", Project: "services/jobs", Kind: "api_endpoint", Name: "GET /jobs", Qualified: "AJobController.list", File: "src/main/java/example/AJobController.java", Confidence: "EXACT", Search: "job endpoint"},
		{ID: "management-controller", Project: "services/jobs", Kind: "api_endpoint", Name: "GET /job-management/jobs", Qualified: "ZJobManagementController.list", File: "src/main/java/example/ZJobManagementController.java", Confidence: "EXACT", Search: "job management endpoint"},
		{ID: "public-test", Project: "services/jobs", Kind: "symbol", Name: "AJobControllerTest", Qualified: "example.AJobControllerTest", File: "src/test/java/example/AJobControllerTest.java", Confidence: "EXACT", Search: "job controller test"},
		{ID: "management-test", Project: "services/jobs", Kind: "symbol", Name: "ZJobManagementControllerTest", Qualified: "example.ZJobManagementControllerTest", File: "src/test/java/example/ZJobManagementControllerTest.java", Confidence: "EXACT", Search: "job management controller test"},
	}
	authIDs := []string{}
	for index := 0; index < maximumContextSourcePlanningCandidates; index++ {
		id := fmt.Sprintf("auth-%02d", index)
		authIDs = append(authIDs, id)
		facts = append(facts, scan.AgentContextFactRecord{
			ID: id, Project: "services/jobs", Kind: "authentication",
			Name:       fmt.Sprintf("JobInternalAuthentication%02d", index),
			File:       fmt.Sprintf("src/main/java/example/auth/JobInternalAuthentication%02d.java", index),
			Confidence: "EXACT", Search: "jobs exact production executable test files internal HTTP contract authentication",
		})
	}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		scan.AgentContextIndexRecord{Facts: facts},
		[]contextConcern{
			newContextConcern(contextConcernAuth, "services/jobs", true, authIDs, "requested authentication"),
			newContextConcern(contextConcernConfiguration, "services/jobs", true, []string{"config"}, "requested configuration"),
			newContextConcern(contextConcernHTTPContract, "services/jobs", true, []string{"public-controller", "management-controller"}, "requested internal contract"),
			newContextConcern(contextConcernTests, "services/jobs", true, []string{"public-test", "management-test"}, "requested tests"),
		},
	)
	seen := map[string]bool{}
	for _, concern := range concerns {
		if concern.exactInventory {
			seen[concern.key] = true
		}
	}
	for _, want := range []string{
		"http_contract:services/jobs#exact-file:src/main/java/example/ZJobManagementController.java",
		"tests:services/jobs#exact-file:src/test/java/example/ZJobManagementControllerTest.java",
	} {
		if !seen[want] {
			t.Errorf("internal interface evidence %q missing from %v", want, seen)
		}
	}
}

func TestContextSourcePlanningPrefersAuthenticationPropertyWhenRequested(t *testing.T) {
	query := "Analyze task cleanup authentication and configuration and identify production and test files to change or create."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "task-isbns", Project: "services/jobs", Kind: "configuration", Name: "taskIsbns", File: "src/main/resources/application.properties", Line: 51, Confidence: "EXACT", Search: "task isbn configuration"},
		{ID: "technical-user", Project: "services/jobs", Kind: "configuration", Name: "technical-user", File: "src/main/resources/application.properties", Line: 65, Confidence: "EXACT", Search: "technical user configuration"},
	}}
	base := newContextConcern(
		contextConcernConfiguration,
		"services/jobs",
		true,
		[]string{"task-isbns", "technical-user"},
		"requested configuration",
	)
	expanded := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{base},
	)
	concern := contextConcern{}
	for _, candidate := range expanded {
		if candidate.exactInventory {
			concern = candidate
			break
		}
	}
	if concern.key == "" {
		t.Fatal("exact configuration concern missing")
	}
	if !slices.Equal(concern.candidateFactIDs, []string{"technical-user"}) {
		t.Fatalf("exact configuration candidates = %v, want only authentication property", concern.candidateFactIDs)
	}
	candidates := contextSourceCandidatesForConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{concern},
	)
	selected := []string{}
	for _, candidate := range candidates {
		if candidate.Path == "src/main/resources/application.properties" {
			selected = append(selected, candidate.FactID)
		}
	}
	if !slices.Equal(selected, []string{"technical-user"}) {
		t.Fatalf("selected configuration facts = %v, want authentication property", selected)
	}
}

func TestContextSourceQualityPrefersRequestedAuthenticationProperty(t *testing.T) {
	query := "Analyze task cleanup authentication and configuration."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "task-isbns", Project: "services/jobs", Kind: "configuration", Name: "taskIsbns", File: "src/main/resources/application.properties", Line: 51, Confidence: "EXACT", Search: "task isbn configuration"},
		{ID: "technical-user", Project: "services/jobs", Kind: "configuration", Name: "technical-user", File: "src/main/resources/application.properties", Line: 65, Confidence: "EXACT", Search: "technical user configuration"},
	}}
	pack := ContextPack{Query: query, selectionQuery: query}
	quality := func(factID string) int {
		return contextSourceCandidateQuality(pack, index, contextSourceOption{
			candidate: sourceCandidate{
				FactID: factID, FactIDs: []string{factID}, Project: "services/jobs",
				Path: "src/main/resources/application.properties",
			},
		})
	}
	if relevant, generic := quality("technical-user"), quality("task-isbns"); relevant <= generic {
		t.Fatalf("authentication property quality = %d, want greater than generic property %d", relevant, generic)
	}
}

func TestContextSourceUtilityCountsExactAuthenticationEvidence(t *testing.T) {
	base := newContextConcern(
		contextConcernAuth,
		"services/jobs",
		true,
		[]string{"management-role"},
		"requested authentication",
	)
	exact := newContextEvidenceConcern(
		base,
		"exact-file:src/main/java/example/SecurityConfig.java",
		[]string{"management-role"},
		"exact authentication evidence",
	)
	exact.exactInventory = true
	option := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "management-role", FactIDs: []string{"management-role"},
			Project: "services/jobs", Path: "src/main/java/example/SecurityConfig.java",
			Role: "call_chain", Kind: "endpoint_security", Name: "role",
		},
		section: ContextSourceSection{
			Project: "services/jobs", Path: "src/main/java/example/SecurityConfig.java",
			StartLine: 20, EndLine: 20, Role: "call_chain", RenderMode: "body",
			Content: "http.securityMatcher(\"/job-management/**\").hasRole(\"TECHNICAL\");",
		},
		estimated: 70, concernKeys: []string{exact.key}, projectKey: "services/jobs",
		evidenceFamily: contextConcernAuth, quality: 280, profiled: true,
	}
	state := newContextSourceSelectionState(1, 1)
	state.selectedProjects["services/jobs"] = true
	state.coveredRoles["call_chain"] = true
	state.selectedEvidenceFamilies["services/jobs\x00"+contextConcernAuth] = 1
	pack := ContextPack{Schema: 1, Query: "internal authentication", BudgetTokens: DefaultContextBudgetTokens}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens, MaxFiles: DefaultContextMaxFiles}

	got, utility, found, err := contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{option},
		[]contextConcern{exact},
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || utility <= 0 || got.candidate.FactID != "management-role" {
		t.Fatalf("exact authentication option = found %t utility %d option %#v", found, utility, got)
	}
}

func TestContextSourceOptionProvesExactHTTPInventoryPath(t *testing.T) {
	base := newContextConcern(
		contextConcernHTTPContract,
		"services/jobs",
		true,
		[]string{"management-endpoint"},
		"requested internal contract",
	)
	concern := newContextEvidenceConcern(
		base,
		"exact-file:src/main/java/example/JobManagementController.java",
		[]string{"management-endpoint"},
		"exact file inventory evidence",
	)
	concern.exactInventory = true
	candidate := sourceCandidate{
		FactID: "management-endpoint", FactIDs: []string{"management-endpoint"},
		Project: "services/jobs", Path: "src/main/java/example/JobManagementController.java",
		Kind: "api_endpoint", Role: "call_chain",
	}
	section := ContextSourceSection{
		Project: "services/jobs", Path: candidate.Path, StartLine: 20, EndLine: 24,
		Role: "call_chain", RenderMode: "declaration_body",
		Content: "@DeleteMapping(\"/job-management/jobs\")\nvoid deleteJobs() {}",
	}
	keys, required := contextSourceOptionConcernsWithAction(
		candidate,
		section,
		[]contextConcern{concern},
		scan.AgentContextIndexRecord{},
		true,
		true,
	)
	if !required || !slices.Equal(keys, []string{concern.key}) {
		t.Fatalf("exact HTTP inventory concerns = %v, required %t", keys, required)
	}
}

func TestContextSourceOptionProvesRouteSpecificExactAuthenticationPath(t *testing.T) {
	base := newContextConcern(
		contextConcernAuth,
		"services/jobs",
		true,
		[]string{"management-security"},
		"requested server authentication",
	)
	concern := newContextEvidenceConcern(
		base,
		"exact-file:src/main/java/example/JobSecurity.java",
		[]string{"management-security"},
		"exact file inventory evidence",
	)
	concern.exactInventory = true
	candidate := sourceCandidate{
		FactID: "management-security", FactIDs: []string{"management-security"},
		Project: "services/jobs", Path: "src/main/java/example/JobSecurity.java",
		Kind: "endpoint_security", Role: "call_chain",
	}
	section := ContextSourceSection{
		Project: "services/jobs", Path: candidate.Path, StartLine: 20, EndLine: 27,
		Role: "call_chain", RenderMode: "declaration_body",
		Content: "SecurityFilterChain management(HttpSecurity http) {\n  return http.securityMatcher(\"/job-management/**\").httpBasic().build();\n}",
	}
	keys, required := contextSourceOptionConcernsWithAction(
		candidate,
		section,
		[]contextConcern{concern},
		scan.AgentContextIndexRecord{},
		true,
		true,
	)
	if !required || !slices.Equal(keys, []string{concern.key}) {
		t.Fatalf("route-specific exact authentication concerns = %v, required %t", keys, required)
	}
}

func TestRenderSourceCandidateUsesIndexedEndpointSecurityLine(t *testing.T) {
	candidate := sourceCandidate{
		FactID: "management-role", FactIDs: []string{"management-role"},
		Project: "services/jobs", Path: "src/main/java/example/SecurityConfig.java",
		StartLine: 3, Kind: "endpoint_security", Name: "role", Role: "call_chain",
	}
	section, err := renderSourceCandidate(candidate, sourceFile{
		Path: candidate.Path,
		Lines: []string{
			"class SecurityConfig {",
			"  void configure(HttpSecurity http) {",
			"    http.securityMatcher(\"/job-management/**\").hasRole(\"TECHNICAL\");",
			"  }",
			"}",
		},
	}, "focused")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 3 || section.EndLine != 3 ||
		!strings.Contains(section.Content, "hasRole") {
		t.Fatalf("endpoint security section = %#v, want exact indexed line", section)
	}
}

func TestContextSourceConcernsKeepPlannedExactKindsBeyondPublicMetadata(t *testing.T) {
	index := missingContractContextIndex()
	missingContractFactByID(index.Facts, "jobs-route").Kind = "api_endpoint"
	index.Facts = append(index.Facts, scan.AgentContextFactRecord{
		ID: "jobs-management-contract", Project: "libraries/job-client", Kind: "api_contract",
		Name: "DELETE /job-management/jobs", Qualified: "JobManagementClient.deleteJobs",
		HTTPMethod: "DELETE", Path: "/job-management/jobs",
		File: "src/main/java/example/JobManagementClient.java",
		Line: 30, EndLine: 34, Confidence: "EXACT",
		Search: "catalog job cleanup internal HTTP contract management endpoint",
	})
	pack, err := compileContextPack(index, ContextRequest{
		Query: broadReleaseQualityQuery, BudgetTokens: DefaultContextBudgetTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	pack.Concerns = []ContextConcern{
		{Kind: contextConcernAuth, Project: "services/jobs", Reason: "bounded public authentication"},
		{Kind: contextConcernTests, Project: "services/jobs", Reason: "bounded public tests"},
	}
	concerns := contextSourceConcerns(pack, index)
	seen := map[string]bool{}
	for _, concern := range concerns {
		if concern.exactInventory {
			seen[concern.kind] = true
		}
	}
	for _, kind := range []string{
		contextConcernAuth,
		contextConcernConfiguration,
		contextConcernHTTPContract,
		contextConcernTests,
	} {
		if !seen[kind] {
			t.Errorf("planned exact kind %q missing beyond public metadata: %v", kind, seen)
		}
	}
}

func TestExpandContextExactInventoryConcernsIncludesAuthenticationOwnerSymbol(t *testing.T) {
	query := "For task cleanup, identify the exact production files to change for internal authentication."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "endpoint-security", Project: "services/jobs", Kind: "endpoint_security", Name: "role", File: "src/main/java/example/SecurityConfig.java", Line: 77, Confidence: "EXACT", Search: "task endpoint security"},
		{ID: "security-owner", Project: "services/jobs", Kind: "symbol", Name: "SecurityConfig", Qualified: "example.SecurityConfig", File: "src/main/java/example/SecurityConfig.java", Line: 20, EndLine: 90, Confidence: "EXACT"},
		{ID: "security-helper", Project: "services/jobs", Kind: "symbol", Name: "configure", Qualified: "example.SecurityConfig.configure", File: "src/main/java/example/SecurityConfig.java", Line: 40, EndLine: 60, Confidence: "EXACT"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{newContextConcern(
			contextConcernAuth,
			"services/jobs",
			true,
			[]string{"endpoint-security"},
			"requested authentication",
		)},
	)
	for _, concern := range concerns {
		if !concern.exactInventory {
			continue
		}
		if !slices.Contains(concern.candidateFactIDs, "security-owner") {
			t.Fatalf("exact authentication candidates = %v, want owner symbol", concern.candidateFactIDs)
		}
		if slices.Contains(concern.candidateFactIDs, "endpoint-security") {
			t.Fatalf("exact authentication candidates retain non-renderable endpoint fact: %v", concern.candidateFactIDs)
		}
		if slices.Contains(concern.candidateFactIDs, "security-helper") {
			t.Fatalf("exact authentication candidates include unrelated helper: %v", concern.candidateFactIDs)
		}
		candidates := contextSourceCandidatesForConcerns(
			ContextPack{Query: query, selectionQuery: query},
			index,
			[]contextConcern{concern},
		)
		selected := []string{}
		for _, candidate := range candidates {
			if candidate.Path == "src/main/java/example/SecurityConfig.java" {
				selected = append(selected, candidate.FactID)
			}
		}
		if !slices.Equal(selected, []string{"security-owner"}) {
			t.Fatalf("planned exact authentication facts = %v, want renderable owner", selected)
		}
		return
	}
	t.Fatal("exact authentication concern missing")
}

func TestExpandContextExactInventoryConcernsPrefersInternalEndpointSecurity(t *testing.T) {
	query := "For task cleanup, identify the exact production files to change for the internal management authentication contract."
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "management-security", Project: "services/jobs", Kind: "endpoint_security", Name: "role", File: "src/main/java/example/SecurityConfig.java", Line: 77, Confidence: "EXACT", Search: "GET /jobmanagement/jobs internal management role security"},
		{ID: "security-owner", Project: "services/jobs", Kind: "symbol", Name: "SecurityConfig", Qualified: "example.SecurityConfig", File: "src/main/java/example/SecurityConfig.java", Line: 20, EndLine: 90, Confidence: "EXACT"},
	}}
	concerns := expandContextEvidenceConcerns(
		ContextPack{Query: query, selectionQuery: query},
		index,
		[]contextConcern{newContextConcern(
			contextConcernAuth,
			"services/jobs",
			true,
			[]string{"management-security"},
			"requested internal authentication",
		)},
	)
	for _, concern := range concerns {
		if !concern.exactInventory {
			continue
		}
		if !slices.Equal(concern.candidateFactIDs, []string{"management-security"}) {
			t.Fatalf("exact internal authentication candidates = %v, want endpoint security", concern.candidateFactIDs)
		}
		return
	}
	t.Fatal("exact internal authentication concern missing")
}
