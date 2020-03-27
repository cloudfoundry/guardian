package processwaiter

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func isProcessAlive(pid int) bool {
	err := unix.Kill(pid, syscall.Signal(0))
	if err == nil || err == unix.EPERM {
		return true
	}

	return false
}
