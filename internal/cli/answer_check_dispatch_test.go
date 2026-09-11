package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAnswerCheckDispatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	request := `{"answer":"See ` + "`src/Handler.go:3`" + `.","root":"/workspace","files":[{"path":"src/Handler.go","ranges":[[3,5]]}]}`
	if code := Run([]string{"answer-check", "--request", request}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var result struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || !result.Valid {
		t.Fatalf("invalid answer-check result %q: %v", stdout.String(), err)
	}
}

func TestAnswerCheckDispatchHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"answer-check", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "answer-check") {
		t.Fatalf("missing command help: %s", stdout.String())
	}
}
