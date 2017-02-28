package locksmith // import "code.cloudfoundry.org/grootfs/store/locksmith"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"code.cloudfoundry.org/grootfs/store"
	errorspkg "github.com/pkg/errors"
)

var FlockSyscall = syscall.Flock

type FileSystem struct {
	storePath string
}

func NewFileSystem(storePath string) *FileSystem {
	return &FileSystem{
		storePath: storePath,
	}
}

func (l *FileSystem) Lock(key string) (*os.File, error) {
	key = strings.Replace(key, "/", "", -1)
	lockFile, err := os.OpenFile(l.path(key), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errorspkg.Wrapf(err, "creating lock file for key `%s`", key)
	}

	fd := int(lockFile.Fd())
	if err := FlockSyscall(fd, syscall.LOCK_EX); err != nil {
		return nil, err
	}

	return lockFile, nil
}

func (l *FileSystem) Unlock(lockFile *os.File) error {
	defer lockFile.Close()
	fd := int(lockFile.Fd())
	return FlockSyscall(fd, syscall.LOCK_UN)
}

func (l *FileSystem) path(key string) string {
	return filepath.Join(l.storePath, store.LocksDirName, fmt.Sprintf("%s.lock", key))
}
