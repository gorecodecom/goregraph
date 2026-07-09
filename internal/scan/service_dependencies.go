package scan

import (
	"fmt"
	"sort"
	"strings"
)

func buildServiceDependencies(project WorkspaceProjectRecord, sources []JavaSourceRecord) []WorkspaceServiceDependencyRecord {
	if len(sources) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var records []WorkspaceServiceDependencyRecord
	for _, source := range sources {
		for _, imp := range source.Imports {
			key, service := serviceDependencyFromJavaImport(imp.Name)
			if service == "" {
				continue
			}
			recordKey := project.Path + "\x00" + service + "\x00" + source.File
			if seen[recordKey] {
				continue
			}
			seen[recordKey] = true
			records = append(records, WorkspaceServiceDependencyRecord{
				FromProject:   project.Path,
				ToService:     service,
				Kind:          "java_service_client",
				Evidence:      fmt.Sprintf("%s imports %s", source.File, imp.Name),
				Confidence:    "EXTRACTED",
				ResolutionKey: key,
			})
		}
		for _, field := range source.Fields {
			key, service := serviceDependencyFromJavaType(field.Type)
			if service == "" {
				continue
			}
			recordKey := project.Path + "\x00" + service + "\x00" + field.File + "\x00" + field.Name
			if seen[recordKey] {
				continue
			}
			seen[recordKey] = true
			records = append(records, WorkspaceServiceDependencyRecord{
				FromProject:   project.Path,
				ToService:     service,
				Kind:          "java_service_client",
				Evidence:      fmt.Sprintf("%s:%d field %s %s", field.File, field.Line, field.Type, field.Name),
				Confidence:    "EXTRACTED",
				ResolutionKey: key,
			})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].ToService != records[j].ToService {
			return records[i].ToService < records[j].ToService
		}
		return records[i].Evidence < records[j].Evidence
	})
	return records
}

func serviceDependencyFromJavaImport(importName string) (string, string) {
	normalized := strings.ToLower(strings.TrimSpace(importName))
	if !strings.Contains(normalized, ".common.") {
		return "", ""
	}
	parts := strings.Split(normalized, ".")
	for _, part := range parts {
		if service := serviceDependencyServiceForKey(part); service != "" {
			return part, service
		}
	}
	return "", ""
}

func serviceDependencyFromJavaType(typeName string) (string, string) {
	key := strings.ToLower(strings.TrimSpace(typeName))
	key = strings.TrimSuffix(key, "[]")
	return key, serviceDependencyServiceForKey(key)
}

func serviceDependencyServiceForKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return ""
	}
	switch key {
	case "userservice", "usermgmtservice":
		return "ms-userservice"
	case "licenseservice", "license",
		"licensemgmtservice", "licenseserviceresponse", "licensesresponse":
		return "ms-licenseservice"
	case "productservice", "productservicemgmt":
		return "ms-productservice"
	case "cadasteruser", "cadasterusermgmt", "cadasterusermgmtservice":
		return "ms-cadasteruser"
	case "cadasterregulation", "cadasterregulationmgmt", "cadasterregulationmgmtservice":
		return "ms-cadasterregulation"
	case "cadastertask", "cadastertaskmgmt", "cadastertaskmgmtservice":
		return "ms-cadastertask"
	case "documenttopic", "documenttopicmgmt", "documenttopicmgmtservice":
		return "ms-documenttopic"
	case "documentinfo", "documentinfoservice":
		return "ms-documentinfo"
	case "documentdownload", "documentdownloadservice":
		return "ms-documentdownload"
	case "documentexport", "documentexportservice":
		return "ms-documentexport"
	case "regulationtree", "regulationtreeservice":
		return "ms-regulationtree"
	case "regulationchange", "regulationchangeservice":
		return "ms-regulationchange"
	default:
		return ""
	}
}
