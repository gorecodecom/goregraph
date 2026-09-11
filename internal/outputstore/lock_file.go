package outputstore

import (
	"errors"
	"fmt"
	"os"
)

func openLockFile(path string, shared, createMissing bool) (*os.File, error) {
	if !shared {
		return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	}
	file, err := os.Open(path)
	if !errors.Is(err, os.ErrNotExist) {
		return file, err
	}
	if !createMissing {
		return nil, fmt.Errorf("output read lock is missing; initialize it with an authorized GoreGraph build or context command first: %w", err)
	}
	// Legacy outputs and optional roots may not have a lock yet. Creation still
	// requires a writable parent; never bypass synchronization on read failure.
	return os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0600)
}
