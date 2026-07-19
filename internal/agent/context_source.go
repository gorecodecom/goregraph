package agent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type sourceCandidate struct {
	FactID    string
	Project   string
	Path      string
	StartLine int
	EndLine   int
	Role      string
	Kind      string
	Name      string
	Qualified string
	Priority  int
}

type sourceFile struct {
	Path  string
	Lines []string
}

func resolveSourcePath(loaded loadedContextIndex, candidate sourceCandidate) (string, error) {
	path := strings.TrimSpace(candidate.Path)
	if path == "" || filepath.IsAbs(path) {
		return "", fmt.Errorf("source path is unsafe")
	}

	parts := []string{loaded.ScopeRoot}
	if loaded.Workspace {
		project := strings.TrimSpace(candidate.Project)
		if project == "" || filepath.IsAbs(project) {
			return "", fmt.Errorf("source path is unsafe")
		}
		parts = append(parts, project)
	}
	parts = append(parts, path)
	resolvedCandidate := filepath.Clean(filepath.Join(parts...))
	if !pathIsWithin(loaded.ScopeRoot, resolvedCandidate) {
		return "", fmt.Errorf("source path is unsafe")
	}

	resolvedRoot, err := filepath.EvalSymlinks(loaded.ScopeRoot)
	if err != nil {
		return "", fmt.Errorf("source path is unsafe")
	}
	resolvedCandidate, err = filepath.EvalSymlinks(resolvedCandidate)
	if err != nil {
		return "", fmt.Errorf("source file is unreadable: %w", err)
	}
	if !pathIsWithin(resolvedRoot, resolvedCandidate) {
		return "", fmt.Errorf("source path is unsafe")
	}

	info, err := os.Stat(resolvedCandidate)
	if err != nil {
		return "", fmt.Errorf("source file is unreadable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("source file is not regular")
	}
	return resolvedCandidate, nil
}

func pathIsWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func readSourceFile(path string) (sourceFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return sourceFile{}, fmt.Errorf("source file is unreadable: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return sourceFile{}, fmt.Errorf("source file is unreadable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return sourceFile{}, fmt.Errorf("source file is not regular")
	}
	if info.Size() > MaxContextSourceFileBytes {
		return sourceFile{}, fmt.Errorf("source file exceeds maximum size")
	}
	body, err := io.ReadAll(io.LimitReader(file, MaxContextSourceFileBytes+1))
	if err != nil {
		return sourceFile{}, fmt.Errorf("source file is unreadable: %w", err)
	}
	if len(body) > MaxContextSourceFileBytes {
		return sourceFile{}, fmt.Errorf("source file exceeds maximum size")
	}
	if !utf8.Valid(body) {
		return sourceFile{}, fmt.Errorf("source file is not valid UTF-8")
	}
	return sourceFile{
		Path:  path,
		Lines: strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n"),
	}, nil
}
