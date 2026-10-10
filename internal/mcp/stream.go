package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// serveStream owns all input buffering. A handoff receives every unread byte,
// including pipelined requests and partial JSON, after the previous response.
func serveStream(input io.Reader, output io.Writer, options Options, pending []byte, checkpoint func([]byte) error) error {
	encoder := json.NewEncoder(output)
	var readErr error
	buffer := make([]byte, 4096)
	for {
		if checkpoint != nil && readErr == nil {
			if err := checkpoint(pending); err != nil {
				return err
			}
		}
		end := bytes.IndexByte(pending, '\n')
		if end >= 0 || errors.Is(readErr, io.EOF) && len(pending) > 0 {
			length := end + 1
			if end < 0 {
				end, length = len(pending), len(pending)
			}
			line := bytes.TrimSpace(pending[:end])
			if len(line) > 0 {
				var req request
				var result response
				if err := json.Unmarshal(line, &req); err != nil {
					result = errorResponse(nil, -32700, "parse error")
				} else {
					result = handle(req, options)
				}
				if err := encoder.Encode(result); err != nil {
					return err
				}
			}
			pending = pending[length:]
			continue
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
		if len(pending) >= bufio.MaxScanTokenSize {
			return bufio.ErrTooLong
		}
		limit := min(len(buffer), bufio.MaxScanTokenSize-len(pending))
		n, err := input.Read(buffer[:limit])
		pending = append(pending, buffer[:n]...)
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			readErr = err
		}
	}
}
