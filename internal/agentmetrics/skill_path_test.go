package agentmetrics

import "testing"

func TestClassifyExternalSkillTarget(t *testing.T) {
	tests := []struct {
		name             string
		workspace        string
		commandDirectory string
		target           string
		want             string
		matched          bool
	}{
		{
			name:             "macOS skill file",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/Users/me/.codex/skills/brainstorming/SKILL.md",
			want:             "/Users/me/.codex/skills/brainstorming/SKILL.md",
			matched:          true,
		},
		{
			name:             "Linux plugin reference",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/opt/codex/plugins/vendor/skills/tdd/references/guide.md",
			want:             "/opt/codex/plugins/vendor/skills/tdd/references/guide.md",
			matched:          true,
		},
		{
			name:             "relative target from external skill directory",
			workspace:        "/work/repo",
			commandDirectory: "/opt/codex/plugins/vendor/skills/review",
			target:           "references/checklist.md",
			want:             "/opt/codex/plugins/vendor/skills/review/references/checklist.md",
			matched:          true,
		},
		{
			name:             "Windows skill file",
			workspace:        `C:\\work\\repo`,
			commandDirectory: `C:\\work\\repo`,
			target:           `C:\\Users\\Me\\.codex\\skills\\TDD\\SKILL.md`,
			want:             "c:/users/me/.codex/skills/tdd/skill.md",
			matched:          true,
		},
		{
			name:             "workspace-local skill fixture",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/work/repo/testdata/skills/example/SKILL.md",
			matched:          false,
		},
		{
			name:             "ordinary external source",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/opt/source/Service.java",
			matched:          false,
		},
		{
			name:             "similar component is not skill directory",
			workspace:        "/work/repo",
			commandDirectory: "/work/repo",
			target:           "/opt/skills-old/review/guide.md",
			matched:          false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, matched := ClassifyExternalSkillTarget(
				test.workspace,
				test.commandDirectory,
				test.target,
			)
			if got != test.want || matched != test.matched {
				t.Fatalf("classification = %q, %v; want %q, %v", got, matched, test.want, test.matched)
			}
		})
	}
}

func TestClassifyExternalSkillTargetRejectsAmbiguousRoots(t *testing.T) {
	for _, values := range [][3]string{
		{"work/repo", "/work/repo", "/opt/skills/tdd/SKILL.md"},
		{"/work/repo", "work/repo", "SKILL.md"},
		{"/work/repo", "/work/repo", ""},
	} {
		if got, ok := ClassifyExternalSkillTarget(values[0], values[1], values[2]); ok || got != "" {
			t.Fatalf("ambiguous classification = %q, %v for %#v", got, ok, values)
		}
	}
}
