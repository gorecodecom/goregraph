package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func configurationIdentityFixture() (ContextPack, scan.AgentContextIndexRecord) {
	pack, index := intrinsicModelNavigationFixture()
	pack.Query = "Prepare a correction plan listing exact production, configuration and test files."
	pack.selectionQuery = pack.Query
	for _, project := range []string{"services/catalog", "services/jobs", "services/archive"} {
		for _, path := range []string{"src/main/resources/application.properties", "src/main/resources/application-dev.properties", "src/test/resources/application-test.properties"} {
			index.Facts = append(index.Facts, scan.AgentContextFactRecord{ID: project + path, Project: project, Kind: "configuration", Name: filepath.Base(path), File: path, Confidence: "EXACT", Summary: "Spring configuration resource"})
		}
	}
	return pack, index
}

func TestAdaptiveConfigurationResourceIdentityFallback(t *testing.T) {
	pack, index := configurationIdentityFixture()
	before := cloneContextPack(pack)
	got := contextConfigurationResources(pack, index)
	resources := []ContextConfigurationResource{
		{Path: "src/main/resources/application-dev.properties", Profile: "dev"},
		{Path: "src/main/resources/application.properties", Profile: "production"},
		{Path: "src/test/resources/application-test.properties", Profile: "test"},
	}
	want := []ContextConfigurationResourceGroup{{Project: "services/catalog", Resources: resources}, {Project: "services/jobs", Resources: resources}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resource identity navigation = %#v, want %#v", got, want)
	}
	for i, j := 0, len(index.Facts)-1; i < j; i, j = i+1, j-1 {
		index.Facts[i], index.Facts[j] = index.Facts[j], index.Facts[i]
	}
	if !reflect.DeepEqual(got, contextConfigurationResources(pack, index)) {
		t.Fatal("identity selection depends on index order")
	}
	if !reflect.DeepEqual(pack, before) {
		t.Fatal("resource navigation mutated evidence")
	}
}

func TestAdaptiveConfigurationResourceIdentityGuards(t *testing.T) {
	for _, mode := range []string{"stale", "nonexact-source", "missing-source", "ordinary", "no-config-intent", "no-files-intent", "strict", "explicit-caller", "explicit-unselected", "ambiguous-entrypoint", "no-entrypoint", "low-resource", "raw-key-group", "invalid-resource", "represented", "endpoint"} {
		t.Run(mode, func(t *testing.T) {
			pack, index := configurationIdentityFixture()
			wantProjects := map[string]int{"services/catalog": 3}
			switch mode {
			case "stale":
				pack.SourceSections[0].SourceState = "stale"
			case "nonexact-source":
				for i := range index.Facts {
					if index.Facts[i].ID == "selected-model" {
						index.Facts[i].Confidence = "RESOLVED"
					}
				}
			case "missing-source":
				pack.SourceSections = nil
			case "ordinary":
				pack.Query = "Show exact production configuration and test files."
				wantProjects = nil
			case "no-config-intent":
				pack.Query = "Prepare a correction plan listing exact production and test files."
				wantProjects = nil
			case "no-files-intent":
				pack.Query = "Prepare a correction plan for configuration."
				wantProjects = nil
			case "strict":
				pack.ProtocolVersion = StrictV1
				wantProjects = nil
			case "explicit-caller":
				pack.Query += " Limit to services/catalog."
			case "explicit-unselected":
				pack.Query += " Limit to services/archive."
				wantProjects = nil
			case "ambiguous-entrypoint":
				pack.Entrypoints = append(pack.Entrypoints, ContextLocation{Project: "services/archive"})
				wantProjects = nil
			case "no-entrypoint":
				pack.Entrypoints = nil
				wantProjects = nil
			case "low-resource", "raw-key-group", "invalid-resource":
				for i := range index.Facts {
					f := &index.Facts[i]
					if f.Kind != "configuration" || f.Project != "services/jobs" {
						continue
					}
					switch mode {
					case "low-resource":
						f.Confidence = "RESOLVED"
					case "raw-key-group":
						f.Name = "mail.template"
						f.Summary = "private-template-value"
					case "invalid-resource":
						f.File = "/private/application.properties"
					}
				}
			case "represented":
				pack.Files = []ContextFile{{Project: "services/jobs", Path: "src/main/resources/application.properties"}}
				pack.SourceOmissions = []ContextSourceOmission{{Project: "services/jobs", Path: "src/main/resources/application-dev.properties"}}
				wantProjects["services/jobs"] = 1
			case "endpoint":
				pack.SourceSections[0] = ContextSourceSection{Project: "services/jobs", Path: "src/main/java/example/ShipmentController.java", StartLine: 10, EndLine: 12, SourceState: "indexed_range_current"}
				index.Facts = append(index.Facts, scan.AgentContextFactRecord{ID: "selected-endpoint", Project: "services/jobs", Kind: "api_endpoint", Name: "GET /shipments", HTTPMethod: "GET", Path: "/shipments", File: pack.SourceSections[0].Path, Line: 10, Confidence: "EXACT"})
				wantProjects["services/jobs"] = 3
			}
			pack.selectionQuery = pack.Query
			got := contextConfigurationResources(pack, index)
			if len(got) != len(wantProjects) {
				t.Fatalf("%s resources = %#v", mode, got)
			}
			for _, group := range got {
				if len(group.Resources) != wantProjects[group.Project] || len(group.KeyGroups) != 0 {
					t.Fatalf("%s widened identity metadata: %#v", mode, got)
				}
			}
		})
	}
}

func TestAdaptiveConfigurationIdentityFallbackYieldsToRequestedKeyGroups(t *testing.T) {
	pack, index := configurationIdentityFixture()
	pack.Query += " Include authentication and retries."
	pack.selectionQuery = pack.Query
	// A positive match in the provider project suppresses its unrelated resource identities.
	index.Facts = append(index.Facts, scan.AgentContextFactRecord{ID: "auth", Project: "services/jobs", Kind: "configuration", Name: "technical-user", File: "src/main/resources/application.properties", Confidence: "EXACT", Summary: "private-password-value", Search: "authentication"})
	for i := 0; i < 8; i++ {
		path := fmt.Sprintf("src/main/resources/application-extra%d.properties", i)
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{ID: path, Project: "services/catalog", Kind: "configuration", Name: filepath.Base(path), File: path, Confidence: "EXACT", Summary: "Spring configuration resource"})
	}
	got := contextConfigurationResources(pack, index)
	count := 0
	found := false
	for _, group := range got {
		count += len(group.Resources)
		if group.Project == "services/jobs" {
			found = reflect.DeepEqual(group.KeyGroups, []string{"technical-user"}) && len(group.Resources) == 1 && group.Resources[0].Path == "src/main/resources/application.properties"
		} else if group.Project != "services/catalog" || len(group.KeyGroups) != 0 {
			t.Fatalf("unrelated or fabricated keys: %#v", group)
		}
	}
	if !found || count != 6 {
		t.Fatalf("positive match lost priority or resource bound changed: %#v", got)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-password-value") {
		t.Fatal("navigation disclosed a configuration value")
	}
	for i, j := 0, len(index.Facts)-1; i < j; i, j = i+1, j-1 {
		index.Facts[i], index.Facts[j] = index.Facts[j], index.Facts[i]
	}
	if !reflect.DeepEqual(got, contextConfigurationResources(pack, index)) {
		t.Fatal("ranking depends on index order")
	}
}

func TestAdaptiveConfigurationIdentityFallbackKeepsPrimarySourceWithinBudget(t *testing.T) {
	pack, index := configurationIdentityFixture()
	pack.Schema = scan.SchemaVersion
	pack.Confidence = "EXACT"
	pack.ConfigurationResources = contextConfigurationResources(pack, index)
	if len(pack.ConfigurationResources) != 2 {
		t.Fatal("missing identity metadata to exercise budget pruning")
	}
	withoutNavigation := cloneContextPack(pack)
	withoutNavigation.ConfigurationResources = nil
	estimate, err := finalizeContextEstimate(withoutNavigation)
	if err != nil {
		t.Fatal(err)
	}
	pack.BudgetTokens = max(MinContextBudgetTokens, estimate.EstimatedTokens)
	got, handled, err := fitInferredConfigurationNavigationWithinBudget(pack, index, ContextRequest{BudgetTokens: pack.BudgetTokens, MaxFiles: 12})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || got.EstimatedTokens > pack.BudgetTokens || !reflect.DeepEqual(got.SourceSections, pack.SourceSections) {
		t.Fatalf("identity budget pruning affected source: handled=%t pack=%#v", handled, got)
	}
}
