package agent

import (
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/watch"
)

// ContextWatcher describes observed operation, independently of index freshness.
type ContextWatcher struct {
	State           string     `json:"state"`
	Coverage        string     `json:"coverage"`
	Root            string     `json:"root,omitempty"`
	LastCheck       *time.Time `json:"last_file_check,omitempty"`
	LastIndexUpdate *time.Time `json:"last_index_update,omitempty"`
	Phase           string     `json:"phase,omitempty"`
	Completed       int        `json:"completed,omitempty"`
	Total           int        `json:"total,omitempty"`
	Error           string     `json:"error,omitempty"`
}

func inspectContextWatcher(directory string) ContextWatcher {
	result := ContextWatcher{State: "unknown", Coverage: "unverified"}
	root, err := watch.Resolve(directory, false)
	if err != nil {
		result.Error = "root_unavailable"
		return result
	}
	status, err := watch.GetStatus(root)
	if err != nil {
		result.Error = "watcher_state_unreadable"
		return result
	}
	result.Coverage = "direct"
	if !status.Running {
		coveringRoot, coveringStatus, covered, err := watch.FindResponsibleWorkspace(root)
		if err != nil {
			result.Error = "workspace_coverage_unverified"
			result.Coverage = "unverified"
			return result
		}
		result.Coverage = "none"
		if covered {
			root, status = coveringRoot, coveringStatus
			result.Coverage = "workspace_project"
		}
	}
	if result.Coverage != "none" {
		result.Root = root.Path
	}
	result.State = "not_running"
	if status.Running {
		result.State = "monitoring"
		if !status.UpdateStarted.IsZero() {
			result.State = "updating"
		}
	} else if status.Supervised {
		result.State = "recovering"
	}
	if !status.LastCheck.IsZero() {
		value := status.LastCheck
		result.LastCheck = &value
	}
	if !status.LastSuccess.IsZero() {
		value := status.LastSuccess
		result.LastIndexUpdate = &value
	}
	if status.Progress != nil {
		result.Phase = status.Progress.Phase
		result.Completed, result.Total = status.Progress.Completed, status.Progress.Total
	}
	if status.LastError != "" {
		result.Error = "watcher_reported_error"
	}
	return result
}

func attachContextWatcher(pack ContextPack, watcher ContextWatcher) (ContextPack, error) {
	variants := []ContextWatcher{
		watcher,
		{State: watcher.State, Coverage: watcher.Coverage, Root: watcher.Root, Error: watcher.Error},
		{State: watcher.State, Coverage: watcher.Coverage, Error: watcher.Error},
	}
	for _, variant := range variants {
		candidate, accepted, err := tryContextPack(pack, pack.BudgetTokens, func(candidate *ContextPack) bool {
			candidate.Watcher = &variant
			return true
		})
		if err != nil || accepted {
			return candidate, err
		}
	}
	// Existing source evidence takes priority when the caller exhausts a small budget.
	return pack, nil
}

func attachContextCallDiagnostics(pack ContextPack, loaded loadedContextIndex) (ContextPack, error) {
	for _, diagnostic := range loaded.Index.CallDiagnostics {
		if len(pack.CallDiagnostics) == 3 {
			break
		}
		if diagnostic.Reason == "external_target" || !contextDiagnosticHasCurrentSource(pack, loaded, diagnostic) {
			continue
		}
		candidate, accepted, err := tryContextPack(pack, pack.BudgetTokens, func(candidate *ContextPack) bool {
			candidate.CallDiagnostics = append(append([]scan.GoCallDiagnosticRecord(nil), candidate.CallDiagnostics...), diagnostic)
			return true
		})
		if err != nil {
			return ContextPack{}, err
		}
		if accepted {
			pack = candidate
		}
	}
	return pack, nil
}

func contextDiagnosticHasCurrentSource(pack ContextPack, loaded loadedContextIndex, diagnostic scan.GoCallDiagnosticRecord) bool {
	candidate := sourceCandidate{Project: diagnostic.Project, Path: diagnostic.File}
	expected := adaptiveIndexedSourceHash(loaded, candidate)
	if expected == "" {
		return false
	}
	for _, section := range pack.SourceSections {
		if section.Project == diagnostic.Project && section.Path == diagnostic.File &&
			section.SourceState == "indexed_range_current" && section.ReadReceipt != "" &&
			diagnostic.Line >= section.StartLine && diagnostic.Line <= section.EndLine {
			resolved, err := resolveSourcePath(loaded, candidate)
			if err != nil {
				return false
			}
			expectedFingerprint := sourceReadFingerprint(sourceFile{Path: resolved, Hash: expected})
			fingerprint, _, err := parseSourceReadReceipt(section.ReadReceipt)
			return err == nil && fingerprint == expectedFingerprint
		}
	}
	return false
}
