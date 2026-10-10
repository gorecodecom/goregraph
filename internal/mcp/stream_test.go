package mcp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHandoffPreservesPipelineAfterCompletedResponse(t *testing.T) {
	first := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"
	second := `{"jsonrpc":"2.0","id":2,"method":"initialize"}` + "\n"
	partial := `{"jsonrpc":"2.0","id":3,"meth`
	var output bytes.Buffer
	handoff := errors.New("handoff")
	err := serveStream(strings.NewReader(first+second+partial), &output, Options{}, nil, func(pending []byte) error {
		if output.Len() == 0 {
			return nil
		}
		if string(pending) != second+partial {
			t.Fatalf("lost unread input: %q", pending)
		}
		if strings.Count(output.String(), `"result"`) != 1 {
			t.Fatal("handoff did not follow exactly one complete response")
		}
		return handoff
	})
	if !errors.Is(err, handoff) {
		t.Fatalf("err=%v", err)
	}
}

type timeoutOnce struct {
	timedOut bool
	input    io.Reader
}

func (reader *timeoutOnce) Read(buffer []byte) (int, error) {
	if !reader.timedOut {
		reader.timedOut = true
		return 0, os.ErrDeadlineExceeded
	}
	return reader.input.Read(buffer)
}

func TestIdleTimeoutPreservesPartialRequest(t *testing.T) {
	partial := []byte(`{"jsonrpc":"2.0","id":7,"method":`)
	reader := &timeoutOnce{input: strings.NewReader(`"initialize"}` + "\n")}
	var output bytes.Buffer
	sawIdle := false
	err := serveStream(reader, &output, Options{}, partial, func(pending []byte) error {
		if reader.timedOut && output.Len() == 0 && bytes.Equal(pending, partial) {
			sawIdle = true
		}
		return nil
	})
	if err != nil || !sawIdle || !strings.Contains(output.String(), `"id":7`) {
		t.Fatalf("idle=%v err=%v output=%s", sawIdle, err, &output)
	}
}

func TestStreamRetainsRequestLimit(t *testing.T) {
	err := Serve(strings.NewReader(strings.Repeat("x", bufio.MaxScanTokenSize)), io.Discard)
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("err=%v", err)
	}
}

func TestStreamEOFAndParseError(t *testing.T) {
	var output bytes.Buffer
	err := Serve(strings.NewReader("invalid\n"+`{"id":8,"method":"initialize"}`), &output)
	if err != nil || !strings.Contains(output.String(), `"code":-32700`) || !strings.Contains(output.String(), `"id":8`) {
		t.Fatalf("err=%v output=%s", err, &output)
	}
}
