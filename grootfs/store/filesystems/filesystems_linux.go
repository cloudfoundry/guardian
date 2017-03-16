package filesystems

import (
	"syscall"

	errorspkg "github.com/pkg/errors"
)

func CheckFSPath(path string, expectedFilesystem int64) error {
	statfs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &statfs)
	if err != nil {
		return errorspkg.Wrapf(err, "Failed to detect type of filesystem")
	}

	if statfs.Type != expectedFilesystem {
		return errorspkg.Errorf("store path filesystem is incompatible (%s)", path)
	}
	return nil
}
