package scan

import "testing"

func TestNormalizeSecurityEvidenceDistinguishesUnknownFromExplicitPublic(t *testing.T) {
	unknown := NormalizeSecurityEvidence(nil)
	if len(unknown) != 1 || unknown[0].Kind != SecurityUnknown {
		t.Fatalf("unknown=%#v", unknown)
	}

	public := NormalizeSecurityEvidence([]AuthRecord{{
		Kind: "permit_all", Source: "security_config_call", Confidence: "EXTRACTED", File: "Security.java", Line: 12,
	}})
	if len(public) != 1 || public[0].Kind != SecurityPublic {
		t.Fatalf("public=%#v", public)
	}
}

func TestNormalizeSecurityEvidenceMapsExplicitKindsAndPreservesProvenance(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want string
	}{
		{name: "http basic", kind: "http_basic", want: SecurityBasic},
		{name: "bearer resource server", kind: "oauth2_resource_server", want: SecurityBearer},
		{name: "oauth2 login", kind: "oauth2_login", want: SecurityOAuth2},
		{name: "OpenAPI api key", kind: "api_key", want: SecurityAPIKey},
		{name: "form session login", kind: "form_login", want: SecuritySession},
		{name: "x509 mtls", kind: "x509", want: SecurityMTLS},
		{name: "role", kind: "has_role", want: SecurityRole},
		{name: "authority", kind: "has_authority", want: SecurityRole},
		{name: "authenticated", kind: "authenticated", want: SecurityAuthenticated},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			records := NormalizeSecurityEvidence([]AuthRecord{{
				Kind:       test.kind,
				Expression: "explicit-expression",
				Source:     "method_annotation",
				Confidence: "EXTRACTED",
				File:       "src/Security.java",
				Line:       17,
			}})
			if len(records) != 1 {
				t.Fatalf("records=%#v", records)
			}
			got := records[0]
			if got.Kind != test.want || got.Expression != "explicit-expression" || got.Source != "method_annotation" || got.File != "src/Security.java" || got.Line != 17 || got.Confidence != ConfidenceExact {
				t.Fatalf("record=%#v, want kind=%q with retained provenance", got, test.want)
			}
		})
	}
}

func TestNormalizeSecurityEvidenceMarksBroaderConfigurationAndConflicts(t *testing.T) {
	records := NormalizeSecurityEvidence([]AuthRecord{
		{Kind: "permit_all", Expression: "/health/**", Source: "security_config_call", Confidence: "EXTRACTED", File: "Security.java", Line: 12},
		{Kind: "authenticated", Expression: "anyRequest", Source: "security_config_call", Confidence: "EXTRACTED", File: "Security.java", Line: 13},
	})
	if len(records) != 2 {
		t.Fatalf("records=%#v", records)
	}
	seen := map[string]bool{}
	for _, record := range records {
		seen[record.Kind] = true
		if !record.Conflicting {
			t.Fatalf("conflict not retained on %#v", record)
		}
		if len(record.Limitations) != 1 {
			t.Fatalf("broader configuration limitation missing from %#v", record)
		}
	}
	if !seen[SecurityPublic] || !seen[SecurityAuthenticated] {
		t.Fatalf("conflicting kinds=%#v", records)
	}
}
