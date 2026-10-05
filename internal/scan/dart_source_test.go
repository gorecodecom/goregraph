package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestDartFoundationDeclarationsAndOpaqueLiterals(t *testing.T) {
	source := parseDartSource(FileRecord{Path: "lib/model.dart"}, `import 'dart:async';
/* class Fake { /* nested */ void wrong() {} } */
abstract interface class Repository { Future<Item> load({required String id, int count = 1}); }
class Item {
 final String id;
 Item(this.id);
 factory Item.fromJson(Map<String, dynamic> json) => Item(json['id']);
 String get label => id;
 void update([int count = 1]) { final raw = r'''class Fake2 { void wrong() {} }'''; }
}
mixin Cache { void clear() {} }
extension ItemFormat on Item { String format() => label; }
enum Status { ready, failed; bool get isReady => this == ready; }
typedef Loader = Future<Item> Function(String id);
Future<Item> fetch(Repository repository) async => repository.load(id: 'x');`)
	if len(source.limitations) != 0 || len(source.types) != 6 {
		t.Fatalf("types=%#v limitations=%v", source.types, source.limitations)
	}
	names := map[string]bool{}
	for _, m := range source.members {
		names[m.symbol.Name] = true
	}
	for _, name := range []string{"load", "id", "Item", "Item.fromJson", "label", "update", "clear", "format", "isReady", "fetch"} {
		if !names[name] {
			t.Errorf("missing %s: %#v", name, source.members)
		}
	}
	if names["wrong"] {
		t.Fatal("string or comment introduced a declaration")
	}
	for _, m := range source.members {
		if m.symbol.Name == "load" && (len(m.parameters) != 2 || !m.parameters[0].named || !m.parameters[0].required) {
			t.Fatal(m.parameters)
		}
	}
	tokens, ok := dartTokens(`final message = "${render('class Fake {}')}"; final raw = r'$literal';`)
	if !ok {
		t.Fatal(tokens)
	}
	stringsSeen := 0
	for _, token := range tokens {
		if token.kind == "string" {
			stringsSeen++
			_, literal := dartLiteral(token)
			if stringsSeen == 1 && literal {
				t.Fatal("interpolation treated as literal")
			}
			if stringsSeen == 2 && !literal {
				t.Fatal("raw dollar is literal")
			}
		}
	}
	if stringsSeen != 2 {
		t.Fatal(tokens)
	}
}

func TestDartPackageMetadataAndDiscovery(t *testing.T) {
	body := `name: sample
environment:
  sdk: ^3.13.0
dependencies:
  flutter:
    sdk: flutter
  api:
    path: ../api
dev_dependencies:
  flutter_test:
    sdk: flutter
flutter:
  assets:
    - assets/logo.png
  fonts:
    - family: Example
      fonts:
        - asset: assets/font.ttf
secret: never_export_me
`
	p := extractDartPackage("app/pubspec.yaml", body)
	if p.Name != "sample" || !p.Flutter || len(p.Dependencies) != 3 || len(p.Assets) != 2 || p.Dependencies[1].Path != "../api" {
		t.Fatal(p)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pubspec.yaml"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasProjectMarker(root) || workspaceProjectBuildSystem(root) != "pub" {
		t.Fatal("pub package not discovered")
	}
	if detectLanguage("lib/main.dart") != "dart" || detectKind("pubspec.yaml") != "build" {
		t.Fatal("Dart detection")
	}
	if _, err := os.Stat(filepath.Join(root, ".dart_tool")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, file := range []string{"lib/main.dart", "lib/main.g.dart", ".dart_tool/hidden.dart"} {
		full := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("class Kept {}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var files []string
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	_, err := WalkProjectFiles(context.Background(), root, cfg, func(file WalkedFile) error { files = append(files, file.Path); return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Base(file) == "hidden.dart" {
			t.Fatal("tool output entered source index")
		}
	}
}
