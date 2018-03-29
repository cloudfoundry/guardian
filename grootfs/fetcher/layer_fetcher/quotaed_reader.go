package layer_fetcher

import (
	"errors"
	"io"
)

type QuotaedReader struct {
	DelegateReader            io.ReadCloser
	QuotaLeft                 int64
	QuotaExceededErrorHandler func() error
}

func NewQuotaedReader(delegateReader io.ReadCloser, quotaLeft int64, errorMsg string) *QuotaedReader {
	return &QuotaedReader{
		DelegateReader: delegateReader,
		QuotaLeft:      quotaLeft,
		QuotaExceededErrorHandler: func() error {
			return errors.New(errorMsg)
		},
	}
}

func (q *QuotaedReader) Read(p []byte) (int, error) {
	if int64(len(p)) > q.QuotaLeft {
		p = p[0 : q.QuotaLeft+1]
	}

	n, err := q.DelegateReader.Read(p)
	q.QuotaLeft = q.QuotaLeft - int64(n)

	if q.QuotaLeft < 0 {
		return n, q.QuotaExceededErrorHandler()
	}

	return n, err
}

func (q *QuotaedReader) Close() error {
	return q.DelegateReader.Close()
}
