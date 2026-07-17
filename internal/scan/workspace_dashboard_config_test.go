package scan

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
