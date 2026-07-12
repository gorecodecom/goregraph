package scan

import (
	"fmt"
	"sort"
	"strings"
)

func extractArchitectureCapabilityFacts(file FileRecord, body string) []ArchitectureCapabilityFact {
	if file.Language != "java" && file.Language != "javascript" && file.Language != "typescript" {
		return nil
	}
	var facts []ArchitectureCapabilityFact
	for index, line := range strings.Split(body, "\n") {
		for _, match := range architectureCapabilityMatches(file.Language, line) {
			facts = append(facts, ArchitectureCapabilityFact{
				ID:         fmt.Sprintf("cap:%s:%s:%s:%d", file.Language, match.capability, file.Path, index+1),
				Language:   file.Language,
				Capability: match.capability,
				Kind:       match.kind,
				Framework:  match.framework,
				File:       file.Path,
				Line:       index + 1,
			})
		}
	}
	return facts
}

type architectureCapabilityMatch struct {
	capability CapabilityID
	kind       string
	framework  string
}

func architectureCapabilityMatches(language, line string) []architectureCapabilityMatch {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	var matches []architectureCapabilityMatch
	add := func(capability CapabilityID, kind, framework string) {
		matches = append(matches, architectureCapabilityMatch{capability: capability, kind: kind, framework: framework})
	}
	if language == "java" {
		if strings.Contains(trimmed, "RestTemplate") || strings.Contains(trimmed, "WebClient") || strings.Contains(trimmed, "FeignClient") || strings.Contains(trimmed, "HttpClient") {
			add(CapabilityAPIClients, "http_client", "Spring/Java HTTP")
		}
		if strings.Contains(trimmed, "JpaRepository") || strings.Contains(trimmed, "CrudRepository") || strings.Contains(trimmed, "@Entity") || strings.Contains(trimmed, "JdbcTemplate") {
			add(CapabilityPersistence, "persistence", "Spring Data")
		}
		if strings.Contains(trimmed, "@KafkaListener") || strings.Contains(trimmed, "KafkaTemplate") || strings.Contains(trimmed, "RabbitTemplate") || strings.Contains(trimmed, "@RabbitListener") {
			add(CapabilityMessaging, "message_consumer_or_producer", "Spring Messaging")
		}
		if strings.Contains(trimmed, "@GrpcService") || strings.Contains(trimmed, "StreamObserver") {
			add(CapabilityMessaging, "rpc_service", "gRPC")
		}
		if strings.Contains(trimmed, "@Valid") || strings.Contains(trimmed, "@Validated") {
			add(CapabilityDataFlow, "validation_boundary", "Jakarta Validation")
		}
	} else {
		if strings.Contains(lower, "fetch(") || strings.Contains(lower, "axios.") || strings.Contains(lower, "httpclient.") {
			add(CapabilityAPIClients, "http_client", "Web/Node HTTP")
		}
		if strings.Contains(lower, "prisma.") || strings.Contains(lower, "typeorm") || strings.Contains(lower, "sequelize") || strings.Contains(lower, "mongoose") {
			add(CapabilityPersistence, "persistence", "Node persistence")
		}
		if strings.Contains(lower, "producer.send(") || strings.Contains(lower, "consumer.subscribe(") || strings.Contains(lower, "kafka.") || strings.Contains(lower, "amqplib") {
			add(CapabilityMessaging, "message_consumer_or_producer", "Node messaging")
		}
		if strings.Contains(lower, "grpc.") || strings.Contains(lower, "@grpc/grpc-js") {
			add(CapabilityMessaging, "rpc_service", "gRPC")
		}
		if strings.Contains(lower, "req.body") || strings.Contains(lower, "res.json(") {
			add(CapabilityDataFlow, "request_response_boundary", "Node HTTP")
		}
	}
	return matches
}

func sortArchitectureCapabilityFacts(facts []ArchitectureCapabilityFact) {
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].Language != facts[j].Language {
			return facts[i].Language < facts[j].Language
		}
		if facts[i].File != facts[j].File {
			return facts[i].File < facts[j].File
		}
		if facts[i].Line != facts[j].Line {
			return facts[i].Line < facts[j].Line
		}
		return facts[i].Capability < facts[j].Capability
	})
}
