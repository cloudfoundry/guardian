package dadoo

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
		writer.Write(p)
	}

	return len(p), nil
}

func (w *DynamicMultiWriter) Attach(writer io.Writer) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.writers = append(w.writers, writer)
}

func (w *DynamicMultiWriter) Count() int {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return len(w.writers)
}
