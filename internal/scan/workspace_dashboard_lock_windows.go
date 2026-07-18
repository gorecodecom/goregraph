package scan

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	windowsLockfileFailImmediately = 0x00000001
	windowsLockfileExclusiveLock   = 0x00000002
	windowsErrorLockViolation      = syscall.Errno(33)
)

var (
	windowsKernel32     = syscall.NewLazyDLL("kernel32.dll")
	windowsLockFileEx   = windowsKernel32.NewProc("LockFileEx")
	windowsUnlockFileEx = windowsKernel32.NewProc("UnlockFileEx")
)

func tryLockDashboardConfigFile(file *os.File) (bool, error) {
	var overlapped syscall.Overlapped
	result, _, callErr := windowsLockFileEx.Call(
		file.Fd(),
		windowsLockfileExclusiveLock|windowsLockfileFailImmediately,
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if result != 0 {
		return true, nil
	}
	if callErr == windowsErrorLockViolation {
		return false, nil
	}
	return false, fmt.Errorf("LockFileEx: %w", callErr)
}

func unlockDashboardConfigFile(file *os.File) error {
	var overlapped syscall.Overlapped
	result, _, callErr := windowsUnlockFileEx.Call(
		file.Fd(),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if result == 0 {
		return fmt.Errorf("UnlockFileEx: %w", callErr)
	}
	return nil
}
