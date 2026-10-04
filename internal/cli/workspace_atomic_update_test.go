package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/doctor"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestWorkspaceUpdateKeepsCommittedProjectsAndProjectionTogether(t *testing.T) {
	for _, failure := range []string{"none", "reconcile", "cancel", "source-change", "output-change", "project-added"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			outputs := []string{filepath.Join(root, ".goregraph-workspace")}
			for _, project := range []string{"a", "b"} {
				path := filepath.Join(root, "services", project)
				writeFile(t, path, "go.mod", "module example.test/"+project+"\n")
				writeFile(t, path, "main.go", "package service\ntype Service struct {}\nfunc (Service) Value() string { return \"old\" }\n")
				if project == "b" {
					writeFile(t, path, "goregraph.yml", "output: custom-output\n")
					outputs = append(outputs, filepath.Join(path, "custom-output"))
				} else {
					outputs = append(outputs, filepath.Join(path, "goregraph-out"))
				}
			}
			var initial, diagnostic bytes.Buffer
			if code := Run([]string{"workspace", "build", "all", root, "--workspace", root, "--no-update-gitignore", "--progress", "off"}, &initial, &diagnostic); code != 0 {
				t.Fatalf("initial build: %d %s", code, diagnostic.String())
			}
			before := workspaceOutputSnapshot(t, outputs)
			for _, project := range []string{"a", "b"} {
				writeFile(t, filepath.Join(root, "services", project), "main.go", "package service\n\ntype Service struct {}\nfunc (Service) Value() string { return \"new\" }\n")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready, release := make(chan struct{}), make(chan struct{})
			finish := sync.OnceFunc(func() { close(release) })
			options := scan.DefaultBuildOptions()
			options.Observer = func(event scan.BuildEvent) {
				if event.Phase == "workspace-project" && event.Project == "services/a" && event.Outcome == "completed" {
					close(ready)
					<-release
					if failure == "cancel" {
						cancel()
					}
					if failure == "output-change" {
						writeFile(t, filepath.Join(root, "services/b"), "goregraph.yml", "output: changed-output\n")
					}
					if failure == "project-added" {
						writeFile(t, filepath.Join(root, "services/c"), "go.mod", "module example.test/c\n")
						writeFile(t, filepath.Join(root, "services/c"), "main.go", "package service\nfunc AddedService() {}\n")
					}
				}
				if event.Phase == "reconcile" && failure == "reconcile" {
					writeFile(t, root, scan.WorkspaceDashboardConfigName, `{"schema":1,"architecture":{"services":{"services/a":{"group":"missing"}}}}`)
				}
				if event.Phase == "reconcile" && failure == "source-change" {
					writeFile(t, filepath.Join(root, "services/a"), "main.go", "package service\nfunc ChangedAfterAnalysis() {}\n")
				}
			}
			var stdout, stderr bytes.Buffer
			done, exited := make(chan int, 1), make(chan struct{})
			go func() {
				defer close(exited)
				done <- runWorkspaceUpdate([]string{root, "--workspace", root, "--no-update-gitignore"}, &stdout, &stderr, buildExecution{ctx: ctx, options: options})
			}()
			defer func() { cancel(); finish(); <-exited }()
			select {
			case <-ready:
			case code := <-done:
				t.Fatalf("update stopped before preparing the first project: %d %s", code, stderr.String())
			case <-time.After(60 * time.Second):
				t.Fatal("first project did not finish preparation")
			}
			if !reflect.DeepEqual(before, workspaceOutputSnapshot(t, outputs)) {
				t.Fatal("a project was published before the workspace projection was ready")
			}
			assertWorkspaceProjectionValid(t, root)
			finish()
			var code int
			select {
			case code = <-done:
			case <-time.After(60 * time.Second):
				t.Fatal("workspace update did not complete")
			}
			after := workspaceOutputSnapshot(t, outputs)
			if failure != "none" && failure != "source-change" && failure != "project-added" {
				if code == 0 || !reflect.DeepEqual(before, after) {
					t.Fatalf("failed update changed committed outputs: code=%d stderr=%s", code, stderr.String())
				}
				assertWorkspaceProjectionValid(t, root)
				if _, err := os.Stat(filepath.Join(root, "services/b/changed-output")); !os.IsNotExist(err) {
					t.Fatalf("changed output root was written outside the transaction: %v", err)
				}
				return
			}
			if code != 0 || reflect.DeepEqual(before, after) {
				t.Fatalf("successful update did not publish: code=%d stderr=%s", code, stderr.String())
			}
			assertWorkspaceProjectionValid(t, root)
			var generation string
			for _, output := range outputs {
				body, err := os.ReadFile(filepath.Join(output, ".goregraph-generation"))
				if err != nil {
					t.Fatal(err)
				}
				if generation != "" && generation != string(body) {
					t.Fatal("related outputs have different committed generations")
				}
				generation = string(body)
			}
			if failure == "source-change" || failure == "project-added" {
				stdout.Reset()
				stderr.Reset()
				if code := runWorkspaceUpdate([]string{root, "--workspace", root, "--no-update-gitignore"}, &stdout, &stderr, buildExecution{ctx: context.Background(), options: scan.DefaultBuildOptions()}); code != 0 {
					t.Fatalf("follow-up update did not catch changes made during preparation: %d %s", code, stderr.String())
				}
				assertWorkspaceProjectionValid(t, root)
				if failure == "source-change" {
					body, err := os.ReadFile(filepath.Join(root, "services/a/goregraph-out/index/files.json"))
					if err != nil {
						t.Fatal(err)
					}
					var files []scan.FileRecord
					if err := json.Unmarshal(body, &files); err != nil {
						t.Fatal(err)
					}
					source, err := os.ReadFile(filepath.Join(root, "services/a/main.go"))
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, file := range files {
						if file.Path == "main.go" {
							found = file.Hash == fmt.Sprintf("%x", sha256.Sum256(source))
						}
					}
					if !found {
						t.Fatal("follow-up update did not index the changed source")
					}
				} else if _, err := os.Stat(filepath.Join(root, "services/c/goregraph-out/agent/context-index.json")); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestWorkspaceUpdateInitializesNewProjectWithoutBlockingCommittedReaders(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "services/a")
	writeFile(t, project, "go.mod", "module example.test/a\n")
	writeFile(t, project, "main.go", "package service\nfunc ExistingService() {}\n")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"workspace", "build", "all", root, "--workspace", root, "--no-update-gitignore", "--progress", "off"}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	assertWorkspaceProjectionValid(t, root)
	before := workspaceOutputSnapshot(t, []string{filepath.Join(root, ".goregraph-workspace"), filepath.Join(project, "goregraph-out")})
	newProject := filepath.Join(root, "services/b")
	writeFile(t, newProject, "go.mod", "module example.test/b\n")
	writeFile(t, newProject, "main.go", "package service\nfunc NewService() {}\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready, release := make(chan struct{}), make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	options := scan.DefaultBuildOptions()
	options.Observer = func(event scan.BuildEvent) {
		if event.Phase == "workspace-project" && event.Project == "services/b" && event.Outcome == "completed" {
			close(ready)
			<-release
		}
	}
	done, exited := make(chan int, 1), make(chan struct{})
	stdout.Reset()
	stderr.Reset()
	go func() {
		defer close(exited)
		done <- runWorkspaceUpdate([]string{root, "--workspace", root, "--no-update-gitignore"}, &stdout, &stderr, buildExecution{ctx: ctx, options: options})
	}()
	defer func() { cancel(); finish(); <-exited }()
	select {
	case <-ready:
	case code := <-done:
		t.Fatalf("new project was not prepared: %d %s", code, stderr.String())
	case <-time.After(60 * time.Second):
		t.Fatal("new project was not prepared")
	}
	readContext, cancelRead := context.WithTimeout(context.Background(), time.Second)
	defer cancelRead()
	if err := scan.WithExistingOutputRead(readContext, root, func() error { return nil }); err != nil {
		t.Fatalf("committed workspace reader blocked on the new project: %v", err)
	}
	if !reflect.DeepEqual(before, workspaceOutputSnapshot(t, []string{filepath.Join(root, ".goregraph-workspace"), filepath.Join(project, "goregraph-out")})) {
		t.Fatal("committed snapshots changed while a new project was being prepared")
	}
	if _, err := os.Stat(filepath.Join(newProject, "goregraph-out")); !os.IsNotExist(err) {
		t.Fatalf("new project was published too early: %v", err)
	}
	assertWorkspaceProjectionValid(t, root)
	finish()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("new project publication failed: %d %s", code, stderr.String())
		}
	case <-time.After(60 * time.Second):
		t.Fatal("new project publication did not complete")
	}
	assertWorkspaceProjectionValid(t, root)
	if _, err := os.Stat(filepath.Join(newProject, "goregraph-out/agent/context-index.json")); err != nil {
		t.Fatal(err)
	}
}

func workspaceOutputSnapshot(t *testing.T, outputs []string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, output := range outputs {
		if err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result[path] = fmt.Sprintf("%x", sha256.Sum256(body))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func assertWorkspaceProjectionValid(t *testing.T, root string) {
	t.Helper()
	result, err := doctor.Run(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range result.Lines {
		if strings.HasPrefix(line, "FAIL workspace-symbols") {
			t.Fatal(line)
		}
		if strings.Contains(line, "workspace symbol projection valid") {
			found = true
		}
	}
	if !found {
		t.Fatalf("workspace projection was not validated: %+v", result)
	}
}
