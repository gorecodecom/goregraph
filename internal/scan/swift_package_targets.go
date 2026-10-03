package scan

import (
	"path"
	"strings"
)

// swiftPackageTargets accepts only a literal Package(..., targets: [...]) initializer.
// It never executes Package.swift; conditional or computed membership stays unknown.
func swiftPackageTargets(file string, body string, sources []swiftSource) ([]swiftTarget, bool) {
	tokens := swiftTokens(body)
	pairs, balanced := csharpPairs(tokens)
	if !balanced {
		return nil, false
	}
	for _, token := range tokens {
		if token.kind == "identifier" && (token.text == "if" || token.text == "switch") {
			return nil, false
		}
	}
	var entries []csharpToken
	for i := 0; i+4 < len(tokens); i++ {
		if tokens[i].text != "let" || tokens[i+1].text != "package" || tokens[i+2].text != "=" || tokens[i+3].text != "Package" || tokens[i+4].text != "(" {
			continue
		}
		end, ok := pairs[i+4]
		if !ok {
			return nil, false
		}
		for _, argument := range csharpSplit(tokens[i+5:end], ",") {
			if len(argument) > 3 && argument[0].text == "targets" && argument[1].text == ":" && argument[2].text == "[" && argument[len(argument)-1].text == "]" {
				entries = argument[3 : len(argument)-1]
			}
		}
	}
	if len(entries) == 0 {
		return nil, false
	}
	base := path.Dir(file)
	var targets []swiftTarget
	dependencyNames := map[string][]string{}
	for _, entry := range csharpSplit(entries, ",") {
		if len(entry) < 5 || entry[0].text != "." || entry[1].text != "target" && entry[1].text != "testTarget" && entry[1].text != "executableTarget" || entry[2].text != "(" || entry[len(entry)-1].text != ")" {
			return nil, false
		}
		name, folder := "", ""
		var selected, excluded, dependencies []string
		selectedSet := false
		seen := map[string]bool{}
		for _, arg := range csharpSplit(entry[3:len(entry)-1], ",") {
			if len(arg) < 3 || arg[1].text != ":" {
				return nil, false
			}
			key := arg[0].text
			if seen[key] {
				return nil, false
			}
			seen[key] = true
			value := arg[2:]
			switch key {
			case "name", "path":
				if len(value) != 1 || value[0].kind != "literal" {
					return nil, false
				}
				if key == "name" {
					name = value[0].text
				} else {
					folder = value[0].text
				}
			case "sources", "exclude", "dependencies":
				if value[0].text != "[" || value[len(value)-1].text != "]" {
					return nil, false
				}
				var values []string
				for _, item := range csharpSplit(value[1:len(value)-1], ",") {
					if len(item) == 1 && item[0].kind == "literal" {
						values = append(values, item[0].text)
					} else if key == "dependencies" && len(item) == 7 && item[0].text == "." && item[1].text == "target" && item[2].text == "(" && item[3].text == "name" && item[4].text == ":" && item[5].kind == "literal" && item[6].text == ")" {
						values = append(values, item[5].text)
					} else if key == "dependencies" && len(item) > 3 && item[0].text == "." && item[1].text == "product" {
						continue
					} else {
						return nil, false
					}
				}
				if key == "sources" {
					selected = values
					selectedSet = true
				} else if key == "exclude" {
					excluded = values
				} else {
					dependencies = values
				}
			case "resources":
				// Resource declarations do not select Swift source membership.
			default:
				return nil, false
			}
		}
		if !swiftLiteralModuleName(name) {
			return nil, false
		}
		if folder == "" {
			folder = "Sources/" + name
			if entry[1].text == "testTarget" {
				folder = "Tests/" + name
			}
		}
		if path.IsAbs(folder) || strings.Contains(folder, "$") {
			return nil, false
		}
		folder = path.Clean(path.Join(base, folder))
		if folder == ".." || strings.HasPrefix(folder, "../") {
			return nil, false
		}
		for _, value := range append(append([]string{}, selected...), excluded...) {
			if path.IsAbs(value) || path.Clean(value) == ".." || strings.HasPrefix(path.Clean(value), "../") || strings.ContainsAny(value, "$\\") {
				return nil, false
			}
		}
		target := swiftTarget{module: path.Join(base, name), files: map[string]bool{}, dependencies: map[string]bool{}}
		for _, source := range sources {
			if source.file == file || !strings.HasPrefix(source.file, folder+"/") {
				continue
			}
			relative := strings.TrimPrefix(source.file, folder+"/")
			contains := func(values []string) bool {
				for _, value := range values {
					value = path.Clean(value)
					if value == "." {
						return true
					}
					if relative == value || strings.HasPrefix(relative, strings.TrimSuffix(value, "/")+"/") {
						return true
					}
				}
				return false
			}
			if contains(excluded) || selectedSet && !contains(selected) {
				continue
			}
			target.files[source.file] = true
		}
		if _, exists := dependencyNames[target.module]; exists {
			return nil, false
		}
		dependencyNames[target.module] = dependencies
		targets = append(targets, target)
	}
	for i := range targets {
		targets[i].dependencies[targets[i].module] = true
		for _, dependency := range dependencyNames[targets[i].module] {
			targets[i].dependencies[path.Join(base, dependency)] = true
		}
	}
	return targets, len(targets) > 0
}
