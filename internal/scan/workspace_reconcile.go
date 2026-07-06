package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
)

var workspaceGroupDirs = []string{"frontend", "frontends", "microservices", "services", "backends"}

type workspaceIndexProject struct {
	record    WorkspaceProjectRecord
	routes    []CodeRouteRecord
	contracts []APIContractRecord
}

type workspaceBackendRoute struct {
	project WorkspaceProjectRecord
	route   CodeRouteRecord
}

// ReconcileWorkspace refreshes workspace-level overlay files after a local scan.
func ReconcileWorkspace(currentRoot string, cfg config.Config) (*WorkspaceRegistryRecord, error) {
	if !cfg.Workspace {
		return nil, nil
	}
	currentAbs, err := filepath.Abs(currentRoot)
	if err != nil {
		return nil, err
	}
	workspaceRoot, ok, err := resolveWorkspaceRoot(currentAbs, cfg.WorkspaceRoot)
	if err != nil || !ok {
		return nil, err
	}

	projects, err := discoverWorkspaceProjects(workspaceRoot, currentAbs, cfg.OutputDir)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, nil
	}

	registry := WorkspaceRegistryRecord{
		Root:      filepath.ToSlash(workspaceRoot),
		Current:   workspaceRel(workspaceRoot, currentAbs),
		Generated: time.Now().UTC().Format(time.RFC3339),
		Projects:  projects,
	}

	indexed, err := loadWorkspaceIndexes(projects)
	if err != nil {
		return nil, err
	}
	context := buildWorkspaceContext(registry, indexed)
	matches := buildWorkspaceContractMatches(indexed)

	workspaceOut := filepath.Join(workspaceRoot, ".goregraph-workspace")
	if err := os.MkdirAll(workspaceOut, 0o755); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(workspaceOut, "registry.json"), registry); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(workspaceOut, "context.json"), context); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(workspaceOut, "contract-matches.json"), matches); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(workspaceOut, "workspace-context.md"), []byte(renderWorkspaceContextReport(context)), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(workspaceOut, "contract-matches.md"), []byte(renderWorkspaceContractMatchesReport(matches)), 0o644); err != nil {
		return nil, err
	}

	for _, project := range indexed {
		out := filepath.Join(project.record.AbsPath, project.record.OutputDir)
		if err := os.WriteFile(filepath.Join(out, "workspace-context.md"), []byte(renderWorkspaceContextReport(context)), 0o644); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(out, "workspace-contract-matches.md"), []byte(renderProjectWorkspaceMatchesReport(project.record.Path, matches)), 0o644); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(out, "frontend-consumers.md"), []byte(renderFrontendConsumersReport(project.record.Path, matches)), 0o644); err != nil {
			return nil, err
		}
	}

	return &registry, nil
}

func resolveWorkspaceRoot(currentAbs, override string) (string, bool, error) {
	if override != "" {
		root := override
		if !filepath.IsAbs(root) {
			root = filepath.Join(currentAbs, root)
		}
		resolved, err := filepath.Abs(root)
		if err != nil {
			return "", false, err
		}
		return resolved, true, nil
	}
	if hasWorkspaceManifest(currentAbs) || hasWorkspaceGroups(currentAbs) {
		return currentAbs, true, nil
	}
	for dir := filepath.Dir(currentAbs); dir != "." && dir != ""; dir = filepath.Dir(dir) {
		if hasWorkspaceManifest(dir) || hasWorkspaceGroups(dir) {
			return dir, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", false, nil
}

func hasWorkspaceManifest(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".goregraph-workspace", "registry.json")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, ".goregraph-workspace.yml")); err == nil {
		return true
	}
	return false
}

func hasWorkspaceGroups(dir string) bool {
	count := 0
	for _, name := range workspaceGroupDirs {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && info.IsDir() {
			count++
		}
	}
	return count > 0
}

func discoverWorkspaceProjects(workspaceRoot, currentAbs, defaultOutput string) ([]WorkspaceProjectRecord, error) {
	projectsByPath := map[string]WorkspaceProjectRecord{}
	for _, group := range workspaceGroupDirs {
		groupPath := filepath.Join(workspaceRoot, group)
		entries, err := os.ReadDir(groupPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			abs := filepath.Join(groupPath, entry.Name())
			addWorkspaceProject(projectsByPath, workspaceRoot, currentAbs, abs, group, defaultOutput)
		}
	}

	entries, err := os.ReadDir(workspaceRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || isWorkspaceGroup(entry.Name()) || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			abs := filepath.Join(workspaceRoot, entry.Name())
			if hasProjectMarker(abs, defaultOutput) {
				addWorkspaceProject(projectsByPath, workspaceRoot, currentAbs, abs, "", defaultOutput)
			}
		}
	}

	projects := make([]WorkspaceProjectRecord, 0, len(projectsByPath))
	for _, project := range projectsByPath {
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Path < projects[j].Path })
	return projects, nil
}

func addWorkspaceProject(projects map[string]WorkspaceProjectRecord, workspaceRoot, currentAbs, abs, group, defaultOutput string) {
	rel := workspaceRel(workspaceRoot, abs)
	outputDir := projectOutputDir(abs, defaultOutput)
	indexed := workspaceFileExists(filepath.Join(abs, outputDir, "manifest.json"))
	status := "not_indexed"
	if indexed {
		status = "indexed"
	}
	if samePath(abs, currentAbs) {
		status = "current"
		indexed = true
	}
	projects[rel] = WorkspaceProjectRecord{
		Name:      filepath.Base(abs),
		Path:      rel,
		AbsPath:   filepath.ToSlash(abs),
		Kind:      workspaceProjectKind(group, abs),
		Service:   workspaceProjectService(group, abs),
		Indexed:   indexed,
		Status:    status,
		OutputDir: outputDir,
	}
}

func projectOutputDir(abs, fallback string) string {
	cfg, err := config.Load(abs)
	if err == nil && cfg.OutputDir != "" {
		return cfg.OutputDir
	}
	if fallback != "" {
		return fallback
	}
	return "goregraph-out"
}

func workspaceProjectKind(group, abs string) string {
	switch group {
	case "frontend", "frontends":
		return "frontend"
	case "microservices", "services", "backends":
		return "backend"
	}
	if workspaceFileExists(filepath.Join(abs, "package.json")) {
		return "frontend"
	}
	if workspaceFileExists(filepath.Join(abs, "pom.xml")) || workspaceFileExists(filepath.Join(abs, "build.gradle")) || workspaceFileExists(filepath.Join(abs, "build.gradle.kts")) {
		return "backend"
	}
	return "project"
}

func workspaceProjectService(group, abs string) string {
	switch group {
	case "microservices", "services", "backends":
		return filepath.Base(abs)
	default:
		return ""
	}
}

func isWorkspaceGroup(name string) bool {
	for _, group := range workspaceGroupDirs {
		if name == group {
			return true
		}
	}
	return false
}

func hasProjectMarker(abs, outputDir string) bool {
	for _, name := range []string{"package.json", "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "go.mod", "pyproject.toml", "requirements.txt", "setup.py", "Cargo.toml", "composer.json", "goregraph.yml"} {
		if workspaceFileExists(filepath.Join(abs, name)) {
			return true
		}
	}
	return workspaceFileExists(filepath.Join(abs, outputDir, "manifest.json"))
}

func loadWorkspaceIndexes(projects []WorkspaceProjectRecord) ([]workspaceIndexProject, error) {
	var result []workspaceIndexProject
	for _, project := range projects {
		if !project.Indexed {
			continue
		}
		out := filepath.Join(project.AbsPath, project.OutputDir)
		if !workspaceFileExists(filepath.Join(out, "manifest.json")) {
			continue
		}
		loaded := workspaceIndexProject{record: project}
		if err := readWorkspaceJSON(filepath.Join(out, "routes.json"), &loaded.routes); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err := readWorkspaceJSON(filepath.Join(out, "api-contracts.json"), &loaded.contracts); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		result = append(result, loaded)
	}
	return result, nil
}

func buildWorkspaceContext(registry WorkspaceRegistryRecord, indexed []workspaceIndexProject) WorkspaceContextRecord {
	var loaded []WorkspaceProjectRecord
	serviceSet := map[string]bool{}
	for _, project := range indexed {
		loaded = append(loaded, project.record)
		if project.record.Service != "" && hasBackendRoutes(project.routes) {
			serviceSet[project.record.Service] = true
		}
	}

	referenced := map[string]bool{}
	for _, project := range indexed {
		for _, contract := range project.contracts {
			if contract.ServiceCandidate != "" {
				referenced[contract.ServiceCandidate] = true
			}
		}
	}
	var known []string
	for service := range serviceSet {
		known = append(known, service)
	}
	sort.Strings(known)
	var missing []string
	for service := range referenced {
		if !serviceSet[service] {
			missing = append(missing, service)
		}
	}
	sort.Strings(missing)
	return WorkspaceContextRecord{
		Root:            registry.Root,
		Current:         registry.Current,
		LoadedIndexes:   loaded,
		Projects:        registry.Projects,
		KnownServices:   known,
		MissingServices: missing,
	}
}

func buildWorkspaceContractMatches(projects []workspaceIndexProject) []WorkspaceContractMatchRecord {
	var backendRoutes []workspaceBackendRoute
	knownServices := map[string]bool{}
	for _, project := range projects {
		for _, route := range project.routes {
			if route.Kind != "backend" {
				continue
			}
			backendRoutes = append(backendRoutes, workspaceBackendRoute{project: project.record, route: route})
			service := project.record.Service
			if service == "" {
				service = serviceCandidateForPath(route.Path)
			}
			if service != "" {
				knownServices[service] = true
			}
		}
	}

	var records []WorkspaceContractMatchRecord
	for _, project := range projects {
		for _, contract := range project.contracts {
			records = append(records, workspaceContractMatch(project.record, contract, backendRoutes, knownServices))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].APIProject != records[j].APIProject {
			return records[i].APIProject < records[j].APIProject
		}
		if records[i].APIFile != records[j].APIFile {
			return records[i].APIFile < records[j].APIFile
		}
		if records[i].APILine != records[j].APILine {
			return records[i].APILine < records[j].APILine
		}
		return records[i].APIPath < records[j].APIPath
	})
	return records
}

func workspaceContractMatch(project WorkspaceProjectRecord, contract APIContractRecord, routes []workspaceBackendRoute, knownServices map[string]bool) WorkspaceContractMatchRecord {
	base := WorkspaceContractMatchRecord{
		APIProject:       project.Path,
		APIHTTPMethod:    contract.HTTPMethod,
		APIPath:          contract.Path,
		APIFile:          contract.File,
		APILine:          contract.Line,
		ServiceCandidate: contract.ServiceCandidate,
	}
	if contract.UnsafeDynamic {
		base.Issue = contractIssueUnsafeDynamic
		base.Confidence = "WEAK_MATCH"
		base.ConfidenceScore = 0.35
		base.Reason = "api path contains complex dynamic expression"
		return base
	}
	if route, ok := exactWorkspaceRoute(contract, routes); ok {
		return workspaceContractIssue(base, route, contractIssueMatched, "RESOLVED", 0.9, "http method and path pattern match backend route")
	}
	if route, ok := pathCompatibleWorkspaceRoute(contract, routes); ok {
		return workspaceContractIssue(base, route, contractIssueMethodMismatch, "WEAK_MATCH", 0.45, "path pattern exists but http method differs")
	}
	if contract.ServiceCandidate != "" && !knownServices[contract.ServiceCandidate] {
		base.Issue = contractIssueUnscanned
		base.Confidence = "OUT_OF_SCOPE"
		base.ConfidenceScore = 0.75
		base.Reason = contract.ServiceCandidate + " has no indexed backend routes in this workspace"
		return base
	}
	base.Issue = contractIssueMissingRoute
	base.Confidence = "WEAK_MATCH"
	base.ConfidenceScore = 0.3
	base.Reason = "no compatible backend route found in indexed workspace services"
	return base
}

func workspaceContractIssue(base WorkspaceContractMatchRecord, route workspaceBackendRoute, issue, confidence string, score float64, reason string) WorkspaceContractMatchRecord {
	base.BackendProject = route.project.Path
	base.BackendService = route.project.Service
	if base.BackendService == "" {
		base.BackendService = serviceCandidateForPath(route.route.Path)
	}
	base.BackendHTTPMethod = route.route.HTTPMethod
	base.BackendPath = route.route.Path
	base.BackendHandler = route.route.Handler
	base.BackendFile = route.route.File
	base.BackendLine = route.route.Line
	base.Issue = issue
	base.Confidence = confidence
	base.ConfidenceScore = score
	base.Reason = reason
	return base
}

func exactWorkspaceRoute(contract APIContractRecord, routes []workspaceBackendRoute) (workspaceBackendRoute, bool) {
	for _, route := range routes {
		if strings.EqualFold(contract.HTTPMethod, route.route.HTTPMethod) && pathsCompatible(contract.Path, route.route.Path) {
			return route, true
		}
	}
	return workspaceBackendRoute{}, false
}

func pathCompatibleWorkspaceRoute(contract APIContractRecord, routes []workspaceBackendRoute) (workspaceBackendRoute, bool) {
	for _, route := range routes {
		if pathsCompatible(contract.Path, route.route.Path) {
			return route, true
		}
	}
	return workspaceBackendRoute{}, false
}

func hasBackendRoutes(routes []CodeRouteRecord) bool {
	for _, route := range routes {
		if route.Kind == "backend" {
			return true
		}
	}
	return false
}

func renderWorkspaceContextReport(record WorkspaceContextRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Context\n\n")
	b.WriteString(fmt.Sprintf("- Workspace root: `%s`\n", record.Root))
	if record.Current != "" {
		b.WriteString(fmt.Sprintf("- Current project: `%s`\n", record.Current))
	}
	b.WriteString("\n## Projects\n\n")
	for _, project := range record.Projects {
		b.WriteString(fmt.Sprintf("- `%s` - %s", project.Path, project.Status))
		if project.Service != "" {
			b.WriteString(fmt.Sprintf(", service `%s`", project.Service))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Loaded Indexes\n\n")
	if len(record.LoadedIndexes) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, project := range record.LoadedIndexes {
			b.WriteString(fmt.Sprintf("- `%s`\n", project.Path))
		}
	}
	b.WriteString("\n## Known Backend Services\n\n")
	if len(record.KnownServices) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, service := range record.KnownServices {
			b.WriteString(fmt.Sprintf("- `%s`\n", service))
		}
	}
	b.WriteString("\n## Referenced But Missing Services\n\n")
	if len(record.MissingServices) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, service := range record.MissingServices {
			b.WriteString(fmt.Sprintf("- `%s`\n", service))
		}
	}
	return b.String()
}

func renderWorkspaceContractMatchesReport(records []WorkspaceContractMatchRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Contract Matches\n\n")
	if len(records) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, record := range records {
		renderWorkspaceContractMatchLine(&b, record)
	}
	return b.String()
}

func renderProjectWorkspaceMatchesReport(projectPath string, records []WorkspaceContractMatchRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Contract Matches\n\n")
	count := 0
	for _, record := range records {
		if record.APIProject != projectPath && record.BackendProject != projectPath {
			continue
		}
		count++
		renderWorkspaceContractMatchLine(&b, record)
	}
	if count == 0 {
		b.WriteString("- none detected\n")
	}
	return b.String()
}

func renderWorkspaceContractMatchLine(b *strings.Builder, record WorkspaceContractMatchRecord) {
	if record.Issue == contractIssueMatched {
		service := emptyAsNone(record.BackendService)
		b.WriteString(fmt.Sprintf("- %s `%s` -> %s %s `%s` via `%s:%d` (%s, frontend `%s` `%s:%d`)\n",
			record.APIHTTPMethod,
			record.APIPath,
			service,
			record.BackendHTTPMethod,
			record.BackendPath,
			record.BackendFile,
			record.BackendLine,
			record.Confidence,
			record.APIProject,
			record.APIFile,
			record.APILine,
		))
		return
	}
	b.WriteString(fmt.Sprintf("- %s `%s` from `%s` `%s:%d`: %s (%s, %s)\n",
		record.APIHTTPMethod,
		record.APIPath,
		record.APIProject,
		record.APIFile,
		record.APILine,
		record.Issue,
		record.Confidence,
		record.Reason,
	))
}

func renderFrontendConsumersReport(projectPath string, records []WorkspaceContractMatchRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Frontend Consumers\n\n")
	count := 0
	for _, record := range records {
		if record.BackendProject != projectPath || record.Issue != contractIssueMatched {
			continue
		}
		count++
		b.WriteString(fmt.Sprintf("- %s `%s` used by `%s` `%s:%d` -> `%s.%s`\n",
			record.APIHTTPMethod,
			record.APIPath,
			record.APIProject,
			record.APIFile,
			record.APILine,
			record.BackendService,
			record.BackendHandler,
		))
	}
	if count == 0 {
		b.WriteString("- none detected\n")
	}
	return b.String()
}

// WorkspaceStatus renders the auto-detected workspace without scanning or writing files.
func WorkspaceStatus(root string, cfg config.Config) (string, error) {
	currentAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	workspaceRoot, ok, err := resolveWorkspaceRoot(currentAbs, cfg.WorkspaceRoot)
	if err != nil {
		return "", err
	}
	if !ok {
		return "No GoreGraph workspace detected.\n", nil
	}
	projects, err := discoverWorkspaceProjects(workspaceRoot, currentAbs, cfg.OutputDir)
	if err != nil {
		return "", err
	}
	indexed, err := loadWorkspaceIndexes(projects)
	if err != nil {
		return "", err
	}
	registry := WorkspaceRegistryRecord{
		Root:     filepath.ToSlash(workspaceRoot),
		Current:  workspaceRel(workspaceRoot, currentAbs),
		Projects: projects,
	}
	return renderWorkspaceContextReport(buildWorkspaceContext(registry, indexed)), nil
}

func readWorkspaceJSON(path string, dest any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func workspaceRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func workspaceFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
