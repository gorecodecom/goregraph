package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/doctor"
	"github.com/gorecodecom/goregraph/internal/query"
	"github.com/gorecodecom/goregraph/internal/scan"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
	Error   *responseError `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func Serve(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if encodeErr := encoder.Encode(errorResponse(nil, -32700, "parse error")); encodeErr != nil {
				return encodeErr
			}
			continue
		}
		if err := encoder.Encode(handle(req)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func handle(req request) response {
	switch req.Method {
	case "initialize":
		return okResponse(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]any{
				"name":    "goregraph",
				"version": "dev",
			},
			"capabilities": map[string]any{"tools": map[string]any{}},
		})
	case "tools/list":
		return okResponse(req.ID, map[string]any{"tools": tools()})
	case "tools/call":
		var params callParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errorResponse(req.ID, -32602, "invalid tool call params")
		}
		text, err := callTool(params.Name, params.Arguments)
		if err != nil {
			return errorResponse(req.ID, -32000, err.Error())
		}
		return okResponse(req.ID, map[string]any{
			"content": []map[string]string{{"type": "text", "text": text}},
		})
	default:
		return errorResponse(req.ID, -32601, "method not found")
	}
}

func tools() []map[string]any {
	return []map[string]any{
		tool("query_code_map", "Search the generated GoreGraph index."),
		tool("get_project_summary", "Read the generated project report."),
		tool("get_output", "Read a generated GoreGraph output by alias, for example graph-full, callgraph, routes, flows, api-contracts, package-graph, maven-graph, navigation, endpoint-flows, spring, endpoints, dependencies, analyzers, workspace, workspace-context, workspace-contracts, frontend-consumers, audit."),
		tool("get_file", "Read indexed metadata for a file."),
		tool("get_symbol", "Find indexed symbols by name."),
		tool("get_related_files", "Explain relations for a file."),
		tool("explain_file", "Explain one indexed file or symbol."),
		tool("doctor", "Check generated output health."),
	}
}

func tool(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"description": description,
		"inputSchema": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
	}
}

func callTool(name string, args map[string]any) (string, error) {
	root := stringArg(args, "root", ".")
	switch name {
	case "query_code_map":
		term := stringArg(args, "term", "")
		return query.Search(root, term)
	case "explain_file", "get_related_files":
		target := stringArg(args, "target", stringArg(args, "file", ""))
		return query.Explain(root, target)
	case "get_project_summary":
		return readReport(root)
	case "get_output":
		name := stringArg(args, "name", stringArg(args, "alias", ""))
		if strings.TrimSpace(name) == "" {
			return "", fmt.Errorf("name is required")
		}
		return query.Search(root, name)
	case "doctor":
		result, err := doctor.Run(root)
		if err != nil {
			return "", err
		}
		return result.String(), nil
	case "get_file":
		return getFile(root, stringArg(args, "file", ""))
	case "get_symbol":
		return getSymbol(root, stringArg(args, "symbol", stringArg(args, "name", "")))
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func readReport(root string) (string, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, "report.md"))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func getFile(root, filePath string) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file is required")
	}
	var files []scan.FileRecord
	if err := readIndexJSON(root, "files.json", &files); err != nil {
		return "", err
	}
	for _, file := range files {
		if file.Path == filepath.ToSlash(filePath) {
			body, _ := json.MarshalIndent(file, "", "  ")
			return string(body) + "\n", nil
		}
	}
	return "", fmt.Errorf("file %q not found in index", filePath)
}

func getSymbol(root, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("symbol is required")
	}
	var symbols []scan.SymbolRecord
	if err := readIndexJSON(root, "symbols.json", &symbols); err != nil {
		return "", err
	}
	var lines []string
	for _, symbol := range symbols {
		if strings.EqualFold(symbol.Name, name) {
			lines = append(lines, fmt.Sprintf("- `%s` (%s) in `%s:%d`", symbol.Name, symbol.Kind, symbol.File, symbol.Line))
		}
	}
	if len(lines) == 0 {
		return "No matching symbols.\n", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func readIndexJSON(root, name string, dest any) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func stringArg(args map[string]any, name, fallback string) string {
	value, ok := args[name]
	if !ok {
		return fallback
	}
	text, ok := value.(string)
	if !ok {
		return fallback
	}
	return text
}

func okResponse(id any, result any) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id any, code int, message string) response {
	return response{JSONRPC: "2.0", ID: id, Error: &responseError{Code: code, Message: message}}
}
