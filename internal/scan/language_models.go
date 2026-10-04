package scan

import "strings"

func csharpEntityReferences(s csharpSource, sources []csharpSource, types map[string][]RichSymbolRecord) []RichRelationRecord {
	var result []RichRelationRecord
	for _, member := range s.members {
		if member.symbol.Kind != "property" && member.symbol.Kind != "field" {
			continue
		}
		at := strings.Index(member.typeName, "<")
		if at < 0 || !strings.HasSuffix(member.typeName, ">") || strings.Contains(member.typeName[at+1:], ",") {
			continue
		}
		if !csharpFrameworkType(s, member.owner, member.typeName[:at], "Microsoft.EntityFrameworkCore", "DbSet", sources, types, map[string]bool{}) {
			continue
		}
		result = append(result, csharpTypeReference(s, member.symbol, member.typeName[at+1:len(member.typeName)-1], "uses_entity", types))
	}
	return result
}

func swiftModelReferences(s swiftSource, types map[string][]RichSymbolRecord) []RichRelationRecord {
	if !hasSwiftImport(s, "SwiftData") {
		return nil
	}
	var result []RichRelationRecord
	for _, member := range s.members {
		query := false
		for _, attribute := range member.attributes {
			if attribute == "Query" {
				query = true
			}
		}
		if !query {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(member.typeName, "["), "]")
		if name != "" {
			result = append(result, swiftTypeReference(s, member.symbol, name, "uses_model", types))
		}
	}
	return result
}
