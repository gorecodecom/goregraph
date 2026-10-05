package scan

import "strings"

func cGoogleTestMembers(s *cFamilySource) {
	imported := false
	for _, header := range s.includes {
		if header == "gtest/gtest.h" {
			imported = true
		}
	}
	if !imported || !s.valid {
		return
	}
	t := s.tokens
	for at := 0; at+7 < len(t); at++ {
		if t[at].text != "TEST" && t[at].text != "TEST_F" {
			continue
		}
		if t[at+1].text != "(" || t[at+2].kind != "identifier" || t[at+3].text != "," || t[at+4].kind != "identifier" || t[at+5].text != ")" || t[at+6].text != "{" {
			continue
		}
		end, ok := s.pairs[at+6]
		if !ok {
			continue
		}
		kind := "test"
		if strings.HasPrefix(t[at+2].text, "DISABLED_") || strings.HasPrefix(t[at+4].text, "DISABLED_") {
			kind = "test_disabled"
		}
		symbol := supplementaryTokenSymbol(s.file, kind, t[at+4].text, t[at+4])
		symbol.Owner = t[at+2].text
		s.members = append(s.members, dartMember{symbol: symbol, start: at + 6, end: end})
		at = end
	}
}

func objcXCTestMethod(s objcSource, m objcMethod, visible map[string]bool, files map[string]objcSource) bool {
	if m.static || m.returnType != "void" || len(m.parameters) > 0 || !strings.HasPrefix(m.symbol.Name, "test") || strings.Contains(m.symbol.Name, ":") {
		return false
	}
	imported := false
	for file := range visible {
		for _, uri := range files[file].imports {
			if strings.Trim(uri, "<>") == "XCTest/XCTest.h" {
				imported = true
			}
		}
	}
	if !imported {
		return false
	}
	owner := m.symbol.Owner
	seen := map[string]bool{}
	for owner != "" && !seen[owner] {
		seen[owner] = true
		owner = objcSuperclass(owner, visible, files)
		if owner == "XCTestCase" {
			return true
		}
	}
	return false
}

func nativeTestCapabilities(code CodeIntelligenceRecord, language, framework string) []ArchitectureCapabilityFact {
	var facts []ArchitectureCapabilityFact
	for _, f := range code.Functions {
		if f.Language == language && f.Kind == "test" {
			facts = append(facts, ArchitectureCapabilityFact{ID: stableID("native-test", f.File, sourceLocation(f.Line), f.Name), Language: language, Capability: CapabilityTests, Kind: "unit_test", Framework: framework, File: f.File, Line: f.Line})
		}
	}
	return facts
}
