package scan

import (
	"context"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

func TestUnitySerializedAssetsLinkScriptsAndLocalObjects(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 6000.6.4f1")
	writeFile(t, root, "Packages/manifest.json", "{}")
	writeFile(t, root, "Assets/Player.cs", "namespace Game; public class Player { public void Tick(){} }")
	writeFile(t, root, "Assets/Player.cs.meta", "guid: 11111111111111111111111111111111\n")
	writeFile(t, root, "Assets/Player.prefab", `%YAML 1.1
%TAG !u! tag:unity3d.com,2011:
--- !u!1 &100
GameObject:
  m_Name: Knight
  m_Component:
  - component: {fileID: 200}
--- !u!114 &200
MonoBehaviour:
  m_GameObject: {fileID: 100}
  m_Script: {fileID: 11500000, guid: 11111111111111111111111111111111, type: 3}
  missing: {fileID: 123456}
  external: {fileID: 123, guid: 22222222222222222222222222222222, type: 3}
  none: {fileID: 0}
`)
	index, _, err := scanProject(root, config.Defaults(), gitignore.Matcher{})
	if err != nil {
		t.Fatal(err)
	}
	exact, unresolved, scripts := 0, 0, 0
	for _, ref := range index.Assets.References {
		if ref.Resolution == SymbolResolutionExact {
			exact++
		} else {
			unresolved++
		}
	}
	for _, ref := range index.SymbolFacts.References {
		if ref.Type == "uses_script" && ref.Resolution == SymbolResolutionExact {
			scripts++
		}
	}
	if exact != 5 || unresolved != 2 || scripts != 1 {
		t.Fatalf("exact=%d unresolved=%d scripts=%d assets=%#v", exact, unresolved, scripts, index.Assets)
	}
	if len(index.Assets.Diagnostics) != 1 || index.Assets.Diagnostics[0].Code != "unity_missing_local_reference" {
		t.Fatal(index.Assets.Diagnostics)
	}
}

func TestAssetInventoryMatchesSnapshotAndAcceptsLargeScene(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 6000.6.4f1")
	writeFile(t, root, "Packages/manifest.json", "{}")
	writeFile(t, root, "Assets/Large.unity", "%YAML 1.1\n--- !u!1 &1\nGameObject:\n  m_Name: Knight\n#"+strings.Repeat("x", 600000))
	writeFile(t, root, "Assets/image.png", "\x89PNG\x00binary")
	writeFile(t, root, "Assets/Binary.asset", "\x00serialized-binary")
	writeFile(t, root, "Art/Model.blend", "BLENDER-v\x00fixture")
	cfg := config.Defaults()
	index, _, err := scanProject(root, cfg, gitignore.Matcher{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _, err := SnapshotProjectFiles(context.Background(), root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Files) != len(snapshot) {
		t.Fatalf("scan=%v snapshot=%v", index.Files, snapshot)
	}
	seen := map[string]FileRecord{}
	for _, file := range index.Files {
		seen[file.Path] = file
	}
	for _, file := range snapshot {
		if seen[file.Path] != file {
			t.Fatalf("inconsistent asset inventory: %v vs %v", seen[file.Path], file)
		}
	}
	if seen["Assets/Large.unity"].Hash == "" || seen["Art/Model.blend"].Kind != "binary_asset" {
		t.Fatal(seen)
	}
}
