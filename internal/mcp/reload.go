package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/gorecodecom/goregraph/internal/agentguide"
	"github.com/gorecodecom/goregraph/internal/watch"
)

// ReloadProtocol identifies executables that preserve an existing stdio session.
const ReloadProtocol = "goregraph-mcp-reload-v1"

var errReconnectRequired = errors.New("reconnect the MCP client to load the installed executable")

type probeOutput struct{ strings.Builder }

func (output *probeOutput) Write(body []byte) (int, error) {
	if output.Len()+len(body) > 4096 {
		return 0, fmt.Errorf("MCP replacement probe output exceeds limit")
	}
	return output.Builder.Write(body)
}

func validateMCPReplacement(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, "mcp", "--reload-protocol")
	var output probeOutput
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("MCP replacement probe: %w", err)
	}
	if strings.TrimSpace(output.String()) != ReloadProtocol {
		return fmt.Errorf("replacement does not support MCP session handoff")
	}
	return nil
}

// ServeInstalled serves stdio while checking for updates at request boundaries
// and while idle. No index, watcher configuration or network update is triggered.
func ServeInstalled(output, diagnostics io.Writer, options Options) error {
	if options.ProtocolVersion == "" {
		options.ProtocolVersion = agentguide.AdaptiveV2
	}
	if _, err := agentguide.Instruction(options.ProtocolVersion); err != nil {
		return err
	}
	replacement, err := watch.NewReplacement()
	if err != nil {
		return err
	}
	input, closeInput, err := reloadInput()
	if err != nil {
		return err
	}
	defer closeInput()
	pending, err := resumedInput()
	if err != nil {
		return err
	}
	lastError := ""
	checkpoint := func(pending []byte) error {
		changed, err := replacement.Changed(context.Background(), validateMCPReplacement)
		if err == nil && changed {
			err = replaceMCP(replacement.Path, pending)
		}
		if errors.Is(err, errReconnectRequired) {
			return err
		}
		if err != nil {
			// Keep the working executable if installation is incomplete or invalid.
			if err.Error() != lastError {
				fmt.Fprintf(diagnostics, "goregraph: MCP update pending: %v\n", err)
			}
			lastError = err.Error()
		} else {
			lastError = ""
		}
		return nil
	}
	return serveStream(input, output, options, pending, checkpoint)
}
