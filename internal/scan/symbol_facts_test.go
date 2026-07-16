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

func TestFinalizeProjectSymbolFactsPreservesExactScriptCapabilityProvider(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/provider.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { Dual } from "./provider";
export function App() {
  const value: Dual = input;
  Dual();
}
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `
export interface Dual {}
export function Dual() {}
`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	resolvedType := assertScriptReference(t, resolved.References, "type_reference", "./provider", "Dual")
	resolvedValue := assertScriptReference(t, resolved.References, "calls_export", "./provider", "Dual")
	if resolvedType.Resolution != SymbolResolutionExact || resolvedValue.Resolution != SymbolResolutionExact ||
		resolvedType.ToSymbolID == "" || resolvedValue.ToSymbolID == "" || resolvedType.ToSymbolID == resolvedValue.ToSymbolID {
		t.Fatalf("capability resolver did not select distinct exact providers: type=%#v value=%#v", resolvedType, resolvedValue)
	}

	finalized := FinalizeProjectSymbolFacts(files, WorkspaceIndex{}, resolved)
	finalType := assertScriptReference(t, finalized.References, "type_reference", "./provider", "Dual")
	finalValue := assertScriptReference(t, finalized.References, "calls_export", "./provider", "Dual")
	if finalType.Resolution != SymbolResolutionExact || finalType.ToSymbolID != resolvedType.ToSymbolID ||
		finalValue.Resolution != SymbolResolutionExact || finalValue.ToSymbolID != resolvedValue.ToSymbolID {
		t.Fatalf("finalization changed exact capability providers: before type=%#v value=%#v; after type=%#v value=%#v",
			resolvedType, resolvedValue, finalType, finalValue)
	}
}
