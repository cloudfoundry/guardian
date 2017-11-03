package layer_fetcher // import "code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"

import (
	"io"
	"os"

	errorspkg "github.com/pkg/errors"
)

type BlobReader struct {
	reader   io.Reader
	filePath string
}

func NewBlobReader(blobPath string) (*BlobReader, error) {
	reader, err := os.Open(blobPath)
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to open blob")
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
