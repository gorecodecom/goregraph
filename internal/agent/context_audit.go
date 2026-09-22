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

// ContextAudit reports source coverage independently of production entrypoints.
type ContextAudit struct {
	Links          []ContextRelationship `json:"links,omitempty"`
	Scope          string                `json:"scope"`
	QueryHash      string                `json:"query_hash"`
	QueryTruncated bool                  `json:"query_truncated,omitempty"`
	Execution      string                `json:"execution"`
	Areas          []ContextAuditArea    `json:"areas,omitempty"`
	Unknown        []string              `json:"unknown,omitempty"`
}

// ContextAuditArea counts delivered files within the selected indexed scope.
type ContextAuditArea struct {
	Name      string `json:"name"`
	Selected  int    `json:"selected"`
	Delivered int    `json:"delivered"`
	Coverage  string `json:"coverage"`
}

func auditSourceKey(project, file string) string { return project + "\x00" + file }

func auditArea(source scan.AgentAuditSource) string {
	p := strings.ToLower(source.File)
	switch {
	case source.Kind == "ci":
		return "ci"
	case source.Kind == "documentation":
		return "documentation"
	case source.Kind == "configuration":
		return "configuration"
	case strings.Contains(p, "playwright") || strings.Contains(p, "/visual") || strings.HasPrefix(p, "tests/storybook/"):
		return "visual_tests"
	case source.Kind == "package" || strings.Contains(p, "vitest") || strings.Contains(p, "setup") || strings.HasPrefix(p, "scripts/"):
		return "runner"
	default:
		return "stories_and_dependencies"
	}
}

func selectAuditSources(loaded loadedContextIndex, query string, files map[string]sourceFile, reads *int) ([]scan.AgentAuditSource, []string) {
	index := loaded.Index
	tokens := contextTokenSet(query)
	topics := []string{}
	for _, topic := range []string{"storybook", "playwright", "vitest"} {
		if tokens[topic] {
			topics = append(topics, topic)
		}
	}
	ciRequested := tokens["ci"] || tokens["gitlab"] || tokens["pipeline"] || tokens["pipelines"] || tokens["deployment"] || tokens["deployments"]
	byKey := map[string]scan.AgentAuditSource{}
	var queue []string
	selected := map[string]bool{}
	for _, source := range index.AuditSources {
		key := auditSourceKey(source.Project, source.File)
		byKey[key] = source
		seed := false
		if source.Kind == "ci" {
			seed = ciRequested && (source.File == ".gitlab-ci.yml" || strings.HasPrefix(source.File, ".github/workflows/"))
		} else {
			for _, have := range source.Topics {
				for _, want := range topics {
					if have == want {
						seed = true
					}
				}
			}
		}
		if seed {
			selected[key] = true
			queue = append(queue, key)
		}
	}
	sort.Strings(queue)
	queue = auditRoundRobinKeys(queue, byKey)
	unknown := []string{}
	if len(topics) == 0 && !ciRequested {
		unknown = append(unknown, "audit scope not recognized; name Storybook, Playwright, Vitest or CI")
	}
	for cursor := 0; cursor < len(queue) && cursor < 2048; cursor++ {
		source := byKey[queue[cursor]]
		if len(source.References) > 0 {
			if *reads >= 44 {
				unknown = appendAuditUnknown(unknown, "source verification limit reached; additional links remain unknown")
				continue
			}
			file, err := readAuditSource(loaded, source, files, reads)
			if err != nil || file.Hash != source.Hash {
				unknown = appendAuditUnknown(unknown, source.File+": references were not followed because the source is unreadable or changed")
				continue
			}
		}
		for _, ref := range source.References {
			key := auditSourceKey(source.Project, ref.File)
			if _, found := byKey[key]; found && !selected[key] {
				selected[key] = true
				queue = append(queue, key)
			}
		}
	}
	if len(queue) > 2048 {
		queue = queue[:2048]
		unknown = append(unknown, "audit traversal limit reached; additional dependencies remain unknown")
	}
	result := make([]scan.AgentAuditSource, 0, len(queue))
	for _, key := range queue {
		result = append(result, byKey[key])
	}
	sort.Slice(result, func(i, j int) bool {
		l, r := result[i], result[j]
		la, ra := auditArea(l), auditArea(r)
		if la != ra {
			return la < ra
		}
		if l.Project != r.Project {
			return l.Project < r.Project
		}
		return l.File < r.File
	})
	keys := make([]string, 0, len(result))
	for _, source := range result {
		keys = append(keys, auditSourceKey(source.Project, source.File))
	}
	keys = auditRoundRobinKeys(keys, byKey)
	result = nil
	for _, key := range keys {
		result = append(result, byKey[key])
	}
	return result, unknown
}

func buildAuditContext(loaded loadedContextIndex, request ContextRequest) (ContextPack, error) {
	query := request.Query
	if len([]rune(query)) > 1600 {
		query = string([]rune(query)[:1600]) + "…"
	}
	sum := sha256.Sum256([]byte(request.Query))
	pack := ContextPack{Schema: scan.SchemaVersion, Mode: "audit", Query: query, Freshness: loaded.Index.Generated, Confidence: "MEDIUM", SourceCoverage: "partial", BudgetTokens: request.BudgetTokens,
		Audit: &ContextAudit{Scope: "selected indexed tooling sources; not repository-wide or runtime coverage", QueryHash: hex.EncodeToString(sum[:]), QueryTruncated: query != request.Query, Execution: "unknown"}}
	if request.ProtocolVersion == AdaptiveV2 {
		pack.ProtocolVersion = AdaptiveV2
	}
	pack.Audit.Unknown = []string{"Effective activation, successful execution, approved visual references and repository-wide absence are not established by source files.", "Only indexed literal file references are followed; module aliases, computed paths and external project triggers require verification."}
	if loaded.Index.AuditVersion != 1 {
		pack.FallbackRequired = true
		pack.Confidence = "LOW"
		pack.SourceCoverage = "none"
		pack.FallbackReason = "audit_index_unavailable"
		pack.Audit.Unknown = append(pack.Audit.Unknown, "This index lacks audit metadata; update the agent index with this version before retrying.")
		return finalizeAuditPack(pack, request, nil)
	}
	files := map[string]sourceFile{}
	reads := 0
	sources, unknown := selectAuditSources(loaded, request.Query, files, &reads)
	pack.Audit.Unknown = append(pack.Audit.Unknown, unknown...)
	if loaded.Index.AuditIncomplete {
		pack.Audit.Unknown = append(pack.Audit.Unknown, "Some workspace projects lack current audit metadata; cross-project coverage is incomplete.")
	}
	areas := map[string]*ContextAuditArea{}
	for _, source := range sources {
		area := auditArea(source)
		if areas[area] == nil {
			areas[area] = &ContextAuditArea{Name: area, Coverage: "none"}
		}
		areas[area].Selected++
	}
	refreshAreas := func() {
		pack.Audit.Areas = nil
		for _, area := range areas {
			area.Coverage = "none"
			if area.Delivered > 0 {
				area.Coverage = "partial"
			}
			if area.Delivered == area.Selected {
				area.Coverage = "complete"
			}
			pack.Audit.Areas = append(pack.Audit.Areas, *area)
		}
		sort.Slice(pack.Audit.Areas, func(i, j int) bool { return pack.Audit.Areas[i].Name < pack.Audit.Areas[j].Name })
	}
	refreshAreas()
	identities := []string{request.Query, "audit"}
	for _, source := range sources {
		identity, _ := json.Marshal(source)
		identities = append(identities, string(identity))
		reason := "source exceeds the audit file or token budget"
		omissionEnd := max(1, source.Lines)
		file, err := readAuditSource(loaded, source, files, &reads)
		if err != nil {
			reason = "indexed audit source is unavailable or unreadable"
		} else {
			identities = append(identities, source.Project+"/"+source.File+":"+file.Hash)
			end := len(file.Lines)
			if end > 1 && file.Lines[end-1] == "" {
				end--
			}
			if end < 1 {
				end = 1
			}
			omissionEnd = end
			section := ContextSourceSection{Project: source.Project, Path: source.File, StartLine: 1, EndLine: end, Role: auditArea(source), RenderMode: "declaration_body", SourceState: "indexed_range_current", Content: renderNumberedSource(file.Lines, 1, end)}
			attachSourceReadReceipt(&section, file)
			if file.Hash != source.Hash {
				section.SourceState = "current_source_changed_since_index"
				reason = "source changed since indexing; indexed relationships may be stale"
			}
			if len(pack.SourceSections) < request.MaxFiles {
				pack.SourceSections = append(pack.SourceSections, section)
				measured, measureErr := finalizeContextEstimate(pack)
				if measureErr != nil {
					return ContextPack{}, measureErr
				}
				// Reserve space for area counters, identity and bounded omission metadata.
				if measured.EstimatedTokens <= request.BudgetTokens-160 {
					areas[auditArea(source)].Delivered++
					reason = ""
				} else {
					pack.SourceSections = pack.SourceSections[:len(pack.SourceSections)-1]
				}
			}
			if file.Hash != source.Hash {
				pack.Audit.Unknown = appendAuditUnknown(pack.Audit.Unknown, source.File+": changed since index; relationships require verification")
			}
		}
		for _, issue := range source.Unknown {
			if len(pack.Audit.Unknown) < 16 {
				pack.Audit.Unknown = append(pack.Audit.Unknown, source.File+": "+issue)
			}
		}
		if reason != "" {
			omission := ContextSourceOmission{Project: source.Project, Path: source.File, StartLine: 1, EndLine: omissionEnd, Role: auditArea(source), Reason: reason}
			pack.SourceOmissions = append(pack.SourceOmissions, omission)
		}
	}
	refreshAreas()
	pack.Audit.Links = auditDeliveredLinks(sources, pack.SourceSections)
	if len(pack.SourceSections) == 0 {
		pack.FallbackRequired = true
		pack.Confidence = "LOW"
		pack.SourceCoverage = "none"
		pack.FallbackReason = "audit_sources_unavailable"
	}
	return finalizeAuditPack(pack, request, identities)
}

func finalizeAuditPack(pack ContextPack, request ContextRequest, identities []string) (ContextPack, error) {
	identities = append(identities, "audit", request.Query, fmt.Sprint(request.BudgetTokens, ":", request.MaxFiles))
	pack.ContextID = contextIdentityForProtocol(request.ProtocolVersion, pack.Freshness, identities, nil, nil)
	if request.PreviousContextID != "" && request.PreviousContextID == pack.ContextID {
		pack.DuplicateOf = pack.ContextID
		pack.SourceSections = nil
		pack.SourceOmissions = nil
		pack.Audit.Links = nil
	}
	for {
		if len(pack.SourceSections) == 0 && pack.DuplicateOf == "" {
			pack.SourceCoverage = "none"
			pack.FallbackRequired = true
			pack.Confidence = "LOW"
			if pack.FallbackReason == "" {
				pack.FallbackReason = "audit_budget_exhausted"
			}
		}
		measured, err := finalizeContextEstimate(pack)
		if err != nil {
			return ContextPack{}, err
		}
		if measured.EstimatedTokens <= request.BudgetTokens {
			pack = measured
			break
		}
		switch {
		case len(pack.SourceSections) == 0 && utf8.RuneCountInString(pack.Query) > 32:
			pack.Query = string([]rune(pack.Query)[:32])
			pack.Audit.QueryTruncated = true
		case len(pack.SourceSections) == 0 && len(pack.Audit.Areas) > 0:
			pack.Audit.Areas = nil
		case len(pack.SourceSections) == 0 && len(pack.Audit.Unknown) > 0:
			pack.Audit.Unknown = pack.Audit.Unknown[:len(pack.Audit.Unknown)-1]
		case len(pack.Audit.Links) > 0:
			pack.Audit.Links = pack.Audit.Links[:len(pack.Audit.Links)-1]
		case len(pack.SourceOmissions) > 0:
			pack.SourceOmissions = pack.SourceOmissions[:len(pack.SourceOmissions)-1]
			pack.SourceUnrepresented++
		case len(pack.Audit.Unknown) > 1:
			pack.Audit.Unknown = pack.Audit.Unknown[:len(pack.Audit.Unknown)-1]
		case len(pack.SourceSections) > 0:
			removed := pack.SourceSections[len(pack.SourceSections)-1]
			pack.SourceSections = pack.SourceSections[:len(pack.SourceSections)-1]
			pack.SourceUnrepresented++
			for i := range pack.Audit.Areas {
				area := &pack.Audit.Areas[i]
				if area.Name == removed.Role {
					area.Delivered--
					area.Coverage = "partial"
					if area.Delivered == 0 {
						area.Coverage = "none"
					}
				}
			}
		case len(pack.Audit.Areas) > 0:
			pack.Audit.Areas = nil
		case utf8.RuneCountInString(pack.Query) > 32:
			pack.Query = string([]rune(pack.Query)[:min(32, len([]rune(pack.Query)))])
			pack.Audit.QueryTruncated = true
		case len(pack.Audit.Unknown) > 0:
			pack.Audit.Unknown = nil
		default:
			return ContextPack{}, errContextPackBudget
		}
	}
	return finalizeContextEstimate(pack)
}

func appendAuditUnknown(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	if len(values) < 16 {
		return append(values, value)
	}
	return values
}

func readAuditSource(loaded loadedContextIndex, source scan.AgentAuditSource, cache map[string]sourceFile, reads *int) (sourceFile, error) {
	key := auditSourceKey(source.Project, source.File)
	if file, found := cache[key]; found {
		return file, nil
	}
	if *reads >= 64 {
		return sourceFile{}, fmt.Errorf("audit source read limit reached")
	}
	*reads = *reads + 1
	resolved, err := resolveSourcePath(loaded, sourceCandidate{Project: source.Project, Path: source.File})
	if err != nil {
		return sourceFile{}, err
	}
	file, err := readSourceFile(resolved)
	if err == nil {
		cache[key] = file
	}
	return file, err
}

func auditRoundRobinKeys(keys []string, sources map[string]scan.AgentAuditSource) []string {
	buckets := map[string][]string{}
	var areas []string
	for _, key := range keys {
		area := auditArea(sources[key])
		if _, ok := buckets[area]; !ok {
			areas = append(areas, area)
		}
		buckets[area] = append(buckets[area], key)
	}
	sort.Strings(areas)
	for _, area := range areas {
		sort.Strings(buckets[area])
	}
	result := make([]string, 0, len(keys))
	for row := 0; len(result) < len(keys); row++ {
		for _, area := range areas {
			if row < len(buckets[area]) {
				result = append(result, buckets[area][row])
			}
		}
	}
	return result
}

func auditDeliveredLinks(sources []scan.AgentAuditSource, sections []ContextSourceSection) []ContextRelationship {
	delivered := map[string]bool{}
	for _, section := range sections {
		if section.SourceState == "indexed_range_current" {
			delivered[auditSourceKey(section.Project, section.Path)] = true
		}
	}
	var links []ContextRelationship
	for _, source := range sources {
		if !delivered[auditSourceKey(source.Project, source.File)] {
			continue
		}
		for _, ref := range source.References {
			if delivered[auditSourceKey(source.Project, ref.File)] && len(links) < 32 {
				links = append(links, ContextRelationship{From: source.Project + "/" + source.File, To: source.Project + "/" + ref.File, Kind: ref.Kind, Reason: "static file reference; not evidence of execution"})
			}
		}
	}
	return links
}
