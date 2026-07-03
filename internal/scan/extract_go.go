package scan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
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
	goTestFuncRE    = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)
)

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
