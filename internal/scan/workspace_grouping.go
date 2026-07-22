package scan

import (
	"path"
	"sort"
	"strings"
	"unicode"
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

type workspaceNamespaceEvidenceTrie struct {
	evidence           []workspaceNamespaceEvidence
	confidence         int
	terminal           []workspaceNamespaceEvidence
	terminalConfidence int
	children           map[string]*workspaceNamespaceEvidenceTrie
}

type workspaceProjectNamespaceTrie struct {
	projects []string
	terminal []string
	children map[string]*workspaceProjectNamespaceTrie
}

func BuildWorkspaceArchitectureLayout(registry WorkspaceRegistryRecord, namespaces []WorkspaceProjectNamespaceRecord, config WorkspaceDashboardConfig) WorkspaceArchitectureLayoutRecord {
	projects := append([]WorkspaceProjectRecord(nil), registry.Projects...)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})

	evidence := workspaceProductionNamespaceEvidence(projects, namespaces)
	namespacePrefixLengths := workspaceNamespacePrefixLengths(projects, evidence)
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

		service := workspaceInferredServiceLayout(project, evidence[project.Path], namespacePrefixLengths[project.Path])
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
	knownProjects := make(map[string]WorkspaceProjectRecord, len(projects))
	for _, project := range projects {
		knownProjects[project.Path] = project
	}
	multiPackageProjects := workspaceMultiPackageProjects(namespaces)
	candidates := make(map[string][]workspaceNamespaceEvidence)
	for _, namespace := range namespaces {
		project, known := knownProjects[namespace.Project]
		if !known || multiPackageProjects[namespace.Project] || !strings.EqualFold(strings.TrimSpace(namespace.Source), "production_package") {
			continue
		}
		segments := workspaceProjectNamespaceSegments(project, workspaceNamespaceSegments(namespace.Namespace))
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
		winningCandidates, segments, resolved := workspaceDominantNamespaceFamily(projectCandidates)
		if !resolved || len(segments) < 2 {
			continue
		}
		confidence := ""
		language := ""
		for _, candidate := range winningCandidates {
			confidence = strongerContextConfidence(confidence, candidate.record.Confidence)
			if language == "" || candidate.record.Language < language {
				language = candidate.record.Language
			}
		}
		evidence[project] = workspaceNamespaceEvidence{
			record: WorkspaceProjectNamespaceRecord{
				Project:    project,
				Namespace:  strings.Join(segments, "."),
				Language:   language,
				Source:     "production_package",
				Confidence: confidence,
			},
			segments: segments,
		}
	}
	return evidence
}

func workspaceMultiPackageProjects(namespaces []WorkspaceProjectNamespaceRecord) map[string]bool {
	units := map[string]map[string]bool{}
	for _, namespace := range namespaces {
		unit := strings.TrimSpace(namespace.PackageUnit)
		if unit == "" || !strings.EqualFold(strings.TrimSpace(namespace.Source), "production_package") {
			continue
		}
		if units[namespace.Project] == nil {
			units[namespace.Project] = map[string]bool{}
		}
		units[namespace.Project][unit] = true
	}
	multiPackage := map[string]bool{}
	for project, projectUnits := range units {
		multiPackage[project] = len(projectUnits) > 1
	}
	return multiPackage
}

func workspaceNamespaceSegments(namespace string) []string {
	return strings.FieldsFunc(strings.TrimSpace(namespace), func(r rune) bool {
		return r == '.' || r == '/' || r == '\\'
	})
}

func workspaceProjectNamespaceSegments(project WorkspaceProjectRecord, segments []string) []string {
	identifiers := make(map[string]bool, 3)
	for _, identifier := range []string{project.Name, project.Service, path.Base(strings.ReplaceAll(project.Path, "\\", "/"))} {
		if normalized := workspaceNamespaceIdentifier(identifier); normalized != "" {
			identifiers[normalized] = true
		}
	}
	for index, segment := range segments {
		if identifiers[workspaceNamespaceIdentifier(segment)] {
			return append([]string(nil), segments[:index+1]...)
		}
	}
	return append([]string(nil), segments...)
}

func workspaceDominantNamespaceFamily(candidates []workspaceNamespaceEvidence) ([]workspaceNamespaceEvidence, []string, bool) {
	trie := &workspaceNamespaceEvidenceTrie{children: map[string]*workspaceNamespaceEvidenceTrie{}}
	for _, candidate := range candidates {
		trie.insert(candidate)
	}
	return trie.dominant(nil)
}

func (trie *workspaceNamespaceEvidenceTrie) insert(candidate workspaceNamespaceEvidence) {
	confidence := contextConfidenceRank(candidate.record.Confidence)
	node := trie
	node.evidence = append(node.evidence, candidate)
	node.confidence += confidence
	for _, segment := range candidate.segments {
		child := node.children[segment]
		if child == nil {
			child = &workspaceNamespaceEvidenceTrie{children: map[string]*workspaceNamespaceEvidenceTrie{}}
			node.children[segment] = child
		}
		node = child
		node.evidence = append(node.evidence, candidate)
		node.confidence += confidence
	}
	node.terminal = append(node.terminal, candidate)
	node.terminalConfidence += confidence
}

func (trie *workspaceNamespaceEvidenceTrie) dominant(prefix []string) ([]workspaceNamespaceEvidence, []string, bool) {
	branchNames := make([]string, 0, len(trie.children))
	for branch := range trie.children {
		branchNames = append(branchNames, branch)
	}
	sort.Strings(branchNames)
	branchCount := len(branchNames)
	if len(trie.terminal) > 0 {
		branchCount++
	}
	if branchCount == 0 {
		return nil, nil, false
	}
	if branchCount == 1 {
		if len(trie.terminal) > 0 {
			return trie.terminal, append([]string(nil), prefix...), len(prefix) >= 2
		}
		branch := branchNames[0]
		return trie.children[branch].dominant(append(prefix, branch))
	}

	winner := ""
	bestCount := -1
	bestConfidence := -1
	tied := false
	updateWinner := func(branch string, count, confidence int) {
		switch {
		case count > bestCount || count == bestCount && confidence > bestConfidence:
			winner = branch
			bestCount = count
			bestConfidence = confidence
			tied = false
		case count == bestCount && confidence == bestConfidence:
			tied = true
		}
	}
	if len(trie.terminal) > 0 {
		updateWinner("", len(trie.terminal), trie.terminalConfidence)
	}
	for _, branch := range branchNames {
		child := trie.children[branch]
		updateWinner(branch, len(child.evidence), child.confidence)
	}
	if tied {
		if len(prefix) < 2 {
			return nil, nil, false
		}
		return trie.evidence, append([]string(nil), prefix...), true
	}
	if winner == "" {
		return trie.terminal, append([]string(nil), prefix...), len(prefix) >= 2
	}
	return trie.children[winner].dominant(append(prefix, winner))
}

func (trie *workspaceProjectNamespaceTrie) insert(project string, segments []string) {
	node := trie
	node.projects = append(node.projects, project)
	for _, segment := range segments {
		child := node.children[segment]
		if child == nil {
			child = &workspaceProjectNamespaceTrie{children: map[string]*workspaceProjectNamespaceTrie{}}
			node.children[segment] = child
		}
		node = child
		node.projects = append(node.projects, project)
	}
	node.terminal = append(node.terminal, project)
}

func (trie *workspaceProjectNamespaceTrie) assignPrefixLengths(prefix []string, projects map[string]WorkspaceProjectRecord, evidence map[string]workspaceNamespaceEvidence, prefixLengths map[string]int) {
	if len(trie.projects) == 0 {
		return
	}
	if len(trie.projects) == 1 {
		project := trie.projects[0]
		prefixLengths[project] = workspaceSingletonNamespacePrefixLength(projects[project], evidence[project])
		return
	}
	branchNames := make([]string, 0, len(trie.children))
	for branch := range trie.children {
		branchNames = append(branchNames, branch)
	}
	sort.Strings(branchNames)
	if len(prefix) < 2 {
		for _, branch := range branchNames {
			trie.children[branch].assignPrefixLengths(append(prefix, branch), projects, evidence, prefixLengths)
		}
		return
	}
	if len(trie.terminal) > 0 || len(branchNames) == 0 {
		for _, project := range trie.projects {
			prefixLengths[project] = len(prefix)
		}
		return
	}
	if len(branchNames) == 1 {
		branch := branchNames[0]
		trie.children[branch].assignPrefixLengths(append(prefix, branch), projects, evidence, prefixLengths)
		return
	}
	for _, branch := range branchNames {
		child := trie.children[branch]
		prefixLength := len(prefix) + 1
		for _, project := range child.projects {
			if len(evidence[project].segments) < prefixLength {
				prefixLengths[project] = len(evidence[project].segments)
				continue
			}
			prefixLengths[project] = prefixLength
		}
	}
}

func workspaceNamespacePrefixLengths(projects []WorkspaceProjectRecord, evidence map[string]workspaceNamespaceEvidence) map[string]int {
	projectByPath := make(map[string]WorkspaceProjectRecord, len(projects))
	trie := &workspaceProjectNamespaceTrie{children: map[string]*workspaceProjectNamespaceTrie{}}
	for _, project := range projects {
		projectByPath[project.Path] = project
		if projectEvidence, ok := evidence[project.Path]; ok {
			trie.insert(project.Path, projectEvidence.segments)
		}
	}
	prefixLengths := make(map[string]int, len(evidence))
	trie.assignPrefixLengths(nil, projectByPath, evidence, prefixLengths)
	return prefixLengths
}

func workspaceSingletonNamespacePrefixLength(project WorkspaceProjectRecord, evidence workspaceNamespaceEvidence) int {
	prefixLength := len(evidence.segments)
	if prefixLength < 2 {
		return 0
	}
	lastSegment := workspaceNamespaceIdentifier(evidence.segments[prefixLength-1])
	for _, identifier := range []string{project.Name, project.Service, path.Base(strings.ReplaceAll(project.Path, "\\", "/"))} {
		if lastSegment != "" && lastSegment == workspaceNamespaceIdentifier(identifier) {
			prefixLength--
			break
		}
	}
	if prefixLength < 2 {
		return 0
	}
	return prefixLength
}

func workspaceNamespaceIdentifier(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func workspaceInferredServiceLayout(project WorkspaceProjectRecord, evidence workspaceNamespaceEvidence, prefixLength int) WorkspaceArchitectureServiceLayoutRecord {
	if len(evidence.segments) > 0 {
		if prefixLength <= 0 {
			return workspaceFallbackServiceLayout(project)
		}
		if prefixLength > len(evidence.segments) {
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
