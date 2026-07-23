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
			Produces:        splitSpringMediaTypes(endpoint.Produces),
			RequestType:     endpoint.RequestType,
			ResponseType:    endpoint.ReturnType,
			Security:        NormalizeSecurityEvidence(endpoint.Auth),
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
		return projectProviderCandidateSortKey(candidates[left]) < projectProviderCandidateSortKey(candidates[right])
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
		route.Kind == "backend" && route.FrameworkBound && supportedScriptProviderFramework(route.Framework) && strings.TrimSpace(route.Handler) != "" &&
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

func projectProviderCandidateSortKey(endpoint APIEndpointRecord) string {
	parameters := append([]APIParameterRecord(nil), endpoint.Parameters...)
	sort.Slice(parameters, func(left, right int) bool {
		return apiParameterSortKey(parameters[left]) < apiParameterSortKey(parameters[right])
	})
	parameterKeys := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		parameterKeys = append(parameterKeys, apiParameterSortKey(parameter))
	}
	return strings.Join([]string{
		projectProviderIdentity(endpoint),
		endpoint.File,
		strconv.Itoa(endpoint.Line),
		endpoint.Path,
		endpoint.RawPath,
		endpoint.Language,
		endpoint.Framework,
		endpoint.Controller,
		endpoint.RequestType,
		endpoint.ResponseType,
		string(endpoint.Confidence),
		string(endpoint.Coverage),
		strings.Join(catalogUniqueSortedStrings(endpoint.Consumes), "\x01"),
		strings.Join(catalogUniqueSortedStrings(endpoint.Produces), "\x01"),
		strings.Join(parameterKeys, "\x01"),
		strings.Join(catalogUniqueSortedStrings(endpoint.Limitations), "\x01"),
		strings.Join(catalogUniqueSortedStrings(endpoint.EvidenceIDs), "\x01"),
	}, "\x00")
}

func mergeProjectProviderEndpoint(target *APIEndpointRecord, extra APIEndpointRecord) {
	target.Parameters = mergeProjectProviderParameters(target.Parameters, extra.Parameters)
	target.Consumes = catalogUniqueSortedStrings(append(target.Consumes, extra.Consumes...))
	target.Produces = catalogUniqueSortedStrings(append(target.Produces, extra.Produces...))
	target.Security = mergeProjectProviderSecurity(target.Security, extra.Security)
	mergeProjectProviderType(&target.RequestType, extra.RequestType, "request_type", &target.Limitations)
	mergeProjectProviderType(&target.ResponseType, extra.ResponseType, "response_type", &target.Limitations)
	target.Confidence = strongerAPIConfidence(target.Confidence, extra.Confidence)
	mergeProjectProviderCoverage(&target.Coverage, extra.Coverage, &target.Limitations)
	target.Limitations = catalogUniqueSortedStrings(append(target.Limitations, extra.Limitations...))
	target.EvidenceIDs = catalogUniqueSortedStrings(append(target.EvidenceIDs, extra.EvidenceIDs...))
}

func mergeProjectProviderSecurity(target, extra []SecurityEvidenceRecord) []SecurityEvidenceRecord {
	merged := append(append([]SecurityEvidenceRecord(nil), target...), extra...)
	hasExplicit := false
	for _, record := range merged {
		if !placeholderUnknownSecurity(record) {
			hasExplicit = true
			break
		}
	}
	if hasExplicit {
		filtered := merged[:0]
		for _, record := range merged {
			if !placeholderUnknownSecurity(record) {
				filtered = append(filtered, record)
			}
		}
		merged = filtered
	}
	markConflictingProviderSecurity(merged)

	seen := map[string]bool{}
	unique := make([]SecurityEvidenceRecord, 0, len(merged))
	for _, record := range merged {
		sortSecurityEvidenceRecord(&record)
		key := securityEvidenceSortKey(record)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, record)
	}
	sort.Slice(unique, func(left, right int) bool {
		return securityEvidenceSortKey(unique[left]) < securityEvidenceSortKey(unique[right])
	})
	return unique
}

func placeholderUnknownSecurity(record SecurityEvidenceRecord) bool {
	return record.Kind == SecurityUnknown && record.Summary == "No auth evidence detected" && record.Expression == "" && record.Source == "" && record.File == "" && record.Line == 0
}

func mergeProjectProviderParameters(target, extra []APIParameterRecord) []APIParameterRecord {
	merged := append([]APIParameterRecord(nil), target...)
	indexByIdentity := map[string]int{}
	for index, parameter := range merged {
		indexByIdentity[parameter.Location+"\x00"+parameter.Name] = index
	}
	for _, parameter := range extra {
		identity := parameter.Location + "\x00" + parameter.Name
		if index, ok := indexByIdentity[identity]; ok {
			merged[index] = richerProjectProviderParameter(merged[index], parameter)
			continue
		}
		indexByIdentity[identity] = len(merged)
		merged = append(merged, parameter)
	}
	sort.Slice(merged, func(left, right int) bool {
		return apiParameterSortKey(merged[left]) < apiParameterSortKey(merged[right])
	})
	return merged
}

func richerProjectProviderParameter(left, right APIParameterRecord) APIParameterRecord {
	leftScore := projectProviderParameterRichness(left)
	rightScore := projectProviderParameterRichness(right)
	if rightScore > leftScore || (rightScore == leftScore && apiParameterSortKey(right) < apiParameterSortKey(left)) {
		return right
	}
	return left
}

func projectProviderParameterRichness(parameter APIParameterRecord) int {
	score := apiConfidenceRank(parameter.Confidence) * 1000
	if parameter.Type != "" {
		score += 100 + len(parameter.Type)
	}
	if parameter.Source != "" {
		score += 10
	}
	return score
}

func apiConfidenceRank(confidence Confidence) int {
	switch confidence {
	case ConfidenceExact:
		return 6
	case ConfidenceResolved:
		return 5
	case ConfidenceNormalized:
		return 4
	case ConfidenceInferred:
		return 3
	case ConfidenceWeak:
		return 2
	default:
		return 1
	}
}

func strongerAPIConfidence(left, right Confidence) Confidence {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	leftRank := apiConfidenceRank(left)
	rightRank := apiConfidenceRank(right)
	if rightRank > leftRank || (rightRank == leftRank && right < left) {
		return right
	}
	return left
}

func mergeProjectProviderCoverage(target *Coverage, extra Coverage, limitations *[]string) {
	if extra == "" {
		return
	}
	if *target == "" {
		*target = extra
		return
	}
	if *target == extra {
		return
	}
	values := catalogUniqueSortedStrings([]string{string(*target), string(extra)})
	*limitations = append(*limitations, "coverage_conflict: "+strings.Join(values, " | "))
	if apiCoverageRank(extra) < apiCoverageRank(*target) || (apiCoverageRank(extra) == apiCoverageRank(*target) && extra < *target) {
		*target = extra
	}
}

func apiCoverageRank(coverage Coverage) int {
	switch coverage {
	case CoverageComplete:
		return 4
	case CoveragePartial:
		return 3
	case CoverageUnavailable:
		return 2
	case CoverageFailed:
		return 1
	default:
		return 0
	}
}

func mergeProjectProviderType(target *string, extra, field string, limitations *[]string) {
	if extra == "" {
		return
	}
	if *target == "" {
		*target = extra
		return
	}
	if *target == extra {
		return
	}
	values := catalogUniqueSortedStrings([]string{*target, extra})
	*limitations = append(*limitations, field+"_conflict: "+strings.Join(values, " | "))
	if len(extra) > len(*target) || (len(extra) == len(*target) && extra < *target) {
		*target = extra
	}
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
		record.Required = annotation.Attributes["required"] != "false" && !springParameterHasDefault(annotation)
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

func springParameterHasDefault(annotation JavaAnnotationRecord) bool {
	value, present := springParameterDefaultExpression(annotation)
	if !present {
		return false
	}
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return true
	}
	return !springDefaultNoneExpression(value)
}

func springParameterDefaultExpression(annotation JavaAnnotationRecord) (string, bool) {
	for _, part := range splitTopLevel(annotation.Arguments, ',') {
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) == 2 && strings.TrimSpace(keyValue[0]) == "defaultValue" {
			return strings.TrimSpace(keyValue[1]), true
		}
	}
	value, present := annotation.Attributes["defaultValue"]
	return value, present
}

func springDefaultNoneExpression(value string) bool {
	return value == "DEFAULT_NONE" || value == "ValueConstants.DEFAULT_NONE" || strings.HasSuffix(value, ".ValueConstants.DEFAULT_NONE")
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
	constants := springConstantIndex(sources)
	securitySchemes := openAPISecuritySchemes(sources)
	for _, source := range sources {
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
			index.Endpoints = append(index.Endpoints, springEndpointsForMethod(source, method, constants, securitySchemes)...)
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
	applyScopedSpringAuth(&index, springAuthScopes(sources, constants))
	sortSpringIndex(&index)
	return index
}

func springConstantIndex(sources []JavaSourceRecord) map[string]string {
	type constantDeclaration struct {
		name      string
		owner     string
		qualified string
		value     string
	}
	ordered := append([]JavaSourceRecord(nil), sources...)
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].File != ordered[right].File {
			return ordered[left].File < ordered[right].File
		}
		return ordered[left].Package < ordered[right].Package
	})

	declarations := make([]constantDeclaration, 0)
	seen := map[string]bool{}
	for _, source := range ordered {
		fields := append([]JavaFieldRecord(nil), source.Fields...)
		sort.Slice(fields, func(left, right int) bool {
			if fields[left].Owner != fields[right].Owner {
				return fields[left].Owner < fields[right].Owner
			}
			return fields[left].Name < fields[right].Name
		})
		for _, field := range fields {
			value, ok := source.Constants[field.Name]
			if !ok || strings.TrimSpace(field.Owner) == "" {
				continue
			}
			qualified := field.Owner + "." + field.Name
			if source.Package != "" {
				qualified = source.Package + "." + qualified
			}
			key := qualified + "\x00" + value
			if seen[key] {
				continue
			}
			seen[key] = true
			declarations = append(declarations, constantDeclaration{
				name: field.Name, owner: field.Owner, qualified: qualified, value: value,
			})
		}
	}
	sort.Slice(declarations, func(left, right int) bool {
		if declarations[left].qualified != declarations[right].qualified {
			return declarations[left].qualified < declarations[right].qualified
		}
		return declarations[left].value < declarations[right].value
	})

	nameCounts := map[string]int{}
	for _, declaration := range declarations {
		nameCounts[declaration.name]++
	}
	result := map[string]string{}
	for _, declaration := range declarations {
		result[declaration.owner+"."+declaration.name] = declaration.value
		result[declaration.qualified] = declaration.value
		if nameCounts[declaration.name] == 1 {
			result[declaration.name] = declaration.value
		}
	}
	return result
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

func springEndpointsForMethod(source JavaSourceRecord, method JavaMethodRecord, constants map[string]string, securitySchemes map[string][]AuthRecord) []SpringEndpointRecord {
	controller := ""
	classPath := ""
	var controllerAnnotations []JavaAnnotationRecord
	for _, typ := range source.Types {
		if typ.Name == method.Owner && hasAnnotation(typ.Annotations, "RestController") {
			controller = typ.Name
			classPath = resolveSpringPath(firstMappingAnnotation(typ.Annotations), constants)
			controllerAnnotations = typ.Annotations
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
				Produces:    springProduces(annotation, constants),
				ReturnType:  returnType,
				Parameters:  method.Parameters,
				Auth:        springAuthRecords(controllerAnnotations, method.Annotations, method.File, securitySchemes),
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

func springAuthRecords(classAnnotations, methodAnnotations []JavaAnnotationRecord, file string, securitySchemes map[string][]AuthRecord) []AuthRecord {
	overriddenFamilies := map[string]bool{}
	for _, annotation := range methodAnnotations {
		if family := springMethodSecurityFamily(annotation.Name); family != "" {
			overriddenFamilies[family] = true
		}
	}
	classSecurityAnnotations := make([]JavaAnnotationRecord, 0, len(classAnnotations))
	for _, annotation := range classAnnotations {
		family := springMethodSecurityFamily(annotation.Name)
		if family != "" && !overriddenFamilies[family] {
			classSecurityAnnotations = append(classSecurityAnnotations, annotation)
		}
	}
	records := springMethodSecurityAuthRecords(classSecurityAnnotations, file, "class_annotation")
	records = append(records, springMethodSecurityAuthRecords(methodAnnotations, file, "method_annotation")...)

	openAPIAnnotations := classAnnotations
	if hasMethodOpenAPISecurityOverride(methodAnnotations) {
		openAPIAnnotations = methodAnnotations
	}
	records = append(records, springOpenAPIAuthRecords(openAPIAnnotations, file, securitySchemes)...)
	sort.Slice(records, func(left, right int) bool {
		return authRecordSortKey(records[left]) < authRecordSortKey(records[right])
	})
	return records
}

func springMethodSecurityFamily(annotation string) string {
	switch annotation {
	case "PreAuthorize":
		return "pre_authorize"
	case "PostAuthorize":
		return "post_authorize"
	case "Secured":
		return "secured"
	case "PermitAll", "DenyAll", "RolesAllowed":
		return "jsr_250"
	default:
		return ""
	}
}

func springMethodSecurityAuthRecords(annotations []JavaAnnotationRecord, file, source string) []AuthRecord {
	var records []AuthRecord
	for _, annotation := range annotations {
		switch annotation.Name {
		case "PermitAll", "DenyAll":
			records = append(records, AuthRecord{
				Kind:       toSnake(annotation.Name),
				Source:     source,
				Confidence: "EXTRACTED",
				File:       file,
				Line:       annotation.Line,
			})
		case "PreAuthorize", "PostAuthorize":
			records = append(records, AuthRecord{
				Kind:       toSnake(annotation.Name),
				Expression: firstNonEmpty(annotation.Attributes["value"], annotation.Arguments),
				Source:     source,
				Confidence: "EXTRACTED",
				File:       file,
				Line:       annotation.Line,
			})
		case "Secured", "RolesAllowed":
			records = append(records, AuthRecord{
				Kind:       toSnake(annotation.Name),
				Expression: firstNonEmpty(annotation.Attributes["value"], annotation.Arguments),
				Source:     source,
				Confidence: "EXTRACTED",
				File:       file,
				Line:       annotation.Line,
			})
		}
	}
	return records
}

func springOpenAPIAuthRecords(annotations []JavaAnnotationRecord, file string, securitySchemes map[string][]AuthRecord) []AuthRecord {
	var records []AuthRecord
	for _, annotation := range annotations {
		names := openAPISecurityRequirementNames(annotation)
		for _, name := range names {
			for _, scheme := range securitySchemes[name] {
				scheme.Expression = name
				records = append(records, scheme)
			}
		}
		if len(names) == 0 && explicitEmptyOpenAPISecurity(annotation) {
			records = append(records, AuthRecord{
				Kind:       "permit_all",
				Expression: strings.TrimSpace(annotation.Arguments),
				Source:     "openapi_security_override",
				Confidence: "EXTRACTED",
				File:       file,
				Line:       annotation.Line,
			})
		}
	}
	return records
}

func hasMethodOpenAPISecurityOverride(annotations []JavaAnnotationRecord) bool {
	for _, annotation := range annotations {
		if annotation.Name == "SecurityRequirement" || annotation.Name == "SecurityRequirements" {
			return true
		}
		if annotation.Name == "Operation" {
			if _, ok := annotation.Attributes["security"]; ok {
				return true
			}
		}
	}
	return false
}

func explicitEmptyOpenAPISecurity(annotation JavaAnnotationRecord) bool {
	if annotation.Name == "SecurityRequirement" {
		return strings.TrimSpace(annotation.Attributes["name"]) == ""
	}
	if annotation.Name == "SecurityRequirements" {
		if value, ok := annotation.Attributes["value"]; ok {
			return emptyOpenAPIContainerExpression(value)
		}
		return emptyOpenAPIContainerExpression(annotation.Arguments)
	}
	if annotation.Name != "Operation" {
		return false
	}
	security, ok := annotation.Attributes["security"]
	if !ok {
		return false
	}
	security = strings.TrimSpace(security)
	return security == "@SecurityRequirement()" || security == "@SecurityRequirement" || emptyOpenAPIContainerExpression(security)
}

func emptyOpenAPIContainerExpression(expression string) bool {
	expression = strings.TrimSpace(expression)
	if equals := strings.IndexByte(expression, '='); equals >= 0 {
		if strings.TrimSpace(expression[:equals]) != "value" {
			return false
		}
		expression = strings.TrimSpace(expression[equals+1:])
	}
	if expression == "" {
		return true
	}
	if !strings.HasPrefix(expression, "{") || !strings.HasSuffix(expression, "}") {
		return false
	}
	member := strings.TrimSpace(expression[1 : len(expression)-1])
	return member == "" || member == "@SecurityRequirement()" || member == "@SecurityRequirement"
}

func openAPISecuritySchemes(sources []JavaSourceRecord) map[string][]AuthRecord {
	schemes := map[string][]AuthRecord{}
	for _, source := range sources {
		for _, annotation := range source.Annotations {
			if annotation.Name != "SecurityScheme" {
				continue
			}
			name := strings.TrimSpace(annotation.Attributes["name"])
			kind := explicitOpenAPISecurityKind(annotation)
			if name == "" || kind == "" {
				continue
			}
			record := AuthRecord{Kind: kind, Expression: name, Source: "openapi_security_scheme", Confidence: "EXTRACTED", File: source.File, Line: annotation.Line}
			schemes[name] = append(schemes[name], record)
		}
	}
	for name := range schemes {
		sort.Slice(schemes[name], func(left, right int) bool {
			return authRecordSortKey(schemes[name][left]) < authRecordSortKey(schemes[name][right])
		})
	}
	return schemes
}

func explicitOpenAPISecurityKind(annotation JavaAnnotationRecord) string {
	typeName := strings.ToUpper(shortJavaName(annotation.Attributes["type"]))
	scheme := strings.ToLower(strings.TrimSpace(annotation.Attributes["scheme"]))
	switch typeName {
	case "APIKEY":
		return "api_key"
	case "OAUTH2":
		return "oauth2"
	case "OPENIDCONNECT":
		return "openid_connect"
	case "MUTUALTLS":
		return "mutual_tls"
	case "HTTP":
		switch scheme {
		case "basic":
			return "http_basic"
		case "bearer":
			return "bearer"
		}
	}
	return ""
}

func openAPISecurityRequirementNames(annotation JavaAnnotationRecord) []string {
	if annotation.Name == "SecurityRequirement" {
		if name := strings.TrimSpace(annotation.Attributes["name"]); name != "" {
			return []string{name}
		}
		return nil
	}
	var result []string
	arguments := annotation.Arguments
	for offset := 0; offset < len(arguments); {
		at := strings.IndexByte(arguments[offset:], '@')
		if at < 0 {
			break
		}
		at += offset
		nameEnd := at + 1
		for nameEnd < len(arguments) {
			char := arguments[nameEnd]
			if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '.' {
				nameEnd++
				continue
			}
			break
		}
		open := nameEnd
		for open < len(arguments) && (arguments[open] == ' ' || arguments[open] == '\t' || arguments[open] == '\r' || arguments[open] == '\n') {
			open++
		}
		if shortJavaName(arguments[at+1:nameEnd]) != "SecurityRequirement" || open >= len(arguments) || arguments[open] != '(' {
			offset = nameEnd
			continue
		}
		close := matchingJavaParen(arguments, open)
		if close < 0 {
			break
		}
		attributes := parseJavaAnnotationAttributes(arguments[open+1 : close])
		if name := strings.TrimSpace(attributes["name"]); name != "" {
			result = append(result, name)
		}
		offset = close + 1
	}
	return result
}

type springAuthScope struct {
	Paths      []string
	Auth       []AuthRecord
	Order      int
	File       string
	Line       int
	Confidence string
}

func springAuthScopes(sources []JavaSourceRecord, constants map[string]string) []springAuthScope {
	var scopes []springAuthScope
	for _, source := range sources {
		if !springSecurityProductionSource(source) {
			continue
		}
		for _, method := range source.Methods {
			if !springSecurityConfigurationMethod(source, method) {
				continue
			}
			scope := springAuthScope{
				Auth:       append([]AuthRecord(nil), method.Auth...),
				Order:      springSecurityOrder(method.Annotations),
				File:       method.File,
				Line:       method.Line,
				Confidence: "EXACT",
			}
			hasMatcher := false
			unresolvedMatcher := false
			for _, call := range method.Calls {
				if call.Method != "securityMatcher" {
					continue
				}
				hasMatcher = true
				if len(call.Arguments) == 0 {
					unresolvedMatcher = true
					continue
				}
				for _, argument := range call.Arguments {
					alternatives := javaResolvedPathAlternatives(argument, constants, nil, nil, 0)
					if len(alternatives) == 0 {
						unresolvedMatcher = true
						continue
					}
					for _, alternative := range alternatives {
						path, ok := springSecurityMatcherPath(alternative)
						if !ok {
							unresolvedMatcher = true
							continue
						}
						scope.Paths = append(scope.Paths, path)
					}
				}
			}
			scope.Paths = catalogUniqueSortedStrings(scope.Paths)
			if hasMatcher && len(scope.Paths) == 0 || unresolvedMatcher {
				scope.Confidence = "PARTIAL"
			}
			sort.Slice(scope.Auth, func(left, right int) bool {
				return authRecordSortKey(scope.Auth[left]) < authRecordSortKey(scope.Auth[right])
			})
			if len(scope.Auth) > 0 {
				scopes = append(scopes, scope)
			}
		}
	}
	sort.Slice(scopes, func(left, right int) bool {
		if scopes[left].Order != scopes[right].Order {
			return scopes[left].Order < scopes[right].Order
		}
		if strings.Join(scopes[left].Paths, "\x00") != strings.Join(scopes[right].Paths, "\x00") {
			return strings.Join(scopes[left].Paths, "\x00") < strings.Join(scopes[right].Paths, "\x00")
		}
		if scopes[left].File != scopes[right].File {
			return scopes[left].File < scopes[right].File
		}
		return scopes[left].Line < scopes[right].Line
	})
	return scopes
}

func springSecurityOrder(annotations []JavaAnnotationRecord) int {
	order := int(^uint(0) >> 1)
	for _, annotation := range annotations {
		if annotation.Name != "Order" {
			continue
		}
		value := firstNonEmpty(annotation.Attributes["value"], annotation.Arguments)
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return order
}

func springSecurityMatcherPath(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") {
		return "", false
	}
	if strings.Contains(strings.TrimSuffix(value, "/**"), "*") {
		return "", false
	}
	if strings.HasSuffix(value, "/**") {
		prefix := strings.TrimSuffix(value, "/**")
		if prefix == "" {
			return "/**", true
		}
		return strings.TrimSuffix(normalizeSpringPath(prefix), "/") + "/**", true
	}
	return normalizeSpringPath(value), true
}

func springGlobalAuthRecords(sources []JavaSourceRecord) []AuthRecord {
	seen := map[string]bool{}
	var records []AuthRecord
	for _, scope := range springAuthScopes(sources, springConstantIndex(sources)) {
		for _, auth := range scope.Auth {
			key := authRecordSortKey(auth)
			if seen[key] {
				continue
			}
			seen[key] = true
			records = append(records, auth)
		}
	}
	sort.Slice(records, func(left, right int) bool {
		return authRecordSortKey(records[left]) < authRecordSortKey(records[right])
	})
	return records
}

func springSecurityProductionSource(source JavaSourceRecord) bool {
	file := "/" + strings.TrimPrefix(filepath.ToSlash(source.File), "/")
	return !strings.Contains(file, "/src/test/")
}

func springSecurityConfigurationMethod(source JavaSourceRecord, method JavaMethodRecord) bool {
	if shortJavaName(method.ReturnType) == "SecurityFilterChain" && importsJavaType(source, "org.springframework.security.web.SecurityFilterChain") {
		return true
	}
	for _, parameter := range method.Parameters {
		if shortJavaName(parameter.Type) == "HttpSecurity" && importsJavaType(source, "org.springframework.security.config.annotation.web.builders.HttpSecurity") {
			return true
		}
	}
	return false
}

func importsJavaType(source JavaSourceRecord, qualifiedName string) bool {
	packageName := qualifiedName[:strings.LastIndex(qualifiedName, ".")]
	for _, imported := range source.Imports {
		if imported.Name == qualifiedName || imported.Name == packageName+".*" {
			return true
		}
	}
	return false
}

func authRecordSortKey(record AuthRecord) string {
	return strings.Join([]string{record.Kind, record.Expression, record.Source, record.Confidence, filepath.ToSlash(record.File), strconv.Itoa(record.Line)}, "\x00")
}

func applyScopedSpringAuth(index *SpringIndex, scopes []springAuthScope) {
	if len(scopes) == 0 {
		return
	}
	for endpointIndex := range index.Endpoints {
		selected, partial := matchingSpringAuthScopes(index.Endpoints[endpointIndex].Path, scopes)
		var scoped []AuthRecord
		for _, scope := range selected {
			for _, auth := range scope.Auth {
				if partial || scope.Confidence == "PARTIAL" {
					auth.Confidence = "PARTIAL"
				}
				scoped = append(scoped, auth)
			}
		}
		index.Endpoints[endpointIndex].Auth = mergeSpringAuthRecords(
			index.Endpoints[endpointIndex].Auth,
			scoped,
		)
	}
}

func matchingSpringAuthScopes(path string, scopes []springAuthScope) ([]springAuthScope, bool) {
	type scopeMatch struct {
		scope       springAuthScope
		specificity int
	}
	var matches []scopeMatch
	for _, scope := range scopes {
		best := -1
		for _, matcher := range scope.Paths {
			if specificity := springSecurityMatcherSpecificity(path, matcher); specificity > best {
				best = specificity
			}
		}
		if best >= 0 {
			matches = append(matches, scopeMatch{scope: scope, specificity: best})
		}
	}
	if len(matches) == 0 {
		for _, scope := range scopes {
			if len(scope.Paths) == 0 && scope.Confidence != "PARTIAL" {
				matches = append(matches, scopeMatch{scope: scope})
			}
		}
	}
	if len(matches) == 0 {
		for _, scope := range scopes {
			if len(scope.Paths) == 0 && scope.Confidence == "PARTIAL" {
				matches = append(matches, scopeMatch{scope: scope})
			}
		}
	}
	if len(matches) == 0 {
		return nil, false
	}

	bestOrder := matches[0].scope.Order
	bestSpecificity := matches[0].specificity
	for _, match := range matches[1:] {
		if match.scope.Order < bestOrder ||
			match.scope.Order == bestOrder && match.specificity > bestSpecificity {
			bestOrder = match.scope.Order
			bestSpecificity = match.specificity
		}
	}
	var selected []springAuthScope
	for _, match := range matches {
		if match.scope.Order == bestOrder && match.specificity == bestSpecificity {
			selected = append(selected, match.scope)
		}
	}
	return selected, springAuthScopesConflict(selected)
}

func springSecurityMatcherSpecificity(path, matcher string) int {
	path = normalizeSpringPath(path)
	matcher = normalizeSpringPath(matcher)
	if strings.HasSuffix(matcher, "/**") {
		prefix := strings.TrimSuffix(matcher, "/**")
		if prefix == "" || path == prefix || strings.HasPrefix(path, prefix+"/") {
			return len(prefix)
		}
		return -1
	}
	if path == matcher {
		return len(matcher) + 1
	}
	return -1
}

func springAuthScopesConflict(scopes []springAuthScope) bool {
	if len(scopes) < 2 {
		return false
	}
	first := springAuthPolicyKey(scopes[0].Auth)
	for _, scope := range scopes[1:] {
		if springAuthPolicyKey(scope.Auth) != first {
			return true
		}
	}
	return false
}

func springAuthPolicyKey(records []AuthRecord) string {
	parts := make([]string, 0, len(records))
	for _, record := range records {
		parts = append(parts, record.Kind+"\x00"+record.Expression)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\x01")
}

func mergeSpringAuthRecords(existing, scoped []AuthRecord) []AuthRecord {
	records := append(append([]AuthRecord(nil), existing...), scoped...)
	seen := map[string]bool{}
	result := make([]AuthRecord, 0, len(records))
	for _, record := range records {
		key := authRecordSortKey(record)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, record)
	}
	sort.Slice(result, func(left, right int) bool {
		return authRecordSortKey(result[left]) < authRecordSortKey(result[right])
	})
	return result
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
	return springMappingMediaTypes(annotation, "consumes", constants)
}

func springProduces(annotation JavaAnnotationRecord, constants map[string]string) string {
	return springMappingMediaTypes(annotation, "produces", constants)
}

func springMappingMediaTypes(annotation JavaAnnotationRecord, attribute string, constants map[string]string) string {
	value := annotation.Attributes[attribute]
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
	for endpointIndex := range index.Endpoints {
		sort.Slice(index.Endpoints[endpointIndex].Auth, func(left, right int) bool {
			return authRecordSortKey(index.Endpoints[endpointIndex].Auth[left]) < authRecordSortKey(index.Endpoints[endpointIndex].Auth[right])
		})
	}
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
