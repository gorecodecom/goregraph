package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDartFlutterBuildDeliversCallsContractsTestsAndCurrentSource(t *testing.T) {
	root := t.TempDir()
	fixtures := map[string]string{
		"pubspec.yaml":      "name: transit\nenvironment:\n  sdk: ^3.13.0\ndependencies:\n  flutter:\n    sdk: flutter\n  http: ^1.6.0\nflutter:\n  assets:\n    - assets/board.json\n",
		"assets/board.json": "{}",
		"lib/service.dart": `import 'package:http/http.dart' as http;
class DepartureService {
 final http.Client client;
 final Uri base;
 DepartureService(this.client,this.base);
 Future<void> refresh() async {
  final request = http.AbortableRequest('POST', base.resolve('/v1/refresh'));
  await client.send(request);
 }
 void verifyBoard() { print('board verified'); }
 void updateBoard() { verifyBoard(); }
}`,
		"lib/board.dart": `import 'package:flutter/material.dart';
import 'service.dart';
class DepartureBoard extends StatelessWidget {
 final DepartureService service;
 DepartureBoard(this.service);
 Widget build(BuildContext context) { service.updateBoard(); return Text('Board'); }
}`,
		"test/service_test.dart": `import 'package:flutter_test/flutter_test.dart';
import 'package:transit/service.dart';
void main() {
 test('updates board', () { final service = DepartureService(client, base); service.updateBoard(); });
}`,
	}
	for file, body := range fixtures {
		writeSourceFile(t, root, file, body)
	}
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	var contracts []scan.APIContractRecord
	read := func(file string, target any) {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, "index", file))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(body, target); err != nil {
			t.Fatal(err)
		}
	}
	read("api-contracts.json", &contracts)
	if len(contracts) != 1 || contracts[0].HTTPMethod != "POST" || contracts[0].Path != "/v1/refresh" {
		t.Fatal(contracts)
	}
	var tests []scan.TestMapRecord
	read("test-map.json", &tests)
	foundTest := false
	for _, test := range tests {
		foundTest = foundTest || test.TargetMethod == "updateBoard"
	}
	if !foundTest {
		t.Fatal("Dart test mapping absent", tests)
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain DepartureService.updateBoard and verifyBoard in Dart", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	source, call := false, false
	for _, section := range pack.SourceSections {
		source = source || section.Path == "lib/service.dart" && strings.Contains(section.Content, "verifyBoard") && section.ReadReceipt != ""
	}
	for _, edge := range pack.CallChain {
		call = call || strings.Contains(edge.To, "verifyBoard")
	}
	if !source || !call || pack.FallbackRequired {
		t.Fatalf("source=%v call=%v pack=%#v", source, call, pack)
	}
}

func TestStructuredLanguagesReachNormalContextSource(t *testing.T) {
	fixtures := []struct{ file, body, query, proof string }{
		{"Bridge.m", "@implementation Bridge\n- (void)prepareBridgeEvidence {}\n- (void)runBridgeEvidence { [self prepareBridgeEvidence]; }\n@end", "Explain Bridge.runBridgeEvidence Objective-C", "[self prepareBridgeEvidence]"},
		{"bridge.cpp", "void prepareBridgeEvidence() {}\nvoid runBridgeEvidence() { prepareBridgeEvidence(); }\n", "Explain runBridgeEvidence C++", "prepareBridgeEvidence();"},
		{"Formula.rb", "class Formula\n def prepareBridgeEvidence()\n end\n def runBridgeEvidence()\n  self.prepareBridgeEvidence()\n end\nend\n", "Explain Formula.runBridgeEvidence Ruby", "self.prepareBridgeEvidence()"},
		{"Service.kt", "class Service {\n fun prepareBridgeEvidence() {}\n fun runBridgeEvidence() { prepareBridgeEvidence() }\n}\n", "Explain Service.runBridgeEvidence Kotlin", "prepareBridgeEvidence()"},
		{"run.cmd", "call :prepareBridgeEvidence\nexit /b\n:prepareBridgeEvidence\necho prepareBridgeEvidenced\nexit /b\n", "Explain prepareBridgeEvidence label Windows batch", "echo prepareBridgeEvidenced"},
		{"index.html", "<main id=\"board\">\n<h1>Board</h1>\n</main>\n", "Explain index.html board HTML element", "id=\"board\""},
		{"styles.css", ".board {\n color: white;\n}\n", "Explain styles.css board CSS selector", ".board"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.file, func(t *testing.T) {
			root := t.TempDir()
			writeSourceFile(t, root, fixture.file, fixture.body)
			cfg := config.Defaults()
			cfg.Workspace = false
			cfg.UpdateGitignore = false
			if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
				t.Fatal(err)
			}
			pack, err := BuildContext(ContextRequest{Root: root, Query: fixture.query, ProtocolVersion: AdaptiveV2})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, section := range pack.SourceSections {
				found = found || section.Path == fixture.file && strings.Contains(section.Content, fixture.proof) && section.ReadReceipt != ""
			}
			if !found {
				t.Fatalf("structured source unavailable: %#v", pack)
			}
		})
	}
}

func TestNativeDeepCrossFileCallsDeliverVerifiedContext(t *testing.T) {
	fixtures := []struct {
		name, query, caller, target, callerProof, targetProof string
		files                                                 map[string]string
	}{
		{"cpp", "Explain runBridgeEvidence C++", "main.cpp", "value.cpp", "prepareBridgeEvidence();", "int prepareBridgeEvidence()", map[string]string{"main.cpp": "#include \"value.h\"\nint runBridgeEvidence(){ return prepareBridgeEvidence(); }\n", "value.h": "int prepareBridgeEvidence();\n", "value.cpp": "int prepareBridgeEvidence(){ return 42; }\n"}},
		{"objectivec", "Explain Client.runBridgeEvidence Objective-C", "Client.m", "Service.m", "[service prepareBridgeEvidence]", "- (void)prepareBridgeEvidence", map[string]string{"Client.m": "#import \"Service.h\"\n@implementation Client\n- (void)runBridgeEvidence:(Service *)service { [service prepareBridgeEvidence]; }\n@end\n", "Service.h": "@interface Service\n- (void)prepareBridgeEvidence;\n@end\n", "Service.m": "#import \"Service.h\"\n@implementation Service\n- (void)prepareBridgeEvidence {}\n@end\n"}},
		{"ruby", "Explain Runner.runBridgeEvidence Ruby", "runner.rb", "service.rb", "Service.prepareBridgeEvidence()", "def self.prepareBridgeEvidence()", map[string]string{"runner.rb": "require_relative 'service'\nclass Runner\n def runBridgeEvidence()\n  Service.prepareBridgeEvidence()\n end\nend\n", "service.rb": "class Service\n def self.prepareBridgeEvidence()\n  42\n end\nend\n"}},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			for file, body := range fixture.files {
				writeSourceFile(t, root, file, body)
			}
			cfg := config.Defaults()
			cfg.Workspace = false
			cfg.UpdateGitignore = false
			if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
				t.Fatal(err)
			}
			pack, err := BuildContext(ContextRequest{Root: root, Query: fixture.query, ProtocolVersion: AdaptiveV2})
			if err != nil {
				t.Fatal(err)
			}
			caller, target := false, false
			for _, section := range pack.SourceSections {
				if section.ReadReceipt == "" {
					t.Fatal("missing source receipt", section)
				}
				caller = caller || section.Path == fixture.caller && strings.Contains(section.Content, fixture.callerProof)
				target = target || section.Path == fixture.target && strings.Contains(section.Content, fixture.targetProof)
			}
			if !caller || !target {
				t.Fatalf("call chain source missing: caller=%v target=%v pack=%#v", caller, target, pack)
			}
		})
	}
}
