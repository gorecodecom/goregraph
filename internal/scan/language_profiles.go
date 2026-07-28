package scan

import "slices"

// LanguageCapabilityProfile describes the implemented static-analysis depth of
// one source-language adapter.
type LanguageCapabilityProfile struct {
	Language        string
	DisplayName     string
	Level           string
	Scope           string
	Symbols         bool
	Relations       bool
	Calls           bool
	Routes          bool
	Tests           bool
	APIClients      bool
	Persistence     bool
	Messaging       bool
	DataFlow        bool
	ExactSymbols    bool
	DirectUsages    bool
	HTTPProvider    bool
	HTTPConsumer    bool
	PatternFamilies []string
	Limitations     string
	Outputs         []string
}

var sourceLanguageCapabilityProfiles = []LanguageCapabilityProfile{
	{
		Language: "c", DisplayName: "C", Level: "index", Scope: "language",
		Symbols: true, Relations: true,
		Limitations: indexAdapterLimitations,
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json"},
	},
	{
		Language: "cpp", DisplayName: "C++", Level: "index", Scope: "language",
		Symbols: true, Relations: true,
		Limitations: indexAdapterLimitations,
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json"},
	},
	{
		Language: "csharp", DisplayName: "C#", Level: "index", Scope: "language",
		Symbols: true, Relations: true,
		Limitations: indexAdapterLimitations,
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json"},
	},
	{
		Language: "go", DisplayName: "Go", Level: "full", Scope: "language+routes",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		PatternFamilies: []string{
			"net/http and common routers", "net/http clients", "database/sql and GORM",
			"Kafka and AMQP", "gRPC", "JSON request/response boundaries", "go test and httptest",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols.json", "relations.json", "callgraph.json", "routes.json",
			"flows.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "java", DisplayName: "Java / Spring", Level: "full", Scope: "language+spring",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		ExactSymbols: true, DirectUsages: true, HTTPProvider: true,
		PatternFamilies: []string{
			"Spring MVC and WebFlux", "Java and Spring HTTP clients", "Spring Data",
			"Spring Messaging", "gRPC", "Jakarta Validation", "JUnit and Spring Test",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "spring.json", "callgraph.json",
			"endpoint-flows.json", "test-map.json",
		},
	},
	{
		Language: "javascript", DisplayName: "JavaScript / Node.js / React", Level: "full", Scope: "language+routes+api",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		ExactSymbols: true, DirectUsages: true, HTTPProvider: true, HTTPConsumer: true,
		PatternFamilies: []string{
			"Express and Fastify", "NestJS", "Next.js", "Web and Node HTTP clients",
			"common Node persistence", "Kafka and AMQP", "gRPC",
			"Node request/response boundaries", "Jest, Vitest, Node Test, and React Testing Library",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "api-contracts.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "kotlin", DisplayName: "Kotlin", Level: "index", Scope: "language",
		Symbols: true, Relations: true,
		Limitations: indexAdapterLimitations,
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json"},
	},
	{
		Language: "php", DisplayName: "PHP", Level: "full", Scope: "language+routes",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		PatternFamilies: []string{
			"Laravel and Symfony routes", "PHP HTTP clients", "Eloquent, Doctrine, and PDO",
			"queues and messaging", "gRPC", "PHP request/response boundaries", "PHPUnit and Pest",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "python", DisplayName: "Python", Level: "full", Scope: "language+routes",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		PatternFamilies: []string{
			"FastAPI, Flask, and Django routes", "requests, httpx, and aiohttp",
			"SQLAlchemy, Django ORM, and DB-API", "Kafka, Celery, and AMQP",
			"gRPC", "Python web and validation boundaries", "pytest and unittest",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "ruby", DisplayName: "Ruby", Level: "index", Scope: "language",
		Symbols: true, Relations: true,
		Limitations: indexAdapterLimitations,
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json"},
	},
	{
		Language: "rust", DisplayName: "Rust", Level: "full", Scope: "language+routes",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		PatternFamilies: []string{
			"Axum, Actix, and Rocket routes", "reqwest", "SQLx, Diesel, and SeaORM",
			"Kafka and AMQP", "tonic gRPC", "Rust web request/response boundaries",
			"Rust and Tokio tests",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "scala", DisplayName: "Scala", Level: "index", Scope: "language",
		Symbols: true, Relations: true,
		Limitations: indexAdapterLimitations,
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json"},
	},
	{
		Language: "shell", DisplayName: "Shell", Level: "partial", Scope: "language",
		Symbols: true, Relations: true, Calls: true,
		Limitations: "Function declarations, sourcing, and best-effort calls only; no route, API, persistence, messaging, data-flow, or test adapter.",
		Outputs:     []string{"symbols-full.json", "relations-full.json", "callgraph.json", "flows.json", "graph-full.json"},
	},
	{
		Language: "swift", DisplayName: "Swift", Level: "index", Scope: "language",
		Symbols: true, Relations: true,
		Limitations: indexAdapterLimitations,
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json"},
	},
	{
		Language: "typescript", DisplayName: "TypeScript / Node.js / React", Level: "full", Scope: "language+react+routes+api",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		ExactSymbols: true, DirectUsages: true, HTTPProvider: true, HTTPConsumer: true,
		PatternFamilies: []string{
			"Express and Fastify", "NestJS", "Next.js", "Web and Node HTTP clients",
			"common Node persistence", "Kafka and AMQP", "gRPC",
			"Node request/response boundaries", "Jest, Vitest, Node Test, and React Testing Library",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "api-contracts.json", "test-map.json", "graph-full.json",
		},
	},
}

const (
	fullAdapterLimitations  = "Static and pattern-backed; runtime-generated behavior, reflection, metaprogramming, dynamic dispatch, and external configuration may remain unresolved."
	indexAdapterLimitations = "Best-effort declarations and imports only; no normalized call, route, test, or architecture adapter."
)

// LanguageCapabilityProfiles returns a deterministic copy of all public
// source-language capability profiles.
func LanguageCapabilityProfiles() []LanguageCapabilityProfile {
	profiles := make([]LanguageCapabilityProfile, len(sourceLanguageCapabilityProfiles))
	for index, profile := range sourceLanguageCapabilityProfiles {
		profile.PatternFamilies = slices.Clone(profile.PatternFamilies)
		profile.Outputs = slices.Clone(profile.Outputs)
		profiles[index] = profile
	}
	return profiles
}

func languageCapabilityProfile(language string) (LanguageCapabilityProfile, bool) {
	for _, profile := range sourceLanguageCapabilityProfiles {
		if profile.Language == language {
			return profile, true
		}
	}
	return LanguageCapabilityProfile{}, false
}

func (profile LanguageCapabilityProfile) analyzerRecord() AnalyzerRecord {
	return AnalyzerRecord{
		Language: profile.Language, Scope: profile.Scope,
		Symbols: profile.Symbols, Relations: profile.Relations,
		Calls: profile.Calls, Endpoints: profile.Routes, Tests: profile.Tests,
		Outputs: slices.Clone(profile.Outputs),
	}
}
