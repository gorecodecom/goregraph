// Package testresults imports explicit JUnit evidence without running project code.
package testresults

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorecodecom/goregraph/internal/outputstore"
)

const File = "test-results/results.json"

const (
	maxReports     = 100
	maxReportBytes = 8 << 20
	maxTotalBytes  = 64 << 20
	maxStoreBytes  = 4 << 20
	maxSuites      = 64
	maxCases       = 100000
)

type Counts struct {
	Tests    int `json:"tests"`
	Passed   int `json:"passed"`
	Failures int `json:"failures"`
	Errors   int `json:"errors"`
	Skipped  int `json:"skipped"`
}

type Report struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	Timestamp string `json:"timestamp,omitempty"`
	Shard     int    `json:"shard,omitempty"`
	Counts    Counts `json:"counts"`
}

type Run struct {
	Suite          string   `json:"suite"`
	Origin         string   `json:"origin"`
	Commit         string   `json:"commit,omitempty"`
	Branch         string   `json:"branch,omitempty"`
	Pipeline       string   `json:"pipeline,omitempty"`
	Job            string   `json:"job,omitempty"`
	ImportedAt     string   `json:"imported_at"`
	ExpectedShards int      `json:"expected_shards,omitempty"`
	Complete       bool     `json:"complete"`
	Status         string   `json:"status"`
	Counts         Counts   `json:"counts"`
	Reports        []Report `json:"reports"`
}

type Record struct {
	Version int   `json:"version"`
	Runs    []Run `json:"runs"`
}

type Options struct {
	Suite          string
	Origin         string
	Commit         string
	Branch         string
	Pipeline       string
	Job            string
	ExpectedShards int
}

var shardSuffix = regexp.MustCompile(`-(\d+)\.xml$`)

// Parse validates all selected reports before any output is changed.
func Parse(paths []string, options Options) (Run, error) {
	options.Suite = strings.TrimSpace(options.Suite)
	if options.Suite == "" || len(options.Suite) > 120 || strings.ContainsAny(options.Suite, "\r\n\t") {
		return Run{}, errors.New("--suite must be a short, non-empty name")
	}
	if options.Origin == "" {
		options.Origin = "local"
	}
	if options.Origin != "local" && options.Origin != "ci-artifact" {
		return Run{}, errors.New("--origin must be local or ci-artifact")
	}
	for label, value := range map[string]string{"commit": options.Commit, "branch": options.Branch, "pipeline": options.Pipeline, "job": options.Job} {
		if len(value) > 256 || strings.ContainsAny(value, "\r\n\t") {
			return Run{}, fmt.Errorf("--%s is too long or contains control characters", label)
		}
	}
	if len(paths) == 0 || len(paths) > maxReports {
		return Run{}, fmt.Errorf("select 1–%d JUnit files", maxReports)
	}
	if options.ExpectedShards < 0 || options.ExpectedShards > maxReports {
		return Run{}, fmt.Errorf("--expected-shards must be 1–%d when provided", maxReports)
	}
	if len(paths) > 1 && options.ExpectedShards == 0 {
		return Run{}, errors.New("multiple reports require --expected-shards")
	}
	run := Run{Suite: options.Suite, Origin: options.Origin, Commit: options.Commit, Branch: options.Branch, Pipeline: options.Pipeline, Job: options.Job, ImportedAt: time.Now().UTC().Format(time.RFC3339), ExpectedShards: options.ExpectedShards, Reports: make([]Report, 0, len(paths))}
	seenPaths, seenShards := map[string]bool{}, map[int]bool{}
	var firstTime, lastTime time.Time
	var totalBytes int64
	complete := true
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return Run{}, err
		}
		if seenPaths[absolute] {
			return Run{}, fmt.Errorf("duplicate report: %s", path)
		}
		seenPaths[absolute] = true
		body, err := readBoundedRegularFile(absolute, maxReportBytes)
		if err != nil {
			return Run{}, fmt.Errorf("%s: %w", path, err)
		}
		totalBytes += int64(len(body))
		if totalBytes > maxTotalBytes {
			return Run{}, fmt.Errorf("report set exceeds %d bytes", maxTotalBytes)
		}
		counts, timestamp, err := parseJUnit(body)
		if err != nil {
			return Run{}, fmt.Errorf("%s: %w", path, err)
		}
		sum := sha256.Sum256(body)
		report := Report{Name: filepath.Base(path), SHA256: hex.EncodeToString(sum[:]), Timestamp: timestamp, Counts: counts}
		if options.ExpectedShards > 1 {
			match := shardSuffix.FindStringSubmatch(report.Name)
			if len(match) != 2 {
				complete = false
			} else {
				report.Shard, _ = strconv.Atoi(match[1])
				if report.Shard < 1 || report.Shard > options.ExpectedShards || seenShards[report.Shard] {
					complete = false
				}
				seenShards[report.Shard] = true
			}
			if timestamp == "" {
				complete = false
			} else if parsed, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
				if firstTime.IsZero() || parsed.Before(firstTime) {
					firstTime = parsed
				}
				if lastTime.IsZero() || parsed.After(lastTime) {
					lastTime = parsed
				}
			}
		}
		run.Reports = append(run.Reports, report)
		run.Counts.Tests += counts.Tests
		run.Counts.Passed += counts.Passed
		run.Counts.Failures += counts.Failures
		run.Counts.Errors += counts.Errors
		run.Counts.Skipped += counts.Skipped
		if run.Counts.Tests > maxCases {
			return Run{}, fmt.Errorf("report set exceeds %d test cases", maxCases)
		}
	}
	if options.ExpectedShards > 0 && len(paths) != options.ExpectedShards {
		complete = false
	}
	if !firstTime.IsZero() && lastTime.Sub(firstTime) > 6*time.Hour {
		complete = false
	}
	if run.Counts.Tests == 0 {
		complete = false
	}
	run.Complete = complete
	sort.Slice(run.Reports, func(i, j int) bool {
		if run.Reports[i].Shard != run.Reports[j].Shard {
			return run.Reports[i].Shard < run.Reports[j].Shard
		}
		return run.Reports[i].Name < run.Reports[j].Name
	})
	switch {
	case !complete:
		run.Status = "incomplete"
	case run.Counts.Failures+run.Counts.Errors > 0:
		run.Status = "failed"
	case run.Counts.Skipped == run.Counts.Tests:
		run.Status = "skipped"
	default:
		run.Status = "passed"
	}
	return run, nil
}

func parseJUnit(body []byte) (Counts, string, error) {
	if len(body) > maxReportBytes {
		return Counts{}, "", errors.New("JUnit XML exceeds size limit")
	}
	decoder := xml.NewDecoder(strings.NewReader(string(body)))
	var counts Counts
	var timestamp string
	declared := map[string]int{}
	depth, roots := 0, 0
	inCase, failed, errored, skipped := false, false, false, false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Counts{}, "", fmt.Errorf("invalid JUnit XML: %w", err)
		}
		switch item := token.(type) {
		case xml.Directive:
			return Counts{}, "", errors.New("JUnit XML directives are not allowed")
		case xml.StartElement:
			depth++
			if depth > 32 {
				return Counts{}, "", errors.New("JUnit XML nesting limit exceeded")
			}
			if depth == 1 {
				if item.Name.Local != "testsuite" && item.Name.Local != "testsuites" {
					return Counts{}, "", errors.New("expected testsuite or testsuites root")
				}
				roots++
				for _, attribute := range item.Attr {
					switch attribute.Name.Local {
					case "tests", "failures", "errors", "skipped":
						value, parseErr := strconv.Atoi(attribute.Value)
						if parseErr != nil || value < 0 {
							return Counts{}, "", errors.New("invalid JUnit summary counts")
						}
						declared[attribute.Name.Local] = value
					}
				}
			}
			if item.Name.Local == "testsuite" && timestamp == "" {
				for _, attribute := range item.Attr {
					if attribute.Name.Local == "timestamp" {
						if parsed, err := time.Parse(time.RFC3339Nano, attribute.Value); err == nil {
							timestamp = parsed.UTC().Format(time.RFC3339Nano)
						}
					}
				}
			}
			if item.Name.Local == "testcase" {
				if inCase {
					return Counts{}, "", errors.New("nested testcase")
				}
				inCase, failed, errored, skipped = true, false, false, false
			}
			if !inCase && (item.Name.Local == "failure" || item.Name.Local == "error" || item.Name.Local == "skipped") {
				return Counts{}, "", errors.New("JUnit outcome outside a testcase")
			}
			if inCase {
				switch item.Name.Local {
				case "failure":
					failed = true
				case "error":
					errored = true
				case "skipped":
					skipped = true
				}
			}
		case xml.EndElement:
			if item.Name.Local == "testcase" && inCase {
				counts.Tests++
				switch {
				case errored:
					counts.Errors++
				case failed:
					counts.Failures++
				case skipped:
					counts.Skipped++
				default:
					counts.Passed++
				}
				if counts.Tests > maxCases {
					return Counts{}, "", fmt.Errorf("JUnit XML exceeds %d cases", maxCases)
				}
				inCase = false
			}
			depth--
		}
	}
	if roots != 1 || depth != 0 {
		return Counts{}, "", errors.New("invalid JUnit XML root")
	}
	actual := map[string]int{"tests": counts.Tests, "failures": counts.Failures, "errors": counts.Errors, "skipped": counts.Skipped}
	for key, value := range declared {
		if actual[key] != value {
			return Counts{}, "", fmt.Errorf("JUnit %s summary does not match testcases", key)
		}
	}
	return counts, timestamp, nil
}

// Load returns a neutral empty record when no import exists.
func Load(outputRoot string) (Record, error) {
	path := filepath.Join(outputRoot, File)
	body, err := readBoundedRegularFile(path, maxStoreBytes)
	if errors.Is(err, os.ErrNotExist) {
		return Record{Version: 1, Runs: []Run{}}, nil
	}
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(body, &record); err != nil {
		return Record{}, err
	}
	if record.Version != 1 {
		return Record{}, fmt.Errorf("unsupported test-result version %d", record.Version)
	}
	return record, nil
}

func readBoundedRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("expected a regular file of at most %d bytes", limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return body, nil
}

// Save replaces the selected suite atomically within the generated output snapshot.
func Save(ctx context.Context, outputRoot string, run Run) error {
	if _, err := os.Stat(filepath.Join(outputRoot, "manifest.json")); err != nil {
		return fmt.Errorf("build GoreGraph output before importing results: %w", err)
	}
	return outputstore.Update(ctx, outputstore.UpdateRequest{Root: outputRoot, Write: func(stage string) error {
		record, err := Load(stage)
		if err != nil {
			return err
		}
		retained := record.Runs[:0]
		for _, old := range record.Runs {
			if old.Suite != run.Suite {
				retained = append(retained, old)
			}
		}
		if len(retained) >= maxSuites {
			return fmt.Errorf("test-result store is limited to %d suites", maxSuites)
		}
		record.Runs = append(retained, run)
		sort.Slice(record.Runs, func(i, j int) bool { return record.Runs[i].Suite < record.Runs[j].Suite })
		body, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return err
		}
		if len(body)+1 > maxStoreBytes {
			return fmt.Errorf("test-result store exceeds %d bytes", maxStoreBytes)
		}
		path := filepath.Join(stage, File)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, append(body, '\n'), 0o644)
	}})
}
