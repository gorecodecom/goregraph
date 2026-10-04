package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
	"go.yaml.in/yaml/v3"
)

// APISpecificationIndexRecord supplements the workspace without adding code projects.
type APISpecificationIndexRecord struct {
	SchemaVersion     int                      `json:"schema_version"`
	Generated         string                   `json:"generated,omitempty"`
	Enabled           bool                     `json:"enabled"`
	InventoryComplete bool                     `json:"inventory_complete"`
	Issues            []string                 `json:"issues,omitempty"`
	Documents         []APISpecificationRecord `json:"documents"`
}

var specificationReadFile = os.ReadFile

// WorkspaceAPISpecificationFingerprint observes contract inputs without parsing or writes.
func WorkspaceAPISpecificationFingerprint(ctx context.Context, root string, projectRoots ...string) (string, error) {
	_, fingerprint, err := snapshotWorkspaceAPISpecifications(ctx, root, false, projectRoots...)
	return fingerprint, err
}

func snapshotWorkspaceAPISpecifications(ctx context.Context, root string, parse bool, projectRoots ...string) (APISpecificationIndexRecord, string, error) {
	index := APISpecificationIndexRecord{SchemaVersion: SchemaVersion, Enabled: true, InventoryComplete: true, Documents: []APISpecificationRecord{}}
	if err := ctx.Err(); err != nil {
		return index, "", err
	}
	enabled, err := workspaceAPISpecificationsEnabled(root)
	if err != nil {
		index.InventoryComplete, index.Issues = false, []string{"workspace_configuration_unavailable"}
		return index, "workspace_configuration_unavailable", nil
	}
	index.Enabled = enabled
	if !enabled {
		return index, "disabled", nil
	}
	cfg, err := config.Load(root)
	if err != nil {
		index.InventoryComplete, index.Issues = false, []string{"workspace_selection_unavailable"}
		return index, "workspace_selection_unavailable", nil
	}
	cfg.UpdateGitignore, cfg.FollowSymlinks = false, false
	repositories := map[string]string{}
	repositoryConfigs := map[string]config.Config{}
	unavailableRepositories := map[string]bool{}
	contractRepositories := map[string]bool{}
	var parts []string
	report, err := WalkProjectFiles(ctx, root, cfg, func(file WalkedFile) error {
		switch strings.ToLower(filepath.Ext(file.Path)) {
		case ".json", ".yml", ".yaml":
		default:
			return nil
		}
		repository := specificationRepository(root, filepath.Dir(filepath.Join(root, filepath.FromSlash(file.Path))), repositories)
		named := strings.Contains(strings.ToLower(filepath.Base(file.Path)), "swagger") || strings.Contains(strings.ToLower(filepath.Base(file.Path)), "openapi")
		selectionRoot := repository
		absolute := filepath.Join(root, filepath.FromSlash(file.Path))
		for _, projectRoot := range projectRoots {
			if strings.HasPrefix(absolute, projectRoot+string(filepath.Separator)) && len(projectRoot) > len(selectionRoot) {
				selectionRoot = projectRoot
			}
		}
		if selectionRoot != "" {
			repositoryConfig, ok := repositoryConfigs[selectionRoot]
			if !ok {
				repositoryConfig, err = config.Load(selectionRoot)
				if err != nil {
					unavailableRepositories[selectionRoot] = true
				}
				repositoryConfigs[selectionRoot] = repositoryConfig
			}
			if unavailableRepositories[selectionRoot] {
				index.InventoryComplete = false
				index.Issues = sortedUniqueStrings(append(index.Issues, "repository_selection_unavailable"))
				parts = append(parts, file.Path+":repository_selection_unavailable")
				return nil
			}
			rel := workspaceRel(selectionRoot, absolute)
			if shouldSkipPath(rel, false, repositoryConfig, gitignore.Matcher{}) || file.Size > repositoryConfig.MaxFileSizeBytes || rel == repositoryConfig.OutputDir || strings.HasPrefix(rel, repositoryConfig.OutputDir+"/") {
				return nil
			}
		}
		body, err := specificationReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			index.InventoryComplete = false
			index.Issues = sortedUniqueStrings(append(index.Issues, "source_unreadable"))
			parts = append(parts, fmt.Sprintf("%s:source_unreadable:%d", file.Path, file.Size))
			if parse && named {
				record := APISpecificationRecord{File: file.Path, Status: "invalid", Limitations: []string{"source_unreadable"}}
				if repository != "" {
					record.Repository = workspaceRel(root, repository)
				}
				index.Documents = append(index.Documents, record)
			}
			return nil
		}
		if !named && !apiSpecificationHeader.Match(body) {
			return nil
		}
		record := APISpecificationRecord{File: file.Path}
		if parse {
			var recognized bool
			record, recognized = parseAPISpecification(file.Path, body)
			if !recognized {
				// The fingerprint still covers candidates that cease to be contracts.
				record = APISpecificationRecord{File: file.Path}
			}
		}
		digest := sha256.Sum256(body)
		record.SourceHash = hex.EncodeToString(digest[:])
		if repository != "" {
			record.Repository = workspaceRel(root, repository)
		}
		if selectionRoot != "" {
			contractRepositories[selectionRoot] = true
		}
		parts = append(parts, record.File+"\x00"+record.Repository+"\x00"+record.SourceHash)
		if parse && record.Status != "" {
			index.Documents = append(index.Documents, record)
		}
		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return index, "", ctx.Err()
		}
		index.InventoryComplete = false
		index.Issues = sortedUniqueStrings(append(index.Issues, "inventory_unavailable"))
		parts = append(parts, "inventory_unavailable")
	}
	if len(parts) == 0 {
		return index, "", nil
	}
	selection, _ := json.Marshal(cfg)
	parts = append(parts, string(selection), report.IgnoreDigest)
	for repositoryRoot := range contractRepositories {
		repositoryConfig := repositoryConfigs[repositoryRoot]
		selection, _ := json.Marshal(repositoryConfig)
		parts = append(parts, workspaceRel(root, repositoryRoot)+string(selection))
	}
	sort.Strings(parts)
	return index, semanticFingerprint(parts), nil
}

func workspaceAPISpecificationProjectRoots(projects []WorkspaceProjectRecord) []string {
	roots := make([]string, 0, len(projects))
	for _, project := range projects {
		if project.AbsPath != "" {
			roots = append(roots, project.AbsPath)
		}
	}
	return roots
}

func workspaceAPISpecificationsEnabled(root string) (bool, error) {
	body, err := os.ReadFile(filepath.Join(root, ".goregraph-workspace.yml"))
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(body, &document); err != nil {
		return false, fmt.Errorf("invalid workspace configuration")
	}
	if len(document.Content) == 0 {
		return true, nil
	}
	value := specificationValue(document.Content[0], "api_specifications")
	if value == nil {
		return true, nil
	}
	if value.Kind != yaml.ScalarNode || value.Tag != "!!bool" || value.Value != "true" && value.Value != "false" {
		return false, fmt.Errorf("api_specifications must be true or false")
	}
	return value.Value == "true", nil
}

func specificationRepository(root, directory string, cache map[string]string) string {
	if cached, ok := cache[directory]; ok {
		return cached
	}
	marker, err := os.Lstat(filepath.Join(directory, ".git"))
	if err == nil && (marker.IsDir() || marker.Mode().IsRegular()) {
		cache[directory] = directory
		return directory
	}
	parent := filepath.Dir(directory)
	owner := ""
	if directory != root && parent != directory {
		owner = specificationRepository(root, parent, cache)
	}
	cache[directory] = owner
	return owner
}

var specificationPathParameter = regexp.MustCompile(`\{[A-Za-z_][A-Za-z0-9_]*\}`)

func linkWorkspaceAPISpecifications(index *APISpecificationIndexRecord, projects []workspaceIndexProject) {
	routes := map[string][]APISpecificationRoute{}
	seen := map[string]bool{}
	add := func(project string, route CodeRouteRecord) {
		if route.Kind != "backend" || route.File == "" || route.Line < 1 || !strings.HasPrefix(route.Path, "/") {
			return
		}
		key := strings.ToUpper(route.HTTPMethod) + " " + specificationPathParameter.ReplaceAllString(route.Path, "{}")
		evidence := APISpecificationRoute{Project: project, File: route.File, Line: route.Line, Path: route.Path}
		identity, _ := json.Marshal(evidence)
		if !seen[key+string(identity)] {
			routes[key] = append(routes[key], evidence)
			seen[key+string(identity)] = true
		}
	}
	for _, project := range projects {
		for _, route := range project.routes {
			add(project.record.Path, route)
		}
		for _, endpoint := range project.endpoints {
			add(project.record.Path, CodeRouteRecord{Kind: "backend", HTTPMethod: endpoint.HTTPMethod, Path: endpoint.Path, File: endpoint.File, Line: endpoint.Line})
		}
	}
	for i := range index.Documents {
		document := &index.Documents[i]
		for _, project := range projects {
			if strings.HasPrefix(document.File, project.record.Path+"/") || project.record.Path == "." {
				if len(project.record.Path) > len(document.OwnerProject) {
					document.OwnerProject = project.record.Path
				}
			}
		}
		for j := range document.Operations {
			operation := &document.Operations[j]
			operation.CodeRoutes, operation.LinkStatus, operation.MatchBasis = nil, "unlinked", ""
			if slices.Contains(document.Limitations, "aliases_not_evaluated") {
				continue
			}
			projects := map[string]bool{}
			for _, base := range operation.basePaths {
				key := operation.Method + " " + specificationPathParameter.ReplaceAllString(base+operation.Path, "{}")
				for _, route := range routes[key] {
					if !specificationRoutePresent(operation.CodeRoutes, route) {
						operation.CodeRoutes = append(operation.CodeRoutes, route)
						projects[route.Project] = true
					}
				}
			}
			if len(operation.CodeRoutes) > 0 {
				operation.MatchBasis = "resolved_path"
			} else if len(operation.basePaths) > 0 && !slices.Contains(operation.basePaths, "") {
				// A documented server prefix can differ from the controller's local path.
				// Keep these candidates separate from a fully matching endpoint.
				key := operation.Method + " " + specificationPathParameter.ReplaceAllString(operation.Path, "{}")
				for _, route := range routes[key] {
					operation.CodeRoutes = append(operation.CodeRoutes, route)
					projects[route.Project] = true
				}
				if len(operation.CodeRoutes) > 0 {
					operation.MatchBasis = "document_path"
				}
			}
			sort.Slice(operation.CodeRoutes, func(a, b int) bool {
				left, right := operation.CodeRoutes[a], operation.CodeRoutes[b]
				return fmt.Sprintf("%s:%s:%09d", left.Project, left.File, left.Line) < fmt.Sprintf("%s:%s:%09d", right.Project, right.File, right.Line)
			})
			operation.LinkStatus = "unlinked"
			if len(projects) == 1 {
				operation.LinkStatus = "unique_route"
				if operation.MatchBasis == "document_path" {
					operation.LinkStatus = "prefix_candidate"
				}
			} else if len(projects) > 1 {
				operation.LinkStatus = "ambiguous"
			}
		}
	}
}

func specificationRoutePresent(routes []APISpecificationRoute, candidate APISpecificationRoute) bool {
	for _, route := range routes {
		if route == candidate {
			return true
		}
	}
	return false
}
