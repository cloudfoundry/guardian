package store

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

//go:generate counterfeiter . VolumeDriver

type VolumeDriver interface {
	Volumes(logger lager.Logger) ([]string, error)
	VolumeSize(lager.Logger, string) (int64, error)
}

type StoreMeasurer struct {
	storePath    string
	volumeDriver VolumeDriver
}

func NewStoreMeasurer(storePath string, volumeDriver VolumeDriver) *StoreMeasurer {
	return &StoreMeasurer{
		storePath:    storePath,
		volumeDriver: volumeDriver,
	}
}

func (s *StoreMeasurer) Usage(logger lager.Logger) (int64, error) {
	logger = logger.Session("measuring-store", lager.Data{"storePath": s.storePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	_, used, err := s.pathStats(s.storePath)
	if err != nil {
		return 0, errorspkg.Wrapf(err, "Invalid path %s", s.storePath)
	}

	logger.Debug("store-usage", lager.Data{"bytes": used})
	return used, nil
}

func (s *StoreMeasurer) CacheUsage(logger lager.Logger, unusedVolumes []string) int64 {
	var size int64
	for _, volume := range unusedVolumes {
		volumeSize, err := s.volumeDriver.VolumeSize(logger, volume)
		if err != nil {
			logger.Error("fetching-volume-size-failed", err, lager.Data{"volume": volume})
			continue
		}
		size += volumeSize
	}

	return size
}

func (s *StoreMeasurer) CommittedQuota(logger lager.Logger) (int64, error) {
	logger = logger.Session("measuring-committed-size")
	logger.Debug("starting")
	defer logger.Debug("ending")

	var totalCommittedSpace int64
	imageDir := filepath.Join(s.storePath, "images")

	err := filepath.Walk(imageDir, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if os.IsNotExist(err) || fileInfo.Name() != "image_quota" {
			return nil
		}

		imageQuota, err := readImageQuota(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		totalCommittedSpace += imageQuota
		return nil
	})
	if err != nil {
		return -1, err
	}

	return totalCommittedSpace, nil
}

func readImageQuota(imageQuotaFilePath string) (int64, error) {
	imageQuotaBytes, err := ioutil.ReadFile(imageQuotaFilePath)
	if err != nil {
		return 0, err
	}

	imageQuota, err := strconv.ParseInt(string(imageQuotaBytes), 10, 64)
	if err != nil {
		return 0, err
	}

	return imageQuota, nil
}

func (s *StoreMeasurer) pathStats(path string) (totalBytes, UsedBytes int64, err error) {
	stats := syscall.Statfs_t{}
	if err = syscall.Statfs(s.storePath, &stats); err != nil {
		return 0, 0, errorspkg.Wrapf(err, "Invalid path %s", s.storePath)
	}

	bsize := uint64(stats.Bsize)
	free := stats.Bfree * bsize
	total := stats.Blocks * bsize
	used := int64(total - free)

	return int64(total), used, nil
}
