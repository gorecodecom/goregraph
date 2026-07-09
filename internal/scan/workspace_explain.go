package scan

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ExplainWorkspaceTarget(workspaceOut string, target string) (WorkspaceExplainRecord, error) {
	graph, err := readWorkspaceGraph(workspaceGraphOut(workspaceOut))
	if err != nil {
		return WorkspaceExplainRecord{}, err
	}
	matched := matchGraphNode(graph.Nodes, target)
	if matched.ID == "" {
		return WorkspaceExplainRecord{}, fmt.Errorf("no workspace graph node matches %q", target)
	}
	byID := map[string]WorkspaceGraphNodeRecord{}
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	record := WorkspaceExplainRecord{Target: target, MatchedNode: matched}
	for _, edge := range graph.Edges {
		if edge.From != matched.ID && edge.To != matched.ID {
			continue
		}
		record.Edges = append(record.Edges, edge)
		if edge.From == matched.ID {
			record.Neighbors = append(record.Neighbors, byID[edge.To])
		} else {
			record.Neighbors = append(record.Neighbors, byID[edge.From])
		}
	}
	return record, nil
}

func RenderWorkspaceExplain(record WorkspaceExplainRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Explain\n\n")
	b.WriteString(fmt.Sprintf("- Target: `%s`\n", record.Target))
	b.WriteString(fmt.Sprintf("- Node: `%s`\n", record.MatchedNode.ID))
	b.WriteString(fmt.Sprintf("- Kind: `%s`\n", record.MatchedNode.Kind))
	if label := graphLabel(record.MatchedNode); label != "" {
		b.WriteString(fmt.Sprintf("- Label: `%s`\n", label))
	}
	if record.MatchedNode.Project != "" {
		b.WriteString(fmt.Sprintf("- Project: `%s`\n", record.MatchedNode.Project))
	}
	if record.MatchedNode.File != "" {
		b.WriteString(fmt.Sprintf("- File: `%s`\n", record.MatchedNode.File))
	}
	if record.MatchedNode.Symbol != "" {
		b.WriteString(fmt.Sprintf("- Symbol: `%s`\n", record.MatchedNode.Symbol))
	}
	b.WriteString("\n## Connections\n\n")
	if len(record.Edges) == 0 {
		b.WriteString("none\n")
		return b.String()
	}
	for i, edge := range record.Edges {
		neighbor := WorkspaceGraphNodeRecord{}
		if i < len(record.Neighbors) {
			neighbor = record.Neighbors[i]
		}
		b.WriteString(fmt.Sprintf("- `%s` -> `%s` (`%s`, `%s`)\n", edge.From, edge.To, edge.Kind, edge.Confidence))
		if neighbor.ID != "" {
			b.WriteString(fmt.Sprintf("  - `%s` %s", neighbor.Kind, graphLabel(neighbor)))
			if neighbor.File != "" {
				b.WriteString(fmt.Sprintf(" in `%s`", neighbor.File))
			}
			if neighbor.Symbol != "" {
				b.WriteString(fmt.Sprintf(" symbol `%s`", neighbor.Symbol))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func workspaceOutputDirFromRoot(root string) string {
	return filepath.Join(root, ".goregraph-workspace")
}
