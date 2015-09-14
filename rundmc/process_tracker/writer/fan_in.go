package writer

import (
	"errors"
	"io"
	"sync"
)

type FanIn interface {
	Write(data []byte) (int, error)
	Close() error
	AddSink(sink io.WriteCloser)
	AddSource(source io.Reader)
}

func NewFanIn() FanIn {
	return &fanIn{hasSink: make(chan struct{})}
}

type fanIn struct {
	w      io.WriteCloser
	closed bool
	writeL sync.Mutex

	hasSink chan struct{}
}

func (fw *fanIn) Write(data []byte) (int, error) {
	<-fw.hasSink

	fw.writeL.Lock()
	defer fw.writeL.Unlock()

	if fw.closed {
		return 0, errors.New("write after close")
	}

	return fw.w.Write(data)
}

func (fw *fanIn) Close() error {
	<-fw.hasSink

	fw.writeL.Lock()
	defer fw.writeL.Unlock()

	if fw.closed {
		return errors.New("already closed")
	}

	fw.closed = true

	return fw.w.Close()
}

//AddSink can only be called once
func (fw *fanIn) AddSink(sink io.WriteCloser) {
	fw.w = sink

	// Allow Write and Close to proceed.
	close(fw.hasSink)
}

func (fw *fanIn) AddSource(source io.Reader) {
	go func() {
		_, err := io.Copy(fw, source)
		if err == nil {
			fw.Close()
		}
	}()
}
