package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceImpactMapsChangedFileToFeatureDossier(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	dossiers := []FeatureDossierRecord{{
		ID:              "feature:get-users",
		Route:           "GET /users/{userId}",
		FrontendProject: "frontend/app",
		BackendProject:  "services/ms-user",
		BackendHandler:  "UserController.get",
		SourceFlowID:    "flow:get-users",
		Confidence:      "MATCHED",
	}}
	flows := []WorkspaceFeatureFlowRecord{{
		ID:              "flow:get-users",
		FrontendProject: "frontend/app",
		FrontendFile:    "src/api/users.ts",
		BackendProject:  "services/ms-user",
		BackendFile:     "src/main/java/UserController.java",
		HTTPMethod:      "GET",
		Path:            "/users/{userId}",
		FieldRisks:      []FieldRiskRecord{{Kind: "missing_field", Reason: "frontend field not returned"}},
	}}
	layout := NewWorkspaceOutputLayout(out)
	if err := writeJSON(layout.Index("feature-dossiers.json"), dossiers); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(layout.Index("feature-flows.json"), flows); err != nil {
		t.Fatal(err)
	}

	impact, err := WorkspaceImpact(out, []string{"frontend/app/src/api/users.ts"})
	if err != nil {
		t.Fatal(err)
	}
	report := RenderWorkspaceImpact(impact)
	for _, want := range []string{"GET /users/{userId}", "frontend/app", "risk"} {
		if !strings.Contains(strings.ToLower(report), strings.ToLower(want)) {
			t.Fatalf("impact report missing %q:\n%s", want, report)
		}
	}
}
