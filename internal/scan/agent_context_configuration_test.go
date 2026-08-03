package scan

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestExtractAgentContextConfigurationFactsKeepsOnlyConfigurationStructure(t *testing.T) {
	const path = "src/main/resources/application.properties"
	const clientURL = "https://SENTINEL_CLIENT_URL.invalid"
	const username = "SENTINEL_CONFIG_USERNAME"
	const password = "SENTINEL_CONFIG_PASSWORD"
	body := "# client transport\n" +
		"client.url=" + clientURL + "\n" +
		"client.username=" + username + "\n" +
		"client.password=" + password + "\n" +
		"client.timeout-ms=2500\n" +
		"retry.max-attempts=3\n"

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	if len(facts) != 3 {
		t.Fatalf("fact count = %d, want file fact plus two key groups: %#v", len(facts), facts)
	}

	fileFact := findAgentContextConfigurationFact(t, facts, "application.properties")
	if fileFact.File != path || fileFact.Line != 1 || fileFact.EndLine != 6 || fileFact.Kind != "configuration" || fileFact.Confidence != string(ConfidenceExact) {
		t.Fatalf("file fact = %#v", fileFact)
	}
	clientFact := findAgentContextConfigurationFact(t, facts, "client")
	if clientFact.Line != 2 || clientFact.EndLine != 5 || clientFact.Qualified != "client" {
		t.Fatalf("client key group = %#v", clientFact)
	}
	retryFact := findAgentContextConfigurationFact(t, facts, "retry")
	if retryFact.Line != 6 || retryFact.EndLine != 6 || retryFact.Qualified != "retry" {
		t.Fatalf("retry key group = %#v", retryFact)
	}
	for _, fact := range facts {
		for _, value := range []string{clientURL, username, password, "2500", "max-attempts=3"} {
			if strings.Contains(fact.Name, value) || strings.Contains(fact.Summary, value) || strings.Contains(fact.Search, value) {
				t.Fatalf("configuration fact retains value %q: %#v", value, fact)
			}
		}
	}
}

func TestExtractAgentContextConfigurationFactsNormalizesLoneCarriageReturns(t *testing.T) {
	const path = "src/main/resources/application.properties"
	body := "# transport\r" +
		"client.url=SENTINEL\r" +
		"client.timeout-ms=2500\r" +
		"retry.max-attempts=3\r"

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	fileFact := findAgentContextConfigurationFact(t, facts, "application.properties")
	if fileFact.Line != 1 || fileFact.EndLine != 4 {
		t.Fatalf("file range = %d-%d, want 1-4", fileFact.Line, fileFact.EndLine)
	}
	clientFact := findAgentContextConfigurationFact(t, facts, "client")
	if clientFact.Line != 2 || clientFact.EndLine != 3 {
		t.Fatalf("client key group range = %d-%d, want 2-3", clientFact.Line, clientFact.EndLine)
	}
	retryFact := findAgentContextConfigurationFact(t, facts, "retry")
	if retryFact.Line != 4 || retryFact.EndLine != 4 {
		t.Fatalf("retry key group range = %d-%d, want 4-4", retryFact.Line, retryFact.EndLine)
	}
}

func TestExtractAgentContextConfigurationFactsAcceptsPropertiesWhitespaceSeparators(t *testing.T) {
	const path = "src/main/resources/application.properties"
	body := "client.url SENTINEL\n" +
		"client.timeout-ms\t2500\n" +
		"retry.max-attempts\f3\n"

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	clientFact := findAgentContextConfigurationFact(t, facts, "client")
	if clientFact.Line != 1 || clientFact.EndLine != 2 {
		t.Fatalf("client key group range = %d-%d, want 1-2", clientFact.Line, clientFact.EndLine)
	}
	retryFact := findAgentContextConfigurationFact(t, facts, "retry")
	if retryFact.Line != 3 || retryFact.EndLine != 3 {
		t.Fatalf("retry key group range = %d-%d, want 3-3", retryFact.Line, retryFact.EndLine)
	}
}

func TestExtractAgentContextConfigurationFactsAcceptsTrailingWhitespaceSeparators(t *testing.T) {
	const path = "src/main/resources/application.properties"
	body := "space.empty \n" +
		"tab.empty\t\n" +
		"form.empty\f\n"

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	for line, name := range []string{"space", "tab", "form"} {
		fact := findAgentContextConfigurationFact(t, facts, name)
		lineNumber := line + 1
		if fact.Line != lineNumber || fact.EndLine != lineNumber {
			t.Fatalf("%s key group range = %d-%d, want %d-%d", name, fact.Line, fact.EndLine, lineNumber, lineNumber)
		}
	}
}

func TestExtractAgentContextConfigurationFactsKeepsEscapedPropertiesSeparatorsInKeyGroups(t *testing.T) {
	const path = "src/main/resources/application.properties"
	body := "client\\:admin.url SENTINEL\n" +
		"client\\:admin.timeout-ms=2500\n" +
		"retry\\=policy.max-attempts:3\n"

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	clientFact := findAgentContextConfigurationFact(t, facts, "client:admin")
	if clientFact.Line != 1 || clientFact.EndLine != 2 {
		t.Fatalf("client:admin key group range = %d-%d, want 1-2", clientFact.Line, clientFact.EndLine)
	}
	retryFact := findAgentContextConfigurationFact(t, facts, "retry=policy")
	if retryFact.Line != 3 || retryFact.EndLine != 3 {
		t.Fatalf("retry=policy key group range = %d-%d, want 3-3", retryFact.Line, retryFact.EndLine)
	}
}

func TestExtractAgentContextConfigurationFactsDecodesPropertiesKeyEscapes(t *testing.T) {
	const path = "src/main/resources/application.properties"
	body := "tab\\tkey.value=SENTINEL\n" +
		"line\\nkey.value=SENTINEL\n" +
		"carriage\\rkey.value=SENTINEL\n" +
		"form\\fkey.value=SENTINEL\n" +
		"caf\\u00E9.value=SENTINEL\n" +
		"rocket\\uD83D\\uDE80.value=SENTINEL\n" +
		"client\\.name.value=SENTINEL\n" +
		"path\\\\:SENTINEL\n"
	wantGroups := []string{
		"tab\tkey",
		"line\nkey",
		"carriage\rkey",
		"form\fkey",
		"caf\u00E9",
		"rocket🚀",
		"client.name",
		"path\\",
	}

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	for index, name := range wantGroups {
		fact := findAgentContextConfigurationFact(t, facts, name)
		lineNumber := index + 1
		if fact.Line != lineNumber || fact.EndLine != lineNumber {
			t.Fatalf("%q key group range = %d-%d, want %d-%d", name, fact.Line, fact.EndLine, lineNumber, lineNumber)
		}
	}
}

func TestExtractAgentContextConfigurationFactsRejectsMalformedPropertiesUnicodeKeys(t *testing.T) {
	const path = "src/main/resources/application.properties"
	body := "invalid\\u12G4.value=SENTINEL\n" +
		"short\\u123.value=SENTINEL\n" +
		"high\\uD83D.value=SENTINEL\n" +
		"low\\uDE80.value=SENTINEL\n"

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	if len(facts) != 1 || facts[0].Name != "application.properties" {
		t.Fatalf("facts for malformed Unicode keys = %#v, want only the file fact", facts)
	}
}

func TestExtractAgentContextConfigurationFactsAcceptsSeparatorlessPropertiesKeys(t *testing.T) {
	const path = "src/main/resources/application.properties"
	body := "# comment\n" +
		"! comment\n" +
		" \t\f\n" +
		"client.enabled\n" +
		"retry.max-attempts\n"

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	clientFact := findAgentContextConfigurationFact(t, facts, "client")
	if clientFact.Line != 4 || clientFact.EndLine != 4 {
		t.Fatalf("client key group range = %d-%d, want 4-4", clientFact.Line, clientFact.EndLine)
	}
	retryFact := findAgentContextConfigurationFact(t, facts, "retry")
	if retryFact.Line != 5 || retryFact.EndLine != 5 {
		t.Fatalf("retry key group range = %d-%d, want 5-5", retryFact.Line, retryFact.EndLine)
	}
}

func TestExtractAgentContextConfigurationFactsJoinsPropertiesContinuationsWithPhysicalRanges(t *testing.T) {
	const path = "src/main/resources/application.properties"
	const sentinel = "SENTINEL_CLIENT_URL"
	body := "client.\\\n" +
		"  url=" + sentinel + "\n" +
		"retry.max-attempts=3\\\n" +
		" \t0\n" +
		"path\\\\\n" +
		"next.enabled\n"

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	if len(facts) != 5 {
		t.Fatalf("fact count = %d, want file fact plus four key groups: %#v", len(facts), facts)
	}
	wantRanges := map[string]agentContextConfigurationRange{
		"client": {start: 1, end: 2},
		"retry":  {start: 3, end: 4},
		"path\\": {start: 5, end: 5},
		"next":   {start: 6, end: 6},
	}
	for name, wantRange := range wantRanges {
		fact := findAgentContextConfigurationFact(t, facts, name)
		if fact.Line != wantRange.start || fact.EndLine != wantRange.end {
			t.Fatalf("%q key group range = %d-%d, want %d-%d", name, fact.Line, fact.EndLine, wantRange.start, wantRange.end)
		}
		if strings.Contains(fact.Name, sentinel) || strings.Contains(fact.Summary, sentinel) || strings.Contains(fact.Search, sentinel) {
			t.Fatalf("continued configuration fact retains value %q: %#v", sentinel, fact)
		}
	}
}

func TestExtractAgentContextConfigurationFactsBuildsLongPropertiesContinuationsWithBoundedAllocations(t *testing.T) {
	const path = "src/main/resources/application.properties"
	const continuationLines = 2048
	const segment = "segment"
	var body strings.Builder
	for range continuationLines - 1 {
		body.WriteString(segment + "\\\n")
	}
	body.WriteString(segment + ".value=SENTINEL\n")
	configurationBody := body.String()
	wantGroup := strings.Repeat(segment, continuationLines)

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, configurationBody)
	fact := findAgentContextConfigurationFact(t, facts, wantGroup)
	if fact.Line != 1 || fact.EndLine != continuationLines {
		t.Fatalf("continued key group range = %d-%d, want 1-%d", fact.Line, fact.EndLine, continuationLines)
	}

	allocations := testing.AllocsPerRun(3, func() {
		_ = extractAgentContextConfigurationFacts(FileRecord{Path: path}, configurationBody)
	})
	if allocations > 128 {
		t.Fatalf("allocations per extraction = %.0f, want <= 128", allocations)
	}
}

func TestExtractAgentContextConfigurationFactsHandlesManyThreeBackslashContinuations(t *testing.T) {
	const path = "src/main/resources/application.properties"
	const continuationLines = 4096
	continuation := strings.Repeat("\\", 3) + "\n"
	var body strings.Builder
	body.WriteString("client")
	for range continuationLines {
		body.WriteString(continuation)
	}
	body.WriteString(".value=SENTINEL\n")
	wantGroup := "client" + strings.Repeat("\\", continuationLines)

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body.String())
	fact := findAgentContextConfigurationFact(t, facts, wantGroup)
	if fact.Line != 1 || fact.EndLine != continuationLines+1 {
		t.Fatalf("continued key group range = %d-%d, want 1-%d", fact.Line, fact.EndLine, continuationLines+1)
	}
}

func TestExtractAgentContextConfigurationFactsSkipsContinuationAllocationsForEvenBackslashes(t *testing.T) {
	const path = "src/main/resources/application.properties"
	const physicalLines = 2048
	configurationBody := strings.Repeat("=\\\\\n", physicalLines)

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, configurationBody)
	if len(facts) != 1 || facts[0].Name != "application.properties" {
		t.Fatalf("facts for empty keys = %#v, want only the file fact", facts)
	}

	allocations := testing.AllocsPerRun(3, func() {
		_ = extractAgentContextConfigurationFacts(FileRecord{Path: path}, configurationBody)
	})
	if allocations > 128 {
		t.Fatalf("allocations per extraction = %.0f, want <= 128", allocations)
	}
}

func TestExtractAgentContextConfigurationFactsKeepsCommentsOutOfPropertiesContinuations(t *testing.T) {
	const path = "src/main/resources/application.properties"
	body := "# ignored\\\n" +
		"client.enabled\n" +
		"! ignored\\\n" +
		"retry.enabled\n" +
		"terminal\\"

	facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
	wantLines := map[string]int{
		"client":   2,
		"retry":    4,
		"terminal": 5,
	}
	for name, wantLine := range wantLines {
		fact := findAgentContextConfigurationFact(t, facts, name)
		if fact.Line != wantLine || fact.EndLine != wantLine {
			t.Fatalf("%q key group range = %d-%d, want %d-%d", name, fact.Line, fact.EndLine, wantLine, wantLine)
		}
	}
}

func TestExtractAgentContextConfigurationFactsLimitsAndSortsKeyGroups(t *testing.T) {
	var body strings.Builder
	for index := 39; index >= 0; index-- {
		fmt.Fprintf(&body, "group%02d.value=SENTINEL_%02d\n", index, index)
	}

	facts := extractAgentContextConfigurationFacts(
		FileRecord{Path: "src/test/resources/application-test.properties"},
		body.String(),
	)
	if len(facts) != 33 {
		t.Fatalf("fact count = %d, want one file fact plus 32 key groups", len(facts))
	}
	groups := make([]string, 0, len(facts)-1)
	for _, fact := range facts {
		if fact.Name != "application-test.properties" {
			groups = append(groups, fact.Name)
		}
		if strings.Contains(fact.Search, "SENTINEL_") || strings.Contains(fact.Summary, "SENTINEL_") {
			t.Fatalf("configuration fact retains sentinel value: %#v", fact)
		}
	}
	if !slices.IsSorted(groups) || groups[0] != "group00" || groups[len(groups)-1] != "group31" {
		t.Fatalf("key groups = %#v, want sorted bounded groups", groups)
	}
}

func TestAppendAgentContextConfigurationFactsSortsIndexWithoutValues(t *testing.T) {
	const sentinel = "SENTINEL_CONFIG_PASSWORD"
	facts := extractAgentContextConfigurationFacts(
		FileRecord{Path: "src/main/resources/application.properties"},
		"client.password="+sentinel+"\n",
	)
	index := appendAgentContextConfigurationFacts(AgentContextIndexRecord{
		Facts: []AgentContextFactRecord{{
			ID: "route", Project: "demo", Kind: "route", Name: "GET /jobs", Qualified: "GET /jobs",
		}},
	}, "demo", facts)

	if len(index.Facts) != 3 {
		t.Fatalf("index facts = %#v", index.Facts)
	}
	for _, fact := range index.Facts {
		if fact.Project != "demo" {
			t.Fatalf("configuration project = %q, want demo", fact.Project)
		}
		if strings.Contains(fact.Name, sentinel) || strings.Contains(fact.Summary, sentinel) || strings.Contains(fact.Search, sentinel) {
			t.Fatalf("indexed configuration fact retains sentinel: %#v", fact)
		}
	}
	if index.Facts[0].Kind != "configuration" || index.Facts[0].Name != "client" || index.Facts[1].Name != "application.properties" || index.Facts[2].Kind != "route" {
		t.Fatalf("sorted index facts = %#v", index.Facts)
	}
}

func findAgentContextConfigurationFact(t *testing.T, facts []AgentContextFactRecord, name string) AgentContextFactRecord {
	t.Helper()
	for _, fact := range facts {
		if fact.Name == name {
			return fact
		}
	}
	t.Fatalf("configuration fact %q missing from %#v", name, facts)
	return AgentContextFactRecord{}
}
