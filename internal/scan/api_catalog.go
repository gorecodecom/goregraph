package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Normalized API security categories retain a language-neutral security meaning.
const (
	SecurityBasic         = "basic"
	SecurityBearer        = "bearer"
	SecurityOAuth2        = "oauth2"
	SecurityAPIKey        = "api_key"
	SecuritySession       = "session"
	SecurityMTLS          = "mtls"
	SecurityRole          = "role"
	SecurityAuthenticated = "authenticated"
	SecurityPublic        = "public"
	SecurityUnknown       = "unknown"
)

// APICatalogRecord is the canonical provider-oriented endpoint inventory.
type APICatalogRecord struct {
	SchemaVersion int                 `json:"schema_version"`
	Generated     string              `json:"generated,omitempty"`
	Root          string              `json:"root,omitempty"`
	Endpoints     []APIEndpointRecord `json:"endpoints"`
}

// APIEndpointRecord describes one provider endpoint and its discovered consumers.
type APIEndpointRecord struct {
	ID              string                   `json:"id"`
	ProviderProject string                   `json:"provider_project"`
	ProviderService string                   `json:"provider_service,omitempty"`
	ProviderRole    string                   `json:"provider_role,omitempty"`
	Transport       string                   `json:"transport"`
	HTTPMethod      string                   `json:"http_method"`
	Path            string                   `json:"path"`
	RawPath         string                   `json:"raw_path,omitempty"`
	Language        string                   `json:"language,omitempty"`
	Framework       string                   `json:"framework,omitempty"`
	Controller      string                   `json:"controller,omitempty"`
	Handler         string                   `json:"handler,omitempty"`
	File            string                   `json:"file,omitempty"`
	Line            int                      `json:"line,omitempty"`
	Parameters      []APIParameterRecord     `json:"parameters,omitempty"`
	Consumes        []string                 `json:"consumes,omitempty"`
	Produces        []string                 `json:"produces,omitempty"`
	RequestType     string                   `json:"request_type,omitempty"`
	ResponseType    string                   `json:"response_type,omitempty"`
	Security        []SecurityEvidenceRecord `json:"security"`
	Consumers       []APIConsumerRecord      `json:"consumers"`
	Mismatches      []APIMismatchRecord      `json:"mismatches,omitempty"`
	Confidence      Confidence               `json:"confidence"`
	Coverage        Coverage                 `json:"coverage"`
	Limitations     []string                 `json:"limitations,omitempty"`
	EvidenceIDs     []string                 `json:"evidence_ids,omitempty"`
}

// APIParameterRecord describes one statically discovered endpoint parameter.
type APIParameterRecord struct {
	Name       string     `json:"name"`
	Location   string     `json:"location"`
	Type       string     `json:"type,omitempty"`
	Required   bool       `json:"required,omitempty"`
	Source     string     `json:"source,omitempty"`
	Confidence Confidence `json:"confidence,omitempty"`
}

// SecurityEvidenceRecord preserves one normalized security fact and its provenance.
type SecurityEvidenceRecord struct {
	Kind        string     `json:"kind"`
	Summary     string     `json:"summary"`
	Expression  string     `json:"expression,omitempty"`
	Source      string     `json:"source,omitempty"`
	File        string     `json:"file,omitempty"`
	Line        int        `json:"line,omitempty"`
	Confidence  Confidence `json:"confidence"`
	Conflicting bool       `json:"conflicting,omitempty"`
	Limitations []string   `json:"limitations,omitempty"`
	EvidenceIDs []string   `json:"evidence_ids,omitempty"`
}

// APIConsumerRecord describes one call site resolved or proposed for an endpoint.
type APIConsumerRecord struct {
	ID          string                   `json:"id"`
	Project     string                   `json:"project"`
	Service     string                   `json:"service,omitempty"`
	Role        string                   `json:"role,omitempty"`
	Caller      string                   `json:"caller,omitempty"`
	File        string                   `json:"file,omitempty"`
	Line        int                      `json:"line,omitempty"`
	HTTPMethod  string                   `json:"http_method,omitempty"`
	Path        string                   `json:"path,omitempty"`
	CallAuth    []SecurityEvidenceRecord `json:"call_auth"`
	Resolution  string                   `json:"resolution"`
	Confidence  Confidence               `json:"confidence"`
	Limitations []string                 `json:"limitations,omitempty"`
	EvidenceIDs []string                 `json:"evidence_ids,omitempty"`
}

// APIMismatchRecord describes one evidence-backed provider-consumer discrepancy.
type APIMismatchRecord struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Severity    string     `json:"severity"`
	Reason      string     `json:"reason"`
	Confidence  Confidence `json:"confidence"`
	EvidenceIDs []string   `json:"evidence_ids,omitempty"`
}

// StableAPIEndpointID returns an order-independent identity for one provider route.
func StableAPIEndpointID(provider, transport, method, routePath, handler, file string, line int) string {
	parts := []string{
		canonicalAPIIdentityPart(provider),
		canonicalAPIIdentityPart(transport),
		strings.ToUpper(strings.TrimSpace(method)),
		normalizeAPIPathParameterNames(routePath),
		canonicalAPIIdentityPart(handler),
		canonicalAPIIdentityPart(file),
		strconv.Itoa(line),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "endpoint:" + hex.EncodeToString(sum[:16])
}

// SortAPICatalog canonicalizes every ordering-insensitive catalog collection.
func SortAPICatalog(catalog *APICatalogRecord) {
	if catalog == nil {
		return
	}
	if catalog.Endpoints == nil {
		catalog.Endpoints = make([]APIEndpointRecord, 0)
	}
	for endpointIndex := range catalog.Endpoints {
		endpoint := &catalog.Endpoints[endpointIndex]
		if endpoint.Security == nil {
			endpoint.Security = make([]SecurityEvidenceRecord, 0)
		}
		if endpoint.Consumers == nil {
			endpoint.Consumers = make([]APIConsumerRecord, 0)
		}
		sort.Slice(endpoint.Parameters, func(left, right int) bool {
			return apiParameterSortKey(endpoint.Parameters[left]) < apiParameterSortKey(endpoint.Parameters[right])
		})
		sort.Strings(endpoint.Consumes)
		sort.Strings(endpoint.Produces)
		sort.Strings(endpoint.Limitations)
		sort.Strings(endpoint.EvidenceIDs)

		for securityIndex := range endpoint.Security {
			sortSecurityEvidenceRecord(&endpoint.Security[securityIndex])
		}
		sort.Slice(endpoint.Security, func(left, right int) bool {
			return securityEvidenceSortKey(endpoint.Security[left]) < securityEvidenceSortKey(endpoint.Security[right])
		})

		for consumerIndex := range endpoint.Consumers {
			consumer := &endpoint.Consumers[consumerIndex]
			if consumer.CallAuth == nil {
				consumer.CallAuth = make([]SecurityEvidenceRecord, 0)
			}
			sort.Strings(consumer.Limitations)
			sort.Strings(consumer.EvidenceIDs)
			for authIndex := range consumer.CallAuth {
				sortSecurityEvidenceRecord(&consumer.CallAuth[authIndex])
			}
			sort.Slice(consumer.CallAuth, func(left, right int) bool {
				return securityEvidenceSortKey(consumer.CallAuth[left]) < securityEvidenceSortKey(consumer.CallAuth[right])
			})
		}
		sort.Slice(endpoint.Consumers, func(left, right int) bool {
			return apiConsumerSortKey(endpoint.Consumers[left]) < apiConsumerSortKey(endpoint.Consumers[right])
		})

		for mismatchIndex := range endpoint.Mismatches {
			sort.Strings(endpoint.Mismatches[mismatchIndex].EvidenceIDs)
		}
		sort.Slice(endpoint.Mismatches, func(left, right int) bool {
			return apiMismatchSortKey(endpoint.Mismatches[left]) < apiMismatchSortKey(endpoint.Mismatches[right])
		})
	}
	sort.Slice(catalog.Endpoints, func(left, right int) bool {
		return apiEndpointSortKey(catalog.Endpoints[left]) < apiEndpointSortKey(catalog.Endpoints[right])
	})
}

// ValidateAPICatalog checks identities, provenance, normalized security, and canonical order.
func ValidateAPICatalog(catalog APICatalogRecord) error {
	endpointIDs := make(map[string]struct{}, len(catalog.Endpoints))
	consumerIDs := make(map[string]struct{})
	for endpointIndex := range catalog.Endpoints {
		endpoint := catalog.Endpoints[endpointIndex]
		if strings.TrimSpace(endpoint.ID) == "" {
			return fmt.Errorf("API catalog contains empty endpoint ID at index %d", endpointIndex)
		}
		if _, exists := endpointIDs[endpoint.ID]; exists {
			return fmt.Errorf("API catalog contains duplicate endpoint ID %q", endpoint.ID)
		}
		endpointIDs[endpoint.ID] = struct{}{}
		if err := validateAPISourceLocation("endpoint "+endpoint.ID, endpoint.File, endpoint.Line); err != nil {
			return err
		}
		if err := validateAPIEvidenceIDs("endpoint "+endpoint.ID, endpoint.EvidenceIDs); err != nil {
			return err
		}
		if !sort.SliceIsSorted(endpoint.Parameters, func(left, right int) bool {
			return apiParameterSortKey(endpoint.Parameters[left]) < apiParameterSortKey(endpoint.Parameters[right])
		}) {
			return fmt.Errorf("endpoint %q parameters are not in canonical order", endpoint.ID)
		}
		if err := validateCanonicalAPIStrings("endpoint "+endpoint.ID, "consumes", endpoint.Consumes); err != nil {
			return err
		}
		if err := validateCanonicalAPIStrings("endpoint "+endpoint.ID, "produces", endpoint.Produces); err != nil {
			return err
		}
		if err := validateCanonicalAPIStrings("endpoint "+endpoint.ID, "limitations", endpoint.Limitations); err != nil {
			return err
		}
		for securityIndex := range endpoint.Security {
			owner := fmt.Sprintf("endpoint %q security[%d]", endpoint.ID, securityIndex)
			if err := validateSecurityEvidence(endpoint.Security[securityIndex], owner); err != nil {
				return err
			}
			if err := validateCanonicalAPIStrings(owner, "limitations", endpoint.Security[securityIndex].Limitations); err != nil {
				return err
			}
		}
		if !sort.SliceIsSorted(endpoint.Security, func(left, right int) bool {
			return securityEvidenceSortKey(endpoint.Security[left]) < securityEvidenceSortKey(endpoint.Security[right])
		}) {
			return fmt.Errorf("endpoint %q security is not in canonical order", endpoint.ID)
		}
		for consumerIndex := range endpoint.Consumers {
			consumer := endpoint.Consumers[consumerIndex]
			if strings.TrimSpace(consumer.ID) == "" {
				return fmt.Errorf("endpoint %q contains empty consumer ID at index %d", endpoint.ID, consumerIndex)
			}
			if _, exists := consumerIDs[consumer.ID]; exists {
				return fmt.Errorf("API catalog contains duplicate consumer ID %q", consumer.ID)
			}
			consumerIDs[consumer.ID] = struct{}{}
			if err := validateAPISourceLocation("consumer "+consumer.ID, consumer.File, consumer.Line); err != nil {
				return err
			}
			if err := validateAPIEvidenceIDs("consumer "+consumer.ID, consumer.EvidenceIDs); err != nil {
				return err
			}
			if err := validateCanonicalAPIStrings("consumer "+consumer.ID, "limitations", consumer.Limitations); err != nil {
				return err
			}
			for authIndex := range consumer.CallAuth {
				owner := fmt.Sprintf("consumer %q call_auth[%d]", consumer.ID, authIndex)
				if err := validateSecurityEvidence(consumer.CallAuth[authIndex], owner); err != nil {
					return err
				}
				if err := validateCanonicalAPIStrings(owner, "limitations", consumer.CallAuth[authIndex].Limitations); err != nil {
					return err
				}
			}
			if !sort.SliceIsSorted(consumer.CallAuth, func(left, right int) bool {
				return securityEvidenceSortKey(consumer.CallAuth[left]) < securityEvidenceSortKey(consumer.CallAuth[right])
			}) {
				return fmt.Errorf("consumer %q call auth is not in canonical order", consumer.ID)
			}
		}
		for mismatchIndex := range endpoint.Mismatches {
			mismatch := endpoint.Mismatches[mismatchIndex]
			if err := validateAPIEvidenceIDs("mismatch "+mismatch.ID, mismatch.EvidenceIDs); err != nil {
				return err
			}
		}
	}

	if !sort.SliceIsSorted(catalog.Endpoints, func(left, right int) bool {
		return apiEndpointSortKey(catalog.Endpoints[left]) < apiEndpointSortKey(catalog.Endpoints[right])
	}) {
		return fmt.Errorf("API catalog endpoints are not in canonical order")
	}
	for endpointIndex := range catalog.Endpoints {
		endpoint := catalog.Endpoints[endpointIndex]
		if !sort.SliceIsSorted(endpoint.Consumers, func(left, right int) bool {
			return apiConsumerSortKey(endpoint.Consumers[left]) < apiConsumerSortKey(endpoint.Consumers[right])
		}) {
			return fmt.Errorf("endpoint %q consumers are not in canonical order", endpoint.ID)
		}
		if !sort.SliceIsSorted(endpoint.Mismatches, func(left, right int) bool {
			return apiMismatchSortKey(endpoint.Mismatches[left]) < apiMismatchSortKey(endpoint.Mismatches[right])
		}) {
			return fmt.Errorf("endpoint %q mismatches are not in canonical order", endpoint.ID)
		}
	}
	return nil
}

func canonicalAPIIdentityPart(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(filepath.ToSlash(value), `\`, "/"))
}

func normalizeAPIPathParameterNames(routePath string) string {
	var normalized strings.Builder
	for index := 0; index < len(routePath); {
		if routePath[index] != '{' {
			normalized.WriteByte(routePath[index])
			index++
			continue
		}
		end := matchingRouteParameterBrace(routePath, index)
		if end < 0 {
			normalized.WriteString(routePath[index:])
			break
		}
		contents := routePath[index+1 : end]
		if constraintAt := strings.IndexByte(contents, ':'); constraintAt >= 0 {
			normalized.WriteString("{_")
			normalized.WriteString(contents[constraintAt:])
			normalized.WriteByte('}')
		} else {
			normalized.WriteString("{_}")
		}
		index = end + 1
	}
	return normalized.String()
}

func matchingRouteParameterBrace(routePath string, start int) int {
	depth := 0
	for index := start; index < len(routePath); index++ {
		switch routePath[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func validateSecurityEvidence(record SecurityEvidenceRecord, owner string) error {
	if !knownSecurityKind(record.Kind) {
		return fmt.Errorf("%s contains unknown security kind %q", owner, record.Kind)
	}
	if err := validateAPISourceLocation(owner, record.File, record.Line); err != nil {
		return err
	}
	return validateAPIEvidenceIDs(owner, record.EvidenceIDs)
}

func knownSecurityKind(kind string) bool {
	switch kind {
	case SecurityBasic, SecurityBearer, SecurityOAuth2, SecurityAPIKey, SecuritySession,
		SecurityMTLS, SecurityRole, SecurityAuthenticated, SecurityPublic, SecurityUnknown:
		return true
	default:
		return false
	}
}

func validateAPISourceLocation(owner, file string, line int) error {
	if line < 0 {
		return fmt.Errorf("%s contains non-one-based or absent line %d", owner, line)
	}
	if file != "" && !isWorkspaceRelativeAPIFile(file) {
		return fmt.Errorf("%s contains non-workspace-relative file %q", owner, file)
	}
	return nil
}

func isWorkspaceRelativeAPIFile(file string) bool {
	if strings.TrimSpace(file) != file || file == "" || strings.ContainsRune(file, '\x00') {
		return false
	}
	portable := strings.ReplaceAll(file, `\`, "/")
	if strings.HasPrefix(portable, "/") || hasWindowsVolumePrefix(portable) {
		return false
	}
	clean := path.Clean(portable)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func hasWindowsVolumePrefix(file string) bool {
	return len(file) >= 2 && ((file[0] >= 'a' && file[0] <= 'z') || (file[0] >= 'A' && file[0] <= 'Z')) && file[1] == ':'
}

func validateAPIEvidenceIDs(owner string, evidenceIDs []string) error {
	previous := ""
	for index, evidenceID := range evidenceIDs {
		if evidenceID == "" {
			return fmt.Errorf("%s contains empty evidence IDs entry at index %d", owner, index)
		}
		if index > 0 && evidenceID <= previous {
			return fmt.Errorf("%s contains duplicate or unsorted evidence IDs at %q", owner, evidenceID)
		}
		previous = evidenceID
	}
	return nil
}

func validateCanonicalAPIStrings(owner, field string, values []string) error {
	if !sort.StringsAreSorted(values) {
		return fmt.Errorf("%s %s are not in canonical order", owner, field)
	}
	return nil
}

func sortSecurityEvidenceRecord(record *SecurityEvidenceRecord) {
	sort.Strings(record.Limitations)
	sort.Strings(record.EvidenceIDs)
}

func apiEndpointSortKey(record APIEndpointRecord) string {
	return joinAPISortKey(record.ProviderProject, record.ProviderService, record.Transport, record.HTTPMethod, record.Path, record.Handler, record.File, strconv.Itoa(record.Line), record.ID)
}

func apiParameterSortKey(record APIParameterRecord) string {
	return joinAPISortKey(record.Location, record.Name, record.Type, strconv.FormatBool(record.Required), record.Source, string(record.Confidence))
}

func securityEvidenceSortKey(record SecurityEvidenceRecord) string {
	return joinAPISortKey(record.Kind, record.Summary, record.Expression, record.Source, record.File, strconv.Itoa(record.Line), string(record.Confidence), strconv.FormatBool(record.Conflicting), strings.Join(record.Limitations, "\x01"), strings.Join(record.EvidenceIDs, "\x01"))
}

func apiConsumerSortKey(record APIConsumerRecord) string {
	return joinAPISortKey(record.Project, record.Service, record.HTTPMethod, record.Path, record.Caller, record.File, strconv.Itoa(record.Line), record.ID)
}

func apiMismatchSortKey(record APIMismatchRecord) string {
	return joinAPISortKey(record.Kind, record.Severity, record.Reason, record.ID, string(record.Confidence), strings.Join(record.EvidenceIDs, "\x01"))
}

func joinAPISortKey(parts ...string) string {
	return strings.Join(parts, "\x00")
}
