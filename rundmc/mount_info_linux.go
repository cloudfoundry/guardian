package rundmc

import (
	"fmt"
	"strings"

	"github.com/docker/docker/pkg/mount"
)

func (g MountOptionsGetter) GetMountOptions(path string) ([]string, error) {
	return g(path)
}

func (c MountPointChecker) IsMountPoint(path string) (bool, error) {
	return c(path)
}

func IsMountPoint(path string) (bool, error) {
	mountInfo, err := getMountInfo(path)
	if err != nil {
		return false, err
	}
	return mountInfo != nil, nil
}

func GetMountOptions(path string) ([]string, error) {
	mountInfo, err := getMountInfo(path)
	if err != nil {
		return nil, err
	}

	if mountInfo == nil {
		return nil, fmt.Errorf("%s is not a mount point", path)
	}

	return strings.Split(mountInfo.Opts, ","), nil
}

func getMountInfo(path string) (*mount.Info, error) {
	mountInfos, err := mount.GetMounts()
	if err != nil {
		return nil, err
	}

	for _, info := range mountInfos {
		if info.Mountpoint == path {
			return info, nil
		}
	}

	return nil, nil
}
