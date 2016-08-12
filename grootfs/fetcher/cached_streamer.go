package fetcher

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/streamer"
	"code.cloudfoundry.org/lager"
)

type CachedStreamer struct {
	cachePath string
	streamer  cloner.Streamer
}

func NewCachedStreamer(cachePath string, streamer cloner.Streamer) *CachedStreamer {
	return &CachedStreamer{
		cachePath: cachePath,
		streamer:  streamer,
	}
}

func (s *CachedStreamer) Stream(logger lager.Logger, digest string) (io.ReadCloser, int64, error) {
	logger = logger.Session("cached-streaming", lager.Data{"digest": digest})
	logger.Debug("start")
	defer logger.Debug("end")

	logger.Debug("lookup-cache")
	if !s.cachedLookup(digest) {
		logger.Debug("cache-not-found")

		content, _, err := s.streamer.Stream(logger, digest)
		if err != nil {
			return nil, 0, fmt.Errorf("reading internal streamer: %s", err)
		}

		if err := s.cache(logger, digest, content); err != nil {
			return nil, 0, fmt.Errorf("creating cache: %s", err)
		}
	}

	logger.Debug("stream-local-cache")
	reader, err := s.cachedReader(logger, digest)
	return reader, 0, err
}

func (s *CachedStreamer) cache(logger lager.Logger, digest string, reader io.ReadCloser) error {
	logger = logger.Session("creating-cache", lager.Data{"digest": digest})
	logger.Debug("start")
	defer logger.Debug("end")

	writer, err := os.Create(s.cachedBlobPath(digest))
	if err != nil {
		return err
	}

	defer writer.Close()
	bufferedReader := bufio.NewReader(reader)
	_, err = bufferedReader.WriteTo(writer)

	return err
}

func (s *CachedStreamer) cachedReader(logger lager.Logger, digest string) (io.ReadCloser, error) {
	reader, err := os.Open(s.cachedBlobPath(digest))
	return streamer.NewCallbackReader(logger, reader.Close, reader), err
}

func (s *CachedStreamer) cachedLookup(digest string) bool {
	_, err := os.Stat(s.cachedBlobPath(digest))
	return !os.IsNotExist(err)
}

func (s *CachedStreamer) cachedBlobPath(digest string) string {
	parsedDigest := strings.Replace(digest, ":", "-", -1)
	return filepath.Join(s.cachePath, parsedDigest)
}
