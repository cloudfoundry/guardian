package writer

import (
	"io"
	"sync"
)

type FanOut interface {
	Write(data []byte) (int, error)
	AddSink(sink io.Writer)
}

func NewFanOut() FanOut {
	return &fanOut{}
}

type fanOut struct {
	sinks  []io.Writer
	sinksL sync.Mutex
}

func (w *fanOut) Write(data []byte) (int, error) {
	w.sinksL.Lock()
	defer w.sinksL.Unlock()

	// the sinks should be nonblocking and never actually error;
	// we can assume lossiness here, and do this all within the lock
	for _, s := range w.sinks {
		s.Write(data)
	}

	return len(data), nil
}

func (w *fanOut) AddSink(sink io.Writer) {
	w.sinksL.Lock()
	defer w.sinksL.Unlock()

	w.sinks = append(w.sinks, sink)
}
