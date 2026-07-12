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
});`, "src/app.test.tsx": `test('saves user',()=>render(<Users/>));`}}}
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
