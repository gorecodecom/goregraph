package scan

import (
	"sort"
	"strings"
)

const broaderSecurityConfigurationLimitation = "Security configuration may cover broader paths and cannot be tied exactly to this endpoint"

// NormalizeSecurityEvidence converts extractor-specific authentication facts into catalog security categories.
func NormalizeSecurityEvidence(records []AuthRecord) []SecurityEvidenceRecord {
	if len(records) == 0 {
		return unknownAPISecurity()
	}

	result := make([]SecurityEvidenceRecord, 0, len(records))
	for _, record := range records {
		kind, summary := normalizedProviderSecurityKind(record)
		normalized := SecurityEvidenceRecord{
			Kind:       kind,
			Summary:    summary,
			Expression: record.Expression,
			Source:     record.Source,
			File:       record.File,
			Line:       record.Line,
			Confidence: apiRouteConfidence(record.Confidence),
		}
		if record.Source == "security_config_call" {
			normalized.Limitations = []string{broaderSecurityConfigurationLimitation}
		}
		result = append(result, normalized)
	}

	markConflictingProviderSecurity(result)
	for index := range result {
		sort.Strings(result[index].Limitations)
	}
	sort.Slice(result, func(left, right int) bool {
		return securityEvidenceSortKey(result[left]) < securityEvidenceSortKey(result[right])
	})
	return result
}

func normalizedProviderSecurityKind(record AuthRecord) (string, string) {
	kind := strings.ToLower(strings.TrimSpace(record.Kind))
	switch kind {
	case "http_basic", "basic":
		return SecurityBasic, "HTTP Basic authentication"
	case "oauth2_resource_server", "bearer":
		return SecurityBearer, "Bearer token authentication"
	case "oauth2_login", "oauth2", "openid_connect":
		return SecurityOAuth2, "OAuth2 authentication"
	case "api_key":
		return SecurityAPIKey, "API key authentication"
	case "form_login", "session":
		return SecuritySession, "Form or session authentication"
	case "x509", "mtls", "mutual_tls":
		return SecurityMTLS, "X.509 or mutual TLS authentication"
	case "has_role", "has_any_role", "has_authority", "has_any_authority", "secured", "roles_allowed", "role":
		return SecurityRole, "Role or authority requirement"
	case "pre_authorize", "post_authorize":
		return normalizedAuthorizationExpression(record.Expression)
	case "authenticated":
		return SecurityAuthenticated, "Authenticated access required"
	case "permit_all", "public":
		return SecurityPublic, "Explicitly public access"
	case "deny_all":
		return SecurityUnknown, "Explicit deny-all access restriction"
	case "unknown", "":
		return SecurityUnknown, "Unclassified explicit security evidence"
	default:
		return SecurityUnknown, "Unclassified explicit security evidence: " + kind
	}
}

func normalizedAuthorizationExpression(expression string) (string, string) {
	lower := strings.ToLower(expression)
	switch {
	case strings.Contains(lower, "permitall"):
		return SecurityPublic, "Explicitly public authorization expression"
	case strings.Contains(lower, "hasrole"), strings.Contains(lower, "hasanyrole"), strings.Contains(lower, "hasauthority"), strings.Contains(lower, "hasanyauthority"):
		return SecurityRole, "Role or authority requirement"
	case strings.Contains(lower, "isauthenticated"), strings.Contains(lower, "authenticated"):
		return SecurityAuthenticated, "Authenticated access required"
	default:
		return SecurityUnknown, "Unclassified authorization expression"
	}
}

func markConflictingProviderSecurity(records []SecurityEvidenceRecord) {
	hasPublic := false
	hasRestricted := false
	for _, record := range records {
		if record.Kind == SecurityPublic {
			hasPublic = true
		} else if record.Kind != SecurityUnknown {
			hasRestricted = true
		}
	}
	if !hasPublic || !hasRestricted {
		return
	}
	for index := range records {
		records[index].Conflicting = true
	}
}
