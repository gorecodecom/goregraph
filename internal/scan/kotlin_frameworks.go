package scan

import "strings"

func (s kotlinSource) annotationLiteral(at int, name string) (string, bool) {
	for i := at - 1; i >= 0 && s.tokens[at].line-s.tokens[i].line <= 8; i-- {
		if s.tokens[i].text == "}" || s.tokens[i].text == "{" || s.tokens[i].text == ";" {
			break
		}
		if s.tokens[i].text != "@" || i+1 >= at || s.tokens[i+1].text != name {
			continue
		}
		if i+2 >= at || s.tokens[i+2].text != "(" {
			return "", true
		}
		end, ok := s.pairs[i+2]
		if !ok || end >= at {
			return "", false
		}
		args := s.tokens[i+3 : end]
		if len(args) == 0 {
			return "", true
		}
		if len(args) == 1 {
			return dartLiteral(args[0])
		}
		if len(args) == 3 && (args[0].text == "path" || args[0].text == "value") && args[1].text == "=" {
			return dartLiteral(args[2])
		}
		return "", false
	}
	return "", false
}
func kotlinDeclarationAt(s kotlinSource, line int, keyword, name string) int {
	for at, token := range s.tokens {
		if token.text != keyword {
			continue
		}
		if keyword == "class" && at+1 < len(s.tokens) && s.tokens[at+1].text == name && token.line == line {
			return at
		}
		if keyword == "fun" {
			for i := at + 1; i < len(s.tokens); i++ {
				if s.tokens[i].text == "(" {
					if i > at+1 && s.tokens[i-1].text == name && s.tokens[i-1].line == line {
						return at
					}
					break
				}
				if s.tokens[i].text == "{" || s.tokens[i].text == ";" || s.tokens[i].text == "fun" {
					break
				}
			}
		}
	}
	return -1
}
func kotlinRouteEvidence(s kotlinSource, m dartMember, result *supplementaryAnalysis) {
	if !s.valid || m.symbol.Kind == "property" {
		return
	}
	at := kotlinDeclarationAt(s, m.symbol.Line, "fun", m.symbol.Name)
	if at < 0 {
		return
	}
	controller := false
	prefix := ""
	for _, typ := range s.types {
		if typ.Name != m.symbol.Owner {
			continue
		}
		classAt := kotlinDeclarationAt(s, typ.Line, "class", typ.Name)
		if classAt < 0 {
			continue
		}
		for _, annotation := range []string{"RestController", "Controller"} {
			if s.hasAnnotation(classAt, annotation) && s.imported(annotation, []string{"org.springframework.web.bind.annotation.RestController", "org.springframework.stereotype.Controller"}) {
				controller = true
			}
		}
		if s.hasAnnotation(classAt, "RequestMapping") {
			if !s.imported("RequestMapping", []string{"org.springframework.web.bind.annotation.RequestMapping"}) {
				return
			}
			var ok bool
			prefix, ok = s.annotationLiteral(classAt, "RequestMapping")
			if !ok {
				return
			}
		}
	}
	if !controller {
		return
	}
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		annotation := strings.ToUpper(method[:1]) + strings.ToLower(method[1:]) + "Mapping"
		if !s.hasAnnotation(at, annotation) || !s.imported(annotation, []string{"org.springframework.web.bind.annotation." + annotation}) {
			continue
		}
		route, ok := s.annotationLiteral(at, annotation)
		if !ok {
			continue
		}
		route = normalizeCodeRoutePath(strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(route, "/"))
		result.code.Routes = append(result.code.Routes, CodeRouteRecord{Language: "kotlin", Framework: "Spring", FrameworkBound: true, Kind: "backend", App: codeFileApp(s.file.Path), Package: s.packageName, RouteID: codeRouteID(codeFileApp(s.file.Path), route), HTTPMethod: method, Path: route, Handler: m.symbol.Owner + "." + m.symbol.Name, File: s.file.Path, Line: m.symbol.Line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "import-verified literal Spring Kotlin controller mapping; runtime configuration and security are not evaluated"})
	}
}
func kotlinCallEvidence(s kotlinSource, m dartMember, at, close int, receiver, name string, env map[string]string, result *supplementaryAnalysis) {
	if env[receiver] == "HttpClient" && s.imported("HttpClient", []string{"io.ktor.client.HttpClient"}) {
		method := strings.ToUpper(name)
		extensionImported := false
		for _, imp := range s.imports {
			if imp == "io.ktor.client.request."+name || imp == "io.ktor.client.request.*" {
				extensionImported = true
			}
		}
		if isHTTPMethod(method) && extensionImported {
			args := dartSplit(s.tokens[at+1 : close])
			if len(args) > 0 && len(args[0]) == 1 {
				if route, ok := dartLiteral(args[0][0]); ok {
					if safe, ok := dartSafeRequestPath(route); ok {
						result.code.APIContracts = append(result.code.APIContracts, literalHTTPContract(s.file, method, safe, m.symbol.Owner+"."+m.symbol.Name, nil, nil, s.tokens[at].line, "import-verified Ktor request; runtime base URLs, plugins and authentication are not evaluated"))
					}
				}
			}
		}
	}
	if env[receiver] == "MethodChannel" && s.imported("MethodChannel", []string{"io.flutter.plugin.common.MethodChannel"}) && (name == "invokeMethod" || name == "setMethodCallHandler") {
		result.capabilities = append(result.capabilities, ArchitectureCapabilityFact{ID: stableID("kotlin-channel", s.file.Path, sourceLocation(s.tokens[at].line), name), Language: "kotlin", Capability: CapabilityMessaging, Kind: "platform_channel_" + name, Framework: "Flutter MethodChannel", File: s.file.Path, Line: s.tokens[at].line})
	}
}

func kotlinAnnotationStart(s kotlinSource, at int) int {
	start := s.tokens[at].line
	for i := at - 1; i >= 0 && s.tokens[at].line-s.tokens[i].line <= 8; {
		if kotlinModifier(s.tokens[i].text) {
			start = s.tokens[i].line
			i--
			continue
		}
		annotationName := i
		if s.tokens[i].text == ")" {
			open, ok := s.pairs[i]
			if !ok {
				break
			}
			annotationName = open - 1
		}
		if annotationName < 1 || s.tokens[annotationName].kind != "identifier" || s.tokens[annotationName-1].text != "@" {
			break
		}
		start = s.tokens[annotationName-1].line
		i = annotationName - 2
	}
	return start
}
