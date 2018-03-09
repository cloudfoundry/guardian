package processwaiter

import (
	"time"

	ps "github.com/mitchellh/go-ps"
)

//go:generate counterfeiter . ProcessWaiter

type ProcessWaiter func(pid int) error

func (w ProcessWaiter) Wait(pid int) error {
	return w(pid)
}

func WaitOnProcess(pid int) error {
	for {
		process, err := ps.FindProcess(pid)
		if err != nil {
			return err
		}
		if process == nil {
			return nil
		}
		time.Sleep(time.Millisecond * 200)
	}
}
