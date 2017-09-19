package store

import (
	"bytes"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

func (s *StoreMeasurer) Size(logger lager.Logger) (int64, error) {
	logger = logger.Session("measuring-size")
	logger.Debug("starting")
	defer logger.Debug("ending")

	total, _, err := s.pathStats(s.storePath)
	if err != nil {
		return 0, errorspkg.Wrapf(err, "Invalid path %s", s.storePath)
	}

	return total, nil
}

func (s *StoreMeasurer) CommittedSize(logger lager.Logger) (int64, error) {
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
		if err != nil {
			logger.Debug("WARNING-cannot-read-image-quota-ignoring", lager.Data{"imagePath": filepath.Join(imageDir, file.Name()), "error": err})
			continue
		}

		totalCommittedSpace += imageQuota
	}

	return totalCommittedSpace, nil
}

func (s *StoreMeasurer) Cache(logger lager.Logger) (int64, error) {
	logger = logger.Session("measuring-cache", lager.Data{"storePath": s.storePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	var cacheSize int64

	volumes, _ := s.volumeDriver.Volumes(logger)
	volumesSize, err := s.calculateTotalVolumesSize(logger, volumes)
	if err != nil {
		return 0, err
	}
	cacheSize += volumesSize

	for _, subdirectory := range []string{MetaDirName, TempDirName} {
		subdirSize, err := duUsage(filepath.Join(s.storePath, subdirectory))
		if err != nil {
			return 0, err
		}
		cacheSize += subdirSize
	}

	logger.Debug("cache-usage", lager.Data{"bytes": cacheSize})
	return cacheSize, nil
}

func (s *StoreMeasurer) PurgeableCache(logger lager.Logger, unusedVolumes []string) (int64, error) {
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

func duUsage(path string) (int64, error) {
	cmd := exec.Command("du", "-bs", path)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	stderrBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = stderrBuffer
	if err := cmd.Run(); err != nil {
		return 0, errorspkg.Wrapf(err, "du failed: %s", stderrBuffer.String())
	}

	usageString := strings.Split(stdoutBuffer.String(), "\t")[0]
	return strconv.ParseInt(usageString, 10, 64)
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
