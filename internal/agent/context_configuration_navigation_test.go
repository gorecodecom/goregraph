package agent

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestContextConfigurationResourcesUsesSelectedAdaptiveProject(t *testing.T) {
	query := "Show exact production configuration files and tests for authentication"
	pack := ContextPack{
		Query: query, selectionQuery: query, ProtocolVersion: AdaptiveV2,
		SourceSections: []ContextSourceSection{{
			Project: "services/billing", Path: "src/main/java/example/BillingController.java",
		}},
	}
	index := configurationNavigationIndex()

	want := []ContextConfigurationResourceGroup{{
		Project:   "services/billing",
		KeyGroups: []string{"technical-user"},
		Resources: []ContextConfigurationResource{
			{Path: "src/main/resources/application.properties", Profile: "production"},
			{Path: "src/test/resources/application-test.properties", Profile: "test"},
		},
	}}
	got := contextConfigurationResources(pack, index)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected-project configuration navigation = %#v, want %#v", got, want)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"services/archive", "billing-password-value", "archive-password-value"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("configuration navigation disclosed unrelated scope or value %q: %s", forbidden, body)
		}
	}
}

func TestContextConfigurationResourcesExplicitScopeOverridesSelectedProject(t *testing.T) {
	query := "Show exact production configuration files and tests for authentication in services/archive"
	pack := ContextPack{
		Query: query, selectionQuery: query, ProtocolVersion: AdaptiveV2,
		Entrypoints: []ContextLocation{{ID: "billing-route", Project: "services/billing"}},
	}

	got := contextConfigurationResources(pack, configurationNavigationIndex())
	if len(got) != 1 || got[0].Project != "services/archive" {
		t.Fatalf("explicit project scope did not win: %#v", got)
	}
	for _, group := range got {
		if group.Project == "services/billing" {
			t.Fatalf("selected project broadened explicit scope: %#v", got)
		}
	}
}

func TestContextConfigurationResourcesUsesSelectedFactsAndScopedConcerns(t *testing.T) {
	query := "Show exact production configuration files and tests for authentication"
	index := configurationNavigationIndex()
	tests := []struct {
		name string
		pack ContextPack
	}{
		{
			name: "selected source fact",
			pack: ContextPack{selectedSourceFactIDs: []string{"billing-service"}},
		},
		{
			name: "requested scoped concern",
			pack: ContextPack{Concerns: []ContextConcern{{
				Kind: contextConcernConfiguration, Project: "services/billing", Covered: true,
			}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pack := test.pack
			pack.Query = query
			pack.selectionQuery = query
			pack.ProtocolVersion = AdaptiveV2
			got := contextConfigurationResources(pack, index)
			if len(got) != 1 || got[0].Project != "services/billing" {
				t.Fatalf("configuration navigation from %s = %#v", test.name, got)
			}
		})
	}
}

func TestContextConfigurationResourcesRequiresSelectedAdaptiveProject(t *testing.T) {
	query := "Show exact production configuration files and tests for authentication"
	index := configurationNavigationIndex()

	withoutSelection := ContextPack{Query: query, selectionQuery: query, ProtocolVersion: AdaptiveV2}
	if got := contextConfigurationResources(withoutSelection, index); len(got) != 0 {
		t.Fatalf("unselected index projects leaked into configuration navigation: %#v", got)
	}

	strict := ContextPack{
		Query: query, selectionQuery: query,
		Entrypoints: []ContextLocation{{ID: "billing-route", Project: "services/billing"}},
	}
	if got := contextConfigurationResources(strict, index); len(got) != 0 {
		t.Fatalf("strict-v1 gained adaptive configuration navigation: %#v", got)
	}
}

func TestContextConfigurationResourcesRemainWithinFinalBudgetWhenSaturated(t *testing.T) {
	query := "Show exact production configuration files and tests for authentication"
	pack := ContextPack{
		Schema: scan.SchemaVersion, Query: query, selectionQuery: query,
		ProtocolVersion: AdaptiveV2, Confidence: "EXACT", BudgetTokens: 1100,
		Entrypoints: []ContextLocation{{ID: "billing-route", Project: "services/billing"}},
	}
	index := configurationNavigationIndex()
	for _, profile := range []string{"alpha", "beta", "gamma", "local", "prod", "stage", "test", "zeta"} {
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: "billing-" + profile, Project: "services/billing", Kind: "configuration",
			Name: "technical-user", File: "src/main/resources/application-" + profile + ".properties",
			Confidence: "EXACT", Search: "technical user authentication configuration",
		})
	}
	pack = finalizeContextSourceDecision(pack, index)
	final, err := finalizeContextPackWithinBudget(pack, ContextRequest{BudgetTokens: 1100, MaxFiles: 12})
	if err != nil {
		t.Fatal(err)
	}
	if len(final.ConfigurationResources) == 0 || final.EstimatedTokens > final.BudgetTokens {
		t.Fatalf("final saturated configuration navigation = groups %d, tokens %d/%d", len(final.ConfigurationResources), final.EstimatedTokens, final.BudgetTokens)
	}
}

func TestInferredConfigurationNavigationYieldsToSelectedSourceBudget(t *testing.T) {
	query := "Show exact production configuration files and tests for authentication"
	primary := ContextSourceSection{
		Project: "services/billing", Path: "src/main/java/example/BillingController.java",
		StartLine: 10, EndLine: 30, Role: "entrypoint", RenderMode: "full",
		SourceState: "indexed_range_current", Content: strings.Repeat("x", 500),
	}
	pack := ContextPack{
		Schema: scan.SchemaVersion, Query: query, selectionQuery: query,
		ProtocolVersion: AdaptiveV2, Confidence: "EXACT", SourceCoverage: "complete",
		SourceSections: []ContextSourceSection{primary},
		ConfigurationResources: []ContextConfigurationResourceGroup{{
			Project: "services/billing", KeyGroups: []string{"technical-user"},
			Resources: []ContextConfigurationResource{
				{Path: "src/main/resources/application-alpha-with-a-long-profile-name.properties", Profile: "alpha-with-a-long-profile-name"},
				{Path: "src/main/resources/application-beta-with-a-long-profile-name.properties", Profile: "beta-with-a-long-profile-name"},
				{Path: "src/main/resources/application-gamma-with-a-long-profile-name.properties", Profile: "gamma-with-a-long-profile-name"},
				{Path: "src/main/resources/application-local-with-a-long-profile-name.properties", Profile: "local-with-a-long-profile-name"},
				{Path: "src/main/resources/application-prod-with-a-long-profile-name.properties", Profile: "prod-with-a-long-profile-name"},
				{Path: "src/test/resources/application-test-with-a-long-profile-name.properties", Profile: "test-with-a-long-profile-name"},
			},
		}},
	}
	index := configurationNavigationIndex()

	partial := pack
	partial.BudgetTokens = 400
	partial, handled, err := fitInferredConfigurationNavigationWithinBudget(
		partial, index, ContextRequest{BudgetTokens: 400, MaxFiles: 12},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || len(partial.ConfigurationResources) != 1 ||
		len(partial.ConfigurationResources[0].Resources) == 0 ||
		len(partial.ConfigurationResources[0].Resources) >= 6 {
		t.Fatalf("partially fitted inferred navigation = handled %t, groups %#v", handled, partial.ConfigurationResources)
	}
	if !reflect.DeepEqual(partial.SourceSections, []ContextSourceSection{primary}) ||
		partial.EstimatedTokens > partial.BudgetTokens {
		t.Fatalf("partial fit changed primary source or exceeded budget: source %#v, tokens %d/%d", partial.SourceSections, partial.EstimatedTokens, partial.BudgetTokens)
	}

	tight := pack
	tight.BudgetTokens = MinContextBudgetTokens
	tight, handled, err = fitInferredConfigurationNavigationWithinBudget(
		tight, index, ContextRequest{BudgetTokens: MinContextBudgetTokens, MaxFiles: 12},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || len(tight.ConfigurationResources) != 0 {
		t.Fatalf("tight inferred navigation = handled %t, groups %#v", handled, tight.ConfigurationResources)
	}
	if !reflect.DeepEqual(tight.SourceSections, []ContextSourceSection{primary}) ||
		tight.EstimatedTokens > tight.BudgetTokens {
		t.Fatalf("tight fit changed primary source or exceeded budget: source %#v, tokens %d/%d", tight.SourceSections, tight.EstimatedTokens, tight.BudgetTokens)
	}

	tooTight := cloneContextPack(pack)
	tooTight.BudgetTokens = MinContextBudgetTokens
	tooTight.SourceSections[0].Content = strings.Repeat("x", 2000)
	unchanged, handled, err := fitInferredConfigurationNavigationWithinBudget(
		tooTight, index, ContextRequest{BudgetTokens: MinContextBudgetTokens, MaxFiles: 12},
	)
	if err != nil {
		t.Fatal(err)
	}
	if handled || !reflect.DeepEqual(unchanged.ConfigurationResources, pack.ConfigurationResources) {
		t.Fatalf("navigation trimming hid a source pack that still needs reselection: handled %t, groups %#v", handled, unchanged.ConfigurationResources)
	}
}

func TestConfigurationNavigationBudgetFitDoesNotChangeExplicitOrStrictBehavior(t *testing.T) {
	index := configurationNavigationIndex()
	resources := []ContextConfigurationResourceGroup{{
		Project: "services/billing", KeyGroups: []string{"technical-user"},
		Resources: []ContextConfigurationResource{{Path: "src/main/resources/application.properties", Profile: "production"}},
	}}
	tests := []ContextPack{
		{ProtocolVersion: AdaptiveV2, Query: "Show exact production configuration files for authentication in services/billing", ConfigurationResources: resources},
		{Query: "Show exact production configuration files for authentication", ConfigurationResources: resources},
	}
	for _, pack := range tests {
		pack.BudgetTokens = MinContextBudgetTokens
		got, handled, err := fitInferredConfigurationNavigationWithinBudget(
			pack, index, ContextRequest{BudgetTokens: MinContextBudgetTokens, MaxFiles: 12},
		)
		if err != nil {
			t.Fatal(err)
		}
		if handled || !reflect.DeepEqual(got.ConfigurationResources, resources) {
			t.Fatalf("explicit or strict navigation changed: handled %t, groups %#v", handled, got.ConfigurationResources)
		}
	}
}

func TestFinalSourceDecisionTrimsInferredNavigationBeforeReselectingSource(t *testing.T) {
	root := t.TempDir()
	content := "class BillingController {\n  void charge() {\n" + strings.Repeat("    account.validate();\n", 14) + "  }\n}\n"
	writeSourceFile(t, root, "BillingController.java", content)
	query := "Show exact production configuration files and tests for authentication"
	pack := ContextPack{
		Schema: scan.SchemaVersion, Query: query, selectionQuery: query,
		ProtocolVersion: AdaptiveV2, Confidence: "EXACT", BudgetTokens: 450,
		Concerns: []ContextConcern{{Kind: contextConcernEntrypoint, Project: "services/billing", Covered: true}},
		Entrypoints: []ContextLocation{{
			ID: "billing-route", Project: "services/billing", Kind: "symbol",
			Label: "BillingController.charge", File: "BillingController.java", Line: 2, EndLine: 17,
			Confidence: "EXACT",
		}},
		Files: []ContextFile{{
			Project: "services/billing", Path: "BillingController.java", StartLine: 2, EndLine: 17,
			Role: "entrypoint", Reason: "selected entrypoint", Confidence: "EXACT",
		}},
		selectedSourceFactIDs: []string{"billing-route"},
	}
	index := configurationNavigationIndex()
	index.Facts = append(index.Facts, scan.AgentContextFactRecord{
		ID: "billing-route", Project: "services/billing", Kind: "symbol",
		Name: "charge", Qualified: "BillingController.charge", File: "BillingController.java",
		Line: 2, EndLine: 17, Confidence: "EXACT",
	})
	for _, profile := range []string{"alpha-long-profile", "beta-long-profile", "gamma-long-profile", "local-long-profile", "prod-long-profile", "test-long-profile"} {
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: "config-" + profile, Project: "services/billing", Kind: "configuration",
			Name: "technical-user", File: "src/main/resources/application-" + profile + ".properties",
			Confidence: "EXACT", Search: "technical user authentication configuration",
		})
	}

	got, err := attachContextSourceWithinFinalBudget(
		pack,
		loadedContextIndex{ScopeRoot: root, Index: index},
		ContextRequest{BudgetTokens: 450, MaxFiles: 12, ProtocolVersion: AdaptiveV2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceSections) != 1 || got.SourceSections[0].Path != "BillingController.java" ||
		!strings.Contains(got.SourceSections[0].Content, "account.validate()") {
		t.Fatalf("exact primary source was reselected or lost: %#v", got.SourceSections)
	}
	resourceCount := 0
	for _, group := range got.ConfigurationResources {
		resourceCount += len(group.Resources)
	}
	if resourceCount == 0 || resourceCount >= maximumContextConfigurationResources {
		t.Fatalf("inferred navigation was not partially retained: %#v", got.ConfigurationResources)
	}
	if got.EstimatedTokens > got.BudgetTokens {
		t.Fatalf("final pack exceeded budget: %d/%d", got.EstimatedTokens, got.BudgetTokens)
	}
}

func configurationNavigationIndex() scan.AgentContextIndexRecord {
	return scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "billing-service", Project: "services/billing", Kind: "symbol", Name: "BillingService", File: "src/main/java/example/BillingService.java", Confidence: "EXACT"},
		{ID: "billing-production", Project: "services/billing", Kind: "configuration", Name: "technical-user", File: "src/main/resources/application.properties", Confidence: "EXACT", Summary: "billing-password-value", Search: "technical user authentication configuration"},
		{ID: "billing-test", Project: "services/billing", Kind: "configuration", Name: "technical-user", File: "src/test/resources/application-test.properties", Confidence: "EXACT", Summary: "billing-password-value", Search: "technical user authentication configuration test"},
		{ID: "archive-production", Project: "services/archive", Kind: "configuration", Name: "technical-user", File: "src/main/resources/application.properties", Confidence: "EXACT", Summary: "archive-password-value", Search: "technical user authentication configuration"},
	}}
}
