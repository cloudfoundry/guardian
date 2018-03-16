package rundmc

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const no_device = -1

func (c MountPointChecker) IsMountPoint(path string) (bool, error) {
	return c(path)
}

func IsMountPoint(path string) (bool, error) {
	dev, err := getDeviceForFile(path)
	if dev == no_device || err != nil {
		return false, err
	}

	parentDev, err := getDeviceForFile(filepath.Dir(path))
	if parentDev == no_device || err != nil {
		return false, err
	}

	return dev != parentDev, nil
}

func getDeviceForFile(path string) (int64, error) {
	var info os.FileInfo
	var err error
	if info, err = os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return no_device, nil
		}
		return 0, fmt.Errorf("failed to stat %s", path)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("failed to convert to system stat %s", path)
	}
	return int64(stat.Dev), nil
}
