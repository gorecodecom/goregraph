package scan

import (
	"path"
	"sort"
	"strings"
	"unicode"
)

var serviceIdentityWrapperTokens = map[string]struct{}{
	"api":        {},
	"client":     {},
	"connector":  {},
	"gateway":    {},
	"management": {},
	"mgmt":       {},
	"ms":         {},
	"service":    {},
	"services":   {},
	"svc":        {},
}

func canonicalServiceIdentityVariants(value string) []string {
	tokens := serviceIdentityTokens(value)
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, wrapper := serviceIdentityWrapperTokens[token]; wrapper {
			continue
		}
		filtered = append(filtered, token)
	}
	if len(filtered) == 0 {
		return nil
	}

	variants := map[string]struct{}{
		strings.Join(filtered, ""): {},
	}
	singular := singularServiceIdentityToken(filtered[len(filtered)-1])
	if singular != filtered[len(filtered)-1] {
		singularTokens := append([]string(nil), filtered...)
		singularTokens[len(singularTokens)-1] = singular
		variants[strings.Join(singularTokens, "")] = struct{}{}
	}

	result := make([]string, 0, len(variants))
	for variant := range variants {
		result = append(result, variant)
	}
	sort.Strings(result)
	return result
}

func serviceIdentityTokens(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	runes := []rune(value)
	var normalized strings.Builder
	for index, current := range runes {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			normalized.WriteByte(' ')
			continue
		}
		if index > 0 && unicode.IsUpper(current) {
			previous := runes[index-1]
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || unicode.IsUpper(previous) && nextIsLower {
				normalized.WriteByte(' ')
			}
		}
		normalized.WriteRune(unicode.ToLower(current))
	}
	return strings.Fields(normalized.String())
}

func singularServiceIdentityToken(token string) string {
	switch {
	case len(token) > 3 && strings.HasSuffix(token, "ies"):
		return strings.TrimSuffix(token, "ies") + "y"
	case len(token) > 4 && (strings.HasSuffix(token, "sses") || strings.HasSuffix(token, "shes") || strings.HasSuffix(token, "ches") || strings.HasSuffix(token, "xes") || strings.HasSuffix(token, "zes")):
		return strings.TrimSuffix(token, "es")
	case len(token) > 2 && strings.HasSuffix(token, "s") && !strings.HasSuffix(token, "ss"):
		return strings.TrimSuffix(token, "s")
	default:
		return token
	}
}

func resolveWorkspaceProjectByServiceKey(projects []WorkspaceProjectRecord, key string) (WorkspaceProjectRecord, []string, bool) {
	keyVariants := canonicalServiceIdentityVariants(key)
	if len(keyVariants) == 0 {
		return WorkspaceProjectRecord{}, nil, false
	}
	keySet := make(map[string]struct{}, len(keyVariants))
	for _, variant := range keyVariants {
		keySet[variant] = struct{}{}
	}

	matches := make(map[string]WorkspaceProjectRecord)
	for _, project := range projects {
		if !serviceIdentityVariantsIntersect(keySet, workspaceProjectServiceIdentityVariants(project)) {
			continue
		}
		candidate := workspaceProjectIdentity(project)
		if candidate == "" {
			continue
		}
		matches[candidate] = project
	}

	candidates := make([]string, 0, len(matches))
	for candidate := range matches {
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)
	if len(candidates) != 1 {
		return WorkspaceProjectRecord{}, candidates, false
	}
	return matches[candidates[0]], candidates, true
}

func workspaceProjectServiceIdentityVariants(project WorkspaceProjectRecord) []string {
	values := []string{project.Service, project.Name}
	if project.Path != "" {
		normalizedPath := strings.ReplaceAll(project.Path, "\\", "/")
		values = append(values, path.Base(path.Clean(normalizedPath)))
	}

	unique := make(map[string]struct{})
	for _, value := range values {
		for _, variant := range canonicalServiceIdentityVariants(value) {
			unique[variant] = struct{}{}
		}
	}
	variants := make([]string, 0, len(unique))
	for variant := range unique {
		variants = append(variants, variant)
	}
	sort.Strings(variants)
	return variants
}

func serviceIdentityVariantsIntersect(keySet map[string]struct{}, projectVariants []string) bool {
	for _, variant := range projectVariants {
		if _, ok := keySet[variant]; ok {
			return true
		}
	}
	return false
}

func workspaceProjectIdentity(project WorkspaceProjectRecord) string {
	if project.Path != "" {
		return project.Path
	}
	if project.Name != "" {
		return project.Name
	}
	return project.Service
}
