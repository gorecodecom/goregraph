//go:build windows

package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

type consoleLauncherResult struct {
	Visible   bool
	Arguments []string
}

func TestWindowsScheduledTaskLauncherIsHiddenAndPreservesArguments(t *testing.T) {
	directory := t.TempDir()
	result := filepath.Join(directory, "launcher-result.json")
	launcher := filepath.Join(directory, "Wälder & O'Brien.js")
	arguments := []string{`C:\Wälder & Co\O'Brien\`, `%GOREGRAPH_LITERAL_PROBE%`, `quoted"value`, ""}
	commandArguments := append([]string{os.Args[0], "-test.run=^TestWindowsWatcherConsoleHelper$", "--"}, arguments...)
	if err := os.WriteFile(launcher, scheduledTaskLauncher(commandArguments), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(os.Getenv("SystemRoot"), "System32", "wscript.exe"), "//B", "//NoLogo", "//E:JScript", launcher)
	command.Env = append(os.Environ(), "GOREGRAPH_CONSOLE_TEST=launcher", "GOREGRAPH_CONSOLE_RESULT="+result, "GOREGRAPH_LITERAL_PROBE=unexpected expansion")
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 23 {
		t.Fatalf("launcher did not preserve child exit code: %v\n%s", err, output)
	}
	body, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	var actual consoleLauncherResult
	if err := json.Unmarshal(body, &actual); err != nil {
		t.Fatal(err)
	}
	if actual.Visible || !reflect.DeepEqual(actual.Arguments, arguments) {
		t.Fatalf("launcher showed a window or changed arguments: %+v", actual)
	}
}

func TestWindowsSupervisorWorkerHasNoConsole(t *testing.T) {
	result := filepath.Join(t.TempDir(), "console.txt")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsWatcherConsoleHelper$")
	command.Env = append(os.Environ(), "GOREGRAPH_CONSOLE_TEST=supervisor", "GOREGRAPH_CONSOLE_RESULT="+result)
	detach(command)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("detached supervisor failed: %v\n%s", err, output)
	}
	body, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "0" {
		t.Fatalf("supervised worker allocated a console window: %s", body)
	}
}

func TestWindowsWatcherConsoleHelper(t *testing.T) {
	role := os.Getenv("GOREGRAPH_CONSOLE_TEST")
	if role == "" {
		t.Skip("subprocess helper")
	}
	result := os.Getenv("GOREGRAPH_CONSOLE_RESULT")
	if role == "launcher" {
		window, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
		visible, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("IsWindowVisible").Call(window)
		body, err := json.Marshal(consoleLauncherResult{Visible: visible != 0, Arguments: os.Args[3:]})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(result, body, 0o600); err != nil {
			t.Fatal(err)
		}
		os.Exit(23)
	}
	if role == "worker" {
		window, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
		if err := os.WriteFile(result, []byte(fmt.Sprint(window)), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	root, executable, options, _ := supervisorFixture(t, false)
	options.command = func(string, Root) *exec.Cmd {
		command := exec.Command(os.Args[0], "-test.run=^TestWindowsWatcherConsoleHelper$")
		command.Env = append(os.Environ(), "GOREGRAPH_CONSOLE_TEST=worker")
		return command
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- supervise(ctx, root, executable, options) }()
	for {
		if _, err := os.Stat(result); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("worker did not report its console state")
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
