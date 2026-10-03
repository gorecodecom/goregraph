package scan

import "testing"

func TestWorkspaceCompilerSnapshotsRetainTheirProjectScope(t *testing.T) {
	builder := newWorkspaceAgentContextBuilder(WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "apps/client", Indexed: true}, {Path: "services/worker", Indexed: true},
	}})
	for _, project := range []string{"apps/client", "services/worker", "outside"} {
		builder.mergeProjectIndex(AgentContextIndexRecord{Root: project, SemanticDependencies: []SemanticDependencyRecord{{Language: "csharp", Inputs: map[string]string{"Service.cs": project + "-hash"}}}})
	}
	dependencies := builder.index("fixture").SemanticDependencies
	if len(dependencies) != 2 {
		t.Fatal(dependencies)
	}
	for _, dependency := range dependencies {
		if dependency.Inputs["Service.cs"] != dependency.Project+"-hash" || dependency.Project == "outside" {
			t.Fatal("compiler input escaped or lost its project scope", dependency)
		}
	}
}
