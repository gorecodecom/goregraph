package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func BenchmarkProjectAgentBuild(b *testing.B) {
	root := b.TempDir()
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	for file := 0; file < 24; file++ {
		var source strings.Builder
		for fn := 0; fn < 40; fn++ {
			fmt.Fprintf(&source, "export function operation%d(input: string): string { const value = input.trim(); return value.toUpperCase(); }\n", fn)
		}
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("module%d.ts", file)), []byte(source.String()), 0644); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := RunBuild(root, cfg, BuildTargetAgent); err != nil {
			b.Fatal(err)
		}
	}
}
