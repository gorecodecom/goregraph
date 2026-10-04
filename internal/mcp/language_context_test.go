package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/agentguide"
	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDefaultMCPLanguageContextIsSourceBackedAndReadOnly(t *testing.T) {
	fixtures := []struct{ language, file, body string }{
		{"CSharp", "Assets/RoundTowerMovement.cs", "public class RoundTowerMovement {\n public void Turn() { VerifyFootSupport(); }\n public void VerifyFootSupport() {}\n}\n"},
		{"Swift", "Sources/Game/RoundTowerMovement.swift", "struct RoundTowerMovement {\n func turn() { verifyFootSupport() }\n func verifyFootSupport() {}\n}\n"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.language, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, fixture.file, fixture.body)
			cfg := config.Defaults()
			cfg.Workspace = false
			if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
				t.Fatal(err)
			}
			before := map[string][]byte{}
			for _, name := range []string{"manifest.json", "agent/context-index.json", "index/symbols-full.json"} {
				body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, filepath.FromSlash(name)))
				if err != nil {
					t.Fatal(err)
				}
				before[name] = body
			}
			request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "task_context", "arguments": map[string]any{"root": root, "query": "Explain RoundTowerMovement Turn and VerifyFootSupport in " + fixture.language}}})
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
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"result"`
			}
			if err := json.Unmarshal(output.Bytes(), &wire); err != nil || wire.Error != nil || len(wire.Result.Content) != 1 {
				t.Fatalf("invalid MCP response: %v %s", err, output.String())
			}
			var pack agent.ContextPack
			if err := json.Unmarshal([]byte(wire.Result.Content[0].Text), &pack); err != nil {
				t.Fatal(err)
			}
			if pack.ProtocolVersion != agentguide.AdaptiveV2 || pack.FallbackRequired || len(pack.Entrypoints) == 0 {
				t.Fatalf("no supported language context: %#v", pack)
			}
			found := false
			for _, section := range pack.SourceSections {
				if section.Path == fixture.file && section.ReadReceipt != "" && strings.Contains(strings.ToLower(section.Content), "verifyfootsupport()") {
					found = true
				}
			}
			if !found {
				t.Fatal(pack.SourceSections)
			}
			for name, original := range before {
				body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, filepath.FromSlash(name)))
				if err != nil || !bytes.Equal(body, original) {
					t.Fatalf("read-only query modified index %s: %v", name, err)
				}
			}
		})
	}
}
