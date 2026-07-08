package scan

import (
	"fmt"
	"sort"
	"strings"
)

func buildDiagnostics(routes []CodeRouteRecord, matches []ContractMatchRecord, endpointFlows []SpringEndpointFlowRecord, flows []CodeFlowRecord, tests []TestMapRecord) DiagnosticsRecord {
	return DiagnosticsRecord{
		Entrypoints:           diagnosticEntrypoints(routes),
		RiskyContracts:        diagnosticRiskyContracts(matches),
		UnscannedServices:     diagnosticUnscannedServices(matches),
		EndpointsWithoutTests: diagnosticEndpointsWithoutTests(endpointFlows, tests),
		WeakFlows:             diagnosticWeakFlows(flows),
		LikelyTests:           diagnosticLikelyTests(tests),
	}
}

func diagnosticEntrypoints(routes []CodeRouteRecord) []DiagnosticRouteRecord {
	limit := minInt(len(routes), 10)
	result := make([]DiagnosticRouteRecord, 0, limit)
	for _, route := range routes[:limit] {
		result = append(result, DiagnosticRouteRecord{
			HTTPMethod: route.HTTPMethod,
			Path:       route.Path,
			RouteID:    route.RouteID,
			Handler:    route.Handler,
			File:       route.File,
			Line:       route.Line,
			Framework:  route.Framework,
			Confidence: route.Confidence,
			Reason:     route.Reason,
		})
	}
	return result
}

func diagnosticRiskyContracts(matches []ContractMatchRecord) []ContractMatchRecord {
	var result []ContractMatchRecord
	for _, match := range matches {
		if match.Issue == contractIssueMatched {
			continue
		}
		result = append(result, match)
	}
	if len(result) > 10 {
		return result[:10]
	}
	return result
}

func diagnosticUnscannedServices(matches []ContractMatchRecord) []DiagnosticServiceRecord {
	counts := map[string]int{}
	for _, match := range matches {
		if match.Issue == contractIssueUnscanned && match.ServiceCandidate != "" {
			counts[match.ServiceCandidate]++
		}
	}
	var services []string
	for service := range counts {
		services = append(services, service)
	}
	sort.Strings(services)
	result := make([]DiagnosticServiceRecord, 0, len(services))
	for _, service := range services {
		result = append(result, DiagnosticServiceRecord{
			Service:   service,
			Contracts: counts[service],
			Reason:    service + " was referenced by frontend contracts but no matching backend service routes were scanned",
		})
	}
	return result
}

func diagnosticEndpointsWithoutTests(endpointFlows []SpringEndpointFlowRecord, tests []TestMapRecord) []SpringEndpointRecord {
	tested := map[string]bool{}
	for _, test := range tests {
		if test.Type == "endpoint" {
			tested[test.HTTPMethod+" "+test.Path] = true
		}
		if test.TargetClass != "" && test.TargetMethod != "" {
			tested[test.TargetClass+"."+test.TargetMethod] = true
		}
	}

	var result []SpringEndpointRecord
	for _, flow := range endpointFlows {
		if tested[flow.HTTPMethod+" "+flow.Path] || tested[flow.Controller+"."+flow.Method] {
			continue
		}
		result = append(result, SpringEndpointRecord{
			HTTPMethod: flow.HTTPMethod,
			Path:       flow.Path,
			Controller: flow.Controller,
			Method:     flow.Method,
			File:       flow.File,
			Line:       flow.Line,
		})
	}
	if len(result) > 10 {
		return result[:10]
	}
	return result
}

func diagnosticWeakFlows(flows []CodeFlowRecord) []DiagnosticFlowRecord {
	var result []DiagnosticFlowRecord
	for _, flow := range flows {
		confidence, reason, ok := weakFlowReason(flow)
		if !ok {
			continue
		}
		result = append(result, DiagnosticFlowRecord{
			HTTPMethod: flow.HTTPMethod,
			Path:       flow.Path,
			RouteID:    flow.RouteID,
			Handler:    flow.Handler,
			File:       flow.File,
			Line:       flow.Line,
			Confidence: confidence,
			Reason:     reason,
		})
	}
	if len(result) > 10 {
		return result[:10]
	}
	return result
}

func weakFlowReason(flow CodeFlowRecord) (string, string, bool) {
	if len(flow.Steps) == 0 {
		return "WEAK_MATCH", "flow has no resolved handler or call steps", true
	}
	for _, step := range flow.Steps {
		switch step.Confidence {
		case "", "EXTRACTED", "RESOLVED":
			continue
		default:
			reason := step.Reason
			if reason == "" {
				reason = "flow contains inferred step " + step.Name
			}
			return step.Confidence, reason, true
		}
	}
	return "", "", false
}

func diagnosticLikelyTests(tests []TestMapRecord) []TestMapRecord {
	limit := minInt(len(tests), 10)
	return append([]TestMapRecord(nil), tests[:limit]...)
}

func renderDiagnosticsReport(record DiagnosticsRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Diagnostics\n\n")

	b.WriteString("## Top Entry Points\n\n")
	if len(record.Entrypoints) == 0 {
		b.WriteString("- none detected\n")
	} else {
		for _, entry := range record.Entrypoints {
			b.WriteString(fmt.Sprintf("- %s `%s` -> `%s` in `%s:%d` (%s, %s)\n",
				entry.HTTPMethod,
				entry.Path,
				emptyAsNone(entry.Handler),
				entry.File,
				entry.Line,
				entry.Framework,
				emptyAsNone(entry.Confidence),
			))
		}
	}

	b.WriteString("\n## Risky Contracts\n\n")
	if len(record.RiskyContracts) == 0 {
		b.WriteString("- none detected\n")
	} else {
		for _, contract := range record.RiskyContracts {
			b.WriteString(fmt.Sprintf("- %s `%s` from `%s:%d`: %s (%s; %s)\n",
				contract.APIHTTPMethod,
				contract.APIPath,
				contract.APIFile,
				contract.APILine,
				contract.Issue,
				contract.Confidence,
				contract.Reason,
			))
		}
	}

	b.WriteString("\n## Workspace Resolved Contracts\n\n")
	if len(record.WorkspaceResolvedContracts) == 0 {
		b.WriteString("- none detected\n")
	} else {
		for _, match := range record.WorkspaceResolvedContracts {
			b.WriteString(fmt.Sprintf("- %s `%s` frontend `%s` `%s:%d` -> backend `%s` %s %s `%s` via `%s:%d` (%s)\n",
				match.APIHTTPMethod,
				match.APIPath,
				match.APIProject,
				match.APIFile,
				match.APILine,
				emptyAsNone(match.BackendProject),
				emptyAsNone(match.BackendService),
				match.BackendHTTPMethod,
				match.BackendPath,
				match.BackendFile,
				match.BackendLine,
				match.Confidence,
			))
		}
	}

	b.WriteString("\n## Unscanned Services\n\n")
	if len(record.UnscannedServices) == 0 {
		b.WriteString("- none detected\n")
	} else {
		for _, service := range record.UnscannedServices {
			b.WriteString(fmt.Sprintf("- `%s` - %d contract(s); %s\n", service.Service, service.Contracts, service.Reason))
		}
	}

	b.WriteString("\n## Endpoints Without Tests\n\n")
	if len(record.EndpointsWithoutTests) == 0 {
		b.WriteString("- none detected\n")
	} else {
		for _, endpoint := range record.EndpointsWithoutTests {
			b.WriteString(fmt.Sprintf("- %s `%s` -> `%s.%s` in `%s:%d`\n",
				endpoint.HTTPMethod,
				endpoint.Path,
				endpoint.Controller,
				endpoint.Method,
				endpoint.File,
				endpoint.Line,
			))
		}
	}

	b.WriteString("\n## Weak Flows\n\n")
	if len(record.WeakFlows) == 0 {
		b.WriteString("- none detected\n")
	} else {
		for _, flow := range record.WeakFlows {
			b.WriteString(fmt.Sprintf("- %s `%s` route `%s` in `%s:%d`: %s (%s)\n",
				flow.HTTPMethod,
				flow.Path,
				emptyAsNone(flow.RouteID),
				flow.File,
				flow.Line,
				flow.Confidence,
				flow.Reason,
			))
		}
	}

	b.WriteString("\n## Likely Tests\n\n")
	if len(record.LikelyTests) == 0 {
		b.WriteString("- none detected\n")
	} else {
		for _, test := range record.LikelyTests {
			b.WriteString(fmt.Sprintf("- `%s` checks `%s` from `%s` (%s)\n",
				test.TestMethod,
				qualifiedName(test.TargetClass, test.TargetMethod),
				test.TestFile,
				test.Confidence,
			))
		}
	}

	return b.String()
}
