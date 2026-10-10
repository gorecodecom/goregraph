//go:build windows

package mcp

import (
	"fmt"
	"io"
	"os"
)

func reloadInput() (io.Reader, func(), error) { return os.Stdin, func() {}, nil }
func resumedInput() ([]byte, error)           { return nil, nil }
func replaceMCP(path string, pending []byte) error {
	return fmt.Errorf("%w: %s", errReconnectRequired, path)
}
