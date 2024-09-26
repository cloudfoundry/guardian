package stopper

import (
	"os"
	"syscall"
)

type DefaultKiller struct{}

func (DefaultKiller) Kill(signal syscall.Signal, pids ...int) {
	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			panic(err) // can't happen on unix systems
		}

		// #nosec G104 - no logger here, and don't want to panic on errors when sending kill signals
		process.Signal(signal)
	}
}
