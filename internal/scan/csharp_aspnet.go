package scan

import "strings"

func csharpBuilderLocal(s csharpSource, start, end int, variables map[string]string, types map[string][]RichSymbolRecord) string {
	if start+3 >= end || s.tokens[start+3].text != "(" {
		return ""
	}
	if s.tokens[start].text == "WebApplication" && s.tokens[start+1].text == "." && s.tokens[start+2].text == "CreateBuilder" && csharpImported(s, "Microsoft.AspNetCore.Builder") && len(csharpFindTypes(s, "", "WebApplication", types)) == 0 {
		return "Microsoft.AspNetCore.Builder.WebApplicationBuilder"
	}
	if variables[s.tokens[start].text] == "Microsoft.AspNetCore.Builder.WebApplicationBuilder" && s.tokens[start+1].text == "." && s.tokens[start+2].text == "Build" {
		return "Microsoft.AspNetCore.Builder.WebApplication"
	}
	return ""
}

func csharpMinimalRoute(s csharpSource, m csharpMember, token csharpToken, receiver string, open, close int, variables map[string]string, types map[string][]RichSymbolRecord, code *CodeIntelligenceRecord) {
	if len(s.limitations) > 0 {
		return
	}
	typ := variables[receiver]
	if typ != "Microsoft.AspNetCore.Builder.WebApplication" && !(typ == "WebApplication" && csharpImported(s, "Microsoft.AspNetCore.Builder") && len(csharpFindTypes(s, m.owner, typ, types)) == 0) {
		return
	}
	methods := map[string]string{"MapGet": "GET", "MapPost": "POST", "MapPut": "PUT", "MapDelete": "DELETE", "MapPatch": "PATCH", "MapHead": "HEAD", "MapOptions": "OPTIONS"}
	method := methods[token.text]
	args := csharpSplit(s.tokens[open+1:close], ",")
	if method == "" || len(args) != 2 || len(args[0]) != 1 || args[0][0].kind != "literal" {
		return
	}
	handler := "<inline>"
	if len(args[1]) == 1 && args[1][0].kind == "identifier" {
		handler = args[1][0].text
	}
	code.Routes = append(code.Routes, CodeRouteRecord{Language: "csharp", Framework: "aspnet-core", FrameworkBound: true, Kind: "backend", HTTPMethod: method, Path: "/" + strings.TrimPrefix(args[0][0].text, "/"), Handler: handler, File: s.file, Line: token.line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "literal minimal API registration on a typed WebApplication; middleware, handler execution and runtime routing are not evaluated"})
}
