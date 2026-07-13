package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

type CanonicalDiagnosticRecord struct {
	ID                    string     `json:"id"`
	Code                  string     `json:"code"`
	Title                 string     `json:"title"`
	Category              string     `json:"category"`
	Severity              Severity   `json:"severity"`
	Confidence            Confidence `json:"confidence"`
	Resolution            Resolution `json:"resolution"`
	Explanation           string     `json:"explanation"`
	PossibleImpact        string     `json:"possible_impact"`
	EvidenceIDs           []string   `json:"evidence_ids,omitempty"`
	AffectedArtifacts     []string   `json:"affected_artifacts,omitempty"`
	NextChecks            []string   `json:"next_checks"`
	ConfigurationGuidance string     `json:"configuration_guidance,omitempty"`
}

type DiagnosticFamilyRecord struct {
	FamilyID         string   `json:"family_id"`
	Code             string   `json:"code"`
	Service          string   `json:"service,omitempty"`
	RoutePattern     string   `json:"route_pattern,omitempty"`
	RootCause        string   `json:"root_cause"`
	AffectedCount    int      `json:"affected_count"`
	ObservedCount    int      `json:"observed_count"`
	ResolvedCount    int      `json:"resolved_count"`
	UnresolvedCount  int      `json:"unresolved_count"`
	OutOfScopeCount  int      `json:"out_of_scope_count"`
	LikelyOwner      string   `json:"likely_owner,omitempty"`
	AffectedProjects []string `json:"affected_projects,omitempty"`
	DiagnosticIDs    []string `json:"diagnostic_ids"`
	EvidenceIDs      []string `json:"evidence_ids,omitempty"`
	NextChecks       []string `json:"next_checks"`
	SuggestedCheck   string   `json:"suggested_check"`
}

func BuildDiagnosticFamilies(project string, diagnostics []CanonicalDiagnosticRecord) []DiagnosticFamilyRecord {
	records := append([]CanonicalDiagnosticRecord(nil), diagnostics...)
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	byKey := map[string]*DiagnosticFamilyRecord{}
	for _, diagnostic := range records {
		pattern := diagnosticRoutePattern(diagnostic.AffectedArtifacts)
		key := diagnostic.Code + "\x00" + project + "\x00" + pattern
		family := byKey[key]
		if family == nil {
			id := stableDiagnosticID("family", key)
			id = "diagnostic-family:" + strings.TrimPrefix(id, "diagnostic:")
			check := "Inspect the cited evidence."
			if len(diagnostic.NextChecks) > 0 {
				check = diagnostic.NextChecks[0]
			}
			rootCause := diagnostic.Explanation
			if rootCause == "" {
				rootCause = diagnosticForCode(diagnostic.Code).Explanation
			}
			family = &DiagnosticFamilyRecord{FamilyID: id, Code: diagnostic.Code, Service: project, RoutePattern: pattern, RootCause: rootCause, LikelyOwner: project, AffectedProjects: nonEmptyStrings(project), SuggestedCheck: check}
			byKey[key] = family
		}
		family.AffectedCount++
		family.ObservedCount++
		switch diagnostic.Resolution {
		case ResolutionMatched:
			family.ResolvedCount++
		case ResolutionOutOfScope:
			family.OutOfScopeCount++
		default:
			family.UnresolvedCount++
		}
		family.DiagnosticIDs = append(family.DiagnosticIDs, diagnostic.ID)
		family.EvidenceIDs = append(family.EvidenceIDs, diagnostic.EvidenceIDs...)
		family.NextChecks = append(family.NextChecks, diagnostic.NextChecks...)
	}
	result := make([]DiagnosticFamilyRecord, 0, len(byKey))
	for _, family := range byKey {
		family.EvidenceIDs = uniqueSortedStrings(family.EvidenceIDs)
		family.NextChecks = uniqueSortedStrings(family.NextChecks)
		if len(family.NextChecks) == 0 {
			family.NextChecks = []string{family.SuggestedCheck}
		}
		result = append(result, *family)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].FamilyID < result[j].FamilyID })
	return result
}

func diagnosticRoutePattern(artifacts []string) string {
	for _, artifact := range artifacts {
		for _, field := range strings.Fields(artifact) {
			if !strings.HasPrefix(field, "/") {
				continue
			}
			path := strings.SplitN(field, "?", 2)[0]
			parts := strings.Split(strings.Trim(path, "/"), "/")
			if len(parts) > 1 {
				return "/" + parts[0] + "/{variant}"
			}
			return path
		}
	}
	return ""
}

func uniqueSortedStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func BuildCanonicalDiagnostics(matches []ContractMatchRecord, capabilities []CapabilityRecord) []CanonicalDiagnosticRecord {
	records := make([]CanonicalDiagnosticRecord, 0, len(matches))
	for _, match := range matches {
		code := firstNonEmpty(match.Issue, "information")
		record := diagnosticForCode(code)
		record.Code = code
		record.Confidence = normalizeDiagnosticConfidence(match.Confidence)
		record.EvidenceIDs = append([]string(nil), match.EvidenceIDs...)
		record.AffectedArtifacts = nonEmptyStrings(match.APIFile, match.BackendFile, match.APIHTTPMethod+" "+match.APIPath)
		record.ID = stableDiagnosticID(record.Code, match.APIHTTPMethod, match.APIPath, match.APIFile, match.BackendFile)
		records = append(records, record)
	}
	for _, capability := range capabilities {
		if capability.Coverage != CoverageFailed {
			continue
		}
		record := diagnosticForCode("analyzer_failed")
		record.Code = "analyzer_failed"
		record.Confidence = ConfidenceExact
		record.AffectedArtifacts = nonEmptyStrings(capability.Project, capability.Language, string(capability.ID))
		record.ID = stableDiagnosticID(record.Code, capability.Project, capability.Language, string(capability.ID))
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}

func diagnosticForCode(code string) CanonicalDiagnosticRecord {
	switch code {
	case "frontend_internal_api":
		return CanonicalDiagnosticRecord{Title: "Frontend-internal route", Category: "expected_behavior", Severity: SeverityInfo, Resolution: ResolutionOutOfScope, Explanation: "The request is handled inside the frontend boundary and is not expected to resolve to a backend service.", PossibleImpact: "No backend impact is expected unless the route was intended to leave the frontend.", NextChecks: []string{"Confirm that the frontend-internal boundary is intentional."}}
	case "method_mismatch":
		return CanonicalDiagnosticRecord{Title: "Frontend and backend use different HTTP methods", Category: "likely_defect", Severity: SeverityError, Resolution: ResolutionPartial, Explanation: "A related backend route exists, but its HTTP method does not match the client contract.", PossibleImpact: "The request may fail at runtime or reach a different handler.", NextChecks: []string{"Compare the client method with the backend route.", "Check for a stale gateway or client contract."}}
	case "indexed_backend_route_missing":
		return CanonicalDiagnosticRecord{Title: "No matching indexed backend route", Category: "missing_scan_coverage", Severity: SeverityWarning, Resolution: ResolutionUnresolved, Explanation: "GoreGraph could not connect the client contract to a route in the indexed backend projects.", PossibleImpact: "The service may be unindexed, dynamically configured, or the route may be missing.", NextChecks: []string{"Confirm that the owning backend project was scanned.", "Inspect gateway prefixes and nearby routes."}}
	case "unscanned_service":
		return CanonicalDiagnosticRecord{Title: "Referenced service is not indexed", Category: "missing_scan_coverage", Severity: SeverityWarning, Resolution: ResolutionUnresolved, Explanation: "A client contract references a service whose route index is not available in the current scan scope.", PossibleImpact: "Endpoint and impact results may be incomplete until the owning project is scanned.", NextChecks: []string{"Scan the owning service project.", "Confirm the configured service alias."}}
	case "dynamic_endpoint_unresolved":
		return CanonicalDiagnosticRecord{Title: "Endpoint is built dynamically", Category: "dynamic_or_ambiguous", Severity: SeverityWarning, Resolution: ResolutionPartial, Explanation: "A dynamic route or service segment is composed at runtime, so static analysis cannot enumerate every possible provider safely.", PossibleImpact: "The provider and downstream path may be incomplete.", NextChecks: []string{"Inspect the dynamic variable or configuration value in the cited source expression.", "Compare its possible values with the expected service and route mapping."}}
	case "analyzer_failed":
		return CanonicalDiagnosticRecord{Title: "Analyzer capability failed", Category: "missing_scan_coverage", Severity: SeverityError, Resolution: ResolutionUnresolved, Explanation: "An expected analyzer capability failed and its facts are incomplete.", PossibleImpact: "Queries may omit relationships from the affected capability.", NextChecks: []string{"Run goregraph doctor and inspect the capability failure.", "Rescan after correcting the analyzer input."}}
	default:
		return CanonicalDiagnosticRecord{Title: "Diagnostic information", Category: "information", Severity: SeverityInfo, Resolution: ResolutionPartial, Explanation: "GoreGraph recorded a relationship that requires manual interpretation.", PossibleImpact: "Static analysis may be incomplete for this relationship.", NextChecks: []string{"Inspect the cited evidence and affected artifacts."}}
	}
}

func normalizeDiagnosticConfidence(value string) Confidence {
	switch strings.ToUpper(value) {
	case "EXACT", "EXTRACTED":
		return ConfidenceExact
	case "RESOLVED", "MATCHED":
		return ConfidenceResolved
	case "NORMALIZED":
		return ConfidenceNormalized
	case "INFERRED":
		return ConfidenceInferred
	case "WEAK", "WEAK_MATCH":
		return ConfidenceWeak
	default:
		return ConfidenceUnknown
	}
}
func stableDiagnosticID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "diagnostic:" + hex.EncodeToString(sum[:16])
}
func nonEmptyStrings(values ...string) []string {
	result := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}
