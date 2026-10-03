package scan

import "strings"

// linkExportedUnityReferences joins explicit editor reports to the native GUID inventory.
func linkExportedUnityReferences(assets *AssetIndexRecord, facts *ProjectSymbolFacts) {
	byGUID := map[string][]AssetNodeRecord{}
	for _, node := range assets.Nodes {
		if node.GUID != "" {
			byGUID[node.GUID] = append(byGUID[node.GUID], node)
		}
	}
	for i := range assets.References {
		ref := &assets.References[i]
		if ref.Resolution == SymbolResolutionExact || !strings.HasPrefix(ref.LocalID, "GlobalObjectId_V1-") {
			continue
		}
		parts := strings.Split(ref.LocalID, "-")
		if len(parts) != 5 || !unityGUID.MatchString(parts[2]) {
			continue
		}
		guid := strings.ToLower(parts[2])
		owners := map[string]bool{}
		for _, node := range byGUID[guid] {
			owners[node.File] = true
		}
		if len(owners) > 1 {
			ref.Resolution = SymbolResolutionAmbiguous
			ref.Reason = "GlobalObjectId matches duplicate GUID owners"
			for j := range facts.References {
				relation := &facts.References[j]
				if relation.FromSymbolID == ref.From && relation.TargetQualifiedName == ref.File+"#"+ref.LocalID {
					relation.Resolution = SymbolResolutionAmbiguous
					relation.Reason = ref.Reason
				}
			}
			continue
		}
		var exact, roots []AssetNodeRecord
		for _, node := range byGUID[guid] {
			if node.LocalID == parts[3] {
				exact = append(exact, node)
			}
			if node.LocalID == "" {
				roots = append(roots, node)
			}
		}
		candidates := exact
		if len(candidates) == 0 {
			candidates = roots
		}
		if len(candidates) != 1 {
			if len(candidates) > 1 {
				ref.Resolution = SymbolResolutionAmbiguous
				ref.Reason = "GlobalObjectId matches duplicate GUID owners"
			}
			continue
		}
		target := candidates[0]
		ref.To = target.ID
		ref.GUID = guid
		ref.Resolution = SymbolResolutionExact
		ref.Reason = "exported GlobalObjectId resolves to native indexed GUID/fileID"
		if target.LocalID == "" {
			ref.Reason = "exported GlobalObjectId resolves to indexed GUID owner; internal object is not decoded"
		}
		for j := range facts.References {
			relation := &facts.References[j]
			if relation.FromSymbolID != ref.From || relation.TargetQualifiedName != ref.File+"#"+ref.LocalID {
				continue
			}
			relation.ToSymbolID = target.ID
			relation.To = target.File
			relation.TargetQualifiedName = target.File
			if target.LocalID != "" {
				relation.TargetQualifiedName += "#" + target.LocalID
			}
			relation.Internal = true
			relation.NonPromotable = false
			relation.Resolution = SymbolResolutionExact
			relation.Confidence = "EXTRACTED"
			relation.ConfidenceScore = 1
			relation.Reason = ref.Reason
		}
	}
}
