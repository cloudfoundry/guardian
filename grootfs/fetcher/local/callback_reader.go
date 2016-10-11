package local // import "code.cloudfoundry.org/grootfs/fetcher/local"

import (
	"io"

	"code.cloudfoundry.org/lager"
)

type CallbackReader struct {
	logger         lager.Logger
	callback       func() error
	internalReader io.ReadCloser
}

func NewCallbackReader(logger lager.Logger, callback func() error, reader io.ReadCloser) *CallbackReader {
	return &CallbackReader{
		logger:         logger.Session("callback-reader"),
		callback:       callback,
		internalReader: reader,
	}
}

func (r *CallbackReader) Read(buf []byte) (int, error) {
	return r.internalReader.Read(buf)
}

func (r *CallbackReader) Close() error {
	if err := r.callback(); err != nil {
		// won't return the error because this is not a failure to close
		r.logger.Error("callback-failed", err)
	}

	return r.internalReader.Close()
}
