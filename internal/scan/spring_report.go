package scan

import (
	"fmt"
	"sort"
	"strings"
)

type affectedHit struct {
	label string
	count int
}

func renderEndpointsReport(index SpringIndex) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Endpoints\n\n")
	if len(index.Endpoints) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, endpoint := range index.Endpoints {
		b.WriteString(fmt.Sprintf("- %s `%s` - `%s.%s`", endpoint.HTTPMethod, endpoint.Path, endpoint.Controller, endpoint.Method))
		if endpoint.RequestType != "" {
			b.WriteString(fmt.Sprintf(" - request `%s`", endpoint.RequestType))
		}
		if endpoint.ReturnType != "" {
			b.WriteString(fmt.Sprintf(" - returns `%s`", endpoint.ReturnType))
		}
		b.WriteString(fmt.Sprintf(" - `%s:%d`\n", endpoint.File, endpoint.Line))
	}
	return b.String()
}

func renderDependenciesReport(index SpringIndex) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Dependencies\n\n")
	if len(index.Dependencies) == 0 && len(index.Repositories) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	if len(index.Dependencies) > 0 {
		b.WriteString("## Spring Beans\n\n")
		for _, dependency := range index.Dependencies {
			b.WriteString(fmt.Sprintf("- `%s` -> `%s` (%s", dependency.From, dependency.To, dependency.Injection))
			if dependency.Field != "" {
				b.WriteString(fmt.Sprintf(", field `%s`", dependency.Field))
			}
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}
	if len(index.Repositories) > 0 {
		b.WriteString("## Repositories\n\n")
		for _, repository := range index.Repositories {
			if repository.Entity != "" {
				b.WriteString(fmt.Sprintf("- `%s` -> `%s`", repository.Name, repository.Entity))
				if repository.IDType != "" {
					b.WriteString(fmt.Sprintf(" (id `%s`)", repository.IDType))
				}
				b.WriteString("\n")
			} else {
				b.WriteString(fmt.Sprintf("- `%s`\n", repository.Name))
			}
		}
	}
	return b.String()
}

func renderAffectedReport(graph RichGraph) string {
	var incoming = map[string]int{}
	labels := map[string]string{}
	for _, node := range graph.Nodes {
		labels[node.ID] = node.Label
	}
	for _, edge := range graph.Edges {
		switch edge.Relation {
		case "imports", "imports_internal", "tests", "sources", "includes", "contains":
			incoming[edge.Target]++
		}
	}
	var hits []affectedHit
	for id, count := range incoming {
		if count > 0 {
			hits = append(hits, affectedHit{label: labels[id], count: count})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].count != hits[j].count {
			return hits[i].count > hits[j].count
		}
		return hits[i].label < hits[j].label
	})

	var b strings.Builder
	b.WriteString("# GoreGraph Affected\n\n")
	if len(hits) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	limit := len(hits)
	if limit > 20 {
		limit = 20
	}
	for _, hit := range hits[:limit] {
		b.WriteString(fmt.Sprintf("- `%s` - %d inbound relations\n", hit.label, hit.count))
	}
	return b.String()
}
