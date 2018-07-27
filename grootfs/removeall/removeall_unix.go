package removeall

import (
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func RemoveAll(path string) error {
	// What if I am removing "." or ".." -> Not allowed
	// What if I am removing "/"
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	parentPath := filepath.Dir(path)

	base := filepath.Base(path)

	parent, err := os.Open(parentPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer parent.Close()

	return removeAll(parent, base)
}

func removeAll(parentFile *os.File, path string) error {
	parentFd := int(parentFile.Fd())
	err := unix.Unlinkat(parentFd, path, 0)
	if err == nil || err == unix.ENOENT {
		return nil
	}

	if err != unix.EISDIR && err != unix.EPERM {
		return err
	}

	fd, err := unix.Openat(parentFd, path, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}

	file := os.NewFile(uintptr(fd), path)

	recurseErr := removeDirEntries(file)

	file.Close()

	unlinkError := unix.Unlinkat(parentFd, path, unix.AT_REMOVEDIR)
	if unlinkError == nil {
		return nil
	}
	if recurseErr != nil {
		return recurseErr
	}
	return unlinkError
}

func removeDirEntries(file *os.File) error {
	for {
		names, readErr := file.Readdirnames(1024)
		var removeErr error
		for _, name := range names {
			err := removeAll(file, name)
			if err != nil {
				removeErr = err
			}
		}
		if readErr == io.EOF {
			return removeErr
		}
		if len(names) == 0 {
			return readErr
		}
	}
}
