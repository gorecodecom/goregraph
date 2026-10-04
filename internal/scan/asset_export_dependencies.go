package scan

import "fmt"

// AssetExportDependency represents a file hash in editor JSON serializers.
type AssetExportDependency struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

// NormalizeAssetExportDependencies accepts Blender maps and Unity dependency lists.
func NormalizeAssetExportDependencies(mapped map[string]string, listed []AssetExportDependency) (map[string]string, error) {
	if len(mapped)+len(listed) > 10000 {
		return nil, fmt.Errorf("export dependency limit exceeded")
	}
	result := make(map[string]string, len(mapped)+len(listed))
	for name, hash := range mapped {
		result[name] = hash
	}
	for _, dependency := range listed {
		if dependency.File == "" || dependency.SHA256 == "" {
			return nil, fmt.Errorf("empty dependency identity")
		}
		if _, exists := result[dependency.File]; exists {
			return nil, fmt.Errorf("duplicate dependency identity")
		}
		result[dependency.File] = dependency.SHA256
	}
	return result, nil
}
