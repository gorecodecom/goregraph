package scan

import (
	"fmt"
	"os"
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
	var traces WorkspaceEndpointTraceIndexRecord
	if err := readWorkspaceJSON(filepath.Join(dir, "workspace-endpoint-traces.json"), &traces); err == nil {
		snapshot.Traces = traces.Traces
	} else if !os.IsNotExist(err) {
		return WorkspaceSnapshotRecord{}, fmt.Errorf("read endpoint traces: %w", err)
	}
	var services WorkspaceServiceMapRecord
	if err := readWorkspaceJSON(filepath.Join(dir, "workspace-service-map.json"), &services); err == nil {
		snapshot.Services = services.Nodes
		snapshot.Capabilities = services.Capabilities
	} else if !os.IsNotExist(err) {
		return WorkspaceSnapshotRecord{}, fmt.Errorf("read service map: %w", err)
	}
	return snapshot, nil
}

func RenderWorkspaceDiffReport(diff WorkspaceDiffRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Diff\n\n")
	b.WriteString(fmt.Sprintf("- New contracts: %d\n", len(diff.NewContracts)))
	b.WriteString(fmt.Sprintf("- Removed contracts: %d\n", len(diff.RemovedContracts)))
	b.WriteString(fmt.Sprintf("- Changed contracts: %d\n", len(diff.ChangedContracts)))
	b.WriteString(fmt.Sprintf("- Added routes: %d\n", len(diff.AddedRoutes)))
	b.WriteString(fmt.Sprintf("- Removed routes: %d\n", len(diff.RemovedRoutes)))
	b.WriteString(fmt.Sprintf("- Changed routes: %d\n", len(diff.ChangedRoutes)))
	b.WriteString(fmt.Sprintf("- New test gaps: %d\n", len(diff.NewTestGaps)))
	b.WriteString(fmt.Sprintf("- Closed test gaps: %d\n", len(diff.ClosedTestGaps)))
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
			diff.NewTestGaps = append(diff.NewTestGaps, flow.ID)
		}
		if !hasMatchedWorkspaceTest(previous.Tests) && hasMatchedWorkspaceTest(flow.Tests) {
			diff.ClosedTestGaps = append(diff.ClosedTestGaps, flow.ID)
		}
	}
	beforeRoutes := map[string]WorkspaceEndpointTraceRecord{}
	for _, trace := range before.Traces {
		beforeRoutes[trace.ID] = trace
	}
	afterRoutes := map[string]WorkspaceEndpointTraceRecord{}
	for _, trace := range after.Traces {
		afterRoutes[trace.ID] = trace
		previous, ok := beforeRoutes[trace.ID]
		if !ok {
			diff.AddedRoutes = append(diff.AddedRoutes, trace)
		} else if previous.Route != trace.Route || previous.Status != trace.Status || previous.Risk != trace.Risk {
			diff.ChangedRoutes = append(diff.ChangedRoutes, WorkspaceRouteDiffRecord{ID: trace.ID, Route: trace.Route, BeforeStatus: previous.Status, AfterStatus: trace.Status, BeforeRisk: previous.Risk, AfterRisk: trace.Risk})
		}
	}
	for _, trace := range before.Traces {
		if _, ok := afterRoutes[trace.ID]; !ok {
			diff.RemovedRoutes = append(diff.RemovedRoutes, trace)
		}
	}
	diff.AddedServices, diff.RemovedServices = stringSetDelta(serviceIDs(before.Services), serviceIDs(after.Services))
	diff.AddedEvidence, diff.RemovedEvidence = stringSetDelta(traceEvidence(before.Traces), traceEvidence(after.Traces))
	beforeCapabilities := capabilityCoverageByID(before.Capabilities)
	for id, coverage := range capabilityCoverageByID(after.Capabilities) {
		if previous, ok := beforeCapabilities[id]; ok && coverageRank(coverage) < coverageRank(previous) {
			diff.CoverageRegressions = append(diff.CoverageRegressions, id+" regressed from "+string(previous)+" to "+string(coverage))
		}
	}
	sort.Slice(diff.NewContracts, func(i, j int) bool { return diff.NewContracts[i].ID < diff.NewContracts[j].ID })
	sort.Slice(diff.RemovedContracts, func(i, j int) bool { return diff.RemovedContracts[i].ID < diff.RemovedContracts[j].ID })
	sort.Slice(diff.ChangedContracts, func(i, j int) bool { return diff.ChangedContracts[i].ID < diff.ChangedContracts[j].ID })
	sort.Slice(diff.AddedRoutes, func(i, j int) bool { return diff.AddedRoutes[i].ID < diff.AddedRoutes[j].ID })
	sort.Slice(diff.RemovedRoutes, func(i, j int) bool { return diff.RemovedRoutes[i].ID < diff.RemovedRoutes[j].ID })
	sort.Slice(diff.ChangedRoutes, func(i, j int) bool { return diff.ChangedRoutes[i].ID < diff.ChangedRoutes[j].ID })
	sort.Strings(diff.NewTestGaps)
	sort.Strings(diff.ClosedTestGaps)
	sort.Strings(diff.CoverageRegressions)
	return diff
}

func serviceIDs(records []WorkspaceServiceNodeRecord) []string {
	result := make([]string, 0, len(records))
	for _, record := range records {
		result = append(result, record.ID)
	}
	return result
}

func traceEvidence(records []WorkspaceEndpointTraceRecord) []string {
	result := []string{}
	for _, record := range records {
		for _, step := range record.Steps {
			result = append(result, step.ID)
		}
	}
	return result
}

func stringSetDelta(before, after []string) ([]string, []string) {
	beforeSet, afterSet := map[string]bool{}, map[string]bool{}
	for _, value := range before {
		beforeSet[value] = true
	}
	for _, value := range after {
		afterSet[value] = true
	}
	added, removed := []string{}, []string{}
	for value := range afterSet {
		if !beforeSet[value] {
			added = append(added, value)
		}
	}
	for value := range beforeSet {
		if !afterSet[value] {
			removed = append(removed, value)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func capabilityCoverageByID(records []CapabilityRecord) map[string]Coverage {
	result := map[string]Coverage{}
	for _, record := range records {
		result[record.Project+":"+record.Language+":"+string(record.ID)] = record.Coverage
	}
	return result
}

func coverageRank(value Coverage) int {
	switch value {
	case CoverageComplete:
		return 4
	case CoveragePartial:
		return 3
	case CoverageUnavailable:
		return 2
	case CoverageFailed:
		return 1
	default:
		return 0
	}
}

func hasMatchedWorkspaceTest(tests []TestMapRecord) bool {
	for _, test := range tests {
		if test.Confidence == "MATCHED" {
			return true
		}
	}
	return false
}
