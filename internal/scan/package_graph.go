package scan

import (
	"fmt"
	"sort"
	"strings"
)

func buildPackageGraph(workspace WorkspaceIndex) PackageGraphRecord {
	packageByName := map[string]NodePackageRecord{}
	for _, pkg := range workspace.NodePackages {
		if pkg.Name != "" {
			packageByName[pkg.Name] = pkg
		}
	}

	nodes := make([]PackageNodeRecord, 0, len(workspace.NodePackages))
	for _, pkg := range workspace.NodePackages {
		kind := "package"
		if len(pkg.Workspaces) > 0 {
			kind = "workspace-root"
		}
		nodes = append(nodes, PackageNodeRecord{
			Name:             emptyAsNone(pkg.Name),
			Path:             pkg.Path,
			Kind:             kind,
			PackageManager:   pkg.PackageManager,
			Scripts:          pkg.Scripts,
			Exports:          clonePackageExports(pkg.Exports),
			ExportConditions: clonePackageExportConditions(pkg.ExportConditions),
			Types:            pkg.Types,
		})
	}

	var edges []PackageEdgeRecord
	for _, pkg := range workspace.NodePackages {
		if pkg.Name == "" {
			continue
		}
		for _, dep := range pkg.Dependencies {
			target, internal := packageByName[dep]
			edge := PackageEdgeRecord{
				From:            pkg.Name,
				To:              dep,
				Type:            "depends_on",
				FromPath:        pkg.Path,
				Confidence:      "EXTRACTED",
				ConfidenceScore: 1.0,
				Reason:          "package-json-dependency",
			}
			if internal {
				edge.ToPath = target.Path
				edge.Reason = "workspace-package-json-dependency"
			}
			edges = append(edges, edge)
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Name != nodes[j].Name {
			return nodes[i].Name < nodes[j].Name
		}
		return nodes[i].Path < nodes[j].Path
	})
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	return PackageGraphRecord{Nodes: nodes, Edges: edges}
}

func clonePackageExports(exports map[string][]string) map[string][]string {
	if len(exports) == 0 {
		return nil
	}
	result := make(map[string][]string, len(exports))
	for specifier, targets := range exports {
		result[specifier] = append([]string(nil), targets...)
	}
	return result
}

func clonePackageExportConditions(conditions map[string]map[string][]string) map[string]map[string][]string {
	if len(conditions) == 0 {
		return nil
	}
	result := make(map[string]map[string][]string, len(conditions))
	for specifier, branches := range conditions {
		result[specifier] = clonePackageExports(branches)
	}
	return result
}

func renderPackageGraphReport(graph PackageGraphRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Package Graph\n\n")
	if len(graph.Nodes) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	b.WriteString("## Packages\n\n")
	for _, node := range graph.Nodes {
		b.WriteString(fmt.Sprintf("- `%s` (%s) - `%s`\n", node.Name, node.Kind, node.Path))
	}
	if len(graph.Edges) == 0 {
		b.WriteString("\n## Dependencies\n\n- none detected\n")
		return b.String()
	}
	b.WriteString("\n## Dependencies\n\n")
	for _, edge := range graph.Edges {
		location := ""
		if edge.ToPath != "" {
			location = fmt.Sprintf(" -> `%s`", edge.ToPath)
		}
		b.WriteString(fmt.Sprintf("- `%s` -> `%s`%s (%s)\n", edge.From, edge.To, location, edge.Reason))
	}
	return b.String()
}
