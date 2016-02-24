package runrunc

import (
	"os"
	"path/filepath"
)

var ChownFunc func(string, int, int) error = os.Chown

type DirectoryCreator struct {
}

func (d DirectoryCreator) MkdirAs(path string, mode os.FileMode, uid, gid int) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if err := d.MkdirAs(filepath.Dir(path), mode, uid, gid); err != nil {
		return err
	}

	if err := os.Mkdir(path, mode); err != nil {
		return err
	}

	return ChownFunc(path, uid, gid)
}
