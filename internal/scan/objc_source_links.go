package scan

import "strings"

func objcType(tokens []dartToken) string {
	var names []string
	for _, t := range tokens {
		if t.text == "<" || t.text == "^" {
			return ""
		}
		if t.kind != "identifier" {
			continue
		}
		switch t.text {
		case "const", "volatile", "__kindof", "_Nullable", "_Nonnull", "nullable", "nonnull", "__strong", "__weak":
			continue
		}
		names = append(names, t.text)
	}
	if len(names) != 1 || names[0] == "id" || names[0] == "Class" {
		return ""
	}
	return names[0]
}

func objcVisibleHeaders(s objcSource, files map[string]objcSource) map[string]bool {
	seen := map[string]bool{s.file.Path: true}
	queue := []objcSource{s}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, uri := range current.imports {
			target := nativeRelativeTarget(current.file.Path, uri)
			header, ok := files[target]
			if !ok || !header.valid || seen[target] {
				continue
			}
			seen[target] = true
			queue = append(queue, header)
		}
	}
	return seen
}

func objcLookup(owner, name string, static bool, visible map[string]bool, files map[string]objcSource, methods map[string][]objcMethod, seen map[string]bool) []objcMethod {
	if owner == "" || seen[owner] {
		return nil
	}
	seen[owner] = true
	var candidates []objcMethod
	for _, m := range methods[owner+"."+name] {
		if m.static != static {
			continue
		}
		permitted := visible[m.symbol.File]
		for file := range visible {
			for _, prototype := range files[file].methods {
				if prototype.symbol.Owner == owner && prototype.symbol.Name == name && prototype.static == static {
					permitted = true
				}
			}
		}
		if permitted {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) > 0 {
		return candidates
	}
	parents := map[string]bool{}
	for file := range visible {
		if parent := files[file].supers[owner]; parent != "" {
			parents[parent] = true
		}
	}
	if len(parents) != 1 {
		return nil
	}
	for parent := range parents {
		return objcLookup(parent, name, static, visible, files, methods, seen)
	}
	return nil
}

func objcSuperclass(owner string, visible map[string]bool, files map[string]objcSource) string {
	parents := map[string]bool{}
	for file := range visible {
		if p := files[file].supers[owner]; p != "" {
			parents[p] = true
		}
	}
	if len(parents) != 1 {
		return ""
	}
	for p := range parents {
		return p
	}
	return ""
}

func objcProperty(owner, name string, visible map[string]bool, files map[string]objcSource) string {
	types := map[string]bool{}
	for file := range visible {
		if typ := files[file].properties[owner][name]; typ != "" {
			types[typ] = true
		}
	}
	if len(types) != 1 {
		return ""
	}
	for typ := range types {
		return typ
	}
	return ""
}

func objcMessage(s objcSource, at int, m objcMethod, env map[string]string, visible map[string]bool, files map[string]objcSource, methods map[string][]objcMethod, depth int) (string, bool, string) {
	if depth > 16 {
		return "", false, ""
	}
	close, ok := s.pairs[at]
	if !ok || at+2 >= close {
		return "", false, ""
	}
	receiverAt := at + 1
	receiver := s.tokens[receiverAt]
	selectorAt := receiverAt + 1
	owner, static := "", false
	if receiver.text == "[" {
		typ, class, name := objcMessage(s, receiverAt, m, env, visible, files, methods, depth+1)
		targets := objcLookup(typ, name, class, visible, files, methods, map[string]bool{})
		if len(targets) == 1 {
			owner = targets[0].returnType
			if owner == "instancetype" {
				owner = typ
			}
		}
		selectorAt = s.pairs[receiverAt] + 1
	} else if receiver.kind == "identifier" {
		owner = env[receiver.text]
		if receiver.text == "self" {
			static = m.static
		}
		if receiver.text == "super" {
			owner = objcSuperclass(m.symbol.Owner, visible, files)
			static = m.static
		}
		if _, shadow := env[receiver.text]; !shadow && receiver.text != "super" {
			for file := range visible {
				for _, symbol := range files[file].classes {
					if symbol.Kind == "class" && symbol.Name == receiver.text {
						owner = receiver.text
						static = true
					}
				}
			}
		}
		if selectorAt+1 < close && s.tokens[selectorAt].text == "." {
			owner = objcProperty(owner, s.tokens[selectorAt+1].text, visible, files)
			static = false
			selectorAt += 2
		}
	} else {
		return "", false, ""
	}
	if selectorAt >= close || s.tokens[selectorAt].kind != "identifier" {
		return "", false, ""
	}
	name := ""
	for j := selectorAt; j < close; j++ {
		if s.tokens[j].kind == "identifier" && j+1 < close && s.tokens[j+1].text == ":" {
			name += s.tokens[j].text + ":"
		} else if j == selectorAt && name == "" {
			name = s.tokens[j].text
		}
		if end, ok := s.pairs[j]; ok && end > j {
			j = end
		}
	}
	return owner, static, name
}

func objcLocalBinding(s objcSource, at, end int, env map[string]string) {
	t := s.tokens
	if t[at].kind != "identifier" || at > 0 && t[at-1].text != "{" && t[at-1].text != ";" && t[at-1].text != "}" {
		return
	}
	j := at + 1
	for j < end && (t[j].text == "*" || strings.HasPrefix(t[j].text, "_Null") || t[j].text == "const") {
		j++
	}
	if j+1 >= end || t[j].kind != "identifier" || t[j+1].text != "=" && t[j+1].text != ";" {
		return
	}
	env[t[j].text] = objcType(t[at:j])
}

func objcBlockEnd(s objcSource, at, end int) int {
	for j := at + 1; j < end; j++ {
		if s.tokens[j].text == ";" || s.tokens[j].text == "]" {
			return at
		}
		if s.tokens[j].text == "{" {
			if close, ok := s.pairs[j]; ok {
				return close
			}
			return at
		}
		if close, ok := s.pairs[j]; ok && close > j {
			j = close
		}
	}
	return at
}
