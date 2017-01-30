package remote

import (
	"compress/gzip"
	"io"
	"os"

	"github.com/pkg/errors"
)

type BlobReader struct {
	reader   io.Reader
	filePath string
}

func NewBlobReader(blobPath string) (*BlobReader, error) {
	zippedReader, err := os.Open(blobPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open blob")
	}

	reader, err := gzip.NewReader(zippedReader)
	if err != nil {
		return nil, errors.Wrap(err, "blob file is not gzipped")
	}

	return &BlobReader{
		filePath: blobPath,
		reader:   reader,
	}, nil
}

func (d *BlobReader) Read(p []byte) (int, error) {
	return d.reader.Read(p)
}

func (d *BlobReader) Close() error {
	return os.Remove(d.filePath)
}
