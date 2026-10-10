//go:build !windows

package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstalledMCPAdoptsReplacementWithoutLosingSession(t *testing.T) {
	if testing.Short() {
		t.Skip("builds two executable versions")
	}
	directory := t.TempDir()
	build := func(name string) string {
		t.Helper()
		path := filepath.Join(directory, name)
		command := exec.Command("go", "build", "-ldflags", "-X github.com/gorecodecom/goregraph/internal/version.Version="+name, "-o", path, "../../cmd/goregraph")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, output)
		}
		return path
	}
	before, after := build("before"), build("after")
	installed := filepath.Join(directory, "goregraph")
	replace := func(source string) {
		t.Helper()
		body, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		staged := installed + ".next"
		if err := os.WriteFile(staged, body, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(staged, installed); err != nil {
			t.Fatal(err)
		}
	}
	replace(before)
	command := exec.Command(installed, "mcp", "--protocol", "strict-v1")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := command.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close(); _ = command.Process.Kill(); _ = command.Wait() })
	lines := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	warnings := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(diagnostics)
		for scanner.Scan() {
			warnings <- scanner.Text()
		}
		close(warnings)
	}()
	read := func(id int, version string) {
		t.Helper()
		select {
		case line := <-lines:
			var reply struct {
				ID     int `json:"id"`
				Result struct {
					ServerInfo struct {
						Version string `json:"version"`
					} `json:"serverInfo"`
					Instructions string `json:"instructions"`
				} `json:"result"`
			}
			if err := json.Unmarshal([]byte(line), &reply); err != nil || reply.ID != id || reply.Result.ServerInfo.Version != version {
				t.Fatalf("reply=%s err=%v", line, err)
			}
			if reply.Result.Instructions != instructionsForProtocol("strict-v1") {
				t.Fatal("protocol options lost during handoff")
			}
		case <-time.After(15 * time.Second):
			t.Fatal("MCP response timed out")
		}
	}
	send := func(id int) {
		t.Helper()
		if _, err := fmt.Fprintf(input, "{\"id\":%d,\"method\":\"initialize\"}\n", id); err != nil {
			t.Fatal(err)
		}
	}
	send(1)
	read(1, "before")
	// An invalid download must not destroy the established connection.
	invalid := filepath.Join(directory, "invalid")
	if err := os.WriteFile(invalid, []byte("not an executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	replace(invalid)
	select {
	case warning := <-warnings:
		if !strings.Contains(warning, "MCP update pending") {
			t.Fatalf("warning=%s", warning)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("idle update check did not detect invalid replacement")
	}
	send(2)
	read(2, "before")
	// A partially received request must survive an idle handoff.
	if _, err := io.WriteString(input, `{"id":3,"method":`); err != nil {
		t.Fatal(err)
	}
	replace(after)
	time.Sleep(2200 * time.Millisecond)
	if _, err := io.WriteString(input, "\"initialize\"}\n{\"id\":4,\"method\":\"initialize\"}\n"); err != nil {
		t.Fatal(err)
	}
	read(3, "after")
	read(4, "after")
	_ = input.Close()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("MCP did not exit on EOF")
	}
}
