//go:build !windows

package pathutil

import "path/filepath"

// Resolve resolves an existing file or directory to its physical path.
func Resolve(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
