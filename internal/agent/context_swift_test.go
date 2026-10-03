package agent

import (
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAdaptiveSwiftContextDeliversAuthoredMethodsAndCalls(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "Package.swift", "// swift-tools-version: 6.0\nimport PackageDescription\nlet package = Package(name: \"Game\")\n")
	writeSourceFile(t, root, "Sources/Game/RoundTowerMovement.swift", "struct RoundTowerMovement {\n func turn(id: Int) { verifyFootSupport() }\n func verifyFootSupport() {}\n"+strings.Repeat(" // Additional implementation.\n", 130)+"}\n")
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain RoundTowerMovement turn and verifyFootSupport in Swift", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, section := range pack.SourceSections {
		if section.Path == "Sources/Game/RoundTowerMovement.swift" && strings.Contains(section.Content, "verifyFootSupport()") && section.ReadReceipt != "" {
			found = true
			if section.EndLine-section.StartLine+1 > 61 {
				t.Fatal(section)
			}
		}
	}
	if !found {
		t.Fatalf("Swift source context absent: %#v", pack)
	}
	if len(pack.Entrypoints) == 0 || pack.FallbackRequired {
		t.Fatalf("Swift method was not available as supported static entrypoint: %#v", pack)
	}
}
