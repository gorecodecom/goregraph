package outputstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkPreparedWorkspace compares two private overlay passes inside one
// recoverable publication, including final validation and durable file writes.
func BenchmarkPreparedWorkspace(b *testing.B) {
	for _, private := range []bool{false, true} {
		name := "nested"
		if private {
			name = "private"
		}
		b.Run(name, func(b *testing.B) {
			ctx := context.Background()
			parent := b.TempDir()
			roots := make([]string, 8)
			requests := make([]UpdateRequest, len(roots))
			for i := range roots {
				root, err := canonicalRoot(filepath.Join(parent, fmt.Sprintf("project-%02d", i)))
				if err != nil {
					b.Fatal(err)
				}
				roots[i] = root
				requests[i] = UpdateRequest{Root: root, Write: func(stage string) error {
					for file := 0; file < 32; file++ {
						if err := os.WriteFile(filepath.Join(stage, fmt.Sprintf("file-%03d", file)), []byte(strings.Repeat("x", 4096)), 0644); err != nil {
							return err
						}
					}
					return nil
				}}
			}
			if err := UpdateMany(ctx, requests); err != nil {
				b.Fatal(err)
			}
			copies, flushes := 0, 0
			observer := func(event Event) {
				if event.Outcome != "completed" {
					return
				}
				switch event.Phase {
				case "copy":
					copies++
				case "sync":
					flushes++
				}
			}
			snapshots := map[string]string{}
			for i := range requests {
				requests[i].Observe = observer
				requests[i].Write = func(stage string) error {
					snapshot := snapshots[stage]
					entries, err := os.ReadDir(snapshot)
					if err != nil {
						return err
					}
					for _, entry := range entries {
						if err := os.Rename(filepath.Join(snapshot, entry.Name()), filepath.Join(stage, entry.Name())); err != nil {
							return err
						}
					}
					return os.Remove(snapshot)
				}
				requests[i].Validate = func(stage string) error {
					body, err := os.ReadFile(filepath.Join(stage, "file-000"))
					if err == nil && len(body) != 4096 {
						return fmt.Errorf("invalid snapshot size: %d", len(body))
					}
					return err
				}
			}
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				err := UpdateManyPrepared(ctx, requests, func(stages map[string]string) error {
					overlays := make([]UpdateRequest, 0, len(roots))
					for _, root := range roots {
						stage := stages[root]
						snapshot, err := os.MkdirTemp(stage, ".prepared-")
						if err != nil {
							return err
						}
						snapshots[stage] = snapshot
						entries, err := os.ReadDir(stage)
						if err != nil {
							return err
						}
						for _, entry := range entries {
							if entry.Name() == filepath.Base(snapshot) {
								continue
							}
							if err := os.Rename(filepath.Join(stage, entry.Name()), filepath.Join(snapshot, entry.Name())); err != nil {
								return err
							}
						}
						overlays = append(overlays, UpdateRequest{Root: snapshot, Observe: observer, Write: func(stage string) error {
							return os.WriteFile(filepath.Join(stage, "file-000"), []byte(strings.Repeat("y", 4096)), 0644)
						}})
					}
					for pass := 0; pass < 2; pass++ {
						if private {
							if err := PreparePrivate(ctx, overlays, nil); err != nil {
								return err
							}
						} else {
							if err := UpdateMany(ctx, overlays); err != nil {
								return err
							}
						}
					}
					return nil
				}, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(copies)/float64(b.N), "root-copies/op")
			b.ReportMetric(float64(flushes)/float64(b.N), "root-syncs/op")
		})
	}
}
