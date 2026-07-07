package scan

import (
	"fmt"
	"sort"
	"strings"
)

func buildFrontendUsage(contracts []APIContractRecord, flows []CodeFlowRecord) []FrontendUsageRecord {
	records := make([]FrontendUsageRecord, 0, len(contracts))
	for _, contract := range contracts {
		record := FrontendUsageRecord{
			App:              contract.App,
			HTTPMethod:       contract.HTTPMethod,
			Path:             contract.Path,
			ServiceCandidate: contract.ServiceCandidate,
			APIFile:          contract.File,
			APILine:          contract.Line,
			APICaller:        contract.Caller,
			RouteConfidence:  "WEAK_MATCH",
			Reason:           "no frontend route flow reached this API contract",
		}
		context := resolveFrontendUsageContext(flows, contract)
		if context.score > 0 {
			record.RouteID = context.routeID
			record.RoutePath = context.routePath
			record.RouteFile = context.routeFile
			record.RouteLine = context.routeLine
			record.Component = context.component
			record.Steps = context.steps
			record.RouteConfidence = context.confidence
			record.Reason = context.reason
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].App != records[j].App {
			return records[i].App < records[j].App
		}
		if records[i].APIFile != records[j].APIFile {
			return records[i].APIFile < records[j].APIFile
		}
		if records[i].APILine != records[j].APILine {
			return records[i].APILine < records[j].APILine
		}
		return records[i].Path < records[j].Path
	})
	return records
}

type frontendUsageContext struct {
	routeID    string
	routePath  string
	routeFile  string
	routeLine  int
	component  string
	steps      []CodeFlowStep
	confidence string
	reason     string
	score      float64
}

func resolveFrontendUsageContext(flows []CodeFlowRecord, contract APIContractRecord) frontendUsageContext {
	var best frontendUsageContext
	for _, flow := range flows {
		if flow.Kind != "frontend" {
			continue
		}
		if contract.App != "" && flow.App != "" && contract.App != flow.App {
			continue
		}
		candidate := scoreFrontendUsageFlow(flow, contract)
		if candidate.score > best.score {
			best = candidate
		}
	}
	return best
}

func scoreFrontendUsageFlow(flow CodeFlowRecord, contract APIContractRecord) frontendUsageContext {
	context := frontendUsageContext{
		routeID:    flow.RouteID,
		routePath:  flow.Path,
		routeFile:  flow.File,
		routeLine:  flow.Line,
		component:  flow.Handler,
		steps:      flow.Steps,
		confidence: "WEAK_MATCH",
		reason:     "frontend route shares app with API contract but no route-flow step reached the API caller",
		score:      0.35,
	}
	for _, step := range flow.Steps {
		if step.Kind == "route_handler" && step.Name != "" {
			context.component = step.Name
		}
		if step.File != contract.File || contract.File == "" {
			continue
		}
		if context.score < 0.82 {
			context.confidence = "RESOLVED"
			context.reason = "route flow reaches API contract file"
			context.score = 0.82
		}
		if contract.Caller != "" && stepMatchesCaller(step, contract.Caller) {
			context.confidence = "RESOLVED"
			context.reason, context.score = frontendCallerResolution(step)
		}
	}
	return context
}

func stepMatchesCaller(step CodeFlowStep, caller string) bool {
	if step.Name == caller {
		return true
	}
	return strings.HasSuffix(step.Name, "."+caller)
}

func renderFrontendUsageReport(records []FrontendUsageRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Frontend Usage\n\n")
	if len(records) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, record := range records {
		b.WriteString(fmt.Sprintf("## %s `%s`\n\n", record.HTTPMethod, record.Path))
		if record.RouteID != "" || record.RoutePath != "" || record.Component != "" {
			b.WriteString(fmt.Sprintf("- Route: `%s` `%s` -> `%s`",
				emptyAsNone(record.RouteID),
				emptyAsNone(record.RoutePath),
				emptyAsNone(record.Component),
			))
			if record.RouteFile != "" {
				b.WriteString(fmt.Sprintf(" in `%s:%d`", record.RouteFile, record.RouteLine))
			}
			b.WriteString("\n")
		} else {
			b.WriteString("- Route: none resolved\n")
		}
		b.WriteString(fmt.Sprintf("- API: `%s:%d` `%s`\n", record.APIFile, record.APILine, emptyAsNone(record.APICaller)))
		if record.ServiceCandidate != "" {
			b.WriteString(fmt.Sprintf("- Service candidate: `%s`\n", record.ServiceCandidate))
		}
		b.WriteString(fmt.Sprintf("- Confidence: %s\n", record.RouteConfidence))
		if record.Reason != "" {
			b.WriteString(fmt.Sprintf("- Evidence: %s\n", record.Reason))
		}
		if len(record.Steps) > 0 {
			b.WriteString("- Chain:\n")
			for _, step := range record.Steps {
				b.WriteString(fmt.Sprintf("  - `%s`", emptyAsNone(step.Name)))
				if step.Kind != "" {
					b.WriteString(fmt.Sprintf(" (%s)", step.Kind))
				}
				if step.File != "" {
					b.WriteString(fmt.Sprintf(" - `%s:%d`", step.File, step.Line))
				}
				if step.Confidence != "" {
					b.WriteString(fmt.Sprintf(" - %s", step.Confidence))
				}
				if step.Reason != "" {
					b.WriteString(fmt.Sprintf(" - %s", step.Reason))
				}
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
