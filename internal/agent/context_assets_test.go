package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestBlenderContextUsesVerifiedExportAndRejectsChangedSource(t *testing.T) {
	root := t.TempDir()
	body := "BLENDER-v\x00synthetic"
	sum := sha256.Sum256([]byte(body))
	writeSourceFile(t, root, "Knight.blend", body)
	report := map[string]any{"schema_version": 1, "engine": "blender", "producer_version": "5.2.2", "source": "Knight.blend", "source_sha256": hex.EncodeToString(sum[:]), "objects": []map[string]any{{"id": "object:Knight", "name": "Knight", "kind": "mesh", "properties": map[string]any{"vertices": 8, "polygons": 6}}}, "limitations": []string{"Explicit frame sample only"}}
	data, _ := json.MarshalIndent(report, "", "  ")
	writeSourceFile(t, root, "Knight.goregraph-blender.json", string(data))
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain Knight mesh geometry vertices and polygons in Blender", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, section := range pack.SourceSections {
		if strings.HasSuffix(section.Path, ".goregraph-blender.json") && section.ReadReceipt != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Blender export source absent: %#v", pack)
	}
	writeSourceFile(t, root, "Knight.blend", body+"changed")
	stale, err := BuildContext(ContextRequest{Root: root, Query: pack.Query, ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range stale.SourceSections {
		if strings.HasSuffix(section.Path, ".goregraph-blender.json") {
			t.Fatalf("changed binary retained current geometry context: %#v", stale)
		}
	}
}

func TestCSharpTypedContextKeepsCallChainAndSourceReceipts(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "ItemsController.cs", `namespace App;
 [ApiController][Route("api/items")]
 class ItemsController : ControllerBase {
 Service service;
 [HttpGet("{id}")] public void Get() { service.Load(); }
 }`)
	writeSourceFile(t, root, "Service.cs", `namespace App;
 class Service { public void Load() {} }
 `)
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Trace GET /api/items/{id} through ItemsController to Service.Load in C#", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.CallChain) == 0 {
		t.Fatalf("C# static call graph was lost: %#v", pack)
	}
	sources := map[string]bool{}
	for _, section := range pack.SourceSections {
		if section.ReadReceipt != "" {
			sources[section.Path] = true
		}
	}
	if !sources["ItemsController.cs"] || !sources["Service.cs"] {
		t.Fatalf("typed call sources missing: %#v", pack.SourceSections)
	}
}

func TestAssetSourceRedactionPreservesEvidenceAndLines(t *testing.T) {
	file := sourceFile{Path: "Model.goregraph-blender.json", Lines: []string{"{", `  "name": "Knight",`, `  "vertices": 8,`, `  "password": "must-not-leak"`, `}`}}
	redacted, err := redactSourceReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(redacted) != len(file.Lines) || strings.Contains(strings.Join(redacted, "\n"), "must-not-leak") || !strings.Contains(redacted[2], "8") {
		t.Fatal(redacted)
	}
	yaml := redactAssetSourceLine("  m_Script: {fileID: 11500000, guid: 11111111111111111111111111111111, type: 3}")
	if !strings.Contains(yaml, "11111111111111111111111111111111") {
		t.Fatal(yaml)
	}
	if strings.Contains(redactAssetSourceLine("  apiKey: must-not-leak"), "must-not-leak") {
		t.Fatal("serialized user value leaked")
	}
}

func TestUnityNativeAssetContextKeepsSourceReceipt(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "Assets/Knight.prefab", `%YAML 1.1
%TAG !u! tag:unity3d.com,2011:
--- !u!1 &100
GameObject:
  m_Name: Knight
  m_IsActive: 1
  m_Component:
  - component: {fileID: 200}
--- !u!4 &200
Transform:
  m_GameObject: {fileID: 100}
  m_LocalScale: {x: 1, y: 1, z: 1}
`)
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain Knight prefab GameObject and Transform components in Unity", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range pack.SourceSections {
		if section.Path == "Assets/Knight.prefab" && section.ReadReceipt != "" && strings.Contains(section.Content, "m_Name: Knight") {
			return
		}
	}
	t.Fatalf("native Unity source evidence missing: %#v", pack)
}

func TestBlenderFrameSampleIsNavigableWithNumericEvidence(t *testing.T) {
	root := t.TempDir()
	body := "BLENDER-v\x00frame-fixture"
	sum := sha256.Sum256([]byte(body))
	writeSourceFile(t, root, "Rig.blend", body)
	report := map[string]any{
		"schema_version": 1, "engine": "blender", "producer_version": "5.2.2", "source": "Rig.blend", "source_sha256": hex.EncodeToString(sum[:]),
		"objects": []map[string]any{{"id": "bone:Rig/Foot", "name": "Foot", "kind": "bone"}},
		"padding": make([]string, 120),
		"samples": []map[string]any{{"object": "bone:Rig/Foot", "frame": 10, "bone_head": []float64{1, 2, 3}, "bone_tail": []float64{1, 2, 4}}},
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	writeSourceFile(t, root, "Rig.goregraph-blender.json", string(data))
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain Foot frame 10 bone head and tail in Blender", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range pack.SourceSections {
		if section.ReadReceipt != "" && strings.Contains(section.Content, `"frame": 10`) && strings.Contains(section.Content, `"bone_head": [`) && strings.Contains(section.Content, "      2,") {
			return
		}
	}
	t.Fatalf("selected frame values did not reach context: %#v", pack)
}

func TestAssetNumericArrayRedactionRetainsOnlySupportedData(t *testing.T) {
	lines := []string{`"bone_matrix": [`, `  [`, `    1,`, `    0`, `  ]`, `],`, `"private_numbers": [`, `  123456789`, `]`}
	redacted := redactAssetSourceLines(lines)
	if redacted[2] != lines[2] || redacted[3] != lines[3] || strings.Contains(strings.Join(redacted, "\n"), "123456789") {
		t.Fatal(redacted)
	}
}
