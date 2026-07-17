package scan

import (
	"reflect"
	"testing"
)

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

func TestBuildWorkspaceArchitectureLayoutKeepsDifferentiationAcrossRootFamiliesAndPermutations(t *testing.T) {
	projects := []WorkspaceProjectRecord{
		{Path: "services/orders", Name: "orders", Kind: "backend"},
		{Path: "services/billing", Name: "billing", Kind: "backend"},
		{Path: "services/audit", Name: "audit", Kind: "backend"},
	}
	namespaces := []WorkspaceProjectNamespaceRecord{
		{Project: "services/orders", Namespace: "org.example.commerce.orders", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "services/billing", Namespace: "org.example.finance.billing", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "services/audit", Namespace: "io.sample.operations.audit", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
	}
	reversedProjects := []WorkspaceProjectRecord{projects[2], projects[1], projects[0]}
	reversedNamespaces := []WorkspaceProjectNamespaceRecord{namespaces[2], namespaces[1], namespaces[0]}

	first := BuildWorkspaceArchitectureLayout(WorkspaceRegistryRecord{Projects: projects}, namespaces, WorkspaceDashboardConfig{Schema: 1})
	second := BuildWorkspaceArchitectureLayout(WorkspaceRegistryRecord{Projects: reversedProjects}, reversedNamespaces, WorkspaceDashboardConfig{Schema: 1})

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("permuted layout differs:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	if got := first.Service("services/orders").GroupID; got != "org.example.commerce" {
		t.Fatalf("orders group = %q", got)
	}
	if got := first.Service("services/billing").GroupID; got != "org.example.finance" {
		t.Fatalf("billing group = %q", got)
	}
	if got := first.Service("services/audit").GroupID; got != "io.sample.operations" {
		t.Fatalf("audit group = %q", got)
	}
}

func TestBuildWorkspaceArchitectureLayoutCohortsSameRootSubtreesIndependently(t *testing.T) {
	projects := []WorkspaceProjectRecord{
		{Path: "services/orders", Name: "orders", Kind: "backend"},
		{Path: "services/billing", Name: "billing", Kind: "backend"},
		{Path: "services/audit", Name: "audit", Kind: "backend"},
	}
	namespaces := []WorkspaceProjectNamespaceRecord{
		{Project: "services/orders", Namespace: "org.example.commerce.orders", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "services/billing", Namespace: "org.example.finance.billing", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "services/audit", Namespace: "org.other.operations.audit", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
	}
	reversedProjects := []WorkspaceProjectRecord{projects[2], projects[1], projects[0]}
	reversedNamespaces := []WorkspaceProjectNamespaceRecord{namespaces[2], namespaces[1], namespaces[0]}

	first := BuildWorkspaceArchitectureLayout(WorkspaceRegistryRecord{Projects: projects}, namespaces, WorkspaceDashboardConfig{Schema: 1})
	second := BuildWorkspaceArchitectureLayout(WorkspaceRegistryRecord{Projects: reversedProjects}, reversedNamespaces, WorkspaceDashboardConfig{Schema: 1})

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("permuted layout differs:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	if got := first.Service("services/orders").GroupID; got != "org.example.commerce" {
		t.Fatalf("orders group = %q", got)
	}
	if got := first.Service("services/billing").GroupID; got != "org.example.finance" {
		t.Fatalf("billing group = %q", got)
	}
	if got := first.Service("services/audit").GroupID; got != "org.other.operations" {
		t.Fatalf("audit group = %q", got)
	}
}

func TestBuildWorkspaceArchitectureLayoutAggregatesCoherentSiblingPackages(t *testing.T) {
	testCases := []struct {
		name       string
		project    WorkspaceProjectRecord
		namespaces []WorkspaceProjectNamespaceRecord
		want       string
	}{
		{
			name:    "java",
			project: WorkspaceProjectRecord{Path: "services/orders", Name: "orders", Kind: "backend"},
			namespaces: []WorkspaceProjectNamespaceRecord{
				{Project: "services/orders", Namespace: "org.example.commerce.api", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
				{Project: "services/orders", Namespace: "org.example.commerce.domain", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
			},
			want: "org.example.commerce",
		},
		{
			name:    "typescript",
			project: WorkspaceProjectRecord{Path: "apps/store", Name: "store", Kind: "frontend"},
			namespaces: []WorkspaceProjectNamespaceRecord{
				{Project: "apps/store", Namespace: "@example/commerce/api", Language: "typescript", Source: "production_package", Confidence: "EXTRACTED"},
				{Project: "apps/store", Namespace: "@example/commerce/domain", Language: "typescript", Source: "production_package", Confidence: "EXTRACTED"},
			},
			want: "@example.commerce",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			layout := BuildWorkspaceArchitectureLayout(
				WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{testCase.project}},
				testCase.namespaces,
				WorkspaceDashboardConfig{Schema: 1},
			)

			service := layout.Service(testCase.project.Path)
			if service.GroupID != testCase.want || service.Source != "production_package" || service.Confidence != "EXTRACTED" {
				t.Fatalf("service layout = %#v", service)
			}
		})
	}
}

func TestBuildWorkspaceArchitectureLayoutAggregatesDominantNamespaceFamilyRegardlessOfInputOrder(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Name: "orders", Kind: "backend"},
		{Path: "services/billing", Name: "billing", Kind: "backend"},
	}}
	orders := []WorkspaceProjectNamespaceRecord{
		{Project: "services/orders", Namespace: "org.example.commerce.orders.api", Language: "java", Source: "production_package", Confidence: "NORMALIZED"},
		{Project: "services/orders", Namespace: "org.example.commerce.orders.domain", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "services/orders", Namespace: "io.sample.orders", Language: "java", Source: "production_package", Confidence: "EXACT"},
	}
	billing := WorkspaceProjectNamespaceRecord{Project: "services/billing", Namespace: "org.example.finance.billing", Language: "java", Source: "production_package", Confidence: "EXTRACTED"}
	forward := append(append([]WorkspaceProjectNamespaceRecord(nil), orders...), billing)
	reversed := []WorkspaceProjectNamespaceRecord{billing, orders[2], orders[1], orders[0]}

	first := BuildWorkspaceArchitectureLayout(registry, forward, WorkspaceDashboardConfig{Schema: 1})
	second := BuildWorkspaceArchitectureLayout(registry, reversed, WorkspaceDashboardConfig{Schema: 1})

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("reversed evidence changed layout:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	ordersLayout := first.Service("services/orders")
	if ordersLayout.GroupID != "org.example.commerce" || ordersLayout.Confidence != "EXTRACTED" {
		t.Fatalf("orders layout = %#v", ordersLayout)
	}
}

func TestBuildWorkspaceArchitectureLayoutFindsDominantNamespaceFamilyWithinSameRoot(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{Path: "services/orders", Name: "orders", Kind: "backend"}}}
	namespaces := []WorkspaceProjectNamespaceRecord{
		{Project: "services/orders", Namespace: "org.example.commerce.orders.api", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "services/orders", Namespace: "org.other.orders", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "services/orders", Namespace: "org.example.commerce.orders.domain", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
	}

	layout := BuildWorkspaceArchitectureLayout(registry, namespaces, WorkspaceDashboardConfig{Schema: 1})

	service := layout.Service("services/orders")
	if service.GroupID != "org.example.commerce" || service.Confidence != "EXTRACTED" {
		t.Fatalf("service layout = %#v", service)
	}
}

func TestBuildWorkspaceArchitectureLayoutUsesConfidenceToResolveEqualNamespaceFamilyFrequency(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{Path: "services/orders", Name: "orders", Kind: "backend"}}}
	namespaces := []WorkspaceProjectNamespaceRecord{
		{Project: "services/orders", Namespace: "io.sample.orders", Language: "java", Source: "production_package", Confidence: "INFERRED"},
		{Project: "services/orders", Namespace: "org.example.commerce.orders", Language: "java", Source: "production_package", Confidence: "EXACT"},
	}

	layout := BuildWorkspaceArchitectureLayout(registry, namespaces, WorkspaceDashboardConfig{Schema: 1})

	service := layout.Service("services/orders")
	if service.GroupID != "org.example.commerce" || service.Confidence != "EXACT" {
		t.Fatalf("service layout = %#v", service)
	}
}

func TestBuildWorkspaceArchitectureLayoutFallsBackOnUnresolvedNamespaceFamilyTie(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{Path: "services/orders", Name: "orders", Kind: "backend"}}}
	namespaces := []WorkspaceProjectNamespaceRecord{
		{Project: "services/orders", Namespace: "org.example.commerce.orders", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: "services/orders", Namespace: "io.sample.orders", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
	}

	layout := BuildWorkspaceArchitectureLayout(registry, namespaces, WorkspaceDashboardConfig{Schema: 1})

	service := layout.Service("services/orders")
	if service.GroupID != "backend:services" || service.Source != "workspace_path" || service.Confidence != "PARTIAL" {
		t.Fatalf("service fallback = %#v", service)
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
