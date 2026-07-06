package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	OutputDir        string
	Include          []string
	Exclude          []string
	MaxFileSizeBytes int64
	FollowSymlinks   bool
	UseGitignore     bool
	UpdateGitignore  bool
	Workspace        bool
	WorkspaceRoot    string
}

func Defaults() Config {
	return Config{
		OutputDir:        "goregraph-out",
		MaxFileSizeBytes: 512 * 1024,
		FollowSymlinks:   false,
		UseGitignore:     true,
		UpdateGitignore:  true,
		Workspace:        true,
		Exclude: []string{
			".git/",
			"node_modules/",
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
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			value := cleanValue(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			switch activeList {
			case "include":
				cfg.Include = appendUnique(cfg.Include, value)
			case "exclude":
				cfg.Exclude = appendUnique(cfg.Exclude, value)
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
		if value == "" {
			switch key {
			case "include", "exclude":
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
		default:
			return fmt.Errorf("unsupported key %q", key)
		}
	}
	return scanner.Err()
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
