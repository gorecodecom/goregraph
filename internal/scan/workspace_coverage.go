package scan

import (
	"sort"
	"strings"
)

type WorkspaceCoverageSummaryRecord struct {
	KnownProjects             int                            `json:"known_projects"`
	IndexedProjects           int                            `json:"indexed_projects"`
	ReferencedServices        int                            `json:"referenced_services"`
	IndexedReferencedServices int                            `json:"indexed_referenced_services"`
	ContractSummary           WorkspaceContractSummaryRecord `json:"contract_summary"`
	NextScans                 []NextScanRecord               `json:"next_scans,omitempty"`
}

type NextScanRecord struct {
	Service           string   `json:"service"`
	Project           string   `json:"project,omitempty"`
	AffectedServices  []string `json:"affected_services,omitempty"`
	AffectedContracts int      `json:"affected_contracts"`
	Command           string   `json:"command,omitempty"`
	Reason            string   `json:"reason"`
}

func BuildWorkspaceCoverage(context WorkspaceContextRecord, summary WorkspaceContractSummaryRecord) WorkspaceCoverageSummaryRecord {
	result := WorkspaceCoverageSummaryRecord{KnownProjects: len(context.Projects), ReferencedServices: len(context.ReferencedServices), ContractSummary: summary}
	for _, project := range context.Projects {
		if project.Indexed {
			result.IndexedProjects++
		}
	}
	indexedServices := map[string]bool{}
	for _, service := range context.KnownServices {
		indexedServices[service] = true
	}
	for _, service := range context.ReferencedServices {
		if indexedServices[service] {
			result.IndexedReferencedServices++
		}
	}
	for _, missing := range context.MissingServiceDetails {
		next := NextScanRecord{Service: missing.Service, Project: missing.Project, AffectedServices: nonEmptyStrings(missing.Service), AffectedContracts: missing.Contracts, Reason: "This referenced service is not indexed; scanning it may resolve affected contracts."}
		if missing.Project != "" {
			next.Command = "goregraph scan " + quoteCommandArgument(missing.Project)
		}
		result.NextScans = append(result.NextScans, next)
	}
	sort.Slice(result.NextScans, func(i, j int) bool {
		if result.NextScans[i].AffectedContracts != result.NextScans[j].AffectedContracts {
			return result.NextScans[i].AffectedContracts > result.NextScans[j].AffectedContracts
		}
		if result.NextScans[i].Project != result.NextScans[j].Project {
			return result.NextScans[i].Project < result.NextScans[j].Project
		}
		return result.NextScans[i].Service < result.NextScans[j].Service
	})
	return result
}

func quoteCommandArgument(value string) string {
	if strings.ContainsAny(value, " \t\"") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}
