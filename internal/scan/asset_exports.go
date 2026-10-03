package scan

import (
	"bytes"
	"encoding/json"
	"path"
	"sort"
	"strconv"
	"strings"
)

type assetExportRecord struct {
	SchemaVersion   int    `json:"schema_version"`
	Engine          string `json:"engine"`
	ProducerVersion string `json:"producer_version"`
	Source          string `json:"source"`
	SourceSHA256    string `json:"source_sha256"`
	Objects         []struct {
		ID         string         `json:"id"`
		Name       string         `json:"name"`
		Kind       string         `json:"kind"`
		Properties map[string]any `json:"properties"`
		References []struct {
			Property string `json:"property"`
			Target   string `json:"target"`
		} `json:"references"`
	} `json:"objects"`
	Samples []struct {
		Object             string      `json:"object"`
		Frame              int         `json:"frame"`
		Vertices           int         `json:"vertices"`
		Triangles          int         `json:"triangles"`
		DegeneratePolygons int         `json:"degenerate_polygons"`
		BoundsMin          []float64   `json:"bounds_min"`
		BoundsMax          []float64   `json:"bounds_max"`
		BoneHead           []float64   `json:"bone_head,omitempty"`
		BoneTail           []float64   `json:"bone_tail,omitempty"`
		BoneMatrix         [][]float64 `json:"bone_matrix,omitempty"`
	} `json:"samples"`
	Limitations []string `json:"limitations"`
	Truncated   bool     `json:"truncated"`
}
type assetExportSource struct {
	file FileRecord
	body string
}

func analyzeAssetExports(files []FileRecord, sources []assetExportSource) (AssetIndexRecord, ProjectSymbolFacts) {
	index := AssetIndexRecord{SchemaVersion: 1, Nodes: []AssetNodeRecord{}, References: []AssetReferenceRecord{}, Diagnostics: []AssetDiagnosticRecord{}}
	facts := ProjectSymbolFacts{}
	byFile := map[string]FileRecord{}
	verified := map[string]bool{}
	for _, file := range files {
		byFile[file.Path] = file
	}
	for _, source := range sources {
		diagnostic := func(code, message string) {
			index.Diagnostics = append(index.Diagnostics, AssetDiagnosticRecord{File: source.file.Path, Line: 1, Code: code, Message: message})
		}
		var report assetExportRecord
		if err := json.Unmarshal([]byte(source.body), &report); err != nil || report.SchemaVersion != 1 || (report.Engine != "blender" && report.Engine != "unity") || report.ProducerVersion == "" {
			diagnostic("asset_invalid_export", "Export schema, engine or producer metadata is invalid")
			continue
		}
		expectedEngine := "unity"
		if strings.HasSuffix(source.file.Path, ".goregraph-blender.json") {
			expectedEngine = "blender"
		}
		if report.Engine != expectedEngine {
			diagnostic("asset_invalid_export", "Export engine does not match its filename")
			continue
		}
		identity := path.Clean(strings.ReplaceAll(report.Source, "\\", "/"))
		if identity == "." || identity == ".." || strings.HasPrefix(identity, "../") || path.IsAbs(identity) || strings.Contains(identity, ":") {
			diagnostic("asset_export_outside_project", "Export source must be a project-relative indexed file")
			continue
		}
		original, ok := byFile[identity]
		if !ok || original.Hash != report.SourceSHA256 || report.SourceSHA256 == "" {
			diagnostic("asset_export_stale", "Export source is missing from the inventory or its SHA-256 has changed; re-export explicitly")
			continue
		}
		if len(report.Objects) > 100000 || len(report.Samples) > 524288 {
			diagnostic("asset_export_limit", "Export exceeds the supported object or frame sample limit")
			continue
		}
		ids := map[string]AssetNodeRecord{}
		duplicate := false
		for _, object := range report.Objects {
			if object.ID == "" {
				duplicate = true
				break
			}
			if _, exists := ids[object.ID]; exists {
				duplicate = true
				break
			}
			encoded, _ := json.Marshal(object.ID)
			offset := strings.Index(source.body, string(encoded))
			line := 1
			if offset >= 0 {
				line += strings.Count(source.body[:offset], "\n")
			}
			node := AssetNodeRecord{ID: stableID("asset-export-object", source.file.Path, object.ID), File: source.file.Path, Line: line, Language: report.Engine, Kind: object.Kind, Name: object.Name, LocalID: object.ID, Properties: safeExportProperties(object.Properties)}
			if node.Properties == nil {
				node.Properties = map[string]any{}
			}
			node.Properties["source_asset"] = identity
			node.Properties["producer_version"] = report.ProducerVersion
			node.Properties["limitations"] = report.Limitations
			node.Properties["truncated"] = report.Truncated
			ids[object.ID] = node
		}
		if duplicate {
			diagnostic("asset_export_duplicate_id", "Export object identities are empty or duplicated")
			continue
		}
		verified[identity] = true
		sampleLines := assetExportSampleLines(source.body)
		for ordinal, sample := range report.Samples {
			if node, ok := ids[sample.Object]; ok {
				existing, _ := node.Properties["frame_samples"].([]any)
				node.Properties["frame_samples"] = append(existing, sample)
				if ordinal < len(sampleLines) {
					id := stableID("asset-export-sample", node.ID, strconv.Itoa(ordinal))
					qualified := source.file.Path + "#sample:" + strconv.Itoa(ordinal)
					line := sampleLines[ordinal]
					facts.Declarations = append(facts.Declarations, RichSymbolRecord{ID: id, Name: node.Name + " frame " + strconv.Itoa(sample.Frame), Kind: "asset", Language: report.Engine, File: source.file.Path, Line: line, QualifiedName: qualified, SourceLocation: sourceLocation(line), Analyzer: report.Engine + "-export", Confidence: ConfidenceExact, Coverage: CoveragePartial, Limitations: []string{"explicit exported frame sample only; no exhaustive runtime or collision proof"}})
				}
			}
		}
		for _, object := range report.Objects {
			node := ids[object.ID]
			index.Nodes = append(index.Nodes, node)
			qualified := source.file.Path + "#" + object.ID
			facts.Declarations = append(facts.Declarations, RichSymbolRecord{ID: node.ID, Name: node.Name, Kind: "asset", Language: report.Engine, File: source.file.Path, Line: node.Line, QualifiedName: qualified, SourceLocation: sourceLocation(node.Line), Analyzer: report.Engine + "-export", Confidence: ConfidenceExact, Coverage: CoveragePartial, Limitations: append([]string{"SHA-256 verifies source identity; exporter values are external evidence, not independent runtime proof"}, report.Limitations...)})
			for _, link := range object.References {
				target, found := ids[link.Target]
				ref := AssetReferenceRecord{From: node.ID, File: source.file.Path, Line: node.Line, Property: link.Property, LocalID: link.Target, Resolution: SymbolResolutionUnresolved, Reason: "external or omitted exported object"}
				if found {
					ref.To = target.ID
					ref.Resolution = SymbolResolutionExact
					ref.Reason = "exported object identities match in source-hash-verified report"
				}
				index.References = append(index.References, ref)
				relation := RichRelationRecord{ID: stableID("asset-export-ref", node.ID, link.Property, link.Target), From: source.file.Path, To: source.file.Path, Type: "uses_asset", Language: report.Engine, Analyzer: report.Engine + "-export", FromSymbolID: node.ID, ToSymbolID: ref.To, TargetQualifiedName: source.file.Path + "#" + link.Target, Line: node.Line, SourceLocation: sourceLocation(node.Line), Resolution: ref.Resolution, NonPromotable: !found, Reason: ref.Reason, Confidence: "INFERRED"}
				if found {
					relation.Internal = true
					relation.Confidence = "EXTRACTED"
					relation.ConfidenceScore = 1
				}
				facts.References = append(facts.References, relation)
			}
		}
	}
	for _, file := range files {
		if file.Language == "blender" && strings.HasSuffix(file.Path, ".blend") && !verified[file.Path] {
			index.Diagnostics = append(index.Diagnostics, AssetDiagnosticRecord{File: file.Path, Line: 1, Code: "blender_export_required", Message: "Binary blend is inventoried and hashed; object and geometry context requires an explicit current export"})
		}
	}
	sort.Slice(index.Nodes, func(i, j int) bool { return index.Nodes[i].ID < index.Nodes[j].ID })
	return index, facts
}

func assetExportSampleLines(body string) []int {
	var envelope struct {
		Samples json.RawMessage `json:"samples"`
	}
	if json.Unmarshal([]byte(body), &envelope) != nil || len(envelope.Samples) == 0 {
		return nil
	}
	base := strings.Index(body, string(envelope.Samples))
	decoder := json.NewDecoder(bytes.NewReader(envelope.Samples))
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') || base < 0 {
		return nil
	}
	var lines []int
	for decoder.More() {
		var sample json.RawMessage
		if decoder.Decode(&sample) != nil {
			return nil
		}
		offset := base + int(decoder.InputOffset()) - len(sample)
		lines = append(lines, 1+strings.Count(body[:offset], "\n"))
	}
	return lines
}

func mergeAssetIndex(target *AssetIndexRecord, extra AssetIndexRecord) {
	target.Nodes = append(target.Nodes, extra.Nodes...)
	target.References = append(target.References, extra.References...)
	target.Diagnostics = append(target.Diagnostics, extra.Diagnostics...)
	sort.Slice(target.Nodes, func(i, j int) bool { return target.Nodes[i].ID < target.Nodes[j].ID })
	sort.Slice(target.References, func(i, j int) bool {
		a, b := target.References[i], target.References[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Property != b.Property {
			return a.Property < b.Property
		}
		return a.To < b.To
	})
	sort.Slice(target.Diagnostics, func(i, j int) bool {
		a, b := target.Diagnostics[i], target.Diagnostics[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Code < b.Code
	})
}

func enrichAssetContext(contextIndex *AgentContextIndexRecord, assets AssetIndexRecord) {
	nodes := map[string]AssetNodeRecord{}
	for _, node := range assets.Nodes {
		nodes[node.File+"#"+node.LocalID] = node
	}
	for i := range contextIndex.Facts {
		fact := &contextIndex.Facts[i]
		node, ok := nodes[fact.Qualified]
		if !ok {
			continue
		}
		body, _ := json.Marshal(node.Properties)
		summary := node.Kind + ": " + string(body)
		runes := []rune(summary)
		if len(runes) > 2048 {
			summary = string(runes[:2048]) + "..."
		}
		fact.Summary = summary
		fact.Search += " " + summary
	}
}

func safeExportProperties(properties map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range strings.Fields("bone_head bone_tail deform channels location scale rotation_mode hidden_render modifiers drivers vertices polygons shape_keys bones use_nodes node_types frame_range slots hierarchy active components position indices blend_shapes animations animation_length bounds_min bounds_max") {
		if value, exists := properties[key]; exists {
			result[key] = value
		}
	}
	return result
}
