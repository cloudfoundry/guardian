package cache_driver // import "code.cloudfoundry.org/grootfs/store/cache_driver"

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

type CacheDriver struct {
	storePath string
}

func NewCacheDriver(storePath string) *CacheDriver {
	return &CacheDriver{
		storePath: storePath,
	}
}

func (c *CacheDriver) FetchBlob(logger lager.Logger, id string,
	blobFunc fetcher.RemoteBlobFunc,
) ([]byte, int64, error) {
	logger = logger.Session("getting-blob-from-cache", lager.Data{"blobID": id})
	logger.Info("starting")
	defer logger.Info("ending")

	hasBlob, err := c.hasBlob(id)
	if err != nil {
		return nil, 0, errorspkg.Wrap(err, "checking if the blob exists")
	}

	defer func() {
		if err != nil {
			c.cleanupCorrupted(logger, id)
		}
	}()

	if hasBlob {
		logger.Debug("cache-hit")

		reader, err := os.Open(c.blobPath(id))
		if err != nil {
			return nil, 0, errorspkg.Wrap(err, "accessing the cached blob")
		}
		stat, err := os.Stat(c.blobPath(id))
		if err != nil {
			return nil, 0, errorspkg.Wrap(err, "accessing cached blob stat")
		}

		blobContents, err := ioutil.ReadAll(reader)
		if err != nil {
			return nil, 0, errorspkg.Wrap(err, "reading cached blob")
		}

		return blobContents, stat.Size(), nil
	}

	logger.Debug("cache-miss")

	blobContent, size, err := blobFunc(logger)
	if err != nil {
		return nil, 0, err
	}

	tempBlobFile, err := ioutil.TempFile("", id)
	if err != nil {
		return nil, 0, errorspkg.Wrap(err, "creating temporary blob file")
	}

	_, err = io.Copy(tempBlobFile, bytes.NewReader(blobContent))
	if err != nil {
		logger.Error("failed-copying-blob-to-cache", err)
		c.cleanupCorrupted(logger, id)
	}

	if err := os.Rename(tempBlobFile.Name(), c.blobPath(id)); err != nil {
		if !os.IsExist(err) {
			return nil, 0, errorspkg.Wrap(err, "creating cached blob file")
		}
	}

	return blobContent, size, nil
}

func (c *CacheDriver) cleanupCorrupted(logger lager.Logger, id string) {
	logger.Debug("cleaning-up-corrupted")
	if err := os.Remove(c.blobPath(id)); err != nil {
		logger.Error("failed cleaning up corrupted state: %s", err)
	}
}

func (c *CacheDriver) Clean(logger lager.Logger) error {
	logger = logger.Session("cache-driver-clean")
	logger.Info("starting")
	defer logger.Info("ending")

	cachePath := path.Join(c.storePath, "cache")
	contents, err := ioutil.ReadDir(cachePath)
	if err != nil {
		return errorspkg.Wrap(err, "reading cache contents")
	}

	totalBlobs := len(contents)
	for i, cachedBlob := range contents {
		logger.Debug("cleaning-up-blob", lager.Data{"blob": cachedBlob.Name(), "count": fmt.Sprintf("%d/%d", i, totalBlobs)})

		if err := os.Remove(path.Join(cachePath, cachedBlob.Name())); err != nil {
			return errorspkg.Wrap(err, "clean up blob `%s`")
		}
	}

	return nil
}

func (c *CacheDriver) blobPath(id string) string {
	return filepath.Join(c.storePath, store.CacheDirName, id)
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
		return false, errorspkg.Errorf("blob `%s` exists but it's not a regular file", blobPath)
	}

	return true, nil
}
