package scan

import (
	"encoding/json"
	"testing"
)

func TestBlenderExportsRequireCurrentIndexedSource(t *testing.T) {
	report := map[string]any{"schema_version": 1, "engine": "blender", "producer_version": "5.2.2", "source": "Art/Knight.blend", "source_sha256": "current", "objects": []map[string]any{{"id": "object:Knight", "name": "Knight", "kind": "mesh", "properties": map[string]any{"vertices": 8}, "references": []map[string]any{{"property": "material", "target": "material:Armor"}}}, {"id": "material:Armor", "name": "Armor", "kind": "material"}}, "samples": []map[string]any{{"object": "object:Knight", "frame": 1, "vertices": 8, "triangles": 12}}}
	for _, hash := range []string{"current", "stale"} {
		body, _ := json.MarshalIndent(report, "", "  ")
		file := FileRecord{Path: "Knight.goregraph-blender.json"}
		assets, facts := analyzeAssetExports([]FileRecord{{Path: "Art/Knight.blend", Hash: hash, Language: "blender"}}, []assetExportSource{{file, string(body)}})
		if hash == "current" {
			if len(facts.Declarations) != 3 || len(facts.References) != 1 || facts.References[0].Resolution != SymbolResolutionExact {
				t.Fatal(facts)
			}
			if len(assets.Nodes) != 2 || len(assets.Diagnostics) != 0 {
				t.Fatal(assets)
			}
			context := BuildProjectAgentContextIndex("game", "", nil, nil, facts.Declarations, facts.References, nil, nil, nil, nil)
			enrichAssetContext(&context, assets)
			summaries := 0
			for _, fact := range context.Facts {
				if fact.Summary != "" {
					summaries++
				}
			}
			if len(context.Facts) != 3 || summaries != 2 {
				t.Fatal(context)
			}
		} else if len(facts.Declarations) != 0 || len(assets.Diagnostics) != 2 || assets.Diagnostics[0].Code != "asset_export_stale" {
			t.Fatal(assets, facts)
		}
	}
	report["source"] = "../Other/Knight.blend"
	body, _ := json.Marshal(report)
	assets, facts := analyzeAssetExports(nil, []assetExportSource{{FileRecord{Path: "x.goregraph-blender.json"}, string(body)}})
	if len(facts.Declarations) != 0 || assets.Diagnostics[0].Code != "asset_export_outside_project" {
		t.Fatal(assets)
	}
}
