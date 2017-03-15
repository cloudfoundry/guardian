package locksmith // import "code.cloudfoundry.org/grootfs/store/locksmith"

import (
	"errors"
	"os"
)

func (l *FileSystem) Lock(key string) (*os.File, error) {
	return nil, errors.New("Implement me")
}

func (l *FileSystem) Unlock(lockFile *os.File) error {
	return errors.New("Implement me")
}
