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
}

func extractWorkspaceRecord(file FileRecord, body string) WorkspaceIndex {
	switch fileBase(file.Path) {
	case "pom.xml":
		if record, ok := extractMavenPackage(file.Path, body); ok {
			return WorkspaceIndex{MavenPackages: []MavenPackageRecord{record}}
		}
	case "package.json":
		if record, ok := extractNodePackage(file.Path, body); ok {
			return WorkspaceIndex{NodePackages: []NodePackageRecord{record}}
		}
	}
	return WorkspaceIndex{}
}

func mergeWorkspaceIndex(index *WorkspaceIndex, add WorkspaceIndex) {
	index.MavenPackages = append(index.MavenPackages, add.MavenPackages...)
	index.NodePackages = append(index.NodePackages, add.NodePackages...)
	sort.Slice(index.MavenPackages, func(i, j int) bool { return index.MavenPackages[i].Path < index.MavenPackages[j].Path })
	sort.Slice(index.NodePackages, func(i, j int) bool { return index.NodePackages[i].Path < index.NodePackages[j].Path })
}

func extractMavenPackage(path, body string) (MavenPackageRecord, bool) {
	var project pomProject
	if err := xml.Unmarshal([]byte(body), &project); err != nil {
		return MavenPackageRecord{}, false
	}
	record := MavenPackageRecord{
		Path:       path,
		GroupID:    strings.TrimSpace(project.GroupID),
		ArtifactID: strings.TrimSpace(project.ArtifactID),
		Version:    strings.TrimSpace(project.Version),
	}
	if record.GroupID == "" {
		record.GroupID = strings.TrimSpace(project.Parent.GroupID)
	}
	if record.Version == "" {
		record.Version = strings.TrimSpace(project.Parent.Version)
	}
	if project.Parent.ArtifactID != "" {
		record.Parent = strings.TrimSpace(project.Parent.GroupID + ":" + project.Parent.ArtifactID + ":" + project.Parent.Version)
	}
	for _, dependency := range project.Dependencies {
		dep := MavenDependencyRecord{
			GroupID:    strings.TrimSpace(dependency.GroupID),
			ArtifactID: strings.TrimSpace(dependency.ArtifactID),
			Version:    strings.TrimSpace(dependency.Version),
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
	}
	return record, record.Name != "" || len(record.Scripts) > 0 || len(record.Workspaces) > 0
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
