package gitupdate

import (
	"context"
	"fmt"
	"os"
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
	}

	var hooksDirectory string
	if options.Execute {
		var err error
		hooksDirectory, err = os.MkdirTemp("", "goregraph-git-hooks-")
		if err != nil {
			return report, fmt.Errorf("create empty Git hooks directory: %w", err)
		}
		defer os.RemoveAll(hooksDirectory)
	}

	repositories, err := collectRepositoryTargets(ctx, targets)
	if err != nil {
		return report, err
	}
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
				if infrastructureErr := gitInfrastructureError(ctx, err); infrastructureErr != nil {
					return report, fmt.Errorf("inspect %s: %w", repository.target.Path, infrastructureErr)
				}
				result = inspectionFailedResult(repository.target, repository.root, err)
			} else if options.Execute {
				result = executeUpdate(ctx, repository.target, state, hooksDirectory)
			} else {
				result = classifyPreview(repository.target, state)
			}
		}
		report.Repositories = append(report.Repositories, result)
	}

	for _, result := range report.Repositories {
		report.Summary[result.Status]++
	}

	return report, nil
}

func inspectionFailedResult(target Target, root string, err error) RepositoryResult {
	return RepositoryResult{
		Path:        target.Path,
		GitRoot:     root,
		Status:      StatusFetchFailed,
		Reason:      fmt.Sprintf("inspect repository: %v", err),
		Remediation: "Repair the repository so Git can read HEAD and repository state, then retry the update.",
	}
}

func executeUpdate(ctx context.Context, target Target, initial repositoryState, hooksDirectory string) RepositoryResult {
	preflight := classifyPreview(target, initial)
	if !canExecute(preflight.Status) {
		return preflight
	}

	preflight.Executed = true
	hookConfig := "core.hooksPath=" + hooksDirectory
	if _, err := runGitWithEnv(ctx, initial.root, []string{"GIT_TERMINAL_PROMPT=0"},
		"-c", hookConfig, "fetch", "--prune", "origin"); err != nil {
		preflight.BranchAfter = initial.branch
		preflight.CommitAfter = initial.head
		return fetchFailedResult(preflight, err)
	}

	fetched, err := inspectRepository(ctx, initial.root)
	if err != nil {
		preflight.BranchAfter = initial.branch
		preflight.CommitAfter = initial.head
		return fetchFailedResult(preflight, fmt.Errorf("inspect after fetch: %w", err))
	}
	afterFetch := classifyExecutedState(target, initial, fetched)
	if !canExecute(afterFetch.Status) {
		return afterFetch
	}
	if finding := inspectTargetTreeSafety(ctx, fetched.root, fetched.targetCommit); finding.reason != "" {
		return safetyRefusalResult(afterFetch, finding)
	}

	if fetched.targetLocalExists {
		_, err = runGit(ctx, initial.root, "-c", hookConfig, "switch", fetched.targetBranch)
	} else {
		_, err = runGit(ctx, initial.root, "-c", hookConfig, "switch", "--track", "-c", fetched.targetBranch, "origin/"+fetched.targetBranch)
	}
	if err != nil {
		return mutationFailureResult(ctx, target, initial, afterFetch, "switch", err)
	}

	if _, err = runGit(ctx, initial.root, "-c", hookConfig, "merge", "--ff-only", "origin/"+fetched.targetBranch); err != nil {
		return mutationFailureResult(ctx, target, initial, afterFetch, "fast-forward", err)
	}

	final, err := inspectRepository(ctx, initial.root)
	if err != nil {
		afterFetch.BranchAfter = fetched.targetBranch
		afterFetch.CommitAfter = fetched.targetCommit
		return fetchFailedResult(afterFetch, fmt.Errorf("inspect final repository state: %w", err))
	}
	result := RepositoryResult{
		Path:         target.Path,
		GitRoot:      final.root,
		Remote:       final.remoteURL,
		BranchBefore: initial.branch,
		BranchAfter:  final.branch,
		CommitBefore: initial.head,
		CommitAfter:  final.head,
		Executed:     true,
	}
	if initial.branch != final.branch || initial.head != final.head {
		result.Status = StatusUpdated
		result.Reason = fmt.Sprintf("switched to and fast-forwarded %s from origin/%s", final.branch, final.branch)
	} else {
		result.Status = StatusUpToDate
		result.Reason = fmt.Sprintf("%s already matches fetched origin/%s", final.branch, final.branch)
	}
	return result
}

func canExecute(status Status) bool {
	return status == StatusWouldUpdate || status == StatusUpToDate
}

func classifyExecutedState(target Target, initial, current repositoryState) RepositoryResult {
	result := classifyPreview(target, current)
	result.BranchBefore = initial.branch
	result.BranchAfter = current.branch
	result.CommitBefore = initial.head
	result.CommitAfter = current.head
	result.Executed = true
	return result
}

func fetchFailedResult(result RepositoryResult, err error) RepositoryResult {
	result.Status = StatusFetchFailed
	result.Reason = err.Error()
	result.Remediation = "Fetch origin and update the default branch manually after resolving the Git error."
	result.Executed = true
	return result
}

func safetyRefusalResult(result RepositoryResult, finding safetyFinding) RepositoryResult {
	result.Status = StatusFetchFailed
	result.Reason = finding.reason
	result.Remediation = finding.remediation
	return result
}

func mutationFailureResult(ctx context.Context, target Target, initial repositoryState, fallback RepositoryResult, operation string, operationErr error) RepositoryResult {
	current, err := inspectRepository(ctx, initial.root)
	if err == nil {
		classified := classifyExecutedState(target, initial, current)
		if !canExecute(classified.Status) {
			return classified
		}
		fallback = classified
	}
	return fetchFailedResult(fallback, fmt.Errorf("%s default branch: %w", operation, operationErr))
}

func collectRepositoryTargets(ctx context.Context, targets []Target) ([]repositoryTarget, error) {
	repositories := make([]repositoryTarget, 0, len(targets))
	seenRoots := make(map[string]int)
	seenNonRepositories := make(map[string]struct{})

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		root, err := canonicalGitRoot(ctx, target.Path)
		if err == nil {
			if index, exists := seenRoots[root]; exists {
				if target.Path < repositories[index].target.Path {
					repositories[index].target = target
				}
				continue
			}
			seenRoots[root] = len(repositories)
			repositories = append(repositories, repositoryTarget{target: target, root: root, sortKey: root})
			continue
		}
		if infrastructureErr := gitInfrastructureError(ctx, err); infrastructureErr != nil {
			return nil, fmt.Errorf("resolve Git root for %s: %w", target.Path, infrastructureErr)
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
	return repositories, nil
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
	case state.safety.reason != "":
		result.BranchAfter = state.branch
		result.CommitAfter = state.head
		return safetyRefusalResult(result, state.safety)
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
