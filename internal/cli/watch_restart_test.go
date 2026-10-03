package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/watch"
)

func TestWatchRestartHelpAndInvalidAutostartOption(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"watch", "restart", "--help"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "goregraph watch restart .") {
		t.Fatalf("restart help failed: code=%d out=%s err=%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"watch", "restart", "--autostart", "off"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "supported only by watch start") {
		t.Fatalf("restart accepted an autostart change: code=%d err=%s", code, stderr.String())
	}
}

func TestWatchRestartRejectsModeChangeWithoutChangingSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOREGRAPH_WATCH_HOME", home)
	root, err := watch.Resolve(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(home, root.ID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"root": root.Path, "workspace": false, "autostart": true, "method": "launchd"})
	path := filepath.Join(directory, "setting.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"watch", "restart", root.Path, "--workspace"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "different project/workspace mode") {
		t.Fatalf("restart accepted a mode change: code=%d err=%s", code, stderr.String())
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(body, after) {
		t.Fatal("rejected restart changed settings")
	}
}
