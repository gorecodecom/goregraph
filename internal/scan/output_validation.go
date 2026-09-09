package scan

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func newOutputGeneration() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func validateGeneratedOutput(root string, target BuildTarget) error {
	manifest, err := readProjectOutputManifest(filepath.Join(root, "manifest.json"))
	if err != nil {
		return err
	}
	if manifest.Tool != ToolName || manifest.Schema != SchemaVersion || !manifest.Index.Complete {
		return fmt.Errorf("generated canonical index is incomplete: %s", root)
	}
	if target.IncludesAgent() && !manifest.Agent.Complete {
		return fmt.Errorf("generated agent projection is incomplete: %s", root)
	}
	if target.IncludesDashboard() && !manifest.Dashboard.Complete {
		return fmt.Errorf("generated dashboard projection is incomplete: %s", root)
	}
	for _, projection := range []ProjectionStatus{manifest.Index, manifest.Agent, manifest.Dashboard} {
		if !projection.Complete {
			continue
		}
		if len(projection.Files) == 0 {
			return fmt.Errorf("generated projection has no files: %s", root)
		}
		for _, name := range projection.Files {
			clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
			if name != clean || filepath.IsAbs(name) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(name, ":") {
				return fmt.Errorf("unsafe generated path %q", name)
			}
			path := filepath.Join(root, filepath.FromSlash(name))
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("generated path is not a regular file: %s", name)
			}
			if strings.HasSuffix(name, ".json") {
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if !json.Valid(body) {
					return fmt.Errorf("invalid generated JSON: %s", name)
				}
			}
		}
	}
	return nil
}
