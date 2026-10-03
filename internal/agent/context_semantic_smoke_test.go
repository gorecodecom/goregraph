package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestSemanticCompilerCallsReachTheNormalContextPack(t *testing.T) {
	for _, language := range []string{"csharp", "swift"} {
		t.Run(language, func(t *testing.T) {
			root := t.TempDir()
			body, ext, target, run := "class Service {\n void Target(){}\n void Run(){Target();}\n}", "cs", "Target", "Run"
			if language == "swift" {
				body, ext, target, run = "struct Service {\n func target(){}\n func run(){target()}\n}", "swift", "target", "run"
			}
			file := "Service." + ext
			lines := strings.Split(body, "\n")
			hash := sha256.Sum256([]byte(body))
			declaration := func(usr, name string, line int) map[string]any {
				return map[string]any{"usr": usr, "name": name, "kind": "method", "file": file, "line": line, "column": strings.Index(lines[line-1], name) + 1}
			}
			report := map[string]any{"schema_version": 1, "language": language, "producer": "compiler contract fixture", "inputs": map[string]string{file: hex.EncodeToString(hash[:])}, "covered_files": []string{file}, "declarations": []map[string]any{declaration("target", target, 2), declaration("run", run, 3)}, "references": []map[string]any{{"usr": "target", "caller": "run", "name": target, "kind": "calls_method_owner", "file": file, "line": 3, "column": strings.Index(lines[2], target) + 1}}}
			encoded, _ := json.Marshal(report)
			writeSourceFile(t, root, file, body)
			writeSourceFile(t, root, "Report.goregraph-"+language+".json", string(encoded))
			cfg := config.Defaults()
			cfg.Workspace, cfg.UpdateGitignore = false, false
			if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
				t.Fatal(err)
			}
			pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain Service." + run + " and its calls", ProtocolVersion: AdaptiveV2})
			if err != nil || len(pack.CallChain) != 1 || !strings.Contains(pack.CallChain[0].Reason, "compiler symbol") || len(pack.SourceSections) == 0 || pack.SourceSections[0].ReadReceipt == "" {
				t.Fatal(err, pack)
			}
		})
	}
}

// Compiler reports are supplied explicitly; this context test never invokes an SDK.
func TestRealSemanticContextDeliversCompilerCallsAndSourceReceipts(t *testing.T) {
	base := os.Getenv("GOREGRAPH_SEMANTIC_SMOKE_ROOT")
	if base == "" {
		t.Skip("explicit compiler report fixtures not supplied")
	}
	for _, language := range []string{"csharp", "swift"} {
		t.Run(language, func(t *testing.T) {
			root := t.TempDir()
			ext, query := "cs", "Explain Service.Run and Extensions.Echo in C#"
			if language == "swift" {
				ext, query = "swift", "Explain Service.run and the wrapped protocol extension in Swift"
			}
			for _, file := range []string{"Service." + ext, "Report.goregraph-" + language + ".json"} {
				body, err := os.ReadFile(filepath.Join(base, language, file))
				if err != nil {
					t.Fatal(err)
				}
				writeSourceFile(t, root, file, string(body))
			}
			cfg := config.Defaults()
			cfg.Workspace = false
			cfg.UpdateGitignore = false
			if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
				t.Fatal(err)
			}
			pack, err := BuildContext(ContextRequest{Root: root, Query: query, ProtocolVersion: AdaptiveV2})
			if err != nil {
				t.Fatal(err)
			}
			compilerCall, source := false, false
			for _, edge := range pack.CallChain {
				compilerCall = compilerCall || strings.Contains(edge.Reason, "compiler symbol")
			}
			for _, section := range pack.SourceSections {
				source = source || section.Path == "Service."+ext && section.ReadReceipt != ""
			}
			if !compilerCall || !source {
				t.Fatalf("compiler call=%v source=%v pack=%#v", compilerCall, source, pack)
			}
		})
	}
}
