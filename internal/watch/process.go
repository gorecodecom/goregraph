package watch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

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
