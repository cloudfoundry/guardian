package layer_fetcher // import "code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"

import (
	"compress/gzip"
	"io"
	"os"
	"strings"

	errorspkg "github.com/pkg/errors"
)

type BlobReader struct {
	reader   io.Reader
	filePath string
}

func NewBlobReader(blobPath, mediaType string) (*BlobReader, error) {
	zippedReader, err := os.Open(blobPath)
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to open blob")
	}

	if mediaType != "" && !strings.Contains(mediaType, "gzip") {
		return &BlobReader{
			filePath: blobPath,
			reader:   zippedReader,
		}, nil
	}

	reader, err := gzip.NewReader(zippedReader)
	if err != nil {
		return nil, errorspkg.Wrap(err, "blob file is not gzipped")
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
