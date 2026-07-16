package scan

import (
	"sort"
	"strconv"
	"strings"
)

type ProjectSymbolFacts struct {
	Declarations []RichSymbolRecord
	References   []RichRelationRecord
}

func MergeProjectSymbolFacts(target *ProjectSymbolFacts, next ProjectSymbolFacts) {
	if target == nil {
		return
	}
	target.Declarations = append(target.Declarations, next.Declarations...)
	target.References = append(target.References, next.References...)
}

func FinalizeProjectSymbolFacts(_ []FileRecord, workspace WorkspaceIndex, facts ProjectSymbolFacts) ProjectSymbolFacts {
	replacedIDs := map[string]string{}
	for index := range facts.Declarations {
		declaration := &facts.Declarations[index]
		oldID := declaration.ID
		if declaration.Language == "java" {
			provenance := javaSourceProvenance(declaration.File, workspace)
			declaration.Artifact = provenance.artifact
			declaration.Coverage = provenance.coverage
			declaration.Limitations = append([]string(nil), provenance.limitations...)
			declaration.ID = StableWorkspaceSymbolID(declaration.Kind, "", declaration.Artifact, declaration.Language, declaration.QualifiedName, declaration.File)
		} else if declaration.Language == "javascript" || declaration.Language == "typescript" {
			if pkg, ok := nearestNodePackage(declaration.File, workspace.NodePackages); ok {
				declaration.WorkspacePackage = pkg.Name
			}
		}
		declaration.DeclarationID = declaration.ID
		replacedIDs[oldID] = declaration.ID
	}
	byQualified := map[string][]RichSymbolRecord{}
	for _, declaration := range facts.Declarations {
		byQualified[declaration.QualifiedName] = append(byQualified[declaration.QualifiedName], declaration)
	}
	for index := range facts.References {
		reference := &facts.References[index]
		if replacement := replacedIDs[reference.FromSymbolID]; replacement != "" {
			reference.FromSymbolID = replacement
		}
		if replacement := replacedIDs[reference.ToSymbolID]; replacement != "" {
			reference.ToSymbolID = replacement
		}
		candidates := byQualified[reference.TargetQualifiedName]
		if len(candidates) == 1 && !reference.preventExact {
			reference.ToSymbolID = candidates[0].ID
			reference.Resolution = SymbolResolutionExact
			reference.Confidence = string(ConfidenceExact)
			reference.ConfidenceScore = 1
			reference.Internal = true
			reference.CandidateSymbolIDs = nil
		} else if len(candidates) > 1 && !reference.preventExact {
			reference.ToSymbolID = ""
			reference.Resolution = SymbolResolutionAmbiguous
			reference.Confidence = string(ConfidenceNormalized)
			reference.ConfidenceScore = javaFactConfidenceScore(ConfidenceNormalized)
			reference.CandidateSymbolIDs = reference.CandidateSymbolIDs[:0]
			for _, candidate := range candidates {
				reference.CandidateSymbolIDs = append(reference.CandidateSymbolIDs, candidate.ID)
			}
			sort.Strings(reference.CandidateSymbolIDs)
		}
		if reference.Language == "java" {
			provenance := javaSourceProvenance(reference.From, workspace)
			reference.DependencyEvidence = javaGradleDependencyEvidence(provenance.artifact, reference.TargetQualifiedName, provenance.gradleDeps)
		}
		category := SymbolUsageDirectReference
		targetIdentity := reference.ToSymbolID
		if reference.Resolution == SymbolResolutionExact && targetIdentity == "" {
			targetIdentity = reference.TargetQualifiedName
		}
		if reference.Resolution == SymbolResolutionUnresolved {
			category = SymbolUsageUnresolved
			targetIdentity = reference.TargetQualifiedName
		} else if reference.Resolution == SymbolResolutionAmbiguous {
			category = SymbolUsageAmbiguous
			targetIdentity = strings.Join(reference.CandidateSymbolIDs, ",")
		}
		reference.ID = StableWorkspaceUsageID(reference.ToSymbolID, "", reference.FromSymbolID, category, reference.Type, targetIdentity, reference.From, reference.Line)
	}
	facts.Declarations = dedupeRichSymbolFacts(facts.Declarations)
	facts.References = dedupeRichRelationFacts(facts.References)
	return facts
}

func linkCallGraphSymbolFacts(graph *CallGraphRecord, facts ProjectSymbolFacts) {
	if graph == nil {
		return
	}
	callFacts := map[string]RichRelationRecord{}
	for _, reference := range facts.References {
		switch reference.Type {
		case "calls_method_owner":
			callFacts[reference.From+"\x00"+reference.TargetQualifiedName+"\x00"+lineKey(reference.Line)] = reference
		case "calls_export", "calls_local", "renders_component":
			if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" {
				continue
			}
			callFacts[reference.From+"\x00"+scriptQualifiedSimpleName(reference.TargetQualifiedName)+"\x00"+lineKey(reference.Line)] = reference
		}
	}
	qualifiedBySimple := map[string][]RichSymbolRecord{}
	declarationByID := map[string]RichSymbolRecord{}
	for _, declaration := range facts.Declarations {
		qualifiedBySimple[declaration.Name] = append(qualifiedBySimple[declaration.Name], declaration)
		declarationByID[declaration.ID] = declaration
	}
	for index := range graph.Edges {
		edge := &graph.Edges[index]
		candidates := qualifiedBySimple[edge.To.Owner]
		for _, candidate := range candidates {
			key := edge.SourceFile + "\x00" + candidate.QualifiedName + "\x00" + lineKey(edge.Line)
			reference, ok := callFacts[key]
			if !ok {
				continue
			}
			edge.FromSymbolID = reference.FromSymbolID
			edge.ToSymbolID = reference.ToSymbolID
			edge.TargetQualifiedName = reference.TargetQualifiedName
			edge.Resolution = reference.Resolution
			edge.CandidateSymbolIDs = append([]string(nil), reference.CandidateSymbolIDs...)
			break
		}
		key := edge.SourceFile + "\x00" + edge.To.Method + "\x00" + lineKey(edge.Line)
		if reference, ok := callFacts[key]; ok {
			target := declarationByID[reference.ToSymbolID]
			if target.ID == "" || target.File != edge.To.File || target.Name != edge.To.Method {
				continue
			}
			edge.FromSymbolID = reference.FromSymbolID
			edge.ToSymbolID = reference.ToSymbolID
			edge.TargetQualifiedName = reference.TargetQualifiedName
			edge.Resolution = reference.Resolution
			edge.CandidateSymbolIDs = append([]string(nil), reference.CandidateSymbolIDs...)
		}
	}
}

func scriptQualifiedSimpleName(qualified string) string {
	if index := strings.LastIndex(qualified, "#"); index >= 0 {
		return qualified[index+1:]
	}
	return qualified
}

func lineKey(line int) string {
	return strconv.Itoa(line)
}

func dedupeRichSymbolFacts(records []RichSymbolRecord) []RichSymbolRecord {
	byID := make(map[string]RichSymbolRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	result := make([]RichSymbolRecord, 0, len(byID))
	for _, record := range byID {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func dedupeRichRelationFacts(records []RichRelationRecord) []RichRelationRecord {
	byID := make(map[string]RichRelationRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	result := make([]RichRelationRecord, 0, len(byID))
	for _, record := range byID {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
