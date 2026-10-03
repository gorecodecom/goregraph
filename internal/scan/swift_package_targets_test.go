package scan

import (
	"strings"
	"testing"
)

func TestSwiftPackageLiteralTargetsRespectPathsDependenciesAndExclusions(t *testing.T) {
	manifest := `let package = Package(name: "Fixture", targets: [
 .target(name: "Core", path: "Shared", exclude: ["Ignored"]),
 .target(name: "App", dependencies: [.target(name: "Core")], path: "Application", sources: ["."]),
 .target(name: "Other"), .testTarget(name: "Empty", sources: [])
])`
	provider := parseSwiftSource(FileRecord{Path: "Shared/Core.swift"}, "struct Helper {func load(){}}")
	app := parseSwiftSource(FileRecord{Path: "Application/App.swift"}, "import Core\nstruct App {func run(){let helper=Helper();helper.load()}}")
	other := parseSwiftSource(FileRecord{Path: "Sources/Other/Other.swift"}, "import Core\nstruct Other {func run(){let helper=Helper();helper.load()}}")
	ignored := parseSwiftSource(FileRecord{Path: "Shared/Ignored/Hidden.swift"}, "struct Hidden{}")
	empty := parseSwiftSource(FileRecord{Path: "Tests/Empty/Test.swift"}, "struct Empty{}")
	sources := []swiftSource{provider, app, other, ignored, empty}
	assignSwiftTargets(sources, []assetExportSource{{file: FileRecord{Path: "Package.swift"}, body: manifest}})
	graph := analyzeSwiftProject(sources).graph
	if len(graph.Edges) != 1 || graph.Edges[0].SourceFile != app.file || graph.Edges[0].To.File != provider.file {
		t.Fatal(graph)
	}
	if len(sources[3].limitations) == 0 || len(sources[4].limitations) == 0 {
		t.Fatal("excluded or explicitly empty target files were assigned", sources)
	}
	for _, change := range []string{
		strings.Replace(manifest, `.target(name: "Core")`, `.target(name: "Core", condition: .when(platforms: [.iOS]))`, 1),
		strings.Replace(manifest, `sources: ["."]`, `sources: computedSources`, 1),
		strings.Replace(manifest, `path: "Shared"`, `path: "../Shared"`, 1),
		strings.Replace(manifest, `name: "Core",`, `name: "Core", name: "Other",`, 1),
	} {
		if _, ok := swiftPackageTargets("Package.swift", change, sources); ok {
			t.Fatal("conditional or invalid package membership was promoted", change)
		}
	}
}

func TestSwiftSynchronizedXcodeGroupsRespectTargetExceptions(t *testing.T) {
	project := `{objects={ ROOT={isa=PBXProject;mainGroup=GROUP;targets=(APP);};
 GROUP={isa=PBXGroup;children=(SYNC);sourceTree="<group>";};
 SYNC={isa=PBXFileSystemSynchronizedRootGroup;path=Sources;sourceTree="<group>";exceptions=(EX);};
 EX={isa=PBXFileSystemSynchronizedBuildFileExceptionSet;target=APP;membershipExceptions=(Excluded.swift);};
 APP={isa=PBXNativeTarget;name=App;fileSystemSynchronizedGroups=(SYNC);};};rootObject=ROOT;}`
	sources := []swiftSource{parseSwiftSource(FileRecord{Path: "Sources/Included.swift"}, "struct Included{}"), parseSwiftSource(FileRecord{Path: "Sources/Excluded.swift"}, "struct Excluded{}")}
	assignSwiftTargets(sources, []assetExportSource{{file: FileRecord{Path: "App.xcodeproj/project.pbxproj"}, body: project}})
	if sources[0].module != "App.xcodeproj/App" || len(sources[0].limitations) > 0 || len(sources[1].limitations) == 0 {
		t.Fatal(sources)
	}
}

func TestBrokenNestedSwiftMetadataDoesNotPoisonSiblingPackage(t *testing.T) {
	sources := []swiftSource{parseSwiftSource(FileRecord{Path: "Sources/Core/Core.swift"}, "struct Core{}"), parseSwiftSource(FileRecord{Path: "Nested/Broken.swift"}, "struct Broken{}")}
	assignSwiftTargets(sources, []assetExportSource{{file: FileRecord{Path: "Package.swift"}, body: `let package = Package(name: "Root", targets: [.target(name: "Core")])`}, {file: FileRecord{Path: "Nested/App.xcodeproj/project.pbxproj"}, body: "invalid"}})
	if len(sources[0].limitations) > 0 || len(sources[1].limitations) == 0 {
		t.Fatal(sources)
	}
}
