package gitupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")

	stdout, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(stdout)), nil
	}

	stderr := ""
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		stderr = strings.TrimSpace(string(exitError.Stderr))
	}
	if stderr != "" {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr)
	}
	return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}
