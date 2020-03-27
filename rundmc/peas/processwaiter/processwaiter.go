package processwaiter

import (
	"time"
)

//go:generate counterfeiter . ProcessWaiter

type ProcessWaiter func(pid int) error

func (w ProcessWaiter) Wait(pid int) error {
	return w(pid)
}

func WaitOnProcess(pid int) error {
	for {
		if !isProcessAlive(pid) {
			return nil
		}

		time.Sleep(time.Millisecond * 200)
	}
}
