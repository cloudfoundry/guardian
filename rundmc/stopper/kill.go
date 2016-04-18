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

		process.Signal(signal)
	}
}
