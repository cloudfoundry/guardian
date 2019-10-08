package mount

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

type RootlessUnmounter struct {
}

func (u RootlessUnmounter) Unmount(path string) error {
	mounted, err := isMountPoint(path)
	if err != nil {
		return err
	}

	if mounted {
		return fmt.Errorf("%q is a mountpoint and cannot be unmounted when running rootless", path)
	}

	return nil
}

func isMountPoint(path string) (bool, error) {
	// append trailing slash to force symlink traversal; symlinking e.g. 'cpu'
	// to 'cpu,cpuacct' is common
	cmd := exec.Command("/bin/mountpoint", path+"/")
	cmdOutput, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}

	// According to the mountpoint command implementation, an error means
	// that the path either does not exist, or is not a mountpoint
	if bytes.Contains(cmdOutput, []byte("is not a mountpoint")) {
		return false, nil
	}

	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return false, nil
	}

	return false, err
}
