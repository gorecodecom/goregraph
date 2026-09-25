package testresults

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func junit(cases string, timestamp string) string {
	return `<testsuites><testsuite name="suite" timestamp="` + timestamp + `">` + cases + `</testsuite></testsuites>`
}

func reportFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseJUnitOutcomesAndNeutralProvenance(t *testing.T) {
	dir := t.TempDir()
	path := reportFile(t, dir, "interactions.xml", junit(`<testcase name="ok"/><testcase name="bad"><failure>assertion</failure></testcase><testcase name="broken"><error>setup</error></testcase><testcase name="skip"><skipped/></testcase>`, "2026-09-25T05:56:14.663Z"))
	run, err := Parse([]string{path}, Options{Suite: "storybook", Commit: "declared-sha"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" || !run.Complete || run.Counts != (Counts{Tests: 4, Passed: 1, Failures: 1, Errors: 1, Skipped: 1}) {
		t.Fatalf("wrong outcome: %+v", run)
	}
	if run.Reports[0].Timestamp != "2026-09-25T05:56:14.663Z" || run.Reports[0].SHA256 == "" {
		t.Fatalf("missing report provenance: %+v", run.Reports[0])
	}
	if run.Commit != "declared-sha" || run.Origin != "local" {
		t.Fatalf("wrong caller metadata: %+v", run)
	}
}

func TestParseRequiresCompleteShardSetForAggregatePass(t *testing.T) {
	dir := t.TempDir()
	paths := []string{}
	for i := 1; i <= 20; i++ {
		paths = append(paths, reportFile(t, dir, "interactions-"+strconv.Itoa(i)+".xml", junit(`<testcase name="ok"/>`, "2026-09-25T05:56:14Z")))
	}
	complete, err := Parse(paths, Options{Suite: "storybook", ExpectedShards: 20})
	if err != nil || !complete.Complete || complete.Status != "passed" {
		t.Fatalf("complete set: %+v, %v", complete, err)
	}
	partial, err := Parse(paths[:19], Options{Suite: "storybook", ExpectedShards: 20})
	if err != nil || partial.Complete || partial.Status != "incomplete" {
		t.Fatalf("partial set: %+v, %v", partial, err)
	}
	if _, err := Parse(paths, Options{Suite: "storybook"}); err == nil {
		t.Fatal("multiple files without expected count were accepted")
	}
	if _, err := Parse([]string{paths[0], paths[0]}, Options{Suite: "storybook", ExpectedShards: 2}); err == nil {
		t.Fatal("duplicate file was accepted")
	}
	stale := reportFile(t, dir, "interactions-4.xml", junit(`<testcase name="ok"/>`, "2026-09-20T05:56:14Z"))
	mixed, err := Parse([]string{paths[0], paths[1], stale}, Options{Suite: "storybook", ExpectedShards: 20})
	if err != nil || mixed.Complete || mixed.Status != "incomplete" {
		t.Fatalf("mixed timestamps: %+v, %v", mixed, err)
	}
}

func TestParseRejectsUnsafeOrInvalidXML(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"doctype.xml":     `<!DOCTYPE testsuite [<!ENTITY x SYSTEM "file:///etc/passwd">]><testsuite><testcase name="a"/></testsuite>`,
		"broken.xml":      `<testsuite><testcase></testsuite>`,
		"other.xml":       `<html><testcase/></html>`,
		"false-green.xml": `<testsuite tests="1" errors="1"><testcase name="a"/></testsuite>`,
		"suite-error.xml": `<testsuite><error>runner failed</error><testcase name="a"/></testsuite>`,
	} {
		path := reportFile(t, dir, name, body)
		if _, err := Parse([]string{path}, Options{Suite: "test"}); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	oversized := reportFile(t, dir, "large.xml", strings.Repeat("x", maxReportBytes+1))
	if _, err := Parse([]string{oversized}, Options{Suite: "test"}); err == nil {
		t.Fatal("accepted oversized report")
	}
}

func TestSaveReplacesOnlyNamedSuiteAndPreservesOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "agent.txt"), []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, run := range []Run{{Suite: "storybook", Status: "passed"}, {Suite: "playwright", Status: "failed"}, {Suite: "storybook", Status: "incomplete"}} {
		if err := Save(context.Background(), root, run); err != nil {
			t.Fatal(err)
		}
	}
	record, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Runs) != 2 || record.Runs[0].Suite != "playwright" || record.Runs[1].Status != "incomplete" {
		t.Fatalf("unexpected saved runs: %+v", record)
	}
	body, err := os.ReadFile(filepath.Join(root, "agent.txt"))
	if err != nil || string(body) != "unchanged" {
		t.Fatalf("agent output changed: %q, %v", body, err)
	}
}
