package scan

import "testing"

func assertRichDeclaration(t *testing.T, records []RichSymbolRecord, kind, qualifiedName, artifact string) RichSymbolRecord {
	t.Helper()
	for _, record := range records {
		if record.Kind == kind && record.QualifiedName == qualifiedName && record.Artifact == artifact {
			return record
		}
	}
	t.Fatalf("missing %s declaration %s in %#v", kind, qualifiedName, records)
	return RichSymbolRecord{}
}

func assertScriptReference(t *testing.T, records []RichRelationRecord, kind, module, exportName string) RichRelationRecord {
	t.Helper()
	for _, record := range records {
		if record.Type == kind && record.TargetModule == module && record.TargetExport == exportName {
			return record
		}
	}
	t.Fatalf("missing %s reference %s#%s in %#v", kind, module, exportName, records)
	return RichRelationRecord{}
}
