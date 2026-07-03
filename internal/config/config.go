package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	OutputDir        string
	Exclude          []string
	MaxFileSizeBytes int64
	FollowSymlinks   bool
	UseGitignore     bool
	UpdateGitignore  bool
}

func Defaults() Config {
	return Config{
		OutputDir:        "goregraph-out",
		MaxFileSizeBytes: 512 * 1024,
		FollowSymlinks:   false,
		UseGitignore:     true,
		UpdateGitignore:  true,
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
	return Defaults(), nil
}

func (c Config) HasExclude(pattern string) bool {
	for _, existing := range c.Exclude {
		if existing == pattern {
			return true
		}
	}
	return false
}
