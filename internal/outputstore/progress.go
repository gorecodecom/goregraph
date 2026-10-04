package outputstore

import "time"

// Event reports real preparation and publication operations, never heartbeats.
type Event struct {
	Phase     string
	Root      string
	File      string
	Outcome   string
	Completed int
	Total     int
	Elapsed   time.Duration
}

func observe(request UpdateRequest, phase, outcome string, completed, total int, started time.Time) {
	if request.Observe != nil {
		request.Observe(Event{Phase: phase, Root: request.Root, Outcome: outcome, Completed: completed, Total: total, Elapsed: time.Since(started)})
	}
}
