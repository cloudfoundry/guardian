package lagregator

import "code.cloudfoundry.org/lager"

type Relogger struct {
	dest lager.Logger
}

func NewRelogger(destination lager.Logger) *Relogger {
	return &Relogger{
		dest: destination,
	}
}

func (r *Relogger) Write(data []byte) (n int, err error) {
	RelogBytes(r.dest, data)
	return len(data), nil
}
