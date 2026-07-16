package scan

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

type pomProject struct {
	XMLName      xml.Name        `xml:"project"`
	GroupID      string          `xml:"groupId"`
	ArtifactID   string          `xml:"artifactId"`
	Version      string          `xml:"version"`
	Parent       pomParent       `xml:"parent"`
	Dependencies []pomDependency `xml:"dependencies>dependency"`
}

type pomParent struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

type pomDependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
}

type packageJSON struct {
	Name             string         `json:"name"`
	Version          string         `json:"version"`
	Private          bool           `json:"private"`
	PackageManager   string         `json:"packageManager"`
	Workspaces       any            `json:"workspaces"`
	Scripts          map[string]any `json:"scripts"`
	Dependencies     map[string]any `json:"dependencies"`
	DevDependencies  map[string]any `json:"devDependencies"`
	PeerDependencies map[string]any `json:"peerDependencies"`
	Exports          any            `json:"exports"`
	Types            string         `json:"types"`
	Typings          string         `json:"typings"`
}

func extractWorkspaceRecord(file FileRecord, body string) WorkspaceIndex {
	switch fileBase(file.Path) {
	case "pom.xml":
		result := WorkspaceIndex{mavenLimitations: mavenExtractionLimitations(file.Path, body)}
		if record, ok := extractMavenPackage(file.Path, body); ok || record.GroupID != "" || record.ArtifactID != "" {
			result.MavenPackages = []MavenPackageRecord{record}
		}
		return result
	case "package.json":
		if record, ok := extractNodePackage(file.Path, body); ok {
			return WorkspaceIndex{NodePackages: []NodePackageRecord{record}}
		}
	case "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts":
		result := WorkspaceIndex{gradleLimitations: gradleExtractionLimitations(file.Path, body)}
		if record, ok := extractGradlePackage(file.Path, body); ok {
			result.GradlePackages = []GradlePackageRecord{record}
		}
		return result
	}
	return WorkspaceIndex{}
}

func mergeWorkspaceIndex(index *WorkspaceIndex, add WorkspaceIndex) {
	index.MavenPackages = append(index.MavenPackages, add.MavenPackages...)
	index.GradlePackages = append(index.GradlePackages, add.GradlePackages...)
	index.NodePackages = append(index.NodePackages, add.NodePackages...)
	index.gradleLimitations = append(index.gradleLimitations, add.gradleLimitations...)
	index.mavenLimitations = append(index.mavenLimitations, add.mavenLimitations...)
	sort.Slice(index.MavenPackages, func(i, j int) bool { return index.MavenPackages[i].Path < index.MavenPackages[j].Path })
	sort.Slice(index.GradlePackages, func(i, j int) bool { return index.GradlePackages[i].Path < index.GradlePackages[j].Path })
	sort.Slice(index.NodePackages, func(i, j int) bool { return index.NodePackages[i].Path < index.NodePackages[j].Path })
	sort.Strings(index.gradleLimitations)
	sort.Strings(index.mavenLimitations)
}

func extractMavenPackage(path, body string) (MavenPackageRecord, bool) {
	var project pomProject
	if err := xml.Unmarshal([]byte(body), &project); err != nil {
		return MavenPackageRecord{}, false
	}
	groupID := strings.TrimSpace(project.GroupID)
	if groupID == "" {
		groupID = strings.TrimSpace(project.Parent.GroupID)
	}
	record := MavenPackageRecord{
		Path:       path,
		GroupID:    literalMavenCoordinate(groupID),
		ArtifactID: literalMavenCoordinate(project.ArtifactID),
		Version:    literalMavenCoordinate(project.Version),
	}
	if record.Version == "" {
		record.Version = literalMavenCoordinate(project.Parent.Version)
	}
	parentGroup := literalMavenCoordinate(project.Parent.GroupID)
	parentArtifact := literalMavenCoordinate(project.Parent.ArtifactID)
	parentVersion := literalMavenCoordinate(project.Parent.Version)
	if parentGroup != "" && parentArtifact != "" {
		record.Parent = strings.Trim(parentGroup+":"+parentArtifact+":"+parentVersion, ":")
	}
	for _, dependency := range project.Dependencies {
		dep := MavenDependencyRecord{
			GroupID:    literalMavenCoordinate(dependency.GroupID),
			ArtifactID: literalMavenCoordinate(dependency.ArtifactID),
			Version:    literalMavenCoordinate(dependency.Version),
			Scope:      strings.TrimSpace(dependency.Scope),
		}
		if dep.GroupID != "" && dep.ArtifactID != "" {
			record.Dependencies = append(record.Dependencies, dep)
		}
	}
	sort.Slice(record.Dependencies, func(i, j int) bool {
		if record.Dependencies[i].GroupID != record.Dependencies[j].GroupID {
			return record.Dependencies[i].GroupID < record.Dependencies[j].GroupID
		}
		return record.Dependencies[i].ArtifactID < record.Dependencies[j].ArtifactID
	})
	return record, record.ArtifactID != ""
}

func mavenExtractionLimitations(path, body string) []string {
	var project pomProject
	if err := xml.Unmarshal([]byte(body), &project); err != nil {
		return nil
	}
	groupID := strings.TrimSpace(project.GroupID)
	if groupID == "" {
		groupID = strings.TrimSpace(project.Parent.GroupID)
	}
	var limitations []string
	if groupID != "" && literalMavenCoordinate(groupID) == "" {
		limitations = append(limitations, path+": computed Maven group is not statically resolved")
	}
	artifactID := strings.TrimSpace(project.ArtifactID)
	if artifactID != "" && literalMavenCoordinate(artifactID) == "" {
		limitations = append(limitations, path+": computed Maven artifact is not statically resolved")
	}
	return limitations
}

func literalMavenCoordinate(value string) string {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "$") {
		return ""
	}
	return value
}

func extractNodePackage(path, body string) (NodePackageRecord, bool) {
	var pkg packageJSON
	if err := json.Unmarshal([]byte(body), &pkg); err != nil {
		return NodePackageRecord{}, false
	}
	record := NodePackageRecord{
		Path:           path,
		Name:           pkg.Name,
		Version:        pkg.Version,
		Private:        pkg.Private,
		PackageManager: pkg.PackageManager,
		Workspaces:     normalizeWorkspaces(pkg.Workspaces),
		Scripts:        sortedKeys(pkg.Scripts),
		Dependencies:   sortedDependencyKeys(pkg.Dependencies, pkg.DevDependencies, pkg.PeerDependencies),
		Exports:        normalizePackageExports(pkg.Exports),
		Types:          strings.TrimSpace(pkg.Types),
	}
	if record.Types == "" {
		record.Types = strings.TrimSpace(pkg.Typings)
	}
	return record, record.Name != "" || len(record.Scripts) > 0 || len(record.Workspaces) > 0
}

func normalizePackageExports(value any) map[string][]string {
	if value == nil {
		return nil
	}
	result := map[string][]string{}
	if text, ok := value.(string); ok {
		result["."] = []string{text}
		return result
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	hasSubpaths := false
	for key := range object {
		if key == "." || strings.HasPrefix(key, "./") {
			hasSubpaths = true
			break
		}
	}
	if !hasSubpaths {
		if leaves := staticStringLeaves(object); len(leaves) > 0 {
			result["."] = leaves
		}
		return result
	}
	for key, raw := range object {
		if key != "." && !strings.HasPrefix(key, "./") {
			continue
		}
		if leaves := staticStringLeaves(raw); len(leaves) > 0 {
			result[key] = leaves
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func staticStringLeaves(value any) []string {
	seen := map[string]bool{}
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case string:
			if typed != "" {
				seen[typed] = true
			}
		case []any:
			for _, item := range typed {
				visit(item)
			}
		case map[string]any:
			for _, item := range typed {
				visit(item)
			}
		}
	}
	visit(value)
	result := make([]string, 0, len(seen))
	for item := range seen {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func sortedDependencyKeys(groups ...map[string]any) []string {
	seen := map[string]bool{}
	for _, group := range groups {
		for name := range group {
			seen[name] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for name := range seen {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func normalizeWorkspaces(value any) []string {
	switch v := value.(type) {
	case []any:
		var workspaces []string
		for _, item := range v {
			if text, ok := item.(string); ok {
				workspaces = append(workspaces, text)
			}
		}
		sort.Strings(workspaces)
		return workspaces
	case map[string]any:
		if packages, ok := v["packages"].([]any); ok {
			return normalizeWorkspaces(packages)
		}
	}
	return nil
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func renderWorkspaceReport(index WorkspaceIndex) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace\n\n")
	if len(index.MavenPackages) == 0 && len(index.NodePackages) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	if len(index.MavenPackages) > 0 {
		b.WriteString("## Maven Packages\n\n")
		for _, pkg := range index.MavenPackages {
			coords := strings.Trim(pkg.GroupID+":"+pkg.ArtifactID+":"+pkg.Version, ":")
			b.WriteString(fmt.Sprintf("- `%s` - `%s`\n", pkg.Path, coords))
		}
		b.WriteString("\n")
	}
	if len(index.NodePackages) > 0 {
		b.WriteString("## Node Packages\n\n")
		for _, pkg := range index.NodePackages {
			b.WriteString(fmt.Sprintf("- `%s`", pkg.Path))
			if pkg.Name != "" {
				b.WriteString(fmt.Sprintf(" - `%s`", pkg.Name))
			}
			if pkg.PackageManager != "" {
				b.WriteString(fmt.Sprintf(" - package manager `%s`", pkg.PackageManager))
			}
			if len(pkg.Scripts) > 0 {
				b.WriteString(fmt.Sprintf(" - scripts `%s`", strings.Join(pkg.Scripts, "`, `")))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
