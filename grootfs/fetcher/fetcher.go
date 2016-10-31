package fetcher // import "code.cloudfoundry.org/grootfs/fetcher"

import (
	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/lager"
)

type RemoteBlobFunc func(logger lager.Logger) ([]byte, int64, error)

//go:generate counterfeiter . CacheDriver
type CacheDriver interface {
	StreamBlob(logger lager.Logger, id string, remoteBlobFunc RemoteBlobFunc) (image_puller.Stream, error)
}
