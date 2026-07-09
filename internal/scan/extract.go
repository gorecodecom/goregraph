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
	mdHeadingRE     = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	rustUseRE       = regexp.MustCompile(`^\s*use\s+([^;]+);`)
	rustSymbolRE    = regexp.MustCompile(`^\s*(?:pub\s+)?(?:async\s+)?(fn|struct|enum|trait|impl)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	jvmImportRE     = regexp.MustCompile(`^\s*import\s+([A-Za-z0-9_.*]+)`)
	jvmSymbolRE     = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|open|data|sealed|abstract|final)\s+)*(class|object|interface|enum|fun|trait)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	swiftImportRE   = regexp.MustCompile(`^\s*import\s+([A-Za-z0-9_]+)`)
	swiftSymbolRE   = regexp.MustCompile(`^\s*(?:(?:public|private|internal|open|final)\s+)*(class|struct|enum|protocol|func)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rubyRequireRE   = regexp.MustCompile(`^\s*require(?:_relative)?\s+["']([^"']+)["']`)
	rubySymbolRE    = regexp.MustCompile(`^\s*(class|module|def)\s+([A-Za-z_][A-Za-z0-9_!?=]*)`)
	cIncludeRE      = regexp.MustCompile(`^\s*#\s*include\s+[<"]([^>"]+)[>"]`)
	cSymbolRE       = regexp.MustCompile(`^\s*(?:template\s*<[^>]+>\s*)?(?:(class|struct|enum)\s+([A-Za-z_][A-Za-z0-9_]*)|(?:[A-Za-z_][A-Za-z0-9_:<>,*&\s]+\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*\([^;]*\)\s*(?:\{|$))`)
	csUsingRE       = regexp.MustCompile(`^\s*using\s+([A-Za-z0-9_.]+)\s*;`)
	csSymbolRE      = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|sealed|abstract|partial|async)\s+)*(class|struct|interface|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)`)
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
			return javaSymbols(extractJavaSource(file, body))
		case "rust":
			symbols = append(symbols, extractRustSymbols(file, line, lineNo)...)
		case "kotlin", "scala":
			symbols = append(symbols, extractJVMSymbols(file, line, lineNo)...)
		case "swift":
			symbols = append(symbols, extractSwiftSymbols(file, line, lineNo)...)
		case "ruby":
			symbols = append(symbols, extractRubySymbols(file, line, lineNo)...)
		case "c", "cpp":
			symbols = append(symbols, extractCSymbols(file, line, lineNo)...)
		case "csharp":
			symbols = append(symbols, extractCSharpSymbols(file, line, lineNo)...)
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
			return javaImportRelations(extractJavaSource(file, body))
		case "rust":
			if match := rustUseRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: strings.TrimSpace(match[1]), Type: "imports", Line: lineNo})
			}
		case "kotlin", "scala":
			if match := jvmImportRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
			}
		case "swift":
			if match := swiftImportRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
			}
		case "ruby":
			if match := rubyRequireRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
			}
		case "c", "cpp":
			if match := cIncludeRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
			}
		case "csharp":
			if match := csUsingRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
			}
		}
	}
	return relations
}

func extractRustSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	if match := rustSymbolRE.FindStringSubmatch(line); len(match) == 3 {
		return []SymbolRecord{{Name: match[2], Kind: normalizeSymbolKind(match[1]), File: file.Path, Line: lineNo}}
	}
	return nil
}

func extractJVMSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	if match := jvmSymbolRE.FindStringSubmatch(line); len(match) == 3 {
		return []SymbolRecord{{Name: match[2], Kind: normalizeSymbolKind(match[1]), File: file.Path, Line: lineNo}}
	}
	return nil
}

func extractSwiftSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	if match := swiftSymbolRE.FindStringSubmatch(line); len(match) == 3 {
		return []SymbolRecord{{Name: match[2], Kind: normalizeSymbolKind(match[1]), File: file.Path, Line: lineNo}}
	}
	return nil
}

func extractRubySymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	if match := rubySymbolRE.FindStringSubmatch(line); len(match) == 3 {
		kind := normalizeSymbolKind(match[1])
		return []SymbolRecord{{Name: match[2], Kind: kind, File: file.Path, Line: lineNo}}
	}
	return nil
}

func extractCSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	if match := cSymbolRE.FindStringSubmatch(line); len(match) == 4 {
		if match[2] != "" {
			return []SymbolRecord{{Name: match[2], Kind: normalizeSymbolKind(match[1]), File: file.Path, Line: lineNo}}
		}
		if match[3] != "" {
			return []SymbolRecord{{Name: match[3], Kind: "function", File: file.Path, Line: lineNo}}
		}
	}
	return nil
}

func extractCSharpSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	if match := csSymbolRE.FindStringSubmatch(line); len(match) == 3 {
		return []SymbolRecord{{Name: match[2], Kind: normalizeSymbolKind(match[1]), File: file.Path, Line: lineNo}}
	}
	return nil
}

func normalizeSymbolKind(kind string) string {
	switch strings.ToLower(kind) {
	case "fn", "fun", "func", "def":
		return "function"
	case "trait", "protocol":
		return "interface"
	default:
		return strings.ToLower(kind)
	}
}
