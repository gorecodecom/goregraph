package scan

func languageCodeCapabilities(language, framework string, code CodeIntelligenceRecord) []ArchitectureCapabilityFact {
	var result []ArchitectureCapabilityFact
	add := func(capability CapabilityID, kind, file string, line int) {
		result = append(result, ArchitectureCapabilityFact{ID: stableID("source-capability", language, string(capability), kind, file, sourceLocation(line)), Language: language, Capability: capability, Kind: kind, Framework: framework, File: file, Line: line})
	}
	for _, route := range code.Routes {
		add(CapabilityRoutes, "http_route", route.File, route.Line)
	}
	for _, request := range code.APIContracts {
		add(CapabilityAPIClients, "http_client", request.File, request.Line)
	}
	for _, function := range code.Functions {
		if function.Kind == "test" {
			add(CapabilityTests, "test_declaration", function.File, function.Line)
		}
	}
	return result
}
