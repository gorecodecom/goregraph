package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

// assetExportDependencyFallback checks binary source freshness without running an importer.
func assetExportDependencyFallback(loaded loadedContextIndex, candidate sourceCandidate, file sourceFile) string {
	if !strings.HasSuffix(candidate.Path, ".goregraph-blender.json") && !strings.HasSuffix(candidate.Path, ".goregraph-unity.json") {
		return ""
	}
	var report struct {
		Source          string                       `json:"source"`
		Hash            string                       `json:"source_sha256"`
		Dependencies    map[string]string            `json:"dependencies"`
		DependencyFiles []scan.AssetExportDependency `json:"dependency_files"`
	}
	if json.Unmarshal([]byte(strings.Join(file.Lines, "\n")), &report) != nil || report.Source == "" || len(report.Hash) != 64 {
		return ContextFallbackEvidenceConflict
	}
	inputs, dependencyErr := scan.NormalizeAssetExportDependencies(report.Dependencies, report.DependencyFiles)
	if dependencyErr != nil {
		return ContextFallbackEvidenceConflict
	}
	if previous, exists := inputs[report.Source]; exists && previous != report.Hash {
		return ContextFallbackEvidenceConflict
	}
	inputs[report.Source] = report.Hash
	remaining := int64(256 * 1024 * 1024)
	for name, expected := range inputs {
		if len(expected) != 64 {
			return ContextFallbackEvidenceConflict
		}
		source := candidate
		source.Path = name
		resolved, err := resolveSourcePath(loaded, source)
		if err != nil {
			return ContextFallbackSourceUnreadable
		}
		input, err := os.Open(resolved)
		if err != nil {
			return ContextFallbackSourceUnreadable
		}
		before, statErr := input.Stat()
		if statErr != nil || !before.Mode().IsRegular() || before.Size() > remaining {
			input.Close()
			return ContextFallbackSourceUnreadable
		}
		hash := sha256.New()
		count, readErr := io.Copy(hash, io.LimitReader(input, remaining+1))
		after, statErr := input.Stat()
		input.Close()
		if readErr != nil || statErr != nil {
			return ContextFallbackSourceUnreadable
		}
		if count != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || hex.EncodeToString(hash.Sum(nil)) != expected {
			return ContextFallbackEvidenceConflict
		}
		remaining -= count
	}
	return ""
}

func contextAssetSource(name string) bool {
	if strings.HasSuffix(name, ".goregraph-blender.json") || strings.HasSuffix(name, ".goregraph-unity.json") {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".unity", ".prefab", ".asset", ".meta", ".mat", ".controller", ".overridecontroller", ".anim", ".playable", ".asmdef", ".asmref":
		return true
	}
	return false
}
