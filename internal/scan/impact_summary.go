package scan

import (
	"sort"
	"strings"
)

type ImpactItemRecord struct {
	ID           string   `json:"id"`
	Relationship string   `json:"relationship"`
	Kind         string   `json:"kind"`
	Project      string   `json:"project,omitempty"`
	Service      string   `json:"service,omitempty"`
	Symbol       string   `json:"symbol,omitempty"`
	File         string   `json:"file,omitempty"`
	Line         int      `json:"line,omitempty"`
	Depth        int      `json:"depth"`
	Confidence   string   `json:"confidence"`
	Reason       string   `json:"reason"`
	EvidenceIDs  []string `json:"evidence_ids,omitempty"`
}

type ImpactSummaryRecord struct {
	ID                  string             `json:"id"`
	TargetID            string             `json:"target_id"`
	TargetLabel         string             `json:"target_label"`
	RiskLevel           string             `json:"risk_level"`
	RiskReasons         []string           `json:"risk_reasons"`
	DirectConsumers     []ImpactItemRecord `json:"direct_consumers"`
	IndirectConsumers   []ImpactItemRecord `json:"indirect_consumers"`
	DependentTests      []ImpactItemRecord `json:"dependent_tests"`
	AffectedPackages    []string           `json:"affected_packages"`
	PublicAPISurface    []ImpactItemRecord `json:"public_api_surface"`
	CoverageUncertainty []string           `json:"coverage_uncertainty"`
	MaxDepth            int                `json:"max_depth"`
}

func BuildImpactSummaries(flows []WorkspaceFeatureFlowRecord, _ WorkspaceServiceMapRecord, coverage WorkspaceCoverageSummaryRecord, maxDepth int) []ImpactSummaryRecord {
	if maxDepth < 1 {
		maxDepth = 1
	}
	result := make([]ImpactSummaryRecord, 0, len(flows))
	for _, flow := range flows {
		nodes := map[string]CanonicalFlowNodeRecord{}
		incoming := map[string][]CanonicalFlowEdgeRecord{}
		for _, node := range flow.Nodes {
			nodes[node.ID] = node
		}
		for _, edge := range flow.Edges {
			incoming[edge.ToNodeID] = append(incoming[edge.ToNodeID], edge)
		}
		target := impactTarget(flow.Nodes)
		if target.ID == "" {
			continue
		}
		summary := ImpactSummaryRecord{ID: StableWorkspaceID("impact-summary", flow.ID, target.ID), TargetID: flow.ID, TargetLabel: firstNonEmpty(stringsTrimSpace(flow.HTTPMethod+" "+flow.Path), target.Symbol, target.ID), MaxDepth: maxDepth}
		queue := []impactQueueItem{{id: target.ID, depth: 0}}
		visited := map[string]bool{target.ID: true}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if current.depth >= maxDepth {
				continue
			}
			edges := append([]CanonicalFlowEdgeRecord(nil), incoming[current.id]...)
			sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
			for _, edge := range edges {
				node, ok := nodes[edge.FromNodeID]
				if !ok {
					continue
				}
				depth := current.depth + 1
				item := impactItem(node, edge, depth, "indirect_consumer")
				if depth == 1 {
					item.Relationship = "direct_consumer"
					summary.DirectConsumers = append(summary.DirectConsumers, item)
				} else {
					summary.IndirectConsumers = append(summary.IndirectConsumers, item)
				}
				if !visited[node.ID] {
					visited[node.ID] = true
					queue = append(queue, impactQueueItem{id: node.ID, depth: depth})
				}
			}
		}
		packages := []string{}
		for _, node := range flow.Nodes {
			if node.Project != "" {
				packages = append(packages, node.Project)
			}
			if node.Kind == "test" {
				summary.DependentTests = append(summary.DependentTests, impactItem(node, CanonicalFlowEdgeRecord{Confidence: node.Confidence, Reason: node.Reason, EvidenceIDs: node.EvidenceIDs}, 1, "dependent_test"))
			}
			if node.Kind == "endpoint" || node.Kind == "api_call" {
				summary.PublicAPISurface = append(summary.PublicAPISurface, impactItem(node, CanonicalFlowEdgeRecord{Confidence: node.Confidence, Reason: node.Reason, EvidenceIDs: node.EvidenceIDs}, 0, "public_api"))
			}
		}
		summary.AffectedPackages = uniqueSortedStrings(packages)
		if coverage.KnownProjects > coverage.IndexedProjects {
			summary.CoverageUncertainty = append(summary.CoverageUncertainty, "Not every known project is indexed, so package impact may be incomplete.")
		}
		if coverage.ReferencedServices > coverage.IndexedReferencedServices {
			summary.CoverageUncertainty = append(summary.CoverageUncertainty, "Not every referenced service is indexed, so downstream impact may be incomplete.")
		}
		summary.RiskLevel, summary.RiskReasons = impactRisk(summary)
		result = append(result, summary)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

type impactQueueItem struct {
	id    string
	depth int
}

func impactTarget(nodes []CanonicalFlowNodeRecord) CanonicalFlowNodeRecord {
	for _, node := range nodes {
		if node.Kind == "endpoint" {
			return node
		}
	}
	if len(nodes) > 0 {
		return nodes[len(nodes)-1]
	}
	return CanonicalFlowNodeRecord{}
}

func impactItem(node CanonicalFlowNodeRecord, edge CanonicalFlowEdgeRecord, depth int, relationship string) ImpactItemRecord {
	return ImpactItemRecord{ID: node.ID, Relationship: relationship, Kind: node.Kind, Project: node.Project, Service: node.Service, Symbol: firstNonEmpty(node.QualifiedName, node.Symbol), File: node.File, Line: node.LineStart, Depth: depth, Confidence: firstNonEmpty(edge.Confidence, node.Confidence, string(ConfidenceUnknown)), Reason: firstNonEmpty(edge.Reason, node.Reason, "Canonical feature-flow relationship."), EvidenceIDs: uniqueSortedStrings(append(append([]string(nil), edge.EvidenceIDs...), node.EvidenceIDs...))}
}

func impactRisk(summary ImpactSummaryRecord) (string, []string) {
	reasons := []string{}
	level := "low"
	if len(summary.PublicAPISurface) > 0 || len(summary.DirectConsumers) > 0 {
		level = "medium"
		reasons = append(reasons, "The target has a public API surface or direct consumers.")
	}
	if len(summary.DirectConsumers) >= 5 || len(summary.IndirectConsumers) >= 10 || len(summary.CoverageUncertainty) > 0 && len(summary.DirectConsumers) > 0 {
		level = "high"
		reasons = append(reasons, "Consumer fan-out or missing coverage increases uncertainty.")
	}
	if len(summary.DependentTests) == 0 {
		reasons = append(reasons, "No linked test was detected; this does not prove that no test exists.")
	}
	return level, reasons
}

func stringsTrimSpace(value string) string {
	return strings.TrimSpace(value)
}
