package scan

import (
	"fmt"
	"strings"
)

func WorkspacePath(workspaceOut, from, to string) ([]WorkspaceGraphNodeRecord, []WorkspaceGraphEdgeRecord, error) {
	graph, err := readWorkspaceGraph(workspaceGraphOut(workspaceOut))
	if err != nil {
		return nil, nil, err
	}
	start := matchGraphNode(graph.Nodes, from)
	end := matchGraphNode(graph.Nodes, to)
	if start.ID == "" || end.ID == "" {
		return nil, nil, fmt.Errorf("path endpoints not found")
	}
	neighbors := map[string][]WorkspaceGraphEdgeRecord{}
	for _, edge := range graph.Edges {
		neighbors[edge.From] = append(neighbors[edge.From], edge)
		neighbors[edge.To] = append(neighbors[edge.To], WorkspaceGraphEdgeRecord{
			ID: edge.ID, From: edge.To, To: edge.From, Kind: edge.Kind, Confidence: edge.Confidence, Meta: edge.Meta,
		})
	}
	queue := []string{start.ID}
	seen := map[string]bool{start.ID: true}
	prevNode := map[string]string{}
	prevEdge := map[string]WorkspaceGraphEdgeRecord{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == end.ID {
			break
		}
		for _, edge := range neighbors[current] {
			if seen[edge.To] {
				continue
			}
			seen[edge.To] = true
			prevNode[edge.To] = current
			prevEdge[edge.To] = edge
			queue = append(queue, edge.To)
		}
	}
	if !seen[end.ID] {
		return nil, nil, fmt.Errorf("no path from %q to %q", from, to)
	}
	byID := map[string]WorkspaceGraphNodeRecord{}
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	var ids []string
	var edges []WorkspaceGraphEdgeRecord
	for current := end.ID; current != ""; current = prevNode[current] {
		ids = append(ids, current)
		if current == start.ID {
			break
		}
		edges = append(edges, prevEdge[current])
	}
	nodes := make([]WorkspaceGraphNodeRecord, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		nodes = append(nodes, byID[ids[i]])
	}
	for i, j := 0, len(edges)-1; i < j; i, j = i+1, j-1 {
		edges[i], edges[j] = edges[j], edges[i]
	}
	return nodes, edges, nil
}

func RenderWorkspacePath(nodes []WorkspaceGraphNodeRecord, edges []WorkspaceGraphEdgeRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Path\n\n")
	if len(nodes) == 0 {
		b.WriteString("No path found.\n")
		return b.String()
	}
	for i, node := range nodes {
		if i > 0 && i-1 < len(edges) {
			b.WriteString(fmt.Sprintf("   via `%s`\n", edges[i-1].Kind))
		}
		b.WriteString(fmt.Sprintf("%d. `%s` %s\n", i+1, node.Kind, graphLabel(node)))
	}
	return b.String()
}
