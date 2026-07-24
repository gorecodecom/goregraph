package scan

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"
)

type AgentContextFactRecord struct {
	ID          string   `json:"id"`
	Project     string   `json:"project,omitempty"`
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Qualified   string   `json:"qualified,omitempty"`
	HTTPMethod  string   `json:"http_method,omitempty"`
	Path        string   `json:"path,omitempty"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	EndLine     int      `json:"end_line,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
	Search      string   `json:"search,omitempty"`
}

type AgentContextEdgeRecord struct {
	ID          string   `json:"id"`
	Project     string   `json:"project,omitempty"`
	FromFactID  string   `json:"from_fact_id,omitempty"`
	ToFactID    string   `json:"to_fact_id,omitempty"`
	FromLabel   string   `json:"from_label,omitempty"`
	ToLabel     string   `json:"to_label,omitempty"`
	Kind        string   `json:"kind"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type AgentContextCoverageRecord struct {
	Project    string `json:"project,omitempty"`
	Capability string `json:"capability"`
	Coverage   string `json:"coverage"`
	Reason     string `json:"reason"`
}

type AgentContextIndexRecord struct {
	SchemaVersion int                          `json:"schema_version"`
	Generated     string                       `json:"generated,omitempty"`
	Root          string                       `json:"root,omitempty"`
	Facts         []AgentContextFactRecord     `json:"facts"`
	Edges         []AgentContextEdgeRecord     `json:"edges"`
	Coverage      []AgentContextCoverageRecord `json:"coverage,omitempty"`
}

const (
	maxCatalogConsumersPerService = 5
	maxCatalogContextValueRunes   = 160
	maxCatalogFactEvidenceIDs     = 8
)

type compactCatalogConsumerSelection struct {
	consumer APIConsumerRecord
	service  string
}

func compactCatalogEndpointKey(endpoint APIEndpointRecord) string {
	return strings.Join([]string{
		strings.TrimSpace(endpoint.ID),
		contextPathKey(endpoint.ProviderProject),
		strings.ToUpper(strings.TrimSpace(endpoint.HTTPMethod)),
		strings.TrimSpace(endpoint.Path),
		contextPathKey(endpoint.File),
		fmt.Sprint(endpoint.Line),
	}, "\x00")
}

func compactCatalogSecurityKey(security SecurityEvidenceRecord) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(security.Kind)),
		strings.TrimSpace(security.Source),
		contextPathKey(security.File),
		fmt.Sprint(security.Line),
		fmt.Sprint(security.Conflicting),
		string(security.Confidence),
	}, "\x00")
}

func compactCatalogConsumerKey(consumer APIConsumerRecord) string {
	return strings.Join([]string{
		contextPathKey(consumer.Project),
		strings.TrimSpace(consumer.Service),
		contextPathKey(consumer.File),
		fmt.Sprint(consumer.Line),
		strings.TrimSpace(consumer.Caller),
		strings.TrimSpace(consumer.ID),
	}, "\x00")
}

func compactCatalogValue(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= maxCatalogContextValueRunes {
		return value
	}
	return string(runes[:maxCatalogContextValueRunes-3]) + "..."
}

func compactCatalogSecurityLabels(security []SecurityEvidenceRecord) []string {
	labels := make([]string, 0, len(security))
	for _, evidence := range security {
		label := strings.ToLower(strings.TrimSpace(evidence.Kind))
		if label != "" {
			labels = append(labels, compactCatalogValue(label))
		}
	}
	sort.Slice(labels, func(i, j int) bool {
		left := strings.ToLower(labels[i])
		right := strings.ToLower(labels[j])
		if left != right {
			return left < right
		}
		return labels[i] < labels[j]
	})
	return orderedContextStrings(labels)
}

func compactCatalogConsumerAuthLabels(security []SecurityEvidenceRecord) []string {
	labels := make([]string, 0, len(security))
	for _, evidence := range security {
		kind := strings.ToLower(strings.TrimSpace(evidence.Kind))
		if compactCatalogConsumerAuthKind(kind) {
			labels = append(labels, kind)
		}
	}
	sort.Strings(labels)
	labels = orderedContextStrings(labels)
	if len(labels) == 0 {
		return []string{SecurityUnknown}
	}
	return labels
}

func compactCatalogConsumerAuthKind(kind string) bool {
	switch kind {
	case SecurityBasic, SecurityBearer, SecurityOAuth2, SecurityAPIKey, SecuritySession,
		SecurityMTLS, SecurityRole, SecurityAuthenticated, SecurityUnknown:
		return true
	default:
		return false
	}
}

func compactCatalogSecurityRequiresAuth(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case SecurityBasic, SecurityBearer, SecurityOAuth2, SecurityAPIKey, SecuritySession,
		SecurityMTLS, SecurityRole, SecurityAuthenticated:
		return true
	default:
		return false
	}
}

func compactCatalogFactEvidenceIDs(kind string, evidenceIDs []string) []string {
	evidenceIDs = compactContextStrings(evidenceIDs)
	switch kind {
	case "api_endpoint", "endpoint_security", "api_consumer":
		if len(evidenceIDs) > maxCatalogFactEvidenceIDs {
			return evidenceIDs[:maxCatalogFactEvidenceIDs]
		}
	}
	return evidenceIDs
}

func usefulCompactCatalogSecurity(security SecurityEvidenceRecord) bool {
	kind := strings.ToLower(strings.TrimSpace(security.Kind))
	return security.Conflicting || kind != "" && kind != SecurityUnknown ||
		strings.TrimSpace(security.Source) != "" || contextPathKey(security.File) != "" || security.Line > 0
}

func selectCompactCatalogConsumers(
	consumers []APIConsumerRecord,
	indexedProjects map[string]bool,
) ([]compactCatalogConsumerSelection, int) {
	groups := map[string][]APIConsumerRecord{}
	serviceLabels := map[string]string{}
	total := 0
	for _, consumer := range consumers {
		project := contextPathKey(consumer.Project)
		file := workspaceAgentFile(project, consumer.File)
		if !indexedProjects[project] || file == "" || consumer.Line <= 0 {
			continue
		}
		consumer.Project = project
		consumer.File = file
		service := firstNonEmpty(strings.TrimSpace(consumer.Service), project)
		groupKey := project + "\x00" + service
		groups[groupKey] = append(groups[groupKey], consumer)
		serviceLabels[groupKey] = compactCatalogValue(service)
		total++
	}

	groupKeys := make([]string, 0, len(groups))
	for groupKey := range groups {
		groupKeys = append(groupKeys, groupKey)
	}
	sort.Strings(groupKeys)
	selected := make([]compactCatalogConsumerSelection, 0, total)
	for _, groupKey := range groupKeys {
		group := groups[groupKey]
		sort.Slice(group, func(i, j int) bool {
			return compactCatalogConsumerKey(group[i]) < compactCatalogConsumerKey(group[j])
		})
		if len(group) > maxCatalogConsumersPerService {
			group = group[:maxCatalogConsumersPerService]
		}
		for _, consumer := range group {
			selected = append(selected, compactCatalogConsumerSelection{
				consumer: consumer,
				service:  serviceLabels[groupKey],
			})
		}
	}
	return selected, total - len(selected)
}

type agentContextBuilder struct {
	project              string
	evidenceByLocation   map[string][]string
	factsByID            map[string]AgentContextFactRecord
	edgesByID            map[string]AgentContextEdgeRecord
	factIDsByLabel       map[string][]string
	factIDsByFile        map[string][]string
	routeFactIDs         map[string][]string
	routeFactIDsBySource map[string][]string
	backendRouteFactIDs  map[string]bool
	symbols              []RichSymbolRecord
	symbolsByID          map[string]RichSymbolRecord
	symbolsByLabel       map[string][]RichSymbolRecord
	selectedSymbolKinds  map[string]string
	symbolFactIDs        map[string]string
	flowRouteFactIDs     []string
	flowStepFactIDs      [][]string
	testFactIDs          []string
	contractFactIDs      []string
	coverageByCapability map[string]AgentContextCoverageRecord
}

func BuildProjectAgentContextIndex(
	project string,
	generated string,
	routes []CodeRouteRecord,
	flows []CodeFlowRecord,
	symbols []RichSymbolRecord,
	relations []RichRelationRecord,
	tests []TestMapRecord,
	contracts []APIContractRecord,
	evidence []EvidenceRecord,
	capabilities []CapabilityRecord,
) AgentContextIndexRecord {
	builder := newAgentContextBuilder(project, symbols, evidence)
	builder.selectSymbols(routes, flows, relations, tests, contracts)
	builder.addRouteFacts(routes)
	builder.addSymbolFacts()
	builder.addFlowFacts(flows)
	builder.addTestFacts(tests)
	builder.addAPIContractFacts(contracts)
	builder.addRelationEdges(relations)
	builder.addFlowEdges(flows)
	builder.addTestEdges(tests)
	builder.addAPIContractEdges(contracts)
	builder.addCoverage(capabilities)
	return builder.index(generated)
}

func newAgentContextBuilder(project string, symbols []RichSymbolRecord, evidence []EvidenceRecord) *agentContextBuilder {
	builder := &agentContextBuilder{
		project:              project,
		evidenceByLocation:   map[string][]string{},
		factsByID:            map[string]AgentContextFactRecord{},
		edgesByID:            map[string]AgentContextEdgeRecord{},
		factIDsByLabel:       map[string][]string{},
		factIDsByFile:        map[string][]string{},
		routeFactIDs:         map[string][]string{},
		routeFactIDsBySource: map[string][]string{},
		backendRouteFactIDs:  map[string]bool{},
		symbols:              append([]RichSymbolRecord(nil), symbols...),
		symbolsByID:          map[string]RichSymbolRecord{},
		symbolsByLabel:       map[string][]RichSymbolRecord{},
		selectedSymbolKinds:  map[string]string{},
		symbolFactIDs:        map[string]string{},
		coverageByCapability: map[string]AgentContextCoverageRecord{},
	}
	for _, item := range evidence {
		if item.ID == "" {
			continue
		}
		key := contextLocationKey(item.File, item.Start.Line)
		builder.evidenceByLocation[key] = append(builder.evidenceByLocation[key], item.ID)
	}
	for _, symbol := range builder.symbols {
		builder.symbolsByID[symbol.ID] = symbol
		for _, label := range []string{symbol.Name, symbol.QualifiedName, symbol.ExportName, qualifiedContextName(symbol.Owner, symbol.Name)} {
			key := contextLabelKey(label)
			if key != "" {
				builder.symbolsByLabel[key] = append(builder.symbolsByLabel[key], symbol)
			}
		}
	}
	return builder
}

func (builder *agentContextBuilder) selectSymbols(
	routes []CodeRouteRecord,
	flows []CodeFlowRecord,
	relations []RichRelationRecord,
	tests []TestMapRecord,
	contracts []APIContractRecord,
) {
	relationSeeds := make(map[string]bool)
	selectRelationSeed := func(value, file string, line int, kind string) {
		symbol, ok := builder.bestMatchingSymbol(value, file, line)
		if !ok {
			return
		}
		builder.selectSymbol(symbol, kind)
		relationSeeds[symbol.ID] = true
	}
	for _, symbol := range builder.symbols {
		if contextTypeNavigationSymbol(symbol) {
			builder.selectSymbol(symbol, "symbol")
		}
	}
	for _, route := range routes {
		selectRelationSeed(route.Handler, route.File, route.Line, "symbol")
		selectRelationSeed(contextSimpleName(route.Handler), route.File, route.Line, "symbol")
	}
	for _, flow := range flows {
		for _, step := range flow.Steps {
			kind := "symbol"
			if contextPersistenceStep(step) {
				kind = "persistence"
			}
			selectRelationSeed(qualifiedContextName(step.Owner, contextSimpleName(step.Name)), step.File, step.Line, kind)
			selectRelationSeed(contextSimpleName(step.Name), step.File, step.Line, kind)
		}
	}
	for _, test := range tests {
		selectRelationSeed(qualifiedContextName(test.TargetClass, test.TargetMethod), test.TargetFile, 0, "symbol")
		selectRelationSeed(test.TargetMethod, test.TargetFile, 0, "symbol")
	}
	for _, contract := range contracts {
		selectRelationSeed(contract.Caller, contract.File, contract.Line, "symbol")
		selectRelationSeed(contextSimpleName(contract.Caller), contract.File, contract.Line, "symbol")
	}
	for _, relation := range relations {
		if relation.NonPromotable {
			continue
		}
		kind, ok := contextSemanticRelationKind(relation.Type)
		if !ok || kind != "call" {
			continue
		}
		from, hasFrom := builder.relationSymbol(relation.FromSymbolID, relation.From)
		to, hasTo := builder.relationSymbol(relation.ToSymbolID, firstNonEmpty(relation.TargetQualifiedName, relation.To))
		fromFile := relation.From
		if hasFrom {
			fromFile = from.File
		}
		toFile := ""
		if hasTo {
			toFile = to.File
		}
		if fromFile == "" || toFile == "" || contextPathKey(fromFile) == contextPathKey(toFile) {
			continue
		}
		startsAtSeed := hasFrom && relationSeeds[from.ID]
		if hasTo && startsAtSeed {
			builder.selectSymbol(to, "symbol")
		}
		if hasFrom && hasTo && relationSeeds[to.ID] {
			builder.selectSymbol(from, "symbol")
		}
	}
}

func (builder *agentContextBuilder) selectSymbol(symbol RichSymbolRecord, kind string) {
	if symbol.ID == "" {
		return
	}
	if previous := builder.selectedSymbolKinds[symbol.ID]; previous == "persistence" {
		return
	}
	builder.selectedSymbolKinds[symbol.ID] = kind
}

func (builder *agentContextBuilder) relationSymbol(id, label string) (RichSymbolRecord, bool) {
	if id != "" {
		symbol, ok := builder.symbolsByID[id]
		return symbol, ok
	}
	candidates := builder.matchingSymbols(label, "")
	if len(candidates) != 1 {
		return RichSymbolRecord{}, false
	}
	return candidates[0], true
}

func (builder *agentContextBuilder) matchingSymbols(value, file string) []RichSymbolRecord {
	candidates := builder.symbolsByLabel[contextLabelKey(value)]
	if len(candidates) == 0 {
		return nil
	}
	if file == "" {
		return candidates
	}
	var sameFile []RichSymbolRecord
	for _, symbol := range candidates {
		if contextPathKey(symbol.File) == contextPathKey(file) {
			sameFile = append(sameFile, symbol)
		}
	}
	if len(sameFile) > 0 {
		return sameFile
	}
	return candidates
}

func (builder *agentContextBuilder) bestMatchingSymbol(value, file string, line int) (RichSymbolRecord, bool) {
	candidates := builder.matchingSymbols(value, file)
	if len(candidates) == 0 {
		return RichSymbolRecord{}, false
	}
	bestIndex := -1
	bestScore := -1
	tied := false
	for index, symbol := range candidates {
		score := 0
		if file != "" && contextPathKey(symbol.File) == contextPathKey(file) {
			score += 100000
		}
		if line > 0 && symbol.Line > 0 && symbol.Line <= line {
			score += 10000 + symbol.Line
		}
		if score > bestScore {
			bestIndex = index
			bestScore = score
			tied = false
		} else if score == bestScore {
			tied = true
		}
	}
	if tied || bestIndex < 0 {
		return RichSymbolRecord{}, false
	}
	return candidates[bestIndex], true
}

func (builder *agentContextBuilder) addRouteFacts(routes []CodeRouteRecord) {
	for _, route := range routes {
		method := strings.ToUpper(strings.TrimSpace(route.HTTPMethod))
		routePath := normalizeCodeRoutePath(route.Path)
		file := contextPathKey(route.File)
		name := strings.TrimSpace(method + " " + routePath)
		fact := AgentContextFactRecord{
			Kind:        "route",
			Name:        name,
			Qualified:   strings.TrimSpace(route.Handler),
			HTTPMethod:  method,
			Path:        routePath,
			File:        file,
			Line:        route.Line,
			Confidence:  route.Confidence,
			EvidenceIDs: builder.evidenceIDs(file, route.Line, route.EvidenceIDs),
			Search: compactContextSearch(
				method,
				routePath,
				route.Handler,
				contextFileBase(file),
				contextFileStem(file),
			),
		}
		id := builder.addFact(fact)
		key := contextRouteKey(method, routePath)
		builder.routeFactIDs[key] = appendUniqueContextID(
			builder.routeFactIDs[key],
			id,
		)
		if route.RouteID != "" {
			builder.routeFactIDsBySource[route.RouteID] = appendUniqueContextID(
				builder.routeFactIDsBySource[route.RouteID],
				id,
			)
		}
		if route.Kind == "backend" {
			builder.backendRouteFactIDs[id] = true
		}
	}
}

func (builder *agentContextBuilder) addSymbolFacts() {
	for _, symbol := range builder.symbols {
		kind, selected := builder.selectedSymbolKinds[symbol.ID]
		if !selected {
			continue
		}
		fact := AgentContextFactRecord{
			Kind:        kind,
			Name:        symbol.Name,
			Qualified:   firstNonEmpty(symbol.QualifiedName, qualifiedContextName(symbol.Owner, symbol.Name)),
			File:        contextPathKey(symbol.File),
			Line:        symbol.Line,
			Confidence:  string(symbol.Confidence),
			EvidenceIDs: builder.evidenceIDs(symbol.File, symbol.Line, symbol.EvidenceIDs),
			Search: compactContextSearch(
				symbol.Name,
				symbol.QualifiedName,
				symbol.Owner,
				symbol.ExportName,
				contextFileBase(symbol.File),
				contextFileStem(symbol.File),
			),
		}
		builder.symbolFactIDs[symbol.ID] = builder.addFact(fact)
	}
}

func (builder *agentContextBuilder) addFlowFacts(flows []CodeFlowRecord) {
	builder.flowRouteFactIDs = make([]string, len(flows))
	builder.flowStepFactIDs = make([][]string, len(flows))
	for flowIndex, flow := range flows {
		builder.flowRouteFactIDs[flowIndex] = builder.routeFactIDForFlow(flow)
		builder.flowStepFactIDs[flowIndex] = make([]string, len(flow.Steps))
		for stepIndex, step := range flow.Steps {
			if stepIndex == 0 && contextRouteHandlerStep(step) {
				if id := builder.flowRouteFactIDs[flowIndex]; id != "" {
					builder.flowStepFactIDs[flowIndex][stepIndex] = id
					continue
				}
			}
			if id := builder.factIDForFlowStep(step); id != "" {
				builder.flowStepFactIDs[flowIndex][stepIndex] = id
				continue
			}
			kind := "symbol"
			if contextPersistenceStep(step) {
				kind = "persistence"
			}
			name := contextSimpleName(step.Name)
			qualified := qualifiedContextName(step.Owner, name)
			fact := AgentContextFactRecord{
				Kind:        kind,
				Name:        firstNonEmpty(strings.TrimSpace(step.Name), name),
				Qualified:   qualified,
				File:        contextPathKey(step.File),
				Line:        step.Line,
				Confidence:  step.Confidence,
				EvidenceIDs: builder.evidenceIDs(step.File, step.Line, step.EvidenceIDs),
				Search: compactContextSearch(
					step.Name,
					step.Owner,
					qualified,
					step.Kind,
					contextFileBase(step.File),
					contextFileStem(step.File),
					flow.HTTPMethod,
					flow.Path,
				),
			}
			builder.flowStepFactIDs[flowIndex][stepIndex] = builder.addFact(fact)
		}
	}
}

func (builder *agentContextBuilder) factIDForFlowStep(step CodeFlowStep) string {
	symbol, ok := builder.bestMatchingSymbol(
		qualifiedContextName(step.Owner, contextSimpleName(step.Name)),
		step.File,
		step.Line,
	)
	if !ok {
		symbol, ok = builder.bestMatchingSymbol(contextSimpleName(step.Name), step.File, step.Line)
	}
	if ok {
		return builder.symbolFactIDs[symbol.ID]
	}
	return ""
}

func (builder *agentContextBuilder) addTestFacts(tests []TestMapRecord) {
	builder.testFactIDs = make([]string, len(tests))
	for index, test := range tests {
		name := firstNonEmpty(test.TestMethod, test.TestCase, test.TestClass, contextFileStem(test.TestFile))
		qualified := qualifiedContextName(test.TestClass, test.TestMethod)
		if name == "" {
			name = contextFileStem(test.TestFile)
		}
		target := qualifiedContextName(test.TargetClass, test.TargetMethod)
		method := strings.ToUpper(strings.TrimSpace(test.HTTPMethod))
		testPath := normalizeOptionalContextPath(test.Path)
		summaryTarget := target
		if method != "" && testPath != "" {
			summaryTarget = strings.TrimSpace(method + " " + testPath)
		}
		summary := ""
		if summaryTarget != "" {
			summary = "tests " + summaryTarget
		}
		fact := AgentContextFactRecord{
			Kind:        "test",
			Name:        name,
			Qualified:   firstNonEmpty(qualified, test.TestClass, test.TestMethod),
			HTTPMethod:  method,
			Path:        testPath,
			File:        contextPathKey(test.TestFile),
			Line:        test.Line,
			Summary:     summary,
			Confidence:  test.Confidence,
			EvidenceIDs: builder.evidenceIDs(test.TestFile, test.Line, nil),
			Search: compactContextSearch(
				name,
				test.TestClass,
				test.TestMethod,
				target,
				test.TargetClass,
				test.TargetMethod,
				test.TargetFile,
				test.HTTPMethod,
				test.Path,
				test.TestCase,
				contextFileBase(test.TestFile),
				contextFileStem(test.TestFile),
			),
		}
		builder.testFactIDs[index] = builder.addFact(fact)
	}
}

func (builder *agentContextBuilder) addAPIContractFacts(contracts []APIContractRecord) {
	builder.contractFactIDs = make([]string, len(contracts))
	for index, contract := range contracts {
		method := strings.ToUpper(strings.TrimSpace(contract.HTTPMethod))
		contractPath := normalizeAPIPath(contract.Path)
		file := contextPathKey(contract.File)
		name := strings.TrimSpace(method + " " + contractPath)
		authKinds := compactContractAuthKinds(contract.Auth)
		reason := compactCatalogValue(strings.TrimSpace(contract.Reason))
		summaryParts := []string{}
		if contract.ServiceCandidate != "" {
			summaryParts = append(summaryParts, "calls "+contract.ServiceCandidate)
		}
		if len(authKinds) > 0 {
			summaryParts = append(summaryParts, "auth "+strings.Join(authKinds, ", "))
		}
		if reason != "" {
			summaryParts = append(summaryParts, reason)
		}
		fact := AgentContextFactRecord{
			Kind:        "api_contract",
			Name:        name,
			Qualified:   strings.TrimSpace(contract.Caller),
			HTTPMethod:  method,
			Path:        contractPath,
			File:        file,
			Line:        contract.Line,
			Summary:     strings.Join(summaryParts, "; "),
			Confidence:  contract.Confidence,
			EvidenceIDs: builder.evidenceIDs(file, contract.Line, nil),
			Search: compactContextSearch(
				method,
				contractPath,
				contract.Caller,
				contract.ServiceCandidate,
				strings.Join(authKinds, " "),
				reason,
				contextFileBase(file),
				contextFileStem(file),
			),
		}
		builder.contractFactIDs[index] = builder.addFact(fact)
	}
}

func compactContractAuthKinds(auth []AuthRecord) []string {
	kinds := make([]string, 0, len(auth))
	for _, record := range auth {
		kind := strings.ToLower(strings.TrimSpace(record.Kind))
		if compactCatalogConsumerAuthKind(kind) {
			kinds = append(kinds, kind)
		}
	}
	sort.Strings(kinds)
	return orderedContextStrings(kinds)
}

func (builder *agentContextBuilder) addRelationEdges(relations []RichRelationRecord) {
	for _, relation := range relations {
		if relation.NonPromotable {
			continue
		}
		kind, ok := contextSemanticRelationKind(relation.Type)
		if !ok || relation.Resolution == SymbolResolutionAmbiguous {
			continue
		}
		fromID := builder.resolveFactID(relation.FromSymbolID, relation.From, relation.From, relation.Line, "route")
		toID := ""
		if relation.Resolution == SymbolResolutionUnresolved {
			if relation.TargetQualifiedName == "" {
				continue
			}
			toID = builder.resolveExactQualifiedFactID(relation.TargetQualifiedName)
		} else {
			toID = builder.resolveFactID(
				relation.ToSymbolID,
				firstNonEmpty(relation.TargetQualifiedName, relation.To),
				relation.To,
				0,
				"",
			)
		}
		if fromID == "" || toID == "" || fromID == toID {
			continue
		}
		fromFact := builder.factsByID[fromID]
		toFact := builder.factsByID[toID]
		if relation.Resolution == SymbolResolutionUnresolved &&
			fromFact.File != "" && contextPathKey(fromFact.File) == contextPathKey(toFact.File) {
			continue
		}
		if relation.FromSymbolID == "" && relation.ToSymbolID == "" &&
			fromFact.File != "" && contextPathKey(fromFact.File) == contextPathKey(toFact.File) {
			continue
		}
		builder.addEdge(AgentContextEdgeRecord{
			FromFactID:  fromID,
			ToFactID:    toID,
			Kind:        kind,
			File:        contextPathKey(relation.From),
			Line:        relation.Line,
			Reason:      relation.Reason,
			Confidence:  relation.Confidence,
			EvidenceIDs: builder.evidenceIDs(relation.From, relation.Line, relation.EvidenceIDs),
		})
	}
}

func (builder *agentContextBuilder) addFlowEdges(flows []CodeFlowRecord) {
	for flowIndex, flow := range flows {
		stepIDs := builder.flowStepFactIDs[flowIndex]
		if len(stepIDs) == 0 {
			continue
		}
		routeID := builder.flowRouteFactIDs[flowIndex]
		if routeID != "" && stepIDs[0] != "" && routeID != stepIDs[0] {
			builder.addEdge(AgentContextEdgeRecord{
				FromFactID: routeID,
				ToFactID:   stepIDs[0],
				Kind:       "call",
				File:       contextPathKey(flow.File),
				Line:       flow.Line,
				Reason:     "flow transition",
			})
		}
		for index := 1; index < len(stepIDs); index++ {
			fromID := stepIDs[index-1]
			step := flow.Steps[index]
			if step.Caller != "" {
				preferredKind := "symbol"
				if contextLabelKey(step.Caller) == contextLabelKey(flow.Handler) {
					preferredKind = "route"
				}
				fromID = builder.resolveFactID(
					"",
					step.Caller,
					step.CallerFile,
					step.CallerLine,
					preferredKind,
				)
			}
			toID := stepIDs[index]
			if fromID == "" || toID == "" || fromID == toID {
				continue
			}
			kind := "call"
			if builder.factsByID[toID].Kind == "persistence" {
				kind = "persistence"
			}
			reason := firstNonEmpty(step.Reason, "flow transition")
			builder.addEdge(AgentContextEdgeRecord{
				FromFactID: fromID,
				ToFactID:   toID,
				Kind:       kind,
				File:       contextPathKey(step.File),
				Line:       step.Line,
				Reason:     reason,
				Confidence: step.Confidence,
				EvidenceIDs: builder.evidenceIDs(
					step.File,
					step.Line,
					step.EvidenceIDs,
				),
			})
		}
	}
}

func (builder *agentContextBuilder) addTestEdges(tests []TestMapRecord) {
	for index, test := range tests {
		fromID := builder.testFactIDs[index]
		targetID := ""
		if strings.EqualFold(test.Type, "endpoint") {
			targetID = builder.compatibleRouteFactID(test.HTTPMethod, test.Path)
		}
		if targetID == "" {
			targetID = builder.resolveFactID(
				"",
				qualifiedContextName(test.TargetClass, test.TargetMethod),
				test.TargetFile,
				0,
				"",
			)
		}
		if targetID == "" {
			targetID = builder.resolveFactID("", test.TargetMethod, test.TargetFile, 0, "")
		}
		if fromID == "" || targetID == "" || fromID == targetID {
			continue
		}
		builder.addEdge(AgentContextEdgeRecord{
			FromFactID: fromID,
			ToFactID:   targetID,
			Kind:       "test_target",
			File:       contextPathKey(test.TestFile),
			Line:       test.Line,
			Reason:     test.Reason,
			Confidence: test.Confidence,
			EvidenceIDs: builder.evidenceIDs(
				test.TestFile,
				test.Line,
				nil,
			),
		})
	}
}

func (builder *agentContextBuilder) addAPIContractEdges(contracts []APIContractRecord) {
	for index, contract := range contracts {
		fromID := builder.contractFactIDs[index]
		if caller, ok := builder.bestMatchingSymbol(contract.Caller, contract.File, contract.Line); ok {
			if callerID := builder.symbolFactIDs[caller.ID]; callerID != "" && callerID != fromID {
				builder.addEdge(AgentContextEdgeRecord{
					FromFactID: callerID,
					ToFactID:   fromID,
					Kind:       "call",
					File:       contextPathKey(contract.File),
					Line:       contract.Line,
					Confidence: contract.Confidence,
					EvidenceIDs: builder.evidenceIDs(
						contract.File,
						contract.Line,
						nil,
					),
				})
			}
		}
		if contract.UnsafeDynamic || isFrontendInternalAPIPath(contract.File, normalizeAPIPath(contract.Path)) {
			continue
		}
		targetID := builder.compatibleRouteFactID(contract.HTTPMethod, contract.Path)
		if fromID == "" || targetID == "" || fromID == targetID {
			continue
		}
		builder.addEdge(AgentContextEdgeRecord{
			FromFactID: fromID,
			ToFactID:   targetID,
			Kind:       "http_contract",
			File:       contextPathKey(contract.File),
			Line:       contract.Line,
			Reason:     "http method and path pattern match",
			Confidence: contract.Confidence,
			EvidenceIDs: builder.evidenceIDs(
				contract.File,
				contract.Line,
				nil,
			),
		})
	}
}

func (builder *agentContextBuilder) addCoverage(capabilities []CapabilityRecord) {
	grouped := map[string][]CapabilityRecord{}
	for _, capability := range capabilities {
		if !contextAgentCapability(capability.ID) || !contextCodeCapability(capability) {
			continue
		}
		key := builder.project + "\x00" + string(capability.ID)
		grouped[key] = append(grouped[key], capability)
	}
	for key, candidates := range grouped {
		winningRank := -1
		for _, candidate := range candidates {
			winningRank = max(winningRank, contextCoverageRank(string(candidate.Coverage)))
		}
		var winning []CapabilityRecord
		for _, candidate := range candidates {
			if contextCoverageRank(string(candidate.Coverage)) == winningRank {
				winning = append(winning, candidate)
			}
		}
		sort.Slice(winning, func(i, j int) bool {
			if winning[i].Language != winning[j].Language {
				return winning[i].Language < winning[j].Language
			}
			if winning[i].StatusReason != winning[j].StatusReason {
				return winning[i].StatusReason < winning[j].StatusReason
			}
			if winning[i].Reason != winning[j].Reason {
				return winning[i].Reason < winning[j].Reason
			}
			return winning[i].Coverage < winning[j].Coverage
		})
		reason := ""
		for _, candidate := range winning {
			reason = firstNonEmpty(candidate.StatusReason, candidate.Reason)
			if reason != "" {
				break
			}
		}
		if len(winning) == 0 {
			continue
		}
		record := AgentContextCoverageRecord{
			Project:    builder.project,
			Capability: string(winning[0].ID),
			Coverage:   string(winning[0].Coverage),
			Reason:     reason,
		}
		builder.coverageByCapability[key] = record
	}
}

func (builder *agentContextBuilder) addFact(fact AgentContextFactRecord) string {
	fact.Project = firstNonEmpty(fact.Project, builder.project)
	fact.File = contextPathKey(fact.File)
	fact.EvidenceIDs = compactContextStrings(fact.EvidenceIDs)
	if fact.Search == "" {
		fact.Search = compactContextSearch(fact.Name, fact.Qualified, fact.HTTPMethod, fact.Path, contextFileBase(fact.File))
	}
	if fact.ID == "" {
		fact.ID = stableID(
			"agent-context-fact",
			fact.Project,
			fact.Kind,
			fact.Name,
			fact.Qualified,
			fact.HTTPMethod,
			fact.Path,
			fact.File,
			fmt.Sprint(fact.Line),
			fmt.Sprint(fact.EndLine),
		)
	}
	if existing, exists := builder.factsByID[fact.ID]; exists {
		existing.EvidenceIDs = compactContextStrings(append(existing.EvidenceIDs, fact.EvidenceIDs...))
		existing.Search = mergeContextSearch(existing.Search, fact.Search)
		existing.Confidence = strongerContextConfidence(existing.Confidence, fact.Confidence)
		existing.Summary = deterministicContextText(existing.Summary, fact.Summary)
		builder.factsByID[fact.ID] = existing
		return fact.ID
	}
	builder.factsByID[fact.ID] = fact
	for _, label := range []string{fact.Name, fact.Qualified} {
		key := contextLabelKey(label)
		if key != "" {
			builder.factIDsByLabel[key] = appendUniqueContextID(builder.factIDsByLabel[key], fact.ID)
		}
	}
	if fact.File != "" {
		key := contextPathKey(fact.File)
		builder.factIDsByFile[key] = appendUniqueContextID(builder.factIDsByFile[key], fact.ID)
	}
	return fact.ID
}

func (builder *agentContextBuilder) addEdge(edge AgentContextEdgeRecord) {
	from, hasFrom := builder.factsByID[edge.FromFactID]
	to, hasTo := builder.factsByID[edge.ToFactID]
	if !hasFrom || !hasTo {
		return
	}
	edge.Project = firstNonEmpty(edge.Project, builder.project)
	edge.File = contextPathKey(edge.File)
	edge.FromLabel = firstNonEmpty(edge.FromLabel, contextFactLabel(from))
	edge.ToLabel = firstNonEmpty(edge.ToLabel, contextFactLabel(to))
	edge.EvidenceIDs = compactContextStrings(edge.EvidenceIDs)
	if edge.ID == "" {
		edge.ID = stableID(
			"agent-context-edge",
			edge.Project,
			edge.FromFactID,
			edge.ToFactID,
			edge.Kind,
			edge.File,
			fmt.Sprint(edge.Line),
		)
	}
	if existing, exists := builder.edgesByID[edge.ID]; exists {
		existing.EvidenceIDs = compactContextStrings(append(existing.EvidenceIDs, edge.EvidenceIDs...))
		existing.Confidence = strongerContextConfidence(existing.Confidence, edge.Confidence)
		existing.Reason = deterministicContextText(existing.Reason, edge.Reason)
		builder.edgesByID[edge.ID] = existing
	} else {
		builder.edgesByID[edge.ID] = edge
	}
}

func (builder *agentContextBuilder) resolveFactID(symbolID, label, file string, line int, preferredKind string) string {
	if symbolID != "" {
		return builder.symbolFactIDs[symbolID]
	}
	if ids := builder.factIDsByLabel[contextLabelKey(label)]; len(ids) > 0 {
		return builder.bestFactID(ids, file, line, preferredKind)
	}
	if file != "" {
		return builder.bestFactID(builder.factIDsByFile[contextPathKey(file)], file, line, preferredKind)
	}
	return ""
}

func (builder *agentContextBuilder) resolveExactQualifiedFactID(qualified string) string {
	qualified = strings.TrimSpace(qualified)
	if qualified == "" {
		return ""
	}
	var matches []string
	for _, id := range builder.factIDsByLabel[contextLabelKey(qualified)] {
		if builder.factsByID[id].Qualified == qualified {
			matches = appendUniqueContextID(matches, id)
		}
	}
	if len(matches) != 1 {
		return ""
	}
	return matches[0]
}

func (builder *agentContextBuilder) bestFactID(ids []string, file string, line int, preferredKind string) string {
	bestID := ""
	bestScore := -1
	tied := false
	for _, id := range ids {
		fact := builder.factsByID[id]
		score := 0
		if preferredKind != "" && fact.Kind == preferredKind {
			score += 1000000
		}
		if file != "" && contextPathKey(fact.File) == contextPathKey(file) {
			score += 100000
		}
		if line > 0 && fact.Line > 0 && fact.Line <= line {
			score += 10000 + fact.Line
		}
		if score > bestScore {
			bestID = id
			bestScore = score
			tied = false
		} else if score == bestScore {
			if id == bestID {
				continue
			}
			tied = true
			if bestID == "" || id < bestID {
				bestID = id
			}
		}
	}
	if tied {
		return ""
	}
	return bestID
}

func (builder *agentContextBuilder) routeFactIDForFlow(flow CodeFlowRecord) string {
	if flow.RouteID != "" {
		ids := builder.routeFactIDsBySource[flow.RouteID]
		if len(ids) == 1 {
			return ids[0]
		}
	}
	key := contextRouteKey(flow.HTTPMethod, flow.Path)
	ids := builder.routeFactIDs[key]
	if len(ids) == 1 {
		return ids[0]
	}
	if len(ids) > 1 {
		return builder.bestFactID(ids, flow.File, flow.Line, "route")
	}
	return builder.compatibleRouteFactID(flow.HTTPMethod, flow.Path)
}

func (builder *agentContextBuilder) compatibleRouteFactID(method, routePath string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	routePath = normalizeOptionalContextPath(routePath)
	if method == "" || routePath == "" {
		return ""
	}
	var candidates []string
	var backend []string
	for id, fact := range builder.factsByID {
		if fact.Kind != "route" || !strings.EqualFold(method, fact.HTTPMethod) ||
			!pathsCompatibleWithKnownBasePrefixes(routePath, fact.Path) {
			continue
		}
		candidates = append(candidates, id)
		if builder.backendRouteFactIDs[id] {
			backend = append(backend, id)
		}
	}
	if len(backend) > 0 {
		candidates = backend
	}
	sort.Strings(candidates)
	if len(candidates) != 1 {
		return ""
	}
	return candidates[0]
}

func (builder *agentContextBuilder) evidenceIDs(file string, line int, explicit []string) []string {
	ids := append([]string(nil), explicit...)
	ids = append(ids, builder.evidenceByLocation[contextLocationKey(file, line)]...)
	return compactContextStrings(ids)
}

func (builder *agentContextBuilder) index(generated string) AgentContextIndexRecord {
	facts := make([]AgentContextFactRecord, 0, len(builder.factsByID))
	for _, fact := range builder.factsByID {
		facts = append(facts, fact)
	}
	sortAgentContextFacts(facts)

	edges := make([]AgentContextEdgeRecord, 0, len(builder.edgesByID))
	for _, edge := range builder.edgesByID {
		edges = append(edges, edge)
	}
	sortAgentContextEdges(edges)

	coverage := make([]AgentContextCoverageRecord, 0, len(builder.coverageByCapability))
	for _, record := range builder.coverageByCapability {
		coverage = append(coverage, record)
	}
	sort.Slice(coverage, func(i, j int) bool {
		return contextCoverageLess(coverage[i], coverage[j])
	})

	return AgentContextIndexRecord{
		SchemaVersion: SchemaVersion,
		Generated:     generated,
		Root:          builder.project,
		Facts:         facts,
		Edges:         edges,
		Coverage:      coverage,
	}
}

func sortAgentContextFacts(facts []AgentContextFactRecord) {
	sort.Slice(facts, func(i, j int) bool {
		left, right := facts[i], facts[j]
		if left.Project != right.Project {
			return left.Project < right.Project
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Qualified != right.Qualified {
			return left.Qualified < right.Qualified
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.ID < right.ID
	})
}

func sortAgentContextEdges(edges []AgentContextEdgeRecord) {
	sort.Slice(edges, func(i, j int) bool {
		left, right := edges[i], edges[j]
		if left.FromLabel != right.FromLabel {
			return left.FromLabel < right.FromLabel
		}
		if left.ToLabel != right.ToLabel {
			return left.ToLabel < right.ToLabel
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.ID < right.ID
	})
}

func contextTypeNavigationSymbol(symbol RichSymbolRecord) bool {
	language := strings.ToLower(symbol.Language)
	kind := strings.ToLower(symbol.Kind)
	switch language {
	case "java", "kotlin":
		switch kind {
		case "class", "interface", "record", "enum", "annotation", "annotation_class", "annotation_type", "@interface":
			return true
		}
	case "javascript", "typescript", "jsx", "tsx":
		if symbol.ExportName == "" {
			return false
		}
		switch kind {
		case "class", "component", "hook", "interface", "type", "enum":
			return true
		case "function", "method":
			return contextHookName(symbol.Name)
		}
	}
	return false
}

func contextHookName(name string) bool {
	runes := []rune(name)
	return len(runes) > 3 && string(runes[:3]) == "use" &&
		(unicode.IsUpper(runes[3]) || unicode.IsDigit(runes[3]) || runes[3] == '_')
}

func contextPersistenceStep(step CodeFlowStep) bool {
	for _, value := range []string{step.Kind, step.Name, step.Owner} {
		for _, token := range contextIdentifierTokens(value) {
			switch strings.ToLower(token) {
			case "repository", "persistence", "entity", "database", "store":
				return true
			}
		}
	}
	return false
}

func contextRouteHandlerStep(step CodeFlowStep) bool {
	kind := strings.ToLower(strings.TrimSpace(step.Kind))
	return kind == "route_handler" || kind == "handler" || kind == "endpoint"
}

func contextSemanticRelationKind(value string) (string, bool) {
	kind := strings.ToLower(strings.TrimSpace(value))
	switch {
	case kind == "call":
		return "call", true
	case kind == "calls", kind == "calls_method_owner", kind == "calls_export", kind == "calls_function":
		return "call", true
	case kind == "use" || kind == "uses" || strings.HasPrefix(kind, "uses_"):
		return "use", true
	case kind == "implements" || strings.HasPrefix(kind, "implements_"):
		return "implements", true
	case kind == "extends" || strings.HasPrefix(kind, "extends_"):
		return "extends", true
	case kind == "instantiates", kind == "renders_component", kind == "type_reference",
		kind == "field_type", kind == "return_type", kind == "parameter_type", kind == "annotation_type":
		return "use", true
	default:
		return "", false
	}
}

func contextAgentCapability(id CapabilityID) bool {
	switch id {
	case CapabilityRoutes, CapabilityCalls, CapabilityTests, CapabilityAPIClients, CapabilityPersistence:
		return true
	default:
		return false
	}
}

func contextCodeCapability(capability CapabilityRecord) bool {
	if capability.SourceClass != "" {
		return capability.SourceClass == "code"
	}
	switch strings.ToLower(strings.TrimSpace(capability.Language)) {
	case "", "unknown", "maven", "gradle", "node", "markdown", "text", "yaml", "yml",
		"json", "xml", "toml", "properties":
		return false
	default:
		return true
	}
}

func contextCoverageRank(value string) int {
	switch Coverage(value) {
	case CoverageFailed:
		return 4
	case CoverageComplete:
		return 3
	case CoveragePartial:
		return 2
	case CoverageUnavailable:
		return 1
	default:
		return 0
	}
}

func contextCoverageLess(left, right AgentContextCoverageRecord) bool {
	if left.Project != right.Project {
		return left.Project < right.Project
	}
	if left.Capability != right.Capability {
		return left.Capability < right.Capability
	}
	if left.Coverage != right.Coverage {
		return left.Coverage < right.Coverage
	}
	return left.Reason < right.Reason
}

func contextRouteKey(method, routePath string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + "\x00" + normalizeOptionalContextPath(routePath)
}

func contextLocationKey(file string, line int) string {
	return contextPathKey(file) + "\x00" + fmt.Sprint(line)
}

func contextPathKey(value string) string {
	return strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "./")
}

func contextLabelKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func contextFileBase(file string) string {
	return path.Base(contextPathKey(file))
}

func contextFileStem(file string) string {
	base := contextFileBase(file)
	return strings.TrimSuffix(base, path.Ext(base))
}

func normalizeOptionalContextPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return normalizeCodeRoutePath(value)
}

func qualifiedContextName(owner, name string) string {
	owner = strings.TrimSpace(owner)
	name = strings.TrimSpace(name)
	if owner == "" {
		return name
	}
	if name == "" || strings.HasPrefix(name, owner+".") {
		return name
	}
	return owner + "." + name
}

func contextSimpleName(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.LastIndexAny(value, ".#"); index >= 0 {
		return value[index+1:]
	}
	return value
}

func contextFactLabel(fact AgentContextFactRecord) string {
	if fact.Kind == "route" {
		return fact.Name
	}
	return firstNonEmpty(fact.Qualified, fact.Name)
}

func compactContextSearch(values ...string) string {
	var aliases []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		aliases = append(aliases, value)
		aliases = append(aliases, contextIdentifierTokens(value)...)
	}
	return strings.Join(orderedContextStrings(aliases), " ")
}

func contextIdentifierTokens(value string) []string {
	var aliases []string
	for _, token := range strings.FieldsFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	}) {
		aliases = append(aliases, contextCamelAliases(token)...)
	}
	return aliases
}

func contextCamelAliases(value string) []string {
	runes := []rune(value)
	if len(runes) == 0 {
		return nil
	}
	var aliases []string
	start := 0
	for index := 1; index < len(runes); index++ {
		current := runes[index]
		previous := runes[index-1]
		nextLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
		caseBoundary := unicode.IsUpper(current) && (unicode.IsLower(previous) || nextLower)
		digitBoundary := unicode.IsDigit(current) != unicode.IsDigit(previous) &&
			(unicode.IsDigit(current) || unicode.IsDigit(previous))
		if caseBoundary || digitBoundary {
			aliases = append(aliases, string(runes[start:index]))
			start = index
		}
	}
	aliases = append(aliases, string(runes[start:]))
	return aliases
}

func compactContextStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func orderedContextStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func mergeContextSearch(left, right string) string {
	values := append(strings.Fields(left), strings.Fields(right)...)
	sort.SliceStable(values, func(i, j int) bool {
		leftKey := strings.ToLower(values[i])
		rightKey := strings.ToLower(values[j])
		if leftKey != rightKey {
			return leftKey < rightKey
		}
		return values[i] < values[j]
	})
	return strings.Join(orderedContextStrings(values), " ")
}

func strongerContextConfidence(left, right string) string {
	if contextConfidenceRank(right) > contextConfidenceRank(left) {
		return right
	}
	if contextConfidenceRank(right) == contextConfidenceRank(left) && right != "" && (left == "" || right < left) {
		return right
	}
	return left
}

func contextConfidenceRank(value string) int {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "EXACT":
		return 9
	case "RESOLVED":
		return 8
	case "MATCHED":
		return 7
	case "EXTRACTED":
		return 6
	case "NORMALIZED":
		return 5
	case "INFERRED":
		return 4
	case "WEAK":
		return 3
	case "UNKNOWN":
		return 2
	case "":
		return 0
	default:
		return 1
	}
}

func deterministicContextText(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" || left <= right {
		return left
	}
	return right
}

func appendUniqueContextID(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
