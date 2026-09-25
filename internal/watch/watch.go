// Package watch keeps GoreGraph projections current for explicitly selected roots.
package watch

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

var (
	pollInterval      = 3 * time.Second
	quietPeriod       = 2 * time.Second
	heartbeatInterval = 2 * time.Second
)

const heartbeatLimit = 10 * time.Second

// Root identifies one independently watched project or workspace.
type Root struct {
	Path      string `json:"root"`
	Workspace bool   `json:"workspace"`
	ID        string `json:"-"`
}

// Status separates the live process from its future login setting.
type Status struct {
	Root           string    `json:"root"`
	Workspace      bool      `json:"workspace"`
	Running        bool      `json:"running"`
	Autostart      bool      `json:"autostart"`
	Method         string    `json:"method,omitempty"`
	AutostartError string    `json:"autostart_error,omitempty"`
	LastSuccess    time.Time `json:"last_success,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
}

type setting struct {
	Root       string `json:"root"`
	Workspace  bool   `json:"workspace"`
	Autostart  bool   `json:"autostart"`
	Method     string `json:"method,omitempty"`
	SetupError string `json:"setup_error,omitempty"`
}

type runtimeState struct {
	Token       string    `json:"token"`
	PID         int       `json:"pid"`
	Stopped     bool      `json:"stopped,omitempty"`
	Heartbeat   time.Time `json:"heartbeat"`
	LastSuccess time.Time `json:"last_success,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

// Resolve canonicalizes a selected root without creating watcher state.
// Missing paths remain addressable so their old autostart can be removed.
func Resolve(path string, workspace bool) (Root, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Root{}, err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		absolute = canonical
	} else if !errors.Is(err, os.ErrNotExist) {
		return Root{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Root{}, err
	}
	if err == nil && !info.IsDir() {
		return Root{}, fmt.Errorf("%s is not a directory", path)
	}
	key := absolute
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	sum := sha256.Sum256([]byte(key))
	return Root{Path: absolute, Workspace: workspace, ID: hex.EncodeToString(sum[:8])}, nil
}

// PreferredWorkspace reports whether the selected root is itself a recognized
// workspace. A project inside a workspace keeps its own project watch scope.
func PreferredWorkspace(root Root) (bool, error) {
	workspaceRoot, found, err := scan.WorkspaceRoot(root.Path, config.Defaults())
	if err != nil || !found {
		return false, err
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(workspaceRoot, root.Path), nil
	}
	return workspaceRoot == root.Path, nil
}

func requireExistingRoot(root Root) error {
	info, err := os.Stat(root.Path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", root.Path)
	}
	return nil
}

var userConfigDir = os.UserConfigDir

func baseDir() (string, error) {
	if custom := os.Getenv("GOREGRAPH_WATCH_HOME"); custom != "" {
		return filepath.Abs(custom)
	}
	directory, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "goregraph", "watch"), nil
}

func stateDir(root Root) (string, error) {
	base, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, root.ID), nil
}

func statePath(root Root, name string) (string, error) {
	directory, err := stateDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, name), nil
}

func ensureStateDir(root Root) error {
	directory, err := stateDir(root)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root.Path, directory)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("watcher state directory must be outside the watched root")
	}
	return os.MkdirAll(directory, 0o700)
}

func readJSON(path string, value any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, value)
}

func writeJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	file, err := os.CreateTemp(filepath.Dir(path), ".watch-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(body); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func loadSetting(root Root) (setting, error) {
	path, err := statePath(root, "setting.json")
	if err != nil {
		return setting{}, err
	}
	var value setting
	if err := readJSON(path, &value); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return setting{Root: root.Path, Workspace: root.Workspace}, nil
		}
		return setting{}, err
	}
	if value.Root != root.Path && (runtime.GOOS != "windows" || !strings.EqualFold(value.Root, root.Path)) {
		return setting{}, fmt.Errorf("watch setting belongs to another root")
	}
	return value, nil
}

// HasSetting reports whether this root has ever been configured by watch start.
func HasSetting(root Root) (bool, error) {
	path, err := statePath(root, "setting.json")
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// HelpSummary returns a cheap status hint for a recognizable current root.
func HelpSummary() string {
	current, err := os.Getwd()
	if err != nil {
		return "  Current root: unknown\n  goregraph watch start <path>  |  goregraph watch status <path>\n"
	}
	var configured *Root
	var fallback *Root
	for directory := current; ; directory = filepath.Dir(directory) {
		root, resolveErr := Resolve(directory, false)
		if resolveErr == nil {
			registered, settingErr := HasSetting(root)
			if settingErr == nil && registered {
				status, statusErr := GetStatus(root)
				if statusErr == nil {
					root.Workspace = status.Workspace
					if status.Running {
						return formatHelpStatus(root, status, current)
					}
					if configured == nil {
						copy := root
						configured = &copy
					}
				}
			}
		}
		for _, name := range []string{".goregraph-workspace.yml", ".goregraph-workspace", "goregraph.yml", "go.mod", "package.json", "pom.xml", "build.gradle", "build.gradle.kts", "goregraph-out"} {
			if _, err := os.Stat(filepath.Join(directory, name)); err == nil {
				if fallback == nil {
					candidate, err := Resolve(directory, name == ".goregraph-workspace.yml" || name == ".goregraph-workspace")
					if err == nil {
						fallback = &candidate
					}
				}
				break
			}
		}
		if filepath.Dir(directory) == directory {
			break
		}
	}
	if configured != nil {
		status, err := GetStatus(*configured)
		if err == nil {
			return formatHelpStatus(*configured, status, current)
		}
	}
	if fallback != nil {
		status, err := GetStatus(*fallback)
		if err == nil {
			return formatHelpStatus(*fallback, status, current)
		}
	}
	return "  Current root: unknown\n  goregraph watch start <path>  |  goregraph watch status <path>\n"
}

func formatHelpStatus(root Root, status Status, current string) string {
	path := "."
	if root.Path != current {
		path = "<path>"
	}
	workspace := ""
	mode := "project"
	if root.Workspace {
		workspace, mode = " --workspace", "workspace"
	}
	return fmt.Sprintf("  Current root (%s): running %t · autostart %t\n  goregraph watch start %s%s  |  goregraph watch status %s\n",
		mode, status.Running, status.Autostart, path, workspace, path)
}

// GetStatus reads only small local state files; it never scans source.
func GetStatus(root Root) (Status, error) {
	saved, err := loadSetting(root)
	if err != nil {
		return Status{}, err
	}
	status := Status{Root: root.Path, Workspace: saved.Workspace, Autostart: saved.Autostart, Method: saved.Method, AutostartError: saved.SetupError}
	path, err := statePath(root, "runtime.json")
	if err != nil {
		return Status{}, err
	}
	var live runtimeState
	if err := readJSON(path, &live); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return status, nil
		}
		return Status{}, err
	}
	status.Running = !live.Stopped && live.Token != "" && ownsLock(root, live.Token) && time.Since(live.Heartbeat) >= 0 && time.Since(live.Heartbeat) < heartbeatLimit
	status.LastSuccess = live.LastSuccess
	status.LastError = live.LastError
	if !status.Running && !live.Stopped && live.Token != "" && status.LastError == "" {
		status.LastError = "watcher process is not active (stale heartbeat or lock)"
	}
	return status, nil
}

// ChangeMode reconfigures a stopped watcher while preserving its login choice.
func ChangeMode(root Root, executable string) error {
	status, err := GetStatus(root)
	if err != nil {
		return err
	}
	if status.Running {
		return fmt.Errorf("stop the watcher before changing its mode")
	}
	if status.Workspace == root.Workspace {
		return nil
	}
	if err := SetAutostart(root, status.Autostart, executable); err != nil {
		return err
	}
	runtimePath, err := statePath(root, "runtime.json")
	if err != nil {
		return err
	}
	if err := os.Remove(runtimePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// RequestStop asks the live watcher to shut down after its current update.
func RequestStop(root Root) (bool, error) {
	status, err := GetStatus(root)
	if err != nil || !status.Running {
		return false, err
	}
	path, err := statePath(root, "runtime.json")
	if err != nil {
		return false, err
	}
	var live runtimeState
	if err := readJSON(path, &live); err != nil {
		return false, err
	}
	stopPath, err := statePath(root, "stop.request")
	if err != nil {
		return false, err
	}
	return true, writeJSON(stopPath, struct {
		Token string `json:"token"`
	}{live.Token})
}

// SetAutostart changes only this user's explicitly selected login entry.
func SetAutostart(root Root, enabled bool, executable string) error {
	if enabled {
		if err := requireExistingRoot(root); err != nil {
			return err
		}
	}
	if err := ensureStateDir(root); err != nil {
		return err
	}
	saved, err := loadSetting(root)
	if err != nil {
		return err
	}
	if enabled {
		method, err := enableAutostart(root, executable)
		if err != nil {
			saved.SetupError = err.Error()
			path, pathErr := statePath(root, "setting.json")
			if pathErr == nil {
				_ = writeJSON(path, saved)
			}
			return err
		}
		saved.Autostart, saved.Method, saved.Workspace = true, method, root.Workspace
	} else {
		if err := disableAutostart(root, saved.Method); err != nil {
			saved.SetupError = err.Error()
			path, pathErr := statePath(root, "setting.json")
			if pathErr == nil {
				_ = writeJSON(path, saved)
			}
			return err
		}
		saved.Autostart, saved.Method = false, ""
	}
	saved.Workspace = root.Workspace
	saved.SetupError = ""
	path, err := statePath(root, "setting.json")
	if err != nil {
		return err
	}
	return writeJSON(path, saved)
}

// Run owns one watcher process and updates only after selected inputs change.
func Run(ctx context.Context, root Root, update func() error) error {
	if err := requireExistingRoot(root); err != nil {
		return err
	}
	if err := ensureStateDir(root); err != nil {
		return err
	}
	release, token, err := acquire(root)
	if err != nil {
		return err
	}
	defer release()
	runtimePath, err := statePath(root, "runtime.json")
	if err != nil {
		return err
	}
	live := runtimeState{Token: token, PID: os.Getpid(), Heartbeat: time.Now()}
	var mutex sync.Mutex
	publish := func() error {
		mutex.Lock()
		defer mutex.Unlock()
		if !ownsLock(root, token) {
			return fmt.Errorf("watcher lock changed")
		}
		live.Heartbeat = time.Now()
		return writeJSON(runtimePath, live)
	}
	if err := publish(); err != nil {
		return err
	}
	stopHeartbeat := make(chan struct{})
	doneHeartbeat := make(chan struct{})
	go func() {
		defer close(doneHeartbeat)
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-ticker.C:
				_ = publish()
			}
		}
	}()
	defer func() {
		close(stopHeartbeat)
		<-doneHeartbeat
		mutex.Lock()
		live.Stopped = true
		mutex.Unlock()
		_ = publish()
	}()

	apply := func() bool {
		if !ownsLock(root, token) {
			return false
		}
		err := update()
		mutex.Lock()
		if err != nil {
			live.LastError = err.Error()
		} else {
			live.LastError = ""
			live.LastSuccess = time.Now()
		}
		mutex.Unlock()
		_ = publish()
		return err == nil
	}
	previous, err := fingerprint(ctx, root)
	if err != nil {
		mutex.Lock()
		live.LastError = err.Error()
		mutex.Unlock()
		_ = publish()
	} else {
		apply()
	}
	var pending string
	var changedAt time.Time
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if !ownsLock(root, token) {
				return fmt.Errorf("watcher lock changed")
			}
			if stopRequested(root, token) {
				return nil
			}
			current, err := fingerprint(ctx, root)
			if err != nil {
				mutex.Lock()
				live.LastError = err.Error()
				mutex.Unlock()
				_ = publish()
				continue
			}
			if current == previous {
				pending = ""
				continue
			}
			if current != pending {
				pending, changedAt = current, time.Now()
				continue
			}
			if time.Since(changedAt) < quietPeriod {
				continue
			}
			if apply() {
				previous = current
			}
			pending = ""
		}
	}
}

func stopRequested(root Root, token string) bool {
	path, err := statePath(root, "stop.request")
	if err != nil {
		return false
	}
	var request struct {
		Token string `json:"token"`
	}
	if readJSON(path, &request) != nil || request.Token != token {
		return false
	}
	_ = os.Remove(path)
	return true
}

func acquire(root Root) (func(), string, error) {
	lock, err := statePath(root, "run.lock")
	if err != nil {
		return nil, "", err
	}
	if err := os.Mkdir(lock, 0o700); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
		status, statusErr := GetStatus(root)
		if statusErr != nil {
			return nil, "", statusErr
		}
		if status.Running {
			return nil, "", fmt.Errorf("watcher already running for %s", root.Path)
		}
		info, statErr := os.Stat(lock)
		if statErr != nil {
			return nil, "", statErr
		}
		if time.Since(info.ModTime()) < heartbeatLimit {
			return nil, "", fmt.Errorf("watcher is starting for %s", root.Path)
		}
		stale := lock + ".stale-" + fmt.Sprint(time.Now().UnixNano())
		if err := os.Rename(lock, stale); err != nil {
			return nil, "", fmt.Errorf("watcher lock changed: %w", err)
		}
		_ = os.RemoveAll(stale)
		if err := os.Mkdir(lock, 0o700); err != nil {
			return nil, "", fmt.Errorf("watcher already starting: %w", err)
		}
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		_ = os.Remove(lock)
		return nil, "", err
	}
	token := hex.EncodeToString(random)
	if err := os.WriteFile(filepath.Join(lock, "owner"), []byte(token), 0o600); err != nil {
		_ = os.Remove(filepath.Join(lock, "owner"))
		_ = os.Remove(lock)
		return nil, "", err
	}
	return func() {
		if ownsLock(root, token) {
			_ = os.Remove(filepath.Join(lock, "owner"))
			_ = os.Remove(lock)
		}
	}, token, nil
}

func ownsLock(root Root, token string) bool {
	lock, err := statePath(root, "run.lock")
	if err != nil {
		return false
	}
	owner, err := os.ReadFile(filepath.Join(lock, "owner"))
	return err == nil && string(owner) == token
}

func fingerprint(ctx context.Context, root Root) (string, error) {
	hash := sha256.New()
	roots := []string{root.Path}
	if root.Workspace {
		cfg := config.Defaults()
		cfg.WorkspaceRoot = root.Path
		plan, err := scan.WorkspaceProjectScanPlan(root.Path, cfg)
		if err != nil {
			return "", err
		}
		roots = roots[:0]
		for _, item := range plan.Items {
			roots = append(roots, item.AbsPath)
		}
		_, _ = fmt.Fprintf(hash, "projects:%v\n", roots)
		for _, name := range []string{".goregraph-workspace.yml", ".goregraph-dashboard.json"} {
			body, err := os.ReadFile(filepath.Join(root.Path, name))
			if err == nil {
				_, _ = hash.Write(body)
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
		}
	}
	for _, path := range roots {
		cfg, err := config.Load(path)
		if err != nil {
			return "", err
		}
		cfg.UpdateGitignore = false
		records, report, err := scan.SnapshotProjectFiles(ctx, path, cfg)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(hash, "%s:%s:%v\n", path, report.IgnoreDigest, cfg)
		for _, record := range records {
			_, _ = fmt.Fprintf(hash, "%s:%s\n", record.Path, record.Hash)
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
