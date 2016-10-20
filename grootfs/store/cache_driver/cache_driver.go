package cache_driver // import "code.cloudfoundry.org/grootfs/store/cache_driver"

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
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

func (c *CacheDriver) StreamBlob(logger lager.Logger, id string,
	blobFunc fetcher.RemoteBlobFunc,
) (io.ReadCloser, int64, error) {
	logger = logger.Session("getting-blob-from-cache", lager.Data{"blobID": id})
	logger.Info("start")
	defer logger.Info("end")

	hasBlob, err := c.hasBlob(id)
	if err != nil {
		return nil, 0, fmt.Errorf("checking if the blob exists: %s", err)
	}

	var (
		blobFile *os.File
		reader   *os.File
	)

	defer func() {
		if err != nil {
			logger.Debug("cleaning-up-corrupted")
			if err = os.Remove(c.blobPath(id)); err != nil {
				logger.Error("failed cleaning up corrupted state: %s", err)
			}
		}
	}()

	if hasBlob {
		logger.Debug("cache-hit")
		reader, err = os.Open(c.blobPath(id))
		if err != nil {
			return nil, 0, fmt.Errorf("accessing the cached blob: %s", err)
		}
		stat, err := os.Stat(c.blobPath(id))
		if err != nil {
			return nil, 0, fmt.Errorf("acessing cached blob stat: %s", err)
		}
		return reader, stat.Size(), nil
	}

	logger.Debug("cache-miss")

	blobContent, size, err := blobFunc(logger)
	if err != nil {
		return nil, 0, err
	}

	blobFile, err = os.OpenFile(c.blobPath(id), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, 0, fmt.Errorf("creating cached blob file: %s", err)
	}

	var rEnd, wEnd *os.File
	rEnd, wEnd, err = os.Pipe()
	if err != nil {
		return nil, 0, fmt.Errorf("creating pipe: %s", err)
	}
	go func() {
		defer wEnd.Close()
		defer blobFile.Close()

		_, err := io.Copy(io.MultiWriter(wEnd, blobFile), bytes.NewReader(blobContent))
		if err != nil {
			logger.Error("failed-copying-blob-to-cache", err)

			if err = os.RemoveAll(blobFile.Name()); err != nil {
				logger.Error("failed cleaning up corrupted state: %s", err)
			}
		}
	}()

	return rEnd, size, nil
}

func (c *CacheDriver) Clean(logger lager.Logger) error {
	logger = logger.Session("cache-driver-clean")
	logger.Info("start")
	defer logger.Info("end")

	blobsPath := path.Join(c.storePath, "cache", "blobs")
	contents, err := ioutil.ReadDir(blobsPath)
	if err != nil {
		return fmt.Errorf("reading cache contents: %s", err.Error())
	}

	totalBlobs := len(contents)
	for i, cachedBlob := range contents {
		logger.Debug("cleaning-up-blob", lager.Data{"blob": cachedBlob.Name(), "count": fmt.Sprintf("%d/%d", i, totalBlobs)})

		if err := os.Remove(path.Join(blobsPath, cachedBlob.Name())); err != nil {
			return fmt.Errorf("clean up blob `%s`: %s", cachedBlob.Name(), err.Error())
		}
	}

	return nil
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
