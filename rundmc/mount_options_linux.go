package rundmc

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/docker/pkg/mount"
)

func (g MountOptionsGetter) GetMountOptions(path string) ([]string, error) {
	return g(path)
}

func GetMountOptions(path string) ([]string, error) {
	if err := verifyExistingDirectory(path); err != nil {
		return nil, err
	}

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

func verifyExistingDirectory(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	return nil
}
