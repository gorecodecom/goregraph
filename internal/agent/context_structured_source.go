package agent

import (
	"fmt"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func renderStructuredSourceCandidate(candidate sourceCandidate, file sourceFile, mode string, anchors []scan.SourceDeclarationAnchor) (ContextSourceSection, error) {
	identifier := contextIdentifier(candidate)
	var matches, exact []scan.SourceDeclarationAnchor
	for _, anchor := range anchors {
		name := anchor.Name
		if dot := strings.LastIndex(name, "."); dot >= 0 {
			name = name[dot+1:]
		}
		if name != identifier && anchor.Name != candidate.Name {
			continue
		}
		matches = append(matches, anchor)
		if candidate.StartLine >= anchor.Line && candidate.StartLine <= anchor.EndLine {
			exact = append(exact, anchor)
		}
	}
	var anchor scan.SourceDeclarationAnchor
	state := "indexed_range_current"
	switch {
	case len(exact) == 1:
		anchor = exact[0]
	case len(matches) == 1:
		anchor = matches[0]
		state = "relocated_current"
	default:
		return ContextSourceSection{}, fmt.Errorf("indexed symbol has no unique declaration-like occurrence")
	}
	start, end := anchor.Line, anchor.EndLine
	switch mode {
	case "body", "focused_body", "declaration_body":
		if mode == "body" && candidate.EndLine > 0 {
			start = minimumPositiveContextLine(start, clampSourceLine(candidate.StartLine, len(file.Lines)))
			end = max(end, clampSourceLine(candidate.EndLine, len(file.Lines)))
		}
		if end-start+1 > 120 {
			return ContextSourceSection{}, fmt.Errorf("source declaration body exceeds 120 lines")
		}
	case "focused":
		start = clampSourceLine(start-28, len(file.Lines))
		end = clampSourceLine(anchor.Line+32, len(file.Lines))
	case "signature":
		end = start
	default:
		return ContextSourceSection{}, fmt.Errorf("unsupported source render mode %q", mode)
	}
	return ContextSourceSection{Project: candidate.Project, Path: candidate.Path, StartLine: start, EndLine: end, Role: candidate.Role, RenderMode: mode, SourceState: state, Content: renderNumberedSource(file.Lines, start, end)}, nil
}
