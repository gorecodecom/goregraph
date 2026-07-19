package agent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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

func renderSourceCandidate(candidate sourceCandidate, file sourceFile, mode string) (ContextSourceSection, error) {
	identifier := contextIdentifier(candidate)
	occurrences := identifierOccurrences(file.Lines, identifier)
	declarations := declarationOccurrences(file.Lines, occurrences)
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
	renderStart, renderEnd, err := sourceRenderRange(file.Lines, declaration.Line, rangeStart, rangeEnd, mode)
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
	for _, word := range []string{"new", "await", "yield", "assert"} {
		if containsSourceWord(prefix, word) {
			return false
		}
	}
	return true
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
	if strings.Contains(prefix, "=") || strings.Contains(prefix, ".") ||
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
	if candidate.EndLine > 0 {
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

func sourceRenderRange(lines []string, declarationLine, verifiedStart, verifiedEnd int, mode string) (int, int, error) {
	switch mode {
	case "body":
		if verifiedEnd-verifiedStart+1 > 120 {
			return 0, 0, fmt.Errorf("source body exceeds 120 lines")
		}
		return verifiedStart, verifiedEnd, nil
	case "focused":
		return clampSourceLine(declarationLine-28, len(lines)), clampSourceLine(declarationLine+32, len(lines)), nil
	case "signature":
		start := sourceAnnotationStart(lines, declarationLine)
		maximumEnd := start + 11
		if maximumEnd > len(lines) {
			maximumEnd = len(lines)
		}
		for line := declarationLine; line <= maximumEnd; line++ {
			if sourceDeclarationTerminated(lines[line-1]) {
				return start, line, nil
			}
		}
		return 0, 0, fmt.Errorf("source signature is unavailable")
	default:
		return 0, 0, fmt.Errorf("unsupported source render mode %q", mode)
	}
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

func sourceDeclarationTerminated(line string) bool {
	return strings.Contains(line, "{") || strings.Contains(line, ":") ||
		strings.Contains(line, ";") || strings.Contains(line, "=>")
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

	parts := []string{loaded.ScopeRoot}
	if loaded.Workspace {
		project := strings.TrimSpace(candidate.Project)
		if project == "" || filepath.IsAbs(project) {
			return "", fmt.Errorf("source path is unsafe")
		}
		parts = append(parts, project)
	}
	parts = append(parts, path)
	resolvedCandidate := filepath.Clean(filepath.Join(parts...))
	if !pathIsWithin(loaded.ScopeRoot, resolvedCandidate) {
		return "", fmt.Errorf("source path is unsafe")
	}

	resolvedRoot, err := filepath.EvalSymlinks(loaded.ScopeRoot)
	if err != nil {
		return "", fmt.Errorf("source path is unsafe")
	}
	resolvedCandidate, err = filepath.EvalSymlinks(resolvedCandidate)
	if err != nil {
		return "", fmt.Errorf("source file is unreadable: %w", err)
	}
	if !pathIsWithin(resolvedRoot, resolvedCandidate) {
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
