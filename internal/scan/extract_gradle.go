package scan

import (
	"regexp"
	"sort"
	"strings"
)

var (
	gradleGroupAssignmentRE      = regexp.MustCompile(`(?m)^\s*group\s*=\s*(.+?)\s*$`)
	gradleArtifactAssignmentRE   = regexp.MustCompile(`(?m)^\s*rootProject\.name\s*=\s*(.+?)\s*$`)
	gradleDependencyStatementRE  = regexp.MustCompile(`(?m)^\s*(implementation|api|compileOnly|runtimeOnly|testImplementation|testRuntimeOnly)\s*(.*?)\s*$`)
	gradleParenthesizedLiteralRE = regexp.MustCompile(`^\(\s*["']([^"']*)["']\s*\)$`)
	gradleBareLiteralRE          = regexp.MustCompile(`^["']([^"']*)["']$`)
)

func extractGradlePackage(filePath, body string) (GradlePackageRecord, bool) {
	record, _ := parseGradleMetadata(filePath, body)
	return record, record.Group != "" || record.Artifact != "" || len(record.Dependencies) > 0
}

func gradleExtractionLimitations(filePath, body string) []string {
	_, limitations := parseGradleMetadata(filePath, body)
	return limitations
}

func parseGradleMetadata(filePath, body string) (GradlePackageRecord, []string) {
	record := GradlePackageRecord{Path: filePath}
	var limitations []string
	if match := gradleGroupAssignmentRE.FindStringSubmatch(body); len(match) == 2 {
		if value, ok := literalGradleValue(match[1]); ok {
			record.Group = value
		} else {
			limitations = append(limitations, filePath+": computed Gradle group is not statically resolved")
		}
	}
	if match := gradleArtifactAssignmentRE.FindStringSubmatch(body); len(match) == 2 {
		if value, ok := literalGradleValue(match[1]); ok {
			record.Artifact = value
		} else {
			limitations = append(limitations, filePath+": computed Gradle artifact is not statically resolved")
		}
	}
	for _, match := range gradleDependencyStatementRE.FindAllStringSubmatch(body, -1) {
		coordinate, ok := literalGradleValue(match[2])
		if !ok {
			limitations = append(limitations, filePath+": computed Gradle "+match[1]+" dependency coordinates are not statically resolved")
			continue
		}
		parts := strings.Split(coordinate, ":")
		if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
			limitations = append(limitations, filePath+": invalid Gradle "+match[1]+" dependency coordinates are not statically resolved")
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
	return record, limitations
}

func literalGradleValue(expression string) (string, bool) {
	expression = strings.TrimSpace(expression)
	value := ""
	if match := gradleParenthesizedLiteralRE.FindStringSubmatch(expression); len(match) == 2 {
		value = match[1]
	} else if match := gradleBareLiteralRE.FindStringSubmatch(expression); len(match) == 2 {
		value = match[1]
	} else {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "$") {
		return "", false
	}
	return value, true
}
