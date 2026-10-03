package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// assetExportDependencyFallback checks binary source freshness without running an importer.
func assetExportDependencyFallback(loaded loadedContextIndex, candidate sourceCandidate, file sourceFile) string {
	if !strings.HasSuffix(candidate.Path, ".goregraph-blender.json") && !strings.HasSuffix(candidate.Path, ".goregraph-unity.json") {
		return ""
	}
	var report struct {
		Source string `json:"source"`
		Hash   string `json:"source_sha256"`
	}
	if json.Unmarshal([]byte(strings.Join(file.Lines, "\n")), &report) != nil || report.Source == "" || len(report.Hash) != 64 {
		return ContextFallbackEvidenceConflict
	}
	source := candidate
	source.Path = report.Source
	resolved, err := resolveSourcePath(loaded, source)
	if err != nil {
		return ContextFallbackSourceUnreadable
	}
	input, err := os.Open(resolved)
	if err != nil {
		return ContextFallbackSourceUnreadable
	}
	defer input.Close()
	before, err := input.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() > 256*1024*1024 {
		return ContextFallbackSourceUnreadable
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(input, 256*1024*1024+1))
	if err != nil {
		return ContextFallbackSourceUnreadable
	}
	after, err := input.Stat()
	if err != nil {
		return ContextFallbackSourceUnreadable
	}
	if count != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || hex.EncodeToString(hash.Sum(nil)) != report.Hash {
		return ContextFallbackEvidenceConflict
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
