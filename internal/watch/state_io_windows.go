//go:build windows

package watch

import (
	"errors"
	"syscall"
)

func stateSharingViolation(err error) bool {
	// ERROR_SHARING_VIOLATION and ERROR_LOCK_VIOLATION are transient Windows
	// file-sharing conflicts, unlike permissions or invalid state contents.
	return errors.Is(err, syscall.Errno(32)) || errors.Is(err, syscall.Errno(33))
}
