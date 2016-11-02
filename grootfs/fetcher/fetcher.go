package fetcher // import "code.cloudfoundry.org/grootfs/fetcher"

import "code.cloudfoundry.org/lager"

type RemoteBlobFunc func(logger lager.Logger) ([]byte, int64, error)

//go:generate counterfeiter . CacheDriver
type CacheDriver interface {
	FetchBlob(logger lager.Logger, id string, remoteBlobFunc RemoteBlobFunc) ([]byte, int64, error)
}
