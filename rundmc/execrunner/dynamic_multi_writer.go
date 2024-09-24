package execrunner

import (
	"io"
	"sync"
)

type DynamicMultiWriter struct {
	mutex   *sync.RWMutex
	writers []io.Writer
}

func NewDynamicMultiWriter() *DynamicMultiWriter {
	return &DynamicMultiWriter{mutex: &sync.RWMutex{}}
}

func (w *DynamicMultiWriter) Write(p []byte) (int, error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	for _, writer := range w.writers {
		// #nosec G104 - for some reason we don't want to return errors from our Write() interface here, ever. Try all writers, and if any fail, ignore it. If all fail, oh well?
		writer.Write(p)
	}

	return len(p), nil
}

func (w *DynamicMultiWriter) Attach(writer io.Writer) int {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.writers = append(w.writers, writer)
	return len(w.writers)
}

func (w *DynamicMultiWriter) Count() int {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return len(w.writers)
}
