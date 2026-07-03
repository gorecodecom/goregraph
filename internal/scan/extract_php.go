package scan

import (
	"regexp"
	"strings"
)

var (
	phpNamespaceRE = regexp.MustCompile(`^\s*namespace\s+([^;]+);`)
	phpUseRE       = regexp.MustCompile(`^\s*use\s+([^;]+);`)
	phpClassRE     = regexp.MustCompile(`^\s*(?:abstract\s+|final\s+)?(class|interface|trait)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	phpFunctionRE  = regexp.MustCompile(`^(\s*)(?:public\s+|protected\s+|private\s+|static\s+|final\s+|abstract\s+)*function\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	phpIncludeRE   = regexp.MustCompile(`\b(require|require_once|include|include_once)\b(.+)`)
	phpFileRE      = regexp.MustCompile(`['"]([^'"]+\.php)['"]`)
)

func extractPHPSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	var symbols []SymbolRecord
	if match := phpNamespaceRE.FindStringSubmatch(line); len(match) == 2 {
		symbols = append(symbols, SymbolRecord{Name: strings.TrimSpace(match[1]), Kind: "namespace", File: file.Path, Line: lineNo})
	}
	if match := phpClassRE.FindStringSubmatch(line); len(match) == 3 {
		symbols = append(symbols, SymbolRecord{Name: match[2], Kind: match[1], File: file.Path, Line: lineNo})
	}
	if match := phpFunctionRE.FindStringSubmatch(line); len(match) == 3 {
		kind := "function"
		if match[1] != "" {
			kind = "method"
		}
		if strings.HasSuffix(match[2], "Test") || strings.HasPrefix(match[2], "test") {
			kind = "test"
		}
		symbols = append(symbols, SymbolRecord{Name: match[2], Kind: kind, File: file.Path, Line: lineNo})
	}
	if strings.HasSuffix(file.Path, "index.php") {
		symbols = append(symbols, SymbolRecord{Name: "PHP front controller", Kind: "entrypoint", File: file.Path, Line: 1})
	}
	return symbols
}

func extractPHPRelations(file FileRecord, line string, lineNo int) []RelationRecord {
	var relations []RelationRecord
	if match := phpUseRE.FindStringSubmatch(line); len(match) == 2 {
		relations = append(relations, RelationRecord{From: file.Path, To: strings.TrimSpace(match[1]), Type: "imports", Line: lineNo})
	}
	if match := phpIncludeRE.FindStringSubmatch(line); len(match) == 3 {
		target := strings.TrimSpace(match[2])
		if fileMatch := phpFileRE.FindStringSubmatch(target); len(fileMatch) == 2 {
			relations = append(relations, RelationRecord{From: file.Path, To: strings.TrimPrefix(fileMatch[1], "/"), Type: "includes", Line: lineNo})
		}
	}
	return relations
}
