package scan

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Serialized callbacks describe saved event wiring, not observed method execution.
func linkUnitySerializedCode(index *AssetIndexRecord, facts *ProjectSymbolFacts, serialized map[string]*yaml.Node, csharp ProjectSymbolFacts, codeSources ...[]csharpSource) {
	scripts := map[string][]RichSymbolRecord{}
	declarations := map[string]RichSymbolRecord{}
	types := map[string][]RichSymbolRecord{}
	members := map[string][]csharpMember{}
	sourceByFile := map[string]csharpSource{}
	if len(codeSources) == 1 {
		for _, source := range codeSources[0] {
			sourceByFile[source.file] = source
			for _, typ := range source.types {
				types[typ.symbol.QualifiedName] = append(types[typ.symbol.QualifiedName], typ.symbol)
			}
			for _, member := range source.members {
				members[member.owner+"."+member.symbol.Name] = append(members[member.owner+"."+member.symbol.Name], member)
			}
		}
	}
	for _, declaration := range csharp.Declarations {
		declarations[declaration.ID] = declaration
	}
	for _, relation := range facts.References {
		if relation.Type == "uses_script" && relation.Resolution == SymbolResolutionExact {
			scripts[relation.FromSymbolID] = append(scripts[relation.FromSymbolID], declarations[relation.ToSymbolID])
		}
	}
	prototypes := map[string][]string{}
	for _, reference := range index.References {
		if reference.Resolution == SymbolResolutionExact && reference.Property == "m_CorrespondingSourceObject" {
			prototypes[reference.From] = append(prototypes[reference.From], reference.To)
		}
	}
	var inheritedScript func(string, map[string]bool) []RichSymbolRecord
	inheritedScript = func(id string, seen map[string]bool) []RichSymbolRecord {
		if len(scripts[id]) > 0 {
			return scripts[id]
		}
		if seen[id] || len(seen) >= 64 || len(prototypes[id]) != 1 {
			return nil
		}
		seen[id] = true
		return inheritedScript(prototypes[id][0], seen)
	}
	for _, object := range index.Nodes {
		if len(scripts[object.ID]) != 0 {
			continue
		}
		owners := inheritedScript(object.ID, map[string]bool{})
		if len(owners) == 1 {
			scripts[object.ID] = owners
			owner := owners[0]
			facts.References = append(facts.References, RichRelationRecord{ID: stableID("unity-inherited-script", object.ID, owner.ID), From: object.File, To: owner.File, Type: "uses_script", Language: "unity", Analyzer: "unity-serialized", Line: object.Line, SourceLocation: sourceLocation(object.Line), FromSymbolID: object.ID, ToSymbolID: owner.ID, TargetQualifiedName: owner.QualifiedName, Internal: true, Resolution: SymbolResolutionExact, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "unique serialized prefab correspondence identifies the m_Script GUID and source class; runtime activation is not evaluated"})
		}
	}
	targets := map[string]string{}
	for _, reference := range index.References {
		if reference.Resolution == SymbolResolutionExact {
			targets[fmt.Sprint(reference.From, "/", reference.Property)] = reference.To
		}
	}
	for _, object := range index.Nodes {
		values := serialized[object.ID]
		if values == nil {
			continue
		}
		if owners := scripts[object.ID]; len(owners) == 1 && len(owners[0].Limitations) == 1 {
			for i := 0; i+1 < len(values.Content); i += 2 {
				key := values.Content[i]
				fields := csharpInheritedMembers(sourceByFile[owners[0].File], owners[0].QualifiedName, key.Value, members, codeSourcesOrEmpty(codeSources), types, map[string]bool{})
				for _, member := range fields {
					field := member.symbol
					if field.Kind != "field" && field.Kind != "property" || len(fields) != 1 || len(field.Limitations) != 1 {
						continue
					}
					facts.References = append(facts.References, RichRelationRecord{ID: stableID("unity-field", object.ID, field.ID), From: object.File, To: field.File, Type: "serialized_field", Language: "unity", Analyzer: "unity-serialized", Line: key.Line, SourceLocation: sourceLocation(key.Line), FromSymbolID: object.ID, ToSymbolID: field.ID, TargetQualifiedName: field.QualifiedName, Internal: true, Resolution: SymbolResolutionExact, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "serialized property name matches the script declaration; serialization lifecycle is not executed"})
				}
			}
		}
		var visit func(*yaml.Node, string)
		visit = func(node *yaml.Node, prefix string) {
			if node.Kind == yaml.AliasNode {
				return
			}
			if node.Kind == yaml.MappingNode {
				name := unityScalar(node, "m_MethodName")
				if name != "" && strings.Contains(prefix, "m_PersistentCalls.m_Calls[") {
					target := targets[object.ID+"/"+prefix+".m_Target"]
					ref := RichRelationRecord{ID: stableID("unity-callback", object.ID, prefix, name), From: object.File, To: name, Type: "persistent_callback", Language: "unity", Analyzer: "unity-serialized", Line: node.Line, SourceLocation: sourceLocation(node.Line), FromSymbolID: object.ID, TargetQualifiedName: name, Resolution: SymbolResolutionUnresolved, NonPromotable: true, Confidence: "INFERRED", Reason: "saved UnityEvent callback; target script or signature is not uniquely verified"}
					mode := unityScalar(node, "m_Mode")
					expected := map[string]string{"1": "", "3": "int", "4": "float", "5": "string", "6": "bool"}
					parameter, knownMode := expected[mode]
					if mode == "0" && len(scripts[object.ID]) == 1 && len(scripts[object.ID][0].Limitations) == 1 && len(codeSources) == 1 {
						parameter, knownMode = unityEventParameters(codeSources[0], scripts[object.ID][0], strings.Split(prefix, ".")[0], members, types)
					}
					var candidates []RichSymbolRecord
					if owners := scripts[target]; len(owners) == 1 && len(owners[0].Limitations) == 1 {
						methods := csharpInheritedMembers(sourceByFile[owners[0].File], owners[0].QualifiedName, name, members, codeSourcesOrEmpty(codeSources), types, map[string]bool{})
						for _, member := range methods {
							method := member.symbol
							if method.Kind != "method" || member.typeName != "void" || member.static {
								continue
							}
							if knownMode && !strings.HasSuffix(method.QualifiedName, "("+parameter+")") {
								continue
							}
							candidates = append(candidates, method)
							ref.CandidateSymbolIDs = append(ref.CandidateSymbolIDs, method.ID)
						}
					}
					if len(candidates) > 1 {
						ref.Resolution = SymbolResolutionAmbiguous
					}
					if len(candidates) == 1 && knownMode && len(candidates[0].Limitations) == 1 {
						method := candidates[0]
						ref.To, ref.ToSymbolID, ref.TargetQualifiedName = method.File, method.ID, method.QualifiedName
						ref.Resolution, ref.NonPromotable, ref.Internal, ref.Confidence, ref.ConfidenceScore = SymbolResolutionExact, false, true, "EXTRACTED", 1
						ref.Reason = "persistent UnityEvent target script and fixed listener signature match; event activation is not execution proof"
					}
					facts.References = append(facts.References, ref)
				}
				for i := 0; i+1 < len(node.Content); i += 2 {
					key := node.Content[i].Value
					if prefix != "" {
						key = prefix + "." + key
					}
					visit(node.Content[i+1], key)
				}
			} else if node.Kind == yaml.SequenceNode {
				for i, child := range node.Content {
					visit(child, fmt.Sprintf("%s[%d]", prefix, i))
				}
			}
		}
		visit(values, "")
	}
}

func codeSourcesOrEmpty(sources [][]csharpSource) []csharpSource {
	if len(sources) == 1 {
		return sources[0]
	}
	return nil
}

func unityEventParameters(sources []csharpSource, owner RichSymbolRecord, fieldName string, members map[string][]csharpMember, types map[string][]RichSymbolRecord) (string, bool) {
	for _, source := range sources {
		if source.file != owner.File {
			continue
		}
		fields := csharpInheritedMembers(source, owner.QualifiedName, fieldName, members, sources, types, map[string]bool{})
		if len(fields) != 1 || len(fields[0].symbol.Limitations) != 1 {
			return "", false
		}
		for _, member := range fields {
			declaringSource := source
			for _, candidate := range sources {
				if candidate.file == member.symbol.File {
					declaringSource = candidate
					break
				}
			}
			name, parameters := member.typeName, ""
			if at := strings.Index(name, "<"); at >= 0 && strings.HasSuffix(name, ">") {
				parameters = name[at+1 : len(name)-1]
				name = name[:at]
			}
			if !csharpFrameworkType(declaringSource, member.owner, name, "UnityEngine.Events", "UnityEvent", sources, types, map[string]bool{}) {
				return "", false
			}
			return parameters, true
		}
	}
	return "", false
}
