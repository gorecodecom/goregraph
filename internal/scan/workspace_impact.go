package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func WorkspaceImpact(workspaceOut string, changedFiles []string) (WorkspaceImpactRecord, error) {
	var dossiers []FeatureDossierRecord
	layout := NewWorkspaceOutputLayout(workspaceOut)
	if err := readWorkspaceJSON(layout.Index("feature-dossiers.json"), &dossiers); err != nil {
		return WorkspaceImpactRecord{}, err
	}
	var flows []WorkspaceFeatureFlowRecord
	if err := readWorkspaceJSON(layout.Index("feature-flows.json"), &flows); err != nil && !os.IsNotExist(err) {
		return WorkspaceImpactRecord{}, err
	}
	flowsByID := map[string]WorkspaceFeatureFlowRecord{}
	for _, flow := range flows {
		flowsByID[flow.ID] = flow
	}
	changed := map[string]bool{}
	for _, file := range changedFiles {
		changed[normalizeWorkspaceImpactPath(file)] = true
	}
	record := WorkspaceImpactRecord{ChangedFiles: changedFiles, RiskSummary: map[string]int{}}
	seen := map[string]bool{}
	for _, dossier := range dossiers {
		flow := flowsByID[dossier.SourceFlowID]
		if !featureDossierTouchesChangedFile(dossier, flow, changed) {
			continue
		}
		if seen[dossier.ID] {
			continue
		}
		seen[dossier.ID] = true
		record.AffectedFeatures = append(record.AffectedFeatures, dossier)
		risk := "low"
		if len(dossier.Risks) > 0 || len(flow.FieldRisks) > 0 {
			risk = "risk"
		} else if len(dossier.Tests) == 0 && len(flow.Tests) == 0 {
			risk = "missing_tests"
		}
		record.RiskSummary[risk]++
	}
	return record, nil
}

func RenderWorkspaceImpact(record WorkspaceImpactRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Impact\n\n")
	b.WriteString(fmt.Sprintf("- Changed files: %d\n", len(record.ChangedFiles)))
	b.WriteString(fmt.Sprintf("- Affected features: %d\n", len(record.AffectedFeatures)))
	for risk, count := range record.RiskSummary {
		b.WriteString(fmt.Sprintf("- %s: %d\n", risk, count))
	}
	if len(record.AffectedFeatures) == 0 {
		b.WriteString("\nNo affected features found.\n")
		return b.String()
	}
	b.WriteString("\n## Affected Features\n\n")
	for _, feature := range record.AffectedFeatures {
		b.WriteString(fmt.Sprintf("- `%s`", feature.Route))
		if feature.FrontendProject != "" {
			b.WriteString(fmt.Sprintf(" frontend `%s`", feature.FrontendProject))
		}
		if feature.BackendProject != "" {
			b.WriteString(fmt.Sprintf(" backend `%s`", feature.BackendProject))
		}
		if len(feature.Risks) > 0 {
			b.WriteString(" risk")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func featureDossierTouchesChangedFile(dossier FeatureDossierRecord, flow WorkspaceFeatureFlowRecord, changed map[string]bool) bool {
	candidates := []string{
		flow.FrontendProject + "/" + flow.FrontendFile,
		flow.BackendProject + "/" + flow.BackendFile,
		dossier.FrontendProject,
		dossier.BackendProject,
	}
	for _, candidate := range candidates {
		if changed[normalizeWorkspaceImpactPath(candidate)] {
			return true
		}
	}
	return false
}

func normalizeWorkspaceImpactPath(path string) string {
	return strings.Trim(filepath.ToSlash(path), "/")
}

func readWorkspaceDossiers(path string) ([]FeatureDossierRecord, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var records []FeatureDossierRecord
	return records, json.Unmarshal(body, &records)
}
