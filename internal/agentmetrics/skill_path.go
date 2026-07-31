package agentmetrics

import (
	pathpkg "path"
	"strings"
	"unicode"
)

// ClassifyExternalSkillTarget resolves target from commandDirectory and reports
// normalized skill-bundle targets outside workspace.
func ClassifyExternalSkillTarget(
	workspace string,
	commandDirectory string,
	target string,
) (string, bool) {
	workspacePath, workspaceWindows, ok := normalizeAbsolutePath(workspace)
	if !ok {
		return "", false
	}
	commandPath, commandWindows, ok := normalizeAbsolutePath(commandDirectory)
	if !ok || commandWindows != workspaceWindows {
		return "", false
	}
	target = strings.TrimSpace(strings.ReplaceAll(target, `\`, "/"))
	if target == "" {
		return "", false
	}
	if !isAbsoluteComparablePath(target) {
		target = pathpkg.Join(commandPath, target)
	}
	targetPath, targetWindows, ok := normalizeAbsolutePath(target)
	if !ok || targetWindows != workspaceWindows || pathWithin(targetPath, workspacePath) {
		return "", false
	}
	if !isSkillBundlePath(targetPath, targetWindows) {
		return "", false
	}
	return targetPath, true
}

func normalizeAbsolutePath(value string) (string, bool, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	if !isAbsoluteComparablePath(value) {
		return "", false, false
	}
	windows := len(value) >= 3 && unicode.IsLetter(rune(value[0])) && value[1] == ':'
	value = pathpkg.Clean(value)
	if windows {
		value = strings.ToLower(value)
	}
	return value, windows, true
}

func isAbsoluteComparablePath(value string) bool {
	if strings.HasPrefix(value, "/") {
		return true
	}
	return len(value) >= 3 && unicode.IsLetter(rune(value[0])) && value[1] == ':' && value[2] == '/'
}

func pathWithin(candidate, root string) bool {
	return candidate == root || strings.HasPrefix(candidate, strings.TrimSuffix(root, "/")+"/")
}

func isSkillBundlePath(value string, windows bool) bool {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	for _, part := range parts {
		if windows && (strings.EqualFold(part, "SKILL.md") || strings.EqualFold(part, "skills")) {
			return true
		}
		if !windows && (part == "SKILL.md" || part == "skills") {
			return true
		}
	}
	return false
}
