package scan

func analyzeSupplementarySources(sources []supplementarySource, files []FileRecord, workspaces ...WorkspaceIndex) supplementaryAnalysis {
	var result supplementaryAnalysis
	var objc []objcSource
	var cSources []cFamilySource
	var rubySources []rubySource
	var kotlinSources []kotlinSource
	for _, source := range sources {
		switch source.file.Language {
		case "html", "css":
			analyzeMarkupSource(source, &result)
		case "batch":
			analyzeBatchSource(source, &result)
		case "ruby":
			rubySources = append(rubySources, parseRubySource(source.file, source.body))
		case "kotlin":
			kotlinSources = append(kotlinSources, parseKotlinSource(source.file, source.body))
		case "c", "cpp":
			cSources = append(cSources, parseCFamilySource(source.file, source.body))
		case "objectivec":
			objc = append(objc, parseObjCSource(source.file, source.body))
		}
	}
	analyzeObjCSources(objc, &result)
	analyzeCFamilySources(cSources, &result)
	analyzeRubySources(rubySources, &result)
	if len(workspaces) > 0 {
		assignKotlinModules(kotlinSources, workspaces[0])
	}
	analyzeKotlinSources(kotlinSources, &result)
	result.capabilities = append(result.capabilities, kotlinCapabilities(result.code)...)
	result.capabilities = append(result.capabilities, nativeTestCapabilities(result.code, "cpp", "GoogleTest")...)
	result.capabilities = append(result.capabilities, nativeTestCapabilities(result.code, "objectivec", "XCTest")...)
	result.capabilities = append(result.capabilities, nativeTestCapabilities(result.code, "ruby", "Minitest")...)
	resolveSupplementaryResources(sources, files, &result.facts)
	for _, contract := range result.code.APIContracts {
		if contract.Language != "html" {
			continue
		}
		result.capabilities = append(result.capabilities, ArchitectureCapabilityFact{ID: stableID("markup-http", contract.File, sourceLocation(contract.Line)), Language: contract.Language, Capability: CapabilityAPIClients, Kind: "form_action", Framework: "HTML", File: contract.File, Line: contract.Line})
	}
	return result
}

func kotlinCapabilities(code CodeIntelligenceRecord) []ArchitectureCapabilityFact {
	var filtered CodeIntelligenceRecord
	for _, f := range code.Functions {
		if f.Language == "kotlin" {
			filtered.Functions = append(filtered.Functions, f)
		}
	}
	for _, r := range code.Routes {
		if r.Language == "kotlin" {
			filtered.Routes = append(filtered.Routes, r)
		}
	}
	for _, c := range code.APIContracts {
		if c.Language == "kotlin" {
			filtered.APIContracts = append(filtered.APIContracts, c)
		}
	}
	return languageCodeCapabilities("kotlin", "Kotlin / Spring / Ktor", filtered)
}
