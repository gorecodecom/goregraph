package scan

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProductionCodeContainsNoAuditedPrivateIdentifiers(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller did not return the test file")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	forbidden := []string{
		"0442483",
		"cadaster",
		"rdbv",
		"regulationchangebasecontroller",
		"vorschrift",
		"weka.request",
	}

	for _, directory := range []string{"cmd", "internal", "scripts"} {
		root := filepath.Join(repositoryRoot, directory)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") || !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".sh") {
				return nil
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			scanner := bufio.NewScanner(file)
			for line := 1; scanner.Scan(); line++ {
				lower := strings.ToLower(scanner.Text())
				for _, identifier := range forbidden {
					if strings.Contains(lower, identifier) {
						t.Errorf("private identifier %q in %s:%d", identifier, filepath.ToSlash(path), line)
					}
				}
			}
			scanErr := scanner.Err()
			if closeErr := file.Close(); scanErr == nil {
				scanErr = closeErr
			}
			return scanErr
		})
		if err != nil {
			t.Fatalf("scan production directory %s: %v", directory, err)
		}
	}
}
