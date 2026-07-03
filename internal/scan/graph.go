package scan

import (
	"sort"
	"strings"
)

func buildGraph(files []FileRecord, symbols []SymbolRecord, relations []RelationRecord) Graph {
	nodes := make([]GraphNode, 0, len(files)+len(symbols))
	for _, file := range files {
		nodes = append(nodes, GraphNode{ID: "file:" + file.Path, Label: file.Path, Type: "file", File: file.Path})
	}
	for _, symbol := range symbols {
		nodes = append(nodes, GraphNode{
			ID:    "symbol:" + symbol.File + ":" + symbol.Kind + ":" + symbol.Name,
			Label: symbol.Name,
			Type:  symbol.Kind,
			File:  symbol.File,
			Line:  symbol.Line,
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	edges := make([]GraphEdge, 0, len(symbols)+len(relations))
	for _, symbol := range symbols {
		edges = append(edges, GraphEdge{From: "file:" + symbol.File, To: "symbol:" + symbol.File + ":" + symbol.Kind + ":" + symbol.Name, Type: "contains"})
	}
	for _, relation := range relations {
		edges = append(edges, GraphEdge{From: "file:" + relation.From, To: relation.To, Type: relation.Type})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Type < edges[j].Type
	})
	return Graph{Nodes: nodes, Edges: edges}
}

func buildTestRelations(files []FileRecord) []RelationRecord {
	fileSet := map[string]bool{}
	for _, file := range files {
		fileSet[file.Path] = true
	}
	var relations []RelationRecord
	for _, file := range files {
		source := sourceForTestFile(file.Path)
		if source == "" || !fileSet[source] {
			continue
		}
		relations = append(relations, RelationRecord{From: file.Path, To: source, Type: "tests", Line: 1})
	}
	return relations
}

func sourceForTestFile(path string) string {
	switch {
	case strings.HasSuffix(path, "_test.go"):
		return strings.TrimSuffix(path, "_test.go") + ".go"
	case strings.HasSuffix(path, ".test.ts"):
		return strings.TrimSuffix(path, ".test.ts") + ".ts"
	case strings.HasSuffix(path, ".spec.ts"):
		return strings.TrimSuffix(path, ".spec.ts") + ".ts"
	case strings.HasSuffix(path, ".test.js"):
		return strings.TrimSuffix(path, ".test.js") + ".js"
	case strings.HasSuffix(path, ".spec.js"):
		return strings.TrimSuffix(path, ".spec.js") + ".js"
	default:
		return ""
	}
}
