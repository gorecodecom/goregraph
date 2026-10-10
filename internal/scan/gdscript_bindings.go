package scan

import (
	"sort"
	"unicode/utf8"
)

// gdscriptBinding describes a local name only within its lexical scope.
type gdscriptBinding struct {
	name                             string
	start, end, nameAt               int
	initializerStart, initializerEnd int
	targetFile                       string
	writes                           int
}

type gdscriptScopeRow struct {
	start, indent     int
	lowerEnd, sameEnd int
	parent            int
}

func gdscriptExpressionEnd(tokens []godotToken, start, limit int) int {
	depth := 0
	for i := start; i < limit; i++ {
		if i > start && depth == 0 && tokens[i].line > tokens[i-1].line && tokens[i-1].text != "\\" {
			return i
		}
		t := tokens[i]
		if t.kind != "punctuation" {
			continue
		}
		switch t.text {
		case ";":
			if depth == 0 {
				return i
			}
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return limit
}

func gdscriptVariableInitializer(tokens []godotToken, at, limit int) (int, int, bool) {
	if at+2 >= limit || tokens[at+1].kind != "identifier" {
		return 0, 0, false
	}
	j := at + 2
	if tokens[j].text == ":" {
		j++
		typeDepth := 0
		for j < limit && tokens[j].text != "=" {
			t := tokens[j]
			if j > at+3 && typeDepth == 0 && t.line > tokens[j-1].line && tokens[j-1].text != "\\" {
				return 0, 0, false
			}
			if t.kind != "identifier" && t.text != "." && t.text != "[" && t.text != "]" {
				return 0, 0, false
			}
			if t.text == "[" {
				typeDepth++
			} else if t.text == "]" {
				typeDepth--
			}
			j++
		}
	}
	if j+1 >= limit || tokens[j].text != "=" || tokens[j+1].text == "=" {
		return 0, 0, false
	}
	start := j + 1
	return start, gdscriptExpressionEnd(tokens, start, limit), true
}

func gdscriptPlainReceiver(tokens []godotToken, at int) bool {
	return at >= 2 && tokens[at-1].text == "." && tokens[at-2].kind == "identifier" && !gdscriptNodePathName(tokens, at-2) && (at < 3 || tokens[at-3].text != ".")
}

func gdscriptNodePathName(tokens []godotToken, at int) bool {
	for j := at; j >= 0; j-- {
		t := tokens[j]
		if t.text == "$" {
			return true
		}
		if t.text == "%" {
			if j > 0 {
				previous := tokens[j-1]
				if previous.line == t.line && (previous.kind == "number" || previous.kind == "string" || previous.text == ")" || previous.text == "]" || previous.kind == "identifier" && !gdscriptCallKeyword(previous.text)) {
					return false
				}
			}
			return true
		}
		if t.kind != "identifier" && t.text != "/" && t.text != "." {
			return false
		}
		if j == 0 || tokens[j-1].line != t.line || tokens[j-1].column+utf8.RuneCountInString(tokens[j-1].text) != t.column {
			return false
		}
	}
	return false
}

func gdscriptOwnSignalReceiver(tokens []godotToken, at int) (string, bool) {
	if gdscriptPlainReceiver(tokens, at) {
		return tokens[at-2].text, false
	}
	if at >= 4 && tokens[at-1].text == "." && tokens[at-2].kind == "identifier" && tokens[at-3].text == "." && tokens[at-4].text == "self" && tokens[at-4].kind == "identifier" && !gdscriptNodePathName(tokens, at-4) && (at < 5 || tokens[at-5].text != ".") {
		return tokens[at-2].text, true
	}
	return "", false
}

func gdscriptConstructorAlias(tokens []godotToken, start, end int) string {
	if end-start != 5 || tokens[start].kind != "identifier" || tokens[start+1].text != "." || tokens[start+2].kind != "identifier" || tokens[start+2].text != "new" || tokens[start+3].text != "(" || tokens[start+4].text != ")" {
		return ""
	}
	return tokens[start].text
}

func gdscriptScopeRows(tokens []godotToken, start, end int) []gdscriptScopeRow {
	if start >= end {
		return nil
	}
	rows := []gdscriptScopeRow{{start: start, indent: tokens[start].column, lowerEnd: end, sameEnd: end, parent: -1}}
	depth := 0
	for i := start; i < end; i++ {
		if i > start && depth == 0 && tokens[i].line > tokens[i-1].line && tokens[i-1].text != "\\" {
			rows = append(rows, gdscriptScopeRow{start: i, indent: tokens[i].column, lowerEnd: end, sameEnd: end, parent: -1})
		}
		if tokens[i].kind != "punctuation" {
			continue
		}
		switch tokens[i].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			if depth > 0 {
				depth--
			}
		}
	}
	var lower, same []int
	for i := range rows {
		for len(lower) > 0 && rows[lower[len(lower)-1]].indent > rows[i].indent {
			last := lower[len(lower)-1]
			lower = lower[:len(lower)-1]
			rows[last].lowerEnd = rows[i].start
		}
		for len(same) > 0 && rows[same[len(same)-1]].indent >= rows[i].indent {
			last := same[len(same)-1]
			same = same[:len(same)-1]
			rows[last].sameEnd = rows[i].start
		}
		if len(same) > 0 {
			rows[i].parent = same[len(same)-1]
		}
		lower = append(lower, i)
		same = append(same, i)
	}
	return rows
}

func gdscriptBindingScopeEnd(tokens []godotToken, rows []gdscriptScopeRow, at, end int) int {
	rowAt := sort.Search(len(rows), func(i int) bool { return rows[i].start > at }) - 1
	if rowAt < 0 {
		return end
	}
	row := rows[rowAt]
	inlineBlock := false
	switch tokens[row.start].text {
	case "if", "elif", "else", "for", "while", "match":
		for i := row.start; i < at; i++ {
			if tokens[i].kind == "punctuation" && tokens[i].text == ":" {
				inlineBlock = true
			}
		}
	}
	pattern := row.parent >= 0 && tokens[rows[row.parent].start].text == "match"
	if tokens[at].text == "for" || inlineBlock || pattern {
		return row.sameEnd
	}
	return row.lowerEnd
}

func gdscriptLocalBinding(bindings []gdscriptBinding, name string, at int) *gdscriptBinding {
	var found *gdscriptBinding
	for i := range bindings {
		candidate := &bindings[i]
		if candidate.name != name || at < candidate.start || at >= candidate.end {
			continue
		}
		if found == nil || candidate.start > found.start {
			found = candidate
		}
	}
	return found
}

func gdscriptMethodBindings(s gdscriptSource, method gdscriptMethod) []gdscriptBinding {
	var bindings []gdscriptBinding
	for name := range method.parameters {
		bindings = append(bindings, gdscriptBinding{name: name, start: method.start - 1, end: method.end, nameAt: -1})
	}
	rows := gdscriptScopeRows(s.tokens, method.start, method.end)
	closures := gdscriptClosures(s.tokens, method.start, method.end, s.topLevel)
	for i := method.start; i+1 < method.end; i++ {
		t := s.tokens[i]
		if closures[i] || t.kind != "identifier" || (t.text != "var" && t.text != "const" && t.text != "for") || s.tokens[i+1].kind != "identifier" {
			continue
		}
		binding := gdscriptBinding{name: s.tokens[i+1].text, start: i, end: gdscriptBindingScopeEnd(s.tokens, rows, i, method.end), nameAt: i + 1}
		if t.text != "for" {
			if start, end, ok := gdscriptVariableInitializer(s.tokens, i, method.end); ok {
				binding.initializerStart, binding.initializerEnd, binding.writes = start, end, 1
			}
		}
		bindings = append(bindings, binding)
	}
	for i := range bindings {
		binding := &bindings[i]
		if binding.initializerEnd <= binding.initializerStart {
			continue
		}
		alias := gdscriptConstructorAlias(s.tokens, binding.initializerStart, binding.initializerEnd)
		if alias != "" && gdscriptLocalBinding(bindings, alias, binding.initializerStart) == nil {
			binding.targetFile = s.aliases[alias]
		}
	}
	for i := method.start; i < method.end; i++ {
		t := s.tokens[i]
		if t.kind != "identifier" || i > 0 && s.tokens[i-1].text == "." || !gdscriptAssignment(s.tokens, i, method.end) {
			continue
		}
		binding := gdscriptLocalBinding(bindings, t.text, i)
		if binding != nil && i != binding.nameAt {
			binding.writes++
		}
	}
	for i := range bindings {
		if bindings[i].writes != 1 {
			bindings[i].targetFile = ""
		}
	}
	return bindings
}
