package scan

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// DartPackageRecord contains allowlisted package metadata, never arbitrary configuration values.
type DartPackageRecord struct {
	Resolution   string                 `json:"resolution,omitempty"`
	Version      string                 `json:"version,omitempty"`
	Path         string                 `json:"path"`
	Name         string                 `json:"name,omitempty"`
	SDK          string                 `json:"sdk,omitempty"`
	Flutter      bool                   `json:"flutter,omitempty"`
	Dependencies []DartDependencyRecord `json:"dependencies,omitempty"`
	Assets       []DartAssetRecord      `json:"assets,omitempty"`
	Workspace    []string               `json:"workspace,omitempty"`
}

// DartDependencyRecord describes an explicitly declared Pub dependency source.
type DartDependencyRecord struct {
	Constraint string `json:"constraint,omitempty"`
	Name       string `json:"name"`
	Scope      string `json:"scope"`
	Path       string `json:"path,omitempty"`
	Source     string `json:"source,omitempty"`
	Line       int    `json:"line"`
}

// DartAssetRecord identifies a declared Flutter asset or font path.
type DartAssetRecord struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
}

var dartPackageNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var dartPackageVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?$`)

func extractDartPackage(file, body string) DartPackageRecord {
	p := DartPackageRecord{Path: file}
	section, subsection, dependency := "", "", ""
	for n, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(dartYAMLComment(line))
		if trimmed == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if strings.Contains(line[:indent], "\t") {
			continue
		}
		if indent == 0 {
			subsection = ""
			dependency = ""
			key, value, ok := strings.Cut(trimmed, ":")
			if !ok {
				section = ""
				continue
			}
			section = key
			if key == "resolution" && dartYAMLScalar(value) == "workspace" {
				p.Resolution = "workspace"
			}
			if key == "version" {
				version := dartYAMLScalar(value)
				if dartPackageVersionPattern.MatchString(version) {
					p.Version = version
				}
			}
			if key == "name" {
				name := dartYAMLScalar(value)
				if dartPackageNamePattern.MatchString(name) || name == "_" {
					p.Name = name
				}
			}
			continue
		}
		key, value, hasKey := strings.Cut(trimmed, ":")
		switch section {
		case "environment":
			if indent == 2 && key == "sdk" {
				p.SDK = dartYAMLScalar(value)
			}
		case "dependencies", "dev_dependencies", "dependency_overrides":
			if indent == 2 && hasKey && dartPackageNamePattern.MatchString(key) {
				dependency = key
				p.Dependencies = append(p.Dependencies, DartDependencyRecord{Name: key, Scope: section, Source: "hosted", Constraint: dartYAMLScalar(value), Line: n + 1})
				if key == "flutter" {
					p.Flutter = true
				}
				if strings.Contains(value, "path:") {
					p.Dependencies[len(p.Dependencies)-1].Source = "unsupported-inline"
				}
				continue
			}
			if indent == 4 && hasKey && dependency != "" && len(p.Dependencies) > 0 {
				d := &p.Dependencies[len(p.Dependencies)-1]
				if key == "path" {
					d.Path = dartYAMLScalar(value)
					d.Source = "path"
				}
				if key == "sdk" || key == "git" || key == "hosted" {
					d.Source = key
				}
			}
		case "flutter":
			p.Flutter = true
			if indent == 2 && hasKey {
				subsection = key
			}
			if subsection == "assets" && strings.HasPrefix(trimmed, "- ") {
				value := dartYAMLScalar(strings.TrimPrefix(trimmed, "- "))
				if !strings.ContainsAny(value, "{}*$") && value != "" {
					p.Assets = append(p.Assets, DartAssetRecord{Path: value, Kind: "asset", Line: n + 1})
				}
			}
			if subsection == "fonts" && strings.Contains(trimmed, "asset:") {
				_, value, _ := strings.Cut(trimmed, "asset:")
				value = dartYAMLScalar(value)
				if value != "" && !strings.ContainsAny(value, "{}*$") {
					p.Assets = append(p.Assets, DartAssetRecord{Path: value, Kind: "font", Line: n + 1})
				}
			}
		case "workspace":
			if strings.HasPrefix(trimmed, "- ") {
				value := dartYAMLScalar(strings.TrimPrefix(trimmed, "- "))
				if value != "" && !strings.ContainsAny(value, "{}$") {
					p.Workspace = append(p.Workspace, value)
				}
			}
		}
	}
	return p
}
func dartYAMLComment(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			if c == '\\' && quote == '"' {
				i++
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
		}
		if c == '#' && (i == 0 || line[i-1] == ' ') {
			return line[:i]
		}
	}
	return line
}
func dartYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '\'' && value[len(value)-1] == '\'' || value[0] == '"' && value[len(value)-1] == '"') {
		return value[1 : len(value)-1]
	}
	if strings.HasPrefix(value, "*") || strings.ContainsAny(value, "{}[]&!|>") {
		return ""
	}
	return value
}

func dartResolveURI(s dartSource, uri string, packages []DartPackageRecord, sourceFiles map[string]int) string {
	if strings.HasPrefix(uri, "dart:") {
		return ""
	}
	if strings.HasPrefix(uri, "package:") {
		rest := strings.TrimPrefix(uri, "package:")
		name, sub, ok := strings.Cut(rest, "/")
		if !ok || sub == "" || strings.Contains(sub, "\\") || strings.HasPrefix(path.Clean(sub), "../") {
			return ""
		}
		for _, pkg := range packages {
			if path.Clean(path.Dir(pkg.Path)) != path.Clean(s.packageRoot) && pkg.Resolution != "workspace" {
				continue
			}
			for _, dep := range pkg.Dependencies {
				if dep.Name == name && dep.Scope == "dependency_overrides" {
					return ""
				}
			}
		}
		var roots []string
		for _, p := range packages {
			if p.Name != name {
				continue
			}
			root := path.Dir(p.Path)
			if root == "." {
				root = ""
			}
			visible := s.packageName == name && s.packageRoot == root
			if !visible {
				for _, from := range packages {
					if path.Dir(from.Path) != path.Dir(path.Join(s.packageRoot, "pubspec.yaml")) {
						continue
					}
					for _, d := range from.Dependencies {
						if d.Name == name && d.Source == "path" && path.Clean(path.Join(s.packageRoot, d.Path)) == path.Clean(root) {
							visible = true
						}
						if d.Name == name && d.Source == "hosted" && from.Resolution == "workspace" && p.Resolution == "workspace" && dartCompatibleVersion(d.Constraint, p.Version) && dartSharedWorkspace(from, p, packages) {
							visible = true
						}
					}
				}
			}
			if visible {
				candidate := path.Join(root, "lib", sub)
				if _, exists := sourceFiles[candidate]; exists {
					roots = append(roots, candidate)
				}
			}
		}
		if len(roots) == 1 {
			return roots[0]
		}
		return ""
	}
	if strings.Contains(uri, ":") || strings.HasPrefix(uri, "/") || strings.Contains(uri, "\\") {
		return ""
	}
	candidate := path.Clean(path.Join(path.Dir(s.file), uri))
	if candidate == ".." || strings.HasPrefix(candidate, "../") {
		return ""
	}
	if s.packageRoot != "" && !strings.HasPrefix(candidate, s.packageRoot+"/") {
		return ""
	}
	if _, exists := sourceFiles[candidate]; exists {
		return candidate
	}
	return ""
}

func dartSharedWorkspace(from, target DartPackageRecord, packages []DartPackageRecord) bool {
	for _, workspace := range packages {
		if len(workspace.Workspace) == 0 {
			continue
		}
		members := map[string]bool{workspace.Path: true}
		queue := []DartPackageRecord{workspace}
		for len(queue) > 0 && len(members) <= len(packages) {
			current := queue[0]
			queue = queue[1:]
			root := path.Dir(current.Path)
			for _, pattern := range current.Workspace {
				if path.IsAbs(pattern) || strings.Contains(pattern, "\\") || strings.HasPrefix(path.Clean(pattern), "../") || strings.Contains(pattern, "**") {
					continue
				}
				for _, candidate := range packages {
					candidateRoot := path.Dir(candidate.Path)
					relative, ok := strings.CutPrefix(candidateRoot, strings.TrimSuffix(root, ".")+"/")
					if root == "." {
						relative = candidateRoot
						ok = true
					}
					if !ok {
						continue
					}
					match, err := path.Match(pattern, relative)
					if err == nil && match && candidate.Resolution == "workspace" && !members[candidate.Path] {
						members[candidate.Path] = true
						queue = append(queue, candidate)
					}
				}
			}
		}
		if members[from.Path] && members[target.Path] {
			return true
		}
	}
	return false
}
func dartCompatibleVersion(constraint, version string) bool {
	parse := func(value string) ([3]int, bool) {
		var parts [3]int
		chunks := strings.Split(value, ".")
		if len(chunks) != 3 {
			return parts, false
		}
		for i, chunk := range chunks {
			number, err := strconv.Atoi(chunk)
			if err != nil || number < 0 {
				return parts, false
			}
			parts[i] = number
		}
		return parts, true
	}
	target, ok := parse(version)
	if !ok {
		return false
	}
	caret := strings.HasPrefix(constraint, "^")
	minimum, ok := parse(strings.TrimPrefix(constraint, "^"))
	if !ok {
		return false
	}
	if !caret {
		return target == minimum
	}
	for i := 0; i < 3; i++ {
		if target[i] < minimum[i] {
			return false
		}
		if target[i] > minimum[i] {
			break
		}
	}
	if minimum[0] > 0 {
		return target[0] == minimum[0]
	}
	if minimum[1] > 0 {
		return target[0] == 0 && target[1] == minimum[1]
	}
	return target == minimum
}
