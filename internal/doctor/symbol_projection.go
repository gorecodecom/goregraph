package doctor

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

const symbolProjectionRemediation = "goregraph workspace clean . --execute && goregraph workspace scan-all ."

// ValidateSymbolProjection validates the canonical workspace symbol inventory
// and every reference emitted by its usage projection.
func ValidateSymbolProjection(index scan.WorkspaceSymbolIndexRecord, usages scan.WorkspaceSymbolUsageIndexRecord, knownEvidence map[string]bool) error {
	knownProjects := symbolProjectionProjects(knownEvidence)
	if index.SchemaVersion != scan.SchemaVersion {
		return invalidSymbolProjection(
			"schema_version",
			"symbol index schema_version is %d, want %d",
			index.SchemaVersion,
			scan.SchemaVersion,
		)
	}
	if usages.SchemaVersion != scan.SchemaVersion {
		return invalidSymbolProjection(
			"schema_version",
			"symbol usage schema_version is %d, want %d",
			usages.SchemaVersion,
			scan.SchemaVersion,
		)
	}

	symbols := make(map[string]scan.CanonicalSymbolRecord, len(index.Symbols))
	for _, symbol := range index.Symbols {
		if symbol.ID == "" {
			return invalidSymbolProjection("<empty symbol ID>", "workspace symbol ID is empty")
		}
		if _, duplicate := symbols[symbol.ID]; duplicate {
			return invalidSymbolProjection(symbol.ID, "duplicate workspace symbol ID %q", symbol.ID)
		}
		symbols[symbol.ID] = symbol
		if err := validateSymbolRecord(symbol, knownProjects, knownEvidence); err != nil {
			return err
		}
	}

	usageIDs := make(map[string]bool, len(usages.Usages))
	for _, usage := range usages.Usages {
		if usage.ID == "" {
			return invalidSymbolProjection("<empty usage ID>", "workspace symbol usage ID is empty")
		}
		if usageIDs[usage.ID] {
			return invalidSymbolProjection(usage.ID, "duplicate workspace symbol usage ID %q", usage.ID)
		}
		usageIDs[usage.ID] = true
		if err := validateSymbolUsageRecord(usage, symbols, knownProjects, knownEvidence); err != nil {
			return err
		}
	}

	coverage := append([]scan.SymbolCoverageRecord(nil), index.Coverage...)
	coverage = append(coverage, usages.Coverage...)
	for _, record := range coverage {
		offending := strings.Join([]string{record.Project, record.Language, record.Capability}, "/")
		if offending == "//" {
			offending = "<empty coverage record>"
		}
		if record.Project == "" || record.Language == "" || record.Capability == "" || record.Reason == "" {
			return invalidSymbolProjection(offending, "workspace symbol coverage %q has missing required fields", offending)
		}
		if err := record.Coverage.Validate(); err != nil {
			return invalidSymbolProjection(offending, "workspace symbol coverage %q: %v", offending, err)
		}
		if !knownProject(record.Project, knownProjects) {
			return invalidSymbolProjection(offending, "workspace symbol coverage %q references unknown project %q", offending, record.Project)
		}
	}
	return nil
}

func validateSymbolRecord(symbol scan.CanonicalSymbolRecord, knownProjects, knownEvidence map[string]bool) error {
	if symbol.Project == "" ||
		symbol.Language == "" ||
		symbol.Kind == "" ||
		symbol.QualifiedName == "" ||
		symbol.DeclarationFile == "" ||
		symbol.Analyzer == "" {
		return invalidSymbolProjection(symbol.ID, "workspace symbol %q has missing required fields", symbol.ID)
	}
	if !knownProject(symbol.Project, knownProjects) {
		return invalidSymbolProjection(symbol.ID, "workspace symbol %q references unknown project %q", symbol.ID, symbol.Project)
	}
	if err := validateProjectionSource(symbol.DeclarationFile); err != nil {
		return invalidSymbolProjection(symbol.ID, "workspace symbol %q has invalid declaration source %q: %v", symbol.ID, symbol.DeclarationFile, err)
	}
	if err := symbol.Confidence.Validate(); err != nil {
		return invalidSymbolProjection(symbol.ID, "workspace symbol %q: %v", symbol.ID, err)
	}
	if err := symbol.Coverage.Validate(); err != nil {
		return invalidSymbolProjection(symbol.ID, "workspace symbol %q: %v", symbol.ID, err)
	}
	if err := validateProjectionEvidence(symbol.ID, symbol.Project, symbol.EvidenceIDs, knownProjects, knownEvidence, true); err != nil {
		return err
	}
	return nil
}

func validateSymbolUsageRecord(
	usage scan.CanonicalSymbolUsageRecord,
	symbols map[string]scan.CanonicalSymbolRecord,
	knownProjects map[string]bool,
	knownEvidence map[string]bool,
) error {
	if usage.ConsumerProject == "" ||
		usage.Category == "" ||
		usage.Language == "" ||
		usage.RelationKind == "" ||
		usage.SourceFile == "" ||
		usage.Confidence == "" ||
		usage.Resolution == "" ||
		usage.Reason == "" ||
		usage.Analyzer == "" {
		return invalidSymbolProjection(usage.ID, "workspace symbol usage %q has missing required fields", usage.ID)
	}
	if !knownProject(usage.ConsumerProject, knownProjects) {
		return invalidSymbolProjection(usage.ID, "workspace symbol usage %q references unknown consumer project %q", usage.ID, usage.ConsumerProject)
	}
	if err := validateProjectionSource(usage.SourceFile); err != nil {
		return invalidSymbolProjection(usage.ID, "workspace symbol usage %q has invalid source %q: %v", usage.ID, usage.SourceFile, err)
	}
	if err := usage.Confidence.Validate(); err != nil {
		return invalidSymbolProjection(usage.ID, "workspace symbol usage %q: %v", usage.ID, err)
	}
	if usage.ConsumerSymbolID != "" {
		consumer, ok := symbols[usage.ConsumerSymbolID]
		if !ok {
			return invalidSymbolProjection(usage.ConsumerSymbolID, "workspace symbol usage %q references unknown consumer symbol %q", usage.ID, usage.ConsumerSymbolID)
		}
		if consumer.Project != usage.ConsumerProject {
			return invalidSymbolProjection(
				usage.ID,
				"workspace symbol usage %q consumer symbol %q belongs to project %q, not %q",
				usage.ID,
				usage.ConsumerSymbolID,
				consumer.Project,
				usage.ConsumerProject,
			)
		}
	}
	if err := validateProjectionEvidence(usage.ID, "", usage.EvidenceIDs, knownProjects, knownEvidence, false); err != nil {
		return err
	}
	if err := validateSortedProjectionIDs(usage.ID, "candidate path", usage.CandidatePathIDs, nil); err != nil {
		return err
	}

	apiRecord := usage.RelationKind == "http_reachability" || usage.Transport != "" || len(usage.APIPath) > 0
	if apiRecord {
		if usage.Transport != "http" {
			return invalidSymbolProjection(usage.ID, "workspace symbol usage %q API transport is %q, want http", usage.ID, usage.Transport)
		}
		if len(usage.APIPath) == 0 {
			return invalidSymbolProjection(usage.ID, "workspace symbol usage %q API path is empty", usage.ID)
		}
		if err := validateSymbolAPIPath(usage, symbols, knownProjects, knownEvidence); err != nil {
			return err
		}
	}

	switch usage.Resolution {
	case scan.SymbolResolutionExact:
		if usage.Category != scan.SymbolUsageDirectReference && usage.Category != scan.SymbolUsageReachedThroughAPI {
			return invalidSymbolProjection(
				usage.ID,
				"workspace symbol usage %q category %q is incompatible with resolution %q",
				usage.ID,
				usage.Category,
				usage.Resolution,
			)
		}
		if usage.ProviderSymbolID == "" {
			return invalidSymbolProjection(usage.ID, "workspace symbol usage %q has no exact provider", usage.ID)
		}
		if _, ok := symbols[usage.ProviderSymbolID]; !ok {
			return invalidSymbolProjection(usage.ProviderSymbolID, "workspace symbol usage %q references unknown provider %q", usage.ID, usage.ProviderSymbolID)
		}
		if usage.Category == scan.SymbolUsageReachedThroughAPI {
			if !apiRecord {
				return invalidSymbolProjection(usage.ID, "workspace symbol usage %q API category has no HTTP path", usage.ID)
			}
			if err := validateSelectedAPIProvider(usage); err != nil {
				return err
			}
		} else if apiRecord {
			return invalidSymbolProjection(usage.ID, "workspace symbol usage %q direct reference contains an API path", usage.ID)
		}
	case scan.SymbolResolutionAmbiguous:
		if usage.Category != scan.SymbolUsageAmbiguous || usage.ProviderSymbolID == "" {
			return invalidSymbolProjection(
				usage.ID,
				"workspace symbol usage %q category %q is incompatible with resolution %q",
				usage.ID,
				usage.Category,
				usage.Resolution,
			)
		}
		if _, ok := symbols[usage.ProviderSymbolID]; !ok {
			return invalidSymbolProjection(usage.ProviderSymbolID, "workspace symbol usage %q references unknown ambiguous provider %q", usage.ID, usage.ProviderSymbolID)
		}
		if err := validateAmbiguousCandidates(usage, symbols); err != nil {
			return err
		}
		if apiRecord {
			if err := validateSelectedAPIProvider(usage); err != nil {
				return err
			}
		}
	case scan.SymbolResolutionUnresolved:
		if usage.Category != scan.SymbolUsageUnresolved ||
			usage.ProviderSymbolID != "" ||
			len(usage.CandidateSymbolIDs) != 0 {
			return invalidSymbolProjection(
				usage.ID,
				"workspace symbol usage %q category %q, provider %q, or candidates are incompatible with resolution %q",
				usage.ID,
				usage.Category,
				usage.ProviderSymbolID,
				usage.Resolution,
			)
		}
	default:
		return invalidSymbolProjection(usage.ID, "workspace symbol usage %q has invalid resolution %q", usage.ID, usage.Resolution)
	}
	return nil
}

func validateSelectedAPIProvider(usage scan.CanonicalSymbolUsageRecord) error {
	final := usage.APIPath[len(usage.APIPath)-1]
	if final.Kind != "selected_symbol" || final.SymbolID != usage.ProviderSymbolID {
		return invalidSymbolProjection(
			usage.ID,
			"workspace symbol usage %q API path does not end with selected provider %q",
			usage.ID,
			usage.ProviderSymbolID,
		)
	}
	return nil
}

func validateAmbiguousCandidates(usage scan.CanonicalSymbolUsageRecord, symbols map[string]scan.CanonicalSymbolRecord) error {
	flowAmbiguous := usage.Transport == "http" && containsProjectionValue(usage.Limitations, "feature_flow_join_ambiguous")
	if flowAmbiguous {
		if len(usage.CandidateSymbolIDs) < 1 || len(usage.CandidatePathIDs) < 2 {
			return invalidSymbolProjection(usage.ID, "workspace symbol usage %q does not disclose its complete ambiguous candidates", usage.ID)
		}
	} else if len(usage.CandidateSymbolIDs) < 2 {
		return invalidSymbolProjection(usage.ID, "workspace symbol usage %q does not disclose its complete ambiguous candidates", usage.ID)
	}
	if err := validateSortedProjectionIDs(usage.ID, "candidate symbol", usage.CandidateSymbolIDs, symbols); err != nil {
		return err
	}
	selected := false
	for _, candidateID := range usage.CandidateSymbolIDs {
		if candidateID == usage.ProviderSymbolID || flowAmbiguous && candidateID == usage.ConsumerSymbolID {
			selected = true
		}
	}
	if !selected {
		return invalidSymbolProjection(usage.ID, "workspace symbol usage %q selected symbol is absent from its candidate set", usage.ID)
	}
	return nil
}

func validateSymbolAPIPath(
	usage scan.CanonicalSymbolUsageRecord,
	symbols map[string]scan.CanonicalSymbolRecord,
	knownProjects map[string]bool,
	knownEvidence map[string]bool,
) error {
	for position, step := range usage.APIPath {
		if step.Position != position {
			return invalidSymbolProjection(
				usage.ID,
				"workspace symbol usage %q API path position %d is %d",
				usage.ID,
				position,
				step.Position,
			)
		}
		if step.Kind == "" || step.Project == "" || step.Label == "" {
			return invalidSymbolProjection(usage.ID, "workspace symbol usage %q API path step %d has missing required fields", usage.ID, position)
		}
		if !knownProject(step.Project, knownProjects) {
			return invalidSymbolProjection(
				usage.ID,
				"workspace symbol usage %q API path step %d references unknown project %q",
				usage.ID,
				position,
				step.Project,
			)
		}
		if step.File != "" {
			if err := validateProjectionSource(step.File); err != nil {
				return invalidSymbolProjection(
					usage.ID,
					"workspace symbol usage %q API path step %d has invalid source %q: %v",
					usage.ID,
					position,
					step.File,
					err,
				)
			}
		}
		if step.SymbolID != "" {
			symbol, ok := symbols[step.SymbolID]
			if !ok {
				return invalidSymbolProjection(step.SymbolID, "workspace symbol usage %q API path references unknown symbol %q", usage.ID, step.SymbolID)
			}
			if symbol.Project != step.Project {
				return invalidSymbolProjection(
					usage.ID,
					"workspace symbol usage %q API path symbol %q belongs to project %q, not %q",
					usage.ID,
					step.SymbolID,
					symbol.Project,
					step.Project,
				)
			}
		}
		if err := validateProjectionEvidence(usage.ID, step.Project, step.EvidenceIDs, knownProjects, knownEvidence, true); err != nil {
			return err
		}
	}
	return nil
}

func validateSortedProjectionIDs(
	usageID string,
	kind string,
	ids []string,
	symbols map[string]scan.CanonicalSymbolRecord,
) error {
	previous := ""
	for _, id := range ids {
		if id == "" {
			return invalidSymbolProjection(usageID, "workspace symbol usage %q contains an empty %s ID", usageID, kind)
		}
		if previous != "" && id <= previous {
			return invalidSymbolProjection(id, "workspace symbol usage %q %s IDs are duplicate or unsorted at %q", usageID, kind, id)
		}
		if symbols != nil {
			if _, ok := symbols[id]; !ok {
				return invalidSymbolProjection(id, "workspace symbol usage %q references unknown %s %q", usageID, kind, id)
			}
		}
		previous = id
	}
	return nil
}

func validateProjectionEvidence(
	offending string,
	ownerProject string,
	ids []string,
	knownProjects map[string]bool,
	knownEvidence map[string]bool,
	requireOwner bool,
) error {
	previous := ""
	for _, id := range ids {
		project, localID, namespaced := strings.Cut(id, "#")
		if !namespaced || project == "" || localID == "" {
			return invalidSymbolProjection(id, "workspace projection record %q has non-namespaced evidence %q", offending, id)
		}
		if previous != "" && id <= previous {
			return invalidSymbolProjection(id, "workspace projection record %q has duplicate or unsorted evidence %q", offending, id)
		}
		if requireOwner && project != ownerProject {
			return invalidSymbolProjection(
				id,
				"workspace projection record %q evidence %q belongs to project %q, not %q",
				offending,
				id,
				project,
				ownerProject,
			)
		}
		if !knownProject(project, knownProjects) {
			return invalidSymbolProjection(id, "workspace projection record %q evidence %q references unknown project %q", offending, id, project)
		}
		if !knownEvidence[id] {
			return invalidSymbolProjection(id, "workspace projection record %q references unknown evidence %q", offending, id)
		}
		previous = id
	}
	return nil
}

func symbolProjectionProjects(knownEvidence map[string]bool) map[string]bool {
	projects := map[string]bool{}
	for value, known := range knownEvidence {
		if !known {
			continue
		}
		if project, _, namespaced := strings.Cut(value, "#"); namespaced {
			if project != "" {
				projects[project] = true
			}
			continue
		}
		if value != "" {
			projects[value] = true
		}
	}
	return projects
}

func knownProject(project string, knownProjects map[string]bool) bool {
	return knownProjects[project]
}

func validateProjectionSource(source string) error {
	normalized := strings.ReplaceAll(strings.TrimSpace(source), `\`, "/")
	if normalized == "" {
		return fmt.Errorf("source is empty")
	}
	cleaned := path.Clean(normalized)
	if strings.HasPrefix(normalized, "/") ||
		strings.HasPrefix(normalized, "../") ||
		strings.Contains(normalized, "/../") ||
		cleaned == "." ||
		cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") ||
		len(normalized) >= 2 && normalized[1] == ':' {
		return fmt.Errorf("source must be project-relative")
	}
	return nil
}

func invalidSymbolProjection(offending, format string, args ...any) error {
	message := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s; offending ID/reference: %s; remediate with: %s", message, offending, symbolProjectionRemediation)
}

func containsProjectionValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func checkWorkspaceSymbolProjection(workspaceRoot string, result *Result) {
	workspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		result.fail("workspace-symbols", err.Error())
		return
	}
	out := filepath.Join(workspaceRoot, ".goregraph-workspace")
	remediation := workspaceSymbolRemediation(workspaceRoot)
	var missing []string
	for _, name := range []string{"symbol-index.json", "symbol-usages.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		result.fail("workspace-symbols", "workspace symbol projection missing: "+strings.Join(missing, ", "))
		result.fix(remediation)
		return
	}

	var registry scan.WorkspaceRegistryRecord
	if err := readJSON(filepath.Join(out, "registry.json"), &registry); err != nil {
		result.fail("workspace-symbols", "registry.json invalid: "+err.Error())
		result.fix(remediation)
		return
	}
	var index scan.WorkspaceSymbolIndexRecord
	if err := readJSON(filepath.Join(out, "symbol-index.json"), &index); err != nil {
		result.fail("workspace-symbols", "symbol-index.json missing or invalid: "+err.Error())
		result.fix(remediation)
		return
	}
	var usages scan.WorkspaceSymbolUsageIndexRecord
	if err := readJSON(filepath.Join(out, "symbol-usages.json"), &usages); err != nil {
		result.fail("workspace-symbols", "symbol-usages.json missing or invalid: "+err.Error())
		result.fix(remediation)
		return
	}
	knownEvidence, err := loadWorkspaceProjectionEvidence(workspaceRoot, registry)
	if err != nil {
		result.fail("workspace-symbols", err.Error())
		result.fix(remediation)
		return
	}
	if err := ValidateSymbolProjection(index, usages, knownEvidence); err != nil {
		result.fail("workspace-symbols", err.Error())
		result.fix(remediation)
		return
	}
	result.ok("workspace-symbols", "workspace symbol projection valid")
}

func loadWorkspaceProjectionEvidence(workspaceRoot string, registry scan.WorkspaceRegistryRecord) (map[string]bool, error) {
	knownEvidence := map[string]bool{}
	for _, project := range registry.Projects {
		if project.Path == "" {
			return nil, fmt.Errorf("workspace registry contains a project with an empty path")
		}
		knownEvidence[project.Path] = true
	}
	for _, project := range registry.Projects {
		if !project.Indexed {
			continue
		}
		projectRoot := project.AbsPath
		if projectRoot == "" {
			projectRoot = filepath.Join(workspaceRoot, filepath.FromSlash(project.Path))
		} else if !filepath.IsAbs(projectRoot) {
			projectRoot = filepath.Join(workspaceRoot, filepath.FromSlash(projectRoot))
		}
		outputDir := project.OutputDir
		if outputDir == "" {
			outputDir = "goregraph-out"
		}
		var evidence []scan.EvidenceRecord
		path := filepath.Join(projectRoot, outputDir, "evidence.json")
		if err := readJSON(path, &evidence); err != nil {
			return nil, fmt.Errorf("workspace project %q evidence.json invalid: %w", project.Path, err)
		}
		localIDs := map[string]bool{}
		for _, record := range evidence {
			if record.ID == "" {
				return nil, fmt.Errorf("workspace project %q contains an empty evidence ID", project.Path)
			}
			if localIDs[record.ID] {
				return nil, fmt.Errorf("workspace project %q contains duplicate evidence ID %q", project.Path, record.ID)
			}
			localIDs[record.ID] = true
			knownEvidence[scan.WorkspaceEvidenceID(project.Path, record.ID)] = true
		}
	}
	return knownEvidence, nil
}

func workspaceSymbolRemediation(workspaceRoot string) string {
	return fmt.Sprintf(
		"goregraph workspace clean %s --execute && goregraph workspace scan-all %s",
		workspaceRoot,
		workspaceRoot,
	)
}

func workspaceDirectoryExists(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".goregraph-workspace"))
	return err == nil && info.IsDir()
}

func explicitWorkspaceState(root string, cfg config.Config) bool {
	if cfg.WorkspaceRoot != "" || workspaceDirectoryExists(root) {
		return true
	}
	_, err := os.Stat(filepath.Join(root, ".goregraph-workspace.yml"))
	return err == nil
}
