package store

import (
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

func (s *StoreMeasurer) CacheUsage(logger lager.Logger, unusedVolumes []string) (int64, error) {
	volumesSize, err := s.calculateTotalVolumesSize(logger, unusedVolumes)
	if err != nil {
		return 0, err
	}

	return volumesSize, nil
}

func (s *StoreMeasurer) calculateTotalVolumesSize(logger lager.Logger, volumes []string) (int64, error) {
	var size int64
	for _, volume := range volumes {
		volumeSize, err := s.volumeDriver.VolumeSize(logger, volume)
		if err != nil {
			return 0, err
		}
		size += volumeSize
	}

	return size, nil
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
