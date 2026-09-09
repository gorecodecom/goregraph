package gitignore

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestScopedIgnorePatterns(t *testing.T) {
	tests := []struct {
		name, rules, path string
		dir, ignored      bool
	}{
		{"nested directory", "dist-offline/\n", "apps/web/dist-offline", true, true},
		{"directory descendants", "dist-offline/\n", "apps/web/dist-offline/bundle.js", false, true},
		{"directory name is file", "dist-offline/\n", "apps/web/dist-offline", false, false},
		{"anchored directory", "/dist-offline/\n", "apps/web/dist-offline", true, false},
		{"anchored root", "/dist-offline/\n", "dist-offline", true, true},
		{"slash pattern anchored", "web/dist/\n", "apps/web/dist", true, false},
		{"recursive wildcard zero", "apps/**/generated/*.js\n", "apps/generated/file.js", false, true},
		{"recursive wildcard many", "apps/**/generated/*.js\n", "apps/web/ui/generated/file.js", false, true},
		{"wildcard excludes slash", "apps/*/generated.js\n", "apps/web/ui/generated.js", false, false},
		{"excluded parent cannot reopen child", "build/\n!build/keep.ts\n", "build/keep.ts", false, true},
		{"reopened directory", "build/\n!build/\n", "build/keep.ts", false, false},
		{"escaped comment", "\\#notes\n", "#notes", false, true},
		{"escaped negation", "\\!notes\n", "!notes", false, true},
		{"escaped trailing space", "notes\\ \n", "notes ", false, true},
		{"unescaped trailing space", "notes  \n", "notes", false, true},
		{"leading space", " notes\n", " notes", false, true},
		{"leading space distinct", " notes\n", "notes", false, false},
		{"character class", "file[0-9].log\n", "logs/file2.log", false, true},
		{"POSIX character class", "file[[:digit:]].log\n", "logs/file2.log", false, true},
		{"negative class excludes slash", "apps/x[!a]y/file.ts\n", "apps/x/y/file.ts", false, false},
		{"negative character class", "file[!0-9].log\n", "logs/filex.log", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.rules).Ignored(tt.path, tt.dir); got != tt.ignored {
				t.Fatalf("Ignored(%q, %v) = %v, want %v", tt.path, tt.dir, got, tt.ignored)
			}
		})
	}
}

func TestScopedChildRulesDoNotLeakToSiblings(t *testing.T) {
	parent := Parse("*.log\n")
	child := parent.WithFile("apps/web", "!keep.log\n/generated/\n")
	for _, tt := range []struct {
		path    string
		ignored bool
	}{
		{"apps/web/keep.log", false}, {"apps/other/keep.log", true},
		{"apps/web/generated/output.ts", true}, {"apps/web/src/generated/output.ts", false},
	} {
		if got := child.Ignored(tt.path, false); got != tt.ignored {
			t.Fatalf("%s: got %v", tt.path, got)
		}
	}
	if !parent.Ignored("apps/web/keep.log", false) {
		t.Fatal("child mutated parent rules")
	}
}

func TestIgnoreParityWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git oracle unavailable")
	}
	root := t.TempDir()
	rules := "dist-offline/\n/root-only/\napps/**/generated/*.js\n*.log\n!important.log\nlocked/\n!locked/keep.ts\n\\#note\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(rules), 0644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) *exec.Cmd {
		command := exec.Command("git", args...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(root, "empty-config"))
		return command
	}
	if output, err := run("init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, output)
	}
	matcher := Parse(rules)
	for _, name := range []string{"apps/web/dist-offline/index.js", "apps/root-only/file.ts", "root-only/file.ts", "apps/generated/file.js", "apps/web/generated/file.js", "logs/output.log", "logs/important.log", "locked/keep.ts", "#note", "src/main.ts"} {
		err := run("check-ignore", "--no-index", "-q", "--", name).Run()
		ignored := err == nil
		if exit, ok := err.(*exec.ExitError); err != nil && (!ok || exit.ExitCode() != 1) {
			t.Fatalf("oracle %s: %v", name, err)
		}
		if got := matcher.Ignored(name, false); got != ignored {
			t.Errorf("%s: matcher=%v git=%v", name, got, ignored)
		}
	}
}
