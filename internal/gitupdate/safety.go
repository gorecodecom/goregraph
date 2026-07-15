package gitupdate

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type safetyFinding struct {
	reason      string
	remediation string
}

func inspectLocalSafety(root string) safetyFinding {
	configPaths, commonDirectory, err := localConfigPaths(root)
	if err != nil {
		return safetyInspectionFailure("repository Git configuration", err)
	}
	for _, configPath := range configPaths {
		finding, inspectErr := inspectLocalConfig(configPath)
		if inspectErr != nil {
			return safetyInspectionFailure("repository Git configuration", inspectErr)
		}
		if finding.reason != "" {
			return finding
		}
	}

	if finding := inspectAttributesFile(filepath.Join(commonDirectory, "info", "attributes")); finding.reason != "" {
		return finding
	}
	finding, err := inspectWorkingTreeAttributes(root)
	if err != nil {
		return safetyInspectionFailure("working-tree .gitattributes", err)
	}
	return finding
}

func localConfigPaths(root string) ([]string, string, error) {
	gitDirectory, err := resolveGitDirectory(root)
	if err != nil {
		return nil, "", err
	}
	commonDirectory := gitDirectory
	if contents, readErr := os.ReadFile(filepath.Join(gitDirectory, "commondir")); readErr == nil {
		commonDirectory = resolveRelativePath(gitDirectory, strings.TrimSpace(string(contents)))
	} else if !os.IsNotExist(readErr) {
		return nil, "", fmt.Errorf("read commondir: %w", readErr)
	}

	paths := []string{filepath.Join(commonDirectory, "config")}
	worktreeConfig := filepath.Join(gitDirectory, "config.worktree")
	if worktreeConfig != paths[0] {
		paths = append(paths, worktreeConfig)
	}
	return paths, commonDirectory, nil
}

func resolveGitDirectory(root string) (string, error) {
	dotGit := filepath.Join(root, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", fmt.Errorf("inspect .git: %w", err)
	}
	if info.IsDir() {
		return dotGit, nil
	}

	contents, err := os.ReadFile(dotGit)
	if err != nil {
		return "", fmt.Errorf("read .git file: %w", err)
	}
	const prefix = "gitdir:"
	line := strings.TrimSpace(string(contents))
	if !strings.HasPrefix(strings.ToLower(line), prefix) {
		return "", fmt.Errorf("unexpected .git file contents")
	}
	gitDirectory := strings.TrimSpace(line[len(prefix):])
	if gitDirectory == "" {
		return "", fmt.Errorf("empty gitdir path")
	}
	return resolveRelativePath(root, gitDirectory), nil
}

func resolveRelativePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}

func inspectLocalConfig(configPath string) (safetyFinding, error) {
	file, err := os.Open(configPath)
	if os.IsNotExist(err) {
		return safetyFinding{}, nil
	}
	if err != nil {
		return safetyFinding{}, fmt.Errorf("open %s: %w", configPath, err)
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			parsed, parseErr := parseConfigSection(line)
			if parseErr != nil {
				return safetyFinding{}, fmt.Errorf("parse %s: %w", configPath, parseErr)
			}
			section = parsed
			continue
		}
		variable := configVariable(line)
		if section == "" || variable == "" {
			continue
		}
		key := strings.ToLower(section + "." + variable)
		if unsafeLocalConfigKey(key) {
			return safetyFinding{
				reason:      fmt.Sprintf("repository-local Git configuration %s may execute a command", key),
				remediation: fmt.Sprintf("Remove repository-local %s and run the Git update again.", key),
			}, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return safetyFinding{}, fmt.Errorf("read %s: %w", configPath, err)
	}
	return safetyFinding{}, nil
}

func parseConfigSection(line string) (string, error) {
	closing := strings.IndexByte(line, ']')
	if closing < 0 {
		return "", fmt.Errorf("unterminated section header")
	}
	content := strings.TrimSpace(line[1:closing])
	if content == "" {
		return "", fmt.Errorf("empty section header")
	}
	nameEnd := strings.IndexAny(content, " \t\"")
	if nameEnd < 0 {
		return strings.ToLower(content), nil
	}
	name := strings.ToLower(strings.TrimSpace(content[:nameEnd]))
	subsection := strings.TrimSpace(content[nameEnd:])
	if subsection == "" {
		return name, nil
	}
	if strings.HasPrefix(subsection, "\"") {
		unquoted, err := strconv.Unquote(subsection)
		if err != nil {
			return "", fmt.Errorf("invalid subsection: %w", err)
		}
		subsection = unquoted
	}
	return name + "." + strings.ToLower(subsection), nil
}

func configVariable(line string) string {
	end := len(line)
	if equals := strings.IndexByte(line, '='); equals >= 0 && equals < end {
		end = equals
	}
	if space := strings.IndexAny(line, " \t"); space >= 0 && space < end {
		end = space
	}
	return strings.ToLower(strings.TrimSpace(line[:end]))
}

func unsafeLocalConfigKey(key string) bool {
	switch key {
	case "core.askpass", "core.attributesfile", "core.sshcommand", "core.gitproxy", "credential.helper", "remote.origin.uploadpack", "remote.origin.vcs":
		return true
	}
	if strings.HasPrefix(key, "credential.") && strings.HasSuffix(key, ".helper") {
		return true
	}
	if strings.HasPrefix(key, "filter.") {
		return strings.HasSuffix(key, ".clean") || strings.HasSuffix(key, ".smudge") || strings.HasSuffix(key, ".process")
	}
	if strings.HasPrefix(key, "url.") && strings.HasSuffix(key, ".insteadof") {
		return true
	}
	return key == "include.path" || strings.HasPrefix(key, "includeif.") && strings.HasSuffix(key, ".path")
}

func inspectWorkingTreeAttributes(root string) (safetyFinding, error) {
	var finding safetyFinding
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Name() != ".gitattributes" {
			return nil
		}
		finding = inspectAttributesFile(path)
		if finding.reason != "" {
			return filepath.SkipAll
		}
		return nil
	})
	return finding, err
}

func inspectAttributesFile(path string) safetyFinding {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return safetyFinding{}
	}
	if err != nil {
		return safetyInspectionFailure(path, err)
	}
	return inspectAttributesContents(path, string(contents))
}

func inspectAttributesContents(source, contents string) safetyFinding {
	for _, line := range strings.Split(contents, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		for _, attribute := range fields[1:] {
			if attribute == "filter" || strings.HasPrefix(attribute, "filter=") {
				return safetyFinding{
					reason:      fmt.Sprintf("%s contains active filter attribute %s", source, attribute),
					remediation: "Remove active filter attributes before running the Git update.",
				}
			}
		}
	}
	return safetyFinding{}
}

func inspectTargetTreeSafety(ctx context.Context, root, commit string) safetyFinding {
	output, err := runGit(ctx, root, "ls-tree", "-r", "-z", commit)
	if err != nil {
		return safetyInspectionFailure("target tree .gitattributes", err)
	}
	for _, record := range strings.Split(output, "\x00") {
		metadata, name, found := strings.Cut(record, "\t")
		if !found || filepath.Base(filepath.FromSlash(name)) != ".gitattributes" {
			continue
		}
		fields := strings.Fields(metadata)
		if len(fields) != 3 || fields[1] != "blob" {
			return safetyInspectionFailure("target tree .gitattributes", fmt.Errorf("unexpected ls-tree record %q", record))
		}
		contents, catErr := runGit(ctx, root, "cat-file", "blob", fields[2])
		if catErr != nil {
			return safetyInspectionFailure("target tree .gitattributes", catErr)
		}
		if finding := inspectAttributesContents(name, contents); finding.reason != "" {
			return finding
		}
	}
	return safetyFinding{}
}

func unsafeRemoteTransport(remoteURL string) safetyFinding {
	lower := strings.ToLower(strings.TrimSpace(remoteURL))
	if helper := strings.Index(lower, "::"); helper > 0 && validScheme(lower[:helper]) {
		return unsafeTransportFinding(remoteURL)
	}
	if separator := strings.Index(lower, "://"); separator > 0 {
		scheme := lower[:separator]
		switch scheme {
		case "file", "git", "http", "https", "ssh":
			return safetyFinding{}
		default:
			return unsafeTransportFinding(remoteURL)
		}
	}
	return safetyFinding{}
}

func validScheme(value string) bool {
	for index, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' && index > 0 || index > 0 && (character == '+' || character == '-' || character == '.') {
			continue
		}
		return false
	}
	return value != ""
}

func unsafeTransportFinding(remoteURL string) safetyFinding {
	return safetyFinding{
		reason:      fmt.Sprintf("origin uses unsafe or custom transport %q", remoteURL),
		remediation: "Replace origin with a local path or a standard file, Git, HTTP(S), or SSH URL.",
	}
}

func safetyInspectionFailure(subject string, err error) safetyFinding {
	return safetyFinding{
		reason:      fmt.Sprintf("cannot safely inspect %s: %v", subject, err),
		remediation: "Resolve the safety inspection error before running the Git update.",
	}
}
