package agent

import (
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

type contextEvidenceInventoryCandidate struct {
	file                    ContextFile
	facets                  map[string]bool
	production              bool
	quality                 int
	dominated               bool
	primaryProjectDuplicate bool
	modelIdentity           string
}

type contextEvidenceInventoryScore struct {
	productionFacets  int
	productionPaths   int
	productionWeak    int
	productionQuality int
	testFacets        int
	testPaths         int
	testWeak          int
	testQuality       int
	key               string
}

func appendContextEvidenceInventory(
	pack ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	concerns []contextConcern,
) (ContextPack, error) {
	candidates := contextEvidenceInventoryCandidates(pack, options, concerns)
	currentScore := contextEvidenceInventoryScoreFor(pack, candidates)
	for _, required := range candidates {
		if contextEvidenceInventoryPathRepresented(pack, required.file) {
			continue
		}
		if required.primaryProjectDuplicate &&
			!contextEvidenceInventoryDuplicateAddsMissingRequestedFacet(pack, required, candidates) {
			continue
		}
		best := ContextPack{}
		bestScore := contextEvidenceInventoryScore{}
		found := false
		replacementIndexes := []int{-1}
		if len(pack.Files) >= request.MaxFiles ||
			contextSourceFileCount(pack) >= request.MaxFiles {
			replacementIndexes = replacementIndexes[:0]
			for index, file := range pack.Files {
				if !contextEvidenceInventoryMandatoryFile(pack, file) {
					replacementIndexes = append(replacementIndexes, index)
				}
			}
		}
		for _, replacementIndex := range replacementIndexes {
			trial := cloneContextPack(pack)
			if replacementIndex >= 0 {
				trial.Files = append(trial.Files[:replacementIndex], trial.Files[replacementIndex+1:]...)
			}
			if !mergeContextFile(&trial, required.file, request.MaxFiles) {
				continue
			}
			if contextSourceFileCount(trial) > request.MaxFiles {
				continue
			}
			trial, err := finalizeContextEstimate(trial)
			if err != nil {
				return ContextPack{}, err
			}
			fits, err := contextSourcePackFits(trial, request)
			if err != nil {
				return ContextPack{}, err
			}
			if !fits {
				continue
			}
			score := contextEvidenceInventoryScoreFor(trial, candidates)
			if !betterContextEvidenceInventoryScore(score, currentScore) ||
				found && !betterContextEvidenceInventoryScore(score, bestScore) {
				continue
			}
			best = trial
			bestScore = score
			found = true
		}
		if found {
			pack = best
			currentScore = bestScore
		}
	}
	return pack, nil
}

func contextEvidenceInventoryCandidates(
	pack ContextPack,
	options []contextSourceOption,
	concerns []contextConcern,
) []contextEvidenceInventoryCandidate {
	ordered := append([]contextSourceOption(nil), options...)
	sort.Slice(ordered, func(i, j int) bool {
		return contextSourceOptionLess(ordered[i], ordered[j])
	})
	concernByKey := make(map[string]contextConcern, len(concerns))
	for _, concern := range concerns {
		if concern.required {
			concernByKey[concern.key] = concern
		}
	}
	byPath := make(map[string]*contextEvidenceInventoryCandidate)
	for _, option := range ordered {
		if !option.profiled {
			continue
		}
		matched := make([]contextConcern, 0, len(option.concernKeys))
		for _, key := range option.concernKeys {
			if concern, ok := concernByKey[key]; ok &&
				contextEvidenceInventoryConcernIsExact(concern) &&
				!(concern.kind == contextConcernDomainModel &&
					contextEvidenceInventoryPrimaryProjectDuplicate(pack, option, options)) {
				matched = append(matched, concern)
			}
		}
		if len(matched) == 0 {
			continue
		}
		primaryProjectDuplicate := false
		for _, concern := range matched {
			if concern.kind == contextConcernDomainModel &&
				contextEvidenceInventoryPrimaryProjectDuplicateCandidate(pack, option, options) {
				primaryProjectDuplicate = true
				break
			}
		}
		sort.Slice(matched, func(i, j int) bool { return matched[i].key < matched[j].key })
		pathKey := contextEvidenceInventoryPathKey(option.section.Project, option.section.Path)
		candidate := byPath[pathKey]
		if candidate == nil {
			candidate = &contextEvidenceInventoryCandidate{
				file: ContextFile{
					Project:   option.section.Project,
					Path:      option.section.Path,
					StartLine: option.section.StartLine,
					EndLine:   option.section.EndLine,
				},
				facets:                  make(map[string]bool),
				production:              option.candidate.Role != "test",
				quality:                 contextSourceEffectiveQuality(pack, option),
				primaryProjectDuplicate: primaryProjectDuplicate,
				modelIdentity: compactContextIdentifier(firstNonEmptyContext(
					option.candidate.Name,
					option.candidate.Qualified,
				)),
			}
			byPath[pathKey] = candidate
		} else {
			candidate.file.StartLine = minimumPositiveContextLine(
				candidate.file.StartLine,
				option.section.StartLine,
			)
			if option.section.EndLine > candidate.file.EndLine {
				candidate.file.EndLine = option.section.EndLine
			}
			candidate.production = candidate.production || option.candidate.Role != "test"
			candidate.quality = max(candidate.quality, contextSourceEffectiveQuality(pack, option))
			candidate.primaryProjectDuplicate =
				candidate.primaryProjectDuplicate || primaryProjectDuplicate
			if candidate.modelIdentity == "" {
				candidate.modelIdentity = compactContextIdentifier(firstNonEmptyContext(
					option.candidate.Name,
					option.candidate.Qualified,
				))
			}
		}
		for _, concern := range matched {
			candidate.facets[concern.key] = true
			candidate.file.Role = mergeContextList(
				candidate.file.Role,
				contextSourceConcernRole(concern.kind),
				",",
			)
			candidate.file.Reason = mergeContextList(
				candidate.file.Reason,
				"selected required "+strings.ReplaceAll(concern.kind, "_", " ")+" evidence",
				";",
			)
		}
	}
	result := make([]contextEvidenceInventoryCandidate, 0, len(byPath))
	for _, candidate := range byPath {
		result = append(result, *candidate)
	}
	for index := range result {
		for otherIndex := range result {
			if index == otherIndex ||
				result[index].production != result[otherIndex].production ||
				normalizeContextProject(result[index].file.Project) !=
					normalizeContextProject(result[otherIndex].file.Project) ||
				result[index].file.Role != result[otherIndex].file.Role ||
				!contextEvidenceInventoryFacetSubset(result[index].facets, result[otherIndex].facets) ||
				len(result[index].facets) >= len(result[otherIndex].facets) {
				continue
			}
			result[index].dominated = true
			break
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return contextEvidenceInventoryCandidateBetter(result[i], result[j])
	})
	return result
}

func contextEvidenceInventoryConcernIsExact(concern contextConcern) bool {
	return concern.kind != contextConcernProject &&
		concern.kind != contextConcernPrimaryPath
}

func contextEvidenceInventoryPrimaryProjectDuplicate(
	pack ContextPack,
	option contextSourceOption,
	options []contextSourceOption,
) bool {
	if option.requestedModel {
		return false
	}
	return contextEvidenceInventoryPrimaryProjectDuplicateCandidate(pack, option, options)
}

func contextEvidenceInventoryPrimaryProjectDuplicateCandidate(
	pack ContextPack,
	option contextSourceOption,
	options []contextSourceOption,
) bool {
	if len(pack.Entrypoints) == 0 {
		return false
	}
	primaryProject := normalizeContextProject(pack.Entrypoints[0].Project)
	if primaryProject == "" ||
		normalizeContextProject(option.candidate.Project) != primaryProject {
		return false
	}
	identity := compactContextIdentifier(firstNonEmptyContext(
		option.candidate.Name,
		option.candidate.Qualified,
	))
	if identity == "" {
		return false
	}
	for _, other := range options {
		if normalizeContextProject(other.candidate.Project) == primaryProject ||
			compactContextIdentifier(firstNonEmptyContext(
				other.candidate.Name,
				other.candidate.Qualified,
			)) != identity {
			continue
		}
		return true
	}
	return false
}

func contextEvidenceInventoryCandidateBetter(
	left contextEvidenceInventoryCandidate,
	right contextEvidenceInventoryCandidate,
) bool {
	if left.production != right.production {
		return left.production
	}
	if left.primaryProjectDuplicate != right.primaryProjectDuplicate {
		return !left.primaryProjectDuplicate
	}
	if left.dominated != right.dominated {
		return !left.dominated
	}
	if len(left.facets) != len(right.facets) {
		return len(left.facets) > len(right.facets)
	}
	if left.quality != right.quality {
		return left.quality > right.quality
	}
	return contextEvidenceInventoryPathKey(left.file.Project, left.file.Path) <
		contextEvidenceInventoryPathKey(right.file.Project, right.file.Path)
}

func contextEvidenceInventoryDuplicateAddsMissingRequestedFacet(
	pack ContextPack,
	candidate contextEvidenceInventoryCandidate,
	candidates []contextEvidenceInventoryCandidate,
) bool {
	for _, counterpart := range candidates {
		if counterpart.primaryProjectDuplicate ||
			counterpart.modelIdentity == "" ||
			counterpart.modelIdentity != candidate.modelIdentity ||
			normalizeContextProject(counterpart.file.Project) ==
				normalizeContextProject(candidate.file.Project) ||
			!counterpart.dominated {
			continue
		}
		dominator, represented := contextEvidenceInventoryRepresentedDominatorCandidate(
			pack,
			counterpart,
			candidates,
		)
		if represented &&
			contextEvidenceInventoryFacetSubset(candidate.facets, dominator.facets) {
			return false
		}
	}
	return true
}

func contextEvidenceInventoryFacetSubset(left, right map[string]bool) bool {
	for key := range left {
		if !right[key] {
			return false
		}
	}
	return true
}

func contextEvidenceInventoryScoreFor(
	pack ContextPack,
	candidates []contextEvidenceInventoryCandidate,
) contextEvidenceInventoryScore {
	productionFacets := make(map[string]bool)
	testFacets := make(map[string]bool)
	representedKeys := make([]string, 0, len(candidates))
	score := contextEvidenceInventoryScore{}
	for _, candidate := range candidates {
		if !contextEvidenceInventoryPathRepresented(pack, candidate.file) {
			continue
		}
		if candidate.dominated &&
			contextEvidenceInventoryRepresentedDominator(pack, candidate, candidates) {
			continue
		}
		representedKeys = append(
			representedKeys,
			contextEvidenceInventoryPathKey(candidate.file.Project, candidate.file.Path),
		)
		if candidate.production {
			score.productionPaths++
			if candidate.dominated {
				score.productionWeak++
			}
			score.productionQuality += candidate.quality
			for key := range candidate.facets {
				productionFacets[key] = true
			}
			continue
		}
		score.testPaths++
		if candidate.dominated {
			score.testWeak++
		}
		score.testQuality += candidate.quality
		for key := range candidate.facets {
			testFacets[key] = true
		}
	}
	score.productionFacets = len(productionFacets)
	score.testFacets = len(testFacets)
	sort.Strings(representedKeys)
	score.key = strings.Join(representedKeys, "\x00")
	return score
}

func contextEvidenceInventoryRepresentedDominator(
	pack ContextPack,
	candidate contextEvidenceInventoryCandidate,
	candidates []contextEvidenceInventoryCandidate,
) bool {
	_, represented := contextEvidenceInventoryRepresentedDominatorCandidate(
		pack,
		candidate,
		candidates,
	)
	return represented
}

func contextEvidenceInventoryRepresentedDominatorCandidate(
	pack ContextPack,
	candidate contextEvidenceInventoryCandidate,
	candidates []contextEvidenceInventoryCandidate,
) (contextEvidenceInventoryCandidate, bool) {
	for _, other := range candidates {
		if candidate.production != other.production ||
			normalizeContextProject(candidate.file.Project) !=
				normalizeContextProject(other.file.Project) ||
			candidate.file.Role != other.file.Role ||
			len(candidate.facets) >= len(other.facets) ||
			!contextEvidenceInventoryFacetSubset(candidate.facets, other.facets) ||
			!contextEvidenceInventoryPathRepresented(pack, other.file) {
			continue
		}
		return other, true
	}
	return contextEvidenceInventoryCandidate{}, false
}

func betterContextEvidenceInventoryScore(
	left contextEvidenceInventoryScore,
	right contextEvidenceInventoryScore,
) bool {
	switch {
	case left.productionFacets != right.productionFacets:
		return left.productionFacets > right.productionFacets
	case left.productionWeak != right.productionWeak:
		return left.productionWeak < right.productionWeak
	case left.productionPaths != right.productionPaths:
		return left.productionPaths > right.productionPaths
	case left.productionQuality != right.productionQuality:
		return left.productionQuality > right.productionQuality
	case left.testFacets != right.testFacets:
		return left.testFacets > right.testFacets
	case left.testWeak != right.testWeak:
		return left.testWeak < right.testWeak
	case left.testPaths != right.testPaths:
		return left.testPaths > right.testPaths
	case left.testQuality != right.testQuality:
		return left.testQuality > right.testQuality
	default:
		return left.key < right.key
	}
}

func contextEvidenceInventoryPathRepresented(pack ContextPack, file ContextFile) bool {
	key := contextEvidenceInventoryPathKey(file.Project, file.Path)
	for _, current := range pack.Files {
		if contextEvidenceInventoryPathKey(current.Project, current.Path) == key {
			return true
		}
	}
	for _, section := range pack.SourceSections {
		if contextEvidenceInventoryPathKey(section.Project, section.Path) == key {
			return true
		}
	}
	return false
}

func contextEvidenceInventoryPathKey(project, path string) string {
	return normalizeContextProject(project) + "\x00" + contextPackSourceFile(path)
}

func contextEvidenceInventoryMandatoryFile(pack ContextPack, file ContextFile) bool {
	if contextEvidenceInventoryRoleContains(
		file.Role,
		"entrypoint",
		"contract",
		"endpoint",
		"endpoint_consumer",
	) || strings.Contains(file.Reason, "selected required primary path evidence") {
		return true
	}
	key := contextEvidenceInventoryPathKey(file.Project, file.Path)
	for _, location := range pack.Entrypoints {
		if contextEvidenceInventoryPathKey(location.Project, location.File) == key {
			return true
		}
	}
	for _, location := range pack.Contracts {
		if contextEvidenceInventoryPathKey(location.Project, location.File) == key {
			return true
		}
	}
	for _, endpoint := range pack.Endpoints {
		if contextEvidenceInventoryPathKey(endpoint.Provider, endpoint.File) == key {
			return true
		}
		for _, consumer := range endpoint.Consumers {
			if contextEvidenceInventoryPathKey(consumer.Project, consumer.File) == key {
				return true
			}
		}
	}
	return false
}

func contextEvidenceInventoryRoleContains(role string, wanted ...string) bool {
	values := make(map[string]bool)
	for _, value := range strings.Split(role, ",") {
		values[strings.TrimSpace(value)] = true
	}
	for _, value := range wanted {
		if values[value] {
			return true
		}
	}
	return false
}

func contextSourceCoverageFromFinalSections(
	pack ContextPack,
	concerns []contextConcern,
	options []contextSourceOption,
) map[string]bool {
	known := make(map[string]contextConcern, len(concerns))
	for _, concern := range concerns {
		known[concern.key] = concern
	}
	covered := make(map[string]bool, len(concerns))
	for _, section := range pack.SourceSections {
		for _, option := range options {
			if option.section != section {
				continue
			}
			for key := range contextSourceOptionProvenConcernKeys(option, known) {
				covered[key] = true
			}
		}
	}
	return covered
}

func contextSourceOptionProvenConcernKeys(
	option contextSourceOption,
	known map[string]contextConcern,
) map[string]bool {
	proven := make(map[string]bool, len(option.concernKeys))
	for _, key := range option.concernKeys {
		concern, ok := known[key]
		if !ok {
			continue
		}
		if concern.kind != contextConcernProject &&
			len(concern.candidateFactIDs) > 0 &&
			!contextSourceOptionMatchesConcernFacts(option, concern) {
			continue
		}
		proven[key] = true
	}
	return proven
}

func contextSourceSectionSupportsDomainModel(section ContextSourceSection) bool {
	if section.RenderMode == "signature" {
		return false
	}
	content := contextSourceSemanticContent(section.Content)
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "@") {
			continue
		}
		declaration, found := contextSourceTypeDeclarationLine(line)
		if !found {
			continue
		}
		declaration, declarationEnd := contextSourceCompleteTypeDeclaration(
			lines,
			index,
			declaration,
		)
		if contextSourceDeclarationHasDomainField(declaration) {
			return true
		}
		if supported, pythonClass := contextSourcePythonClassSupportsDomainField(
			lines,
			index,
			declarationEnd,
			declaration,
		); pythonClass {
			return supported
		}
		body := declaration.inlineBody
		if following := strings.Join(lines[declarationEnd+1:], "\n"); following != "" {
			if body != "" {
				body += "\n"
			}
			body += following
		}
		if declaration.inlineBody == "" {
			trimmed := strings.TrimLeft(body, " \t\r\n")
			if strings.HasPrefix(trimmed, "{") {
				body = strings.TrimPrefix(trimmed, "{")
			}
		}
		if contextSourceDomainModelMembers(body) {
			return true
		}
		return false
	}
	return contextSourceDomainModelMembers(content)
}

type contextSourceTypeDeclaration struct {
	header       string
	inlineBody   string
	kind         string
	hasBodyBrace bool
}

func contextSourceTypeDeclarationLine(line string) (contextSourceTypeDeclaration, bool) {
	line = strings.TrimSpace(line)
	header := line
	inlineBody := ""
	hasBodyBrace := false
	if opening := strings.Index(line, "{"); opening >= 0 {
		header = strings.TrimSpace(line[:opening])
		inlineBody = line[opening+1:]
		hasBodyBrace = true
	}
	fields := strings.Fields(strings.TrimSuffix(header, ":"))
	for len(fields) > 0 && contextSourceDeclarationModifier(fields[0]) {
		fields = fields[1:]
	}
	if len(fields) < 2 {
		return contextSourceTypeDeclaration{}, false
	}
	switch fields[0] {
	case "class", "interface", "struct", "record", "enum", "type":
		return contextSourceTypeDeclaration{
			header:       header,
			inlineBody:   inlineBody,
			kind:         fields[0],
			hasBodyBrace: hasBodyBrace,
		}, true
	default:
		return contextSourceTypeDeclaration{}, false
	}
}

func contextSourceCompleteTypeDeclaration(
	lines []string,
	start int,
	declaration contextSourceTypeDeclaration,
) (contextSourceTypeDeclaration, int) {
	end := start
	depth := contextSourceParenthesisDepth(declaration.header)
	for depth > 0 && end+1 < len(lines) {
		end++
		line := strings.TrimSpace(lines[end])
		headerPart := line
		if opening := strings.Index(line, "{"); opening >= 0 {
			headerPart = strings.TrimSpace(line[:opening])
			declaration.inlineBody = line[opening+1:]
			declaration.hasBodyBrace = true
		}
		declaration.header += "\n" + headerPart
		depth += contextSourceParenthesisDepth(headerPart)
	}
	return declaration, end
}

func contextSourceParenthesisDepth(value string) int {
	depth := 0
	for _, character := range value {
		switch character {
		case '(':
			depth++
		case ')':
			depth--
		}
	}
	return depth
}

func contextSourcePythonClassSupportsDomainField(
	lines []string,
	declarationStart int,
	declarationEnd int,
	declaration contextSourceTypeDeclaration,
) (bool, bool) {
	if declaration.kind != "class" || declaration.hasBodyBrace {
		return false, false
	}
	colon := contextSourceTopLevelSeparator(declaration.header, ':')
	if colon < 0 {
		return false, false
	}
	classIndent := sourceLeadingIndent(lines[declarationStart])
	inlineSuite := strings.TrimSpace(declaration.header[colon+1:])
	for _, line := range lines[declarationEnd+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "{") &&
			(sourceLeadingIndent(line) <= classIndent || inlineSuite != "") {
			return false, false
		}
		break
	}
	if inlineSuite != "" {
		return contextSourcePythonDomainModelFieldLine(inlineSuite), true
	}

	bodyIndent := -1
	for _, line := range lines[declarationEnd+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := sourceLeadingIndent(line)
		if indent <= classIndent {
			break
		}
		if bodyIndent < 0 {
			bodyIndent = indent
		}
		if indent != bodyIndent {
			continue
		}
		member := strings.TrimSpace(line)
		if strings.HasPrefix(member, "def ") ||
			strings.HasPrefix(member, "async def ") ||
			strings.HasPrefix(member, "class ") {
			continue
		}
		if contextSourcePythonDomainModelFieldLine(member) {
			return true, true
		}
	}
	return false, true
}

func contextSourcePythonDomainModelFieldLine(line string) bool {
	line = strings.TrimSpace(line)
	declaration := contextSourceDomainModelDeclaration(line)
	colon := contextSourceTopLevelSeparator(declaration, ':')
	if colon > 0 &&
		strings.TrimSpace(declaration[colon+1:]) != "" &&
		contextSourcePythonIdentifier(strings.TrimSpace(declaration[:colon])) {
		return contextSourceDomainModelFieldLine(declaration)
	}
	return contextSourcePythonAssignmentFieldLine(line)
}

func contextSourcePythonAssignmentFieldLine(line string) bool {
	assignment := contextSourceTopLevelSeparator(line, '=')
	if assignment <= 0 || assignment+1 >= len(line) ||
		line[assignment+1] == '=' ||
		strings.ContainsRune("!<>=:+-*/%@&|^~", rune(line[assignment-1])) {
		return false
	}
	return contextSourcePythonIdentifier(strings.TrimSpace(line[:assignment])) &&
		strings.TrimSpace(line[assignment+1:]) != ""
}

func contextSourcePythonIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			character == '_' ||
			(index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

func contextSourceDeclarationHasDomainField(declaration contextSourceTypeDeclaration) bool {
	parameters, found := contextSourceDeclarationParameters(declaration.header)
	if !found || strings.TrimSpace(parameters) == "" {
		return false
	}
	if declaration.kind == "record" {
		return true
	}
	fields := strings.Fields(strings.NewReplacer(",", " ", "\n", " ").Replace(parameters))
	for index, field := range fields {
		if (field == "val" || field == "var") && index+1 < len(fields) {
			return true
		}
	}
	return false
}

func contextSourceDeclarationParameters(header string) (string, bool) {
	opening := strings.Index(header, "(")
	if opening < 0 {
		return "", false
	}
	depth := 0
	for index := opening; index < len(header); index++ {
		switch header[index] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return header[opening+1 : index], true
			}
		}
	}
	return "", false
}

func contextSourceDomainModelMembers(body string) bool {
	for _, member := range contextSourceInlineDeclarationMembers(body) {
		if contextSourceDomainModelFieldLine(member) {
			return true
		}
	}
	return false
}

func contextSourceInlineDeclarationMembers(body string) []string {
	var members []string
	memberStart := 0
	parenthesisDepth := 0
	braceDepth := 0
	blockEndsMember := false

	for index, character := range body {
		switch character {
		case '(':
			parenthesisDepth++
		case ')':
			if parenthesisDepth > 0 {
				parenthesisDepth--
			}
		case '{':
			if braceDepth == 0 {
				blockEndsMember = parenthesisDepth == 0
			}
			braceDepth++
		case '}':
			if braceDepth == 0 {
				members = append(members, body[memberStart:index])
				return members
			}
			braceDepth--
			if braceDepth == 0 && blockEndsMember {
				members = append(members, body[memberStart:index+1])
				memberStart = index + 1
				blockEndsMember = false
			}
		case ';':
			if braceDepth == 0 && parenthesisDepth == 0 {
				if strings.TrimSpace(body[memberStart:index]) != "" {
					members = append(members, body[memberStart:index+1])
				}
				memberStart = index + 1
				blockEndsMember = false
			}
		}
	}
	if memberStart < len(body) {
		members = append(members, body[memberStart:])
	}
	return members
}

func contextSourceDomainModelFieldLine(line string) bool {
	line = contextSourceDomainModelDeclaration(line)
	if line == "" || line == "{" || line == "}" || strings.Contains(line, "(") {
		return false
	}
	if _, declaration := contextSourceTypeDeclarationLine(line); declaration {
		return false
	}
	fieldHeader := line
	if opening := strings.Index(fieldHeader, "{"); opening >= 0 {
		fieldHeader = fieldHeader[:opening]
	}
	fields := strings.Fields(strings.TrimSpace(fieldHeader))
	for len(fields) > 0 && contextSourceDeclarationModifier(fields[0]) {
		fields = fields[1:]
	}
	if len(fields) == 0 || contextSourceDomainModelStatement(fields[0]) {
		return false
	}
	if strings.HasSuffix(line, ";") ||
		strings.Contains(line, ": ") ||
		strings.Contains(line, "\t") ||
		len(fields) >= 2 {
		return true
	}
	return false
}

func contextSourceDomainModelDeclaration(line string) string {
	line = strings.TrimSpace(line)
	for line != "" {
		switch line[0] {
		case '@':
			end, found := contextSourceLeadingAnnotationEnd(line)
			if !found {
				return line
			}
			line = strings.TrimSpace(line[end:])
		case '[':
			end, found := contextSourceBalancedDelimiterEnd(line, 0, '[', ']')
			if !found {
				return line
			}
			line = strings.TrimSpace(line[end:])
		default:
			if initializer := contextSourceTopLevelSeparator(line, '='); initializer >= 0 {
				line = line[:initializer]
			}
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func contextSourceLeadingAnnotationEnd(line string) (int, bool) {
	if line == "" || line[0] != '@' {
		return 0, false
	}
	end := 1
	for end < len(line) {
		character := line[end]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '.' || character == '$' || character == ':' {
			end++
			continue
		}
		break
	}
	if end == 1 {
		return 0, false
	}
	parenthesis := end
	for parenthesis < len(line) {
		switch line[parenthesis] {
		case ' ', '\t', '\r', '\n':
			parenthesis++
		default:
			if line[parenthesis] == '(' {
				return contextSourceBalancedDelimiterEnd(line, parenthesis, '(', ')')
			}
			return end, true
		}
	}
	return end, true
}

func contextSourceBalancedDelimiterEnd(
	value string,
	start int,
	opening byte,
	closing byte,
) (int, bool) {
	depth := 0
	quote := byte(0)
	escaped := false
	for index := start; index < len(value); index++ {
		character := value[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"', '`':
			quote = character
		case opening:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return index + 1, true
			}
		}
	}
	return 0, false
}

func contextSourceTopLevelSeparator(value string, separator byte) int {
	parentheses := 0
	brackets := 0
	braces := 0
	quote := byte(0)
	escaped := false
	for index := 0; index < len(value); index++ {
		character := value[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"', '`':
			quote = character
		case '(':
			parentheses++
		case ')':
			if parentheses > 0 {
				parentheses--
			}
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		default:
			if character == separator &&
				parentheses == 0 &&
				brackets == 0 &&
				braces == 0 {
				return index
			}
		}
	}
	return -1
}

func contextSourceDomainModelStatement(value string) bool {
	value = strings.ToLower(strings.Trim(value, "{}[]();:,"))
	switch value {
	case "assert", "break", "case", "catch", "continue", "default", "do", "else",
		"finally", "for", "goto", "if", "import", "module", "namespace", "package",
		"return", "switch", "throw", "using", "while", "yield":
		return true
	default:
		return false
	}
}

func contextSourceDeclarationModifier(value string) bool {
	switch value {
	case "public", "protected", "private", "internal", "export", "abstract",
		"final", "sealed", "static", "partial", "readonly", "open", "data":
		return true
	default:
		return false
	}
}

func contextDomainModelEvidenceConcerns(
	base contextConcern,
	index scan.AgentContextIndexRecord,
	requestedModels map[string]bool,
) []contextConcern {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	modelIDs := make([]string, 0, len(requestedModels))
	for modelID := range requestedModels {
		if slicesContainsString(base.candidateFactIDs, modelID) {
			modelIDs = append(modelIDs, modelID)
		}
	}
	sort.Strings(modelIDs)

	result := make([]contextConcern, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		model, ok := factByID[modelID]
		if !ok {
			continue
		}
		concern := newContextEvidenceConcern(
			base,
			"model:"+modelID,
			contextDomainModelEvidenceFactIDs(index, modelID),
			"domain model evidence for requested model "+model.Name,
		)
		concern.project = normalizeContextProject(model.Project)
		result = append(result, concern)
	}
	return result
}

func contextDomainModelEvidenceFactIDs(
	index scan.AgentContextIndexRecord,
	modelID string,
) []string {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	model, ok := factByID[modelID]
	if !ok {
		return nil
	}
	result := []string{modelID}
	for _, edge := range index.Edges {
		if edge.FromFactID != modelID ||
			strings.ToLower(strings.TrimSpace(edge.Kind)) != "extends" {
			continue
		}
		base, found := factByID[edge.ToFactID]
		if found && normalizeContextProject(base.Project) == normalizeContextProject(model.Project) {
			result = append(result, base.ID)
		}
	}
	return orderedContextConcernIDs(result)
}

func slicesContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
