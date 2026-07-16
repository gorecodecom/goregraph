package scan

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

func TestExtractScriptCanonicalDeclarations(t *testing.T) {
	body := `
export class UserService {}
class LocalService {}
export default class DefaultService {}
export interface User { id: string }
export type UserID = string
export enum Role { Admin }
export function loadUser() {}
export const saveUser = async () => {}
export function UserCard() { return <div /> }
const LocalCard = () => <span />
`
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/components/UserCard.tsx", Language: "typescript"}, body)

	for _, want := range []struct {
		kind       string
		qualified  string
		exportName string
	}{
		{kind: "class", qualified: "src/components/UserCard#UserService", exportName: "UserService"},
		{kind: "class", qualified: "src/components/UserCard#LocalService"},
		{kind: "class", qualified: "src/components/UserCard#DefaultService", exportName: "default"},
		{kind: "interface", qualified: "src/components/UserCard#User", exportName: "User"},
		{kind: "type", qualified: "src/components/UserCard#UserID", exportName: "UserID"},
		{kind: "enum", qualified: "src/components/UserCard#Role", exportName: "Role"},
		{kind: "function", qualified: "src/components/UserCard#loadUser", exportName: "loadUser"},
		{kind: "function", qualified: "src/components/UserCard#saveUser", exportName: "saveUser"},
		{kind: "component", qualified: "src/components/UserCard#UserCard", exportName: "UserCard"},
		{kind: "component", qualified: "src/components/UserCard#LocalCard"},
	} {
		declaration := assertRichDeclaration(t, facts.Declarations, want.kind, want.qualified, "")
		if declaration.ExportName != want.exportName {
			t.Fatalf("%s export = %q, want %q", want.qualified, declaration.ExportName, want.exportName)
		}
		if declaration.Module != "src/components/UserCard" {
			t.Fatalf("%s module = %q", want.qualified, declaration.Module)
		}
	}
}

func TestExtractScriptImportAndReexportShapes(t *testing.T) {
	body := `
import DefaultCard from "./DefaultCard";
import {
  UserCard,
  type CardProps as Props,
} from "./cards";
import * as api from "@weka/api";
import type { User } from "@models/user";
import type DefaultUser from "./DefaultUser";
import "./setup";
const lazy = import("./lazy");
const unknown = import(modulePath);
export { UserService as Service } from "./service";
export * from "./models";
export type { UserRole } from "./roles";
`
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/App.tsx", Language: "typescript"}, body)

	for _, want := range []struct {
		kind, module, exportName string
	}{
		{kind: "imports_value", module: "./DefaultCard", exportName: "default"},
		{kind: "imports_value", module: "./cards", exportName: "UserCard"},
		{kind: "imports_type", module: "./cards", exportName: "CardProps"},
		{kind: "imports_namespace", module: "@weka/api", exportName: "*"},
		{kind: "imports_type", module: "@models/user", exportName: "User"},
		{kind: "imports_type", module: "./DefaultUser", exportName: "default"},
		{kind: "imports_module", module: "./setup", exportName: "*"},
		{kind: "imports_module", module: "./lazy", exportName: "*"},
		{kind: "reexports_value", module: "./service", exportName: "UserService"},
		{kind: "reexports_all", module: "./models", exportName: "*"},
		{kind: "reexports_type", module: "./roles", exportName: "UserRole"},
	} {
		assertScriptReference(t, facts.References, want.kind, want.module, want.exportName)
	}

	dynamic := assertScriptReference(t, facts.References, "imports_module", "", "")
	if dynamic.Resolution == SymbolResolutionExact || dynamic.Confidence == string(ConfidenceExact) || dynamic.Reason == "" {
		t.Fatalf("computed import must remain explicitly unresolved: %#v", dynamic)
	}
}

func TestScriptSameLineAliasesRemainDistinctAcrossFactStages(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	appFacts := ExtractScriptSymbolFacts(files[0], `
import { foo as a, foo as b } from "./api";
export function useAliases() { a(); b(); }
`)
	apiFacts := ExtractScriptSymbolFacts(files[1], `
export function foo() {}
export { foo as a, foo as b };
`)

	assertAliases := func(stage string, references []RichRelationRecord, kind, module, exportName string, wantLocal bool) {
		t.Helper()
		matches := scriptReferences(t, references, kind, module, exportName)
		if len(matches) != 2 {
			t.Errorf("%s %s aliases = %#v, want two references", stage, kind, matches)
			return
		}
		aliases := map[string]bool{}
		ids := map[string]bool{}
		for _, reference := range matches {
			alias := reference.scriptExportAlias
			if wantLocal {
				alias = reference.scriptLocalName
			}
			aliases[alias] = true
			ids[reference.ID] = true
		}
		if !aliases["a"] || !aliases["b"] || len(aliases) != 2 || len(ids) != 2 {
			t.Errorf("%s %s identities = %#v, want distinct a/b aliases and IDs", stage, kind, matches)
		}
	}

	assertAliases("extracted", appFacts.References, "imports_value", "./api", "foo", true)
	assertAliases("extracted", apiFacts.References, "exports_local", "src/api", "foo", false)

	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, appFacts)
	MergeProjectSymbolFacts(&facts, apiFacts)
	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	assertAliases("resolved", resolved.References, "imports_value", "./api", "foo", true)
	assertAliases("resolved", resolved.References, "exports_local", "src/api", "foo", false)

	finalized := FinalizeProjectSymbolFacts(files, WorkspaceIndex{}, resolved)
	assertAliases("finalized", finalized.References, "imports_value", "./api", "foo", true)
	assertAliases("finalized", finalized.References, "exports_local", "src/api", "foo", false)
}

func TestScriptTopLevelSameLineAliasedUsagesRemainDistinct(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { foo as a, foo as b } from "./api";
a(); b();
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function foo() {}`))

	extracted := scriptReferences(t, facts.References, "calls_export", "./api", "foo")
	if len(extracted) != 2 {
		t.Fatalf("top-level extracted alias usages = %#v, want two references", extracted)
	}
	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	references := scriptReferences(t, resolved.References, "calls_export", "./api", "foo")
	if len(references) != 2 {
		t.Fatalf("top-level resolved alias usages = %#v, want two references", references)
	}
	aliases := map[string]bool{}
	for _, reference := range references {
		if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" || reference.FromSymbolID != "" {
			t.Fatalf("top-level alias usage did not resolve exactly without an owner: %#v", reference)
		}
		aliases[reference.scriptLocalName] = true
	}
	if !aliases["a"] || !aliases["b"] || len(aliases) != 2 {
		t.Fatalf("top-level alias usage identities = %#v, want a and b", references)
	}
}

func TestScriptVarShadowsAcrossBlocksButLetAndConstDoNot(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { foo, bar, baz, qux } from "./api";
export function App(ready) {
  if (ready) {
    var foo = localFoo;
    let bar = localBar;
    const baz = localBaz;
  }
  foo();
  bar();
  baz();
}
if (ready) {
  var qux = localQux;
}
qux();
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `
export function foo() {}
export function bar() {}
export function baz() {}
export function qux() {}
`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	foo := scriptReferenceAtLine(t, resolved.References, "calls_export", 9)
	if foo.Resolution == SymbolResolutionExact || foo.ToSymbolID != "" || !strings.Contains(foo.Reason, "shadow") {
		t.Fatalf("function-scoped var did not shadow imported call: %#v", foo)
	}
	qux := scriptReferenceAtLine(t, resolved.References, "calls_export", 16)
	if qux.Resolution == SymbolResolutionExact || qux.ToSymbolID != "" || !strings.Contains(qux.Reason, "shadow") {
		t.Fatalf("module-scoped var did not shadow imported call: %#v", qux)
	}
	for _, name := range []string{"bar", "baz"} {
		reference := assertScriptReference(t, resolved.References, "calls_export", "./api", name)
		if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" {
			t.Fatalf("block-scoped %s leaked outside its block: %#v", name, reference)
		}
	}
}

func TestScriptStarReexportDoesNotForwardDefault(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/barrel.ts", Language: "typescript"},
		{Path: "src/service.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import DefaultService, { named } from "./barrel";
export function App() { new DefaultService(); named(); }
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export * from "./service";`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `
export default class Service {}
export function named() {}
`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	defaultImport := assertScriptReference(t, resolved.References, "imports_value", "./barrel", "default")
	if defaultImport.Resolution == SymbolResolutionExact || defaultImport.ToSymbolID != "" {
		t.Fatalf("star-only barrel forwarded default export: %#v", defaultImport)
	}
	named := assertScriptReference(t, resolved.References, "imports_value", "./barrel", "named")
	if named.Resolution != SymbolResolutionExact || named.ToSymbolID == "" {
		t.Fatalf("star barrel did not forward named export: %#v", named)
	}
}

func TestScriptTypeAndValueCapabilitiesSurviveReexportsAndUsages(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.tsx", Language: "typescript"},
		{Path: "src/barrel.ts", Language: "typescript"},
		{Path: "src/types.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import type { Shape, FactoryType, Box as TypeBox } from "./barrel";
import { ShapeValue, Box, Role, Factory, Dual } from "./barrel";
export function App() {
  const shape: Shape = value;
  const shapeValue: ShapeValue = value;
  const box: Box = new Box();
  const role: Role = value;
  const dual: Dual = value;
  const invalidFactoryType: FactoryType = value;
  Factory();
  Dual();
  FactoryType();
  new TypeBox();
  new ShapeValue();
  return <ShapeValue />;
}
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `
export type { Shape, Factory as FactoryType } from "./types";
export { Shape as ShapeValue, Box, Role, Factory, Dual } from "./types";
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `
export interface Shape {}
export class Box {}
export enum Role { Admin }
export function Factory() {}
export interface Dual {}
export function Dual() {}
`))

	for qualified, capability := range map[string]scriptSymbolCapability{
		"src/types#Shape":   scriptTypeCapability,
		"src/types#Box":     scriptTypeCapability | scriptValueCapability,
		"src/types#Role":    scriptTypeCapability | scriptValueCapability,
		"src/types#Factory": scriptValueCapability,
	} {
		declaration := scriptDeclarationByQualified(t, facts.Declarations, qualified)
		if declaration.scriptCapability != capability {
			t.Fatalf("%s capability = %d, want %d", qualified, declaration.scriptCapability, capability)
		}
	}
	typeImport := assertScriptReference(t, facts.References, "imports_type", "./barrel", "Box")
	if !typeImport.scriptTypeOnly || typeImport.scriptLocalName != "TypeBox" {
		t.Fatalf("type-only import metadata = %#v", typeImport)
	}
	typeReexport := assertScriptReference(t, facts.References, "reexports_type", "./types", "Factory")
	if !typeReexport.scriptTypeOnly || typeReexport.scriptExportAlias != "FactoryType" {
		t.Fatalf("type-only re-export metadata = %#v", typeReexport)
	}

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	for _, want := range []struct {
		kind, module, exportName string
		exact                    bool
	}{
		{kind: "imports_type", module: "./barrel", exportName: "Shape", exact: true},
		{kind: "imports_type", module: "./barrel", exportName: "Box", exact: true},
		{kind: "imports_type", module: "./barrel", exportName: "FactoryType"},
		{kind: "reexports_type", module: "./types", exportName: "Shape", exact: true},
		{kind: "reexports_type", module: "./types", exportName: "Factory"},
	} {
		reference := assertScriptReference(t, resolved.References, want.kind, want.module, want.exportName)
		if want.exact && (reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "") {
			t.Fatalf("%s %s capability binding did not resolve: %#v", want.kind, want.exportName, reference)
		}
		if !want.exact && (reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "") {
			t.Fatalf("%s %s capability binding resolved incorrectly: %#v", want.kind, want.exportName, reference)
		}
	}
	assertExact := func(kind, exportName string) {
		t.Helper()
		reference := assertScriptReference(t, resolved.References, kind, "./barrel", exportName)
		if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" {
			t.Fatalf("%s %s capability did not resolve: %#v", kind, exportName, reference)
		}
	}
	assertUnresolved := func(kind, exportName string) {
		t.Helper()
		reference := assertScriptReference(t, resolved.References, kind, "./barrel", exportName)
		if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" {
			t.Fatalf("%s %s crossed type/value capability: %#v", kind, exportName, reference)
		}
	}

	assertExact("type_reference", "Shape")
	assertExact("type_reference", "ShapeValue")
	assertExact("type_reference", "Box")
	assertExact("type_reference", "Role")
	assertExact("type_reference", "Dual")
	assertExact("calls_export", "Factory")
	assertExact("calls_export", "Dual")
	assertUnresolved("type_reference", "FactoryType")
	assertUnresolved("calls_export", "FactoryType")
	assertUnresolved("instantiates", "ShapeValue")
	assertUnresolved("renders_component", "ShapeValue")

	boxInstances := scriptReferences(t, resolved.References, "instantiates", "./barrel", "Box")
	if len(boxInstances) != 2 {
		t.Fatalf("Box value/type-only instances = %#v, want two references", boxInstances)
	}
	for _, reference := range boxInstances {
		switch reference.scriptLocalName {
		case "Box":
			if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" {
				t.Fatalf("value-capable Box did not instantiate: %#v", reference)
			}
		case "TypeBox":
			if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" {
				t.Fatalf("type-only Box instantiated as a value: %#v", reference)
			}
		default:
			t.Fatalf("unexpected Box binding: %#v", reference)
		}
	}

	dualType := assertScriptReference(t, resolved.References, "type_reference", "./barrel", "Dual")
	dualValue := assertScriptReference(t, resolved.References, "calls_export", "./barrel", "Dual")
	if dualType.ToSymbolID == dualValue.ToSymbolID {
		t.Fatalf("dual namespace usages selected the same declaration: type=%#v value=%#v", dualType, dualValue)
	}
	for _, kind := range []string{"imports_value", "reexports_value"} {
		module := "./barrel"
		if kind == "reexports_value" {
			module = "./types"
		}
		reference := assertScriptReference(t, resolved.References, kind, module, "Dual")
		if reference.Resolution != SymbolResolutionAmbiguous || len(reference.CandidateSymbolIDs) != 2 {
			t.Fatalf("%s Dual binding did not preserve both namespaces: %#v", kind, reference)
		}
	}
}

func TestScriptNamespaceUsageIdentityUsesReceiverAlias(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import * as a from "./api";
import * as b from "./api";
a.foo(); b.foo();
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function foo() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	references := scriptReferences(t, resolved.References, "calls_export", "./api", "foo")
	if len(references) != 2 {
		t.Fatalf("namespace receiver usages = %#v, want two references", references)
	}
	aliases := map[string]bool{}
	for _, reference := range references {
		if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" {
			t.Fatalf("namespace receiver did not resolve exactly: %#v", reference)
		}
		aliases[reference.scriptLocalName] = true
	}
	if !aliases["a"] || !aliases["b"] || len(aliases) != 2 {
		t.Fatalf("namespace usage identities = %#v, want receiver aliases a and b", references)
	}
}

func TestExtractScriptIgnoresCommentsStringsAndTemplates(t *testing.T) {
	body := `
// export class CommentFake {}
/* import { BlockFake } from "./block"; */
const quoted = "export interface StringFake {}";
const templated = ` + "`export function TemplateFake() { import(\"./fake\") }`" + `;
export class RealService {}
`
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/real.ts", Language: "typescript"}, body)
	if len(facts.Declarations) != 1 || facts.Declarations[0].QualifiedName != "src/real#RealService" {
		t.Fatalf("lexically sanitized declarations = %#v", facts.Declarations)
	}
	if len(facts.References) != 0 {
		t.Fatalf("lexically sanitized references = %#v", facts.References)
	}
}

func TestExtractScriptMasksRegexLiteralContents(t *testing.T) {
	body := `
import { loadUser } from "./api";
const escaped = /; export class RegexFake {} import(".\/fake") loadUser()/gi;
const charClass = /[/] ; export function OtherFake() {} loadUser()/m;
const logical = ready && /; export interface LogicalFake {} loadUser()/;
if (ready) /; export type ControlFake = string; loadUser()/.test(value);
loadUser();
`
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/real.ts", Language: "typescript"}, body)
	for _, declaration := range facts.Declarations {
		if declaration.Name == "RegexFake" || declaration.Name == "OtherFake" ||
			declaration.Name == "LogicalFake" || declaration.Name == "ControlFake" {
			t.Fatalf("regex literal created declaration: %#v", declaration)
		}
	}
	if references := scriptReferences(t, facts.References, "calls_export", "./api", "loadUser"); len(references) != 1 || references[0].Line != 7 {
		t.Fatalf("regex literal call extraction = %#v, want only real line-7 call", references)
	}
	for _, reference := range facts.References {
		if reference.TargetModule == "./fake" {
			t.Fatalf("regex literal created import: %#v", reference)
		}
	}
}

func TestExtractScriptMasksRegexAfterStatementBlockButPreservesDivision(t *testing.T) {
	body := `
import { loadUser } from "./api";
if (ready) {}
/; export class BlockRegexFake {} loadUser()/.test(value);
const objectDivision = {} / loadUser() / divisor;
const identifierDivision = total / loadUser() / divisor;
`
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/real.ts", Language: "typescript"}, body)
	for _, declaration := range facts.Declarations {
		if declaration.Name == "BlockRegexFake" {
			t.Fatalf("block-following regex created declaration: %#v", declaration)
		}
	}
	references := scriptReferences(t, facts.References, "calls_export", "./api", "loadUser")
	if len(references) != 2 || references[0].Line == 4 || references[1].Line == 4 {
		t.Fatalf("block regex or division call extraction = %#v, want only division calls", references)
	}
}

func TestExtractScriptMasksRegexAfterElseAndFinallyBlocks(t *testing.T) {
	body := `
import { loadUser } from "./api";
if (ready) {
  work();
} else
{
}
/; export class ElseRegexFake {} loadUser()/.test(value);
try {
  work();
} finally
  {
  }
/; export class FinallyRegexFake {} loadUser()/.test(value);
const objectDivision = { finally: true } / loadUser() / divisor;
`
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/real.ts", Language: "typescript"}, body)
	for _, declaration := range facts.Declarations {
		if declaration.Name == "ElseRegexFake" || declaration.Name == "FinallyRegexFake" {
			t.Fatalf("else/finally regex created declaration: %#v", declaration)
		}
	}
	references := scriptReferences(t, facts.References, "calls_export", "./api", "loadUser")
	if len(references) != 1 || references[0].Line != 15 {
		t.Fatalf("else/finally regex or object division extraction = %#v, want only line-15 division call", references)
	}
}

func TestExtractScriptMasksRegexAfterBareAndLabelledBlocks(t *testing.T) {
	body := `
import { loadUser } from "./api";
{
}
/; export class BareRegexFake {} loadUser()/.test(value);
retry:
{
}
/; export class LabelRegexFake {} loadUser()/.test(value);
const objectDivision = { retry: true } / loadUser() / divisor;
const nestedObjectDivision = { retry: {} / loadUser() / divisor };
`
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/real.ts", Language: "typescript"}, body)
	for _, declaration := range facts.Declarations {
		if declaration.Name == "BareRegexFake" || declaration.Name == "LabelRegexFake" {
			t.Fatalf("bare/labelled block regex created declaration: %#v", declaration)
		}
	}
	references := scriptReferences(t, facts.References, "calls_export", "./api", "loadUser")
	if len(references) != 2 {
		t.Fatalf("bare/labelled regex or object division extraction = %#v, want two division calls", references)
	}
	for _, reference := range references {
		if reference.Line != 10 && reference.Line != 11 {
			t.Fatalf("bare/labelled regex emitted call at line %d: %#v", reference.Line, references)
		}
	}
}

func TestExtractScriptMasksRegexAfterSemicolonlessStatementBlocks(t *testing.T) {
	body := `
import { loadUser } from "./api";
work()
{
}
/; export class SemicolonlessBareFake {} loadUser()/.test(value);
work()
retry:
{
}
/; export class SemicolonlessLabelFake {} loadUser()/.test(value);
const objectDivision = { retry: true } / loadUser() / divisor;
`
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/real.ts", Language: "typescript"}, body)
	for _, declaration := range facts.Declarations {
		if declaration.Name == "SemicolonlessBareFake" || declaration.Name == "SemicolonlessLabelFake" {
			t.Fatalf("semicolonless statement block regex created declaration: %#v", declaration)
		}
	}
	references := scriptReferences(t, facts.References, "calls_export", "./api", "loadUser")
	if len(references) != 1 || references[0].Line != 12 {
		t.Fatalf("semicolonless block regex or object division extraction = %#v, want only line-12 division call", references)
	}
}

func TestExtractScriptMasksRegexAfterOptionalCatchAndTypedFunctionBlocks(t *testing.T) {
	body := `
import { loadUser } from "./api";
import type { User } from "./User";
try {
  work();
} catch {
}
/; export class CatchFake {} loadUser()/.test(value);
function load(): User {
  return value;
}
/; export class TypedFunctionFake {} loadUser()/.test(value);
const objectDivision = { catch: true } / loadUser() / divisor;
`
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/real.ts", Language: "typescript"}, body)
	for _, declaration := range facts.Declarations {
		if declaration.Name == "CatchFake" || declaration.Name == "TypedFunctionFake" {
			t.Fatalf("optional-catch or typed-function regex created declaration: %#v", declaration)
		}
	}
	references := scriptReferences(t, facts.References, "calls_export", "./api", "loadUser")
	if len(references) != 1 || references[0].Line != 13 {
		t.Fatalf("optional-catch or typed-function regex extraction = %#v, want only line-13 division call", references)
	}
}

func TestScriptReturnTypesRequireFunctionLikeSignature(t *testing.T) {
	file := FileRecord{Path: "src/types.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `
import type { User } from "./User";
function makeUser(): User { throw new Error(); }
class Factory {
  makeUser(): User { throw new Error(); }
}
const makeUser = (): User => { throw new Error(); };
const misleading = condition ? (makeValue()) : User;
`)

	references := scriptReferences(t, facts.References, "type_reference", "./User", "User")
	if len(references) != 3 {
		t.Fatalf("return type references = %#v, want only function, method, and arrow signatures", references)
	}
	for _, reference := range references {
		if reference.Line == 8 {
			t.Fatalf("ternary lookalike emitted return type reference: %#v", reference)
		}
	}
}

func TestScriptReturnTypeBelongsToSameArrowParameterList(t *testing.T) {
	file := FileRecord{Path: "src/types.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `
import type { User } from "./User";
const misleading = condition ? makeValue() : User => User;
`)

	if references := scriptReferences(t, facts.References, "type_reference", "./User", "User"); len(references) != 0 {
		t.Fatalf("ternary expression emitted arrow return type: %#v", references)
	}
}

func TestScriptParameterTypesRequireProvenParameterSpan(t *testing.T) {
	file := FileRecord{Path: "src/types.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `
import type { User } from "./User";
const value = { first: source, model: User };
function consume(model: User) {}
class Consumer {
  consume(model: User) {}
}
const consume = (model: User) => model;
`)

	references := scriptReferences(t, facts.References, "type_reference", "./User", "User")
	if len(references) != 3 {
		t.Fatalf("parameter type references = %#v, want only function, method, and arrow parameters", references)
	}
	for _, reference := range references {
		if reference.Line == 3 {
			t.Fatalf("object literal value emitted parameter type relation: %#v", reference)
		}
	}
}

func TestScriptParameterTypesIgnoreDefaultValueExpressions(t *testing.T) {
	file := FileRecord{Path: "src/types.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `
import type { Nested, User } from "./User";
function consume(value = { first: source, model: Nested }) {}
const consume = (value = { first: source, model: Nested }) => value;
function consumeTyped(value: User = { first: source, model: Nested }) {}
const consumeTyped = (value: User = { first: source, model: Nested }) => value;
`)

	references := scriptReferences(t, facts.References, "type_reference", "./User", "User")
	if len(references) != 2 {
		t.Fatalf("valid parameter annotations = %#v, want line-5 and line-6 User annotations", references)
	}
	if nested := scriptReferences(t, facts.References, "type_reference", "./User", "Nested"); len(nested) != 0 {
		t.Fatalf("default expressions emitted parameter type relations: %#v", nested)
	}
}

func TestResolveScriptRelativeAliasAndWorkspaceImports(t *testing.T) {
	files := []FileRecord{
		{Path: "apps/web/src/App.tsx", Language: "typescript"},
		{Path: "apps/web/src/components/UserCard.tsx", Language: "typescript"},
		{Path: "apps/web/src/models/index.ts", Language: "typescript"},
		{Path: "packages/shared/src/index.ts", Language: "typescript"},
		{Path: "packages/ui/src/user.ts", Language: "typescript"},
	}
	bodies := map[string]string{
		"apps/web/src/App.tsx": `
import { UserCard } from "./components/UserCard";
import type { User } from "@models";
import { sharedValue } from "@weka/shared";
import { UserService } from "@weka/ui/user";
export function App() { return <UserCard /> }
`,
		"apps/web/src/components/UserCard.tsx": `export function UserCard() { return <div /> }`,
		"apps/web/src/models/index.ts":         `export interface User { id: string }`,
		"packages/shared/src/index.ts":         `export const sharedValue = () => 1`,
		"packages/ui/src/user.ts":              `export class UserService {}`,
	}
	var facts ProjectSymbolFacts
	for _, file := range files {
		MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(file, bodies[file.Path]))
	}
	packages := []NodePackageRecord{
		{Path: "apps/web/package.json", Name: "web", Dependencies: []string{"@weka/shared", "@weka/ui"}},
		{Path: "packages/shared/package.json", Name: "@weka/shared", Types: "src/index.ts"},
		{Path: "packages/ui/package.json", Name: "@weka/ui", Exports: map[string][]string{"./user": {"./src/user.ts"}}},
	}
	configs := map[string]ScriptResolutionConfig{
		"apps/web/tsconfig.json": {BaseURL: ".", Paths: map[string][]string{"@models": {"src/models"}}},
	}

	resolved := ResolveScriptSymbolFacts(files, packages, configs, facts)

	wants := []struct {
		module, exportName, qualified string
	}{
		{module: "./components/UserCard", exportName: "UserCard", qualified: "apps/web/src/components/UserCard#UserCard"},
		{module: "@models", exportName: "User", qualified: "apps/web/src/models/index#User"},
		{module: "@weka/shared", exportName: "sharedValue", qualified: "packages/shared/src/index#sharedValue"},
		{module: "@weka/ui/user", exportName: "UserService", qualified: "packages/ui/src/user#UserService"},
	}
	for _, want := range wants {
		kind := "imports_value"
		if want.exportName == "User" {
			kind = "imports_type"
		}
		reference := assertScriptReference(t, resolved.References, kind, want.module, want.exportName)
		declaration := scriptDeclarationByQualified(t, resolved.Declarations, want.qualified)
		if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID != declaration.ID {
			t.Fatalf("%s#%s resolution = %#v, want exact %s", want.module, want.exportName, reference, declaration.ID)
		}
	}
}

func TestResolveScriptRootWorkspacePackageAndStaticModuleImport(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/lazy.ts", Language: "typescript"},
		{Path: "packages/ui/src/index.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import "./lazy";
import { UserService } from "@weka/ui";
export function App() { return new UserService() }
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function lazy() {}`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `export class UserService {}`))
	packages := []NodePackageRecord{
		{Path: "package.json", Name: "app", Dependencies: []string{"@weka/ui"}},
		{Path: "packages/ui/package.json", Name: "@weka/ui", Types: "src/index.ts"},
	}

	resolved := ResolveScriptSymbolFacts(files, packages, nil, facts)
	workspaceImport := assertScriptReference(t, resolved.References, "imports_value", "@weka/ui", "UserService")
	provider := scriptDeclarationByQualified(t, resolved.Declarations, "packages/ui/src/index#UserService")
	if workspaceImport.Resolution != SymbolResolutionExact || workspaceImport.ToSymbolID != provider.ID {
		t.Fatalf("root workspace import = %#v", workspaceImport)
	}
	moduleImport := assertScriptReference(t, resolved.References, "imports_module", "./lazy", "*")
	if moduleImport.Resolution != SymbolResolutionExact || moduleImport.TargetQualifiedName != "src/lazy" || !moduleImport.Internal {
		t.Fatalf("static module import = %#v", moduleImport)
	}
}

func TestScriptReexportsPreserveOriginalDeclaration(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/services/index.ts", Language: "typescript"},
		{Path: "src/core/UserService.ts", Language: "typescript"},
	}
	bodies := map[string]string{
		"src/App.ts":              `import { Service } from "./services"; export function run() { return new Service() }`,
		"src/services/index.ts":   `export { UserService as Service } from "../core/UserService";`,
		"src/core/UserService.ts": `export class UserService {}`,
	}
	var facts ProjectSymbolFacts
	for _, file := range files {
		MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(file, bodies[file.Path]))
	}

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "./services", "Service")
	declaration := scriptDeclarationByQualified(t, resolved.Declarations, "src/core/UserService#UserService")
	if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID != declaration.ID || reference.TargetQualifiedName != declaration.QualifiedName {
		t.Fatalf("re-export resolution = %#v, want original declaration %#v", reference, declaration)
	}
}

func TestScriptLocalExportClausesAndDefaultNamesResolve(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/service.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import DefaultService, { Service, factory } from "./service";
export function App() {
  factory();
  new Service();
  new DefaultService();
}
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `
class LocalService {}
const factory = () => {};
export { LocalService as Service, factory };
export default LocalService;
`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	localService := scriptDeclarationByQualified(t, resolved.Declarations, "src/service#LocalService")
	factory := scriptDeclarationByQualified(t, resolved.Declarations, "src/service#factory")
	for _, want := range []struct {
		kind, exportName string
		target           RichSymbolRecord
	}{
		{kind: "imports_value", exportName: "Service", target: localService},
		{kind: "imports_value", exportName: "factory", target: factory},
		{kind: "imports_value", exportName: "default", target: localService},
		{kind: "calls_export", exportName: "factory", target: factory},
		{kind: "instantiates", exportName: "Service", target: localService},
		{kind: "instantiates", exportName: "default", target: localService},
	} {
		reference := assertScriptReference(t, resolved.References, want.kind, "./service", want.exportName)
		if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID != want.target.ID {
			t.Fatalf("local export %s#%s = %#v, want %s", want.kind, want.exportName, reference, want.target.ID)
		}
	}
}

func TestScriptLocalExportClauseForwardsImportedBinding(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/barrel.ts", Language: "typescript"},
		{Path: "src/service.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `import { Service } from "./barrel";`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `
import { LocalService } from "./service";
export { LocalService as Service };
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `export class LocalService {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "./barrel", "Service")
	declaration := scriptDeclarationByQualified(t, resolved.Declarations, "src/service#LocalService")
	if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID != declaration.ID {
		t.Fatalf("local export of imported binding = %#v, want %s", reference, declaration.ID)
	}
}

func TestScriptSemicolonlessLocalExportStopsAtLineBoundary(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/service.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `import { Service } from "./service"`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `
class LocalService {}
export { LocalService as Service }
const label = from + 1
`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "./service", "Service")
	declaration := scriptDeclarationByQualified(t, resolved.Declarations, "src/service#LocalService")
	if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID != declaration.ID {
		t.Fatalf("semicolonless local export = %#v, want exact declaration %s", reference, declaration.ID)
	}
}

func TestScriptCombinedDefaultNamespaceImportRetainsAlias(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import DefaultAPI, * as api from "./api";
export function App() { api.loadUser(); return new DefaultAPI(); }
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `
export default class API {}
export function loadUser() {}
`))

	namespace := assertScriptReference(t, facts.References, "imports_namespace", "./api", "*")
	if namespace.scriptLocalName != "api" {
		t.Fatalf("combined namespace local alias = %q, want api", namespace.scriptLocalName)
	}
	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	call := assertScriptReference(t, resolved.References, "calls_export", "./api", "loadUser")
	if call.Resolution != SymbolResolutionExact || call.ToSymbolID == "" {
		t.Fatalf("combined namespace call = %#v", call)
	}
}

func TestScriptReexportCycleRemainsUnresolved(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/a.ts", Language: "typescript"},
		{Path: "src/b.ts", Language: "typescript"},
	}
	bodies := map[string]string{
		"src/App.ts": `import { User } from "./a";`,
		"src/a.ts":   `export { User } from "./b";`,
		"src/b.ts":   `export { User } from "./a";`,
	}
	var facts ProjectSymbolFacts
	for _, file := range files {
		MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(file, bodies[file.Path]))
	}
	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "./a", "User")
	if reference.Resolution != SymbolResolutionUnresolved || reference.ToSymbolID != "" || reference.Reason != "cyclic re-export" {
		t.Fatalf("cyclic re-export = %#v", reference)
	}
}

func TestScriptConditionalPackageTargetsRemainAmbiguous(t *testing.T) {
	files := []FileRecord{
		{Path: "app/src/App.ts", Language: "typescript"},
		{Path: "packages/ui/src/user.ts", Language: "typescript"},
		{Path: "packages/ui/src/user-browser.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `import { User } from "@weka/ui/user";`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export interface User { id: string }`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `export interface User { id: string }`))
	packages := []NodePackageRecord{
		{Path: "app/package.json", Name: "app", Dependencies: []string{"@weka/ui"}},
		{Path: "packages/ui/package.json", Name: "@weka/ui", Exports: map[string][]string{"./user": {"./src/user-browser.ts", "./src/user.ts"}}},
	}
	resolved := ResolveScriptSymbolFacts(files, packages, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "@weka/ui/user", "User")
	if reference.Resolution != SymbolResolutionAmbiguous || reference.ToSymbolID != "" || len(reference.CandidateSymbolIDs) != 2 {
		t.Fatalf("conditional package export = %#v", reference)
	}
}

func TestScriptStaticImportSelectsConditionalImportBranch(t *testing.T) {
	files := []FileRecord{
		{Path: "app/src/App.ts", Language: "typescript"},
		{Path: "packages/ui/src/esm.ts", Language: "typescript"},
		{Path: "packages/ui/src/cjs.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(
		files[0],
		`import { CjsOnly, EsmOnly } from "@weka/ui";`,
	))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export class EsmOnly {}`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `export class CjsOnly {}`))
	provider, ok := extractNodePackage("packages/ui/package.json", `{
		"name": "@weka/ui",
		"exports": {
			".": {
				"import": "./src/esm.ts",
				"require": "./src/cjs.ts"
			}
		}
	}`)
	if !ok {
		t.Fatal("conditional package fixture was not extracted")
	}
	packages := []NodePackageRecord{
		{Path: "app/package.json", Name: "app", Dependencies: []string{"@weka/ui"}},
		provider,
	}

	resolved := ResolveScriptSymbolFacts(files, packages, nil, facts)
	esm := assertScriptReference(t, resolved.References, "imports_value", "@weka/ui", "EsmOnly")
	esmDeclaration := scriptDeclarationByQualified(t, resolved.Declarations, "packages/ui/src/esm#EsmOnly")
	if esm.Resolution != SymbolResolutionExact || esm.ToSymbolID != esmDeclaration.ID {
		t.Fatalf("ESM import branch = %#v, want exact %s", esm, esmDeclaration.ID)
	}
	cjs := assertScriptReference(t, resolved.References, "imports_value", "@weka/ui", "CjsOnly")
	if cjs.Resolution != SymbolResolutionUnresolved || cjs.ToSymbolID != "" {
		t.Fatalf("ESM import resolved require-only export: %#v", cjs)
	}
}

func TestScriptStaticImportUsesConditionalDefaultFallback(t *testing.T) {
	files := []FileRecord{
		{Path: "app/src/App.ts", Language: "typescript"},
		{Path: "packages/ui/src/default.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(
		files[0],
		`import { DefaultOnly } from "@weka/ui";`,
	))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export class DefaultOnly {}`))
	provider, ok := extractNodePackage("packages/ui/package.json", `{
		"name": "@weka/ui",
		"exports": {
			".": {
				"default": "./src/default.ts"
			}
		}
	}`)
	if !ok {
		t.Fatal("default package fixture was not extracted")
	}

	resolved := ResolveScriptSymbolFacts(files, []NodePackageRecord{
		{Path: "app/package.json", Name: "app", Dependencies: []string{"@weka/ui"}},
		provider,
	}, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "@weka/ui", "DefaultOnly")
	declaration := scriptDeclarationByQualified(t, resolved.Declarations, "packages/ui/src/default#DefaultOnly")
	if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID != declaration.ID {
		t.Fatalf("conditional default fallback = %#v, want exact %s", reference, declaration.ID)
	}
}

func TestScriptPackageTargetAmbiguitySurvivesOneExistingFile(t *testing.T) {
	files := []FileRecord{
		{Path: "app/src/App.ts", Language: "typescript"},
		{Path: "packages/ui/src/user.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `import { User } from "@weka/ui/user";`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export interface User { id: string }`))
	packages := []NodePackageRecord{
		{Path: "app/package.json", Name: "app", Dependencies: []string{"@weka/ui"}},
		{Path: "packages/ui/package.json", Name: "@weka/ui", Exports: map[string][]string{"./user": {"./src/missing.ts", "./src/user.ts"}}},
	}

	resolved := ResolveScriptSymbolFacts(files, packages, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "@weka/ui/user", "User")
	if reference.Resolution != SymbolResolutionAmbiguous || reference.ToSymbolID != "" || len(reference.CandidateSymbolIDs) != 1 {
		t.Fatalf("package target ambiguity collapsed after one file survived: %#v", reference)
	}
}

func TestScriptModuleAmbiguitySurvivesSingleExportCandidate(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/user.ts", Language: "typescript"},
		{Path: "src/user.tsx", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `import { User } from "./user";`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export interface User { id: string }`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `export function UserCard() { return <div /> }`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "./user", "User")
	if reference.Resolution != SymbolResolutionAmbiguous || reference.ToSymbolID != "" || len(reference.CandidateSymbolIDs) != 1 {
		t.Fatalf("module ambiguity collapsed to one export candidate: %#v", reference)
	}
}

func TestScriptExplicitExtensionSelectsPhysicalFileExports(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/user.ts", Language: "typescript"},
		{Path: "src/user.js", Language: "javascript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `import { User } from "./user.ts";`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export interface User { id: string }`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `export class User {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "./user.ts", "User")
	var declaration RichSymbolRecord
	for _, candidate := range resolved.Declarations {
		if candidate.File == "src/user.ts" && candidate.QualifiedName == "src/user#User" {
			declaration = candidate
			break
		}
	}
	if declaration.ID == "" {
		t.Fatalf("missing src/user.ts declaration in %#v", resolved.Declarations)
	}
	if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID != declaration.ID {
		t.Fatalf("explicit .ts import = %#v, want exact %s", reference, declaration.ID)
	}
}

func TestScriptDuplicateWorkspacePackageNamesRemainAmbiguous(t *testing.T) {
	files := []FileRecord{
		{Path: "app/src/App.ts", Language: "typescript"},
		{Path: "packages/ui-a/src/index.ts", Language: "typescript"},
		{Path: "packages/ui-b/src/index.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `import { User } from "@weka/ui";`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export interface User { source: "a" }`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `export interface User { source: "b" }`))
	packages := []NodePackageRecord{
		{Path: "app/package.json", Name: "app", Dependencies: []string{"@weka/ui"}},
		{Path: "packages/ui-a/package.json", Name: "@weka/ui", Types: "src/index.ts"},
		{Path: "packages/ui-b/package.json", Name: "@weka/ui", Types: "src/index.ts"},
	}

	resolved := ResolveScriptSymbolFacts(files, packages, nil, facts)
	reference := assertScriptReference(t, resolved.References, "imports_value", "@weka/ui", "User")
	if reference.Resolution != SymbolResolutionAmbiguous || reference.ToSymbolID != "" || len(reference.CandidateSymbolIDs) != 2 {
		t.Fatalf("duplicate workspace package providers = %#v, want two ambiguous candidates", reference)
	}
}

func TestScriptAliasCannotBypassWorkspaceDependency(t *testing.T) {
	files := []FileRecord{
		{Path: "apps/web/src/App.ts", Language: "typescript"},
		{Path: "packages/ui/src/user.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `import { User } from "@ui/user";`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export interface User { id: string }`))
	configs := map[string]ScriptResolutionConfig{
		"apps/web/tsconfig.json": {
			BaseURL: ".",
			Paths:   map[string][]string{"@ui/*": {"../../packages/ui/src/*"}},
		},
	}
	packages := []NodePackageRecord{
		{Path: "apps/web/package.json", Name: "web"},
		{Path: "packages/ui/package.json", Name: "@weka/ui"},
	}

	unresolved := ResolveScriptSymbolFacts(files, packages, configs, facts)
	reference := assertScriptReference(t, unresolved.References, "imports_value", "@ui/user", "User")
	if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || !strings.Contains(reference.Reason, "dependency") {
		t.Fatalf("alias bypassed workspace dependency gating: %#v", reference)
	}

	packages[0].Dependencies = []string{"@weka/ui"}
	exact := ResolveScriptSymbolFacts(files, packages, configs, facts)
	reference = assertScriptReference(t, exact.References, "imports_value", "@ui/user", "User")
	if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" {
		t.Fatalf("declared alias dependency did not resolve: %#v", reference)
	}
}

func TestNearestScriptConfigPrefersTsconfigAtEqualDepth(t *testing.T) {
	configs := map[string]ScriptResolutionConfig{
		"apps/web/jsconfig.json": {BaseURL: "js"},
		"apps/web/tsconfig.json": {BaseURL: "ts"},
	}
	for range 100 {
		path, config, ok := nearestScriptConfig("apps/web/src/App.ts", configs)
		if !ok || path != "apps/web/tsconfig.json" || config.BaseURL != "ts" {
			t.Fatalf("equal-depth config selection = %q %#v, want tsconfig", path, config)
		}
	}
}

func TestExtractNodePackageKeepsResolutionOnlyMetadata(t *testing.T) {
	record, ok := extractNodePackage("package.json", `{
  "dependencies": {"@weka/ui": "workspace:*"},
  "types": "./src/index.ts",
  "exports": "./src/index.ts"
}`)
	if !ok {
		t.Fatal("dependency/resolution-only package.json was discarded")
	}
	if !reflect.DeepEqual(record.Dependencies, []string{"@weka/ui"}) || record.Types != "./src/index.ts" || !reflect.DeepEqual(record.Exports["."], []string{"./src/index.ts"}) {
		t.Fatalf("resolution-only package metadata = %#v", record)
	}
}

func TestScriptFactsAreStableAcrossFileOrder(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	bodies := map[string]string{
		"src/App.ts": `import { loadUser } from "./api"; export function App() { loadUser() }`,
		"src/api.ts": `export function loadUser() {}`,
	}
	build := func(order []FileRecord) ProjectSymbolFacts {
		var facts ProjectSymbolFacts
		for _, file := range order {
			MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(file, bodies[file.Path]))
		}
		return ResolveScriptSymbolFacts(order, nil, nil, facts)
	}
	forward := build(files)
	reverse := build([]FileRecord{files[1], files[0]})
	if !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("script facts depend on file order:\nforward: %#v\nreverse: %#v", forward, reverse)
	}
}

func TestScriptSameNameWithoutModuleEvidenceIsNotExact(t *testing.T) {
	files := []FileRecord{
		{Path: "src/a/UserService.ts", Language: "typescript"},
		{Path: "src/b/UserService.ts", Language: "typescript"},
		{Path: "src/consumer.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `export class UserService {}`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export class UserService {}`))
	facts.References = append(facts.References, newScriptReference(files[2], "instantiates", "", "UserService", 1, "name-only constructor", true))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := assertScriptReference(t, resolved.References, "instantiates", "", "UserService")
	if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" {
		t.Fatalf("name-only duplicate resolved without module evidence: %#v", reference)
	}
}

func TestExtractScriptResolutionConfig(t *testing.T) {
	body := `{
  // local aliases
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@models/*": ["src/models/*",],
    },
  },
}`
	config, ok := ExtractScriptResolutionConfig("apps/web/tsconfig.json", body)
	if !ok {
		t.Fatal("expected commented tsconfig with trailing commas to parse")
	}
	want := ScriptResolutionConfig{BaseURL: ".", Paths: map[string][]string{"@models/*": {"src/models/*"}}}
	if !reflect.DeepEqual(config, want) {
		t.Fatalf("config = %#v, want %#v", config, want)
	}
	if _, ok := ExtractScriptResolutionConfig("apps/web/tsconfig.json", `{broken`); ok {
		t.Fatal("malformed config must not parse")
	}
}

func TestExtractNodePackageScriptResolutionFields(t *testing.T) {
	body := `{
  "name": "@weka/ui",
  "types": "./src/index.ts",
  "exports": {
    ".": "./src/index.ts",
    "./user": {"types": "./src/user.d.ts", "import": "./src/user.ts"},
    "./ignored": {"custom": 7}
  }
}`
	record, ok := extractNodePackage("packages/ui/package.json", body)
	if !ok {
		t.Fatal("expected package to be extracted")
	}
	if record.Types != "./src/index.ts" {
		t.Fatalf("types = %q", record.Types)
	}
	want := map[string][]string{
		".":      {"./src/index.ts"},
		"./user": {"./src/user.d.ts", "./src/user.ts"},
	}
	if !reflect.DeepEqual(record.Exports, want) {
		t.Fatalf("exports = %#v, want %#v", record.Exports, want)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, wantCondition := range []string{
		`"export_conditions"`,
		`"types":["./src/user.d.ts"]`,
		`"import":["./src/user.ts"]`,
	} {
		if !strings.Contains(string(encoded), wantCondition) {
			t.Fatalf("conditional exports lost %s: %s", wantCondition, encoded)
		}
	}
}

func TestScriptUsageFormsBindImportedAndLocalSymbols(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.tsx", Language: "typescript"},
		{Path: "src/models/User.ts", Language: "typescript"},
		{Path: "src/components/UserCard.tsx", Language: "typescript"},
		{Path: "src/services/UserService.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	bodies := map[string]string{
		"src/App.tsx": `
import type { User } from "./models/User";
import { UserCard } from "./components/UserCard";
import { UserService } from "./services/UserService";
import { loadUser } from "./api";
import * as api from "./api";
function helper() {}
export function App() {
  const user: User = { id: "1" };
  const service = new UserService();
  loadUser();
  api.loadUser();
  helper();
  return <UserCard user={user} />;
}
`,
		"src/models/User.ts":          `export interface User { id: string }`,
		"src/components/UserCard.tsx": `export function UserCard() { return <div /> }`,
		"src/services/UserService.ts": `export class UserService {}`,
		"src/api.ts":                  `export function loadUser() {}`,
	}
	var facts ProjectSymbolFacts
	for _, file := range files {
		MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(file, bodies[file.Path]))
	}
	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	app := scriptDeclarationByQualified(t, resolved.Declarations, "src/App#App")

	wants := []struct {
		kind, module, exportName, qualified string
	}{
		{kind: "type_reference", module: "./models/User", exportName: "User", qualified: "src/models/User#User"},
		{kind: "renders_component", module: "./components/UserCard", exportName: "UserCard", qualified: "src/components/UserCard#UserCard"},
		{kind: "instantiates", module: "./services/UserService", exportName: "UserService", qualified: "src/services/UserService#UserService"},
		{kind: "calls_export", module: "./api", exportName: "loadUser", qualified: "src/api#loadUser"},
		{kind: "calls_local", module: "src/App", exportName: "helper", qualified: "src/App#helper"},
	}
	for _, want := range wants {
		reference := assertScriptReference(t, resolved.References, want.kind, want.module, want.exportName)
		declaration := scriptDeclarationByQualified(t, resolved.Declarations, want.qualified)
		if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID != declaration.ID || reference.FromSymbolID != app.ID {
			t.Fatalf("%s usage = %#v, want %s -> %s", want.kind, reference, app.ID, declaration.ID)
		}
	}

	namespaceCalls := scriptReferences(t, resolved.References, "calls_export", "./api", "loadUser")
	if len(namespaceCalls) != 2 {
		t.Fatalf("named plus namespace calls = %d, want 2: %#v", len(namespaceCalls), namespaceCalls)
	}
}

func TestScriptDynamicAndComputedUsageRemainUnresolved(t *testing.T) {
	file := FileRecord{Path: "src/registry.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `
export function invoke(name: string, modulePath: string) {
  registry[name]();
  return import(modulePath);
}
`)
	resolved := ResolveScriptSymbolFacts([]FileRecord{file}, nil, nil, facts)
	computed := assertScriptReference(t, resolved.References, "calls_export", "", "")
	dynamic := assertScriptReference(t, resolved.References, "imports_module", "", "")
	for _, reference := range []RichRelationRecord{computed, dynamic} {
		if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || reference.Reason == "" {
			t.Fatalf("dynamic/computed reference resolved: %#v", reference)
		}
	}
}

func TestScriptDynamicImportKeywordMustBeStandalone(t *testing.T) {
	file := FileRecord{Path: "src/loader.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `
loader.import("./fake");
loader. import("./spaced");
import.meta.resolve("./meta");
import . meta.resolve("./spaced-meta");
const registry = { import() { return "./method"; } };
const real = import("./real");
`)

	if references := scriptReferences(t, facts.References, "imports_module", "./real", "*"); len(references) != 1 {
		t.Fatalf("missing standalone dynamic import: %#v", facts.References)
	}
	for _, reference := range facts.References {
		if reference.Type == "imports_module" && reference.TargetModule != "./real" {
			t.Fatalf("member named import was extracted as dynamic import: %#v", reference)
		}
	}
}

func TestScriptImportMetaIsNotStandaloneImportKeyword(t *testing.T) {
	for _, source := range []string{"import.meta", "import . meta"} {
		if isStandaloneScriptImport(source, 0) {
			t.Fatalf("%q classified as standalone import keyword", source)
		}
	}
}

func TestScriptDynamicImportOwnershipUsesOffsets(t *testing.T) {
	file := FileRecord{Path: "src/loader.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `export function first() { return import("./first"); }; export function second() { return import("./second"); }`)
	first := scriptDeclarationByQualified(t, facts.Declarations, "src/loader#first")
	second := scriptDeclarationByQualified(t, facts.Declarations, "src/loader#second")

	firstImport := assertScriptReference(t, facts.References, "imports_module", "./first", "*")
	if firstImport.FromSymbolID != first.ID {
		t.Fatalf("first dynamic import owner = %q, want %q: %#v", firstImport.FromSymbolID, first.ID, firstImport)
	}
	secondImport := assertScriptReference(t, facts.References, "imports_module", "./second", "*")
	if secondImport.FromSymbolID != second.ID {
		t.Fatalf("second dynamic import owner = %q, want %q: %#v", secondImport.FromSymbolID, second.ID, secondImport)
	}
}

func TestScriptSameLineUsageReferencesRetainDistinctOwners(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { loadUser } from "./api";
export const first = () => { loadUser(); }; export const second = () => { loadUser(); };
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function loadUser() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	references := scriptReferences(t, resolved.References, "calls_export", "./api", "loadUser")
	if len(references) != 2 {
		t.Fatalf("same-line owner calls = %#v, want two references", references)
	}
	owners := map[string]bool{}
	for _, reference := range references {
		if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" || reference.FromSymbolID == "" {
			t.Fatalf("same-line owner call did not resolve exactly: %#v", reference)
		}
		owners[reference.FromSymbolID] = true
	}
	if len(owners) != 2 {
		t.Fatalf("same-line owner identities = %#v, want two distinct owners", references)
	}
}

func TestScriptExpressionArrowDoesNotOwnLaterTopLevelCall(t *testing.T) {
	file := FileRecord{Path: "src/top-level.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `
const first = () => 1;
helper();
function helper() {}
`)
	resolved := ResolveScriptSymbolFacts([]FileRecord{file}, nil, nil, facts)
	reference := assertScriptReference(t, resolved.References, "calls_local", "src/top-level", "helper")
	if reference.FromSymbolID != "" {
		t.Fatalf("top-level call was assigned to an unrelated arrow: %#v", reference)
	}
}

func TestScriptShadowedImportsNeverResolveExact(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { loadUser } from "./api"; import * as api from "./api";
export function App(loadUser: () => void) {
  loadUser();
  {
    const loadUser = () => {};
    loadUser();
  }
}
export function Other() {
  loadUser();
}
export const Arrow = (loadUser) => {
  loadUser();
};
export function Namespace(api) {
  api.loadUser();
}
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function loadUser() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	for _, line := range []int{4, 7, 14, 17} {
		reference := scriptReferenceAtLine(t, resolved.References, "calls_export", line)
		if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || !strings.Contains(reference.Reason, "shadow") {
			t.Fatalf("shadowed imported call at line %d resolved exact: %#v", line, reference)
		}
	}
	reference := scriptReferenceAtLine(t, resolved.References, "calls_export", 11)
	if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" {
		t.Fatalf("unshadowed imported call did not resolve: %#v", reference)
	}
}

func TestScriptExtendedParameterAndCatchScopesShadowImports(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { loadUser } from "./api";
const defaultArrow = (loadUser = fallback) => loadUser();
const destructuredArrow = ({ loadUser }) => loadUser();
const restArrow = (...loadUser) => loadUser();
try {
  work();
} catch (loadUser) {
  loadUser();
}
class Handler {
  method(loadUser) {
    loadUser();
  }
}
const handler = {
  method(loadUser) {
    loadUser();
  }
};
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function loadUser() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	for _, line := range []int{3, 4, 5, 9, 13, 18} {
		reference := scriptReferenceAtLine(t, resolved.References, "calls_export", line)
		if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || !strings.Contains(reference.Reason, "shadow") {
			t.Fatalf("extended shadow scope at line %d resolved exact: %#v", line, reference)
		}
	}
}

func TestScriptTypedArrowReturnAnnotationPreservesParameterShadow(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { loadUser } from "./api";
const invoke = (loadUser): void => loadUser();
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function loadUser() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := scriptReferenceAtLine(t, resolved.References, "calls_export", 3)
	if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || !strings.Contains(reference.Reason, "shadow") {
		t.Fatalf("typed arrow parameter did not shadow imported call: %#v", reference)
	}
}

func TestScriptAsyncComplexTypedArrowPreservesParameterShadow(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { loadUser } from "./api";
const invoke = async <T>(loadUser): Promise<Result<T[]> | ((value: T) => void)> => loadUser();
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function loadUser() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := scriptReferenceAtLine(t, resolved.References, "calls_export", 3)
	if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || !strings.Contains(reference.Reason, "shadow") {
		t.Fatalf("async complex typed arrow did not shadow imported call: %#v", reference)
	}
}

func TestScriptDestructuredLocalBindingsShadowImports(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { loadUser } from "./api";
export function ObjectBinding(source) {
  const { loadUser } = source;
  loadUser();
}
export function ArrayBinding(source) {
  let [loadUser] = source;
  loadUser();
}
export function RestBinding(source) {
  var { ...loadUser } = source;
  loadUser();
}
export function DefaultBinding(source) {
  const { loadUser = fallback } = source;
  loadUser();
}
export function PropertyKeyOnly(source) {
  const { loadUser: localLoadUser } = source;
  loadUser();
}
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function loadUser() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	for _, line := range []int{5, 9, 13, 17} {
		reference := scriptReferenceAtLine(t, resolved.References, "calls_export", line)
		if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || !strings.Contains(reference.Reason, "shadow") {
			t.Fatalf("destructured local binding at line %d resolved imported call: %#v", line, reference)
		}
	}
	reference := scriptReferenceAtLine(t, resolved.References, "calls_export", 21)
	if reference.Resolution != SymbolResolutionExact || reference.ToSymbolID == "" {
		t.Fatalf("object property key incorrectly shadowed imported call: %#v", reference)
	}
}

func TestScriptLaterCommaSeparatedDeclaratorsShadowImports(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { loadUser } from "./api";
export function Multiple(source) {
  const first = build({ values: [1, 2], label: "," }), { nested: [ignored, ...loadUser] = [] } = source;
  loadUser();
}
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function loadUser() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := scriptReferenceAtLine(t, resolved.References, "calls_export", 5)
	if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || !strings.Contains(reference.Reason, "shadow") {
		t.Fatalf("later destructured declarator did not shadow imported call: %#v", reference)
	}
}

func TestScriptNewlineAfterDeclaratorCommaContinuesShadowBinding(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { loadUser } from "./api";
export function Multiple(source) {
  const first = build(),
    { nested: [ignored, ...loadUser] = [] } = source;
  loadUser();
}
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function loadUser() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := scriptReferenceAtLine(t, resolved.References, "calls_export", 6)
	if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || !strings.Contains(reference.Reason, "shadow") {
		t.Fatalf("newline-continued declarator did not shadow imported call: %#v", reference)
	}
}

func TestScriptLeadingCommaContinuesMultilineShadowBinding(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/api.ts", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { loadUser } from "./api";
export function Multiple(source) {
  const first = build({ nested: [1, 2] })
    , { loadUser = fallback } = source;
  loadUser();
}
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `export function loadUser() {}`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	reference := scriptReferenceAtLine(t, resolved.References, "calls_export", 6)
	if reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "" || !strings.Contains(reference.Reason, "shadow") {
		t.Fatalf("leading-comma declarator did not shadow imported call: %#v", reference)
	}
}

func TestScriptNestedDeclarationsAreNonselectableAndScopeLocal(t *testing.T) {
	file := FileRecord{Path: "src/App.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `
export function Outer() {
  function helper() {}
  helper();
}
export function Sibling() {
  helper();
}
`)
	for _, declaration := range facts.Declarations {
		if declaration.Name == "helper" {
			t.Fatalf("nested helper became a module declaration: %#v", declaration)
		}
	}
	resolved := ResolveScriptSymbolFacts([]FileRecord{file}, nil, nil, facts)
	for _, line := range []int{4, 7} {
		for _, reference := range resolved.References {
			if reference.Line == line && reference.TargetExport == "helper" && (reference.Resolution == SymbolResolutionExact || reference.ToSymbolID != "") {
				t.Fatalf("scope-local helper leaked at line %d: %#v", line, reference)
			}
		}
	}
}

func TestExtractScriptRejectsTypeTokensExpressionsAndNestedDeclarations(t *testing.T) {
	file := FileRecord{Path: "src/Declarations.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, `
import type Imported from "./Imported";
const ClassFactory = class ExpressionClass {};
const FunctionFactory = function ExpressionFunction() {};
export function Same() {}
export function Outer() {
  function Same() {}
  class NestedClass {}
}
export class RealService {}
`)
	wants := map[string]int{
		"src/Declarations#Same":        5,
		"src/Declarations#Outer":       6,
		"src/Declarations#RealService": 10,
	}
	if len(facts.Declarations) != len(wants) {
		t.Fatalf("declarations = %#v, want only top-level named declarations", facts.Declarations)
	}
	for qualified, line := range wants {
		declaration := scriptDeclarationByQualified(t, facts.Declarations, qualified)
		if declaration.Line != line {
			t.Fatalf("%s line = %d, want %d", qualified, declaration.Line, line)
		}
	}
}

func TestScriptTypeAndJSXReferencesRequireProvenContext(t *testing.T) {
	files := []FileRecord{
		{Path: "src/App.ts", Language: "typescript"},
		{Path: "src/View.tsx", Language: "typescript"},
		{Path: "src/UserService.ts", Language: "typescript"},
		{Path: "src/UserCard.tsx", Language: "typescript"},
	}
	var facts ProjectSymbolFacts
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[0], `
import { UserService } from "./UserService";
import { UserCard } from "./UserCard";
export function App(value: number) {
  const object = {ctor: UserService};
  const typed: UserService = object.ctor;
  return value < UserCard > 0;
}
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[1], `
import { UserCard } from "./UserCard";
export function View() { return <UserCard />; }
`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[2], `export class UserService {}`))
	MergeProjectSymbolFacts(&facts, ExtractScriptSymbolFacts(files[3], `export function UserCard() { return <div /> }`))

	resolved := ResolveScriptSymbolFacts(files, nil, nil, facts)
	typeReferences := scriptReferences(t, resolved.References, "type_reference", "./UserService", "UserService")
	if len(typeReferences) != 1 || typeReferences[0].Line != 6 || typeReferences[0].Resolution != SymbolResolutionExact {
		t.Fatalf("proven type references = %#v, want only variable annotation line 6", typeReferences)
	}
	for _, reference := range resolved.References {
		if reference.From == "src/App.ts" && reference.Type == "renders_component" {
			t.Fatalf(".ts comparison emitted JSX relation: %#v", reference)
		}
	}
	jsx := assertScriptReference(t, resolved.References, "renders_component", "./UserCard", "UserCard")
	if jsx.From != "src/View.tsx" || jsx.Resolution != SymbolResolutionExact {
		t.Fatalf("tsx component reference = %#v", jsx)
	}
}

func TestRunWritesScriptCanonicalSymbolFactsAndCallEdges(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@api":["src/api"]}}}`)
	writeFile(t, root, "src/api.ts", `export function loadUser() {}`)
	writeFile(t, root, "src/UserCard.tsx", `export function UserCard() { return <div /> }`)
	writeFile(t, root, "src/App.tsx", `
import { loadUser } from "@api";
import { UserCard } from "./UserCard";
function helper() {}
export function App() {
  loadUser();
  helper();
  return <UserCard />;
}
`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []RichSymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols-full.json"), &symbols)
	app := scriptDeclarationByQualified(t, symbols, "src/App#App")
	loadUser := scriptDeclarationByQualified(t, symbols, "src/api#loadUser")
	userCard := scriptDeclarationByQualified(t, symbols, "src/UserCard#UserCard")
	helper := scriptDeclarationByQualified(t, symbols, "src/App#helper")

	var relations []RichRelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations-full.json"), &relations)
	for _, want := range []struct {
		kind, module, exportName string
		declaration              RichSymbolRecord
	}{
		{kind: "calls_export", module: "@api", exportName: "loadUser", declaration: loadUser},
		{kind: "calls_local", module: "src/App", exportName: "helper", declaration: helper},
		{kind: "renders_component", module: "./UserCard", exportName: "UserCard", declaration: userCard},
	} {
		reference := assertScriptReference(t, relations, want.kind, want.module, want.exportName)
		if reference.FromSymbolID != app.ID || reference.ToSymbolID != want.declaration.ID || reference.Resolution != SymbolResolutionExact {
			t.Fatalf("integrated %s relation = %#v", want.kind, reference)
		}
	}

	var graph CallGraphRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "callgraph.json"), &graph)
	for _, target := range []RichSymbolRecord{loadUser, helper} {
		found := false
		for _, edge := range graph.Edges {
			if edge.From.Method == "App" && edge.To.Method == target.Name && edge.FromSymbolID == app.ID && edge.ToSymbolID == target.ID && edge.Resolution == SymbolResolutionExact {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing canonical App -> %s call edge in %#v", target.Name, graph.Edges)
		}
	}
}

func TestLinkCallGraphScriptFactsRejectsMismatchedLegacyTarget(t *testing.T) {
	providerA := RichSymbolRecord{ID: "provider-a", Name: "loadUser", QualifiedName: "src/a#loadUser", File: "src/a.ts", Language: "typescript"}
	providerB := RichSymbolRecord{ID: "provider-b", Name: "loadUser", QualifiedName: "src/b#loadUser", File: "src/b.ts", Language: "typescript"}
	facts := ProjectSymbolFacts{
		Declarations: []RichSymbolRecord{providerA, providerB},
		References: []RichRelationRecord{{
			From:                "src/App.ts",
			Type:                "calls_export",
			Line:                5,
			FromSymbolID:        "app",
			ToSymbolID:          providerA.ID,
			TargetQualifiedName: providerA.QualifiedName,
			Resolution:          SymbolResolutionExact,
		}},
	}
	graph := CallGraphRecord{Edges: []CallGraphEdgeRecord{{
		From:       MethodRefRecord{Method: "App", File: "src/App.ts"},
		To:         MethodRefRecord{Method: "loadUser", File: providerB.File},
		SourceFile: "src/App.ts",
		Line:       5,
	}}}
	linkCallGraphSymbolFacts(&graph, facts)
	if graph.Edges[0].FromSymbolID != "" || graph.Edges[0].ToSymbolID != "" {
		t.Fatalf("mismatched legacy target received canonical IDs: %#v", graph.Edges[0])
	}
}

func TestScanStoresScriptConfigsWithoutBodiesAndMarksMalformedPartial(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "apps/web/tsconfig.json", `{
  // source-only comment must not be retained
  "compilerOptions": {"baseUrl": ".", "paths": {"@models/*": ["src/models/*",],},},
}`)
	writeFile(t, root, "apps/web/src/App.ts", `export function App() {}`)
	index, _, err := scanProject(root, config.Defaults(), gitignore.Matcher{})
	if err != nil {
		t.Fatalf("scanProject returned error: %v", err)
	}
	configRecord, ok := index.ScriptConfigs["apps/web/tsconfig.json"]
	if !ok || configRecord.BaseURL != "." || !reflect.DeepEqual(configRecord.Paths["@models/*"], []string{"src/models/*"}) {
		t.Fatalf("script configs = %#v", index.ScriptConfigs)
	}
	encoded, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "source-only comment") || strings.Contains(string(encoded), "export function App") {
		t.Fatalf("scan index retained raw source body: %s", encoded)
	}

	brokenRoot := t.TempDir()
	writeFile(t, brokenRoot, "tsconfig.json", `{broken`)
	writeFile(t, brokenRoot, "src/App.ts", `export function App() {}`)
	broken, _, err := scanProject(brokenRoot, config.Defaults(), gitignore.Matcher{})
	if err != nil {
		t.Fatalf("malformed config aborted scan: %v", err)
	}
	app := scriptDeclarationByQualified(t, broken.SymbolFacts.Declarations, "src/App#App")
	if app.Coverage != CoveragePartial || len(app.Limitations) == 0 {
		t.Fatalf("malformed config coverage = %#v", app)
	}
}

func scriptReferences(t *testing.T, references []RichRelationRecord, kind, module, exportName string) []RichRelationRecord {
	t.Helper()
	var result []RichRelationRecord
	for _, reference := range references {
		if reference.Type == kind && reference.TargetModule == module && reference.TargetExport == exportName {
			result = append(result, reference)
		}
	}
	return result
}

func scriptReferenceAtLine(t *testing.T, references []RichRelationRecord, kind string, line int) RichRelationRecord {
	t.Helper()
	for _, reference := range references {
		if reference.Type == kind && reference.Line == line {
			return reference
		}
	}
	t.Fatalf("missing %s reference at line %d in %#v", kind, line, references)
	return RichRelationRecord{}
}

func scriptDeclarationByQualified(t *testing.T, declarations []RichSymbolRecord, qualified string) RichSymbolRecord {
	t.Helper()
	for _, declaration := range declarations {
		if declaration.QualifiedName == qualified {
			return declaration
		}
	}
	t.Fatalf("missing script declaration %s in %#v", qualified, declarations)
	return RichSymbolRecord{}
}
