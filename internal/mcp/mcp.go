package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agent"
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

type Options struct {
	ExpertTools bool
}

func Serve(input io.Reader, output io.Writer) error {
	return ServeWithOptions(input, output, Options{})
}

func ServeWithOptions(input io.Reader, output io.Writer, options Options) error {
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
		if err := encoder.Encode(handle(req, options)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func handle(req request, options Options) response {
	switch req.Method {
	case "initialize":
		return okResponse(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"instructions":    serverInstructions(),
			"serverInfo": map[string]any{
				"name":    "goregraph",
				"version": "dev",
			},
			"capabilities": map[string]any{"tools": map[string]any{}},
		})
	case "tools/list":
		return okResponse(req.ID, map[string]any{"tools": tools(options)})
	case "tools/call":
		var params callParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errorResponse(req.ID, -32602, "invalid tool call params")
		}
		text, err := callTool(options, params.Name, params.Arguments)
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

func tools(options Options) []map[string]any {
	listed := []map[string]any{taskContextTool()}
	if options.ExpertTools {
		listed = append(listed, legacyTools()...)
	}
	return listed
}

func taskContextTool() map[string]any {
	return map[string]any{
		"name":        "task_context",
		"description": "Return one evidence-backed Context Pack with current, line-numbered source for the central coding path. Treat source_sections as already read. When source_coverage is absent, partial, or none, inspect only relevant uncovered ranges. If fallback_required is true, inspect source directly. Call at most twice per task.",
		"inputSchema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"query"},
			"properties": map[string]any{
				"root":          map[string]any{"type": "string"},
				"query":         map[string]any{"type": "string", "minLength": 1},
				"budget_tokens": map[string]any{"type": "integer", "minimum": agent.MinContextBudgetTokens, "maximum": agent.MaxContextBudgetTokens, "default": agent.DefaultContextBudgetTokens},
				"max_files":     map[string]any{"type": "integer", "minimum": agent.MinContextMaxFiles, "maximum": agent.MaxContextMaxFiles, "default": agent.DefaultContextMaxFiles},
			},
		},
	}
}

func serverInstructions() string {
	return "Call task_context before Read or Grep for indexed code questions. " +
		"Treat source_sections as current source already read. " +
		"Do not re-read or grep included ranges. " +
		"If source_coverage is absent, partial, or none, inspect only relevant uncovered ranges from source_omissions or files. " +
		"If fallback_required is true, stop using GoreGraph and inspect source directly. " +
		"At most one narrower task_context retry may use an exact route, qualified symbol, or file returned by the first call."
}

func legacyTools() []map[string]any {
	return []map[string]any{
		tool("query_code_map", "Search the generated GoreGraph index."),
		tool("get_project_summary", "Read the generated project report."),
		tool("get_output", "Read a generated GoreGraph output by alias, for example graph-full, callgraph, routes, flows, api-contracts, package-graph, maven-graph, navigation, endpoint-flows, spring, endpoints, dependencies, analyzers, workspace, workspace-context, workspace-contracts, workspace-features, frontend-consumers, audit."),
		tool("get_file", "Read indexed metadata for a file."),
		tool("get_symbol", "Find indexed symbols by name."),
		tool("get_related_files", "Explain relations for a file."),
		tool("explain_file", "Explain one indexed file or symbol."),
		tool("doctor", "Check generated output health."),
		agentTool("workspace_summary", "Return a compact workspace orientation."),
		agentTool("workspace_delta", "Compare a before workspace snapshot with the current root."),
		agentTool("service_context", "Return generated context for a service."),
		agentTool("endpoint_search", "Search generated endpoint facts."),
		agentTool("endpoint_trace", "Return evidence-backed directed endpoint traces."),
		agentTool("symbol_trace", "Return directed traces containing a symbol."),
		agentTool("trace_from", "Traverse upstream and downstream from a trace node."),
		agentTool("data_flow", "Return evidence-backed data-flow mappings and explicit gaps."),
		agentTool("impact_summary", "Return bounded evidence-backed impact and blast-radius summaries."),
		agentTool("diagnostics", "Return canonical evidence-backed diagnostics."),
		agentTool("coverage", "Return analyzer capability coverage."),
		agentTool("evidence", "Return stable source evidence records."),
		agentTool("tests", "Return generated test mappings."),
		agentTool("change_context", "Return generated change context."),
		agentTool("symbol_inventory", "List canonical workspace symbol declarations by project, package, module, or name."),
		agentTool("symbol_resolve", "Resolve human text to every matching canonical symbol candidate without choosing one."),
		agentTool("symbol_usages", "Return exact direct references for a stable symbol ID; human text should first use symbol_resolve."),
		agentTool("symbol_api_consumers", "Return HTTP reachability consumers for a stable symbol ID; human text should first use symbol_resolve."),
		agentTool("symbol_explain", "Explain a stable symbol or usage ID from canonical projections; human text should first use symbol_resolve."),
	}
}

func agentTool(name, description string) map[string]any {
	return map[string]any{"name": name, "description": description, "inputSchema": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"root": map[string]any{"type": "string"}, "query": map[string]any{"type": "string"}, "detail": map[string]any{"type": "string", "enum": []string{"summary", "standard", "full"}}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}, "continuation": map[string]any{"type": "string"}}}}
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

func callTool(options Options, name string, args map[string]any) (string, error) {
	if name == "task_context" {
		return callTaskContext(args)
	}
	if !options.ExpertTools {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
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
		if task, ok := agentTaskForTool(name); ok {
			result, err := (agent.Service{}).Run(agent.Request{Root: root, Task: task, Query: stringArg(args, "query", ""), Detail: stringArg(args, "detail", "standard"), Limit: intArg(args, "limit", 20), Continuation: stringArg(args, "continuation", "")})
			if err != nil {
				return "", err
			}
			body, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return "", err
			}
			return string(body) + "\n", nil
		}
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func callTaskContext(args map[string]any) (string, error) {
	for name := range args {
		switch name {
		case "root", "query", "budget_tokens", "max_files":
		default:
			return "", fmt.Errorf("unknown task_context argument: %s", name)
		}
	}

	root := "."
	if value, ok := args["root"]; ok {
		var valid bool
		root, valid = value.(string)
		if !valid {
			return "", fmt.Errorf("task_context root must be a string")
		}
	}
	queryValue, ok := args["query"]
	if !ok {
		return "", fmt.Errorf("task_context query is required")
	}
	query, ok := queryValue.(string)
	if !ok || strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("task_context query must be a non-empty string")
	}
	budgetTokens, err := boundedIntegerArg(args, "budget_tokens", agent.MinContextBudgetTokens, agent.MaxContextBudgetTokens)
	if err != nil {
		return "", err
	}
	maxFiles, err := boundedIntegerArg(args, "max_files", agent.MinContextMaxFiles, agent.MaxContextMaxFiles)
	if err != nil {
		return "", err
	}
	pack, err := agent.BuildContext(agent.ContextRequest{
		Root:         root,
		Query:        query,
		BudgetTokens: budgetTokens,
		MaxFiles:     maxFiles,
	})
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(pack)
	if err != nil {
		return "", err
	}
	return string(body) + "\n", nil
}

func boundedIntegerArg(args map[string]any, name string, minimum, maximum int) (int, error) {
	value, ok := args[name]
	if !ok {
		return 0, nil
	}
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number {
		return 0, fmt.Errorf("task_context %s must be an integer", name)
	}
	if number < float64(minimum) || number > float64(maximum) {
		return 0, fmt.Errorf("task_context %s must be between %d and %d", name, minimum, maximum)
	}
	return int(number), nil
}

func agentTaskForTool(name string) (string, bool) {
	tasks := map[string]string{
		"workspace_summary":    "workspace-summary",
		"workspace_delta":      "workspace-delta",
		"service_context":      "service-context",
		"endpoint_search":      "endpoint-search",
		"endpoint_trace":       "endpoint-trace",
		"symbol_trace":         "symbol-trace",
		"trace_from":           "trace-from",
		"data_flow":            "data-flow",
		"impact_summary":       "impact-summary",
		"diagnostics":          "diagnostics",
		"coverage":             "coverage",
		"evidence":             "evidence",
		"tests":                "tests",
		"change_context":       "change-context",
		"symbol_inventory":     "symbol-inventory",
		"symbol_resolve":       "symbol-resolve",
		"symbol_usages":        "symbol-usages",
		"symbol_api_consumers": "symbol-api-consumers",
		"symbol_explain":       "symbol-explain",
	}
	task, ok := tasks[name]
	return task, ok
}

func readReport(root string) (string, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(scan.NewProjectOutputLayout(filepath.Join(root, cfg.OutputDir)).Dashboard("report.md"))
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
	body, err := os.ReadFile(scan.NewProjectOutputLayout(filepath.Join(root, cfg.OutputDir)).Index(name))
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
func intArg(args map[string]any, name string, fallback int) int {
	value, ok := args[name]
	if !ok {
		return fallback
	}
	number, ok := value.(float64)
	if !ok {
		return fallback
	}
	return int(number)
}

func okResponse(id any, result any) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id any, code int, message string) response {
	return response{JSONRPC: "2.0", ID: id, Error: &responseError{Code: code, Message: message}}
}
