package scan

import "testing"

func TestBuildVerificationCommandsUsesDetectedMavenTest(t *testing.T) {
	project := WorkspaceProjectRecord{Path: "services/users", Kind: "maven"}
	tests := []TestMapRecord{{TestFile: "src/test/java/UserControllerTest.java", TestClass: "UserControllerTest", TestMethod: "deletesUser", Confidence: "EXACT"}}
	got := BuildVerificationCommands(project, tests)
	if len(got) != 1 || got[0].Tool != "maven" || got[0].WorkingDirectory != "services/users" || len(got[0].Args) == 0 {
		t.Fatalf("commands=%#v", got)
	}
}

func TestBuildVerificationCommandsSupportsDetectedRunners(t *testing.T) {
	cases := []struct {
		runner string
		file   string
	}{
		{"gradle", "src/test/java/UserTest.java"},
		{"jest", "src/user test.ts"},
		{"vitest", "src/user.test.ts"},
		{"playwright", "tests/user.spec.ts"},
	}
	for _, tc := range cases {
		t.Run(tc.runner, func(t *testing.T) {
			got := BuildVerificationCommands(WorkspaceProjectRecord{Path: `frontend\app`, TestRunner: tc.runner}, []TestMapRecord{{TestFile: tc.file, TestMethod: "works", Confidence: "INFERRED"}})
			if len(got) != 1 || got[0].Tool != tc.runner || got[0].Display == "" || got[0].WorkingDirectory != "frontend/app" {
				t.Fatalf("commands=%#v", got)
			}
		})
	}
}

func TestBuildVerificationCommandsReportsUnsupportedRunner(t *testing.T) {
	got := BuildVerificationCommands(WorkspaceProjectRecord{Path: "app", Kind: "backend"}, []TestMapRecord{{TestFile: "test.custom"}})
	if len(got) != 1 || got[0].MissingPrerequisite == "" || got[0].Display != "" {
		t.Fatalf("unsupported command=%#v", got)
	}
}

func TestBuildTestLinksDistinguishesEvidenceStrength(t *testing.T) {
	flow := WorkspaceFeatureFlowRecord{ID: "flow", Tests: []TestMapRecord{{TestFile: "DirectTest.java", Confidence: "EXACT", Reason: "calls endpoint"}, {TestFile: "CandidateTest.java", Confidence: "WEAK_MATCH", Reason: "similar name"}}}
	links := BuildTestLinks(flow)
	if len(links) != 2 || links[0].Relation != "candidate" && links[1].Relation != "candidate" {
		t.Fatalf("links=%#v", links)
	}
	missing := BuildTestLinks(WorkspaceFeatureFlowRecord{ID: "missing", TestReason: "No linked test detected."})
	if len(missing) != 1 || missing[0].Relation != "not_detected" {
		t.Fatalf("missing links=%#v", missing)
	}
}
