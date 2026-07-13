package scan

import "testing"

func TestCanonicalDiagnosticsSeparateExpectedBehaviorFromDefects(t *testing.T) {
	matches := []ContractMatchRecord{
		{APIHTTPMethod: "GET", APIPath: "/internal/session", APIFile: "src/api.ts", Issue: "frontend_internal_api", Confidence: "EXTRACTED", Reason: "frontend-internal-api-route", EvidenceIDs: []string{"evidence:internal"}},
		{APIHTTPMethod: "PUT", APIPath: "/users/{id}", APIFile: "src/users.ts", Issue: "method_mismatch", Confidence: "MATCHED", Reason: "backend route uses POST", EvidenceIDs: []string{"evidence:mismatch"}},
	}
	records := BuildCanonicalDiagnostics(matches, nil)
	if len(records) != 2 {
		t.Fatalf("len = %d, want 2", len(records))
	}
	internal := findCanonicalDiagnostic(t, records, "frontend_internal_api")
	if internal.Category != "expected_behavior" || internal.Severity != SeverityInfo || internal.Resolution != ResolutionOutOfScope {
		t.Fatalf("unexpected internal diagnostic: %#v", internal)
	}
	mismatch := findCanonicalDiagnostic(t, records, "method_mismatch")
	if mismatch.Category != "likely_defect" || mismatch.Severity != SeverityError || mismatch.Resolution != ResolutionPartial {
		t.Fatalf("unexpected mismatch diagnostic: %#v", mismatch)
	}
	for _, record := range records {
		if record.ID == "" || record.Title == "" || record.Explanation == "" || record.PossibleImpact == "" || len(record.NextChecks) == 0 || len(record.EvidenceIDs) == 0 {
			t.Fatalf("incomplete canonical diagnostic: %#v", record)
		}
	}
}

func TestDiagnosticFamiliesCollapseRouteVariantsByRootCause(t *testing.T) {
	records := []CanonicalDiagnosticRecord{
		{ID: "diagnostic:a", Code: "indexed_backend_route_missing", Explanation: "No matching route.", AffectedArtifacts: []string{"GET /tree/alpha"}, EvidenceIDs: []string{"evidence:a"}, NextChecks: []string{"Check the backend route."}},
		{ID: "diagnostic:b", Code: "indexed_backend_route_missing", Explanation: "No matching route.", AffectedArtifacts: []string{"GET /tree/beta/children"}, EvidenceIDs: []string{"evidence:b"}, NextChecks: []string{"Check the backend route."}},
		{ID: "diagnostic:c", Code: "method_mismatch", Explanation: "HTTP method differs.", AffectedArtifacts: []string{"POST /tree/alpha"}, EvidenceIDs: []string{"evidence:c"}, NextChecks: []string{"Compare methods."}},
	}
	families := BuildDiagnosticFamilies("services/tree", records)
	t.Logf("diagnostic families: %#v", families)
	if len(families) != 2 {
		t.Fatalf("families = %#v, want 2", families)
	}
	missing := families[0]
	if missing.Code != "indexed_backend_route_missing" {
		missing = families[1]
	}
	if missing.RoutePattern != "/tree/{variant}" || missing.AffectedCount != 2 || len(missing.DiagnosticIDs) != 2 || len(missing.EvidenceIDs) != 2 {
		t.Fatalf("missing-route family not collapsed: %#v", missing)
	}
	if missing.FamilyID == "" || missing.RootCause == "" || missing.SuggestedCheck == "" {
		t.Fatalf("family lacks guidance: %#v", missing)
	}
}

func findCanonicalDiagnostic(t *testing.T, records []CanonicalDiagnosticRecord, code string) CanonicalDiagnosticRecord {
	t.Helper()
	for _, record := range records {
		if record.Code == code {
			return record
		}
	}
	t.Fatalf("missing diagnostic %s", code)
	return CanonicalDiagnosticRecord{}
}
