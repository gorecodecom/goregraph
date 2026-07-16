package scan

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// ScriptResolutionConfig contains the statically resolvable module settings from tsconfig or jsconfig.
type ScriptResolutionConfig struct {
	BaseURL string              `json:"base_url,omitempty"`
	Paths   map[string][]string `json:"paths,omitempty"`
}

var (
	scriptTypeDeclarationRE = regexp.MustCompile(`(?m)(?:^|;)[ \t]*(export\s+)?(default\s+)?(class|interface|type|enum)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	scriptFunctionRE        = regexp.MustCompile(`(?m)(?:^|;)[ \t]*(export\s+)?(default\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	scriptArrowRE           = regexp.MustCompile(`(?m)(?:^|;)[ \t]*(export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s*)?(?:\([^;=]*\)|[A-Za-z_$][A-Za-z0-9_$]*)\s*=>`)
	scriptImportKeywordRE   = regexp.MustCompile(`\bimport\b`)
	scriptExportKeywordRE   = regexp.MustCompile(`\bexport\b`)
	scriptQuotedModuleRE    = regexp.MustCompile(`(?:from\s*)?["']([^"']+)["']`)
	scriptNamedBindingRE    = regexp.MustCompile(`\{([^}]*)\}`)
	scriptVariableTypeRE    = regexp.MustCompile(`\b(?:const|let|var)\s+[A-Za-z_$][A-Za-z0-9_$]*\s*:\s*([A-Z][A-Za-z0-9_$]*)\b`)
	scriptParameterTypeRE   = regexp.MustCompile(`(?:\(|,)\s*[A-Za-z_$][A-Za-z0-9_$]*\??\s*:\s*([A-Z][A-Za-z0-9_$]*)\b`)
	scriptReturnTypeRE      = regexp.MustCompile(`\)\s*:\s*([A-Z][A-Za-z0-9_$]*)\b`)
	scriptJSXUsageRE        = regexp.MustCompile(`<([A-Z][A-Za-z0-9_$]*)\b`)
	scriptNewUsageRE        = regexp.MustCompile(`\bnew\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	scriptMemberCallRE      = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\.([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	scriptDirectCallRE      = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	scriptComputedCallRE    = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*\[[^\]]+\]\s*\(`)
	scriptVariableBindingRE = regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
)

// ExtractScriptSymbolFacts extracts conservative declaration and module-binding facts.
func ExtractScriptSymbolFacts(file FileRecord, body string) ProjectSymbolFacts {
	masked := maskScriptLexical(body)
	declarations := extractScriptDeclarations(file, masked)
	moduleReferences := extractScriptModuleReferences(file, body, masked)
	spans := scriptDeclarationSpans(file, masked, declarations)
	bindScriptReferenceOwners(moduleReferences, spans)
	facts := ProjectSymbolFacts{Declarations: declarations, References: moduleReferences}
	facts.References = append(facts.References, extractScriptUsageReferences(file, masked, declarations, moduleReferences, spans)...)
	sort.Slice(facts.Declarations, func(i, j int) bool { return facts.Declarations[i].ID < facts.Declarations[j].ID })
	sort.Slice(facts.References, func(i, j int) bool { return facts.References[i].ID < facts.References[j].ID })
	return facts
}

func extractScriptDeclarations(file FileRecord, masked string) []RichSymbolRecord {
	module := scriptModuleIdentity(file.Path)
	var declarations []RichSymbolRecord
	add := func(kind, name, exportName string, offset int) {
		qualifiedName := module + "#" + name
		line := scriptLineAt(masked, offset)
		id := StableWorkspaceSymbolID(kind, "", module, file.Language, qualifiedName, file.Path)
		declarations = append(declarations, RichSymbolRecord{
			ID:             id,
			Name:           name,
			Kind:           kind,
			Language:       file.Language,
			File:           file.Path,
			Line:           line,
			SourceLocation: sourceLocation(line),
			QualifiedName:  qualifiedName,
			Module:         module,
			ExportName:     exportName,
			DeclarationID:  id,
			Analyzer:       "script-source",
			Confidence:     ConfidenceExact,
			Coverage:       CoverageComplete,
			scriptOffset:   offset,
		})
	}
	for _, match := range scriptTypeDeclarationRE.FindAllStringSubmatchIndex(masked, -1) {
		if scriptBraceDepthAt(masked, match[0]) != 0 {
			continue
		}
		kind := masked[match[6]:match[7]]
		name := masked[match[8]:match[9]]
		exportName := ""
		if match[2] >= 0 {
			exportName = name
			if match[4] >= 0 {
				exportName = "default"
			}
		}
		add(kind, name, exportName, match[0])
	}
	for _, match := range scriptFunctionRE.FindAllStringSubmatchIndex(masked, -1) {
		if scriptBraceDepthAt(masked, match[0]) != 0 {
			continue
		}
		name := masked[match[6]:match[7]]
		kind := "function"
		if isLikelyReactComponent(name, file.Path) {
			kind = "component"
		}
		exportName := ""
		if match[2] >= 0 {
			exportName = name
			if match[4] >= 0 {
				exportName = "default"
			}
		}
		add(kind, name, exportName, match[0])
	}
	for _, match := range scriptArrowRE.FindAllStringSubmatchIndex(masked, -1) {
		if scriptBraceDepthAt(masked, match[0]) != 0 {
			continue
		}
		name := masked[match[4]:match[5]]
		kind := "function"
		if isLikelyReactComponent(name, file.Path) {
			kind = "component"
		}
		exportName := ""
		if match[2] >= 0 {
			exportName = name
		}
		add(kind, name, exportName, match[0])
	}
	return declarations
}

func extractScriptModuleReferences(file FileRecord, body, masked string) []RichRelationRecord {
	var references []RichRelationRecord
	for _, location := range scriptImportKeywordRE.FindAllStringIndex(masked, -1) {
		statement, statementMasked := scriptStatementAt(body, masked, location[0])
		line := scriptLineAt(masked, location[0])
		trimmedMasked := strings.TrimSpace(statementMasked)
		if strings.HasPrefix(trimmedMasked, "import(") || strings.HasPrefix(trimmedMasked, "import (") {
			if module, ok := scriptStaticCallModule(statement); ok {
				references = append(references, newScriptReference(file, "imports_module", module, "*", line, "static import()", false))
			} else {
				references = append(references, newScriptReference(file, "imports_module", "", "", line, "computed import() module specifier", true))
			}
			continue
		}
		module := scriptModuleSpecifier(statement)
		if module == "" {
			continue
		}
		prefix := statement
		if index := strings.LastIndex(prefix, module); index >= 0 {
			prefix = prefix[:index]
		}
		trimmed := strings.TrimSpace(strings.TrimPrefix(prefix, "import"))
		if strings.HasPrefix(trimmed, "type ") {
			typeClause := strings.TrimSpace(strings.TrimPrefix(trimmed, "type "))
			if strings.HasPrefix(typeClause, "{") {
				appendScriptNamedReferences(&references, file, "imports_type", module, typeClause, line, false)
			} else if fields := strings.Fields(typeClause); len(fields) > 0 {
				reference := newScriptReference(file, "imports_type", module, "default", line, "default type-only import", false)
				reference.scriptLocalName = strings.TrimSuffix(fields[0], ",")
				references = append(references, reference)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "{") {
			appendScriptNamedReferences(&references, file, "imports_value", module, trimmed, line, false)
			continue
		}
		if strings.HasPrefix(trimmed, "*") {
			reference := newScriptReference(file, "imports_namespace", module, "*", line, "namespace import", false)
			if fields := strings.Fields(trimmed); len(fields) >= 3 {
				reference.scriptLocalName = fields[2]
			}
			references = append(references, reference)
			continue
		}
		if strings.HasPrefix(trimmed, `"`) || strings.HasPrefix(trimmed, `'`) {
			references = append(references, newScriptReference(file, "imports_module", module, "*", line, "side-effect import", false))
			continue
		}
		defaultReference := newScriptReference(file, "imports_value", module, "default", line, "default import", false)
		if fields := strings.Fields(strings.TrimSuffix(trimmed, ",")); len(fields) > 0 {
			defaultReference.scriptLocalName = strings.TrimSuffix(fields[0], ",")
		}
		references = append(references, defaultReference)
		if comma := strings.Index(trimmed, ","); comma >= 0 {
			rest := strings.TrimSpace(trimmed[comma+1:])
			if strings.HasPrefix(rest, "{") {
				appendScriptNamedReferences(&references, file, "imports_value", module, rest, line, false)
			} else if strings.HasPrefix(rest, "*") {
				reference := newScriptReference(file, "imports_namespace", module, "*", line, "namespace import", false)
				if fields := strings.Fields(rest); len(fields) >= 3 && fields[1] == "as" {
					reference.scriptLocalName = fields[2]
				}
				references = append(references, reference)
			}
		}
	}
	for _, location := range scriptExportKeywordRE.FindAllStringIndex(masked, -1) {
		statement, statementMasked := scriptStatementAt(body, masked, location[0])
		line := scriptLineAt(masked, location[0])
		trimmed := strings.TrimSpace(strings.TrimPrefix(statement, "export"))
		if !strings.Contains(statementMasked, " from ") {
			module := scriptModuleIdentity(file.Path)
			switch {
			case strings.HasPrefix(trimmed, "type {"):
				appendScriptNamedReferences(&references, file, "exports_local", module, strings.TrimPrefix(trimmed, "type "), line, true)
			case strings.HasPrefix(trimmed, "{"):
				appendScriptNamedReferences(&references, file, "exports_local", module, trimmed, line, true)
			case strings.HasPrefix(trimmed, "default "):
				fields := strings.Fields(strings.TrimPrefix(trimmed, "default "))
				if len(fields) > 0 {
					localName := strings.TrimSuffix(fields[0], ";")
					if isScriptIdentifier(localName) && localName != "class" && localName != "function" {
						reference := newScriptReference(file, "exports_local", module, localName, line, "default local export", false)
						reference.scriptExportAlias = "default"
						references = append(references, reference)
					}
				}
			}
			continue
		}
		module := scriptModuleSpecifier(statement)
		if module == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "type {"):
			appendScriptNamedReferences(&references, file, "reexports_type", module, strings.TrimPrefix(trimmed, "type "), line, true)
		case strings.HasPrefix(trimmed, "{"):
			appendScriptNamedReferences(&references, file, "reexports_value", module, trimmed, line, true)
		case strings.HasPrefix(trimmed, "*"):
			references = append(references, newScriptReference(file, "reexports_all", module, "*", line, "star re-export", false))
		}
	}
	return dedupeRichRelationFacts(references)
}

func appendScriptNamedReferences(target *[]RichRelationRecord, file FileRecord, kind, module, clause string, line int, reexport bool) {
	match := scriptNamedBindingRE.FindStringSubmatch(clause)
	if len(match) != 2 {
		return
	}
	for _, raw := range strings.Split(match[1], ",") {
		fields := strings.Fields(strings.TrimSpace(raw))
		if len(fields) == 0 {
			continue
		}
		itemKind := kind
		if fields[0] == "type" {
			itemKind = strings.Replace(kind, "value", "type", 1)
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		reference := newScriptReference(file, itemKind, module, fields[0], line, "named module binding", false)
		alias := fields[0]
		if len(fields) >= 3 && fields[1] == "as" {
			alias = fields[2]
		}
		if reexport {
			reference.scriptExportAlias = alias
		} else {
			reference.scriptLocalName = alias
		}
		*target = append(*target, reference)
	}
}

func newScriptReference(file FileRecord, kind, module, exportName string, line int, reason string, unresolved bool) RichRelationRecord {
	resolution := SymbolResolutionUnresolved
	confidence := ConfidenceNormalized
	if unresolved {
		confidence = ConfidenceUnknown
	}
	target := module
	if exportName != "" {
		target += "#" + exportName
	}
	id := StableWorkspaceUsageID("", "", "", SymbolUsageUnresolved, kind, target, file.Path, line)
	return RichRelationRecord{
		ID:                  id,
		From:                file.Path,
		To:                  target,
		Type:                kind,
		Language:            file.Language,
		Analyzer:            "script-source",
		Line:                line,
		SourceLocation:      sourceLocation(line),
		Confidence:          string(confidence),
		ConfidenceScore:     javaFactConfidenceScore(confidence),
		TargetQualifiedName: target,
		TargetModule:        module,
		TargetExport:        exportName,
		Resolution:          resolution,
		Reason:              reason,
		preventExact:        true,
	}
}

func isScriptIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if index == 0 {
			if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char == '_' || char == '$') {
				return false
			}
			continue
		}
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '$') {
			return false
		}
	}
	return true
}

type scriptDeclarationSpan struct {
	start       int
	end         int
	endLine     int
	declaration RichSymbolRecord
}

func scriptDeclarationSpans(file FileRecord, masked string, declarations []RichSymbolRecord) []scriptDeclarationSpan {
	byOffset := map[int]RichSymbolRecord{}
	for _, declaration := range declarations {
		byOffset[declaration.scriptOffset] = declaration
	}
	var spans []scriptDeclarationSpan
	add := func(start, signatureEnd int, multilineBlock bool) {
		declaration := byOffset[start]
		if declaration.ID == "" {
			return
		}
		end := scriptStatementEnd(masked, signatureEnd)
		searchEnd := end
		if multilineBlock {
			searchEnd = len(masked)
		}
		if open := strings.Index(masked[signatureEnd:searchEnd], "{"); open >= 0 {
			open += signatureEnd
			if close := matchingScriptBrace(masked, open); close >= 0 {
				end = close + 1
			}
		}
		spans = append(spans, scriptDeclarationSpan{start: start, end: end, endLine: scriptLineAt(masked, end), declaration: declaration})
	}
	for _, match := range scriptFunctionRE.FindAllStringSubmatchIndex(masked, -1) {
		add(match[0], match[1], true)
	}
	for _, match := range scriptArrowRE.FindAllStringSubmatchIndex(masked, -1) {
		add(match[0], match[1], false)
	}
	for _, match := range scriptTypeDeclarationRE.FindAllStringSubmatchIndex(masked, -1) {
		kind := masked[match[6]:match[7]]
		if kind == "class" {
			add(match[0], match[1], true)
		}
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end > spans[j].end
	})
	return spans
}

func scriptStatementEnd(masked string, start int) int {
	for index := start; index < len(masked); index++ {
		if masked[index] == ';' || masked[index] == '\n' {
			return index + 1
		}
	}
	return len(masked)
}

func matchingScriptBrace(masked string, open int) int {
	depth := 0
	for index := open; index < len(masked); index++ {
		switch masked[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func bindScriptReferenceOwners(references []RichRelationRecord, spans []scriptDeclarationSpan) {
	for index := range references {
		if references[index].Type != "imports_module" || !strings.Contains(references[index].Reason, "import()") {
			continue
		}
		line := references[index].Line
		bestStart := -1
		for _, span := range spans {
			if line >= span.declaration.Line && line <= span.endLine && span.start > bestStart {
				references[index].FromSymbolID = span.declaration.ID
				bestStart = span.start
			}
		}
	}
}

func extractScriptUsageReferences(file FileRecord, masked string, declarations []RichSymbolRecord, imports []RichRelationRecord, spans []scriptDeclarationSpan) []RichRelationRecord {
	bindings := map[string]RichRelationRecord{}
	namespaces := map[string]RichRelationRecord{}
	for _, reference := range imports {
		if reference.scriptLocalName == "" || !strings.HasPrefix(reference.Type, "imports_") {
			continue
		}
		if reference.Type == "imports_namespace" {
			namespaces[reference.scriptLocalName] = reference
		} else {
			bindings[reference.scriptLocalName] = reference
		}
	}
	locals := map[string]RichSymbolRecord{}
	for _, declaration := range declarations {
		locals[declaration.Name] = declaration
	}
	var references []RichRelationRecord
	add := func(kind, name string, offset int, binding RichRelationRecord, reason string) {
		reference := newScriptReference(file, kind, binding.TargetModule, binding.TargetExport, scriptLineAt(masked, offset), reason, false)
		reference.scriptLocalName = name
		if owner := innermostScriptOwner(spans, offset); owner.ID != "" {
			reference.FromSymbolID = owner.ID
		}
		references = append(references, reference)
	}
	addUnresolved := func(kind, name string, offset int, reason string) {
		reference := newScriptReference(file, kind, "", name, scriptLineAt(masked, offset), reason, true)
		if owner := innermostScriptOwner(spans, offset); owner.ID != "" {
			reference.FromSymbolID = owner.ID
		}
		references = append(references, reference)
	}
	for _, usageRE := range []*regexp.Regexp{scriptVariableTypeRE, scriptParameterTypeRE, scriptReturnTypeRE} {
		for _, match := range usageRE.FindAllStringSubmatchIndex(masked, -1) {
			name := masked[match[2]:match[3]]
			if binding, ok := bindings[name]; ok {
				if reason := scriptShadowReason(masked, name, match[0]); reason != "" {
					addUnresolved("type_reference", name, match[0], reason)
				} else {
					add("type_reference", name, match[0], binding, "explicit TypeScript type binding")
				}
			}
		}
	}
	if strings.HasSuffix(strings.ToLower(file.Path), ".tsx") || strings.HasSuffix(strings.ToLower(file.Path), ".jsx") {
		for _, match := range scriptJSXUsageRE.FindAllStringSubmatchIndex(masked, -1) {
			name := masked[match[2]:match[3]]
			if binding, ok := bindings[name]; ok {
				if reason := scriptShadowReason(masked, name, match[0]); reason != "" {
					addUnresolved("renders_component", name, match[0], reason)
				} else {
					add("renders_component", name, match[0], binding, "JSX imported component binding")
				}
			}
		}
	}
	for _, match := range scriptNewUsageRE.FindAllStringSubmatchIndex(masked, -1) {
		name := masked[match[2]:match[3]]
		if binding, ok := bindings[name]; ok {
			if reason := scriptShadowReason(masked, name, match[0]); reason != "" {
				addUnresolved("instantiates", name, match[0], reason)
			} else {
				add("instantiates", name, match[0], binding, "constructor imported binding")
			}
			continue
		}
		reference := newScriptReference(file, "instantiates", "", name, scriptLineAt(masked, match[0]), "constructor has no static module binding", true)
		if owner := innermostScriptOwner(spans, match[0]); owner.ID != "" {
			reference.FromSymbolID = owner.ID
		}
		references = append(references, reference)
	}
	memberMethodOffsets := map[int]bool{}
	for _, match := range scriptMemberCallRE.FindAllStringSubmatchIndex(masked, -1) {
		ownerName := masked[match[2]:match[3]]
		methodName := masked[match[4]:match[5]]
		memberMethodOffsets[match[4]] = true
		if namespace, ok := namespaces[ownerName]; ok {
			namespace.TargetExport = methodName
			if reason := scriptShadowReason(masked, ownerName, match[0]); reason != "" {
				addUnresolved("calls_export", methodName, match[0], reason)
			} else {
				add("calls_export", methodName, match[0], namespace, "namespace imported call binding")
			}
		}
	}
	for _, match := range scriptDirectCallRE.FindAllStringSubmatchIndex(masked, -1) {
		nameOffset := match[2]
		name := masked[match[2]:match[3]]
		if memberMethodOffsets[nameOffset] || isScriptNonCall(masked, match[0], name) {
			continue
		}
		if binding, ok := bindings[name]; ok {
			if reason := scriptShadowReason(masked, name, match[0]); reason != "" {
				addUnresolved("calls_export", name, match[0], reason)
			} else {
				add("calls_export", name, match[0], binding, "direct imported call binding")
			}
			continue
		}
		if local, ok := locals[name]; ok {
			if reason := scriptShadowReason(masked, name, match[0]); reason != "" {
				addUnresolved("calls_local", name, match[0], reason)
			} else {
				binding := RichRelationRecord{TargetModule: local.Module, TargetExport: local.Name}
				add("calls_local", name, match[0], binding, "same-module lexical call")
			}
		}
	}
	for _, match := range scriptComputedCallRE.FindAllStringSubmatchIndex(masked, -1) {
		reference := newScriptReference(file, "calls_export", "", "", scriptLineAt(masked, match[0]), "computed property call", true)
		if owner := innermostScriptOwner(spans, match[0]); owner.ID != "" {
			reference.FromSymbolID = owner.ID
		}
		references = append(references, reference)
	}
	return dedupeRichRelationFacts(references)
}

func scriptBraceDepthAt(masked string, offset int) int {
	depth := 0
	for index := 0; index < offset; index++ {
		switch masked[index] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
}

func scriptShadowReason(masked, name string, usageOffset int) string {
	for _, match := range scriptFunctionRE.FindAllStringSubmatchIndex(masked, -1) {
		open := strings.Index(masked[match[1]:], "{")
		if open < 0 {
			continue
		}
		open += match[1]
		close := matchingScriptBrace(masked, open)
		if close < 0 || usageOffset <= open || usageOffset >= close {
			continue
		}
		paramsStart := strings.Index(masked[match[0]:open], "(")
		paramsEnd := strings.LastIndex(masked[match[0]:open], ")")
		if paramsStart < 0 || paramsEnd <= paramsStart {
			continue
		}
		params := masked[match[0]+paramsStart+1 : match[0]+paramsEnd]
		if scriptParameterNames(params)[name] {
			return "lexically shadowed by function parameter"
		}
	}
	for _, match := range scriptArrowRE.FindAllStringSubmatchIndex(masked, -1) {
		signature := masked[match[0]:match[1]]
		equals := strings.Index(signature, "=")
		arrow := strings.LastIndex(signature, "=>")
		if equals < 0 || arrow <= equals {
			continue
		}
		params := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(signature[equals+1:arrow]), "async"))
		params = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(params, ")"), "("))
		statementEnd := scriptStatementEnd(masked, match[1])
		openRelative := strings.Index(masked[match[1]:statementEnd], "{")
		if openRelative < 0 {
			continue
		}
		open := match[1] + openRelative
		close := matchingScriptBrace(masked, open)
		if close < 0 || usageOffset <= open || usageOffset >= close {
			continue
		}
		if scriptParameterNames(params)[name] {
			return "lexically shadowed by arrow parameter"
		}
	}
	for _, match := range scriptVariableBindingRE.FindAllStringSubmatchIndex(masked, -1) {
		if masked[match[2]:match[3]] != name {
			continue
		}
		start, end := scriptContainingScope(masked, match[0])
		if usageOffset > start && usageOffset < end {
			return "lexically shadowed by local variable"
		}
	}
	for _, match := range scriptFunctionRE.FindAllStringSubmatchIndex(masked, -1) {
		if scriptBraceDepthAt(masked, match[0]) == 0 || masked[match[6]:match[7]] != name {
			continue
		}
		start, end := scriptContainingScope(masked, match[0])
		if usageOffset > start && usageOffset < end {
			return "lexically shadowed by nested function"
		}
	}
	for _, match := range scriptTypeDeclarationRE.FindAllStringSubmatchIndex(masked, -1) {
		if scriptBraceDepthAt(masked, match[0]) == 0 || masked[match[8]:match[9]] != name {
			continue
		}
		start, end := scriptContainingScope(masked, match[0])
		if usageOffset > start && usageOffset < end {
			return "lexically shadowed by nested declaration"
		}
	}
	return ""
}

func scriptParameterNames(params string) map[string]bool {
	names := map[string]bool{}
	for _, raw := range strings.Split(params, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
			continue
		}
		name := raw
		if index := strings.IndexAny(name, "?:="); index >= 0 {
			name = name[:index]
		}
		name = strings.TrimSpace(name)
		if name != "" {
			names[name] = true
		}
	}
	return names
}

func scriptContainingScope(masked string, offset int) (int, int) {
	stack := []int{}
	for index := 0; index < offset; index++ {
		switch masked[index] {
		case '{':
			stack = append(stack, index)
		case '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) == 0 {
		return 0, len(masked)
	}
	open := stack[len(stack)-1]
	close := matchingScriptBrace(masked, open)
	if close < 0 {
		close = len(masked)
	}
	return open, close
}

func innermostScriptOwner(spans []scriptDeclarationSpan, offset int) RichSymbolRecord {
	best := scriptDeclarationSpan{start: -1, end: int(^uint(0) >> 1)}
	for _, span := range spans {
		if offset < span.start || offset >= span.end {
			continue
		}
		if best.start < span.start || (best.start == span.start && span.end < best.end) {
			best = span
		}
	}
	return best.declaration
}

func isScriptNonCall(masked string, offset int, name string) bool {
	if name == "import" || isCodeKeyword(name) {
		return true
	}
	prefixStart := offset - 16
	if prefixStart < 0 {
		prefixStart = 0
	}
	prefix := strings.TrimSpace(masked[prefixStart:offset])
	return strings.HasSuffix(prefix, "function") || strings.HasSuffix(prefix, "new") || strings.HasSuffix(prefix, ".")
}

func scriptModuleIdentity(file string) string {
	extension := path.Ext(file)
	return strings.TrimSuffix(path.Clean(strings.ReplaceAll(file, `\`, "/")), extension)
}

func scriptLineAt(body string, offset int) int {
	return 1 + strings.Count(body[:offset], "\n")
}

func scriptStatementAt(body, masked string, start int) (string, string) {
	end := len(masked)
	depth := 0
	for index := start; index < len(masked); index++ {
		switch masked[index] {
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			if depth > 0 {
				depth--
			}
		}
		if masked[index] == ';' && depth == 0 {
			end = index + 1
			break
		}
		if masked[index] == '\n' && depth == 0 {
			statement := strings.TrimSpace(body[start:index])
			waitingForModule := (strings.HasPrefix(statement, "import ") || strings.HasPrefix(statement, "export *")) &&
				scriptModuleSpecifier(statement) == "" && !strings.HasPrefix(statement, "import(") && !strings.HasPrefix(statement, "import (")
			if waitingForModule {
				continue
			}
			end = index + 1
			break
		}
	}
	return body[start:end], masked[start:end]
}

func scriptModuleSpecifier(statement string) string {
	matches := scriptQuotedModuleRE.FindAllStringSubmatch(statement, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1][1]
}

func scriptStaticCallModule(statement string) (string, bool) {
	open := strings.Index(statement, "(")
	close := strings.LastIndex(statement, ")")
	if open < 0 || close <= open {
		return "", false
	}
	argument := strings.TrimSpace(statement[open+1 : close])
	if len(argument) < 2 || argument[0] != argument[len(argument)-1] || (argument[0] != '\'' && argument[0] != '"') {
		return "", false
	}
	return argument[1 : len(argument)-1], true
}

func maskScriptLexical(body string) string {
	masked := []byte(body)
	for index := 0; index < len(masked); {
		switch {
		case index+1 < len(masked) && masked[index] == '/' && masked[index+1] == '/':
			for index < len(masked) && masked[index] != '\n' {
				masked[index] = ' '
				index++
			}
		case index+1 < len(masked) && masked[index] == '/' && masked[index+1] == '*':
			masked[index], masked[index+1] = ' ', ' '
			index += 2
			for index < len(masked) && !(index+1 < len(masked) && masked[index] == '*' && masked[index+1] == '/') {
				if masked[index] != '\n' {
					masked[index] = ' '
				}
				index++
			}
			if index+1 < len(masked) {
				masked[index], masked[index+1] = ' ', ' '
				index += 2
			}
		case masked[index] == '\'' || masked[index] == '"' || masked[index] == '`':
			quote := masked[index]
			index++
			for index < len(masked) {
				if masked[index] == '\\' {
					masked[index] = ' '
					if index+1 < len(masked) && masked[index+1] != '\n' {
						masked[index+1] = ' '
					}
					index += 2
					continue
				}
				if masked[index] == quote {
					index++
					break
				}
				if masked[index] != '\n' {
					masked[index] = ' '
				}
				index++
			}
		default:
			index++
		}
	}
	return string(masked)
}

// ExtractScriptResolutionConfig parses JSON config with deterministic comment and trailing-comma removal.
func ExtractScriptResolutionConfig(_ string, body string) (ScriptResolutionConfig, bool) {
	cleaned := stripScriptJSONComments(body)
	cleaned = stripScriptJSONTrailingCommas(cleaned)
	var document struct {
		CompilerOptions struct {
			BaseURL string              `json:"baseUrl"`
			Paths   map[string][]string `json:"paths"`
		} `json:"compilerOptions"`
	}
	if err := json.Unmarshal([]byte(cleaned), &document); err != nil {
		return ScriptResolutionConfig{}, false
	}
	config := ScriptResolutionConfig{
		BaseURL: strings.TrimSpace(document.CompilerOptions.BaseURL),
		Paths:   document.CompilerOptions.Paths,
	}
	for key, targets := range config.Paths {
		filtered := targets[:0]
		for _, target := range targets {
			if strings.Count(key, "*") <= 1 && strings.Count(target, "*") <= 1 {
				filtered = append(filtered, target)
			}
		}
		sort.Strings(filtered)
		if len(filtered) == 0 {
			delete(config.Paths, key)
		} else {
			config.Paths[key] = filtered
		}
	}
	return config, true
}

// ResolveScriptSymbolFacts resolves static module-backed bindings to canonical declarations.
func ResolveScriptSymbolFacts(files []FileRecord, packages []NodePackageRecord, configs map[string]ScriptResolutionConfig, facts ProjectSymbolFacts) ProjectSymbolFacts {
	resolver := scriptFactResolver{
		files:        map[string]bool{},
		declarations: map[string]map[string][]RichSymbolRecord{},
		locals:       map[string]map[string][]RichSymbolRecord{},
		packages:     append([]NodePackageRecord(nil), packages...),
		configs:      configs,
		facts:        facts,
	}
	for _, file := range files {
		resolver.files[path.Clean(strings.ReplaceAll(file.Path, `\`, "/"))] = true
	}
	for _, declaration := range facts.Declarations {
		if resolver.locals[declaration.Module] == nil {
			resolver.locals[declaration.Module] = map[string][]RichSymbolRecord{}
		}
		resolver.locals[declaration.Module][declaration.Name] = append(resolver.locals[declaration.Module][declaration.Name], declaration)
		if declaration.ExportName == "" {
			continue
		}
		if resolver.declarations[declaration.Module] == nil {
			resolver.declarations[declaration.Module] = map[string][]RichSymbolRecord{}
		}
		resolver.declarations[declaration.Module][declaration.ExportName] = append(resolver.declarations[declaration.Module][declaration.ExportName], declaration)
	}
	sort.Slice(resolver.packages, func(i, j int) bool { return resolver.packages[i].Path < resolver.packages[j].Path })
	result := ProjectSymbolFacts{Declarations: append([]RichSymbolRecord(nil), facts.Declarations...), References: append([]RichRelationRecord(nil), facts.References...)}
	for index := range result.References {
		reference := &result.References[index]
		if reference.Type == "imports_module" {
			resolver.resolveModuleReference(reference)
			continue
		}
		if reference.TargetModule == "" ||
			reference.TargetExport == "" ||
			reference.Type == "reexports_all" ||
			reference.Type == "exports_local" {
			continue
		}
		resolved := resolver.resolveReference(*reference)
		if !resolved.ambiguous && len(resolved.modules) == 1 && len(resolved.candidates) == 1 {
			candidate := resolved.candidates[0]
			reference.To = candidate.QualifiedName
			reference.ToSymbolID = candidate.ID
			reference.TargetQualifiedName = candidate.QualifiedName
			reference.Resolution = SymbolResolutionExact
			reference.Confidence = string(ConfidenceExact)
			reference.ConfidenceScore = 1
			reference.Internal = true
			reference.CandidateSymbolIDs = nil
			reference.Reason = resolved.reason
			reference.preventExact = false
			targetIdentity := candidate.ID + "\x00" + reference.TargetModule + "\x00" + reference.TargetExport
			reference.ID = StableWorkspaceUsageID(candidate.ID, "", reference.FromSymbolID, SymbolUsageDirectReference, reference.Type, targetIdentity, reference.From, reference.Line)
			continue
		}
		if resolved.ambiguous || len(resolved.modules) > 1 || len(resolved.candidates) > 1 {
			reference.Resolution = SymbolResolutionAmbiguous
			reference.Confidence = string(ConfidenceNormalized)
			reference.ConfidenceScore = javaFactConfidenceScore(ConfidenceNormalized)
			reference.ToSymbolID = ""
			reference.CandidateSymbolIDs = reference.CandidateSymbolIDs[:0]
			for _, candidate := range resolved.candidates {
				reference.CandidateSymbolIDs = append(reference.CandidateSymbolIDs, candidate.ID)
			}
			sort.Strings(reference.CandidateSymbolIDs)
			reference.Reason = resolved.reason
			reference.preventExact = true
			reference.ID = StableWorkspaceUsageID("", "", reference.FromSymbolID, SymbolUsageAmbiguous, reference.Type, strings.Join(reference.CandidateSymbolIDs, ","), reference.From, reference.Line)
			continue
		}
		if resolved.reason != "" {
			reference.Reason = resolved.reason
		}
	}
	result.Declarations = dedupeRichSymbolFacts(result.Declarations)
	result.References = dedupeRichRelationFacts(result.References)
	return result
}

func (resolver scriptFactResolver) resolveModuleReference(reference *RichRelationRecord) {
	if reference == nil || reference.TargetModule == "" {
		return
	}
	resolved := resolver.resolveModule(reference.From, reference.TargetModule)
	if resolved.ambiguous || len(resolved.modules) != 1 {
		if resolved.ambiguous || len(resolved.modules) > 1 {
			reference.Resolution = SymbolResolutionAmbiguous
			reference.Reason = "ambiguous static module import"
		} else if resolved.reason != "" {
			reference.Reason = resolved.reason
		}
		return
	}
	module := resolved.modules[0]
	reference.To = module
	reference.TargetQualifiedName = module
	reference.Resolution = SymbolResolutionExact
	reference.Confidence = string(ConfidenceExact)
	reference.ConfidenceScore = 1
	reference.Internal = true
	reference.Reason = resolved.reason
	reference.preventExact = false
	reference.ID = StableWorkspaceUsageID("", "", reference.FromSymbolID, SymbolUsageDirectReference, reference.Type, module, reference.From, reference.Line)
}

type scriptFactResolver struct {
	files        map[string]bool
	declarations map[string]map[string][]RichSymbolRecord
	locals       map[string]map[string][]RichSymbolRecord
	packages     []NodePackageRecord
	configs      map[string]ScriptResolutionConfig
	facts        ProjectSymbolFacts
}

type scriptReferenceResolution struct {
	modules    []string
	candidates []RichSymbolRecord
	reason     string
	ambiguous  bool
}

type scriptModuleResolution struct {
	modules   []string
	reason    string
	ambiguous bool
}

type scriptExportResolution struct {
	candidates []RichSymbolRecord
	cyclic     bool
	ambiguous  bool
}

func (resolver scriptFactResolver) resolveReference(reference RichRelationRecord) scriptReferenceResolution {
	if reference.Type == "calls_local" && reference.TargetModule == scriptModuleIdentity(reference.From) {
		candidates := dedupeScriptDeclarations(resolver.locals[reference.TargetModule][reference.TargetExport])
		if len(candidates) == 1 {
			return scriptReferenceResolution{modules: []string{reference.TargetModule}, candidates: candidates, reason: "same-module lexical declaration"}
		}
		if len(candidates) > 1 {
			return scriptReferenceResolution{modules: []string{reference.TargetModule}, candidates: candidates, reason: "ambiguous same-module declaration"}
		}
		return scriptReferenceResolution{modules: []string{reference.TargetModule}, reason: "same-module declaration not found"}
	}
	moduleResolution := resolver.resolveModule(reference.From, reference.TargetModule)
	if len(moduleResolution.modules) == 0 {
		return scriptReferenceResolution{reason: moduleResolution.reason, ambiguous: moduleResolution.ambiguous}
	}
	var candidates []RichSymbolRecord
	cyclic := false
	ambiguous := moduleResolution.ambiguous
	for _, module := range moduleResolution.modules {
		resolved := resolver.resolveExport(module, reference.TargetExport, map[string]bool{})
		candidates = append(candidates, resolved.candidates...)
		cyclic = cyclic || resolved.cyclic
		ambiguous = ambiguous || resolved.ambiguous
	}
	candidates = dedupeScriptDeclarations(candidates)
	if cyclic && len(candidates) == 0 {
		return scriptReferenceResolution{modules: moduleResolution.modules, reason: "cyclic re-export", ambiguous: ambiguous}
	}
	if ambiguous || len(moduleResolution.modules) > 1 || len(candidates) > 1 {
		return scriptReferenceResolution{modules: moduleResolution.modules, candidates: candidates, reason: "ambiguous static module export", ambiguous: true}
	}
	if len(candidates) == 1 {
		return scriptReferenceResolution{modules: moduleResolution.modules, candidates: candidates, reason: "static module and export binding"}
	}
	return scriptReferenceResolution{modules: moduleResolution.modules, reason: "module resolved but export was not found"}
}

func (resolver scriptFactResolver) resolveExport(module, exportName string, visited map[string]bool) scriptExportResolution {
	key := module + "\x00" + exportName
	if visited[key] {
		return scriptExportResolution{cyclic: true}
	}
	visited[key] = true
	defer delete(visited, key)
	if direct := resolver.declarations[module][exportName]; len(direct) > 0 {
		return scriptExportResolution{candidates: append([]RichSymbolRecord(nil), direct...)}
	}
	var result []RichSymbolRecord
	cyclic := false
	ambiguous := false
	moduleFile := scriptFileForModule(module, resolver.files)
	for _, reference := range resolver.facts.References {
		if reference.From != moduleFile || reference.Type != "exports_local" {
			continue
		}
		publicExport := reference.scriptExportAlias
		if publicExport == "" {
			publicExport = reference.TargetExport
		}
		if publicExport != exportName {
			continue
		}
		localCandidates := resolver.locals[module][reference.TargetExport]
		result = append(result, localCandidates...)
		if len(localCandidates) > 0 {
			continue
		}
		for _, imported := range resolver.facts.References {
			if imported.From != moduleFile ||
				!strings.HasPrefix(imported.Type, "imports_") ||
				imported.scriptLocalName != reference.TargetExport ||
				imported.TargetModule == "" ||
				imported.TargetExport == "" {
				continue
			}
			targetModules := resolver.resolveModule(imported.From, imported.TargetModule)
			ambiguous = ambiguous || targetModules.ambiguous
			for _, targetModule := range targetModules.modules {
				resolved := resolver.resolveExport(targetModule, imported.TargetExport, visited)
				result = append(result, resolved.candidates...)
				cyclic = cyclic || resolved.cyclic
				ambiguous = ambiguous || resolved.ambiguous
			}
		}
	}
	if result = dedupeScriptDeclarations(result); len(result) > 0 {
		return scriptExportResolution{candidates: result, cyclic: cyclic, ambiguous: ambiguous}
	}
	for _, reference := range resolver.facts.References {
		if reference.From != moduleFile || !strings.HasPrefix(reference.Type, "reexports_") {
			continue
		}
		sourceExport := reference.TargetExport
		publicExport := reference.scriptExportAlias
		if reference.Type == "reexports_all" {
			sourceExport = exportName
			publicExport = exportName
		}
		if publicExport == "" {
			publicExport = sourceExport
		}
		if publicExport != exportName {
			continue
		}
		targetModules := resolver.resolveModule(reference.From, reference.TargetModule)
		ambiguous = ambiguous || targetModules.ambiguous
		for _, targetModule := range targetModules.modules {
			resolved := resolver.resolveExport(targetModule, sourceExport, visited)
			result = append(result, resolved.candidates...)
			cyclic = cyclic || resolved.cyclic
			ambiguous = ambiguous || resolved.ambiguous
		}
	}
	return scriptExportResolution{candidates: dedupeScriptDeclarations(result), cyclic: cyclic, ambiguous: ambiguous}
}

func (resolver scriptFactResolver) resolveModule(fromFile, specifier string) scriptModuleResolution {
	if strings.HasPrefix(specifier, ".") {
		modules, reason := resolver.resolveFileModules(path.Join(path.Dir(fromFile), specifier))
		return scriptModuleResolution{modules: modules, reason: reason, ambiguous: len(modules) > 1}
	}
	if modules := resolver.resolveAliasModules(fromFile, specifier); len(modules) > 0 {
		if reason := resolver.aliasDependencyLimitation(fromFile, modules); reason != "" {
			return scriptModuleResolution{reason: reason}
		}
		return scriptModuleResolution{modules: modules, reason: "TypeScript path alias", ambiguous: len(modules) > 1}
	}
	if modules, reason := resolver.resolveWorkspaceModules(fromFile, specifier); len(modules) > 0 || reason != "" {
		return scriptModuleResolution{modules: modules, reason: reason, ambiguous: strings.Contains(reason, "ambiguous")}
	}
	return scriptModuleResolution{reason: "external or unresolved module specifier"}
}

func (resolver scriptFactResolver) aliasDependencyLimitation(fromFile string, modules []string) string {
	consumer, consumerKnown := nearestNodePackage(fromFile, resolver.packages)
	for _, module := range modules {
		provider, providerKnown := nearestNodePackage(module, resolver.packages)
		if !providerKnown || provider.Name == "" {
			continue
		}
		if consumerKnown && consumer.Path == provider.Path {
			continue
		}
		if !consumerKnown || !scriptContainsString(consumer.Dependencies, provider.Name) {
			return "TypeScript path alias crosses workspace package without declared dependency on " + provider.Name
		}
	}
	return ""
}

func (resolver scriptFactResolver) resolveFileModules(base string) ([]string, string) {
	files := scriptFileCandidates(base, resolver.files)
	modules := make([]string, 0, len(files))
	for _, file := range files {
		modules = append(modules, scriptModuleIdentity(file))
	}
	sort.Strings(modules)
	if len(modules) > 1 {
		return modules, "ambiguous module path"
	}
	if len(modules) == 1 {
		return modules, "relative module path"
	}
	return nil, "module file not found"
}

func (resolver scriptFactResolver) resolveAliasModules(fromFile, specifier string) []string {
	configPath, config, ok := nearestScriptConfig(fromFile, resolver.configs)
	if !ok {
		return nil
	}
	var modules []string
	keys := make([]string, 0, len(config.Paths))
	for key := range config.Paths {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, pattern := range keys {
		wildcard, matches := matchScriptAlias(pattern, specifier)
		if !matches {
			continue
		}
		for _, target := range config.Paths[pattern] {
			if strings.Count(target, "*") > 1 {
				continue
			}
			target = strings.Replace(target, "*", wildcard, 1)
			base := path.Join(path.Dir(configPath), config.BaseURL, target)
			for _, file := range scriptFileCandidates(base, resolver.files) {
				modules = append(modules, scriptModuleIdentity(file))
			}
		}
	}
	if len(modules) == 0 && config.BaseURL != "" {
		base := path.Join(path.Dir(configPath), config.BaseURL, specifier)
		for _, file := range scriptFileCandidates(base, resolver.files) {
			modules = append(modules, scriptModuleIdentity(file))
		}
	}
	sort.Strings(modules)
	return modules
}

func (resolver scriptFactResolver) resolveWorkspaceModules(fromFile, specifier string) ([]string, string) {
	provider, subpath, ok := workspacePackageForSpecifier(specifier, resolver.packages)
	if !ok {
		return nil, ""
	}
	consumer, ok := nearestNodePackage(fromFile, resolver.packages)
	if !ok || !scriptContainsString(consumer.Dependencies, provider.Name) {
		return nil, "workspace package is not a declared dependency"
	}
	key := "."
	if subpath != "" {
		key = "./" + subpath
	}
	targets := append([]string(nil), provider.Exports[key]...)
	if len(targets) == 0 && key == "." && provider.Types != "" {
		targets = []string{provider.Types}
	}
	root := path.Dir(provider.Path)
	if len(targets) == 0 && key == "." {
		targets = []string{"src/index", "index"}
	}
	var modules []string
	for _, target := range targets {
		for _, file := range scriptFileCandidates(path.Join(root, strings.TrimPrefix(target, "./")), resolver.files) {
			modules = append(modules, scriptModuleIdentity(file))
		}
	}
	modules = sortedUniqueStrings(modules)
	if len(modules) > 1 || len(targets) > 1 {
		return modules, "ambiguous workspace package export"
	}
	if len(modules) == 1 {
		return modules, "workspace package dependency and static export"
	}
	return nil, "workspace package target was not found"
}

func scriptFileCandidates(base string, files map[string]bool) []string {
	base = path.Clean(strings.ReplaceAll(base, `\`, "/"))
	var candidates []string
	add := func(candidate string) {
		if files[candidate] {
			candidates = append(candidates, candidate)
		}
	}
	add(base)
	if path.Ext(base) == "" {
		for _, extension := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
			add(base + extension)
		}
		for _, extension := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
			add(path.Join(base, "index") + extension)
		}
	}
	return sortedUniqueStrings(candidates)
}

func scriptFileForModule(module string, files map[string]bool) string {
	candidates := scriptFileCandidates(module, files)
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}

func nearestScriptConfig(file string, configs map[string]ScriptResolutionConfig) (string, ScriptResolutionConfig, bool) {
	var candidates []string
	for configPath := range configs {
		dir := path.Dir(configPath)
		if dir == "." || file == dir || strings.HasPrefix(file, dir+"/") {
			candidates = append(candidates, configPath)
		}
	}
	if len(candidates) == 0 {
		return "", ScriptResolutionConfig{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftDir := path.Dir(candidates[i])
		rightDir := path.Dir(candidates[j])
		if len(leftDir) != len(rightDir) {
			return len(leftDir) > len(rightDir)
		}
		leftTS := path.Base(candidates[i]) == "tsconfig.json"
		rightTS := path.Base(candidates[j]) == "tsconfig.json"
		if leftTS != rightTS {
			return leftTS
		}
		return candidates[i] < candidates[j]
	})
	best := candidates[0]
	return best, configs[best], true
}

func matchScriptAlias(pattern, specifier string) (string, bool) {
	if !strings.Contains(pattern, "*") {
		return "", pattern == specifier
	}
	parts := strings.SplitN(pattern, "*", 2)
	if !strings.HasPrefix(specifier, parts[0]) || !strings.HasSuffix(specifier, parts[1]) || len(specifier) < len(parts[0])+len(parts[1]) {
		return "", false
	}
	return specifier[len(parts[0]) : len(specifier)-len(parts[1])], true
}

func workspacePackageForSpecifier(specifier string, packages []NodePackageRecord) (NodePackageRecord, string, bool) {
	best := NodePackageRecord{}
	for _, candidate := range packages {
		if candidate.Name == "" || (specifier != candidate.Name && !strings.HasPrefix(specifier, candidate.Name+"/")) {
			continue
		}
		if len(candidate.Name) > len(best.Name) {
			best = candidate
		}
	}
	if best.Name == "" {
		return NodePackageRecord{}, "", false
	}
	return best, strings.TrimPrefix(strings.TrimPrefix(specifier, best.Name), "/"), true
}

func nearestNodePackage(file string, packages []NodePackageRecord) (NodePackageRecord, bool) {
	best := NodePackageRecord{}
	bestDir := ""
	for _, candidate := range packages {
		dir := path.Dir(candidate.Path)
		if (dir == "." || file == dir || strings.HasPrefix(file, dir+"/")) && len(dir) >= len(bestDir) {
			best, bestDir = candidate, dir
		}
	}
	return best, bestDir != ""
}

func scriptContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sortedUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func dedupeScriptDeclarations(values []RichSymbolRecord) []RichSymbolRecord {
	seen := map[string]RichSymbolRecord{}
	for _, value := range values {
		seen[value.ID] = value
	}
	result := make([]RichSymbolRecord, 0, len(seen))
	for _, value := range seen {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func stripScriptJSONComments(body string) string {
	result := []byte(body)
	inString := false
	escaped := false
	for index := 0; index < len(result); index++ {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if result[index] == '\\' {
				escaped = true
			} else if result[index] == '"' {
				inString = false
			}
			continue
		}
		if result[index] == '"' {
			inString = true
			continue
		}
		if index+1 < len(result) && result[index] == '/' && result[index+1] == '/' {
			for index < len(result) && result[index] != '\n' {
				result[index] = ' '
				index++
			}
			index--
			continue
		}
		if index+1 < len(result) && result[index] == '/' && result[index+1] == '*' {
			result[index], result[index+1] = ' ', ' '
			index += 2
			for index+1 < len(result) && !(result[index] == '*' && result[index+1] == '/') {
				if result[index] != '\n' {
					result[index] = ' '
				}
				index++
			}
			if index+1 < len(result) {
				result[index], result[index+1] = ' ', ' '
				index++
			}
		}
	}
	return string(result)
}

func stripScriptJSONTrailingCommas(body string) string {
	result := []byte(body)
	inString := false
	escaped := false
	for index := 0; index < len(result); index++ {
		if inString {
			if escaped {
				escaped = false
			} else if result[index] == '\\' {
				escaped = true
			} else if result[index] == '"' {
				inString = false
			}
			continue
		}
		if result[index] == '"' {
			inString = true
			continue
		}
		if result[index] != ',' {
			continue
		}
		next := index + 1
		for next < len(result) && (result[next] == ' ' || result[next] == '\t' || result[next] == '\r' || result[next] == '\n') {
			next++
		}
		if next < len(result) && (result[next] == '}' || result[next] == ']') {
			result[index] = ' '
		}
	}
	return string(result)
}
