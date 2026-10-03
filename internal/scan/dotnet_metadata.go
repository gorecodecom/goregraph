package scan

import (
	"encoding/json"
	"encoding/xml"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

func extractDotnetMetadata(file FileRecord, body string) ProjectSymbolFacts {
	facts := ProjectSymbolFacts{}
	if strings.EqualFold(path.Ext(file.Path), ".csproj") {
		var project struct {
			XMLName xml.Name `xml:"Project"`
			Groups  []struct {
				AssemblyName string `xml:"AssemblyName"`
				Framework    string `xml:"TargetFramework"`
				Frameworks   string `xml:"TargetFrameworks"`
			} `xml:"PropertyGroup"`
			Items []struct {
				Project []struct {
					Include string `xml:"Include,attr"`
				} `xml:"ProjectReference"`
				Packages []struct {
					Include string `xml:"Include,attr"`
					Version string `xml:"Version,attr"`
				} `xml:"PackageReference"`
			} `xml:"ItemGroup"`
		}
		if xml.Unmarshal([]byte(body), &project) != nil || project.XMLName.Local != "Project" {
			return facts
		}
		name := strings.TrimSuffix(path.Base(file.Path), path.Ext(file.Path))
		for _, group := range project.Groups {
			if group.AssemblyName != "" {
				name = group.AssemblyName
			}
		}
		symbol := dotnetMetadataSymbol(file, name, "dotnet-project")
		facts.Declarations = append(facts.Declarations, symbol)
		for _, group := range project.Items {
			for _, reference := range group.Project {
				target := path.Clean(path.Join(path.Dir(file.Path), strings.ReplaceAll(reference.Include, "\\", "/")))
				facts.References = append(facts.References, dotnetMetadataReference(file, symbol, target, "uses_project"))
			}
			for _, reference := range group.Packages {
				facts.References = append(facts.References, dotnetMetadataReference(file, symbol, reference.Include, "uses_package"))
			}
		}
	}
	if path.Ext(file.Path) == ".asmdef" || path.Ext(file.Path) == ".asmref" {
		var assembly struct {
			Name       string   `json:"name"`
			Reference  string   `json:"reference"`
			References []string `json:"references"`
		}
		if json.Unmarshal([]byte(body), &assembly) != nil {
			return facts
		}
		name := assembly.Name
		if name == "" {
			name = path.Base(file.Path)
		}
		symbol := dotnetMetadataSymbol(file, name, "unity-assembly")
		facts.Declarations = append(facts.Declarations, symbol)
		for _, reference := range append(assembly.References, assembly.Reference) {
			if reference != "" {
				facts.References = append(facts.References, dotnetMetadataReference(file, symbol, reference, "uses_assembly"))
			}
		}
	}
	return facts
}
func dotnetMetadataSymbol(file FileRecord, name, analyzer string) RichSymbolRecord {
	return RichSymbolRecord{ID: stableID("dotnet-metadata", file.Path, name), Name: name, QualifiedName: file.Path, Kind: "assembly", Language: "csharp", File: file.Path, Line: 1, SourceLocation: sourceLocation(1), Analyzer: analyzer, Confidence: ConfidenceExact, Coverage: CoveragePartial, Limitations: []string{"literal project metadata only; MSBuild conditions, generated code and assembly loading are not evaluated"}}
}
func dotnetMetadataReference(file FileRecord, from RichSymbolRecord, target, kind string) RichRelationRecord {
	return RichRelationRecord{ID: stableID("dotnet-ref", from.ID, kind, target), From: file.Path, To: target, Type: kind, Language: "csharp", Analyzer: from.Analyzer, FromSymbolID: from.ID, TargetQualifiedName: target, Line: 1, SourceLocation: sourceLocation(1), Resolution: SymbolResolutionUnresolved, NonPromotable: true, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "literal dependency declaration; referenced project or package is not resolved or executed"}
}

func resolveDotnetMetadata(facts ProjectSymbolFacts, inventories ...[]unitySource) ProjectSymbolFacts {
	facts.References = append([]RichRelationRecord(nil), facts.References...)
	byFile := map[string]RichSymbolRecord{}
	byName := map[string][]RichSymbolRecord{}
	byGUID := map[string][]RichSymbolRecord{}
	guidOwners := map[string]int{}
	for _, symbol := range facts.Declarations {
		byFile[symbol.File] = symbol
		if strings.HasSuffix(symbol.File, ".asmdef") {
			byName[symbol.Name] = append(byName[symbol.Name], symbol)
		}
	}
	for _, inventory := range inventories {
		for _, source := range inventory {
			if !strings.HasSuffix(source.file.Path, ".meta") {
				continue
			}
			var meta struct {
				GUID string `yaml:"guid"`
			}
			if yaml.Unmarshal([]byte(source.body), &meta) != nil || !unityGUID.MatchString(meta.GUID) {
				continue
			}
			guidOwners[strings.ToLower(meta.GUID)]++
			if symbol, ok := byFile[strings.TrimSuffix(source.file.Path, ".meta")]; ok {
				guid := strings.ToLower(meta.GUID)
				byGUID[guid] = append(byGUID[guid], symbol)
			}
		}
	}
	for i := range facts.References {
		ref := &facts.References[i]
		var candidates []RichSymbolRecord
		if ref.Type == "uses_project" {
			if symbol, ok := byFile[ref.To]; ok {
				candidates = append(candidates, symbol)
			}
		}
		if ref.Type == "uses_assembly" {
			if strings.HasPrefix(ref.To, "GUID:") {
				guid := strings.ToLower(strings.TrimPrefix(ref.To, "GUID:"))
				candidates = byGUID[guid]
				if guidOwners[guid] > 1 {
					ref.Resolution = SymbolResolutionAmbiguous
					ref.Reason = "Unity GUID has multiple indexed metadata owners"
					for _, candidate := range candidates {
						ref.CandidateSymbolIDs = append(ref.CandidateSymbolIDs, candidate.ID)
					}
					continue
				}
			} else {
				candidates = byName[ref.To]
			}
		}
		if len(candidates) == 1 {
			target := candidates[0]
			ref.ToSymbolID = target.ID
			ref.To = target.File
			ref.TargetQualifiedName = target.QualifiedName
			ref.Internal = true
			ref.NonPromotable = false
			ref.Resolution = SymbolResolutionExact
			ref.Reason = "unique indexed dependency metadata"
		} else if len(candidates) > 1 {
			ref.Resolution = SymbolResolutionAmbiguous
			for _, candidate := range candidates {
				ref.CandidateSymbolIDs = append(ref.CandidateSymbolIDs, candidate.ID)
			}
		}
	}
	return facts
}
