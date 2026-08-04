package scan

import (
	"fmt"
	"sort"
	"strings"
)

var javaDependencyBoundarySuffixes = []string{"Client", "Service", "Gateway", "Connector", "Api"}

var javaDependencyIgnoredImportPrefixes = []string{
	"java.",
	"javax.",
	"jakarta.",
	"org.junit.",
	"org.springframework.",
}

func buildServiceDependencies(project WorkspaceProjectRecord, sources []JavaSourceRecord) []WorkspaceServiceDependencyRecord {
	if len(sources) == 0 {
		return nil
	}
	localTypes := javaDependencyLocalTypes(sources)
	seen := map[string]bool{}
	var records []WorkspaceServiceDependencyRecord
	for _, source := range sources {
		for _, imp := range source.Imports {
			typeName, ok := javaDependencyImportedBoundary(imp, localTypes)
			if !ok {
				continue
			}
			evidence, used := javaDependencyUsageEvidence(source, typeName)
			if !used {
				continue
			}
			variants := canonicalServiceIdentityVariants(typeName)
			if len(variants) == 0 {
				continue
			}
			resolutionKey := variants[0]
			recordKey := project.Path + "\x00" + resolutionKey + "\x00" + source.File
			if seen[recordKey] {
				continue
			}
			seen[recordKey] = true
			records = append(records, WorkspaceServiceDependencyRecord{
				FromProject:   project.Path,
				Kind:          "java_client_import",
				Evidence:      fmt.Sprintf("%s imports %s; %s", source.File, imp.Name, evidence),
				Confidence:    "EXTRACTED",
				ResolutionKey: resolutionKey,
			})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].ResolutionKey != records[j].ResolutionKey {
			return records[i].ResolutionKey < records[j].ResolutionKey
		}
		return records[i].Evidence < records[j].Evidence
	})
	return records
}

func javaDependencyLocalTypes(sources []JavaSourceRecord) map[string]struct{} {
	localTypes := make(map[string]struct{})
	for _, source := range sources {
		for _, javaType := range source.Types {
			if javaType.QualifiedName != "" {
				localTypes[javaType.QualifiedName] = struct{}{}
			} else if source.Package != "" {
				localTypes[source.Package+"."+javaType.Name] = struct{}{}
			} else {
				localTypes[javaType.Name] = struct{}{}
			}
		}
	}
	return localTypes
}

func javaDependencyImportedBoundary(imp JavaImportRecord, localTypes map[string]struct{}) (string, bool) {
	importName := strings.TrimSpace(imp.Name)
	if imp.Static || importName == "" || strings.HasSuffix(importName, ".*") {
		return "", false
	}
	for _, prefix := range javaDependencyIgnoredImportPrefixes {
		if strings.HasPrefix(importName, prefix) {
			return "", false
		}
	}
	typeName := shortJavaName(importName)
	if !javaDependencyBoundaryType(typeName) {
		return "", false
	}
	if _, local := localTypes[importName]; local {
		return "", false
	}
	return typeName, true
}

func javaDependencyBoundaryType(typeName string) bool {
	for _, suffix := range javaDependencyBoundarySuffixes {
		if len(typeName) > len(suffix) && strings.HasSuffix(typeName, suffix) {
			return true
		}
	}
	return false
}

func javaDependencyUsageEvidence(source JavaSourceRecord, typeName string) (string, bool) {
	for _, field := range source.Fields {
		if javaDependencySimpleType(field.Type) == typeName {
			return fmt.Sprintf("field %s:%d %s %s", field.File, field.Line, field.Type, field.Name), true
		}
	}
	for _, method := range source.Methods {
		if method.Name != method.Owner || method.ReturnType != "" {
			continue
		}
		for _, parameter := range method.Parameters {
			if javaDependencySimpleType(parameter.Type) == typeName {
				return fmt.Sprintf("constructor %s:%d parameter %s %s", method.File, method.Line, parameter.Type, parameter.Name), true
			}
		}
	}
	return "", false
}

func javaDependencySimpleType(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "...")
	value = strings.TrimSuffix(value, "[]")
	if generic := strings.Index(value, "<"); generic >= 0 {
		value = value[:generic]
	}
	return shortJavaName(strings.TrimSpace(value))
}
