package filesystems

import "errors"

func CheckFSPath(path string, expectedFilesystem int64, mountOptions ...string) error {
	return errors.New("Implement me")
}
