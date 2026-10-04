package outputstore

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writerLockPath(root string) string {
	return filepath.Join(filepath.Dir(root), ".goregraph-lock-writer-"+rootKey(root)+".lock")
}

func legacyWriterLockPath(root string) string {
	return filepath.Join(filepath.Dir(root), ".goregraph-writer-"+rootKey(root)+".lock")
}

func acquireWriterLocks(ctx context.Context, roots []string) ([]*fileLock, error) {
	var locks []*fileLock
	for _, root := range roots {
		// A running pre-release writer may still own the previous lease name.
		// Retain serialization during upgrade, without creating legacy artifacts.
		if _, err := os.Lstat(legacyWriterLockPath(root)); err == nil {
			legacy, err := acquireFileLockMode(ctx, legacyWriterLockPath(root), false, false)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				releaseLocks(locks)
				return nil, err
			}
			if legacy != nil {
				locks = append(locks, legacy)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			releaseLocks(locks)
			return nil, err
		}
		lock, err := acquireFileLock(ctx, writerLockPath(root), false)
		if err != nil {
			releaseLocks(locks)
			return nil, fmt.Errorf("lock output writer %s: %w", root, err)
		}
		locks = append(locks, lock)
	}
	return locks, nil
}

// An initial build has no committed snapshot. Its readers wait for the writer
// without holding publication locks needed by that writer to finish.
func acquireInitializationLocks(ctx context.Context, roots []string) ([]*fileLock, error) {
	var locks []*fileLock
	fail := func(err error) ([]*fileLock, error) { releaseLocks(locks); return nil, err }
	for _, root := range roots {
		if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
			if err != nil {
				return fail(err)
			}
			continue
		}
		for _, path := range []string{legacyWriterLockPath(root), writerLockPath(root)} {
			if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				return fail(err)
			}
			lock, err := acquireFileLockMode(ctx, path, true, false)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return fail(err)
			}
			locks = append(locks, lock)
		}
	}
	return locks, nil
}

func discardStages(transaction journal, ops fileOperations) error {
	var result error
	for _, entry := range transaction.Entries {
		result = errors.Join(result, ops.removeAll(entry.Stage))
	}
	return result
}

// Staging is unpublished and has no recovery journal. A crashed writer can
// leave scratch directories; the next serialized writer retires only these.
func removeAbandonedStages(root string, ops fileOperations) error {
	entries, err := os.ReadDir(filepath.Dir(root))
	if err != nil {
		return err
	}
	prefix, suffix := ".goregraph-stage-", "-"+rootKey(root)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		generation := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		if len(generation) != 32 {
			continue
		}
		if _, err := hex.DecodeString(generation); err != nil {
			continue
		}
		if err := ops.removeAll(filepath.Join(filepath.Dir(root), name)); err != nil {
			return err
		}
	}
	return nil
}
