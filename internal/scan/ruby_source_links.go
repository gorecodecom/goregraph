package scan

import "strings"

type rubyImport struct {
	uri  string
	line int
}

func rubyRelativeLiteral(body string) (string, bool) {
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "(") {
		body = strings.TrimSpace(body[1:])
	}
	if len(body) < 2 || body[0] != '\'' && body[0] != '"' {
		return "", false
	}
	close := strings.IndexByte(body[1:], body[0])
	if close < 0 {
		return "", false
	}
	value := body[1 : close+1]
	tail := strings.TrimSpace(strings.SplitN(body[close+2:], "\n", 2)[0])
	if strings.HasPrefix(tail, ")") {
		tail = strings.TrimSpace(tail[1:])
	}
	if tail != "" && !strings.HasPrefix(tail, "#") && tail != ";" {
		return "", false
	}
	if strings.ContainsAny(value, "#{}\\$\x00") {
		return "", false
	}
	return value, true
}

func rubyImportTarget(from, uri string) string {
	if !strings.HasSuffix(uri, ".rb") {
		uri += ".rb"
	}
	return nativeRelativeTarget(from, uri)
}

func rubyVisibleSources(s rubySource, files map[string]rubySource) map[string]bool {
	seen := map[string]bool{s.file.Path: true}
	queue := []rubySource{s}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, imp := range current.imports {
			target := rubyImportTarget(current.file.Path, imp.uri)
			other, ok := files[target]
			if !ok || !other.valid || seen[target] {
				continue
			}
			seen[target] = true
			queue = append(queue, other)
		}
	}
	return seen
}

func rubyKnownOwner(owner string, visible map[string]bool, files map[string]rubySource) bool {
	for file := range visible {
		for _, symbol := range files[file].symbols {
			if symbol.Owner == "" && symbol.Name == owner || symbol.Owner+"::"+symbol.Name == owner {
				return true
			}
		}
	}
	return false
}

func rubyLookup(owner, name string, static bool, visible map[string]bool, files map[string]rubySource, seen map[string]bool) []dartMember {
	if seen[owner] {
		return nil
	}
	seen[owner] = true
	var direct []dartMember
	mixins, parents := map[string]bool{}, map[string]bool{}
	for file := range visible {
		s := files[file]
		if s.unsafeOwners[owner] {
			return nil
		}
		for _, m := range s.members {
			if m.symbol.Owner == owner && m.symbol.Name == name && m.static == static {
				direct = append(direct, m)
			}
		}
		if !static {
			for _, include := range s.mixins[owner] {
				mixins[include] = true
			}
		}
		if p := s.supers[owner]; p != "" {
			parents[p] = true
		}
	}
	if len(direct) > 0 {
		return direct
	}
	// Multiple mixins are kept ambiguous instead of guessing Ruby's runtime lookup chain.
	var inherited []dartMember
	for mixin := range mixins {
		inherited = append(inherited, rubyLookup(mixin, name, false, visible, files, seen)...)
	}
	if len(inherited) > 0 {
		return inherited
	}
	if len(parents) != 1 {
		return nil
	}
	for parent := range parents {
		return rubyLookup(parent, name, static, visible, files, seen)
	}
	return nil
}
