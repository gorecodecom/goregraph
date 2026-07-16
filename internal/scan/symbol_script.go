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

type scriptSymbolCapability uint8

const (
	scriptTypeCapability scriptSymbolCapability = 1 << iota
	scriptValueCapability
)

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
	scriptVariableBindingRE = regexp.MustCompile(`\b(?:const|let|var)\b`)
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
	for index := range facts.References {
		refreshScriptReferenceID(&facts.References[index])
	}
	facts.References = dedupeRichRelationFacts(facts.References)
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
			ID:               id,
			Name:             name,
			Kind:             kind,
			Language:         file.Language,
			File:             file.Path,
			Line:             line,
			SourceLocation:   sourceLocation(line),
			QualifiedName:    qualifiedName,
			Module:           module,
			ExportName:       exportName,
			DeclarationID:    id,
			Analyzer:         "script-source",
			Confidence:       ConfidenceExact,
			Coverage:         CoverageComplete,
			scriptOffset:     offset,
			scriptCapability: scriptDeclarationCapability(kind),
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

func scriptDeclarationCapability(kind string) scriptSymbolCapability {
	switch kind {
	case "class", "enum":
		return scriptTypeCapability | scriptValueCapability
	case "interface", "type":
		return scriptTypeCapability
	default:
		return scriptValueCapability
	}
}

func extractScriptModuleReferences(file FileRecord, body, masked string) []RichRelationRecord {
	var references []RichRelationRecord
	for _, location := range scriptImportKeywordRE.FindAllStringIndex(masked, -1) {
		if !isStandaloneScriptImport(masked, location[0]) {
			continue
		}
		statement, statementMasked := scriptStatementAt(body, masked, location[0])
		line := scriptLineAt(masked, location[0])
		trimmedMasked := strings.TrimSpace(statementMasked)
		if strings.HasPrefix(trimmedMasked, "import(") || strings.HasPrefix(trimmedMasked, "import (") {
			if module, ok := scriptStaticCallModule(statement); ok {
				reference := newScriptReference(file, "imports_module", module, "*", line, "static import()", false)
				reference.scriptOffset = location[0]
				references = append(references, reference)
			} else {
				reference := newScriptReference(file, "imports_module", "", "", line, "computed import() module specifier", true)
				reference.scriptOffset = location[0]
				references = append(references, reference)
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
				appendScriptNamedReferences(&references, file, "imports_type", module, typeClause, line, false, true)
			} else if fields := strings.Fields(typeClause); len(fields) > 0 {
				reference := newScriptReference(file, "imports_type", module, "default", line, "default type-only import", false)
				reference.scriptLocalName = strings.TrimSuffix(fields[0], ",")
				reference.scriptTypeOnly = true
				references = append(references, reference)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "{") {
			appendScriptNamedReferences(&references, file, "imports_value", module, trimmed, line, false, false)
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
				appendScriptNamedReferences(&references, file, "imports_value", module, rest, line, false, false)
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
				appendScriptNamedReferences(&references, file, "exports_local", module, strings.TrimPrefix(trimmed, "type "), line, true, true)
			case strings.HasPrefix(trimmed, "{"):
				appendScriptNamedReferences(&references, file, "exports_local", module, trimmed, line, true, false)
			case strings.HasPrefix(trimmed, "default "):
				fields := strings.Fields(strings.TrimPrefix(trimmed, "default "))
				if len(fields) > 0 {
					localName := strings.TrimSuffix(fields[0], ";")
					if isScriptIdentifier(localName) && localName != "class" && localName != "function" {
						reference := newScriptReference(file, "exports_local", module, localName, line, "default local export", false)
						reference.scriptExportAlias = "default"
						reference.ExportAlias = "default"
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
			appendScriptNamedReferences(&references, file, "reexports_type", module, strings.TrimPrefix(trimmed, "type "), line, true, true)
		case strings.HasPrefix(trimmed, "{"):
			appendScriptNamedReferences(&references, file, "reexports_value", module, trimmed, line, true, false)
		case strings.HasPrefix(trimmed, "*"):
			references = append(references, newScriptReference(file, "reexports_all", module, "*", line, "star re-export", false))
		}
	}
	return references
}

func isStandaloneScriptImport(masked string, offset int) bool {
	previous := offset - 1
	for previous >= 0 && (masked[previous] == ' ' || masked[previous] == '\t' || masked[previous] == '\r' || masked[previous] == '\n') {
		previous--
	}
	if previous >= 0 && masked[previous] == '.' {
		return false
	}
	open := nextScriptNonSpace(masked, offset+len("import"))
	if open < len(masked) && masked[open] == '.' {
		return false
	}
	if open >= len(masked) || masked[open] != '(' {
		return true
	}
	close := matchingScriptDelimiter(masked, open, '(', ')')
	if close < 0 {
		return true
	}
	next := nextScriptNonSpace(masked, close+1)
	return next >= len(masked) || masked[next] != '{'
}

func appendScriptNamedReferences(target *[]RichRelationRecord, file FileRecord, kind, module, clause string, line int, reexport, typeOnly bool) {
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
		itemTypeOnly := typeOnly
		if fields[0] == "type" {
			itemKind = strings.Replace(kind, "value", "type", 1)
			fields = fields[1:]
			itemTypeOnly = true
		}
		if len(fields) == 0 {
			continue
		}
		reference := newScriptReference(file, itemKind, module, fields[0], line, "named module binding", false)
		reference.scriptTypeOnly = itemTypeOnly
		alias := fields[0]
		if len(fields) >= 3 && fields[1] == "as" {
			alias = fields[2]
		}
		if reexport {
			reference.scriptExportAlias = alias
			reference.ExportAlias = alias
		} else {
			reference.scriptLocalName = alias
			reference.LocalName = alias
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
		scriptOffset:        -1,
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
		spans = append(spans, scriptDeclarationSpan{start: start, end: end, declaration: declaration})
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
		if references[index].Type != "imports_module" ||
			!strings.Contains(references[index].Reason, "import()") ||
			references[index].scriptOffset < 0 {
			continue
		}
		if owner := innermostScriptOwner(spans, references[index].scriptOffset); owner.ID != "" {
			bindScriptReferenceOwner(&references[index], owner.ID)
		}
	}
}

func bindScriptReferenceOwner(reference *RichRelationRecord, ownerID string) {
	if reference == nil || ownerID == "" {
		return
	}
	reference.FromSymbolID = ownerID
	refreshScriptReferenceID(reference)
}

func refreshScriptReferenceID(reference *RichRelationRecord) {
	if reference == nil {
		return
	}
	targetIdentity := scriptReferenceAliasIdentity(*reference, reference.TargetQualifiedName)
	reference.ID = StableWorkspaceUsageID(
		"",
		"",
		reference.FromSymbolID,
		SymbolUsageUnresolved,
		reference.Type,
		targetIdentity,
		reference.From,
		reference.Line,
	)
}

func scriptReferenceAliasIdentity(reference RichRelationRecord, targetIdentity string) string {
	if reference.scriptLocalName != "" {
		targetIdentity += "\x00local\x00" + reference.scriptLocalName
	}
	if reference.scriptExportAlias != "" {
		targetIdentity += "\x00export\x00" + reference.scriptExportAlias
	}
	return targetIdentity
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
		if binding.Type == "imports_namespace" && binding.scriptLocalName != "" {
			reference.scriptLocalName = binding.scriptLocalName
		}
		reference.scriptTypeOnly = binding.scriptTypeOnly
		refreshScriptReferenceID(&reference)
		if owner := innermostScriptOwner(spans, offset); owner.ID != "" {
			bindScriptReferenceOwner(&reference, owner.ID)
		}
		references = append(references, reference)
	}
	addUnresolved := func(kind, name string, offset int, reason string) {
		reference := newScriptReference(file, kind, "", name, scriptLineAt(masked, offset), reason, true)
		if owner := innermostScriptOwner(spans, offset); owner.ID != "" {
			bindScriptReferenceOwner(&reference, owner.ID)
		}
		references = append(references, reference)
	}
	for _, match := range scriptVariableTypeRE.FindAllStringSubmatchIndex(masked, -1) {
		name := masked[match[2]:match[3]]
		if binding, ok := bindings[name]; ok {
			if reason := scriptShadowReason(masked, name, match[0]); reason != "" {
				addUnresolved("type_reference", name, match[0], reason)
			} else {
				add("type_reference", name, match[0], binding, "explicit TypeScript variable type binding")
			}
		}
	}
	for _, match := range scriptParameterTypeRE.FindAllStringSubmatchIndex(masked, -1) {
		if !isProvenScriptParameterType(masked, match[2]) {
			continue
		}
		name := masked[match[2]:match[3]]
		if binding, ok := bindings[name]; ok {
			if reason := scriptShadowReason(masked, name, match[0]); reason != "" {
				addUnresolved("type_reference", name, match[0], reason)
			} else {
				add("type_reference", name, match[0], binding, "explicit TypeScript parameter type binding")
			}
		}
	}
	for _, match := range scriptReturnTypeRE.FindAllStringSubmatchIndex(masked, -1) {
		if !isProvenScriptReturnType(masked, match[0], match[3]) {
			continue
		}
		name := masked[match[2]:match[3]]
		if binding, ok := bindings[name]; ok {
			if reason := scriptShadowReason(masked, name, match[0]); reason != "" {
				addUnresolved("type_reference", name, match[0], reason)
			} else {
				add("type_reference", name, match[0], binding, "explicit TypeScript return type binding")
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
			bindScriptReferenceOwner(&reference, owner.ID)
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
			bindScriptReferenceOwner(&reference, owner.ID)
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
	if scriptParameterScopeShadows(masked, name, usageOffset) {
		return "lexically shadowed by function or method parameter"
	}
	if scriptArrowScopeShadows(masked, name, usageOffset) {
		return "lexically shadowed by arrow parameter"
	}
	if scriptVariableScopeShadows(masked, name, usageOffset) {
		return "lexically shadowed by local variable"
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

func scriptParameterScopeShadows(masked, name string, usageOffset int) bool {
	for open := strings.IndexByte(masked, '('); open >= 0; {
		close := matchingScriptDelimiter(masked, open, '(', ')')
		if close < 0 {
			return false
		}
		next := nextScriptNonSpace(masked, close+1)
		for next < len(masked) && masked[next] == ':' {
			next++
			for next < len(masked) && masked[next] != '{' && masked[next] != '\n' && masked[next] != ';' {
				next++
			}
			next = nextScriptNonSpace(masked, next)
		}
		if next < len(masked) && masked[next] == '{' && isScriptParameterScopePrefix(masked, open) {
			bodyEnd := matchingScriptBrace(masked, next)
			if bodyEnd < 0 {
				bodyEnd = len(masked)
			}
			if usageOffset > next && usageOffset < bodyEnd && scriptParameterBindsName(masked[open+1:close], name) {
				return true
			}
		}
		relative := strings.IndexByte(masked[close+1:], '(')
		if relative < 0 {
			break
		}
		open = close + 1 + relative
	}
	return false
}

func isScriptParameterScopePrefix(masked string, open int) bool {
	index := open - 1
	for index >= 0 && (masked[index] == ' ' || masked[index] == '\t') {
		index--
	}
	end := index + 1
	for index >= 0 && ((masked[index] >= 'A' && masked[index] <= 'Z') ||
		(masked[index] >= 'a' && masked[index] <= 'z') ||
		(masked[index] >= '0' && masked[index] <= '9') ||
		masked[index] == '_' || masked[index] == '$') {
		index--
	}
	prefix := masked[index+1 : end]
	switch prefix {
	case "if", "for", "while", "switch", "with":
		return false
	case "catch":
		return true
	}
	if prefix != "" {
		return true
	}
	before := strings.TrimSpace(masked[maxInt(0, open-32):open])
	return strings.HasSuffix(before, "function")
}

func scriptArrowScopeShadows(masked, name string, usageOffset int) bool {
	for search := 0; search < len(masked); {
		relative := strings.Index(masked[search:], "=>")
		if relative < 0 {
			break
		}
		arrow := search + relative
		paramsStart, paramsEnd, ok := scriptArrowParameterRange(masked, arrow)
		if !ok || !scriptParameterBindsName(masked[paramsStart:paramsEnd], name) {
			search = arrow + 2
			continue
		}
		bodyStart := nextScriptNonSpace(masked, arrow+2)
		if bodyStart >= len(masked) {
			return usageOffset > arrow
		}
		bodyEnd := len(masked)
		if masked[bodyStart] == '{' {
			if close := matchingScriptBrace(masked, bodyStart); close >= 0 {
				bodyEnd = close
			}
		} else {
			bodyEnd = scriptArrowExpressionEnd(masked, bodyStart)
		}
		if usageOffset >= bodyStart && usageOffset < bodyEnd {
			return true
		}
		search = arrow + 2
	}
	return false
}

func scriptArrowParameterRange(masked string, arrow int) (int, int, bool) {
	end := arrow
	for end > 0 && isScriptWhitespace(masked[end-1]) {
		end--
	}
	if end == 0 {
		return 0, 0, false
	}
	for close := end - 1; close >= 0; close-- {
		if masked[close] != ')' {
			continue
		}
		open := matchingScriptDelimiterBackward(masked, close, '(', ')')
		if open < 0 {
			continue
		}
		annotation := strings.TrimSpace(masked[close+1 : end])
		if annotation != "" && !strings.HasPrefix(annotation, ":") {
			close = open
			continue
		}
		if isScriptArrowParameterOpen(masked, open) && isPlausibleScriptParameterList(masked[open+1:close]) {
			return open + 1, close, true
		}
		close = open
	}
	start := end - 1
	for start >= 0 && isScriptIdentifierByte(masked[start]) {
		start--
	}
	return start + 1, end, start+1 < end
}

func isScriptArrowParameterOpen(masked string, open int) bool {
	previous := open - 1
	for previous >= 0 && isScriptWhitespace(masked[previous]) {
		previous--
	}
	if previous < 0 {
		return true
	}
	if isScriptIdentifierByte(masked[previous]) {
		end := previous + 1
		for previous >= 0 && isScriptIdentifierByte(masked[previous]) {
			previous--
		}
		return masked[previous+1:end] == "async"
	}
	return !isScriptIdentifierByte(masked[previous]) &&
		masked[previous] != '.' &&
		masked[previous] != ')' &&
		masked[previous] != ']'
}

func isPlausibleScriptParameterList(params string) bool {
	for _, raw := range splitScriptTopLevel(params, ',') {
		parameter := strings.TrimSpace(raw)
		if parameter == "" {
			continue
		}
		parameter = strings.TrimSpace(strings.TrimPrefix(parameter, "..."))
		if parameter == "" {
			return false
		}
		if parameter[0] == '{' || parameter[0] == '[' {
			continue
		}
		end := 0
		for end < len(parameter) && isScriptIdentifierByte(parameter[end]) {
			end++
		}
		if end == 0 {
			return false
		}
		rest := strings.TrimSpace(parameter[end:])
		if rest != "" && !strings.HasPrefix(rest, "?") && !strings.HasPrefix(rest, ":") && !strings.HasPrefix(rest, "=") {
			return false
		}
	}
	return true
}

func scriptVariableScopeShadows(masked, name string, usageOffset int) bool {
	for _, location := range scriptVariableBindingRE.FindAllStringIndex(masked, -1) {
		statementStart := nextScriptNonSpace(masked, location[1])
		statementEnd := scriptVariableStatementEnd(masked, statementStart)
		if statementStart >= statementEnd {
			continue
		}
		bindsName := false
		for _, declarator := range splitScriptTopLevel(masked[statementStart:statementEnd], ',') {
			pattern := declarator
			if equals := findScriptTopLevel(pattern, '='); equals >= 0 {
				pattern = pattern[:equals]
			}
			if scriptBindingPatternNames(pattern)[name] {
				bindsName = true
				break
			}
		}
		if !bindsName {
			continue
		}
		start, end := scriptContainingScope(masked, location[0])
		if masked[location[0]:location[1]] == "var" {
			start, end = scriptContainingFunctionOrModuleScope(masked, location[0])
		}
		if usageOffset > start && usageOffset < end {
			return true
		}
	}
	return false
}

func scriptContainingFunctionOrModuleScope(masked string, offset int) (int, int) {
	bestStart, bestEnd := -1, len(masked)
	for open := strings.IndexByte(masked, '('); open >= 0 && open < offset; {
		close := matchingScriptDelimiter(masked, open, '(', ')')
		if close < 0 {
			break
		}
		bodyStart := scriptParameterBlockBodyStart(masked, close)
		if bodyStart >= 0 && bodyStart < offset && isScriptFunctionParameterScopePrefix(masked, open, close, bodyStart) {
			bodyEnd := matchingScriptBrace(masked, bodyStart)
			if bodyEnd < 0 {
				bodyEnd = len(masked)
			}
			if offset < bodyEnd && bodyStart > bestStart {
				bestStart, bestEnd = bodyStart, bodyEnd
			}
		}
		relative := strings.IndexByte(masked[close+1:], '(')
		if relative < 0 {
			break
		}
		open = close + 1 + relative
	}
	for search := 0; search < offset; {
		relative := strings.Index(masked[search:], "=>")
		if relative < 0 {
			break
		}
		arrow := search + relative
		bodyStart := nextScriptNonSpace(masked, arrow+2)
		if bodyStart < offset && bodyStart < len(masked) && masked[bodyStart] == '{' {
			bodyEnd := matchingScriptBrace(masked, bodyStart)
			if bodyEnd < 0 {
				bodyEnd = len(masked)
			}
			if offset < bodyEnd && bodyStart > bestStart {
				bestStart, bestEnd = bodyStart, bodyEnd
			}
		}
		search = arrow + 2
	}
	if bestStart < 0 {
		return 0, len(masked)
	}
	return bestStart, bestEnd
}

func isScriptFunctionParameterScopePrefix(masked string, open, close, bodyStart int) bool {
	statementStart := scriptStatementStart([]byte(masked), open)
	if hasScriptIdentifierWord(masked[statementStart:open], "function") {
		return true
	}
	if hasScriptLineBreak([]byte(masked), close+1, bodyStart) {
		return false
	}
	switch scriptIdentifierBefore(masked, open) {
	case "catch", "for", "if", "switch", "while", "with":
		return false
	default:
		return isScriptParameterScopePrefix(masked, open)
	}
}

func scriptIdentifierBefore(masked string, before int) string {
	index := before - 1
	for index >= 0 && isScriptWhitespace(masked[index]) {
		index--
	}
	end := index + 1
	for index >= 0 && isScriptIdentifierByte(masked[index]) {
		index--
	}
	return masked[index+1 : end]
}

func scriptVariableStatementEnd(masked string, start int) int {
	round, square, curly := 0, 0, 0
	for index := start; index < len(masked); index++ {
		switch masked[index] {
		case '(':
			round++
		case ')':
			if round > 0 {
				round--
			}
		case '[':
			square++
		case ']':
			if square > 0 {
				square--
			}
		case '{':
			curly++
		case '}':
			if curly > 0 {
				curly--
			}
		case ';', '\n':
			if round == 0 && square == 0 && curly == 0 {
				if masked[index] == '\n' {
					previous := index - 1
					for previous >= start && isScriptWhitespace(masked[previous]) {
						previous--
					}
					if previous >= start && masked[previous] == ',' {
						continue
					}
					next := nextScriptNonSpace(masked, index+1)
					if next < len(masked) && masked[next] == ',' {
						continue
					}
				}
				return index
			}
		}
	}
	return len(masked)
}

func scriptBindingPatternNames(pattern string) map[string]bool {
	names := map[string]bool{}
	var collect func(string)
	collect = func(value string) {
		value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "..."))
		if value == "" {
			return
		}
		if equals := findScriptTopLevel(value, '='); equals >= 0 {
			value = strings.TrimSpace(value[:equals])
		}
		if value == "" {
			return
		}
		switch value[0] {
		case '{':
			inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "{"), "}"))
			for _, entry := range splitScriptTopLevel(inner, ',') {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}
				if colon := findScriptTopLevel(entry, ':'); colon >= 0 {
					collect(entry[colon+1:])
				} else {
					collect(entry)
				}
			}
		case '[':
			inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
			for _, entry := range splitScriptTopLevel(inner, ',') {
				collect(entry)
			}
		default:
			end := 0
			for end < len(value) && isScriptIdentifierByte(value[end]) {
				end++
			}
			if end > 0 {
				names[value[:end]] = true
			}
		}
	}
	collect(pattern)
	return names
}

func splitScriptTopLevel(value string, separator byte) []string {
	var result []string
	start := 0
	round, square, curly := 0, 0, 0
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			curly++
		case '}':
			curly--
		default:
			if value[index] == separator && round == 0 && square == 0 && curly == 0 {
				result = append(result, value[start:index])
				start = index + 1
			}
		}
	}
	return append(result, value[start:])
}

func findScriptTopLevel(value string, target byte) int {
	round, square, curly := 0, 0, 0
	for index := 0; index < len(value); index++ {
		if value[index] == target && round == 0 && square == 0 && curly == 0 {
			return index
		}
		switch value[index] {
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			curly++
		case '}':
			curly--
		}
	}
	return -1
}

func scriptParameterBindsName(params, name string) bool {
	for index := 0; index+len(name) <= len(params); index++ {
		if params[index:index+len(name)] != name {
			continue
		}
		beforeOK := index == 0 || !isScriptIdentifierByte(params[index-1])
		after := index + len(name)
		afterOK := after == len(params) || !isScriptIdentifierByte(params[after])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func isScriptIdentifierByte(value byte) bool {
	return (value >= 'A' && value <= 'Z') ||
		(value >= 'a' && value <= 'z') ||
		(value >= '0' && value <= '9') ||
		value == '_' || value == '$'
}

func isScriptWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func nextScriptNonSpace(masked string, start int) int {
	for start < len(masked) && isScriptWhitespace(masked[start]) {
		start++
	}
	return start
}

func matchingScriptDelimiter(masked string, open int, opener, closer byte) int {
	depth := 0
	for index := open; index < len(masked); index++ {
		switch masked[index] {
		case opener:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func scriptArrowExpressionEnd(masked string, start int) int {
	round, square, curly := 0, 0, 0
	for index := start; index < len(masked); index++ {
		switch masked[index] {
		case '(':
			round++
		case ')':
			if round == 0 {
				return index
			}
			round--
		case '[':
			square++
		case ']':
			if square == 0 {
				return index
			}
			square--
		case '{':
			curly++
		case '}':
			if curly == 0 {
				return index
			}
			curly--
		case ';', '\n':
			if round == 0 && square == 0 && curly == 0 {
				return index
			}
		}
	}
	return len(masked)
}

func isProvenScriptReturnType(masked string, closeParen, typeEnd int) bool {
	openParen := matchingScriptDelimiterBackward(masked, closeParen, '(', ')')
	if openParen < 0 {
		return false
	}
	next := nextScriptNonSpace(masked, typeEnd)
	if next+1 < len(masked) && masked[next:next+2] == "=>" {
		paramsStart, paramsEnd, ok := scriptArrowParameterRange(masked, next)
		return ok && paramsStart == openParen+1 && paramsEnd == closeParen
	}
	return next < len(masked) && masked[next] == '{' && isScriptParameterScopePrefix(masked, openParen)
}

func isProvenScriptParameterType(masked string, offset int) bool {
	for search := 0; search < len(masked); {
		relative := strings.Index(masked[search:], "=>")
		if relative < 0 {
			break
		}
		arrow := search + relative
		paramsStart, paramsEnd, ok := scriptArrowParameterRange(masked, arrow)
		if ok && offset >= paramsStart && offset < paramsEnd &&
			isScriptParameterAnnotationAt(masked[paramsStart:paramsEnd], offset-paramsStart) {
			return true
		}
		search = arrow + 2
	}
	for open := strings.IndexByte(masked, '('); open >= 0; {
		close := matchingScriptDelimiter(masked, open, '(', ')')
		if close < 0 {
			return false
		}
		if offset > open && offset < close &&
			isScriptParameterScopePrefix(masked, open) &&
			scriptParameterBlockBodyStart(masked, close) >= 0 &&
			isScriptParameterAnnotationAt(masked[open+1:close], offset-open-1) {
			return true
		}
		relative := strings.IndexByte(masked[close+1:], '(')
		if relative < 0 {
			break
		}
		open = close + 1 + relative
	}
	return false
}

func isScriptParameterAnnotationAt(params string, offset int) bool {
	start := 0
	for _, segment := range splitScriptTopLevel(params, ',') {
		end := start + len(segment)
		if offset >= start && offset < end {
			relative := offset - start
			colon := findScriptTopLevel(segment, ':')
			equals := findScriptTopLevel(segment, '=')
			return colon >= 0 && colon < relative &&
				(equals < 0 || (colon < equals && relative < equals))
		}
		start = end + 1
	}
	return false
}

func scriptParameterBlockBodyStart(masked string, close int) int {
	index := nextScriptNonSpace(masked, close+1)
	if index < len(masked) && masked[index] == '{' {
		return index
	}
	if index >= len(masked) || masked[index] != ':' {
		return -1
	}
	round, square, angle := 0, 0, 0
	for index++; index < len(masked); index++ {
		switch masked[index] {
		case '(':
			round++
		case ')':
			if round > 0 {
				round--
			}
		case '[':
			square++
		case ']':
			if square > 0 {
				square--
			}
		case '<':
			angle++
		case '>':
			if angle > 0 {
				angle--
			}
		case '{':
			if round == 0 && square == 0 && angle == 0 {
				return index
			}
		case ';', '\n':
			if round == 0 && square == 0 && angle == 0 {
				return -1
			}
		}
	}
	return -1
}

func matchingScriptDelimiterBackward(masked string, close int, opener, closer byte) int {
	depth := 0
	for index := close; index >= 0; index-- {
		switch masked[index] {
		case closer:
			depth++
		case opener:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
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
		case masked[index] == '/' && isScriptRegexStart(masked, index):
			end := scriptRegexLiteralEnd(masked, index)
			if end < 0 {
				index++
				continue
			}
			for index < end {
				if masked[index] != '\n' {
					masked[index] = ' '
				}
				index++
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

func isScriptRegexStart(masked []byte, slash int) bool {
	index := slash - 1
	for index >= 0 && (masked[index] == ' ' || masked[index] == '\t' || masked[index] == '\r' || masked[index] == '\n') {
		index--
	}
	if index < 0 {
		return true
	}
	switch masked[index] {
	case '=', '(', '[', '{', ',', ':', ';', '!', '?', '&', '|', '+', '-', '*', '%', '^', '~', '<':
		return true
	case '>':
		return index > 0 && masked[index-1] == '='
	case '}':
		return isScriptStatementBlockClose(masked, index)
	case ')':
		open := matchingScriptDelimiterBackward(string(masked), index, '(', ')')
		if open < 0 {
			return false
		}
		wordEnd := open
		for wordEnd > 0 && (masked[wordEnd-1] == ' ' || masked[wordEnd-1] == '\t') {
			wordEnd--
		}
		wordStart := wordEnd
		for wordStart > 0 && isScriptIdentifierByte(masked[wordStart-1]) {
			wordStart--
		}
		switch string(masked[wordStart:wordEnd]) {
		case "if", "while", "for", "with", "switch":
			return true
		}
	}
	end := index + 1
	for index >= 0 && isScriptIdentifierByte(masked[index]) {
		index--
	}
	switch string(masked[index+1 : end]) {
	case "return", "case", "throw", "else", "do", "typeof", "void", "delete", "yield", "await":
		return true
	}
	return false
}

func isScriptStatementBlockClose(masked []byte, close int) bool {
	open := matchingScriptDelimiterBackward(string(masked), close, '{', '}')
	if open < 0 {
		return false
	}
	return isScriptStatementBlockOpen(masked, open)
}

func isScriptStatementBlockOpen(masked []byte, open int) bool {
	previous := open - 1
	for previous >= 0 && isScriptWhitespace(masked[previous]) {
		previous--
	}
	if previous < 0 || masked[previous] == ';' || masked[previous] == '}' {
		return true
	}
	if hasScriptLineBreak(masked, previous+1, open) && scriptTokenCanEndStatement(masked, previous) {
		return true
	}
	if isScriptIdentifierByte(masked[previous]) {
		end := previous + 1
		for previous >= 0 && isScriptIdentifierByte(masked[previous]) {
			previous--
		}
		switch string(masked[previous+1 : end]) {
		case "catch", "else", "finally":
			return true
		}
	}
	if masked[previous] == ':' {
		return isScriptLabelledBlockOpen(masked, open, previous)
	}
	if masked[previous] == ')' {
		paramsOpen := matchingScriptDelimiterBackward(string(masked), previous, '(', ')')
		if paramsOpen < 0 {
			return false
		}
		wordEnd := paramsOpen
		for wordEnd > 0 && (masked[wordEnd-1] == ' ' || masked[wordEnd-1] == '\t') {
			wordEnd--
		}
		wordStart := wordEnd
		for wordStart > 0 && isScriptIdentifierByte(masked[wordStart-1]) {
			wordStart--
		}
		switch string(masked[wordStart:wordEnd]) {
		case "if", "for", "while", "switch", "with", "catch":
			return true
		}
		statementStart := scriptStatementStart(masked, paramsOpen)
		prefix := string(masked[statementStart:paramsOpen])
		return hasScriptIdentifierWord(prefix, "function") && !strings.Contains(prefix, "=")
	}
	statementStart := scriptStatementStart(masked, open)
	prefix := strings.TrimSpace(string(masked[statementStart:open]))
	if hasScriptIdentifierWord(prefix, "function") && !strings.Contains(prefix, "=") {
		return true
	}
	return (strings.HasPrefix(prefix, "class ") || strings.HasPrefix(prefix, "export class ")) && !strings.Contains(prefix, "=")
}

func hasScriptIdentifierWord(value, word string) bool {
	for search := 0; search < len(value); {
		relative := strings.Index(value[search:], word)
		if relative < 0 {
			return false
		}
		start := search + relative
		end := start + len(word)
		beforeOK := start == 0 || !isScriptIdentifierByte(value[start-1])
		afterOK := end == len(value) || !isScriptIdentifierByte(value[end])
		if beforeOK && afterOK {
			return true
		}
		search = end
	}
	return false
}

func isScriptLabelledBlockOpen(masked []byte, open, colon int) bool {
	labelEnd := colon
	for labelEnd > 0 && isScriptWhitespace(masked[labelEnd-1]) {
		labelEnd--
	}
	labelStart := labelEnd
	for labelStart > 0 && isScriptIdentifierByte(masked[labelStart-1]) {
		labelStart--
	}
	if labelStart == labelEnd {
		return false
	}
	previous := labelStart - 1
	for previous >= 0 && isScriptWhitespace(masked[previous]) {
		previous--
	}
	if previous >= 0 &&
		hasScriptLineBreak(masked, previous+1, labelStart) &&
		scriptTokenCanEndStatement(masked, previous) {
		previous = -1
	}
	if previous >= 0 && masked[previous] != ';' && masked[previous] != '{' && masked[previous] != '}' {
		return false
	}
	if outer := containingScriptBraceOpen(masked, open); outer >= 0 && !isScriptStatementBlockOpen(masked, outer) {
		return false
	}
	return true
}

func hasScriptLineBreak(masked []byte, start, end int) bool {
	for index := start; index < end; index++ {
		if masked[index] == '\n' || masked[index] == '\r' {
			return true
		}
	}
	return false
}

func scriptTokenCanEndStatement(masked []byte, index int) bool {
	if index < 0 {
		return false
	}
	if isScriptIdentifierByte(masked[index]) {
		return true
	}
	switch masked[index] {
	case ')', ']', '}', '\'', '"', '`':
		return true
	}
	return false
}

func containingScriptBraceOpen(masked []byte, before int) int {
	stack := []int{}
	for index := 0; index < before; index++ {
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
		return -1
	}
	return stack[len(stack)-1]
}

func scriptStatementStart(masked []byte, before int) int {
	for index := before - 1; index >= 0; index-- {
		if masked[index] == ';' || masked[index] == '\n' {
			return index + 1
		}
	}
	return 0
}

func scriptRegexLiteralEnd(masked []byte, slash int) int {
	inClass := false
	escaped := false
	for index := slash + 1; index < len(masked); index++ {
		if masked[index] == '\n' || masked[index] == '\r' {
			return -1
		}
		if escaped {
			escaped = false
			continue
		}
		if masked[index] == '\\' {
			escaped = true
			continue
		}
		switch masked[index] {
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if inClass {
				continue
			}
			index++
			for index < len(masked) && ((masked[index] >= 'A' && masked[index] <= 'Z') || (masked[index] >= 'a' && masked[index] <= 'z')) {
				index++
			}
			return index
		}
	}
	return -1
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
		if resolver.locals[declaration.File] == nil {
			resolver.locals[declaration.File] = map[string][]RichSymbolRecord{}
		}
		resolver.locals[declaration.File][declaration.Name] = append(resolver.locals[declaration.File][declaration.Name], declaration)
		if declaration.ExportName == "" {
			continue
		}
		if resolver.declarations[declaration.File] == nil {
			resolver.declarations[declaration.File] = map[string][]RichSymbolRecord{}
		}
		resolver.declarations[declaration.File][declaration.ExportName] = append(resolver.declarations[declaration.File][declaration.ExportName], declaration)
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
			targetIdentity = scriptReferenceAliasIdentity(*reference, targetIdentity)
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
			targetIdentity := scriptReferenceAliasIdentity(*reference, strings.Join(reference.CandidateSymbolIDs, ","))
			reference.ID = StableWorkspaceUsageID("", "", reference.FromSymbolID, SymbolUsageAmbiguous, reference.Type, targetIdentity, reference.From, reference.Line)
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
	resolved := resolver.resolveModule(
		reference.From,
		reference.TargetModule,
		scriptReferencePackageCondition(reference.Type),
	)
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
	reference.To = module.identity
	reference.TargetQualifiedName = module.identity
	reference.Resolution = SymbolResolutionExact
	reference.Confidence = string(ConfidenceExact)
	reference.ConfidenceScore = 1
	reference.Internal = true
	reference.Reason = resolved.reason
	reference.preventExact = false
	targetIdentity := scriptReferenceAliasIdentity(*reference, module.identity)
	reference.ID = StableWorkspaceUsageID("", "", reference.FromSymbolID, SymbolUsageDirectReference, reference.Type, targetIdentity, reference.From, reference.Line)
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
	modules    []scriptResolvedModule
	candidates []RichSymbolRecord
	reason     string
	ambiguous  bool
}

type scriptModuleResolution struct {
	modules   []scriptResolvedModule
	reason    string
	ambiguous bool
}

type scriptResolvedModule struct {
	identity string
	file     string
}

type scriptExportResolution struct {
	candidates []RichSymbolRecord
	cyclic     bool
	ambiguous  bool
}

func (resolver scriptFactResolver) resolveReference(reference RichRelationRecord) scriptReferenceResolution {
	requiredCapability := scriptReferenceCapability(reference)
	if requiredCapability == scriptValueCapability && reference.scriptTypeOnly {
		return scriptReferenceResolution{reason: "type-only binding cannot be used as a value"}
	}
	if reference.Type == "calls_local" && reference.TargetModule == scriptModuleIdentity(reference.From) {
		candidates := filterScriptDeclarationsByCapability(
			dedupeScriptDeclarations(resolver.locals[reference.From][reference.TargetExport]),
			requiredCapability,
		)
		if len(candidates) == 1 {
			return scriptReferenceResolution{modules: []scriptResolvedModule{{identity: reference.TargetModule, file: reference.From}}, candidates: candidates, reason: "same-module lexical declaration"}
		}
		if len(candidates) > 1 {
			return scriptReferenceResolution{modules: []scriptResolvedModule{{identity: reference.TargetModule, file: reference.From}}, candidates: candidates, reason: "ambiguous same-module declaration"}
		}
		return scriptReferenceResolution{modules: []scriptResolvedModule{{identity: reference.TargetModule, file: reference.From}}, reason: "same-module declaration not found"}
	}
	moduleResolution := resolver.resolveModule(
		reference.From,
		reference.TargetModule,
		scriptReferencePackageCondition(reference.Type),
	)
	if len(moduleResolution.modules) == 0 {
		return scriptReferenceResolution{reason: moduleResolution.reason, ambiguous: moduleResolution.ambiguous}
	}
	var candidates []RichSymbolRecord
	cyclic := false
	ambiguous := moduleResolution.ambiguous
	for _, module := range moduleResolution.modules {
		resolved := resolver.resolveExport(module, reference.TargetExport, requiredCapability, map[string]bool{})
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

func scriptReferenceCapability(reference RichRelationRecord) scriptSymbolCapability {
	switch reference.Type {
	case "type_reference", "imports_type", "reexports_type":
		return scriptTypeCapability
	case "calls_export", "calls_local", "instantiates", "renders_component":
		return scriptValueCapability
	default:
		return 0
	}
}

func filterScriptDeclarationsByCapability(declarations []RichSymbolRecord, required scriptSymbolCapability) []RichSymbolRecord {
	if required == 0 {
		return declarations
	}
	result := make([]RichSymbolRecord, 0, len(declarations))
	for _, declaration := range declarations {
		if declaration.scriptCapability&required != 0 {
			result = append(result, declaration)
		}
	}
	return result
}

func (resolver scriptFactResolver) resolveExport(module scriptResolvedModule, exportName string, required scriptSymbolCapability, visited map[string]bool) scriptExportResolution {
	key := module.file + "\x00" + exportName + "\x00" + string(rune(required))
	if visited[key] {
		return scriptExportResolution{cyclic: true}
	}
	visited[key] = true
	defer delete(visited, key)
	if direct := filterScriptDeclarationsByCapability(resolver.declarations[module.file][exportName], required); len(direct) > 0 {
		return scriptExportResolution{candidates: append([]RichSymbolRecord(nil), direct...)}
	}
	var result []RichSymbolRecord
	cyclic := false
	ambiguous := false
	moduleFile := module.file
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
		if required == scriptValueCapability && reference.scriptTypeOnly {
			continue
		}
		localCandidates := filterScriptDeclarationsByCapability(resolver.locals[moduleFile][reference.TargetExport], required)
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
			if required == scriptValueCapability && imported.scriptTypeOnly {
				continue
			}
			targetModules := resolver.resolveModule(
				imported.From,
				imported.TargetModule,
				scriptReferencePackageCondition(imported.Type),
			)
			ambiguous = ambiguous || targetModules.ambiguous
			for _, targetModule := range targetModules.modules {
				resolved := resolver.resolveExport(targetModule, imported.TargetExport, required, visited)
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
			if exportName == "default" {
				continue
			}
			sourceExport = exportName
			publicExport = exportName
		}
		if publicExport == "" {
			publicExport = sourceExport
		}
		if publicExport != exportName {
			continue
		}
		if required == scriptValueCapability && reference.scriptTypeOnly {
			continue
		}
		targetModules := resolver.resolveModule(
			reference.From,
			reference.TargetModule,
			scriptReferencePackageCondition(reference.Type),
		)
		ambiguous = ambiguous || targetModules.ambiguous
		for _, targetModule := range targetModules.modules {
			resolved := resolver.resolveExport(targetModule, sourceExport, required, visited)
			result = append(result, resolved.candidates...)
			cyclic = cyclic || resolved.cyclic
			ambiguous = ambiguous || resolved.ambiguous
		}
	}
	return scriptExportResolution{candidates: dedupeScriptDeclarations(result), cyclic: cyclic, ambiguous: ambiguous}
}

func (resolver scriptFactResolver) resolveModule(fromFile, specifier, condition string) scriptModuleResolution {
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
	if modules, reason := resolver.resolveWorkspaceModules(fromFile, specifier, condition); len(modules) > 0 || reason != "" {
		return scriptModuleResolution{modules: modules, reason: reason, ambiguous: strings.Contains(reason, "ambiguous")}
	}
	return scriptModuleResolution{reason: "external or unresolved module specifier"}
}

func (resolver scriptFactResolver) aliasDependencyLimitation(fromFile string, modules []scriptResolvedModule) string {
	consumer, consumerKnown := nearestNodePackage(fromFile, resolver.packages)
	for _, module := range modules {
		provider, providerKnown := nearestNodePackage(module.file, resolver.packages)
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

func (resolver scriptFactResolver) resolveFileModules(base string) ([]scriptResolvedModule, string) {
	files := scriptFileCandidates(base, resolver.files)
	modules := make([]scriptResolvedModule, 0, len(files))
	for _, file := range files {
		modules = append(modules, scriptResolvedModule{identity: scriptModuleIdentity(file), file: file})
	}
	if len(modules) > 1 {
		return modules, "ambiguous module path"
	}
	if len(modules) == 1 {
		return modules, "relative module path"
	}
	return nil, "module file not found"
}

func (resolver scriptFactResolver) resolveAliasModules(fromFile, specifier string) []scriptResolvedModule {
	configPath, config, ok := nearestScriptConfig(fromFile, resolver.configs)
	if !ok {
		return nil
	}
	var modules []scriptResolvedModule
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
				modules = append(modules, scriptResolvedModule{identity: scriptModuleIdentity(file), file: file})
			}
		}
	}
	if len(modules) == 0 && config.BaseURL != "" {
		base := path.Join(path.Dir(configPath), config.BaseURL, specifier)
		for _, file := range scriptFileCandidates(base, resolver.files) {
			modules = append(modules, scriptResolvedModule{identity: scriptModuleIdentity(file), file: file})
		}
	}
	sortScriptResolvedModules(modules)
	return modules
}

func (resolver scriptFactResolver) resolveWorkspaceModules(fromFile, specifier, condition string) ([]scriptResolvedModule, string) {
	providers, subpath, ok := workspacePackagesForSpecifier(specifier, resolver.packages)
	if !ok {
		return nil, ""
	}
	consumer, ok := nearestNodePackage(fromFile, resolver.packages)
	if !ok || !scriptContainsString(consumer.Dependencies, providers[0].Name) {
		return nil, "workspace package is not a declared dependency"
	}
	key := "."
	if subpath != "" {
		key = "./" + subpath
	}
	var modules []scriptResolvedModule
	targetCount := 0
	unselectedConditional := false
	for _, provider := range providers {
		targets, conditional := scriptPackageExportTargets(provider, key, condition)
		if conditional && condition == "" {
			unselectedConditional = true
		}
		if len(targets) == 0 && key == "." && provider.Types != "" && (condition == "types" || !conditional) {
			targets = []string{provider.Types}
		}
		root := path.Dir(provider.Path)
		if len(targets) == 0 && key == "." && !conditional {
			targets = []string{"src/index", "index"}
		}
		targetCount += len(targets)
		for _, target := range targets {
			for _, file := range scriptFileCandidates(path.Join(root, strings.TrimPrefix(target, "./")), resolver.files) {
				modules = append(modules, scriptResolvedModule{identity: scriptModuleIdentity(file), file: file})
			}
		}
	}
	modules = uniqueScriptResolvedModules(modules)
	if unselectedConditional || len(providers) > 1 || len(modules) > 1 || targetCount > 1 {
		return modules, "ambiguous workspace package export"
	}
	if len(modules) == 1 {
		return modules, "workspace package dependency and static export"
	}
	return nil, "workspace package target was not found"
}

func scriptPackageExportTargets(provider NodePackageRecord, key, condition string) ([]string, bool) {
	branches := provider.ExportConditions[key]
	if len(branches) == 0 {
		return append([]string(nil), provider.Exports[key]...), false
	}
	for _, branch := range scriptPackageConditionBranches(condition) {
		if targets := branches[branch]; len(targets) > 0 {
			return append([]string(nil), targets...), true
		}
	}
	if condition == "" {
		return append([]string(nil), provider.Exports[key]...), true
	}
	return nil, true
}

func scriptPackageConditionBranches(condition string) []string {
	switch condition {
	case "types":
		return []string{"types", "import", "default"}
	case "import":
		return []string{"import", "default"}
	default:
		return nil
	}
}

func scriptReferencePackageCondition(referenceType string) string {
	switch referenceType {
	case "imports_type", "reexports_type":
		return "types"
	case "imports_value", "imports_namespace", "imports_module", "reexports_value", "reexports_all":
		return "import"
	default:
		return ""
	}
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

func workspacePackagesForSpecifier(specifier string, packages []NodePackageRecord) ([]NodePackageRecord, string, bool) {
	bestName := ""
	var matches []NodePackageRecord
	for _, candidate := range packages {
		if candidate.Name == "" || (specifier != candidate.Name && !strings.HasPrefix(specifier, candidate.Name+"/")) {
			continue
		}
		if len(candidate.Name) > len(bestName) {
			bestName = candidate.Name
			matches = []NodePackageRecord{candidate}
		} else if candidate.Name == bestName {
			matches = append(matches, candidate)
		}
	}
	if bestName == "" {
		return nil, "", false
	}
	return matches, strings.TrimPrefix(strings.TrimPrefix(specifier, bestName), "/"), true
}

func sortScriptResolvedModules(modules []scriptResolvedModule) {
	sort.Slice(modules, func(i, j int) bool {
		if modules[i].file != modules[j].file {
			return modules[i].file < modules[j].file
		}
		return modules[i].identity < modules[j].identity
	})
}

func uniqueScriptResolvedModules(modules []scriptResolvedModule) []scriptResolvedModule {
	seen := map[string]bool{}
	result := make([]scriptResolvedModule, 0, len(modules))
	for _, module := range modules {
		if seen[module.file] {
			continue
		}
		seen[module.file] = true
		result = append(result, module)
	}
	sortScriptResolvedModules(result)
	return result
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
