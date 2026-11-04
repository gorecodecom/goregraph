package agent

import (
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestReviewContextDistributesEvidenceWithinProject(t *testing.T) {
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion}
	for _, fixture := range []struct{ project, name string }{
		{"keeper", "RouteAuthorization"},
		{"keeper", "BranchCheckout"},
		{"neighbor", "NeighborWriteRoute"},
	} {
		file := fixture.name + ".java"
		writeContextSourceFile(t, root, filepath.Join(fixture.project, file), "class "+fixture.name+" { void execute() {} }\n")
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: fixture.name, Project: fixture.project, File: file,
			Kind: "symbol", Name: fixture.name, Qualified: fixture.name,
			Line: 1, EndLine: 1, Confidence: "EXACT",
		})
	}
	loaded := loadedContextIndex{Index: index, ScopeRoot: root, Workspace: true}
	request, err := normalizeContextRequest(ContextRequest{
		Root: filepath.Join(root, "keeper"), Mode: "review", ProtocolVersion: AdaptiveV2,
		Query: "RouteAuthorization; BranchCheckout; NeighborWriteRoute",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, maxFiles := range []int{1, 2} {
		request.MaxFiles = maxFiles
		pack, err := buildReviewContext(loaded, request)
		if err != nil {
			t.Fatal(err)
		}
		if len(pack.Review) != 3 || len(pack.SourceSections) != maxFiles || pack.EstimatedTokens > request.BudgetTokens ||
			!pack.FallbackRequired || pack.SourceCoverage != "partial" || len(pack.Entrypoints) != 0 {
			t.Fatalf("invalid review pack: %+v", pack)
		}
		for i, want := range []string{"RouteAuthorization.java", "BranchCheckout.java", ""} {
			topic := pack.Review[i]
			if i >= maxFiles {
				if len(topic.Sections) != 0 || topic.Gap == "" {
					t.Fatalf("missing gap for topic %d: %+v", i, topic)
				}
				continue
			}
			if len(topic.Sections) != 1 {
				t.Fatalf("topic %d does not reference one section: %+v", i, topic)
			}
			section := pack.SourceSections[topic.Sections[0]-1]
			if section.Project != "keeper" || section.Path != want || section.ReadReceipt == "" {
				t.Fatalf("topic %d has wrong evidence: %+v", i, section)
			}
		}
	}
}
