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

func hideConsole(command *exec.Cmd) {
	const createNoWindow = 0x08000000
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= createNoWindow
}
