package gitupdate

type Mode string

type Status string

const (
	ModePreview Mode = "preview"
	ModeExecute Mode = "execute"

	StatusUpToDate          Status = "up_to_date"
	StatusWouldUpdate       Status = "would_update"
	StatusUpdated           Status = "updated"
	StatusDirty             Status = "dirty"
	StatusAhead             Status = "ahead"
	StatusDiverged          Status = "diverged"
	StatusNotGit            Status = "not_git"
	StatusMissingRemote     Status = "missing_remote"
	StatusBlockedWorktree   Status = "blocked_worktree"
	StatusDetachedHead      Status = "detached_head"
	StatusOperationProgress Status = "operation_in_progress"
	StatusDefaultUnknown    Status = "default_branch_unknown"
	StatusFetchFailed       Status = "fetch_failed"
)

type Target struct {
	Path string
}

type Options struct {
	Execute       bool
	WorkspaceRoot string
}

type RepositoryResult struct {
	Path         string `json:"path"`
	GitRoot      string `json:"git_root,omitempty"`
	Remote       string `json:"remote,omitempty"`
	BranchBefore string `json:"branch_before,omitempty"`
	BranchAfter  string `json:"branch_after,omitempty"`
	CommitBefore string `json:"commit_before,omitempty"`
	CommitAfter  string `json:"commit_after,omitempty"`
	Status       Status `json:"status"`
	Reason       string `json:"reason"`
	Remediation  string `json:"remediation,omitempty"`
	Executed     bool   `json:"executed"`
}

type Report struct {
	Mode          Mode               `json:"mode"`
	WorkspaceRoot string             `json:"workspace_root,omitempty"`
	Repositories  []RepositoryResult `json:"repositories"`
	Summary       map[Status]int     `json:"summary"`
}

func (r Report) ExitCode() int {
	for _, repository := range r.Repositories {
		switch repository.Status {
		case StatusUpToDate, StatusWouldUpdate, StatusUpdated:
		default:
			return 1
		}
	}

	return 0
}
