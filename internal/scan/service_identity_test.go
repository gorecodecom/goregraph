package scan

import (
	"reflect"
	"testing"
)

func TestCanonicalServiceIdentityVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "camel case wrappers", input: "InventoryMgmtService", want: []string{"inventory"}},
		{name: "path wrappers", input: "services/inventory-service", want: []string{"inventory"}},
		{name: "compound client", input: "OrderCatalogClient", want: []string{"ordercatalog"}},
		{name: "compound api", input: "order-catalog-api", want: []string{"ordercatalog"}},
		{name: "terminal plural", input: "UserDetailsService", want: []string{"userdetail", "userdetails"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := canonicalServiceIdentityVariants(test.input)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("canonicalServiceIdentityVariants(%q) = %#v, want %#v", test.input, got, test.want)
			}
		})
	}
}

func TestResolveWorkspaceProjectByServiceKey(t *testing.T) {
	projects := []WorkspaceProjectRecord{
		{Name: "order-service", Service: "orders-api", Path: "services/order-service"},
		{Name: "inventory-service", Service: "inventory-service", Path: "services/inventory-service"},
	}

	project, candidates, ok := resolveWorkspaceProjectByServiceKey(projects, "InventoryMgmtService")
	if !ok {
		t.Fatalf("resolveWorkspaceProjectByServiceKey() did not resolve; candidates = %#v", candidates)
	}
	if project.Path != "services/inventory-service" {
		t.Fatalf("resolved project path = %q, want services/inventory-service", project.Path)
	}
	wantCandidates := []string{"services/inventory-service"}
	if !reflect.DeepEqual(candidates, wantCandidates) {
		t.Fatalf("candidates = %#v, want %#v", candidates, wantCandidates)
	}
}

func TestResolveWorkspaceProjectByServiceKeyDoesNotUseSubstringMatches(t *testing.T) {
	projects := []WorkspaceProjectRecord{{
		Name:    "user-service",
		Service: "user-service",
		Path:    "services/user-service",
	}}

	project, candidates, ok := resolveWorkspaceProjectByServiceKey(projects, "UserDetailsService")
	if ok || project.Path != "" || len(candidates) != 0 {
		t.Fatalf("unexpected substring resolution: project=%#v candidates=%#v ok=%t", project, candidates, ok)
	}
}

func TestResolveWorkspaceProjectByServiceKeyPreservesAmbiguity(t *testing.T) {
	projects := []WorkspaceProjectRecord{
		{Name: "inventory-api", Path: "services/inventory-api"},
		{Name: "inventory-service", Path: "modules/inventory-service"},
	}

	project, candidates, ok := resolveWorkspaceProjectByServiceKey(projects, "InventoryClient")
	if ok || project.Path != "" {
		t.Fatalf("ambiguous resolution selected a project: project=%#v ok=%t", project, ok)
	}
	wantCandidates := []string{"modules/inventory-service", "services/inventory-api"}
	if !reflect.DeepEqual(candidates, wantCandidates) {
		t.Fatalf("candidates = %#v, want %#v", candidates, wantCandidates)
	}
}
