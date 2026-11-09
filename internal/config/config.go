package config

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	OutputDir               string
	Include                 []string
	Exclude                 []string
	MaxFileSizeBytes        int64
	MaxAssetFileSizeBytes   int64
	MaxBinaryAssetSizeBytes int64
	FollowSymlinks          bool
	UseGitignore            bool
	UpdateGitignore         bool
	Workspace               bool
	WorkspaceRoot           string
	EditorURLTemplate       string
	ModulePurposes          []ModulePurpose
	Warnings                []Warning `json:"-"`
}

// Warning identifies ignored optional metadata without retaining its value.
type Warning struct {
	Line    int
	Key     string
	Message string
}

// ModulePurpose describes an explicitly configured module for source navigation.
type ModulePurpose struct {
	Path        string
	Description string
}

func Defaults() Config {
	return Config{
		OutputDir:               "goregraph-out",
		MaxFileSizeBytes:        512 * 1024,
		MaxAssetFileSizeBytes:   16 * 1024 * 1024,
		MaxBinaryAssetSizeBytes: 256 * 1024 * 1024,
		FollowSymlinks:          false,
		UseGitignore:            true,
		UpdateGitignore:         true,
		Workspace:               true,
		Exclude: []string{
			".git/",
			".godot/",
			".worktrees/",
			"node_modules/",
			".dart_tool/",
			".pub-cache/",
			"vendor/",
			"target/",
			"build/",
			"dist/",
			"coverage/",
			".idea/",
			".vscode/",
			".gitignore",
			"goregraph-out/",
			".goregraph-workspace/",
		},
	}
}

func Load(root string) (Config, error) {
	if root == "" {
		return Config{}, fmt.Errorf("project root is required")
	}
	resolved, err := filepath.Abs(root)
	if err != nil {
		return Config{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Config{}, fmt.Errorf("project root %q is not readable: %w", root, err)
	}
	if !info.IsDir() {
		return Config{}, fmt.Errorf("project root %q is not a directory", root)
	}
	cfg := Defaults()
	configPath := filepath.Join(resolved, "goregraph.yml")
	body, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("project config %q is not readable: %w", configPath, err)
	}
	if err := applyProjectConfig(&cfg, string(body)); err != nil {
		return Config{}, fmt.Errorf("project config %q is invalid: %w", configPath, err)
	}
	return cfg, nil
}

func applyProjectConfig(cfg *Config, body string) error {
	scanner := bufio.NewScanner(strings.NewReader(body))
	activeList := ""
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := stripComment(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || trimmed == "-" && activeList == "module_purposes" {
			value := cleanValue(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
			switch activeList {
			case "include":
				cfg.Include = appendUnique(cfg.Include, value)
			case "exclude":
				cfg.Exclude = appendUnique(cfg.Exclude, value)
			case "module_purposes":
				if err := addModulePurpose(cfg, value); err != nil {
					cfg.Warnings = append(cfg.Warnings, Warning{Line: lineNumber, Key: activeList, Message: err.Error() + "; entry ignored"})
				}
			default:
				return fmt.Errorf("list item %q has no supported parent key", value)
			}
			continue
		}

		activeList = ""
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return fmt.Errorf("line %q is missing ':'", trimmed)
		}
		key = strings.TrimSpace(key)
		value = cleanValue(strings.TrimSpace(value))
		if !supportedProjectKey(key) {
			return fmt.Errorf("unsupported key %q; check the spelling or upgrade GoreGraph if this configuration requires a newer version", key)
		}
		if key == "module_purposes" && value != "" {
			cfg.Warnings = append(cfg.Warnings, Warning{Line: lineNumber, Key: key, Message: "expected a list of module descriptions; value ignored"})
			continue
		}
		if value == "" {
			switch key {
			case "include", "exclude", "module_purposes":
				activeList = key
				continue
			default:
				return fmt.Errorf("key %q requires a value", key)
			}
		}

		switch key {
		case "version":
			continue
		case "output":
			if err := validateOutputDir(value); err != nil {
				return err
			}
			cfg.OutputDir = value
		case "max_file_size_kb":
			kb, err := strconv.ParseInt(value, 10, 64)
			if err != nil || kb <= 0 {
				return fmt.Errorf("max_file_size_kb must be a positive integer")
			}
			cfg.MaxFileSizeBytes = kb * 1024
		case "max_asset_file_size_kb", "max_binary_asset_size_kb":
			kb, err := strconv.ParseInt(value, 10, 64)
			if err != nil || kb <= 0 || kb > 1048576 {
				return fmt.Errorf("%s must be a positive integer up to 1048576", key)
			}
			if key == "max_asset_file_size_kb" {
				cfg.MaxAssetFileSizeBytes = kb * 1024
			} else {
				cfg.MaxBinaryAssetSizeBytes = kb * 1024
			}
		case "follow_symlinks":
			parsed, err := parseBool(value)
			if err != nil {
				return fmt.Errorf("follow_symlinks must be true or false")
			}
			cfg.FollowSymlinks = parsed
		case "use_gitignore":
			parsed, err := parseBool(value)
			if err != nil {
				return fmt.Errorf("use_gitignore must be true or false")
			}
			cfg.UseGitignore = parsed
		case "update_gitignore":
			parsed, err := parseBool(value)
			if err != nil {
				return fmt.Errorf("update_gitignore must be true or false")
			}
			cfg.UpdateGitignore = parsed
		case "editor_url_template":
			if !strings.Contains(value, "{file}") {
				return fmt.Errorf("editor_url_template must contain {file}")
			}
			cfg.EditorURLTemplate = value
		default:
			return fmt.Errorf("unsupported key %q", key)
		}
	}
	return scanner.Err()
}

func supportedProjectKey(key string) bool {
	switch key {
	case "version", "include", "exclude", "module_purposes", "output", "max_file_size_kb", "max_asset_file_size_kb", "max_binary_asset_size_kb", "follow_symlinks", "use_gitignore", "update_gitignore", "editor_url_template":
		return true
	default:
		return false
	}
}

func addModulePurpose(cfg *Config, value string) error {
	directory, description, ok := strings.Cut(value, ": ")
	directory = strings.TrimSpace(directory)
	description = strings.TrimSpace(description)
	if !ok || directory == "." || directory == ".." || strings.HasPrefix(directory, "../") ||
		path.IsAbs(directory) || path.Clean(directory) != directory ||
		strings.ContainsAny(directory, ":\\\x00") || len(directory) > 160 ||
		description == "" || len(description) > 240 {
		return fmt.Errorf("module_purposes requires a relative directory and a description of up to 240 bytes")
	}
	for _, purpose := range cfg.ModulePurposes {
		if purpose.Path == directory {
			return fmt.Errorf("module_purposes contains a duplicate directory")
		}
	}
	if len(cfg.ModulePurposes) == 32 {
		return fmt.Errorf("module_purposes accepts up to 32 directories")
	}
	cfg.ModulePurposes = append(cfg.ModulePurposes, ModulePurpose{Path: directory, Description: description})
	return nil
}

func validateOutputDir(value string) error {
	if value == "" {
		return fmt.Errorf("output must not be empty")
	}
	slashed := filepath.ToSlash(value)
	if filepath.IsAbs(value) || strings.HasPrefix(slashed, "/") {
		return fmt.Errorf("output must be relative to the project root")
	}
	cleaned := filepath.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(filepath.ToSlash(cleaned), "../") {
		return fmt.Errorf("output must stay inside the project root")
	}
	return nil
}

func stripComment(line string) string {
	if before, _, ok := strings.Cut(line, "#"); ok {
		return before
	}
	return line
}

func cleanValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return filepath.ToSlash(value)
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool")
	}
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (c Config) HasInclude(pattern string) bool {
	for _, existing := range c.Include {
		if existing == pattern {
			return true
		}
	}
	return false
}

func (c Config) HasExclude(pattern string) bool {
	for _, existing := range c.Exclude {
		if existing == pattern {
			return true
		}
	}
	return false
}
