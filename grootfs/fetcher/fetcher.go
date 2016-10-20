package fetcher // import "code.cloudfoundry.org/grootfs/fetcher"

import (
	"io"

	"code.cloudfoundry.org/lager"
)

type RemoteBlobFunc func(logger lager.Logger) ([]byte, int64, error)

//go:generate counterfeiter . CacheDriver
type CacheDriver interface {
	StreamBlob(logger lager.Logger, id string, remoteBlobFunc RemoteBlobFunc) (io.ReadCloser, int64, error)
	Clean(logger lager.Logger) error
}
