package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Compiler checks are explicitly enabled and only compile disposable fixtures.
func TestRealLanguageCompilerFixtures(t *testing.T) {
	if compiler := os.Getenv("GOREGRAPH_SWIFTC_SMOKE"); compiler != "" {
		t.Run("Swift", func(t *testing.T) {
			root := t.TempDir()
			body := `import Foundation
class Parent {func load(id: Int) -> String {"int"};func load(name: String) -> String {"string"};func numeric(_ value: Int) -> String {"int"};func numeric(_ value: Double) -> String {"double"};func rotate(degrees:Double)->Double {degrees}}
class Child: Parent {func check() {precondition(load(id: 1)=="int");precondition(super.load(name: "Knight")=="string");precondition(numeric(1)=="int");precondition(rotate(degrees:90)==90)}}
let child = Child()
child.check()
print("Swift fixture passed")
`
			writeFile(t, root, "main.swift", body)
			result := analyzeSwiftProject([]swiftSource{parseSwiftSource(FileRecord{Path: "main.swift"}, body)})
			if len(result.graph.Edges) != 4 {
				t.Fatal(result.graph)
			}
			binary := filepath.Join(root, "fixture")
			args := []string{"-module-cache-path", filepath.Join(root, "cache")}
			if sdk := os.Getenv("GOREGRAPH_SWIFT_SDK_SMOKE"); sdk != "" {
				args = append(args, "-sdk", sdk)
			}
			args = append(args, filepath.Join(root, "main.swift"), "-o", binary)
			compilerFixtureCommand(t, root, compiler, args...)
			if output := compilerFixtureCommand(t, root, binary); !strings.Contains(output, "Swift fixture passed") {
				t.Fatal(output)
			}
			frameworkBody := `import SwiftUI
import SwiftData
import CoreData
@Model final class Item {var name:String;init(name:String){self.name=name}}
struct ItemView: View {
 @Query var items:[Item]
 @State private var text=""
 var body:some View {TextField("Name",text:$text)}
}
func persist(_ context:ModelContext) throws {try context.save()}
func persistCoreData(_ context:NSManagedObjectContext) throws {try context.save()}
`
			writeFile(t, root, "Framework.swift", frameworkBody)
			frameworkAnalysis := analyzeSwiftProject([]swiftSource{parseSwiftSource(FileRecord{Path: "Framework.swift"}, frameworkBody)})
			persistence := 0
			for _, fact := range frameworkAnalysis.capabilities {
				if fact.Capability == CapabilityPersistence {
					persistence++
				}
			}
			if persistence != 2 {
				t.Fatal(frameworkAnalysis.capabilities)
			}
			args = args[:len(args)-3]
			args = append(args, "-typecheck", "-swift-version", "6", filepath.Join(root, "Framework.swift"))
			compilerFixtureCommand(t, root, compiler, args...)
		})
	}
	if compiler := os.Getenv("GOREGRAPH_DOTNET_SMOKE"); compiler != "" {
		t.Run("CSharp", func(t *testing.T) {
			root := t.TempDir()
			body := `using System;
class Parent {public string Load(int id) => "int";public string Load(string name) => "string";public string Change(ref int id){id++;return "ref";} public string Constant(short value)=>"short";public string Constant(long value)=>"long";}
class Child: Parent {public void Check(){int value=1;if(Load(id: 1)!="int" || Load(name: "Knight")!="string" || Change(ref value)!="ref" || value!=2 || Constant(1)!="short" || Constant(50000)!="long") throw new Exception("binding failed");}}
class Program {static void Main(){new Child().Check();Console.WriteLine("CSharp fixture passed");}}
`
			writeFile(t, root, "Program.cs", body)
			writeFile(t, root, "Fixture.csproj", `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net10.0</TargetFramework><OutputType>Exe</OutputType><NuGetAudit>false</NuGetAudit></PropertyGroup></Project>`)
			writeFile(t, root, "NuGet.Config", `<configuration><packageSources><clear /></packageSources></configuration>`)
			result := analyzeCSharpProject([]csharpSource{parseCSharpSource(FileRecord{Path: "Program.cs"}, body)})
			if len(result.graph.Edges) != 5 {
				t.Fatal(result.graph)
			}
			compilerFixtureCommand(t, root, compiler, "build", "--nologo", "--verbosity", "quiet")
			if output := compilerFixtureCommand(t, root, compiler, filepath.Join(root, "bin", "Debug", "net10.0", "Fixture.dll")); !strings.Contains(output, "CSharp fixture passed") {
				t.Fatal(output)
			}
		})
	}
	if os.Getenv("GOREGRAPH_SWIFTC_SMOKE") == "" && os.Getenv("GOREGRAPH_DOTNET_SMOKE") == "" {
		t.Skip("compiler fixture paths not explicitly configured")
	}
}

func compilerFixtureCommand(t *testing.T, root, program string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = root
	command.Env = append(os.Environ(), "DOTNET_CLI_HOME="+filepath.Join(root, "dotnet-home"), "DOTNET_CLI_TELEMETRY_OPTOUT=1", "DOTNET_SKIP_FIRST_TIME_EXPERIENCE=1", "NUGET_PACKAGES="+filepath.Join(root, "packages"))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("disposable compiler fixture failed: %v\n%s", err, output)
	}
	return string(output)
}
