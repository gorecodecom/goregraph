package scan

import (
	"context"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestUnityFileWalkKeepsSourcesAndExcludesGeneratedDirectories(t *testing.T) {
	for _, unity := range []bool{true, false} {
		root := t.TempDir()
		if unity {
			writeFile(t, root, "ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 6000.3.0f1\n")
			writeFile(t, root, "Packages/manifest.json", `{}`)
		}
		for _, name := range []string{"Library", "Temp", "obj", "Logs", "UserSettings"} {
			writeFile(t, root, name+"/Generated.cs", "class Generated {}")
		}
		writeFile(t, root, "Assets/Scripts/RoundTowerMovement.cs", "class RoundTowerMovement {}")
		writeFile(t, root, "Assets/Library/Movement.cs", "class Movement {}")
		seen := map[string]bool{}
		_, err := WalkProjectFiles(context.Background(), root, config.Defaults(), func(file WalkedFile) error { seen[file.Path] = true; return nil })
		if err != nil {
			t.Fatal(err)
		}
		if !seen["Assets/Scripts/RoundTowerMovement.cs"] || !seen["Assets/Library/Movement.cs"] {
			t.Fatalf("lost authored sources: %v", seen)
		}
		for _, name := range []string{"Library", "Temp", "obj", "Logs", "UserSettings"} {
			if seen[name+"/Generated.cs"] == unity {
				t.Fatalf("generated selection changed outside Unity or included Unity cache: unity=%v paths=%v", unity, seen)
			}
		}
	}
}

func TestWorkspaceDiscoveryExcludesUnityIL2CPPBuildProjects(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "game/Unity/Game.csproj", "<Project />")
	writeFile(t, root, "game/work/builds/one/Il2CppOutputProject/native/CMakeLists.txt", "project(Generated)")
	writeFile(t, root, "tools/native/CMakeLists.txt", "project(Tool)")
	projects, err := discoverWorkspaceProjects(root, root, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(workspaceProjectPaths(projects), ","); got != "game/Unity,tools/native" {
		t.Fatalf("incorrect workspace sources: %s", got)
	}
}

func TestCSharpDeclarationsEnterAgentContextWithoutInventedCalls(t *testing.T) {
	symbols := []RichSymbolRecord{}
	for _, kind := range []string{"class", "interface", "record", "enum", "struct", "method"} {
		symbols = append(symbols, RichSymbolRecord{ID: kind, Language: "csharp", Kind: kind, Name: "RoundTower" + kind, File: "Assets/Scripts/RoundTower.cs", Line: 2})
	}
	index := BuildProjectAgentContextIndex("Unity", "", nil, nil, symbols, nil, nil, nil, nil, nil)
	if len(index.Facts) != 6 || len(index.Edges) != 0 {
		t.Fatalf("declarations missing or invented execution edges: %#v", index)
	}
	for _, fact := range index.Facts {
		if fact.Kind != "symbol" || fact.File != "Assets/Scripts/RoundTower.cs" || fact.Line != 2 {
			t.Fatalf("incorrect C# declaration: %#v", fact)
		}
	}
	source, ok := extractAgentAuditSource(FileRecord{Path: "Assets/Scripts/RoundTower.cs", Hash: "source-hash"}, "class RoundTower {}")
	if !ok || source.Kind != "code" || source.Hash != "source-hash" || len(source.References) != 0 {
		t.Fatalf("C# source cannot be read by identity: %#v %v", source, ok)
	}
}
