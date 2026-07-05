package scan

import (
	"fmt"
	"sort"
	"strings"
)

const (
	contractIssueMatched        = "matched"
	contractIssueMethodMismatch = "method_mismatch"
	contractIssueMissingRoute   = "missing_backend_route"
	contractIssueUnsafeDynamic  = "unsafe_dynamic"
)

func buildContractMatches(contracts []APIContractRecord, routes []CodeRouteRecord) []ContractMatchRecord {
	backendRoutes := backendCodeRoutes(routes)
	records := make([]ContractMatchRecord, 0, len(contracts))
	for _, contract := range contracts {
		if contract.UnsafeDynamic {
			records = append(records, contractIssue(contract, CodeRouteRecord{}, contractIssueUnsafeDynamic, "WEAK_MATCH", 0.35, "api path contains complex dynamic expression"))
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

func exactContractRoute(contract APIContractRecord, routes []CodeRouteRecord) (CodeRouteRecord, bool) {
	for _, route := range routes {
		if strings.EqualFold(contract.HTTPMethod, route.HTTPMethod) && pathsCompatible(contract.Path, route.Path) {
			return route, true
		}
	}
	return CodeRouteRecord{}, false
}

func pathCompatibleContractRoute(contract APIContractRecord, routes []CodeRouteRecord) (CodeRouteRecord, bool) {
	for _, route := range routes {
		if pathsCompatible(contract.Path, route.Path) {
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
		if isPlaceholder(leftParts[i]) || isPlaceholder(rightParts[i]) {
			continue
		}
		if !strings.EqualFold(leftParts[i], rightParts[i]) {
			return false
		}
	}
	return true
}

func routeParts(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func isPlaceholder(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
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
		record.BackendPath = route.Path
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
