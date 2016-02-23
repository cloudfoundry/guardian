package runrunc

import (
	"fmt"
	"os"
)

var ChownFunc func(string, int, int) error = os.Chown

type DirectoryCreator struct {
}

func (d DirectoryCreator) MkdirAs(path string, mode os.FileMode, uid, gid int) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if err := os.MkdirAll(path, mode); err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	return ChownFunc(path, uid, gid)
}
