package scan

import (
	"path"
	"strings"
	"unicode"
)

// godotToken keeps physical source positions and separates literals from executable text.
type godotToken struct {
	text         string
	kind         string
	line, column int
}

func godotTokens(body string, resource bool) []godotToken {
	var tokens []godotToken
	runes := []rune(body)
	line, column := 1, 0
	advance := func(r rune) {
		if r == '\n' {
			line++
			column = 0
		} else {
			column++
		}
	}
	for i := 0; i < len(runes); {
		r := runes[i]
		if unicode.IsSpace(r) {
			advance(r)
			i++
			continue
		}
		if r == '#' || resource && r == ';' {
			for i < len(runes) && runes[i] != '\n' {
				advance(runes[i])
				i++
			}
			continue
		}
		startLine, startColumn := line, column
		if r == '\'' || r == '"' {
			quote := r
			triple := i+2 < len(runes) && runes[i+1] == quote && runes[i+2] == quote
			width := 1
			if triple {
				width = 3
			}
			for j := 0; j < width; j++ {
				advance(runes[i])
				i++
			}
			var value strings.Builder
			valid, closed := !triple, false
			for i < len(runes) {
				if runes[i] == '\\' {
					valid = false // Escaped resource paths require engine interpretation.
					advance(runes[i])
					i++
					if i < len(runes) {
						value.WriteRune(runes[i])
						advance(runes[i])
						i++
					}
					continue
				}
				if runes[i] == quote && (!triple || i+2 < len(runes) && runes[i+1] == quote && runes[i+2] == quote) {
					for j := 0; j < width; j++ {
						advance(runes[i])
						i++
					}
					closed = true
					break
				}
				if runes[i] == '\n' && !triple {
					valid = false
				}
				value.WriteRune(runes[i])
				advance(runes[i])
				i++
			}
			kind := "opaque"
			if valid && closed {
				kind = "string"
			}
			tokens = append(tokens, godotToken{value.String(), kind, startLine, startColumn})
			continue
		}
		start := i
		kind := "punctuation"
		if unicode.IsLetter(r) || r == '_' {
			kind = "identifier"
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				advance(runes[i])
				i++
			}
		} else if unicode.IsDigit(r) {
			kind = "number"
			for i < len(runes) && unicode.IsDigit(runes[i]) {
				advance(runes[i])
				i++
			}
		} else {
			advance(r)
			i++
		}
		tokens = append(tokens, godotToken{string(runes[start:i]), kind, startLine, startColumn})
	}
	return tokens
}

func godotResourcePath(file, uri string) string {
	if uri == "" || strings.ContainsAny(uri, "\\\x00\r\n") {
		return ""
	}
	var target string
	if strings.HasPrefix(uri, "res://") {
		target = strings.TrimPrefix(uri, "res://")
	} else {
		if strings.Contains(uri, ":") || path.IsAbs(uri) {
			return ""
		}
		target = path.Join(path.Dir(file), uri)
	}
	target = path.Clean(target)
	if target == "." || target == ".." || strings.HasPrefix(target, "../") || path.IsAbs(target) {
		return ""
	}
	return target
}
