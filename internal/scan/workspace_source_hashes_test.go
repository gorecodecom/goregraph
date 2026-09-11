package scan

import "testing"

func TestWorkspaceSourceHashesKeepProjectIdentity(t *testing.T) {
	builder := newWorkspaceAgentContextBuilder(WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "backend", Indexed: true}, {Path: "frontend", Indexed: true},
	}})
	for _, project := range []string{"backend", "frontend"} {
		builder.mergeProjectIndex(AgentContextIndexRecord{Root: project, SourceHashes: map[string]string{
			"src/shared.ts": project + "-hash", "../outside.ts": "unsafe", "/absolute.ts": "unsafe",
		}})
	}
	hashes := builder.index("").SourceHashes
	if len(hashes) != 2 || hashes["backend/src/shared.ts"] != "backend-hash" || hashes["frontend/src/shared.ts"] != "frontend-hash" {
		t.Fatalf("source hashes lost project scope or accepted escaping paths: %#v", hashes)
	}
}
