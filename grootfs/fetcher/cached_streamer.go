package fetcher

import (
	"fmt"
	"io"

	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/lager"
)

type CachedStreamer struct {
	cacheDriver CacheDriver
	streamer    image_puller.Streamer
}

type StreamBlob func(logger lager.Logger) (io.ReadCloser, error)

//go:generate counterfeiter . CacheDriver
type CacheDriver interface {
	Blob(logger lager.Logger, id string, streamBlob StreamBlob) (io.ReadCloser, error)
}

func NewCachedStreamer(cacheDriver CacheDriver, streamer image_puller.Streamer) *CachedStreamer {
	return &CachedStreamer{
		cacheDriver: cacheDriver,
		streamer:    streamer,
	}
}

func (s *CachedStreamer) Stream(logger lager.Logger, digest string) (io.ReadCloser, int64, error) {
	logger = logger.Session("cached-streaming", lager.Data{"digest": digest})
	logger.Info("start")
	defer logger.Info("end")

	content, err := s.cacheDriver.Blob(logger, digest, func(logger lager.Logger) (io.ReadCloser, error) {
		content, _, err := s.streamer.Stream(logger, digest)
		if err != nil {
			return nil, fmt.Errorf("reading internal streamer: %s", err)
		}

		return content, nil
	})

	if err != nil {
		return nil, 0, fmt.Errorf("fetching blob from cache driver: %s", err)
	}

	return content, 0, nil
}
