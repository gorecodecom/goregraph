package scan

import (
	"path"
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
	sanitized := sanitizeGradleLexical(body)
	record.Group, limitations = resolveGradleAssignments(filePath, "group", gradleGroupAssignmentRE.FindAllStringSubmatch(sanitized, -1), limitations)
	record.Artifact, limitations = resolveGradleAssignments(filePath, "artifact", gradleArtifactAssignmentRE.FindAllStringSubmatch(sanitized, -1), limitations)
	if record.Artifact == "" {
		if artifact := derivedGradleSubprojectArtifact(filePath); artifact != "" {
			record.Artifact = artifact
			limitations = append(
				limitations,
				filePath+": Gradle artifact derived from build file directory; settings project renames are not statically resolved",
			)
		}
	}
	for _, match := range gradleDependencyStatementRE.FindAllStringSubmatch(sanitized, -1) {
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

func derivedGradleSubprojectArtifact(filePath string) string {
	normalized := strings.Trim(strings.ReplaceAll(filePath, "\\", "/"), "/")
	switch path.Base(normalized) {
	case "build.gradle", "build.gradle.kts":
	default:
		return ""
	}
	root := path.Dir(normalized)
	if root == "." || root == "" {
		return ""
	}
	return path.Base(root)
}

func resolveGradleAssignments(filePath, kind string, matches [][]string, limitations []string) (string, []string) {
	if len(matches) == 0 {
		return "", limitations
	}
	if len(matches) > 1 {
		return "", append(limitations, filePath+": multiple Gradle "+kind+" assignments are not statically resolved")
	}
	if value, ok := literalGradleValue(matches[0][1]); ok {
		return value, limitations
	}
	return "", append(limitations, filePath+": computed Gradle "+kind+" is not statically resolved")
}

func sanitizeGradleLexical(body string) string {
	const (
		gradleLexCode = iota
		gradleLexLineComment
		gradleLexBlockComment
		gradleLexSingleQuote
		gradleLexDoubleQuote
		gradleLexTripleSingleQuote
		gradleLexTripleDoubleQuote
	)
	result := []byte(body)
	state := gradleLexCode
	for index := 0; index < len(result); {
		if result[index] == '\n' {
			if state == gradleLexLineComment {
				state = gradleLexCode
			}
			index++
			continue
		}
		switch state {
		case gradleLexCode:
			switch {
			case index+1 < len(result) && result[index] == '/' && result[index+1] == '/':
				blankGradleBytes(result, index, 2)
				index += 2
				state = gradleLexLineComment
			case index+1 < len(result) && result[index] == '/' && result[index+1] == '*':
				blankGradleBytes(result, index, 2)
				index += 2
				state = gradleLexBlockComment
			case hasGradleTripleQuote(result, index, '\''):
				blankGradleBytes(result, index, 3)
				index += 3
				state = gradleLexTripleSingleQuote
			case hasGradleTripleQuote(result, index, '"'):
				blankGradleBytes(result, index, 3)
				index += 3
				state = gradleLexTripleDoubleQuote
			case result[index] == '\'':
				index++
				state = gradleLexSingleQuote
			case result[index] == '"':
				index++
				state = gradleLexDoubleQuote
			default:
				index++
			}
		case gradleLexLineComment:
			result[index] = ' '
			index++
		case gradleLexBlockComment:
			if index+1 < len(result) && result[index] == '*' && result[index+1] == '/' {
				blankGradleBytes(result, index, 2)
				index += 2
				state = gradleLexCode
			} else {
				result[index] = ' '
				index++
			}
		case gradleLexSingleQuote, gradleLexDoubleQuote:
			quote := byte('\'')
			if state == gradleLexDoubleQuote {
				quote = '"'
			}
			if result[index] == '\\' && index+1 < len(result) {
				index += 2
			} else if result[index] == quote {
				index++
				state = gradleLexCode
			} else {
				index++
			}
		case gradleLexTripleSingleQuote, gradleLexTripleDoubleQuote:
			quote := byte('\'')
			if state == gradleLexTripleDoubleQuote {
				quote = '"'
			}
			if hasGradleTripleQuote(result, index, quote) {
				blankGradleBytes(result, index, 3)
				index += 3
				state = gradleLexCode
			} else {
				result[index] = ' '
				index++
			}
		}
	}
	return string(result)
}

func hasGradleTripleQuote(body []byte, index int, quote byte) bool {
	return index+2 < len(body) && body[index] == quote && body[index+1] == quote && body[index+2] == quote
}

func blankGradleBytes(body []byte, index, count int) {
	for offset := 0; offset < count; offset++ {
		body[index+offset] = ' '
	}
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
