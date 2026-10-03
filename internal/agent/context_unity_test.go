package agent

import (
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveUnityContextDeliversAuthoredCSharp(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 6000.3.0f1\n")
	writeSourceFile(t, root, "Packages/manifest.json", `{}`)
	writeSourceFile(t, root, "Assets/Scripts/RoundTowerMovement.cs", "using UnityEngine;\npublic class RoundTowerMovement : MonoBehaviour {\n public void Turn() { transform.Rotate(0, 90, 0); }\n"+strings.Repeat(" // Additional class implementation.\n", 130)+"}\n")
	writeSourceFile(t, root, "Library/Generated.cs", "class RoundTowerMovementGenerated {}")
	writeSourceFile(t, root, "Assets/Scripts/FigureGroundContact.cs", "public class FigureGroundContact {\n public void Check() { VerifyFootSupport(); }\n}\n")
	for _, name := range []string{"MaterialAssets", "EffectAssets", "GalleryAssets"} {
		writeSourceFile(t, root, "Assets/Scripts/"+name+".cs", "public class "+name+" {\n // Explain RoundTowerMovement and FigureGroundContact method Turn in Unity\n public void Preview() {}\n}\n")
	}
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain RoundTowerMovement and FigureGroundContact and the Turn method in Unity", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.SourceSections) == 0 {
		t.Fatalf("no C# source context: %#v", pack)
	}
	found := false
	for _, section := range pack.SourceSections {
		if strings.HasPrefix(section.Path, "Library/") {
			t.Fatalf("generated cache became project context: %#v", section)
		}
		if section.Path == "Assets/Scripts/RoundTowerMovement.cs" && strings.Contains(section.Content, "transform.Rotate") && section.ReadReceipt != "" {
			if (section.RenderMode != "focused" && section.RenderMode != "declaration_body") || section.EndLine-section.StartLine+1 > 61 {
				t.Fatalf("oversized class was not delivered as a bounded window: %#v", section)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("authored declaration body or receipt missing: %#v", pack.SourceSections)
	}
	contactFound := false
	for _, section := range pack.SourceSections {
		contactFound = contactFound || section.Path == "Assets/Scripts/FigureGroundContact.cs"
	}
	if !contactFound {
		t.Fatalf("explicitly named declaration displaced by incidental vocabulary: %#v", pack.SourceSections)
	}
	if len(pack.CallChain) != 0 || len(pack.Endpoints) != 0 {
		t.Fatalf("invented execution analysis for index adapter: %#v", pack)
	}
	if pack.FallbackRequired || len(pack.Entrypoints) == 0 {
		t.Fatalf("supported static C# method context missing: %#v", pack)
	}
	strict, err := BuildContext(ContextRequest{Root: root, Query: pack.Query})
	if err != nil || len(strict.SourceSections) > 1 {
		t.Fatalf("strict mode unexpectedly expanded named source candidates: %#v %v", strict, err)
	}
	bounded, err := BuildContext(ContextRequest{Root: root, Query: pack.Query, ProtocolVersion: AdaptiveV2, BudgetTokens: MinContextBudgetTokens, MaxFiles: 1})
	if err != nil || bounded.EstimatedTokens > MinContextBudgetTokens || len(bounded.SourceSections) > 1 {
		t.Fatalf("unsupported source evidence exceeded bounds: %#v %v", bounded, err)
	}
}
