package scan

import (
	"encoding/json"
	"math"
)

// validAssetExportSamples rejects null coordinates rather than decoding them as zero.
func validAssetExportSamples(body string) bool {
	var envelope struct {
		Samples []map[string]any `json:"samples"`
	}
	if json.Unmarshal([]byte(body), &envelope) != nil {
		return false
	}
	number := func(value any, integer bool) bool {
		n, ok := value.(float64)
		return ok && !math.IsInf(n, 0) && !math.IsNaN(n) && (!integer || n >= 0 && n == math.Trunc(n))
	}
	vector := func(value any, size int) bool {
		items, ok := value.([]any)
		if !ok || len(items) != size {
			return false
		}
		for _, item := range items {
			if !number(item, false) {
				return false
			}
		}
		return true
	}
	for _, sample := range envelope.Samples {
		for _, key := range []string{"vertices", "triangles", "degenerate_polygons"} {
			if value, found := sample[key]; found && !number(value, true) {
				return false
			}
		}
		for _, key := range []string{"bounds_min", "bounds_max", "bone_head", "bone_tail"} {
			if value, found := sample[key]; found && !vector(value, 3) {
				return false
			}
		}
		if value, found := sample["bone_matrix"]; found {
			rows, ok := value.([]any)
			if !ok || len(rows) != 4 {
				return false
			}
			for _, row := range rows {
				if !vector(row, 4) {
					return false
				}
			}
		}
		positions, positionsPresent := sample["vertex_positions"].([]any)
		indices, indicesPresent := sample["triangle_indices"].([]any)
		if _, exists := sample["vertex_positions"]; exists && !positionsPresent {
			return false
		}
		if _, exists := sample["triangle_indices"]; exists && !indicesPresent {
			return false
		}
		complete, marked := sample["geometry_complete"].(bool)
		if _, exists := sample["geometry_complete"]; exists && !marked {
			return false
		}
		if !positionsPresent && !indicesPresent && !complete {
			continue
		}
		if !positionsPresent || !indicesPresent || !complete || sample["geometry_space"] != "world" || float64(len(positions)) != sample["vertices"] || float64(len(indices)) != sample["triangles"] || len(positions) > 100000 || len(indices) > 200000 {
			return false
		}
		for _, point := range positions {
			if !vector(point, 3) {
				return false
			}
		}
		for _, triangle := range indices {
			items, ok := triangle.([]any)
			if !ok || len(items) != 3 {
				return false
			}
			for _, index := range items {
				if !number(index, true) || index.(float64) >= float64(len(positions)) {
					return false
				}
			}
		}
	}
	return true
}
