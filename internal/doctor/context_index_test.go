package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDoctorRejectsMalformedContextIndex(t *testing.T) {
	root := contextIndexProject(t)
	path := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(root)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 || !containsLine(result.Lines, "context-index.json invalid") {
		t.Fatalf("malformed context index passed Doctor: %v", result.Lines)
	}
}

func TestDoctorRejectsDuplicateContextIndexFactIDs(t *testing.T) {
	root := contextIndexProject(t)
	path := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
	writeTestJSON(t, path, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "duplicate", Kind: "symbol", Name: "First"},
			{ID: "duplicate", Kind: "symbol", Name: "Second"},
		},
		Edges: []scan.AgentContextEdgeRecord{},
	})

	result, err := Run(root)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 || !containsLine(result.Lines, "duplicate fact ID") {
		t.Fatalf("duplicate context fact IDs passed Doctor: %v", result.Lines)
	}
}

func TestDoctorRejectsDanglingContextIndexFactReferences(t *testing.T) {
	for _, test := range []struct {
		name string
		edge scan.AgentContextEdgeRecord
	}{
		{
			name: "from",
			edge: scan.AgentContextEdgeRecord{
				ID: "edge", FromFactID: "missing", ToFactID: "known",
				FromLabel: "Missing", ToLabel: "Known", Kind: "call",
			},
		},
		{
			name: "to",
			edge: scan.AgentContextEdgeRecord{
				ID: "edge", FromFactID: "known", ToFactID: "missing",
				FromLabel: "Known", ToLabel: "Missing", Kind: "call",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := contextIndexProject(t)
			path := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
			writeTestJSON(t, path, scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts: []scan.AgentContextFactRecord{{
					ID: "known", Kind: "symbol", Name: "Known",
				}},
				Edges: []scan.AgentContextEdgeRecord{test.edge},
			})

			result, err := Run(root)

			if err != nil {
				t.Fatal(err)
			}
			if result.Failures == 0 || !containsLine(result.Lines, "dangling "+test.name+"_fact_id") {
				t.Fatalf("dangling %s fact reference passed Doctor: %v", test.name, result.Lines)
			}
		})
	}
}

func TestDoctorRejectsInvalidContextIndexPaths(t *testing.T) {
	for _, file := range []string{
		"/tmp/UserController.java",
		"C:/src/UserController.java",
		`\\server\share\UserController.java`,
		"src/../UserController.java",
	} {
		t.Run(file, func(t *testing.T) {
			root := contextIndexProject(t)
			path := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
			writeTestJSON(t, path, scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts: []scan.AgentContextFactRecord{{
					ID: "fact", Kind: "symbol", Name: "UserController", File: file,
				}},
				Edges: []scan.AgentContextEdgeRecord{},
			})

			result, err := Run(root)

			if err != nil {
				t.Fatal(err)
			}
			if result.Failures == 0 || !containsLine(result.Lines, "invalid relative path") {
				t.Fatalf("invalid context path %q passed Doctor: %v", file, result.Lines)
			}
		})
	}
}

func TestDoctorRejectsUnsortedContextIndexRecords(t *testing.T) {
	sortedFacts := []scan.AgentContextFactRecord{
		{ID: "a", Project: "app", Kind: "symbol", Name: "A"},
		{ID: "b", Project: "app", Kind: "symbol", Name: "B"},
	}
	for _, test := range []struct {
		name  string
		index scan.AgentContextIndexRecord
	}{
		{
			name: "facts",
			index: scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts:         []scan.AgentContextFactRecord{sortedFacts[1], sortedFacts[0]},
				Edges:         []scan.AgentContextEdgeRecord{},
			},
		},
		{
			name: "edges",
			index: scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts:         sortedFacts,
				Edges: []scan.AgentContextEdgeRecord{
					{ID: "edge-b", FromFactID: "b", ToFactID: "a", FromLabel: "B", ToLabel: "A", Kind: "call"},
					{ID: "edge-a", FromFactID: "a", ToFactID: "b", FromLabel: "A", ToLabel: "B", Kind: "call"},
				},
			},
		},
		{
			name: "coverage",
			index: scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts:         sortedFacts,
				Edges:         []scan.AgentContextEdgeRecord{},
				Coverage: []scan.AgentContextCoverageRecord{
					{Project: "app", Capability: "tests", Coverage: "COMPLETE", Reason: "ok"},
					{Project: "app", Capability: "routes", Coverage: "COMPLETE", Reason: "ok"},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := contextIndexProject(t)
			path := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
			writeTestJSON(t, path, test.index)

			result, err := Run(root)

			if err != nil {
				t.Fatal(err)
			}
			if result.Failures == 0 || !containsLine(result.Lines, "unsorted "+test.name) {
				t.Fatalf("unsorted context %s passed Doctor: %v", test.name, result.Lines)
			}
		})
	}
}

func TestDoctorRejectsWrongContextIndexSchema(t *testing.T) {
	root := contextIndexProject(t)
	path := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
	writeTestJSON(t, path, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion - 1,
		Facts:         []scan.AgentContextFactRecord{},
		Edges:         []scan.AgentContextEdgeRecord{},
	})

	result, err := Run(root)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 || !containsLine(result.Lines, "context index schema") {
		t.Fatalf("wrong context schema passed Doctor: %v", result.Lines)
	}
}

func TestDoctorRejectsInvalidContextIndexEdgeIdentity(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{ID: "a", Kind: "symbol", Name: "A"},
		{ID: "b", Kind: "symbol", Name: "B"},
	}
	for _, test := range []struct {
		name  string
		edges []scan.AgentContextEdgeRecord
		want  string
	}{
		{
			name: "empty edge ID",
			edges: []scan.AgentContextEdgeRecord{{
				FromFactID: "a", ToFactID: "b", FromLabel: "A", ToLabel: "B", Kind: "call",
			}},
			want: "empty edge ID",
		},
		{
			name: "duplicate edge ID",
			edges: []scan.AgentContextEdgeRecord{
				{ID: "duplicate", FromFactID: "a", ToFactID: "b", FromLabel: "A", ToLabel: "B", Kind: "call"},
				{ID: "duplicate", FromFactID: "b", ToFactID: "a", FromLabel: "B", ToLabel: "A", Kind: "call"},
			},
			want: "duplicate ID",
		},
		{
			name: "fact edge ID collision",
			edges: []scan.AgentContextEdgeRecord{{
				ID: "a", FromFactID: "a", ToFactID: "b", FromLabel: "A", ToLabel: "B", Kind: "call",
			}},
			want: "duplicate ID",
		},
		{
			name: "empty from fact ID",
			edges: []scan.AgentContextEdgeRecord{{
				ID: "edge", ToFactID: "b", FromLabel: "A", ToLabel: "B", Kind: "call",
			}},
			want: "empty from_fact_id",
		},
		{
			name: "empty to fact ID",
			edges: []scan.AgentContextEdgeRecord{{
				ID: "edge", FromFactID: "a", FromLabel: "A", ToLabel: "B", Kind: "call",
			}},
			want: "empty to_fact_id",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := contextIndexProject(t)
			path := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
			writeTestJSON(t, path, scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts:         facts,
				Edges:         test.edges,
			})

			result, err := Run(root)

			if err != nil {
				t.Fatal(err)
			}
			if result.Failures == 0 || !containsLine(result.Lines, test.want) {
				t.Fatalf("%s passed Doctor: %v", test.name, result.Lines)
			}
		})
	}
}

func TestDoctorRejectsNegativeContextIndexLines(t *testing.T) {
	for _, test := range []struct {
		name  string
		index scan.AgentContextIndexRecord
	}{
		{
			name: "fact line",
			index: scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts: []scan.AgentContextFactRecord{{
					ID: "fact", Kind: "symbol", Name: "Fact", Line: -1,
				}},
				Edges: []scan.AgentContextEdgeRecord{},
			},
		},
		{
			name: "fact end line",
			index: scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts: []scan.AgentContextFactRecord{{
					ID: "fact", Kind: "symbol", Name: "Fact", EndLine: -1,
				}},
				Edges: []scan.AgentContextEdgeRecord{},
			},
		},
		{
			name: "edge line",
			index: scan.AgentContextIndexRecord{
				SchemaVersion: scan.SchemaVersion,
				Facts: []scan.AgentContextFactRecord{
					{ID: "a", Kind: "symbol", Name: "A"},
					{ID: "b", Kind: "symbol", Name: "B"},
				},
				Edges: []scan.AgentContextEdgeRecord{{
					ID: "edge", FromFactID: "a", ToFactID: "b",
					FromLabel: "A", ToLabel: "B", Kind: "call", Line: -1,
				}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := contextIndexProject(t)
			path := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
			writeTestJSON(t, path, test.index)

			result, err := Run(root)

			if err != nil {
				t.Fatal(err)
			}
			if result.Failures == 0 || !containsLine(result.Lines, "negative") {
				t.Fatalf("negative %s passed Doctor: %v", test.name, result.Lines)
			}
		})
	}
}

func TestDoctorRejectsCompleteAgentManifestWithoutContextIndex(t *testing.T) {
	root := contextIndexProject(t)
	path := filepath.Join(root, "goregraph-out", "manifest.json")
	var manifest scan.Manifest
	readTestJSON(t, path, &manifest)
	manifest.Agent.Files = []string{"agent/agent-guide.md"}
	writeTestJSON(t, path, manifest)

	result, err := Run(root)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 || !containsLine(result.Lines, "omits required file agent/context-index.json") {
		t.Fatalf("guide-only complete agent manifest passed Doctor: %v", result.Lines)
	}
}

func contextIndexProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	return root
}
