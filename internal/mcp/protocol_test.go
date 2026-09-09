package mcp

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/agentguide"
)

func TestAdaptiveProtocolIsOptInAtServerBoundary(t *testing.T) {
	strict := tools(Options{})[0]
	if !reflect.DeepEqual(strict, taskContextTool()) {
		t.Fatal("default tool schema changed")
	}
	adaptive := tools(Options{ProtocolVersion: agentguide.AdaptiveV2})[0]
	if adaptive["description"] == strict["description"] {
		t.Fatal("adaptive server advertises strict instructions")
	}
	if !reflect.DeepEqual(adaptive["inputSchema"], strict["inputSchema"]) {
		t.Fatal("protocol selection changed task arguments")
	}
	text, err := callTool(Options{ProtocolVersion: agentguide.AdaptiveV2}, "task_context", map[string]any{"root": t.TempDir(), "query": "find the handler"})
	if err != nil {
		t.Fatal(err)
	}
	var pack agent.ContextPack
	if err := json.Unmarshal([]byte(text), &pack); err != nil {
		t.Fatal(err)
	}
	if pack.ProtocolVersion != agentguide.AdaptiveV2 || pack.FallbackReason != agent.ContextFallbackIndexMissing {
		t.Fatalf("pack=%+v", pack)
	}
}

func TestServerRejectsUnknownProtocolBeforeReadingInput(t *testing.T) {
	var output bytes.Buffer
	if err := ServeWithOptions(strings.NewReader(""), &output, Options{ProtocolVersion: "unknown"}); err == nil {
		t.Fatal("accepted unknown protocol")
	}
	if output.Len() != 0 {
		t.Fatal("invalid server wrote output")
	}
}
