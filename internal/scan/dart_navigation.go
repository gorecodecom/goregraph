package scan

import "strings"

// DartSourceDeclaration is a bounded lexical declaration anchor for read-only source navigation.
type DartSourceDeclaration struct {
	Name, Kind    string
	Line, EndLine int
}

// DartSourceDeclarations shares the indexer's parser with source verification without building an index.
func DartSourceDeclarations(file, body string) []DartSourceDeclaration {
	s := parseDartSource(FileRecord{Path: file}, body)
	if len(s.limitations) > 0 {
		return nil
	}
	project := newDartProject([]dartSource{s}, nil)
	project.extractDartTests()
	project.extractDartClosures()
	s = project.sources[0]
	var result []DartSourceDeclaration
	for _, typ := range s.types {
		end := typ.symbol.Line
		if typ.end < len(s.tokens) {
			end = s.tokens[typ.end].line
		}
		result = append(result, DartSourceDeclaration{Name: typ.symbol.Name, Kind: typ.symbol.Kind, Line: typ.symbol.Line, EndLine: end})
	}
	for _, member := range s.members {
		end := member.symbol.Line
		if member.end < len(s.tokens) {
			end = s.tokens[member.end].line
		}
		result = append(result, DartSourceDeclaration{Name: member.symbol.Name, Kind: member.symbol.Kind, Line: member.symbol.Line, EndLine: end})
	}
	return result
}

// DartSourceCodeMask preserves byte positions and newlines while masking comments and every string form.
func DartSourceCodeMask(body string) string {
	tokens, _ := dartTokens(body)
	masked := []byte(body)
	for i, b := range masked {
		if b != '\n' && b != '\r' {
			masked[i] = ' '
		}
	}
	for _, token := range tokens {
		if token.kind == "string" {
			continue
		}
		if token.start >= 0 && token.end <= len(body) {
			copy(masked[token.start:token.end], body[token.start:token.end])
		}
	}
	return strings.ReplaceAll(string(masked), "\r\n", "\n")
}
