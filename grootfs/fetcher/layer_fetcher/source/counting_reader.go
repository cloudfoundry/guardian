package source // import "code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source"

import "io"

type CountingReader struct {
	bytesRead int64
	delegate  io.Reader
}

func NewCountingReader(delegate io.Reader) *CountingReader {
	return &CountingReader{
		delegate: delegate,
	}
}

func (r *CountingReader) Read(p []byte) (int, error) {
	read, err := r.delegate.Read(p)
	r.bytesRead += int64(read)
	return read, err
}

func (r *CountingReader) GetBytesRead() int64 {
	return r.bytesRead
}
