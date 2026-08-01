package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gorecodecom/goregraph/internal/scan"
)

type sourceCandidate struct {
	FactID         string
	FactIDs        []string
	Project        string
	Path           string
	StartLine      int
	EndLine        int
	Role           string
	Kind           string
	Name           string
	Qualified      string
	SourceState    string
	Priority       int
	InventoryOnly  bool
	InventoryGroup string
}

type sourceFile struct {
	Path  string
	Lines []string
}

type ContextSourceSection struct {
	Project     string `json:"project,omitempty"`
	Path        string `json:"path"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	Role        string `json:"role"`
	RenderMode  string `json:"render_mode"`
	SourceState string `json:"source_state"`
	Content     string `json:"content"`
}

type sourceOccurrence struct {
	Line  int
	Start int
	End   int
}

var contextSourceRolePriority = map[string]int{
	"entrypoint":   0,
	"call_chain":   1,
	"domain_model": 2,
	"contract":     3,
	"persistence":  4,
	"test":         5,
}

var errContextPackBudget = errors.New("context pack exceeds the requested token or byte budget")

func contextSourceCandidates(pack ContextPack, index scan.AgentContextIndexRecord) []sourceCandidate {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	includeTests := contextQueryRequestsTests(contextSelectionQuery(pack))

	candidates := make([]sourceCandidate, 0, len(pack.selectedSourceFactIDs))
	for _, id := range pack.selectedSourceFactIDs {
		fact, ok := factByID[id]
		if !ok || strings.TrimSpace(fact.File) == "" || contextPackSourceFile(fact.File) == "" {
			continue
		}
		role := contextSourceRole(pack, index, fact)
		if role == "test" {
			if !includeTests {
				continue
			}
		}
		candidates = append(candidates, sourceCandidate{
			FactID: fact.ID, Project: fact.Project, Path: fact.File,
			FactIDs:   []string{fact.ID},
			StartLine: fact.Line, EndLine: fact.EndLine, Role: role,
			Kind: fact.Kind, Name: fact.Name, Qualified: fact.Qualified,
			Priority: contextSourceRolePriority[role],
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return contextSourceCandidateLess(candidates[i], candidates[j])
	})

	merged := make([]sourceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		combined := candidate
		strongest := candidate
		absorbed := false
		for {
			remaining := merged[:0]
			mergedRange := false
			for _, existing := range merged {
				if !sourceCandidateRangesOverlap(existing, combined) {
					remaining = append(remaining, existing)
					continue
				}
				absorbed = true
				mergedRange = true
				combined.StartLine = minimumPositiveContextLine(combined.StartLine, existing.StartLine)
				combined.FactIDs = orderedContextConcernIDs(append(combined.FactIDs, existing.FactIDs...))
				if sourceCandidateRangeEnd(existing) > sourceCandidateRangeEnd(combined) {
					combined.EndLine = sourceCandidateRangeEnd(existing)
				}
				if contextSourceCandidateLess(existing, strongest) {
					strongest = existing
				}
			}
			merged = remaining
			if !mergedRange {
				break
			}
		}
		if !absorbed {
			merged = append(merged, candidate)
			continue
		}
		strongest.StartLine = combined.StartLine
		strongest.EndLine = sourceCandidateRangeEnd(combined)
		strongest.FactIDs = orderedContextConcernIDs(combined.FactIDs)
		merged = append(merged, strongest)
	}
	sort.Slice(merged, func(i, j int) bool {
		return contextSourceCandidateLess(merged[i], merged[j])
	})
	return merged
}

const (
	maximumContextSourcePlanningCandidates = 8
	maximumContextSourceProvingCandidates  = 4
)

func contextSourceCandidatesForConcerns(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	concerns []contextConcern,
) []sourceCandidate {
	return contextSourceCandidatesForConcernsWithModels(
		pack,
		index,
		concerns,
		contextRequestedDomainModelIDs(pack, index),
	)
}

func contextSourceCandidatesForConcernsWithModels(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	concerns []contextConcern,
	requestedModelIDs map[string]bool,
) []sourceCandidate {
	selected := make(map[string]bool, len(pack.selectedSourceFactIDs))
	for _, factID := range pack.selectedSourceFactIDs {
		selected[factID] = true
	}
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	aliases := contextProjectAliases(index.Facts, index.Coverage)
	query := contextSelectionQuery(pack)
	explicitProjects := contextExplicitProjects(query, aliases)
	semanticQueryTokens := contextSourceConcernSemanticQueryTokens(query)
	domainTokens := contextSourceConcernProjectDomainQueryTokens(
		semanticQueryTokens,
		aliases,
		explicitProjects,
	)
	anchorTokens := contextSourceAnchorTokens(pack, factByID)
	for _, concern := range concerns {
		if concern.kind == contextConcernEntrypoint ||
			concern.kind == contextConcernPrimaryPath ||
			concern.kind == contextConcernProject {
			continue
		}
		facts := make([]scan.AgentContextFactRecord, 0, len(concern.candidateFactIDs))
		for _, factID := range concern.candidateFactIDs {
			fact, ok := factByID[factID]
			if !ok || strings.TrimSpace(fact.File) == "" || contextPackSourceFile(fact.File) == "" {
				continue
			}
			facts = append(facts, fact)
		}
		domainFacts := make([]scan.AgentContextFactRecord, 0, len(facts))
		for _, fact := range facts {
			if contextSourceFactMatchesDomain(fact, domainTokens) {
				domainFacts = append(domainFacts, fact)
			}
		}
		if len(domainFacts) > 0 {
			facts = domainFacts
		}
		scoreByFactID := make(map[string]int, len(facts))
		for _, fact := range facts {
			scoreByFactID[fact.ID] = contextSourceConcernFactScoreWithTokensAndIndex(
				fact,
				concern,
				query,
				semanticQueryTokens,
				anchorTokens,
				index,
			)
		}
		factLess := func(left, right scan.AgentContextFactRecord) bool {
			leftScore := scoreByFactID[left.ID]
			rightScore := scoreByFactID[right.ID]
			if leftScore != rightScore {
				return leftScore > rightScore
			}
			if left.Project != right.Project {
				return left.Project < right.Project
			}
			if left.File != right.File {
				return left.File < right.File
			}
			if left.Line != right.Line {
				return left.Line < right.Line
			}
			return left.ID < right.ID
		}
		bestBySource := make(map[string]scan.AgentContextFactRecord, len(facts))
		for _, fact := range facts {
			key := normalizeContextProject(fact.Project) + "\x00" + contextPackSourceFile(fact.File)
			current, found := bestBySource[key]
			if !found || factLess(fact, current) {
				bestBySource[key] = fact
			}
		}
		facts = facts[:0]
		for _, fact := range bestBySource {
			facts = append(facts, fact)
		}
		sort.Slice(facts, func(left, right int) bool {
			return factLess(facts[left], facts[right])
		})
		limit := len(facts)
		if limit > maximumContextSourcePlanningCandidates {
			limit = maximumContextSourcePlanningCandidates
		}
		selectedForConcern := make(map[string]bool, limit+2)
		for _, fact := range facts[:limit] {
			selected[fact.ID] = true
			selectedForConcern[fact.ID] = true
		}
		if concern.kind == contextConcernPersistence {
			remaining := maximumContextSourcePlanningCandidates - len(selectedForConcern)
			if remaining > 2 {
				remaining = 2
			}
			paired := 0
			for _, fact := range facts {
				if paired == remaining {
					break
				}
				if selectedForConcern[fact.ID] ||
					!contextPersistenceFactMatchesRequestedDomainModel(
						pack,
						index,
						fact,
						requestedModelIDs,
					) {
					continue
				}
				selected[fact.ID] = true
				selectedForConcern[fact.ID] = true
				paired++
			}
		}
	}

	expanded := pack
	expanded.selectedSourceFactIDs = make([]string, 0, len(selected))
	for factID := range selected {
		expanded.selectedSourceFactIDs = append(expanded.selectedSourceFactIDs, factID)
	}
	sort.Strings(expanded.selectedSourceFactIDs)
	return contextSourceCandidates(expanded, index)
}

func contextSourceFactMatchesDomain(
	fact scan.AgentContextFactRecord,
	domainTokens map[string]bool,
) bool {
	factTokens := contextExpandedTokenSet(strings.Join([]string{
		fact.Search,
		fact.Name,
		fact.Qualified,
		fact.Summary,
	}, " "))
	for token := range domainTokens {
		if factTokens[token] {
			return true
		}
	}
	return false
}

func contextSourceAnchorTokens(
	pack ContextPack,
	factByID map[string]scan.AgentContextFactRecord,
) map[string]bool {
	result := make(map[string]bool)
	add := func(values ...string) {
		for token := range contextExpandedTokenSet(strings.Join(values, " ")) {
			result[token] = true
		}
	}
	for _, endpoint := range pack.Endpoints {
		add(endpoint.HTTPMethod, endpoint.Path, endpoint.Handler)
	}
	for _, entrypoint := range pack.Entrypoints {
		add(entrypoint.Label, entrypoint.File)
	}
	for _, contract := range pack.Contracts {
		fact, ok := factByID[contract.ID]
		if !ok {
			continue
		}
		add(fact.Name, fact.Qualified)
	}
	return result
}

func contextSourceConcernFactLess(
	left,
	right scan.AgentContextFactRecord,
	concern contextConcern,
	query string,
	anchorTokens map[string]bool,
	index scan.AgentContextIndexRecord,
) bool {
	leftScore := contextSourceConcernFactScoreWithIndex(left, concern, query, anchorTokens, index)
	rightScore := contextSourceConcernFactScoreWithIndex(right, concern, query, anchorTokens, index)
	if leftScore != rightScore {
		return leftScore > rightScore
	}
	if left.Project != right.Project {
		return left.Project < right.Project
	}
	if left.File != right.File {
		return left.File < right.File
	}
	if left.Line != right.Line {
		return left.Line < right.Line
	}
	return left.ID < right.ID
}

func contextSourceConcernFactScore(
	fact scan.AgentContextFactRecord,
	concern contextConcern,
	query string,
	anchorTokens map[string]bool,
) int {
	return contextSourceConcernFactScoreWithIndex(
		fact,
		concern,
		query,
		anchorTokens,
		scan.AgentContextIndexRecord{},
	)
}

func contextSourceConcernFactScoreWithIndex(
	fact scan.AgentContextFactRecord,
	concern contextConcern,
	query string,
	anchorTokens map[string]bool,
	index scan.AgentContextIndexRecord,
) int {
	return contextSourceConcernFactScoreWithTokensAndIndex(
		fact,
		concern,
		query,
		contextSourceConcernSemanticQueryTokens(query),
		anchorTokens,
		index,
	)
}

func contextSourceConcernFactScoreWithTokensAndIndex(
	fact scan.AgentContextFactRecord,
	concern contextConcern,
	query string,
	semanticQueryTokens map[string]bool,
	anchorTokens map[string]bool,
	index scan.AgentContextIndexRecord,
) int {
	score := 10 * contextSourceConcernSemanticMatchCount(fact, semanticQueryTokens)
	domainTokens := make(map[string]bool)
	for token := range contextConcernDomainQueryTokens(semanticQueryTokens) {
		domainTokens[token] = true
	}
	for token := range contextTokenSet(concern.project) {
		delete(domainTokens, token)
	}
	score += 80 * contextStableFactIdentityMatchCount(fact, domainTokens)
	if concern.kind == contextConcernPersistence &&
		contextPersistenceMatchesPrimaryDomainModel(index, fact, domainTokens) {
		score += 260
	}
	factKind := normalizedContextConcernKind(fact.Kind)
	if factKind == concern.kind {
		score += 100
	}
	score += 160 * contextEvidenceFacetFactScore(fact, concern.kind, concern.facet)
	score += contextConcernFactShapeScore(fact, concern.kind)
	if concern.kind == contextConcernSideEffects &&
		contextActionFamiliesOverlap(
			contextEndpointRequestedActions(query),
			contextFactActionFamilies(fact),
		) {
		score += 260
	}
	value := strings.Join([]string{
		fact.Name,
		fact.Qualified,
		filepath.Base(fact.File),
		fact.Search,
		fact.Summary,
	}, " ")
	if contextValueRequestsConcern(value, concern.kind) {
		score += 60
	}
	if concern.kind != contextConcernEntrypoint &&
		concern.kind != contextConcernHTTPContract &&
		(strings.TrimSpace(fact.HTTPMethod) != "" || strings.TrimSpace(fact.Path) != "") {
		score -= 40
	}
	switch strings.ToUpper(strings.TrimSpace(fact.Confidence)) {
	case "EXACT":
		score += 20
	case "RESOLVED", "EXTRACTED":
		score += 10
	}
	if contextGenericPersistenceFact(fact) {
		score -= 30
	}
	if concern.kind == contextConcernPersistence && contextPersistenceOwnerFact(fact) {
		score += 100
	}
	if concern.kind == contextConcernPersistence && contextDomainModelDependencyFact(fact) {
		score -= 60
	}
	if concern.kind == contextConcernPersistence && contextPersistenceDerivedOwnerFact(fact) {
		score -= 60
	}
	factTokens := contextExpandedTokenSet(strings.Join([]string{
		fact.Name,
		fact.Qualified,
		filepath.Base(fact.File),
		fact.HTTPMethod,
		fact.Path,
	}, " "))
	for token := range anchorTokens {
		if factTokens[token] {
			score += 15
		}
	}
	return score
}

func contextSourceConcernSemanticQueryTokens(query string) map[string]bool {
	tokens := contextExpandedTokenSet(query)
	delete(tokens, "call")
	delete(tokens, "calls")
	return tokens
}

func contextSourceConcernProjectDomainQueryTokens(
	queryTokens map[string]bool,
	aliases map[string][]string,
	explicitProjects map[string]bool,
) map[string]bool {
	base := contextConcernDomainQueryTokens(queryTokens)
	result := make(map[string]bool, len(base))
	for token := range base {
		result[token] = true
	}
	for project := range explicitProjects {
		for _, alias := range aliases[project] {
			for token := range contextExpandedTokenSet(alias) {
				delete(result, token)
			}
		}
	}
	return result
}

func contextSourceConcernSemanticMatchCount(
	fact scan.AgentContextFactRecord,
	queryTokens map[string]bool,
) int {
	factTokens := contextExpandedTokenSet(strings.Join([]string{
		fact.Name,
		fact.Qualified,
		fact.Search,
		fact.HTTPMethod,
		fact.Path,
	}, " "))
	matches := 0
	for token := range queryTokens {
		if factTokens[token] {
			matches++
		}
	}
	return matches
}

func contextSourceOmissionCandidateAllowed(
	pack ContextPack,
	concern contextConcern,
	candidate sourceCandidate,
	index scan.AgentContextIndexRecord,
) bool {
	if !contextQueryRequestsMissingContract(contextSelectionQuery(pack)) ||
		candidate.Role != "call_chain" && candidate.Role != "entrypoint" ||
		concern.kind == contextConcernDomainModel ||
		concern.kind == contextConcernPersistence ||
		concern.kind == contextConcernHTTPContract ||
		concern.kind == contextConcernTests {
		return true
	}
	seed, ok := contextConcernPlanningSeed(index, contextSelectionQuery(pack))
	if !ok {
		return true
	}
	reachable, _ := reachableContextConcernEvidence(index, seed.ID)
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		if reachable[factID] {
			return true
		}
	}
	operations := contextSourceOperationalActionsForCandidate(candidate, index)
	if len(operations) == 0 {
		return true
	}
	requested := contextSourceOperationalActions(contextSelectionQuery(pack))
	for operation := range operations {
		if requested[operation] {
			return true
		}
	}
	return false
}

func contextSourceOperationalActionsForCandidate(
	candidate sourceCandidate,
	index scan.AgentContextIndexRecord,
) map[string]bool {
	values := []string{
		candidate.Kind,
		candidate.Name,
		candidate.Qualified,
		candidate.Path,
	}
	factIDs := make(map[string]bool)
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		factIDs[factID] = true
	}
	for _, fact := range index.Facts {
		if !factIDs[fact.ID] {
			continue
		}
		values = append(
			values,
			fact.Name,
			fact.Qualified,
			fact.Search,
			fact.Summary,
			fact.Path,
			fact.File,
		)
	}
	return contextSourceOperationalActions(strings.Join(values, " "))
}

func contextSourceOperationalActions(value string) map[string]bool {
	tokens := contextExpandedTokenSet(value)
	vocabulary := map[string][]string{
		"maintenance":  {"maintenance", "maintain", "wartung"},
		"housekeeping": {"housekeeping", "aufraeumen", "aufräumen"},
		"batch":        {"batch", "batching", "stapel"},
		"cleanup":      {"cleanup", "clean", "bereinigen", "bereinigung"},
		"purge":        {"purge", "purged", "purging"},
		"sweep":        {"sweep", "sweeping"},
		"archive":      {"archive", "archived", "archival", "archivieren"},
		"repair":       {"repair", "repairs", "reparatur"},
	}
	result := make(map[string]bool)
	for operation, aliases := range vocabulary {
		for _, alias := range aliases {
			if tokens[alias] {
				result[operation] = true
				break
			}
		}
	}
	return result
}

func contextLocationIDs(locations []ContextLocation) map[string]bool {
	result := make(map[string]bool, len(locations))
	for _, location := range locations {
		result[location.ID] = true
	}
	return result
}

func contextFactMatchesSelectedEndpoint(fact scan.AgentContextFactRecord, endpoints []ContextEndpoint) bool {
	for _, endpoint := range endpoints {
		if strings.TrimSpace(fact.Project) == strings.TrimSpace(endpoint.Provider) &&
			strings.ToUpper(strings.TrimSpace(fact.HTTPMethod)) == strings.ToUpper(strings.TrimSpace(endpoint.HTTPMethod)) &&
			strings.TrimSpace(fact.Path) == strings.TrimSpace(endpoint.Path) &&
			strings.TrimSpace(fact.Qualified) == strings.TrimSpace(endpoint.Handler) &&
			strings.TrimSpace(fact.File) == strings.TrimSpace(endpoint.File) && fact.Line == endpoint.Line {
			return true
		}
	}
	return false
}

func contextQueryRequestsTests(query string) bool {
	tokens := contextTokenSet(query)
	for _, token := range []string{"test", "tests", "testing", "junit", "jest", "playwright"} {
		if tokens[token] {
			return true
		}
	}
	return false
}

func contextSourceCandidateLess(left, right sourceCandidate) bool {
	if left.Priority != right.Priority {
		return left.Priority < right.Priority
	}
	if left.Project != right.Project {
		return left.Project < right.Project
	}
	if left.Path != right.Path {
		return left.Path < right.Path
	}
	if left.StartLine != right.StartLine {
		return left.StartLine < right.StartLine
	}
	if left.EndLine != right.EndLine {
		return left.EndLine < right.EndLine
	}
	return left.FactID < right.FactID
}

func sourceCandidateRangesOverlap(left, right sourceCandidate) bool {
	if left.Project != right.Project || left.Path != right.Path {
		return false
	}
	return sourceCandidateRangeStart(left) <= sourceCandidateRangeEnd(right)+8 &&
		sourceCandidateRangeStart(right) <= sourceCandidateRangeEnd(left)+8
}

func sourceCandidateRangeStart(candidate sourceCandidate) int {
	if candidate.StartLine > 0 {
		return candidate.StartLine
	}
	return 1
}

func sourceCandidateRangeEnd(candidate sourceCandidate) int {
	if candidate.EndLine > 0 {
		return candidate.EndLine
	}
	return sourceCandidateRangeStart(candidate)
}

func attachContextSource(
	pack ContextPack,
	loaded loadedContextIndex,
	request ContextRequest,
) (ContextPack, error) {
	return selectContextSourceOptions(pack, loaded, request)
}

func finalizeContextPackWithinBudget(pack ContextPack, request ContextRequest) (ContextPack, error) {
	pack, err := finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, err
	}
	fits, err := contextSourcePackFits(pack, request)
	if err != nil {
		return ContextPack{}, err
	}
	if !fits {
		return ContextPack{}, errContextPackBudget
	}
	return pack, nil
}

func contextSourcePackFits(pack ContextPack, request ContextRequest) (bool, error) {
	view, err := contextBudgetView(pack)
	if err != nil {
		return false, err
	}
	if view.EstimatedTokens > request.BudgetTokens {
		return false, nil
	}
	body, err := json.Marshal(view)
	if err != nil {
		return false, err
	}
	return len(body) <= contextByteBudget(request.BudgetTokens), nil
}

func stableContextSourceOmissionReason(err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "source file is missing"
	case errors.Is(err, os.ErrPermission):
		return "source file is unreadable"
	}
	message := err.Error()
	for _, stable := range []string{
		"indexed symbol is absent from current source",
		"indexed symbol is ambiguous in current source",
		"indexed symbol has no unique declaration-like occurrence",
	} {
		if strings.Contains(message, stable) {
			return stable
		}
	}
	switch {
	case strings.Contains(message, "source path is unsafe"):
		return "source path escapes project root"
	case strings.Contains(message, "source file is not regular"):
		return "source file is not a regular file"
	case strings.Contains(message, "source file exceeds maximum size"):
		return "source file exceeds 2097152 bytes"
	case strings.Contains(message, "source file is not valid UTF-8"):
		return "source file is not UTF-8 text"
	case strings.Contains(message, "source file is unreadable"):
		return "source file is unreadable"
	default:
		return "source section does not fit the response budget"
	}
}

func renderSourceCandidate(candidate sourceCandidate, file sourceFile, mode string) (ContextSourceSection, error) {
	if isContextConfigurationResource(file.Path) {
		start, end := indexedSourceRange(candidate, len(file.Lines))
		content := renderNumberedSource(file.Lines, start, end)
		sourceState := candidate.SourceState
		if sourceState == "" {
			sourceState = "indexed_range_current"
		}
		return ContextSourceSection{
			Project:     candidate.Project,
			Path:        candidate.Path,
			StartLine:   start,
			EndLine:     end,
			Role:        candidate.Role,
			RenderMode:  mode,
			SourceState: sourceState,
			Content:     redactContextConfigurationValues(file.Path, content),
		}, nil
	}
	identifier := contextIdentifier(candidate)
	occurrences := identifierOccurrences(file.Lines, identifier)
	codeLines := sourceCodeMask(file.Lines)
	declarations := declarationOccurrences(file.Path, codeLines, occurrences)
	indexedStart, indexedEnd := indexedSourceRange(candidate, len(file.Lines))

	declaration := sourceOccurrence{}
	state := ""
	indexedDeclarations := make([]sourceOccurrence, 0)
	exactIndexedDeclarations := make([]sourceOccurrence, 0, 1)
	for _, occurrence := range declarations {
		if occurrence.Line < indexedStart || occurrence.Line > indexedEnd {
			continue
		}
		indexedDeclarations = append(indexedDeclarations, occurrence)
		if occurrence.Line == candidate.StartLine {
			exactIndexedDeclarations = append(exactIndexedDeclarations, occurrence)
		}
	}
	switch {
	case len(exactIndexedDeclarations) == 1:
		declaration = exactIndexedDeclarations[0]
		state = "indexed_range_current"
	case len(indexedDeclarations) == 1:
		declaration = indexedDeclarations[0]
		state = "indexed_range_current"
	}
	if state == "" && len(declarations) == 1 {
		declaration = declarations[0]
		state = "relocated_current"
	}
	if state == "" && len(occurrences) == 0 {
		if fields, accessor := javaGeneratedAccessorFieldDeclarations(
			file.Path,
			codeLines,
			candidate,
		); accessor {
			switch {
			case len(fields) == 1:
				declaration = fields[0]
				state = "relocated_current"
			case len(fields) > 1:
				return ContextSourceSection{}, fmt.Errorf("indexed symbol is ambiguous in current source")
			}
		}
	}
	if state == "" {
		switch {
		case len(occurrences) == 0:
			return ContextSourceSection{}, fmt.Errorf("indexed symbol is absent from current source")
		case len(declarations) > 1:
			return ContextSourceSection{}, fmt.Errorf("indexed symbol is ambiguous in current source")
		default:
			return ContextSourceSection{}, fmt.Errorf("indexed symbol has no unique declaration-like occurrence")
		}
	}

	rangeStart, rangeEnd := verifiedSourceRange(candidate, len(file.Lines), declaration.Line, state)
	renderStart, renderEnd, err := sourceRenderRange(
		file.Path,
		file.Lines,
		codeLines,
		declaration,
		rangeStart,
		rangeEnd,
		mode,
	)
	if err != nil {
		return ContextSourceSection{}, err
	}
	if candidate.SourceState != "" {
		state = candidate.SourceState
	}
	return ContextSourceSection{
		Project:     candidate.Project,
		Path:        candidate.Path,
		StartLine:   renderStart,
		EndLine:     renderEnd,
		Role:        candidate.Role,
		RenderMode:  mode,
		SourceState: state,
		Content:     renderNumberedSource(file.Lines, renderStart, renderEnd),
	}, nil
}

func redactContextConfigurationValues(path, content string) string {
	if !isContextConfigurationResource(path) {
		return content
	}
	lines := strings.Split(content, "\n")
	yamlValueIndent := -1
	var yamlQuotedScalar byte
	propertiesContinuation := false
	for index, line := range lines {
		prefix, source := contextConfigurationLinePrefix(line)
		trimmed := strings.TrimSpace(source)
		if isContextConfigurationYAML(path) && yamlQuotedScalar != 0 {
			if trimmed == "" {
				continue
			}
			lines[index] = prefix + source[:contextConfigurationIndent(source)] + "<redacted>"
			if contextConfigurationYAMLQuotedScalarCloses(source, yamlQuotedScalar) {
				yamlQuotedScalar = 0
			}
			continue
		}
		if isContextConfigurationYAML(path) && yamlValueIndent >= 0 {
			if trimmed == "" {
				continue
			}
			indent := contextConfigurationIndent(source)
			if indent > yamlValueIndent {
				lines[index] = prefix + source[:indent] + "<redacted>"
				continue
			}
			yamlValueIndent = -1
		}
		if !isContextConfigurationYAML(path) && propertiesContinuation {
			if trimmed == "" {
				propertiesContinuation = false
				continue
			}
			lines[index] = prefix + source[:contextConfigurationIndent(source)] + "<redacted>"
			propertiesContinuation = contextConfigurationPropertyContinues(source)
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") ||
			(!isContextConfigurationYAML(path) && strings.HasPrefix(trimmed, "!")) {
			continue
		}
		if isContextConfigurationYAML(path) {
			if contextConfigurationYAMLStructureMarker(trimmed) {
				continue
			}
			if header, blockIndent, block := contextConfigurationYAMLBlockScalar(source); block {
				lines[index] = prefix + header + "<redacted>"
				yamlValueIndent = blockIndent
				continue
			}
			if delimiter := contextConfigurationYAMLMappingDelimiter(source); delimiter >= 0 {
				value := strings.TrimSpace(source[delimiter+1:])
				if value != "" && !strings.HasPrefix(value, "#") {
					lines[index] = prefix + source[:delimiter+1] + " <redacted>"
					yamlQuotedScalar = contextConfigurationYAMLOpenQuotedScalar(value)
					if yamlQuotedScalar == 0 {
						yamlValueIndent = contextConfigurationYAMLMappingIndent(source, delimiter)
					}
				}
				continue
			}
			if listPrefix, scalar := contextConfigurationYAMLListScalar(source); scalar {
				lines[index] = prefix + listPrefix + "<redacted>"
				yamlQuotedScalar = contextConfigurationYAMLOpenQuotedScalar(source[len(listPrefix):])
				if yamlQuotedScalar == 0 {
					yamlValueIndent = contextConfigurationIndent(source)
				}
				continue
			}
			lines[index] = prefix + source[:contextConfigurationIndent(source)] + "<redacted>"
			yamlQuotedScalar = contextConfigurationYAMLOpenQuotedScalar(trimmed)
			if yamlQuotedScalar == 0 {
				yamlValueIndent = contextConfigurationIndent(source)
			}
			continue
		}
		if keyPrefix, hasValue := contextConfigurationPropertyKeyPrefix(source); hasValue {
			lines[index] = prefix + keyPrefix + "<redacted>"
		}
		propertiesContinuation = contextConfigurationPropertyContinues(source)
	}
	return strings.Join(lines, "\n")
}

func contextConfigurationYAMLBlockScalar(line string) (string, int, bool) {
	delimiter := contextConfigurationYAMLMappingDelimiter(line)
	if delimiter >= 0 && contextConfigurationYAMLBlockIndicator(line[delimiter+1:]) {
		return line[:delimiter+1] + " ", contextConfigurationYAMLMappingIndent(line, delimiter), true
	}
	start := contextConfigurationIndent(line)
	if start >= len(line) || line[start] != '-' {
		return "", 0, false
	}
	valueStart := start + 1
	for valueStart < len(line) && (line[valueStart] == ' ' || line[valueStart] == '\t') {
		valueStart++
	}
	if !contextConfigurationYAMLBlockIndicator(line[valueStart:]) {
		return "", 0, false
	}
	return line[:valueStart], start, true
}

func contextConfigurationYAMLMappingDelimiter(line string) int {
	singleQuoted := false
	doubleQuoted := false
	escaped := false
	for index := 0; index < len(line); index++ {
		character := line[index]
		if escaped {
			escaped = false
			continue
		}
		if doubleQuoted && character == '\\' {
			escaped = true
			continue
		}
		if !doubleQuoted && character == '\'' {
			singleQuoted = !singleQuoted
			continue
		}
		if !singleQuoted && character == '"' {
			doubleQuoted = !doubleQuoted
			continue
		}
		if character == ':' && !singleQuoted && !doubleQuoted &&
			(index+1 == len(line) || strings.ContainsRune(" \t", rune(line[index+1]))) {
			return index
		}
	}
	return -1
}

func contextConfigurationYAMLMappingIndent(line string, delimiter int) int {
	indent := contextConfigurationIndent(line)
	if indent >= len(line) || line[indent] != '-' {
		return indent
	}
	keyStart := indent + 1
	for keyStart < delimiter && (line[keyStart] == ' ' || line[keyStart] == '\t') {
		keyStart++
	}
	if keyStart < delimiter {
		return keyStart
	}
	return indent
}

func contextConfigurationYAMLOpenQuotedScalar(value string) byte {
	var quote byte
	escaped := false
	for index := 0; index < len(value); index++ {
		character := value[index]
		switch quote {
		case '"':
			if escaped {
				escaped = false
				continue
			}
			if character == '\\' {
				escaped = true
				continue
			}
			if character == quote {
				quote = 0
			}
		case '\'':
			if character != quote {
				continue
			}
			if index+1 < len(value) && value[index+1] == quote {
				index++
				continue
			}
			quote = 0
		default:
			if character == '\'' || character == '"' {
				quote = character
			}
		}
	}
	return quote
}

func contextConfigurationYAMLQuotedScalarCloses(value string, quote byte) bool {
	if quote == '\'' {
		for index := 0; index < len(value); index++ {
			if value[index] != quote {
				continue
			}
			if index+1 < len(value) && value[index+1] == quote {
				index++
				continue
			}
			return true
		}
		return false
	}
	escaped := false
	for index := 0; index < len(value); index++ {
		if escaped {
			escaped = false
			continue
		}
		if value[index] == '\\' {
			escaped = true
			continue
		}
		if value[index] == quote {
			return true
		}
	}
	return false
}

func contextConfigurationYAMLStructureMarker(value string) bool {
	if strings.HasPrefix(value, "%") {
		return true
	}
	switch value {
	case "---", "...", "-", "?", "{", "}", "[", "]":
		return true
	default:
		return false
	}
}

func contextConfigurationYAMLBlockIndicator(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && (value[0] == '|' || value[0] == '>')
}

func contextConfigurationPropertyContinues(line string) bool {
	backslashes := 0
	for index := len(line) - 1; index >= 0 && line[index] == '\\'; index-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func contextConfigurationPropertyKeyPrefix(line string) (string, bool) {
	start := 0
	for start < len(line) && (line[start] == ' ' || line[start] == '\t' || line[start] == '\f') {
		start++
	}
	escaped := false
	for index := start; index < len(line); index++ {
		if escaped {
			escaped = false
			continue
		}
		switch line[index] {
		case '\\':
			escaped = true
		case '=', ':':
			return line[:index+1], true
		case ' ', '\t', '\f':
			return line[:index+1], true
		}
	}
	return "", false
}

func contextConfigurationIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func contextConfigurationYAMLListScalar(line string) (string, bool) {
	start := contextConfigurationIndent(line)
	if start >= len(line) || line[start] != '-' {
		return "", false
	}
	valueStart := start + 1
	for valueStart < len(line) && (line[valueStart] == ' ' || line[valueStart] == '\t') {
		valueStart++
	}
	value := strings.TrimSpace(line[valueStart:])
	if value == "" || strings.HasPrefix(value, "#") {
		return "", false
	}
	return line[:valueStart], true
}

func contextConfigurationLinePrefix(line string) (string, string) {
	if tab := strings.Index(line, "\t"); tab > 0 {
		if _, err := strconv.Atoi(line[:tab]); err == nil {
			return line[:tab+1], line[tab+1:]
		}
	}
	return "", line
}

func isContextConfigurationResource(value string) bool {
	base := filepath.Base(value)
	extension := strings.ToLower(filepath.Ext(base))
	if extension != ".properties" && extension != ".yml" && extension != ".yaml" {
		return false
	}
	name := strings.TrimSuffix(base, extension)
	return name == "application" || name == "bootstrap" ||
		strings.HasPrefix(name, "application-") || strings.HasPrefix(name, "bootstrap-")
}

func isContextConfigurationYAML(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".yml" || extension == ".yaml"
}

func javaGeneratedAccessorFieldDeclarations(
	path string,
	codeLines []string,
	candidate sourceCandidate,
) ([]sourceOccurrence, bool) {
	if !strings.EqualFold(filepath.Ext(path), ".java") {
		return nil, false
	}
	identifier := contextIdentifier(candidate)
	field, booleanAccessor, ok := javaGeneratedAccessorFieldName(identifier)
	if !ok {
		return nil, false
	}
	owner, ok := javaAccessorOwnerIdentifier(candidate)
	if !ok {
		return nil, true
	}
	owners := javaTypeDeclarationOccurrences(codeLines, owner)
	if len(owners) != 1 {
		return nil, true
	}
	ownerDeclaration := owners[0]
	occurrences := identifierOccurrences(codeLines, field)
	declarations := make([]sourceOccurrence, 0, len(occurrences))
	for _, occurrence := range occurrences {
		if javaFieldDeclarationLike(
			codeLines[occurrence.Line-1],
			occurrence.Start,
			occurrence.End,
			booleanAccessor,
		) && javaDirectOwnerField(codeLines, ownerDeclaration, occurrence) &&
			javaAccessorGenerationEnabled(codeLines, ownerDeclaration, occurrence, identifier) {
			declarations = append(declarations, occurrence)
		}
	}
	return declarations, true
}

func javaAccessorOwnerIdentifier(candidate sourceCandidate) (string, bool) {
	qualified := strings.TrimSpace(candidate.Qualified)
	identifier := contextIdentifier(candidate)
	if qualified == "" || identifier == "" {
		return "", false
	}
	for _, suffix := range []string{"." + identifier, "#" + identifier, "::" + identifier} {
		if strings.HasSuffix(qualified, suffix) {
			qualified = strings.TrimSuffix(qualified, suffix)
			break
		}
	}
	if qualified == "" {
		return "", false
	}
	for _, separator := range []string{".", "#", "::", "$"} {
		if index := strings.LastIndex(qualified, separator); index >= 0 {
			qualified = qualified[index+len(separator):]
		}
	}
	qualified = strings.TrimSpace(qualified)
	return qualified, qualified != ""
}

func javaTypeDeclarationOccurrences(
	codeLines []string,
	identifier string,
) []sourceOccurrence {
	occurrences := identifierOccurrences(codeLines, identifier)
	declarations := make([]sourceOccurrence, 0, len(occurrences))
	for _, occurrence := range occurrences {
		prefix := codeLines[occurrence.Line-1][:occurrence.Start]
		keyword, index := lastDeclarationKeyword(prefix)
		if index < 0 ||
			(keyword != "class" && keyword != "interface" &&
				keyword != "record" && keyword != "enum") ||
			strings.TrimSpace(prefix[index+len(keyword):]) != "" {
			continue
		}
		declarations = append(declarations, occurrence)
	}
	return declarations
}

func javaDirectOwnerField(
	codeLines []string,
	owner sourceOccurrence,
	field sourceOccurrence,
) bool {
	if field.Line < owner.Line ||
		field.Line == owner.Line && field.Start <= owner.End {
		return false
	}
	depth := 0
	foundOwnerBody := false
	for lineNumber := owner.Line; lineNumber <= field.Line; lineNumber++ {
		line := codeLines[lineNumber-1]
		start := 0
		end := len(line)
		if lineNumber == owner.Line {
			start = owner.End
		}
		if lineNumber == field.Line {
			end = field.Start
		}
		for index := start; index < end; index++ {
			switch line[index] {
			case '{':
				depth++
				foundOwnerBody = true
			case '}':
				depth--
				if depth < 0 || foundOwnerBody && depth == 0 {
					return false
				}
			}
		}
	}
	return foundOwnerBody && depth == 1
}

func javaAccessorGenerationEnabled(
	codeLines []string,
	owner sourceOccurrence,
	field sourceOccurrence,
	identifier string,
) bool {
	getter := strings.HasPrefix(identifier, "get") || strings.HasPrefix(identifier, "is")
	setter := strings.HasPrefix(identifier, "set")
	if !getter && !setter {
		return false
	}
	hasApplicableAnnotation := func(start, end int) bool {
		if start < 1 || end < start || end > len(codeLines) {
			return false
		}
		annotations := []string{"Data"}
		if getter {
			annotations = append(annotations, "Getter", "Value")
		}
		if setter {
			annotations = append(annotations, "Setter")
		}
		for _, annotation := range annotations {
			if javaLombokAnnotationApplied(codeLines, start, end, annotation) {
				return true
			}
		}
		return false
	}
	if hasApplicableAnnotation(sourceAnnotationStart(codeLines, owner.Line), owner.Line) {
		return true
	}
	return field.Line > 0 &&
		hasApplicableAnnotation(sourceAnnotationStart(codeLines, field.Line), field.Line)
}

func javaLombokAnnotationApplied(
	codeLines []string,
	start int,
	end int,
	annotation string,
) bool {
	qualified := "@lombok." + annotation
	simple := "@" + annotation
	simpleImported := javaImportsLombokAnnotation(codeLines, annotation)
	for lineNumber := start; lineNumber <= end; lineNumber++ {
		line := codeLines[lineNumber-1]
		if containsJavaAnnotationToken(line, qualified) ||
			simpleImported && containsJavaAnnotationToken(line, simple) {
			return true
		}
	}
	return false
}

func javaImportsLombokAnnotation(codeLines []string, annotation string) bool {
	want := "import lombok." + annotation + ";"
	for _, line := range codeLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == want || trimmed == "import lombok.*;" {
			return true
		}
	}
	return false
}

func containsJavaAnnotationToken(line, annotation string) bool {
	searchFrom := 0
	for searchFrom <= len(line) {
		index := strings.Index(line[searchFrom:], annotation)
		if index < 0 {
			return false
		}
		start := searchFrom + index
		end := start + len(annotation)
		beforeSafe := start == 0
		if !beforeSafe {
			previous, _ := utf8.DecodeLastRuneInString(line[:start])
			beforeSafe = unicode.IsSpace(previous)
		}
		afterSafe := end == len(line)
		if !afterSafe {
			next, _ := utf8.DecodeRuneInString(line[end:])
			afterSafe = !isSourceIdentifierRune(next) && next != '.'
		}
		if beforeSafe && afterSafe {
			return true
		}
		searchFrom = end
	}
	return false
}

func javaGeneratedAccessorFieldName(identifier string) (string, bool, bool) {
	prefix := ""
	booleanAccessor := false
	switch {
	case strings.HasPrefix(identifier, "is"):
		prefix = "is"
		booleanAccessor = true
	case strings.HasPrefix(identifier, "get"):
		prefix = "get"
	case strings.HasPrefix(identifier, "set"):
		prefix = "set"
	default:
		return "", false, false
	}
	suffix := strings.TrimPrefix(identifier, prefix)
	if suffix == "" {
		return "", false, false
	}
	first, width := utf8.DecodeRuneInString(suffix)
	if first == utf8.RuneError || !unicode.IsUpper(first) {
		return "", false, false
	}
	return string(unicode.ToLower(first)) + suffix[width:], booleanAccessor, true
}

func javaFieldDeclarationLike(
	line string,
	start int,
	end int,
	booleanAccessor bool,
) bool {
	prefix := strings.TrimSpace(line[:start])
	suffix := strings.TrimSpace(line[end:])
	if prefix == "" || sourcePrefixIsUnsafe(prefix) ||
		strings.ContainsAny(prefix, "(){};,") ||
		!strings.HasPrefix(suffix, ";") && !strings.HasPrefix(suffix, "=") {
		return false
	}
	fields := strings.Fields(prefix)
	if len(fields) == 0 || !conservativeCallablePrefix(".java", prefix) {
		return false
	}
	if !booleanAccessor {
		return true
	}
	fieldType := fields[len(fields)-1]
	return fieldType == "boolean"
}

func contextIdentifier(candidate sourceCandidate) string {
	value := strings.TrimSpace(candidate.Name)
	qualified := strings.TrimSpace(candidate.Qualified)
	if qualified != "" && (candidate.Kind == "route" || candidate.Kind == "api_endpoint" || candidate.Kind == "api_contract") || value == "" {
		value = qualified
	}
	separatorIndex, separatorWidth := -1, 0
	for _, separator := range []string{".", "#", "::"} {
		if index := strings.LastIndex(value, separator); index > separatorIndex {
			separatorIndex, separatorWidth = index, len(separator)
		}
	}
	if separatorIndex >= 0 {
		value = value[separatorIndex+separatorWidth:]
	}
	if index := strings.IndexAny(value, "( "); index >= 0 {
		value = value[:index]
	}
	return strings.Trim(value, "`*&#")
}

func identifierOccurrences(lines []string, identifier string) []sourceOccurrence {
	if identifier == "" {
		return nil
	}
	var occurrences []sourceOccurrence
	for lineIndex, line := range lines {
		searchFrom := 0
		for searchFrom <= len(line) {
			index := strings.Index(line[searchFrom:], identifier)
			if index < 0 {
				break
			}
			start := searchFrom + index
			end := start + len(identifier)
			if isWholeSourceToken(line, start, end) {
				occurrences = append(occurrences, sourceOccurrence{Line: lineIndex + 1, Start: start, End: end})
			}
			searchFrom = end
		}
	}
	return occurrences
}

type sourceLexicalState struct {
	blockComment bool
	stringEnd    string
}

func sourceCodeMask(lines []string) []string {
	masked := make([]string, len(lines))
	state := sourceLexicalState{}
	for lineIndex, line := range lines {
		body := []byte(line)
		for index := 0; index < len(line); {
			switch {
			case state.blockComment:
				body[index] = ' '
				if strings.HasPrefix(line[index:], "*/") {
					body[index+1] = ' '
					index += 2
					state.blockComment = false
					continue
				}
				index++
			case state.stringEnd != "":
				if strings.HasPrefix(line[index:], state.stringEnd) {
					blankSourceBytes(body, index, len(state.stringEnd))
					index += len(state.stringEnd)
					state.stringEnd = ""
					continue
				}
				body[index] = ' '
				if line[index] == '\\' && index+1 < len(line) {
					body[index+1] = ' '
					index += 2
					continue
				}
				index++
			case strings.HasPrefix(line[index:], "/*"):
				blankSourceBytes(body, index, 2)
				index += 2
				state.blockComment = true
			case strings.HasPrefix(line[index:], "//") || strings.HasPrefix(line[index:], "--") || line[index] == '#':
				blankSourceBytes(body, index, len(line)-index)
				index = len(line)
			case strings.HasPrefix(line[index:], `"""`) || strings.HasPrefix(line[index:], `'''`):
				state.stringEnd = line[index : index+3]
				blankSourceBytes(body, index, 3)
				index += 3
			case line[index] == '"' || line[index] == '\'' || line[index] == '`':
				state.stringEnd = line[index : index+1]
				body[index] = ' '
				index++
			default:
				index++
			}
		}
		masked[lineIndex] = string(body)
	}
	return masked
}

func blankSourceBytes(body []byte, start, width int) {
	for index := start; index < start+width; index++ {
		body[index] = ' '
	}
}

func isWholeSourceToken(line string, start, end int) bool {
	if start > 0 {
		previous, _ := utf8.DecodeLastRuneInString(line[:start])
		if isSourceIdentifierRune(previous) {
			return false
		}
	}
	if end < len(line) {
		next, _ := utf8.DecodeRuneInString(line[end:])
		if isSourceIdentifierRune(next) {
			return false
		}
	}
	return true
}

func isSourceIdentifierRune(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsDigit(value) || value == '_' || value == '$'
}

func declarationOccurrences(path string, lines []string, occurrences []sourceOccurrence) []sourceOccurrence {
	declarations := make([]sourceOccurrence, 0, len(occurrences))
	for _, occurrence := range occurrences {
		if declarationLikeOccurrence(path, lines[occurrence.Line-1], occurrence.Start, occurrence.End) {
			declarations = append(declarations, occurrence)
		}
	}
	return declarations
}

func declarationLikeOccurrence(path, line string, start, end int) bool {
	prefix := strings.TrimSpace(line[:start])
	if prefix == "" || sourcePrefixIsUnsafe(prefix) {
		return false
	}

	if keyword, index := lastDeclarationKeyword(prefix); index >= 0 {
		between := strings.TrimSpace(prefix[index+len(keyword):])
		if between == "" || keyword == "func" && goReceiverPrefix(between) {
			return true
		}
	}

	suffix := strings.TrimSpace(line[end:])
	if !strings.HasPrefix(suffix, "(") {
		return false
	}
	if strings.ContainsAny(prefix, "(){};,") {
		return false
	}
	return conservativeCallablePrefix(path, prefix)
}

func conservativeCallablePrefix(path, prefix string) bool {
	fields := strings.Fields(prefix)
	if len(fields) == 0 {
		return false
	}
	for _, value := range prefix {
		if unicode.IsLetter(value) || unicode.IsDigit(value) || unicode.IsSpace(value) ||
			strings.ContainsRune("_$*?<>[]@", value) {
			continue
		}
		return false
	}
	if sourceDeclarationModifier(fields[0]) {
		return true
	}
	return cStyleSourcePath(path) && conservativeCStyleReturnTypePrefix(path, prefix)
}

func cStyleSourcePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".c", ".h", ".cc", ".cpp", ".cxx", ".hh", ".hpp", ".hxx", ".java", ".cs":
		return true
	default:
		return false
	}
}

func conservativeCStyleReturnTypePrefix(path, prefix string) bool {
	fields := strings.Fields(prefix)
	if len(fields) == 1 {
		return conservativeCStyleTypeToken(fields[0])
	}
	if !cFamilySourcePath(path) {
		return false
	}

	for len(fields) > 1 && fields[len(fields)-1] == "*" {
		fields = fields[:len(fields)-1]
	}
	if len(fields) == 1 {
		return conservativeCStyleTypeToken(fields[0])
	}
	if len(fields) == 2 && sourceCTypeTag(fields[0]) {
		return sourceTypeIdentifier(fields[1], false)
	}
	return safeMultiTokenCType(strings.Join(fields, " "))
}

func cFamilySourcePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".c", ".h", ".cc", ".cpp", ".cxx", ".hh", ".hpp", ".hxx":
		return true
	default:
		return false
	}
}

func conservativeCStyleTypeToken(value string) bool {
	for strings.HasSuffix(value, "[]") || strings.HasSuffix(value, "*") || strings.HasSuffix(value, "?") {
		switch {
		case strings.HasSuffix(value, "[]"):
			value = strings.TrimSuffix(value, "[]")
		case strings.HasSuffix(value, "*"):
			value = strings.TrimSuffix(value, "*")
		default:
			value = strings.TrimSuffix(value, "?")
		}
	}
	if genericStart := strings.IndexByte(value, '<'); genericStart >= 0 {
		if genericStart == 0 || !strings.HasSuffix(value, ">") ||
			!conservativeCStyleTypeToken(value[genericStart+1:len(value)-1]) {
			return false
		}
		value = value[:genericStart]
	}
	if strings.ContainsAny(value, "<>[]*?") {
		return false
	}
	return sourcePrimitiveType(value) || sourceTypeIdentifier(value, true)
}

func sourceTypeIdentifier(value string, requireTypeShape bool) bool {
	if value == "" || sourceExpressionPrefixWord(value) {
		return false
	}
	for index, current := range value {
		if unicode.IsLetter(current) || current == '_' || current == '$' ||
			index > 0 && unicode.IsDigit(current) {
			continue
		}
		return false
	}
	first, _ := utf8.DecodeRuneInString(value)
	return !requireTypeShape || unicode.IsUpper(first) || strings.ContainsAny(value, "_$")
}

func sourceExpressionPrefixWord(value string) bool {
	switch strings.ToLower(value) {
	case "alignof", "await", "co_await", "co_return", "co_yield", "delete", "new", "sizeof", "throw", "typeof", "yield":
		return true
	default:
		return false
	}
}

func sourcePrimitiveType(value string) bool {
	switch value {
	case "_Bool", "auto", "bool", "boolean", "byte", "char", "char8_t", "char16_t", "char32_t",
		"decimal", "double", "dynamic", "float", "int", "long", "object", "sbyte", "short",
		"signed", "string", "uint", "ulong", "unsigned", "ushort", "void", "wchar_t":
		return true
	default:
		return false
	}
}

func sourceCTypeTag(value string) bool {
	switch value {
	case "enum", "struct", "union":
		return true
	default:
		return false
	}
}

func safeMultiTokenCType(value string) bool {
	switch value {
	case "long double", "long int", "long long", "long long int",
		"short int", "signed char", "signed int", "signed long", "signed long int",
		"signed long long", "signed long long int", "signed short", "signed short int",
		"unsigned char", "unsigned int", "unsigned long", "unsigned long int",
		"unsigned long long", "unsigned long long int", "unsigned short", "unsigned short int":
		return true
	default:
		return false
	}
}

func sourceDeclarationModifier(value string) bool {
	switch value {
	case "public", "protected", "private", "internal", "static", "final", "abstract",
		"virtual", "override", "async", "synchronized", "native", "extern", "inline",
		"constexpr", "const", "volatile", "sealed", "open", "partial", "unsafe":
		return true
	default:
		return false
	}
}

func lastDeclarationKeyword(prefix string) (string, int) {
	lastKeyword, lastIndex := "", -1
	for _, keyword := range []string{"class", "interface", "record", "enum", "type", "func", "function", "def", "fn", "fun"} {
		searchFrom := 0
		for searchFrom <= len(prefix) {
			index := strings.Index(prefix[searchFrom:], keyword)
			if index < 0 {
				break
			}
			index += searchFrom
			if index > lastIndex && isWholeSourceToken(prefix, index, index+len(keyword)) {
				lastKeyword, lastIndex = keyword, index
			}
			searchFrom = index + len(keyword)
		}
	}
	return lastKeyword, lastIndex
}

func goReceiverPrefix(prefix string) bool {
	if !strings.HasPrefix(prefix, "(") || !strings.HasSuffix(prefix, ")") || strings.ContainsAny(prefix, "{};") {
		return false
	}
	depth := 0
	for _, value := range prefix {
		switch value {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func sourcePrefixIsUnsafe(prefix string) bool {
	trimmed := strings.TrimSpace(prefix)
	if strings.ContainsAny(trimmed, "\"'`") || strings.Contains(trimmed, "//") ||
		strings.Contains(trimmed, "/*") || strings.Contains(trimmed, "--") ||
		strings.Contains(trimmed, "#") || strings.HasPrefix(trimmed, "*") {
		return true
	}
	if strings.Contains(prefix, "=") || strings.Contains(prefix, ".") || strings.Contains(prefix, "/") ||
		strings.Contains(prefix, "->") || strings.Contains(prefix, "::") {
		return true
	}
	for _, word := range []string{"return", "throw", "if", "for", "while", "switch", "case", "go", "defer"} {
		if containsSourceWord(prefix, word) {
			return true
		}
	}
	return false
}

func containsSourceWord(text, word string) bool {
	searchFrom := 0
	for searchFrom <= len(text) {
		index := strings.Index(text[searchFrom:], word)
		if index < 0 {
			return false
		}
		start := searchFrom + index
		end := start + len(word)
		if isWholeSourceToken(text, start, end) {
			return true
		}
		searchFrom = end
	}
	return false
}

func indexedSourceRange(candidate sourceCandidate, lineCount int) (int, int) {
	start := clampSourceLine(candidate.StartLine, lineCount)
	end := candidate.EndLine
	if end <= 0 {
		end = start + 28
	}
	end = clampSourceLine(end, lineCount)
	if end < start {
		end = start
	}
	return start, end
}

func verifiedSourceRange(candidate sourceCandidate, lineCount, declarationLine int, state string) (int, int) {
	if state == "indexed_range_current" {
		start, end := indexedSourceRange(candidate, lineCount)
		if candidate.EndLine <= 0 {
			end = clampSourceLine(declarationLine+28, lineCount)
		}
		return start, end
	}

	span := 29
	hasExplicitBodySpan := candidate.EndLine > candidate.StartLine
	isEndpointFact := strings.EqualFold(candidate.Kind, "route") ||
		strings.EqualFold(candidate.Kind, "api_endpoint")
	if candidate.EndLine > 0 && (hasExplicitBodySpan || !isEndpointFact) {
		span = candidate.EndLine - candidate.StartLine + 1
		if span < 1 {
			span = 1
		}
	}
	return declarationLine, clampSourceLine(declarationLine+span-1, lineCount)
}

func clampSourceLine(line, lineCount int) int {
	if lineCount < 1 {
		return 0
	}
	if line < 1 {
		return 1
	}
	if line > lineCount {
		return lineCount
	}
	return line
}

func sourceRenderRange(
	path string,
	lines []string,
	codeLines []string,
	declaration sourceOccurrence,
	verifiedStart int,
	verifiedEnd int,
	mode string,
) (int, int, error) {
	switch mode {
	case "declaration_body":
		if start, end, ok := sourceDeclarationBodyRange(path, lines, codeLines, declaration); ok {
			if end-start+1 > 120 {
				return 0, 0, fmt.Errorf("source declaration body exceeds 120 lines")
			}
			return start, end, nil
		}
		return 0, 0, fmt.Errorf("source declaration body is unavailable")
	case "body":
		if verifiedEnd-verifiedStart+1 > 120 {
			return 0, 0, fmt.Errorf("source body exceeds 120 lines")
		}
		return verifiedStart, verifiedEnd, nil
	case "focused":
		return clampSourceLine(declaration.Line-28, len(lines)), clampSourceLine(declaration.Line+32, len(lines)), nil
	case "signature":
		start := sourceAnnotationStart(codeLines, declaration.Line)
		maximumEnd := start + 11
		if maximumEnd > len(lines) {
			maximumEnd = len(lines)
		}
		if end := sourceSignatureEnd(codeLines, declaration, maximumEnd); end > 0 {
			return start, end, nil
		}
		return 0, 0, fmt.Errorf("source signature is unavailable")
	default:
		return 0, 0, fmt.Errorf("unsupported source render mode %q", mode)
	}
}

func sourceDeclarationBodyRange(
	path string,
	lines []string,
	codeLines []string,
	declaration sourceOccurrence,
) (int, int, bool) {
	if declaration.Line < 1 || declaration.Line > len(lines) {
		return 0, 0, false
	}
	start := sourceAnnotationStart(codeLines, declaration.Line)
	if strings.EqualFold(filepath.Ext(path), ".py") {
		end, ok := sourceIndentedDeclarationEnd(lines, codeLines, declaration.Line)
		return start, end, ok
	}

	depth := 0
	foundBody := false
	for lineNumber := declaration.Line; lineNumber <= len(codeLines); lineNumber++ {
		line := codeLines[lineNumber-1]
		offset := 0
		if lineNumber == declaration.Line {
			offset = declaration.End
		}
		for index := offset; index < len(line); index++ {
			switch line[index] {
			case '{':
				depth++
				foundBody = true
			case '}':
				if !foundBody || depth == 0 {
					return 0, 0, false
				}
				depth--
				if depth == 0 {
					return start, lineNumber, true
				}
			}
		}
		if lineNumber-start+1 > 120 {
			return 0, 0, false
		}
	}
	return 0, 0, false
}

func sourceIndentedDeclarationEnd(lines, codeLines []string, declarationLine int) (int, bool) {
	declaration := lines[declarationLine-1]
	codeDeclaration := strings.TrimSpace(codeLines[declarationLine-1])
	declarationIndent := sourceLeadingIndent(declaration)
	colon := strings.LastIndex(codeDeclaration, ":")
	if colon < 0 {
		return 0, false
	}
	if strings.TrimSpace(codeDeclaration[colon+1:]) != "" {
		return declarationLine, true
	}
	end := declarationLine
	hasBody := false
	for lineNumber := declarationLine + 1; lineNumber <= len(lines); lineNumber++ {
		line := lines[lineNumber-1]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			end = lineNumber
			continue
		}
		if sourceLeadingIndent(line) <= declarationIndent {
			break
		}
		hasBody = true
		end = lineNumber
		if end-declarationLine+1 > 120 {
			return 0, false
		}
	}
	return end, hasBody
}

func sourceLeadingIndent(value string) int {
	indent := 0
	for _, current := range value {
		switch current {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			return indent
		}
	}
	return indent
}

func sourceSignatureEnd(lines []string, declaration sourceOccurrence, maximumEnd int) int {
	parentheses, brackets, angles, braces := 0, 0, 0, 0
	for lineNumber := declaration.Line; lineNumber <= maximumEnd; lineNumber++ {
		line := lines[lineNumber-1]
		start := 0
		if lineNumber == declaration.Line {
			start = declaration.End
		}
		for index := start; index < len(line); index++ {
			atDeclarationLevel := parentheses == 0 && brackets == 0 && angles == 0 && braces == 0
			if atDeclarationLevel && strings.HasPrefix(line[index:], "=>") {
				return lineNumber
			}
			switch line[index] {
			case '(':
				parentheses++
			case ')':
				if parentheses == 0 {
					return 0
				}
				parentheses--
			case '[':
				brackets++
			case ']':
				if brackets == 0 {
					return 0
				}
				brackets--
			case '<':
				angles++
			case '>':
				if angles > 0 {
					angles--
				}
			case '{':
				if atDeclarationLevel {
					return lineNumber
				}
				braces++
			case '}':
				if braces == 0 {
					return 0
				}
				braces--
			case ':', ';':
				if atDeclarationLevel {
					return lineNumber
				}
			}
		}
	}
	return 0
}

func sourceAnnotationStart(lines []string, declarationLine int) int {
	start := declarationLine
	end := declarationLine - 2
	for end >= 0 {
		annotationStart, ok := precedingSourceAnnotation(lines, end)
		if !ok {
			break
		}
		start = annotationStart + 1
		end = annotationStart - 1
	}
	return start
}

func precedingSourceAnnotation(lines []string, end int) (int, bool) {
	if trimmed := strings.TrimSpace(lines[end]); strings.HasPrefix(trimmed, "@") {
		return end, true
	}

	parenthesisBalance := 0
	for line := end; line >= 0; line-- {
		trimmed := strings.TrimSpace(lines[line])
		if trimmed == "" {
			return 0, false
		}
		parenthesisBalance += strings.Count(trimmed, "(") - strings.Count(trimmed, ")")
		if strings.HasPrefix(trimmed, "@") && strings.Contains(trimmed, "(") && parenthesisBalance == 0 {
			return line, true
		}
		if parenthesisBalance >= 0 {
			return 0, false
		}
	}
	return 0, false
}

func renderNumberedSource(lines []string, start, end int) string {
	rendered := make([]string, 0, end-start+1)
	for line := start; line <= end; line++ {
		rendered = append(rendered, strconv.Itoa(line)+"\t"+lines[line-1])
	}
	return strings.Join(rendered, "\n")
}

func resolveSourcePath(loaded loadedContextIndex, candidate sourceCandidate) (string, error) {
	path := strings.TrimSpace(candidate.Path)
	if path == "" || isPortableAbsolutePath(path) {
		return "", fmt.Errorf("source path is unsafe")
	}

	projectRoot := filepath.Clean(loaded.ScopeRoot)
	if loaded.Workspace {
		project := strings.TrimSpace(candidate.Project)
		if project == "" || isPortableAbsolutePath(project) {
			return "", fmt.Errorf("source path is unsafe")
		}
		projectRoot = filepath.Clean(filepath.Join(loaded.ScopeRoot, project))
		if !pathIsWithin(loaded.ScopeRoot, projectRoot) {
			return "", fmt.Errorf("source path is unsafe")
		}
	}
	resolvedCandidate := filepath.Clean(filepath.Join(projectRoot, path))
	if !pathIsWithin(projectRoot, resolvedCandidate) {
		return "", fmt.Errorf("source path is unsafe")
	}

	resolvedRoot, err := filepath.EvalSymlinks(loaded.ScopeRoot)
	if err != nil {
		return "", fmt.Errorf("source path is unsafe")
	}
	resolvedProjectRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return "", fmt.Errorf("source file is unreadable: %w", err)
	}
	if !pathIsWithin(resolvedRoot, resolvedProjectRoot) {
		return "", fmt.Errorf("source path is unsafe")
	}
	resolvedCandidate, err = filepath.EvalSymlinks(resolvedCandidate)
	if err != nil {
		return "", fmt.Errorf("source file is unreadable: %w", err)
	}
	if !pathIsWithin(resolvedProjectRoot, resolvedCandidate) {
		return "", fmt.Errorf("source path is unsafe")
	}

	info, err := os.Stat(resolvedCandidate)
	if err != nil {
		return "", fmt.Errorf("source file is unreadable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("source file is not regular")
	}
	return resolvedCandidate, nil
}

func isPortableAbsolutePath(path string) bool {
	if filepath.IsAbs(path) ||
		strings.HasPrefix(path, "/") ||
		strings.HasPrefix(path, `\`) {
		return true
	}
	return len(path) >= 2 &&
		(path[0] >= 'A' && path[0] <= 'Z' || path[0] >= 'a' && path[0] <= 'z') &&
		path[1] == ':'
}

func pathIsWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func readSourceFile(path string) (sourceFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return sourceFile{}, fmt.Errorf("source file is unreadable: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return sourceFile{}, fmt.Errorf("source file is unreadable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return sourceFile{}, fmt.Errorf("source file is not regular")
	}
	if info.Size() > MaxContextSourceFileBytes {
		return sourceFile{}, fmt.Errorf("source file exceeds maximum size")
	}
	body, err := io.ReadAll(io.LimitReader(file, MaxContextSourceFileBytes+1))
	if err != nil {
		return sourceFile{}, fmt.Errorf("source file is unreadable: %w", err)
	}
	if len(body) > MaxContextSourceFileBytes {
		return sourceFile{}, fmt.Errorf("source file exceeds maximum size")
	}
	if !utf8.Valid(body) {
		return sourceFile{}, fmt.Errorf("source file is not valid UTF-8")
	}
	normalized := strings.ReplaceAll(string(body), "\r\n", "\n")
	if isContextConfigurationResource(path) {
		normalized = strings.ReplaceAll(normalized, "\r", "\n")
	}
	return sourceFile{
		Path:  path,
		Lines: strings.Split(normalized, "\n"),
	}, nil
}
