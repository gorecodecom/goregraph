package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agentguide"
	"github.com/gorecodecom/goregraph/internal/agentmetrics"
	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/version"
)

type generatedBlock struct {
	Name string
	Body string
}

type documentSpec struct {
	Path   string
	Blocks []generatedBlock
}

type pendingDocumentWrite struct {
	path string
	body string
	mode os.FileMode
}

var generatedMarkerPattern = regexp.MustCompile(
	`^<!-- goregraph:generated ([a-z0-9-]+) (start|end) -->$`,
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "--write" && args[0] != "--check" {
		fmt.Fprintln(stderr, "usage: go run ./scripts/sync-docs --write|--check")
		return 2
	}
	root, err := repositoryRoot()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	if err := synchronizeDocuments(root, repositoryDocumentSpecs(), args[0] == "--write"); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func repositoryRoot() (string, error) {
	_, sourcePath, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("resolve sync-docs source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(sourcePath), "..", ".."))
	if !filepath.IsAbs(root) {
		return "", errors.New("sync-docs repository root is not absolute")
	}
	return root, nil
}

func repositoryDocumentSpecs() []documentSpec {
	instruction := renderAgentInstruction()
	currentContract := renderCurrentContract()
	releaseEvidence := renderCurrentReleaseEvidenceStatus()
	return []documentSpec{
		{
			Path: "README.md",
			Blocks: []generatedBlock{
				{Name: "current-contract", Body: currentContract},
				{Name: "agent-instruction-quick-start", Body: instruction},
				{Name: "language-coverage", Body: renderLanguageCoverage()},
				{Name: "agent-instruction-reference", Body: instruction},
				{Name: "release-evidence-status", Body: releaseEvidence},
			},
		},
		{
			Path: "COMMANDS.md",
			Blocks: []generatedBlock{
				{Name: "agent-instruction-quick-start", Body: instruction},
				{Name: "agent-instruction-context", Body: instruction},
				{Name: "agent-instruction-mcp", Body: instruction},
			},
		},
		{
			Path: "SCHEMA.md",
			Blocks: []generatedBlock{
				{Name: "current-contract", Body: currentContract},
			},
		},
		{
			Path: "docs/OUTPUTS.md",
			Blocks: []generatedBlock{
				{Name: "current-contract", Body: currentContract},
				{Name: "agent-instruction", Body: instruction},
				{Name: "language-inventory", Body: renderLanguageInventorySummary()},
			},
		},
		{
			Path: "docs/BENCHMARKING.md",
			Blocks: []generatedBlock{
				{Name: "agent-instruction", Body: instruction},
				{Name: "agent-benchmark-metrics", Body: renderAgentBenchmarkMetrics()},
				{Name: "release-evidence-status", Body: releaseEvidence},
			},
		},
		{
			Path: "docs/RELEASE.md",
			Blocks: []generatedBlock{
				{Name: "current-contract", Body: currentContract},
				{Name: "agent-instruction", Body: instruction},
				{Name: "release-evidence-status", Body: releaseEvidence},
			},
		},
	}
}

func renderLanguageCoverage() string {
	var body strings.Builder
	body.WriteString("Coverage describes implemented static analyzers, not proof that runtime behavior is absent. **Full** adapters emit normalized symbols, relations, calls, routes, and tests for their supported syntax. **Pattern-backed** capabilities recognize only the listed static families. **Integration** and **Index** are intentionally shallower. `—` means unavailable.\n\n")
	body.WriteString("| Language / framework | Adapter | Symbols | Imports | Calls | Routes | Tests | API clients | Persistence | Messaging / RPC | Data flow | Exact symbols | Direct usages | HTTP reachability |\n")
	body.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	profiles := scan.LanguageCapabilityProfiles()
	for _, profile := range profiles {
		if profile.Language == "typescript" {
			continue
		}
		name := languageProfileDisplayName(profile)
		core := levelCapabilityLabel(profile.Level)
		fmt.Fprintf(
			&body,
			"| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			name,
			levelLabel(profile.Level),
			enabledLabel(profile.Symbols, core),
			enabledLabel(profile.Relations, core),
			enabledLabel(profile.Calls, core),
			enabledLabel(profile.Routes, core),
			enabledLabel(profile.Tests, core),
			enabledLabel(profile.APIClients, "Pattern-backed"),
			enabledLabel(profile.Persistence, "Pattern-backed"),
			enabledLabel(profile.Messaging, "Pattern-backed"),
			enabledLabel(profile.DataFlow, "Pattern-backed"),
			enabledLabel(profile.ExactSymbols, "Full"),
			enabledLabel(profile.DirectUsages, "Full"),
			httpReachabilityLabel(profile),
		)
	}
	body.WriteString("\nPattern-backed extraction can miss runtime-generated behavior such as routes, reflective or dynamic dispatch, metaprogramming, dependency-injection aliases, arbitrary client wrappers, ORM behavior assembled at runtime, and configuration outside indexed source. Missing static evidence is not proof of runtime absence.\n\n")
	body.WriteString("Shell integration does not provide routes, tests, or architecture capabilities. Index adapters provide best-effort declarations and imports only; they do not provide normalized calls, routes, tests, or architecture facts.\n\n")
	body.WriteString("Supported static pattern families:\n")
	for _, profile := range profiles {
		if len(profile.PatternFamilies) == 0 || profile.Language == "typescript" {
			continue
		}
		name := languageProfileDisplayName(profile)
		fmt.Fprintf(&body, "- **%s:** %s.\n", name, strings.Join(profile.PatternFamilies, "; "))
	}
	body.WriteString("\nFor HTTP reachability, **Provider** means a supported Java/Spring or Node.js provider chain. **Consumer + provider** means supported JavaScript/TypeScript frontend origins plus supported Node.js handlers. These are static, evidence-backed relationships, not runtime reachability guarantees.")
	return body.String()
}

func renderLanguageInventorySummary() string {
	var fullAdapters []string
	var indexAdapters []string
	for _, profile := range scan.LanguageCapabilityProfiles() {
		if profile.Language == "typescript" {
			continue
		}
		switch profile.Level {
		case "full":
			fullAdapters = append(fullAdapters, languageProfileDisplayName(profile))
		case "index":
			indexAdapters = append(indexAdapters, languageProfileDisplayName(profile))
		}
	}
	return fmt.Sprintf(
		"GoreGraph provides full adapters for %s. They emit normalized symbols, imports, calls, routes, tests, and pattern-backed architecture evidence for their supported static syntax.\n\n"+
			"Shell integration provides symbols, imports, and calls, but does not provide routes, tests, or architecture facts. Index adapters for %s provide best-effort declarations and imports only. All records share the Schema %d index.",
		humanList(fullAdapters),
		humanList(indexAdapters),
		scan.SchemaVersion,
	)
}

func renderAgentInstruction() string {
	return "```text\n" + agentguide.AssistedInstruction + "\n```"
}

func renderCurrentContract() string {
	return fmt.Sprintf(
		"Source version: GoreGraph %s with output Schema %d.",
		version.Version,
		scan.SchemaVersion,
	)
}

func renderCurrentReleaseEvidenceStatus() string {
	return "The last controlled three-by-three release benchmark passed for candidate 0edc6d8. " +
		"Effective-token medians were 142796 baseline and 20105 assisted, an 85.92% reduction; " +
		"mean effective tokens were 138549 baseline and 23000 assisted, an 83.40% reduction. " +
		"Tool-call medians were 28 and 3, and source-read medians were 19 and 2. All six runs had " +
		"zero external skill reads. The signed 12-point review scored baseline quality at a median " +
		"of 11 and assisted quality at 12, with every assisted run scoring 12/12. That result qualifies " +
		"the runtime candidate 0edc6d8 and the final release descendant, whose later changes are " +
		"confined to documentation, tests, and documentation-sync tooling. This evidence covers " +
		"one frozen historical three-repository Java case and is not a general token-savings guarantee."
}

func renderAgentBenchmarkMetrics() string {
	return "The standard release summary schema is:\n\n```text\n" +
		agentmetrics.ReleaseSummaryHeader +
		"\n```\n\nThe monotonic Golden-versus-candidate summary schema is:\n\n```text\n" +
		agentmetrics.RegressionSummaryHeader +
		"\n```\n\n`effective_tokens` is `input_tokens - cached_input_tokens + output_tokens` and is the prospective comparison metric. " +
		"It represents uncached input plus output. `total_tokens` is `input_tokens + output_tokens`. " +
		"`reasoning_output_tokens` is recorded separately, and reasoning output is already part of output, so it is not added again.\n\n" +
		"`external_skill_read_calls` counts transcript-observed read or search targets outside the benchmark workspace that resolve to a skill bundle. " +
		"Controlled baseline and assisted release runs require zero; normal GoreGraph workflows may continue to use task-scoped skills.\n\n" +
		"`source_read_calls` remains the total number of direct source-read terminal calls; Spring `.properties`, `.yml`, and `.yaml` configuration resources are included as source targets. " +
		"`bounded_omission_read_calls` counts exact ranged reads wholly authorized by an earlier full Context Pack. " +
		"`unauthorized_source_read_calls` counts every other source read, search, or inventory terminal call. " +
		"A compound call is bounded only when every source target is bounded, and included-source overlap is never bounded. " +
		"Bounded reads remain part of tool and token totals and cannot exceed the case contract's `max_source_omissions`. " +
		"The monotonic gate compares unauthorized reads; the matched release gate continues to compare total `source_read_calls`."
}

func levelLabel(level string) string {
	switch level {
	case "full":
		return "Full"
	case "partial":
		return "Integration"
	case "index":
		return "Index"
	default:
		return "Unavailable"
	}
}

func levelCapabilityLabel(level string) string {
	return levelLabel(level)
}

func enabledLabel(enabled bool, label string) string {
	if !enabled {
		return "—"
	}
	return label
}

func httpReachabilityLabel(profile scan.LanguageCapabilityProfile) string {
	switch {
	case profile.HTTPConsumer && profile.HTTPProvider:
		return "Consumer + provider"
	case profile.HTTPProvider:
		return "Provider"
	case profile.HTTPConsumer:
		return "Consumer"
	default:
		return "—"
	}
}

func languageProfileDisplayName(profile scan.LanguageCapabilityProfile) string {
	if profile.Language == "javascript" {
		return "JavaScript / TypeScript / Node.js / React"
	}
	return profile.DisplayName
}

func humanList(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	case 2:
		return values[0] + " and " + values[1]
	default:
		return strings.Join(values[:len(values)-1], ", ") + ", and " + values[len(values)-1]
	}
}

func synchronizeDocuments(root string, specs []documentSpec, write bool) error {
	pending := make([]pendingDocumentWrite, 0, len(specs))
	var failures []error
	for _, spec := range specs {
		path := filepath.Join(root, filepath.FromSlash(spec.Path))
		info, err := os.Stat(path)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", spec.Path, err))
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", spec.Path, err))
			continue
		}
		updated, stale, err := replaceGeneratedBlocks(string(body), spec.Blocks)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", spec.Path, err))
			continue
		}
		if !write {
			for _, name := range stale {
				failures = append(failures, fmt.Errorf("%s: block %s is stale", spec.Path, name))
			}
			continue
		}
		if len(stale) > 0 {
			pending = append(pending, pendingDocumentWrite{
				path: path, body: updated, mode: info.Mode().Perm(),
			})
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	for _, document := range pending {
		if err := os.WriteFile(document.path, []byte(document.body), document.mode); err != nil {
			return err
		}
	}
	return nil
}

func replaceGeneratedBlocks(body string, blocks []generatedBlock) (string, []string, error) {
	lineEnding := "\n"
	if strings.Contains(body, "\r\n") {
		lineEnding = "\r\n"
	}
	expected := make(map[string]generatedBlock, len(blocks))
	for _, block := range blocks {
		if block.Name == "" {
			return "", nil, errors.New("generated block name must not be empty")
		}
		if _, duplicate := expected[block.Name]; duplicate {
			return "", nil, fmt.Errorf("duplicate generated block specification %s", block.Name)
		}
		expected[block.Name] = block
	}

	type markerCounts struct {
		start int
		end   int
	}
	counts := make(map[string]markerCounts, len(blocks))
	for _, line := range strings.Split(body, "\n") {
		match := generatedMarkerPattern.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if match == nil {
			continue
		}
		name, kind := match[1], match[2]
		if _, known := expected[name]; !known {
			return "", nil, fmt.Errorf("unknown generated block %s", name)
		}
		count := counts[name]
		if kind == "start" {
			count.start++
		} else {
			count.end++
		}
		counts[name] = count
	}

	type replacement struct {
		start   int
		end     int
		desired string
		name    string
	}
	replacements := make([]replacement, 0, len(blocks))
	for _, block := range blocks {
		count := counts[block.Name]
		switch {
		case count.start == 0:
			return "", nil, fmt.Errorf("missing generated block %s", block.Name)
		case count.start > 1 || count.end > 1:
			return "", nil, fmt.Errorf("duplicate generated block %s", block.Name)
		case count.end == 0:
			return "", nil, fmt.Errorf("missing end marker for generated block %s", block.Name)
		}
		startMarker := generatedMarker(block.Name, "start")
		endMarker := generatedMarker(block.Name, "end")
		start := strings.Index(body, startMarker)
		endRelative := strings.Index(body[start+len(startMarker):], endMarker)
		if endRelative < 0 {
			return "", nil, fmt.Errorf("missing end marker for generated block %s", block.Name)
		}
		end := start + len(startMarker) + endRelative + len(endMarker)
		blockBody := strings.ReplaceAll(strings.TrimRight(block.Body, "\r\n"), "\r\n", "\n")
		blockBody = strings.ReplaceAll(blockBody, "\n", lineEnding)
		desired := startMarker + lineEnding + blockBody + lineEnding + endMarker
		replacements = append(replacements, replacement{
			start: start, end: end, desired: desired, name: block.Name,
		})
	}

	stale := make([]string, 0, len(replacements))
	for index := len(replacements) - 1; index >= 0; index-- {
		current := replacements[index]
		if body[current.start:current.end] == current.desired {
			continue
		}
		stale = append(stale, current.name)
		body = body[:current.start] + current.desired + body[current.end:]
	}
	for left, right := 0, len(stale)-1; left < right; left, right = left+1, right-1 {
		stale[left], stale[right] = stale[right], stale[left]
	}
	return body, stale, nil
}

func generatedMarker(name, kind string) string {
	return "<!-- goregraph:generated " + name + " " + kind + " -->"
}
