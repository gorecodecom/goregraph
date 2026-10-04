package scan

func buildWorkspaceProjectDiagnostics(project workspaceIndexProject, matches []WorkspaceContractMatchRecord) []CanonicalDiagnosticRecord {
	var contracts []ContractMatchRecord
	for _, match := range matches {
		if match.APIProject != project.record.Path || match.Issue == contractIssueMatched {
			continue
		}
		contracts = append(contracts, ContractMatchRecord{
			APIHTTPMethod: match.APIHTTPMethod, APIPath: match.APIPath,
			APIFile: match.APIFile, APILine: match.APILine,
			BackendHTTPMethod: match.BackendHTTPMethod, BackendPath: match.BackendPath,
			BackendFile: match.BackendFile, BackendLine: match.BackendLine,
			Issue: match.Issue, Confidence: match.Confidence,
			EvidenceIDs: workspaceFactEvidenceIDs(project, nil, match.APIFile, match.APILine),
		})
	}
	return BuildCanonicalDiagnostics(contracts, project.capabilities)
}
