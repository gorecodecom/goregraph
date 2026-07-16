package scan

import (
	"regexp"
	"sort"
	"strings"
)

var (
	gradleGroupLiteralRE      = regexp.MustCompile(`(?m)^\s*group\s*=\s*["']([^"']+)["']\s*$`)
	gradleArtifactLiteralRE   = regexp.MustCompile(`(?m)^\s*rootProject\.name\s*=\s*["']([^"']+)["']\s*$`)
	gradleDependencyLiteralRE = regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:\(\s*["']([^"']+)["']\s*\)|["']([^"']+)["'])\s*$`)
	gradleGroupStatementRE    = regexp.MustCompile(`(?m)^\s*group\s*=`)
	gradleArtifactStatementRE = regexp.MustCompile(`(?m)^\s*rootProject\.name\s*=`)
	gradleDependencyCallRE    = regexp.MustCompile(`(?m)^\s*(?:implementation|api|compileOnly|runtimeOnly|testImplementation|testRuntimeOnly)\b`)
)

func extractGradlePackage(filePath, body string) (GradlePackageRecord, bool) {
	record := GradlePackageRecord{Path: filePath}
	if match := gradleGroupLiteralRE.FindStringSubmatch(body); len(match) == 2 {
		record.Group = strings.TrimSpace(match[1])
	}
	if match := gradleArtifactLiteralRE.FindStringSubmatch(body); len(match) == 2 {
		record.Artifact = strings.TrimSpace(match[1])
	}
	for _, match := range gradleDependencyLiteralRE.FindAllStringSubmatch(body, -1) {
		coordinate := match[2]
		if coordinate == "" {
			coordinate = match[3]
		}
		parts := strings.Split(strings.TrimSpace(coordinate), ":")
		if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
			continue
		}
		dependency := GradleDependencyRecord{Group: parts[0], Artifact: parts[1], Scope: match[1]}
		if len(parts) == 3 {
			dependency.Version = parts[2]
		}
		record.Dependencies = append(record.Dependencies, dependency)
	}
	sort.Slice(record.Dependencies, func(i, j int) bool {
		if record.Dependencies[i].Group != record.Dependencies[j].Group {
			return record.Dependencies[i].Group < record.Dependencies[j].Group
		}
		if record.Dependencies[i].Artifact != record.Dependencies[j].Artifact {
			return record.Dependencies[i].Artifact < record.Dependencies[j].Artifact
		}
		return record.Dependencies[i].Scope < record.Dependencies[j].Scope
	})
	return record, record.Group != "" || record.Artifact != "" || len(record.Dependencies) > 0
}

func gradleExtractionLimitations(filePath, body string) []string {
	var limitations []string
	if gradleGroupStatementRE.MatchString(body) && !gradleGroupLiteralRE.MatchString(body) {
		limitations = append(limitations, filePath+": computed Gradle group is not statically resolved")
	}
	if gradleArtifactStatementRE.MatchString(body) && !gradleArtifactLiteralRE.MatchString(body) {
		limitations = append(limitations, filePath+": computed Gradle artifact is not statically resolved")
	}
	literalDependencies := len(gradleDependencyLiteralRE.FindAllStringSubmatch(body, -1))
	dependencyCalls := len(gradleDependencyCallRE.FindAllString(body, -1))
	if dependencyCalls > literalDependencies {
		limitations = append(limitations, filePath+": computed Gradle dependency coordinates are not statically resolved")
	}
	return limitations
}
