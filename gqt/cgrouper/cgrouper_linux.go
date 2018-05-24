package cgrouper

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func CleanGardenCgroups(cgroupsRootPath, tag string) error {
	subsystems, err := ioutil.ReadDir(cgroupsRootPath)
	if err != nil {
		return err
	}

	for _, subsystem := range subsystems {
		if err := unmountIfExists(filepath.Join(cgroupsRootPath, subsystem.Name())); err != nil {
			return err
		}
	}

	return unmountIfExists(cgroupsRootPath)
}

func unmountIfExists(unmountPath string) error {
	unmountErr := unix.Unmount(unmountPath, unix.MNT_FORCE)
	if unmountErr != nil && !os.IsNotExist(unmountErr) {
		return fmt.Errorf("Failed to unmount %s: %s", unmountPath, unmountErr)
	}

	return nil
}
