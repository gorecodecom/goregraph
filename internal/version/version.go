package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is set by release builds through ldflags.
	Version = "0.4.0"
	// Commit is set by release builds through ldflags.
	Commit = "dev"
	// Built is set by release builds through ldflags.
	Built = "unknown"
)

// Info returns human-readable build metadata for the current binary.
func Info(schemaVersion int) string {
	return fmt.Sprintf(
		"goregraph %s\ncommit: %s\nbuilt: %s\ngo: %s\nplatform: %s/%s\nschema: %d\n",
		Version,
		Commit,
		Built,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		schemaVersion,
	)
}
