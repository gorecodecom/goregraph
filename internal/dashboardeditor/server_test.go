package dashboardeditor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const testDashboardHTML = "<!doctype html><html><head><title>fixture</title></head><body>map</body></html>"

type testEditorServer struct {
	t             *testing.T
	URL           string
	Token         string
	WorkspaceRoot string
	DashboardPath string
	cancel        context.CancelFunc
	done          chan error
	stopOnce      sync.Once
}

func TestServePublishesLoopbackFragmentAndCallsReadyBeforeOpen(t *testing.T) {
	root, dashboardPath := writeTestDashboard(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var events []string
	opened := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, Options{
			WorkspaceRoot: root,
			DashboardPath: dashboardPath,
			OnReady: func(rawURL string) {
				events = append(events, "ready")
			},
			OpenURL: func(rawURL string) error {
				events = append(events, "open")
				opened <- rawURL
				return errors.New("browser unavailable")
			},
		})
	}()

	var rawURL string
	select {
	case rawURL = <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not publish the editor URL")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" {
		t.Fatalf("editor URL = %q", rawURL)
	}
	fragment, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	token := fragment.Get("token")
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(token) {
		t.Fatalf("token = %q", token)
	}
	if parsed.RawQuery != "" || strings.Contains(parsed.Path, token) {
		t.Fatalf("token must only appear in URL fragment: %q", rawURL)
	}
	if strings.Join(events, ",") != "ready,open" {
		t.Fatalf("callback order = %v", events)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop after cancellation")
	}
}

func TestServerGetsAndSavesConfiguration(t *testing.T) {
	server := newTestServer(t)

	response := server.request(t, http.MethodGet, "/api/config", "", true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", response.StatusCode, readBody(t, response))
	}
	assertJSONContentType(t, response)
	var initial configResponse
	decodeResponse(t, response, &initial)
	if initial.Revision != "missing" || initial.Config.Schema != 1 {
		t.Fatalf("GET response = %+v", initial)
	}

	body := `{"revision":"missing","config":{"schema":1,"architecture":{"groups":{"group-a":{"label":"Group A"}}}}}`
	response = server.request(t, http.MethodPut, "/api/config", body, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", response.StatusCode, readBody(t, response))
	}
	var saved configResponse
	decodeResponse(t, response, &saved)
	if saved.Revision == "" || saved.Revision == "missing" || saved.Config.Architecture.Groups["group-a"].Label != "Group A" {
		t.Fatalf("PUT response = %+v", saved)
	}

	loaded, revision, err := scan.LoadWorkspaceDashboardConfig(server.WorkspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if revision != saved.Revision || loaded.Architecture.Groups["group-a"].Label != "Group A" {
		t.Fatalf("persisted config = %+v revision = %q", loaded, revision)
	}
}

func TestServerRejectsUnauthorizedRequestsAndRevisionConflicts(t *testing.T) {
	server := newTestServer(t)
	body := `{"revision":"missing","config":{"schema":1,"architecture":{}}}`

	request, err := http.NewRequest(http.MethodPut, server.URL+"/api/config", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response := doRequest(t, request)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", response.StatusCode)
	}
	response.Body.Close()

	request, err = http.NewRequest(http.MethodGet, server.URL+"/api/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-GoreGraph-Editor-Token", strings.Repeat("0", 64))
	response = doRequest(t, request)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d", response.StatusCode)
	}
	response.Body.Close()

	response = server.request(t, http.MethodPut, "/api/config", strings.Replace(body, "missing", "stale", 1), true)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("conflict status = %d, body = %s", response.StatusCode, readBody(t, response))
	}
	response.Body.Close()
}

func TestServerEnforcesHostOriginMethodsAndRoutes(t *testing.T) {
	server := newTestServer(t)

	request, err := http.NewRequest(http.MethodGet, server.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "attacker.example"
	response := doRequest(t, request)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("host status = %d", response.StatusCode)
	}
	response.Body.Close()

	request, err = http.NewRequest(http.MethodGet, server.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", "http://attacker.example")
	response = doRequest(t, request)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("origin status = %d", response.StatusCode)
	}
	response.Body.Close()

	for _, testCase := range []struct {
		method string
		path   string
		want   int
	}{
		{method: http.MethodPost, path: "/api/config", want: http.StatusMethodNotAllowed},
		{method: http.MethodDelete, path: "/", want: http.StatusMethodNotAllowed},
		{method: http.MethodGet, path: "/api/other", want: http.StatusNotFound},
		{method: http.MethodGet, path: "/README.md", want: http.StatusNotFound},
	} {
		response = server.request(t, testCase.method, testCase.path, "", true)
		if response.StatusCode != testCase.want {
			t.Errorf("%s %s status = %d, want %d", testCase.method, testCase.path, response.StatusCode, testCase.want)
		}
		if response.Header.Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("%s %s unexpectedly enables CORS", testCase.method, testCase.path)
		}
		response.Body.Close()
	}

	request, err = http.NewRequest(http.MethodPut, server.URL+"/api/config", strings.NewReader(`{"revision":"missing","config":{"schema":1,"architecture":{}}}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-GoreGraph-Editor-Token", server.Token)
	request.Header.Set("Content-Type", "application/json")
	response = doRequest(t, request)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("missing write Origin status = %d", response.StatusCode)
	}
	response.Body.Close()
}

func TestServerRejectsInvalidAndOversizedWrites(t *testing.T) {
	server := newTestServer(t)
	tests := []struct {
		name        string
		body        string
		contentType string
		want        int
	}{
		{name: "wrong content type", body: `{}`, contentType: "text/plain", want: http.StatusUnsupportedMediaType},
		{name: "malformed JSON", body: `{`, contentType: "application/json", want: http.StatusBadRequest},
		{name: "trailing JSON", body: `{"revision":"missing","config":{"schema":1,"architecture":{}}}{}`, contentType: "application/json", want: http.StatusBadRequest},
		{name: "unknown envelope field", body: `{"revision":"missing","config":{"schema":1,"architecture":{}},"extra":true}`, contentType: "application/json", want: http.StatusBadRequest},
		{name: "unknown config field", body: `{"revision":"missing","config":{"schema":1,"architecture":{},"secret":"no"}}`, contentType: "application/json", want: http.StatusBadRequest},
		{name: "invalid config", body: `{"revision":"missing","config":{"schema":2,"architecture":{}}}`, contentType: "application/json", want: http.StatusBadRequest},
		{name: "oversized", body: strings.Repeat(" ", 1<<20) + `{"revision":"missing","config":{"schema":1,"architecture":{}}}`, contentType: "application/json", want: http.StatusRequestEntityTooLarge},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := server.requestWithContentType(t, http.MethodPut, "/api/config", testCase.body, testCase.contentType, true)
			if response.StatusCode != testCase.want {
				t.Fatalf("status = %d, want %d, body = %s", response.StatusCode, testCase.want, readBody(t, response))
			}
			assertJSONContentType(t, response)
			response.Body.Close()
		})
	}
}

func TestServerInjectsEditorMetadataWithoutChangingDashboard(t *testing.T) {
	server := newTestServer(t)
	before, err := os.ReadFile(server.DashboardPath)
	if err != nil {
		t.Fatal(err)
	}

	response := server.request(t, http.MethodGet, "/", "", false)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	served := readBody(t, response)
	for _, want := range []string{`"editor_enabled":true`, `"api_base":"/api/config"`, "<title>fixture</title>"} {
		if !strings.Contains(served, want) {
			t.Fatalf("served HTML missing %q: %s", want, served)
		}
	}
	if strings.Contains(served, server.Token) {
		t.Fatal("served HTML contains the session token")
	}
	after, err := os.ReadFile(server.DashboardPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || string(after) != testDashboardHTML {
		t.Fatalf("dashboard file changed: %q", after)
	}
}

func TestServerServesOnlyAllowlistedSiblingAssets(t *testing.T) {
	server := newTestServer(t)
	assetDir := filepath.Join(filepath.Dir(server.DashboardPath), "workspace-map-assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	assetName := "code-usages-0123456789abcdef.js"
	if err := os.WriteFile(filepath.Join(assetDir, assetName), []byte("safe asset"), 0o644); err != nil {
		t.Fatal(err)
	}

	response := server.request(t, http.MethodGet, "/workspace-map-assets/"+assetName, "", false)
	if response.StatusCode != http.StatusOK || readBody(t, response) != "safe asset" {
		t.Fatalf("asset response status = %d", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("asset Content-Type = %q", response.Header.Get("Content-Type"))
	}

	for _, path := range []string{
		"/../../README.md",
		"/%2e%2e/%2e%2e/README.md",
		"/workspace-map-assets/%2e%2e/workspace-map.html",
		"/workspace-map-assets/%252e%252e%252fworkspace-map.html",
		"/workspace-map-assets/not-generated.js",
		"/workspace-map-assets/code-usages-0123456789abcdef.js/extra",
	} {
		response = server.request(t, http.MethodGet, path, "", false)
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d", path, response.StatusCode)
		}
		response.Body.Close()
	}
}

func TestServerRejectsSymlinkedAssetEscape(t *testing.T) {
	server := newTestServer(t)
	assetDir := filepath.Join(filepath.Dir(server.DashboardPath), "workspace-map-assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(server.WorkspaceRoot, "outside.js")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(assetDir, "code-usages-fedcba9876543210.js")
	if err := os.Symlink(outside, asset); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	response := server.request(t, http.MethodGet, "/workspace-map-assets/"+filepath.Base(asset), "", false)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("symlinked asset status = %d, body = %s", response.StatusCode, readBody(t, response))
	}
	response.Body.Close()
}

func TestServerHidesConfigurationFilesystemErrors(t *testing.T) {
	server := newTestServer(t)
	configPath := filepath.Join(server.WorkspaceRoot, scan.WorkspaceDashboardConfigName)
	if err := os.Mkdir(configPath, 0o755); err != nil {
		t.Fatal(err)
	}

	response := server.request(t, http.MethodGet, "/api/config", "", true)
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.StatusCode)
	}
	body := readBody(t, response)
	if strings.Contains(body, server.WorkspaceRoot) || strings.Contains(body, configPath) {
		t.Fatalf("filesystem path disclosed: %s", body)
	}
}

func TestServeRejectsMissingDashboardBeforeReady(t *testing.T) {
	readyCalled := false
	err := Serve(context.Background(), Options{
		WorkspaceRoot: t.TempDir(),
		DashboardPath: filepath.Join(t.TempDir(), "missing.html"),
		OnReady:       func(string) { readyCalled = true },
	})
	if err == nil {
		t.Fatal("Serve succeeded for a missing dashboard")
	}
	if readyCalled {
		t.Fatal("OnReady called before dashboard validation")
	}
}

type configResponse struct {
	Revision string                        `json:"revision"`
	Config   scan.WorkspaceDashboardConfig `json:"config"`
}

func newTestServer(t *testing.T) *testEditorServer {
	t.Helper()
	root, dashboardPath := writeTestDashboard(t)
	ready := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, Options{
			WorkspaceRoot: root,
			DashboardPath: dashboardPath,
			OnReady:       func(value string) { ready <- value },
		})
	}()

	var rawURL string
	select {
	case rawURL = <-ready:
	case err := <-done:
		t.Fatalf("Serve stopped before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not become ready")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	server := &testEditorServer{
		t:             t,
		URL:           "http://" + parsed.Host,
		Token:         fragment.Get("token"),
		WorkspaceRoot: root,
		DashboardPath: dashboardPath,
		cancel:        cancel,
		done:          done,
	}
	t.Cleanup(server.stop)
	return server
}

func (server *testEditorServer) stop() {
	server.stopOnce.Do(func() {
		server.cancel()
		select {
		case err := <-server.done:
			if err != nil {
				server.t.Errorf("Serve() = %v", err)
			}
		case <-time.After(5 * time.Second):
			server.t.Error("Serve did not stop after cancellation")
		}
	})
}

func (server *testEditorServer) request(t *testing.T, method, path, body string, api bool) *http.Response {
	t.Helper()
	contentType := ""
	if method == http.MethodPut {
		contentType = "application/json"
	}
	return server.requestWithContentType(t, method, path, body, contentType, api)
}

func (server *testEditorServer) requestWithContentType(t *testing.T, method, path, body, contentType string, api bool) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if api {
		request.Header.Set("X-GoreGraph-Editor-Token", server.Token)
	}
	request.Header.Set("Origin", server.URL)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return doRequest(t, request)
}

func writeTestDashboard(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dashboardPath := filepath.Join(root, ".goregraph-workspace", "dashboard", "workspace-map.html")
	if err := os.MkdirAll(filepath.Dir(dashboardPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dashboardPath, []byte(testDashboardHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, dashboardPath
}

func doRequest(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func decodeResponse(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func assertJSONContentType(t *testing.T, response *http.Response) {
	t.Helper()
	if response.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", response.Header.Get("Content-Type"))
	}
}
