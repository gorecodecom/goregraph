package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestAssetFreshnessRejectsChangedLinkedLibrariesAndEscapingDependencies(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "Main.blend", "main")
	writeSourceFile(t, root, "Library.blend", "library")
	hash := func(value string) string { sum := sha256.Sum256([]byte(value)); return hex.EncodeToString(sum[:]) }
	report := map[string]any{"source": "Main.blend", "source_sha256": hash("main"), "dependencies": map[string]string{"Library.blend": hash("library")}}
	body, _ := json.Marshal(report)
	file := sourceFile{Lines: strings.Split(string(body), "\n")}
	loaded := loadedContextIndex{ScopeRoot: root}
	candidate := sourceCandidate{Path: "Main.goregraph-blender.json"}
	if reason := assetExportDependencyFallback(loaded, candidate, file); reason != "" {
		t.Fatal(reason)
	}
	writeSourceFile(t, root, "Library.blend", "changed")
	if reason := assetExportDependencyFallback(loaded, candidate, file); reason != ContextFallbackEvidenceConflict {
		t.Fatal(reason)
	}
	report["dependencies"] = map[string]string{"../outside/Private.blend": hash("private")}
	body, _ = json.Marshal(report)
	file.Lines = strings.Split(string(body), "\n")
	if reason := assetExportDependencyFallback(loaded, candidate, file); reason != ContextFallbackSourceUnreadable {
		t.Fatal(reason)
	}
}

func TestUnityExportFreshnessIncludesImportedMetadata(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "Assets/Unit.prefab", "prefab")
	writeSourceFile(t, root, "Assets/Mesh.fbx.meta", "import settings")
	hash := func(value string) string { sum := sha256.Sum256([]byte(value)); return hex.EncodeToString(sum[:]) }
	report := map[string]any{"source": "Assets/Unit.prefab", "source_sha256": hash("prefab"), "dependency_files": []map[string]string{{"file": "Assets/Mesh.fbx.meta", "sha256": hash("import settings")}}}
	body, _ := json.Marshal(report)
	file := sourceFile{Lines: strings.Split(string(body), "\n")}
	loaded := loadedContextIndex{ScopeRoot: root}
	candidate := sourceCandidate{Path: "Unit.goregraph-unity.json"}
	if reason := assetExportDependencyFallback(loaded, candidate, file); reason != "" {
		t.Fatal(reason)
	}
	writeSourceFile(t, root, "Assets/Mesh.fbx.meta", "changed import settings")
	if reason := assetExportDependencyFallback(loaded, candidate, file); reason != ContextFallbackEvidenceConflict {
		t.Fatal("changed import settings did not invalidate editor evidence", reason)
	}
}
