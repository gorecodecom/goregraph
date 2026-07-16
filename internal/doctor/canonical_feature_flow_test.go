package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDoctorRejectsInvalidCanonicalFeatureFlow(t *testing.T) {
	out := t.TempDir()
	flows := []scan.WorkspaceFeatureFlowRecord{{
		ModelVersion: 1,
		Nodes:        []scan.CanonicalFlowNodeRecord{{ID: "node:a", Kind: "api_call"}},
		Edges:        []scan.CanonicalFlowEdgeRecord{{ID: "edge:a", FromNodeID: "node:a", ToNodeID: "node:missing", EdgeType: "invokes_api", Confidence: "RESOLVED", Reason: "matched"}},
	}}
	body, err := json.Marshal(flows)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(out, "index"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "index", "workspace-feature-flows.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	result := Result{}
	checkCanonicalFeatureFlows(out, &result)
	if result.Failures != 1 {
		t.Fatalf("doctor result=%#v", result)
	}
}
