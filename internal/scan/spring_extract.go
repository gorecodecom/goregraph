package scan

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// BuildProjectAPICatalog creates the complete provider inventory for one project.
func BuildProjectAPICatalog(project, generated string, routes []CodeRouteRecord, spring SpringIndex, contracts []APIContractRecord, capabilities []CapabilityRecord) APICatalogRecord {
	_ = contracts // Project contracts describe outbound calls; workspace reconciliation attaches consumers later.
	project = filepath.ToSlash(strings.TrimSpace(project))
	coverageByLanguage := routeCoverageByLanguage(capabilities)
	javaEvidence := springRouteEvidence(routes)
	candidates := make([]APIEndpointRecord, 0, len(spring.Endpoints)+len(routes))

	for _, endpoint := range spring.Endpoints {
		path := canonicalProviderPath(endpoint.Path)
		handler := endpoint.Controller + "." + endpoint.Method
		record := APIEndpointRecord{
			ProviderProject: project,
			ProviderRole:    "backend",
			Transport:       "http",
			HTTPMethod:      strings.ToUpper(strings.TrimSpace(endpoint.HTTPMethod)),
			Path:            path,
			Language:        "java",
			Framework:       "Spring",
			Controller:      endpoint.Controller,
			Handler:         handler,
			File:            filepath.ToSlash(endpoint.File),
			Line:            endpoint.Line,
			Parameters:      springAPIParameters(endpoint.Parameters),
			Consumes:        splitSpringMediaTypes(endpoint.Consumes),
			RequestType:     endpoint.RequestType,
			ResponseType:    endpoint.ReturnType,
			Security:        unknownAPISecurity(),
			Consumers:       []APIConsumerRecord{},
			Confidence:      ConfidenceExact,
			Coverage:        routeCoverage("java", coverageByLanguage),
			EvidenceIDs:     append([]string(nil), javaEvidence[springProviderEvidenceKey(endpoint)]...),
		}
		if path != endpoint.Path {
			record.RawPath = endpoint.Path
		}
		record.ID = StableAPIEndpointID(project, record.Transport, record.HTTPMethod, record.Path, record.Handler, record.File, record.Line)
		candidates = append(candidates, record)
	}

	for _, route := range routes {
		if !supportedScriptProviderRoute(route) {
			continue
		}
		path := canonicalProviderPath(route.Path)
		record := APIEndpointRecord{
			ProviderProject: project,
			ProviderRole:    "backend",
			Transport:       "http",
			HTTPMethod:      strings.ToUpper(strings.TrimSpace(route.HTTPMethod)),
			Path:            path,
			Language:        route.Language,
			Framework:       route.Framework,
			Handler:         route.Handler,
			File:            filepath.ToSlash(route.File),
			Line:            route.Line,
			Security:        unknownAPISecurity(),
			Consumers:       []APIConsumerRecord{},
			Confidence:      apiRouteConfidence(route.Confidence),
			Coverage:        routeCoverage(route.Language, coverageByLanguage),
			EvidenceIDs:     append([]string(nil), route.EvidenceIDs...),
		}
		if path != route.Path {
			record.RawPath = route.Path
		}
		record.ID = StableAPIEndpointID(project, record.Transport, record.HTTPMethod, record.Path, record.Handler, record.File, record.Line)
		candidates = append(candidates, record)
	}

	sort.Slice(candidates, func(left, right int) bool {
		leftKey := projectProviderIdentity(candidates[left]) + "\x00" + candidates[left].File
		rightKey := projectProviderIdentity(candidates[right]) + "\x00" + candidates[right].File
		if leftKey != rightKey {
			return leftKey < rightKey
		}
		if candidates[left].Line != candidates[right].Line {
			return candidates[left].Line < candidates[right].Line
		}
		return candidates[left].Path < candidates[right].Path
	})
	catalog := APICatalogRecord{SchemaVersion: SchemaVersion, Generated: generated, Root: project, Endpoints: []APIEndpointRecord{}}
	for _, candidate := range candidates {
		last := len(catalog.Endpoints) - 1
		if last >= 0 && projectProviderIdentity(catalog.Endpoints[last]) == projectProviderIdentity(candidate) {
			mergeProjectProviderEndpoint(&catalog.Endpoints[last], candidate)
			continue
		}
		catalog.Endpoints = append(catalog.Endpoints, candidate)
	}
	SortAPICatalog(&catalog)
	return catalog
}

func supportedScriptProviderRoute(route CodeRouteRecord) bool {
	return (route.Language == "javascript" || route.Language == "typescript") &&
		route.Kind == "backend" && supportedScriptProviderFramework(route.Framework) && strings.TrimSpace(route.Handler) != "" &&
		strings.TrimSpace(route.File) != "" && route.Line > 0
}

func supportedScriptProviderFramework(framework string) bool {
	switch strings.ToLower(strings.TrimSpace(framework)) {
	case "express", "fastify":
		return true
	default:
		return false
	}
}

func canonicalProviderPath(value string) string {
	value = normalizeCodeRoutePath(value)
	return regexp.MustCompile(`(^|/):([A-Za-z_$][A-Za-z0-9_$]*)`).ReplaceAllString(value, `${1}{${2}}`)
}

func projectProviderIdentity(endpoint APIEndpointRecord) string {
	return endpoint.ProviderProject + "\x00" + strings.ToUpper(endpoint.HTTPMethod) + "\x00" + normalizeAPIPathParameterNames(endpoint.Path) + "\x00" + endpoint.Handler
}

func mergeProjectProviderEndpoint(target *APIEndpointRecord, extra APIEndpointRecord) {
	if len(target.Parameters) == 0 && len(extra.Parameters) > 0 {
		target.Parameters = extra.Parameters
	}
	if len(target.Consumes) == 0 && len(extra.Consumes) > 0 {
		target.Consumes = extra.Consumes
	}
	if target.RequestType == "" {
		target.RequestType = extra.RequestType
	}
	if target.ResponseType == "" {
		target.ResponseType = extra.ResponseType
	}
	target.EvidenceIDs = append(target.EvidenceIDs, extra.EvidenceIDs...)
	target.EvidenceIDs = catalogUniqueSortedStrings(target.EvidenceIDs)
}

func springAPIParameters(parameters []JavaParameterRecord) []APIParameterRecord {
	result := make([]APIParameterRecord, 0, len(parameters))
	for _, parameter := range parameters {
		result = append(result, springAPIParameter(parameter))
	}
	return result
}

func springAPIParameter(parameter JavaParameterRecord) APIParameterRecord {
	record := APIParameterRecord{Name: parameter.Name, Location: "unknown", Type: parameter.Type, Source: "java_parameter", Confidence: ConfidenceUnknown}
	for _, annotation := range parameter.Annotations {
		location := ""
		switch annotation.Name {
		case "PathVariable":
			location = "path"
		case "RequestParam":
			location = "query"
		case "RequestHeader":
			location = "header"
		case "CookieValue":
			location = "cookie"
		case "RequestBody", "RequestPart":
			location = "body"
		}
		if location == "" {
			continue
		}
		record.Location = location
		record.Source = "parameter_annotation"
		record.Confidence = ConfidenceExact
		record.Required = annotation.Attributes["required"] != "false" && annotation.Attributes["defaultValue"] == ""
		name := firstNonEmpty(annotation.Attributes["name"], annotation.Attributes["value"])
		if name == "" && annotation.Arguments != "" && !strings.Contains(annotation.Arguments, "=") {
			name = trimJavaValue(annotation.Arguments)
		}
		if strings.TrimSpace(name) != "" {
			record.Name = strings.Trim(strings.TrimSpace(name), `"`)
		}
		break
	}
	return record
}

func splitSpringMediaTypes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(value), "{"), "}"))
	parts := splitTopLevel(value, ',')
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), `"`)
		if part != "" {
			result = append(result, normalizeSpringMediaType(part))
		}
	}
	return catalogUniqueSortedStrings(result)
}

func normalizeSpringMediaType(value string) string {
	switch value {
	case "MediaType.APPLICATION_JSON_VALUE":
		return "application/json"
	case "MediaType.APPLICATION_XML_VALUE":
		return "application/xml"
	case "MediaType.APPLICATION_OCTET_STREAM_VALUE":
		return "application/octet-stream"
	case "MediaType.APPLICATION_FORM_URLENCODED_VALUE":
		return "application/x-www-form-urlencoded"
	case "MediaType.MULTIPART_FORM_DATA_VALUE":
		return "multipart/form-data"
	case "MediaType.TEXT_HTML_VALUE":
		return "text/html"
	case "MediaType.TEXT_PLAIN_VALUE":
		return "text/plain"
	default:
		return value
	}
}

func springRouteEvidence(routes []CodeRouteRecord) map[string][]string {
	result := map[string][]string{}
	for _, route := range routes {
		if route.Language != "java" || route.Framework != "Spring" {
			continue
		}
		key := strings.ToUpper(route.HTTPMethod) + "\x00" + normalizeAPIPathParameterNames(canonicalProviderPath(route.Path)) + "\x00" + route.Handler + "\x00" + filepath.ToSlash(route.File) + "\x00" + strconv.Itoa(route.Line)
		result[key] = append(result[key], route.EvidenceIDs...)
		result[key] = catalogUniqueSortedStrings(result[key])
	}
	return result
}

func springProviderEvidenceKey(endpoint SpringEndpointRecord) string {
	return strings.ToUpper(endpoint.HTTPMethod) + "\x00" + normalizeAPIPathParameterNames(canonicalProviderPath(endpoint.Path)) + "\x00" + endpoint.Controller + "." + endpoint.Method + "\x00" + filepath.ToSlash(endpoint.File) + "\x00" + strconv.Itoa(endpoint.Line)
}

func routeCoverageByLanguage(capabilities []CapabilityRecord) map[string]Coverage {
	result := map[string]Coverage{}
	for _, capability := range capabilities {
		if capability.ID == CapabilityRoutes {
			result[capability.Language] = capability.Coverage
		}
	}
	return result
}

func routeCoverage(language string, coverage map[string]Coverage) Coverage {
	if value := coverage[language]; value != "" {
		return value
	}
	return CoveragePartial
}

func apiRouteConfidence(value string) Confidence {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "EXACT", "EXTRACTED":
		return ConfidenceExact
	case "RESOLVED":
		return ConfidenceResolved
	case "NORMALIZED":
		return ConfidenceNormalized
	case "INFERRED":
		return ConfidenceInferred
	case "WEAK", "PARTIAL":
		return ConfidenceWeak
	default:
		return ConfidenceUnknown
	}
}

func unknownAPISecurity() []SecurityEvidenceRecord {
	return []SecurityEvidenceRecord{{Kind: SecurityUnknown, Summary: "No auth evidence detected", Confidence: ConfidenceUnknown}}
}

func catalogUniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func buildSpringIndex(sources []JavaSourceRecord) SpringIndex {
	typeByName := map[string]JavaTypeRecord{}
	fileByType := map[string]string{}
	constants := map[string]string{}
	for _, source := range sources {
		for name, value := range source.Constants {
			constants[name] = value
		}
		for _, typ := range source.Types {
			typeByName[typ.Name] = typ
			fileByType[typ.Name] = typ.File
		}
	}

	var index SpringIndex
	for _, source := range sources {
		for _, typ := range source.Types {
			componentKind := springComponentKind(typ.Annotations)
			if hasAnnotation(typ.Annotations, "SpringBootApplication") {
				index.Applications = append(index.Applications, SpringApplicationRecord{
					Name:             typ.Name,
					File:             typ.File,
					Line:             typ.Line,
					ScanBasePackages: firstAnnotationValue(typ.Annotations, "SpringBootApplication", "scanBasePackages", constants),
				})
			}
			if componentKind != "" {
				index.Components = append(index.Components, SpringComponentRecord{
					Name:        typ.Name,
					Kind:        componentKind,
					File:        typ.File,
					Line:        typ.Line,
					Package:     typ.Package,
					Annotations: annotationNames(typ.Annotations),
				})
			}
			if hasAnnotation(typ.Annotations, "Entity") {
				index.Entities = append(index.Entities, SpringEntityRecord{
					Name:    typ.Name,
					File:    typ.File,
					Line:    typ.Line,
					Table:   firstAnnotationValue(typ.Annotations, "Table", "name", constants),
					Package: typ.Package,
				})
			}
			if componentKind == "repository" {
				repository := SpringRepositoryRecord{Name: typ.Name, File: typ.File, Line: typ.Line}
				repository.Entity, repository.IDType = parseRepositoryGeneric(typ.Extends)
				repository.EntityFile = fileByType[repository.Entity]
				index.Repositories = append(index.Repositories, repository)
			}
		}

		for _, method := range source.Methods {
			if hasAnnotation(method.Annotations, "Bean") {
				index.Beans = append(index.Beans, SpringBeanRecord{Name: beanName(method), Type: method.ReturnType, Config: method.Owner, File: method.File, Line: method.Line, MethodName: method.Name})
			}
			index.Endpoints = append(index.Endpoints, springEndpointsForMethod(source, method, constants)...)
		}

		for _, field := range source.Fields {
			if field.Owner == "" {
				continue
			}
			if _, ok := typeByName[field.Type]; !ok {
				continue
			}
			injection := "field"
			if field.Final && typeHasAnnotation(sources, field.Owner, "RequiredArgsConstructor") {
				injection = "constructor"
			}
			index.Dependencies = append(index.Dependencies, SpringDependencyRecord{
				From: field.Owner, To: field.Type, FromFile: field.File, ToFile: fileByType[field.Type], Field: field.Name, Injection: injection, Line: field.Line,
			})
		}
	}

	index.DTOs = springDTORecords(sources)
	applyGlobalSpringAuth(&index, springGlobalAuthRecords(sources))
	sortSpringIndex(&index)
	return index
}

func springComponentKind(annotations []JavaAnnotationRecord) string {
	switch {
	case hasAnnotation(annotations, "RestController"):
		return "rest_controller"
	case hasAnnotation(annotations, "Controller"):
		return "controller"
	case hasAnnotation(annotations, "Service"):
		return "service"
	case hasAnnotation(annotations, "Repository"):
		return "repository"
	case hasAnnotation(annotations, "Configuration"):
		return "configuration"
	case hasAnnotation(annotations, "Component"):
		return "component"
	default:
		return ""
	}
}

func springEndpointsForMethod(source JavaSourceRecord, method JavaMethodRecord, constants map[string]string) []SpringEndpointRecord {
	controller := ""
	classPath := ""
	for _, typ := range source.Types {
		if typ.Name == method.Owner && hasAnnotation(typ.Annotations, "RestController") {
			controller = typ.Name
			classPath = resolveSpringPath(firstMappingAnnotation(typ.Annotations), constants)
			break
		}
	}
	if controller == "" {
		return nil
	}

	annotation := firstMappingAnnotation(method.Annotations)
	if annotation.Name == "" {
		return nil
	}

	httpMethods := springHTTPMethods(annotation)
	paths := splitSpringPaths(resolveSpringPath(annotation, constants))
	if len(paths) == 0 {
		paths = []string{""}
	}

	var endpoints []SpringEndpointRecord
	for _, httpMethod := range httpMethods {
		for _, methodPath := range paths {
			requestType, requestKind := requestMetadata(method.Parameters)
			returnType := firstNonEmpty(openAPIResponseType(method.Annotations), method.ReturnType)
			endpoints = append(endpoints, SpringEndpointRecord{
				HTTPMethod:  httpMethod,
				Path:        joinSpringPaths(classPath, methodPath),
				Controller:  controller,
				Method:      method.Name,
				File:        method.File,
				Line:        annotation.Line,
				RequestType: requestType,
				RequestKind: requestKind,
				Consumes:    springConsumes(annotation, constants),
				ReturnType:  returnType,
				Parameters:  method.Parameters,
				Auth:        springAuthRecords(method.Annotations, method.File),
			})
		}
	}
	return endpoints
}

func springDTORecords(sources []JavaSourceRecord) []DTORecord {
	var records []DTORecord
	for _, source := range sources {
		for _, typ := range source.Types {
			if !isSpringDTOType(typ, source.Fields) {
				continue
			}
			record := DTORecord{
				Name:       typ.Name,
				Package:    typ.Package,
				File:       typ.File,
				Line:       typ.Line,
				Kind:       typ.Kind,
				Confidence: "EXTRACTED",
				Source:     "java_type_fields",
				Fields:     springDTOFields(typ.Name, source.Fields),
			}
			if len(record.Fields) > 0 {
				records = append(records, record)
			}
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
	return records
}

func isSpringDTOType(typ JavaTypeRecord, fields []JavaFieldRecord) bool {
	if springComponentKind(typ.Annotations) != "" || hasAnnotation(typ.Annotations, "Entity") {
		return false
	}
	switch {
	case strings.HasSuffix(typ.Name, "Request"), strings.HasSuffix(typ.Name, "Response"), strings.HasSuffix(typ.Name, "Dto"), strings.HasSuffix(typ.Name, "DTO"):
		return true
	}
	for _, field := range fields {
		if field.Owner == typ.Name {
			return true
		}
	}
	return false
}

func springDTOFields(owner string, fields []JavaFieldRecord) []DTOFieldRecord {
	var records []DTOFieldRecord
	for _, field := range fields {
		if field.Owner != owner {
			continue
		}
		records = append(records, DTOFieldRecord{
			Name:       field.Name,
			Type:       field.Type,
			Required:   fieldRequired(field.Annotations),
			Source:     fieldSource(field.Annotations),
			Confidence: "EXTRACTED",
		})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
	return records
}

func fieldRequired(annotations []JavaAnnotationRecord) bool {
	for _, annotation := range annotations {
		switch annotation.Name {
		case "NotNull", "NotBlank", "NotEmpty", "NonNull":
			return true
		}
	}
	return false
}

func fieldSource(annotations []JavaAnnotationRecord) string {
	if len(annotations) > 0 {
		return "field_annotation"
	}
	return "field"
}

func springAuthRecords(annotations []JavaAnnotationRecord, file string) []AuthRecord {
	var records []AuthRecord
	for _, annotation := range annotations {
		switch annotation.Name {
		case "PreAuthorize", "PostAuthorize":
			records = append(records, AuthRecord{
				Kind:       toSnake(annotation.Name),
				Expression: firstNonEmpty(annotation.Attributes["value"], annotation.Arguments),
				Source:     "method_annotation",
				Confidence: "EXTRACTED",
				File:       file,
				Line:       annotation.Line,
			})
		case "Secured", "RolesAllowed":
			records = append(records, AuthRecord{
				Kind:       toSnake(annotation.Name),
				Expression: firstNonEmpty(annotation.Attributes["value"], annotation.Arguments),
				Source:     "method_annotation",
				Confidence: "EXTRACTED",
				File:       file,
				Line:       annotation.Line,
			})
		}
	}
	return records
}

func springGlobalAuthRecords(sources []JavaSourceRecord) []AuthRecord {
	seen := map[string]bool{}
	var records []AuthRecord
	for _, source := range sources {
		for _, method := range source.Methods {
			for _, auth := range method.Auth {
				key := auth.Kind + ":" + auth.Expression
				if seen[key] {
					continue
				}
				seen[key] = true
				records = append(records, auth)
			}
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Kind != records[j].Kind {
			return records[i].Kind < records[j].Kind
		}
		return records[i].Expression < records[j].Expression
	})
	return records
}

func applyGlobalSpringAuth(index *SpringIndex, global []AuthRecord) {
	if len(global) == 0 {
		return
	}
	for i := range index.Endpoints {
		if len(index.Endpoints[i].Auth) == 0 {
			index.Endpoints[i].Auth = append([]AuthRecord(nil), global...)
		}
	}
}

func openAPIResponseType(annotations []JavaAnnotationRecord) string {
	for _, annotation := range annotations {
		if annotation.Name != "Operation" && annotation.Name != "ApiResponse" {
			continue
		}
		if match := regexp.MustCompile(`implementation\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\.class`).FindStringSubmatch(annotation.Arguments); len(match) == 2 {
			return match[1]
		}
	}
	return ""
}

func toSnake(value string) string {
	var b strings.Builder
	for index, r := range value {
		if index > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func firstMappingAnnotation(annotations []JavaAnnotationRecord) JavaAnnotationRecord {
	for _, annotation := range annotations {
		switch annotation.Name {
		case "RequestMapping", "GetMapping", "PostMapping", "PutMapping", "DeleteMapping", "PatchMapping":
			return annotation
		}
	}
	return JavaAnnotationRecord{}
}

func springHTTPMethods(annotation JavaAnnotationRecord) []string {
	switch annotation.Name {
	case "GetMapping":
		return []string{"GET"}
	case "PostMapping":
		return []string{"POST"}
	case "PutMapping":
		return []string{"PUT"}
	case "DeleteMapping":
		return []string{"DELETE"}
	case "PatchMapping":
		return []string{"PATCH"}
	case "RequestMapping":
		method := annotation.Attributes["method"]
		method = strings.TrimPrefix(method, "RequestMethod.")
		if method != "" {
			return []string{strings.ToUpper(method)}
		}
	}
	return []string{"ANY"}
}

func resolveSpringPath(annotation JavaAnnotationRecord, constants map[string]string) string {
	if annotation.Name == "" {
		return ""
	}
	value := annotation.Attributes["path"]
	if value == "" {
		value = annotation.Attributes["value"]
	}
	if value == "" && annotation.Arguments != "" && !strings.Contains(annotation.Arguments, "=") {
		value = trimJavaValue(annotation.Arguments)
	}
	value = strings.TrimSpace(value)
	if resolved, ok := constants[value]; ok {
		return resolved
	}
	return value
}

func splitSpringPaths(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if isSpringPathArray(path) {
		path = strings.TrimPrefix(path, "{")
		path = strings.TrimSuffix(path, "}")
	}
	parts := splitTopLevel(path, ',')
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, `"`))
		if part != "" {
			paths = append(paths, part)
		}
	}
	return paths
}

func joinSpringPaths(base, child string) string {
	base = normalizeSpringPath(base)
	child = normalizeSpringPath(child)
	if base == "" {
		if child == "" {
			return "/"
		}
		return child
	}
	if child == "" || child == "/" {
		return base
	}
	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(child, "/")
}

func normalizeSpringPath(path string) string {
	path = strings.TrimSpace(strings.Trim(path, `"`))
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func requestMetadata(params []JavaParameterRecord) (string, string) {
	for _, param := range params {
		if hasAnnotation(param.Annotations, "RequestBody") {
			return param.Type, "body"
		}
	}
	for _, param := range params {
		if hasAnnotation(param.Annotations, "RequestPart") {
			return param.Type, "multipart"
		}
	}
	for _, param := range params {
		if param.Type == "MultipartFile" {
			return param.Type, "multipart"
		}
	}
	return "", ""
}

func springConsumes(annotation JavaAnnotationRecord, constants map[string]string) string {
	value := annotation.Attributes["consumes"]
	if value == "" {
		return ""
	}
	if resolved, ok := constants[value]; ok {
		return resolved
	}
	switch value {
	case "MediaType.MULTIPART_FORM_DATA_VALUE":
		return "multipart/form-data"
	default:
		return value
	}
}

func isSpringPathArray(path string) bool {
	trimmed := strings.TrimSpace(path)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "{"), "}"))
	return strings.Contains(inner, ",") || strings.Contains(inner, `"`)
}

func parseRepositoryGeneric(extends string) (string, string) {
	match := regexp.MustCompile(`JpaRepository\s*<\s*([^,\s>]+)\s*,\s*([^>\s]+)`).FindStringSubmatch(extends)
	if len(match) != 3 {
		return "", ""
	}
	return strings.TrimSpace(match[1]), strings.TrimSpace(match[2])
}

func beanName(method JavaMethodRecord) string {
	for _, annotation := range method.Annotations {
		if annotation.Name == "Bean" {
			if name := annotation.Attributes["name"]; name != "" {
				return name
			}
			if value := annotation.Attributes["value"]; value != "" {
				return value
			}
		}
	}
	return method.Name
}

func hasAnnotation(annotations []JavaAnnotationRecord, name string) bool {
	for _, annotation := range annotations {
		if annotation.Name == name {
			return true
		}
	}
	return false
}

func firstAnnotationValue(annotations []JavaAnnotationRecord, name, key string, constants map[string]string) string {
	for _, annotation := range annotations {
		if annotation.Name != name {
			continue
		}
		value := annotation.Attributes[key]
		if value == "" {
			value = annotation.Attributes["value"]
		}
		if resolved, ok := constants[value]; ok {
			return resolved
		}
		return value
	}
	return ""
}

func annotationNames(annotations []JavaAnnotationRecord) []string {
	names := make([]string, 0, len(annotations))
	for _, annotation := range annotations {
		names = append(names, annotation.Name)
	}
	sort.Strings(names)
	return names
}

func typeHasAnnotation(sources []JavaSourceRecord, typeName, annotationName string) bool {
	for _, source := range sources {
		for _, typ := range source.Types {
			if typ.Name == typeName && hasAnnotation(typ.Annotations, annotationName) {
				return true
			}
		}
	}
	return false
}

func sortSpringIndex(index *SpringIndex) {
	sort.Slice(index.Applications, func(i, j int) bool { return index.Applications[i].Name < index.Applications[j].Name })
	sort.Slice(index.Components, func(i, j int) bool { return index.Components[i].Name < index.Components[j].Name })
	sort.Slice(index.Endpoints, func(i, j int) bool {
		if index.Endpoints[i].Path != index.Endpoints[j].Path {
			return index.Endpoints[i].Path < index.Endpoints[j].Path
		}
		return index.Endpoints[i].HTTPMethod < index.Endpoints[j].HTTPMethod
	})
	sort.Slice(index.Dependencies, func(i, j int) bool {
		if index.Dependencies[i].From != index.Dependencies[j].From {
			return index.Dependencies[i].From < index.Dependencies[j].From
		}
		return index.Dependencies[i].To < index.Dependencies[j].To
	})
	sort.Slice(index.Repositories, func(i, j int) bool { return index.Repositories[i].Name < index.Repositories[j].Name })
	sort.Slice(index.Entities, func(i, j int) bool { return index.Entities[i].Name < index.Entities[j].Name })
	sort.Slice(index.Beans, func(i, j int) bool { return index.Beans[i].Name < index.Beans[j].Name })
}
