package filesystems

import (
	"fmt"
	"syscall"

	"github.com/pkg/errors"
)

const (
	XfsType   = 0x58465342
	BtrfsType = 0x9123683E
)

func CheckFSPath(path string, expectedFilesystem int64, expectedFilesystemName string) error {
	statfs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &statfs)
	if err != nil {
		return errors.Wrapf(err, "Failed to detect type of filesystem")
	}

	if statfs.Type != expectedFilesystem {
		return fmt.Errorf("filesystem driver requires store filesystem to be %s", expectedFilesystemName)
	}
	return nil
}
