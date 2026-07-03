package scan

import (
	"regexp"
	"strings"
)

var (
	pythonClassRE  = regexp.MustCompile(`^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)`)
	pythonFuncRE   = regexp.MustCompile(`^(\s*)def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pythonImportRE = regexp.MustCompile(`^\s*import\s+([A-Za-z_][A-Za-z0-9_\.]*)`)
	pythonFromRE   = regexp.MustCompile(`^\s*from\s+([A-Za-z_][A-Za-z0-9_\.]*)\s+import\s+`)
)

func extractPythonSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	var symbols []SymbolRecord
	if match := pythonClassRE.FindStringSubmatch(line); len(match) == 2 {
		symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "class", File: file.Path, Line: lineNo})
	}
	if match := pythonFuncRE.FindStringSubmatch(line); len(match) == 3 {
		kind := "function"
		if match[1] != "" {
			kind = "method"
		}
		if strings.HasPrefix(match[2], "test_") {
			kind = "test"
		}
		symbols = append(symbols, SymbolRecord{Name: match[2], Kind: kind, File: file.Path, Line: lineNo})
	}
	if strings.Contains(line, `__name__`) && strings.Contains(line, `__main__`) {
		symbols = append(symbols, SymbolRecord{Name: "Python main guard", Kind: "entrypoint", File: file.Path, Line: lineNo})
	}
	return symbols
}

func extractPythonRelations(file FileRecord, line string, lineNo int) []RelationRecord {
	var relations []RelationRecord
	if match := pythonImportRE.FindStringSubmatch(line); len(match) == 2 {
		relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
	}
	if match := pythonFromRE.FindStringSubmatch(line); len(match) == 2 {
		relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
	}
	return relations
}
