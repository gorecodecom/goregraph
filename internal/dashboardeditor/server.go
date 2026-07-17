// Package dashboardeditor serves a writable workspace dashboard on an authenticated loopback session.
package dashboardeditor

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	configPath           = "/api/config"
	tokenHeader          = "X-GoreGraph-Editor-Token"
	maxConfigBodyBytes   = 1 << 20
	readHeaderTimeout    = 5 * time.Second
	shutdownTimeout      = 5 * time.Second
	editorMetadata       = `<script>globalThis.__goregraphEditor={"editor_enabled":true,"api_base":"/api/config"};</script>`
	dashboardAssetPrefix = "/workspace-map-assets/"
)

var dashboardAssetName = regexp.MustCompile(`^code-usages-[0-9a-f]{16}\.js$`)

// Options configures one foreground dashboard editor session.
type Options struct {
	WorkspaceRoot string
	DashboardPath string
	OpenURL       func(string) error
	OnReady       func(string)
}

type editorHandler struct {
	workspaceRoot string
	dashboardPath string
	dashboardDir  string
	host          string
	origin        string
	tokenHash     [sha256.Size]byte
}

type configPayload struct {
	Revision string                        `json:"revision"`
	Config   scan.WorkspaceDashboardConfig `json:"config"`
}

type saveConfigRequest struct {
	Revision string                        `json:"revision"`
	Config   scan.WorkspaceDashboardConfig `json:"config"`
}

// Serve starts an ephemeral IPv4 loopback editor and blocks until cancellation or a server failure.
func Serve(ctx context.Context, options Options) error {
	if err := validateOptions(options); err != nil {
		return err
	}

	token, err := newSessionToken()
	if err != nil {
		return fmt.Errorf("create dashboard editor session: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start dashboard editor listener: %w", err)
	}

	host := listener.Addr().String()
	origin := "http://" + host
	handler := &editorHandler{
		workspaceRoot: options.WorkspaceRoot,
		dashboardPath: options.DashboardPath,
		dashboardDir:  filepath.Dir(options.DashboardPath),
		host:          host,
		origin:        origin,
		tokenHash:     sha256.Sum256([]byte(token)),
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()

	editorURL := origin + "/#token=" + url.QueryEscape(token)
	if options.OnReady != nil {
		options.OnReady(editorURL)
	}
	if options.OpenURL != nil {
		_ = options.OpenURL(editorURL)
	}

	select {
	case err := <-serveResult:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("stop dashboard editor server: %w", err)
		}
		err := <-serveResult
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func validateOptions(options Options) error {
	if strings.TrimSpace(options.WorkspaceRoot) == "" {
		return errors.New("dashboard editor workspace root is required")
	}
	info, err := os.Stat(options.WorkspaceRoot)
	if err != nil {
		return errors.New("dashboard editor workspace is unavailable")
	}
	if !info.IsDir() {
		return errors.New("dashboard editor workspace is not a directory")
	}
	if strings.TrimSpace(options.DashboardPath) == "" {
		return errors.New("dashboard editor path is required")
	}
	info, err = os.Stat(options.DashboardPath)
	if err != nil {
		return errors.New("dashboard editor file is unavailable")
	}
	if !info.Mode().IsRegular() {
		return errors.New("dashboard editor file is not a regular file")
	}
	return nil
}

func newSessionToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (handler *editorHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Content-Type-Options", "nosniff")

	if request.Host != handler.host || request.Header.Get("Origin") != "" && request.Header.Get("Origin") != handler.origin {
		writeError(response, http.StatusForbidden, "request is not from this dashboard editor session")
		return
	}
	if unsafeURLPath(request.URL) {
		writeError(response, http.StatusNotFound, "not found")
		return
	}

	switch {
	case request.URL.Path == "/":
		handler.serveDashboard(response, request)
	case request.URL.Path == configPath:
		handler.serveConfig(response, request)
	case strings.HasPrefix(request.URL.Path, dashboardAssetPrefix):
		handler.serveAsset(response, request)
	default:
		writeError(response, http.StatusNotFound, "not found")
	}
}

func (handler *editorHandler) serveDashboard(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	body, err := os.ReadFile(handler.dashboardPath)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "dashboard is unavailable")
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(injectEditorMetadata(body))
}

func (handler *editorHandler) serveConfig(response http.ResponseWriter, request *http.Request) {
	if !handler.authenticated(request.Header.Get(tokenHeader)) {
		writeError(response, http.StatusUnauthorized, "dashboard editor authentication required")
		return
	}

	switch request.Method {
	case http.MethodGet:
		handler.getConfig(response)
	case http.MethodPut:
		if request.Header.Get("Origin") != handler.origin {
			writeError(response, http.StatusForbidden, "request is not from this dashboard editor session")
			return
		}
		handler.putConfig(response, request)
	default:
		response.Header().Set("Allow", http.MethodGet+", "+http.MethodPut)
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (handler *editorHandler) authenticated(provided string) bool {
	providedHash := sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(providedHash[:], handler.tokenHash[:]) == 1
}

func (handler *editorHandler) getConfig(response http.ResponseWriter) {
	config, revision, err := scan.LoadWorkspaceDashboardConfig(handler.workspaceRoot)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "dashboard configuration is unavailable")
		return
	}
	writeJSON(response, http.StatusOK, configPayload{Revision: revision, Config: config})
}

func (handler *editorHandler) putConfig(response http.ResponseWriter, request *http.Request) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(response, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	request.Body = http.MaxBytesReader(response, request.Body, maxConfigBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var saveRequest saveConfigRequest
	if err := decoder.Decode(&saveRequest); err != nil {
		writeDecodeError(response, err)
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		writeDecodeError(response, err)
		return
	}
	if strings.TrimSpace(saveRequest.Revision) == "" {
		writeError(response, http.StatusBadRequest, "revision must not be blank")
		return
	}
	if err := scan.ValidateWorkspaceDashboardConfig(saveRequest.Config); err != nil {
		writeError(response, http.StatusBadRequest, err.Error())
		return
	}

	revision, err := scan.SaveWorkspaceDashboardConfig(handler.workspaceRoot, saveRequest.Revision, saveRequest.Config)
	if errors.Is(err, scan.ErrDashboardConfigConflict) {
		writeError(response, http.StatusConflict, "dashboard configuration changed; reload before saving")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "dashboard configuration could not be saved")
		return
	}
	writeJSON(response, http.StatusOK, configPayload{Revision: revision, Config: saveRequest.Config})
}

func (handler *editorHandler) serveAsset(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	name := strings.TrimPrefix(request.URL.Path, dashboardAssetPrefix)
	if !dashboardAssetName.MatchString(name) {
		writeError(response, http.StatusNotFound, "not found")
		return
	}

	assetPath := filepath.Join(handler.dashboardDir, "workspace-map-assets", name)
	info, err := os.Lstat(assetPath)
	if errors.Is(err, os.ErrNotExist) || err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		writeError(response, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "dashboard asset is unavailable")
		return
	}
	if !handler.assetStaysInDashboard(assetPath, name) {
		writeError(response, http.StatusNotFound, "not found")
		return
	}
	body, err := os.ReadFile(assetPath)
	if errors.Is(err, os.ErrNotExist) {
		writeError(response, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "dashboard asset is unavailable")
		return
	}
	response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(body)
}

func (handler *editorHandler) assetStaysInDashboard(assetPath, name string) bool {
	dashboardDir, err := filepath.EvalSymlinks(handler.dashboardDir)
	if err != nil {
		return false
	}
	asset, err := filepath.EvalSymlinks(assetPath)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(dashboardDir, asset)
	if err != nil {
		return false
	}
	return filepath.ToSlash(relative) == "workspace-map-assets/"+name
}

func unsafeURLPath(requestURL *url.URL) bool {
	value := requestURL.EscapedPath()
	for range 3 {
		decoded, err := url.PathUnescape(value)
		if err != nil {
			return true
		}
		value = decoded
	}
	if strings.ContainsAny(value, "\\\x00") {
		return true
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func injectEditorMetadata(body []byte) []byte {
	position := bytes.Index(bytes.ToLower(body), []byte("</head>"))
	if position < 0 {
		result := make([]byte, 0, len(editorMetadata)+len(body))
		result = append(result, editorMetadata...)
		return append(result, body...)
	}
	result := make([]byte, 0, len(editorMetadata)+len(body))
	result = append(result, body[:position]...)
	result = append(result, editorMetadata...)
	return append(result, body[position:]...)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request contains multiple JSON values")
		}
		return err
	}
	return nil
}

func writeDecodeError(response http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(response, http.StatusRequestEntityTooLarge, "request body exceeds 1 MiB")
		return
	}
	writeError(response, http.StatusBadRequest, err.Error())
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, struct {
		Error string `json:"error"`
	}{Error: message})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
