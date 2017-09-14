package store

import (
	"bytes"
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

func (s *StoreMeasurer) Cache(logger lager.Logger) (int64, error) {
	logger = logger.Session("measuring-cache", lager.Data{"storePath": s.storePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	var cacheSize int64

	volumes, _ := s.volumeDriver.Volumes(logger)
	for _, volume := range volumes {
		volumeSize, err := s.volumeDriver.VolumeSize(logger, volume)
		if err != nil {
			return 0, err
		}
		cacheSize += volumeSize
	}

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
