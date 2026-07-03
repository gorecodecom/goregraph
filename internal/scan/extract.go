package scan

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	goPackageRE     = regexp.MustCompile(`^\s*package\s+([A-Za-z_][A-Za-z0-9_]*)\s*$`)
	goModuleRE      = regexp.MustCompile(`^\s*module\s+(.+)\s*$`)
	goFuncRE        = regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	goTypeRE        = regexp.MustCompile(`^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(struct|interface|[A-Za-z_][A-Za-z0-9_]*)`)
	goImportOneRE   = regexp.MustCompile(`^\s*import\s+(?:[._A-Za-z0-9]+\s+)?"([^"]+)"`)
	goImportBlockRE = regexp.MustCompile(`^\s*(?:[._A-Za-z0-9]+\s+)?"([^"]+)"`)
	tsImportRE      = regexp.MustCompile(`^\s*import\s+(?:.+?\s+from\s+)?["']([^"']+)["']`)
	tsExportClassRE = regexp.MustCompile(`^\s*export\s+class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	tsClassRE       = regexp.MustCompile(`^\s*class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	tsFuncRE        = regexp.MustCompile(`^\s*(?:export\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	javaClassRE     = regexp.MustCompile(`\b(class|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	javaImportRE    = regexp.MustCompile(`^\s*import\s+(?:static\s+)?([^;]+);`)
	mdHeadingRE     = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	goTestFuncRE    = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)
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
	return symbols
}

func extractGoFileSymbols(file FileRecord, body string) ([]SymbolRecord, bool) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.Path, body, parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	symbols := []SymbolRecord{{
		Name: parsed.Name.Name,
		Kind: "package",
		File: file.Path,
		Line: fset.Position(parsed.Name.Pos()).Line,
	}}
	for _, decl := range parsed.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			kind := "function"
			if d.Recv != nil {
				kind = "method"
			}
			if strings.HasSuffix(file.Path, "_test.go") && goTestFuncRE.MatchString(d.Name.Name) {
				kind = "test"
			}
			symbols = append(symbols, SymbolRecord{Name: d.Name.Name, Kind: kind, File: file.Path, Line: fset.Position(d.Name.Pos()).Line})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				symbols = append(symbols, SymbolRecord{Name: typeSpec.Name.Name, Kind: "type", File: file.Path, Line: fset.Position(typeSpec.Name.Pos()).Line})
			}
		}
	}
	return symbols, true
}

func extractGoSymbols(file FileRecord, line string, lineNo int) []SymbolRecord {
	var symbols []SymbolRecord
	if match := goPackageRE.FindStringSubmatch(line); len(match) == 2 {
		symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "package", File: file.Path, Line: lineNo})
	}
	if file.Path == "go.mod" {
		if match := goModuleRE.FindStringSubmatch(line); len(match) == 2 {
			symbols = append(symbols, SymbolRecord{Name: "module " + strings.TrimSpace(match[1]), Kind: "module", File: file.Path, Line: lineNo})
		}
	}
	if match := goFuncRE.FindStringSubmatch(line); len(match) == 2 {
		kind := "function"
		if strings.HasSuffix(file.Path, "_test.go") && goTestFuncRE.MatchString(match[1]) {
			kind = "test"
		}
		symbols = append(symbols, SymbolRecord{Name: match[1], Kind: kind, File: file.Path, Line: lineNo})
	}
	if match := goTypeRE.FindStringSubmatch(line); len(match) == 3 {
		symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "type", File: file.Path, Line: lineNo})
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

func extractPackageScripts(file FileRecord, body string) []SymbolRecord {
	var parsed struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil
	}
	var names []string
	for name := range parsed.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	symbols := make([]SymbolRecord, 0, len(names))
	for _, name := range names {
		symbols = append(symbols, SymbolRecord{Name: name, Kind: "script", File: file.Path, Line: 1})
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
		case "java":
			if match := javaImportRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: strings.TrimSpace(match[1]), Type: "imports", Line: lineNo})
			}
		}
	}
	return relations
}

func extractGoFileRelations(file FileRecord, body string) ([]RelationRecord, bool) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.Path, body, parser.ImportsOnly)
	if err != nil {
		return nil, false
	}
	relations := make([]RelationRecord, 0, len(parsed.Imports))
	for _, imported := range parsed.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			continue
		}
		relations = append(relations, RelationRecord{From: file.Path, To: path, Type: "imports", Line: fset.Position(imported.Path.Pos()).Line})
	}
	return relations, true
}

func extractGoImport(file FileRecord, line string, lineNo int, inBlock bool) (RelationRecord, bool, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "import (" {
		return RelationRecord{}, true, false
	}
	if inBlock && trimmed == ")" {
		return RelationRecord{}, false, false
	}
	if inBlock {
		if match := goImportBlockRE.FindStringSubmatch(line); len(match) == 2 {
			return RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo}, true, true
		}
		return RelationRecord{}, true, false
	}
	if match := goImportOneRE.FindStringSubmatch(line); len(match) == 2 {
		return RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo}, false, true
	}
	return RelationRecord{}, false, false
}
