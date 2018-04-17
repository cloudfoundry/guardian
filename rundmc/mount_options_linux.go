package rundmc

import (
	"strings"

	"github.com/docker/docker/pkg/mount"
)

func (g MountOptionsGetter) GetMountOptions(path string, mountInfos []*mount.Info) ([]string, error) {
	return g(path, mountInfos)
}

func GetMountOptions(path string, mountInfos []*mount.Info) ([]string, error) {
	mountInfo, err := getMountInfo(path, mountInfos)
	if err != nil {
		return nil, err
	}

	if mountInfo == nil {
		return []string{}, nil
	}

	return strings.Split(mountInfo.Opts, ","), nil
}

func getMountInfo(path string, mountInfos []*mount.Info) (*mount.Info, error) {
	for _, info := range mountInfos {
		if info.Mountpoint == path {
			return info, nil
		}
	}

	return nil, nil
}
