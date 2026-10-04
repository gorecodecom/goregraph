//go:build !windows

package watch

func stateSharingViolation(err error) bool { return false }
