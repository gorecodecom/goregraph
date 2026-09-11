//go:build !windows

package outputstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type fileLock struct{ file *os.File }

func acquireFileLock(ctx context.Context, path string, shared bool) (*fileLock, error) {
	return acquireFileLockMode(ctx, path, shared, true)
}

func acquireFileLockMode(ctx context.Context, path string, shared, createMissing bool) (*fileLock, error) {
	file, err := openLockFile(path, shared, createMissing)
	if err != nil {
		return nil, err
	}
	mode := syscall.LOCK_EX | syscall.LOCK_NB
	if shared {
		mode = syscall.LOCK_SH | syscall.LOCK_NB
	}
	for {
		if err := ctx.Err(); err != nil {
			file.Close()
			return nil, err
		}
		err := syscall.Flock(int(file.Fd()), mode)
		if err == nil {
			return &fileLock{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			file.Close()
			return nil, err
		}
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
	return errors.Join(syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN), lock.file.Close())
}

func durableRename(source, target string) error {
	if err := os.Rename(source, target); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(target))
}

func syncDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}
