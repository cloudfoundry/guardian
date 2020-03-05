package preparerootfs

import (
	"fmt"
	"os"
	"path/filepath"
)

type SymlinkRefusingFileCreator struct{}

func (SymlinkRefusingFileCreator) CreateFiles(dir string, files ...string) error {
	for _, file := range files {
		if err := touchFile(filepath.Join(dir, file)); err != nil {
			return err
		}
	}
	return nil
}

func touchFile(path string) error {
	if err := mkdirRefusingSymlinks(filepath.Dir(path)); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE, 0444)
	if err != nil {
		return err
	}
	return file.Close()
}

func mkdirRefusingSymlinks(path string) error {
	err := statNonSymlink(path)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	parentPath := filepath.Dir(path)
	err = statNonSymlink(parentPath)
	if err != nil {
		if os.IsNotExist(err) {
			if recurseErr := mkdirRefusingSymlinks(parentPath); recurseErr != nil {
				return recurseErr
			}
		} else {
			return err
		}
	}

	return os.Mkdir(path, 0755)
}

func statNonSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink", path)
	}
	return nil
}
