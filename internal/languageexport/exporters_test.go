package languageexport

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testReport struct {
	Declarations []struct{ USR, Name, Kind string } `json:"declarations"`
	References   []struct{ USR, Name, Kind string } `json:"references"`
}

func writeTestBundle(t *testing.T, language, root string) {
	t.Helper()
	files, err := Files(language)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func exporterCommand(t *testing.T, root, program string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = root
	command.Env = append(os.Environ(), "DOTNET_CLI_HOME="+filepath.Join(root, "dotnet-home"), "DOTNET_CLI_TELEMETRY_OPTOUT=1", "DOTNET_SKIP_FIRST_TIME_EXPERIENCE=1", "DOTNET_GENERATE_ASPNET_CERTIFICATE=false", "NUGET_PACKAGES="+filepath.Join(root, "packages"))
	out, err := command.CombinedOutput()
	return string(out), err
}

func writeTestRequest(t *testing.T, root string, request any) string {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, "request.json")
	if err := os.WriteFile(name, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

func readTestReport(t *testing.T, file string) testReport {
	t.Helper()
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var report testReport
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatal(err)
	}
	return report
}

// SDK tests compile only disposable exporter bundles and never build a user project.
func TestRealRoslynExporterBindingsAndFailureBoundaries(t *testing.T) {
	dotnet := os.Getenv("GOREGRAPH_DOTNET_SMOKE")
	if dotnet == "" {
		t.Skip("explicit .NET SDK fixture path not supplied")
	}
	root := t.TempDir()
	writeTestBundle(t, "csharp", root)
	body := `public static class Extensions {public static T Echo<T>(this T value) where T:class => value;}
public record Counter(int Count);
public class Number(int value) {public int Value=>value;public static Number operator +(Number a,Number b)=>new(a.Value+b.Value);}
public class Service {public T Pick<T>(T value)=>value;public int Pick(int value)=>value;
public object Run(){Counter counter=new(1);Number sum=new Number(1)+new Number(2);return Pick("value".Echo())!;}}
`
	source := filepath.Join(root, "Service.cs")
	os.WriteFile(source, []byte(body), 0o644)
	request := writeTestRequest(t, root, map[string]any{"root": root, "inputs": []string{"Service.cs"}, "modules": []any{map[string]any{"name": "Fixture", "files": []string{"Service.cs"}}}})
	// Only Program.cs may be compiled, even if foreign C# files are alongside it.
	os.WriteFile(filepath.Join(root, "MustNotCompile.cs"), []byte("#error User source entered the helper build"), 0o644)
	if out, err := exporterCommand(t, root, dotnet, "build", "--nologo", "--verbosity", "quiet"); err != nil {
		t.Fatal(err, out)
	}
	program := filepath.Join(root, "bin", "Debug", "net10.0", "Exporter.dll")
	output := filepath.Join(root, "Report.goregraph-csharp.json")
	if out, err := exporterCommand(t, root, dotnet, program, request, output); err != nil {
		t.Fatal(err, out)
	}
	report := readTestReport(t, output)
	echo, generic, operator, record, property := false, false, false, false, false
	for _, ref := range report.References {
		if ref.Kind != "calls_method_owner" {
			continue
		}
		echo = echo || strings.Contains(ref.USR, "Echo``1")
		generic = generic || strings.Contains(ref.USR, "Pick``1")
		operator = operator || strings.Contains(ref.USR, "op_Addition")
		record = record || strings.Contains(ref.USR, "Counter.#ctor")
		if ref.Name == "!" {
			t.Fatal("null suppression became an invocation", ref)
		}
	}
	for _, decl := range report.Declarations {
		property = property || decl.Name == "Count" && decl.Kind == "property"
	}
	if !echo || !generic || !operator || !record || !property {
		t.Fatal(report)
	}
	if _, err := exporterCommand(t, root, dotnet, program, request, output); err == nil {
		t.Fatal("existing report overwritten")
	}
	os.WriteFile(source, []byte("class Broken {void Run(){Undefined();}}"), 0o644)
	bad := filepath.Join(root, "Bad.goregraph-csharp.json")
	if _, err := exporterCommand(t, root, dotnet, program, request, bad); err == nil {
		t.Fatal("compiler errors accepted")
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Fatal("failed compilation wrote semantic evidence", err)
	}
}

func TestRealSourceKitExporterRejectsCompilerErrorsAndMethodValues(t *testing.T) {
	library, sdk := os.Getenv("GOREGRAPH_SOURCEKIT_SMOKE"), os.Getenv("GOREGRAPH_SWIFT_SDK_SMOKE")
	if library == "" || sdk == "" {
		t.Skip("explicit SourceKit library and Swift SDK fixture paths not supplied")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeTestBundle(t, "swift", root)
	source := filepath.Join(root, "Service.swift")
	os.WriteFile(source, []byte(`struct Number {let value:Int;static func +(left:Number,right:Number)->Number{Number(value:left.value+right.value)}}
func sink(_ body:()->Void){body()}
func run(){sink {};let stored=sink;stored({})}
func combine()->Number{Number(value:1)+Number(value:2)}
func retainOperator(){let operation:(Number,Number)->Number = (+);_ = operation(Number(value:1),Number(value:2))}
`), 0o644)
	request := writeTestRequest(t, root, map[string]any{"root": root, "inputs": []string{"Service.swift"}, "sourcekit_library": library, "modules": []any{map[string]any{"name": "Fixture", "files": []string{"Service.swift"}, "compiler_args": []string{"-sdk", sdk, "-module-cache-path", filepath.Join(root, "cache")}}}})
	output := filepath.Join(root, "Report.goregraph-swift.json")
	if out, err := exporterCommand(t, root, python, filepath.Join(root, "sourcekit.py"), request, output); err != nil {
		t.Fatal(err, out)
	}
	report := readTestReport(t, output)
	operatorUSRs := map[string]bool{}
	for _, declaration := range report.Declarations {
		if declaration.Name == "+" {
			operatorUSRs[declaration.USR] = true
		}
	}
	call, value, operator, operatorValue := false, false, false, false
	for _, ref := range report.References {
		operator = operator || operatorUSRs[ref.USR] && ref.Kind == "calls_method_owner"
		operatorValue = operatorValue || operatorUSRs[ref.USR] && ref.Kind == "uses_symbol"
		if ref.Name != "sink" {
			continue
		}
		call = call || ref.Kind == "calls_method_owner"
		value = value || ref.Kind == "uses_symbol"
	}
	if !call || !value || !operator || !operatorValue {
		t.Fatal("trailing closure or method value binding absent", report)
	}
	os.WriteFile(source, []byte("import Observation\n@Observable class Model {var value=1}"), 0o644)
	guardRequest := writeTestRequest(t, root, map[string]any{"root": root, "inputs": []string{"Service.swift"}, "sourcekit_library": "must-not-be-loaded", "modules": []any{map[string]any{"name": "Fixture", "files": []string{"Service.swift"}}}})
	guardOutput := filepath.Join(root, "Macro.goregraph-swift.json")
	if out, err := exporterCommand(t, root, python, filepath.Join(root, "sourcekit.py"), guardRequest, guardOutput); err == nil || !strings.Contains(out, "allow_macro_expansion=true") {
		t.Fatal("attached macro reached SourceKit without an explicit opt-in", err, out)
	}
	// Restore the compiler request after the preflight-only macro check.
	request = writeTestRequest(t, root, map[string]any{"root": root, "inputs": []string{"Service.swift"}, "sourcekit_library": library, "modules": []any{map[string]any{"name": "Fixture", "files": []string{"Service.swift"}, "compiler_args": []string{"-sdk", sdk, "-module-cache-path", filepath.Join(root, "cache")}}}})
	os.WriteFile(source, []byte("struct Broken {func run(){let missing:Unknown=absent();missing.foo()}}"), 0o644)
	bad := filepath.Join(root, "Bad.goregraph-swift.json")
	if _, err := exporterCommand(t, root, python, filepath.Join(root, "sourcekit.py"), request, bad); err == nil {
		t.Fatal("SourceKit compiler errors accepted")
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Fatal("failed compilation wrote semantic evidence", err)
	}
}
