package cli

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/gorecodecom/goregraph/internal/watch"
)

func printWatchActivity(stdout io.Writer, status watch.Status) {
	switch {
	case !status.Running && status.Supervised:
		fmt.Fprintln(stdout, "Activity: waiting for supervised watcher")
	case !status.Running:
		fmt.Fprintln(stdout, "Activity: stopped")
	case !status.UpdateStarted.IsZero():
		fmt.Fprintln(stdout, "Activity: updating index")
		fmt.Fprintf(stdout, "Update started: %s\n", status.UpdateStarted.Format("2006-01-02 15:04:05 MST"))
		if progress := status.Progress; progress != nil {
			fmt.Fprintf(stdout, "Update phase: %s\n", progress.Phase)
			if !status.Progress.PhaseStarted.IsZero() {
				fmt.Fprintf(stdout, "Phase elapsed: %s\n", time.Since(status.Progress.PhaseStarted).Round(time.Second))
			}
			if progress.Project != "" {
				fmt.Fprintf(stdout, "Current project: %s\n", progress.Project)
			}
			if progress.File != "" {
				fmt.Fprintf(stdout, "Current file: %s\n", progress.File)
			}
			if progress.Total > 0 {
				fmt.Fprintf(stdout, "Phase progress: %d/%d\n", progress.Completed, progress.Total)
			}
			if progress.ProjectsTotal > 0 {
				fmt.Fprintf(stdout, "Projects prepared: %d/%d\n", progress.ProjectsCompleted, progress.ProjectsTotal)
			}
			fmt.Fprintf(stdout, "Last observed progress: %s\n", progress.LastProgress.Format("2006-01-02 15:04:05 MST"))
		}
	case status.LastCheck.IsZero():
		fmt.Fprintln(stdout, "Activity: awaiting first reported file check")
	default:
		fmt.Fprintln(stdout, "Activity: monitoring files")
	}
	printWatchUpdateSummary(stdout, status.LastUpdate)
	if !status.LastCheck.IsZero() {
		fmt.Fprintf(stdout, "Last successful file check: %s\n", status.LastCheck.Format("2006-01-02 15:04:05 MST"))
	}
	if !status.LastSuccess.IsZero() {
		fmt.Fprintf(stdout, "Last successful index update: %s\n", status.LastSuccess.Format("2006-01-02 15:04:05 MST"))
	}
}

// coveringWorkspaceWatcher reads existing state and registry metadata only.
func coveringWorkspaceWatcher(requested watch.Root) (watch.Root, watch.Status, bool, error) {
	return watch.FindCoveringWorkspace(requested)
}

// printWatchUpdateSummary keeps completed attempts distinct from live progress.
func printWatchUpdateSummary(writer io.Writer, summary *watch.UpdateSummary) {
	if summary == nil {
		return
	}
	outcome := "failed"
	if summary.Succeeded {
		outcome = "succeeded"
	}
	fmt.Fprintf(writer, "Last update attempt: %s (%s)\n", outcome, summary.Duration.Round(time.Millisecond))
	phases := make([]string, 0, len(summary.PhaseDurations))
	for phase := range summary.PhaseDurations {
		phases = append(phases, phase)
	}
	sort.Slice(phases, func(i, j int) bool {
		left, right := summary.PhaseDurations[phases[i]], summary.PhaseDurations[phases[j]]
		if left != right {
			return left > right
		}
		return phases[i] < phases[j]
	})
	for i, phase := range phases {
		if i == 3 {
			break
		}
		fmt.Fprintf(writer, "Observed phase time: %s %s\n", phase, summary.PhaseDurations[phase].Round(time.Millisecond))
	}
}
