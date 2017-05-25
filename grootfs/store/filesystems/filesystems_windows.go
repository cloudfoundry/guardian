package filesystems

import "errors"

func CheckFSPath(path string, filesystem string, mountOptions ...string) error {
	return errors.New("Implement me")
}
