package gitupdate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type repositoryState struct {
	root              string
	remoteURL         string
	branch            string
	head              string
	targetBranch      string
	targetCommit      string
	targetLocalExists bool
	dirty             bool
	operation         string
	currentComparison branchComparison
	targetComparison  branchComparison
	worktreeConflict  string
}

type branchComparison struct {
	branch   string
	remote   string
	ahead    int
	behind   int
	compared bool
}

func canonicalGitRoot(ctx context.Context, path string) (string, error) {
	dir, err := commandDirectory(path)
	if err != nil {
		return "", err
	}

	root, err := runGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return canonicalPath(root)
}

func commandDirectory(path string) (string, error) {
	if path == "" {
		path = "."
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return filepath.Dir(path), nil
	}
	return path, nil
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func inspectRepository(ctx context.Context, root string) (repositoryState, error) {
	state := repositoryState{root: root}
	var err error

	state.head, err = runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return state, err
	}
	state.branch, _ = runGit(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	state.operation, err = activeOperation(ctx, root)
	if err != nil {
		return state, err
	}
	status, err := runGit(ctx, root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return state, err
	}
	state.dirty = status != ""

	state.remoteURL, _ = runGit(ctx, root, "remote", "get-url", "origin")
	if state.remoteURL == "" {
		return state, nil
	}

	state.targetBranch, state.targetCommit = resolveDefaultBranch(ctx, root)
	if state.targetBranch == "" {
		return state, nil
	}
	_, state.targetLocalExists = refCommit(ctx, root, "refs/heads/"+state.targetBranch)

	if state.branch != "" {
		state.currentComparison, err = compareBranch(ctx, root, state.branch)
		if err != nil {
			return state, err
		}
	}
	if state.targetBranch != state.branch {
		state.targetComparison, err = compareBranch(ctx, root, state.targetBranch)
		if err != nil {
			return state, err
		}
	}
	state.worktreeConflict, err = targetBranchWorktree(ctx, root, state.targetBranch)
	if err != nil {
		return state, err
	}

	return state, nil
}

func activeOperation(ctx context.Context, root string) (string, error) {
	gitDir, err := runGit(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	operations := []string{
		"MERGE_HEAD",
		"CHERRY_PICK_HEAD",
		"REVERT_HEAD",
		"rebase-apply",
		"rebase-merge",
	}
	for _, operation := range operations {
		if _, err := os.Stat(filepath.Join(gitDir, operation)); err == nil {
			return operation, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect Git operation %s: %w", operation, err)
		}
	}
	return "", nil
}

func resolveDefaultBranch(ctx context.Context, root string) (string, string) {
	if remoteHead, err := runGit(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		const prefix = "origin/"
		if strings.HasPrefix(remoteHead, prefix) {
			branch := strings.TrimPrefix(remoteHead, prefix)
			if commit, exists := refCommit(ctx, root, "refs/remotes/"+remoteHead); exists {
				return branch, commit
			}
		}
	}

	mainCommit, hasMain := refCommit(ctx, root, "refs/remotes/origin/main")
	masterCommit, hasMaster := refCommit(ctx, root, "refs/remotes/origin/master")
	if hasMain == hasMaster {
		return "", ""
	}
	if hasMain {
		return "main", mainCommit
	}
	return "master", masterCommit
}

func refCommit(ctx context.Context, root, ref string) (string, bool) {
	commit, err := runGit(ctx, root, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return commit, err == nil && commit != ""
}

func compareBranch(ctx context.Context, root, branch string) (branchComparison, error) {
	comparison := branchComparison{
		branch: branch,
		remote: "origin/" + branch,
	}
	if _, exists := refCommit(ctx, root, "refs/heads/"+branch); !exists {
		return comparison, nil
	}
	if _, exists := refCommit(ctx, root, "refs/remotes/origin/"+branch); !exists {
		return comparison, nil
	}

	counts, err := runGit(ctx, root, "rev-list", "--left-right", "--count", branch+"...origin/"+branch)
	if err != nil {
		return comparison, err
	}
	fields := strings.Fields(counts)
	if len(fields) != 2 {
		return comparison, fmt.Errorf("unexpected rev-list count %q", counts)
	}
	comparison.ahead, err = strconv.Atoi(fields[0])
	if err != nil {
		return comparison, fmt.Errorf("parse ahead count %q: %w", fields[0], err)
	}
	comparison.behind, err = strconv.Atoi(fields[1])
	if err != nil {
		return comparison, fmt.Errorf("parse behind count %q: %w", fields[1], err)
	}
	comparison.compared = true
	return comparison, nil
}

func targetBranchWorktree(ctx context.Context, root, targetBranch string) (string, error) {
	output, err := runGit(ctx, root, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}

	var worktree string
	for _, line := range append(strings.Split(output, "\n"), "") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			worktree = strings.TrimPrefix(line, "worktree ")
		case line == "branch refs/heads/"+targetBranch:
			canonicalWorktree, canonicalErr := canonicalPath(worktree)
			if canonicalErr != nil {
				return "", canonicalErr
			}
			if canonicalWorktree != root {
				return canonicalWorktree, nil
			}
		case line == "":
			worktree = ""
		}
	}
	return "", nil
}
