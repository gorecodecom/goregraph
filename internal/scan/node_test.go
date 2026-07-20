package scan

import (
	"os/exec"
	"strings"
)

func nodeScriptCommand(node, source string) *exec.Cmd {
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(source)
	return cmd
}
