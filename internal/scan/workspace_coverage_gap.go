package scan

import "path/filepath"

// qualifyIncompleteProviderMatches retains positive evidence while making gaps
// caused by interrupted file analysis explicit to every workspace projection.
func qualifyIncompleteProviderMatches(matches []WorkspaceContractMatchRecord, projects []WorkspaceProjectRecord, manifests map[string]OutputManifest) {
	for i := range matches {
		match := &matches[i]
		if match.Confidence != "UNRESOLVED" && match.Issue != contractIssueUnscanned {
			continue
		}
		switch match.Issue {
		case contractIssueMissingRoute, contractIssueScannedServiceNoRoute, contractIssueIndexedBackendRouteMissing, contractIssueUnscanned:
		default:
			continue
		}
		if match.ServiceCandidate == "" && match.BackendProject == "" {
			continue
		}
		var evidence []string
		for _, project := range projects {
			if project.Path == match.APIProject || !project.Indexed {
				continue
			}
			if match.BackendProject != "" && match.BackendProject != project.Path {
				continue
			}
			if match.ServiceCandidate != "" && match.ServiceCandidate != project.Service && match.ServiceCandidate != project.Name && match.ServiceCandidate != project.Path {
				continue
			}
			manifest := manifests[filepath.Join(project.AbsPath, project.OutputDir)]
			if len(manifest.AnalysisIssues) > 0 {
				evidence = append(evidence, "incomplete_provider_project="+project.Path)
			}
		}
		if len(evidence) == 0 {
			continue
		}
		match.Issue = "provider_analysis_incomplete"
		match.Confidence = "UNRESOLVED"
		match.ConfidenceScore = 0.3
		match.Reason = "provider file analysis is incomplete; the indexed evidence cannot establish whether the route is missing"
		match.ResolutionClass = "provider_analysis_incomplete"
		match.ResolutionHint = "inspect provider manifest analysis_issues and the affected source files, then rebuild with an adequate analysis budget"
		match.ResolutionEvidence = append(match.ResolutionEvidence, evidence...)
		match.LikelyOwner = "analysis_coverage"
		match.MissingRouteKind = ""
	}
}
