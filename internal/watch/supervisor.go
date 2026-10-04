package watch

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorecodecom/goregraph/internal/version"
)

// ErrSupervisorUpgrade requests an in-place supervisor replacement after drain.
var ErrSupervisorUpgrade = errors.New("supervisor executable changed")

func runtimeAlive(root Root, live runtimeState, lockName string) bool {
	age := time.Since(live.Heartbeat)
	return !live.Stopped && live.Token != "" && ownsNamedLock(root, live.Token, lockName) && age >= 0 && age < heartbeatLimit
}

func liveSupervisor(root Root) (runtimeState, bool, error) {
	path, err := statePath(root, "supervisor.json")
	if err != nil {
		return runtimeState{}, false, err
	}
	var live runtimeState
	if err := readJSON(path, &live); err != nil && !errors.Is(err, os.ErrNotExist) {
		return live, false, err
	}
	age := time.Since(live.Heartbeat)
	pending := live.Upgrading && !live.Stopped && age >= 0 && age < heartbeatLimit
	return live, pending || runtimeAlive(root, live, "supervisor.lock"), nil
}

// ExecutablePath preserves stable installation symlinks used by package managers.
func ExecutablePath() (string, error) {
	path := os.Args[0]
	if !filepath.IsAbs(path) && !strings.ContainsAny(path, `/\`) {
		resolved, err := exec.LookPath(path)
		if err != nil {
			return "", err
		}
		path = resolved
	}
	return filepath.Abs(path)
}

type supervisorOptions struct {
	interval time.Duration
	retry    time.Duration
	command  func(string, Root) *exec.Cmd
	validate func(context.Context, string) error
	reload   bool
}

// Supervise restarts failed workers and adopts replacements of the installed
// executable only after the previous worker has finished its current update.
func Supervise(ctx context.Context, root Root) error {
	executable, err := ExecutablePath()
	if err != nil {
		return err
	}
	return supervise(ctx, root, executable, supervisorOptions{
		interval: 2 * time.Second, retry: 2 * time.Second,
		command: func(path string, root Root) *exec.Cmd {
			command := exec.Command(path, runArguments(root)...)
			command.Stdout, command.Stderr = os.Stdout, os.Stderr
			return command
		},
		validate: validateReplacement,
		reload:   runtime.GOOS != "windows",
	})
}

type executableIdentity struct {
	info os.FileInfo
	hash [32]byte
}

func readExecutable(path string, previous *executableIdentity) (executableIdentity, error) {
	info, err := os.Stat(path)
	if err != nil {
		return executableIdentity{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 512<<20 {
		return executableIdentity{}, fmt.Errorf("installed executable is not a supported regular file")
	}
	if previous != nil && os.SameFile(info, previous.info) && info.Size() == previous.info.Size() && info.ModTime() == previous.info.ModTime() {
		return *previous, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return executableIdentity{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, (512<<20)+1)); err != nil {
		return executableIdentity{}, err
	}
	end, err := file.Stat()
	if err != nil || info.Size() != end.Size() || info.ModTime() != end.ModTime() {
		return executableIdentity{}, fmt.Errorf("installed executable is being changed")
	}
	identity := executableIdentity{info: info}
	copy(identity.hash[:], hash.Sum(nil))
	return identity, nil
}

type versionProbeOutput struct{ text strings.Builder }

func (output *versionProbeOutput) Write(body []byte) (int, error) {
	if output.text.Len()+len(body) > 4096 {
		return 0, fmt.Errorf("replacement version response is too large")
	}
	return output.text.Write(body)
}

func validateReplacement(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, probe := range []struct {
		args   []string
		prefix string
	}{
		{[]string{"version"}, "goregraph "},
		{[]string{"watch", "supervise", "--help"}, "Usage: goregraph watch "},
	} {
		command := exec.CommandContext(ctx, path, probe.args...)
		var output versionProbeOutput
		command.Stdout, command.Stderr = &output, &output
		if err := command.Run(); err != nil {
			return fmt.Errorf("replacement compatibility check: %w", err)
		}
		if !strings.HasPrefix(output.text.String(), probe.prefix) {
			return fmt.Errorf("replacement does not support the GoreGraph supervisor protocol")
		}
	}
	return nil
}

func supervise(ctx context.Context, root Root, executable string, options supervisorOptions) (result error) {
	if err := requireExistingRoot(root); err != nil {
		return err
	}
	if err := ensureStateDir(root); err != nil {
		return err
	}
	previous, _, err := liveSupervisor(root)
	if err != nil {
		return err
	}
	if previous.Upgrading && time.Since(previous.Heartbeat) < heartbeatLimit && namedStopRequested(root, previous.Token, "supervisor.stop.request") {
		previous.Upgrading, previous.Stopped = false, true
		path, _ := statePath(root, "supervisor.json")
		return writeJSON(path, previous)
	}
	release, token, err := acquireNamed(root, "supervisor.lock", "supervisor.json")
	if err != nil {
		return err
	}
	defer release()
	path, _ := statePath(root, "supervisor.json")
	live := runtimeState{Token: token, PID: os.Getpid(), Version: version.Version, Commit: version.Commit}
	var mutex sync.Mutex
	publish := func() error {
		mutex.Lock()
		defer mutex.Unlock()
		if !ownsNamedLock(root, token, "supervisor.lock") {
			return fmt.Errorf("supervisor lock changed")
		}
		live.Heartbeat = time.Now()
		return writeJSON(path, live)
	}
	recordError := func(err error) {
		mutex.Lock()
		live.LastError = ""
		if err != nil {
			live.LastError = err.Error()
		}
		mutex.Unlock()
		_ = publish()
	}
	if err := publish(); err != nil {
		return err
	}
	heartbeats, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeats:
				return
			case <-ticker.C:
				_ = publish()
			}
		}
	}()
	defer func() {
		close(heartbeats)
		<-finished
		mutex.Lock()
		live.Upgrading = errors.Is(result, ErrSupervisorUpgrade)
		live.Stopped = !live.Upgrading
		mutex.Unlock()
		_ = publish()
	}()

	ticker := time.NewTicker(options.interval)
	defer ticker.Stop()
	var worker <-chan error
	var active, pending executableIdentity
	var retryAt time.Time
	ownedPID := 0
	stopping, replacing := false, false
	// A service-manager restart can leave an in-progress worker alive. Adopt its
	// shutdown rather than starting a competing writer or killing that update.
	status, err := GetStatus(root)
	if err != nil {
		return err
	}
	if status.Running {
		worker, replacing = awaitExistingWorker(root, options.interval), true
	}
	for {
		if namedStopRequested(root, token, "supervisor.stop.request") || ctx.Err() != nil {
			stopping = true
		}
		if !ownsNamedLock(root, token, "supervisor.lock") {
			stopping = true
		}
		if stopping || replacing {
			if worker == nil {
				if stopping {
					return nil
				}
				if options.reload && active.info != nil {
					return ErrSupervisorUpgrade
				}
				replacing = false
			} else if _, err := requestWorkerStop(root); err != nil {
				recordError(err)
			}
		}
		if worker == nil && !stopping && !time.Now().Before(retryAt) {
			identity, err := readExecutable(executable, nil)
			if err == nil && active.info != nil && identity.hash != active.hash {
				err = options.validate(ctx, executable)
			}
			if err == nil {
				command := options.command(executable, root)
				err = command.Start()
				if err == nil {
					done := make(chan error, 1)
					go func() { done <- command.Wait() }()
					worker, active, pending = done, identity, executableIdentity{}
					ownedPID = command.Process.Pid
				}
			}
			recordError(err)
			retryAt = time.Now().Add(options.retry)
		}
		select {
		case err := <-worker:
			if ownedPID != 0 {
				err = errors.Join(err, releaseExitedWorker(root, ownedPID))
				ownedPID = 0
			}
			worker = nil
			if replacing || stopping {
				retryAt = time.Time{}
			} else {
				if err == nil {
					err = fmt.Errorf("watcher exited unexpectedly; restarting")
				}
				recordError(err)
				retryAt = time.Now().Add(options.retry)
			}
		case <-ticker.C:
			if worker != nil && !stopping && !replacing {
				identity, err := readExecutable(executable, &active)
				if err != nil {
					recordError(err)
					pending = executableIdentity{}
				} else if identity.hash != active.hash {
					if pending.info != nil && pending.hash == identity.hash {
						if err := options.validate(ctx, executable); err != nil {
							recordError(err)
						} else {
							replacing = true
						}
					}
					pending = identity
				} else {
					active, pending = identity, executableIdentity{}
				}
			}
		}
	}
}

// The supervisor may release a crashed child's lock only after command.Wait
// has proved that its own child exited and the runtime still names that PID.
func releaseExitedWorker(root Root, pid int) error {
	path, err := statePath(root, "runtime.json")
	if err != nil {
		return err
	}
	var live runtimeState
	if err := readJSON(path, &live); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if live.PID != pid || !ownsLock(root, live.Token) {
		return nil
	}
	live.Stopped = true
	if err := writeJSON(path, live); err != nil {
		return err
	}
	lock, _ := statePath(root, "run.lock")
	if err := os.Remove(filepath.Join(lock, "owner")); err != nil {
		return err
	}
	return os.Remove(lock)
}

func awaitExistingWorker(root Root, interval time.Duration) <-chan error {
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			status, err := GetStatus(root)
			if err != nil || !status.Running {
				done <- err
				return
			}
		}
	}()
	return done
}
