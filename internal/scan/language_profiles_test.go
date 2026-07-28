package scan

import (
	"reflect"
	"testing"
)

func TestLanguageCapabilityProfilesMatchImplementedAdapters(t *testing.T) {
	type expectedProfile struct {
		level                      string
		symbols, relations, calls  bool
		routes, tests              bool
		architecture               bool
		exactSymbols, directUsages bool
		httpProvider, httpConsumer bool
	}
	expected := map[string]expectedProfile{
		"java": {
			level: "full", symbols: true, relations: true, calls: true,
			routes: true, tests: true, architecture: true,
			exactSymbols: true, directUsages: true, httpProvider: true,
		},
		"javascript": {
			level: "full", symbols: true, relations: true, calls: true,
			routes: true, tests: true, architecture: true,
			exactSymbols: true, directUsages: true,
			httpProvider: true, httpConsumer: true,
		},
		"typescript": {
			level: "full", symbols: true, relations: true, calls: true,
			routes: true, tests: true, architecture: true,
			exactSymbols: true, directUsages: true,
			httpProvider: true, httpConsumer: true,
		},
		"go": {
			level: "full", symbols: true, relations: true, calls: true,
			routes: true, tests: true, architecture: true,
		},
		"php": {
			level: "full", symbols: true, relations: true, calls: true,
			routes: true, tests: true, architecture: true,
		},
		"rust": {
			level: "full", symbols: true, relations: true, calls: true,
			routes: true, tests: true, architecture: true,
		},
		"python": {
			level: "full", symbols: true, relations: true, calls: true,
			routes: true, tests: true, architecture: true,
		},
		"shell": {
			level: "partial", symbols: true, relations: true, calls: true,
		},
		"kotlin": {level: "index", symbols: true, relations: true},
		"scala":  {level: "index", symbols: true, relations: true},
		"swift":  {level: "index", symbols: true, relations: true},
		"ruby":   {level: "index", symbols: true, relations: true},
		"c":      {level: "index", symbols: true, relations: true},
		"cpp":    {level: "index", symbols: true, relations: true},
		"csharp": {level: "index", symbols: true, relations: true},
	}

	profiles := LanguageCapabilityProfiles()
	if len(profiles) != len(expected) {
		t.Fatalf("profile count = %d, want %d: %#v", len(profiles), len(expected), profiles)
	}
	languages := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		languages = append(languages, profile.Language)
		want, ok := expected[profile.Language]
		if !ok {
			t.Fatalf("unexpected language profile %#v", profile)
		}
		if profile.Level != want.level ||
			profile.Symbols != want.symbols ||
			profile.Relations != want.relations ||
			profile.Calls != want.calls ||
			profile.Routes != want.routes ||
			profile.Tests != want.tests ||
			profile.APIClients != want.architecture ||
			profile.Persistence != want.architecture ||
			profile.Messaging != want.architecture ||
			profile.DataFlow != want.architecture ||
			profile.ExactSymbols != want.exactSymbols ||
			profile.DirectUsages != want.directUsages ||
			profile.HTTPProvider != want.httpProvider ||
			profile.HTTPConsumer != want.httpConsumer {
			t.Fatalf("%s profile = %#v, want %#v", profile.Language, profile, want)
		}
		if profile.DisplayName == "" || profile.Scope == "" ||
			profile.Limitations == "" || len(profile.Outputs) == 0 {
			t.Fatalf("%s profile lacks public truth metadata: %#v", profile.Language, profile)
		}
		if want.architecture && len(profile.PatternFamilies) == 0 {
			t.Fatalf("%s profile lacks architecture pattern families", profile.Language)
		}
	}
	wantLanguages := []string{
		"c", "cpp", "csharp", "go", "java", "javascript", "kotlin", "php",
		"python", "ruby", "rust", "scala", "shell", "swift", "typescript",
	}
	if !reflect.DeepEqual(languages, wantLanguages) {
		t.Fatalf("profile order = %q, want %q", languages, wantLanguages)
	}
}

func TestAnalyzerInventoryProjectsLanguageProfiles(t *testing.T) {
	analyzers := analyzerCapabilities()
	for _, profile := range LanguageCapabilityProfiles() {
		analyzer, ok := analyzers[profile.Language]
		if !ok {
			t.Fatalf("missing analyzer for %s", profile.Language)
		}
		if analyzer.Scope != profile.Scope ||
			analyzer.Symbols != profile.Symbols ||
			analyzer.Relations != profile.Relations ||
			analyzer.Calls != profile.Calls ||
			analyzer.Endpoints != profile.Routes ||
			analyzer.Tests != profile.Tests ||
			!reflect.DeepEqual(analyzer.Outputs, profile.Outputs) {
			t.Fatalf("%s analyzer = %#v, profile = %#v", profile.Language, analyzer, profile)
		}
	}
}
