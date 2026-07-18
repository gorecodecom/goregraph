package agent

import (
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

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
