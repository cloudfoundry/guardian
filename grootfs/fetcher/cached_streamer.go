package fetcher

import (
	"fmt"
	"io"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/lager"
)

type CachedStreamer struct {
	cacheDriver CacheDriver
	streamer    cloner.Streamer
}

func NewCachedStreamer(cacheDriver CacheDriver, streamer cloner.Streamer) *CachedStreamer {
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
		return nil, 0, fmt.Errorf("fetching blob from cache driver: ", err)
	}

	return content, 0, nil
}
