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
//go:generate counterfeiter . UnusedVolumeGetter

type VolumeDriver interface {
	VolumeSize(lager.Logger, string) (int64, error)
	Volumes(lager.Logger) ([]string, error)
}

type UnusedVolumeGetter interface {
	UnusedVolumes(lager.Logger) ([]string, error)
}

type StoreMeasurer struct {
	storePath          string
	volumeDriver       VolumeDriver
	unusedVolumeGetter UnusedVolumeGetter
}

func NewStoreMeasurer(storePath string, volumeDriver VolumeDriver, unusedVolumeGetter UnusedVolumeGetter) *StoreMeasurer {
	return &StoreMeasurer{
		storePath:          storePath,
		volumeDriver:       volumeDriver,
		unusedVolumeGetter: unusedVolumeGetter,
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

func (s *StoreMeasurer) UnusedVolumesSize(logger lager.Logger) (int64, error) {
	unusedVols, err := s.unusedVolumeGetter.UnusedVolumes(logger)
	if err != nil {
		return 0, err
	}
	return s.countVolumesSize(logger, unusedVols)
}

func (s *StoreMeasurer) TotalVolumesSize(logger lager.Logger) (int64, error) {
	vols, err := s.volumeDriver.Volumes(logger)
	if err != nil {
		return 0, err
	}
	return s.countVolumesSize(logger, vols)
}

func (s *StoreMeasurer) countVolumesSize(logger lager.Logger, volumes []string) (int64, error) {
	var size int64
	for _, volume := range volumes {
		volumeSize, err := s.volumeDriver.VolumeSize(logger, volume)
		if err != nil && !os.IsNotExist(err) {
			return 0, err
		}
		size += volumeSize
	}

	return size, nil
}

func (s *StoreMeasurer) CommittedQuota(logger lager.Logger) (int64, error) {
	logger = logger.Session("measuring-committed-size")
	logger.Debug("starting")
	defer logger.Debug("ending")

	var totalCommittedSpace int64

	imageDir := filepath.Join(s.storePath, "images")
	files, err := ioutil.ReadDir(imageDir)
	if err != nil {
		return 0, errorspkg.Wrapf(err, "Cannot list images in %s", imageDir)
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		imageQuota, err := readImageQuota(filepath.Join(imageDir, file.Name()))
		if err != nil && !os.IsNotExist(err) {
			return 0, err
		}

		totalCommittedSpace += imageQuota
	}

	return totalCommittedSpace, nil
}

func readImageQuota(imageDir string) (int64, error) {
	quotaFilePath := filepath.Join(imageDir, "image_quota")
	imageQuotaBytes, err := ioutil.ReadFile(quotaFilePath)
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
