package scan

import "testing"

func TestUnityExportDuplicateGUIDCannotBindUniqueFileID(t *testing.T) {
	const guid = "12345678901234567890123456789012"
	identity := "GlobalObjectId_V1-2-" + guid + "-100-0"
	assets := AssetIndexRecord{
		Nodes: []AssetNodeRecord{
			{ID: "one", File: "Assets/One.prefab", GUID: guid, LocalID: "100"},
			{ID: "two", File: "Assets/Two.prefab", GUID: guid, LocalID: "200"},
		},
		References: []AssetReferenceRecord{{From: "export", File: "one.goregraph-unity.json", LocalID: identity, Resolution: SymbolResolutionUnresolved}},
	}
	facts := ProjectSymbolFacts{References: []RichRelationRecord{{FromSymbolID: "export", TargetQualifiedName: "one.goregraph-unity.json#" + identity, Resolution: SymbolResolutionUnresolved, NonPromotable: true}}}
	linkExportedUnityReferences(&assets, &facts)
	if assets.References[0].Resolution != SymbolResolutionAmbiguous || assets.References[0].To != "" || facts.References[0].Resolution != SymbolResolutionAmbiguous || facts.References[0].ToSymbolID != "" {
		t.Fatalf("duplicate GUID acquired a false exact target: assets=%#v facts=%#v", assets, facts)
	}
}
