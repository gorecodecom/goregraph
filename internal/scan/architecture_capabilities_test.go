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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := extractArchitectureCapabilityFacts(FileRecord{Path: "fixture", Language: tc.language}, tc.body)
			assertArchitectureCapabilityFact(t, facts, tc.language, tc.capability)
		})
	}
}
