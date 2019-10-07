package mount

import (
	"os"

	"golang.org/x/sys/unix"
)

type RootfulUnmounter struct {
}

func (u RootfulUnmounter) Unmount(path string) error {
	err := unix.Unmount(path, 0)
	// do not error if unmountPath does not exist or is not a mountpoint
	if os.IsNotExist(err) || err == unix.EINVAL {
		return nil
	}
	return err
}

func (u RootfulUnmounter) IsRootless() bool {
	return false
}
