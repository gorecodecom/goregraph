package scan

import (
	"encoding/json"

	"github.com/gorecodecom/goregraph/internal/config"
)

const (
	currentExtractorRevision = "2"
	currentResolverRevision  = "1"
	currentAgentRevision     = "2"
	currentDashboardRevision = "2"
)

// BuildIdentity records inputs that affect analysis independently of release labels.
type BuildIdentity struct {
	ExtractorRevision    string `json:"extractor_revision"`
	ResolverRevision     string `json:"resolver_revision"`
	AgentRevision        string `json:"agent_revision"`
	DashboardRevision    string `json:"dashboard_revision"`
	ConfigDigest         string `json:"config_digest"`
	AnalysisPolicyDigest string `json:"analysis_policy_digest"`
	IgnoreDigest         string `json:"ignore_digest"`
	SourceFingerprint    string `json:"source_fingerprint"`
}

// CurrentBuildIdentity excludes observers, deadlines and release-only metadata.
func CurrentBuildIdentity(cfg config.Config, options BuildOptions, ignoreDigest, source string) BuildIdentity {
	selection := struct {
		Include, Exclude             []string
		MaxFileSize                  int64
		FollowSymlinks, UseGitignore bool
		OutputDir, EditorURLTemplate string
	}{cfg.Include, cfg.Exclude, cfg.MaxFileSizeBytes, cfg.FollowSymlinks, cfg.UseGitignore, cfg.OutputDir, cfg.EditorURLTemplate}
	body, _ := json.Marshal(selection)
	policy, _ := json.Marshal(options.FileTimeout)
	return BuildIdentity{
		ExtractorRevision: currentExtractorRevision, ResolverRevision: currentResolverRevision, AgentRevision: currentAgentRevision, DashboardRevision: currentDashboardRevision,
		ConfigDigest:         semanticFingerprint([]string{string(body)}),
		AnalysisPolicyDigest: semanticFingerprint([]string{string(policy)}),
		IgnoreDigest:         ignoreDigest, SourceFingerprint: source,
	}
}

func (identity BuildIdentity) fingerprint(projection string) string {
	agentRevision, dashboardRevision := identity.AgentRevision, identity.DashboardRevision
	identity.AgentRevision = ""
	identity.DashboardRevision = ""
	switch projection {
	case "agent":
		identity.AgentRevision = agentRevision
	case "dashboard":
		identity.DashboardRevision = dashboardRevision
	}
	body, _ := json.Marshal(identity)
	return semanticFingerprint([]string{projection, string(body)})
}

func identityChange(previous, current BuildIdentity, target BuildTarget) string {
	switch {
	case previous.ExtractorRevision != current.ExtractorRevision:
		return "extractor revision changed"
	case previous.ResolverRevision != current.ResolverRevision:
		return "resolver revision changed"
	case previous.ConfigDigest != current.ConfigDigest:
		return "scan configuration changed"
	case previous.AnalysisPolicyDigest != current.AnalysisPolicyDigest:
		return "analysis policy changed"
	case previous.IgnoreDigest != current.IgnoreDigest:
		return "ignore rules changed"
	case target.IncludesAgent() && previous.AgentRevision != current.AgentRevision:
		return "agent revision changed"
	case target.IncludesDashboard() && previous.DashboardRevision != current.DashboardRevision:
		return "dashboard revision changed"
	}
	return ""
}
