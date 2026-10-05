package agent

import (
	"fmt"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func renderDartSourceCandidate(candidate sourceCandidate, file sourceFile, mode string) (ContextSourceSection, error) {
	declarations := scan.DartSourceDeclarations(candidate.Path, strings.Join(file.Lines, "\n"))
	identifier := contextIdentifier(candidate)
	var matches, exact []scan.DartSourceDeclaration
	for _, d := range declarations {
		name := d.Name
		if at := strings.LastIndex(name, "."); at >= 0 {
			name = name[at+1:]
		}
		if name != identifier && d.Name != candidate.Name {
			continue
		}
		matches = append(matches, d)
		if candidate.StartLine >= d.Line && candidate.StartLine <= d.EndLine {
			exact = append(exact, d)
		}
	}
	var declaration scan.DartSourceDeclaration
	state := "indexed_range_current"
	switch {
	case len(exact) == 1:
		declaration = exact[0]
	case len(matches) == 1:
		declaration = matches[0]
		state = "relocated_current"
	default:
		return ContextSourceSection{}, fmt.Errorf("Dart declaration is absent, ambiguous, or malformed in current source")
	}
	start, end := declaration.Line, declaration.EndLine
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
		end = clampSourceLine(declaration.Line+32, len(file.Lines))
	case "signature":
		end = start
		if declaration.EndLine > start {
			code := strings.Split(scan.DartSourceCodeMask(strings.Join(file.Lines, "\n")), "\n")
			for end < declaration.EndLine && end-start < 11 {
				if strings.Contains(code[end-1], "{") || strings.Contains(code[end-1], "=>") || strings.HasSuffix(strings.TrimSpace(code[end-1]), ";") {
					break
				}
				end++
			}
		}
	default:
		return ContextSourceSection{}, fmt.Errorf("unsupported source render mode %q", mode)
	}
	return ContextSourceSection{Project: candidate.Project, Path: candidate.Path, StartLine: start, EndLine: end, Role: candidate.Role, RenderMode: mode, SourceState: state, Content: renderNumberedSource(file.Lines, start, end)}, nil
}
