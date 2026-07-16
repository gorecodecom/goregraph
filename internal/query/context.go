package query

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agent"
)

type ContextOptions struct {
	Root         string
	Query        string
	Format       string
	BudgetTokens int
	MaxFiles     int
}

func RunContext(options ContextOptions) (string, error) {
	pack, err := agent.BuildContext(agent.ContextRequest{
		Root:         options.Root,
		Query:        options.Query,
		BudgetTokens: options.BudgetTokens,
		MaxFiles:     options.MaxFiles,
	})
	if err != nil {
		return "", err
	}
	if options.Format == "" || options.Format == "markdown" {
		return RenderContextMarkdown(pack), nil
	}
	if options.Format != "json" {
		return "", fmt.Errorf("context format must be json or markdown")
	}
	body, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return "", err
	}
	return string(body) + "\n", nil
}

func RenderContextMarkdown(pack agent.ContextPack) string {
	lines := []string{
		"# GoreGraph Context",
		"",
		"Query: " + contextInline(pack.Query),
	}
	if pack.Freshness != "" {
		lines = append(lines, "Freshness: "+contextInline(pack.Freshness))
	}
	lines = append(lines,
		"Confidence: "+contextInline(pack.Confidence),
		fmt.Sprintf("Budget tokens: %d / %d", pack.EstimatedTokens, pack.BudgetTokens),
		"Fallback required: "+contextYesNo(pack.FallbackRequired),
	)

	lines = appendContextLocationSection(lines, "Entrypoints", pack.Entrypoints)
	if len(pack.CallChain) > 0 {
		lines = append(lines, "", "## Call chain")
		for _, relationship := range pack.CallChain {
			entry := fmt.Sprintf("- %s --%s--> %s",
				contextInline(relationship.From),
				contextInline(relationship.Kind),
				contextInline(relationship.To),
			)
			entry = appendContextDetails(entry, relationship.Confidence, relationship.Reason, nil)
			lines = append(lines, entry)
		}
	}
	lines = appendContextLocationSection(lines, "Contracts", pack.Contracts)
	lines = appendContextLocationSection(lines, "Persistence", pack.Persistence)
	lines = appendContextLocationSection(lines, "Tests", pack.Tests)
	if len(pack.Files) > 0 {
		lines = append(lines, "", "## Files to inspect")
		for _, file := range pack.Files {
			entry := "- " + contextCodeReference(file.Project, file.Path, file.StartLine, file.EndLine)
			entry = appendContextDetails(entry, file.Confidence, file.Reason, nil)
			if role := contextInline(file.Role); role != "" {
				entry += " — " + role
			}
			lines = append(lines, entry)
		}
	}
	if len(pack.Uncertainties) > 0 {
		lines = append(lines, "", "## Uncertainties")
		for _, uncertainty := range pack.Uncertainties {
			entry := "- " + contextInline(uncertainty.Scope)
			if reason := contextInline(uncertainty.Reason); reason != "" {
				entry += " — " + reason
			}
			lines = append(lines, entry)
		}
	}
	if pack.FallbackRequired || pack.FallbackReason != "" {
		lines = append(lines, "", "## Fallback")
		if reason := contextInline(pack.FallbackReason); reason != "" {
			lines = append(lines, "- "+reason)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func appendContextLocationSection(
	lines []string,
	heading string,
	locations []agent.ContextLocation,
) []string {
	if len(locations) == 0 {
		return lines
	}
	lines = append(lines, "", "## "+heading)
	for _, location := range locations {
		entry := "- " + contextInline(location.Kind)
		if label := contextInline(location.Label); label != "" {
			entry += " " + label
		}
		if location.File != "" {
			entry += " — " + contextCodeReference(
				location.Project,
				location.File,
				location.Line,
				location.EndLine,
			)
		}
		entry = appendContextDetails(entry, location.Confidence, location.Reason, location.EvidenceIDs)
		lines = append(lines, entry)
	}
	return lines
}

func appendContextDetails(entry, confidence, reason string, evidenceIDs []string) string {
	if confidence = contextInline(confidence); confidence != "" {
		entry += " — " + confidence
	}
	if reason = contextInline(reason); reason != "" {
		entry += " — " + reason
	}
	if len(evidenceIDs) > 0 {
		values := make([]string, 0, len(evidenceIDs))
		for _, evidenceID := range evidenceIDs {
			if value := contextInline(evidenceID); value != "" {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			entry += " — evidence: " + strings.Join(values, ", ")
		}
	}
	return entry
}

func contextCodeReference(project, path string, startLine, endLine int) string {
	project = contextInline(project)
	path = contextInline(path)
	if project != "" {
		path = project + "/" + path
	}
	switch {
	case startLine <= 0:
	case endLine <= 0 || endLine == startLine:
		path += fmt.Sprintf(":%d", startLine)
	default:
		path += fmt.Sprintf(":%d-%d", startLine, endLine)
	}
	return "`" + path + "`"
}

func contextInline(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	return strings.ReplaceAll(value, "`", "'")
}

func contextYesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
