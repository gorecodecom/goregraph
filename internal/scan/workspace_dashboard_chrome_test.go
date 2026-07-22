package scan

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestWorkspaceDashboardPanelControlsArePersistentAndAccessible(t *testing.T) {
	html := RenderWorkspaceDashboardHTML(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "C:/workspaces/example"},
		nil,
		nil,
	)

	for _, want := range []string{
		`id="workspace-shell"`,
		`id="workspace-sidebar"`,
		`id="toggle-left-panel"`,
		`aria-controls="workspace-sidebar"`,
		`id="toggle-right-panel"`,
		`aria-controls="details"`,
		`function loadDashboardPanelState(storage,key)`,
		`function saveDashboardPanelState(storage,key,value)`,
		`function applyDashboardPanelState(value)`,
		`function setDashboardPanelVisibility(side,visible)`,
		`localStorage`,
		`Show navigation panel`,
		`Show details panel`,
		`left-panel-hidden`,
		`right-panel-hidden`,
		`@media (max-width:1240px){.shell.left-panel-hidden,.shell.right-panel-hidden,.shell.left-panel-hidden.right-panel-hidden{grid-template-columns:1fr}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard panel contract missing %q", want)
		}
	}
}

func TestWorkspaceDashboardPanelStatePersistsIndependently(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for dashboard panel state tests")
	}
	prefixEnd := strings.Index(workspaceDashboardScript, "const workspaceGraph")
	if prefixEnd < 0 {
		t.Fatal("dashboard script is missing the panel helper boundary")
	}
	source := workspaceDashboardScript[:prefixEnd] + `
const values=new Map();
const storage={getItem:key=>values.has(key)?values.get(key):null,setItem:(key,value)=>values.set(key,value)};
const key=dashboardPanelStorageKey("C:/workspaces/example",3);
const initial=loadDashboardPanelState(storage,key);
const leftHidden=toggleDashboardPanelState(initial,"left");
saveDashboardPanelState(storage,key,leftHidden);
const restored=loadDashboardPanelState(storage,key);
const bothHidden=toggleDashboardPanelState(restored,"right");
const malformed=loadDashboardPanelState({getItem:()=>"not json"},key);
const blocked=loadDashboardPanelState({getItem:()=>{throw new Error("blocked")}},key);
process.stdout.write(JSON.stringify({key,initial,leftHidden,restored,bothHidden,malformed,blocked}));`
	output, err := nodeScriptCommand(node, source).CombinedOutput()
	if err != nil {
		t.Fatalf("dashboard panel state script failed: %v\n%s", err, output)
	}
	var got struct {
		Key        string                                   `json:"key"`
		Initial    struct{ LeftVisible, RightVisible bool } `json:"initial"`
		LeftHidden struct{ LeftVisible, RightVisible bool } `json:"leftHidden"`
		Restored   struct{ LeftVisible, RightVisible bool } `json:"restored"`
		BothHidden struct{ LeftVisible, RightVisible bool } `json:"bothHidden"`
		Malformed  struct{ LeftVisible, RightVisible bool } `json:"malformed"`
		Blocked    struct{ LeftVisible, RightVisible bool } `json:"blocked"`
	}
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("decode dashboard panel state: %v\n%s", err, output)
	}
	if !strings.Contains(got.Key, "3") || !strings.Contains(got.Key, "C%3A%2Fworkspaces%2Fexample") {
		t.Fatalf("panel storage key is not workspace/schema scoped: %q", got.Key)
	}
	if !got.Initial.LeftVisible || !got.Initial.RightVisible {
		t.Fatalf("initial panel state = %#v", got.Initial)
	}
	if got.LeftHidden.LeftVisible || !got.LeftHidden.RightVisible || got.Restored != got.LeftHidden {
		t.Fatalf("left panel state did not persist: hidden=%#v restored=%#v", got.LeftHidden, got.Restored)
	}
	if got.BothHidden.LeftVisible || got.BothHidden.RightVisible {
		t.Fatalf("right panel did not toggle independently: %#v", got.BothHidden)
	}
	if !got.Malformed.LeftVisible || !got.Malformed.RightVisible || !got.Blocked.LeftVisible || !got.Blocked.RightVisible {
		t.Fatalf("storage fallback is not safe: malformed=%#v blocked=%#v", got.Malformed, got.Blocked)
	}
}

func TestWorkspaceDashboardCodeExplorerGroupsAreCollapsible(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion},
	)

	for _, want := range []string{
		`codeExpandedGroups:new Set()`,
		`function codeGroupIsOpen(group,records)`,
		`data-code-group="`,
		`class="code-symbol-group-toggle"`,
		`aria-expanded="`,
		`aria-controls="`,
		`records.length+" symbols"`,
		`class="code-symbol-group-body"`,
		`.code-symbol-group-toggle:focus-visible`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("Code Explorer disclosure contract missing %q", want)
		}
	}
}
