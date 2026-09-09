//go:build windows

package outputstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	lockFileEx   = kernel32.NewProc("LockFileEx")
	unlockFileEx = kernel32.NewProc("UnlockFileEx")
	moveFileEx   = kernel32.NewProc("MoveFileExW")
)

type fileLock struct{ file *os.File }

func acquireFileLock(ctx context.Context, path string, shared bool) (*fileLock, error) {
	file, err := openLockFile(path, shared)
	if err != nil {
		return nil, err
	}
	flags := uintptr(1) // LOCKFILE_FAIL_IMMEDIATELY
	if !shared {
		flags |= 2
	} // LOCKFILE_EXCLUSIVE_LOCK
	for {
		if err := ctx.Err(); err != nil {
			file.Close()
			return nil, err
		}
		var overlapped syscall.Overlapped
		result, _, lockErr := lockFileEx.Call(file.Fd(), flags, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
		if result != 0 {
			return &fileLock{file: file}, nil
		}
		if !errors.Is(lockErr, syscall.Errno(33)) {
			file.Close()
			return nil, lockErr
		} // ERROR_LOCK_VIOLATION
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (lock *fileLock) release() error {
	var overlapped syscall.Overlapped
	result, _, err := unlockFileEx.Call(lock.file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if result != 0 {
		err = nil
	}
	return errors.Join(err, lock.file.Close())
}

func durableRename(source, target string) error {
	from, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	// Windows readers can briefly prevent a directory promotion even after our
	// own handles are closed. Bound retries so persistent locks or ACL errors
	// still fail, including during rollback after cancellation.
	deadline := time.Now().Add(2 * time.Second)
	for {
		// WRITE_THROUGH persists the rename before backups are removed.
		result, _, moveErr := moveFileEx.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), 0x1|0x8)
		if result != 0 {
			return nil
		}
		if !errors.Is(moveErr, syscall.ERROR_ACCESS_DENIED) && !errors.Is(moveErr, syscall.Errno(32)) && !errors.Is(moveErr, syscall.Errno(33)) {
			return &os.LinkError{Op: "rename", Old: source, New: target, Err: moveErr}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return &os.LinkError{Op: "rename", Old: source, New: target, Err: fmt.Errorf("still blocked after 2s; check open files and directory permissions: %w", moveErr)}
		}
		time.Sleep(min(20*time.Millisecond, remaining))
	}
}

func syncDirectory(string) error {
	// Windows directory handles cannot use FlushFileBuffers. Journal contents
	// use File.Sync and rename metadata uses MoveFileExW WRITE_THROUGH above.
	return nil
}
