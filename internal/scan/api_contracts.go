package scan

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	codeHelperStartRE = regexp.MustCompile(`\b(Get|Post|Put|Patch|Delete)Helper(?:WithStatus)?\s*\(`)
	codeFetchAPIRE    = regexp.MustCompile("\\bfetch\\s*\\(\\s*(?:\"([^\"]+)\"|'([^']+)'|`([^`]+)`)")
	codeMethodRE      = regexp.MustCompile(`\bmethod\s*:\s*["']([A-Za-z]+)["']`)
	codePathLiteralRE = regexp.MustCompile(`["'](/[^"']+)["']|` + "`" + `(/[^` + "`" + `]+)` + "`")
	codeTemplateVarRE = regexp.MustCompile(`\$\{([^}]+)\}`)
)

func extractAPIContracts(file FileRecord, lines []string, functions []CodeFunctionRecord) []APIContractRecord {
	if isLowSignalCodeFile(file.Path) {
		return nil
	}
	switch file.Language {
	case "javascript", "typescript":
	default:
		return nil
	}

	var records []APIContractRecord
	for i, line := range lines {
		if match := codeHelperStartRE.FindStringSubmatch(line); len(match) == 2 {
			callText := collectCallText(lines, i, 5)
			if path, ok := firstPathLiteral(callText); ok {
				records = append(records, apiContract(file, helperHTTPMethod(match[1]), path, apiContractCaller(functions, i+1), i+1, "helper-call-argument"))
			}
			continue
		}
		if match := codeFetchAPIRE.FindStringSubmatch(line); len(match) == 4 {
			method := "GET"
			if methodMatch := codeMethodRE.FindStringSubmatch(line); len(methodMatch) == 2 {
				method = strings.ToUpper(methodMatch[1])
			}
			records = append(records, apiContract(file, method, firstNonEmpty(match[1], match[2], match[3]), apiContractCaller(functions, i+1), i+1, "fetch-call"))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].File != records[j].File {
			return records[i].File < records[j].File
		}
		if records[i].Line != records[j].Line {
			return records[i].Line < records[j].Line
		}
		return records[i].Path < records[j].Path
	})
	return records
}

func apiContractCaller(functions []CodeFunctionRecord, line int) string {
	var best CodeFunctionRecord
	for _, function := range functions {
		if function.Line <= 0 || function.EndLine <= 0 {
			continue
		}
		if line < function.Line || line > function.EndLine {
			continue
		}
		if best.Name == "" || function.Line >= best.Line {
			best = function
		}
	}
	return best.Name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func apiContract(file FileRecord, method, path, caller string, line int, reason string) APIContractRecord {
	normalizedPath, query, params, unsafeDynamic := normalizeAPIPathDetails(path)
	serviceCandidate := serviceCandidateForPath(normalizedPath)
	if isFrontendInternalAPIPath(file.Path, normalizedPath) {
		serviceCandidate = ""
		reason += "; frontend-internal-api-route"
	}
	return APIContractRecord{
		Language:         file.Language,
		App:              codeFileApp(file.Path),
		Package:          codeFilePackage(file.Path),
		HTTPMethod:       method,
		Path:             normalizedPath,
		RawPath:          path,
		Query:            query,
		QueryParams:      params,
		ServiceCandidate: serviceCandidate,
		UnsafeDynamic:    unsafeDynamic,
		Caller:           strings.TrimSpace(caller),
		File:             file.Path,
		Line:             line,
		Confidence:       "EXTRACTED",
		ConfidenceScore:  0.9,
		Reason:           reason,
	}
}

func isFrontendInternalAPIPath(filePath, apiPath string) bool {
	if !strings.HasPrefix(apiPath, "/api/") {
		return false
	}
	path := filepath.ToSlash(filePath)
	if strings.Contains(path, "/app/api/") || strings.Contains(path, "/pages/api/") {
		return false
	}
	return strings.Contains(path, "/src/app/") ||
		strings.HasPrefix(path, "src/app/") ||
		strings.HasPrefix(path, "app/") ||
		strings.Contains(path, "/app/")
}

func helperHTTPMethod(name string) string {
	name = strings.TrimSuffix(name, "Helper")
	name = strings.TrimSuffix(name, "HelperWithStatus")
	switch strings.ToLower(name) {
	case "get":
		return "GET"
	case "post":
		return "POST"
	case "put":
		return "PUT"
	case "patch":
		return "PATCH"
	case "delete":
		return "DELETE"
	default:
		return strings.ToUpper(name)
	}
}

func collectCallText(lines []string, start, maxLines int) string {
	depth := 0
	seenOpen := false
	var parts []string
	for i := start; i < len(lines) && i < start+maxLines; i++ {
		line := stripCodeLineComment("javascript", lines[i])
		parts = append(parts, strings.TrimSpace(line))
		depth += strings.Count(line, "(")
		if strings.Contains(line, "(") {
			seenOpen = true
		}
		depth -= strings.Count(line, ")")
		if seenOpen && depth <= 0 {
			break
		}
	}
	return strings.Join(parts, " ")
}

func firstPathLiteral(callText string) (string, bool) {
	for _, match := range codePathLiteralRE.FindAllStringSubmatch(callText, -1) {
		for _, group := range match[1:] {
			if strings.HasPrefix(group, "/") {
				return group, true
			}
		}
	}
	return "", false
}

func normalizeAPIPath(path string) string {
	normalized, _, _, _ := normalizeAPIPathDetails(path)
	return normalized
}

func normalizeAPIPathDetails(raw string) (string, string, []QueryParamRecord, bool) {
	raw = strings.TrimSpace(raw)
	rawPath, query := splitRawPathQuery(raw)
	path, unsafePath := normalizeTemplatePath(rawPath)
	params, unsafeQuery := normalizeQueryParams(query)
	return path, query, params, unsafePath || unsafeQuery
}

func splitRawPathQuery(raw string) (string, string) {
	templateDepth := 0
	for i := 0; i < len(raw); i++ {
		if i+1 < len(raw) && raw[i] == '$' && raw[i+1] == '{' {
			templateDepth++
			i++
			continue
		}
		if raw[i] == '}' && templateDepth > 0 {
			templateDepth--
			continue
		}
		if raw[i] == '?' && templateDepth == 0 {
			return raw[:i], raw[i+1:]
		}
	}
	return raw, ""
}

func normalizeTemplatePath(path string) (string, bool) {
	unsafeDynamic := false
	var trimmed bool
	path, trimmed = trimOptionalTemplatePathSuffix(path)
	if trimmed {
		unsafeDynamic = false
	}
	path = codeTemplateVarRE.ReplaceAllStringFunc(path, func(match string) string {
		groups := codeTemplateVarRE.FindStringSubmatch(match)
		if len(groups) != 2 {
			unsafeDynamic = true
			return "{dynamic}"
		}
		name, safe := normalizeTemplateExpression(groups[1])
		if !safe {
			unsafeDynamic = true
		}
		return "{" + name + "}"
	})
	return normalizeCodeRoutePath(path), unsafeDynamic
}

func trimOptionalTemplatePathSuffix(path string) (string, bool) {
	idx := strings.LastIndex(path, "${")
	if idx < 0 {
		return path, false
	}
	end := strings.Index(path[idx:], "}")
	expression := path[idx+2:]
	if end >= 0 {
		expression = path[idx+2 : idx+end]
	}
	trimStart := idx
	if strings.Contains(expression, "?") {
		if idx > 0 && path[idx-1] == '/' {
			trimStart = idx - 1
		}
		return path[:trimStart], true
	}
	if strings.Contains(expression, "||") && idx > 0 && path[idx-1] != '/' {
		return path[:idx], true
	}
	if end >= 0 && idx > 0 && path[idx-1] != '/' && isOptionalPathSuffixExpression(expression) {
		return path[:idx], true
	}
	return path, false
}

func isOptionalPathSuffixExpression(expression string) bool {
	switch strings.ToLower(strings.TrimSpace(expression)) {
	case "filter", "query", "params", "search":
		return true
	default:
		return false
	}
}

func normalizeQueryParams(query string) ([]QueryParamRecord, bool) {
	if query == "" {
		return nil, false
	}
	unsafeDynamic := false
	var params []QueryParamRecord
	for _, pair := range strings.Split(query, "&") {
		if pair == "" {
			continue
		}
		name, value, _ := strings.Cut(pair, "=")
		normalizedValue, unsafeValue := normalizeTemplateValue(value)
		if unsafeValue {
			unsafeDynamic = true
		}
		params = append(params, QueryParamRecord{Name: strings.TrimSpace(name), Value: normalizedValue})
	}
	sort.Slice(params, func(i, j int) bool {
		if params[i].Name != params[j].Name {
			return params[i].Name < params[j].Name
		}
		return params[i].Value < params[j].Value
	})
	return params, unsafeDynamic
}

func normalizeTemplateValue(value string) (string, bool) {
	unsafeDynamic := false
	value = codeTemplateVarRE.ReplaceAllStringFunc(value, func(match string) string {
		groups := codeTemplateVarRE.FindStringSubmatch(match)
		if len(groups) != 2 {
			unsafeDynamic = true
			return "{dynamic}"
		}
		name, safe := normalizeTemplateExpression(groups[1])
		if !safe {
			unsafeDynamic = true
		}
		return "{" + name + "}"
	})
	return value, unsafeDynamic
}

func normalizeTemplateExpression(expr string) (string, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" || strings.ContainsAny(expr, "?:+-*/<>=&|![](){}'\"` ") {
		return "dynamic", false
	}
	parts := strings.FieldsFunc(expr, func(r rune) bool {
		return r == '.' || r == '_'
	})
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return sanitizePlaceholder(parts[len(parts)-1]), true
	}
	return sanitizePlaceholder(expr), true
}

func sanitizePlaceholder(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "dynamic"
	}
	return b.String()
}

func serviceCandidateForPath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	segment := strings.ToLower(strings.Split(path, "/")[0])
	segment = strings.Trim(segment, "{}")
	switch segment {
	case "cadasters", "cadastermgmt", "cadastertask":
		return "ms-cadaster"
	case "users":
		return "ms-userservice"
	case "products":
		return "ms-productservice"
	case "tasks":
		return "ms-task"
	case "licenses":
		return "ms-licenseservice"
	}
	segment = strings.TrimSuffix(segment, "s")
	if segment == "" || segment == "dynamic" {
		return ""
	}
	return "ms-" + segment
}

func renderAPIContractsReport(records []APIContractRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph API Contracts\n\n")
	if len(records) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, record := range records {
		dynamic := ""
		if record.UnsafeDynamic {
			dynamic = ", unsafe dynamic URL"
		}
		query := ""
		if record.Query != "" {
			query = "?" + record.Query
		}
		caller := ""
		if record.Caller != "" {
			caller = fmt.Sprintf(", caller `%s`", record.Caller)
		}
		b.WriteString(fmt.Sprintf("- %s `%s%s` from `%s:%d` (app `%s`, service `%s`%s, %s%s)\n",
			record.HTTPMethod,
			record.Path,
			query,
			record.File,
			record.Line,
			record.App,
			emptyAsNone(record.ServiceCandidate),
			caller,
			record.Reason,
			dynamic,
		))
	}
	return b.String()
}
