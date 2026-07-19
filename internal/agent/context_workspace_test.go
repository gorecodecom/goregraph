package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestWorkspaceContextAttachesSourceAcrossProjectRoots(t *testing.T) {
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-19T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "route", Project: "services/users", Kind: "route", Name: "DELETE /users/{id}",
				Qualified: "UserController.deleteUser", HTTPMethod: "DELETE", Path: "/users/{id}",
				File: "src/UserController.java", Line: 2, EndLine: 4, Confidence: "EXACT",
			},
			{
				ID: "service", Project: "services/shared", Kind: "symbol", Name: "removeUser",
				Qualified: "UserOperations.removeUser", File: "src/UserOperations.java",
				Line: 2, EndLine: 4, Confidence: "EXACT",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "call", FromFactID: "route", ToFactID: "service", FromLabel: "deleteUser",
			ToLabel: "removeUser", Kind: "call", Confidence: "EXACT",
		}},
	}
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), index)
	writeContextSourceFile(t, root, "services/users/src/UserController.java", "public class UserController {\n    public void deleteUser() {\n        operations.removeUser();\n    }\n}\n")
	writeContextSourceFile(t, root, "services/shared/src/UserOperations.java", "public class UserOperations {\n    public void removeUser() {\n        repository.delete();\n    }\n}\n")

	pack, err := BuildContext(ContextRequest{Root: root, Query: "DELETE /users/{id}"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.SourceSections) != 2 || pack.SourceCoverage != "complete" ||
		pack.SourceSections[0].Project != "services/users" ||
		pack.SourceSections[1].Project != "services/shared" ||
		!strings.Contains(pack.SourceSections[0].Content, "deleteUser") ||
		!strings.Contains(pack.SourceSections[1].Content, "removeUser") {
		t.Fatalf("workspace source = %#v", pack)
	}
}

func TestBuildContextPreservesWorkspaceRelationshipReason(t *testing.T) {
	workspaceIndex := scan.BuildWorkspaceAgentContextIndex(
		scan.WorkspaceRegistryRecord{Projects: []scan.WorkspaceProjectRecord{{
			Path: "services/users", Indexed: true,
		}}},
		[]scan.AgentContextIndexRecord{{
			Root: "services/users",
			Facts: []scan.AgentContextFactRecord{
				{ID: "route", Kind: "route", Name: "GET /users", Search: "get users"},
				{ID: "service", Kind: "symbol", Name: "findUsers", Qualified: "UserService.findUsers"},
			},
			Edges: []scan.AgentContextEdgeRecord{{
				ID: "call", FromFactID: "route", ToFactID: "service",
				Kind: "call", Reason: "java calls method owner reference", Confidence: "EXACT",
			}, {
				ID: "duplicate-call", FromFactID: "route", ToFactID: "service",
				Kind: "call", File: "Users.java", Line: 42,
				Reason: "java calls method owner reference", Confidence: "EXACT",
			}},
		}},
		nil,
		nil,
		scan.WorkspaceEndpointTraceIndexRecord{},
		scan.APICatalogRecord{},
		"2026-07-16T00:00:00Z",
	)
	root := writeContextIndexFixture(t, workspaceIndex)

	pack, err := BuildContext(ContextRequest{Root: root, Query: "GET /users"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.CallChain) != 1 || pack.CallChain[0].Reason != "method" {
		t.Fatalf("workspace relationship reason = %#v", pack.CallChain)
	}
}
