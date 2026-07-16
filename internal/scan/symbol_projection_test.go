package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStableWorkspaceSymbolIDUsesEveryCanonicalIdentityPart(t *testing.T) {
	base := StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java")
	cases := []string{
		StableWorkspaceSymbolID("interface", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-task", "com.weka:users", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users-v2", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "typescript", "com.weka.UserService", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService.Inner", "src/main/java/com/weka/UserService.java"),
		StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService", "src/test/java/com/weka/UserService.java"),
	}
	for _, candidate := range cases {
		if candidate == base {
			t.Fatalf("identity input was ignored: %q", candidate)
		}
	}
	if again := StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java"); again != base {
		t.Fatalf("stable ID changed: %q != %q", again, base)
	}
}

func TestStableWorkspaceSymbolIDNormalizesWhitespaceAndSlashes(t *testing.T) {
	windows := StableWorkspaceSymbolID(" class ", ` microservices\ms-user `, " com.weka:users ", " java ", " com.weka.UserService ", ` src\main\java\com\weka\UserService.java `)
	portable := StableWorkspaceSymbolID("class", "microservices/ms-user", "com.weka:users", "java", "com.weka.UserService", "src/main/java/com/weka/UserService.java")
	if windows != portable {
		t.Fatalf("normalized identity changed: %q != %q", windows, portable)
	}
	if !strings.HasPrefix(portable, "symbol:") || len(portable) != len("symbol:")+32 {
		t.Fatalf("symbol ID has wrong format: %q", portable)
	}
}

func TestRichSymbolAndRelationRemainLegacyReadable(t *testing.T) {
	legacySymbol := []byte(`{"id":"s","name":"UserService","kind":"class","language":"java","file":"UserService.java","line":3}`)
	legacyRelation := []byte(`{"id":"r","from":"Client.java","to":"UserService.java","type":"imports_internal","confidence":"EXTRACTED","confidence_score":1}`)
	var symbol RichSymbolRecord
	var relation RichRelationRecord
	if err := json.Unmarshal(legacySymbol, &symbol); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(legacyRelation, &relation); err != nil {
		t.Fatal(err)
	}
	if symbol.Name != "UserService" || relation.From != "Client.java" {
		t.Fatalf("legacy fields changed: %#v %#v", symbol, relation)
	}
}

func TestStableWorkspaceUsageIDSeparatesCategoryTargetAndLocation(t *testing.T) {
	base := StableWorkspaceUsageID("symbol:provider", "frontend/app", "symbol:consumer", SymbolUsageDirectReference, "imports_type", "symbol:provider", "src/app.ts", 7)
	variants := []string{
		StableWorkspaceUsageID("symbol:provider", "frontend/app", "symbol:consumer", SymbolUsageReachedThroughAPI, "http_path", "symbol:provider", "src/app.ts", 7),
		StableWorkspaceUsageID("", "frontend/app", "symbol:consumer", SymbolUsageUnresolved, "imports_type", "@weka/users#UserService", "src/app.ts", 7),
		StableWorkspaceUsageID("symbol:provider", "frontend/app", "symbol:consumer", SymbolUsageDirectReference, "imports_type", "symbol:provider", "src/app.ts", 8),
	}
	for _, candidate := range variants {
		if candidate == base {
			t.Fatalf("usage identity input was ignored: %q", candidate)
		}
	}
	if !strings.HasPrefix(base, "usage:") || len(base) != len("usage:")+32 {
		t.Fatalf("usage ID has wrong format: %q", base)
	}
}

func TestWorkspaceEvidenceIDNamespacesLocalEvidence(t *testing.T) {
	if got := WorkspaceEvidenceID("microservices/ms-user", "evidence:1234"); got != "microservices/ms-user#evidence:1234" {
		t.Fatalf("WorkspaceEvidenceID() = %q", got)
	}
}
