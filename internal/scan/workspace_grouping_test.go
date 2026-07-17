package scan

import "testing"

func TestBuildWorkspaceArchitectureLayoutUsesFirstDifferentiatingProductionNamespace(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Name: "orders", Indexed: true},
		{Path: "services/billing", Name: "billing", Indexed: true},
	}}
	namespaces := []WorkspaceProjectNamespaceRecord{
		{Project: "services/orders", Namespace: "org.example.commerce.orders", Language: "java", Source: "production_package"},
		{Project: "services/billing", Namespace: "org.example.finance.billing", Language: "java", Source: "production_package"},
	}

	layout := BuildWorkspaceArchitectureLayout(registry, namespaces, WorkspaceDashboardConfig{Schema: 1})

	if got := layout.Service("services/orders").GroupID; got != "org.example.commerce" {
		t.Fatalf("orders group = %q", got)
	}
	if got := layout.Service("services/billing").GroupID; got != "org.example.finance" {
		t.Fatalf("billing group = %q", got)
	}
}

func TestBuildWorkspaceArchitectureLayoutKeepsManualOrderAndPlacesNewServices(t *testing.T) {
	config := WorkspaceDashboardConfig{Schema: 1, Architecture: DashboardArchitectureConfig{
		GroupOrder: []string{"custom"},
		Groups:     map[string]DashboardGroupConfig{"custom": {Label: "Core"}},
		Services:   map[string]DashboardServiceConfig{"services/orders": {Group: "custom", Order: 10}},
	}}
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{Path: "services/orders"}, {Path: "services/new"}}}

	layout := BuildWorkspaceArchitectureLayout(registry, nil, config)

	if !layout.Service("services/orders").Manual || layout.Groups[0].Label != "Core" {
		t.Fatalf("manual layout lost: %#v", layout)
	}
	if layout.Service("services/orders").Order != 10 {
		t.Fatalf("manual order = %d", layout.Service("services/orders").Order)
	}
	if layout.Service("services/new").GroupID == "" {
		t.Fatalf("new service was not auto-placed: %#v", layout)
	}
}

func TestBuildWorkspaceArchitectureLayoutNormalizesSlashNamespacesAndIgnoresTests(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "apps/store", Name: "store", Kind: "frontend"},
		{Path: "apps/admin", Name: "admin", Kind: "frontend"},
	}}
	namespaces := []WorkspaceProjectNamespaceRecord{
		{Project: "apps/store", Namespace: "@example/commerce/store", Language: "typescript", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "apps/admin", Namespace: "@example/operations/admin", Language: "typescript", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "apps/store", Namespace: "@example/tests/store", Language: "typescript", Source: "test_package", Confidence: "EXTRACTED"},
	}

	layout := BuildWorkspaceArchitectureLayout(registry, namespaces, WorkspaceDashboardConfig{Schema: 1})

	store := layout.Service("apps/store")
	if store.GroupID != "@example.commerce" || store.Source != "production_package" || store.Confidence != "EXTRACTED" {
		t.Fatalf("store layout = %#v", store)
	}
	if got := layout.Service("apps/admin").GroupID; got != "@example.operations" {
		t.Fatalf("admin group = %q", got)
	}
}

func TestBuildWorkspaceArchitectureLayoutTracksStaleServices(t *testing.T) {
	config := WorkspaceDashboardConfig{Schema: 1, Architecture: DashboardArchitectureConfig{
		Groups: map[string]DashboardGroupConfig{"custom": {Label: "Core"}},
		Services: map[string]DashboardServiceConfig{
			"services/current": {Group: "custom"},
			"services/removed": {Group: "custom"},
		},
	}}
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{Path: "services/current"}}}

	layout := BuildWorkspaceArchitectureLayout(registry, nil, config)

	if len(layout.StaleServices) != 1 || layout.StaleServices[0] != "services/removed" {
		t.Fatalf("stale services = %#v", layout.StaleServices)
	}
}

func TestBuildWorkspaceServiceMapWithLayoutPublishesArchitectureMetadata(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Name: "Orders", Indexed: true},
		{Path: "services/billing", Name: "Billing", Indexed: true},
	}}
	layout := WorkspaceArchitectureLayoutRecord{
		Groups: []WorkspaceArchitectureGroupRecord{{ID: "commerce", Label: "Commerce", Order: 2, Source: "dashboard_config", Confidence: "EXACT", Manual: true}},
		Services: []WorkspaceArchitectureServiceLayoutRecord{
			{Project: "services/orders", GroupID: "commerce", Order: 5, Source: "dashboard_config", Confidence: "EXACT", Manual: true},
			{Project: "services/billing", GroupID: "commerce", Order: 1, Source: "production_package", Confidence: "EXTRACTED"},
		},
	}

	serviceMap := BuildWorkspaceServiceMapWithLayout(registry, nil, nil, nil, layout)

	if len(serviceMap.ArchitectureGroups) != 1 || serviceMap.ArchitectureGroups[0].ID != "commerce" {
		t.Fatalf("architecture groups = %#v", serviceMap.ArchitectureGroups)
	}
	orders := requireServiceMapNode(t, serviceMap, "services/orders")
	if orders.Domain != "commerce" || orders.ArchitectureOrder != 5 || orders.DomainSource != "dashboard_config" || orders.DomainConfidence != "EXACT" || !orders.DomainManual {
		t.Fatalf("orders node = %#v", orders)
	}
	if serviceMap.Nodes[0].Project != "services/billing" {
		t.Fatalf("service order = %#v", serviceMap.Nodes)
	}
}
