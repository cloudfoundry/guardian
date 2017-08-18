package locksmith // import "code.cloudfoundry.org/grootfs/store/locksmith"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const ExclusiveMetricsLockingTime = "ExclusiveLockingTime"
const SharedMetricsLockingTime = "SharedLockingTime"

type FileSystem struct {
	storePath      string
	metricsEmitter groot.MetricsEmitter
	lockType       int
	metricName     string
}

func NewExclusiveFileSystem(storePath string, metricsEmitter groot.MetricsEmitter) *FileSystem {
	return &FileSystem{
		storePath:      storePath,
		metricsEmitter: metricsEmitter,
		lockType:       syscall.LOCK_EX,
		metricName:     ExclusiveMetricsLockingTime,
	}
}

func NewSharedFileSystem(storePath string, metricsEmitter groot.MetricsEmitter) *FileSystem {
	return &FileSystem{
		storePath:      storePath,
		metricsEmitter: metricsEmitter,
		lockType:       syscall.LOCK_SH,
		metricName:     SharedMetricsLockingTime,
	}
}

var FlockSyscall = syscall.Flock

func (l *FileSystem) Lock(key string) (*os.File, error) {
	defer l.metricsEmitter.TryEmitDurationFrom(lager.NewLogger("nil"), l.metricName, time.Now())

	key = strings.Replace(key, "/", "", -1)
	lockFile, err := os.OpenFile(l.path(key), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errorspkg.Wrapf(err, "creating lock file for key `%s`", key)
	}

	fd := int(lockFile.Fd())
	if err := FlockSyscall(fd, l.lockType); err != nil {
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
