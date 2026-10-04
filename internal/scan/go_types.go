package scan

import (
	"go/ast"
	"path"
)

type goFileScope struct {
	path, packageName, importPath string
	imports                       []goImportBinding
	types                         map[string]goTypeDeclaration
	globals                       map[string]*ast.Ident
}

type goTypeDeclaration struct {
	file   *goFileScope
	fields map[string]ast.Expr
}

type goTypeReference struct {
	packageKey, name string
}

type goPackageIndex struct {
	files   map[string][]*goFileScope
	imports map[string]map[string]bool
}

func (file *goFileScope) key() string { return path.Dir(file.path) + "#" + file.packageName }

func newGoFunctionScope(file *goFileScope, signature *ast.FuncType) *goFunctionScope {
	scope := &goFunctionScope{file: file, packageName: file.packageName, imports: file.imports, parameters: signature.Params.NumFields()}
	if parameters := signature.Params.List; len(parameters) > 0 {
		_, scope.variadic = parameters[len(parameters)-1].Type.(*ast.Ellipsis)
	}
	return scope
}

func (index goPackageIndex) declaration(reference goTypeReference) (goTypeDeclaration, bool) {
	var found []goTypeDeclaration
	for _, file := range index.files[reference.packageKey] {
		if definition, ok := file.types[reference.name]; ok {
			found = append(found, definition)
		}
	}
	if len(found) != 1 {
		return goTypeDeclaration{}, false
	}
	return found[0], true
}

func (index goPackageIndex) typeReference(file *goFileScope, alias, name string) (goTypeReference, bool) {
	if alias == "" {
		reference := goTypeReference{packageKey: file.key(), name: name}
		_, ok := index.declaration(reference)
		return reference, ok
	}
	if !ast.IsExported(name) {
		return goTypeReference{}, false
	}
	var matches []goTypeReference
	for _, imported := range file.imports {
		for key := range index.imports[imported.path] {
			files := index.files[key]
			if imported.alias != alias && !(imported.alias == "" && len(files) > 0 && files[0].packageName == alias) {
				continue
			}
			reference := goTypeReference{packageKey: key, name: name}
			if _, ok := index.declaration(reference); ok {
				matches = append(matches, reference)
			}
		}
	}
	if len(matches) != 1 {
		return goTypeReference{}, false
	}
	return matches[0], true
}

func (index goPackageIndex) declaredType(file *goFileScope, expression ast.Expr) (goTypeReference, bool) {
	alias, name := goTypeName(expression)
	return index.typeReference(file, alias, name)
}

func (index goPackageIndex) receiver(file *goFileScope, expression ast.Expr) (goTypeReference, bool, bool) {
	switch value := unwrapGoExpression(expression).(type) {
	case *ast.Ident:
		if value.Obj != nil {
			alias, name := goReceiverType(value)
			reference, ok := index.typeReference(file, alias, name)
			return reference, value.Obj.Kind == ast.Typ, ok
		}
		var globals []struct {
			file *goFileScope
			name *ast.Ident
		}
		for _, candidate := range index.files[file.key()] {
			if global := candidate.globals[value.Name]; global != nil {
				globals = append(globals, struct {
					file *goFileScope
					name *ast.Ident
				}{candidate, global})
			}
		}
		if len(globals) == 1 {
			alias, name := goReceiverType(globals[0].name)
			reference, ok := index.typeReference(globals[0].file, alias, name)
			return reference, false, ok
		}
		if len(globals) > 1 {
			return goTypeReference{}, false, false
		}
		reference, ok := index.typeReference(file, "", value.Name)
		return reference, true, ok
	case *ast.SelectorExpr:
		owner, methodExpression, ok := index.receiver(file, value.X)
		if ok && !methodExpression {
			definition, ok := index.declaration(owner)
			if !ok || owner.packageKey != file.key() && !ast.IsExported(value.Sel.Name) {
				return goTypeReference{}, false, false
			}
			field := definition.fields[value.Sel.Name]
			if field == nil {
				return goTypeReference{}, false, false
			}
			reference, ok := index.declaredType(definition.file, field)
			return reference, false, ok
		}
		alias, name := goTypeName(value)
		reference, ok := index.typeReference(file, alias, name)
		return reference, true, ok
	case *ast.StarExpr:
		// A pointer type denotes a method expression; dereferencing a variable
		// preserves its receiver type without adding an explicit receiver argument.
		if reference, ok := index.declaredType(file, value); ok {
			return reference, true, true
		}
		return index.receiver(file, value.X)
	case *ast.CompositeLit:
		reference, ok := index.declaredType(file, value.Type)
		return reference, false, ok
	case *ast.UnaryExpr:
		return index.receiver(file, value.X)
	}
	return goTypeReference{}, false, false
}
