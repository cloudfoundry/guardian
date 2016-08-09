package streamer

import (
	"io"

	"code.cloudfoundry.org/lager"
)

type CommandReader struct {
	logger         lager.Logger
	cmdWait        func() error
	internalReader io.ReadCloser
}

func NewCommandReader(logger lager.Logger, cmdWait func() error, reader io.ReadCloser) *CommandReader {
	return &CommandReader{
		logger:         logger,
		cmdWait:        cmdWait,
		internalReader: reader,
	}
}

func (r *CommandReader) Read(buf []byte) (int, error) {
	return r.internalReader.Read(buf)
}

func (r *CommandReader) Close() error {
	if err := r.cmdWait(); err != nil {
		// won't return the error because this is not a failure to close
		r.logger.Error("command-failed", err)
	}

	return r.internalReader.Close()
}
