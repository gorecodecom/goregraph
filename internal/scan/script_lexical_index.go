package scan

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

// scriptLexicalIndex is local to one extraction; offsets always refer to masked text.
// Independent delimiter stacks preserve the conservative scanner's handling of
// incomplete and mismatched syntax without manufacturing a parsed syntax tree.
type scriptLexicalIndex struct {
	ctx              context.Context
	masked           string
	pairs            map[int]int
	parenParents     map[int]int
	parenCloses      []int
	lines            []int
	scopes           []scriptLexicalScope
	functions        []int
	arrows           map[int]scriptLexicalRange
	arrowCandidates  []scriptArrowCandidate
	arrowMaxEnds     []int
	parameterRanges  []scriptLexicalRange
	parameterMaxEnds []int
	shadows          [5]map[string][]scriptLexicalRange
	owners           []scriptDeclarationSpan
	ownerMaxEnds     []int
}

type scriptLexicalRange struct{ start, end int }

type scriptArrowCandidate struct {
	parameters scriptLexicalRange
	visible    scriptLexicalRange
}

type scriptLexicalScope struct {
	start, end, parent int
	function           bool
}

func newScriptLexicalIndex(ctx context.Context, masked string) (*scriptLexicalIndex, error) {
	index := &scriptLexicalIndex{
		ctx: ctx, masked: masked, pairs: make(map[int]int), parenParents: make(map[int]int), lines: []int{0},
		scopes: []scriptLexicalScope{{start: -1, end: len(masked), parent: -1}},
		arrows: make(map[int]scriptLexicalRange),
	}
	var stacks [3][]int
	scopeStack := []int{0}
	for offset := 0; offset < len(masked); offset++ {
		if offset%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		value := masked[offset]
		if value == '\n' {
			index.lines = append(index.lines, offset+1)
		}
		kind, opening := -1, false
		switch value {
		case '(':
			kind, opening = 0, true
		case ')':
			kind = 0
		case '[':
			kind, opening = 1, true
		case ']':
			kind = 1
		case '{':
			kind, opening = 2, true
		case '}':
			kind = 2
		}
		if kind < 0 {
			continue
		}
		if opening {
			if kind == 0 && len(stacks[0]) > 0 {
				index.parenParents[offset] = stacks[0][len(stacks[0])-1]
			}
			stacks[kind] = append(stacks[kind], offset)
			if kind == 2 {
				index.scopes = append(index.scopes, scriptLexicalScope{start: offset, end: len(masked), parent: scopeStack[len(scopeStack)-1]})
				scopeStack = append(scopeStack, len(index.scopes)-1)
			}
		} else if count := len(stacks[kind]); count > 0 {
			open := stacks[kind][count-1]
			stacks[kind] = stacks[kind][:count-1]
			index.pairs[open], index.pairs[offset] = offset, open
			if kind == 0 {
				index.parenCloses = append(index.parenCloses, offset)
			}
			if kind == 2 {
				index.scopes[scopeStack[len(scopeStack)-1]].end = offset
				scopeStack = scopeStack[:len(scopeStack)-1]
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return index, nil
}

func (index *scriptLexicalIndex) match(offset int) int {
	if paired, ok := index.pairs[offset]; ok {
		return paired
	}
	return -1
}

func (index *scriptLexicalIndex) lineAt(offset int) int {
	return sort.Search(len(index.lines), func(i int) bool { return index.lines[i] > offset })
}

func (index *scriptLexicalIndex) scopeAt(offset int) int {
	scope := sort.Search(len(index.scopes), func(i int) bool { return index.scopes[i].start >= offset }) - 1
	if scope < 0 {
		return 0
	}
	for scope > 0 && index.scopes[scope].end < offset {
		scope = index.scopes[scope].parent
	}
	return scope
}

func (index *scriptLexicalIndex) depthAt(offset int) int {
	depth := 0
	for scope := index.scopeAt(offset); scope > 0; scope = index.scopes[scope].parent {
		depth++
	}
	return depth
}

func (index *scriptLexicalIndex) scopeRange(offset int, function bool) scriptLexicalRange {
	scope := index.scopeAt(offset)
	if function && len(index.functions) > 0 {
		scope = index.functions[scope]
	}
	current := index.scopes[scope]
	if scope == 0 {
		return scriptLexicalRange{0, current.end}
	}
	return scriptLexicalRange{current.start, current.end}
}

func (index *scriptLexicalIndex) addShadow(kind int, names map[string]bool, span scriptLexicalRange) {
	if span.start >= span.end {
		return
	}
	for name := range names {
		if index.shadows[kind] == nil {
			index.shadows[kind] = make(map[string][]scriptLexicalRange)
		}
		index.shadows[kind][name] = append(index.shadows[kind][name], span)
	}
}

// Parameter shadowing intentionally includes annotation identifiers, as the old
// conservative extractor did; refining that policy is separate from indexing.
func scriptParameterNames(params string) map[string]bool {
	names := make(map[string]bool)
	for offset := 0; offset < len(params); {
		if !isScriptIdentifierByte(params[offset]) {
			offset++
			continue
		}
		start := offset
		for offset < len(params) && isScriptIdentifierByte(params[offset]) {
			offset++
		}
		names[params[start:offset]] = true
	}
	return names
}

func (index *scriptLexicalIndex) buildBindings() error {
	masked := index.masked
	if err := index.buildArrowCandidates(); err != nil {
		return err
	}
	// Visit the same outer parenthesis groups as the original scope recognizer.
	for open := strings.IndexByte(masked, '('); open >= 0; {
		if err := index.ctx.Err(); err != nil {
			return err
		}
		close := index.match(open)
		if close < 0 {
			break
		}
		next := nextScriptNonSpace(masked, close+1)
		for next < len(masked) && masked[next] == ':' {
			next++
			for next < len(masked) && masked[next] != '{' && masked[next] != '\n' && masked[next] != ';' {
				next++
			}
			next = nextScriptNonSpace(masked, next)
		}
		if next < len(masked) && masked[next] == '{' && isScriptParameterScopePrefix(masked, open) {
			end := index.match(next)
			if end < 0 {
				end = len(masked)
			}
			index.addShadow(0, scriptParameterNames(masked[open+1:close]), scriptLexicalRange{next + 1, end})
		}
		body := scriptParameterBlockBodyStart(masked, close)
		if body >= 0 {
			if isScriptParameterScopePrefix(masked, open) {
				index.parameterRanges = append(index.parameterRanges, scriptLexicalRange{open + 1, close})
			}
			if isScriptFunctionParameterScopePrefix(masked, open, close, body) {
				index.markFunction(body)
			}
		}
		relative := strings.IndexByte(masked[close+1:], '(')
		if relative < 0 {
			break
		}
		open = close + 1 + relative
	}
	for search := 0; search < len(masked); {
		if err := index.ctx.Err(); err != nil {
			return err
		}
		relative := strings.Index(masked[search:], "=>")
		if relative < 0 {
			break
		}
		arrow := search + relative
		start, end, ok := index.arrowParameterRange(arrow)
		body := nextScriptNonSpace(masked, arrow+2)
		if body < len(masked) && masked[body] == '{' {
			index.markFunction(body)
		}
		if ok {
			params := scriptLexicalRange{start, end}
			index.arrows[arrow] = params
			index.parameterRanges = append(index.parameterRanges, params)
			bodyEnd := len(masked)
			if body < len(masked) {
				if masked[body] == '{' {
					if close := index.match(body); close >= 0 {
						bodyEnd = close
					}
				} else {
					bodyEnd = scriptArrowExpressionEnd(masked, body)
				}
			}
			index.addShadow(1, scriptParameterNames(masked[start:end]), scriptLexicalRange{body, bodyEnd})
		}
		search = arrow + 2
	}
	index.functions = make([]int, len(index.scopes))
	for scope := 1; scope < len(index.scopes); scope++ {
		if index.scopes[scope].function {
			index.functions[scope] = scope
		} else {
			index.functions[scope] = index.functions[index.scopes[scope].parent]
		}
	}
	for _, location := range scriptVariableBindingRE.FindAllStringIndex(masked, -1) {
		if err := index.ctx.Err(); err != nil {
			return err
		}
		start := nextScriptNonSpace(masked, location[1])
		end := scriptVariableStatementEnd(masked, start)
		if start >= end {
			continue
		}
		span := index.scopeRange(location[0], masked[location[0]:location[1]] == "var")
		span.start++
		for _, declarator := range splitScriptTopLevel(masked[start:end], ',') {
			if equals := findScriptTopLevel(declarator, '='); equals >= 0 {
				declarator = declarator[:equals]
			}
			index.addShadow(2, scriptBindingPatternNames(declarator), span)
		}
	}
	for _, match := range scriptFunctionRE.FindAllStringSubmatchIndex(masked, -1) {
		if err := index.ctx.Err(); err != nil {
			return err
		}
		if index.depthAt(match[0]) == 0 {
			continue
		}
		span := index.scopeRange(match[0], false)
		span.start++
		index.addShadow(3, map[string]bool{masked[match[6]:match[7]]: true}, span)
	}
	for _, match := range scriptTypeDeclarationRE.FindAllStringSubmatchIndex(masked, -1) {
		if err := index.ctx.Err(); err != nil {
			return err
		}
		if index.depthAt(match[0]) == 0 {
			continue
		}
		span := index.scopeRange(match[0], false)
		span.start++
		index.addShadow(4, map[string]bool{masked[match[8]:match[9]]: true}, span)
	}
	for _, bindings := range index.shadows {
		for name, ranges := range bindings {
			if err := index.ctx.Err(); err != nil {
				return err
			}
			sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
			merged := ranges[:0]
			for _, span := range ranges {
				if len(merged) == 0 || merged[len(merged)-1].end < span.start {
					merged = append(merged, span)
				} else if span.end > merged[len(merged)-1].end {
					merged[len(merged)-1].end = span.end
				}
			}
			bindings[name] = merged
		}
	}
	sort.Slice(index.parameterRanges, func(i, j int) bool { return index.parameterRanges[i].start < index.parameterRanges[j].start })
	index.parameterMaxEnds = make([]int, len(index.parameterRanges))
	end := 0
	for i, span := range index.parameterRanges {
		end = maxInt(end, span.end)
		index.parameterMaxEnds[i] = end
	}
	return index.ctx.Err()
}

func (index *scriptLexicalIndex) markFunction(open int) {
	scope := sort.Search(len(index.scopes), func(i int) bool { return index.scopes[i].start >= open })
	if scope < len(index.scopes) && index.scopes[scope].start == open {
		index.scopes[scope].function = true
	}
}

func (index *scriptLexicalIndex) arrowParameterRange(arrow int) (int, int, bool) {
	masked := index.masked
	end := arrow
	for end > 0 && isScriptWhitespace(masked[end-1]) {
		end--
	}
	if end == 0 {
		return 0, 0, false
	}
	immediateEnd := len(strings.TrimRightFunc(masked[:end], unicode.IsSpace))
	if immediateEnd > 0 && masked[immediateEnd-1] == ')' {
		close := immediateEnd - 1
		if open := index.match(close); open >= 0 && isScriptArrowParameterOpen(masked, open) && isPlausibleScriptParameterList(masked[open+1:close]) {
			return open + 1, close, true
		}
	}
	candidate := sort.Search(len(index.arrowCandidates), func(i int) bool { return index.arrowCandidates[i].visible.start > end }) - 1
	for ; candidate >= 0 && index.arrowMaxEnds[candidate] > end; candidate-- {
		value := index.arrowCandidates[candidate]
		if end < value.visible.end {
			return value.parameters.start, value.parameters.end, true
		}
	}
	start := end - 1
	for start >= 0 && isScriptIdentifierByte(masked[start]) {
		start--
	}
	return start + 1, end, start+1 < end
}

// A typed parameter group can be considered until its enclosing parentheses
// close. Indexing that visibility reproduces the backward recognizer's jumps
// over completed groups without revisiting the preceding file at each arrow.
func (index *scriptLexicalIndex) buildArrowCandidates() error {
	for _, close := range index.parenCloses {
		if err := index.ctx.Err(); err != nil {
			return err
		}
		masked := index.masked
		annotation := strings.TrimLeftFunc(masked[close+1:], unicode.IsSpace)
		if !strings.HasPrefix(annotation, ":") {
			continue
		}
		open := index.match(close)
		if !isScriptArrowParameterOpen(masked, open) || !isPlausibleScriptParameterList(masked[open+1:close]) {
			continue
		}
		start := close + 1
		// TrimSpace above also accepts Unicode whitespace, as the original
		// annotation recognizer did.
		for start < len(masked) && masked[start] != ':' {
			start++
		}
		end := len(masked)
		if parent, ok := index.parenParents[open]; ok {
			if paired := index.match(parent); paired >= 0 {
				end = paired
			}
		}
		index.arrowCandidates = append(index.arrowCandidates, scriptArrowCandidate{parameters: scriptLexicalRange{open + 1, close}, visible: scriptLexicalRange{start + 1, end}})
	}
	index.arrowMaxEnds = make([]int, len(index.arrowCandidates))
	end := 0
	for i, candidate := range index.arrowCandidates {
		end = maxInt(end, candidate.visible.end)
		index.arrowMaxEnds[i] = end
	}
	return index.ctx.Err()
}

func (index *scriptLexicalIndex) shadowReason(name string, offset int) string {
	reasons := [...]string{"lexically shadowed by function or method parameter", "lexically shadowed by arrow parameter", "lexically shadowed by local variable", "lexically shadowed by nested function", "lexically shadowed by nested declaration"}
	for kind, bindings := range index.shadows {
		ranges := bindings[name]
		i := sort.Search(len(ranges), func(i int) bool { return ranges[i].start > offset }) - 1
		if i >= 0 && offset < ranges[i].end {
			return reasons[kind]
		}
	}
	return ""
}

func (index *scriptLexicalIndex) isParameterType(offset int) bool {
	i := sort.Search(len(index.parameterRanges), func(i int) bool { return index.parameterRanges[i].start > offset }) - 1
	for ; i >= 0 && index.parameterMaxEnds[i] > offset; i-- {
		span := index.parameterRanges[i]
		if offset < span.end && isScriptParameterAnnotationAt(index.masked[span.start:span.end], offset-span.start) {
			return true
		}
	}
	return false
}

func (index *scriptLexicalIndex) isReturnType(closeParen, typeEnd int) bool {
	open := index.match(closeParen)
	if open < 0 {
		return false
	}
	next := nextScriptNonSpace(index.masked, typeEnd)
	if next+1 < len(index.masked) && index.masked[next:next+2] == "=>" {
		params, ok := index.arrows[next]
		return ok && params.start == open+1 && params.end == closeParen
	}
	return next < len(index.masked) && index.masked[next] == '{' && isScriptParameterScopePrefix(index.masked, open)
}

func (index *scriptLexicalIndex) setOwners(spans []scriptDeclarationSpan) {
	index.owners = spans
	index.ownerMaxEnds = make([]int, len(spans))
	end := 0
	for i, span := range spans {
		end = maxInt(end, span.end)
		index.ownerMaxEnds[i] = end
	}
}

func (index *scriptLexicalIndex) ownerAt(offset int) RichSymbolRecord {
	i := sort.Search(len(index.owners), func(i int) bool { return index.owners[i].start > offset }) - 1
	for ; i >= 0 && index.ownerMaxEnds[i] > offset; i-- {
		if span := index.owners[i]; offset < span.end {
			return span.declaration
		}
	}
	return RichSymbolRecord{}
}
