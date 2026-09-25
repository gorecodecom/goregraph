//go:build windows

package watch

import (
	"os/exec"
	"syscall"
)

func detach(command *exec.Cmd) {
	const detachedProcess = 0x00000008
	const createNewProcessGroup = 0x00000200
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
}
