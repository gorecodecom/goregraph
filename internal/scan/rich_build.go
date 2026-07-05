package scan

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

func buildRichSymbols(files []FileRecord, symbols []SymbolRecord) []RichSymbolRecord {
	languageByFile := languageMap(files)
	rich := make([]RichSymbolRecord, 0, len(symbols))
	for _, symbol := range symbols {
		rich = append(rich, RichSymbolRecord{
			ID:             stableID("symbol", symbol.File, symbol.Kind, symbol.Name, fmt.Sprint(symbol.Line)),
			Name:           symbol.Name,
			Kind:           symbol.Kind,
			Language:       languageByFile[symbol.File],
			File:           symbol.File,
			Line:           symbol.Line,
			SourceLocation: sourceLocation(symbol.Line),
		})
	}
	sort.Slice(rich, func(i, j int) bool { return rich[i].ID < rich[j].ID })
	return rich
}

func buildRichRelations(files []FileRecord, relations []RelationRecord) []RichRelationRecord {
	languageByFile := languageMap(files)
	rich := make([]RichRelationRecord, 0, len(relations))
	for _, relation := range relations {
		rich = append(rich, RichRelationRecord{
			ID:              stableID("relation", relation.From, relation.Type, relation.To, fmt.Sprint(relation.Line)),
			From:            relation.From,
			To:              relation.To,
			Type:            relation.Type,
			Language:        languageByFile[relation.From],
			Line:            relation.Line,
			SourceLocation:  sourceLocation(relation.Line),
			Confidence:      "EXTRACTED",
			ConfidenceScore: 1.0,
			Internal:        isInternalRelation(relation),
		})
	}
	sort.Slice(rich, func(i, j int) bool { return rich[i].ID < rich[j].ID })
	return rich
}

func buildRichGraph(files []FileRecord, symbols []RichSymbolRecord, relations []RichRelationRecord) RichGraph {
	nodesByID := map[string]RichGraphNode{}
	for _, file := range files {
		id := stableID("file", file.Path)
		nodesByID[id] = RichGraphNode{
			ID:             id,
			Label:          file.Path,
			Kind:           "file",
			Language:       file.Language,
			SourceFile:     file.Path,
			SourceLocation: "L1",
		}
	}
	for _, symbol := range symbols {
		nodesByID[symbol.ID] = RichGraphNode{
			ID:             symbol.ID,
			Label:          symbol.Name,
			Kind:           "symbol",
			Language:       symbol.Language,
			SourceFile:     symbol.File,
			SourceLocation: symbol.SourceLocation,
		}
	}

	edges := make([]RichGraphEdge, 0, len(symbols)+len(relations))
	for _, symbol := range symbols {
		edges = append(edges, RichGraphEdge{
			ID:              stableID("edge", "contains", symbol.File, symbol.ID),
			Source:          stableID("file", symbol.File),
			Target:          symbol.ID,
			Type:            "contains",
			Relation:        "contains",
			Confidence:      "EXTRACTED",
			ConfidenceScore: 1.0,
			SourceFile:      symbol.File,
			SourceLocation:  symbol.SourceLocation,
		})
	}
	for _, relation := range relations {
		targetID := stableRelationTargetID(relation)
		if _, ok := nodesByID[targetID]; !ok {
			nodesByID[targetID] = RichGraphNode{
				ID:    targetID,
				Label: relation.To,
				Kind:  richTargetKind(relation),
			}
		}
		edges = append(edges, RichGraphEdge{
			ID:              relation.ID,
			Source:          stableID("file", relation.From),
			Target:          targetID,
			Type:            relation.Type,
			Relation:        relation.Type,
			Confidence:      relation.Confidence,
			ConfidenceScore: relation.ConfidenceScore,
			SourceFile:      relation.From,
			SourceLocation:  relation.SourceLocation,
		})
	}

	nodes := make([]RichGraphNode, 0, len(nodesByID))
	for _, node := range nodesByID {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	return RichGraph{Directed: true, Nodes: nodes, Edges: edges}
}

func languageMap(files []FileRecord) map[string]string {
	languages := map[string]string{}
	for _, file := range files {
		languages[file.Path] = file.Language
	}
	return languages
}

func stableID(parts ...string) string {
	hash := sha1.New()
	for _, part := range parts {
		hash.Write([]byte(part))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))[:24]
}

func sourceLocation(line int) string {
	if line <= 0 {
		return ""
	}
	return fmt.Sprintf("L%d", line)
}

func isInternalRelation(relation RelationRecord) bool {
	switch relation.Type {
	case "imports_internal", "tests", "includes", "sources":
		return true
	case "imports":
		return strings.Contains(relation.To, "/")
	default:
		return false
	}
}

func stableRelationTargetID(relation RichRelationRecord) string {
	if relation.Internal {
		return stableID("file", relation.To)
	}
	return stableID("external", relation.To)
}

func richTargetKind(relation RichRelationRecord) string {
	if relation.Internal {
		return "file"
	}
	return "external"
}
