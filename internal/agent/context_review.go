package agent

import (
	"fmt"
	"strings"

	"github.com/gorecodecom/goregraph/internal/pathutil"
	"github.com/gorecodecom/goregraph/internal/scan"
)

// ContextReviewTopic describes the evidence delivered for a requested review topic.
type ContextReviewTopic struct {
	Topic    string `json:"topic"`
	Sections []int  `json:"sections,omitempty"`
	Gap      string `json:"gap,omitempty"`
}

func buildReviewContext(loaded loadedContextIndex, request ContextRequest) (ContextPack, error) {
	var topics []string
	seenTopics := map[string]bool{}
	for _, value := range strings.Split(request.Query, ";") {
		topic := strings.TrimSpace(value)
		if topic != "" && !seenTopics[topic] {
			topics = append(topics, topic)
			seenTopics[topic] = true
		}
	}
	if len(topics) == 0 || len(topics) > 8 {
		return ContextPack{}, fmt.Errorf("review requires one to eight topics separated by semicolons")
	}
	root, err := pathutil.Resolve(request.Root)
	if err != nil {
		return ContextPack{}, err
	}
	var facts []scan.AgentContextFactRecord
	for _, fact := range loaded.Index.Facts {
		if auditSourceWithinRoot(loaded, scan.AgentAuditSource{Project: fact.Project, File: fact.File}, root) {
			facts = append(facts, fact)
		}
	}
	loaded.Index.Facts = facts
	loaded.ReviewRoot = root
	pack := ContextPack{
		Schema: scan.SchemaVersion, Mode: "review", Query: request.Query,
		Freshness: loaded.Index.Generated, BudgetTokens: request.BudgetTokens,
		Confidence: "LOW", SourceCoverage: "none", FallbackRequired: true,
		FallbackReason: "review_evidence_partial",
		Uncertainties:  []ContextUncertainty{{Scope: "review", Reason: "Selected source candidates do not establish complete review coverage or runtime behavior."}},
	}
	if request.ProtocolVersion == AdaptiveV2 {
		pack.ProtocolVersion = AdaptiveV2
		pack = applyAdaptiveContextHealth(pack, loaded.Health)
	}
	for _, topic := range topics {
		pack.Review = append(pack.Review, ContextReviewTopic{Topic: topic, Gap: "No verified indexed candidate found."})
	}
	pack.ContextID = contextIdentityForProtocol(request.ProtocolVersion, pack.Freshness,
		[]string{"review", root, request.Query, fmt.Sprint(request.MaxFiles, ":", request.BudgetTokens)}, nil, nil)
	choices := make([][]ContextSourceSection, len(topics))
	for i, topic := range topics {
		if reviewHTTPTopic(topic) {
			choices[i] = reviewHTTPSections(loaded, topic)
			if len(choices[i]) > 0 {
				pack.Review[i].Gap = "Route evidence did not fit the shared review budget."
				continue
			}
		}
		part := request
		part.Query = topic
		part.PreviousContextID = ""
		part.MaxFiles = min(3, request.MaxFiles)
		candidates, err := attachAdaptiveFallbackEvidence(ContextPack{
			Query: topic, FallbackRequired: true, FallbackReason: ContextFallbackInsufficientRelevance,
		}, loaded, part)
		if err != nil {
			return ContextPack{}, err
		}
		for _, section := range candidates.SourceSections {
			if section.SourceState == "indexed_range_current" || section.SourceState == "relocated_current" {
				choices[i] = append(choices[i], section)
			}
		}
		if len(choices[i]) > 0 {
			pack.Review[i].Gap = "Candidate did not fit the shared review budget."
		}
	}
	files := map[string]bool{}
	for round := 0; round < 3; round++ {
		for i, sections := range choices {
			if round >= len(sections) {
				continue
			}
			section := sections[round]
			key := auditSourceKey(section.Project, section.Path)
			if !files[key] && len(files) >= request.MaxFiles {
				continue
			}
			next := pack
			next.Review = append([]ContextReviewTopic(nil), pack.Review...)
			topic := &next.Review[i]
			n := 0
			for j, existing := range pack.SourceSections {
				if existing.Project == section.Project && existing.Path == section.Path &&
					existing.StartLine <= section.EndLine && section.StartLine <= existing.EndLine {
					if existing.StartLine <= section.StartLine && existing.EndLine >= section.EndLine {
						n = j + 1
					} else {
						n = -1
					}
					break
				}
			}
			if n < 0 {
				continue
			}
			if n == 0 {
				next.SourceSections = append(append([]ContextSourceSection(nil), pack.SourceSections...), section)
				n = len(next.SourceSections)
			}
			topic.Sections = append(append([]int(nil), topic.Sections...), n)
			topic.Gap = "Candidates only; verify remaining behavior and coverage."
			if reviewHTTPTopic(topic.Topic) {
				topic.Gap = "Verify the remaining routes, middleware and authorization. These excerpts do not establish effective access controls."
			}
			next.SourceCoverage = "partial"
			fits, err := contextSourcePackFits(next, request)
			if err != nil {
				return ContextPack{}, err
			}
			if fits {
				pack = next
				files[key] = true
			}
		}
	}
	identities := []string{"review", root, request.Query, fmt.Sprint(request.MaxFiles, ":", request.BudgetTokens)}
	for _, section := range pack.SourceSections {
		identities = append(identities, section.Project, section.Path, section.ReadReceipt)
	}
	pack.ContextID = contextIdentityForProtocol(request.ProtocolVersion, pack.Freshness, identities, nil, nil)
	measured, err := finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, err
	}
	if measured.EstimatedTokens > request.BudgetTokens {
		return ContextPack{}, fmt.Errorf("review topics exceed context budget; shorten the topics or split the review")
	}
	return measured, nil
}
