package scan

import (
	"github.com/gorecodecom/goregraph/internal/config"
	"path/filepath"
	"testing"
)

func TestReferenceLanguagesReachSharedCapabilityGate(t *testing.T) {
	cases := []struct {
		name      string
		files     map[string]string
		languages []string
	}{{name: "java-spring", languages: []string{"java"}, files: map[string]string{"pom.xml": "<project><modelVersion>4.0.0</modelVersion></project>", "src/UserController.java": `package demo;
@RestController
class UserController {
  private WebClient client;
  @PostMapping("/users")
  User create(@Valid UserRequest request) { return service.save(request); }
}
@Entity
class User { @Id String id; }
interface UserRepository extends JpaRepository<User,String> {}
class Events { @KafkaListener(topics="users") void consume(User user) {} }
@GrpcService
class UserGrpc {}
`, "src/UserControllerTest.java": `class UserControllerTest {
  @Test
  void createsUser() { client.post().uri("/users"); }
}`}}, {name: "typescript-react-node", languages: []string{"typescript"}, files: map[string]string{"package.json": `{"scripts":{"test":"vitest"}}`, "src/app.tsx": `interface User { id:string }
export function Users() {
  const save=()=>fetch('/users',{method:'POST'});
  return <button onClick={save}>Save</button>;
}`, "src/server.ts": `import express from 'express';
const app=express();
app.post('/users', async function createUser(req,res) {
  const user=await prisma.user.create({data:req.body});
  await producer.send({topic:'users'});
  res.json(user);
});`, "src/app.test.tsx": `test('saves user',()=>render(<Users/>));`}}, {name: "go", languages: []string{"go"}, files: map[string]string{"go.mod": "module example.com/users\n", "main.go": `package main
func routes() { http.HandleFunc("/users", createUser) }
func createUser(w http.ResponseWriter, r *http.Request) {
  json.NewDecoder(r.Body).Decode(&user)
  db.Exec("insert into users")
  writer.WriteMessages(context.Background(), kafka.Message{})
  client.Do(request)
  json.NewEncoder(w).Encode(user)
}
func rpc() { grpc.NewServer() }
`, "main_test.go": `package main
func TestCreateUser(t *testing.T) { httptest.NewRecorder() }
`}}, {name: "php", languages: []string{"php"}, files: map[string]string{"composer.json": `{"require":{}}`, "src/UserController.php": `<?php
Route::post('/users', [UserController::class, 'create']);
class User extends Model {}
class UserController {
  function create(Request $request) {
    $request->validate([]);
    $response = Http::post('/audit');
    dispatch(new UserCreated());
    return response()->json($response);
  }
}
class UserGrpc extends Grpc\Client {}
`, "tests/UserTest.php": `<?php
class UserTest extends TestCase {}
`}}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for path, body := range tc.files {
				writeFile(t, root, path, body)
			}
			if _, err := Run(root, config.Defaults()); err != nil {
				t.Fatal(err)
			}
			var records []CapabilityRecord
			readJSON(t, filepath.Join(root, "goregraph-out", "capabilities.json"), &records)
			var facts []ArchitectureCapabilityFact
			readJSON(t, filepath.Join(root, "goregraph-out", "architecture-capabilities.json"), &facts)
			for _, language := range tc.languages {
				for _, capability := range []CapabilityID{CapabilitySymbols, CapabilityRelations, CapabilityCalls, CapabilityRoutes, CapabilityAPIClients, CapabilityTests, CapabilityPersistence, CapabilityMessaging, CapabilityDataFlow} {
					assertCapabilityCoverage(t, records, language, capability, CoverageComplete)
				}
				for _, capability := range []CapabilityID{CapabilityAPIClients, CapabilityPersistence, CapabilityMessaging, CapabilityDataFlow} {
					assertArchitectureCapabilityFact(t, facts, language, capability)
				}
			}
		})
	}
}

func assertArchitectureCapabilityFact(t *testing.T, facts []ArchitectureCapabilityFact, language string, capability CapabilityID) {
	t.Helper()
	for _, fact := range facts {
		if fact.Language == language && fact.Capability == capability && fact.File != "" && fact.Line > 0 {
			return
		}
	}
	t.Fatalf("missing evidence fact for %s/%s", language, capability)
}
