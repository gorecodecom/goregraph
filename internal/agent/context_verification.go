package agent

import (
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const maximumContextVerificationRequests = 3

const adaptivePartialAnalysisReason = "analysis coverage is partial; selected evidence may omit unsupported or budget-limited files"

func adaptiveContextHealthReserve(health scan.ProjectionHealth) (int, error) {
	base := ContextPack{ProtocolVersion: AdaptiveV2}
	before, err := json.Marshal(base)
	if err != nil {
		return 0, err
	}
	after, err := json.Marshal(applyAdaptiveContextHealth(base, health))
	if err != nil {
		return 0, err
	}
	return (len(after)-len(before)+3)/4 + 1, nil
}

const (
	ContextFallbackIndexMissing          = "index_missing"
	ContextFallbackIndexStale            = "index_stale"
	ContextFallbackAmbiguousEntrypoint   = "ambiguous_entrypoint"
	ContextFallbackUnsupportedAnalysis   = "unsupported_analysis"
	ContextFallbackBudgetExhausted       = "budget_exhausted"
	ContextFallbackSourceUnreadable      = "source_unreadable"
	ContextFallbackEvidenceConflict      = "evidence_conflict"
	ContextFallbackInsufficientRelevance = "insufficient_relevance"
	ContextFallbackInsufficientEvidence  = "insufficient_evidence"
)

// ContextVerificationRequest identifies one exact source range that may resolve
// a gap already selected by the context compiler.
type ContextVerificationRequest struct {
	Project   string `json:"project,omitempty"`
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Reason    string `json:"reason"`
}

func adaptiveContextMetadata(pack ContextPack) ContextPack {
	if pack.ProtocolVersion != AdaptiveV2 {
		pack.ProtocolVersion = ""
		pack.Generation = ""
		pack.Health = nil
		pack.VerificationRequests = nil
		return pack
	}
	if pack.FallbackRequired {
		if pack.FallbackReason == "" && len(pack.Concerns) > 0 {
			pack.FallbackReason = ContextFallbackInsufficientEvidence
		}
		pack.FallbackReason = contextFallbackReasonCode(pack)
	} else if pack.SourceCoverage == "partial" {
		for _, concern := range pack.Concerns {
			if !concern.Covered {
				pack.FallbackRequired = true
				pack.FallbackReason = ContextFallbackInsufficientEvidence
				break
			}
		}
	}
	pack.VerificationRequests = contextVerificationRequests(pack)
	return pack
}

func applyAdaptiveContextHealth(pack ContextPack, health scan.ProjectionHealth) ContextPack {
	if pack.ProtocolVersion != AdaptiveV2 {
		pack.Health = nil
		pack.Generation = ""
		return pack
	}
	health.Reasons = append([]string(nil), health.Reasons...)
	pack.Health = &health
	pack.Generation = health.GenerationID
	if health.Coverage == "partial" &&
		!contextUncertaintyExists(pack.Uncertainties, "analysis", adaptivePartialAnalysisReason) &&
		len(pack.Uncertainties) < maximumContextUncertainty {
		pack.Uncertainties = append(pack.Uncertainties, ContextUncertainty{
			Scope: "analysis", Reason: adaptivePartialAnalysisReason,
		})
	}
	return pack
}

func adaptiveHealthFallbackReason(pack ContextPack) string {
	if pack.ProtocolVersion != AdaptiveV2 || pack.Health == nil {
		return ""
	}
	switch {
	case pack.Health.Integrity == "invalid", pack.Health.Freshness == "stale":
		return ContextFallbackIndexStale
	case pack.Health.Coverage == "unsupported":
		return ContextFallbackUnsupportedAnalysis
	default:
		return ""
	}
}

func adaptiveContextFailurePack(request ContextRequest, reason string) (ContextPack, error) {
	pack, err := newContextEnvelope(scanEmptyContextIndex(), request)
	if err != nil {
		return ContextPack{}, err
	}
	pack.FallbackRequired = true
	pack.FallbackReason = reason
	pack.SourceCoverage = "none"
	pack = applyAdaptiveContextHealth(pack, scan.HealthForProjection(scan.OutputManifest{}, "agent", false))
	pack = adaptiveContextMetadata(pack)
	return finalizeContextPackWithinBudget(pack, request)
}

func adaptiveHealthFallbackPack(
	index scan.AgentContextIndexRecord,
	request ContextRequest,
	contextID string,
	health scan.ProjectionHealth,
	reason string,
) (ContextPack, error) {
	pack, err := fallbackContextPack(index, request, reason, nil)
	if err != nil {
		return ContextPack{}, err
	}
	pack.ContextID = contextID
	pack = applyAdaptiveContextHealth(pack, health)
	pack = adaptiveContextMetadata(pack)
	return finalizeContextPackWithinBudget(pack, request)
}

func scanEmptyContextIndex() scan.AgentContextIndexRecord {
	return scan.AgentContextIndexRecord{SchemaVersion: scan.SchemaVersion}
}

func contextFallbackReasonCode(pack ContextPack) string {
	reason := strings.ToLower(strings.TrimSpace(pack.FallbackReason))
	for _, code := range []string{
		ContextFallbackIndexMissing,
		ContextFallbackIndexStale,
		ContextFallbackAmbiguousEntrypoint,
		ContextFallbackUnsupportedAnalysis,
		ContextFallbackBudgetExhausted,
		ContextFallbackSourceUnreadable,
		ContextFallbackEvidenceConflict,
		ContextFallbackInsufficientRelevance,
		ContextFallbackInsufficientEvidence,
	} {
		if reason == code {
			return code
		}
	}
	for _, omission := range pack.SourceOmissions {
		omissionReason := strings.ToLower(omission.Reason)
		if strings.Contains(omissionReason, "unreadable") ||
			strings.Contains(omissionReason, "not utf-8") ||
			strings.Contains(omissionReason, "not a regular file") {
			return ContextFallbackSourceUnreadable
		}
		if strings.Contains(omissionReason, "ambiguous") ||
			strings.Contains(omissionReason, "conflict") ||
			strings.Contains(omissionReason, "contradict") {
			return ContextFallbackEvidenceConflict
		}
		if strings.Contains(omissionReason, "absent from current source") {
			return ContextFallbackIndexStale
		}
	}
	switch {
	case strings.Contains(reason, "no sufficiently relevant") || strings.Contains(reason, "confidence is low"):
		return ContextFallbackInsufficientRelevance
	case strings.Contains(reason, "budget") || strings.Contains(reason, "exceeds"):
		return ContextFallbackBudgetExhausted
	case strings.Contains(reason, "ambiguous") || strings.Contains(reason, "not exactly one") ||
		strings.Contains(reason, "no sufficiently relevant") || strings.Contains(reason, "do not align"):
		return ContextFallbackAmbiguousEntrypoint
	case strings.Contains(reason, "contradict") || strings.Contains(reason, "conflict"):
		return ContextFallbackEvidenceConflict
	case strings.Contains(reason, "stale") || strings.Contains(reason, "absent from current source"):
		return ContextFallbackIndexStale
	case strings.Contains(reason, "unreadable"):
		return ContextFallbackSourceUnreadable
	default:
		return ContextFallbackUnsupportedAnalysis
	}
}

func contextVerificationRequests(pack ContextPack) []ContextVerificationRequest {
	type rankedRequest struct {
		request  ContextVerificationRequest
		priority int
	}
	ranked := make([]rankedRequest, 0, len(pack.SourceOmissions))
	for _, omission := range pack.SourceOmissions {
		rawPath := strings.TrimSpace(omission.Path)
		requestPath := path.Clean(strings.ReplaceAll(rawPath, `\`, "/"))
		if isPortableAbsolutePath(rawPath) || !safeContextVerificationPath(requestPath) ||
			omission.StartLine <= 0 || omission.EndLine < omission.StartLine ||
			!verifiableContextOmissionReason(omission.Reason) {
			continue
		}
		request := ContextVerificationRequest{
			Project: strings.TrimSpace(omission.Project),
			Path:    requestPath, StartLine: omission.StartLine, EndLine: omission.EndLine,
			Reason: strings.TrimSpace(omission.Reason),
		}
		for _, unseen := range contextUnseenVerificationRanges(pack, request) {
			ranked = append(ranked, rankedRequest{request: unseen, priority: contextVerificationPriority(pack, omission)})
		}
	}
	sort.SliceStable(ranked, func(left, right int) bool {
		if ranked[left].priority != ranked[right].priority {
			return ranked[left].priority < ranked[right].priority
		}
		if pack.ProtocolVersion == AdaptiveV2 {
			return false
		}
		leftRequest, rightRequest := ranked[left].request, ranked[right].request
		if leftRequest.Project != rightRequest.Project {
			return leftRequest.Project < rightRequest.Project
		}
		if leftRequest.Path != rightRequest.Path {
			return leftRequest.Path < rightRequest.Path
		}
		if leftRequest.StartLine != rightRequest.StartLine {
			return leftRequest.StartLine < rightRequest.StartLine
		}
		return leftRequest.EndLine < rightRequest.EndLine
	})

	requests := make([]ContextVerificationRequest, 0, min(len(ranked), maximumContextVerificationRequests))
	seen := map[string]bool{}
	for _, candidate := range ranked {
		request := candidate.request
		key := strings.Join([]string{
			normalizeContextProject(request.Project),
			contextPackSourceFile(request.Path),
			contextIntString(request.StartLine),
			contextIntString(request.EndLine),
		}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		requests = append(requests, request)
		if len(requests) == maximumContextVerificationRequests {
			break
		}
	}
	return requests
}

func contextUnseenVerificationRanges(pack ContextPack, request ContextVerificationRequest) []ContextVerificationRequest {
	remaining := []ContextVerificationRequest{request}
	for _, section := range pack.SourceSections {
		if normalizeContextProject(section.Project) != normalizeContextProject(request.Project) ||
			contextPackSourceFile(section.Path) != contextPackSourceFile(request.Path) ||
			section.StartLine <= 0 || section.EndLine < section.StartLine {
			continue
		}
		next := make([]ContextVerificationRequest, 0, len(remaining)+1)
		for _, current := range remaining {
			if section.EndLine < current.StartLine || section.StartLine > current.EndLine {
				next = append(next, current)
				continue
			}
			if current.StartLine < section.StartLine {
				left := current
				left.EndLine = section.StartLine - 1
				next = append(next, left)
			}
			if current.EndLine > section.EndLine {
				right := current
				right.StartLine = section.EndLine + 1
				next = append(next, right)
			}
		}
		remaining = next
	}
	return remaining
}

func safeContextVerificationPath(path string) bool {
	if path == "" || path == "." || isPortableAbsolutePath(path) {
		return false
	}
	return path != ".." && !strings.HasPrefix(path, "../")
}

func verifiableContextOmissionReason(reason string) bool {
	reason = strings.ToLower(strings.TrimSpace(reason))
	for _, excluded := range []string{
		"source file is missing",
		"source file is unreadable",
		"source path escapes project root",
		"source file is not a regular file",
		"source file exceeds 2097152 bytes",
		"source file is not utf-8 text",
	} {
		if reason == excluded {
			return false
		}
	}
	return true
}

func contextVerificationPriority(pack ContextPack, omission ContextSourceOmission) int {
	project := normalizeContextProject(omission.Project)
	role := strings.ToLower(omission.Role)
	for _, concern := range pack.Concerns {
		if concern.Covered {
			continue
		}
		concernProject := normalizeContextProject(concern.Project)
		if concernProject != "" && concernProject != project {
			continue
		}
		switch normalizedContextConcernKind(concern.Kind) {
		case contextConcernEntrypoint:
			if strings.Contains(role, "entrypoint") {
				return 0
			}
		case contextConcernPrimaryPath, contextConcernSideEffects,
			contextConcernAuth, contextConcernConfiguration, contextConcernResilience:
			if strings.Contains(role, "call_chain") {
				return 0
			}
		case contextConcernTests, contextConcernPersistence, contextConcernHTTPContract:
			if role == contextSourceConcernRole(normalizedContextConcernKind(concern.Kind)) {
				return 0
			}
		}
	}
	return 1
}

func contextIntString(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}
