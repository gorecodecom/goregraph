package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
)

func binaryAssetFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".blend", ".fbx", ".glb", ".png", ".jpg", ".jpeg", ".tga", ".exr", ".psd", ".wav", ".ogg", ".mp3":
		return true
	}
	return false
}
func assetFileSizeLimit(name string, cfg config.Config) int64 {
	limit := cfg.MaxFileSizeBytes
	assetLimit := int64(0)
	if detectLanguage(name) == "unity" || strings.HasSuffix(name, ".goregraph-blender.json") {
		assetLimit = cfg.MaxAssetFileSizeBytes
	}
	if binaryAssetFile(name) {
		assetLimit = cfg.MaxBinaryAssetSizeBytes
	}
	if assetLimit > limit {
		limit = assetLimit
	}
	return limit
}

// binaryAssetRecord hashes asset bytes incrementally; it never executes an importer.
func binaryAssetRecord(ctx context.Context, root string, file WalkedFile) (FileRecord, error) {
	name := filepath.Join(root, filepath.FromSlash(file.Path))
	input, err := os.Open(name)
	if err != nil {
		return FileRecord{}, err
	}
	defer input.Close()
	before, err := input.Stat()
	if err != nil {
		return FileRecord{}, err
	}
	hash := sha256.New()
	buffer := make([]byte, 64*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return FileRecord{}, err
		}
		n, err := input.Read(buffer)
		if n > 0 {
			hash.Write(buffer[:n])
			total += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return FileRecord{}, err
		}
	}
	after, err := input.Stat()
	if err != nil {
		return FileRecord{}, err
	}
	if total != file.Size || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return FileRecord{}, fmt.Errorf("asset changed during inventory: %s", file.Path)
	}
	language := "unity"
	if strings.EqualFold(filepath.Ext(file.Path), ".blend") {
		language = "blender"
	}
	return FileRecord{Path: file.Path, Size: total, Hash: hex.EncodeToString(hash.Sum(nil)), Language: language, Kind: "binary_asset"}, nil
}

func projectBinaryAssetFile(root, name string) bool {
	if strings.EqualFold(filepath.Ext(name), ".blend") {
		return true
	}
	return binaryAssetFile(name) && workspaceRegularFileExists(filepath.Join(root, "ProjectSettings", "ProjectVersion.txt")) && workspaceRegularFileExists(filepath.Join(root, "Packages", "manifest.json"))
}

func projectAssetFileSizeLimit(name string, cfg config.Config, unity bool) int64 {
	if binaryAssetFile(name) && !unity && !strings.EqualFold(filepath.Ext(name), ".blend") {
		return cfg.MaxFileSizeBytes
	}
	return assetFileSizeLimit(name, cfg)
}
