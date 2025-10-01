package locksmith // import "code.cloudfoundry.org/grootfs/store/locksmith"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager/v3"
	errorspkg "github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	ExclusiveMetricsLockingTime = "ExclusiveLockingTime"
	SharedMetricsLockingTime    = "SharedLockingTime"
)

type FileSystem struct {
	locksDir       string
	metricsEmitter groot.MetricsEmitter
	lockType       int
	metricName     string
	FlockSyscall   func(fd int, how int) (err error)
}

func NewExclusiveFileSystem(locksDir string) *FileSystem {
	return &FileSystem{
		locksDir:     locksDir,
		lockType:     unix.LOCK_EX,
		metricName:   ExclusiveMetricsLockingTime,
		FlockSyscall: unix.Flock,
	}
}

func NewSharedFileSystem(locksDir string) *FileSystem {
	return &FileSystem{
		locksDir:     locksDir,
		lockType:     unix.LOCK_SH,
		metricName:   SharedMetricsLockingTime,
		FlockSyscall: unix.Flock,
	}
}

func (l *FileSystem) WithMetrics(e groot.MetricsEmitter) *FileSystem {
	l.metricsEmitter = e
	return l
}

func (l *FileSystem) LockWithTimeout(key string, timeout time.Duration) (*os.File, error) {
	gotLock := make(chan string, 1)
	var file *os.File
	var err error

	go func() {
		file, err = l.Lock(key)
		gotLock <- "got the lock!"
	}()

	select {
	case <-gotLock:
		return file, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timed out waiting for the '%s' file lock after '%vs'", key, timeout.Seconds())
	}
}

func (l *FileSystem) Lock(key string) (*os.File, error) {
	if l.metricsEmitter != nil {
		defer l.metricsEmitter.TryEmitDurationFrom(lager.NewLogger("nil"), l.metricName, time.Now())
	}

	if err := os.MkdirAll(l.locksDir, 0755); err != nil {
		return nil, err
	}
	key = strings.Replace(key, "/", "", -1)
	lockFile, err := os.OpenFile(l.path(key), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errorspkg.Wrapf(err, "creating lock file for key `%s`", key)
	}

	fd := int(lockFile.Fd())
	if err := l.FlockSyscall(fd, l.lockType); err != nil { // read here, goroutine 18
		return nil, err
	}

	return lockFile, nil
}

func (l *FileSystem) Unlock(lockFile *os.File) error {
	defer lockFile.Close()
	fd := int(lockFile.Fd())
	return l.FlockSyscall(fd, unix.LOCK_UN)
}

func (l *FileSystem) path(key string) string {
	return filepath.Join(l.locksDir, key+".lock")
}
