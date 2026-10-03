// Package assetexport provides explicit exporter templates. It never starts an editor.
package assetexport

import (
	"embed"
	"fmt"
)

//go:embed scripts/*
var scripts embed.FS

// Script returns an exporter for an explicitly requested engine.
func Script(engine string) ([]byte, error) {
	name := ""
	switch engine {
	case "blender":
		name = "blender.py"
	case "unity":
		name = "GoreGraphAssetExporter.cs"
	default:
		return nil, fmt.Errorf("unsupported asset exporter %q", engine)
	}
	return scripts.ReadFile("scripts/" + name)
}
