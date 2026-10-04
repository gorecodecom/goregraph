//go:build windows

package watch

import "fmt"

// ReloadSupervisor is unavailable on Windows; its supervisor replaces workers.
func ReloadSupervisor(root Root) error {
	return fmt.Errorf("in-place supervisor replacement is unsupported on Windows")
}
