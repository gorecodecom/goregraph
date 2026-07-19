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
	FactID    string
	Project   string
	Path      string
	StartLine int
	EndLine   int
	Role      string
	Kind      string
	Name      string
	Qualified string
	Priority  int
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
	"entrypoint":  0,
	"call_chain":  1,
	"contract":    2,
	"persistence": 3,
	"test":        4,
}

func contextSourceCandidates(pack ContextPack, index scan.AgentContextIndexRecord) []sourceCandidate {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	entrypointIDs := contextLocationIDs(pack.Entrypoints)
	contractIDs := contextLocationIDs(pack.Contracts)
	persistenceIDs := contextLocationIDs(pack.Persistence)
	testIDs := contextLocationIDs(pack.Tests)
	includeTests := contextQueryRequestsTests(pack.Query)

	candidates := make([]sourceCandidate, 0, len(pack.selectedSourceFactIDs))
	for _, id := range pack.selectedSourceFactIDs {
		fact, ok := factByID[id]
		if !ok || strings.TrimSpace(fact.File) == "" || contextPackSourceFile(fact.File) == "" {
			continue
		}
		role := "call_chain"
		isTestCandidate := testIDs[id] || strings.EqualFold(fact.Kind, "test") || contextFactUsesTestSource(fact)
		if isTestCandidate {
			if !includeTests {
				continue
			}
			role = "test"
		} else {
			switch {
			case strings.EqualFold(fact.Kind, "api_endpoint"):
				if contextFactMatchesSelectedEndpoint(fact, pack.Endpoints) {
					role = "entrypoint"
				}
			case entrypointIDs[id]:
				role = "entrypoint"
			case contractIDs[id]:
				role = "contract"
			case persistenceIDs[id]:
				role = "persistence"
			}
		}
		candidates = append(candidates, sourceCandidate{
			FactID: fact.ID, Project: fact.Project, Path: fact.File,
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
		merged = append(merged, strongest)
	}
	sort.Slice(merged, func(i, j int) bool {
		return contextSourceCandidateLess(merged[i], merged[j])
	})
	return merged
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
	return sourceCandidateRangeStart(left) <= sourceCandidateRangeEnd(right) &&
		sourceCandidateRangeStart(right) <= sourceCandidateRangeEnd(left)
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
	candidates := contextSourceCandidates(pack, loaded.Index)
	pack.SourceCoverage = "complete"
	pack.SourceUnrepresented = len(candidates)
	var err error
	pack, err = finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, err
	}

	uncoveredCandidates := 0
	for _, source := range candidates {
		sectionAdded := false
		omissionReason := "source section does not fit the response budget"
		if len(pack.SourceSections) < MaxContextSourceSections {
			path, resolveErr := resolveSourcePath(loaded, source)
			if resolveErr != nil {
				omissionReason = stableContextSourceOmissionReason(resolveErr)
			} else {
				file, readErr := readSourceFile(path)
				if readErr != nil {
					omissionReason = stableContextSourceOmissionReason(readErr)
				} else {
					for _, mode := range []string{"body", "focused", "signature"} {
						section, renderErr := renderSourceCandidate(source, file, mode)
						if renderErr != nil {
							if reason := stableContextSourceOmissionReason(renderErr); reason != "source section does not fit the response budget" {
								omissionReason = reason
							}
							continue
						}
						candidate := cloneContextPack(pack)
						candidate.SourceSections = append(candidate.SourceSections, section)
						candidate.SourceUnrepresented--
						candidate, err = finalizeContextEstimate(candidate)
						if err != nil {
							return ContextPack{}, err
						}
						fits, fitErr := contextSourcePackFits(candidate, request)
						if fitErr != nil {
							return ContextPack{}, fitErr
						}
						if fits {
							pack = candidate
							sectionAdded = true
							break
						}
					}
				}
			}
		}
		if sectionAdded {
			continue
		}

		uncoveredCandidates++
		if len(pack.SourceOmissions) >= MaxContextSourceOmissions {
			continue
		}
		candidate := cloneContextPack(pack)
		candidate.SourceOmissions = append(candidate.SourceOmissions, ContextSourceOmission{
			Project: source.Project, Path: source.Path, Role: source.Role, Reason: omissionReason,
		})
		candidate.SourceUnrepresented--
		candidate, err = finalizeContextEstimate(candidate)
		if err != nil {
			return ContextPack{}, err
		}
		fits, fitErr := contextSourcePackFits(candidate, request)
		if fitErr != nil {
			return ContextPack{}, fitErr
		}
		if fits {
			pack = candidate
		}
	}

	switch {
	case len(pack.SourceSections) == 0:
		pack.SourceCoverage = "none"
	case uncoveredCandidates > 0:
		pack.SourceCoverage = "partial"
	default:
		pack.SourceCoverage = "complete"
	}
	return finalizeContextPackWithinBudget(pack, request)
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
		return ContextPack{}, fmt.Errorf("context pack exceeds the requested token or byte budget")
	}
	return pack, nil
}

func contextSourcePackFits(pack ContextPack, request ContextRequest) (bool, error) {
	if pack.EstimatedTokens > request.BudgetTokens {
		return false, nil
	}
	body, err := json.Marshal(pack)
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
	identifier := contextIdentifier(candidate)
	occurrences := identifierOccurrences(file.Lines, identifier)
	codeLines := sourceCodeMask(file.Lines)
	declarations := declarationOccurrences(codeLines, occurrences)
	indexedStart, indexedEnd := indexedSourceRange(candidate, len(file.Lines))

	declaration := sourceOccurrence{}
	state := ""
	for _, occurrence := range declarations {
		if occurrence.Line < indexedStart || occurrence.Line > indexedEnd {
			continue
		}
		if state != "" {
			state = ""
			break
		}
		declaration = occurrence
		state = "indexed_range_current"
	}
	if state == "" && len(declarations) == 1 {
		declaration = declarations[0]
		state = "relocated_current"
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
	renderStart, renderEnd, err := sourceRenderRange(file.Lines, codeLines, declaration, rangeStart, rangeEnd, mode)
	if err != nil {
		return ContextSourceSection{}, err
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

func contextIdentifier(candidate sourceCandidate) string {
	value := strings.TrimSpace(candidate.Name)
	if candidate.Kind == "route" || candidate.Kind == "api_endpoint" || value == "" {
		value = strings.TrimSpace(candidate.Qualified)
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

func declarationOccurrences(lines []string, occurrences []sourceOccurrence) []sourceOccurrence {
	declarations := make([]sourceOccurrence, 0, len(occurrences))
	for _, occurrence := range occurrences {
		if declarationLikeOccurrence(lines[occurrence.Line-1], occurrence.Start, occurrence.End) {
			declarations = append(declarations, occurrence)
		}
	}
	return declarations
}

func declarationLikeOccurrence(line string, start, end int) bool {
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
	return conservativeCallablePrefix(prefix)
}

func conservativeCallablePrefix(prefix string) bool {
	fields := strings.Fields(prefix)
	if len(fields) == 0 || !sourceDeclarationModifier(fields[0]) {
		return false
	}
	for _, value := range prefix {
		if unicode.IsLetter(value) || unicode.IsDigit(value) || unicode.IsSpace(value) ||
			strings.ContainsRune("_$*?<>[]@", value) {
			continue
		}
		return false
	}
	return true
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
	lines []string,
	codeLines []string,
	declaration sourceOccurrence,
	verifiedStart int,
	verifiedEnd int,
	mode string,
) (int, int, error) {
	switch mode {
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
	if path == "" || filepath.IsAbs(path) {
		return "", fmt.Errorf("source path is unsafe")
	}

	projectRoot := filepath.Clean(loaded.ScopeRoot)
	if loaded.Workspace {
		project := strings.TrimSpace(candidate.Project)
		if project == "" || filepath.IsAbs(project) {
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
	return sourceFile{
		Path:  path,
		Lines: strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n"),
	}, nil
}
