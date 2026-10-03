package scan

import "strings"

func swiftInheritedMembers(owner, name string, members map[string][]swiftMember, sources []swiftSource, types map[string][]RichSymbolRecord, visited map[string]bool) []swiftMember {
	if owner == "" || visited[owner] {
		return nil
	}
	visited[owner] = true
	if declared := members[owner+"."+name]; len(declared) > 0 {
		return declared
	}
	var result []swiftMember
	for _, s := range sources {
		for _, typ := range s.types {
			if typ.symbol.QualifiedName != owner || len(typ.symbol.Limitations) != 1 {
				continue
			}
			for _, base := range typ.bases {
				for _, target := range swiftFindTypes(s, base, types) {
					result = append(result, swiftInheritedMembers(target.QualifiedName, name, members, sources, types, visited)...)
				}
			}
		}
	}
	return result
}

func swiftSuperOwner(s swiftSource, owner string, types map[string][]RichSymbolRecord) string {
	for _, typ := range s.types {
		if typ.symbol.QualifiedName != owner {
			continue
		}
		for _, base := range typ.bases {
			if candidates := swiftFindTypes(s, strings.TrimSpace(base), types); len(candidates) == 1 && candidates[0].Kind == "class" {
				return candidates[0].QualifiedName
			}
		}
	}
	return ""
}
