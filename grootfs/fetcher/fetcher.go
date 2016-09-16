package fetcher

import (
	"io"

	"code.cloudfoundry.org/lager"
)

type StreamBlob func(logger lager.Logger) (io.ReadCloser, int64, error)

//go:generate counterfeiter . CacheDriver
type CacheDriver interface {
	Blob(logger lager.Logger, id string, streamBlob StreamBlob) (io.ReadCloser, int64, error)
}
