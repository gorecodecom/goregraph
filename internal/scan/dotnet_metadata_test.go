package scan

import "testing"

func TestDotnetProjectAndUnityAssemblyDependencies(t *testing.T) {
	facts := ProjectSymbolFacts{}
	for _, item := range []struct{ file, body string }{
		{"Game/Game.csproj", `<Project><PropertyGroup><AssemblyName>Game</AssemblyName></PropertyGroup><ItemGroup><ProjectReference Include="../Core/Core.csproj"/><PackageReference Include="NUnit" Version="4.0"/></ItemGroup></Project>`},
		{"Core/Core.csproj", `<Project/>`},
		{"Assets/Game.asmdef", `{"name":"Runtime","references":["Core"]}`},
		{"Assets/Core/Core.asmdef", `{"name":"Core"}`},
	} {
		MergeProjectSymbolFacts(&facts, extractDotnetMetadata(FileRecord{Path: item.file}, item.body))
	}
	facts = resolveDotnetMetadata(facts)
	facts = FinalizeProjectSymbolFacts(nil, WorkspaceIndex{}, facts)
	if len(facts.Declarations) != 4 || len(facts.References) != 3 {
		t.Fatal(facts)
	}
	exact, external := 0, 0
	for _, ref := range facts.References {
		if ref.Resolution == SymbolResolutionExact {
			exact++
		} else if ref.NonPromotable {
			external++
		}
	}
	if exact != 2 || external != 1 {
		t.Fatal(facts)
	}
}

func TestAssetCollectionsRemainSeparateFromUnityCode(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "game/Unity/ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 6000.6.4f1")
	writeFile(t, root, "game/Unity/Packages/manifest.json", "{}")
	writeFile(t, root, "game/Art/Blender/Knight.blend", "BLENDER-v\x00fixture")
	projects, err := discoverWorkspaceProjects(root, root, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("asset source absent: %#v", projects)
	}
	for _, project := range projects {
		if project.Path == "game/Art/Blender" && (project.Kind != "assets" || project.Service != "") {
			t.Fatalf("asset collection became a service: %#v", project)
		}
	}
}

func TestUnityAssemblyGUIDAndReferenceFolderBindings(t *testing.T) {
	const guid = "12345678901234567890123456789012"
	metadata := ProjectSymbolFacts{}
	for _, item := range []struct{ file, body string }{
		{"Assets/Core/Core.asmdef", `{"name":"Core"}`},
		{"Assets/Game/Game.asmdef", `{"name":"Game","references":["GUID:` + guid + `"]}`},
		{"Assets/Shared/Shared.asmref", `{"reference":"GUID:` + guid + `"}`},
	} {
		MergeProjectSymbolFacts(&metadata, extractDotnetMetadata(FileRecord{Path: item.file}, item.body))
	}
	inventory := []unitySource{{FileRecord{Path: "Assets/Core/Core.asmdef.meta"}, "guid: " + guid}}
	resolved := resolveDotnetMetadata(metadata, inventory)
	for _, ref := range resolved.References {
		if ref.Resolution != SymbolResolutionExact || ref.To != "Assets/Core/Core.asmdef" {
			t.Fatalf("unique assembly GUID not resolved: %#v", ref)
		}
	}
	sources := []csharpSource{{file: "Assets/Game/Player.cs"}, {file: "Assets/Shared/Movement.cs"}}
	assignCSharpModules(sources, resolved)
	if !sources[0].visibleModules["Assets/Core/Core.asmdef"] || sources[1].module != "Assets/Core/Core.asmdef" {
		t.Fatalf("assembly boundaries lost: %#v", sources)
	}
	inventory = append(inventory, unitySource{FileRecord{Path: "Assets/Other.png.meta"}, "guid: " + guid})
	duplicates := resolveDotnetMetadata(metadata, inventory)
	for _, ref := range duplicates.References {
		if ref.Resolution != SymbolResolutionAmbiguous || !ref.NonPromotable || ref.ToSymbolID != "" {
			t.Fatalf("duplicate GUID incorrectly promoted: %#v", ref)
		}
	}
}
