package store

import (
	"syscall"

	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

type StoreMeasurer struct {
	storePath string
}

func NewStoreMeasurer(storePath string) *StoreMeasurer {
	return &StoreMeasurer{
		storePath: storePath,
	}
}

func (s *StoreMeasurer) MeasureStore(logger lager.Logger) (int64, error) {
	logger = logger.Session("measuring-store", lager.Data{"storePath": s.storePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	usage, err := s.measurePath(s.storePath)
	if err != nil {
		return 0, err
	}
	logger.Debug("store-usage", lager.Data{"bytes": usage})
	return usage, nil
}

func (s *StoreMeasurer) measurePath(path string) (int64, error) {
	stats := syscall.Statfs_t{}
	err := syscall.Statfs(path, &stats)
	if err != nil {
		return 0, errorspkg.Wrapf(err, "Invalid path %s", path)
	}

	bsize := uint64(stats.Bsize)
	free := stats.Bfree * bsize
	total := stats.Blocks * bsize

	return int64(total - free), nil
}
