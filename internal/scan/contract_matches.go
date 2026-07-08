package scan

import (
	"fmt"
	"sort"
	"strings"
)

const (
	contractIssueMatched                    = "matched"
	contractIssueMethodMismatch             = "method_mismatch"
	contractIssueMissingRoute               = "missing_backend_route"
	contractIssueScannedServiceNoRoute      = "scanned_service_no_route"
	contractIssueIndexedBackendRouteMissing = "indexed_backend_route_missing"
	contractIssueDynamicEndpointUnresolved  = "dynamic_endpoint_unresolved"
	contractIssueGatewayOrProxyPrefix       = "gateway_or_proxy_prefix"
	contractIssueFrontendInternalAPI        = "frontend_internal_api"
	contractIssueUnscanned                  = "unscanned_service"
	contractIssueUnsafeDynamic              = "unsafe_dynamic"
)

func buildContractMatches(contracts []APIContractRecord, routes []CodeRouteRecord) []ContractMatchRecord {
	backendRoutes := backendCodeRoutes(routes)
	scannedServices := backendRouteServices(backendRoutes)
	records := make([]ContractMatchRecord, 0, len(contracts))
	for _, contract := range contracts {
		if isFrontendInternalAPIContract(contract) {
			records = append(records, contractIssue(contract, CodeRouteRecord{}, contractIssueFrontendInternalAPI, "OUT_OF_SCOPE", 0.8, "frontend-internal API route; not matched against backend services"))
			continue
		}
		if route, ok := exactContractRoute(contract, backendRoutes); ok {
			records = append(records, contractIssue(contract, route, contractIssueMatched, "RESOLVED", 0.9, "http method and path pattern match backend route"))
			continue
		}
		if route, ok := pathCompatibleContractRoute(contract, backendRoutes); ok {
			records = append(records, contractIssue(contract, route, contractIssueMethodMismatch, "WEAK_MATCH", 0.45, "path pattern exists but http method differs"))
			continue
		}
		if route, ok := gatewayPrefixCompatibleContractRoute(contract, backendRoutes); ok {
			records = append(records, contractIssue(contract, route, contractIssueGatewayOrProxyPrefix, "WEAK_MATCH", 0.4, "path pattern matches after removing a common gateway or proxy prefix"))
			continue
		}
		if contract.UnsafeDynamic {
			records = append(records, contractIssue(contract, CodeRouteRecord{}, contractIssueUnsafeDynamic, "WEAK_MATCH", 0.35, "api path contains complex dynamic expression"))
			continue
		}
		if contract.ServiceCandidate != "" && !scannedServices[contract.ServiceCandidate] {
			records = append(records, contractIssue(contract, CodeRouteRecord{}, contractIssueUnscanned, "OUT_OF_SCOPE", 0.75, contract.ServiceCandidate+" was not scanned; contract cannot be matched in this run"))
			continue
		}
		if contract.ServiceCandidate != "" && scannedServices[contract.ServiceCandidate] {
			issue, reason := indexedBackendRouteGapIssue(contract, contract.ServiceCandidate)
			if similar := similarCodeRouteHints(contract, backendRoutes, 3); len(similar) > 0 {
				reason += "; similar backend routes: " + strings.Join(similar, ", ")
			}
			records = append(records, contractIssue(contract, CodeRouteRecord{}, issue, "WEAK_MATCH", 0.35, reason))
			continue
		}
		records = append(records, contractIssue(contract, CodeRouteRecord{}, contractIssueMissingRoute, "WEAK_MATCH", 0.3, "no compatible backend route found"))
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].APIFile != records[j].APIFile {
			return records[i].APIFile < records[j].APIFile
		}
		if records[i].APILine != records[j].APILine {
			return records[i].APILine < records[j].APILine
		}
		if records[i].Issue != records[j].Issue {
			return records[i].Issue < records[j].Issue
		}
		return records[i].APIPath < records[j].APIPath
	})
	return records
}

func backendCodeRoutes(routes []CodeRouteRecord) []CodeRouteRecord {
	var backend []CodeRouteRecord
	for _, route := range routes {
		if route.Kind == "backend" {
			backend = append(backend, route)
		}
	}
	return backend
}

func backendRouteServices(routes []CodeRouteRecord) map[string]bool {
	services := map[string]bool{}
	for _, route := range routes {
		service := serviceCandidateForPath(route.Path)
		if service != "" {
			services[service] = true
		}
	}
	return services
}

func exactContractRoute(contract APIContractRecord, routes []CodeRouteRecord) (CodeRouteRecord, bool) {
	for _, route := range routes {
		if strings.EqualFold(contract.HTTPMethod, route.HTTPMethod) && pathsCompatibleWithKnownBasePrefixes(contract.Path, route.Path) {
			return route, true
		}
	}
	return CodeRouteRecord{}, false
}

func pathCompatibleContractRoute(contract APIContractRecord, routes []CodeRouteRecord) (CodeRouteRecord, bool) {
	for _, route := range routes {
		if pathsCompatibleWithKnownBasePrefixes(contract.Path, route.Path) {
			return route, true
		}
	}
	return CodeRouteRecord{}, false
}

func gatewayPrefixCompatibleContractRoute(contract APIContractRecord, routes []CodeRouteRecord) (CodeRouteRecord, bool) {
	for _, route := range routes {
		if strings.EqualFold(contract.HTTPMethod, route.HTTPMethod) && pathsCompatibleWithoutGatewayPrefix(contract.Path, route.Path) {
			return route, true
		}
	}
	return CodeRouteRecord{}, false
}

func pathsCompatible(left, right string) bool {
	leftParts := routeParts(left)
	rightParts := routeParts(right)
	if len(leftParts) != len(rightParts) {
		return false
	}
	for i := range leftParts {
		leftPlaceholder := isPlaceholder(leftParts[i])
		rightPlaceholder := isPlaceholder(rightParts[i])
		if leftPlaceholder && rightPlaceholder {
			continue
		}
		if placeholderCompatibleWithStatic(leftParts[i], rightParts[i]) || placeholderCompatibleWithStatic(rightParts[i], leftParts[i]) {
			continue
		}
		if leftPlaceholder != rightPlaceholder {
			return false
		}
		if !strings.EqualFold(leftParts[i], rightParts[i]) {
			return false
		}
	}
	return true
}

func pathsCompatibleWithKnownBasePrefixes(left, right string) bool {
	if pathsCompatible(left, right) {
		return true
	}
	for _, leftVariant := range knownBasePrefixPathVariants(left) {
		for _, rightVariant := range knownBasePrefixPathVariants(right) {
			if pathsCompatible(leftVariant, rightVariant) {
				return true
			}
		}
	}
	return false
}

func routeParts(path string) []string {
	path = expandKnownPathConstants(path)
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func isPlaceholder(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func placeholderCompatibleWithStatic(placeholder, static string) bool {
	if !isPlaceholder(placeholder) || isPlaceholder(static) {
		return false
	}
	switch strings.ToLower(strings.Trim(placeholder, "{}")) {
	case "type":
		switch strings.ToLower(static) {
		case "new", "changed", "guidance":
			return true
		}
	}
	return false
}

func expandKnownPathConstants(path string) string {
	replacements := map[string]string{
		"RegulationChangeBaseController.PATH_BASE":                      "/cadasters",
		"RegulationChangeBaseController.PATH_FRAGMENT_CHANGES_NEW":      "/{cadasterId}/regulations/changes/new",
		"RegulationChangeBaseController.PATH_FRAGMENT_CHANGES_CHANGED":  "/{cadasterId}/regulations/changes/changed",
		"RegulationChangeBaseController.PATH_FRAGMENT_CHANGES_GUIDANCE": "/{cadasterId}/regulations/changes/guidance",
	}
	for token, value := range replacements {
		path = strings.ReplaceAll(path, token, value)
	}
	path = strings.ReplaceAll(path, ` + "`, "")
	path = strings.ReplaceAll(path, `"`, "")
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return path
}

func knownBasePrefixPathVariants(path string) []string {
	variants := []string{path}
	expanded := expandKnownPathConstants(path)
	parts := routeParts(expanded)
	if len(parts) < 2 {
		return variants
	}
	first := parts[0]
	if isConfigBasePathSegment(first) || isServiceBasePathSegment(first) {
		variants = append(variants, "/"+strings.Join(parts[1:], "/"))
	}
	return variants
}

func displayRoutePath(path string) string {
	variants := knownBasePrefixPathVariants(path)
	if len(variants) > 1 {
		return variants[1]
	}
	return normalizeCodeRoutePath(expandKnownPathConstants(path))
}

func isConfigBasePathSegment(segment string) bool {
	lower := strings.ToLower(segment)
	return strings.Contains(lower, ".base_path") || strings.Contains(lower, "base_path")
}

func isServiceBasePathSegment(segment string) bool {
	lower := strings.ToLower(segment)
	switch lower {
	case "cadastertask",
		"containertree",
		"documentdownload",
		"documentexport",
		"documentinfo",
		"documenttopic",
		"invoiceservice",
		"productservice",
		"task",
		"userservice":
		return true
	default:
		return strings.HasSuffix(lower, "service")
	}
}

func pathsCompatibleWithoutGatewayPrefix(left, right string) bool {
	if pathsCompatibleWithKnownBasePrefixes(left, right) {
		return false
	}
	for _, leftVariant := range gatewayPrefixPathVariants(left) {
		if pathsCompatibleWithKnownBasePrefixes(leftVariant, right) {
			return true
		}
	}
	for _, rightVariant := range gatewayPrefixPathVariants(right) {
		if pathsCompatibleWithKnownBasePrefixes(left, rightVariant) {
			return true
		}
	}
	return false
}

func gatewayPrefixPathVariants(path string) []string {
	parts := routeParts(path)
	if len(parts) < 2 || !isGatewayPrefixSegment(parts[0]) {
		return nil
	}
	return []string{"/" + strings.Join(parts[1:], "/")}
}

func isGatewayPrefixSegment(segment string) bool {
	switch strings.ToLower(segment) {
	case "api", "rest", "backend", "proxy", "gateway":
		return true
	default:
		return false
	}
}

func isFrontendInternalAPIContract(contract APIContractRecord) bool {
	return strings.Contains(contract.Reason, "frontend-internal-api-route")
}

func similarCodeRouteHints(contract APIContractRecord, routes []CodeRouteRecord, limit int) []string {
	type candidate struct {
		label string
		score int
	}
	var candidates []candidate
	for _, route := range routes {
		score := routeSimilarityScore(contract.HTTPMethod, contract.Path, route.HTTPMethod, route.Path)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, candidate{
			label: strings.ToUpper(route.HTTPMethod) + " " + displayRoutePath(route.Path),
			score: score,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].label < candidates[j].label
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	hints := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		hints = append(hints, candidate.label)
	}
	return hints
}

func routeSimilarityScore(apiMethod, apiPath, routeMethod, routePath string) int {
	apiParts := routeParts(apiPath)
	routeParts := routeParts(routePath)
	if len(apiParts) == 0 || len(routeParts) == 0 {
		return 0
	}
	score := 0
	if strings.EqualFold(apiMethod, routeMethod) {
		score += 3
	}
	if strings.EqualFold(apiParts[0], routeParts[0]) {
		score += 4
	}
	common := len(apiParts)
	if len(routeParts) < common {
		common = len(routeParts)
	}
	for i := 1; i < common; i++ {
		switch {
		case isPlaceholder(apiParts[i]) && isPlaceholder(routeParts[i]):
			score += 2
		case strings.EqualFold(apiParts[i], routeParts[i]):
			score += 2
		}
	}
	if absInt(len(apiParts)-len(routeParts)) <= 1 {
		score++
	}
	if score < 5 {
		return 0
	}
	return score
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func indexedBackendRouteGapIssue(contract APIContractRecord, service string) (string, string) {
	if hasDynamicEndpointSegment(contract.Path) {
		return contractIssueDynamicEndpointUnresolved, service + " is indexed, but the frontend path contains a dynamic endpoint segment that cannot be resolved statically"
	}
	return contractIssueIndexedBackendRouteMissing, service + " indexed backend service has no route compatible with this frontend contract"
}

func hasDynamicEndpointSegment(path string) bool {
	parts := routeParts(path)
	if len(parts) == 0 {
		return false
	}
	last := strings.ToLower(parts[len(parts)-1])
	return last == "{endpoint}" || last == "{dynamicendpoint}"
}

func contractIssue(contract APIContractRecord, route CodeRouteRecord, issue, confidence string, score float64, reason string) ContractMatchRecord {
	record := ContractMatchRecord{
		APIHTTPMethod:    contract.HTTPMethod,
		APIPath:          contract.Path,
		APIRawPath:       contract.RawPath,
		APIFile:          contract.File,
		APILine:          contract.Line,
		APIApp:           contract.App,
		ServiceCandidate: contract.ServiceCandidate,
		Issue:            issue,
		Confidence:       confidence,
		ConfidenceScore:  score,
		Reason:           reason,
	}
	if route.Path != "" {
		record.BackendHTTPMethod = route.HTTPMethod
		record.BackendPath = displayRoutePath(route.Path)
		record.BackendHandler = route.Handler
		record.BackendFile = route.File
		record.BackendLine = route.Line
	}
	return record
}

func renderContractMatchesReport(records []ContractMatchRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Contract Matches\n\n")
	if len(records) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, record := range records {
		if record.Issue == contractIssueMatched {
			b.WriteString(fmt.Sprintf("- %s `%s` -> %s `%s` via `%s:%d` (%s)\n",
				record.APIHTTPMethod,
				record.APIPath,
				record.BackendHTTPMethod,
				record.BackendPath,
				record.BackendFile,
				record.BackendLine,
				record.Confidence,
			))
			continue
		}
		target := "no backend route"
		if record.BackendPath != "" {
			target = fmt.Sprintf("%s `%s`", record.BackendHTTPMethod, record.BackendPath)
		}
		b.WriteString(fmt.Sprintf("- %s `%s` -> %s: %s (%s, %s)\n",
			record.APIHTTPMethod,
			record.APIPath,
			target,
			record.Issue,
			record.Confidence,
			record.Reason,
		))
	}
	return b.String()
}

func renderPotentiallyBrokenContractsReport(records []ContractMatchRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Potentially Broken Contracts\n\n")
	count := 0
	for _, record := range records {
		if record.Issue == contractIssueMatched {
			continue
		}
		count++
		location := fmt.Sprintf("`%s:%d`", record.APIFile, record.APILine)
		target := "no compatible backend route"
		if record.BackendPath != "" {
			target = fmt.Sprintf("backend has %s `%s`", record.BackendHTTPMethod, record.BackendPath)
		}
		b.WriteString(fmt.Sprintf("- %s `%s` at %s: %s; %s (%s)\n",
			record.APIHTTPMethod,
			record.APIPath,
			location,
			record.Issue,
			target,
			record.Reason,
		))
	}
	if count == 0 {
		b.WriteString("- none detected\n")
	}
	return b.String()
}
