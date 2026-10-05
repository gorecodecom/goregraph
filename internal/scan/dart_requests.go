package scan

import "strings"

type dartRequestDescriptor struct{ method, path, reason string }
type dartRequestState struct {
	urls     map[string]string
	requests map[string]dartRequestDescriptor
}

func (state dartRequestState) clone() dartRequestState {
	copy := dartRequestState{urls: map[string]string{}, requests: map[string]dartRequestDescriptor{}}
	for k, v := range state.urls {
		copy.urls[k] = v
	}
	for k, v := range state.requests {
		copy.requests[k] = v
	}
	return copy
}
func (p *dartProject) trackRequestAssignment(s dartSource, at int, env map[string]string, state *dartRequestState) {
	t := s.tokens
	if at+2 >= len(t) || t[at].kind != "identifier" || t[at+1].text != "=" {
		return
	}
	name := t[at].text
	delete(state.urls, name)
	delete(state.requests, name)
	end := at + 2
	for end < len(t) && t[end].text != ";" {
		if close, ok := s.pairs[end]; ok && close > end {
			end = close + 1
			continue
		}
		end++
	}
	expression := t[at+2 : end]
	if value, known, _ := p.uriPath(s, expression, env, state.urls); known {
		state.urls[name] = value
		env[name] = "Uri"
		return
	}
	open := at + 2
	for open < end && t[open].text != "(" {
		open++
	}
	close, ok := s.pairs[open]
	if !ok || close > end {
		return
	}
	callee := dartJoined(t[at+2 : open])
	callee = strings.TrimPrefix(callee, "new")
	callee = strings.TrimPrefix(callee, "const")
	local := callee
	if dot := strings.LastIndex(local, "."); dot >= 0 {
		local = local[dot+1:]
	}
	if local != "Request" && local != "AbortableRequest" {
		return
	}
	if !p.externalImport(s, "package:http/http.dart", callee, env) {
		return
	}
	args := dartSplit(t[open+1 : close])
	if len(args) < 2 || len(args[0]) != 1 {
		return
	}
	method, literal := dartLiteral(args[0][0])
	method = strings.ToUpper(method)
	if !literal || !isHTTPMethod(method) && method != "HEAD" {
		return
	}
	route, known, reason := p.uriPath(s, args[1], env, state.urls)
	if !known {
		return
	}
	state.requests[name] = dartRequestDescriptor{method: method, path: route, reason: "; literal " + local + " descriptor" + reason}
}
