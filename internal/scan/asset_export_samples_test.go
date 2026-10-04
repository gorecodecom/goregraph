package scan

import (
	"encoding/json"
	"testing"
)

func TestAssetSurfaceSamplesRejectNullOrInvalidGeometry(t *testing.T) {
	valid := `{"samples":[{"object":"mesh:A","vertices":3,"triangles":1,"geometry_complete":true,"geometry_space":"world","vertex_positions":[[0,0,0],[1,0,0],[0,1,0]],"triangle_indices":[[0,1,2]],"bounds_min":[0,0,0],"bounds_max":[1,1,0]}]}`
	if !validAssetExportSamples(valid) {
		t.Fatal("valid sampled surface rejected")
	}
	var envelope map[string][]map[string]any
	json.Unmarshal([]byte(valid), &envelope)
	for _, change := range []struct {
		key   string
		value any
	}{
		{"bounds_min", []any{nil, 0, 0}}, {"vertex_positions", nil}, {"triangle_indices", [][]int{{0, 1, 3}}},
		{"triangle_indices", [][]any{{0, 1, nil}}}, {"geometry_complete", false}, {"geometry_space", "local"}, {"vertices", 4},
	} {
		var candidate map[string][]map[string]any
		json.Unmarshal([]byte(valid), &candidate)
		candidate["samples"][0][change.key] = change.value
		body, _ := json.Marshal(candidate)
		if validAssetExportSamples(string(body)) {
			t.Fatal("invalid surface accepted", change)
		}
	}
}

func TestAssetExportDependenciesMustRemainIndexedAndCurrent(t *testing.T) {
	report := map[string]any{"schema_version": 1, "engine": "blender", "producer_version": "5.2.2", "source": "Main.blend", "source_sha256": "main", "dependencies": map[string]string{"Library.blend": "library"}, "objects": []map[string]any{{"id": "object:A", "name": "A", "kind": "mesh"}}}
	body, _ := json.Marshal(report)
	for _, hash := range []string{"library", "changed"} {
		assets, facts := analyzeAssetExports([]FileRecord{{Path: "Main.blend", Hash: "main"}, {Path: "Library.blend", Hash: hash}}, []assetExportSource{{file: FileRecord{Path: "Main.goregraph-blender.json"}, body: string(body)}})
		if hash == "library" && len(facts.Declarations) != 1 || hash == "changed" && (len(facts.Declarations) != 0 || len(assets.Diagnostics) != 1 || assets.Diagnostics[0].Code != "asset_export_stale") {
			t.Fatal(assets, facts)
		}
	}
}

func TestEditorDependencyListsRejectDuplicateHashClaims(t *testing.T) {
	listed := []AssetExportDependency{{File: "Model.fbx.meta", SHA256: "current"}}
	if result, err := NormalizeAssetExportDependencies(nil, listed); err != nil || result["Model.fbx.meta"] != "current" {
		t.Fatal(result, err)
	}
	if _, err := NormalizeAssetExportDependencies(map[string]string{"Model.fbx.meta": "old"}, listed); err == nil {
		t.Fatal("conflicting dependency forms accepted")
	}
	if _, err := NormalizeAssetExportDependencies(nil, append(listed, listed...)); err == nil {
		t.Fatal("duplicate editor input accepted")
	}
}
