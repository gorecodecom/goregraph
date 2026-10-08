package watch

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var errWatcherStopped = errors.New("watcher stop requested")

// fingerprintUntilStopped keeps control requests responsive when a read-only
// filesystem check blocks inside an OS call that does not honor cancellation.
func fingerprintUntilStopped(ctx context.Context, root Root, token string, snapshot func(context.Context, Root) (string, error)) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if stopRequested(root, token) {
		return "", errWatcherStopped
	}
	checkCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		value string
		err   error
	}
	completed := make(chan result, 1)
	go func() {
		value, err := snapshot(checkCtx, root)
		completed <- result{value, err}
	}()
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			if !ownsLock(root, token) {
				return "", fmt.Errorf("watcher lock changed")
			}
			if stopRequested(root, token) {
				return "", errWatcherStopped
			}
		case received := <-completed:
			if !ownsLock(root, token) {
				return "", fmt.Errorf("watcher lock changed")
			}
			if stopRequested(root, token) {
				return "", errWatcherStopped
			}
			return received.value, received.err
		}
	}
}
