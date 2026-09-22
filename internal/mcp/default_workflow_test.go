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

func TestServerDefaultContextMatchesAdaptiveAndRetainsExplicitStrict(t *testing.T) {
	root := writeMCPContextFixture(t)
	for _, protocol := range []string{"", agentguide.StrictV1, agentguide.AdaptiveV2} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			input, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "task_context", "arguments": map[string]any{"root": root, "query": "DELETE /users/{id}"}}})
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := ServeWithOptions(bytes.NewReader(append(input, '\n')), &output, Options{ProtocolVersion: protocol}); err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Error  *responseError `json:"error"`
				Result struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"result"`
			}
			if err := json.Unmarshal(output.Bytes(), &wire); err != nil {
				t.Fatal(err)
			}
			if wire.Error != nil || len(wire.Result.Content) != 1 {
				t.Fatalf("unexpected wire response: %s", output.String())
			}
			effective := protocol
			if effective == "" {
				effective = agentguide.AdaptiveV2
			}
			want, err := agent.BuildContext(agent.ContextRequest{Root: root, Query: "DELETE /users/{id}", ProtocolVersion: effective})
			if err != nil {
				t.Fatal(err)
			}
			var got agent.ContextPack
			if err := json.Unmarshal([]byte(wire.Result.Content[0].Text), &got); err != nil {
				t.Fatal(err)
			}
			expected, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			var normalized agent.ContextPack
			if err := json.Unmarshal(expected, &normalized); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, normalized) {
				t.Fatalf("MCP context differs from %s compiler: %+v / %+v", effective, got, normalized)
			}
			if effective == agentguide.AdaptiveV2 && got.SourceCoverage != "complete" && !got.FallbackRequired && len(got.VerificationRequests) == 0 {
				t.Fatalf("incomplete default context has no next step: %+v", got)
			}
		})
	}
}

func TestServerDefaultAdvertisesAdaptiveWorkflowAndSafeParameterDefaults(t *testing.T) {
	input := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\"}\n")
	var output bytes.Buffer
	if err := Serve(input, &output); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"verification_requests", "caller's permissions", "before optional skills", "pass only root and query", "256 to 6000", "1 to 20"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("missing agent-facing guidance %q", want)
		}
	}
	if strings.Contains(output.String(), "run no source-reading commands") {
		t.Fatal("default MCP still advertises the strict source prohibition")
	}
}

func TestTaskContextReportsBothInvalidLimitsBeforeAccessingRoot(t *testing.T) {
	_, err := callTool(Options{}, "task_context", map[string]any{"root": "missing-root", "query": "explain the handler", "budget_tokens": float64(30000), "max_files": float64(50)})
	if err == nil {
		t.Fatal("accepted invalid limits")
	}
	for _, want := range []string{"budget_tokens must be between 256 and 6000", "max_files must be between 1 and 20", "omit both optional limits", "No context was generated"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing combined parameter error %q: %v", want, err)
		}
	}
}

func TestDefaultServerPreservesExplicitToolingAudit(t *testing.T) {
	root, files, requirements := loadStorybookAuditFixture(t)
	buildStorybookAuditFixture(t, root)
	query := "Welche Storybook-Erweiterungen fehlen? Berücksichtige Konfiguration, Stories, Fixtures, Test-Runner, CI und Dokumentation. Prüfe automatische Accessibility-Prüfungen, visuelle Referenzbilder und erfolgreiche Ausführung."
	request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "task_context", "arguments": map[string]any{"root": root, "query": query, "mode": "audit", "budget_tokens": 6000, "max_files": 20}}})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Serve(bytes.NewReader(append(request, '\n')), &output); err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Error  *responseError `json:"error"`
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Error != nil || wire.Result.IsError || len(wire.Result.Content) != 1 {
		t.Fatalf("unexpected audit response: %s", output.String())
	}
	var pack agent.ContextPack
	if err := json.Unmarshal([]byte(wire.Result.Content[0].Text), &pack); err != nil {
		t.Fatal(err)
	}
	if pack.ProtocolVersion != agentguide.AdaptiveV2 || pack.FallbackRequired || len(pack.Entrypoints) != 0 {
		t.Fatalf("default MCP lost the explicit tooling audit contract: %+v", pack)
	}
	verifyStorybookAuditSources(t, pack, files)
	for _, item := range evaluateStorybookAudit(pack, requirements) {
		if !item.Covered {
			t.Errorf("default MCP audit omitted %s: %v", item.ID, item.Missing)
		}
	}
}
