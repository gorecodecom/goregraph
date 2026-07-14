package scan

import (
	"path/filepath"
	"sort"
	"strings"
)

type TestLinkRecord struct {
	ID          string   `json:"id"`
	Relation    string   `json:"relation"`
	TestFile    string   `json:"test_file,omitempty"`
	TestName    string   `json:"test_name,omitempty"`
	Line        int      `json:"line,omitempty"`
	Confidence  string   `json:"confidence"`
	Reason      string   `json:"reason"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type VerificationCommandRecord struct {
	Tool                string   `json:"tool,omitempty"`
	WorkingDirectory    string   `json:"working_directory,omitempty"`
	Args                []string `json:"args,omitempty"`
	Display             string   `json:"display,omitempty"`
	TestFile            string   `json:"test_file,omitempty"`
	TestName            string   `json:"test_name,omitempty"`
	Confidence          string   `json:"confidence"`
	Reason              string   `json:"reason"`
	MissingPrerequisite string   `json:"missing_prerequisite,omitempty"`
}

func BuildTestLinks(flow WorkspaceFeatureFlowRecord) []TestLinkRecord {
	if len(flow.Tests) == 0 {
		return []TestLinkRecord{{ID: StableWorkspaceID("test-link", flow.ID, "not-detected"), Relation: "not_detected", Confidence: string(ConfidenceUnknown), Reason: firstNonEmpty(flow.TestReason, "No linked test detected; analyzer coverage may be incomplete.")}}
	}
	links := make([]TestLinkRecord, 0, len(flow.Tests))
	for _, test := range flow.Tests {
		confidence := strings.ToUpper(firstNonEmpty(test.Confidence, string(ConfidenceUnknown)))
		relation := "indirect"
		switch {
		case strings.Contains(confidence, "WEAK"):
			relation = "candidate"
		case confidence == "INFERRED":
			relation = "inferred"
		case confidence == "EXACT" || confidence == "EXTRACTED" || confidence == "MATCHED" || confidence == "RESOLVED":
			relation = "direct"
		}
		name := firstNonEmpty(test.TestMethod, test.TestClass, test.TestCase, test.TestFile)
		links = append(links, TestLinkRecord{ID: StableWorkspaceID("test-link", flow.ID, test.TestFile, name, relation), Relation: relation, TestFile: filepath.ToSlash(test.TestFile), TestName: name, Line: test.Line, Confidence: confidence, Reason: firstNonEmpty(test.Reason, "Linked from the generated test map.")})
	}
	sort.Slice(links, func(i, j int) bool { return links[i].ID < links[j].ID })
	return links
}

func BuildVerificationCommands(project WorkspaceProjectRecord, tests []TestMapRecord) []VerificationCommandRecord {
	runner := strings.ToLower(firstNonEmpty(project.TestRunner, project.BuildSystem, project.Kind))
	workingDirectory := filepath.ToSlash(project.Path)
	if len(tests) == 0 {
		return nil
	}
	commands := make([]VerificationCommandRecord, 0, len(tests))
	for _, test := range tests {
		file := filepath.ToSlash(test.TestFile)
		name := firstNonEmpty(test.TestMethod, test.TestClass, test.TestCase)
		confidence := firstNonEmpty(test.Confidence, string(ConfidenceUnknown))
		record := VerificationCommandRecord{Tool: runner, WorkingDirectory: workingDirectory, TestFile: file, TestName: name, Confidence: confidence, Reason: "Derived from detected " + runner + " project metadata and a linked test record."}
		switch runner {
		case "maven":
			selector := firstNonEmpty(test.TestClass, strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
			if test.TestMethod != "" {
				selector += "#" + test.TestMethod
			}
			record.Args = []string{"-Dtest=" + selector, "test"}
			record.Display = "mvn " + strings.Join(record.Args, " ")
		case "gradle":
			selector := qualifiedFlowName(test.TestClass, test.TestMethod)
			if selector == "" {
				selector = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
			}
			record.Args = []string{"test", "--tests", selector}
			record.Display = "./gradlew test --tests " + quoteDisplayArgument(selector)
		case "jest":
			record.Args = []string{file}
			record.Display = "npx jest " + quoteDisplayArgument(file)
		case "vitest":
			record.Args = []string{"run", file}
			record.Display = "npx vitest run " + quoteDisplayArgument(file)
		case "playwright":
			record.Args = []string{"test", file}
			record.Display = "npx playwright test " + quoteDisplayArgument(file)
		default:
			record.Tool = ""
			record.Reason = "No supported test runner was detected from project metadata."
			record.MissingPrerequisite = "Detect Maven, Gradle, Jest, Vitest, or Playwright project metadata before deriving a command."
		}
		commands = append(commands, record)
	}
	return commands
}

func quoteDisplayArgument(value string) string {
	if strings.ContainsAny(value, " \t\"") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}
