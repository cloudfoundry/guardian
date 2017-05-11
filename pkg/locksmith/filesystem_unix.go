// +build !windows

package locksmith

import (
	"fmt"
	"os"
	"syscall"
)

var FlockSyscall = syscall.Flock

type FileSystem struct {
}

func NewFileSystem() *FileSystem {
	return &FileSystem{}
}

type fileSystemUnlocker struct {
	f *os.File
}

func (u *fileSystemUnlocker) Unlock() error {
	defer u.f.Close()
	fd := int(u.f.Fd())
	return FlockSyscall(fd, syscall.LOCK_UN)
}

func (l *FileSystem) Lock(path string) (Unlocker, error) {
	lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("creating lock file for path `%s`: %s", path, err)
	}

	fd := int(lockFile.Fd())
	if err := FlockSyscall(fd, syscall.LOCK_EX); err != nil {
		return nil, err
	}

	return &fileSystemUnlocker{f: lockFile}, nil
}
