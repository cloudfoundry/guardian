package mount

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/sys/unix"
)

type RootfulUnmounter struct {
}

func (u RootfulUnmounter) Unmount(log lager.Logger, path string) error {
	var err error
	log = log.Session("rootful-unmounter", lager.Data{"path": path})
	log.Debug("start")
	defer log.Debug("finish")

	for i := 0; i < 50; i++ {
		err = unix.Unmount(path, 0)
		if err == nil {
			return nil
		}

		// do not error if unmountPath does not exist or is not a mountpoint
		if os.IsNotExist(err) {
			log.Debug("unmount-path-does-not-exist")
			return nil
		}
		if err == unix.EINVAL {
			log.Debug("unmount-path-not-a-mountpoint")
			return nil
		}

		log.Debug("retrying-to-unmount-path", lager.Data{"attempt-number": i + 1})
		time.Sleep(time.Millisecond * 100)
	}

	return err
}
