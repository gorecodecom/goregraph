package scan

import (
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

const swiftTargetProject = `// !$*UTF8*$!
{ objects = {
 ROOT = { isa = PBXProject; mainGroup = GROUP; targets = (CORE,APP,OTHER); };
 GROUP = { isa = PBXGroup; children = (CFILE,AFILE,OFILE); sourceTree = "<group>"; };
 CFILE = {isa=PBXFileReference; path=Core.swift; sourceTree="<group>";};
 AFILE = {isa=PBXFileReference; path=App.swift; sourceTree="<group>";};
 OFILE = {isa=PBXFileReference; path=Other.swift; sourceTree="<group>";};
 CB = {isa=PBXBuildFile; fileRef=CFILE;}; AB={isa=PBXBuildFile; fileRef=AFILE;}; OB={isa=PBXBuildFile; fileRef=OFILE;};
 CP = {isa=PBXSourcesBuildPhase;files=(CB);}; AP={isa=PBXSourcesBuildPhase;files=(AB);}; OP={isa=PBXSourcesBuildPhase;files=(OB);};
 CORE={isa=PBXNativeTarget;name=Core;buildPhases=(CP);};
 APP={isa=PBXNativeTarget;name=App;buildPhases=(AP);dependencies=(DEP);};
 OTHER={isa=PBXNativeTarget;name=Other;buildPhases=(OP);};
 DEP={isa=PBXTargetDependency;target=CORE;};
 }; rootObject=ROOT; }`

func TestSwiftXcodeTargetsRestrictStaticImportsAndSharedSources(t *testing.T) {
	provider := parseSwiftSource(FileRecord{Path: "Core.swift"}, "struct Helper {func load() {}}")
	app := parseSwiftSource(FileRecord{Path: "App.swift"}, "import Core\nstruct App {func run(){let helper=Helper();helper.load()}}")
	other := parseSwiftSource(FileRecord{Path: "Other.swift"}, "import Core\nstruct Other {func run(){let helper=Helper();helper.load()}}")
	sources := []swiftSource{provider, app, other}
	assignSwiftTargets(sources, []assetExportSource{{file: FileRecord{Path: "App.xcodeproj/project.pbxproj"}, body: swiftTargetProject}})
	result := analyzeSwiftProject(sources)
	if len(result.graph.Edges) != 1 || result.graph.Edges[0].SourceFile != "App.swift" || result.graph.Edges[0].To.File != "Core.swift" {
		t.Fatal(result.graph)
	}
	shared := strings.Replace(swiftTargetProject, "files=(OB)", "files=(OB,CB)", 1)
	sources = []swiftSource{provider, app, other}
	assignSwiftTargets(sources, []assetExportSource{{file: FileRecord{Path: "App.xcodeproj/project.pbxproj"}, body: shared}})
	if edges := analyzeSwiftProject(sources).graph.Edges; len(edges) != 0 {
		t.Fatal("shared source promoted", edges)
	}
}

func TestSwiftTargetMetadataIsConsumedByTheNormalScanner(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Core.swift", "struct Helper {func load(){}}")
	writeFile(t, root, "App.swift", "import Core\nstruct App {func run(){let helper=Helper();helper.load()}}")
	writeFile(t, root, "Other.swift", "import Core\nstruct Other {func run(){let helper=Helper();helper.load()}}")
	writeFile(t, root, "App.xcodeproj/project.pbxproj", swiftTargetProject)
	index, _, err := scanProject(root, config.Defaults(), gitignore.Matcher{})
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Swift.graph.Edges) != 1 || index.Swift.graph.Edges[0].SourceFile != "App.swift" {
		t.Fatal(index.Swift.graph)
	}
}

func TestSwiftConditionalModuleNamesRemainUnassigned(t *testing.T) {
	project := strings.Replace(swiftTargetProject, "CORE={isa=PBXNativeTarget;name=Core;", "CONFIG={isa=XCBuildConfiguration;buildSettings={\"PRODUCT_MODULE_NAME[sdk=iphoneos*]\"=DeviceCore;};}; LIST={buildConfigurations=(CONFIG);}; CORE={isa=PBXNativeTarget;name=Core;buildConfigurationList=LIST;", 1)
	sources := []swiftSource{parseSwiftSource(FileRecord{Path: "Core.swift"}, "struct Helper{}")}
	assignSwiftTargets(sources, []assetExportSource{{file: FileRecord{Path: "App.xcodeproj/project.pbxproj"}, body: project}})
	if len(sources[0].limitations) == 0 || !strings.HasPrefix(sources[0].module, "unassigned/") {
		t.Fatal("conditional SDK module name was treated as exact", sources[0].module)
	}
}

func TestOpenStepMetadataRejectsMalformedDuplicateAndEscapingValues(t *testing.T) {
	for _, body := range []string{"{a=1;a=2;}", "{a=(1,2;}", "{a=\"unterminated;}", "/* unfinished", "{a={b=1;};} trailing"} {
		if _, ok := parseOpenStep(body); ok {
			t.Fatal(body)
		}
	}
	project := strings.Replace(swiftTargetProject, "path=Core.swift", "path=../Core.swift", 1)
	provider := parseSwiftSource(FileRecord{Path: "Core.swift"}, "struct Helper{}")
	sources := []swiftSource{provider}
	assignSwiftTargets(sources, []assetExportSource{{file: FileRecord{Path: "App.xcodeproj/project.pbxproj"}, body: project}})
	if len(sources[0].limitations) == 0 {
		t.Fatal("escaping target membership accepted")
	}
}
