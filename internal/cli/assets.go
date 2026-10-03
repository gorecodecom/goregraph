package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/gorecodecom/goregraph/internal/assetexport"
)

// runAssets only writes an explicitly requested exporter; it never executes an editor.
func runAssets(args []string, stdout, stderr io.Writer) int {
	const help = "Usage: goregraph assets exporter blender|unity --output <new-file>\n\nWrites an opt-in exporter template. Never starts Unity/Blender, imports assets, or changes the index.\n"
	if len(args) == 0 || len(args) == 1 && isHelp(args[0]) {
		fmt.Fprint(stdout, help)
		return 0
	}
	if len(args) != 4 || args[0] != "exporter" || args[2] != "--output" {
		fmt.Fprint(stderr, help)
		return 2
	}
	body, err := assetexport.Script(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	output, err := os.OpenFile(args[3], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	_, writeErr := output.Write(body)
	closeErr := output.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(args[3])
		fmt.Fprintln(stderr, "error: exporter file could not be written")
		return 1
	}
	fmt.Fprintln(stdout, "Exporter written:", args[3])
	return 0
}
