package scan

import (
	"sort"
	"strings"
)

func buildGraph(files []FileRecord, symbols []SymbolRecord, relations []RelationRecord) Graph {
	fileIDs := map[string]bool{}
	nodeIDs := map[string]bool{}
	nodes := make([]GraphNode, 0, len(files)+len(symbols))
	for _, file := range files {
		id := "file:" + file.Path
		fileIDs[file.Path] = true
		nodeIDs[id] = true
		nodes = append(nodes, GraphNode{ID: id, Label: file.Path, Type: "file", File: file.Path})
	}
	for _, symbol := range symbols {
		id := "symbol:" + symbol.File + ":" + symbol.Kind + ":" + symbol.Name
		nodeIDs[id] = true
		nodes = append(nodes, GraphNode{
			ID:    id,
			Label: symbol.Name,
			Type:  symbol.Kind,
			File:  symbol.File,
			Line:  symbol.Line,
		})
	}

	edges := make([]GraphEdge, 0, len(symbols)+len(relations))
	for _, symbol := range symbols {
		edges = append(edges, GraphEdge{From: "file:" + symbol.File, To: "symbol:" + symbol.File + ":" + symbol.Kind + ":" + symbol.Name, Type: "contains"})
	}
	for _, relation := range relations {
		target := graphRelationTarget(relation.To, fileIDs)
		if !nodeIDs[target.id] {
			nodeIDs[target.id] = true
			nodes = append(nodes, GraphNode{ID: target.id, Label: target.label, Type: target.kind})
		}
		edges = append(edges, GraphEdge{From: "file:" + relation.From, To: target.id, Type: relation.Type})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
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

type graphTarget struct {
	id    string
	label string
	kind  string
}

func graphRelationTarget(to string, fileIDs map[string]bool) graphTarget {
	if fileIDs[to] {
		return graphTarget{id: "file:" + to, label: to, kind: "file"}
	}
	return graphTarget{id: "dependency:" + to, label: to, kind: "dependency"}
}

func resolveLocalImportRelations(index *Index) {
	module := modulePath(index.Symbols)
	if module == "" {
		return
	}
	packages := goPackageFiles(index.Files)
	for i := range index.Relations {
		relation := &index.Relations[i]
		if relation.Type != "imports" || !strings.HasPrefix(relation.To, module+"/") {
			continue
		}
		dir := strings.TrimPrefix(relation.To, module+"/")
		if target, ok := packages[dir]; ok {
			relation.To = target
		}
	}
}

func modulePath(symbols []SymbolRecord) string {
	for _, symbol := range symbols {
		if symbol.Kind == "module" && strings.HasPrefix(symbol.Name, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(symbol.Name, "module "))
		}
	}
	return ""
}

func goPackageFiles(files []FileRecord) map[string]string {
	byDir := map[string]string{}
	for _, file := range files {
		if file.Language != "go" || strings.HasSuffix(file.Path, "_test.go") || file.Path == "go.mod" {
			continue
		}
		dir := strings.TrimSuffix(file.Path, "/"+fileBase(file.Path))
		if dir == file.Path {
			dir = "."
		}
		existing, ok := byDir[dir]
		if !ok || file.Path < existing {
			byDir[dir] = file.Path
		}
	}
	return byDir
}

func fileBase(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
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
