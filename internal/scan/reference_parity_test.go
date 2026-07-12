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
`}}, {name: "rust", languages: []string{"rust"}, files: map[string]string{"Cargo.toml": "[package]\nname='users'\nversion='0.1.0'\n", "src/main.rs": `use axum::{routing::post, Json, Router};
fn routes() -> Router { Router::new().route("/users", post(create_user)) }
async fn create_user(Json(user): Json<User>) -> Json<User> {
  sqlx::query("insert into users").execute(&pool).await;
  let _ = reqwest::Client::new().post("/audit").send().await;
  let producer: rdkafka::producer::FutureProducer = producer();
  tonic::transport::Server::builder();
  Json(user)
}
#[tokio::test]
async fn creates_user() { create_user(request()).await; }
`}}, {name: "python", languages: []string{"python"}, files: map[string]string{"requirements.txt": "fastapi\nsqlalchemy\nhttpx\n", "app.py": `from fastapi import FastAPI
from pydantic import BaseModel
app = FastAPI()
class User(BaseModel):
    name: str
@app.post("/users")
def create_user(user: User):
    database.execute("insert into users")
    requests.post("/audit")
    kafka.send("users", user)
    grpc.server(pool)
    return JSONResponse(user.dict())
`, "test_app.py": `import pytest
def test_create_user():
    create_user(User(name="Ada"))
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
				if language == "rust" {
					var routes []CodeRouteRecord
					readJSON(t, filepath.Join(root, "goregraph-out", "routes.json"), &routes)
					assertTypedRoute(t, routes, "rust", "POST", "/users", "create_user")
					var calls CallGraphRecord
					readJSON(t, filepath.Join(root, "goregraph-out", "callgraph.json"), &calls)
					if len(calls.Edges) == 0 {
						t.Fatal("Rust fixture produced no call edges")
					}
					var tests []TestMapRecord
					readJSON(t, filepath.Join(root, "goregraph-out", "test-map.json"), &tests)
					if len(tests) == 0 {
						t.Fatal("Rust fixture produced no test mapping")
					}
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

func assertTypedRoute(t *testing.T, routes []CodeRouteRecord, language, method, path, handler string) {
	t.Helper()
	for _, route := range routes {
		if route.Language == language && route.HTTPMethod == method && route.Path == path && route.Handler == handler {
			return
		}
	}
	t.Fatalf("missing route %s %s for %s handler %s", method, path, language, handler)
}
