package agent

import (
	"os"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestRealUnityPersistentEventContext(t *testing.T) {
	root := os.Getenv("GOREGRAPH_UNITY_EVENT_SMOKE_ROOT")
	if root == "" {
		t.Skip("real Unity serialized fixture not supplied")
	}
	assertUnityPersistentEventContext(t, root)
}

func TestUnityPersistentEventContextRetainsNamedReceiverAfterPrimaryField(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "Assets/Receiver.cs", "public class Receiver {public void Press(){} public void Press(int value){}}")
	writeSourceFile(t, root, "Assets/Receiver.cs.meta", "guid: 11111111111111111111111111111111\n")
	writeSourceFile(t, root, "Assets/Events.cs", "public class Events {public int clicked;}")
	writeSourceFile(t, root, "Assets/Events.cs.meta", "guid: 22222222222222222222222222222222\n")
	writeSourceFile(t, root, "Assets/Events.prefab", `%YAML 1.1
--- !u!114 &100
MonoBehaviour:
  m_Script: {fileID: 11500000, guid: 11111111111111111111111111111111, type: 3}
--- !u!114 &200
MonoBehaviour:
  m_Script: {fileID: 11500000, guid: 22222222222222222222222222222222, type: 3}
  clicked:
    m_PersistentCalls:
      m_Calls:
      - m_Target: {fileID: 100}
        m_MethodName: Press
        m_Mode: 1
`)
	assertUnityPersistentEventContext(t, root)
}

func assertUnityPersistentEventContext(t *testing.T, root string) {
	t.Helper()
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain the Events.clicked UnityEvent persistent connection to Receiver.Press and its saved callback", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	foundCode, foundAsset := false, false
	for _, section := range pack.SourceSections {
		if section.Path == "Assets/Receiver.cs" && section.ReadReceipt != "" {
			foundCode = true
		}
		if section.Path == "Assets/Events.prefab" && strings.Contains(section.Content, "m_MethodName: Press") && section.ReadReceipt != "" {
			foundAsset = true
		}
	}
	if !foundCode || !foundAsset {
		t.Fatalf("code=%v asset=%v pack=%#v", foundCode, foundAsset, pack)
	}
	if len(pack.CallChain) != 0 || pack.EstimatedTokens > DefaultContextBudgetTokens {
		t.Fatal("saved callbacks became execution claims or exceeded the budget", pack)
	}
}
