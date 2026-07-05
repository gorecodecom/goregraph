package scan

import "strings"

func resolveJavaImportRelations(sources []JavaSourceRecord) []RelationRecord {
	typesByQualifiedName := map[string]JavaTypeRecord{}
	for _, source := range sources {
		for _, typ := range source.Types {
			qualified := typ.Name
			if typ.Package != "" {
				qualified = typ.Package + "." + typ.Name
			}
			typesByQualifiedName[qualified] = typ
		}
	}

	var relations []RelationRecord
	for _, source := range sources {
		for _, imported := range source.Imports {
			if typ, ok := typesByQualifiedName[imported.Name]; ok {
				relations = append(relations, RelationRecord{From: source.File, To: typ.File, Type: "imports_internal", Line: imported.Line})
				continue
			}
			relations = append(relations, RelationRecord{From: source.File, To: imported.Name, Type: "imports_external", Line: imported.Line})
		}
	}
	return relations
}

func replaceJavaImportRelations(existing []RelationRecord, javaRelations []RelationRecord) []RelationRecord {
	result := make([]RelationRecord, 0, len(existing)+len(javaRelations))
	for _, relation := range existing {
		if relation.Type == "imports" && strings.HasSuffix(relation.From, ".java") {
			continue
		}
		result = append(result, relation)
	}
	return append(result, javaRelations...)
}

func javaTestRelations(sources []JavaSourceRecord) []RelationRecord {
	productionByType := map[string]JavaTypeRecord{}
	for _, source := range sources {
		if isJavaTestPath(source.File) {
			continue
		}
		for _, typ := range source.Types {
			productionByType[typ.Name] = typ
		}
	}

	var relations []RelationRecord
	for _, source := range sources {
		if !isJavaTestPath(source.File) {
			continue
		}
		for _, typ := range source.Types {
			name := strings.TrimSuffix(typ.Name, "Test")
			if target, ok := productionByType[name]; ok {
				relations = append(relations, RelationRecord{From: source.File, To: target.File, Type: "tests", Line: typ.Line})
			}
		}
	}
	return relations
}

func isJavaTestPath(path string) bool {
	return strings.Contains(path, "/src/test/") || strings.HasPrefix(path, "src/test/")
}
