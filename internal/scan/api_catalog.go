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

// BuildWorkspaceAPICatalog merges complete provider inventories with canonically resolved consumers.
func BuildWorkspaceAPICatalog(registry WorkspaceRegistryRecord, projects []workspaceIndexProject, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, generated string) (APICatalogRecord, error) {
	projectsByPath := make(map[string]workspaceIndexProject, len(projects))
	registryByPath := make(map[string]WorkspaceProjectRecord, len(registry.Projects))
	for _, project := range registry.Projects {
		registryByPath[filepath.ToSlash(project.Path)] = project
	}

	catalog := APICatalogRecord{SchemaVersion: SchemaVersion, Generated: generated, Root: filepath.ToSlash(registry.Root), Endpoints: []APIEndpointRecord{}}
	for _, project := range projects {
		projectPath := filepath.ToSlash(project.record.Path)
		if projectPath == "" {
			return APICatalogRecord{}, fmt.Errorf("workspace API catalog contains project with empty path")
		}
		if _, exists := projectsByPath[projectPath]; exists {
			return APICatalogRecord{}, fmt.Errorf("workspace API catalog contains duplicate project path %q", projectPath)
		}
		projectsByPath[projectPath] = project
		projectCatalog := BuildProjectAPICatalog(projectPath, generated, project.routes, SpringIndex{Endpoints: project.endpoints}, nil, project.capabilities)
		metadata := project.record
		if registered, ok := registryByPath[projectPath]; ok {
			metadata = registered
		}
		for _, endpoint := range projectCatalog.Endpoints {
			endpoint.ProviderProject = projectPath
			endpoint.ProviderService = metadata.Service
			endpoint.ProviderRole = firstNonEmpty(metadata.Kind, endpoint.ProviderRole)
			applyWorkspaceFlowTypes(&endpoint, flows)
			catalog.Endpoints = append(catalog.Endpoints, endpoint)
		}
	}

	for _, match := range matches {
		candidateIndexes := workspaceCatalogEndpointCandidates(catalog.Endpoints, match)
		if !resolvedWorkspaceCatalogMatch(match) {
			if ambiguousWorkspaceCatalogMatch(match) {
				for _, endpointIndex := range candidateIndexes {
					addWorkspaceCatalogMismatch(&catalog.Endpoints[endpointIndex], APIConsumerRecord{}, "ambiguous_route_match", "WARNING", "Canonical route evidence is ambiguous; the consumer was preserved as an unresolved candidate and was not assigned to a provider endpoint.", ConfidenceInferred, workspaceCatalogEvidenceIDs(catalog.Endpoints[endpointIndex], match.ID))
				}
			}
			continue
		}
		if len(candidateIndexes) != 1 {
			if len(candidateIndexes) > 1 {
				for _, endpointIndex := range candidateIndexes {
					addWorkspaceCatalogMismatch(&catalog.Endpoints[endpointIndex], APIConsumerRecord{}, "ambiguous_route_match", "WARNING", "Canonical match metadata resolves to multiple provider endpoints; the consumer was not assigned.", ConfidenceInferred, workspaceCatalogEvidenceIDs(catalog.Endpoints[endpointIndex], match.ID))
				}
			}
			continue
		}

		endpoint := &catalog.Endpoints[candidateIndexes[0]]
		consumerProject, hasConsumerProject := projectsByPath[filepath.ToSlash(match.APIProject)]
		contract, hasContract := exactWorkspaceCatalogContract(consumerProject.contracts, match)
		consumerMetadata := consumerProject.record
		if registered, ok := registryByPath[filepath.ToSlash(match.APIProject)]; ok {
			consumerMetadata = registered
		}
		consumer := APIConsumerRecord{
			ID:          StableWorkspaceID("api-consumer", match.ID, match.APIProject, match.APIHTTPMethod, match.APIPath, match.APIFile, strconv.Itoa(match.APILine)),
			Project:     filepath.ToSlash(match.APIProject),
			Service:     consumerMetadata.Service,
			Role:        consumerMetadata.Kind,
			Caller:      match.APICaller,
			File:        filepath.ToSlash(match.APIFile),
			Line:        match.APILine,
			HTTPMethod:  strings.ToUpper(strings.TrimSpace(match.APIHTTPMethod)),
			Path:        match.APIPath,
			Resolution:  strings.ToUpper(match.Issue),
			Confidence:  apiRouteConfidence(match.Confidence),
			EvidenceIDs: catalogUniqueSortedStrings([]string{match.ID}),
		}
		if hasConsumerProject && hasContract {
			consumer.Caller = firstNonEmpty(match.APICaller, contract.Caller)
			consumer.CallAuth = normalizeConsumerCallAuth(contract.Auth)
		} else {
			consumer.CallAuth = []SecurityEvidenceRecord{}
			consumer.Limitations = []string{"Outbound contract authentication evidence was unavailable for this canonical match."}
		}
		endpoint.Consumers = append(endpoint.Consumers, consumer)
		addWorkspaceSecurityMismatches(endpoint, consumer, match.ID, hasContract && len(contract.Auth) > 0)
	}

	for endpointIndex := range catalog.Endpoints {
		addWorkspaceProviderConflictMismatch(&catalog.Endpoints[endpointIndex])
	}
	SortAPICatalog(&catalog)
	if err := ValidateAPICatalog(catalog); err != nil {
		return APICatalogRecord{}, err
	}
	return catalog, nil
}

func resolvedWorkspaceCatalogMatch(match WorkspaceContractMatchRecord) bool {
	return match.Issue == contractIssueMatched && strings.EqualFold(match.Confidence, "RESOLVED") && strings.TrimSpace(match.BackendProject) != ""
}

func ambiguousWorkspaceCatalogMatch(match WorkspaceContractMatchRecord) bool {
	return strings.Contains(strings.ToLower(match.Issue), "ambiguous") || strings.Contains(strings.ToLower(match.Confidence), "ambiguous")
}

func workspaceCatalogEndpointCandidates(endpoints []APIEndpointRecord, match WorkspaceContractMatchRecord) []int {
	var candidates []int
	for index, endpoint := range endpoints {
		if match.BackendProject != "" && filepath.ToSlash(match.BackendProject) != endpoint.ProviderProject {
			continue
		}
		if match.BackendHTTPMethod != "" && !strings.EqualFold(match.BackendHTTPMethod, endpoint.HTTPMethod) {
			continue
		}
		if match.BackendPath != "" && normalizeAPIPathParameterNames(canonicalProviderPath(match.BackendPath)) != normalizeAPIPathParameterNames(endpoint.Path) {
			continue
		}
		if match.BackendHandler != "" && match.BackendHandler != endpoint.Handler {
			continue
		}
		if match.BackendFile != "" && filepath.ToSlash(match.BackendFile) != endpoint.File {
			continue
		}
		if match.BackendLine > 0 && match.BackendLine != endpoint.Line {
			continue
		}
		candidates = append(candidates, index)
	}
	return candidates
}

func exactWorkspaceCatalogContract(contracts []APIContractRecord, match WorkspaceContractMatchRecord) (APIContractRecord, bool) {
	var candidates []APIContractRecord
	for _, contract := range contracts {
		if !strings.EqualFold(contract.HTTPMethod, match.APIHTTPMethod) || contract.Path != match.APIPath || filepath.ToSlash(contract.File) != filepath.ToSlash(match.APIFile) || contract.Line != match.APILine {
			continue
		}
		if match.APICaller != "" && contract.Caller != match.APICaller {
			continue
		}
		candidates = append(candidates, contract)
	}
	if len(candidates) != 1 {
		return APIContractRecord{}, false
	}
	return candidates[0], true
}

func normalizeConsumerCallAuth(records []AuthRecord) []SecurityEvidenceRecord {
	if len(records) == 0 {
		return []SecurityEvidenceRecord{}
	}
	return NormalizeSecurityEvidence(records)
}

func applyWorkspaceFlowTypes(endpoint *APIEndpointRecord, flows []WorkspaceFeatureFlowRecord) {
	requestTypes := map[string]bool{}
	responseTypes := map[string]bool{}
	for _, flow := range flows {
		if filepath.ToSlash(flow.BackendProject) != endpoint.ProviderProject || !strings.EqualFold(flow.HTTPMethod, endpoint.HTTPMethod) || !pathsCompatibleWithKnownBasePrefixes(flow.Path, endpoint.Path) {
			continue
		}
		flowHandler := qualifiedName(flow.BackendController, flow.BackendMethod)
		if flowHandler == "" || flowHandler != endpoint.Handler {
			continue
		}
		if flow.BackendRequestType != "" {
			requestTypes[flow.BackendRequestType] = true
		}
		if flow.BackendReturnType != "" {
			responseTypes[flow.BackendReturnType] = true
		}
	}
	mergeWorkspaceFlowType(&endpoint.RequestType, requestTypes, "request_type", &endpoint.Limitations)
	mergeWorkspaceFlowType(&endpoint.ResponseType, responseTypes, "response_type", &endpoint.Limitations)
}

func mergeWorkspaceFlowType(target *string, candidates map[string]bool, field string, limitations *[]string) {
	if *target != "" || len(candidates) == 0 {
		return
	}
	values := make([]string, 0, len(candidates))
	for value := range candidates {
		values = append(values, value)
	}
	sort.Strings(values)
	if len(values) == 1 {
		*target = values[0]
		return
	}
	*limitations = append(*limitations, "workspace_flow_"+field+"_conflict: "+strings.Join(values, " | "))
}

func addWorkspaceSecurityMismatches(endpoint *APIEndpointRecord, consumer APIConsumerRecord, matchID string, hasCallAuthEvidence bool) {
	providerKinds := workspaceSecurityKinds(endpoint.Security)
	consumerKinds := workspaceSecurityKinds(consumer.CallAuth)
	evidenceIDs := workspaceCatalogEvidenceIDs(*endpoint, matchID, consumer.ID)
	if providerKinds[SecurityPublic] && workspaceHasCredentialKind(consumerKinds) {
		addWorkspaceCatalogMismatch(endpoint, consumer, "credentials_on_public_endpoint", "INFO", "Consumer credentials are sent to an explicitly public provider endpoint.", ConfidenceExact, evidenceIDs)
		return
	}
	if workspaceProviderRequiresAuthentication(providerKinds) && !hasCallAuthEvidence {
		addWorkspaceCatalogMismatch(endpoint, consumer, "missing_call_auth_evidence", "WARNING", "Call authentication evidence is absent; this is incomplete static evidence, not a proven authentication failure.", ConfidenceInferred, evidenceIDs)
		return
	}
	if workspaceProviderRequiresAuthentication(providerKinds) && hasCallAuthEvidence && !workspaceSecurityCompatible(providerKinds, consumerKinds) {
		addWorkspaceCatalogMismatch(endpoint, consumer, "auth_scheme_mismatch", "WARNING", "Provider authentication kinds "+strings.Join(sortedSecurityKinds(providerKinds), ", ")+" do not match consumer call authentication kinds "+strings.Join(sortedSecurityKinds(consumerKinds), ", ")+".", ConfidenceInferred, evidenceIDs)
	}
}

func addWorkspaceProviderConflictMismatch(endpoint *APIEndpointRecord) {
	for _, security := range endpoint.Security {
		if !security.Conflicting {
			continue
		}
		addWorkspaceCatalogMismatch(endpoint, APIConsumerRecord{}, "conflicting_provider_security", "WARNING", "Provider security evidence contains conflicting public and authenticated rules; effective runtime access cannot be proven statically.", ConfidenceInferred, workspaceCatalogEvidenceIDs(*endpoint))
		return
	}
}

func workspaceSecurityKinds(records []SecurityEvidenceRecord) map[string]bool {
	result := map[string]bool{}
	for _, record := range records {
		if record.Kind != SecurityUnknown {
			result[record.Kind] = true
		}
	}
	return result
}

func workspaceHasCredentialKind(kinds map[string]bool) bool {
	for kind := range kinds {
		if kind != SecurityPublic && kind != SecurityUnknown {
			return true
		}
	}
	return false
}

func workspaceProviderRequiresAuthentication(kinds map[string]bool) bool {
	for kind := range kinds {
		if kind != SecurityPublic && kind != SecurityUnknown {
			return true
		}
	}
	return false
}

func workspaceSecurityCompatible(provider, consumer map[string]bool) bool {
	if provider[SecurityAuthenticated] || provider[SecurityRole] {
		return workspaceHasCredentialKind(consumer)
	}
	for kind := range provider {
		if consumer[kind] || (kind == SecurityBearer && consumer[SecurityOAuth2]) || (kind == SecurityOAuth2 && consumer[SecurityBearer]) {
			return true
		}
	}
	return false
}

func sortedSecurityKinds(kinds map[string]bool) []string {
	result := make([]string, 0, len(kinds))
	for kind := range kinds {
		result = append(result, kind)
	}
	sort.Strings(result)
	return result
}

func addWorkspaceCatalogMismatch(endpoint *APIEndpointRecord, consumer APIConsumerRecord, kind, severity, reason string, confidence Confidence, evidenceIDs []string) {
	consumerIdentity := consumer.ID
	if consumerIdentity == "" {
		consumerIdentity = "provider"
	}
	record := APIMismatchRecord{
		ID: StableWorkspaceID("api-mismatch", endpoint.ID, consumerIdentity, kind), Kind: kind, Severity: severity,
		Reason: reason, Confidence: confidence, EvidenceIDs: catalogUniqueSortedStrings(evidenceIDs),
	}
	for _, existing := range endpoint.Mismatches {
		if existing.ID == record.ID {
			return
		}
	}
	endpoint.Mismatches = append(endpoint.Mismatches, record)
}

func workspaceCatalogEvidenceIDs(endpoint APIEndpointRecord, extra ...string) []string {
	ids := append([]string(nil), endpoint.EvidenceIDs...)
	ids = append(ids, extra...)
	if len(catalogUniqueSortedStrings(ids)) == 0 {
		ids = append(ids, endpoint.ID)
	}
	return catalogUniqueSortedStrings(ids)
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
