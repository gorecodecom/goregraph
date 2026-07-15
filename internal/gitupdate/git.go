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
	return runGitWithEnv(ctx, dir, nil, args...)
}

func runGitWithEnv(ctx context.Context, dir string, environment []string, args ...string) (string, error) {
	gitArgs := make([]string, 0, len(args)+2)
	gitArgs = append(gitArgs, "-c", "core.fsmonitor=false")
	gitArgs = append(gitArgs, args...)
	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1")
	cmd.Env = append(cmd.Env, environment...)

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
