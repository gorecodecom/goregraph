package scan

import (
	"regexp"
	"strings"
)

var nativeRawStart = regexp.MustCompile("^" + cRawStart.String())

// nativePreprocessorSafe accepts only unconditional directives and a canonical header guard.
// It does not evaluate macros, arbitrary branches or compiler command-line definitions.
func nativePreprocessorSafe(body string) bool {
	lines := strings.Split(nativeDirectiveMask(body), "\n")
	type directive struct {
		line int
		text string
	}
	var directives []directive
	for at, line := range lines {
		text := strings.TrimSpace(line)
		if strings.HasPrefix(text, "#") {
			directives = append(directives, directive{at, text})
		}
	}
	guard := ""
	first, last := -1, -1
	if len(directives) >= 3 {
		fields := strings.Fields(directives[0].text)
		if len(fields) == 2 && fields[0] == "#ifndef" && nativeMacroName(fields[1]) && strings.Fields(directives[1].text)[0] == "#define" {
			define := strings.Fields(directives[1].text)
			if len(define) == 2 && define[1] == fields[1] && strings.TrimSpace(directives[len(directives)-1].text) == "#endif" {
				guard = fields[1]
				first = directives[0].line
				last = directives[len(directives)-1].line
				for _, line := range lines[:first] {
					if strings.TrimSpace(line) != "" {
						guard = ""
					}
				}
				for _, line := range lines[last+1:] {
					if strings.TrimSpace(line) != "" {
						guard = ""
					}
				}
			}
		}
	}
	for _, d := range directives {
		fields := strings.Fields(d.text)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "#include", "#import":
		case "#pragma":
			if strings.Join(fields, " ") != "#pragma once" {
				return false
			}
		case "#ifndef":
			if guard == "" || d.line != first {
				return false
			}
		case "#define":
			if guard == "" || d.line != directives[1].line {
				return false
			}
		case "#endif":
			if guard == "" || d.line != last {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func nativeMacroName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (i == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// nativeDirectiveMask removes opaque/comment bodies while retaining literal directive operands.
func nativeDirectiveMask(body string) string {
	masked := []byte(body)
	blank := func(start, end int) {
		for i := start; i < end; i++ {
			if masked[i] != '\n' {
				masked[i] = ' '
			}
		}
	}
	for at := 0; at < len(body); {
		end := at
		if strings.HasPrefix(body[at:], "//") {
			end = at
			for end < len(body) && body[end] != '\n' {
				end++
			}
		} else if strings.HasPrefix(body[at:], "/*") {
			close := strings.Index(body[at+2:], "*/")
			end = len(body)
			if close >= 0 {
				end = at + close + 4
			}
		} else if match := nativeRawStart.FindStringSubmatchIndex(body[at:]); match != nil {
			delimiter := body[at+match[2] : at+match[3]]
			content := at + match[1]
			close := strings.Index(body[content:], ")"+delimiter+"\"")
			end = len(body)
			if close >= 0 {
				end = content + close + len(delimiter) + 2
			}
		} else if body[at] == '\'' || body[at] == '"' {
			quote := body[at]
			end = at + 1
			for end < len(body) {
				if body[end] == '\\' {
					end += 2
					continue
				}
				if body[end] == quote {
					end++
					break
				}
				end++
			}
			if end > len(body) {
				end = len(body)
			}
			lineStart := strings.LastIndexByte(body[:at], '\n') + 1
			if strings.HasPrefix(strings.TrimSpace(body[lineStart:at]), "#") {
				at = end
				continue
			}
		}
		if end > at {
			blank(at, end)
			at = end
		} else {
			at++
		}
	}
	return string(masked)
}
