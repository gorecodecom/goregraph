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
