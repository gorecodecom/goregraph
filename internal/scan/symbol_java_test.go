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

func TestJavaLexicalSanitizationPreventsFalseFacts(t *testing.T) {
	body := `package com.weka.users;

class Real {
    String inline = "@com.fake.Inline new InlineFake() { }";
    String block = """
        @com.fake.TextBlock
        class TextBlockFake {
            Object value = new TextBlockFake();
        }
        """;
    // @com.fake.Comment new CommentFake() class CommentFake { }
    /*
     * @com.fake.BlockComment
     * class BlockFake { Object value = new BlockFake(); }
     */
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Real.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	if len(facts.Declarations) != 1 || facts.Declarations[0].QualifiedName != "com.weka.users.Real" {
		t.Fatalf("lexical noise created declarations: %#v", facts.Declarations)
	}
	for _, reference := range facts.References {
		if reference.Type == "annotation_type" || reference.Type == "instantiates" {
			t.Fatalf("lexical noise created reference: %#v", reference)
		}
	}
	if facts.Declarations[0].Coverage != CoverageComplete || facts.Declarations[0].Confidence != ConfidenceExact {
		t.Fatalf("real declaration lost exact provenance: %#v", facts.Declarations[0])
	}
}

func TestJavaNextLineBracesPreserveNestedOwnersAndEndLines(t *testing.T) {
	body := `package com.weka.users;

class Outer
{
    class Inner
    {
    }
}

class After
{
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Outer.java", Language: "java"}, body)
	byName := map[string]JavaTypeRecord{}
	for _, typ := range source.Types {
		byName[typ.QualifiedName] = typ
	}
	if got := byName["com.weka.users.Outer.Inner"].Owner; got != "com.weka.users.Outer" {
		t.Fatalf("next-line nested owner = %q, want com.weka.users.Outer", got)
	}
	if got := byName["com.weka.users.Outer.Inner"].EndLine; got != 7 {
		t.Fatalf("next-line nested end = %d, want 7", got)
	}
	if got := byName["com.weka.users.Outer"].EndLine; got != 8 {
		t.Fatalf("next-line outer end = %d, want 8", got)
	}
	if got := byName["com.weka.users.After"].Owner; got != "" {
		t.Fatalf("next-line top-level owner = %q, want empty", got)
	}
}

func TestJavaTypeParametersAreNotResolvedAsProjectTypes(t *testing.T) {
	body := `package com.weka.users;

class Base<T> {}
class Box<T> extends Base<T> {
    T value;
    T service;

    <R> R convert(R input) {
        return input;
    }

    void invoke() { service.find(); }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Box.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	assertJavaReference(t, facts.References, "extends_type", "com.weka.users.Base", 4)
	for _, reference := range facts.References {
		if strings.HasSuffix(reference.TargetQualifiedName, ".T") || strings.HasSuffix(reference.TargetQualifiedName, ".R") {
			t.Fatalf("type variable became a project reference: %#v", reference)
		}
	}
}

func TestJavaAnnotationOwnershipDistinguishesMembersAndTypes(t *testing.T) {
	body := `package com.weka.users;

class Owner {
    @com.weka.MemberMarker
    String value;

    @com.weka.TypeMarker(
        value = "nested"
    )
    class Nested {}
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Owner.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	owner := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Owner", "")
	nested := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Owner.Nested", "")
	memberAnnotation := assertJavaReference(t, facts.References, "annotation_type", "com.weka.MemberMarker", 4)
	typeAnnotation := assertJavaReference(t, facts.References, "annotation_type", "com.weka.TypeMarker", 7)
	if memberAnnotation.FromSymbolID != owner.ID {
		t.Fatalf("member annotation owner = %#v, want %s", memberAnnotation, owner.ID)
	}
	if typeAnnotation.FromSymbolID != nested.ID {
		t.Fatalf("type annotation owner = %#v, want %s", typeAnnotation, nested.ID)
	}
}

func TestJavaReceiverTypesUseCanonicalOwnerScopes(t *testing.T) {
	body := `package com.weka.users;

class ServiceA { void find() {} }
class ServiceB { void find() {} }
class First {
    class Same {
        ServiceA service;
        void run() { service.find(); }
    }
}
class Second {
    class Same {
        ServiceB service;
        void run() { service.find(); }
    }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Owners.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	first := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.First.Same", "")
	second := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Second.Same", "")
	firstCall := assertJavaReference(t, facts.References, "calls_method_owner", "com.weka.users.ServiceA", 8)
	secondCall := assertJavaReference(t, facts.References, "calls_method_owner", "com.weka.users.ServiceB", 14)
	if firstCall.FromSymbolID != first.ID || secondCall.FromSymbolID != second.ID {
		t.Fatalf("receiver owners collided: first=%#v second=%#v", firstCall, secondCall)
	}
}

func TestJavaDiamondInstantiationAndDeclaratorArrays(t *testing.T) {
	body := `package com.weka.users;

class Box<T> {}
class UserService {}
class Consumer {
    UserService services[];

    Consumer(UserService inputs []) {
        Box<UserService> box = new Box<>();
    }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Consumer.java", Language: "java"}, body)
	if len(source.Fields) != 1 || source.Fields[0].Name != "services" || source.Fields[0].Type != "UserService[]" {
		t.Fatalf("declarator-side field array = %#v, want services UserService[]", source.Fields)
	}
	if len(source.Methods) != 1 || len(source.Methods[0].Parameters) != 1 || source.Methods[0].Parameters[0].Name != "inputs" || source.Methods[0].Parameters[0].Type != "UserService[]" {
		t.Fatalf("declarator-side parameter array = %#v, want inputs UserService[]", source.Methods)
	}
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	assertJavaReference(t, facts.References, "field_type", "com.weka.users.UserService", 6)
	assertJavaReference(t, facts.References, "parameter_type", "com.weka.users.UserService", 8)
	assertJavaReference(t, facts.References, "instantiates", "com.weka.users.Box", 9)
}

func TestJavaAnnotatedGenericScopesAndMultilineTypeHeaders(t *testing.T) {
	body := `package com.weka.users;

@interface TypeUse { String value() default ""; }
class Bound {}
class Base<T> {}
interface Port<T> {}

class Child<
    @TypeUse("class") T extends Bound
>
    extends Base<T>
    implements Port<T>
{
    public <
        @TypeUse("method") R extends Bound
    > R map(R input)
    {
        return input;
    }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Child.java", Language: "java"}, body)
	var child JavaTypeRecord
	for _, candidate := range source.Types {
		if candidate.QualifiedName == "com.weka.users.Child" {
			child = candidate
			break
		}
	}
	if child.QualifiedName == "" {
		t.Fatalf("multiline type declaration was not extracted: %#v", source.Types)
	}
	if child.Line != 8 || !reflect.DeepEqual(child.TypeParameters, []string{"T"}) || child.Extends != "Base<T>" || !reflect.DeepEqual(child.Implements, []string{"Port<T>"}) {
		t.Fatalf("multiline type header = %#v, want line 8 with T, Base<T>, and Port<T>", child)
	}
	var method JavaMethodRecord
	for _, candidate := range source.Methods {
		if candidate.Name == "map" {
			method = candidate
			break
		}
	}
	if method.Name == "" || !reflect.DeepEqual(method.TypeParameters, []string{"R"}) || method.ReturnType != "R" || len(method.Parameters) != 1 || method.Parameters[0].Type != "R" {
		t.Fatalf("annotated multiline generic method = %#v, want scoped R method", method)
	}
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	assertJavaReference(t, facts.References, "extends_type", "com.weka.users.Base", 8)
	assertJavaReference(t, facts.References, "implements_type", "com.weka.users.Port", 8)
	for _, reference := range facts.References {
		if strings.HasSuffix(reference.TargetQualifiedName, ".T") || strings.HasSuffix(reference.TargetQualifiedName, ".R") {
			t.Fatalf("annotated type variable became a project reference: %#v", reference)
		}
	}
}

func TestJavaQualifiedReceiverChainResolvesNestedOwner(t *testing.T) {
	providerBody := `package com.provider;
class Outer {
    static class Inner { static void run() {} }
}
`
	consumerBody := `package com.consumer;
import com.provider.Outer;
class Inner {}
class Consumer {
    void call() { Outer.Inner.run(); }
}
`
	provider := extractJavaSource(FileRecord{Path: "src/main/java/com/provider/Outer.java", Language: "java"}, providerBody)
	consumer := extractJavaSource(FileRecord{Path: "src/main/java/com/consumer/Consumer.java", Language: "java"}, consumerBody)
	providerFacts := ExtractJavaSymbolFacts(provider, providerBody, WorkspaceIndex{})
	consumerFacts := ExtractJavaSymbolFacts(consumer, consumerBody, WorkspaceIndex{})
	facts := ProjectSymbolFacts{
		Declarations: append(providerFacts.Declarations, consumerFacts.Declarations...),
		References:   append(providerFacts.References, consumerFacts.References...),
	}
	facts = FinalizeProjectSymbolFacts(nil, WorkspaceIndex{}, facts)
	providerInner := assertRichDeclaration(t, facts.Declarations, "class", "com.provider.Outer.Inner", "")
	consumerOwner := assertRichDeclaration(t, facts.Declarations, "class", "com.consumer.Consumer", "")
	call := assertJavaReference(t, facts.References, "calls_method_owner", "com.provider.Outer.Inner", 5)
	if call.ToSymbolID != providerInner.ID || call.FromSymbolID != consumerOwner.ID {
		t.Fatalf("qualified receiver chain resolved incorrectly: %#v", call)
	}
}

func TestJavaOneLineMethodPreservesLiteralHTTPAndBareCallArguments(t *testing.T) {
	body := `class OneLine {
    void call() { get("/users"); helper("value"); }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/OneLine.java", Language: "java"}, body)
	if len(source.Methods) != 1 {
		t.Fatalf("one-line method extraction = %#v, want one method", source.Methods)
	}
	method := source.Methods[0]
	if len(method.HTTPRequests) != 1 || method.HTTPRequests[0].HTTPMethod != "GET" || method.HTTPRequests[0].Path != "/users" {
		t.Fatalf("one-line HTTP extraction = %#v, want GET /users", method.HTTPRequests)
	}
	for _, call := range method.Calls {
		if call.Method == "helper" && reflect.DeepEqual(call.Arguments, []string{`"value"`}) {
			return
		}
	}
	t.Fatalf("one-line bare call arguments = %#v, want helper(\"value\")", method.Calls)
}

func TestJavaAccumulatedTypeHeaderFinalizesBeforeFollowingTopLevelType(t *testing.T) {
	body := `package com.weka.users;

class First
{}
class Second {}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Types.java", Language: "java"}, body)
	if len(source.Types) != 2 {
		t.Fatalf("type extraction = %#v, want two top-level types", source.Types)
	}
	first, second := source.Types[0], source.Types[1]
	if first.QualifiedName != "com.weka.users.First" || first.Owner != "" || first.EndLine != 4 {
		t.Fatalf("accumulated first type = %#v, want top-level type ending on line 4", first)
	}
	if second.QualifiedName != "com.weka.users.Second" || second.Owner != "" {
		t.Fatalf("following type inherited stale owner: %#v", second)
	}
}

func TestJavaReceiverChainsResolveFieldsParametersNestedAndQualifiedTypes(t *testing.T) {
	body := `package com.weka.users;

class Service { void run() {} }
class Holder {
    Service service;
}
class Base {
    Service inherited;
}
class Outer {
    static Service service;
    static class Inner { static void run() {} }
}
class Consumer extends Base {
    Service field;
    void call(Service parameter, Holder holder) {
        this.field.run();
        super.inherited.run();
        parameter.run();
        holder.service.run();
        Outer.service.run();
        Outer.Inner.run();
        com.weka.users.Outer.Inner.run();
    }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Consumer.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	service := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Service", "")
	inner := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Outer.Inner", "")
	consumer := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Consumer", "")
	for _, line := range []int{17, 18, 19, 20, 21} {
		call := assertJavaReference(t, facts.References, "calls_method_owner", "com.weka.users.Service", line)
		if call.ToSymbolID != service.ID || call.FromSymbolID != consumer.ID || call.Resolution != SymbolResolutionExact {
			t.Fatalf("semantic receiver at line %d = %#v, want exact Service from Consumer", line, call)
		}
	}
	for _, line := range []int{22, 23} {
		call := assertJavaReference(t, facts.References, "calls_method_owner", "com.weka.users.Outer.Inner", line)
		if call.ToSymbolID != inner.ID || call.Resolution != SymbolResolutionExact {
			t.Fatalf("nested/static receiver at line %d = %#v, want exact Outer.Inner", line, call)
		}
	}
	for _, reference := range facts.References {
		if reference.Type == "calls_method_owner" && strings.Contains(reference.TargetQualifiedName, "Outer.service") {
			t.Fatalf("member access was treated as a nested type: %#v", reference)
		}
	}
}

func TestJavaInlineCommentsCannotCreateHTTPOrCallArguments(t *testing.T) {
	body := `class Inline {
    void call() { helper("real" /*, "fake" */); get("/real"); /* get("/block"); helper("block"); */ } // get("/line"); helper("line");
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/Inline.java", Language: "java"}, body)
	if len(source.Methods) != 1 {
		t.Fatalf("inline method extraction = %#v, want one method", source.Methods)
	}
	method := source.Methods[0]
	if len(method.HTTPRequests) != 1 || method.HTTPRequests[0].Path != "/real" {
		t.Fatalf("inline comment HTTP leakage = %#v, want only /real", method.HTTPRequests)
	}
	var helperCalls []JavaCallRecord
	for _, call := range method.Calls {
		if call.Method == "helper" {
			helperCalls = append(helperCalls, call)
		}
	}
	if len(helperCalls) != 1 || !reflect.DeepEqual(helperCalls[0].Arguments, []string{`"real"`}) {
		t.Fatalf("inline comment call leakage = %#v, want helper(\"real\")", helperCalls)
	}
}

func TestJavaUppercaseFieldWinsOverNestedTypeReceiver(t *testing.T) {
	body := `package com.weka.users;

class Service { void run() {} }
class Outer {
    static Service SERVICE;
    static class SERVICE { static void run() {} }
}
class Consumer {
    void call() { Outer.SERVICE.run(); }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Consumer.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	service := assertRichDeclaration(t, facts.Declarations, "class", "com.weka.users.Service", "")
	call := assertJavaReference(t, facts.References, "calls_method_owner", "com.weka.users.Service", 9)
	if call.ToSymbolID != service.ID || call.Resolution != SymbolResolutionExact {
		t.Fatalf("uppercase field receiver = %#v, want exact Service", call)
	}
	for _, reference := range facts.References {
		if reference.Type == "calls_method_owner" && reference.TargetQualifiedName == "com.weka.users.Outer.SERVICE" {
			t.Fatalf("uppercase field was treated as nested type: %#v", reference)
		}
	}
}

func TestJavaArrayAndVarargsReceiversDoNotResolveToElementType(t *testing.T) {
	body := `package com.weka.users;

class Service { void clone() {} }
class Consumer {
    Service[] services;
    void call(Service... parameters) {
        services.clone();
        parameters.clone();
    }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Consumer.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	for _, line := range []int{7, 8} {
		var found bool
		for _, reference := range facts.References {
			if reference.Type != "calls_method_owner" || reference.Line != line {
				continue
			}
			found = true
			if reference.Resolution != SymbolResolutionUnresolved || reference.ToSymbolID != "" || reference.TargetQualifiedName == "com.weka.users.Service" {
				t.Fatalf("array receiver at line %d resolved to element type: %#v", line, reference)
			}
		}
		if !found {
			t.Fatalf("missing unresolved array receiver fact at line %d in %#v", line, facts.References)
		}
	}
}

func TestJavaInitializedFieldProvidesSameFileReceiverMetadata(t *testing.T) {
	body := `package com.weka.users;

class Service { void run() {} }
class Outer {
    static Service SERVICE = new Service();
    static class SERVICE { static void run() {} }
}
class Consumer {
    void call() { Outer.SERVICE.run(); }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Consumer.java", Language: "java"}, body)
	var initializedField bool
	for _, field := range source.Fields {
		if field.Owner == "Outer" && field.Name == "SERVICE" && field.Type == "Service" {
			initializedField = true
		}
	}
	if !initializedField {
		t.Fatalf("initialized field metadata missing: %#v", source.Fields)
	}
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	call := assertJavaReference(t, facts.References, "calls_method_owner", "com.weka.users.Service", 9)
	if call.Resolution != SymbolResolutionExact {
		t.Fatalf("initialized field receiver = %#v, want exact Service", call)
	}
}

func TestRunResolvesImportedInitializedFieldBeforeNestedType(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main/java/com/consumer/Consumer.java", `package com.consumer;

import com.provider.Outer;
class Consumer {
    void call() { Outer.SERVICE.run(); }
}
`)
	writeFile(t, root, "src/main/java/com/provider/Outer.java", `package com.provider;

class Service { void run() {} }
public class Outer {
    public static Service SERVICE = new Service();
    public static class SERVICE { public static void run() {} }
}
`)
	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	var relations []RichRelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations-full.json"), &relations)
	call := assertJavaReference(t, relations, "calls_method_owner", "com.provider.Service", 5)
	if call.Resolution != SymbolResolutionExact || call.ToSymbolID == "" {
		t.Fatalf("cross-file initialized field receiver = %#v, want exact provider Service", call)
	}
	for _, reference := range relations {
		if reference.Type == "calls_method_owner" && reference.TargetQualifiedName == "com.provider.Outer.SERVICE" {
			t.Fatalf("cross-file field was finalized as nested type: %#v", reference)
		}
	}
}

func TestRunLeavesUnprovenImportedUppercaseReceiverUnresolved(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main/java/com/consumer/Consumer.java", `package com.consumer;

import com.provider.Outer;
class Consumer {
    void call() { Outer.SERVICE.run(); }
}
`)
	writeFile(t, root, "src/main/java/com/provider/Outer.java", `package com.provider;

@interface Marker {}
class Service { void run() {} }
public class Outer {
    public static @Marker Service SERVICE = new Service();
    public static class SERVICE { public static void run() {} }
}
`)
	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	var relations []RichRelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations-full.json"), &relations)
	call := assertJavaReference(t, relations, "calls_method_owner", "com.provider.Outer.SERVICE", 5)
	if call.Resolution != SymbolResolutionUnresolved || call.ToSymbolID != "" {
		t.Fatalf("unproven imported uppercase receiver = %#v, want unresolved collision", call)
	}
}

func TestJavaScopedArrayTypeVariableReceiversEmitNoProjectCallFact(t *testing.T) {
	body := `package com.weka.users;

class Generic<T> {
    T[] values;
    void call(T... parameters) {
        values.clone();
        parameters.clone();
    }
}
`
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/weka/users/Generic.java", Language: "java"}, body)
	facts := ExtractJavaSymbolFacts(source, body, WorkspaceIndex{})
	for _, reference := range facts.References {
		if reference.Type == "calls_method_owner" && (reference.Line == 6 || reference.Line == 7) {
			t.Fatalf("scoped array type variable emitted project call fact: %#v", reference)
		}
	}
}
