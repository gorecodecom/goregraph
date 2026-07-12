package scan

import (
	"fmt"
	"sort"
	"strings"
)

func extractArchitectureCapabilityFacts(file FileRecord, body string) []ArchitectureCapabilityFact {
	if file.Language != "java" && file.Language != "javascript" && file.Language != "typescript" && file.Language != "go" && file.Language != "php" && file.Language != "rust" && file.Language != "python" {
		return nil
	}
	var facts []ArchitectureCapabilityFact
	for index, line := range strings.Split(body, "\n") {
		seen := map[string]bool{}
		for _, match := range architectureCapabilityMatches(file.Language, line) {
			key := string(match.capability) + ":" + match.kind + ":" + match.framework
			if seen[key] {
				continue
			}
			seen[key] = true
			facts = append(facts, ArchitectureCapabilityFact{
				ID:         fmt.Sprintf("cap:%s:%s:%s:%s:%d", file.Language, match.capability, match.kind, file.Path, index+1),
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
		if strings.Contains(trimmed, "@GetMapping") || strings.Contains(trimmed, "@PostMapping") || strings.Contains(trimmed, "@PutMapping") || strings.Contains(trimmed, "@DeleteMapping") || strings.Contains(trimmed, "@PatchMapping") || strings.Contains(trimmed, "@RequestMapping") {
			add(CapabilityRoutes, "http_route", "Spring MVC/WebFlux")
		}
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
		if strings.Contains(trimmed, "@Test") || strings.Contains(trimmed, "MockMvc") || strings.Contains(trimmed, "WebTestClient") {
			add(CapabilityTests, "test", "JUnit/Spring Test")
		}
	} else if language == "javascript" || language == "typescript" {
		if strings.Contains(lower, "app.get(") || strings.Contains(lower, "app.post(") || strings.Contains(lower, "router.get(") || strings.Contains(lower, "router.post(") {
			add(CapabilityRoutes, "http_route", "Express/Fastify")
		}
		if strings.Contains(trimmed, "@Controller(") || strings.Contains(trimmed, "@Get(") || strings.Contains(trimmed, "@Post(") {
			add(CapabilityRoutes, "http_route", "NestJS")
		}
		if strings.Contains(lower, "nexthandler") || strings.Contains(lower, "nextrequest") || strings.Contains(lower, "getserversideprops") {
			add(CapabilityRoutes, "http_route", "Next.js")
		}
		if strings.Contains(lower, "fetch(") || strings.Contains(lower, "axios.") || strings.Contains(lower, "httpclient.") {
			add(CapabilityAPIClients, "http_client", "Web/Node HTTP")
		}
		if strings.Contains(lower, "prisma.") || strings.Contains(lower, "typeorm") || strings.Contains(lower, "sequelize") || strings.Contains(lower, "mongoose") || strings.Contains(lower, "db.query(") {
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
		if strings.Contains(lower, "test(") || strings.Contains(lower, "it(") || strings.Contains(lower, "describe(") || strings.Contains(lower, "render(") {
			add(CapabilityTests, "test", "Jest/Vitest/Node Test/RTL")
		}
	} else if language == "go" {
		if strings.Contains(trimmed, "http.HandleFunc(") || strings.Contains(trimmed, ".HandleFunc(") || strings.Contains(trimmed, ".GET(") || strings.Contains(trimmed, ".POST(") || strings.Contains(trimmed, ".PUT(") || strings.Contains(trimmed, ".DELETE(") {
			add(CapabilityRoutes, "http_route", "Go HTTP/router")
		}
		if strings.Contains(trimmed, "http.Get(") || strings.Contains(trimmed, "http.Post(") || strings.Contains(trimmed, "http.NewRequest(") || strings.Contains(trimmed, "client.Do(") {
			add(CapabilityAPIClients, "http_client", "Go net/http")
		}
		if strings.Contains(trimmed, "sql.Open(") || strings.Contains(trimmed, "db.Query(") || strings.Contains(trimmed, "db.Exec(") || strings.Contains(lower, "gorm.") {
			add(CapabilityPersistence, "persistence", "Go SQL/GORM")
		}
		if strings.Contains(lower, "kafka.newreader(") || strings.Contains(lower, "kafka.newwriter(") || strings.Contains(trimmed, "WriteMessages(") || strings.Contains(lower, "amqp.dial(") {
			add(CapabilityMessaging, "message_consumer_or_producer", "Go Kafka/AMQP")
		}
		if strings.Contains(lower, "grpc.newserver(") || strings.Contains(trimmed, "RegisterService(") {
			add(CapabilityMessaging, "rpc_service", "gRPC")
		}
		if strings.Contains(trimmed, "json.NewDecoder(") || strings.Contains(trimmed, "json.NewEncoder(") || strings.Contains(trimmed, ".Bind(") {
			add(CapabilityDataFlow, "request_response_boundary", "Go HTTP")
		}
		if strings.HasPrefix(trimmed, "func Test") || strings.Contains(trimmed, "httptest.New") {
			add(CapabilityTests, "test", "Go test/httptest")
		}
	} else if language == "php" {
		if strings.Contains(trimmed, "Route::") || strings.Contains(trimmed, "#[Route(") || strings.Contains(trimmed, "@Route(") {
			add(CapabilityRoutes, "http_route", "Laravel/Symfony")
		}
		if strings.Contains(trimmed, "Http::") || strings.Contains(trimmed, "GuzzleHttp") || strings.Contains(trimmed, "new Client(") {
			add(CapabilityAPIClients, "http_client", "PHP HTTP client")
		}
		if strings.Contains(trimmed, "extends Model") || strings.Contains(trimmed, "EntityManager") || strings.Contains(trimmed, "new PDO(") || strings.Contains(lower, "doctrine") {
			add(CapabilityPersistence, "persistence", "Eloquent/Doctrine/PDO")
		}
		if strings.Contains(trimmed, "Queue::") || strings.Contains(trimmed, "dispatch(") || strings.Contains(trimmed, "AMQP") || strings.Contains(trimmed, "MessageBusInterface") {
			add(CapabilityMessaging, "message_consumer_or_producer", "PHP queue/messaging")
		}
		if strings.Contains(lower, "grpc\\client") || strings.Contains(trimmed, "BaseStub") {
			add(CapabilityMessaging, "rpc_service", "gRPC")
		}
		if strings.Contains(trimmed, "->validate(") || strings.Contains(trimmed, "Request $") || strings.Contains(trimmed, "JsonResponse(") || strings.Contains(trimmed, "response()->json(") {
			add(CapabilityDataFlow, "request_response_boundary", "PHP HTTP")
		}
		if strings.Contains(trimmed, "extends TestCase") || strings.HasPrefix(trimmed, "test(") || strings.HasPrefix(trimmed, "it(") {
			add(CapabilityTests, "test", "PHPUnit/Pest")
		}
	} else if language == "rust" {
		if codeRustAttributeRouteRE.MatchString(trimmed) || codeRustRouterRouteRE.MatchString(trimmed) {
			add(CapabilityRoutes, "http_route", "Axum/Actix/Rocket")
		}
		if strings.Contains(lower, "reqwest::") || strings.Contains(lower, "reqwest.") || strings.Contains(lower, "client.get(") || strings.Contains(lower, "client.post(") {
			add(CapabilityAPIClients, "http_client", "Rust reqwest")
		}
		if strings.Contains(lower, "sqlx::") || strings.Contains(lower, "diesel::") || strings.Contains(lower, "sea_orm::") || strings.Contains(lower, ".execute(") {
			add(CapabilityPersistence, "persistence", "Rust SQLx/Diesel/SeaORM")
		}
		if strings.Contains(lower, "rdkafka::") || strings.Contains(lower, "futureproducer") || strings.Contains(lower, "lapin::") {
			add(CapabilityMessaging, "message_consumer_or_producer", "Rust Kafka/AMQP")
		}
		if strings.Contains(lower, "tonic::") || strings.Contains(lower, "server::builder") {
			add(CapabilityMessaging, "rpc_service", "tonic gRPC")
		}
		if strings.Contains(trimmed, "Json<") || strings.Contains(trimmed, "web::Json") || strings.Contains(trimmed, "HttpResponse::") {
			add(CapabilityDataFlow, "request_response_boundary", "Rust web")
		}
		if strings.Contains(trimmed, "#[test]") || strings.Contains(trimmed, "#[tokio::test]") {
			add(CapabilityTests, "test", "Rust test/tokio test")
		}
	} else if language == "python" {
		if codePythonRouteRE.MatchString(trimmed) || strings.Contains(trimmed, "path(") || strings.Contains(trimmed, "re_path(") {
			add(CapabilityRoutes, "http_route", "FastAPI/Flask/Django")
		}
		if strings.Contains(lower, "requests.get(") || strings.Contains(lower, "requests.post(") || strings.Contains(lower, "httpx.") || strings.Contains(lower, "aiohttp.") {
			add(CapabilityAPIClients, "http_client", "Python requests/httpx/aiohttp")
		}
		if strings.Contains(lower, "sqlalchemy") || strings.Contains(lower, "django.db") || strings.Contains(lower, "models.model") || strings.Contains(lower, "psycopg") || strings.Contains(lower, ".execute(") {
			add(CapabilityPersistence, "persistence", "SQLAlchemy/Django ORM/DB-API")
		}
		if strings.Contains(lower, "kafka") || strings.Contains(lower, "celery") || strings.Contains(lower, "pika.") {
			add(CapabilityMessaging, "message_consumer_or_producer", "Python Kafka/Celery/AMQP")
		}
		if strings.Contains(lower, "grpc.") || strings.Contains(lower, "_pb2_grpc") {
			add(CapabilityMessaging, "rpc_service", "Python gRPC")
		}
		if strings.Contains(trimmed, "BaseModel") || strings.Contains(lower, "request.json") || strings.Contains(lower, "jsonify(") || strings.Contains(trimmed, "JSONResponse(") {
			add(CapabilityDataFlow, "request_response_boundary", "Python web/validation")
		}
		if strings.HasPrefix(trimmed, "def test_") || strings.Contains(lower, "pytest") || strings.Contains(trimmed, "unittest.TestCase") {
			add(CapabilityTests, "test", "pytest/unittest")
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
