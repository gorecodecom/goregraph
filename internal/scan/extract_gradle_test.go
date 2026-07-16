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
