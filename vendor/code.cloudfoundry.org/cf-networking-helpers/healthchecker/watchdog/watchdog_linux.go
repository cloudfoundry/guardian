package watchdog

import (
	"os"
	"syscall"
)

var HandledSignals = []os.Signal{
	syscall.SIGUSR1,
}

func (w *Watchdog) shouldExitOnSignal(sig os.Signal) bool {
	if sig == syscall.SIGUSR1 {
		w.logger.Info("Received USR1 signal, exiting")
		return true
	}
	return false
}
