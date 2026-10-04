//go:build windows

package watch

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestStateIORetriesOnlyTransientWindowsSharingConflicts(t *testing.T) {
	attempts := 0
	err := retryStateIO(func() error {
		attempts++
		if attempts < 3 {
			return &os.PathError{Op: "open", Path: "runtime.json", Err: syscall.Errno(32)}
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("sharing conflict was not retried: attempts=%d err=%v", attempts, err)
	}
	attempts = 0
	err = retryStateIO(func() error { attempts++; return os.ErrPermission })
	if !errors.Is(err, os.ErrPermission) || attempts != 1 {
		t.Fatalf("non-transient permission failure was hidden: attempts=%d err=%v", attempts, err)
	}
	attempts = 0
	err = retryStateIO(func() error { attempts++; return syscall.Errno(33) })
	if !errors.Is(err, syscall.Errno(33)) || attempts != 20 {
		t.Fatalf("retry was not bounded: attempts=%d err=%v", attempts, err)
	}
}
