package mount

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

type RootfulUnmounter struct {
}

func (u RootfulUnmounter) Unmount(path string) error {
	var err error

	for i := 0; i < 50; i++ {
		err = unix.Unmount(path, 0)
		// do not error if unmountPath does not exist or is not a mountpoint
		if os.IsNotExist(err) || err == unix.EINVAL {
			return nil
		}

		if err == nil {
			return nil
		}

		time.Sleep(time.Millisecond * 100)
	}

	return err
}
