package watch

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
)

func xmlValue(value string) string {
	var body bytes.Buffer
	_ = xml.EscapeText(&body, []byte(value))
	return body.String()
}

func runArguments(root Root) []string {
	args := []string{"watch", "run", root.Path}
	if root.Workspace {
		args = append(args, "--workspace")
	}
	return args
}

func enableAutostart(root Root, executable string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return enableLaunchAgent(root, executable)
	case "windows":
		return enableScheduledTask(root, executable)
	case "linux":
		return enableLinuxAutostart(root, executable)
	default:
		return "", fmt.Errorf("autostart is unsupported on %s", runtime.GOOS)
	}
}

func disableAutostart(root Root, method string) error {
	switch method {
	case "", "none":
		return nil
	case "launchd":
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path := filepath.Join(home, "Library", "LaunchAgents", "com.gorecode.goregraph.watch."+root.ID+".plist")
		err = os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	case "taskscheduler":
		output, err := exec.Command("schtasks", "/Delete", "/TN", taskName(root), "/F").CombinedOutput()
		if err != nil {
			return fmt.Errorf("remove scheduled task: %v: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	case "systemd":
		name := "goregraph-watch-" + root.ID + ".service"
		output, err := exec.Command("systemctl", "--user", "disable", name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("disable user service: %v: %s", err, strings.TrimSpace(string(output)))
		}
		base, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(base, "systemd", "user", name)); err != nil && !os.IsNotExist(err) {
			return err
		}
		output, err = exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput()
		if err != nil {
			return fmt.Errorf("reload user services: %v: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	case "xdg":
		base, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		err = os.Remove(filepath.Join(base, "autostart", "goregraph-watch-"+root.ID+".desktop"))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	default:
		return fmt.Errorf("unknown autostart method %q", method)
	}
}

func enableLaunchAgent(root Root, executable string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	label := "com.gorecode.goregraph.watch." + root.ID
	var args strings.Builder
	for _, arg := range append([]string{executable}, runArguments(root)...) {
		fmt.Fprintf(&args, "    <string>%s</string>\n", xmlValue(arg))
	}
	body := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n" +
		"<plist version=\"1.0\"><dict>\n" +
		"  <key>Label</key><string>" + label + "</string>\n" +
		"  <key>ProgramArguments</key><array>\n" + args.String() + "  </array>\n" +
		"  <key>RunAtLoad</key><true/>\n" +
		"</dict></plist>\n"
	if err := os.WriteFile(filepath.Join(dir, label+".plist"), []byte(body), 0o600); err != nil {
		return "", err
	}
	return "launchd", nil
}

func taskName(root Root) string { return "GoreGraphWatch-" + root.ID }

func powershellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func scheduledTaskAction(root Root, executable string) (string, string) {
	parts := append([]string{executable}, runArguments(root)...)
	for i := range parts {
		parts[i] = powershellArgument(parts[i])
	}
	script := "$ErrorActionPreference = 'Stop'; try { & " + strings.Join(parts, " ") + "; exit $LASTEXITCODE } catch { exit 1 }"
	units := utf16.Encode([]rune(script))
	encoded := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(encoded[i*2:], unit)
	}
	return `%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe`,
		"-NoProfile -NonInteractive -WindowStyle Hidden -EncodedCommand " + base64.StdEncoding.EncodeToString(encoded)
}

func scheduledTaskXML(root Root, executable, username string) []byte {
	command, arguments := scheduledTaskAction(root, executable)
	body := "<?xml version=\"1.0\" encoding=\"UTF-16\"?>\n" +
		"<Task version=\"1.2\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">" +
		"<Triggers><LogonTrigger><Enabled>true</Enabled><UserId>" + xmlValue(username) + "</UserId></LogonTrigger></Triggers>" +
		"<Principals><Principal id=\"Author\"><UserId>" + xmlValue(username) + "</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals>" +
		"<Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><ExecutionTimeLimit>PT0S</ExecutionTimeLimit><Enabled>true</Enabled></Settings>" +
		"<Actions Context=\"Author\"><Exec><Command>" + xmlValue(command) + "</Command><Arguments>" + xmlValue(arguments) + "</Arguments></Exec></Actions></Task>"
	units := utf16.Encode([]rune(body))
	encoded := make([]byte, 2+len(units)*2)
	encoded[0], encoded[1] = 0xff, 0xfe
	for i, unit := range units {
		binary.LittleEndian.PutUint16(encoded[2+i*2:], unit)
	}
	return encoded
}

func enableScheduledTask(root Root, executable string) (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp("", "goregraph-watch-*.xml")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(scheduledTaskXML(root, executable, current.Username)); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	output, err := exec.Command("schtasks", "/Create", "/TN", taskName(root), "/XML", file.Name(), "/F").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("create scheduled task: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return "taskscheduler", nil
}

func systemdArgument(value string) string {
	value = strings.ReplaceAll(value, "%", "%%")
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return "\"" + value + "\""
}

func enableLinuxAutostart(root Root, executable string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		if err := exec.Command("systemctl", "--user", "show-environment").Run(); err == nil {
			dir := filepath.Join(base, "systemd", "user")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return "", err
			}
			name := "goregraph-watch-" + root.ID + ".service"
			parts := append([]string{executable}, runArguments(root)...)
			for i := range parts {
				parts[i] = systemdArgument(parts[i])
			}
			body := "[Unit]\nDescription=GoreGraph watcher for " + root.ID + "\n\n[Service]\nType=simple\nExecStart=" + strings.Join(parts, " ") + "\nRestart=no\n\n[Install]\nWantedBy=default.target\n"
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				return "", err
			}
			if output, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
				_ = os.Remove(path)
				return "", fmt.Errorf("reload user services: %v: %s", err, strings.TrimSpace(string(output)))
			}
			if output, err := exec.Command("systemctl", "--user", "enable", name).CombinedOutput(); err != nil {
				_ = os.Remove(path)
				return "", fmt.Errorf("enable user service: %v: %s", err, strings.TrimSpace(string(output)))
			}
			return "systemd", nil
		}
	}
	if os.Getenv("XDG_CURRENT_DESKTOP") == "" && os.Getenv("DESKTOP_SESSION") == "" {
		return "", fmt.Errorf("no supported Linux user session autostart manager is available")
	}
	dir := filepath.Join(base, "autostart")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	parts := append([]string{executable}, runArguments(root)...)
	for i := range parts {
		parts[i] = systemdArgument(parts[i])
	}
	body := "[Desktop Entry]\nType=Application\nName=GoreGraph Watch " + root.ID + "\nExec=" + strings.Join(parts, " ") + "\nTerminal=false\n"
	if err := os.WriteFile(filepath.Join(dir, "goregraph-watch-"+root.ID+".desktop"), []byte(body), 0o600); err != nil {
		return "", err
	}
	return "xdg", nil
}
