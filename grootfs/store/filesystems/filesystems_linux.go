package filesystems

import (
	"bufio"
	"os"
	"strings"
	"syscall"

	errorspkg "github.com/pkg/errors"
)

func CheckFSPath(path string, expectedFilesystem int64, mountOptions ...string) error {
	statfs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &statfs)
	if err != nil {
		return errorspkg.Wrapf(err, "Failed to detect type of filesystem")
	}

	if statfs.Type != expectedFilesystem {
		return errorspkg.Errorf("Store path filesystem (%s) is incompatible with requested driver", path)
	}

	return checkMountOptions(path, mountOptions...)
}

func checkMountOptions(path string, options ...string) error {
	mounts, err := os.Open("/proc/mounts")
	if err != nil {
		return errorspkg.Errorf("Failed to open /proc/mounts: %s", err.Error())
	}

	scanner := bufio.NewScanner(mounts)
	for scanner.Scan() {
		mountPoint := scanner.Text()
		if !strings.Contains(mountPoint, path) {
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
