package scan

import "testing"

func TestReferenceAdaptersRecognizeFrameworkCapabilityFamilies(t *testing.T) {
	cases := []struct {
		name       string
		language   string
		body       string
		capability CapabilityID
	}{
		{"spring mvc route", "java", `@PostMapping("/users")`, CapabilityRoutes},
		{"spring web client", "java", `private WebClient client;`, CapabilityAPIClients},
		{"spring declarative client", "java", `@FeignClient(name="users")`, CapabilityAPIClients},
		{"spring data", "java", `interface Users extends JpaRepository<User, String> {}`, CapabilityPersistence},
		{"spring jdbc", "java", `JdbcTemplate jdbc;`, CapabilityPersistence},
		{"spring kafka", "java", `@KafkaListener(topics="users")`, CapabilityMessaging},
		{"spring rabbit", "java", `RabbitTemplate rabbit;`, CapabilityMessaging},
		{"java grpc", "java", `@GrpcService`, CapabilityMessaging},
		{"jakarta validation", "java", `User create(@Valid Request request)`, CapabilityDataFlow},
		{"junit", "java", `@Test`, CapabilityTests},
		{"express route", "typescript", `app.post('/users', handler)`, CapabilityRoutes},
		{"nestjs route", "typescript", `@Controller('users')`, CapabilityRoutes},
		{"next route", "typescript", `export async function GET(req: NextRequest) {}`, CapabilityRoutes},
		{"fetch", "typescript", `fetch('/users')`, CapabilityAPIClients},
		{"axios", "typescript", `axios.post('/users')`, CapabilityAPIClients},
		{"prisma", "typescript", `prisma.user.create({})`, CapabilityPersistence},
		{"typeorm", "typescript", `import { Entity } from 'typeorm'`, CapabilityPersistence},
		{"sql", "typescript", `db.query('select 1')`, CapabilityPersistence},
		{"kafkajs", "typescript", `producer.send({topic:'users'})`, CapabilityMessaging},
		{"amqp", "typescript", `import amqp from 'amqplib'`, CapabilityMessaging},
		{"node grpc", "typescript", `import grpc from '@grpc/grpc-js'`, CapabilityMessaging},
		{"node boundary", "typescript", `const user = req.body`, CapabilityDataFlow},
		{"vitest rtl", "typescript", `test('users', () => render(<Users />))`, CapabilityTests},
		{"go route", "go", `router.GET("/users", listUsers)`, CapabilityRoutes},
		{"go http client", "go", `request, _ := http.NewRequest("GET", url, nil)`, CapabilityAPIClients},
		{"go sql", "go", `db.Query("select id from users")`, CapabilityPersistence},
		{"go gorm", "go", `gorm.Open(driver)`, CapabilityPersistence},
		{"go kafka", "go", `writer.WriteMessages(ctx, message)`, CapabilityMessaging},
		{"go amqp", "go", `amqp.Dial(url)`, CapabilityMessaging},
		{"go grpc", "go", `grpc.NewServer()`, CapabilityMessaging},
		{"go json boundary", "go", `json.NewDecoder(r.Body).Decode(&request)`, CapabilityDataFlow},
		{"go test", "go", `func TestUsers(t *testing.T) {}`, CapabilityTests},
		{"laravel route", "php", `Route::post('/users', [Users::class, 'create']);`, CapabilityRoutes},
		{"symfony route", "php", `#[Route('/users')]`, CapabilityRoutes},
		{"laravel http", "php", `Http::post('/users');`, CapabilityAPIClients},
		{"guzzle", "php", `use GuzzleHttp\\Client;`, CapabilityAPIClients},
		{"eloquent", "php", `class User extends Model {}`, CapabilityPersistence},
		{"doctrine", "php", `private EntityManager $entities;`, CapabilityPersistence},
		{"php queue", "php", `Queue::push($job);`, CapabilityMessaging},
		{"php amqp", "php", `new AMQPConnection();`, CapabilityMessaging},
		{"php grpc", "php", `class UsersClient extends Grpc\Client {}`, CapabilityMessaging},
		{"php validation", "php", `$request->validate([]);`, CapabilityDataFlow},
		{"phpunit", "php", `class UsersTest extends TestCase {}`, CapabilityTests},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := extractArchitectureCapabilityFacts(FileRecord{Path: "fixture", Language: tc.language}, tc.body)
			assertArchitectureCapabilityFact(t, facts, tc.language, tc.capability)
		})
	}
}
