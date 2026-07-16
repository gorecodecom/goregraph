package scan

import (
	"sort"
	"strings"
	"testing"
)

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

func TestFinalizeProjectSymbolFactsUsesResolutionSpecificTargetIdentity(t *testing.T) {
	provider := RichSymbolRecord{ID: "provider", Kind: "class", Language: "java", QualifiedName: "com.weka.UserService", File: "UserService.java"}
	second := RichSymbolRecord{ID: "second", Kind: "class", Language: "java", QualifiedName: "com.weka.Duplicate", File: "one/Duplicate.java"}
	third := RichSymbolRecord{ID: "third", Kind: "class", Language: "java", QualifiedName: "com.weka.Duplicate", File: "two/Duplicate.java"}
	references := []RichRelationRecord{
		{ID: "old-exact", From: "Consumer.java", FromSymbolID: "consumer", Type: "field_type", TargetQualifiedName: "com.weka.UserService", Line: 3, Resolution: SymbolResolutionUnresolved},
		{ID: "old-ambiguous", From: "Consumer.java", FromSymbolID: "consumer", Type: "field_type", TargetQualifiedName: "com.weka.Duplicate", Line: 4, Resolution: SymbolResolutionUnresolved},
		{ID: "old-unresolved", From: "Consumer.java", FromSymbolID: "consumer", Type: "field_type", TargetQualifiedName: "com.external.Missing", Line: 5, Resolution: SymbolResolutionUnresolved},
	}
	facts := FinalizeProjectSymbolFacts(nil, WorkspaceIndex{}, ProjectSymbolFacts{
		Declarations: []RichSymbolRecord{provider, second, third},
		References:   references,
	})
	byLine := map[int]RichRelationRecord{}
	for _, reference := range facts.References {
		byLine[reference.Line] = reference
	}
	exact := byLine[3]
	wantExact := StableWorkspaceUsageID(exact.ToSymbolID, "", exact.FromSymbolID, SymbolUsageDirectReference, exact.Type, exact.ToSymbolID, exact.From, exact.Line)
	if exact.ID != wantExact {
		t.Fatalf("exact usage ID = %q, want provider target identity %q", exact.ID, wantExact)
	}
	ambiguous := byLine[4]
	candidates := append([]string(nil), ambiguous.CandidateSymbolIDs...)
	sort.Strings(candidates)
	wantAmbiguous := StableWorkspaceUsageID("", "", ambiguous.FromSymbolID, SymbolUsageAmbiguous, ambiguous.Type, strings.Join(candidates, ","), ambiguous.From, ambiguous.Line)
	if ambiguous.ID != wantAmbiguous {
		t.Fatalf("ambiguous usage ID = %q, want candidate identity %q", ambiguous.ID, wantAmbiguous)
	}
	unresolved := byLine[5]
	wantUnresolved := StableWorkspaceUsageID("", "", unresolved.FromSymbolID, SymbolUsageUnresolved, unresolved.Type, unresolved.TargetQualifiedName, unresolved.From, unresolved.Line)
	if unresolved.ID != wantUnresolved {
		t.Fatalf("unresolved usage ID = %q, want canonical target identity %q", unresolved.ID, wantUnresolved)
	}
}
