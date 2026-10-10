package watch

import (
	"context"
	"debug/buildinfo"
	"fmt"
	"runtime/debug"
)

// Replacement monitors the stable installation path used to launch a process.
// It shares the watcher's content checks, including package-manager symlinks.
type Replacement struct {
	Path            string
	active          executableIdentity
	startedReplaced bool
}

// NewReplacement captures the installed executable without starting a watcher.
func NewReplacement() (*Replacement, error) {
	path, err := ExecutablePath()
	if err != nil {
		return nil, err
	}
	identity, err := readExecutable(path, nil)
	if err != nil {
		return nil, err
	}
	replacement := &Replacement{Path: path, active: identity}
	// The installation can change between exec and this startup snapshot.
	// Build metadata belongs to the running image, unlike os.Stat(path).
	if loaded, ok := debug.ReadBuildInfo(); ok {
		installed, err := buildinfo.ReadFile(path)
		if err != nil {
			return nil, err
		}
		replacement.startedReplaced = loaded.String() != installed.String()
	}
	return replacement, nil
}

// Changed validates a replacement and checks that it stayed unchanged during
// validation. The caller must hand off at a safe request boundary.
func (replacement *Replacement) Changed(ctx context.Context, validate func(context.Context, string) error) (bool, error) {
	identity, err := readExecutable(replacement.Path, &replacement.active)
	if err != nil {
		return false, err
	}
	if identity.hash == replacement.active.hash && !replacement.startedReplaced {
		return false, nil
	}
	if err := validate(ctx, replacement.Path); err != nil {
		return false, err
	}
	confirmed, err := readExecutable(replacement.Path, nil)
	if err != nil {
		return false, err
	}
	if confirmed.hash != identity.hash {
		return false, fmt.Errorf("installed executable changed during replacement validation")
	}
	return true, nil
}
