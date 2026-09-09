package scan

// ProjectionHealth separates output integrity from input freshness and analysis coverage.
type ProjectionHealth struct {
	GenerationID string   `json:"generation_id,omitempty"`
	Integrity    string   `json:"integrity"`
	Freshness    string   `json:"freshness"`
	Coverage     string   `json:"coverage"`
	Reasons      []string `json:"reasons,omitempty"`
}

// HealthForProjection requires callers to validate files and explicitly attest live inputs.
// A generated timestamp alone does not establish freshness.
func HealthForProjection(manifest OutputManifest, projection string, liveVerified bool) ProjectionHealth {
	health := ProjectionHealth{GenerationID: manifest.GenerationID, Integrity: "unavailable", Freshness: "unknown", Coverage: "unknown"}
	status := manifest.Index
	switch projection {
	case "agent":
		status = manifest.Agent
	case "dashboard":
		status = manifest.Dashboard
	case "index":
	default:
		health.Reasons = []string{"unknown_projection"}
		return health
	}
	if manifest.Tool != ToolName || status.GeneratedAt == "" {
		health.Reasons = []string{"projection_missing"}
		return health
	}
	if manifest.Schema != SchemaVersion || !status.Complete {
		health.Integrity = "invalid"
		health.Reasons = []string{"projection_incomplete"}
		return health
	}
	health.Integrity = "valid"
	health.Coverage = manifest.AnalysisCoverage
	if health.Coverage == "" {
		health.Coverage = "unknown"
	}
	if len(manifest.AnalysisIssues) > 0 {
		health.Coverage = "partial"
		health.Reasons = append(health.Reasons, "analysis_incomplete")
	}
	identity := manifest.BuildIdentity
	revisionChanged := identity.ExtractorRevision != "" && (identity.ExtractorRevision != currentExtractorRevision || identity.ResolverRevision != currentResolverRevision ||
		projection == "agent" && identity.AgentRevision != currentAgentRevision || projection == "dashboard" && identity.DashboardRevision != currentDashboardRevision)
	if revisionChanged {
		health.Freshness = "stale"
		health.Reasons = append(health.Reasons, "analyzer_revision_changed")
	} else if status.Stale {
		health.Freshness = "stale"
		health.Reasons = append(health.Reasons, "projection_inputs_changed")
	} else if manifest.BuildIdentity.ExtractorRevision == "" || status.InputFingerprint == "" {
		health.Reasons = append(health.Reasons, "input_identity_unknown")
	} else if liveVerified {
		health.Freshness = "current"
	} else {
		health.Reasons = append(health.Reasons, "live_inputs_not_verified")
	}
	return health
}

func analysisCoverage(capabilities []CapabilityRecord, issues []AnalysisIssue) string {
	if len(issues) > 0 {
		return "partial"
	}
	supported, partial := false, false
	for _, capability := range capabilities {
		if capability.SourceClass != "code" {
			continue
		}
		switch capability.Coverage {
		case CoverageComplete:
			supported = true
		case CoveragePartial:
			supported = true
			partial = true
		case CoverageFailed:
			partial = true
		}
	}
	if partial {
		return "partial"
	}
	if supported {
		return "complete"
	}
	return "unsupported"
}

func applyAnalysisIssues(capabilities []CapabilityRecord, files []FileRecord, issues []AnalysisIssue) {
	affected := map[string]bool{}
	for _, issue := range issues {
		for _, file := range files {
			if file.Path == issue.File {
				affected[file.Language] = true
				break
			}
		}
	}
	for i := range capabilities {
		capability := &capabilities[i]
		if !affected[capability.Language] || capability.Coverage == CoverageUnavailable {
			continue
		}
		capability.Coverage = CoveragePartial
		capability.Reason = "One or more eligible files exceeded the analysis budget; consult manifest analysis_issues."
		capability.StatusReason = "analysis_budget_exceeded"
	}
}
