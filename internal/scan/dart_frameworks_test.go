package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDartHTTPPersistenceAndStateEvidence(t *testing.T) {
	result := dartTestProject(map[string]string{"lib/api.dart": `import 'package:http/http.dart' as http; import 'package:dio/dio.dart'; import 'package:sqflite/sqflite.dart'; import 'package:shared_preferences/shared_preferences.dart'; import 'package:flutter_riverpod/flutter_riverpod.dart';
final boardProvider=Provider((ref)=>0);
class API {
 final http.Client client; final Dio dio; final Database db; final SharedPreferencesAsync prefs; final Uri base;
 API(this.client,this.dio,this.db,this.prefs,this.base);
 Future<void> load(WidgetRef ref) async { await client.get(Uri.parse('https://example.test/api/items?token=private')); await dio.post('/api/items'); await client.get(base.resolve('/api/board')); await db.query('items'); await prefs.setBool('enabled',true); ref.watch(boardProvider); }
}`})
	if len(result.code.APIContracts) != 3 || result.code.APIContracts[0].Path != "/api/items" {
		t.Fatal(result.code.APIContracts, result.facts.References)
	}
	found := map[CapabilityID]int{}
	for _, c := range result.capabilities {
		found[c.Capability]++
	}
	if found[CapabilityPersistence] != 2 || found[CapabilityMessaging] != 1 {
		t.Fatal(result.capabilities)
	}
	serialized, _ := json.Marshal(result.code.APIContracts)
	if strings.Contains(string(serialized), "private") {
		t.Fatal("query value leaked", string(serialized))
	}
}

func TestDartHTTPUnknownTargetsAndFrameworkShadows(t *testing.T) {
	for _, body := range []string{
		`import 'package:http/http.dart' as http; class Client { Future<void> get(Uri uri) async {} } void run(Client client) { client.get(Uri.parse('https://example.test/items')); }`,
		`import 'package:http/http.dart' as http; void run(dynamic http) { http.get(Uri.parse('https://example.test/items')); }`,
		`import 'package:http/http.dart' as http; void run(http.Client client,String host) { client.get(Uri.parse('https://$host/items')); }`,
		`import 'package:http/http.dart' as http; class Uri { static Uri parse(String text)=>Uri(); } void run(http.Client client) { client.get(Uri.parse('https://example.test/items')); }`,
		`import 'package:shared_preferences/shared_preferences.dart'; class SharedPreferencesAsync { void setBool(String key,bool value) {} } void run(SharedPreferencesAsync prefs) { prefs.setBool('x',true); }`,
	} {
		result := dartTestProject(map[string]string{"lib/api.dart": body})
		if len(result.code.APIContracts) > 0 {
			t.Fatal("unproven HTTP binding", body, result.code.APIContracts)
		}
		for _, cap := range result.capabilities {
			if cap.Capability == CapabilityPersistence {
				t.Fatal("shadowed storage promoted", cap)
			}
		}
	}
}

func TestDartAbortableRequestsAndUriVariables(t *testing.T) {
	result := dartTestProject(map[string]string{"lib/api.dart": `import 'package:http/http.dart' as http; class API { final http.Client client; final Uri base; API(this.client,this.base); Future<void> refresh() async { final request=http.AbortableRequest('POST',base.resolve('/v1/refresh'),abortTrigger:abort.future)..followRedirects=false; await client.send(request); final uri=Uri.https('example.test','/items'); await client.get(uri); request=unknown(); await client.send(request); } }`})
	if len(result.code.APIContracts) != 2 || result.code.APIContracts[0].HTTPMethod != "POST" || result.code.APIContracts[0].Path != "/v1/refresh" {
		t.Fatal(result.code.APIContracts)
	}
}

func TestDartContractsReachCrossLanguageWorkspaceMatching(t *testing.T) {
	result := dartTestProject(map[string]string{"lib/api.dart": `import 'package:http/http.dart' as http;
class DepartureAPI { final http.Client client; final Uri base; DepartureAPI(this.client,this.base); Future<void> refresh() async { final request=http.AbortableRequest('POST',base.resolve('/v1/refresh')); await client.send(request); } }`})
	provider := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "services/departures", Service: "departures", Kind: "backend"}, routes: []CodeRouteRecord{{Language: "go", Framework: "net/http", Kind: "backend", HTTPMethod: "POST", Path: "/v1/refresh", Handler: "refresh", File: "main.go", Line: 8}}}
	consumer := workspaceIndexProject{record: WorkspaceProjectRecord{Path: "apps/transit", Service: "transit", Kind: "frontend"}, contracts: result.code.APIContracts}
	matches := buildWorkspaceContractMatches([]workspaceIndexProject{provider, consumer})
	if len(matches) != 1 || matches[0].Issue != contractIssueMatched || matches[0].BackendProject != provider.record.Path {
		t.Fatal(matches)
	}
}
