package scan

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func WorkspaceDiff(beforeDir, afterDir string) (WorkspaceDiffRecord, error) {
	before, err := readWorkspaceSnapshot(beforeDir)
	if err != nil {
		return WorkspaceDiffRecord{}, err
	}
	after, err := readWorkspaceSnapshot(afterDir)
	if err != nil {
		return WorkspaceDiffRecord{}, err
	}
	return buildWorkspaceDiff(before, after), nil
}

func readWorkspaceSnapshot(dir string) (WorkspaceSnapshotRecord, error) {
	var snapshot WorkspaceSnapshotRecord
	if err := readWorkspaceJSON(filepath.Join(dir, "contract-matches.json"), &snapshot.Contracts); err != nil {
		return WorkspaceSnapshotRecord{}, fmt.Errorf("read contract matches: %w", err)
	}
	if err := readWorkspaceJSON(filepath.Join(dir, "feature-flows.json"), &snapshot.Flows); err != nil {
		return WorkspaceSnapshotRecord{}, fmt.Errorf("read feature flows: %w", err)
	}
	return snapshot, nil
}

func RenderWorkspaceDiffReport(diff WorkspaceDiffRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Diff\n\n")
	b.WriteString(fmt.Sprintf("- New contracts: %d\n", len(diff.NewContracts)))
	b.WriteString(fmt.Sprintf("- Removed contracts: %d\n", len(diff.RemovedContracts)))
	b.WriteString(fmt.Sprintf("- Changed contracts: %d\n", len(diff.ChangedContracts)))
	b.WriteString(fmt.Sprintf("- Coverage regressions: %d\n", len(diff.CoverageRegressions)))
	if len(diff.ChangedContracts) > 0 {
		b.WriteString("\n## Changed Contracts\n\n")
		for _, record := range diff.ChangedContracts {
			b.WriteString(fmt.Sprintf("- `%s` %s/%s -> %s/%s\n", record.Route, record.BeforeConfidence, record.BeforeIssue, record.AfterConfidence, record.AfterIssue))
		}
	}
	if len(diff.CoverageRegressions) > 0 {
		b.WriteString("\n## Coverage Regressions\n\n")
		for _, regression := range diff.CoverageRegressions {
			b.WriteString("- " + regression + "\n")
		}
	}
	return b.String()
}

func buildWorkspaceDiff(before, after WorkspaceSnapshotRecord) WorkspaceDiffRecord {
	beforeContracts := map[string]WorkspaceContractMatchRecord{}
	afterContracts := map[string]WorkspaceContractMatchRecord{}
	for _, contract := range before.Contracts {
		beforeContracts[contract.ID] = contract
	}
	for _, contract := range after.Contracts {
		afterContracts[contract.ID] = contract
	}
	diff := WorkspaceDiffRecord{}
	for _, contract := range after.Contracts {
		if _, ok := beforeContracts[contract.ID]; !ok {
			diff.NewContracts = append(diff.NewContracts, contract)
			continue
		}
		previous := beforeContracts[contract.ID]
		if previous.Issue != contract.Issue || previous.Confidence != contract.Confidence {
			diff.ChangedContracts = append(diff.ChangedContracts, WorkspaceContractDiffRecord{
				ID:               contract.ID,
				Route:            contract.APIHTTPMethod + " " + contract.APIPath,
				BeforeIssue:      previous.Issue,
				AfterIssue:       contract.Issue,
				BeforeConfidence: previous.Confidence,
				AfterConfidence:  contract.Confidence,
			})
		}
	}
	for _, contract := range before.Contracts {
		if _, ok := afterContracts[contract.ID]; !ok {
			diff.RemovedContracts = append(diff.RemovedContracts, contract)
		}
	}
	beforeFlows := map[string]WorkspaceFeatureFlowRecord{}
	for _, flow := range before.Flows {
		beforeFlows[flow.ID] = flow
	}
	for _, flow := range after.Flows {
		previous, ok := beforeFlows[flow.ID]
		if !ok {
			continue
		}
		if hasMatchedWorkspaceTest(previous.Tests) && !hasMatchedWorkspaceTest(flow.Tests) {
			diff.CoverageRegressions = append(diff.CoverageRegressions, flow.ID+" lost matched tests")
		}
	}
	sort.Slice(diff.NewContracts, func(i, j int) bool { return diff.NewContracts[i].ID < diff.NewContracts[j].ID })
	sort.Slice(diff.RemovedContracts, func(i, j int) bool { return diff.RemovedContracts[i].ID < diff.RemovedContracts[j].ID })
	sort.Slice(diff.ChangedContracts, func(i, j int) bool { return diff.ChangedContracts[i].ID < diff.ChangedContracts[j].ID })
	sort.Strings(diff.CoverageRegressions)
	return diff
}

func hasMatchedWorkspaceTest(tests []TestMapRecord) bool {
	for _, test := range tests {
		if test.Confidence == "MATCHED" {
			return true
		}
	}
	return false
}
