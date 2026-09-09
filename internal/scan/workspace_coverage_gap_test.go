package scan

import (
	"path/filepath"
	"testing"
)

func TestInterruptedProviderAnalysisDoesNotProveMissingRoute(t *testing.T) {
	projects := []WorkspaceProjectRecord{{Path: "web"}, {Path: "orders", Name: "orders", AbsPath: "orders", OutputDir: "out", Indexed: true}}
	manifests := map[string]OutputManifest{filepath.Join("orders", "out"): {AnalysisIssues: []AnalysisIssue{{File: "routes.ts"}}}}
	matches := []WorkspaceContractMatchRecord{
		{APIProject: "web", ServiceCandidate: "orders", Issue: contractIssueUnscanned, Confidence: "OUT_OF_SCOPE"},
		{APIProject: "web", Issue: contractIssueMissingRoute, Confidence: "UNRESOLVED"},
		{APIProject: "web", BackendProject: "orders", Issue: contractIssueMatched, Confidence: "RESOLVED"},
		{APIProject: "web", ServiceCandidate: "other", Issue: contractIssueUnscanned, Confidence: "OUT_OF_SCOPE"},
		{APIProject: "web", ServiceCandidate: "orders", Issue: contractIssueUnsafeDynamic, Confidence: "UNRESOLVED"},
	}
	qualifyIncompleteProviderMatches(matches, projects, manifests)
	for _, index := range []int{0} {
		if matches[index].Issue != "provider_analysis_incomplete" || matches[index].Confidence != "UNRESOLVED" || len(matches[index].ResolutionEvidence) != 1 {
			t.Fatalf("match %d claims absence from partial analysis: %+v", index, matches[index])
		}
	}
	if matches[1].Issue != contractIssueMissingRoute || matches[2].Confidence != "RESOLVED" || matches[3].Issue != contractIssueUnscanned || matches[4].Issue != contractIssueUnsafeDynamic {
		t.Fatal("positive or unrelated evidence changed")
	}
}
