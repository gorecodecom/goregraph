package scan

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	dashboardConfigTestWorkerRootEnv     = "GOREGRAPH_DASHBOARD_CONFIG_TEST_ROOT"
	dashboardConfigTestWorkerRevisionEnv = "GOREGRAPH_DASHBOARD_CONFIG_TEST_REVISION"
	dashboardConfigTestWorkerIDEnv       = "GOREGRAPH_DASHBOARD_CONFIG_TEST_WORKER_ID"
)

func TestWorkspaceDashboardConfigRoundTripAndConflict(t *testing.T) {
	root := t.TempDir()
	config := WorkspaceDashboardConfig{Schema: 1, Architecture: DashboardArchitectureConfig{
		GroupOrder: []string{"org.example.alpha"},
		Groups:     map[string]DashboardGroupConfig{"org.example.alpha": {Label: "Alpha"}},
		Services:   map[string]DashboardServiceConfig{"services/api": {Group: "org.example.alpha", Order: 20}},
	}}
	revision, err := SaveWorkspaceDashboardConfig(root, "missing", config)
	if err != nil {
		t.Fatal(err)
	}
	loaded, loadedRevision, err := LoadWorkspaceDashboardConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if revision != loadedRevision || !reflect.DeepEqual(config, loaded) {
		t.Fatalf("round trip mismatch: revision=%q loaded=%#v", loadedRevision, loaded)
	}
	if _, err := SaveWorkspaceDashboardConfig(root, "missing", config); !errors.Is(err, ErrDashboardConfigConflict) {
		t.Fatalf("stale save error = %v", err)
	}
}

func TestWorkspaceDashboardConfigRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, WorkspaceDashboardConfigName)
	if err := os.WriteFile(path, []byte(`{"schema":1,"architecture":{},"secret":"no"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadWorkspaceDashboardConfig(root); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
}

func TestLoadWorkspaceDashboardConfigReturnsDefaultForAbsentFile(t *testing.T) {
	config, revision, err := LoadWorkspaceDashboardConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(config, WorkspaceDashboardConfig{Schema: 1}) {
		t.Fatalf("config = %#v", config)
	}
	if revision != missingDashboardConfigRevision {
		t.Fatalf("revision = %q", revision)
	}
}

func TestLoadWorkspaceDashboardConfigRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"missing schema", `{"architecture":{}}`},
		{"unsupported schema", `{"schema":2,"architecture":{}}`},
		{"trailing JSON", `{"schema":1,"architecture":{}} {}`},
		{"blank group ID", `{"schema":1,"architecture":{"groups":{" ":{"label":"Alpha"}}}}`},
		{"blank group label", `{"schema":1,"architecture":{"groups":{"org.example.alpha":{"label":" "}}}}`},
		{"duplicate group order", `{"schema":1,"architecture":{"groupOrder":["org.example.alpha","org.example.alpha"],"groups":{"org.example.alpha":{"label":"Alpha"}}}}`},
		{"missing service group", `{"schema":1,"architecture":{"services":{"services/api":{"group":"org.example.alpha"}}}}`},
		{"empty service ID", `{"schema":1,"architecture":{"groups":{"org.example.alpha":{"label":"Alpha"}},"services":{"":{"group":"org.example.alpha"}}}}`},
		{"absolute service ID", `{"schema":1,"architecture":{"groups":{"org.example.alpha":{"label":"Alpha"}},"services":{"/services/api":{"group":"org.example.alpha"}}}}`},
		{"traversing service ID", `{"schema":1,"architecture":{"groups":{"org.example.alpha":{"label":"Alpha"}},"services":{"../services/api":{"group":"org.example.alpha"}}}}`},
		{"backslash-traversing service ID", `{"schema":1,"architecture":{"groups":{"org.example.alpha":{"label":"Alpha"}},"services":{"services\\\\..\\\\api":{"group":"org.example.alpha"}}}}`},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, WorkspaceDashboardConfigName), []byte(testCase.data), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := LoadWorkspaceDashboardConfig(root); err == nil {
				t.Fatal("LoadWorkspaceDashboardConfig succeeded")
			}
		})
	}
}

func TestSaveWorkspaceDashboardConfigAllowsOnlyOneConcurrentWriter(t *testing.T) {
	if root := os.Getenv(dashboardConfigTestWorkerRootEnv); root != "" {
		runDashboardConfigSaveWorker(t, root)
		return
	}

	root := t.TempDir()
	config := WorkspaceDashboardConfig{Schema: 1, Architecture: DashboardArchitectureConfig{
		Groups: map[string]DashboardGroupConfig{"org.example.alpha": {Label: "Alpha"}},
	}}
	revision, err := SaveWorkspaceDashboardConfig(root, missingDashboardConfigRevision, config)
	if err != nil {
		t.Fatal(err)
	}

	const writerCount = 8
	workers := make([]*exec.Cmd, 0, writerCount)
	outputs := make([]bytes.Buffer, writerCount)
	for writer := 0; writer < writerCount; writer++ {
		command := exec.Command(os.Args[0], "-test.run=^TestSaveWorkspaceDashboardConfigAllowsOnlyOneConcurrentWriter$")
		command.Stdout = &outputs[writer]
		command.Stderr = &outputs[writer]
		command.Env = append(os.Environ(),
			dashboardConfigTestWorkerRootEnv+"="+root,
			dashboardConfigTestWorkerRevisionEnv+"="+revision,
			dashboardConfigTestWorkerIDEnv+"="+strconv.Itoa(writer),
		)
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		workers = append(workers, command)
	}
	if err := waitForDashboardConfigWorkers(root, writerCount); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dashboard-config-start"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	successes := 0
	conflicts := 0
	for workerIndex, worker := range workers {
		err := worker.Wait()
		output := outputs[workerIndex].Bytes()
		if err != nil {
			t.Fatalf("worker failed: %v\n%s", err, output)
		}
		switch {
		case strings.Contains(string(output), "result=success"):
			successes++
		case strings.Contains(string(output), "result=conflict"):
			conflicts++
		default:
			t.Fatalf("worker output = %q", output)
		}
	}
	if successes != 1 || conflicts != writerCount-1 {
		t.Fatalf("successes = %d, conflicts = %d", successes, conflicts)
	}
	for writer := 0; writer < writerCount; writer++ {
		if err := os.Remove(filepath.Join(root, fmt.Sprintf(".dashboard-config-ready-%d", writer))); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(root, ".dashboard-config-start")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != WorkspaceDashboardConfigName {
		t.Fatalf("workspace files = %#v", entries)
	}
}

func runDashboardConfigSaveWorker(t *testing.T, root string) {
	workerID, err := strconv.Atoi(os.Getenv(dashboardConfigTestWorkerIDEnv))
	if err != nil {
		t.Fatal(err)
	}
	readyPath := filepath.Join(root, fmt.Sprintf(".dashboard-config-ready-%d", workerID))
	if err := os.WriteFile(readyPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := waitForDashboardConfigStart(root); err != nil {
		t.Fatal(err)
	}
	config := WorkspaceDashboardConfig{Schema: 1, Architecture: DashboardArchitectureConfig{
		Groups: map[string]DashboardGroupConfig{
			"org.example.alpha": {Label: fmt.Sprintf("Alpha %d", workerID)},
		},
	}}
	_, err = SaveWorkspaceDashboardConfig(root, os.Getenv(dashboardConfigTestWorkerRevisionEnv), config)
	if err == nil {
		fmt.Fprintln(os.Stdout, "result=success")
		return
	}
	if errors.Is(err, ErrDashboardConfigConflict) {
		fmt.Fprintln(os.Stdout, "result=conflict")
		return
	}
	t.Fatal(err)
}

func waitForDashboardConfigWorkers(root string, workerCount int) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		ready := 0
		for worker := 0; worker < workerCount; worker++ {
			if _, err := os.Stat(filepath.Join(root, fmt.Sprintf(".dashboard-config-ready-%d", worker))); err == nil {
				ready++
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if ready == workerCount {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("only %d of %d dashboard configuration workers became ready", ready, workerCount)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForDashboardConfigStart(root string) error {
	deadline := time.Now().Add(5 * time.Second)
	startPath := filepath.Join(root, ".dashboard-config-start")
	for {
		if _, err := os.Stat(startPath); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("dashboard configuration test start timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
