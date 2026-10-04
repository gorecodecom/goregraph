package watch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func startManaged(root Root, executable string) (bool, error) {
	switch runtime.GOOS {
	case "darwin":
		return true, startLaunchAgent(root, executable)
	case "linux":
		if _, err := exec.LookPath("systemctl"); err != nil || exec.Command("systemctl", "--user", "show-environment").Run() != nil {
			return false, nil
		}
		base, err := os.UserConfigDir()
		if err != nil {
			return true, err
		}
		dir := filepath.Join(base, "systemd", "user")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return true, err
		}
		name := "goregraph-watch-" + root.ID + ".service"
		parts := append([]string{executable}, supervisedArguments(root)...)
		for i := range parts {
			parts[i] = systemdArgument(parts[i])
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(systemdServiceBody(root, parts)), 0o600); err != nil {
			return true, err
		}
		for _, args := range [][]string{{"--user", "daemon-reload"}, {"--user", "start", name}} {
			if output, err := exec.Command("systemctl", args...).CombinedOutput(); err != nil {
				return true, fmt.Errorf("start user service: %v: %s", err, strings.TrimSpace(string(output)))
			}
		}
		return true, nil
	default:
		return false, nil
	}
}

func startLaunchAgent(root Root, executable string) error {
	directory, err := stateDir(root)
	if err != nil {
		return err
	}
	path := filepath.Join(directory, "launch-agent.plist")
	saved, err := loadSetting(root)
	if err != nil {
		return err
	}
	if saved.Autostart {
		if _, err := enableLaunchAgent(root, executable); err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, "Library", "LaunchAgents", "com.gorecode.goregraph.watch."+root.ID+".plist")
	} else {
		body, err := launchAgentBody(root, executable)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			return err
		}
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	service := domain + "/com.gorecode.goregraph.watch." + root.ID
	if exec.Command("launchctl", "print", service).Run() == nil {
		if output, err := exec.Command("launchctl", "bootout", service).CombinedOutput(); err != nil {
			return fmt.Errorf("unload stopped watcher service: %v: %s", err, strings.TrimSpace(string(output)))
		}
	}
	if output, err := exec.Command("launchctl", "bootstrap", domain, path).CombinedOutput(); err != nil {
		return fmt.Errorf("load watcher service: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
