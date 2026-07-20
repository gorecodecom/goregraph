package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	DefaultContextBudgetTokens         = 4000
	MinContextBudgetTokens             = 256
	MaxContextBudgetTokens             = 6000
	DefaultContextMetadataBudgetTokens = 1100
	DefaultContextMaxBytes             = 16000
	MaxContextBytes                    = 24000
	MinContextMaxFiles                 = 1
	DefaultContextMaxFiles             = 12
	MaxContextMaxFiles                 = 20
	MaxContextSourceSections           = 12
	MaxContextSourceOmissions          = 3
	MaxContextSourceFileBytes          = 2 * 1024 * 1024
	contextQueryJSONBudgetBytes        = 256
)

type ContextRequest struct {
	Root              string `json:"root,omitempty"`
	Query             string `json:"query"`
	BudgetTokens      int    `json:"budget_tokens,omitempty"`
	MaxFiles          int    `json:"max_files,omitempty"`
	PreviousContextID string `json:"previous_context_id,omitempty"`
}

type ContextLocation struct {
	ID          string   `json:"id"`
	Project     string   `json:"project,omitempty"`
	Kind        string   `json:"kind"`
	Label       string   `json:"label"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	EndLine     int      `json:"end_line,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type ContextRelationship struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Reason     string `json:"reason,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

type ContextConcern struct {
	Kind    string `json:"kind"`
	Project string `json:"project,omitempty"`
	Covered bool   `json:"covered"`
	Reason  string `json:"reason,omitempty"`
}

type ContextFile struct {
	Project    string `json:"project,omitempty"`
	Path       string `json:"path"`
	StartLine  int    `json:"start_line,omitempty"`
	EndLine    int    `json:"end_line,omitempty"`
	Role       string `json:"role"`
	Reason     string `json:"reason"`
	Confidence string `json:"confidence,omitempty"`
}

type ContextUncertainty struct {
	Scope  string `json:"scope"`
	Reason string `json:"reason"`
}

type ContextSourceOmission struct {
	Project string `json:"project,omitempty"`
	Path    string `json:"path"`
	Role    string `json:"role"`
	Reason  string `json:"reason"`
}

type ContextEndpointConsumer struct {
	Project        string `json:"project"`
	File           string `json:"file,omitempty"`
	Line           int    `json:"line,omitempty"`
	Authentication string `json:"authentication"`
	Confidence     string `json:"confidence,omitempty"`
}

type ContextEndpoint struct {
	Provider           string                    `json:"provider"`
	HTTPMethod         string                    `json:"http_method"`
	Path               string                    `json:"path"`
	Handler            string                    `json:"handler,omitempty"`
	File               string                    `json:"file,omitempty"`
	Line               int                       `json:"line,omitempty"`
	RequestType        string                    `json:"request_type,omitempty"`
	ResponseType       string                    `json:"response_type,omitempty"`
	Security           string                    `json:"security"`
	SecurityConfidence string                    `json:"security_confidence,omitempty"`
	Consumers          []ContextEndpointConsumer `json:"consumers,omitempty"`
	OmittedConsumers   int                       `json:"omitted_consumers,omitempty"`
	Limitations        []string                  `json:"limitations,omitempty"`
}

type ContextPack struct {
	Schema              int    `json:"schema"`
	Query               string `json:"query"`
	selectionQuery      string
	budgetQuery         string
	Freshness           string                  `json:"freshness,omitempty"`
	Confidence          string                  `json:"confidence"`
	FallbackRequired    bool                    `json:"fallback_required"`
	FallbackReason      string                  `json:"fallback_reason,omitempty"`
	Concerns            []ContextConcern        `json:"concerns,omitempty"`
	Entrypoints         []ContextLocation       `json:"entrypoints,omitempty"`
	Endpoints           []ContextEndpoint       `json:"endpoints,omitempty"`
	CallChain           []ContextRelationship   `json:"call_chain,omitempty"`
	Contracts           []ContextLocation       `json:"contracts,omitempty"`
	Persistence         []ContextLocation       `json:"persistence,omitempty"`
	Tests               []ContextLocation       `json:"tests,omitempty"`
	Files               []ContextFile           `json:"files,omitempty"`
	Uncertainties       []ContextUncertainty    `json:"uncertainties,omitempty"`
	SourceSections      []ContextSourceSection  `json:"source_sections,omitempty"`
	SourceOmissions     []ContextSourceOmission `json:"source_omissions,omitempty"`
	SourceCoverage      string                  `json:"source_coverage,omitempty"`
	SourceUnrepresented int                     `json:"source_unrepresented,omitempty"`
	EstimatedTokens     int                     `json:"estimated_tokens"`
	BudgetTokens        int                     `json:"budget_tokens"`
	ContextID           string                  `json:"context_id,omitempty"`
	DuplicateOf         string                  `json:"duplicate_of,omitempty"`
	RetryAllowed        bool                    `json:"retry_allowed"`
	RetryAnchors        []string                `json:"retry_anchors,omitempty"`

	selectedSourceFactIDs []string
	selectedFactIDs       []string
	selectedEdgeIDs       []string
	selectedConcernKeys   []string
}

func BuildContext(request ContextRequest) (ContextPack, error) {
	request, err := normalizeContextRequest(request)
	if err != nil {
		return ContextPack{}, err
	}
	loaded, err := loadContextIndex(request)
	if err != nil {
		return ContextPack{}, err
	}
	metadataRequest := request
	metadataBudget := contextMetadataBudget(request.BudgetTokens)
	publicConcerns := []ContextConcern(nil)
	if seed, ok := contextConcernPlanningSeed(loaded.Index, request.Query); ok {
		publicConcerns = publicContextConcerns(planContextConcerns(request.Query, loaded.Index, seed))
		metadataBudget = contextMetadataBudgetForConcerns(request.BudgetTokens, publicConcerns)
		metadataRequest.BudgetTokens = metadataBudget
		var concernTokens int
		var measureErr error
		publicConcerns, concernTokens, measureErr = contextConcernsWithinMetadataBudget(publicConcerns, metadataBudget)
		if measureErr != nil {
			return ContextPack{}, measureErr
		}
		metadataRequest.BudgetTokens -= concernTokens
	} else {
		metadataRequest.BudgetTokens = metadataBudget
	}
	pack, err := compileContextPack(loaded.Index, metadataRequest)
	if err != nil {
		return ContextPack{}, err
	}
	if len(publicConcerns) > 0 && !pack.FallbackRequired {
		pack.Concerns = publicConcerns
		pack, err = finalizeContextEstimate(pack)
		if err != nil {
			return ContextPack{}, err
		}
		fits, fitErr := contextPackFitsBudget(pack, metadataBudget)
		if fitErr != nil {
			return ContextPack{}, fitErr
		}
		if !fits {
			return ContextPack{}, fmt.Errorf("required context concerns exceed metadata budget")
		}
	}
	pack.BudgetTokens = request.BudgetTokens
	pack.ContextID = contextIdentity(
		pack.Freshness,
		pack.selectedFactIDs,
		pack.selectedEdgeIDs,
		pack.selectedConcernKeys,
	)
	if request.PreviousContextID != "" && request.PreviousContextID == pack.ContextID {
		return duplicateContextPack(pack)
	}
	if pack.FallbackRequired || pack.Confidence == "LOW" {
		pack.SourceCoverage = "none"
		return finalizeContextPackWithinBudget(pack, request)
	}
	metadataPack := pack
	pack, err = attachContextSource(pack, loaded, request)
	if err != nil && request.BudgetTokens == MinContextBudgetTokens {
		compact := metadataPack
		compact.Files = nil
		pack, err = attachContextSource(compact, loaded, request)
	}
	if err != nil {
		return ContextPack{}, err
	}
	pack.RetryAllowed, pack.RetryAnchors = contextRetryPermission(pack, loaded.Index)
	return finalizeContextPackWithinBudget(pack, request)
}

func contextIdentity(freshness string, factIDs, edgeIDs, concernKeys []string) string {
	identity := struct {
		Freshness   string   `json:"freshness"`
		FactIDs     []string `json:"fact_ids"`
		EdgeIDs     []string `json:"edge_ids"`
		ConcernKeys []string `json:"concern_keys"`
	}{
		Freshness:   strings.TrimSpace(freshness),
		FactIDs:     orderedContextIdentityValues(factIDs),
		EdgeIDs:     orderedContextIdentityValues(edgeIDs),
		ConcernKeys: orderedContextIdentityValues(concernKeys),
	}
	body, _ := json.Marshal(identity)
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:12])
}

func orderedContextIdentityValues(values []string) []string {
	ordered := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			ordered = append(ordered, value)
		}
	}
	sort.Strings(ordered)
	result := ordered[:0]
	for _, value := range ordered {
		if len(result) > 0 && result[len(result)-1] == value {
			continue
		}
		result = append(result, value)
	}
	return result
}

func duplicateContextPack(pack ContextPack) (ContextPack, error) {
	duplicate := ContextPack{
		Schema:       pack.Schema,
		Freshness:    pack.Freshness,
		Confidence:   pack.Confidence,
		ContextID:    pack.ContextID,
		DuplicateOf:  pack.ContextID,
		BudgetTokens: pack.BudgetTokens,
	}
	return finalizeContextEstimate(duplicate)
}

func contextMetadataBudget(total int) int {
	if total < DefaultContextMetadataBudgetTokens {
		return total
	}
	return DefaultContextMetadataBudgetTokens
}

func contextMetadataBudgetForConcerns(total int, concerns []ContextConcern) int {
	base := contextMetadataBudget(total)
	projects := map[string]bool{}
	for _, concern := range concerns {
		if concern.Kind == contextConcernProject {
			if project := normalizeContextProject(concern.Project); project != "" {
				projects[project] = true
			}
		}
	}
	if len(projects) <= 1 || total <= base {
		return base
	}
	expanded := base + (len(projects)-1)*300
	maximum := total / 2
	if maximum < base {
		return base
	}
	if expanded > maximum {
		return maximum
	}
	return expanded
}

func contextConcernMetadataTokens(concerns []ContextConcern) (int, error) {
	withoutConcerns, err := json.Marshal(ContextPack{})
	if err != nil {
		return 0, err
	}
	withConcerns, err := json.Marshal(ContextPack{
		Concerns:       concerns,
		SourceCoverage: "complete",
	})
	if err != nil {
		return 0, err
	}
	extraRunes := utf8.RuneCount(withConcerns) - utf8.RuneCount(withoutConcerns)
	return (extraRunes+3)/4 + 1, nil
}

func contextConcernsWithinMetadataBudget(
	concerns []ContextConcern,
	metadataBudget int,
) ([]ContextConcern, int, error) {
	selected := corePublicContextConcerns(concerns)
	selectedTokens, err := contextConcernMetadataTokens(selected)
	if err != nil {
		return nil, 0, err
	}
	minimumCompilationBudget := MinContextBudgetTokens - selectedTokens
	for _, concern := range concerns {
		if concern.Kind == contextConcernEntrypoint || concern.Kind == contextConcernPrimaryPath {
			continue
		}
		candidate := append(append([]ContextConcern(nil), selected...), concern)
		candidateTokens, measureErr := contextConcernMetadataTokens(candidate)
		if measureErr != nil {
			return nil, 0, measureErr
		}
		if metadataBudget-candidateTokens < minimumCompilationBudget {
			continue
		}
		selected = candidate
		selectedTokens = candidateTokens
	}
	return selected, selectedTokens, nil
}

func contextByteBudget(tokens int) int {
	bytes := tokens * 4
	if bytes > MaxContextBytes {
		return MaxContextBytes
	}
	return bytes
}

func normalizeContextRequest(request ContextRequest) (ContextRequest, error) {
	if strings.TrimSpace(request.Root) == "" {
		request.Root = "."
	}
	request.Query = strings.TrimSpace(request.Query)
	if request.Query == "" {
		return ContextRequest{}, fmt.Errorf("context query is required")
	}
	if request.BudgetTokens == 0 {
		request.BudgetTokens = DefaultContextBudgetTokens
	}
	if request.MaxFiles == 0 {
		request.MaxFiles = DefaultContextMaxFiles
	}
	if request.BudgetTokens < MinContextBudgetTokens || request.BudgetTokens > MaxContextBudgetTokens {
		return ContextRequest{}, fmt.Errorf(
			"budget-tokens must be between %d and %d",
			MinContextBudgetTokens,
			MaxContextBudgetTokens,
		)
	}
	if request.MaxFiles < MinContextMaxFiles || request.MaxFiles > MaxContextMaxFiles {
		return ContextRequest{}, fmt.Errorf(
			"max-files must be between %d and %d",
			MinContextMaxFiles,
			MaxContextMaxFiles,
		)
	}
	return request, nil
}

func EstimateContextTokens(value any) (int, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	runes := utf8.RuneCount(body)
	return (runes + 3) / 4, nil
}

func finalizeContextEstimate(pack ContextPack) (ContextPack, error) {
	for range 12 {
		estimate, err := EstimateContextTokens(pack)
		if err != nil {
			return ContextPack{}, err
		}
		if estimate == pack.EstimatedTokens {
			return pack, nil
		}
		pack.EstimatedTokens = estimate
	}
	return ContextPack{}, fmt.Errorf("context token estimate did not converge")
}

func newContextEnvelope(index scan.AgentContextIndexRecord, request ContextRequest) (ContextPack, error) {
	freshness := strings.TrimSpace(index.Generated)
	if freshness == "" {
		freshness = "unknown"
	}
	publicQuery, compacted := compactContextQuery(request.Query)
	budgetQuery := publicQuery
	if compacted {
		budgetQuery = contextQueryBudgetPlaceholder()
	}
	pack, err := finalizeContextEstimate(ContextPack{
		Schema:         scan.SchemaVersion,
		Query:          publicQuery,
		selectionQuery: request.Query,
		budgetQuery:    budgetQuery,
		Freshness:      freshness,
		Confidence:     "LOW",
		BudgetTokens:   request.BudgetTokens,
	})
	if err != nil {
		return ContextPack{}, err
	}
	fits, err := contextPackFitsBudget(pack, request.BudgetTokens)
	if err != nil {
		return ContextPack{}, err
	}
	if !fits {
		return ContextPack{}, fmt.Errorf(
			"context envelope exceeds budget %d",
			request.BudgetTokens,
		)
	}
	return pack, nil
}

func compactContextQuery(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if contextQueryFitsPublicBudget(value) {
		return value, false
	}
	if primary := contextPrimaryQuery(value); primary != "" {
		value = strings.TrimSpace(primary)
	}
	if contextQueryFitsPublicBudget(value) {
		return value, true
	}
	runes := []rune(value)
	maximumPrefix := len(runes)
	if maximumPrefix >= maximumContextQueryAnchorRunes {
		maximumPrefix = maximumContextQueryAnchorRunes - 1
	}
	for length := maximumPrefix; length >= 0; length-- {
		candidate := string(runes[:length]) + "…"
		if contextQueryFitsPublicBudget(candidate) {
			return candidate, true
		}
	}
	return "…", true
}

func contextQueryFitsPublicBudget(value string) bool {
	if len([]rune(value)) > maximumContextQueryAnchorRunes {
		return false
	}
	body, err := json.Marshal(value)
	return err == nil && len(body) <= contextQueryJSONBudgetBytes
}

// contextQueryJSONBudgetBytes includes JSON quotes and escaping. Reserving that
// payload keeps the mandatory Context Pack envelope safe at the 1024-byte minimum.
func contextQueryBudgetPlaceholder() string {
	return strings.Repeat("x", contextQueryJSONBudgetBytes-2)
}

func contextSelectionQuery(pack ContextPack) string {
	if strings.TrimSpace(pack.selectionQuery) != "" {
		return pack.selectionQuery
	}
	return pack.Query
}

func contextBudgetView(pack ContextPack) (ContextPack, error) {
	view := pack
	if pack.budgetQuery != "" {
		view.Query = pack.budgetQuery
	}
	return finalizeContextEstimate(view)
}

func fallbackContextPack(
	index scan.AgentContextIndexRecord,
	request ContextRequest,
	reason string,
	uncertainties []ContextUncertainty,
) (ContextPack, error) {
	pack, err := newContextEnvelope(index, request)
	if err != nil {
		return ContextPack{}, err
	}
	pack.FallbackRequired = true
	pack.FallbackReason = strings.TrimSpace(reason)
	pack.SourceCoverage = "none"
	pack, err = finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, err
	}
	fallbackRequest := request
	if fallbackRequest.BudgetTokens > MinContextBudgetTokens {
		fallbackRequest.BudgetTokens = MinContextBudgetTokens
	}
	fits, err := contextSourcePackFits(pack, fallbackRequest)
	if err != nil {
		return ContextPack{}, err
	}
	if !fits {
		return ContextPack{}, fmt.Errorf("context fallback exceeds the minimum token or byte envelope")
	}
	if len(uncertainties) > 0 {
		candidate := cloneContextPack(pack)
		candidate.Uncertainties = append(candidate.Uncertainties, uncertainties[0])
		candidate, err = finalizeContextEstimate(candidate)
		if err != nil {
			return ContextPack{}, err
		}
		fits, err = contextSourcePackFits(candidate, fallbackRequest)
		if err != nil {
			return ContextPack{}, err
		}
		if fits {
			pack = candidate
		}
	}
	return pack, nil
}
