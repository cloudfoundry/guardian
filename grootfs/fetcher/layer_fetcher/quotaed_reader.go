package layer_fetcher

import (
	"io"
)

type QuotaedReader struct {
	DelegateReader            io.Reader
	QuotaLeft                 int64
	QuotaExceededErrorHandler func() error
}

func (q *QuotaedReader) Read(p []byte) (int, error) {
	if q.QuotaLeft < 0 {
		return q.DelegateReader.Read(p)
	}

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
