package scan

import (
	"regexp"
	"strings"
)

var (
	tsImportRE      = regexp.MustCompile(`^\s*import\s+(?:.+?\s+from\s+)?["']([^"']+)["']`)
	tsExportClassRE = regexp.MustCompile(`^\s*export\s+class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	tsClassRE       = regexp.MustCompile(`^\s*class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	tsFuncRE        = regexp.MustCompile(`^\s*(?:export\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	javaClassRE     = regexp.MustCompile(`\b(class|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	javaImportRE    = regexp.MustCompile(`^\s*import\s+(?:static\s+)?([^;]+);`)
	mdHeadingRE     = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
)

func extractSymbols(file FileRecord, body string) []SymbolRecord {
	if file.Language == "go" && strings.HasSuffix(file.Path, ".go") {
		if symbols, ok := extractGoFileSymbols(file, body); ok {
			return symbols
		}
	}
	lines := strings.Split(body, "\n")
	var symbols []SymbolRecord
	for index, line := range lines {
		lineNo := index + 1
		switch file.Language {
		case "go":
			symbols = append(symbols, extractGoSymbols(file, line, lineNo)...)
		case "typescript", "javascript":
			symbols = append(symbols, extractScriptSymbols(file, line, lineNo)...)
		case "python":
			symbols = append(symbols, extractPythonSymbols(file, line, lineNo)...)
		case "php":
			symbols = append(symbols, extractPHPSymbols(file, line, lineNo)...)
		case "shell":
			symbols = append(symbols, extractShellSymbols(file, line, lineNo)...)
		case "java":
			if match := javaClassRE.FindStringSubmatch(line); len(match) == 3 {
				symbols = append(symbols, SymbolRecord{Name: match[2], Kind: match[1], File: file.Path, Line: lineNo})
			}
		case "markdown":
			if match := mdHeadingRE.FindStringSubmatch(line); len(match) == 3 {
				symbols = append(symbols, SymbolRecord{Name: strings.TrimSpace(match[2]), Kind: "heading", File: file.Path, Line: lineNo})
			}
		}
	}
	if file.Path == "package.json" {
		symbols = append(symbols, extractPackageScripts(file, body)...)
	}
	if file.Path == "composer.json" {
		symbols = append(symbols, extractComposerAutoloads(file, body)...)
	}
	return symbols
}

func extractScriptSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	var symbols []SymbolRecord
	if match := tsExportClassRE.FindStringSubmatch(line); len(match) == 2 {
		symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "class", File: file.Path, Line: lineNo})
	} else if match := tsClassRE.FindStringSubmatch(line); len(match) == 2 {
		symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "class", File: file.Path, Line: lineNo})
	}
	if match := tsFuncRE.FindStringSubmatch(line); len(match) == 2 {
		symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "function", File: file.Path, Line: lineNo})
	}
	return symbols
}

func extractRelations(file FileRecord, body string) []RelationRecord {
	if file.Language == "go" && strings.HasSuffix(file.Path, ".go") {
		if relations, ok := extractGoFileRelations(file, body); ok {
			return relations
		}
	}
	lines := strings.Split(body, "\n")
	var relations []RelationRecord
	inGoImportBlock := false
	for index, line := range lines {
		lineNo := index + 1
		switch file.Language {
		case "go":
			relation, inBlock, ok := extractGoImport(file, line, lineNo, inGoImportBlock)
			inGoImportBlock = inBlock
			if ok {
				relations = append(relations, relation)
			}
		case "typescript", "javascript":
			if match := tsImportRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
			}
		case "python":
			relations = append(relations, extractPythonRelations(file, line, lineNo)...)
		case "php":
			relations = append(relations, extractPHPRelations(file, line, lineNo)...)
		case "shell":
			relations = append(relations, extractShellRelations(file, line, lineNo)...)
		case "java":
			if match := javaImportRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: strings.TrimSpace(match[1]), Type: "imports", Line: lineNo})
			}
		}
	}
	return relations
}
