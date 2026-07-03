package scan

import (
	"regexp"
	"strings"
)

var (
	shellFuncRE   = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][A-Za-z0-9_-]*)\s*(?:\(\))?\s*\{`)
	shellSourceRE = regexp.MustCompile(`^\s*(?:source|\.)\s+(.+?)(?:\s|$)`)
)

func extractShellSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	var symbols []SymbolRecord
	if match := shellFuncRE.FindStringSubmatch(line); len(match) == 2 {
		symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "function", File: file.Path, Line: lineNo})
	}
	if lineNo == 1 && strings.HasPrefix(line, "#!") {
		symbols = append(symbols, SymbolRecord{Name: "shell script", Kind: "entrypoint", File: file.Path, Line: lineNo})
	}
	return symbols
}

func extractShellRelations(file FileRecord, line string, lineNo int) []RelationRecord {
	if match := shellSourceRE.FindStringSubmatch(line); len(match) == 2 {
		return []RelationRecord{{From: file.Path, To: strings.Trim(match[1], `"'`), Type: "sources", Line: lineNo}}
	}
	return nil
}
