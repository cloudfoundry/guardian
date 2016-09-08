package cache_driver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
)

type CacheDriver struct {
	storePath string
}

func NewCacheDriver(storePath string) *CacheDriver {
	return &CacheDriver{
		storePath: storePath,
	}
}

func (c *CacheDriver) Blob(logger lager.Logger, id string,
	streamBlob fetcher.StreamBlob,
) (io.ReadCloser, error) {
	logger = logger.Session("streaming-blob-from-cache", lager.Data{"blobID": id})
	logger.Info("start")
	defer logger.Info("end")

	hasBlob, err := c.hasBlob(id)
	if err != nil {
		return nil, fmt.Errorf("checking if the blob exists: %s", err)
	}

	var (
		blobFile *os.File
		reader   *os.File
		stream   io.ReadCloser
	)

	defer func() {
		if err != nil {
			logger.Debug("cleaning-up-corrupted")
			if err = os.RemoveAll(c.blobPath(id)); err != nil {
				logger.Error("failed cleaning up corrupted state: %s", err)
			}
		}
	}()

	if hasBlob {
		logger.Debug("cache-hit")
		reader, err = os.Open(c.blobPath(id))
		if err != nil {
			return nil, fmt.Errorf("accessing the cached blob: %s", err)
		}
		return reader, nil
	}

	logger.Debug("cache-miss")
	blobFile, err = os.OpenFile(c.blobPath(id), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("creating cached blob file: %s", err)
	}

	stream, err = streamBlob(logger)
	if err != nil {
		return nil, err
	}

	var rEnd, wEnd *os.File
	rEnd, wEnd, err = os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("creating pipe: %s", err)
	}
	go func() {
		defer wEnd.Close()
		defer blobFile.Close()
		defer stream.Close()

		_, err := io.Copy(io.MultiWriter(wEnd, blobFile), stream)
		if err != nil {
			logger.Error("failed-copying-blob-to-cache", err)

			if err = os.RemoveAll(blobFile.Name()); err != nil {
				logger.Error("failed cleaning up corrupted state: %s", err)
			}
		}
	}()

	return rEnd, nil
}

func (c *CacheDriver) blobPath(id string) string {
	id = strings.Replace(id, ":", "-", 1)
	return filepath.Join(c.storePath, store.CACHE_DIR_NAME, "blobs", id)
}

func (c *CacheDriver) hasBlob(id string) (bool, error) {
	blobPath := c.blobPath(id)

	fi, err := os.Stat(blobPath)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if !fi.Mode().IsRegular() {
		return false, fmt.Errorf("blob `%s` exists but it's not a regular file", blobPath)
	}

	return true, nil
}
