package scan

import "testing"

func TestKotlinFrameworkEvidenceRequiresLiteralImportsAndTypedReceivers(t *testing.T) {
	file := FileRecord{Path: "src/Controller.kt", Language: "kotlin"}
	body := `package demo
import org.springframework.web.bind.annotation.RestController
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PostMapping
import io.ktor.client.HttpClient
import io.ktor.client.request.get
import io.flutter.plugin.common.MethodChannel
@RestController
@RequestMapping("/v1")
class Controller {
 @GetMapping("/departures")
 fun departures() { }
 @PostMapping(path = "/refresh")
 fun refresh() { }
 fun fetch(client: HttpClient, channel: MethodChannel) {
  client.get("https://api.example/v1/departures?token=secret")
  channel.invokeMethod("refresh", null)
 }
}`
	result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
	if len(result.code.Routes) != 2 || len(result.code.APIContracts) != 1 {
		t.Fatal(result.code.Routes, result.code.APIContracts)
	}
	if result.code.Routes[0].Path != "/v1/departures" || result.code.Routes[1].Path != "/v1/refresh" {
		t.Fatal(result.code.Routes)
	}
	if result.code.APIContracts[0].Path != "/v1/departures" {
		t.Fatal(result.code.APIContracts)
	}
	found := false
	for _, fact := range result.capabilities {
		if fact.Kind == "form_action" {
			t.Fatal("Kotlin request mislabeled as HTML", fact)
		}
		if fact.Framework == "Flutter MethodChannel" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing typed platform-channel evidence", result.capabilities)
	}
}

func TestKotlinFrameworkShadowsAndDynamicRoutesStayOpen(t *testing.T) {
	for _, body := range []string{
		`@RestController class Controller { @GetMapping("/fake") fun fake() {} }`,
		`import org.springframework.web.bind.annotation.RestController
import org.springframework.web.bind.annotation.GetMapping
class RestController {}
@RestController class Controller { @GetMapping("/fake") fun fake() {} }`,
		`import org.springframework.web.bind.annotation.RestController
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.RequestMapping
@RestController @RequestMapping(BASE) class Controller { @GetMapping("/fake") fun fake() {} }`,
	} {
		file := FileRecord{Path: "Controller.kt", Language: "kotlin"}
		result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
		if len(result.code.Routes) != 0 {
			t.Fatal("unproven route", result.code.Routes)
		}
	}
}
