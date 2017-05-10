package locksmith // import "code.cloudfoundry.org/grootfs/store/locksmith"

import (
	"errors"
	"os"

	"code.cloudfoundry.org/grootfs/groot"
)

func NewExclusiveFileSystem(storePath string, metricsEmitter groot.MetricsEmitter) *FileSystem {
	return &FileSystem{}
}

func NewSharedFileSystem(storePath string, metricsEmitter groot.MetricsEmitter) *FileSystem {
	return &FileSystem{}
}

func (l *FileSystem) Lock(key string) (*os.File, error) {
	return nil, errors.New("Implement me")
}

func (l *FileSystem) Unlock(lockFile *os.File) error {
	return errors.New("Implement me")
}
