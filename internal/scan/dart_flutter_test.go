package scan

import "testing"

func TestDartFlutterWidgetsCallbacksRoutesAndTests(t *testing.T) {
	result := dartTestProject(map[string]string{
		"lib/screen.dart":       `import 'package:flutter/material.dart'; class Screen extends StatelessWidget { Screen(); void reload() {} Widget build(BuildContext context) { return TextButton(onPressed: reload, child: Text('Reload')); } }`,
		"lib/main.dart":         `import 'package:flutter/material.dart'; import 'package:go_router/go_router.dart'; import 'screen.dart'; final router=GoRouter(routes: [GoRoute(path:'/items',builder:(context,state)=>Screen())]); void open(BuildContext context) { Navigator.pushNamed(context,'/items'); }`,
		"test/screen_test.dart": `import 'package:flutter_test/flutter_test.dart'; import '../lib/screen.dart'; void main() { testWidgets('shows screen',(tester) async { await tester.pumpWidget(Screen()); }); }`,
	})
	if len(result.code.Routes) != 1 || result.code.Routes[0].Kind != "frontend" || result.code.Routes[0].Path != "/items" {
		t.Fatal(result.code.Routes)
	}
	if !dartHasCall(result, "shows screen", "Screen") || len(result.tests) != 1 {
		t.Fatal(result.graph, result.tests)
	}
	found := map[string]bool{}
	for _, ref := range result.facts.References {
		found[ref.Type] = true
	}
	for _, kind := range []string{"registers_callback", "navigates_to", "instantiates_type"} {
		if !found[kind] {
			t.Error("missing", kind, result.facts.References)
		}
	}
	for _, function := range result.code.Functions {
		if function.Name == "main" && len(function.Calls) > 0 {
			t.Fatal("generic callback guesses enabled")
		}
	}
}

func TestDartFlutterFrameworkShadowsAndDynamicRoutes(t *testing.T) {
	for _, body := range []string{
		`class GoRoute { GoRoute({String? path}); } final route=GoRoute(path:'/items');`,
		`import 'package:go_router/go_router.dart'; final route=GoRoute(path:'/items/$id');`,
		`import 'package:go_router/go_router.dart' as router; void run(dynamic router) { router.GoRoute(path:'/items'); }`,
		`import 'package:flutter_test/flutter_test.dart'; void main(Function testWidgets) { testWidgets('fake',(tester) async {}); }`,
	} {
		result := dartTestProject(map[string]string{"lib/main.dart": body})
		if len(result.code.Routes) > 0 {
			t.Fatal("unproven framework route", result.code.Routes)
		}
		for _, f := range result.code.Functions {
			if f.Kind == "test" {
				t.Fatal("shadowed test function", f)
			}
		}
	}
}
