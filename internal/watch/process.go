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
		lock, err := statePath(root, "run.lock")
		if err != nil {
			return err
		}
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			if _, err := os.Lstat(lock); os.IsNotExist(err) {
				break
			} else if err != nil {
				return err
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
	if status.Running {
		return fmt.Errorf("watcher already running for %s", root.Path)
	}
	if err := ensureStateDir(root); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
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
	command := exec.Command(executable, runArguments(root)...)
	command.Stdin = nil
	command.Stdout = log
	command.Stderr = log
	detach(command)
	if err := command.Start(); err != nil {
		return err
	}
	_ = command.Process.Release()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := GetStatus(root)
		if err == nil && status.Running {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("watcher did not start; see %s", filepath.Join(directory, "watch.log"))
}
