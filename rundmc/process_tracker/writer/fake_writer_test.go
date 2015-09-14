package writer_test

import "sync"

type fakeWriter struct {
	mu             sync.Mutex
	nWriteReturn   int
	errWriteReturn error
	bytesWritten   []byte
	writeCallCount int

	errCloseReturn error
	closeCallCount int
}

func (fw *fakeWriter) Write(p []byte) (int, error) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.writeCallCount++
	if fw.errWriteReturn != nil {
		return 0, fw.errWriteReturn
	}
	fw.bytesWritten = p
	return fw.nWriteReturn, nil
}

func (fw *fakeWriter) Close() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.closeCallCount++
	if fw.errCloseReturn != nil {
		return fw.errCloseReturn
	}
	return nil
}

func (fw *fakeWriter) writeArgument() []byte {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	return fw.bytesWritten
}

func (fw *fakeWriter) writeCalls() int {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	return fw.writeCallCount
}

func (fw *fakeWriter) closeCalls() int {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	return fw.closeCallCount
}
