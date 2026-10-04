//go:build !windows

package watch

import (
	"os"
	"syscall"
)

// ReloadSupervisor replaces the drained process while preserving native service
// manager ownership. Supervise has already released its worker and lease.
func ReloadSupervisor(root Root) error {
	executable, err := ExecutablePath()
	if err != nil {
		return err
	}
	return syscall.Exec(executable, append([]string{executable}, supervisedArguments(root)...), os.Environ())
}
