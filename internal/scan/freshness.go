package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/version"
)

type ArtifactFreshnessRecord struct {
	Artifact          string `json:"artifact"`
	GeneratedAt       string `json:"generated_at"`
	GoreGraphVersion  string `json:"goregraph_version"`
	Schema            int    `json:"schema"`
	SourceFingerprint string `json:"source_fingerprint"`
	Stale             bool   `json:"stale"`
	Reason            string `json:"reason"`
}

type ArtifactFreshnessIndex struct {
	Schema            int                       `json:"schema"`
	GeneratedAt       string                    `json:"generated_at"`
	GoreGraphVersion  string                    `json:"goregraph_version"`
	SourceFingerprint string                    `json:"source_fingerprint"`
	Artifacts         []ArtifactFreshnessRecord `json:"artifacts"`
}

func BuildArtifactFreshness(manifest Manifest, files []FileRecord, artifacts []string) ArtifactFreshnessIndex {
	fingerprint := sourceFingerprint(files)
	names := append([]string(nil), artifacts...)
	sort.Strings(names)
	index := ArtifactFreshnessIndex{
		Schema:            manifest.Schema,
		GeneratedAt:       manifest.GeneratedAt,
		GoreGraphVersion:  version.Version,
		SourceFingerprint: fingerprint,
		Artifacts:         make([]ArtifactFreshnessRecord, 0, len(names)),
	}
	for _, artifact := range names {
		index.Artifacts = append(index.Artifacts, ArtifactFreshnessRecord{
			Artifact: artifact, GeneratedAt: manifest.GeneratedAt, GoreGraphVersion: version.Version,
			Schema: manifest.Schema, SourceFingerprint: fingerprint, Stale: false,
			Reason: "generated from the current source fingerprint",
		})
	}
	return index
}

func sourceFingerprint(files []FileRecord) string {
	parts := make([]string, 0, len(files))
	for _, file := range files {
		parts = append(parts, file.Path+"\x00"+file.Hash)
	}
	return semanticFingerprint(parts)
}

func semanticFingerprint(parts []string) string {
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func BuildWorkspaceFreshness(indexed []workspaceIndexProject, generatedAt string) ArtifactFreshnessIndex {
	parts := make([]string, 0, len(indexed))
	artifacts := []ArtifactFreshnessRecord{}
	for _, project := range indexed {
		parts = append(parts, project.record.Path+"\x00"+project.freshness.SourceFingerprint)
		for _, record := range project.freshness.Artifacts {
			record.Artifact = filepathJoinSlash(project.record.Path, record.Artifact)
			artifacts = append(artifacts, record)
		}
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Artifact < artifacts[j].Artifact })
	return ArtifactFreshnessIndex{Schema: SchemaVersion, GeneratedAt: generatedAt, GoreGraphVersion: version.Version, SourceFingerprint: semanticFingerprint(parts), Artifacts: artifacts}
}

func filepathJoinSlash(parts ...string) string {
	return strings.ReplaceAll(strings.Join(parts, "/"), "\\", "/")
}
