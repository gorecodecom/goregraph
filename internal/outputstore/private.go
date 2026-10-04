package outputstore

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// PreparePrivate writes into snapshots owned by an enclosing prepared
// transaction. It never publishes them or repeats copying and flushing; the
// enclosing transaction validates and durably publishes the final snapshots.
// A failed callback requires the enclosing transaction to discard its stages.
func PreparePrivate(ctx context.Context, requests []UpdateRequest, validate func() error) error {
	ordered, err := normalizeRequests(requests)
	if err != nil {
		return err
	}
	for _, request := range ordered {
		if !strings.HasPrefix(filepath.Base(request.Root), ".prepared-") || !strings.HasPrefix(filepath.Base(filepath.Dir(request.Root)), ".goregraph-stage-") {
			return fmt.Errorf("private preparation requires an owned transaction snapshot: %s", request.Root)
		}
		if err := directoryExists(request.Root); err != nil {
			return err
		}
		if err := rejectPendingJournal(request.Root); err != nil {
			return err
		}
	}
	for i, request := range ordered {
		if err := ctx.Err(); err != nil {
			return err
		}
		started := time.Now()
		observe(request, "write", "started", i, len(ordered), started)
		if err := request.Write(request.Root); err != nil {
			return fmt.Errorf("prepare private output %s: %w", request.Root, err)
		}
		observe(request, "write", "completed", i+1, len(ordered), started)
		if err := directoryExists(request.Root); err != nil {
			return err
		}
		if request.Validate != nil {
			started := time.Now()
			observe(request, "validate", "started", i, len(ordered), started)
			if err := request.Validate(request.Root); err != nil {
				return err
			}
			observe(request, "validate", "completed", i+1, len(ordered), started)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if validate != nil {
		return errors.Join(validate(), ctx.Err())
	}
	return nil
}
