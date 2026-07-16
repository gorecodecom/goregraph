package scan

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestExtractJavaCanonicalDeclarations(t *testing.T) {
	body := `package com.weka.users;

public class Outer {
    record Snapshot(String value) {
    }
}

interface Port {
}

enum State {
    ACTIVE
}

class SecondTopLevel {
}
`
	source := extractJavaSource(FileRecord{
		Path:     "src/main/java/com/weka/users/Outer.java",
		Language: "java",
	}, body)
	workspace := WorkspaceIndex{MavenPackages: []MavenPackageRecord{{
		Path:       "pom.xml",
		GroupID:    "com.weka",
		ArtifactID: "users-api",
	}}}

	facts := ExtractJavaSymbolFacts(source, body, workspace)

	wants := []struct {
		kind          string
		qualifiedName string
		line          int
	}{
		{kind: "class", qualifiedName: "com.weka.users.Outer", line: 3},
		{kind: "record", qualifiedName: "com.weka.users.Outer.Snapshot", line: 4},
		{kind: "interface", qualifiedName: "com.weka.users.Port", line: 8},
		{kind: "enum", qualifiedName: "com.weka.users.State", line: 11},
		{kind: "class", qualifiedName: "com.weka.users.SecondTopLevel", line: 15},
	}
	for _, want := range wants {
		record := assertRichDeclaration(t, facts.Declarations, want.kind, want.qualifiedName, "com.weka:users-api")
		if record.File != source.File || record.Line != want.line || record.SourceLocation != sourceLocation(want.line) {
			t.Fatalf("declaration location = %#v, want %s:%d", record, source.File, want.line)
		}
		if record.ID == "" || record.DeclarationID != record.ID {
			t.Fatalf("declaration IDs = %#v, want one stable canonical ID", record)
		}
		if record.Analyzer != "java-source" || record.Confidence != ConfidenceExact || record.Coverage != CoverageComplete {
			t.Fatalf("declaration provenance = %#v, want exact complete java-source", record)
		}
	}

	again := ExtractJavaSymbolFacts(source, body, workspace)
	if !reflect.DeepEqual(facts.Declarations, again.Declarations) {
		t.Fatalf("declarations are not stable:\nfirst:  %#v\nsecond: %#v", facts.Declarations, again.Declarations)
	}

	byName := map[string]JavaTypeRecord{}
	for _, typ := range source.Types {
		byName[typ.QualifiedName] = typ
	}
	if got := byName["com.weka.users.Outer.Snapshot"].Owner; got != "com.weka.users.Outer" {
		t.Fatalf("nested record owner = %q, want com.weka.users.Outer", got)
	}
	if got := byName["com.weka.users.Outer.Snapshot"].EndLine; got != 5 {
		t.Fatalf("nested record end line = %d, want 5", got)
	}
	if got := byName["com.weka.users.Port"].Owner; got != "" {
		t.Fatalf("top-level interface owner = %q, want empty", got)
	}
	if got := byName["com.weka.users.SecondTopLevel"].QualifiedName; strings.Contains(got, "Outer") {
		t.Fatalf("second top-level type inherited stale owner: %q", got)
	}
}

func TestExtractJavaReferenceMatrix(t *testing.T) {
	body := `package com.weka.users;

import java.util.List;
import java.util.Optional;
import com.weka.users.UserService;
import static com.weka.users.UserService.create;

class UserService {}
class Base<T> {}
interface UserPort {}

@com.weka.audit.Audited
class Consumer extends Base<UserService> implements UserPort {
    private List<UserService[]> services;
    private UserService userService;

    Consumer(UserService userService) {
        this.userService = userService;
    }

    Optional<UserService> find(UserService input) {
        UserService created = new UserService();
        return userService.find();
    }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Consumer.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	consumer := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Consumer", "")

	wants := []struct {
		kind   string
		target string
		line   int
	}{
		{kind: "imports_type", target: "java.util.List", line: 3},
		{kind: "annotation_type", target: "com.weka.audit.Audited", line: 12},
		{kind: "field_type", target: "com.weka.users.UserService", line: 14},
		{kind: "parameter_type", target: "com.weka.users.UserService", line: 17},
		{kind: "return_type", target: "com.weka.users.UserService", line: 21},
		{kind: "extends_type", target: "com.weka.users.Base", line: 13},
		{kind: "implements_type", target: "com.weka.users.UserPort", line: 13},
		{kind: "instantiates", target: "com.weka.users.UserService", line: 22},
		{kind: "static_import", target: "com.weka.users.UserService", line: 6},
		{kind: "calls_method_owner", target: "com.weka.users.UserService", line: 23},
	}
	for _, want := range wants {
		record := assertJavaReference(t, facts.References, want.kind, want.target, want.line)
		if record.From != source.File || record.FromSymbolID != consumer.ID {
			t.Fatalf("%s source identity = %#v, want file %s and symbol %s", want.kind, record, source.File, consumer.ID)
		}
		if record.Analyzer != "java-source" || record.Reason == "" || len(record.EvidenceIDs) == 0 {
			t.Fatalf("%s provenance = %#v, want analyzer, reason, and evidence", want.kind, record)
		}
	}

	call := assertJavaReference(t, facts.References, "calls_method_owner", "com.weka.users.UserService", 23)
	provider := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.UserService", "")
	if call.ToSymbolID != provider.ID || call.Resolution != SymbolResolutionExact {
		t.Fatalf("resolved call owner = %#v, want exact provider %s", call, provider.ID)
	}
	for _, reference := range facts.References {
		if reference.Type == "annotation_type" && reference.TargetQualifiedName == "com.weka.users.Audited" {
			t.Fatalf("fully qualified annotation also emitted a guessed target: %#v", reference)
		}
	}
}

func TestJavaNestedAndGenericTypeNormalization(t *testing.T) {
	body := `package com.weka.users;

import java.util.Map;

class Outer {
    static class Inner {}
}

class GenericConsumer {
    Map<String, ? extends Outer$Inner[]> nested;
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/GenericConsumer.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})

	nested := assertJavaReference(t, facts.References, "field_type", "com.weka.users.Outer.Inner", 10)
	inner := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Outer.Inner", "")
	if nested.ToSymbolID != inner.ID || nested.Resolution != SymbolResolutionExact {
		t.Fatalf("nested generic reference = %#v, want exact inner declaration %s", nested, inner.ID)
	}

	unresolvedBody := `package com.weka.consumer;

class Consumer {
    UserService service;
}
`
	unresolvedSource := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/consumer/Consumer.java", Language: "java"}, unresolvedBody)
	unresolvedFacts := ExtractJavaSymbolFacts(unresolvedSource, unresolvedBody, WorkspaceIndex{})
	unresolved := assertJavaReference(t, unresolvedFacts.References, "field_type", "com.weka.consumer.UserService", 4)
	if unresolved.ToSymbolID != "" || unresolved.Resolution == SymbolResolutionExact || unresolved.Confidence == string(ConfidenceExact) {
		t.Fatalf("name-only reference was promoted to exact: %#v", unresolved)
	}

	ownerBody := `package com.weka.users;

class UserService {}
class OuterOwner {
    static class Inner {}

    UserService create() {
        return new UserService();
    }
}
`
	ownerSource := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/OuterOwner.java", Language: "java"}, ownerBody)
	ownerFacts := ExtractJavaSymbolFacts(ownerSource, ownerBody, WorkspaceIndex{})
	outer := assertRichDeclaration(t, ownerFacts.Declarations, "class", "com.weka.users.OuterOwner", "")
	instantiation := assertJavaReference(t, ownerFacts.References, "instantiates", "com.weka.users.UserService", 8)
	if instantiation.FromSymbolID != outer.ID {
		t.Fatalf("post-nested instantiation owner = %#v, want outer %s", instantiation, outer.ID)
	}
}

func assertJavaReference(t *testing.T, records []RichRelationRecord, kind, target string, line int) RichRelationRecord {
	t.Helper()
	for _, record := range records {
		if record.Type == kind && record.TargetQualifiedName == target && record.Line == line {
			return record
		}
	}
	t.Fatalf("missing %s reference to %s at line %d in %#v", kind, target, line, records)
	return RichRelationRecord{}
}

func TestRunWritesJavaCanonicalSymbolFacts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pom.xml", `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.weka</groupId>
  <artifactId>users-api</artifactId>
</project>`)
	writeFile(t, root, "src/main/java/com/weka/users/UserService.java", `package com.weka.users;

public class UserService {
    public void find() {}
}
`)
	writeFile(t, root, "src/main/java/com/weka/users/Consumer.java", `package com.weka.users;

class Consumer {
    private UserService userService;

    void run() {
        userService.find();
    }
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []RichSymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols-full.json"), &symbols)
	consumer := assertRichDeclaration(t, symbols, "class", "com.weka.users.Consumer", "com.weka:users-api")
	provider := assertRichDeclaration(t, symbols, "class", "com.weka.users.UserService", "com.weka:users-api")
	if len(consumer.EvidenceIDs) != 1 || len(provider.EvidenceIDs) != 1 {
		t.Fatalf("declaration evidence IDs missing: consumer=%#v provider=%#v", consumer, provider)
	}

	var relations []RichRelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations-full.json"), &relations)
	call := assertJavaReference(t, relations, "calls_method_owner", "com.weka.users.UserService", 7)
	if call.FromSymbolID != consumer.ID || call.ToSymbolID != provider.ID || call.Resolution != SymbolResolutionExact {
		t.Fatalf("integrated call relation = %#v, want %s -> %s", call, consumer.ID, provider.ID)
	}

	var graph CallGraphRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "callgraph.json"), &graph)
	if len(graph.Edges) != 1 {
		t.Fatalf("callgraph edges = %#v, want one", graph.Edges)
	}
	edge := graph.Edges[0]
	if edge.FromSymbolID != consumer.ID || edge.ToSymbolID != provider.ID || edge.TargetQualifiedName != provider.QualifiedName {
		t.Fatalf("callgraph canonical owners = %#v, want %s -> %s", edge, consumer.ID, provider.ID)
	}

	var evidence []EvidenceRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "evidence.json"), &evidence)
	known := map[string]bool{}
	for _, record := range evidence {
		known[record.ID] = true
	}
	for _, id := range append(consumer.EvidenceIDs, provider.EvidenceIDs...) {
		if !known[id] {
			t.Fatalf("declaration evidence %s is not present in evidence.json", id)
		}
	}
}
