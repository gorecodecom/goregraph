package scan

import (
	"fmt"
	"sort"
	"strings"
)

func buildMavenGraph(workspace WorkspaceIndex) MavenGraphRecord {
	nodesByID := map[string]MavenNodeRecord{}
	var edges []MavenEdgeRecord
	for _, pkg := range workspace.MavenPackages {
		fromID := mavenPackageID(pkg.GroupID, pkg.ArtifactID)
		if fromID == "" {
			continue
		}
		nodesByID[fromID] = MavenNodeRecord{
			ID:       fromID,
			GroupID:  pkg.GroupID,
			Artifact: pkg.ArtifactID,
			Version:  pkg.Version,
			Kind:     "module",
			Path:     pkg.Path,
			Parent:   pkg.Parent,
		}
		for _, dep := range pkg.Dependencies {
			toID := mavenPackageID(dep.GroupID, dep.ArtifactID)
			if toID == "" {
				continue
			}
			if _, ok := nodesByID[toID]; !ok {
				nodesByID[toID] = MavenNodeRecord{
					ID:       toID,
					GroupID:  dep.GroupID,
					Artifact: dep.ArtifactID,
					Version:  dep.Version,
					Kind:     "dependency",
					Scope:    dep.Scope,
				}
			}
			edges = append(edges, MavenEdgeRecord{
				From:            fromID,
				To:              toID,
				Type:            "depends_on",
				Scope:           dep.Scope,
				FromPath:        pkg.Path,
				Confidence:      "EXTRACTED",
				ConfidenceScore: 1.0,
				Reason:          "pom-dependency",
			})
		}
	}

	nodes := make([]MavenNodeRecord, 0, len(nodesByID))
	for _, node := range nodesByID {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind < nodes[j].Kind
		}
		return nodes[i].ID < nodes[j].ID
	})
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	return MavenGraphRecord{Nodes: nodes, Edges: edges}
}

func mavenPackageID(groupID, artifactID string) string {
	groupID = strings.TrimSpace(groupID)
	artifactID = strings.TrimSpace(artifactID)
	if groupID == "" || artifactID == "" {
		return ""
	}
	return groupID + ":" + artifactID
}

func renderMavenGraphReport(graph MavenGraphRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Maven Graph\n\n")
	if len(graph.Nodes) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	b.WriteString("## Modules And Dependencies\n\n")
	for _, node := range graph.Nodes {
		version := ""
		if node.Version != "" {
			version = fmt.Sprintf(" `%s`", node.Version)
		}
		location := ""
		if node.Path != "" {
			location = fmt.Sprintf(" - `%s`", node.Path)
		}
		b.WriteString(fmt.Sprintf("- `%s` (%s)%s%s\n", node.ID, node.Kind, version, location))
	}
	if len(graph.Edges) == 0 {
		b.WriteString("\n## Dependency Edges\n\n- none detected\n")
		return b.String()
	}
	b.WriteString("\n## Dependency Edges\n\n")
	for _, edge := range graph.Edges {
		scope := emptyAsNone(edge.Scope)
		b.WriteString(fmt.Sprintf("- `%s` -> `%s` (scope %s, %s)\n", edge.From, edge.To, scope, edge.Reason))
	}
	return b.String()
}
