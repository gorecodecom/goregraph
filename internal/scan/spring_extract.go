package scan

import (
	"regexp"
	"sort"
	"strings"
)

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
			endpoints = append(endpoints, SpringEndpointRecord{
				HTTPMethod:  httpMethod,
				Path:        joinSpringPaths(classPath, methodPath),
				Controller:  controller,
				Method:      method.Name,
				File:        method.File,
				Line:        annotation.Line,
				RequestType: requestBodyType(method.Parameters),
				ReturnType:  method.ReturnType,
				Parameters:  method.Parameters,
			})
		}
	}
	return endpoints
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
	path = strings.TrimPrefix(path, "{")
	path = strings.TrimSuffix(path, "}")
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

func requestBodyType(params []JavaParameterRecord) string {
	for _, param := range params {
		if hasAnnotation(param.Annotations, "RequestBody") {
			return param.Type
		}
	}
	return ""
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
