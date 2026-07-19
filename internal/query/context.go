package query

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

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
	if freshness := contextInline(pack.Freshness); freshness != "" {
		lines = append(lines, "Freshness: "+freshness)
	}
	lines = append(lines,
		"Confidence: "+contextInline(pack.Confidence),
		fmt.Sprintf("Budget tokens: %d / %d", pack.EstimatedTokens, pack.BudgetTokens),
		"Fallback required: "+contextYesNo(pack.FallbackRequired),
		"Source coverage: "+contextSourceCoverage(pack.SourceCoverage),
		fmt.Sprintf("Source unrepresented: %d", pack.SourceUnrepresented),
	)

	lines = appendContextLocationSection(lines, "Entrypoints", pack.Entrypoints)
	if len(pack.CallChain) > 0 {
		entries := make([]string, 0, len(pack.CallChain))
		for _, relationship := range pack.CallChain {
			if !contextRelationshipHasContent(relationship) {
				continue
			}
			entry := fmt.Sprintf("- %s --%s--> %s",
				contextInline(relationship.From),
				contextInline(relationship.Kind),
				contextInline(relationship.To),
			)
			entry = appendContextDetails(entry, relationship.Confidence, relationship.Reason, nil)
			entries = append(entries, entry)
		}
		if len(entries) > 0 {
			lines = append(lines, "", "## Call chain")
			lines = append(lines, entries...)
		}
	}
	lines = appendContextLocationSection(lines, "Contracts", pack.Contracts)
	lines = appendContextLocationSection(lines, "Persistence", pack.Persistence)
	lines = appendContextLocationSection(lines, "Tests", pack.Tests)
	if len(pack.Files) > 0 {
		entries := make([]string, 0, len(pack.Files))
		for _, file := range pack.Files {
			if contextInline(file.Path) == "" {
				continue
			}
			entry := "- " + contextCodeReference(file.Project, file.Path, file.StartLine, file.EndLine)
			entry = appendContextDetails(entry, file.Confidence, file.Reason, nil)
			if role := contextInline(file.Role); role != "" {
				entry += " — " + role
			}
			entries = append(entries, entry)
		}
		if len(entries) > 0 {
			lines = append(lines, "", "## Files to inspect")
			lines = append(lines, entries...)
		}
	}
	lines = appendContextSourceSections(lines, pack.SourceSections)
	lines = appendContextSourceOmissions(lines, pack.SourceOmissions)
	if len(pack.Uncertainties) > 0 {
		entries := make([]string, 0, len(pack.Uncertainties))
		for _, uncertainty := range pack.Uncertainties {
			scope := contextInline(uncertainty.Scope)
			reason := contextInline(uncertainty.Reason)
			if scope == "" && reason == "" {
				continue
			}
			entry := "- " + scope
			if reason != "" {
				if scope != "" {
					entry += " — "
				}
				entry += reason
			}
			entries = append(entries, entry)
		}
		if len(entries) > 0 {
			lines = append(lines, "", "## Uncertainties")
			lines = append(lines, entries...)
		}
	}
	if reason := contextInline(pack.FallbackReason); reason != "" {
		lines = append(lines, "", "## Fallback", "- "+reason)
	}
	return strings.Join(lines, "\n") + "\n"
}

func appendContextSourceSections(
	lines []string,
	sections []agent.ContextSourceSection,
) []string {
	entries := make([][]string, 0, len(sections))
	for _, section := range sections {
		if contextInline(section.Path) == "" || section.Content == "" {
			continue
		}
		entry := []string{
			fmt.Sprintf("### %d. %s", len(entries)+1, contextCodeReference(
				section.Project,
				section.Path,
				section.StartLine,
				section.EndLine,
			)),
		}
		if role := contextInline(section.Role); role != "" {
			entry = append(entry, "Role: "+role)
		}
		if mode := contextInline(section.RenderMode); mode != "" {
			entry = append(entry, "Render mode: "+mode)
		}
		if state := contextInline(section.SourceState); state != "" {
			entry = append(entry, "Source state: "+state)
		}
		entry = append(entry, "")
		for _, sourceLine := range strings.Split(section.Content, "\n") {
			entry = append(entry, "    "+sourceLine)
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return lines
	}
	lines = append(lines, "", "## Source sections")
	for _, entry := range entries {
		lines = append(lines, "")
		lines = append(lines, entry...)
	}
	return lines
}

func appendContextSourceOmissions(
	lines []string,
	omissions []agent.ContextSourceOmission,
) []string {
	entries := make([]string, 0, len(omissions))
	for _, omission := range omissions {
		if contextInline(omission.Path) == "" {
			continue
		}
		entry := "- " + contextCodeReference(omission.Project, omission.Path, 0, 0)
		if role := contextInline(omission.Role); role != "" {
			entry += " — role: " + role
		}
		if reason := contextInline(omission.Reason); reason != "" {
			entry += " — reason: " + reason
		}
		entries = append(entries, entry)
	}
	if len(entries) > 0 {
		lines = append(lines, "", "## Source omissions")
		lines = append(lines, entries...)
	}
	return lines
}

func appendContextLocationSection(
	lines []string,
	heading string,
	locations []agent.ContextLocation,
) []string {
	if len(locations) == 0 {
		return lines
	}
	entries := make([]string, 0, len(locations))
	for _, location := range locations {
		kind := contextInline(location.Kind)
		label := contextInline(location.Label)
		file := contextInline(location.File)
		if kind == "" && label == "" {
			label = contextInline(location.ID)
		}
		if kind == "" && label == "" && file == "" {
			continue
		}
		entry := "- "
		switch {
		case kind != "" && label != "":
			entry += kind + " " + label
		case kind != "":
			entry += kind
		case label != "":
			entry += label
		default:
			entry += "context"
		}
		if file != "" {
			entry += " — " + contextCodeReference(
				location.Project,
				file,
				location.Line,
				location.EndLine,
			)
		}
		entry = appendContextDetails(entry, location.Confidence, location.Reason, location.EvidenceIDs)
		entries = append(entries, entry)
	}
	if len(entries) > 0 {
		lines = append(lines, "", "## "+heading)
		lines = append(lines, entries...)
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
	if values := contextEvidenceValues(evidenceIDs); len(values) > 0 {
		entry += " — evidence: " + strings.Join(values, ", ")
	}
	return entry
}

func contextEvidenceValues(evidenceIDs []string) []string {
	values := make([]string, 0, len(evidenceIDs))
	for _, evidenceID := range evidenceIDs {
		if value := contextInline(evidenceID); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func contextRelationshipHasContent(relationship agent.ContextRelationship) bool {
	return contextInline(relationship.From) != "" &&
		contextInline(relationship.To) != "" &&
		contextInline(relationship.Kind) != ""
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
	value = strings.Map(func(current rune) rune {
		if unicode.IsControl(current) {
			return ' '
		}
		return current
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	return strings.ReplaceAll(value, "`", "'")
}

func contextYesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func contextSourceCoverage(value string) string {
	if value = contextInline(value); value != "" {
		return value
	}
	return "unknown"
}
