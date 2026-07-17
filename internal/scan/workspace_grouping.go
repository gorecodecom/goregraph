package scan

import (
	"path"
	"sort"
	"strings"
)

const (
	workspaceArchitectureConfigSource   = "dashboard_config"
	workspaceArchitectureFallbackSource = "workspace_path"
	workspaceArchitectureExact          = "EXACT"
	workspaceArchitectureExtracted      = "EXTRACTED"
	workspaceArchitecturePartial        = "PARTIAL"
)

type workspaceNamespaceEvidence struct {
	record   WorkspaceProjectNamespaceRecord
	segments []string
}

func BuildWorkspaceArchitectureLayout(registry WorkspaceRegistryRecord, namespaces []WorkspaceProjectNamespaceRecord, config WorkspaceDashboardConfig) WorkspaceArchitectureLayoutRecord {
	projects := append([]WorkspaceProjectRecord(nil), registry.Projects...)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})

	evidence := workspaceProductionNamespaceEvidence(projects, namespaces)
	namespacePrefixLength := workspaceNamespacePrefixLength(evidence)
	groups := make(map[string]WorkspaceArchitectureGroupRecord, len(config.Architecture.Groups))
	for groupID, group := range config.Architecture.Groups {
		groups[groupID] = WorkspaceArchitectureGroupRecord{
			ID:         groupID,
			Label:      group.Label,
			Source:     workspaceArchitectureConfigSource,
			Confidence: workspaceArchitectureExact,
			Manual:     true,
		}
	}

	projectPaths := make(map[string]bool, len(projects))
	services := make([]WorkspaceArchitectureServiceLayoutRecord, 0, len(projects))
	labels := make(map[string]string, len(projects))
	for _, project := range projects {
		projectPaths[project.Path] = true
		labels[project.Path] = firstNonEmpty(project.Service, project.Name, project.Path)
		if configured, ok := config.Architecture.Services[project.Path]; ok {
			services = append(services, WorkspaceArchitectureServiceLayoutRecord{
				Project:    project.Path,
				GroupID:    configured.Group,
				Order:      configured.Order,
				Source:     workspaceArchitectureConfigSource,
				Confidence: workspaceArchitectureExact,
				Manual:     true,
			})
			continue
		}

		service := workspaceInferredServiceLayout(project, evidence[project.Path], namespacePrefixLength)
		services = append(services, service)
		if _, exists := groups[service.GroupID]; !exists {
			groups[service.GroupID] = workspaceInferredGroup(service)
		}
	}

	staleServices := make([]string, 0)
	for project := range config.Architecture.Services {
		if !projectPaths[project] {
			staleServices = append(staleServices, project)
		}
	}
	sort.Strings(staleServices)

	groupRecords := make([]WorkspaceArchitectureGroupRecord, 0, len(groups))
	for _, group := range groups {
		groupRecords = append(groupRecords, group)
	}
	groupOrder := make(map[string]int, len(config.Architecture.GroupOrder))
	for order, groupID := range config.Architecture.GroupOrder {
		groupOrder[groupID] = order
	}
	sort.Slice(groupRecords, func(i, j int) bool {
		iOrder, iConfigured := groupOrder[groupRecords[i].ID]
		jOrder, jConfigured := groupOrder[groupRecords[j].ID]
		if iConfigured != jConfigured {
			return iConfigured
		}
		if iConfigured && iOrder != jOrder {
			return iOrder < jOrder
		}
		return groupRecords[i].ID < groupRecords[j].ID
	})
	for order := range groupRecords {
		groupRecords[order].Order = order
	}

	sort.Slice(services, func(i, j int) bool {
		if services[i].Order != services[j].Order {
			return services[i].Order < services[j].Order
		}
		if labels[services[i].Project] != labels[services[j].Project] {
			return labels[services[i].Project] < labels[services[j].Project]
		}
		if services[i].Project != services[j].Project {
			return services[i].Project < services[j].Project
		}
		return StableWorkspaceID("service", services[i].Project) < StableWorkspaceID("service", services[j].Project)
	})

	return WorkspaceArchitectureLayoutRecord{Groups: groupRecords, Services: services, StaleServices: staleServices}
}

func (record WorkspaceArchitectureLayoutRecord) Service(project string) WorkspaceArchitectureServiceLayoutRecord {
	for _, service := range record.Services {
		if service.Project == project {
			return service
		}
	}
	return WorkspaceArchitectureServiceLayoutRecord{}
}

func workspaceProductionNamespaceEvidence(projects []WorkspaceProjectRecord, namespaces []WorkspaceProjectNamespaceRecord) map[string]workspaceNamespaceEvidence {
	knownProjects := make(map[string]bool, len(projects))
	for _, project := range projects {
		knownProjects[project.Path] = true
	}
	candidates := make(map[string][]workspaceNamespaceEvidence)
	for _, namespace := range namespaces {
		if !knownProjects[namespace.Project] || !strings.EqualFold(strings.TrimSpace(namespace.Source), "production_package") {
			continue
		}
		segments := workspaceNamespaceSegments(namespace.Namespace)
		if len(segments) == 0 {
			continue
		}
		candidates[namespace.Project] = append(candidates[namespace.Project], workspaceNamespaceEvidence{record: namespace, segments: segments})
	}

	evidence := make(map[string]workspaceNamespaceEvidence, len(candidates))
	for project, projectCandidates := range candidates {
		sort.Slice(projectCandidates, func(i, j int) bool {
			iNamespace := strings.Join(projectCandidates[i].segments, ".")
			jNamespace := strings.Join(projectCandidates[j].segments, ".")
			if iNamespace != jNamespace {
				return iNamespace < jNamespace
			}
			if projectCandidates[i].record.Language != projectCandidates[j].record.Language {
				return projectCandidates[i].record.Language < projectCandidates[j].record.Language
			}
			return projectCandidates[i].record.Confidence < projectCandidates[j].record.Confidence
		})
		evidence[project] = projectCandidates[0]
	}
	return evidence
}

func workspaceNamespaceSegments(namespace string) []string {
	return strings.FieldsFunc(strings.TrimSpace(namespace), func(r rune) bool {
		return r == '.' || r == '/' || r == '\\'
	})
}

func workspaceNamespacePrefixLength(evidence map[string]workspaceNamespaceEvidence) int {
	projects := make([]string, 0, len(evidence))
	for project := range evidence {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	if len(projects) == 0 {
		return 0
	}
	prefixLength := len(evidence[projects[0]].segments)
	for _, project := range projects[1:] {
		segments := evidence[project].segments
		if len(segments) < prefixLength {
			prefixLength = len(segments)
		}
		for index := 0; index < prefixLength; index++ {
			if evidence[projects[0]].segments[index] != segments[index] {
				prefixLength = index
				break
			}
		}
	}
	return prefixLength + 1
}

func workspaceInferredServiceLayout(project WorkspaceProjectRecord, evidence workspaceNamespaceEvidence, prefixLength int) WorkspaceArchitectureServiceLayoutRecord {
	if len(evidence.segments) > 0 {
		if prefixLength <= 0 || prefixLength > len(evidence.segments) {
			prefixLength = len(evidence.segments)
		}
		confidence := strings.TrimSpace(evidence.record.Confidence)
		if confidence == "" {
			confidence = workspaceArchitectureExtracted
		}
		return WorkspaceArchitectureServiceLayoutRecord{
			Project:    project.Path,
			GroupID:    strings.Join(evidence.segments[:prefixLength], "."),
			Source:     evidence.record.Source,
			Confidence: confidence,
		}
	}
	return workspaceFallbackServiceLayout(project)
}

func workspaceFallbackServiceLayout(project WorkspaceProjectRecord) WorkspaceArchitectureServiceLayoutRecord {
	role := workspaceServiceRole(project)
	parent := path.Dir(strings.Trim(strings.ReplaceAll(project.Path, "\\", "/"), "/"))
	if parent == "." || parent == "" {
		parent = "workspace"
	}
	return WorkspaceArchitectureServiceLayoutRecord{
		Project:    project.Path,
		GroupID:    role + ":" + parent,
		Source:     workspaceArchitectureFallbackSource,
		Confidence: workspaceArchitecturePartial,
	}
}

func workspaceInferredGroup(service WorkspaceArchitectureServiceLayoutRecord) WorkspaceArchitectureGroupRecord {
	label := service.GroupID
	if separator := strings.LastIndexAny(label, ".:"); separator >= 0 && separator+1 < len(label) {
		label = label[separator+1:]
	}
	return WorkspaceArchitectureGroupRecord{
		ID:         service.GroupID,
		Label:      label,
		Source:     service.Source,
		Confidence: service.Confidence,
	}
}
