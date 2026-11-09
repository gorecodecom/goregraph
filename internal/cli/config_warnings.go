package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/gorecodecom/goregraph/internal/config"
)

// projectConfigLoader reports optional metadata issues once per command and path.
type projectConfigLoader struct {
	stderr   io.Writer
	reported map[string]bool
}

func newProjectConfigLoader(stderr io.Writer) *projectConfigLoader {
	return &projectConfigLoader{stderr: stderr, reported: make(map[string]bool)}
}

func (loader *projectConfigLoader) load(root string) (config.Config, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return cfg, err
	}
	path, err := filepath.Abs(filepath.Join(root, "goregraph.yml"))
	if err != nil {
		return config.Config{}, err
	}
	if !loader.reported[path] {
		for _, warning := range cfg.Warnings {
			fmt.Fprintf(loader.stderr, "warning: %s:%d: %s: %s\n", path, warning.Line, warning.Key, warning.Message)
		}
		loader.reported[path] = true
	}
	return cfg, nil
}
