package scan

import (
	"reflect"
	"testing"
)

func TestExtractGradleLiteralMetadata(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		want GradlePackageRecord
	}{
		{
			name: "kotlin",
			path: "build.gradle.kts",
			body: `group = "com.weka"
dependencies {
    implementation("com.weka:shared:1.2")
}`,
			want: GradlePackageRecord{
				Path:  "build.gradle.kts",
				Group: "com.weka",
				Dependencies: []GradleDependencyRecord{{
					Group: "com.weka", Artifact: "shared", Version: "1.2", Scope: "implementation",
				}},
			},
		},
		{
			name: "groovy",
			path: "build.gradle",
			body: `group = 'com.weka'
dependencies {
    implementation 'com.weka:shared:1.2'
}`,
			want: GradlePackageRecord{
				Path:  "build.gradle",
				Group: "com.weka",
				Dependencies: []GradleDependencyRecord{{
					Group: "com.weka", Artifact: "shared", Version: "1.2", Scope: "implementation",
				}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := extractGradlePackage(test.path, test.body)
			if !ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("extractGradlePackage() = %#v, %v, want %#v, true", got, ok, test.want)
			}
		})
	}

	settings, ok := extractGradlePackage("settings.gradle", `rootProject.name = "users-api"`)
	if !ok || settings.Artifact != "users-api" {
		t.Fatalf("settings metadata = %#v, %v, want users-api", settings, ok)
	}
}

func TestExtractGradleProvenanceAndComputedCoverage(t *testing.T) {
	var workspace WorkspaceIndex
	mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(FileRecord{Path: "build.gradle"}, `group = "com.weka"
dependencies {
    implementation("com.weka:shared:1.2")
}`))
	mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(FileRecord{Path: "settings.gradle"}, `rootProject.name = "users-api"`))

	body := `package com.weka.users;

import com.weka.shared.Shared;

class Consumer {
    Shared shared;
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Consumer.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, workspace)
	assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Consumer", "com.weka:users-api")
	ref := assertJavaReference(t, facts.References, "imports_type", "com.weka.shared.Shared", 3)
	wantEvidence := []string{"gradle:com.weka:users-api -> com.weka:shared"}
	if !reflect.DeepEqual(ref.DependencyEvidence, wantEvidence) {
		t.Fatalf("Gradle dependency evidence = %#v, want %#v", ref.DependencyEvidence, wantEvidence)
	}

	var computed WorkspaceIndex
	mergeWorkspaceIndex(&computed, extractWorkspaceRecord(FileRecord{Path: "build.gradle"}, `group = providers.gradleProperty("group")
dependencies {
    implementation(sharedCoordinates)
}`))
	mergeWorkspaceIndex(&computed, extractWorkspaceRecord(FileRecord{Path: "settings.gradle"}, `rootProject.name = "users-api"`))
	computedFacts := ExtractJavaSymbolFacts(source, body, computed)
	declaration := assertRichDeclaration(t, computedFacts.Declarations, "class", "com.weka.users.Consumer", "users-api")
	if declaration.Coverage != CoveragePartial || len(declaration.Limitations) == 0 {
		t.Fatalf("computed Gradle declaration provenance = %#v, want partial coverage with limitation", declaration)
	}
	if declaration.Artifact == "com.weka:users-api" {
		t.Fatalf("computed group was guessed: %#v", declaration)
	}
}

func TestGradleMultiModuleDerivesPartialSubprojectProvenance(t *testing.T) {
	var workspace WorkspaceIndex
	mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(
		FileRecord{Path: "settings.gradle"},
		`rootProject.name = "platform"
include("users-api", "orders-api")`,
	))
	mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(
		FileRecord{Path: "users-api/build.gradle"},
		`group = "com.weka"`,
	))
	mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(
		FileRecord{Path: "orders-api/build.gradle"},
		`group = "com.weka"
dependencies {
    implementation("com.weka:users-api:1.0")
}`,
	))

	providerBody := `package com.weka.users.api;

public class UserService {}
`
	consumerBody := `package com.weka.orders;

import com.weka.users.api.UserService;

class OrderService {
    UserService users;
}
`
	sources := []JavaSourceRecord{
		extractJavaSource(FileRecord{
			Path:     "users-api/src/main/java/com/weka/users/api/UserService.java",
			Language: "java",
		}, providerBody),
		extractJavaSource(FileRecord{
			Path:     "orders-api/src/main/java/com/weka/orders/OrderService.java",
			Language: "java",
		}, consumerBody),
	}
	facts := ExtractJavaProjectSymbolFacts(sources, map[string]string{
		sources[0].File: providerBody,
		sources[1].File: consumerBody,
	}, workspace)

	provider := assertRichDeclaration(
		t,
		facts.Declarations,
		"class",
		"com.weka.users.api.UserService",
		"com.weka:users-api",
	)
	if provider.Coverage != CoveragePartial || len(provider.Limitations) == 0 {
		t.Fatalf("derived Gradle provider provenance = %#v, want partial with limitation", provider)
	}
	consumer := assertRichDeclaration(
		t,
		facts.Declarations,
		"class",
		"com.weka.orders.OrderService",
		"com.weka:orders-api",
	)
	if consumer.Coverage != CoveragePartial || len(consumer.Limitations) == 0 {
		t.Fatalf("derived Gradle consumer provenance = %#v, want partial with limitation", consumer)
	}
	reference := assertJavaReference(t, facts.References, "imports_type", "com.weka.users.api.UserService", 3)
	wantEvidence := []string{"gradle:com.weka:orders-api -> com.weka:users-api"}
	if !reflect.DeepEqual(reference.DependencyEvidence, wantEvidence) {
		t.Fatalf("multi-module Gradle dependency evidence = %#v, want %#v", reference.DependencyEvidence, wantEvidence)
	}
}

func TestGradleMultiModuleRequiresMatchingDependencyForCrossProjectExact(t *testing.T) {
	tests := []struct {
		name         string
		dependencies string
	}{
		{name: "missing dependency"},
		{
			name: "mismatched dependency",
			dependencies: `dependencies {
    implementation("com.weka:inventory-api:1.0")
}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var workspace WorkspaceIndex
			mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(
				FileRecord{Path: "settings.gradle"},
				`rootProject.name = "platform"
include("users-api", "orders-api")`,
			))
			mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(
				FileRecord{Path: "users-api/build.gradle"},
				`group = "com.weka"`,
			))
			mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(
				FileRecord{Path: "orders-api/build.gradle"},
				"group = \"com.weka\"\n"+test.dependencies,
			))

			providerBody := `package com.weka.users.api;

public class UserService {}
`
			consumerBody := `package com.weka.orders;

import com.weka.users.api.UserService;

class OrderService {
    UserService users;
}
`
			sources := []JavaSourceRecord{
				extractJavaSource(FileRecord{
					Path:     "users-api/src/main/java/com/weka/users/api/UserService.java",
					Language: "java",
				}, providerBody),
				extractJavaSource(FileRecord{
					Path:     "orders-api/src/main/java/com/weka/orders/OrderService.java",
					Language: "java",
				}, consumerBody),
			}
			facts := ExtractJavaProjectSymbolFacts(sources, map[string]string{
				sources[0].File: providerBody,
				sources[1].File: consumerBody,
			}, workspace)
			provider := assertRichDeclaration(
				t,
				facts.Declarations,
				"class",
				"com.weka.users.api.UserService",
				"com.weka:users-api",
			)
			reference := assertJavaReference(t, facts.References, "imports_type", "com.weka.users.api.UserService", 3)
			if reference.ToSymbolID != "" || reference.Resolution != SymbolResolutionUnresolved {
				t.Fatalf("cross-project reference without matching dependency became exact: %#v", reference)
			}
			if !reflect.DeepEqual(reference.CandidateSymbolIDs, []string{provider.ID}) {
				t.Fatalf("cross-project candidates = %#v, want provider %q", reference.CandidateSymbolIDs, provider.ID)
			}
			if len(reference.DependencyEvidence) != 0 {
				t.Fatalf("cross-project dependency evidence = %#v, want none", reference.DependencyEvidence)
			}

			finalized := FinalizeProjectSymbolFacts(nil, workspace, facts)
			finalReference := assertJavaReference(t, finalized.References, "imports_type", "com.weka.users.api.UserService", 3)
			if finalReference.ToSymbolID != "" ||
				finalReference.Resolution != SymbolResolutionUnresolved ||
				!finalReference.NonPromotable {
				t.Fatalf("finalization promoted unsupported cross-project reference: %#v", finalReference)
			}
		})
	}
}

func TestGradleIncludedProjectsDoNotInheritRootArtifactAsComplete(t *testing.T) {
	tests := []struct {
		name         string
		settingsPath string
		settingsBody string
		sourcePath   string
	}{
		{
			name:         "groovy",
			settingsPath: "settings.gradle",
			settingsBody: `rootProject.name = "platform"
include 'users-api'`,
			sourcePath: "users-api/src/main/java/com/weka/users/UserService.java",
		},
		{
			name:         "kotlin",
			settingsPath: "settings.gradle.kts",
			settingsBody: `rootProject.name = "platform"
include(":services:users-api")`,
			sourcePath: "services/users-api/src/main/java/com/weka/users/UserService.java",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var workspace WorkspaceIndex
			mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(
				FileRecord{Path: "build.gradle"},
				`group = "com.weka"`,
			))
			mergeWorkspaceIndex(&workspace, extractWorkspaceRecord(
				FileRecord{Path: test.settingsPath},
				test.settingsBody,
			))
			body := `package com.weka.users;

public class UserService {}
`
			source := extractJavaSource(FileRecord{
				Path:     test.sourcePath,
				Language: "java",
			}, body)
			facts := ExtractJavaSymbolFacts(source, body, workspace)
			declaration := assertRichDeclaration(
				t,
				facts.Declarations,
				"class",
				"com.weka.users.UserService",
				"com.weka:users-api",
			)
			if declaration.Coverage != CoveragePartial || len(declaration.Limitations) == 0 {
				t.Fatalf("included Gradle project provenance = %#v, want partial with limitation", declaration)
			}
			if declaration.Artifact == "com.weka:platform" {
				t.Fatalf("included Gradle project inherited root artifact: %#v", declaration)
			}
		})
	}
}

func TestGradleComputedIncludeArgumentsStayUnresolved(t *testing.T) {
	body := `include(projectName)
include(provider("users-api"))
`
	record, ok := extractGradlePackage("settings.gradle.kts", body)
	if ok || len(record.included) != 0 {
		t.Fatalf("computed Gradle includes were guessed: %#v, %v", record, ok)
	}
	limitations := gradleExtractionLimitations("settings.gradle.kts", body)
	if len(limitations) != 2 {
		t.Fatalf("computed Gradle include limitations = %#v, want two", limitations)
	}
}

func TestGradleInterpolationAndUnknownCallsStayUnresolved(t *testing.T) {
	body := `group = "com.${tenant}"
rootProject.name = "$service-api"
dependencies {
    implementation("com.weka:${module}:1.2")
    api(sharedCoordinates)
    println("com.weka:not-a-dependency:1.0")
    custom("com.weka:also-not-a-dependency:1.0")
}`
	record, ok := extractGradlePackage("build.gradle", body)
	if ok || record.Group != "" || record.Artifact != "" || len(record.Dependencies) != 0 {
		t.Fatalf("interpolated/unknown Gradle metadata was extracted: %#v, %v", record, ok)
	}
	limitations := gradleExtractionLimitations("build.gradle", body)
	if len(limitations) != 4 {
		t.Fatalf("Gradle limitations = %#v, want group, artifact, and two recognized dependency limitations", limitations)
	}
}

func TestGradleCountsPartialCoveragePerRecognizedConfiguration(t *testing.T) {
	body := `dependencies {
    implementation("com.weka:shared:1.2")
    runtimeOnly(runtimeCoordinates)
    println("com.weka:fake:1.0")
}`
	record, ok := extractGradlePackage("build.gradle.kts", body)
	if !ok || len(record.Dependencies) != 1 || record.Dependencies[0].Scope != "implementation" {
		t.Fatalf("recognized literal dependency = %#v, %v", record, ok)
	}
	limitations := gradleExtractionLimitations("build.gradle.kts", body)
	if len(limitations) != 1 {
		t.Fatalf("recognized computed dependency limitations = %#v, want one", limitations)
	}
}

func TestComputedMavenCoordinatesStayPartial(t *testing.T) {
	pom := `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>${project.group}</groupId>
  <artifactId>users-api</artifactId>
</project>`
	workspace := extractWorkspaceRecord(FileRecord{Path: "pom.xml"}, pom)
	if len(workspace.MavenPackages) != 1 || workspace.MavenPackages[0].GroupID != "" {
		t.Fatalf("computed Maven group was retained: %#v", workspace.MavenPackages)
	}
	body := `package com.weka.users;
class Consumer {}`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Consumer.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, workspace)
	declaration := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Consumer", "users-api")
	if declaration.Coverage != CoveragePartial || len(declaration.Limitations) == 0 {
		t.Fatalf("computed Maven provenance = %#v, want partial with limitation", declaration)
	}

	computedArtifactPOM := `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.weka</groupId>
  <artifactId>${service.name}</artifactId>
</project>`
	computedArtifact := extractWorkspaceRecord(FileRecord{Path: "pom.xml"}, computedArtifactPOM)
	if len(computedArtifact.MavenPackages) != 1 || computedArtifact.MavenPackages[0].GroupID != "com.weka" || computedArtifact.MavenPackages[0].ArtifactID != "" {
		t.Fatalf("computed Maven artifact handling = %#v, want retained literal group only", computedArtifact.MavenPackages)
	}
	computedArtifactFacts := ExtractJavaSymbolFacts(source, body, computedArtifact)
	partial := assertRichDeclaration(t, computedArtifactFacts.Declarations, "class", "com.weka.users.Consumer", "com.weka")
	if partial.Coverage != CoveragePartial || len(partial.Limitations) == 0 {
		t.Fatalf("computed Maven artifact provenance = %#v, want partial", partial)
	}
}

func TestGradleIgnoresCommentsAndMultilineStrings(t *testing.T) {
	body := `/*
group = "com.fake.block"
implementation("com.fake:block-comment:1")
*/
// rootProject.name = "fake-line"
def docs = """
group = "com.fake.text"
rootProject.name = "fake-text"
implementation("com.fake:text:1")
"""
group = "com.weka"
rootProject.name = "users-api"
dependencies {
    implementation("com.weka:shared:1.2")
}`
	record, ok := extractGradlePackage("build.gradle", body)
	if !ok || record.Group != "com.weka" || record.Artifact != "users-api" {
		t.Fatalf("Gradle comments/string metadata leaked into provenance: %#v, %v", record, ok)
	}
	if len(record.Dependencies) != 1 || record.Dependencies[0].Group != "com.weka" || record.Dependencies[0].Artifact != "shared" {
		t.Fatalf("Gradle comments/string dependencies leaked: %#v", record.Dependencies)
	}
	if limitations := gradleExtractionLimitations("build.gradle", body); len(limitations) != 0 {
		t.Fatalf("Gradle comments/string limitations leaked: %#v", limitations)
	}
}

func TestGradleMultipleAssignmentsStayPartial(t *testing.T) {
	body := `group = "com.first"
group = "com.second"
rootProject.name = "users-api"
rootProject.name = providers.gradleProperty("artifact")
`
	record, ok := extractGradlePackage("build.gradle", body)
	if ok || record.Group != "" || record.Artifact != "" {
		t.Fatalf("multiple Gradle assignments were guessed: %#v, %v", record, ok)
	}
	limitations := gradleExtractionLimitations("build.gradle", body)
	if len(limitations) != 2 {
		t.Fatalf("multiple Gradle assignment limitations = %#v, want group and artifact limitations", limitations)
	}
}
