package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAnswerCheckCLI(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"--help"}, 0},
		{nil, 2},
		{[]string{"--request", `{"answer":"` + "`src/F.go:1`" + `","files":[{"path":"src/F.go","ranges":[[1,1]]}]}`}, 0},
		{[]string{"--request", `{"answer":"` + "`src/F.go:2`" + `","files":[{"path":"src/F.go","ranges":[[1,1]]}]}`}, 1},
		{[]string{"--request", `{"answer":"x","unknown":1}`}, 2},
		{[]string{"--request", "{}", "--request-file", "x"}, 2},
	} {
		var out, stderr bytes.Buffer
		got := runAnswerCheck(tc.args, &out, &stderr)
		if got != tc.code {
			t.Fatalf("%v: code=%d stdout=%s stderr=%s", tc.args, got, out.String(), stderr.String())
		}
		if got == 1 {
			var report map[string]any
			if err := json.Unmarshal(out.Bytes(), &report); err != nil || report["valid"] != false {
				t.Fatalf("missing JSON findings: %s", out.String())
			}
		}
	}
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, []byte(`{"answer":"Nothing"}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := runAnswerCheck([]string{"--request-file", path}, &out, &stderr); code != 1 || !strings.Contains(out.String(), "no_references") {
		t.Fatalf("file request: %d %s %s", code, out.String(), stderr.String())
	}
}

func TestAnswerCheckRequestFileAndOutputBoundaries(t *testing.T) {
	directory := t.TempDir()
	oversized := filepath.Join(directory, "oversized.json")
	if err := os.WriteFile(oversized, []byte(strings.Repeat(" ", (1<<20)+1)), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{directory, oversized, filepath.Join(directory, "missing.json")} {
		var out, stderr bytes.Buffer
		if got := runAnswerCheck([]string{"--request-file", path}, &out, &stderr); got != 2 || out.Len() != 0 {
			t.Errorf("expected bounded file rejection: %s %d %s %s", path, got, out.String(), stderr.String())
		}
	}
	var stderr bytes.Buffer
	request := `{"answer":"` + "`F.go`" + `","files":[{"path":"F.go"}]}`
	if got := runAnswerCheck([]string{"--request", request}, answerCheckFailWriter{}, &stderr); got != 1 {
		t.Fatalf("output failure exit=%d stderr=%s", got, stderr.String())
	}
}

type answerCheckFailWriter struct{}

func (answerCheckFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestAnswerCheckRejectsFIFOWithoutWaitingForWriter(t *testing.T) {
	executable, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo unavailable")
	}
	fifo := filepath.Join(t.TempDir(), "request.fifo")
	if out, err := exec.Command(executable, fifo).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %s %v", out, err)
	}
	done := make(chan int, 1)
	go func() { done <- runAnswerCheck([]string{"--request-file", fifo}, io.Discard, io.Discard) }()
	select {
	case code := <-done:
		if code != 2 {
			t.Fatalf("FIFO accepted: exit %d", code)
		}
	case <-time.After(500 * time.Millisecond):
		release, err := os.OpenFile(fifo, os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		<-done
		release.Close()
		t.Fatal("request FIFO blocked waiting for a writer")
	}
}
