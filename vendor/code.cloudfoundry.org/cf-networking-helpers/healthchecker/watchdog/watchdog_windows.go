package watchdog

import (
	"os"

	"golang.org/x/sys/windows"
)

var HandledSignals = []os.Signal{
	windows.SIGTERM,
	windows.SIGINT,
}

func (w *Watchdog) shouldExitOnSignal(sig os.Signal) bool {
	if sig == windows.SIGTERM {
		w.logger.Info("Received TERM signal, exiting")
		return true
	} else if sig == windows.SIGINT {
		w.logger.Info("Received INT signal, exiting")
		return true
	}
	return false
}
