package scan

import "testing"

func TestContractMatchesRejectsUnsafeContractWithoutStaticPath(t *testing.T) {
	matches := buildContractMatches(
		[]APIContractRecord{{
			HTTPMethod: "DELETE", RawPath: "request.resolvePath()", UnsafeDynamic: true,
			File: "src/main/java/example/JobClient.java", Line: 12,
		}},
		[]CodeRouteRecord{{
			Kind: "backend", HTTPMethod: "DELETE", Path: "/",
			Handler: "RootController.delete", File: "RootController.java", Line: 8,
		}},
	)
	if len(matches) != 1 || matches[0].Issue != contractIssueUnsafeDynamic || matches[0].BackendPath != "" {
		t.Fatalf("empty unsafe path matched a provider: %#v", matches)
	}
}

func TestPathsCompatibleDoesNotReplacePlaceholderWithFixedStaticValue(t *testing.T) {
	if pathsCompatible("/records/{type}", "/records/new") {
		t.Fatal("placeholder {type} should not exactly match an arbitrary static segment")
	}
}

func TestKnownBasePrefixVariantsUseTechnicalSuffixes(t *testing.T) {
	for _, prefix := range []string{"billingservice", "billingapi", "billingmgmt", "billingsvc"} {
		if !pathsCompatibleWithKnownBasePrefixes("/"+prefix+"/items/{id}", "/items/{itemId}") {
			t.Errorf("technical service prefix %q was not handled structurally", prefix)
		}
	}
}

func TestAbsentRouteConstantRemainsUnresolved(t *testing.T) {
	if pathsCompatibleWithKnownBasePrefixes("/MissingRoutes.BASE_PATH/items/{id}", "/inventory/items/{itemId}") {
		t.Fatal("constant absent from indexed source was replaced or ignored")
	}
}
