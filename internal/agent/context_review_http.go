package agent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func reviewHTTPTopic(topic string) bool {
	tokens := contextTokenSet(topic)
	if tokens["http"] || tokens["middleware"] {
		return true
	}
	for word := range tokens {
		if word == "route" || word == "routes" || word == "endpoint" || word == "endpoints" ||
			strings.HasPrefix(word, "routen") || strings.HasPrefix(word, "endpunkt") {
			return true
		}
	}
	return false
}

// reviewHTTPSections follows indexed Go routes to their current registrations and
// the package-level handlers or wrappers named directly by those registrations.
func reviewHTTPSections(loaded loadedContextIndex, topic string) []ContextSourceSection {
	var routes []scan.AgentContextFactRecord
	for _, fact := range loaded.Index.Facts {
		if fact.Kind == "route" && strings.EqualFold(filepath.Ext(fact.File), ".go") && fact.Line > 0 {
			routes = append(routes, fact)
		}
	}
	words := contextTokenSet(topic)
	score := func(route scan.AgentContextFactRecord) int {
		points := 0
		text := strings.ToLower(route.Path + " " + route.Name)
		for word := range words {
			if len(word) >= 4 && strings.Contains(text, word) {
				points += 4
			}
		}
		switch strings.ToUpper(route.HTTPMethod) {
		case "POST", "PUT", "PATCH", "DELETE":
			points += 2
		}
		for _, action := range []string{"delete", "apply", "write", "checkout", "update", "remove"} {
			if strings.Contains(text, action) {
				points++
			}
		}
		return points
	}
	sort.SliceStable(routes, func(i, j int) bool {
		return score(routes[i]) > score(routes[j])
	})
	parsed := map[string]*ast.File{}
	files := map[string]sourceFile{}
	positions := token.NewFileSet()
	for _, route := range routes {
		candidate := sourceCandidate{Project: route.Project, Path: route.File}
		key := auditSourceKey(route.Project, route.File)
		if _, seen := parsed[key]; !seen {
			if len(parsed) >= 8 {
				continue
			}
			parsed[key] = nil
			sourcePath, err := resolveSourcePath(loaded, candidate)
			if err != nil {
				continue
			}
			file, err := readSourceFile(sourcePath)
			hashKey := contextPackSourceFile(route.File)
			if loaded.Workspace {
				hashKey = path.Join(filepath.ToSlash(route.Project), hashKey)
			}
			if err != nil || loaded.Index.SourceHashes[hashKey] != file.Hash {
				continue
			}
			tree, err := parser.ParseFile(positions, sourcePath, strings.Join(file.Lines, "\n"), 0)
			if err != nil {
				continue
			}
			parsed[key], files[key] = tree, file
		}
		tree := parsed[key]
		if tree == nil {
			continue
		}
		var registration *ast.CallExpr
		ast.Inspect(tree, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || positions.Position(call.Pos()).Line != route.Line {
				return registration == nil
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if ok && (selector.Sel.Name == "HandleFunc" || selector.Sel.Name == "Handle") {
				registration = call
			}
			return registration == nil
		})
		if registration == nil {
			continue
		}
		file := files[key]
		section := func(start, end int, role string) ContextSourceSection {
			value := ContextSourceSection{Project: route.Project, Path: route.File, StartLine: start, EndLine: end,
				Role: role, RenderMode: "focused", SourceState: "indexed_range_current",
				Content: renderNumberedSource(file.Lines, start, end)}
			attachSourceReadReceipt(&value, file)
			return value
		}
		end := positions.Position(registration.End()).Line
		if end-route.Line > 12 {
			continue
		}
		result := []ContextSourceSection{section(route.Line, end, "route_registration")}
		// A selector may refer to an imported handler. Only follow names bound to
		// declarations in this parsed file; the registration itself stays visible.
		arg := registration.Args[1]
		for len(result) < 3 {
			expr := arg
			if call, ok := arg.(*ast.CallExpr); ok {
				expr = call.Fun
				arg = nil
				if len(call.Args) == 1 {
					arg = call.Args[0]
				}
			} else {
				arg = nil
			}
			if id, ok := expr.(*ast.Ident); ok && id.Obj != nil {
				if decl, ok := id.Obj.Decl.(*ast.FuncDecl); ok {
					start := positions.Position(decl.Pos()).Line
					last := min(positions.Position(decl.End()).Line, start+9)
					result = append(result, section(start, last, "route_callable"))
				}
			}
			if arg == nil {
				break
			}
		}
		return result
	}
	return nil
}
