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
