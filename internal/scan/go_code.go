package scan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"
)

type goImportBinding struct {
	alias string
	path  string
}

type goFunctionScope struct {
	file            *goFileScope
	interfaceMethod bool
	packageName     string
	importPath      string
	imports         []goImportBinding
	parameters      int
	variadic        bool
}

type goCallScope struct {
	receiver           ast.Expr
	receiverPackage    string
	offset             int
	arguments          int
	ellipsis           bool
	importAlias        string
	receiverType       string
	receiverExpression bool
	dynamic            bool
}

func extractGoCodeIntelligence(file FileRecord, body string) ([]CodeFunctionRecord, *goFileScope) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.Path, body, 0)
	if err != nil {
		return nil, nil
	}
	var imports []goImportBinding
	for _, item := range parsed.Imports {
		value, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			continue
		}
		binding := goImportBinding{path: value}
		if item.Name != nil {
			binding.alias = item.Name.Name
		}
		imports = append(imports, binding)
	}
	fileScope := &goFileScope{path: file.Path, packageName: parsed.Name.Name, imports: imports, types: map[string]goTypeDeclaration{}, globals: map[string]*ast.Ident{}}
	var functions []CodeFunctionRecord
	for _, declaration := range parsed.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, specification := range group.Specs {
			switch item := specification.(type) {
			case *ast.TypeSpec:
				// Aliases and embedded fields need additional type checking.
				if item.Assign.IsValid() {
					continue
				}
				definition := goTypeDeclaration{file: fileScope, fields: map[string]ast.Expr{}}
				switch declaration := item.Type.(type) {
				case *ast.StructType:
					for _, field := range declaration.Fields.List {
						for _, name := range field.Names {
							definition.fields[name.Name] = field.Type
						}
					}
				case *ast.InterfaceType:
					for _, field := range declaration.Methods.List {
						signature, ok := field.Type.(*ast.FuncType)
						if !ok {
							continue
						}
						for _, name := range field.Names {
							scope := newGoFunctionScope(fileScope, signature)
							scope.interfaceMethod = true
							functions = append(functions, CodeFunctionRecord{Name: name.Name, Owner: item.Name.Name, Kind: "method", Language: "go", File: file.Path, Line: fset.Position(field.Pos()).Line, EndLine: fset.Position(field.End()).Line, goScope: scope})
						}
					}
				default:
					continue
				}
				fileScope.types[item.Name.Name] = definition
			case *ast.ValueSpec:
				for _, name := range item.Names {
					fileScope.globals[name.Name] = name
				}
			}
		}
	}
	lines := strings.Split(body, "\n")
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		scope := newGoFunctionScope(fileScope, function.Type)
		record := CodeFunctionRecord{Name: function.Name.Name, Kind: "function", Language: "go", File: file.Path, Line: fset.Position(function.Pos()).Line, EndLine: fset.Position(function.End()).Line, goScope: scope}
		if function.Recv != nil && len(function.Recv.List) > 0 {
			_, record.Owner = goTypeName(function.Recv.List[0].Type)
			record.Kind = "method"
		}
		if strings.HasSuffix(file.Path, "_test.go") && strings.HasPrefix(record.Name, "Test") {
			record.Kind = "test"
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			binding := &goCallScope{offset: fset.Position(call.Pos()).Offset, arguments: len(call.Args), ellipsis: call.Ellipsis.IsValid()}
			entry := CodeCallRecord{Line: fset.Position(call.Pos()).Line, goScope: binding}
			entry.Raw = strings.TrimSpace(stripCodeLineComment("go", lines[entry.Line-1]))
			switch target := unwrapGoExpression(call.Fun).(type) {
			case *ast.Ident:
				if target.Obj != nil && target.Obj.Kind == ast.Typ {
					return true
				}
				entry.Method = target.Name
				binding.dynamic = target.Obj != nil && target.Obj.Kind != ast.Fun
			case *ast.SelectorExpr:
				binding.receiver = target.X
				entry.Method = target.Sel.Name
				if receiver, ok := target.X.(*ast.Ident); ok {
					entry.Receiver = receiver.Name
					if receiver.Obj == nil {
						binding.importAlias = receiver.Name
					} else {
						binding.importAlias, binding.receiverType = goReceiverType(receiver)
						binding.receiverExpression = receiver.Obj.Kind == ast.Typ
						binding.dynamic = binding.receiverType == ""
					}
				} else {
					binding.importAlias, binding.receiverType = goTypeName(target.X)
					binding.receiverExpression = true
					binding.dynamic = binding.receiverType == ""
				}
			default:
				return true
			}
			if !isLowValueCallTarget(entry.Method) {
				record.Calls = append(record.Calls, entry)
			}
			return true
		})
		functions = append(functions, record)
	}
	return functions, fileScope
}

func unwrapGoExpression(expression ast.Expr) ast.Expr {
	for {
		switch value := expression.(type) {
		case *ast.ParenExpr:
			expression = value.X
		case *ast.IndexExpr:
			expression = value.X
		case *ast.IndexListExpr:
			expression = value.X
		default:
			return expression
		}
	}
}

func goPredeclaredType(name string) bool {
	switch name {
	case "any", "bool", "byte", "complex64", "complex128", "error", "float32", "float64", "int", "int8", "int16", "int32", "int64", "rune", "string", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return true
	}
	return false
}

func goTypeName(expression ast.Expr) (string, string) {
	switch value := unwrapGoExpression(expression).(type) {
	case *ast.Ident:
		return "", value.Name
	case *ast.StarExpr:
		return goTypeName(value.X)
	case *ast.SelectorExpr:
		if owner, ok := value.X.(*ast.Ident); ok && owner.Obj == nil {
			return owner.Name, value.Sel.Name
		}
	}
	return "", ""
}

func goReceiverType(receiver *ast.Ident) (string, string) {
	switch declaration := receiver.Obj.Decl.(type) {
	case *ast.Field:
		return goTypeName(declaration.Type)
	case *ast.TypeSpec:
		return "", declaration.Name.Name
	case *ast.ValueSpec:
		if declaration.Type != nil {
			return goTypeName(declaration.Type)
		}
		for index, name := range declaration.Names {
			if name.Name != receiver.Name || index >= len(declaration.Values) {
				continue
			}
			return goValueType(declaration.Values[index])
		}
	case *ast.AssignStmt:
		for index, left := range declaration.Lhs {
			name, ok := left.(*ast.Ident)
			if ok && name.Name == receiver.Name && index < len(declaration.Rhs) {
				return goValueType(declaration.Rhs[index])
			}
		}
	}
	return "", ""
}

func goValueType(expression ast.Expr) (string, string) {
	switch value := unwrapGoExpression(expression).(type) {
	case *ast.CompositeLit:
		return goTypeName(value.Type)
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			return goValueType(value.X)
		}
	case *ast.CallExpr:
		if name, ok := value.Fun.(*ast.Ident); ok && name.Name == "new" && name.Obj == nil && len(value.Args) == 1 {
			return goTypeName(value.Args[0])
		}
	}
	return "", ""
}

func bindGoCodePackages(code *CodeIntelligenceRecord, symbols []SymbolRecord) {
	modules := map[string]string{}
	for _, symbol := range symbols {
		if symbol.Kind == "module" && strings.HasPrefix(symbol.Name, "module ") && path.Base(symbol.File) == "go.mod" {
			modules[path.Dir(symbol.File)] = strings.TrimSpace(strings.TrimPrefix(symbol.Name, "module "))
		}
	}
	for _, file := range code.goFiles {
		if file == nil {
			continue
		}
		directory := path.Dir(file.path)
		for parent := directory; ; parent = path.Dir(parent) {
			if module, ok := modules[parent]; ok {
				suffix := strings.TrimPrefix(directory, parent+"/")
				if parent == "." {
					suffix = directory
				}
				file.importPath = module
				if suffix != "." && directory != parent {
					file.importPath += "/" + suffix
				}
				break
			}
			if parent == "." || parent == path.Dir(parent) {
				break
			}
		}
	}
	packages := goPackageIndex{files: map[string][]*goFileScope{}, imports: map[string]map[string]bool{}}
	for _, file := range code.goFiles {
		if file == nil {
			continue
		}
		key := file.key()
		packages.files[key] = append(packages.files[key], file)
		if file.importPath != "" {
			if packages.imports[file.importPath] == nil {
				packages.imports[file.importPath] = map[string]bool{}
			}
			packages.imports[file.importPath][key] = true
		}
	}
	for _, function := range code.Functions {
		if function.goScope == nil {
			continue
		}
		function.goScope.importPath = function.goScope.file.importPath
		for _, call := range function.Calls {
			if call.goScope == nil || call.goScope.receiver == nil {
				continue
			}
			reference, expression, ok := packages.receiver(function.goScope.file, call.goScope.receiver)
			if !ok {
				continue
			}
			call.goScope.dynamic = false
			call.goScope.importAlias = ""
			call.goScope.receiverPackage, call.goScope.receiverType = reference.packageKey, reference.name
			call.goScope.receiverExpression = expression
		}
	}
}

func resolveGoCodeCall(from CodeFunctionRecord, call CodeCallRecord, index codeFunctionIndex) (CodeFunctionRecord, bool) {
	target, reason := resolveGoCodeCallWithReason(from, call, index)
	return target, reason == ""
}

func resolveGoCodeCallWithReason(from CodeFunctionRecord, call CodeCallRecord, index codeFunctionIndex) (CodeFunctionRecord, string) {
	if from.goScope == nil || call.goScope == nil {
		return CodeFunctionRecord{}, "scope_unavailable"
	}
	if call.goScope.dynamic {
		if call.goScope.receiver == nil {
			return CodeFunctionRecord{}, "dynamic_function_value"
		}
		return CodeFunctionRecord{}, "receiver_type_unresolved"
	}
	arityMismatch := false
	var matches []CodeFunctionRecord
	for _, candidate := range index.byName[call.Method] {
		if candidate.Language != "go" || candidate.goScope == nil || candidate.Owner != call.goScope.receiverType {
			continue
		}
		local := path.Dir(candidate.File) == path.Dir(from.File) && candidate.goScope.packageName == from.goScope.packageName
		if call.goScope.receiverPackage != "" {
			local = candidate.goScope.file.key() == call.goScope.receiverPackage && (local || ast.IsExported(call.Method))
		} else if call.goScope.importAlias != "" {
			local = ast.IsExported(call.Method) && goImportedPackageMatches(from.goScope, call.goScope.importAlias, candidate.goScope)
		} else if !local && candidate.Owner == "" {
			local = ast.IsExported(call.Method) && goImportedPackageMatches(from.goScope, ".", candidate.goScope)
		}
		if local {
			if !goCallArityMatches(call.goScope, candidate.goScope) {
				arityMismatch = true
				continue
			}
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 1 {
		return matches[0], ""
	}
	if len(matches) > 1 {
		return CodeFunctionRecord{}, "ambiguous_target"
	}
	if arityMismatch {
		return CodeFunctionRecord{}, "argument_mismatch"
	}
	if call.goScope.importAlias != "" {
		return CodeFunctionRecord{}, "external_target"
	}
	if call.goScope.receiver != nil {
		return CodeFunctionRecord{}, "unsupported_type_binding"
	}
	return CodeFunctionRecord{}, "target_not_indexed"
}

func goImportedPackageMatches(from *goFunctionScope, alias string, candidate *goFunctionScope) bool {
	if candidate.importPath == "" {
		return false
	}
	for _, imported := range from.imports {
		if imported.path == candidate.importPath && (imported.alias == alias || imported.alias == "" && candidate.packageName == alias) {
			return true
		}
	}
	return false
}

func goCallArityMatches(call *goCallScope, target *goFunctionScope) bool {
	arguments := call.arguments
	if call.receiverExpression {
		arguments--
	}
	if target.variadic {
		return arguments >= target.parameters-1 && (!call.ellipsis || arguments == target.parameters)
	}
	return !call.ellipsis && arguments == target.parameters
}
