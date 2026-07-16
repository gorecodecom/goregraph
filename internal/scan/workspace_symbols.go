package scan

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

var workspaceSymbolRename = os.Rename

type workspaceSymbolLookup struct {
	byID                     map[string]CanonicalSymbolRecord
	byProjectLocalID         map[string]string
	javaByQualifiedName      map[string][]CanonicalSymbolRecord
	scriptByProjectModule    map[string][]CanonicalSymbolRecord
	scriptByWorkspacePackage map[string][]CanonicalSymbolRecord
	scriptPackageConditions  map[string]map[string]bool
}

// BuildWorkspaceSymbolProjection reconciles project-local declarations and
// references into canonical workspace symbol projections.
func BuildWorkspaceSymbolProjection(registry WorkspaceRegistryRecord, projects []workspaceIndexProject, generated string) (WorkspaceSymbolIndexRecord, WorkspaceSymbolUsageIndexRecord, error) {
	if err := validateWorkspaceSymbolProjectPaths(registry, projects); err != nil {
		return WorkspaceSymbolIndexRecord{}, WorkspaceSymbolUsageIndexRecord{}, err
	}
	symbolIndex := WorkspaceSymbolIndexRecord{
		SchemaVersion: SchemaVersion,
		Generated:     generated,
		Root:          registry.Root,
		Symbols:       []CanonicalSymbolRecord{},
		Coverage:      []SymbolCoverageRecord{},
	}
	usageIndex := WorkspaceSymbolUsageIndexRecord{
		SchemaVersion: SchemaVersion,
		Generated:     generated,
		Root:          registry.Root,
		Usages:        []CanonicalSymbolUsageRecord{},
		Coverage:      []SymbolCoverageRecord{},
	}

	lookup := workspaceSymbolLookup{
		byID:                     map[string]CanonicalSymbolRecord{},
		byProjectLocalID:         map[string]string{},
		javaByQualifiedName:      map[string][]CanonicalSymbolRecord{},
		scriptByProjectModule:    map[string][]CanonicalSymbolRecord{},
		scriptByWorkspacePackage: map[string][]CanonicalSymbolRecord{},
		scriptPackageConditions:  map[string]map[string]bool{},
	}
	canonicalIDs := map[string]bool{}
	for _, project := range projects {
		indexWorkspaceScriptPackageConditions(project.packages, lookup)
		for _, declaration := range project.symbols {
			if !isWorkspaceCanonicalDeclaration(declaration) {
				continue
			}
			canonical := canonicalWorkspaceSymbol(project, declaration)
			localKey := project.record.Path + "\x00" + declaration.ID
			if _, exists := lookup.byProjectLocalID[localKey]; exists {
				return WorkspaceSymbolIndexRecord{}, WorkspaceSymbolUsageIndexRecord{}, fmt.Errorf("duplicate workspace symbol declaration %q", canonical.ID)
			}
			if canonicalIDs[canonical.ID] {
				return WorkspaceSymbolIndexRecord{}, WorkspaceSymbolUsageIndexRecord{}, fmt.Errorf("duplicate workspace symbol declaration %q", canonical.ID)
			}
			canonicalIDs[canonical.ID] = true
			lookup.byProjectLocalID[localKey] = canonical.ID
			lookup.byID[canonical.ID] = canonical
			symbolIndex.Symbols = append(symbolIndex.Symbols, canonical)
			if declaration.Language == "java" && declaration.QualifiedName != "" {
				lookup.javaByQualifiedName[declaration.QualifiedName] = append(lookup.javaByQualifiedName[declaration.QualifiedName], canonical)
			}
			if isScriptLanguage(declaration.Language) {
				if declaration.Module != "" && declaration.ExportName != "" {
					key := scriptProjectModuleKey(project.record.Path, declaration.Module, declaration.ExportName)
					lookup.scriptByProjectModule[key] = append(lookup.scriptByProjectModule[key], canonical)
				}
				if declaration.WorkspacePackage != "" && declaration.ExportName != "" {
					indexWorkspaceScriptPackageExport(
						project,
						declaration.WorkspacePackage,
						declaration.File,
						declaration.ExportName,
						canonical,
						lookup,
					)
				}
			}
		}
	}
	for _, project := range projects {
		if err := validateWorkspaceProjectReferenceIDs(project, workspaceProjectSymbolReferences(project), lookup); err != nil {
			return WorkspaceSymbolIndexRecord{}, WorkspaceSymbolUsageIndexRecord{}, err
		}
	}
	for _, project := range projects {
		indexWorkspaceScriptExportRelations(project, lookup)
	}

	sort.Slice(symbolIndex.Symbols, func(i, j int) bool {
		return symbolIndex.Symbols[i].ID < symbolIndex.Symbols[j].ID
	})
	sortWorkspaceSymbolLookup(lookup.javaByQualifiedName)
	sortWorkspaceSymbolLookup(lookup.scriptByProjectModule)
	sortWorkspaceSymbolLookup(lookup.scriptByWorkspacePackage)

	for _, project := range projects {
		for _, reference := range workspaceProjectSymbolReferences(project) {
			if !isWorkspaceSymbolReference(reference) {
				continue
			}
			consumerID := lookup.byProjectLocalID[project.record.Path+"\x00"+reference.FromSymbolID]
			candidates, dependencyEvidence := resolveWorkspaceSymbolCandidates(project, reference, consumerID, lookup)
			usageIndex.Usages = append(
				usageIndex.Usages,
				buildWorkspaceSymbolUsages(project, consumerID, reference, candidates, dependencyEvidence)...,
			)
		}
	}
	usageIndex.Usages = mergeWorkspaceSymbolUsageRecords(usageIndex.Usages)
	coverage := buildWorkspaceSymbolCoverage(registry, projects)
	symbolIndex.Coverage = append([]SymbolCoverageRecord(nil), coverage...)
	usageIndex.Coverage = append([]SymbolCoverageRecord(nil), coverage...)
	return symbolIndex, usageIndex, nil
}

func validateWorkspaceSymbolProjectPaths(registry WorkspaceRegistryRecord, projects []workspaceIndexProject) error {
	paths := make([]string, 0, len(registry.Projects)+len(projects))
	for _, project := range registry.Projects {
		paths = append(paths, project.Path)
	}
	for _, project := range projects {
		paths = append(paths, project.record.Path)
	}
	for _, projectPath := range paths {
		if strings.Contains(projectPath, "#") {
			return fmt.Errorf(
				"workspace project path %q contains evidence namespace separator #",
				projectPath,
			)
		}
	}
	return nil
}

func validateWorkspaceProjectReferenceIDs(project workspaceIndexProject, references []RichRelationRecord, lookup workspaceSymbolLookup) error {
	for _, reference := range references {
		referenceID := reference.ID
		if referenceID == "" {
			referenceID = "<missing>"
		}
		for _, local := range []struct {
			field string
			value string
		}{
			{field: "from_symbol_id", value: reference.FromSymbolID},
			{field: "to_symbol_id", value: reference.ToSymbolID},
		} {
			if local.value == "" {
				continue
			}
			if lookup.byProjectLocalID[project.record.Path+"\x00"+local.value] == "" {
				return fmt.Errorf(
					"workspace project %q reference %q has unknown %s %q",
					project.record.Path,
					referenceID,
					local.field,
					local.value,
				)
			}
		}
	}
	return nil
}

func validateWorkspaceSymbolProjectionEvidence(
	symbols WorkspaceSymbolIndexRecord,
	usages WorkspaceSymbolUsageIndexRecord,
	projects []workspaceIndexProject,
) error {
	knownProjects := map[string]bool{}
	knownEvidence := map[string]string{}
	for _, project := range projects {
		projectPath := project.record.Path
		knownProjects[projectPath] = true
		expectedOwner := project.record.Name
		if expectedOwner == "" {
			expectedOwner = filepath.Base(filepath.FromSlash(projectPath))
		}
		localIDs := map[string]bool{}
		for _, evidence := range project.evidence {
			if evidence.ID == "" || strings.Contains(evidence.ID, "#") {
				return fmt.Errorf(
					"workspace project %q contains malformed evidence ID %q",
					projectPath,
					evidence.ID,
				)
			}
			if localIDs[evidence.ID] {
				return fmt.Errorf(
					"workspace project %q contains duplicate evidence ID %q",
					projectPath,
					evidence.ID,
				)
			}
			if evidence.Project == "" || evidence.Project != expectedOwner {
				return fmt.Errorf(
					"workspace evidence %q declares owner %q, want %q for project %q",
					evidence.ID,
					evidence.Project,
					expectedOwner,
					projectPath,
				)
			}
			if strings.TrimSpace(evidence.File) == "" {
				return fmt.Errorf(
					"workspace project %q evidence %q has no source file",
					projectPath,
					evidence.ID,
				)
			}
			localIDs[evidence.ID] = true
			knownEvidence[WorkspaceEvidenceID(projectPath, evidence.ID)] = projectPath
		}
	}

	for _, symbol := range symbols.Symbols {
		if err := validateWorkspaceProjectionEvidenceIDs(
			symbol.ID,
			symbol.Project,
			true,
			symbol.EvidenceIDs,
			knownProjects,
			knownEvidence,
		); err != nil {
			return err
		}
	}
	for _, usage := range usages.Usages {
		apiUsage := usage.Transport == "http" ||
			usage.Category == SymbolUsageReachedThroughAPI ||
			len(usage.APIPath) > 0
		if err := validateWorkspaceProjectionEvidenceIDs(
			usage.ID,
			usage.ConsumerProject,
			!apiUsage,
			usage.EvidenceIDs,
			knownProjects,
			knownEvidence,
		); err != nil {
			return err
		}
		for _, step := range usage.APIPath {
			if err := validateWorkspaceProjectionEvidenceIDs(
				usage.ID,
				step.Project,
				true,
				step.EvidenceIDs,
				knownProjects,
				knownEvidence,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateWorkspaceProjectionEvidenceIDs(
	offendingID string,
	ownerProject string,
	requireOwner bool,
	ids []string,
	knownProjects map[string]bool,
	knownEvidence map[string]string,
) error {
	previous := ""
	for _, id := range ids {
		projectPath, localID, namespaced := strings.Cut(id, "#")
		if !namespaced ||
			projectPath == "" ||
			localID == "" ||
			strings.Contains(localID, "#") {
			return fmt.Errorf(
				"workspace projection record %q contains malformed evidence ID %q",
				offendingID,
				id,
			)
		}
		if previous != "" && id <= previous {
			return fmt.Errorf(
				"workspace projection record %q contains duplicate or unsorted evidence ID %q",
				offendingID,
				id,
			)
		}
		if !knownProjects[projectPath] {
			return fmt.Errorf(
				"workspace projection record %q evidence %q references unknown project %q",
				offendingID,
				id,
				projectPath,
			)
		}
		if requireOwner && projectPath != ownerProject {
			return fmt.Errorf(
				"workspace projection record %q evidence %q belongs to project %q, not %q",
				offendingID,
				id,
				projectPath,
				ownerProject,
			)
		}
		if knownEvidence[id] != projectPath {
			return fmt.Errorf(
				"workspace projection record %q references unknown evidence %q",
				offendingID,
				id,
			)
		}
		previous = id
	}
	return nil
}

func writeWorkspaceSymbolProjectionPair(out string, symbols WorkspaceSymbolIndexRecord, usages WorkspaceSymbolUsageIndexRecord) error {
	if err := validateWorkspaceSymbolProjectionPair(symbols, usages); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(out, ".symbol-projection-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	const symbolFile = "symbol-index.json"
	const usageFile = "symbol-usages.json"
	stagedSymbol := filepath.Join(staging, symbolFile)
	stagedUsage := filepath.Join(staging, usageFile)
	if err := writeJSON(stagedSymbol, symbols); err != nil {
		return err
	}
	if err := writeJSON(stagedUsage, usages); err != nil {
		return err
	}

	liveSymbol := filepath.Join(out, symbolFile)
	liveUsage := filepath.Join(out, usageFile)
	backupSymbol := filepath.Join(out, "."+symbolFile+".backup-"+filepath.Base(staging))
	backupUsage := filepath.Join(out, "."+usageFile+".backup-"+filepath.Base(staging))
	symbolBackedUp := false
	usageBackedUp := false
	if workspaceFileExists(liveSymbol) {
		if err := workspaceSymbolRename(liveSymbol, backupSymbol); err != nil {
			return err
		}
		symbolBackedUp = true
	}
	if workspaceFileExists(liveUsage) {
		if err := workspaceSymbolRename(liveUsage, backupUsage); err != nil {
			restoreErr := restoreWorkspaceSymbolBackup(backupSymbol, liveSymbol, symbolBackedUp)
			return combineWorkspaceSymbolErrors(err, restoreErr)
		}
		usageBackedUp = true
	}

	if err := workspaceSymbolRename(stagedSymbol, liveSymbol); err != nil {
		restoreSymbolErr := restoreWorkspaceSymbolBackup(backupSymbol, liveSymbol, symbolBackedUp)
		restoreUsageErr := restoreWorkspaceSymbolBackup(backupUsage, liveUsage, usageBackedUp)
		return combineWorkspaceSymbolErrors(err, restoreSymbolErr, restoreUsageErr)
	}
	if err := workspaceSymbolRename(stagedUsage, liveUsage); err != nil {
		removeErr := os.Remove(liveSymbol)
		if os.IsNotExist(removeErr) {
			removeErr = nil
		}
		restoreSymbolErr := restoreWorkspaceSymbolBackup(backupSymbol, liveSymbol, symbolBackedUp)
		restoreUsageErr := restoreWorkspaceSymbolBackup(backupUsage, liveUsage, usageBackedUp)
		return combineWorkspaceSymbolErrors(err, removeErr, restoreSymbolErr, restoreUsageErr)
	}

	var cleanupErrors []error
	if symbolBackedUp {
		cleanupErrors = append(cleanupErrors, os.Remove(backupSymbol))
	}
	if usageBackedUp {
		cleanupErrors = append(cleanupErrors, os.Remove(backupUsage))
	}
	return combineWorkspaceSymbolErrors(cleanupErrors...)
}

func validateWorkspaceSymbolProjectionPair(symbols WorkspaceSymbolIndexRecord, usages WorkspaceSymbolUsageIndexRecord) error {
	if symbols.SchemaVersion != SchemaVersion {
		return fmt.Errorf("symbol index schema version %d does not match %d", symbols.SchemaVersion, SchemaVersion)
	}
	if usages.SchemaVersion != SchemaVersion {
		return fmt.Errorf("symbol usage schema version %d does not match %d", usages.SchemaVersion, SchemaVersion)
	}
	symbolIDs := map[string]bool{}
	for _, symbol := range symbols.Symbols {
		if symbol.ID == "" {
			return fmt.Errorf("workspace symbol has empty ID")
		}
		if symbolIDs[symbol.ID] {
			return fmt.Errorf("duplicate workspace symbol ID %q", symbol.ID)
		}
		symbolIDs[symbol.ID] = true
		if err := symbol.Confidence.Validate(); err != nil {
			return fmt.Errorf("workspace symbol %q: %w", symbol.ID, err)
		}
		if err := symbol.Coverage.Validate(); err != nil {
			return fmt.Errorf("workspace symbol %q: %w", symbol.ID, err)
		}
	}
	usageIDs := map[string]bool{}
	for _, usage := range usages.Usages {
		if usage.ID == "" {
			return fmt.Errorf("workspace symbol usage has empty ID")
		}
		if usageIDs[usage.ID] {
			return fmt.Errorf("duplicate workspace symbol usage ID %q", usage.ID)
		}
		usageIDs[usage.ID] = true
		if err := validateWorkspaceSymbolUsage(usage, symbolIDs); err != nil {
			return fmt.Errorf("workspace symbol usage %q: %w", usage.ID, err)
		}
	}
	for _, record := range append(append([]SymbolCoverageRecord(nil), symbols.Coverage...), usages.Coverage...) {
		if err := record.Coverage.Validate(); err != nil {
			return fmt.Errorf("workspace symbol coverage %s/%s/%s: %w", record.Project, record.Language, record.Capability, err)
		}
	}
	return nil
}

func validateWorkspaceSymbolUsage(usage CanonicalSymbolUsageRecord, symbols map[string]bool) error {
	switch usage.Resolution {
	case SymbolResolutionExact:
		if usage.ProviderSymbolID == "" {
			return fmt.Errorf("EXACT usage must have one provider")
		}
		if usage.Category != SymbolUsageDirectReference && usage.Category != SymbolUsageReachedThroughAPI {
			return fmt.Errorf("EXACT usage must be a direct reference or API reachability record")
		}
		if !symbols[usage.ProviderSymbolID] {
			return fmt.Errorf("EXACT provider %q is not in symbol index", usage.ProviderSymbolID)
		}
		if usage.Category == SymbolUsageReachedThroughAPI {
			if usage.Transport != "http" {
				return fmt.Errorf("API reachability transport must be http")
			}
			if len(usage.APIPath) == 0 {
				return fmt.Errorf("API reachability path is empty")
			}
			for position, step := range usage.APIPath {
				if step.Position != position {
					return fmt.Errorf("API reachability path position %d is %d", position, step.Position)
				}
			}
			selected := usage.APIPath[len(usage.APIPath)-1]
			if selected.Kind != "selected_symbol" || selected.SymbolID != usage.ProviderSymbolID {
				return fmt.Errorf("API reachability path does not end at provider %q", usage.ProviderSymbolID)
			}
		}
	case SymbolResolutionAmbiguous:
		flowAmbiguous := usage.Transport == "http" &&
			workspaceSymbolUsageHasLimitation(usage, "feature_flow_join_ambiguous")
		if usage.Category != SymbolUsageAmbiguous ||
			usage.ProviderSymbolID == "" ||
			flowAmbiguous && (len(usage.CandidateSymbolIDs) < 1 || len(usage.CandidatePathIDs) < 2) ||
			!flowAmbiguous && len(usage.CandidateSymbolIDs) < 2 {
			return fmt.Errorf("AMBIGUOUS usage must disclose a selected candidate and the complete candidate set")
		}
		selectedDisclosed := false
		previous := ""
		for _, candidateID := range usage.CandidateSymbolIDs {
			if candidateID == previous {
				return fmt.Errorf("AMBIGUOUS candidate set contains duplicate %q", candidateID)
			}
			if previous != "" && candidateID < previous {
				return fmt.Errorf("AMBIGUOUS candidate set is not sorted")
			}
			if !symbols[candidateID] {
				return fmt.Errorf("AMBIGUOUS candidate %q is not in symbol index", candidateID)
			}
			if candidateID == usage.ProviderSymbolID ||
				flowAmbiguous && candidateID == usage.ConsumerSymbolID {
				selectedDisclosed = true
			}
			previous = candidateID
		}
		if !selectedDisclosed {
			return fmt.Errorf("AMBIGUOUS selected provider/origin is not in the candidate set")
		}
		if !symbols[usage.ProviderSymbolID] {
			return fmt.Errorf("AMBIGUOUS provider %q is not in symbol index", usage.ProviderSymbolID)
		}
		previous = ""
		for _, candidateID := range usage.CandidatePathIDs {
			if candidateID == "" || candidateID == previous {
				return fmt.Errorf("AMBIGUOUS path candidate set contains invalid or duplicate %q", candidateID)
			}
			if previous != "" && candidateID < previous {
				return fmt.Errorf("AMBIGUOUS path candidate set is not sorted")
			}
			previous = candidateID
		}
	case SymbolResolutionUnresolved:
		if usage.Category != SymbolUsageUnresolved || usage.ProviderSymbolID != "" {
			return fmt.Errorf("UNRESOLVED usage must be unscoped")
		}
	default:
		return fmt.Errorf("invalid symbol resolution %q", usage.Resolution)
	}
	return nil
}

func workspaceSymbolUsageHasLimitation(usage CanonicalSymbolUsageRecord, want string) bool {
	for _, limitation := range usage.Limitations {
		if limitation == want {
			return true
		}
	}
	return false
}

func finalizeWorkspaceSymbolUsageProjection(
	symbols WorkspaceSymbolIndexRecord,
	usages WorkspaceSymbolUsageIndexRecord,
	matches []WorkspaceContractMatchRecord,
	flows []WorkspaceFeatureFlowRecord,
	traces WorkspaceEndpointTraceIndexRecord,
	projectGroups ...[]workspaceIndexProject,
) (WorkspaceSymbolUsageIndexRecord, error) {
	apiUsages := BuildWorkspaceSymbolAPIUsages(symbols, matches, flows, traces)
	usages.Usages = mergeWorkspaceSymbolUsageRecords(usages.Usages, apiUsages)
	if len(projectGroups) > 0 {
		usages.Coverage = mergeWorkspaceSymbolCoverageRecords(
			usages.Coverage,
			BuildWorkspaceSymbolAPIUsageCoverage(symbols, matches, flows, projectGroups[0]),
		)
	}
	if err := validateWorkspaceSymbolProjectionPair(symbols, usages); err != nil {
		return WorkspaceSymbolUsageIndexRecord{}, err
	}
	return usages, nil
}

func mergeWorkspaceSymbolCoverageRecords(groups ...[]SymbolCoverageRecord) []SymbolCoverageRecord {
	byKey := map[string]SymbolCoverageRecord{}
	for _, group := range groups {
		for _, record := range group {
			key := strings.Join([]string{record.Project, record.Language, record.Capability}, "\x00")
			byKey[key] = record
		}
	}
	records := make([]SymbolCoverageRecord, 0, len(byKey))
	for _, record := range byKey {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Project != records[j].Project {
			return records[i].Project < records[j].Project
		}
		if records[i].Language != records[j].Language {
			return records[i].Language < records[j].Language
		}
		return records[i].Capability < records[j].Capability
	})
	return records
}

func restoreWorkspaceSymbolBackup(backup, live string, backedUp bool) error {
	if !backedUp {
		return nil
	}
	if workspaceFileExists(live) {
		if err := os.Remove(live); err != nil {
			return err
		}
	}
	return workspaceSymbolRename(backup, live)
}

func combineWorkspaceSymbolErrors(values ...error) error {
	var messages []string
	for _, value := range values {
		if value != nil {
			messages = append(messages, value.Error())
		}
	}
	if len(messages) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

func buildWorkspaceSymbolCoverage(registry WorkspaceRegistryRecord, projects []workspaceIndexProject) []SymbolCoverageRecord {
	projectByPath := map[string]workspaceIndexProject{}
	for _, project := range projects {
		projectByPath[project.record.Path] = project
	}
	registryProjects := append([]WorkspaceProjectRecord(nil), registry.Projects...)
	sort.Slice(registryProjects, func(i, j int) bool {
		return registryProjects[i].Path < registryProjects[j].Path
	})
	var coverage []SymbolCoverageRecord
	for _, registryProject := range registryProjects {
		project, loaded := projectByPath[registryProject.Path]
		if !loaded {
			coverage = append(
				coverage,
				SymbolCoverageRecord{
					Project: registryProject.Path, Language: "unknown", Capability: "declarations",
					Coverage: CoverageUnavailable, Reason: "project is present in the workspace registry but its symbol index is not loaded",
				},
				SymbolCoverageRecord{
					Project: registryProject.Path, Language: "unknown", Capability: "direct_usages",
					Coverage: CoverageUnavailable, Reason: "project is present in the workspace registry but its symbol index is not loaded",
				},
			)
			continue
		}
		languages := workspaceSymbolLanguages(project)
		if len(languages) == 0 {
			languages = []string{"unknown"}
		}
		for _, language := range languages {
			if !isWorkspaceSymbolLanguageSupported(language) {
				if language == "unknown" &&
					(workspaceSymbolCapabilityHasIssues(project, language, "declarations") ||
						workspaceSymbolCapabilityHasIssues(project, language, "direct_usages")) {
					coverage = append(
						coverage,
						buildWorkspaceSymbolCoverageRecord(project, language, "declarations"),
						buildWorkspaceSymbolCoverageRecord(project, language, "direct_usages"),
					)
					continue
				}
				coverage = append(
					coverage,
					SymbolCoverageRecord{
						Project: project.record.Path, Language: language, Capability: "declarations",
						Coverage: CoverageUnavailable, Reason: "workspace symbol reconciliation is not supported for this language",
					},
					SymbolCoverageRecord{
						Project: project.record.Path, Language: language, Capability: "direct_usages",
						Coverage: CoverageUnavailable, Reason: "workspace symbol reconciliation is not supported for this language",
					},
				)
				continue
			}
			coverage = append(
				coverage,
				buildWorkspaceSymbolCoverageRecord(project, language, "declarations"),
				buildWorkspaceSymbolCoverageRecord(project, language, "direct_usages"),
			)
		}
	}
	sort.Slice(coverage, func(i, j int) bool {
		if coverage[i].Project != coverage[j].Project {
			return coverage[i].Project < coverage[j].Project
		}
		if coverage[i].Language != coverage[j].Language {
			return coverage[i].Language < coverage[j].Language
		}
		return coverage[i].Capability < coverage[j].Capability
	})
	return coverage
}

func buildWorkspaceSymbolCoverageRecord(project workspaceIndexProject, language, capability string) SymbolCoverageRecord {
	relevant := workspaceSymbolCapabilityFiles(language, capability)
	failures := workspaceSymbolFactFailures(project.loadFailures, relevant)
	missing := workspaceSymbolMissingFacts(project.missingFacts, relevant)
	record := SymbolCoverageRecord{
		Project:    project.record.Path,
		Language:   language,
		Capability: capability,
	}
	switch capability {
	case "declarations":
		record.Coverage, record.Reason, record.Limitations = workspaceDeclarationCoverage(project, language)
	case "direct_usages":
		record.Coverage = CoveragePartial
		record.Reason = "static direct-usage resolution cannot observe every runtime or generated binding"
		record.Limitations = workspaceSymbolLanguageLimitations(language)
	}
	if len(failures) > 0 {
		record.Coverage = CoverageFailed
		record.Reason = "project symbol facts could not be read: " + strings.Join(failures, "; ")
		return record
	}
	if len(missing) > 0 {
		if record.Coverage == CoverageComplete {
			record.Coverage = CoveragePartial
		}
		record.Reason += "; optional symbol inputs are missing: " + strings.Join(missing, ", ")
	}
	return record
}

func workspaceDeclarationCoverage(project workspaceIndexProject, language string) (Coverage, string, []string) {
	coverage := CoverageComplete
	var sources []string
	var limitations []string
	for _, capability := range project.capabilities {
		if capability.ID != CapabilitySymbols || capability.Language != language {
			continue
		}
		coverage = lowerWorkspaceCoverage(coverage, capability.Coverage)
		if capability.Coverage != CoverageComplete {
			reason := capability.Reason
			if reason == "" {
				reason = capability.StatusReason
			}
			source := fmt.Sprintf("symbols capability is %s", capability.Coverage)
			if reason != "" {
				source += ": " + reason
			}
			sources = append(sources, source)
		}
	}
	for _, symbol := range project.symbols {
		if symbol.Language != language || symbol.Coverage == "" {
			continue
		}
		coverage = lowerWorkspaceCoverage(coverage, symbol.Coverage)
		if symbol.Coverage != CoverageComplete {
			sources = append(
				sources,
				fmt.Sprintf("symbol %q coverage is %s", symbol.QualifiedName, symbol.Coverage),
			)
			limitations = append(limitations, symbol.Limitations...)
		}
	}
	if len(sources) == 0 {
		return coverage, "indexed declarations are projected with canonical workspace identities", nil
	}
	return coverage, "declaration coverage reflects project symbol facts: " + strings.Join(sources, "; "), sortedUniqueStrings(limitations)
}

func lowerWorkspaceCoverage(current, candidate Coverage) Coverage {
	if candidate == "" || coverageRank(candidate) >= coverageRank(current) {
		return current
	}
	return candidate
}

func workspaceSymbolCapabilityHasIssues(project workspaceIndexProject, language, capability string) bool {
	relevant := workspaceSymbolCapabilityFiles(language, capability)
	return len(workspaceSymbolFactFailures(project.loadFailures, relevant)) > 0 ||
		len(workspaceSymbolMissingFacts(project.missingFacts, relevant)) > 0
}

func workspaceSymbolCapabilityFiles(language, capability string) []string {
	if capability == "declarations" {
		return []string{"capabilities.json", "symbols-full.json"}
	}
	files := []string{"callgraph.json", "evidence.json", "relations-full.json", "symbols-full.json"}
	switch language {
	case "java":
		files = append(files, "maven-graph.json")
	case "javascript", "typescript":
		files = append(files, "package-graph.json")
	default:
		files = append(files, "maven-graph.json", "package-graph.json")
	}
	sort.Strings(files)
	return files
}

func workspaceSymbolFactFailures(failures, relevant []string) []string {
	relevantSet := workspaceSymbolStringSet(relevant)
	var result []string
	for _, failure := range failures {
		for filename := range relevantSet {
			if strings.Contains(failure, "/"+filename+":") {
				result = append(result, failure)
				break
			}
		}
	}
	return sortedStrings(result)
}

func workspaceSymbolMissingFacts(missing, relevant []string) []string {
	relevantSet := workspaceSymbolStringSet(relevant)
	var result []string
	for _, filename := range missing {
		if relevantSet[filename] {
			result = append(result, filename)
		}
	}
	return sortedStrings(result)
}

func workspaceSymbolStringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func workspaceSymbolLanguages(project workspaceIndexProject) []string {
	languages := map[string]bool{}
	for _, symbol := range project.symbols {
		if symbol.Language != "" {
			languages[symbol.Language] = true
		}
	}
	for _, relation := range project.relations {
		if relation.Language != "" {
			languages[relation.Language] = true
		}
	}
	for _, capability := range project.capabilities {
		if capability.ID == CapabilitySymbols && capability.Language != "" {
			languages[capability.Language] = true
		}
	}
	result := make([]string, 0, len(languages))
	for language := range languages {
		result = append(result, language)
	}
	sort.Strings(result)
	return result
}

func isWorkspaceSymbolLanguageSupported(language string) bool {
	return language == "java" || isScriptLanguage(language)
}

func workspaceSymbolLanguageLimitations(language string) []string {
	switch language {
	case "java":
		return []string{
			"dependency_injection",
			"generated_code",
			"reflection",
			"runtime_loading",
			"runtime_proxy",
			"unindexed_dependency_artifact",
		}
	case "javascript", "typescript":
		return []string{
			"bundler_only_alias",
			"computed_property",
			"dynamic_import",
			"generated_code",
			"unindexed_workspace_package",
		}
	default:
		return nil
	}
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func resolveWorkspaceSymbolCandidates(project workspaceIndexProject, reference RichRelationRecord, consumerID string, lookup workspaceSymbolLookup) ([]CanonicalSymbolRecord, []string) {
	if reference.NonPromotable {
		return nil, nil
	}
	if reference.ToSymbolID != "" {
		if canonicalID := lookup.byProjectLocalID[project.record.Path+"\x00"+reference.ToSymbolID]; canonicalID != "" {
			return []CanonicalSymbolRecord{lookup.byID[canonicalID]}, nil
		}
	}
	switch {
	case reference.Language == "java" && reference.TargetQualifiedName != "":
		candidates := append([]CanonicalSymbolRecord(nil), lookup.javaByQualifiedName[reference.TargetQualifiedName]...)
		if reference.Resolution == SymbolResolutionAmbiguous {
			if len(candidates) > 1 {
				return candidates, nil
			}
			return nil, nil
		}
		if len(candidates) == 0 {
			return nil, nil
		}
		return filterJavaWorkspaceCandidates(project, lookup.byID[consumerID], reference.From, candidates, reference.DependencyEvidence)
	case isScriptLanguage(reference.Language):
		if reference.TargetModule == "" || reference.TargetExport == "" {
			return nil, nil
		}
		localKey := scriptProjectModuleKey(project.record.Path, reference.TargetModule, reference.TargetExport)
		if candidates := lookup.scriptByProjectModule[localKey]; len(candidates) > 0 {
			if reference.Resolution == SymbolResolutionAmbiguous && len(candidates) < 2 {
				return nil, nil
			}
			return append([]CanonicalSymbolRecord(nil), candidates...), nil
		}
		candidates := workspaceScriptPackageCandidates(
			lookup,
			reference.TargetModule,
			reference.TargetExport,
			reference.Type,
		)
		if len(candidates) == 0 {
			return nil, nil
		}
		if reference.Resolution == SymbolResolutionAmbiguous {
			if len(candidates) > 1 {
				return candidates, nil
			}
			return nil, nil
		}
		return filterScriptWorkspaceCandidates(project, lookup.byID[consumerID], reference.From, candidates)
	default:
		return nil, nil
	}
}

func workspaceProjectSymbolReferences(project workspaceIndexProject) []RichRelationRecord {
	references := append([]RichRelationRecord(nil), project.relations...)
	for _, edge := range project.callGraph.Edges {
		if edge.TargetQualifiedName == "" && edge.ToSymbolID == "" {
			continue
		}
		references = append(references, RichRelationRecord{
			ID:                  edge.ID,
			From:                edge.SourceFile,
			Type:                edge.Type,
			Language:            "java",
			Analyzer:            "callgraph",
			Line:                edge.Line,
			Confidence:          edge.Confidence,
			ConfidenceScore:     edge.ConfidenceScore,
			EvidenceIDs:         append([]string(nil), edge.EvidenceIDs...),
			FromSymbolID:        edge.FromSymbolID,
			ToSymbolID:          edge.ToSymbolID,
			TargetQualifiedName: edge.TargetQualifiedName,
			Resolution:          edge.Resolution,
			CandidateSymbolIDs:  append([]string(nil), edge.CandidateSymbolIDs...),
			Reason:              edge.Reason,
		})
	}
	return references
}

func isWorkspaceSymbolReference(reference RichRelationRecord) bool {
	if reference.ToSymbolID != "" {
		return true
	}
	if reference.Language == "java" {
		return reference.TargetQualifiedName != ""
	}
	if isScriptLanguage(reference.Language) {
		return reference.TargetModule != "" && reference.TargetExport != ""
	}
	return false
}

func dedupeWorkspaceSymbolUsages(usages []CanonicalSymbolUsageRecord) []CanonicalSymbolUsageRecord {
	byID := make(map[string]CanonicalSymbolUsageRecord, len(usages))
	for _, usage := range usages {
		if _, exists := byID[usage.ID]; !exists {
			byID[usage.ID] = usage
		}
	}
	result := make([]CanonicalSymbolUsageRecord, 0, len(byID))
	for _, usage := range byID {
		result = append(result, usage)
	}
	return result
}

func mergeWorkspaceSymbolUsageRecords(groups ...[]CanonicalSymbolUsageRecord) []CanonicalSymbolUsageRecord {
	var usages []CanonicalSymbolUsageRecord
	for _, group := range groups {
		usages = append(usages, group...)
	}
	usages = dedupeWorkspaceSymbolUsages(usages)
	sort.Slice(usages, func(i, j int) bool {
		return usages[i].ID < usages[j].ID
	})
	return usages
}

func filterJavaWorkspaceCandidates(project workspaceIndexProject, consumer CanonicalSymbolRecord, sourceFile string, candidates []CanonicalSymbolRecord, existingEvidence []string) ([]CanonicalSymbolRecord, []string) {
	dependencyTargets := map[string]string{}
	prefix := "maven:"
	if project.record.BuildSystem == "gradle" {
		prefix = "gradle:"
	}
	consumerArtifact := consumer.Artifact
	if consumerArtifact == "" {
		consumerArtifact = inferWorkspaceMavenConsumer(project.maven, sourceFile)
	}
	for _, edge := range project.maven.Edges {
		if edge.From != consumerArtifact || edge.To == "" {
			continue
		}
		dependencyTargets[edge.To] = prefix + edge.From + " -> " + edge.To
	}
	for _, evidence := range existingEvidence {
		if !strings.HasPrefix(evidence, "maven:") && !strings.HasPrefix(evidence, "gradle:") {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(evidence, "maven:"), "gradle:"), " -> ", 2)
		if len(parts) == 2 {
			dependencyTargets[parts[1]] = evidence
		}
	}
	var filtered []CanonicalSymbolRecord
	var evidence []string
	for _, candidate := range candidates {
		if candidate.Project == project.record.Path {
			filtered = append(filtered, candidate)
			continue
		}
		if dependency := dependencyTargets[candidate.Artifact]; dependency != "" {
			filtered = append(filtered, candidate)
			evidence = append(evidence, dependency)
		}
	}
	if len(filtered) == 0 && len(dependencyTargets) == 0 && len(candidates) > 1 {
		return candidates, nil
	}
	sort.Strings(evidence)
	return filtered, dedupeStrings(evidence)
}

func filterScriptWorkspaceCandidates(project workspaceIndexProject, consumer CanonicalSymbolRecord, sourceFile string, candidates []CanonicalSymbolRecord) ([]CanonicalSymbolRecord, []string) {
	dependencyTargets := map[string]string{}
	consumerPackage := consumer.WorkspacePackage
	if consumerPackage == "" {
		consumerPackage = inferWorkspaceNodeConsumer(project.packages, sourceFile)
	}
	for _, edge := range project.packages.Edges {
		if edge.From != consumerPackage || edge.To == "" {
			continue
		}
		dependencyTargets[edge.To] = "node:" + edge.From + " -> " + edge.To
	}
	var filtered []CanonicalSymbolRecord
	var evidence []string
	for _, candidate := range candidates {
		if dependency := dependencyTargets[candidate.WorkspacePackage]; dependency != "" {
			filtered = append(filtered, candidate)
			evidence = append(evidence, dependency)
		}
	}
	sort.Strings(evidence)
	return filtered, dedupeStrings(evidence)
}

func inferWorkspaceMavenConsumer(graph MavenGraphRecord, sourceFile string) string {
	if module := closestWorkspaceMavenModule(graph.Nodes, sourceFile); module != "" {
		return module
	}
	from := map[string]bool{}
	for _, edge := range graph.Edges {
		if edge.From != "" {
			from[edge.From] = true
		}
	}
	return oneWorkspaceIdentity(from)
}

func closestWorkspaceMavenModule(nodes []MavenNodeRecord, sourceFile string) string {
	bestID := ""
	bestLength := -1
	for _, node := range nodes {
		if node.Kind != "module" || node.ID == "" {
			continue
		}
		dir := workspaceDescriptorDir(node.Path)
		if dir != "" && !workspacePathContains(dir, sourceFile) {
			continue
		}
		if len(dir) > bestLength {
			bestID = node.ID
			bestLength = len(dir)
		} else if len(dir) == bestLength && bestID != node.ID {
			bestID = ""
		}
	}
	return bestID
}

func inferWorkspaceNodeConsumer(graph PackageGraphRecord, sourceFile string) string {
	bestName := ""
	bestLength := -1
	for _, node := range graph.Nodes {
		if node.Name == "" {
			continue
		}
		dir := workspaceDescriptorDir(node.Path)
		if dir != "" && !workspacePathContains(dir, sourceFile) {
			continue
		}
		if len(dir) > bestLength {
			bestName = node.Name
			bestLength = len(dir)
		} else if len(dir) == bestLength && bestName != node.Name {
			bestName = ""
		}
	}
	if bestName != "" {
		return bestName
	}
	from := map[string]bool{}
	for _, edge := range graph.Edges {
		if edge.From != "" {
			from[edge.From] = true
		}
	}
	return oneWorkspaceIdentity(from)
}

func workspaceDescriptorDir(path string) string {
	path = strings.TrimPrefix(filepath.ToSlash(path), "./")
	dir := filepath.ToSlash(filepath.Dir(path))
	if dir == "." {
		return ""
	}
	return strings.Trim(dir, "/")
}

func workspacePathContains(dir, path string) bool {
	path = strings.TrimPrefix(filepath.ToSlash(path), "./")
	return path == dir || strings.HasPrefix(path, dir+"/")
}

func oneWorkspaceIdentity(values map[string]bool) string {
	if len(values) != 1 {
		return ""
	}
	for value := range values {
		return value
	}
	return ""
}

func buildWorkspaceSymbolUsages(project workspaceIndexProject, consumerID string, reference RichRelationRecord, candidates []CanonicalSymbolRecord, dependencyEvidence []string) []CanonicalSymbolUsageRecord {
	projectPath := project.record.Path
	base := CanonicalSymbolUsageRecord{
		ConsumerProject:     projectPath,
		ConsumerSymbolID:    consumerID,
		Language:            reference.Language,
		RelationKind:        reference.Type,
		TargetQualifiedName: reference.TargetQualifiedName,
		TargetModule:        reference.TargetModule,
		TargetExport:        reference.TargetExport,
		SourceFile:          reference.From,
		SourceLine:          reference.Line,
		Analyzer:            reference.Analyzer,
		EvidenceIDs:         workspaceFactEvidenceIDs(project, reference.EvidenceIDs, reference.From, reference.Line),
		DependencyEvidence:  append([]string(nil), dependencyEvidence...),
	}
	sort.Strings(base.DependencyEvidence)
	sort.Strings(base.Limitations)
	targetIdentity := workspaceReferenceTargetIdentity(reference)
	switch len(candidates) {
	case 0:
		base.Category = SymbolUsageUnresolved
		base.Confidence = ConfidenceNormalized
		base.Resolution = SymbolResolutionUnresolved
		base.Reason = "no indexed declaration matches the evidenced target"
		base.ID = StableWorkspaceUsageID(
			"",
			projectPath,
			consumerID,
			base.Category,
			base.RelationKind,
			targetIdentity,
			base.SourceFile,
			base.SourceLine,
		)
		return []CanonicalSymbolUsageRecord{base}
	case 1:
		base.ProviderSymbolID = candidates[0].ID
		base.Category = SymbolUsageDirectReference
		base.Confidence = ConfidenceExact
		base.Resolution = SymbolResolutionExact
		base.Reason = "one indexed declaration matches the evidenced target"
		base.ID = StableWorkspaceUsageID(
			base.ProviderSymbolID,
			projectPath,
			consumerID,
			base.Category,
			base.RelationKind,
			base.ProviderSymbolID,
			base.SourceFile,
			base.SourceLine,
		)
		return []CanonicalSymbolUsageRecord{base}
	default:
		candidateIDs := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			candidateIDs = append(candidateIDs, candidate.ID)
		}
		sort.Strings(candidateIDs)
		usages := make([]CanonicalSymbolUsageRecord, 0, len(candidateIDs))
		for _, candidateID := range candidateIDs {
			usage := base
			usage.ProviderSymbolID = candidateID
			usage.Category = SymbolUsageAmbiguous
			usage.Confidence = ConfidenceNormalized
			usage.Resolution = SymbolResolutionAmbiguous
			usage.Reason = "multiple indexed declarations remain after dependency filtering"
			usage.CandidateSymbolIDs = append([]string(nil), candidateIDs...)
			usage.ID = StableWorkspaceUsageID(
				candidateID,
				projectPath,
				consumerID,
				usage.Category,
				usage.RelationKind,
				strings.Join(candidateIDs, ","),
				usage.SourceFile,
				usage.SourceLine,
			)
			usages = append(usages, usage)
		}
		return usages
	}
}

func sortWorkspaceSymbolLookup(index map[string][]CanonicalSymbolRecord) {
	for key := range index {
		sort.Slice(index[key], func(i, j int) bool {
			return index[key][i].ID < index[key][j].ID
		})
	}
}

func indexWorkspaceScriptExportRelations(project workspaceIndexProject, lookup workspaceSymbolLookup) {
	for _, reference := range project.relations {
		if !isScriptLanguage(reference.Language) {
			continue
		}
		publicExport := reference.ExportAlias
		if publicExport == "" {
			publicExport = reference.TargetExport
		}
		if publicExport == "" || publicExport == "*" {
			continue
		}
		var target CanonicalSymbolRecord
		switch reference.Type {
		case "exports_local":
			if reference.ToSymbolID != "" {
				target = lookup.byID[lookup.byProjectLocalID[project.record.Path+"\x00"+reference.ToSymbolID]]
			}
			if target.ID == "" {
				target = exactWorkspaceScriptLocalDeclaration(project, reference, lookup)
			}
		case "reexports_value", "reexports_type":
			if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" {
				continue
			}
			target = lookup.byID[lookup.byProjectLocalID[project.record.Path+"\x00"+reference.ToSymbolID]]
		default:
			continue
		}
		if target.ID == "" {
			continue
		}
		module := scriptModuleIdentity(reference.From)
		if module != "" {
			key := scriptProjectModuleKey(project.record.Path, module, publicExport)
			lookup.scriptByProjectModule[key] = append(lookup.scriptByProjectModule[key], target)
		}
		if target.WorkspacePackage != "" {
			indexWorkspaceScriptPackageExport(
				project,
				target.WorkspacePackage,
				reference.From,
				publicExport,
				target,
				lookup,
			)
		}
	}
}

func indexWorkspaceScriptPackageExport(
	project workspaceIndexProject,
	workspacePackage string,
	moduleFile string,
	exportName string,
	target CanonicalSymbolRecord,
	lookup workspaceSymbolLookup,
) {
	for _, entry := range workspaceScriptPackageSpecifiers(project.packages, workspacePackage, moduleFile) {
		key := scriptWorkspacePackageKey(entry.specifier, exportName, entry.condition)
		lookup.scriptByWorkspacePackage[key] = append(lookup.scriptByWorkspacePackage[key], target)
	}
}

type workspaceScriptPackageSpecifierRecord struct {
	specifier string
	condition string
}

func workspaceScriptPackageSpecifiers(graph PackageGraphRecord, workspacePackage, moduleFile string) []workspaceScriptPackageSpecifierRecord {
	module := workspaceScriptPhysicalModule(moduleFile)
	if workspacePackage == "" || module == "" {
		return nil
	}
	entries := map[string]workspaceScriptPackageSpecifierRecord{}
	for _, node := range graph.Nodes {
		if node.Name != workspacePackage {
			continue
		}
		root := workspaceDescriptorDir(node.Path)
		if root != "" && !workspacePathContains(root, moduleFile) {
			continue
		}
		exportKeys := make([]string, 0, len(node.Exports))
		for exportKey := range node.Exports {
			exportKeys = append(exportKeys, exportKey)
		}
		sort.Strings(exportKeys)
		for _, exportKey := range exportKeys {
			if exportKey != "." && !strings.HasPrefix(exportKey, "./") {
				continue
			}
			specifier := workspaceScriptPackageSpecifier(node.Name, exportKey)
			conditions := node.ExportConditions[exportKey]
			if len(conditions) == 0 {
				if workspaceScriptPackageTargetsModule(root, node.Exports[exportKey], module) {
					entry := workspaceScriptPackageSpecifierRecord{specifier: specifier}
					entries[entry.specifier+"\x00"] = entry
				}
				continue
			}
			conditionNames := make([]string, 0, len(conditions))
			for condition := range conditions {
				conditionNames = append(conditionNames, condition)
			}
			sort.Strings(conditionNames)
			for _, condition := range conditionNames {
				if workspaceScriptPackageTargetsModule(root, conditions[condition], module) {
					entry := workspaceScriptPackageSpecifierRecord{specifier: specifier, condition: condition}
					entries[entry.specifier+"\x00"+entry.condition] = entry
				}
			}
		}
		if len(node.Exports["."]) == 0 &&
			node.Types != "" &&
			workspaceScriptPackageTargetsModule(root, []string{node.Types}, module) {
			entry := workspaceScriptPackageSpecifierRecord{specifier: node.Name}
			entries[entry.specifier+"\x00"] = entry
		}
	}
	result := make([]workspaceScriptPackageSpecifierRecord, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].specifier != result[j].specifier {
			return result[i].specifier < result[j].specifier
		}
		return result[i].condition < result[j].condition
	})
	return result
}

func workspaceScriptPackageCandidates(
	lookup workspaceSymbolLookup,
	specifier string,
	exportName string,
	referenceType string,
) []CanonicalSymbolRecord {
	var candidates []CanonicalSymbolRecord
	candidates = append(
		candidates,
		lookup.scriptByWorkspacePackage[scriptWorkspacePackageKey(specifier, exportName, "")]...,
	)
	condition := scriptReferencePackageCondition(referenceType)
	if branch := activeWorkspaceScriptPackageCondition(lookup, specifier, condition); branch != "" {
		candidates = append(
			candidates,
			lookup.scriptByWorkspacePackage[scriptWorkspacePackageKey(specifier, exportName, branch)]...,
		)
	}
	return dedupeCanonicalWorkspaceSymbols(candidates)
}

func indexWorkspaceScriptPackageConditions(graph PackageGraphRecord, lookup workspaceSymbolLookup) {
	for _, node := range graph.Nodes {
		for exportKey, branches := range node.ExportConditions {
			if exportKey != "." && !strings.HasPrefix(exportKey, "./") {
				continue
			}
			specifier := workspaceScriptPackageSpecifier(node.Name, exportKey)
			if lookup.scriptPackageConditions[specifier] == nil {
				lookup.scriptPackageConditions[specifier] = map[string]bool{}
			}
			for condition := range branches {
				lookup.scriptPackageConditions[specifier][condition] = true
			}
		}
	}
}

func activeWorkspaceScriptPackageCondition(lookup workspaceSymbolLookup, specifier, condition string) string {
	available := lookup.scriptPackageConditions[specifier]
	for _, branch := range scriptPackageConditionBranches(condition) {
		if available[branch] {
			return branch
		}
	}
	return ""
}

func dedupeCanonicalWorkspaceSymbols(symbols []CanonicalSymbolRecord) []CanonicalSymbolRecord {
	byID := make(map[string]CanonicalSymbolRecord, len(symbols))
	for _, symbol := range symbols {
		byID[symbol.ID] = symbol
	}
	result := make([]CanonicalSymbolRecord, 0, len(byID))
	for _, symbol := range byID {
		result = append(result, symbol)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func workspaceScriptPackageTargetsModule(root string, targets []string, module string) bool {
	for _, target := range targets {
		if target == "" || strings.Contains(target, "*") {
			continue
		}
		physical := path.Join(root, strings.TrimPrefix(strings.ReplaceAll(target, `\`, "/"), "./"))
		if workspaceScriptPhysicalModule(physical) == module {
			return true
		}
	}
	return false
}

func workspaceScriptPackageSpecifier(packageName, exportKey string) string {
	if exportKey == "." {
		return packageName
	}
	return packageName + "/" + strings.TrimPrefix(exportKey, "./")
}

func workspaceScriptPhysicalModule(file string) string {
	file = path.Clean(strings.ReplaceAll(file, `\`, "/"))
	for _, suffix := range []string{
		".d.mts",
		".d.cts",
		".d.ts",
		".tsx",
		".jsx",
		".mjs",
		".cjs",
		".mts",
		".cts",
		".ts",
		".js",
	} {
		if strings.HasSuffix(file, suffix) {
			return strings.TrimSuffix(file, suffix)
		}
	}
	return file
}

func exactWorkspaceScriptLocalDeclaration(project workspaceIndexProject, reference RichRelationRecord, lookup workspaceSymbolLookup) CanonicalSymbolRecord {
	var found CanonicalSymbolRecord
	for _, declaration := range project.symbols {
		if declaration.File != reference.From || declaration.Name != reference.TargetExport {
			continue
		}
		canonicalID := lookup.byProjectLocalID[project.record.Path+"\x00"+declaration.ID]
		if canonicalID == "" || (found.ID != "" && found.ID != canonicalID) {
			return CanonicalSymbolRecord{}
		}
		found = lookup.byID[canonicalID]
	}
	return found
}

func scriptProjectModuleKey(project, module, exportName string) string {
	return project + "\x00" + module + "\x00" + exportName
}

func scriptWorkspacePackageKey(workspacePackage, exportName, condition string) string {
	return workspacePackage + "\x00" + exportName + "\x00" + condition
}

func isScriptLanguage(language string) bool {
	return language == "javascript" || language == "typescript"
}

func workspaceReferenceTargetIdentity(reference RichRelationRecord) string {
	if isScriptLanguage(reference.Language) {
		return reference.TargetModule + "\x00" + reference.TargetExport
	}
	return reference.TargetQualifiedName
}

func dedupeStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func canonicalWorkspaceSymbol(project workspaceIndexProject, declaration RichSymbolRecord) CanonicalSymbolRecord {
	scope := declaration.Artifact
	if scope == "" {
		scope = declaration.WorkspacePackage
	}
	if scope == "" {
		scope = declaration.Module
	}
	id := StableWorkspaceSymbolID(
		declaration.Kind,
		project.record.Path,
		scope,
		declaration.Language,
		declaration.QualifiedName,
		declaration.File,
	)
	return CanonicalSymbolRecord{
		ID:               id,
		Project:          project.record.Path,
		Service:          project.record.Service,
		ProjectKind:      project.record.Kind,
		Module:           declaration.Module,
		Package:          declaration.Package,
		Application:      declaration.Application,
		WorkspacePackage: declaration.WorkspacePackage,
		Artifact:         declaration.Artifact,
		Language:         declaration.Language,
		Kind:             declaration.Kind,
		Name:             declaration.Name,
		QualifiedName:    declaration.QualifiedName,
		ExportName:       declaration.ExportName,
		DeclarationFile:  declaration.File,
		DeclarationLine:  declaration.Line,
		EvidenceIDs:      workspaceFactEvidenceIDs(project, declaration.EvidenceIDs, declaration.File, declaration.Line),
		Analyzer:         declaration.Analyzer,
		Confidence:       declaration.Confidence,
		Coverage:         declaration.Coverage,
		Limitations:      sortedStrings(declaration.Limitations),
	}
}

func isWorkspaceCanonicalDeclaration(declaration RichSymbolRecord) bool {
	return isWorkspaceSymbolLanguageSupported(declaration.Language) &&
		declaration.QualifiedName != "" &&
		declaration.Kind != "" &&
		declaration.File != ""
}

func workspaceEvidenceIDs(project string, localIDs []string) []string {
	result := make([]string, 0, len(localIDs))
	for _, localID := range localIDs {
		if localID == "" {
			continue
		}
		result = append(result, WorkspaceEvidenceID(project, localID))
	}
	sort.Strings(result)
	return dedupeStrings(result)
}

func workspaceFactEvidenceIDs(project workspaceIndexProject, localIDs []string, file string, line int) []string {
	ids := append([]string(nil), localIDs...)
	for _, evidence := range project.evidence {
		if evidence.ID == "" || evidence.File != file {
			continue
		}
		if line > 0 && evidence.Start.Line > 0 && evidence.Start.Line != line {
			continue
		}
		ids = append(ids, evidence.ID)
	}
	return workspaceEvidenceIDs(project.record.Path, ids)
}
