package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// SymbolUsageCategory classifies how a consumer reaches a canonical symbol.
type SymbolUsageCategory string

const (
	SymbolUsageDirectReference   SymbolUsageCategory = "direct_reference"
	SymbolUsageReachedThroughAPI SymbolUsageCategory = "reached_through_api"
	SymbolUsageAmbiguous         SymbolUsageCategory = "ambiguous"
	SymbolUsageUnresolved        SymbolUsageCategory = "unresolved"
)

// SymbolResolution reports whether symbol evidence selects one, several, or no providers.
type SymbolResolution string

const (
	SymbolResolutionExact      SymbolResolution = "EXACT"
	SymbolResolutionAmbiguous  SymbolResolution = "AMBIGUOUS"
	SymbolResolutionUnresolved SymbolResolution = "UNRESOLVED"
)

// CanonicalSymbolRecord identifies one workspace declaration with its provenance.
type CanonicalSymbolRecord struct {
	ID               string     `json:"id"`
	Project          string     `json:"project"`
	Service          string     `json:"service,omitempty"`
	ProjectKind      string     `json:"project_kind,omitempty"`
	Module           string     `json:"module,omitempty"`
	Package          string     `json:"package,omitempty"`
	Application      string     `json:"application,omitempty"`
	WorkspacePackage string     `json:"workspace_package,omitempty"`
	Artifact         string     `json:"artifact,omitempty"`
	Language         string     `json:"language"`
	Kind             string     `json:"kind"`
	Name             string     `json:"name"`
	QualifiedName    string     `json:"qualified_name"`
	ExportName       string     `json:"export_name,omitempty"`
	DeclarationFile  string     `json:"declaration_file"`
	DeclarationLine  int        `json:"declaration_line"`
	EvidenceIDs      []string   `json:"evidence_ids,omitempty"`
	Analyzer         string     `json:"analyzer"`
	Confidence       Confidence `json:"confidence"`
	Coverage         Coverage   `json:"coverage"`
	Limitations      []string   `json:"limitations,omitempty"`
}

// SymbolAPIPathStepRecord describes one ordered step in an API-mediated symbol usage.
type SymbolAPIPathStepRecord struct {
	Position    int      `json:"position"`
	Kind        string   `json:"kind"`
	Project     string   `json:"project"`
	SymbolID    string   `json:"symbol_id,omitempty"`
	Label       string   `json:"label"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

// CanonicalSymbolUsageRecord describes one direct or API-mediated symbol usage.
type CanonicalSymbolUsageRecord struct {
	ID                  string                    `json:"id"`
	ProviderSymbolID    string                    `json:"provider_symbol_id,omitempty"`
	ConsumerProject     string                    `json:"consumer_project"`
	ConsumerSymbolID    string                    `json:"consumer_symbol_id,omitempty"`
	Category            SymbolUsageCategory       `json:"category"`
	Language            string                    `json:"language"`
	RelationKind        string                    `json:"relation_kind"`
	TargetQualifiedName string                    `json:"target_qualified_name,omitempty"`
	TargetModule        string                    `json:"target_module,omitempty"`
	TargetExport        string                    `json:"target_export,omitempty"`
	SourceFile          string                    `json:"source_file"`
	SourceLine          int                       `json:"source_line"`
	Confidence          Confidence                `json:"confidence"`
	Resolution          SymbolResolution          `json:"resolution"`
	Reason              string                    `json:"reason"`
	Analyzer            string                    `json:"analyzer"`
	EvidenceIDs         []string                  `json:"evidence_ids,omitempty"`
	CandidateSymbolIDs  []string                  `json:"candidate_symbol_ids,omitempty"`
	CandidatePathIDs    []string                  `json:"candidate_path_ids,omitempty"`
	DependencyEvidence  []string                  `json:"dependency_evidence,omitempty"`
	Transport           string                    `json:"transport,omitempty"`
	APIPath             []SymbolAPIPathStepRecord `json:"api_path,omitempty"`
	Limitations         []string                  `json:"limitations,omitempty"`
}

// SymbolCoverageRecord describes analyzer coverage for one project capability.
type SymbolCoverageRecord struct {
	Project     string   `json:"project"`
	Language    string   `json:"language"`
	Capability  string   `json:"capability"`
	Coverage    Coverage `json:"coverage"`
	Reason      string   `json:"reason"`
	Limitations []string `json:"limitations,omitempty"`
}

// WorkspaceSymbolIndexRecord is the canonical symbol inventory for a workspace.
type WorkspaceSymbolIndexRecord struct {
	SchemaVersion int                     `json:"schema_version"`
	Generated     string                  `json:"generated,omitempty"`
	Root          string                  `json:"root,omitempty"`
	Symbols       []CanonicalSymbolRecord `json:"symbols"`
	Coverage      []SymbolCoverageRecord  `json:"coverage"`
}

// WorkspaceSymbolUsageIndexRecord is the canonical symbol-usage inventory for a workspace.
type WorkspaceSymbolUsageIndexRecord struct {
	SchemaVersion int                          `json:"schema_version"`
	Generated     string                       `json:"generated,omitempty"`
	Root          string                       `json:"root,omitempty"`
	Usages        []CanonicalSymbolUsageRecord `json:"usages"`
	Coverage      []SymbolCoverageRecord       `json:"coverage"`
}

// StableWorkspaceSymbolID returns the canonical identity of a workspace declaration.
func StableWorkspaceSymbolID(kind, project, scope, language, qualifiedName, declarationFile string) string {
	return stableSymbolProjectionID("symbol", []string{kind, project, scope, language, qualifiedName, declarationFile})
}

// StableWorkspaceUsageID returns the canonical identity of a workspace symbol usage.
func StableWorkspaceUsageID(providerSymbolID, consumerProject, consumerSymbolID string, category SymbolUsageCategory, relationKind, targetIdentity, sourceFile string, sourceLine int) string {
	return stableSymbolProjectionID("usage", []string{
		providerSymbolID,
		consumerProject,
		consumerSymbolID,
		string(category),
		relationKind,
		targetIdentity,
		sourceFile,
		fmt.Sprint(sourceLine),
	})
}

// WorkspaceEvidenceID namespaces project-local evidence for workspace projection.
func WorkspaceEvidenceID(project, localEvidenceID string) string {
	return project + "#" + localEvidenceID
}

func stableSymbolProjectionID(prefix string, parts []string) string {
	normalized := make([]string, len(parts))
	for index, part := range parts {
		normalized[index] = strings.TrimSpace(strings.ReplaceAll(filepath.ToSlash(part), `\`, "/"))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\x00")))
	return prefix + ":" + hex.EncodeToString(sum[:16])
}
