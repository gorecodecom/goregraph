package gitupdate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
)

type repositoryTarget struct {
	target  Target
	root    string
	sortKey string
}

func Run(ctx context.Context, targets []Target, options Options) (Report, error) {
	report := Report{
		Mode:          ModePreview,
		WorkspaceRoot: options.WorkspaceRoot,
		Repositories:  make([]RepositoryResult, 0, len(targets)),
		Summary:       make(map[Status]int),
	}
	if options.Execute {
		report.Mode = ModeExecute
		return report, errors.New("Git update execution is not implemented")
	}

	repositories := collectRepositoryTargets(ctx, targets)
	for _, repository := range repositories {
		if err := ctx.Err(); err != nil {
			return report, err
		}

		var result RepositoryResult
		if repository.root == "" {
			result = RepositoryResult{
				Path:        repository.target.Path,
				Status:      StatusNotGit,
				Reason:      "path is not inside a Git repository",
				Remediation: "Choose a path inside a Git repository.",
			}
		} else {
			state, err := inspectRepository(ctx, repository.root)
			if err != nil {
				return report, fmt.Errorf("inspect %s: %w", repository.target.Path, err)
			}
			result = classifyPreview(repository.target, state)
		}
		report.Repositories = append(report.Repositories, result)
		report.Summary[result.Status]++
	}

	return report, nil
}

func collectRepositoryTargets(ctx context.Context, targets []Target) []repositoryTarget {
	repositories := make([]repositoryTarget, 0, len(targets))
	seenRoots := make(map[string]struct{})
	seenNonRepositories := make(map[string]struct{})

	for _, target := range targets {
		root, err := canonicalGitRoot(ctx, target.Path)
		if err == nil {
			if _, exists := seenRoots[root]; exists {
				continue
			}
			seenRoots[root] = struct{}{}
			repositories = append(repositories, repositoryTarget{target: target, root: root, sortKey: root})
			continue
		}

		sortKey := target.Path
		if absolute, absoluteErr := filepath.Abs(target.Path); absoluteErr == nil {
			sortKey = filepath.Clean(absolute)
		}
		if _, exists := seenNonRepositories[sortKey]; exists {
			continue
		}
		seenNonRepositories[sortKey] = struct{}{}
		repositories = append(repositories, repositoryTarget{target: target, sortKey: sortKey})
	}

	sort.Slice(repositories, func(i, j int) bool {
		return repositories[i].sortKey < repositories[j].sortKey
	})
	return repositories
}

func classifyPreview(target Target, state repositoryState) RepositoryResult {
	result := RepositoryResult{
		Path:         target.Path,
		GitRoot:      state.root,
		Remote:       state.remoteURL,
		BranchBefore: state.branch,
		BranchAfter:  state.targetBranch,
		CommitBefore: state.head,
		CommitAfter:  state.targetCommit,
	}

	switch {
	case state.operation != "":
		result.Status = StatusOperationProgress
		result.Reason = "a Git operation is in progress"
		result.Remediation = "Complete or abort the Git operation before updating."
	case state.dirty:
		result.Status = StatusDirty
		result.Reason = "repository has tracked or untracked changes"
		result.Remediation = "Commit, stash, or remove local changes before updating."
	case state.branch == "":
		result.Status = StatusDetachedHead
		result.Reason = "HEAD is detached"
		result.Remediation = "Switch to a local branch before updating."
	case state.remoteURL == "":
		result.Status = StatusMissingRemote
		result.Reason = "origin remote is not configured"
		result.Remediation = "Configure an origin remote before updating."
	case state.targetBranch == "":
		result.Status = StatusDefaultUnknown
		result.Reason = "cached origin refs do not identify an unambiguous default branch"
		result.Remediation = "Set origin/HEAD or keep exactly one of origin/main and origin/master."
	case comparisonDiverged(state.currentComparison):
		setDivergedResult(&result, state.currentComparison)
	case comparisonAhead(state.currentComparison):
		setAheadResult(&result, state.currentComparison)
	case comparisonDiverged(state.targetComparison):
		setDivergedResult(&result, state.targetComparison)
	case comparisonAhead(state.targetComparison):
		setAheadResult(&result, state.targetComparison)
	case state.worktreeConflict != "":
		result.Status = StatusBlockedWorktree
		result.Reason = fmt.Sprintf("target branch %s is checked out in worktree %s", state.targetBranch, state.worktreeConflict)
		result.Remediation = "Switch that worktree to another branch or remove it before updating."
	case state.branch == state.targetBranch && state.head == state.targetCommit:
		result.Status = StatusUpToDate
		result.Reason = fmt.Sprintf("%s already matches cached origin/%s", state.targetBranch, state.targetBranch)
	default:
		result.Status = StatusWouldUpdate
		result.Reason = fmt.Sprintf("would switch to and fast-forward %s using cached origin/%s", state.targetBranch, state.targetBranch)
	}

	return result
}

func comparisonDiverged(comparison branchComparison) bool {
	return comparison.compared && comparison.ahead > 0 && comparison.behind > 0
}

func comparisonAhead(comparison branchComparison) bool {
	return comparison.compared && comparison.ahead > 0 && comparison.behind == 0
}

func setDivergedResult(result *RepositoryResult, comparison branchComparison) {
	result.Status = StatusDiverged
	result.Reason = fmt.Sprintf("branch %s has diverged from %s", comparison.branch, comparison.remote)
	result.Remediation = "Reconcile the local and remote histories before updating."
}

func setAheadResult(result *RepositoryResult, comparison branchComparison) {
	result.Status = StatusAhead
	result.Reason = fmt.Sprintf("branch %s is ahead of %s", comparison.branch, comparison.remote)
	result.Remediation = "Push or otherwise reconcile local commits before updating."
}
