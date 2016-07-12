package logging

import "code.cloudfoundry.org/lager"

type writer struct {
	lager.Logger
}

func Writer(log lager.Logger) *writer {
	return &writer{
		log,
	}
}

func (w *writer) Write(p []byte) (n int, err error) {
	w.Logger.Info("received-data", lager.Data{"data": string(p)})
	return len(p), nil
}
