package store

import (
	"syscall"

	errorspkg "github.com/pkg/errors"
)

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
