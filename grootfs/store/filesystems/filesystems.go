package filesystems

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	errorspkg "github.com/pkg/errors"
)

const (
	// Full list of file system type codes can be found here: http://man7.org/linux/man-pages/man2/statfs.2.html
	XfsType = int64(0x58465342)
)

func CheckFSPath(path string, filesystem string, mountOptions ...string) error {
	statfs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &statfs)
	if err != nil {
		return errorspkg.Wrapf(err, "Failed to detect type of filesystem")
	}

	fsType, err := filesystemCode(filesystem)
	if err != nil {
		return err
	}

	if statfs.Type != fsType {
		return errorspkg.Errorf("Store path filesystem (%s) is incompatible with native driver (must be XFS mountpoint); expected type (hex): %x, actual type (hex): %x", path, fsType, statfs.Type)
	}

	return checkMountOptions(path, filesystem, mountOptions...)
}

func checkMountOptions(path, filesystem string, options ...string) error {
	mounts, err := os.Open("/proc/mounts")
	if err != nil {
		return errorspkg.Errorf("Failed to open /proc/mounts: %s", err.Error())
	}

	scanner := bufio.NewScanner(mounts)
	for scanner.Scan() {
		mountPoint := scanner.Text()
		if !strings.Contains(mountPoint, fmt.Sprintf("%s %s", filesystem, path)) {
			continue
		}

		for _, option := range options {
			if !strings.Contains(mountPoint, option) {
				return errorspkg.Errorf("'%s' option missing at the mount point '%s'", option, mountPoint)
			}
		}

		return nil
	}

	return nil
}

func filesystemCode(filesystem string) (int64, error) {
	switch filesystem {
	case "xfs":
		return XfsType, nil
	default:
		return 0, errorspkg.Errorf("filesystem %s is not supported", filesystem)
	}
}
