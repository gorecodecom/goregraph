package scan

import (
	"fmt"
	"net/url"
	"strings"
)

func (p *dartProject) frameworkEvidence(result *dartAnalysis, s dartSource, member dartMember, at, close int, receiver, name string, env map[string]string, state dartRequestState) {
	t := s.tokens
	qualified := strings.TrimPrefix(receiver+"."+name, ".")
	add := func(capability CapabilityID, kind, framework string) {
		result.capabilities = append(result.capabilities, ArchitectureCapabilityFact{ID: stableID("dart-architecture", s.file, fmt.Sprint(at), kind), Language: "dart", Capability: capability, Kind: kind, Framework: framework, File: s.file, Line: t[at].line})
	}
	method := strings.ToUpper(strings.TrimSuffix(name, "Uri"))
	if isHTTPMethod(method) || method == "HEAD" || name == "send" {
		clientType := dartBaseType(env[receiver])
		framework := ""
		if p.externalImport(s, "package:http/http.dart", qualified, env) || clientType != "" && p.externalImport(s, "package:http/http.dart", clientType, nil) && strings.HasSuffix(clientType, "Client") {
			framework = "http"
		}
		if clientType != "" && p.externalImport(s, "package:dio/dio.dart", clientType, nil) && strings.HasSuffix(clientType, "Dio") {
			framework = "Dio"
		}
		if framework != "" {
			args := dartSplit(t[at+1 : close])
			requestPath := ""
			known := false
			reason := ""
			if len(args) > 0 {
				if framework == "Dio" && len(args[0]) == 1 {
					requestPath, known = dartLiteral(args[0][0])
				}
				if !known {
					requestPath, known, reason = p.uriPath(s, args[0], env, state.urls)
				}
			}
			if name == "send" {
				known = false
				if len(args) > 0 && len(args[0]) == 1 {
					if request, ok := state.requests[args[0][0].text]; ok {
						method = request.method
						requestPath = request.path
						known = true
						reason = request.reason
					}
				}
			}
			kind := "http_client_dynamic_target"
			if known {
				kind = "http_client"
			}
			add(CapabilityAPIClients, kind, framework)
			if known {
				if safe, ok := dartSafeRequestPath(requestPath); ok {
					contract := literalHTTPContract(FileRecord{Path: s.file, Language: "dart"}, method, safe, strings.TrimPrefix(member.symbol.Owner+"."+member.symbol.Name, "."), nil, nil, t[at].line, "import-verified "+framework+" request; authentication, redirects and runtime base URLs are not evaluated"+reason)
					contract.Package = s.packageName
					result.code.APIContracts = append(result.code.APIContracts, contract)
				}
			}
		}
	}
	// Persistent storage facts describe the selected API, never actual database contents.
	persistence := map[string]struct {
		uri     string
		types   []string
		methods []string
	}{
		"sqflite":            {"package:sqflite/sqflite.dart", []string{"Database", "Transaction", "Batch"}, []string{"query", "rawQuery", "insert", "rawInsert", "update", "rawUpdate", "delete", "rawDelete", "execute", "transaction", "batch"}},
		"shared_preferences": {"package:shared_preferences/shared_preferences.dart", []string{"SharedPreferences", "SharedPreferencesAsync", "SharedPreferencesWithCache"}, []string{"getString", "getBool", "getInt", "getDouble", "getStringList", "setString", "setBool", "setInt", "setDouble", "setStringList", "remove", "clear", "reload"}},
	}
	for framework, pattern := range persistence {
		typeName := dartBaseType(env[receiver])
		validType := false
		for _, typ := range pattern.types {
			local := typeName
			if dot := strings.LastIndex(local, "."); dot >= 0 {
				local = local[dot+1:]
			}
			validType = validType || local == typ && p.externalImport(s, pattern.uri, typeName, nil)
		}
		for _, method := range pattern.methods {
			if name == method && validType {
				add(CapabilityPersistence, "storage_"+name, framework)
			}
		}
		if framework == "sqflite" && (name == "openDatabase" || name == "deleteDatabase") && p.externalImport(s, pattern.uri, qualified, env) {
			add(CapabilityPersistence, "database_"+name, framework)
		}
	}
	for _, pattern := range []struct {
		uri, typ string
		methods  []string
	}{
		{"dart:async", "StreamController", []string{"add", "addError", "listen", "close"}},
		{"package:flutter_bloc/flutter_bloc.dart", "Cubit", []string{"emit"}},
		{"package:bloc/bloc.dart", "Bloc", []string{"add", "emit", "on"}},
	} {
		typeName := dartBaseType(env[receiver])
		if typeName == pattern.typ && p.externalImport(s, pattern.uri, typeName, nil) {
			for _, method := range pattern.methods {
				if name == method {
					add(CapabilityMessaging, "stream_"+name, pattern.typ)
				}
			}
		}
	}
	if receiver == "ref" && (name == "watch" || name == "read" || name == "listen") {
		typ := dartBaseType(env[receiver])
		for _, uri := range []string{"package:flutter_riverpod/flutter_riverpod.dart", "package:riverpod/riverpod.dart"} {
			if (typ == "WidgetRef" || typ == "Ref") && p.externalImport(s, uri, typ, nil) {
				add(CapabilityMessaging, "provider_"+name, "Riverpod")
				args := dartSplit(t[at+1 : close])
				if len(args) > 0 && len(args[0]) == 1 {
					result.facts.References = append(result.facts.References, dartReference(s, member.symbol, args[0][0].text, "uses_provider", t[at].line, p.visible(s, args[0][0].text)))
				}
			}
		}
	}
}

func (p *dartProject) uriPath(s dartSource, tokens []dartToken, env map[string]string, knownURLs ...map[string]string) (string, bool, string) {
	if len(tokens) == 1 && len(knownURLs) > 0 {
		if value, ok := knownURLs[0][tokens[0].text]; ok {
			return value, true, "; literal local Uri assignment"
		}
	}
	// Uri.resolve preserves a literal path even when the host is configured at runtime.
	if len(tokens) >= 6 && tokens[0].kind == "identifier" && tokens[1].text == "." && tokens[2].text == "resolve" && tokens[3].text == "(" && tokens[len(tokens)-1].text == ")" && dartBaseType(env[tokens[0].text]) == "Uri" && len(p.visible(s, "Uri")) == 0 {
		value, ok := dartLiteral(tokens[4])
		if ok && len(tokens) == 6 && strings.HasPrefix(value, "/") {
			return value, true, "; only the literal path is known, the Uri base remains unresolved"
		}
	}
	if len(tokens) < 6 || tokens[0].text != "Uri" || tokens[1].text != "." || tokens[3].text != "(" || tokens[len(tokens)-1].text != ")" || len(p.visible(s, "Uri")) > 0 {
		return "", false, ""
	}
	if _, shadow := env["Uri"]; shadow {
		return "", false, ""
	}
	args := dartSplit(tokens[4 : len(tokens)-1])
	if len(args) == 0 || len(args[0]) != 1 {
		return "", false, ""
	}
	first, ok := dartLiteral(args[0][0])
	if !ok {
		return "", false, ""
	}
	switch tokens[2].text {
	case "parse":
		if len(args) != 1 {
			return "", false, ""
		}
		return first, true, ""
	case "http", "https":
		if len(args) < 2 || len(args[1]) != 1 {
			return "", false, ""
		}
		route, ok := dartLiteral(args[1][0])
		if !ok {
			return "", false, ""
		}
		return tokens[2].text + "://" + first + "/" + strings.TrimPrefix(route, "/"), true, ""
	}
	return "", false, ""
}
func dartSafeRequestPath(value string) (string, bool) {
	if value == "" || strings.ContainsAny(value, "\r\n\x00") {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil {
		return "", false
	}
	if parsed.IsAbs() && parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	if !parsed.IsAbs() && !strings.HasPrefix(value, "/") {
		return "", false
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), true
}

func literalHTTPContract(file FileRecord, method, safeURL, caller string, dynamic, fields []string, line int, reason string) APIContractRecord {
	parsed, err := url.Parse(safeURL)
	route := safeURL
	if err == nil && parsed.IsAbs() {
		route = parsed.EscapedPath()
		if route == "" {
			route = "/"
		}
	}
	contract := apiContract(file, method, route, caller, dynamic, fields, line, reason)
	contract.RawPath = safeURL
	return contract
}
