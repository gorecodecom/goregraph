package watch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Restart waits for the current update to finish before launching a new watcher.
// It preserves the existing autostart setting and never kills an update.
func Restart(ctx context.Context, root Root) error {
	return restartWithStart(ctx, root, Start)
}

func restartWithStart(ctx context.Context, root Root, start func(Root) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireExistingRoot(root); err != nil {
		return err
	}
	requested, err := RequestStop(root)
	if err != nil {
		return err
	}
	if requested {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			remaining := false
			for _, name := range []string{"run.lock", "supervisor.lock"} {
				lock, err := statePath(root, name)
				if err != nil {
					return err
				}
				if _, err := os.Lstat(lock); err == nil {
					remaining = true
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			_, supervised, err := liveSupervisor(root)
			if err != nil {
				return err
			}
			remaining = remaining || supervised
			if !remaining {
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("watcher has not stopped yet; the current update may still be finishing: %w", ctx.Err())
			case <-ticker.C:
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return start(root)
}

// Start launches a detached watcher after an explicit user command.
func Start(root Root) error {
	if err := requireExistingRoot(root); err != nil {
		return err
	}
	status, err := GetStatus(root)
	if err != nil {
		return err
	}
	if status.Running || status.Supervised {
		return fmt.Errorf("watcher already running for %s", root.Path)
	}
	if err := ensureStateDir(root); err != nil {
		return err
	}
	executable, err := ExecutablePath()
	if err != nil {
		return err
	}
	managed, err := startManaged(root, executable)
	if err != nil {
		return err
	}
	if managed {
		return waitForStart(root)
	}
	directory, err := stateDir(root)
	if err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(directory, "watch.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	command := exec.Command(executable, supervisedArguments(root)...)
	command.Stdin = nil
	command.Stdout = log
	command.Stderr = log
	detach(command)
	if err := command.Start(); err != nil {
		return err
	}
	_ = command.Process.Release()
	return waitForStart(root)
}

func waitForStart(root Root) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := GetStatus(root)
		if err == nil && status.Running {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	directory, _ := stateDir(root)
	return fmt.Errorf("watcher did not start; see %s", filepath.Join(directory, "watch.log"))
}
