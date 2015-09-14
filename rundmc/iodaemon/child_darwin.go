package iodaemon

import (
	"os/exec"
	"syscall"
)

func child(executablePath string, argv []string) *exec.Cmd {
	return &exec.Cmd{
		Path:        executablePath,
		Args:        argv,
		SysProcAttr: &syscall.SysProcAttr{},
	}
}
