package scan

import (
	"fmt"
	"strings"
)

func RenderCoverageReport(record CoverageRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Capability Coverage\n\n")
	b.WriteString("Coverage describes what GoreGraph analyzed, not whether the project contains a feature.\n\n")
	b.WriteString("- `COMPLETE`: the active adapter emits this capability for its supported patterns.\n")
	b.WriteString("- `PARTIAL`: useful facts are emitted, but known patterns remain unsupported.\n")
	b.WriteString("- `UNAVAILABLE`: no active adapter emits this capability.\n")
	b.WriteString("- `FAILED`: the capability was expected but analysis failed.\n\n")
	for _, capability := range record.Capabilities {
		b.WriteString(fmt.Sprintf("- `%s` / `%s`: **%s** — %s\n", capability.Language, capability.ID, capability.Coverage, capability.Reason))
	}
	return b.String()
}
