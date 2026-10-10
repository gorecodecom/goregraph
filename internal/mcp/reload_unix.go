//go:build !windows

package mcp

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"
)

const resumeInputEnv = "GOREGRAPH_MCP_RESUME_V1"

type deadlineInput struct{ file *os.File }

func (input deadlineInput) Read(buffer []byte) (int, error) {
	if err := input.file.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return 0, err
	}
	return input.file.Read(buffer)
}

func reloadInput() (io.Reader, func(), error) {
	fd, err := syscall.Dup(int(os.Stdin.Fd()))
	if err != nil {
		return nil, nil, err
	}
	syscall.CloseOnExec(fd)
	if err := syscall.SetNonblock(fd, true); err != nil {
		syscall.Close(fd)
		return nil, nil, err
	}
	file := os.NewFile(uintptr(fd), "goregraph-mcp-stdin")
	closeInput := func() { _ = file.Close() }
	if err := file.SetReadDeadline(time.Time{}); err != nil {
		// Redirected regular files do not support deadlines and never wait for input.
		info, statErr := file.Stat()
		if statErr != nil || !info.Mode().IsRegular() {
			closeInput()
			return nil, nil, err
		}
		return file, closeInput, nil
	}
	return deadlineInput{file}, closeInput, nil
}

func resumedInput() ([]byte, error) {
	encoded := os.Getenv(resumeInputEnv)
	_ = os.Unsetenv(resumeInputEnv)
	if len(encoded) > base64.StdEncoding.EncodedLen(bufio.MaxScanTokenSize) {
		return nil, fmt.Errorf("MCP handoff input exceeds limit")
	}
	return base64.StdEncoding.DecodeString(encoded)
}

func replaceMCP(path string, pending []byte) error {
	if len(pending) > bufio.MaxScanTokenSize {
		return fmt.Errorf("MCP handoff input exceeds limit")
	}
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, resumeInputEnv+"=") {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, resumeInputEnv+"="+base64.StdEncoding.EncodeToString(pending))
	return syscall.Exec(path, append([]string{path}, os.Args[1:]...), environment)
}
