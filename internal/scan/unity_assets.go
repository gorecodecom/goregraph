package scan

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

var unityDocumentHeader = regexp.MustCompile(`(?m)^--- !u!([0-9]+) &(-?[0-9]+)(?: stripped)?[ \t]*\r?$`)
var unityGUID = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

type unitySource struct {
	file FileRecord
	body string
}

func analyzeUnityAssets(files []FileRecord, sources []unitySource, csharp ProjectSymbolFacts) (AssetIndexRecord, ProjectSymbolFacts) {
	index := AssetIndexRecord{SchemaVersion: 1, Nodes: []AssetNodeRecord{}, References: []AssetReferenceRecord{}, Diagnostics: []AssetDiagnosticRecord{}}
	facts := ProjectSymbolFacts{}
	byFile := map[string]FileRecord{}
	byGUID := map[string][]string{}
	guidByFile := map[string]string{}
	for _, file := range files {
		byFile[file.Path] = file
	}
	for _, source := range sources {
		if path.Ext(source.file.Path) != ".meta" {
			continue
		}
		var meta struct {
			GUID string `yaml:"guid"`
		}
		if err := yaml.Unmarshal([]byte(source.body), &meta); err != nil {
			index.Diagnostics = append(index.Diagnostics, AssetDiagnosticRecord{File: source.file.Path, Line: 1, Code: "unity_invalid_metadata", Message: "Unity metadata is not valid YAML"})
			continue
		}
		if !unityGUID.MatchString(meta.GUID) {
			continue
		}
		target := strings.TrimSuffix(source.file.Path, ".meta")
		meta.GUID = strings.ToLower(meta.GUID)
		if _, exists := byFile[target]; exists {
			byGUID[meta.GUID] = append(byGUID[meta.GUID], target)
			guidByFile[target] = meta.GUID
		}
	}
	for _, targets := range byGUID {
		sort.Strings(targets)
		if len(targets) > 1 {
			for _, file := range targets {
				index.Diagnostics = append(index.Diagnostics, AssetDiagnosticRecord{File: file + ".meta", Line: 1, Code: "unity_duplicate_guid", Message: "GUID is assigned to multiple indexed assets"})
			}
		}
	}
	objects := map[string]AssetNodeRecord{}
	roots := map[string]AssetNodeRecord{}
	for _, file := range files {
		if file.Language != "unity" && guidByFile[file.Path] == "" {
			continue
		}
		if path.Ext(file.Path) == ".meta" {
			continue
		}
		node := AssetNodeRecord{ID: stableID("unity-asset", file.Path), File: file.Path, Line: 1, Language: "unity", Kind: "asset", Name: path.Base(file.Path), GUID: guidByFile[file.Path]}
		roots[file.Path] = node
		index.Nodes = append(index.Nodes, node)
	}
	for _, source := range sources {
		headers := unityDocumentHeader.FindAllStringSubmatchIndex(source.body, -1)
		if len(headers) == 0 {
			continue
		}
		normalized := unityDocumentHeader.ReplaceAllString(source.body, "---")
		normalized = regexp.MustCompile(`(?m)^%TAG[^\r\n]*`).ReplaceAllString(normalized, "# Unity tag directive")
		decoder := yaml.NewDecoder(strings.NewReader(normalized))
		for document := 0; ; document++ {
			var node yaml.Node
			err := decoder.Decode(&node)
			if err == io.EOF {
				break
			}
			if err != nil {
				index.Diagnostics = append(index.Diagnostics, AssetDiagnosticRecord{File: source.file.Path, Line: 1, Code: "unity_invalid_asset", Message: "Unity asset contains unsupported or malformed serialized YAML"})
				break
			}
			if document >= len(headers) || len(node.Content) == 0 {
				continue
			}
			header := headers[document]
			localID := source.body[header[4]:header[5]]
			root := node.Content[0]
			if root.Kind != yaml.MappingNode || len(root.Content) < 2 {
				continue
			}
			kind := root.Content[0].Value
			values := root.Content[1]
			name := unityScalar(values, "m_Name")
			if name == "" {
				name = kind + " " + localID
			}
			object := AssetNodeRecord{ID: stableID("unity-object", source.file.Path, localID), File: source.file.Path, Line: root.Line, Language: "unity", Kind: kind, Name: name, LocalID: localID, GUID: guidByFile[source.file.Path], Properties: map[string]any{}}
			for _, key := range []string{"m_IsActive", "m_Enabled", "m_Layer", "m_TagString", "m_LoopTime", "m_Speed", "m_IsTrigger", "m_Weight", "m_ApplyRootMotion"} {
				if value := unityScalar(values, key); value != "" {
					object.Properties[key] = value
				}
			}
			objects[source.file.Path+"#"+localID] = object
			index.Nodes = append(index.Nodes, object)
			index.References = append(index.References, AssetReferenceRecord{From: roots[source.file.Path].ID, To: object.ID, File: source.file.Path, Line: root.Line, Property: "contains", Resolution: SymbolResolutionExact, Reason: "serialized object belongs to asset document"})
			unityReferences(values, "", object, &index.References)
		}
	}
	for i := range index.References {
		ref := &index.References[i]
		if ref.To != "" {
			continue
		}
		ref.Resolution = SymbolResolutionUnresolved
		targetFile := ref.File
		if ref.GUID != "" {
			targets := byGUID[strings.ToLower(ref.GUID)]
			if len(targets) > 1 {
				ref.Resolution = SymbolResolutionAmbiguous
				for _, target := range targets {
					ref.Candidates = append(ref.Candidates, roots[target].ID)
				}
				ref.Reason = "duplicate asset GUID"
				continue
			}
			if len(targets) == 0 {
				ref.Reason = "asset GUID is outside the indexed inventory or supplied by an external package"
				continue
			}
			targetFile = targets[0]
		}
		if object, ok := objects[targetFile+"#"+ref.LocalID]; ok {
			ref.To = object.ID
			ref.Resolution = SymbolResolutionExact
			ref.Reason = "serialized GUID and fileID identify an indexed object"
			continue
		}
		if ref.GUID != "" {
			if root, ok := roots[targetFile]; ok {
				ref.To = root.ID
				ref.Resolution = SymbolResolutionExact
				ref.Reason = "GUID identifies indexed asset; internal object is not decoded"
				continue
			}
		}
		ref.Reason = "local fileID does not identify a serialized object in this asset"
		index.Diagnostics = append(index.Diagnostics, AssetDiagnosticRecord{File: ref.File, Line: ref.Line, Code: "unity_missing_local_reference", Message: ref.Reason})
	}
	for _, node := range index.Nodes {
		qualified := node.File + "#" + node.LocalID
		if node.LocalID == "" {
			qualified = node.File
		}
		facts.Declarations = append(facts.Declarations, RichSymbolRecord{ID: node.ID, Name: node.Name, Kind: "asset", Language: "unity", File: node.File, Line: node.Line, QualifiedName: qualified, SourceLocation: sourceLocation(node.Line), Analyzer: "unity-serialized", Confidence: ConfidenceExact, Coverage: CoveragePartial, Limitations: []string{"serialized asset data only; no Unity runtime or rendered geometry evaluation"}})
	}
	nodesByID := map[string]AssetNodeRecord{}
	for _, node := range index.Nodes {
		nodesByID[node.ID] = node
	}
	for _, ref := range index.References {
		relation := RichRelationRecord{ID: stableID("unity-reference", ref.From, ref.Property, ref.LocalID, ref.GUID, fmt.Sprint(ref.Line)), From: ref.File, To: ref.GUID + "#" + ref.LocalID, Type: "uses_asset", Language: "unity", Analyzer: "unity-serialized", Line: ref.Line, SourceLocation: sourceLocation(ref.Line), FromSymbolID: ref.From, ToSymbolID: ref.To, Resolution: ref.Resolution, NonPromotable: ref.Resolution != SymbolResolutionExact, Confidence: "INFERRED", Reason: ref.Reason, CandidateSymbolIDs: ref.Candidates, TargetQualifiedName: ref.GUID + "#" + ref.LocalID}
		if node, ok := nodesByID[ref.To]; ok {
			relation.To = node.File
			relation.TargetQualifiedName = node.File
			if node.LocalID != "" {
				relation.TargetQualifiedName += "#" + node.LocalID
			}
		}
		if ref.Resolution == SymbolResolutionExact {
			relation.Internal = true
			relation.Confidence = "EXTRACTED"
			relation.ConfidenceScore = 1
		}
		facts.References = append(facts.References, relation)
		if strings.HasSuffix(ref.Property, "m_Script") && ref.Resolution == SymbolResolutionExact {
			for _, symbol := range csharp.Declarations {
				if symbol.File == relation.To && symbol.Kind == "class" && symbol.Name == strings.TrimSuffix(path.Base(symbol.File), ".cs") {
					script := relation
					script.ID = stableID("unity-script", ref.From, symbol.ID)
					script.ToSymbolID = symbol.ID
					script.TargetQualifiedName = symbol.QualifiedName
					script.Type = "uses_script"
					script.Reason = "Unity m_Script GUID identifies the C# MonoScript asset and matching class declaration"
					facts.References = append(facts.References, script)
				}
			}
		}
	}
	sort.Slice(index.Nodes, func(i, j int) bool { return index.Nodes[i].ID < index.Nodes[j].ID })
	return index, facts
}

func unityScalar(node *yaml.Node, key string) string {
	if node.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key && node.Content[i+1].Kind == yaml.ScalarNode {
			return node.Content[i+1].Value
		}
	}
	return ""
}
func unityReferences(node *yaml.Node, prefix string, from AssetNodeRecord, result *[]AssetReferenceRecord) {
	if node.Kind == yaml.AliasNode {
		return
	}
	if node.Kind == yaml.MappingNode {
		fileID := unityScalar(node, "fileID")
		guid := unityScalar(node, "guid")
		if fileID != "" {
			if fileID == "0" || guid == "00000000000000000000000000000000" {
				return
			}
			*result = append(*result, AssetReferenceRecord{From: from.ID, File: from.File, Line: node.Line, Property: prefix, GUID: guid, LocalID: fileID})
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if prefix != "" {
				key = prefix + "." + key
			}
			unityReferences(node.Content[i+1], key, from, result)
		}
	} else {
		for i, child := range node.Content {
			unityReferences(child, fmt.Sprintf("%s[%d]", prefix, i), from, result)
		}
	}
}
