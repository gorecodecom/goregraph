package scan

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// APISpecificationRecord describes a document, independently of executable routes.
type APISpecificationRecord struct {
	File         string                      `json:"file"`
	Repository   string                      `json:"repository,omitempty"`
	OwnerProject string                      `json:"owner_project,omitempty"`
	SourceHash   string                      `json:"source_hash"`
	Version      string                      `json:"version,omitempty"`
	Line         int                         `json:"line,omitempty"`
	Status       string                      `json:"status"`
	Limitations  []string                    `json:"limitations,omitempty"`
	Operations   []APISpecificationOperation `json:"operations,omitempty"`
	Models       []APISpecificationModel     `json:"models,omitempty"`
}

// APISpecificationOperation retains contract and matching code locations separately.
type APISpecificationOperation struct {
	Method        string                  `json:"method"`
	Path          string                  `json:"path"`
	ResolvedPaths []string                `json:"resolved_paths,omitempty"`
	Line          int                     `json:"line"`
	OperationID   string                  `json:"operation_id,omitempty"`
	Models        []string                `json:"models,omitempty"`
	LinkStatus    string                  `json:"link_status"`
	MatchBasis    string                  `json:"match_basis,omitempty"`
	CodeRoutes    []APISpecificationRoute `json:"code_routes,omitempty"`
	basePaths     []string
}

// APISpecificationRoute is an indexed code route compatible with a contract.
type APISpecificationRoute struct {
	Project string `json:"project"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Path    string `json:"path"`
}

// APISpecificationModel describes named schemas and directly declared fields.
type APISpecificationModel struct {
	Name   string                  `json:"name"`
	Line   int                     `json:"line"`
	Fields []APISpecificationField `json:"fields,omitempty"`
}

// APISpecificationField deliberately omits examples, defaults and other values.
type APISpecificationField struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Model    string `json:"model,omitempty"`
	Line     int    `json:"line"`
	Required *bool  `json:"required,omitempty"`
}

var apiSpecificationHeader = regexp.MustCompile(`(?m)(?:^|[{,])\s*['"]?(?:swagger|openapi)['"]?\s*:`)
var openAPISpecificationVersion = regexp.MustCompile(`^3\.[012]\.[0-9]+$`)
var numericAPISpecificationVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?$`)

func parseAPISpecification(file string, body []byte) (APISpecificationRecord, bool) {
	record := APISpecificationRecord{File: file, Status: "invalid"}
	named := strings.Contains(strings.ToLower(path.Base(file)), "swagger") || strings.Contains(strings.ToLower(path.Base(file)), "openapi")
	if !named && !apiSpecificationHeader.Match(body) {
		return record, false
	}
	invalid := func() (APISpecificationRecord, bool) {
		record.Status, record.Operations, record.Models = "invalid", nil, nil
		record.Limitations = []string{"invalid_document"}
		return record, true
	}
	if strings.HasSuffix(strings.ToLower(file), ".json") && !json.Valid(body) {
		return invalid()
	}
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	var document, trailing yaml.Node
	if decoder.Decode(&document) != nil || decoder.Decode(&trailing) != io.EOF || len(document.Content) != 1 {
		return invalid()
	}
	root := document.Content[0]
	version := specificationValue(root, "swagger")
	swagger := version != nil
	if version == nil {
		version = specificationValue(root, "openapi")
	} else if specificationValue(root, "openapi") != nil {
		return invalid()
	}
	if version == nil {
		if named {
			return invalid()
		}
		return record, false
	}
	count := 0
	if !validateSpecificationNode(root, 0, &count, &record) || version.Kind != yaml.ScalarNode {
		return invalid()
	}
	record.Version, record.Line = version.Value, version.Line
	if swagger && record.Version != "2.0" || !swagger && !openAPISpecificationVersion.MatchString(record.Version) {
		if !numericAPISpecificationVersion.MatchString(record.Version) {
			if !named {
				return APISpecificationRecord{}, false
			}
			record.Version = ""
		}
		record.Status, record.Limitations = "unsupported", []string{"unsupported_version"}
		return record, true
	}
	if strings.HasPrefix(record.Version, "3.2.") {
		specificationLimitation(&record, "additional_operations_not_evaluated")
	}
	paths := specificationValue(root, "paths")
	if paths != nil && paths.Kind == yaml.AliasNode {
		record.Status = "partial"
		return record, true
	}
	if paths != nil && paths.Kind != yaml.MappingNode {
		return invalid()
	}
	if paths == nil && (record.Version == "2.0" || strings.HasPrefix(record.Version, "3.0.")) {
		return invalid()
	}
	if specificationValue(root, "webhooks") != nil {
		specificationLimitation(&record, "webhooks_not_evaluated")
	}
	for i := 0; paths != nil && i+1 < len(paths.Content); i += 2 {
		key, item := paths.Content[i], paths.Content[i+1]
		if strings.HasPrefix(key.Value, "x-") {
			continue
		}
		if !strings.HasPrefix(key.Value, "/") || item.Kind != yaml.MappingNode {
			return invalid()
		}
		if specificationValue(item, "$ref") != nil {
			// Path-item references can have conflicting siblings; do not guess a merge.
			specificationLimitation(&record, "path_references_not_evaluated")
			continue
		}
		for j := 0; j+1 < len(item.Content); j += 2 {
			method, operation := item.Content[j], item.Content[j+1]
			if !specificationHTTPMethod(method.Value) {
				continue
			}
			if operation.Kind != yaml.MappingNode {
				return invalid()
			}
			bases := specificationBasePaths(root, item, operation, &record)
			var resolvedPaths []string
			for _, base := range bases {
				resolvedPaths = append(resolvedPaths, base+key.Value)
			}
			record.Operations = append(record.Operations, APISpecificationOperation{
				Method: strings.ToUpper(method.Value), Path: key.Value, Line: method.Line,
				ResolvedPaths: resolvedPaths,
				OperationID:   specificationScalar(operation, "operationId"), LinkStatus: "unlinked",
				Models:    specificationModelReferences(operation),
				basePaths: bases,
			})
		}
	}
	schemas := specificationValue(root, "definitions")
	if record.Version != "2.0" {
		schemas = specificationValue(specificationValue(root, "components"), "schemas")
	}
	if schemas != nil && schemas.Kind == yaml.AliasNode {
		schemas = nil
	}
	if schemas != nil && schemas.Kind != yaml.MappingNode {
		return invalid()
	}
	for i := 0; schemas != nil && i+1 < len(schemas.Content); i += 2 {
		name, schema := schemas.Content[i], schemas.Content[i+1]
		model := APISpecificationModel{Name: name.Value, Line: name.Line}
		required := map[string]bool{}
		if node := specificationValue(schema, "required"); node != nil && node.Kind == yaml.SequenceNode {
			for _, field := range node.Content {
				required[field.Value] = true
			}
		}
		properties := specificationValue(schema, "properties")
		for j := 0; properties != nil && properties.Kind == yaml.MappingNode && j+1 < len(properties.Content); j += 2 {
			field, definition := properties.Content[j], properties.Content[j+1]
			var requiredState *bool
			requiredValue := required[field.Value]
			requiredNode := specificationValue(schema, "required")
			unknownRequired := requiredNode != nil && requiredNode.Kind != yaml.SequenceNode || specificationValue(schema, "allOf") != nil || specificationValue(schema, "anyOf") != nil || specificationValue(schema, "oneOf") != nil
			if requiredValue || !unknownRequired {
				requiredState = &requiredValue
			}
			model.Fields = append(model.Fields, APISpecificationField{
				Name: field.Value, Type: specificationScalar(definition, "type"), Line: field.Line,
				Model: specificationLocalModel(specificationScalar(definition, "$ref")), Required: requiredState,
			})
		}
		record.Models = append(record.Models, model)
	}
	sort.Slice(record.Operations, func(i, j int) bool {
		return record.Operations[i].Path+record.Operations[i].Method < record.Operations[j].Path+record.Operations[j].Method
	})
	sort.Slice(record.Models, func(i, j int) bool { return record.Models[i].Name < record.Models[j].Name })
	record.Status = "parsed"
	if len(record.Limitations) > 0 {
		record.Status = "partial"
	}
	return record, true
}

func validateSpecificationNode(node *yaml.Node, depth int, count *int, record *APISpecificationRecord) bool {
	*count = *count + 1
	if depth > 100 || *count > 50000 {
		return false
	}
	if node.Kind == yaml.AliasNode {
		specificationLimitation(record, "aliases_not_evaluated")
		return true
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Kind != yaml.ScalarNode || seen[key.Value] {
				return false
			}
			seen[key.Value] = true
			if key.Value == "<<" {
				specificationLimitation(record, "aliases_not_evaluated")
			}
			if key.Value == "$ref" && !strings.HasPrefix(value.Value, "#/") {
				specificationLimitation(record, "external_references_not_evaluated")
			}
			if key.Value == "allOf" || key.Value == "oneOf" || key.Value == "anyOf" {
				specificationLimitation(record, "composed_models_not_evaluated")
			}
		}
	}
	for _, child := range node.Content {
		if !validateSpecificationNode(child, depth+1, count, record) {
			return false
		}
	}
	return true
}

func specificationValue(node *yaml.Node, name string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == name {
			return node.Content[i+1]
		}
	}
	return nil
}

func specificationScalar(node *yaml.Node, name string) string {
	value := specificationValue(node, name)
	if value != nil && value.Kind == yaml.ScalarNode {
		return value.Value
	}
	return ""
}

func specificationLimitation(record *APISpecificationRecord, code string) {
	for _, existing := range record.Limitations {
		if existing == code {
			return
		}
	}
	record.Limitations = append(record.Limitations, code)
}

func specificationHTTPMethod(method string) bool {
	switch method {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return true
	}
	return false
}

func specificationBasePaths(root, item, operation *yaml.Node, record *APISpecificationRecord) []string {
	if record.Version == "2.0" {
		base := specificationScalar(root, "basePath")
		if base == "" {
			return []string{""}
		}
		if !strings.HasPrefix(base, "/") || strings.ContainsAny(base, "{}?#") {
			specificationLimitation(record, "server_path_unresolved")
			return nil
		}
		return []string{strings.TrimSuffix(base, "/")}
	}
	servers := specificationValue(operation, "servers")
	if servers == nil {
		servers = specificationValue(item, "servers")
	}
	if servers == nil {
		servers = specificationValue(root, "servers")
	}
	if servers == nil || servers.Kind == yaml.SequenceNode && len(servers.Content) == 0 {
		return []string{""}
	}
	if servers.Kind != yaml.SequenceNode {
		specificationLimitation(record, "server_path_unresolved")
		return nil
	}
	var bases []string
	for _, server := range servers.Content {
		raw := specificationScalar(server, "url")
		parsed, err := url.Parse(raw)
		if raw == "" || err != nil || strings.ContainsAny(raw, "{}") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && !strings.HasPrefix(parsed.Path, "/") {
			specificationLimitation(record, "server_path_unresolved")
			return nil
		}
		bases = append(bases, strings.TrimSuffix(parsed.EscapedPath(), "/"))
	}
	return sortedUniqueStrings(bases)
}

func specificationLocalModel(ref string) string {
	for _, prefix := range []string{"#/definitions/", "#/components/schemas/"} {
		if strings.HasPrefix(ref, prefix) {
			name := strings.TrimPrefix(ref, prefix)
			if !strings.Contains(name, "/") {
				return strings.ReplaceAll(strings.ReplaceAll(name, "~1", "/"), "~0", "~")
			}
		}
	}
	return ""
}

func specificationModelReferences(node *yaml.Node) []string {
	var models []string
	var visit func(*yaml.Node)
	visit = func(current *yaml.Node) {
		if current.Kind == yaml.AliasNode {
			return
		}
		if model := specificationLocalModel(specificationScalar(current, "$ref")); model != "" {
			models = append(models, model)
		}
		for _, child := range current.Content {
			visit(child)
		}
	}
	visit(node)
	return sortedUniqueStrings(models)
}
